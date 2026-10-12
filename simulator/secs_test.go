package main

import (
	"bytes"
	"cvd/protocol/contract"
	"encoding/json"
	"testing"
	"time"
)

func TestSECSAdapterIdentityDedupAndPrograms(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine()
	e.Paused = true
	e.EquipmentID = "TEL-CVDTIN-08A-01"
	a := &secsAdapter{e: e, dir: dir, key: bytes.Repeat([]byte{7}, 32)}
	c := Command{EquipmentID: e.EquipmentID, CommandID: "secs-1", Type: "fault", TargetID: "PM1", ExpiresAt: time.Now().Add(time.Minute)}
	b, _ := json.Marshal(c)
	signed, err := contract.Sign(b, a.key)
	if err != nil {
		t.Fatal(err)
	}
	first, err := a.Execute(signed)
	if err != nil {
		t.Fatal(err)
	}
	var accepted Result
	if json.Unmarshal(first, &accepted) != nil || accepted.Status != "succeeded" || !e.Chambers[0].Fault {
		t.Fatal("valid control did not execute")
	}
	count := len(e.Commands)
	second, err := a.Execute(signed)
	if err != nil || string(first) != string(second) || len(e.Commands) != count {
		t.Fatal("duplicate mutated equipment", err)
	}
	loaded, err := Load(dir + "/state.json")
	if err != nil || !loaded.Paused {
		t.Fatal("not durable", err)
	}
	c.EquipmentID = "TEL-CVDTIN-08A-02"
	c.CommandID = "wrong"
	b, _ = json.Marshal(c)
	signed, _ = contract.Sign(b, a.key)
	if _, err = a.Execute(signed); err == nil {
		t.Fatal("cross tool accepted")
	}
	id := e.EquipmentID + "-recipe-2"
	settings := map[string]any{"recipeVersion": id, "stepSeconds": map[string]int{"preheat": 1, "stabilize": 1, "deposition": 4, "purge": 1}}
	c.EquipmentID = e.EquipmentID
	c.Type = "configure"
	c.Parameters = settings
	b, _ = json.Marshal(c)
	program, _ := contract.Sign(b, a.key)
	if err = a.SaveProgram(id, program); err != nil {
		t.Fatal(err)
	}
	settings["stepSeconds"].(map[string]int)["deposition"] = 5
	b, _ = json.Marshal(c)
	modified, _ := contract.Sign(b, a.key)
	if err = a.SaveProgram(id, modified); err == nil {
		t.Fatal("mutable program")
	}
	if err = a.SaveProgram("../escape", program); err == nil {
		t.Fatal("path accepted")
	}
	if !e.Paused || e.ProcessSeconds != 30 {
		t.Fatal("program download changed running recipe")
	}
}

func TestSampleDelayIsRecordedWithoutInventedTelemetry(t *testing.T) {
	e := NewEngine()
	e.Paused = true
	a := &secsAdapter{e: e, dir: t.TempDir()}
	now := time.Now().UTC()
	if err := a.sample(now); err != nil {
		t.Fatal(err)
	}
	if err := a.sample(now.Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	samples, gaps := 0, 0
	for _, event := range e.Events {
		if event.Kind == "telemetry" {
			samples++
		}
		if event.Kind == "data-gap" {
			gaps++
			if event.Payload.(map[string]any)["missedSeconds"] != 2 {
				t.Fatal(event)
			}
		}
	}
	if samples != 2 || gaps != 1 || e.TickNumber != 0 {
		t.Fatal(samples, gaps, e.TickNumber)
	}
}

func TestVacuumSignedControlAuthorization(t *testing.T) {
	e := NewEngine()
	e.EquipmentID = "TEL-CVDTIN-08A-01"
	a := &secsAdapter{e: e, dir: t.TempDir(), key: bytes.Repeat([]byte{9}, 32)}
	for _, role := range []string{"Operator", "Engineer", "Leader"} {
		c := map[string]any{"equipmentId": e.EquipmentID, "commandId": role, "type": "maintenance-request", "expiresAt": time.Now().Add(time.Minute), "parameters": map[string]any{"selectedPumpIds": []string{"LL1"}}, "actor": map[string]string{"role": role}}
		raw, _ := json.Marshal(c)
		signed, _ := contract.Sign(raw, a.key)
		out, err := a.Execute(signed)
		if role == "Operator" {
			if err == nil {
				t.Fatal("Operator accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if role == "Engineer" {
			var r Result
			json.Unmarshal(out, &r)
			if r.Status != "succeeded" {
				t.Fatal(r)
			}
		}
	}
}
