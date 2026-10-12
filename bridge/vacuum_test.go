package main

import (
	"cvd/protocol/contract"
	sp "cvd/protocol/sparkplug"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVacuumEvidencePreserved(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "outbox"), 0700)
	b := &Bridge{nodes: map[string]*Node{}, dir: dir}
	now := time.Now()
	id := "TEL-CVDTIN-08A-01"
	send := func(kind string, seq uint64, metrics ...*sp.Payload_Metric) {
		raw, err := sp.Encode(seq, metrics, now)
		if err != nil {
			t.Fatal(err)
		}
		if err = b.receive(contract.Topic(kind, id), raw); err != nil {
			t.Fatal(err)
		}
	}
	send("NBIRTH", 0, sp.Metric("bdSeq", uint64(1), now))
	send("DBIRTH", 1, sp.Metric("CVD/Event", []byte{}, now))
	body := []byte(`{"equipmentId":"TEL-CVDTIN-08A-01","eventId":"pump1","kind":"vacuum-result","sequence":1,"payload":{"pumpId":"LL1","pump":{"modelVersion":"synthetic-vacuum-v1","cycle":{"id":"c1","status":"failed","pressures":[1013,2,0.1]}}}}`)
	send("DDATA", 2, sp.Metric("CVD/Event", body, now))
	raw, err := os.ReadFile(filepath.Join(dir, "outbox", "pump1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var d Delivery
	json.Unmarshal(raw, &d)
	if d.Topic != "cvd.events.v1" {
		t.Fatal(d.Topic)
	}
	var event map[string]any
	json.Unmarshal(d.Body, &event)
	if event["payload"].(map[string]any)["pumpId"] != "LL1" {
		t.Fatal("payload lost")
	}
}
