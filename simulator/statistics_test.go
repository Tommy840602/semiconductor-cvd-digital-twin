package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestStatisticsClassification(t *testing.T) {
	for _, c := range []struct {
		f    int
		p, d bool
		s    string
	}{{0, false, false, "idle"}, {1 | 16, true, false, "processing"}, {1 | 32, false, false, "fault"}, {65536, true, false, "paused"}, {2 << 6, false, false, "maintenance"}, {4 << 6, false, false, "dummy"}, {7 << 6, false, false, "paused"}, {8 << 6, false, false, "idle"}, {0, false, true, "dummy"}} {
		if got := statisticsState(c.f, c.p, c.d); got != c.s {
			t.Fatalf("%+v got %s", c, got)
		}
	}
}
func closedEvents(e *Engine) int {
	n := 0
	for _, v := range e.Events {
		if v.Kind == "wafer-closed" {
			n++
		}
	}
	return n
}
func TestStatisticsNormalReturnExactlyOnceAndRestart(t *testing.T) {
	e := NewEngine()
	start(e, 1)
	for n := 0; n < 1000 && e.StatisticsClosureCount < 1; n++ {
		tick(t, e)
	}
	if e.StatisticsClosureCount != 1 {
		t.Fatal("no closure")
	}
	w := e.Lots[0].Wafers[0]
	if w.ClosedAt == nil || w.LastReturnAt == nil || !w.ClosedAt.Equal(*w.LastReturnAt) || w.Location != "LP1" {
		t.Fatal(w)
	}
	before := closedEvents(e)
	e.closeWafers()
	if before != closedEvents(e) {
		t.Fatal("duplicate")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	e.Save(path)
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r.closeWafers()
	if r.StatisticsClosureCount != e.StatisticsClosureCount || r.StatisticsEra != e.StatisticsEra {
		t.Fatal("restart changed ledger")
	}
}
func TestStatisticsRecoveryRequiresPEAndReturn(t *testing.T) {
	e, r, i := completeSupplement(t)
	if closedEvents(e) != 0 {
		t.Fatal("closed awaiting PE")
	}
	a := DispositionAction{IncidentID: i.ID, ExecutionID: r.ExecutionID, ResultVersion: i.ResultVersion, AuthorizationID: "statistics-pe"}
	if x := dispositionSend(e, "disposition-confirm", a); x.Status != "succeeded" {
		t.Fatal(x)
	}
	w := e.wafer(i.WaferID)
	if w.ClosedAt != nil {
		t.Fatal("closed before return")
	}
	for n := 0; n < 100 && w.ClosedAt == nil; n++ {
		tick(t, e)
	}
	if w.ClosedAt == nil || i.ConfirmedAt == nil || w.ClosedAt.Before(*i.ConfirmedAt) || w.Location != e.lot(i.LotID).LP {
		t.Fatal("closure evidence")
	}
	before := e.StatisticsClosureCount
	release := DispositionAction{ReleaseID: "statistics-release", Generation: e.HoldGeneration, Evidence: e.releaseEvidence()}
	if x := dispositionSend(e, "equipment-release", release); x.Status != "succeeded" {
		t.Fatal(x)
	}
	if before != e.StatisticsClosureCount {
		t.Fatal("Leader counted twice")
	}
}
func TestStatisticsLegacyAndCommandBoundary(t *testing.T) {
	e := NewEngine()
	start(e, 1)
	w := e.Lots[0].Wafers[0]
	w.Result = "success"
	w.Attempts = 1
	e.closeWafers()
	if w.ClosedAt != nil {
		t.Fatal("invented legacy time")
	}
	e.Paused = true
	e.Tick(base)
	e.Tick(base.Add(time.Second))
	s := e.telemetryPayload(base, base.Add(time.Second), map[string]bool{})["statistics"].(Statistics)
	if s.State != "paused" {
		t.Fatal(s)
	}
	e.statisticsDirty = true
	s = e.statisticsPayload(map[string]bool{})
	if s.State != "unknown" {
		t.Fatal("command interval assumed")
	}
}

// A robot can pick the completed wafer in the same tick; classification uses
// the purpose captured when PM processing advanced, not its now-empty slot.
func TestStatisticsDummyCompletingAndMoving(t *testing.T) {
	e := NewEngine()
	e.statisticsBefore = 0
	e.statisticsDummy = map[string]bool{"PM1": true}
	e.Chambers[0].WaferID = ""
	if got := e.statisticsPayload(map[string]bool{"PM1": true}); got.State != "dummy" {
		t.Fatal(got)
	}
}

func TestStatisticsScrapReturnBeforePE(t *testing.T) {
	e, p, r := recoveryFixture(t)
	r.DispositionType = "scrap"
	r.StepSeconds = [4]int{}
	r.Target = 0
	r.RootRate = 0
	if x := recoverySend(e, "wafer-scrap", r); x.Status != "succeeded" {
		t.Fatal(x)
	}
	cmd(e, "clear", p.ID, "", nil)
	i := e.Incidents[r.IncidentID]
	for n := 0; n < 100 && !i.Returned; n++ {
		tick(t, e)
	}
	w := e.wafer(r.WaferID)
	if !i.Returned || w.ClosedAt != nil {
		t.Fatal("premature scrap closure")
	}
	a := DispositionAction{IncidentID: i.ID, ExecutionID: r.ExecutionID, ResultVersion: i.ResultVersion, AuthorizationID: "late-pe"}
	if x := dispositionSend(e, "disposition-confirm", a); x.Status != "succeeded" {
		t.Fatal(x)
	}
	if w.ClosedAt == nil || !w.ClosedAt.Equal(*w.LastReturnAt) || e.StatisticsClosureCount != 1 {
		t.Fatal("wrong final evidence time")
	}
	e.closeWafers()
	if e.StatisticsClosureCount != 1 {
		t.Fatal("replayed scrap counted twice")
	}
}
func TestStatisticsTwoSupplementsCloseOnce(t *testing.T) {
	e, r, i := completeSupplement(t)
	a := DispositionAction{IncidentID: i.ID, ExecutionID: r.ExecutionID, ResultVersion: i.ResultVersion, AuthorizationID: "reject-stats", Reason: "second disposition required"}
	if x := dispositionSend(e, "disposition-reject", a); x.Status != "succeeded" {
		t.Fatal(x)
	}
	for n := 0; n < 3; n++ {
		tick(t, e)
	}
	if e.StatisticsClosureCount != 0 || e.wafer(r.WaferID).ClosedAt != nil {
		t.Fatal("rejected supplement counted")
	}

	p := e.chamber(r.PMID)
	second := *r
	second.ExecutionID = "statistics-second"
	second.Revision++
	second.SourceRunID = p.ProcessRunID
	second.ExceptionVersion = e.wafer(r.WaferID).ExceptionVersion
	second.Baseline = p.Process.SampleThickness
	second.Target = p.Process.Thickness + second.RootRate*2
	if x := recoverySend(e, "wafer-supplement", &second); x.Status != "succeeded" {
		t.Fatal(x)
	}
	for n := 0; n < 10 && e.Recoveries[second.ExecutionID].Status != "awaiting_result"; n++ {
		tick(t, e)
	}
	if e.StatisticsClosureCount != 0 {
		t.Fatal("second supplement prematurely counted")
	}
	a.ExecutionID = second.ExecutionID
	a.ResultVersion = i.ResultVersion
	a.AuthorizationID = "statistics-final-pe"
	if x := dispositionSend(e, "disposition-confirm", a); x.Status != "succeeded" {
		t.Fatal(x)
	}
	for n := 0; n < 100 && e.wafer(r.WaferID).ClosedAt == nil; n++ {
		tick(t, e)
	}
	if e.wafer(r.WaferID).ClosedAt == nil || e.StatisticsClosureCount != 1 {
		t.Fatal("two supplements must close once")
	}
}

func TestStatisticsUpgradeCheckpointReadOnly(t *testing.T) {
	dir := os.Getenv("CVD_STATISTICS_CHECKPOINT_DIR")
	if dir == "" {
		t.Skip("set backup directory for deployment rehearsal")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) != 10 {
		t.Fatal("expected ten checkpoints")
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var old Engine
		if json.Unmarshal(raw, &old) != nil {
			t.Fatal(file)
		}
		next, err := Load(file)
		if err != nil {
			t.Fatal(err)
		}
		if !next.Paused || !reflect.DeepEqual(old.Lots, next.Lots) || !reflect.DeepEqual(old.Incidents, next.Incidents) || !reflect.DeepEqual(old.Maintenance, next.Maintenance) || !reflect.DeepEqual(old.Chambers, next.Chambers) || old.RecipeVersion != next.RecipeVersion || old.DispositionHold != next.DispositionHold {
			t.Fatal("business state changed", file)
		}
	}
}
