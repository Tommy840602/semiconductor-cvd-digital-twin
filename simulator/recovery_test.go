package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func recoveryFixture(t *testing.T) (*Engine, *Chamber, *Recovery) {
	t.Helper()
	key := filepath.Join(t.TempDir(), "key")
	os.WriteFile(key, []byte("01234567890123456789012345678901ab"), 0600)
	t.Setenv("CVD_RECOVERY_KEY_FILE", key)
	e := processEngine(t)
	p := e.Chambers[0]
	for p.Process.Thickness == 0 {
		tick(t, e)
	}
	cmd(e, "fault", p.ID, "L1", nil)
	r := &Recovery{IncidentID: e.activeIncident(p.ID, p.WaferID).ID, DispositionType: "supplement", ExecutionID: "execution-1", RequestID: "request-1", Revision: 1, WaferID: p.WaferID, PMID: p.ID, SourceRunID: p.ProcessRunID, ExceptionVersion: e.wafer(p.WaferID).ExceptionVersion, Baseline: p.Process.SampleThickness, StepSeconds: [4]int{0, 0, 2, 0}, Target: 10, RootRate: processRootRate(p.Process)}
	return e, p, r
}
func recoverySend(e *Engine, kind string, r *Recovery) Result {
	b, _ := json.Marshal(r)
	key, _ := os.ReadFile(os.Getenv("CVD_RECOVERY_KEY_FILE"))
	m := hmac.New(sha256.New, key)
	m.Write([]byte(kind + "\n" + string(b)))
	return cmd(e, kind, r.PMID, "", map[string]any{"payload": string(b), "signature": hex.EncodeToString(m.Sum(nil))})
}
func TestRecoveryAllStepCombinations(t *testing.T) {
	for mask := 1; mask < 16; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			e, p, r := recoveryFixture(t)
			r.StepSeconds = [4]int{}
			total := 0
			for i := 0; i < 4; i++ {
				if mask&(1<<i) != 0 {
					r.StepSeconds[i] = i + 1
					total += i + 1
				}
			}
			r.Target = p.Process.Thickness + r.RootRate*float64(r.StepSeconds[2])
			baseline := p.Process.Thickness
			originalRecipe := e.ProcessSeconds
			if x := recoverySend(e, "wafer-supplement", r); x.Status != "succeeded" {
				t.Fatal(x)
			}
			for i := 0; i < 3; i++ {
				tick(t, e)
			}
			if p.Process.Thickness != baseline || e.Recoveries[r.ExecutionID].Status != "approved_waiting" {
				t.Fatal("fault interlock bypass")
			}
			cmd(e, "clear", p.ID, "", nil)
			for i := 0; i < total; i++ {
				tick(t, e)
			}
			if p.Process.Elapsed != total || e.Recoveries[r.ExecutionID].Status != "awaiting_result" {
				t.Fatalf("%+v %+v", p.Process, e.Recoveries[r.ExecutionID])
			}
			if p.Process.BaselineThickness != baseline || e.ProcessSeconds != originalRecipe {
				t.Fatal("baseline or recipe changed")
			}
			if r.StepSeconds[2] == 0 && p.Process.Thickness != baseline {
				t.Fatal("non deposition growth")
			}
			if r.StepSeconds[2] > 0 && p.Process.Thickness <= baseline {
				t.Fatal("no growth")
			}
			counts := map[string]int{}
			for _, ev := range e.Events {
				if ev.Kind == "process-phase-changed" {
					b, _ := json.Marshal(ev.Payload)
					var v map[string]any
					json.Unmarshal(b, &v)
					if v["processRunId"] == p.ProcessRunID {
						counts[v["phase"].(string)]++
					}
				}
			}
			for i, name := range []string{"preheat", "stabilize", "deposition", "purge"} {
				if (counts[name] > 0) != (r.StepSeconds[i] > 0) {
					t.Fatal("unselected phase", name)
				}
			}
			attempts := e.wafer(r.WaferID).Attempts
			recoverySend(e, "wafer-supplement", r)
			tick(t, e)
			if e.wafer(r.WaferID).Attempts != attempts {
				t.Fatal("duplicate attempt")
			}
		})
	}
}
func TestRecoveryRestartAndFailure(t *testing.T) {
	e, p, r := recoveryFixture(t)
	r.StepSeconds = [4]int{0, 0, 8, 0}
	r.Target = 100
	recoverySend(e, "wafer-supplement", r)
	cmd(e, "clear", p.ID, "", nil)
	tick(t, e)
	tick(t, e)
	before := p.Process.Thickness
	path := filepath.Join(t.TempDir(), "state.json")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p = loaded.chamber(r.PMID)
	recoverySend(loaded, "wafer-supplement", r)
	tick(t, loaded)
	if p.Process.Thickness != before {
		t.Fatal("restart bypassed clock")
	}
	if x := cmd(loaded, "recovery-clock", "", "", nil); x.Status != "succeeded" {
		t.Fatal(x)
	}
	for i := 0; i < 6; i++ {
		tick(t, loaded)
	}
	if loaded.wafer(r.WaferID).Result != "exception" || loaded.Recoveries[r.ExecutionID].Status != "failed" || p.Process.Elapsed != 8 || !loaded.DispatchPaused || !loaded.Lots[0].Paused {
		t.Fatal("failure not held")
	}
	root := p.Process.RootRate
	r.ExecutionID = "execution-2"
	r.SourceRunID = p.ProcessRunID
	r.ExceptionVersion = loaded.wafer(r.WaferID).ExceptionVersion
	r.Baseline = p.Process.SampleThickness
	r.StepSeconds = [4]int{0, 0, 4, 0}
	r.RootRate = root
	if x := recoverySend(loaded, "wafer-supplement", r); x.Status != "succeeded" {
		t.Fatal(x)
	}
	tick(t, loaded)
	after := p.Process.Thickness
	cmd(loaded, "fault", p.ID, "", nil)
	if loaded.Recoveries[r.ExecutionID].Status != "failed" || p.Process.Thickness != after || math.Abs(p.Process.RootRate-root) > 1e-12 {
		t.Fatal("refault lost baseline")
	}
	if p.Process.Thickness <= before {
		t.Fatal("no cumulative growth")
	}
}
func TestRecoveryCancellationAndBinding(t *testing.T) {
	e, p, r := recoveryFixture(t)
	if x := cmd(e, "wafer-supplement", p.ID, "", map[string]any{}); x.Status != "rejected" {
		t.Fatal("unsigned accepted")
	}
	bad := *r
	bad.Baseline[0]++
	if recoverySend(e, "wafer-supplement", &bad).Status != "rejected" {
		t.Fatal("wrong baseline")
	}
	bad = *r
	bad.PMID = "PM4"
	if recoverySend(e, "wafer-supplement", &bad).Status != "rejected" {
		t.Fatal("wrong pm")
	}
	bad = *r
	bad.StepSeconds = [4]int{}
	if recoverySend(e, "wafer-supplement", &bad).Status != "rejected" {
		t.Fatal("empty steps")
	}
	bad.StepSeconds = [4]int{3600, 1, 0, 0}
	if recoverySend(e, "wafer-supplement", &bad).Status != "rejected" {
		t.Fatal("too long")
	}
	recoverySend(e, "cancel-supplement", r)
	recoverySend(e, "wafer-supplement", r)
	cmd(e, "clear", p.ID, "", nil)
	tick(t, e)
	if e.Recoveries[r.ExecutionID].Status != "cancelled" || e.wafer(r.WaferID).Attempts != 1 {
		t.Fatal("late command escaped cancellation fence")
	}
	r.ExecutionID = "execution-2"
	recoverySend(e, "wafer-supplement", r)
	tick(t, e)
	if recoverySend(e, "cancel-supplement", r).Status != "rejected" {
		t.Fatal("started cancelled")
	}
}
func TestRecoveryDurationAndGrowth(t *testing.T) {
	growth := []float64{}
	for _, seconds := range []int{1, 10, 3600} {
		e, p, r := recoveryFixture(t)
		r.StepSeconds = [4]int{0, 0, seconds, 0}
		r.Target = 10000
		recoverySend(e, "wafer-supplement", r)
		cmd(e, "clear", p.ID, "", nil)
		for i := 0; i < seconds; i++ {
			tick(t, e)
		}
		if p.Process.Elapsed != seconds || p.Remaining != 0 {
			t.Fatal("duration")
		}
		growth = append(growth, p.Process.DeltaThickness)
	}
	if !(growth[0] < growth[1] && growth[1] < growth[2]) {
		t.Fatal("growth normalized to target", growth)
	}
}
