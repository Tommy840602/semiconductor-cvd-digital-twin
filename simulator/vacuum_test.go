package main

import (
	"math"
	"path/filepath"
	"testing"
	"time"
)

func TestVacuumModelBoundaries(t *testing.T) {
	for _, w := range []float64{0, .6, .8, 1} {
		p := &VacuumPump{Wear: w}
		p.refresh()
		if math.Abs(p.Health-100*(1-w)) > 1e-9 {
			t.Fatal(p)
		}
		if p.Remaining > 0 && pumpEfficiency(w+float64(p.Remaining)*.001)*12 < 3-1e-10 {
			t.Fatal("RUL wrong", p)
		}
	}
	if pumpPressure(.25, 12) > .01*(1+1e-10) || pumpPressure(.24, 12) <= .01 {
		t.Fatal("timeout boundary")
	}
}
func TestVacuumFailureRecoveryAndQualification(t *testing.T) {
	e := NewEngine()
	e.initVacuum()
	start(e, 1)
	ll := e.Locks[0]
	ll.Pump.SinceService = 900
	for n := 0; n < 30 && !ll.Pump.Fault; n++ {
		tick(t, e)
	}
	w := e.lockWafer(ll.ID)
	if w == nil || !e.VacuumHold || w.Result != "pending" {
		t.Fatal("no retained wafer", ll, e.VacuumHold)
	}
	if cmd(e, "resume", "", "", nil).Status != "rejected" {
		t.Fatal("bypass")
	}
	params := map[string]any{"cycleId": ll.Pump.Cycle.ID, "faultGeneration": ll.Pump.FaultGeneration}
	if r := cmd(e, "vacuum-recover", ll.ID, "", params); r.Status != "succeeded" {
		t.Fatal(r)
	}
	for n := 0; n < 15; n++ {
		tick(t, e)
	}
	if w.Location != e.Lots[0].LP || !ll.Pump.Fault || !e.VacuumHold {
		t.Fatal("recovery changed hold", ll, w)
	}
	requireMaintenance(t, e, "maintenance-request", map[string]any{"selectedPumpIds": []string{"LL1"}})
	untilMaintenance(t, e, "maintenance")
	requireMaintenance(t, e, "maintenance-end", nil)
	requireMaintenance(t, e, "vacuum-test", nil)
	for n := 0; n < 15; n++ {
		tick(t, e)
	}
	if e.Maintenance.PumpTest.Status != "passed" {
		t.Fatal(e.Maintenance.PumpTest)
	}
	requireMaintenance(t, e, "maintenance-carrier-load", map[string]any{"lp": "LP1"})
	requireMaintenance(t, e, "qualification-start", nil)
	untilMaintenance(t, e, "passed")
	requireMaintenance(t, e, "maintenance-carrier-restore", nil)
	requireMaintenance(t, e, "resume", nil)
	if e.VacuumHold || e.DispatchPaused || ll.Pump.SinceService < 1 {
		t.Fatal("not resumed")
	}
}
func TestVacuumIsolationAndPersistence(t *testing.T) {
	engines := make([]*Engine, 10)
	for i := range engines {
		engines[i] = NewEngine()
		engines[i].initVacuum()
	}
	e := engines[0]
	p := e.Locks[1].Pump
	p.SinceService = 650
	e.startPump(e.Locks[1], "production")
	e.stepPump(e.Locks[1])
	if !p.Warn || e.VacuumHold {
		t.Fatal("warning stopped")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Locks[1].Pump.Lifetime != 1 || loaded.Locks[1].Pump.Cycle.Elapsed != 1 {
		t.Fatal("lost progress")
	}
	loaded.Tick(time.Now())
	if loaded.Locks[1].Pump.Cycle.Elapsed != 1 {
		t.Fatal("pause advanced")
	}
	for i, x := range engines {
		for n, l := range x.Locks {
			if !(i == 0 && n == 1) && l.Pump.Lifetime != 0 {
				t.Fatal("cross pump")
			}
		}
	}

}

func TestVacuumLegacyCycleMigration(t *testing.T) {
	e := NewEngine()
	ll := e.Locks[0]
	ll.Phase = "pumping"
	ll.Remaining = 2
	path := filepath.Join(t.TempDir(), "state.json")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	x, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	x.Paused = false
	tick(t, x)
	tick(t, x)
	if x.Locks[0].Phase != "vacuum" || x.Locks[0].Pump.Lifetime != 0 || x.Locks[0].Pump.Legacy {
		t.Fatal("legacy cycle replayed")
	}
	x.startPump(x.Locks[0], "dummy")
	if x.Locks[0].Pump.Lifetime != 1 || x.Locks[0].Pump.Cycle.Purpose != "dummy" {
		t.Fatal("new cycle not modeled")
	}
}
func TestVacuumDeadlineAndCycleCount(t *testing.T) {
	e := NewEngine()
	e.initVacuum()
	ll := e.Locks[0]
	e.startPump(ll, "test")
	ll.Pump.Cycle.Efficiency = .25
	for n := 0; n < 12; n++ {
		e.stepPump(ll)
	}
	if ll.Pump.Cycle.Status != "passed" || ll.Pump.Lifetime != 1 {
		t.Fatal("deadline should pass")
	}
	e.startPump(ll, "production")
	ll.Pump.Cycle.Efficiency = .24
	for n := 0; n < 12; n++ {
		e.stepPump(ll)
	}
	if ll.Pump.Cycle.Status != "failed" || ll.Pump.Lifetime != 2 || !e.VacuumHold {
		t.Fatal("deadline failure missing")
	}
}

func TestAllPumpsFailedChannelDrainsAllLots(t *testing.T) {
	e := NewEngine()
	for i := 1; i <= 3; i++ {
		start(e, i)
	}
	e.initVacuum()
	// Put one completed wafer of each lot in a PM, with all LLs isolated and empty.
	for i, l := range e.Lots {
		w := l.Wafers[0]
		w.Location = e.Chambers[i].ID
		w.Result = "success"
		e.Chambers[i].WaferID = w.ID
		l.Phase = "processing"
	}
	for _, ll := range e.Locks {
		ll.Pump.SinceService = 900
		e.startPump(ll, "return")
		for n := 0; n < 12; n++ {
			e.stepPump(ll)
		}
	}
	ll := e.Locks[0]
	params := map[string]any{"cycleId": ll.Pump.Cycle.ID, "faultGeneration": ll.Pump.FaultGeneration}
	if r := cmd(e, "vacuum-service-channel", ll.ID, "", params); r.Status != "rejected" {
		t.Fatal("repair without atmosphere")
	}
	if r := cmd(e, "vacuum-recover", ll.ID, "", params); r.Status != "succeeded" {
		t.Fatal(r)
	}
	for n := 0; n < 8; n++ {
		tick(t, e)
	}
	if r := cmd(e, "vacuum-service-channel", ll.ID, "", params); r.Status != "succeeded" {
		t.Fatal(r)
	}
	for n := 0; n < 100; n++ {
		tick(t, e)
	}
	if !e.drained() || !e.VacuumHold || !ll.Pump.NeedsService {
		t.Fatal("not drained or lost hold", e.drained(), ll)
	}
	for _, l := range e.Lots {
		if l.Wafers[0].Location != l.LP || l.Wafers[0].Result != "success" {
			t.Fatal("wrong slot", l.Wafers[0])
		}
	}
	if r := cmd(e, "resume", "", "", nil); r.Status != "rejected" {
		t.Fatal("channel bypass")
	}
	if r := maintenanceCmd(e, "maintenance-request", map[string]any{"selectedPumpIds": []string{"LL2"}}); r.Status != "rejected" {
		t.Fatal("omitted channel")
	}
	requireMaintenance(t, e, "maintenance-request", map[string]any{"selectedPumpIds": []string{"LL1", "LL2", "LL3"}})
	untilMaintenance(t, e, "maintenance")
	requireMaintenance(t, e, "maintenance-end", nil)
	requireMaintenance(t, e, "vacuum-test", nil)
	for n := 0; n < 10; n++ {
		tick(t, e)
	}
	if !e.vacuumReady() {
		t.Fatal("all pumps not qualified", e.Maintenance.PumpTest)
	}
	requireMaintenance(t, e, "maintenance-carrier-load", map[string]any{"lp": "LP1"})
	requireMaintenance(t, e, "qualification-start", nil)
	untilMaintenance(t, e, "passed")
	requireMaintenance(t, e, "maintenance-carrier-restore", nil)
	requireMaintenance(t, e, "resume", nil)
}
func TestSelectedServiceIsIdempotentAndEvidenceInvalidates(t *testing.T) {
	e := NewEngine()
	e.initVacuum()
	for _, ll := range e.Locks {
		ll.Pump.SinceService = 500
		ll.Pump.Lifetime = 500
		ll.Pump.Wear = .5
	}
	requireMaintenance(t, e, "maintenance-request", map[string]any{"selectedPumpIds": []string{"LL1", "LL3"}})
	c := Command{CommandID: "service", Type: "maintenance-end", ExpiresAt: base.Add(time.Hour), Parameters: map[string]any{"maintenanceId": e.Maintenance.ID, "expectedGeneration": e.Maintenance.Generation}}
	if e.Execute(c, base).Status != "succeeded" {
		t.Fatal("service")
	}
	e.Execute(c, base)
	if e.Locks[0].Pump.Generation != 1 || e.Locks[0].Pump.Lifetime != 500 || e.Locks[1].Pump.SinceService != 500 {
		t.Fatal("service reset incorrect")
	}
	requireMaintenance(t, e, "vacuum-test", nil)
	for n := 0; n < 10; n++ {
		tick(t, e)
	}
	if !e.vacuumReady() {
		t.Fatal("not ready")
	}
	e.invalidateMaintenance("fault")
	if e.vacuumReady() {
		t.Fatal("stale test allowed")
	}
}

func TestPumpTestRestartDoesNotPass(t *testing.T) {
	e := NewEngine()
	e.initVacuum()
	requireMaintenance(t, e, "maintenance-request", map[string]any{"selectedPumpIds": []string{"LL1"}})
	requireMaintenance(t, e, "maintenance-end", nil)
	requireMaintenance(t, e, "vacuum-test", nil)
	tick(t, e)
	path := filepath.Join(t.TempDir(), "state.json")
	e.Save(path)
	x, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if x.Maintenance.PumpTest.Status != "interrupted" || x.vacuumReady() || !x.VacuumHold {
		t.Fatal("restart manufactured pass")
	}
	if x.Locks[0].Pump.Lifetime != 1 {
		t.Fatal("restart double count")
	}
}
func TestVacuumRecoveryRejectsStaleAndPreservesFinishedWafer(t *testing.T) {
	e := NewEngine()
	start(e, 1)
	e.initVacuum()
	ll := e.Locks[0]
	w := e.Lots[0].Wafers[0]
	w.Result = "success"
	w.Location = ll.ID
	ll.Pump.SinceService = 900
	e.startPump(ll, "return")
	for n := 0; n < 12; n++ {
		e.stepPump(ll)
	}
	if cmd(e, "vacuum-recover", ll.ID, "", map[string]any{"cycleId": "old", "faultGeneration": 0}).Status != "rejected" {
		t.Fatal("stale accepted")
	}
	params := map[string]any{"cycleId": ll.Pump.Cycle.ID, "faultGeneration": ll.Pump.FaultGeneration}
	e.ATM.WaferID = "occupied"
	if cmd(e, "vacuum-recover", ll.ID, "", params).Status != "rejected" {
		t.Fatal("busy accepted")
	}
	e.ATM.WaferID = ""
	if cmd(e, "vacuum-recover", ll.ID, "", params).Status != "succeeded" {
		t.Fatal("recover rejected")
	}
	for n := 0; n < 10; n++ {
		tick(t, e)
	}
	if w.Result != "success" || w.Location != e.Lots[0].LP || !e.VacuumHold {
		t.Fatal("result lost")
	}
}
