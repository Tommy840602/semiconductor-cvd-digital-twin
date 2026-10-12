package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func cmd(e *Engine, typ, target, lot string, params map[string]any) Result {
	return e.Execute(Command{CommandID: fmt.Sprintf("c%d", len(e.Commands)), Type: typ, TargetID: target, LotID: lot, ExpiresAt: base.Add(24 * time.Hour), Parameters: params}, base)
}
func start(e *Engine, i int) {
	cmd(e, "start", "", fmt.Sprintf("L%d", i), map[string]any{"lp": fmt.Sprintf("LP%d", i), "ll": fmt.Sprintf("LL%d", i)})
}
func tick(t *testing.T, e *Engine) {
	t.Helper()
	e.Tick(base.Add(time.Duration(e.TickNumber+1) * time.Second))
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestThreeLotsConcurrent(t *testing.T) {
	e := NewEngine()
	for i := 1; i <= 3; i++ {
		start(e, i)
	}
	max := 0
	mixed := false
	for n := 0; n < 2000; n++ {
		tick(t, e)
		running := 0
		batch := map[string]bool{}
		for _, p := range e.Chambers {
			if p.WaferID != "" {
				running++
				w := e.wafer(p.WaferID)
				batch[w.LotID] = true
			}
		}
		if len(batch) == 3 {
			mixed = true
		}
		if running > max {
			max = running
		}
		if e.Lots[0].Phase == "complete" && e.Lots[1].Phase == "complete" && e.Lots[2].Phase == "complete" {
			break
		}
	}
	if !mixed {
		t.Fatal("three batches never overlap")
	}
	if max != 4 {
		t.Fatalf("parallel=%d", max)
	}
	for _, l := range e.Lots {
		if l.Phase != "complete" {
			t.Fatalf("unfinished %+v", l)
		}
		for _, w := range l.Wafers {
			if w.Location != l.LP || w.Result != "success" || w.Attempts != 1 {
				t.Fatal(w)
			}
		}
	}
}
func TestFaultDisposition(t *testing.T) {
	for _, action := range []string{"scrap"} {
		t.Run(action, func(t *testing.T) {
			e := NewEngine()
			start(e, 1)
			for e.Chambers[0].WaferID == "" {
				tick(t, e)
			}
			id := e.Chambers[0].WaferID
			cmd(e, "fault", "PM1", "L1", nil)
			for i := 0; i < 40; i++ {
				tick(t, e)
			}
			if e.wafer(id).Result != "exception" || !e.Lots[0].Paused {
				t.Fatal("fault not held")
			}
			cmd(e, "clear", "PM1", "L1", nil)
			for i := 0; i < 40; i++ {
				tick(t, e)
			}
			if e.wafer(id).Result != "exception" {
				t.Fatal("clear resumed abnormal wafer")
			}
			if r := cmd(e, "resume", "", "L1", nil); r.Status != "rejected" {
				t.Fatal("premature resume")
			}
			if r := cmd(e, action, "PM1", "L1", nil); r.Status != "rejected" {
				t.Fatal("direct scrap bypass")
			}
			if r := cmd(e, "resume", "", "L1", nil); r.Status != "rejected" {
				t.Fatal("release bypass")
			}
			if !e.DispositionHold || e.wafer(id).Location != "PM1" {
				t.Fatal("hold lost")
			}

		})
	}
}
func TestCheckpointAndDuplicate(t *testing.T) {
	e := NewEngine()
	start(e, 1)
	tick(t, e)
	p := filepath.Join(t.TempDir(), "state.json")
	if err := e.Save(p); err != nil {
		t.Fatal(err)
	}
	r, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Paused || r.ATM.WaferID != e.ATM.WaferID {
		t.Fatal("checkpoint")
	}
	before := len(r.Lots)
	r.Execute(Command{CommandID: "c0"}, base)
	if len(r.Lots) != before {
		t.Fatal("duplicate")
	}
	if result := r.Execute(Command{CommandID: "expired", ExpiresAt: base}, base); result.Status != "rejected" {
		t.Fatal("expired accepted")
	}
}
func TestCapacityAndSamples(t *testing.T) {
	e := NewEngine()
	start(e, 1)
	r := cmd(e, "start", "", "other", map[string]any{"lp": "LP2", "ll": "LL1"})
	if r.Status != "rejected" {
		t.Fatal("occupied LL accepted")
	}
	for n := 0; n < 40; n++ {
		tick(t, e)
		for _, ll := range e.Locks {
			if ll.Phase == "pumping" || ll.Phase == "venting" {
				if e.ATM.To == ll.ID || e.VAC.To == ll.ID {
					t.Fatal("transfer during pressure transition")
				}
			}
		}
	}
	samples := 0
	events := 0
	for _, v := range e.Events {
		if v.Kind == "telemetry" {
			samples++
		} else {
			events++
		}
	}
	if samples != 40 || events == 0 {
		t.Fatal(samples, events)
	}
}

func TestSharedEnvelope(t *testing.T) {
	b, err := os.ReadFile("../contracts/example-event.json")
	if err != nil {
		t.Fatal(err)
	}
	var e Event
	if err = json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	if e.WaferID != "L1-W01" || e.Sequence != 1 || e.Quality != "good" {
		t.Fatal(e)
	}
}

func TestRejectedFaultDoesNotPause(t *testing.T) {
	e := NewEngine()
	r := cmd(e, "fault", "unknown", "", nil)
	if r.Status != "rejected" || e.DispatchPaused {
		t.Fatal(r, e.DispatchPaused)
	}
}
func TestStateVersionRejectsStaleControl(t *testing.T) {
	e := NewEngine()
	e.Version = 4
	r := e.Execute(Command{CommandID: "stale", Type: "configure", ExpiresAt: base.Add(time.Hour), ExpectedStateVersion: 3, Parameters: map[string]any{"processSeconds": float64(8)}}, base)
	if r.Status != "rejected" || e.ProcessSeconds != 30 {
		t.Fatal(r)
	}
}

func TestPauseStillDrainsHealthyPM(t *testing.T) {
	e := NewEngine()
	start(e, 1)
	for e.Chambers[3].WaferID == "" {
		tick(t, e)
	}
	cmd(e, "fault", "PM1", "L1", nil)
	for i := 0; i < 40; i++ {
		tick(t, e)
	}
	for _, p := range e.Chambers[1:] {
		if p.WaferID != "" {
			t.Fatal("healthy PM did not drain")
		}
	}
	if !e.DispatchPaused {
		t.Fatal("dispatch resumed")
	}
}

func TestBufferOverflowPausesSimulation(t *testing.T) {
	e := NewEngine()
	start(e, 1)
	tick(t, e)
	before := e.TickNumber
	e.EnforceBuffer(base.Add(11 * time.Minute))
	e.Tick(base.Add(11 * time.Minute))
	if !e.Paused || e.TickNumber != before {
		t.Fatal("buffer overflow did not freeze state")
	}
	if e.Events[len(e.Events)-1].Kind != "data-gap" {
		t.Fatal("missing gap event")
	}
}

func TestSingleLockCapacityAndCycles(t *testing.T) {
	e := NewEngine()
	start(e, 1)
	pumps, vents := 0, 0
	previous := ""
	for n := 0; n < 1500 && e.Lots[0].Phase != "complete"; n++ {
		tick(t, e)
		phase := e.Locks[0].Phase
		if phase != previous {
			if phase == "pumping" {
				pumps++
			}
			if phase == "venting" {
				vents++
			}
		}
		previous = phase
	}
	if e.Lots[0].Phase != "complete" || pumps < 25 || vents < 25 {
		t.Fatalf("cycles %d/%d, phase %s", pumps, vents, e.Lots[0].Phase)
	}
	for i, w := range e.Lots[0].Wafers {
		if w.Slot != i+1 || w.Location != "LP1" {
			t.Fatal("original LP slot lost")
		}
	}
	e.Lots[0].Wafers[0].Location = "LL1"
	e.Lots[0].Wafers[1].Location = "LL1"
	e.Locks[0].LotID = "L1"
	if e.Validate() == nil {
		t.Fatal("two wafers accepted in single LL")
	}
}
func TestLegacyCapacityMigration(t *testing.T) {
	e := NewEngine()
	start(e, 1)
	e.ModelVersion = 0
	e.DispatchPaused = true
	e.Chambers[1].Fault = true
	for _, w := range e.Lots[0].Wafers {
		w.Location = "LL1"
	}
	path := filepath.Join(t.TempDir(), "legacy.json")
	b, _ := json.Marshal(e)
	os.WriteFile(path, b, 0600)
	restored, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Paused || !restored.DispatchPaused || !restored.Chambers[1].Fault || restored.ModelVersion != 6 {
		t.Fatal("migration lost pause/fault")
	}
	count := 0
	for _, event := range restored.Events {
		if event.Kind == "capacity-migration" {
			count++
		}
	}
	if count != 25 {
		t.Fatal("migration audit missing", count)
	}
	for _, w := range restored.Lots[0].Wafers {
		if w.Location != "LP1" || w.Result != "pending" {
			t.Fatal(w)
		}
	}
}
func TestPausedPendingWaferDoesNotBlockReturn(t *testing.T) {
	e := NewEngine()
	start(e, 1)
	// A single pending wafer at vacuum competes with a finished PM after a pause.
	l := e.Lots[0]
	l.Paused = true
	l.Wafers[0].Location = "PM1"
	l.Wafers[0].Result = "success"
	e.Chambers[0].WaferID = l.Wafers[0].ID
	l.Wafers[1].Location = "LL1"
	e.Locks[0].Phase = "vacuum"
	e.Locks[0].Pressure = .01
	for i := 0; i < 40; i++ {
		tick(t, e)
	}
	if l.Wafers[0].Location != "LP1" || l.Wafers[1].Location != "LP1" || e.Chambers[0].WaferID != "" || !l.Paused {
		t.Fatal("single slot deadlock")
	}
}

func TestPausedSamplingDoesNotAdvanceProcess(t *testing.T) {
	e := NewEngine()
	e.Paused = true
	before := e.TickNumber
	start := time.Now().UTC()
	for n := 0; n < 3; n++ {
		e.Tick(start.Add(time.Duration(n) * time.Second))
	}
	if e.TickNumber != before || len(e.Events) != 3 {
		t.Fatalf("paused sample/process invariant: tick=%d events=%d", e.TickNumber, len(e.Events))
	}
	for _, event := range e.Events {
		if event.Kind != "telemetry" {
			t.Fatal(event.Kind)
		}
	}
	e.Tick(start.Add(11 * time.Minute))
	if len(e.Events) != 3 {
		t.Fatal("exhausted buffer grew")
	}
	e.EnforceBuffer(start.Add(11 * time.Minute))
	e.EnforceBuffer(start.Add(12 * time.Minute))
	if len(e.Events) != 4 || e.Events[3].Kind != "data-gap" {
		t.Fatal("paused buffer must report one durable gap")
	}
}
