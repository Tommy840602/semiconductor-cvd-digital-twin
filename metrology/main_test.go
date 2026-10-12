package main

import (
	"bytes"
	"cvd/protocol/contract"
	m "cvd/protocol/metrology"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func command(t *testing.T, id string, key []byte) []byte {
	t.Helper()
	s := m.Source{EquipmentID: "TEL-CVDTIN-08A-01", WaferID: id, LotID: "lot", Slot: 1, PM: "PM1", ProcessRunID: id + "/run", RecipeVersion: "r1", Thickness: 10, Samples: []float64{10, 10, 10, 10, 10}, Target: 10, Tolerance: 10, UniformityLimit: 5}
	b, _ := json.Marshal(map[string]any{"equipmentId": m.Station, "commandId": id, "expiresAt": time.Now().Add(time.Hour), "type": "measure", "parameters": map[string]any{"measurementId": id, "source": s}})
	raw, e := contract.Sign(b, key)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func TestRestartFIFOAndDedup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	key := bytes.Repeat([]byte{1}, 32)
	a, e := load(path, key)
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"a", "b"} {
		for i := 0; i < 2; i++ {
			if _, e = a.Execute(command(t, id, key)); e != nil {
				t.Fatal(e)
			}
		}
	}
	if len(a.s.Queue) != 2 {
		t.Fatal("duplicate")
	}
	a, e = load(path, key)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	a.tick(now)
	if a.s.Jobs["a"].State != "measuring" || a.s.Jobs["b"].State != "queued" {
		t.Fatal("FIFO")
	}
	a, _ = load(path, key)
	a.tick(now.Add(4 * time.Second))
	if a.s.Jobs["a"].Result == nil {
		t.Fatal("lost result")
	}
	seq := a.s.Sequence
	a, _ = load(path, key)
	a.Execute(command(t, "a", key))
	if a.s.Sequence != seq || len(a.s.Queue) != 1 {
		t.Fatal("re-executed completed")
	}
	if e = a.Ack(seq); e != nil {
		t.Fatal(e)
	}
	a, _ = load(path, key)
	if len(a.s.Events) != 0 || a.s.Jobs["a"].Result == nil {
		t.Fatal("ACK lost immutable result")
	}
}
