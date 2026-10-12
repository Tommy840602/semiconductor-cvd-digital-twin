package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"
)

// Incidents live in the equipment checkpoint, independently of delivery/Redis.
// Source is immutable; result and return evidence advance explicitly.
type Incident struct {
	ConfirmedAt    *time.Time        `json:"confirmedAt,omitempty"`
	ReturnedAt     *time.Time        `json:"returnedAt,omitempty"`
	ID             string            `json:"id"`
	EquipmentID    string            `json:"equipmentId"`
	PMID           string            `json:"pmId"`
	WaferID        string            `json:"waferId"`
	LotID          string            `json:"lotId"`
	SourceRunID    string            `json:"sourceRunId"`
	Source         *ProcessTelemetry `json:"source,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
	Status         string            `json:"status"`
	ExecutionID    string            `json:"executionId"`
	ResultVersion  int               `json:"resultVersion"`
	ConfirmationID string            `json:"confirmationId"`
	Returned       bool              `json:"returned"`
	Released       bool              `json:"released"`
	Legacy         bool              `json:"legacy"`
}
type ReleaseEvidence struct {
	IncidentID     string `json:"incidentId"`
	ExecutionID    string `json:"executionId"`
	ResultVersion  int    `json:"resultVersion"`
	ConfirmationID string `json:"confirmationId"`
}
type DispositionAction struct {
	EquipmentID     string            `json:"equipmentId"`
	IncidentID      string            `json:"incidentId"`
	ExecutionID     string            `json:"executionId"`
	ResultVersion   int               `json:"resultVersion"`
	AuthorizationID string            `json:"authorizationId"`
	ReleaseID       string            `json:"releaseId"`
	Generation      int64             `json:"generation"`
	Evidence        []ReleaseEvidence `json:"evidence"`
	Reason          string            `json:"reason"`
}
type ReleaseResult struct {
	ReleaseID  string    `json:"releaseId"`
	Generation int64     `json:"generation"`
	Status     string    `json:"status"`
	Reason     string    `json:"reason"`
	At         time.Time `json:"at"`
}

func (e *Engine) dispositionInit() {
	if e.Incidents == nil {
		e.Incidents = map[string]*Incident{}
	}
	if e.Releases == nil {
		e.Releases = map[string]ReleaseResult{}
	}
	if e.Authorizations == nil {
		e.Authorizations = map[string]string{}
	}
}
func (e *Engine) activeIncident(pm, wafer string) *Incident {
	for _, i := range e.Incidents {
		if !i.Released && i.PMID == pm && i.WaferID == wafer {
			return i
		}
	}
	return nil
}
func (e *Engine) incidentFault(p *Chamber) *Incident {
	e.dispositionInit()
	i := e.activeIncident(p.ID, p.WaferID)
	if i == nil {
		i = &Incident{ID: fmt.Sprintf("%s-incident-%d", e.Epoch, e.Sequence+1), EquipmentID: e.EquipmentID, PMID: p.ID, WaferID: p.WaferID, SourceRunID: p.ProcessRunID, CreatedAt: e.Now}
		if p.Process != nil {
			copy := *p.Process
			i.Source = &copy
		}
		if w := e.wafer(p.WaferID); w != nil {
			i.LotID = w.LotID
		}
		e.Incidents[i.ID] = i
	}
	if r := e.Recoveries[i.ExecutionID]; r != nil && (r.Status == "awaiting_result" || r.Status == "returning") {
		r.Status = "failed"
		r.Reason = "結果失效：再次故障"
		r.ReasonCode = systemMessageCodes[r.Reason]
	}
	i.ResultVersion++
	i.Status = "awaiting_ee"
	i.ConfirmationID = ""
	i.ConfirmedAt = nil
	i.Returned = false
	e.HoldGeneration++
	e.DispositionHold = true
	e.DispatchPaused = true
	e.emit("disposition-exception", i.LotID, i.WaferID, *i)
	return i
}
func (e *Engine) dispositionReturns() {
	for _, i := range e.Incidents {
		if i.Released || i.WaferID == "" || i.Returned {
			continue
		}
		w := e.wafer(i.WaferID)
		l := e.lot(i.LotID)
		if w == nil || l == nil || w.Location != l.LP || !isTerminal(w) {
			continue
		}
		i.Returned = true
		at := e.Now
		i.ReturnedAt = &at
		if i.ConfirmationID != "" {
			i.Status = "resolved"
		} else {
			i.Status = "awaiting_result"
		}
		if r := e.Recoveries[i.ExecutionID]; r != nil {
			r.Status = i.Status
		}
		e.emit("disposition-returned", i.LotID, i.WaferID, *i)
	}
}
func (e *Engine) dispositionCommand(c Command) error {
	var a DispositionAction
	if err := verifyApproval(c, &a); err != nil {
		return err
	}
	if a.EquipmentID != e.EquipmentID {
		return errors.New("核准機台不符")
	}
	e.dispositionInit()
	if c.Type == "equipment-release" {
		return e.releaseEquipment(a)
	}
	if a.AuthorizationID == "" {
		return errors.New("缺少結果確認識別")
	}
	raw, _ := json.Marshal(a)
	binding := c.Type + string(raw)
	if old, ok := e.Authorizations[a.AuthorizationID]; ok {
		if old != binding {
			return errors.New("確認識別內容衝突")
		}
		return nil
	}
	i := e.Incidents[a.IncidentID]
	if i == nil || i.Released || i.ExecutionID != a.ExecutionID || i.ResultVersion != a.ResultVersion || i.Status != "awaiting_result" {
		return errors.New("處置結果已改變，需重新確認")
	}
	p := e.chamber(i.PMID)
	if (p == nil && !i.Legacy) || (p != nil && p.Fault) {
		return errors.New("PM 故障尚未解除")
	}
	r := e.Recoveries[i.ExecutionID]
	if r == nil {
		return errors.New("執行結果未知")
	}
	if c.Type == "disposition-reject" {
		if a.Reason == "" {
			return errors.New("請填否決原因")
		}
		if r.DispositionType == "scrap" || i.Returned {
			// Keep the same return evidence for PE to review again; never reload a wafer.
			r.Reason = a.Reason
			r.ReasonCode = ""
		} else {
			w := e.wafer(i.WaferID)
			if w == nil || w.Location != i.PMID {
				return errors.New("晶圓不在原 PM")
			}
			w.Result = "exception"
			w.ExceptionVersion++
			i.Status = "awaiting_ee"
			r.Status = "failed"
			r.Reason = a.Reason
			r.ReasonCode = ""
			i.ResultVersion++
			e.HoldGeneration++
		}
	} else if c.Type == "disposition-confirm" {
		if i.WaferID == "" {
			i.Returned = true
			i.Status = "resolved"
		} else {
			w := e.wafer(i.WaferID)
			if w == nil {
				return errors.New("晶圓不存在")
			}
			if r.DispositionType == "scrap" || i.Returned {
				if !i.Returned || w.Location != e.lot(i.LotID).LP {
					return errors.New("尚未回原 LP")
				}
				if r.DispositionType != "scrap" && !(r.DispositionType == "legacy_return" && i.Legacy) && (r.Result == nil || r.Result.Quality != "pass") {
					return errors.New("品質不合格")
				}
				i.Status = "resolved"
			} else {
				if w.Location != i.PMID || p.WaferID != w.ID || w.Result != "awaiting_result" || p.Process == nil || p.Process.ExecutionID != a.ExecutionID || p.Process.Quality != "pass" {
					return errors.New("晶圓或品質結果不符")
				}
				w.Result = "success"
				i.Status = "returning"
			}
		}
		i.ConfirmationID = a.AuthorizationID
		at := e.Now
		i.ConfirmedAt = &at
		r.Status = i.Status
		r.Reason = ""
		r.ReasonCode = systemMessageCodes[r.Reason]
	} else {
		return errors.New("未知結果動作")
	}
	e.Authorizations[a.AuthorizationID] = binding
	e.emit(c.Type, i.LotID, i.WaferID, map[string]any{"authorization": a, "incident": i})
	return nil
}
func (e *Engine) releaseEvidence() []ReleaseEvidence {
	out := []ReleaseEvidence{}
	for _, i := range e.Incidents {
		if !i.Released {
			out = append(out, ReleaseEvidence{i.ID, i.ExecutionID, i.ResultVersion, i.ConfirmationID})
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].IncidentID < out[b].IncidentID })
	return out
}
func (e *Engine) releaseEquipment(a DispositionAction) error {
	if a.ReleaseID == "" {
		return errors.New("缺少放行識別")
	}
	if old, ok := e.Releases[a.ReleaseID]; ok {
		if old.Status != "released" {
			return errors.New(old.Reason)
		}
		return nil
	}
	reject := func(reason string) error {
		e.Releases[a.ReleaseID] = ReleaseResult{a.ReleaseID, a.Generation, "invalidated", reason, e.Now}
		return errors.New(reason)
	}
	if !e.DispositionHold || a.Generation != e.HoldGeneration {
		return reject("異常集合已改變，需重新核准")
	}
	sort.Slice(a.Evidence, func(x, y int) bool { return a.Evidence[x].IncidentID < a.Evidence[y].IncidentID })
	if !reflect.DeepEqual(e.releaseEvidence(), a.Evidence) {
		return reject("放行處置集合或結果版本不符")
	}
	if e.Paused || e.Validate() != nil || (len(e.Events) > 0 && e.Now.Sub(e.Events[0].SourceTime) > 10*time.Minute) {
		return reject("設備時鐘／通訊／位置互鎖未滿足")
	}
	for _, p := range e.Chambers {
		if p.Fault {
			return reject("仍有 PM 故障")
		}
	}
	for _, i := range e.Incidents {
		if !i.Released && (i.Status != "resolved" || !i.Returned || i.ConfirmationID == "") {
			return reject("仍有未確認或未回收處置")
		}
	}
	for _, i := range e.Incidents {
		if !i.Released && i.WaferID != "" {
			w := e.wafer(i.WaferID)
			l := e.lot(i.LotID)
			if w == nil || l == nil || w.Location != l.LP || !isTerminal(w) {
				return reject("晶圓尚未回原槽位")
			}
		}
	}
	for _, r := range e.Recoveries {
		if r.Status == "approved_waiting" || r.Status == "executing" {
			return reject("仍有活動處置")
		}
	}
	for _, i := range e.Incidents {
		if !i.Released {
			i.Released = true
		}
	}
	e.DispositionHold = false
	e.DispatchPaused = e.maintenanceHeld() || e.VacuumHold
	for _, l := range e.Lots {
		if l.Phase != "complete" && (!e.maintenanceHeld() || l.Purpose == "dummy") {
			l.Paused = false
		}
	}
	result := ReleaseResult{a.ReleaseID, a.Generation, "released", "", e.Now}
	e.Releases[a.ReleaseID] = result
	e.emit("equipment-released", "", "", result)
	return nil
}

func (e *Engine) beginDisposition(r *Recovery) {
	i := e.Incidents[r.IncidentID]
	started := e.Now
	r.StartedAt = &started
	r.FinishedAt = &started
	i.ResultVersion++
	if r.DispositionType == "equipment" || r.DispositionType == "legacy_return" {
		r.Status = "awaiting_result"
		i.Status = "awaiting_result"
		i.Returned = true
	} else {
		p := e.chamber(r.PMID)
		w := e.wafer(r.WaferID)
		e.processResult(p, w, false, "scrapped")
		if p.Process != nil {
			copy := *p.Process
			r.Result = &copy
		}
		w.Result = "scrap"
		p.Remaining = 0
		r.Status = "returning"
		i.Status = "returning"
	}
	r.Reason = ""
	r.ReasonCode = systemMessageCodes[r.Reason]
	e.emit("disposition-executed", i.LotID, i.WaferID, *r)
}

// Migration preserves physical positions, executions, and old signatures. It
// only introduces missing holds; historical released cycles stay legacy.
func (e *Engine) migrateDispositions() {
	for _, p := range e.Chambers {
		w := e.wafer(p.WaferID)
		if p.Fault || (w != nil && (w.Result == "exception" || (p.Process != nil && p.Process.ExecutionID != ""))) {
			i := e.incidentFault(p)
			i.Legacy = true
			if p.Process != nil {
				if r := e.Recoveries[p.Process.ExecutionID]; r != nil {
					r.IncidentID = i.ID
					r.DispositionType = "supplement"
					i.ExecutionID = r.ExecutionID
					if r.Status == "executing" {
						i.Status = "executing"
					}
					if r.Status == "completed" && w != nil && w.Result == "success" {
						w.Result = "awaiting_result"
						r.Status = "awaiting_result"
						i.Status = "awaiting_result"
					}
				}
			}
		}
	}
	for _, r := range e.Recoveries {
		if r.IncidentID != "" {
			continue
		}
		if r.Status == "approved_waiting" {
			// Explicit cancellation fence prevents a delayed old approval starting.
			r.Status = "cancelled"
			r.Reason = "升級：舊待執行核准需补齊排錯資料並重新送審"
			r.ReasonCode = systemMessageCodes[r.Reason]
		}
		w := e.wafer(r.WaferID)
		if !e.DispatchPaused || w == nil || (r.Status != "completed" && r.Status != "failed") || w.Location != e.lot(w.LotID).LP {
			continue
		}
		e.HoldGeneration++
		e.DispositionHold = true
		id := fmt.Sprintf("%s-legacy-%s", e.Epoch, r.ExecutionID)
		e.Incidents[id] = &Incident{ID: id, EquipmentID: e.EquipmentID, PMID: r.PMID, WaferID: w.ID, LotID: w.LotID, SourceRunID: r.SourceRunID, Source: r.Result, CreatedAt: e.Now, Status: "awaiting_result", ExecutionID: r.ExecutionID, ResultVersion: 1, Returned: true, Legacy: true}
		r.IncidentID = id
		r.DispositionType = "supplement"
		if w.Result == "scrap" {
			r.DispositionType = "scrap"
			r.Reason = "舊版補做後已報廢回槽，保留原結果待 PE 確認"
			r.ReasonCode = systemMessageCodes[r.Reason]
		}
		r.Status = "awaiting_result"
	}
}

// Old direct scrap had no Recovery execution. A paused active lot with an
// already-returned abnormal wafer still needs disposition, without reloading it.
func (e *Engine) migrateLegacyReturned() {
	if !e.DispatchPaused {
		return
	}
	e.dispositionInit()
	for _, l := range e.Lots {
		if !l.Paused || l.Phase == "complete" {
			continue
		}
		for _, w := range l.Wafers {
			if w.Location != l.LP || !(w.Result == "scrap" || (w.Result == "success" && w.Attempts > 1)) {
				continue
			}
			seen := false
			for _, i := range e.Incidents {
				if i.WaferID == w.ID {
					seen = true
					break
				}
			}
			if seen {
				continue
			}
			id := fmt.Sprintf("%s-legacy-return-%s", e.Epoch, w.ID)
			i := &Incident{ID: id, EquipmentID: e.EquipmentID, WaferID: w.ID, LotID: l.ID, CreatedAt: e.Now, Status: "awaiting_ee", ResultVersion: 1, Returned: true, Legacy: true}
			e.Incidents[id] = i
			e.HoldGeneration++
			e.DispositionHold = true
			e.emit("disposition-legacy-return", l.ID, w.ID, *i)
		}
	}
}
