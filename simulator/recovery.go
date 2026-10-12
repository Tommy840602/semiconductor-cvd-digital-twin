package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"strings"
	"time"
)

// The signed payload is an immutable approved prescription. The same execution ID
// is retained through command retries, cancellation fences and checkpoints.
type Recovery struct {
	LegacyResult     string            `json:"legacyResult,omitempty"`
	IncidentID       string            `json:"incidentId"`
	DispositionType  string            `json:"dispositionType"`
	EquipmentID      string            `json:"equipmentId"`
	StartedAt        *time.Time        `json:"startedAt,omitempty"`
	FinishedAt       *time.Time        `json:"finishedAt,omitempty"`
	ExecutionID      string            `json:"executionId"`
	RequestID        string            `json:"requestId"`
	Revision         int               `json:"revision"`
	WaferID          string            `json:"waferId"`
	PMID             string            `json:"pmId"`
	SourceRunID      string            `json:"sourceRunId"`
	ExceptionVersion int               `json:"exceptionVersion"`
	Baseline         [5]float64        `json:"baseline"`
	StepSeconds      [4]int            `json:"stepSeconds"`
	Target           float64           `json:"targetThicknessNm"`
	RootRate         float64           `json:"rootRate"`
	Status           string            `json:"status"`
	Reason           string            `json:"reason"`
	ReasonCode       string            `json:"reasonCode,omitempty"`
	Result           *ProcessTelemetry `json:"result,omitempty"`
}

func verifyApproval(c Command, out any) error {
	raw, _ := c.Parameters["payload"].(string)
	signature, _ := c.Parameters["signature"].(string)
	key, err := os.ReadFile(os.Getenv("CVD_RECOVERY_KEY_FILE"))
	if err != nil || len(strings.TrimSpace(string(key))) < 32 {
		return errors.New("核准簽章金鑰不可用")
	}
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(string(key))))
	mac.Write([]byte(c.Type + "\n" + raw))
	sig, err := hex.DecodeString(signature)
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return errors.New("未核准或簽章無效")
	}
	if json.Unmarshal([]byte(raw), out) != nil {
		return errors.New("核准內容無效")
	}
	return nil
}
func signedRecovery(c Command) (*Recovery, error) {
	var r Recovery
	if err := verifyApproval(c, &r); err != nil {
		return nil, err
	}
	if r.ExecutionID == "" || r.RequestID == "" || r.Revision < 1 {
		return nil, errors.New("缺少核准綁定")
	}
	return &r, nil
}

func (e *Engine) recoveryCommand(c Command) error {
	r, err := signedRecovery(c)
	if err != nil {
		return err
	}
	if e.EquipmentID != "" && r.EquipmentID != e.EquipmentID {
		return errors.New("核准機台不符")
	}
	if e.Recoveries == nil {
		e.Recoveries = map[string]*Recovery{}
	}
	old := e.Recoveries[r.ExecutionID]
	if c.Type == "cancel-supplement" {
		if old != nil && old.Status != "approved_waiting" && old.Status != "cancelled" {
			return errors.New("補做已開始或結束，無法撤回")
		}
		if old == nil {
			old = r
			e.Recoveries[r.ExecutionID] = old
		}
		if i := e.Incidents[old.IncidentID]; i != nil && i.ExecutionID == old.ExecutionID {
			i.Status = "awaiting_ee"
			i.ExecutionID = ""
		}
		old.Status = "cancelled"
		old.Reason = "設備確認未開始並已取消"
		old.ReasonCode = systemMessageCodes[old.Reason]
		return nil
	}
	if old != nil {
		return nil
	}
	i := e.Incidents[r.IncidentID]
	if i == nil || i.Released || i.WaferID != r.WaferID || i.PMID != r.PMID || i.Status != "awaiting_ee" {
		return errors.New("異常來源綁定不符")
	}
	if r.DispositionType == "" {
		return errors.New("缺少處置類型")
	}
	if (c.Type == "wafer-supplement" && r.DispositionType != "supplement") || (c.Type == "wafer-scrap" && r.DispositionType != "scrap") || (c.Type == "equipment-check" && r.DispositionType != "equipment") || (c.Type == "legacy-return-check" && r.DispositionType != "legacy_return") {
		return errors.New("處置類型不符")
	}
	total := 0
	for _, v := range r.StepSeconds {
		if v < 0 || v > 3600 {
			return errors.New("步驟秒數無效")
		}
		total += v
	}
	if r.DispositionType == "supplement" && (total < 1 || total > 3600 || !finite(r.Target) || r.Target <= 0 || !finite(r.RootRate) || r.RootRate <= 0) {
		return errors.New("處方數值無效")
	}
	for _, v := range r.Baseline {
		if !finite(v) || v < 0 {
			return errors.New("基線無效")
		}
	}
	for _, x := range e.Recoveries {
		if x.WaferID == r.WaferID && (x.Status == "approved_waiting" || x.Status == "executing") {
			return errors.New("同片已有活動補做")
		}
	}
	r.Status = "approved_waiting"
	r.Result = nil
	if reason, permanent := e.recoveryBlock(r); permanent {
		return errors.New(reason)
	} else {
		r.Reason = reason
		r.ReasonCode = systemMessageCodes[r.Reason]
	}
	e.Recoveries[r.ExecutionID] = r
	i.ExecutionID = r.ExecutionID
	i.Status = "approved_waiting"
	i.ConfirmationID = ""
	return nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func (e *Engine) recoveryBlock(r *Recovery) (string, bool) {
	p := e.chamber(r.PMID)
	if r.DispositionType == "legacy_return" {
		i := e.Incidents[r.IncidentID]
		w := e.wafer(r.WaferID)
		if i == nil || !i.Legacy || !i.Returned || w == nil || w.Location != e.lot(w.LotID).LP || !isTerminal(w) || w.Result != r.LegacyResult || w.ExceptionVersion != r.ExceptionVersion {
			return "既有回收事實已改變", true
		}
		if e.Paused {
			return "等待工程確認恢復時鐘", false
		}
		return "", false
	}
	if r.DispositionType == "equipment" {
		i := e.Incidents[r.IncidentID]
		if p == nil || i == nil || i.WaferID != "" {
			return "設備異常綁定不符", true
		}
		if p.Fault || e.Paused {
			return "等待解除故障及恢復時鐘", false
		}
		return "", false
	}
	w := e.wafer(r.WaferID)
	if p == nil || w == nil || p.WaferID != r.WaferID || w.Location != r.PMID || w.Result != "exception" || p.ProcessRunID != r.SourceRunID || w.ExceptionVersion != r.ExceptionVersion {
		return "晶圓／來源狀態已改變，需重新提案", true
	}
	if r.DispositionType == "supplement" && (p.Process == nil || p.Process.SampleThickness != r.Baseline || math.Abs(processRootRate(p.Process)-r.RootRate) > 1e-9) {
		return "膜厚或速率基線已改變", true
	}
	if p.Fault {
		return "等待解除 PM 故障", false
	}
	if e.Paused {
		return "等待工程確認恢復補做時鐘", false
	}
	if len(e.Events) > 0 && e.Now.Sub(e.Events[0].SourceTime) > 10*time.Minute {
		return "等待通訊緩衝恢復", false
	}
	return "", false
}
func processRootRate(t *ProcessTelemetry) float64 {
	if t.RootRate > 0 {
		return t.RootRate
	}
	d := t.StepSeconds
	if d == [4]int{} {
		d = phaseDurations(t.Duration)
	}
	if d[2] < 1 {
		return 0
	}
	return t.TargetThickness / float64(d[2])
}
func (e *Engine) advanceRecoveries() {
	for _, r := range e.Recoveries {
		if r.Status != "approved_waiting" {
			continue
		}
		reason, invalid := e.recoveryBlock(r)
		r.Reason = reason
		r.ReasonCode = systemMessageCodes[r.Reason]
		if invalid {
			r.Status = "invalidated"
			continue
		}
		if reason != "" {
			continue
		}
		if r.DispositionType == "supplement" {
			e.beginSupplement(r)
		} else {
			e.beginDisposition(r)
		}
	}
}
func (e *Engine) beginSupplement(r *Recovery) {
	p := e.chamber(r.PMID)
	w := e.wafer(r.WaferID)
	e.processResult(p, w, false, "supplement-source")
	old := p.Process
	w.Attempts++
	w.Result = "processing"
	t := *old
	t.StepSeconds = r.StepSeconds
	t.Duration = 0
	for _, v := range r.StepSeconds {
		t.Duration += v
	}
	t.Elapsed = 0
	t.CompletedTick = 0
	t.Phase = ""
	t.PhaseElapsed = 0
	t.PhaseDuration = 0
	t.PhaseProgress = 0
	t.RunProgress = 0
	t.Rate = 0
	t.Quality = "not-evaluated"
	t.SourceRunID = r.SourceRunID
	t.RecoveryRequestID = r.RequestID
	t.RecoveryRevision = r.Revision
	t.ExecutionID = r.ExecutionID
	t.RootRate = r.RootRate
	t.BaselineThickness = old.Thickness
	t.DeltaThickness = 0
	t.TargetThickness = r.Target
	t.ProcessRunID = w.ID + "/recovery/" + r.ExecutionID
	p.Process = &t
	p.ProcessRunID = t.ProcessRunID
	p.Remaining = t.Duration
	if p.GasChannels == nil {
		p.GasChannels = map[string]Gas{}
	}
	started := e.Now
	r.StartedAt = &started
	r.Status = "executing"
	if i := e.Incidents[r.IncidentID]; i != nil {
		i.Status = "executing"
	}
	r.Reason = ""
	r.ReasonCode = systemMessageCodes[r.Reason]
	e.DispatchPaused = true
	e.lot(w.LotID).Paused = true
	e.emit("recovery-started", w.LotID, w.ID, map[string]any{"executionId": r.ExecutionID, "requestId": r.RequestID, "revision": r.Revision, "processRunId": t.ProcessRunID})
}
func (e *Engine) endRecovery(p *Chamber, status, reason string) {
	if p.Process == nil || p.Process.ExecutionID == "" {
		return
	}
	r := e.Recoveries[p.Process.ExecutionID]
	if r == nil || r.Status != "executing" {
		return
	}
	finished := e.Now
	r.FinishedAt = &finished
	r.Status = status
	r.Reason = reason
	r.ReasonCode = systemMessageCodes[r.Reason]
	t := *p.Process
	r.Result = &t
	if i := e.Incidents[r.IncidentID]; i != nil {
		i.ResultVersion++
		i.Status = status
		if status == "failed" {
			i.Status = "awaiting_ee"
			e.HoldGeneration++
		}
	}
	w := e.wafer(p.WaferID)
	e.emit("recovery-"+status, w.LotID, w.ID, map[string]any{"executionId": r.ExecutionID, "requestId": r.RequestID, "revision": r.Revision, "result": t, "reason": reason})
}
