package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func maintenanceCmd(e *Engine, kind string, extra map[string]any) Result {
	p := map[string]any{}
	if e.Maintenance != nil {
		p["maintenanceId"] = e.Maintenance.ID
		p["expectedGeneration"] = e.Maintenance.Generation
	}
	for k, v := range extra {
		p[k] = v
	}
	return cmd(e, kind, "", "", p)
}
func requireMaintenance(t *testing.T, e *Engine, kind string, p map[string]any) {
	t.Helper()
	if kind == "resume" && e.Maintenance != nil && e.Maintenance.Run != nil {
		if p == nil {
			p = map[string]any{}
		}
		p["dummyApproval"] = map[string]any{"runId": e.Maintenance.Run.ID, "recipeVersion": e.RecipeVersion, "confirmationId": "test-pe-confirmation", "digest": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	}
	if r := maintenanceCmd(e, kind, p); r.Status != "succeeded" {
		t.Fatalf("%s: %+v", kind, r)
	}
}
func untilMaintenance(t *testing.T, e *Engine, phase string) {
	t.Helper()
	for i := 0; i < 2500 && e.Maintenance.Phase != phase; i++ {
		tick(t, e)
	}
	if e.Maintenance.Phase != phase {
		t.Fatalf("want %s got %+v", phase, e.Maintenance)
	}
}
func TestMaintenanceThreeOccupiedPortsAndFourPM(t *testing.T) {
	e := NewEngine()
	for i := 1; i <= 3; i++ {
		start(e, i)
	}
	for i := 0; i < 14; i++ {
		tick(t, e)
	}
	requireMaintenance(t, e, "maintenance-request", nil)
	untilMaintenance(t, e, "maintenance")
	snapshot := []Wafer{}
	for _, l := range e.Lots {
		for _, w := range l.Wafers {
			snapshot = append(snapshot, *w)
		}
	}
	if r := cmd(e, "resume", "", "", nil); r.Status != "rejected" {
		t.Fatal("resume bypass")
	}
	requireMaintenance(t, e, "maintenance-carrier-load", map[string]any{"lp": "LP2"})
	if !e.Lots[1].Parked {
		t.Fatal("not parked")
	}
	requireMaintenance(t, e, "maintenance-end", nil)
	requireMaintenance(t, e, "qualification-start", nil)
	untilMaintenance(t, e, "passed")
	run := e.Maintenance.Run
	if len(run.Results) != 4 {
		t.Fatal(run)
	}
	for i := 1; i <= 4; i++ {
		r := run.Results[fmt.Sprintf("PM%d", i)]
		if !r.Complete || r.Result.Quality != "pass" {
			t.Fatal(r)
		}
	}
	requireMaintenance(t, e, "maintenance-carrier-restore", nil)
	if e.Maintenance.Phase != "ready" {
		t.Fatal(e.Maintenance)
	}
	after := []Wafer{}
	for _, l := range e.Lots {
		if l.Purpose != "dummy" {
			for _, w := range l.Wafers {
				after = append(after, *w)
			}
		}
	}
	if !reflect.DeepEqual(snapshot, after) {
		t.Fatal("production changed during dummy")
	}
	requireMaintenance(t, e, "resume", nil)
	if e.maintenanceHeld() || e.DispatchPaused {
		t.Fatal("not released")
	}
}
func TestMaintenanceFailureRecipeAndRestart(t *testing.T) {
	e := NewEngine()
	requireMaintenance(t, e, "maintenance-request", nil)
	requireMaintenance(t, e, "maintenance-end", nil)
	requireMaintenance(t, e, "maintenance-carrier-load", map[string]any{"lp": "LP1"})
	requireMaintenance(t, e, "qualification-start", nil)
	for e.Chambers[0].Process == nil {
		tick(t, e)
	}
	if r := cmd(e, "configure", "", "", map[string]any{"processSeconds": float64(9)}); r.Status != "rejected" {
		t.Fatal("recipe allowed")
	}
	e.Chambers[0].Process.TargetThickness = 1000
	untilMaintenance(t, e, "failed")
	requireMaintenance(t, e, "qualification-start", nil)
	untilMaintenance(t, e, "passed")
	path := filepath.Join(t.TempDir(), "state.json")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	e2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !e2.Paused || !e2.maintenanceHeld() {
		t.Fatal("restart lost hold")
	}
	requireMaintenance(t, e2, "maintenance-carrier-restore", nil)
	if r := cmd(e2, "configure", "", "", map[string]any{"processSeconds": float64(9)}); r.Status != "succeeded" {
		t.Fatal(r)
	}
	if r := maintenanceCmd(e2, "resume", nil); r.Status != "rejected" {
		t.Fatal("old recipe qualification reused")
	}
	if e2.Maintenance.Generation == e.Maintenance.Generation {
		t.Fatal("no fence")
	}
}
func TestMaintenanceRejectsInFlightSwapAndStaleCommands(t *testing.T) {
	e := processEngine(t)
	requireMaintenance(t, e, "maintenance-request", nil)
	if r := maintenanceCmd(e, "maintenance-carrier-load", map[string]any{"lp": "LP1"}); r.Status != "rejected" {
		t.Fatal("in flight swap")
	}
	c := Command{CommandID: "stale", Type: "maintenance-end", ExpiresAt: base.Add(time.Hour), Parameters: map[string]any{"maintenanceId": "wrong", "expectedGeneration": 1}}
	if r := e.Execute(c, base); r.Status != "rejected" {
		t.Fatal("stale command")
	}
}
func TestMaintenanceDoesNotLetDispositionReleaseResumeProduction(t *testing.T) {
	e, r, i := completeSupplement(t)
	requireMaintenance(t, e, "maintenance-request", nil)
	a := DispositionAction{IncidentID: i.ID, ExecutionID: r.ExecutionID, ResultVersion: i.ResultVersion, AuthorizationID: "maint-confirm"}
	if result := dispositionSend(e, "disposition-confirm", a); result.Status != "succeeded" {
		t.Fatal(result)
	}
	for n := 0; n < 100 && !i.Returned; n++ {
		tick(t, e)
	}
	if !i.Returned {
		t.Fatal("not returned")
	}
	release := DispositionAction{ReleaseID: "maint-release", Generation: e.HoldGeneration, Evidence: e.releaseEvidence()}
	if x := dispositionSend(e, "equipment-release", release); x.Status != "succeeded" {
		t.Fatal(x)
	}
	if e.DispositionHold || !e.DispatchPaused || !e.maintenanceHeld() {
		t.Fatal("maintenance bypass")
	}
	untilMaintenance(t, e, "maintenance")
}
func TestMaintenanceMidRunRestartAndUtilization(t *testing.T) {
	e := NewEngine()
	requireMaintenance(t, e, "maintenance-request", nil)
	tick(t, e)
	tick(t, e)
	for _, v := range latestUtil(t, e).PM {
		if v {
			t.Fatal("maintenance busy")
		}
	}
	requireMaintenance(t, e, "maintenance-end", nil)
	requireMaintenance(t, e, "maintenance-carrier-load", map[string]any{"lp": "LP3"})
	requireMaintenance(t, e, "qualification-start", nil)
	for e.Chambers[0].Process == nil {
		tick(t, e)
	}
	tick(t, e)
	if !latestUtil(t, e).PM["PM1"] {
		t.Fatal("dummy not counted")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	e.Save(path)
	restored, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	requireMaintenance(t, restored, "recovery-clock", nil)
	untilMaintenance(t, restored, "passed")
	if len(restored.Maintenance.Runs[0].Results) != 4 {
		t.Fatal("restart lost history result references")
	}
}
func TestMaintenanceLoadedSlotsReplayAndEmptyRestore(t *testing.T) {
	e := NewEngine()
	requireMaintenance(t, e, "maintenance-request", nil)
	c := Command{CommandID: "load-once", Type: "maintenance-carrier-load", ExpiresAt: base.Add(time.Hour), Parameters: map[string]any{"maintenanceId": e.Maintenance.ID, "expectedGeneration": e.Maintenance.Generation, "lp": "LP1"}}
	if r := e.Execute(c, base); r.Status != "succeeded" {
		t.Fatal(r)
	}
	l := e.lot(e.Maintenance.LoadedLotID)
	if l == nil || len(l.Wafers) != 4 {
		t.Fatal("missing loaded slots")
	}
	if r := e.Execute(c, base); r.Status != "succeeded" {
		t.Fatal(r)
	}
	if r := cmd(e, "pause", "", l.ID, nil); r.Status != "rejected" {
		t.Fatal("dummy paused through production command")
	}
	if len(e.Lots) != 1 {
		t.Fatal("replay duplicated carrier")
	}
	for n := 0; n < 10; n++ {
		tick(t, e)
	}
	for _, w := range l.Wafers {
		if w.Location != "LP1" {
			t.Fatal("automatic dummy start")
		}
	}
	requireMaintenance(t, e, "maintenance-carrier-restore", nil)
	if e.Locks[0].LotID != "" || !l.Parked {
		t.Fatal("empty LP not restored")
	}
}

func TestDummyPEGrantRequiredAndFenced(t *testing.T) {
	e := NewEngine()
	requireMaintenance(t, e, "maintenance-request", nil)
	requireMaintenance(t, e, "maintenance-end", nil)
	requireMaintenance(t, e, "maintenance-carrier-load", map[string]any{"lp": "LP1"})
	requireMaintenance(t, e, "qualification-start", nil)
	untilMaintenance(t, e, "passed")
	requireMaintenance(t, e, "maintenance-carrier-restore", nil)
	if maintenanceCmd(e, "resume", nil).Status != "rejected" {
		t.Fatal("unsigned PE qualification bypass")
	}
	bad := map[string]any{"dummyApproval": map[string]any{"runId": "other-run", "recipeVersion": e.RecipeVersion, "confirmationId": "pe", "digest": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}}
	if maintenanceCmd(e, "resume", bad).Status != "rejected" {
		t.Fatal("cross-run authorization accepted")
	}
	requireMaintenance(t, e, "resume", nil)
}

func TestDummyRejectedRunRetestAndRestart(t *testing.T) {
	e := NewEngine()
	requireMaintenance(t, e, "maintenance-request", nil)
	requireMaintenance(t, e, "maintenance-end", nil)
	requireMaintenance(t, e, "maintenance-carrier-load", map[string]any{"lp": "LP1"})
	requireMaintenance(t, e, "qualification-start", nil)
	untilMaintenance(t, e, "passed")
	old := e.Maintenance.Run.ID
	requireMaintenance(t, e, "qualification-start", map[string]any{"peRetest": true})
	if e.Maintenance.Run.ID == old || len(e.Maintenance.Run.Results) != 0 {
		t.Fatal("retest reused old results")
	}
	untilMaintenance(t, e, "passed")
	requireMaintenance(t, e, "maintenance-carrier-restore", nil)
	file := filepath.Join(t.TempDir(), "state.json")
	if err := e.Save(file); err != nil {
		t.Fatal(err)
	}
	restored, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.DispatchPaused || !restored.maintenanceHeld() {
		t.Fatal("restart removed maintenance hold")
	}
	if maintenanceCmd(restored, "resume", nil).Status != "rejected" {
		t.Fatal("restart bypassed PE grant")
	}
}
