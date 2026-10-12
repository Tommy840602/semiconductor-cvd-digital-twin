package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Wafer struct {
	MetrologyRunID   string     `json:"metrologyRunId,omitempty"`
	ClosureSequence  int64      `json:"closureSequence,omitempty"`
	ClosedAt         *time.Time `json:"closedAt,omitempty"`
	LastReturnAt     *time.Time `json:"lastReturnAt,omitempty"`
	Purpose          string     `json:"purpose"`
	TargetPM         string     `json:"targetPM,omitempty"`
	ID               string     `json:"id"`
	LotID            string     `json:"lotId"`
	Slot             int        `json:"slot"`
	Location         string     `json:"location"`
	Result           string     `json:"result"`
	Attempts         int        `json:"attempts"`
	ExceptionVersion int        `json:"exceptionVersion"`
}
type Lot struct {
	Purpose   string   `json:"purpose"`
	CarrierID string   `json:"carrierId"`
	Parked    bool     `json:"parked"`
	ID        string   `json:"id"`
	LP        string   `json:"lp"`
	LL        string   `json:"ll"`
	Phase     string   `json:"phase"`
	Paused    bool     `json:"paused"`
	Wafers    []*Wafer `json:"wafers"`
	Order     int      `json:"order"`
}
type Lock struct {
	ReturnLotID  string      `json:"returnLotId,omitempty"`
	Pump         *VacuumPump `json:"pump,omitempty"`
	WaferID      string      `json:"waferId"`
	PumpState    string      `json:"pumpState"`
	ValveOpening float64     `json:"valveOpeningPct"`
	ID           string      `json:"id"`
	LotID        string      `json:"lotId"`
	Phase        string      `json:"phase"`
	Remaining    int         `json:"remaining"`
	Pressure     float64     `json:"pressure"`
}
type Chamber struct {
	Process           *ProcessTelemetry `json:"processTelemetry"`
	ProcessRunID      string            `json:"processRunId"`
	GasChannels       map[string]Gas    `json:"gasChannels"`
	PumpState         string            `json:"pumpState"`
	ValveOpening      float64           `json:"valveOpeningPct"`
	Step              string            `json:"step"`
	TargetTemperature float64           `json:"targetTemperature"`
	TargetPressure    float64           `json:"targetPressure"`
	TargetFlow        float64           `json:"targetFlow"`
	ID                string            `json:"id"`
	WaferID           string            `json:"waferId"`
	Fault             bool              `json:"fault"`
	Remaining         int               `json:"remaining"`
	Temperature       float64           `json:"temperature"`
	Pressure          float64           `json:"pressure"`
	Flow              float64           `json:"flow"`
}
type Robot struct {
	ID        string `json:"id"`
	WaferID   string `json:"waferId"`
	From      string `json:"from"`
	To        string `json:"to"`
	Remaining int    `json:"remaining"`
}
type Event struct {
	SchemaVersion int       `json:"schemaVersion"`
	EventID       string    `json:"eventId"`
	EquipmentID   string    `json:"equipmentId"`
	SourceEpoch   string    `json:"sourceEpoch"`
	Sequence      int64     `json:"sequence"`
	SourceTime    time.Time `json:"sourceTime"`
	Quality       string    `json:"quality"`
	LotID         string    `json:"lotId,omitempty"`
	WaferID       string    `json:"waferId,omitempty"`
	Kind          string    `json:"kind"`
	Payload       any       `json:"payload"`
}
type Command struct {
	EquipmentID          string         `json:"equipmentId"`
	CommandID            string         `json:"commandId"`
	Type                 string         `json:"type"`
	TargetID             string         `json:"targetId"`
	LotID                string         `json:"lotId"`
	ExpiresAt            time.Time      `json:"expiresAt"`
	ExpectedStateVersion int64          `json:"expectedStateVersion"`
	Parameters           map[string]any `json:"parameters"`
}
type Result struct {
	CommandID  string `json:"commandId"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	ReasonCode string `json:"reasonCode,omitempty"`
}
type Engine struct {
	VacuumHold             bool   `json:"vacuumHold"`
	StatisticsEra          string `json:"statisticsEra"`
	StatisticsClosureCount int64  `json:"statisticsClosureCount"`
	statisticsBefore       int
	statisticsDirty        bool
	statisticsDummy        map[string]bool

	maintenanceCommandID        string                   `json:"-"`
	Maintenance                 *Maintenance             `json:"maintenance,omitempty"`
	DummyApprovalModelVersion   int                      `json:"dummyApprovalModelVersion"`
	MaintenanceModelVersion     int                      `json:"maintenanceModelVersion"`
	utilizationTick             time.Time                `json:"-"`
	DispositionMigrationVersion int                      `json:"dispositionMigrationVersion"`
	DispositionHold             bool                     `json:"dispositionHold"`
	HoldGeneration              int64                    `json:"holdGeneration"`
	Incidents                   map[string]*Incident     `json:"incidents"`
	Releases                    map[string]ReleaseResult `json:"releases"`
	Authorizations              map[string]string        `json:"authorizations"`
	EquipmentID                 string                   `json:"equipmentId"`
	RecipeVersion               string                   `json:"recipeVersion"`
	Recoveries                  map[string]*Recovery     `json:"recoveries"`
	LastDispatchOrder           int                      `json:"lastDispatchOrder"`
	NextLoadLock                int                      `json:"nextLoadLock"`
	ModelSeed                   uint64                   `json:"modelSeed"`
	ModelVersion                int                      `json:"modelVersion"`
	DispatchPaused              bool                     `json:"dispatchPaused"`
	Version                     int64                    `json:"version"`
	Epoch                       string                   `json:"epoch"`
	Sequence                    int64                    `json:"sequence"`
	TickNumber                  int64                    `json:"tick"`
	Lots                        []*Lot                   `json:"lots"`
	Locks                       []*Lock                  `json:"locks"`
	Chambers                    []*Chamber               `json:"chambers"`
	ATM                         Robot                    `json:"atm"`
	VAC                         Robot                    `json:"vac"`
	RecipeSteps                 *[4]int                  `json:"recipeSteps,omitempty"`
	ProcessSeconds              int                      `json:"processSeconds"`
	Paused                      bool                     `json:"paused"`
	Commands                    map[string]Result        `json:"commands"`
	Events                      []Event                  `json:"events"`
	Now                         time.Time                `json:"sourceTime"`
}

func NewEngine() *Engine {
	e := &Engine{DummyApprovalModelVersion: 1, MaintenanceModelVersion: 1, ModelVersion: 6, DispositionMigrationVersion: 1, ModelSeed: 20260915, Epoch: fmt.Sprint(time.Now().UnixNano()), ProcessSeconds: 30, ATM: Robot{ID: "ATM"}, VAC: Robot{ID: "VAC"}, Commands: map[string]Result{}, Lots: []*Lot{}, Events: []Event{}}
	for i := 1; i <= 3; i++ {
		e.Locks = append(e.Locks, &Lock{ID: fmt.Sprintf("LL%d", i), Phase: "empty", Pressure: 1013})
	}
	for i := 1; i <= 4; i++ {
		e.Chambers = append(e.Chambers, &Chamber{ID: fmt.Sprintf("PM%d", i), Temperature: 25})
	}
	return e
}
func (e *Engine) emit(kind, lot, wafer string, payload any) {
	if kind == "process-result" {
		if p, ok := payload.(map[string]any); ok {
			if reason, ok := p["reason"].(string); ok {
				if code := systemMessageCodes[reason]; code != "" {
					p["reasonCode"] = code
				}
			}
		}
	}

	e.Sequence++
	e.Events = append(e.Events, Event{1, fmt.Sprintf("%s-%d", e.Epoch, e.Sequence), e.EquipmentID, e.Epoch, e.Sequence, e.Now, "good", lot, wafer, kind, payload})
}
func (e *Engine) wafer(id string) *Wafer {
	for _, l := range e.Lots {
		for _, w := range l.Wafers {
			if w.ID == id {
				return w
			}
		}
	}
	return nil
}
func (e *Engine) lot(id string) *Lot {
	for _, l := range e.Lots {
		if l.ID == id {
			return l
		}
	}
	return nil
}
func (e *Engine) lock(id string) *Lock {
	for _, l := range e.Locks {
		if l.ID == id {
			return l
		}
	}
	return nil
}
func (e *Engine) chamber(id string) *Chamber {
	for _, p := range e.Chambers {
		if p.ID == id {
			return p
		}
	}
	return nil
}
func (e *Engine) terminal(l *Lot) bool {
	for _, w := range l.Wafers {
		if (w.Result != "success" && w.Result != "scrap") || w.Location != l.LP {
			return false
		}
	}
	return true
}
func (e *Engine) dispatchOrder() []*Lot {
	result := make([]*Lot, 0, len(e.Lots))
	for _, l := range e.Lots {
		if l.Order > e.LastDispatchOrder {
			result = append(result, l)
		}
	}
	for _, l := range e.Lots {
		if l.Order <= e.LastDispatchOrder {
			result = append(result, l)
		}
	}
	return result
}
func (e *Engine) feedSlots() int {
	free := 0
	for _, p := range e.Chambers {
		if !p.Fault && p.WaferID == "" && !(e.VAC.WaferID != "" && e.VAC.To == p.ID) {
			free++
		}
	}
	for _, ll := range e.Locks {
		if w := e.lockWafer(ll.ID); w != nil && w.Result == "pending" {
			free--
		}
	}
	if e.ATM.WaferID != "" && e.wafer(e.ATM.WaferID).Result == "pending" && e.lock(e.ATM.To) != nil {
		free--
	}
	return free
}
func (e *Engine) move(r *Robot, w *Wafer, to string) {
	r.WaferID = w.ID
	r.From = w.Location
	r.To = to
	r.Remaining = 1
	w.Location = r.ID
	e.emit("movement-start", w.LotID, w.ID, map[string]any{"from": r.From, "to": to, "robot": r.ID, "processRunId": w.MetrologyRunID, "result": w.Result, "slot": w.Slot, "purpose": w.Purpose})
}
func (e *Engine) finish(r *Robot) {
	if r.WaferID == "" {
		return
	}
	r.Remaining--
	if r.Remaining > 0 {
		return
	}
	w := e.wafer(r.WaferID)
	w.Location = r.To
	if w.Location == e.lot(w.LotID).LP && isTerminal(w) {
		at := e.Now
		w.LastReturnAt = &at
	}
	if p := e.chamber(r.To); p != nil {
		p.WaferID = w.ID
		if p.Fault {
			w.Result = "exception"
			w.ExceptionVersion++
			e.lot(w.LotID).Paused = true
			e.incidentFault(p)
		} else {
			p.Remaining = e.ProcessSeconds
			w.Result = "processing"
			w.Attempts++
			e.beginProcess(p, w)
		}
	}
	e.emit("movement-complete", w.LotID, w.ID, map[string]any{"from": r.From, "to": r.To, "robot": r.ID, "processRunId": w.MetrologyRunID, "result": w.Result, "slot": w.Slot, "purpose": w.Purpose})
	*r = Robot{ID: r.ID}
}
func (e *Engine) Tick(now time.Time) {
	e.Now = now.UTC()
	e.statisticsInit()
	e.statisticsBefore = e.statisticsFlags()
	e.statisticsDummy = map[string]bool{}
	utilizationStart := e.utilizationTick
	utilizationEnd := now.UTC().Truncate(time.Second)
	e.utilizationTick = utilizationEnd
	utilizationActive := map[string]bool{"PM1": false, "PM2": false, "PM3": false, "PM4": false}
	e.Version++
	e.initVacuum()
	if e.DispositionHold || e.VacuumHold {
		e.DispatchPaused = true
	}
	e.dispositionReturns()
	e.closeWafers()
	e.advanceRecoveries()
	if e.Paused {
		// Sampling continues while process time is paused. At the existing
		// delivery-buffer limit, preserve important events and the data-gap alarm.
		if len(e.Events) == 0 || now.Sub(e.Events[0].SourceTime) <= 10*time.Minute {
			e.updateMaintenance()
			e.emit("telemetry", "", "", e.telemetryPayload(utilizationStart, utilizationEnd, utilizationActive))
		}
		return
	}
	e.TickNumber++
	e.finish(&e.ATM)
	e.finish(&e.VAC)
	for _, p := range e.Chambers {
		if p.WaferID != "" && !p.Fault && p.Remaining > 0 && e.wafer(p.WaferID).Result == "processing" {
			utilizationActive[p.ID] = true
			e.statisticsDummy[p.ID] = e.wafer(p.WaferID).Purpose == "dummy"
			if p.Process != nil {
				e.stepProcess(p)
				continue
			}
			p.Remaining--
			p.Step = "deposition"
			if p.Remaining > e.ProcessSeconds*2/3 {
				p.Step = "stabilize"
			} else if p.Remaining < e.ProcessSeconds/3 {
				p.Step = "purge"
			}
			p.TargetTemperature = 400
			p.TargetPressure = 2
			p.TargetFlow = 100
			if p.Step != "deposition" {
				p.TargetFlow = 0
			}
			p.Temperature += (p.TargetTemperature - p.Temperature) * 0.15
			p.Pressure += (p.TargetPressure - p.Pressure) * 0.5
			p.Flow = p.TargetFlow
			if p.Remaining == 0 {
				w := e.wafer(p.WaferID)
				w.Result = "success"
				e.emit("process-complete", w.LotID, w.ID, map[string]any{"pm": p.ID, "attempt": w.Attempts})
			}
		} else {
			p.Flow = 0
			p.TargetFlow = 0
			if p.Fault {
				p.Step = "fault"
			} else if p.WaferID != "" && e.wafer(p.WaferID).Result == "exception" {
				p.Step = "exception"
			} else {
				p.Step = "idle"
				p.Temperature += (25 - p.Temperature) * 0.02
				if p.WaferID != "" && e.wafer(p.WaferID).Result == "success" {
					p.Step = "complete"
				}
			}
			if p.Process != nil {
				p.Process.Rate = 0
				p.Process.HeaterPower = 0
				p.Process.TemperatureDeviation = p.Temperature - p.TargetTemperature
				for gas := range p.GasChannels {
					p.GasChannels[gas] = Gas{}
				}
			}
		}
	}
	for _, ll := range e.Locks {
		if e.stepPump(ll) {
			continue
		}
		if ll.Remaining > 0 {
			ll.Remaining--
			if ll.Remaining == 0 {
				if ll.Phase == "pumping" {
					ll.Pump.Legacy = false
					ll.Phase = "vacuum"
					ll.Pressure = 0.01
				} else {
					ll.Phase = "atmosphere"
					ll.Pressure = 1013
				}
			}
		}
	}
	e.recoverVacuum()
	e.updatePumpTest()
	e.updateReturnLocks()
	// Return completed wafers before feeding another wafer through the single slot.
	if e.VAC.WaferID == "" {
		for _, p := range e.Chambers {
			if p.WaferID == "" || p.Fault {
				continue
			}
			w := e.wafer(p.WaferID)
			if !isTerminal(w) {
				continue
			}
			ll := e.returnLock(w)
			if ll == nil || ll.Pump.Fault || ll.Pump.ChannelTesting {
				continue
			}
			if isTerminal(w) && (p.Process == nil || p.Process.CompletedTick != e.TickNumber) && ll.Phase == "vacuum" && e.lockWafer(ll.ID) == nil && !e.lockBusy(ll.ID) {
				e.move(&e.VAC, w, ll.ID)
				p.WaferID = ""
				p.Process = nil
				p.Flow = 0
				p.TargetFlow = 0
				p.Step = "idle"
				p.PumpState = "idle"
				p.ValveOpening = 0
				p.ProcessRunID = ""
				p.GasChannels = nil
				break
			}
		}
	}
	if e.VAC.WaferID == "" {
		for _, l := range e.dispatchOrder() {
			if !e.transferAllowed(l) || l.Phase == "complete" {
				continue
			}
			ll := e.lock(l.LL)
			w := e.lockWafer(ll.ID)
			if ll.ReturnLotID != "" || ll.Pump.Fault || ll.Pump.ChannelTesting || ll.Phase != "vacuum" || e.lockBusy(ll.ID) || w == nil || w.Result != "pending" {
				continue
			}
			for _, p := range e.Chambers {
				if p.WaferID == "" && !p.Fault && (w.TargetPM == "" || w.TargetPM == p.ID) {
					e.move(&e.VAC, w, p.ID)
					l.Phase = "processing"
					e.LastDispatchOrder = l.Order
					break
				}
			}
			if e.VAC.WaferID != "" {
				break
			}
		}
	}
	// Atmospheric returns take priority over fresh loading.
	if e.ATM.WaferID == "" {
		for _, ll := range e.Locks {
			l := e.lot(ll.LotID)
			w := e.lockWafer(ll.ID)
			if w != nil {
				l = e.lot(w.LotID)
			}
			if !ll.Pump.Fault && l != nil && w != nil && ll.Phase == "atmosphere" && ll.Remaining == 0 && !e.lockBusy(ll.ID) && (isTerminal(w) || !e.transferAllowed(l)) {
				e.move(&e.ATM, w, l.LP)
				break
			}
		}
	}
	startLock := e.NextLoadLock
	for offset := 0; offset < len(e.Locks); offset++ {
		lockIndex := (startLock + offset) % len(e.Locks)
		ll := e.Locks[lockIndex]
		l := e.lot(ll.LotID)
		if ll.ReturnLotID != "" || ll.Pump.ChannelTesting || ll.Pump.Fault || (ll.Pump.Cycle != nil && ll.Pump.Cycle.Purpose == "test" && e.Maintenance != nil && e.Maintenance.PumpTest != nil && e.Maintenance.PumpTest.Status == "running") || l == nil || e.lockBusy(ll.ID) || ll.Remaining > 0 {
			continue
		}
		w := e.lockWafer(ll.ID)
		canFeed := e.feedAllowed(l) && e.feedSlots() > 0 && e.pendingAtLP(l)
		returnWaiting := e.returnWaiting(l)
		if ll.Phase == "vacuum" {
			// A pending wafer is withdrawn on pause/fault so it cannot block PM returns.
			if (w != nil && (isTerminal(w) || !e.transferAllowed(l) || (e.feedSlots() < 0 && e.returnWaiting(l)))) || (w == nil && !returnWaiting && canFeed) {
				ll.Phase = "venting"
				ll.Remaining = 3
			}
		} else if ll.Phase == "atmosphere" || ll.Phase == "empty" {
			if w != nil {
				if isTerminal(w) || !e.transferAllowed(l) || (e.feedSlots() < 0 && e.returnWaiting(l)) {
					if e.ATM.WaferID == "" {
						e.move(&e.ATM, w, l.LP)
					}
				} else {
					ll.Phase = "pumping"
					ll.Remaining = 3
				}
			} else if returnWaiting {
				ll.Phase = "pumping"
				ll.Remaining = 3
			} else if canFeed && e.ATM.WaferID == "" {
				for _, w := range l.Wafers {
					if w.Location == l.LP && w.Result == "pending" {
						ll.Phase = "atmosphere"
						e.move(&e.ATM, w, ll.ID)
						e.NextLoadLock = (lockIndex + 1) % len(e.Locks)
						break
					}
				}
			}
		}
	}
	for _, l := range e.Lots {
		if !l.Parked && l.Phase != "complete" && e.terminal(l) {
			l.Phase = "complete"
			ll := e.lock(l.LL)
			ll.LotID = ""
			if ll.ReturnLotID == "" && !ll.Pump.Fault && !e.lockBusy(ll.ID) {
				ll.Phase = "empty"
				ll.Pressure = 1013
				ll.Remaining = 0
			}
			e.emit("lot-terminal", l.ID, "", nil)
			e.emit("lot-complete", l.ID, "", nil)
		}
	}
	for _, ll := range e.Locks {
		e.vacuumTelemetry(ll)
	}
	e.updateMaintenance()
	e.closeWafers()
	e.emit("telemetry", "", "", e.telemetryPayload(utilizationStart, utilizationEnd, utilizationActive))
}
func isTerminal(w *Wafer) bool { return w.Result == "success" || w.Result == "scrap" }
func (e *Engine) lockWafer(id string) *Wafer {
	for _, l := range e.Lots {
		for _, w := range l.Wafers {
			if w.Location == id {
				return w
			}
		}
	}
	return nil
}
func (e *Engine) lockBusy(id string) bool {
	for _, r := range []*Robot{&e.ATM, &e.VAC} {
		if r.WaferID != "" && (r.From == id || r.To == id) {
			return true
		}
	}
	return false
}
func (e *Engine) pendingAtLP(l *Lot) bool {
	for _, w := range l.Wafers {
		if w.Location == l.LP && w.Result == "pending" {
			return true
		}
	}
	return false
}
func (e *Engine) returnWaiting(l *Lot) bool {
	for _, p := range e.Chambers {
		if p.WaferID != "" && !p.Fault {
			w := e.wafer(p.WaferID)
			if w.LotID == l.ID && isTerminal(w) {
				return true
			}
		}
	}
	return false
}
func (e *Engine) copyChambers() any {
	b, _ := json.Marshal(e.Chambers)
	var v any
	json.Unmarshal(b, &v)
	return v
}
func (e *Engine) copyLocks() any {
	b, _ := json.Marshal(e.Locks)
	var v any
	json.Unmarshal(b, &v)
	return v
}
func (e *Engine) Execute(c Command, now time.Time) Result {
	if e.EquipmentID != "" && c.EquipmentID != e.EquipmentID {
		return Result{CommandID: c.CommandID, Status: "rejected", Reason: "機台識別不符", ReasonCode: systemMessageCodes["機台識別不符"]}
	}
	if r, ok := e.Commands[c.CommandID]; ok {
		// Re-acknowledge a retry without applying it again. The gateway publishes
		// persisted result events, so an earlier acknowledged event may be gone.
		e.Now = now.UTC()
		e.Version++
		e.emit("command-result", c.LotID, "", r)
		return r
	}
	e.Now = now.UTC()
	r := Result{CommandID: c.CommandID, Status: "succeeded"}
	e.maintenanceCommandID = c.CommandID
	defer func() { e.maintenanceCommandID = "" }()
	err := e.apply(c, now)
	if err == nil {
		e.statisticsDirty = true
		e.closeWafers()
	}
	if err != nil {
		r.Status = "rejected"
		r.Reason = err.Error()
		r.ReasonCode = systemMessageCodes[r.Reason]
	}
	e.Commands[c.CommandID] = r
	e.Version++
	e.emit("command-result", c.LotID, "", r)
	return r
}
func (e *Engine) apply(c Command, now time.Time) error {
	if c.CommandID == "" {
		return errors.New("缺少 commandId")
	}
	if !c.ExpiresAt.After(now) {
		return errors.New("指令已過期")
	}
	if c.ExpectedStateVersion > 0 && c.ExpectedStateVersion != e.Version {
		return errors.New("設備狀態已變更")
	}
	l := e.lot(c.LotID)
	p := e.chamber(c.TargetID)
	switch c.Type {
	case "vacuum-recover", "vacuum-test", "vacuum-service-channel":
		return e.vacuumCommand(c)
	case "maintenance-request", "maintenance-end", "maintenance-carrier-load", "qualification-start", "maintenance-carrier-restore":
		return e.maintenanceCommand(c)
	case "disposition-confirm", "disposition-reject", "equipment-release":
		return e.dispositionCommand(c)
	case "wafer-supplement", "wafer-scrap", "equipment-check", "legacy-return-check", "cancel-supplement":
		return e.recoveryCommand(c)
	case "recovery-clock":
		if err := e.Validate(); err != nil {
			return err
		}
		if len(e.Events) > 0 && now.Sub(e.Events[0].SourceTime) > 10*time.Minute {
			return errors.New("等待通訊緩衝恢復")
		}
		e.Paused = false
		e.DispatchPaused = true
	case "rework":
		return errors.New("完整重做已停用，請由 EE 提案並由 PE 核准單片補做")
	case "start":
		if e.VacuumHold {
			return errors.New("vacuum.resume-blocked")
		}
		if e.maintenanceHeld() {
			return errors.New("保養期間停止正式批次派片")
		}
		if e.DispositionHold {
			return errors.New("整台異常尚未經 Leader 放行")
		}
		if c.LotID == "" || l != nil {
			return errors.New("批次識別重複或空白")
		}
		lp, _ := c.Parameters["lp"].(string)
		llid, _ := c.Parameters["ll"].(string)
		ll := e.lock(llid)
		if (lp != "LP1" && lp != "LP2" && lp != "LP3") || ll == nil || ll.LotID != "" {
			return errors.New("LP 或 LL 不可用")
		}
		for _, x := range e.Lots {
			if x.LP == lp && !x.Parked && x.Phase != "complete" {
				return errors.New("LP 已占用")
			}
		}
		l = &Lot{Purpose: "production", CarrierID: c.LotID + "-FOUP", ID: c.LotID, LP: lp, LL: llid, Phase: "loading", Order: len(e.Lots) + 1}
		for i := 1; i <= 25; i++ {
			l.Wafers = append(l.Wafers, &Wafer{Purpose: "production", ID: fmt.Sprintf("%s-W%02d", l.ID, i), LotID: l.ID, Slot: i, Location: lp, Result: "pending"})
		}
		e.Lots = append(e.Lots, l)
		ll.LotID = l.ID
		ll.Phase = "atmosphere"
		e.emit("lot-start", l.ID, "", map[string]any{"lp": lp, "ll": llid})
	case "pause", "stop":
		if l != nil && l.Purpose == "dummy" {
			return errors.New("Dummy 測試由保養流程控制，不能使用正式批次暫停")
		}
		if l == nil {
			return errors.New("批次不存在")
		}
		l.Paused = true
	case "resume":
		e.initVacuum()
		if !e.vacuumReady() {
			return errors.New("vacuum.resume-blocked")
		}
		if err := e.maintenanceResume(c); err != nil {
			return err
		}
		if e.DispositionHold {
			return errors.New("需由不同帳號 Leader 核准整台放行")
		}
		for _, pm := range e.Chambers {
			if pm.Fault {
				return errors.New("仍有腔體故障")
			}
		}
		for _, x := range e.Lots {
			for _, w := range x.Wafers {
				if w.Result == "exception" || (w.Attempts > 1 && w.Location != x.LP) {
					return errors.New("異常晶圓尚未結案回槽")
				}
				if w.Result == "scrap" && w.Location != x.LP {
					return errors.New("報廢晶圓尚未回槽")
				}
			}
		}
		if l != nil {
			l.Paused = false
		}
		e.Paused = false
		e.DispatchPaused = false
		e.VacuumHold = false
		if e.maintenanceHeld() {
			e.Maintenance.LastCommandID = c.CommandID
			e.Maintenance.Phase = "released"
			e.maintenanceEvent("released")
		}
	case "fault":
		if p == nil {
			return errors.New("PM 不存在")
		}
		e.DispatchPaused = true
		p.Fault = true
		e.invalidateMaintenance("fault")
		if p.WaferID != "" {
			w := e.wafer(p.WaferID)
			w.Result = "exception"
			w.ExceptionVersion++
			e.processResult(p, w, false, "fault")
			e.endRecovery(p, "failed", "製程再次故障")
			e.lot(w.LotID).Paused = true
		}
		e.incidentFault(p)
		e.emit("alarm", c.LotID, p.WaferID, map[string]any{"pm": p.ID, "active": true})
	case "clear":
		if p == nil {
			return errors.New("PM 不存在")
		}
		p.Fault = false
		e.emit("alarm", c.LotID, p.WaferID, map[string]any{"pm": p.ID, "active": false})
	case "scrap":
		return errors.New("報廢需 EE 提案與 PE 核准")
	case "configure":
		if e.maintenanceHeld() && e.Maintenance.Phase == "qualifying" {
			return errors.New("Dummy 驗證執行中不可修改 recipe")
		}
		if version, ok := c.Parameters["recipeVersion"].(string); ok && e.RecipeVersion != "" {
			revision := func(v string) int {
				parts := strings.Split(v, "-recipe-")
				if len(parts) != 2 {
					return 0
				}
				n, _ := strconv.Atoi(parts[1])
				return n
			}
			if revision(version) < revision(e.RecipeVersion) {
				return errors.New("recipe 版本已過期")
			}
		}
		if raw, exists := c.Parameters["stepSeconds"]; exists {
			steps, ok := raw.(map[string]any)
			if !ok || len(steps) != 4 {
				return errors.New("請提供完整的四個製程步驟")
			}
			var values [4]int
			total := 0
			for i, name := range []string{"preheat", "stabilize", "deposition", "purge"} {
				v, ok := steps[name].(float64)
				if !ok || v < 1 || v > 3600 || v != float64(int(v)) {
					return errors.New("每個步驟須為 1 至 3600 的整數秒")
				}
				values[i] = int(v)
				total += int(v)
			}
			if total > 3600 {
				return errors.New("總製程時間不可超過 3600 秒")
			}
			e.RecipeSteps = &values
			e.ProcessSeconds = total
		} else {
			v, ok := c.Parameters["processSeconds"].(float64)
			if !ok || v < 5 || v > 3600 || v != float64(int(v)) {
				return errors.New("模擬時間須為 5 至 3600 的整數秒")
			}
			e.ProcessSeconds = int(v)
			e.RecipeSteps = nil
		}
		if version, ok := c.Parameters["recipeVersion"].(string); ok {
			e.RecipeVersion = version
		}

		e.invalidateMaintenance("recipe")
	default:
		return errors.New("未知指令")
	}
	return nil
}
func (e *Engine) Validate() error {
	seen := map[string]bool{}
	for _, l := range e.Lots {
		capacity := 25
		if l.Purpose == "dummy" {
			capacity = 4
		}
		if len(l.Wafers) != capacity {
			return errors.New("批次容量錯誤")
		}
		for _, w := range l.Wafers {
			if seen[w.ID] {
				return errors.New("晶圓重複")
			}
			seen[w.ID] = true
			valid := w.Location == l.LP || w.Location == l.LL || (e.lock(w.Location) != nil && e.lock(w.Location).ReturnLotID == l.ID) || (w.Location == "ATM" && e.ATM.WaferID == w.ID) || (w.Location == "VAC" && e.VAC.WaferID == w.ID)
			if p := e.chamber(w.Location); p != nil {
				valid = p.WaferID == w.ID
			}
			if !valid {
				return fmt.Errorf("晶圓 %s 位置不一致: %s", w.ID, w.Location)
			}
		}
	}
	for _, ll := range e.Locks {
		count := 0
		for _, l := range e.Lots {
			for _, w := range l.Wafers {
				if w.Location == ll.ID {
					count++
					if ll.LotID != l.ID && ll.ReturnLotID != l.ID {
						return errors.New("LL 批次不一致")
					}
				}
			}
		}
		for _, r := range []*Robot{&e.ATM, &e.VAC} {
			if r.WaferID != "" && r.To == ll.ID {
				count++
			}
			if r.WaferID != "" && (r.To == ll.ID || r.From == ll.ID) {
				expected := "vacuum"
				if r.ID == "ATM" {
					expected = "atmosphere"
				}
				if ll.Phase != expected {
					return fmt.Errorf("%s 壓力階段與搬送不一致", ll.ID)
				}
			}
		}
		if count > 1 {
			return fmt.Errorf("%s 超過單片容量", ll.ID)
		}
	}

	return nil
}
func (e *Engine) Save(path string) error {
	if err := e.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
func Load(path string) (*Engine, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewEngine(), nil
	}
	if err != nil {
		return nil, err
	}
	e := new(Engine)
	if err = json.Unmarshal(b, e); err != nil {
		return nil, err
	}
	e.initVacuum()
	e.interruptPumpTests()
	e.Paused = true
	e.MaintenanceModelVersion = 1
	e.DummyApprovalModelVersion = 1
	if e.Maintenance != nil && e.Maintenance.Run != nil {
		for i, r := range e.Maintenance.Runs {
			if r.ID == e.Maintenance.Run.ID {
				e.Maintenance.Runs[i] = e.Maintenance.Run
			}
		}
	}
	for _, l := range e.Lots {
		if l.Purpose == "" {
			l.Purpose = "production"
		}
		if l.CarrierID == "" {
			l.CarrierID = l.ID + "-FOUP"
		}
		for _, w := range l.Wafers {
			if w.Purpose == "" {
				w.Purpose = l.Purpose
			}
		}
	}
	if e.maintenanceHeld() {
		e.DispatchPaused = true
	}
	if e.ModelVersion < 2 {
		if e.ATM.WaferID != "" || e.VAC.WaferID != "" {
			return nil, errors.New("舊版搬送尚未結束，請先完成搬送再升級單片 LL")
		}
		e.Now = time.Now().UTC()
		for _, l := range e.Lots {
			for _, w := range l.Wafers {
				if w.Location == l.LL {
					w.Location = l.LP
					e.emit("capacity-migration", l.ID, w.ID, map[string]any{"from": l.LL, "to": l.LP, "slot": w.Slot, "reason": "LL capacity changed from 25 to 1; checkpoint relocation"})
				}
			}
			if l.Phase != "complete" {
				l.Phase = "ready"
			}
		}
		for _, ll := range e.Locks {
			ll.Remaining = 0
			ll.Pressure = 1013
			ll.Phase = "atmosphere"
			if ll.LotID == "" {
				ll.Phase = "empty"
			}
		}
		e.ModelVersion = 2
		e.Version++
	}
	if e.ModelVersion < 3 {
		e.ModelVersion = 3
		e.ModelSeed = 20260915
		e.Version++
	}
	if e.ModelVersion < 4 {
		e.ModelVersion = 4
		e.LastDispatchOrder = 0
		e.NextLoadLock = 0
		e.Version++
	}
	if e.ModelVersion < 5 {
		e.ModelVersion = 5
		e.Version++
	}
	if e.Recoveries == nil {
		e.Recoveries = map[string]*Recovery{}
	}
	e.dispositionInit()
	if e.ModelVersion < 6 {
		e.migrateDispositions()
		e.ModelVersion = 6
		e.Version++
	}
	if e.DispositionMigrationVersion < 1 {
		e.migrateLegacyReturned()
		e.DispositionMigrationVersion = 1
		e.Version++
	}
	if e.DispositionHold {
		e.DispatchPaused = true
	}
	return e, e.Validate()
}

func (e *Engine) EnforceBuffer(now time.Time) {
	if len(e.Events) > 0 && now.Sub(e.Events[0].SourceTime) > 10*time.Minute {
		e.Paused = true
		for _, event := range e.Events {
			if event.Kind == "data-gap" {
				if payload, ok := event.Payload.(map[string]any); ok && payload["reason"] == "delivery buffer full" {
					return
				}
			}
		}
		e.Now = now.UTC()
		e.emit("data-gap", "", "", map[string]any{"reason": "delivery buffer full"})
	}
}
