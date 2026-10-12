package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestPhaseDurations(t *testing.T) {
	for _, total := range []int{5, 30, 3600} {
		d := phaseDurations(total)
		sum := 0
		for _, n := range d {
			if n < 1 {
				t.Fatal(d)
			}
			sum += n
		}
		if sum != total {
			t.Fatal(d)
		}
	}
	if phaseDurations(5) != [4]int{1, 1, 2, 1} {
		t.Fatal("short phases")
	}
}
func processEngine(t *testing.T) *Engine {
	e := NewEngine()
	start(e, 1)
	for e.Chambers[0].Process == nil {
		tick(t, e)
	}
	return e
}
func TestProcessNormalResults(t *testing.T) {
	e := NewEngine()
	e.ProcessSeconds = 5
	start(e, 1)
	for i := 0; i < 1500 && e.Lots[0].Phase != "complete"; i++ {
		tick(t, e)
		for _, p := range e.Chambers {
			if p.Process != nil {
				q := p.Process
				if q.Thickness < 0 || math.IsNaN(q.Thickness) || q.HeaterPower < 0 || q.HeaterPower > 100 {
					t.Fatal(q)
				}
				if q.Phase != "deposition" && q.Rate != 0 {
					t.Fatal("growth outside deposition")
				}
				for _, g := range p.GasChannels {
					if g.Actual < 0 || math.IsNaN(g.Actual) {
						t.Fatal(g)
					}
				}
			}
		}
	}
	results, phases, finalSamples := 0, 0, 0
	for _, event := range e.Events {
		if event.Kind == "process-result" {
			results++
			b, _ := json.Marshal(event.Payload)
			var p struct {
				Complete bool
				Result   ProcessTelemetry
			}
			json.Unmarshal(b, &p)
			if !p.Complete || p.Result.Quality != "pass" || p.Result.Thickness <= 0 {
				t.Fatal(string(b))
			}
		}
		if event.Kind == "process-phase-changed" {
			phases++
		}
		if event.Kind == "telemetry" {
			b, _ := json.Marshal(event.Payload)
			var p struct{ Chambers []Chamber }
			json.Unmarshal(b, &p)
			for _, c := range p.Chambers {
				if c.Process != nil && c.Process.RunProgress == 1 {
					finalSamples++
				}
			}
		}
	}
	if results != 25 || phases != 125 || finalSamples < 25 {
		t.Fatal(results, phases, finalSamples)
	}
}
func TestProcessCheckpointDeterminism(t *testing.T) {
	e := processEngine(t)
	path := filepath.Join(t.TempDir(), "s.json")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	before := r.Chambers[0].Process.Elapsed
	r.Tick(base.Add(time.Hour))
	if r.Chambers[0].Process.Elapsed != before {
		t.Fatal("paused advanced")
	}
	r.Paused = false
	for i := 0; i < 100; i++ {
		tick(t, e)
		tick(t, r)
		if !reflect.DeepEqual(e.Chambers, r.Chambers) {
			t.Fatal("restored curve differs")
		}
	}
}
func TestOldProcessRemainsLegacy(t *testing.T) {
	e := processEngine(t)
	e.ModelVersion = 2
	e.Chambers[0].Process = nil
	e.Chambers[0].ProcessRunID = ""
	path := filepath.Join(t.TempDir(), "old.json")
	e.Save(path)
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Chambers[0].Process != nil {
		t.Fatal("fabricated legacy data")
	}
	r.Paused = false
	remaining := r.Chambers[0].Remaining
	tick(t, r)
	if r.Chambers[0].Remaining != remaining-1 {
		t.Fatal("legacy restarted")
	}
}
func TestLLPressureCurve(t *testing.T) {
	e := NewEngine()
	ll := e.Locks[0]
	ll.Phase = "pumping"
	prev := 1014.0
	for n := 3; n >= 0; n-- {
		ll.Remaining = n
		e.vacuumTelemetry(ll)
		if ll.Pressure <= 0 || ll.Pressure >= prev {
			t.Fatal(ll)
		}
		prev = ll.Pressure
	}
	ll.Phase = "venting"
	prev = 0
	for n := 3; n >= 0; n-- {
		ll.Remaining = n
		e.vacuumTelemetry(ll)
		if ll.Pressure <= prev {
			t.Fatal(ll)
		}
		prev = ll.Pressure
	}
}
func TestProcessContractFixture(t *testing.T) {
	b, err := os.ReadFile("../contracts/process-event.json")
	if err != nil {
		t.Fatal(err)
	}
	var e Event
	if err = json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	b, _ = json.Marshal(e.Payload)
	var payload struct{ Chambers []Chamber }
	json.Unmarshal(b, &payload)
	if payload.Chambers[0].Process.ModelRevision != modelRevision || payload.Chambers[0].GasChannels["precursor"].Actual != 40 {
		t.Fatal(string(b))
	}
}

func TestLegacyReworkRejectedWithoutReset(t *testing.T) {
	e := processEngine(t)
	p := e.Chambers[0]
	for p.Process.Thickness == 0 {
		tick(t, e)
	}
	oldID := p.ProcessRunID
	id := p.WaferID
	cmd(e, "fault", p.ID, "L1", nil)
	cmd(e, "clear", p.ID, "L1", nil)
	thickness := p.Process.Thickness
	for i := 0; i < 5; i++ {
		tick(t, e)
	}
	if p.Process.Thickness != thickness {
		t.Fatal("fault progressed")
	}
	if r := cmd(e, "rework", p.ID, "L1", nil); r.Status != "rejected" {
		t.Fatal(r)
	}
	if p.ProcessRunID != oldID || p.Process.Thickness != thickness || e.wafer(id).Attempts != 1 {
		t.Fatal("legacy rework changed wafer")
	}
	count := 0
	for _, ev := range e.Events {
		if ev.Kind == "process-result" {
			count++
		}
	}
	if count != 1 {
		t.Fatal("previous result missing")
	}
}

func TestFaultTelemetryStopsActiveOutputs(t *testing.T) {
	e := processEngine(t)
	p := e.Chambers[0]
	for p.Process.Thickness == 0 {
		tick(t, e)
	}
	thickness := p.Process.Thickness
	cmd(e, "fault", p.ID, "L1", nil)
	tick(t, e)
	if p.Process.Thickness != thickness || p.Process.Rate != 0 || p.Process.HeaterPower != 0 {
		t.Fatal("fault active outputs or growth", p.Process)
	}
	for _, gas := range p.GasChannels {
		if gas.Actual != 0 || gas.Target != 0 {
			t.Fatal("fault gas remains active", gas)
		}
	}
}
