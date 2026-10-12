package main

import (
	"cvd/protocol/contract"
	sp "cvd/protocol/sparkplug"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMetrologyTopics(t *testing.T) {
	for _, s := range []string{"events", "snapshot", "connectivity", "command-accepted"} {
		if deviceTopic("KLA-SIM-01", s) != "metrology."+s+".v1" {
			t.Fatal("KLA mixed with CVD")
		}
		if deviceTopic("TEL-CVDTIN-08A-01", s) != "cvd."+s+".v1" {
			t.Fatal("CVD changed")
		}
	}
}

func TestMetrologyMapToOutbox(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "outbox"), 0700)
	b := &Bridge{nodes: map[string]*Node{}, dir: dir}
	now := time.Now()
	id := "KLA-SIM-01"
	send := func(kind string, seq uint64, metrics ...*sp.Payload_Metric) {
		raw, e := sp.Encode(seq, metrics, now)
		if e != nil {
			t.Fatal(e)
		}
		if e = b.receive(contract.Topic(kind, id), raw); e != nil {
			t.Fatal(e)
		}
	}
	send("NBIRTH", 0, sp.Metric("bdSeq", uint64(1), now))
	send("DBIRTH", 1, sp.Metric("CVD/Event", []byte{}, now))
	defects := make([]map[string]any, 1000)
	for i := range defects {
		defects[i] = map[string]any{"defectId": fmt.Sprint(i), "x": 1, "y": 2, "sizeUm": .5, "type": "particle"}
	}
	payload := map[string]any{"measurementId": "m1", "state": "completed", "result": map[string]any{"defects": defects}}
	raw, _ := json.Marshal(map[string]any{"equipmentId": id, "eventId": "map-1", "kind": "measurement-state", "sequence": 1, "sourceTime": now, "payload": payload})
	send("DDATA", 2, sp.Metric("CVD/Event", raw, now))
	out, e := os.ReadFile(filepath.Join(dir, "outbox", "map-1.json"))
	if e != nil {
		t.Fatal(e)
	}
	var d Delivery
	json.Unmarshal(out, &d)
	if d.Topic != "metrology.events.v1" || !d.Ack {
		t.Fatal("routing")
	}
	var n map[string]any
	json.Unmarshal(d.Body, &n)
	if len(n["payload"].(map[string]any)["result"].(map[string]any)["defects"].([]any)) != 1000 {
		t.Fatal("truncated")
	}
}
