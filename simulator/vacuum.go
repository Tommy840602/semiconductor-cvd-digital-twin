package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

const vacuumModel = "synthetic-vacuum-v1"
const vacuumTimeout = 12

type PumpCycle struct {
	ID         string    `json:"id"`
	Purpose    string    `json:"purpose"`
	Status     string    `json:"status"`
	Elapsed    int       `json:"elapsedSeconds"`
	Efficiency float64   `json:"efficiency"`
	StartTime  string    `json:"startTime"`
	EndTime    string    `json:"endTime,omitempty"`
	Pressures  []float64 `json:"pressures"`
}
type VacuumPump struct {
	NeedsService    bool       `json:"needsService"`
	ChannelTesting  bool       `json:"channelTesting"`
	Model           string     `json:"modelVersion"`
	EnabledAt       string     `json:"enabledAt"`
	Lifetime        int        `json:"lifetimeCycles"`
	SinceService    int        `json:"cyclesSinceService"`
	Generation      int        `json:"serviceGeneration"`
	Wear            float64    `json:"wear"`
	Health          float64    `json:"health"`
	Remaining       int        `json:"remainingCycles"`
	Warn            bool       `json:"warning"`
	Fault           bool       `json:"fault"`
	FaultGeneration int        `json:"faultGeneration"`
	Recovering      bool       `json:"recovering"`
	Cycle           *PumpCycle `json:"cycle,omitempty"`
	Legacy          bool       `json:"legacyCycle,omitempty"`
}
type PumpTest struct {
	ID          string            `json:"id"`
	Status      string            `json:"status"`
	Generations map[string]int    `json:"generations"`
	Results     map[string]string `json:"results"`
}

func pumpEfficiency(w float64) float64 { return math.Max(.1, 1-.9*w) }
func pumpPressure(eff float64, t int) float64 {
	return 1013 * math.Exp(-math.Log(1013/.01)*eff*float64(t)/3)
}
func (p *VacuumPump) refresh() {
	p.Health = 100 * (1 - p.Wear)
	p.Remaining = 0
	for n := 1; n <= 1000; n++ {
		if pumpEfficiency(math.Min(1, p.Wear+float64(n)*.001))*vacuumTimeout < 3-1e-10 {
			break
		}
		p.Remaining++
	}
}
func (e *Engine) initVacuum() {
	for _, ll := range e.Locks {
		if ll.Pump == nil {
			ll.Pump = &VacuumPump{Model: vacuumModel, EnabledAt: e.Now.Format("2006-01-02T15:04:05Z07:00"), Legacy: ll.Phase == "pumping"}
			ll.Pump.refresh()
		}
	}
}
func (e *Engine) pumpEvent(kind string, ll *Lock) {
	raw, _ := json.Marshal(ll.Pump)
	var p any
	json.Unmarshal(raw, &p)
	e.emit("vacuum-"+kind, ll.LotID, "", map[string]any{"pumpId": ll.ID, "pump": p, "pressure": ll.Pressure, "commandId": e.maintenanceCommandID})
}
func (e *Engine) startPump(ll *Lock, purpose string) {
	p := ll.Pump
	p.Lifetime++
	p.SinceService++
	p.Wear = math.Min(1, float64(p.SinceService)*.001)
	p.refresh()
	p.Cycle = &PumpCycle{ID: fmt.Sprintf("%s-%s-%d", e.EquipmentID, ll.ID, p.Lifetime), Purpose: purpose, Status: "running", Efficiency: pumpEfficiency(p.Wear), StartTime: e.Now.Format("2006-01-02T15:04:05Z07:00"), Pressures: []float64{ll.Pressure}}
	ll.Phase = "pumping"
	ll.Remaining = vacuumTimeout
	if p.Wear >= .6 && !p.Warn {
		p.Warn = true
		e.pumpEvent("warning", ll)
	}
	e.pumpEvent("started", ll)
}
func (e *Engine) stepPump(ll *Lock) bool {
	p := ll.Pump
	if ll.Phase != "pumping" || p.Legacy {
		return false
	}
	if p.Cycle == nil || p.Cycle.Status != "running" {
		purpose := "production"
		if l := e.lot(ll.LotID); l != nil && l.Purpose == "dummy" {
			purpose = "dummy"
		}
		e.startPump(ll, purpose)
	}
	c := p.Cycle
	c.Elapsed++
	ll.Pressure = pumpPressure(c.Efficiency, c.Elapsed)
	c.Pressures = append(c.Pressures, ll.Pressure)
	ll.Remaining = vacuumTimeout - c.Elapsed
	if ll.Pressure <= .01*(1+1e-10) {
		c.Status = "passed"
		ll.Phase = "vacuum"
		ll.Remaining = 0
	} else if c.Elapsed >= vacuumTimeout {
		c.Status = "failed"
		ll.Phase = "vacuum-fault"
		ll.Remaining = 0
		p.Fault = true
		p.FaultGeneration++
		e.VacuumHold = true
		e.DispatchPaused = true
		e.invalidateMaintenance("vacuum-fault")
	}
	if c.Status != "running" {
		c.EndTime = e.Now.Format("2006-01-02T15:04:05Z07:00")
		e.pumpEvent("result", ll)
		if c.Purpose == "channel-test" {
			if c.Status == "passed" {
				ll.Phase = "venting"
				ll.Remaining = 3
			} else {
				p.ChannelTesting = false
			}
		}
		if c.Purpose == "test" && e.Maintenance != nil && e.Maintenance.PumpTest != nil {
			test := e.Maintenance.PumpTest
			test.Results[ll.ID] = c.Status
			if c.Status == "passed" {
				ll.Phase = "venting"
				ll.Remaining = 3
			}
		}
	}
	return true
}
func (e *Engine) vacuumReady() bool {
	for _, ll := range e.Locks {
		if ll.Pump.Fault || ll.Pump.Recovering || ll.Pump.NeedsService || ll.Pump.ChannelTesting {
			return false
		}
	}
	m := e.Maintenance
	if m == nil || len(m.SelectedPumps) == 0 {
		return !e.VacuumHold
	}
	t := m.PumpTest
	if t == nil || t.Status != "passed" {
		return false
	}
	for _, id := range m.SelectedPumps {
		p := e.lock(id).Pump
		if t.Generations[id] != p.Generation || p.Model != vacuumModel {
			return false
		}
	}
	return true
}
func (e *Engine) updatePumpTest() {
	if e.Maintenance == nil || e.Maintenance.PumpTest == nil {
		return
	}
	t := e.Maintenance.PumpTest
	if t.Status != "running" {
		return
	}
	complete := true
	passed := true
	for id := range t.Generations {
		ll := e.lock(id)
		if t.Results[id] == "" || ll.Remaining > 0 {
			complete = false
		}
		if t.Results[id] != "passed" {
			passed = false
		}
	}
	if complete {
		t.Status = "failed"
		if passed {
			t.Status = "passed"
		}
		e.maintenanceEvent("pump-test-result")
	}
}
func selectedPumps(c Command) ([]string, error) {
	raw, ok := c.Parameters["selectedPumpIds"]
	if !ok {
		return nil, nil
	}
	b, _ := json.Marshal(raw)
	var ids []string
	if json.Unmarshal(b, &ids) != nil || len(ids) == 0 || len(ids) > 3 {
		return nil, errors.New("vacuum.invalid-selection")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if (id != "LL1" && id != "LL2" && id != "LL3") || seen[id] {
			return nil, errors.New("vacuum.invalid-selection")
		}
		seen[id] = true
	}
	return ids, nil
}
func (e *Engine) vacuumCommand(c Command) error {
	e.initVacuum()
	if c.Type == "vacuum-test" {
		if err := e.maintenanceFence(c); err != nil {
			return err
		}
		m := e.Maintenance
		if (m.Phase != "waiting" && m.Phase != "failed") || len(m.SelectedPumps) == 0 || !e.drained() || e.Paused || e.DispositionHold {
			return errors.New("vacuum.test-blocked")
		}
		if m.PumpTest != nil && m.PumpTest.Status == "running" {
			return errors.New("vacuum.test-blocked")
		}
		for _, id := range m.SelectedPumps {
			ll := e.lock(id)
			if ll.Pump.Fault || ll.Pressure < 1012 || e.lockBusy(id) {
				return errors.New("vacuum.test-blocked")
			}
		}
		t := &PumpTest{ID: c.CommandID, Status: "running", Generations: map[string]int{}, Results: map[string]string{}}
		m.PumpTest = t
		for _, id := range m.SelectedPumps {
			ll := e.lock(id)
			t.Generations[id] = ll.Pump.Generation
			e.startPump(ll, "test")
		}
		e.maintenanceEvent("pump-test-started")
		return nil
	}
	ll := e.lock(c.TargetID)
	if ll == nil {
		return errors.New("vacuum.invalid-selection")
	}
	if c.Type == "vacuum-service-channel" {
		return e.serviceChannel(c, ll)
	}
	p := ll.Pump
	gen := fmt.Sprint(c.Parameters["faultGeneration"])
	if !p.Fault || p.Cycle == nil || fmt.Sprint(c.Parameters["cycleId"]) != p.Cycle.ID || gen != fmt.Sprint(p.FaultGeneration) || e.lockBusy(ll.ID) || e.ATM.WaferID != "" || e.VAC.WaferID != "" || p.Recovering {
		return errors.New("vacuum.recovery-blocked")
	}
	if w := e.lockWafer(ll.ID); w != nil {
		l := e.lot(w.LotID)
		if l == nil || l.Parked {
			return errors.New("vacuum.recovery-blocked")
		}
		for _, other := range l.Wafers {
			if other.ID != w.ID && other.Slot == w.Slot && other.Location == l.LP {
				return errors.New("vacuum.recovery-blocked")
			}
		}
	}
	p.Recovering = true
	ll.Phase = "venting"
	ll.Remaining = 3
	e.pumpEvent("recovery-started", ll)
	return nil
}
func (e *Engine) recoverVacuum() {
	for _, ll := range e.Locks {
		p := ll.Pump
		if p.ChannelTesting && ll.Remaining == 0 && ll.Phase == "atmosphere" {
			p.ChannelTesting = false
			e.pumpEvent("channel-ready", ll)
		}
		if !p.Recovering || ll.Remaining > 0 || ll.Pressure < 1012 || e.lockBusy(ll.ID) {
			continue
		}
		if w := e.lockWafer(ll.ID); w != nil {
			if e.ATM.WaferID == "" {
				e.move(&e.ATM, w, e.lot(w.LotID).LP)
			}
		} else {
			p.Recovering = false
			ll.Phase = "vacuum-fault"
			e.pumpEvent("recovered", ll)
		}
	}
}

// A return reservation is independent of a lot's normal LL binding.
func (e *Engine) returnLock(w *Wafer) *Lock {
	normal := e.lock(e.lot(w.LotID).LL)
	for _, ll := range e.Locks {
		if ll.ReturnLotID == w.LotID {
			return ll
		}
	}
	if !e.VacuumHold {
		return normal
	}
	if !normal.Pump.Fault && !normal.Pump.ChannelTesting && normal.ReturnLotID == "" {
		return normal
	}
	for _, ll := range e.Locks {
		if ll.Pump.Fault || ll.Pump.Recovering || ll.Pump.ChannelTesting || ll.ReturnLotID != "" || ll.Remaining > 0 || e.lockBusy(ll.ID) || e.lockWafer(ll.ID) != nil {
			continue
		}
		ll.ReturnLotID = w.LotID
		if ll.Phase != "vacuum" {
			e.startPump(ll, "return")
		}
		return ll
	}
	return nil
}
func (e *Engine) updateReturnLocks() {
	for _, ll := range e.Locks {
		if ll.ReturnLotID == "" || e.lockBusy(ll.ID) || ll.Remaining > 0 || ll.Pump.Fault {
			continue
		}
		w := e.lockWafer(ll.ID)
		if w != nil && ll.Phase == "vacuum" {
			ll.Phase = "venting"
			ll.Remaining = 3
		}
		if w == nil && ll.Phase == "atmosphere" {
			ll.ReturnLotID = ""
		}
	}
}
func (e *Engine) servicePump(ll *Lock) {
	e.pumpEvent("service-before", ll)
	p := ll.Pump
	p.Model = vacuumModel
	p.Wear = 0
	p.SinceService = 0
	p.Generation++
	p.Fault = false
	p.Warn = false
	p.Cycle = nil
	p.refresh()
	ll.Phase = "atmosphere"
	ll.Pressure = 1013
	e.pumpEvent("serviced", ll)
}
func (e *Engine) serviceChannel(c Command, ll *Lock) error {
	p := ll.Pump
	if !e.VacuumHold || !e.DispatchPaused || e.Paused || !p.Fault || p.Cycle == nil || fmt.Sprint(c.Parameters["cycleId"]) != p.Cycle.ID || fmt.Sprint(c.Parameters["faultGeneration"]) != fmt.Sprint(p.FaultGeneration) || e.lockWafer(ll.ID) != nil || e.lockBusy(ll.ID) || ll.Remaining > 0 || ll.Pressure < 1012 || p.Recovering || e.ATM.WaferID != "" || e.VAC.WaferID != "" {
		return errors.New("vacuum.recovery-blocked")
	}
	for _, x := range e.Locks {
		if x.Pump.ChannelTesting || (x.Pump.Cycle != nil && x.Pump.Cycle.Status == "running") {
			return errors.New("vacuum.test-blocked")
		}
	}
	if e.maintenanceHeld() && e.Maintenance.Phase != "draining" && e.Maintenance.Phase != "waiting" && e.Maintenance.Phase != "failed" {
		return errors.New("vacuum.test-blocked")
	}
	e.servicePump(ll)
	p.NeedsService = true
	p.ChannelTesting = true
	e.startPump(ll, "channel-test")
	return nil
}

func (e *Engine) interruptPumpTests() {
	for _, ll := range e.Locks {
		p := ll.Pump
		c := p.Cycle
		if c == nil || c.Status != "running" || (c.Purpose != "test" && c.Purpose != "channel-test") {
			continue
		}
		c.Status = "interrupted"
		c.EndTime = e.Now.Format("2006-01-02T15:04:05Z07:00")
		p.Fault = true
		p.FaultGeneration++
		p.ChannelTesting = false
		ll.Phase = "vacuum-fault"
		ll.Remaining = 0
		e.VacuumHold = true
		e.DispatchPaused = true
		e.pumpEvent("result", ll)
		if e.Maintenance != nil && e.Maintenance.PumpTest != nil {
			e.Maintenance.PumpTest.Status = "interrupted"
			e.Maintenance.PumpTest.Results[ll.ID] = "interrupted"
			e.maintenanceEvent("pump-test-interrupted")
		}
	}
}
