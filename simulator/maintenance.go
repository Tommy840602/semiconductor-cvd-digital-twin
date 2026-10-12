package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Qualification struct {
	ID            string                         `json:"id"`
	Generation    int64                          `json:"generation"`
	RecipeVersion string                         `json:"recipeVersion"`
	StepSeconds   [4]int                         `json:"stepSeconds"`
	LotID         string                         `json:"lotId"`
	Status        string                         `json:"status"`
	Results       map[string]QualificationResult `json:"results"`
}
type QualificationResult struct {
	WaferID  string           `json:"waferId"`
	Complete bool             `json:"complete"`
	Result   ProcessTelemetry `json:"result"`
}
type Maintenance struct {
	SelectedPumps []string         `json:"selectedPumpIds,omitempty"`
	PumpTest      *PumpTest        `json:"pumpTest,omitempty"`
	LastCommandID string           `json:"lastCommandId"`
	ID            string           `json:"id"`
	Generation    int64            `json:"generation"`
	Phase         string           `json:"phase"`
	SelectedLP    string           `json:"selectedLP"`
	ParkedLotID   string           `json:"parkedLotId"`
	CarrierID     string           `json:"carrierId"`
	LoadedLotID   string           `json:"loadedLotId"`
	Loaded        bool             `json:"loaded"`
	SavedLock     *Lock            `json:"savedLock,omitempty"`
	SavedDispatch int              `json:"savedDispatch"`
	SavedNextLock int              `json:"savedNextLock"`
	Run           *Qualification   `json:"run,omitempty"`
	Runs          []*Qualification `json:"runs"`
	Blockers      []string         `json:"blockers"`
	CreatedAt     time.Time        `json:"createdAt"`
}

func (e *Engine) maintenanceHeld() bool {
	return e.Maintenance != nil && e.Maintenance.Phase != "released"
}
func (e *Engine) feedAllowed(l *Lot) bool {
	if l.Parked || l.Paused {
		return false
	}
	if l.Purpose == "dummy" {
		return e.maintenanceHeld() && e.Maintenance.Phase == "qualifying" && e.Maintenance.Run != nil && e.Maintenance.Run.LotID == l.ID && !e.DispositionHold
	}
	return !e.maintenanceHeld() && !e.DispatchPaused && !e.VacuumHold
}
func (e *Engine) transferAllowed(l *Lot) bool {
	if l.Parked {
		return false
	}
	if e.VacuumHold || (e.maintenanceHeld() && e.Maintenance.Phase == "draining") {
		return !e.DispositionHold
	}
	return e.feedAllowed(l)
}
func (e *Engine) drained() bool {
	if e.ATM.WaferID != "" || e.VAC.WaferID != "" {
		return false
	}
	for _, p := range e.Chambers {
		if p.WaferID != "" {
			return false
		}
	}
	for _, l := range e.Locks {
		if l.Remaining > 0 || e.lockWafer(l.ID) != nil {
			return false
		}
	}
	return true
}
func (e *Engine) maintenanceBlockers() []string {
	reasons := []string{}
	if !e.drained() {
		reasons = append(reasons, "仍有在途／腔體晶圓或 LL 抽排氣未完成")
	}
	if e.DispositionHold {
		reasons = append(reasons, "異常處置尚未由 Leader 整台放行")
	}
	for _, p := range e.Chambers {
		if p.Fault {
			reasons = append(reasons, p.ID+" 故障未解除")
		}
	}
	return reasons
}
func (e *Engine) maintenanceEvent(kind string) {
	if e.maintenanceCommandID != "" {
		e.Maintenance.LastCommandID = e.maintenanceCommandID
	}
	// Copy before appending: later mutations must never rewrite historic evidence.
	raw, _ := json.Marshal(e.Maintenance)
	var m any
	json.Unmarshal(raw, &m)
	e.emit("maintenance-"+kind, "", "", map[string]any{"maintenance": m})
}
func (e *Engine) updateMaintenance() {
	if !e.maintenanceHeld() {
		return
	}
	m := e.Maintenance
	m.Blockers = e.maintenanceBlockers()
	if len(m.SelectedPumps) > 0 && m.Phase != "draining" && m.Phase != "maintenance" && !e.vacuumReady() {
		m.Blockers = append(m.Blockers, "vacuum.test-required")
	}
	e.DispatchPaused = true
	if m.Phase == "draining" && len(m.Blockers) == 0 {
		m.Phase = "maintenance"
		e.maintenanceEvent("entered")
	}
	if m.Phase == "qualifying" && m.Run != nil {
		l := e.lot(m.Run.LotID)
		if l != nil && e.terminal(l) && e.drained() {
			passed := m.Run.Generation == m.Generation && m.Run.RecipeVersion == e.RecipeVersion && len(m.Run.Results) == 4 && len(m.Blockers) == 0
			for i := 1; i <= 4; i++ {
				r, ok := m.Run.Results[fmt.Sprintf("PM%d", i)]
				if !ok || !r.Complete || r.Result.Quality != "pass" {
					passed = false
				}
			}
			m.Run.Status = "failed"
			m.Phase = "failed"
			if passed {
				m.Run.Status = "passed"
				m.Phase = "passed"
			}
			e.maintenanceEvent("result")
		}
	}
	if m.Phase == "waiting" || m.Phase == "failed" {
		m.Blockers = append(m.Blockers, "四個 PM 尚未通過本輪 Dummy 驗證")
	}
	if m.Phase == "passed" && m.Loaded {
		m.Blockers = append(m.Blockers, "請先卸下 Dummy 載具並換回原 FOUP")
	}
}
func (e *Engine) invalidateMaintenance(reason string) {
	if !e.maintenanceHeld() {
		return
	}
	m := e.Maintenance
	m.Generation++
	if reason != "recipe" && m.PumpTest != nil {
		m.PumpTest.Status = "invalidated"
	}
	if m.Run != nil {
		m.Run.Status = "invalidated"
	}
	if reason != "recipe" && m.Phase == "qualifying" {
		m.Phase = "failed"
	}
	if m.Phase == "passed" || m.Phase == "ready" {
		m.Phase = "waiting"
	}
	e.maintenanceEvent("invalidated-" + reason)
}
func (e *Engine) qualificationResult(p *Chamber, w *Wafer, complete bool, result ProcessTelemetry) {
	if w.Purpose != "dummy" || e.Maintenance == nil || e.Maintenance.Run == nil || e.Maintenance.Run.LotID != w.LotID {
		return
	}
	// Fault/supplement evidence remains traceable, but cannot override a failed validation attempt.
	if old, exists := e.Maintenance.Run.Results[p.ID]; exists && !old.Complete {
		return
	}
	if result.Uniformity != nil {
		value := *result.Uniformity
		result.Uniformity = &value
	}
	e.Maintenance.Run.Results[p.ID] = QualificationResult{w.ID, complete, result}
	e.maintenanceEvent("pm-result")
}
func (e *Engine) maintenanceFence(c Command) error {
	if e.Maintenance == nil {
		return errors.New("沒有保養週期")
	}
	id, _ := c.Parameters["maintenanceId"].(string)
	var gen int64
	switch n := c.Parameters["expectedGeneration"].(type) {
	case float64:
		if n != float64(int64(n)) {
			return errors.New("保養版本無效")
		}
		gen = int64(n)
	case int64:
		gen = n
	case int:
		gen = int64(n)
	}
	if id != e.Maintenance.ID || gen != e.Maintenance.Generation {
		return errors.New("保養週期或版本已變更，請重新載入")
	}
	return nil
}
func (e *Engine) maintenanceCommand(c Command) error {
	e.initVacuum()
	if c.Type == "maintenance-request" {
		selected, err := selectedPumps(c)
		if err != nil {
			return err
		}
		if e.maintenanceHeld() {
			if err := e.maintenanceFence(c); err != nil {
				return err
			}
			if !e.drained() || e.Maintenance.Loaded || !(e.Maintenance.Phase == "waiting" || e.Maintenance.Phase == "failed") {
				return errors.New("已有進行中的保養")
			}
		}
		if e.Maintenance != nil {
			if err := e.maintenanceFence(c); err != nil {
				return err
			}
		} else if id, _ := c.Parameters["maintenanceId"].(string); id != "" {
			return errors.New("保養週期不符")
		}
		gen := int64(1)
		if e.Maintenance != nil {
			gen = e.Maintenance.Generation + 1
		}
		for _, ll := range e.Locks {
			if ll.Pump.NeedsService {
				found := false
				for _, id := range selected {
					if id == ll.ID {
						found = true
					}
				}
				if !found {
					return errors.New("vacuum.service-required")
				}
			}
		}
		if e.maintenanceHeld() {
			e.maintenanceEvent("superseded")
		}
		e.Maintenance = &Maintenance{SelectedPumps: selected, LastCommandID: c.CommandID, ID: e.EquipmentID + "-maintenance-" + c.CommandID, Generation: gen, Phase: "draining", CreatedAt: e.Now, Runs: []*Qualification{}, SavedDispatch: e.LastDispatchOrder, SavedNextLock: e.NextLoadLock}
		e.DispatchPaused = true
		e.maintenanceEvent("requested")
		e.updateMaintenance()
		return nil
	}
	if err := e.maintenanceFence(c); err != nil {
		return err
	}
	m := e.Maintenance
	switch c.Type {
	case "maintenance-end":
		if m.Phase != "maintenance" {
			return errors.New("機台尚未進入保養中")
		}
		if len(e.maintenanceBlockers()) > 0 {
			return errors.New(strings.Join(e.maintenanceBlockers(), "；"))
		}
		for _, id := range m.SelectedPumps {
			ll := e.lock(id)
			e.servicePump(ll)
			ll.Pump.NeedsService = false
		}

		m.PumpTest = nil
		m.Phase = "waiting"
	case "maintenance-carrier-load":
		if m.Phase != "maintenance" && m.Phase != "waiting" && m.Phase != "failed" && !(c.Parameters["peRetest"] == true && m.Phase == "ready") {
			return errors.New("此階段不可換載")
		}
		if m.Loaded || !e.drained() || len(e.maintenanceBlockers()) > 0 {
			return errors.New("需完成排空、處置及卸載後才能換載")
		}
		lp, _ := c.Parameters["lp"].(string)
		if lp != "LP1" && lp != "LP2" && lp != "LP3" {
			return errors.New("請選擇 LP1～LP3")
		}
		var parked *Lot
		for _, l := range e.Lots {
			if l.LP == lp && l.Purpose != "dummy" && l.Phase != "complete" && !l.Parked {
				parked = l
				break
			}
		}
		var ll *Lock
		if parked != nil {
			ll = e.lock(parked.LL)
		} else {
			for _, x := range e.Locks {
				if x.LotID == "" {
					ll = x
					break
				}
			}
		}
		if ll == nil {
			return errors.New("沒有可供 Dummy 載具使用的 LL")
		}
		m.ParkedLotID = ""
		if parked != nil {
			for _, w := range parked.Wafers {
				if w.Location != lp {
					return errors.New("正式晶圓尚未全部回槽")
				}
			}
			m.ParkedLotID = parked.ID
			parked.Parked = true
		}
		saved := *ll
		m.SavedLock = &saved
		m.SelectedLP = lp
		m.Loaded = true
		m.CarrierID = m.ID + "-dummy-carrier"
		ll.LotID = ""
		ll.Phase = "atmosphere"
		e.prepareDummy(m.ID + "-carrier-" + c.CommandID)
	case "qualification-start":
		if !e.vacuumReady() {
			return errors.New("vacuum.test-required")
		}
		if m.Phase != "waiting" && m.Phase != "failed" && !(c.Parameters["peRetest"] == true && (m.Phase == "passed" || m.Phase == "ready")) {
			return errors.New("目前不可開始測試")
		}
		if !m.Loaded || !e.drained() || len(e.maintenanceBlockers()) > 0 || e.Paused {
			return errors.New("需換入 Dummy 載具、排空、解除異常並恢復測試時鐘")
		}
		runID := fmt.Sprintf("%s-run-%d", m.ID, len(m.Runs)+1)
		steps := phaseDurations(e.ProcessSeconds)
		if e.RecipeSteps != nil {
			steps = *e.RecipeSteps
		}
		run := &Qualification{ID: runID, Generation: m.Generation, RecipeVersion: e.RecipeVersion, StepSeconds: steps, LotID: runID + "-dummy", Status: "executing", Results: map[string]QualificationResult{}}
		l := e.lot(m.LoadedLotID)
		if l == nil || l.Phase != "prepared" {
			l = e.prepareDummy(run.ID)
		}
		run.LotID = l.ID
		l.Phase = "loading"
		ll := e.lock(l.LL)
		ll.LotID = l.ID
		ll.Phase = "atmosphere"
		ll.Remaining = 0
		m.Run = run
		m.Runs = append(m.Runs, run)
		m.Phase = "qualifying"
	case "maintenance-carrier-restore":
		if !m.Loaded || !e.drained() || m.Phase == "qualifying" || len(e.maintenanceBlockers()) > 0 {
			return errors.New("測試片尚未全部回槽或仍有阻擋")
		}
		for _, l := range e.Lots {
			if l.Purpose == "dummy" && l.LP == m.SelectedLP {
				l.Parked = true
			}
		}
		if l := e.lot(m.ParkedLotID); l != nil {
			l.Parked = false
		}
		pump := e.lock(m.SavedLock.ID).Pump
		*e.lock(m.SavedLock.ID) = *m.SavedLock
		e.lock(m.SavedLock.ID).Pump = pump
		m.Loaded = false
		e.LastDispatchOrder = m.SavedDispatch
		e.NextLoadLock = m.SavedNextLock
		if m.Phase == "passed" {
			m.Phase = "ready"
		}
	default:
		return errors.New("未知保養指令")
	}
	m.LastCommandID = c.CommandID
	e.updateMaintenance()
	e.maintenanceEvent(c.Type)
	return nil
}
func (e *Engine) maintenanceResume(c Command) error {
	if !e.maintenanceHeld() {
		return nil
	}
	m := e.Maintenance
	if err := e.maintenanceFence(c); err != nil {
		return err
	}
	if m.Phase != "ready" || m.Loaded || m.Run == nil || m.Run.Status != "passed" || m.Run.Generation != m.Generation || m.Run.RecipeVersion != e.RecipeVersion || len(e.maintenanceBlockers()) > 0 {
		return errors.New("需完成四 PM 驗證、換回 FOUP 及所有互鎖才能復機")
	}
	a, ok := c.Parameters["dummyApproval"].(map[string]any)
	if !ok || a["runId"] != m.Run.ID || a["recipeVersion"] != e.RecipeVersion {
		return errors.New("需完成四片 KLA 量測與 PE 合格確認")
	}
	digest, _ := a["digest"].(string)
	id, _ := a["confirmationId"].(string)
	if len(digest) != 64 || id == "" {
		return errors.New("PE 復機授權無效")
	}
	return nil
}

func (e *Engine) prepareDummy(id string) *Lot {
	m := e.Maintenance
	for _, old := range e.Lots {
		if old.Purpose == "dummy" && old.LP == m.SelectedLP {
			old.Parked = true
		}
	}
	l := &Lot{ID: id + "-dummy", LP: m.SelectedLP, LL: m.SavedLock.ID, Phase: "prepared", Purpose: "dummy", CarrierID: m.CarrierID, Order: len(e.Lots) + 1}
	for i := 1; i <= 4; i++ {
		l.Wafers = append(l.Wafers, &Wafer{ID: fmt.Sprintf("%s-W%02d", id, i), LotID: l.ID, Slot: i, Location: l.LP, Result: "pending", Purpose: "dummy", TargetPM: fmt.Sprintf("PM%d", i)})
	}
	e.Lots = append(e.Lots, l)
	m.LoadedLotID = l.ID
	e.lock(l.LL).LotID = l.ID
	return l
}
