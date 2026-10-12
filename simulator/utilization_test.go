package main

import (
	"encoding/json"
	"testing"
	"time"
)

func latestUtil(t *testing.T, e *Engine) *Utilization {
	t.Helper()
	for i := len(e.Events) - 1; i >= 0; i-- {
		if e.Events[i].Kind == "telemetry" {
			b, _ := json.Marshal(e.Events[i].Payload)
			var p struct {
				Utilization *Utilization `json:"utilization"`
			}
			if err := json.Unmarshal(b, &p); err != nil {
				t.Fatal(err)
			}
			return p.Utilization
		}
	}
	t.Fatal("missing telemetry")
	return nil
}
func TestUtilizationPausedGapsAndRestart(t *testing.T) {
	e := NewEngine()
	e.Paused = true
	e.Tick(base)
	if latestUtil(t, e) != nil {
		t.Fatal("invented startup interval")
	}
	e.Tick(base.Add(time.Second))
	u := latestUtil(t, e)
	if u == nil || len(u.PM) != 4 {
		t.Fatal(u)
	}
	for _, active := range u.PM {
		if active {
			t.Fatal("paused processing")
		}
	}
	e.Tick(base.Add(5 * time.Second))
	if latestUtil(t, e) != nil {
		t.Fatal("invented gap")
	}
	e.Tick(base.Add(6 * time.Second))
	if latestUtil(t, e) == nil {
		t.Fatal("not resumed")
	}
	b, _ := json.Marshal(e)
	var restart Engine
	json.Unmarshal(b, &restart)
	restart.Tick(base.Add(7 * time.Second))
	if latestUtil(t, &restart) != nil {
		t.Fatal("invented restart interval")
	}
}
func TestUtilizationActualStepsAndFaultHold(t *testing.T) {
	e := processEngine(t)
	e.DispatchPaused = true
	phases := map[string]bool{}
	for e.Chambers[0].Remaining > 0 {
		phase := e.Chambers[0].Process.Phase
		tick(t, e)
		u := latestUtil(t, e)
		if u == nil || !u.PM["PM1"] {
			t.Fatal("active step not counted", phase)
		}
		phases[phase] = true
	}
	if len(phases) != 4 {
		t.Fatal(phases)
	}
	e = processEngine(t)
	e.Chambers[0].Fault = true
	tick(t, e)
	if latestUtil(t, e).PM["PM1"] {
		t.Fatal("fault counted")
	}
	e.Chambers[0].Fault = false
	e.wafer(e.Chambers[0].WaferID).Result = "exception"
	tick(t, e)
	if latestUtil(t, e).PM["PM1"] {
		t.Fatal("hold counted")
	}
}
func TestUtilizationSupplementOnlyExecutedSeconds(t *testing.T) {
	e, p, r := recoveryFixture(t)
	if result := recoverySend(e, "wafer-supplement", r); result.Status != "succeeded" {
		t.Fatal(result)
	}
	tick(t, e)
	if latestUtil(t, e).PM[p.ID] {
		t.Fatal("waiting approval execution counted")
	}
	cmd(e, "clear", p.ID, "", nil)
	for i := 0; i < 2; i++ {
		tick(t, e)
		if !latestUtil(t, e).PM[p.ID] {
			t.Fatal("supplement missing")
		}
	}
	tick(t, e)
	if latestUtil(t, e).PM[p.ID] {
		t.Fatal("awaiting PE result counted")
	}
}
