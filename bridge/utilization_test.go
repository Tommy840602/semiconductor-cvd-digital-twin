package main

import (
	"cvd/protocol/contract"
	sp "cvd/protocol/sparkplug"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestUtilizationEventBytesToKafkaOutbox(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "outbox"), 0700)
	b := &Bridge{nodes: map[string]*Node{}, dir: dir}
	id := "TEL-CVDTIN-08A-01"
	now := time.Now()
	send := func(kind string, seq uint64, ms ...*sp.Payload_Metric) {
		raw, err := sp.Encode(seq, ms, now)
		if err != nil {
			t.Fatal(err)
		}
		if err = b.receive(contract.Topic(kind, id), raw); err != nil {
			t.Fatal(err)
		}
	}
	send("NBIRTH", 0, sp.Metric("bdSeq", uint64(1), now))
	send("DBIRTH", 1, sp.Metric("CVD/Event", []byte{}, now))
	raw := []byte(`{"equipmentId":"TEL-CVDTIN-08A-01","eventId":"util-1","kind":"telemetry","sourceEpoch":"epoch","sequence":17,"sourceTime":"2026-09-19T00:00:01Z","payload":{"utilization":{"version":1,"intervalStart":"2026-09-19T00:00:00Z","intervalEnd":"2026-09-19T00:00:01Z","pm":{"PM1":true,"PM2":false,"PM3":true,"PM4":false}}}}`)
	send("DDATA", 2, sp.Metric("CVD/Event", raw, now))
	out, err := os.ReadFile(filepath.Join(dir, "outbox", "util-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var delivery Delivery
	json.Unmarshal(out, &delivery)
	var before, after map[string]any
	json.Unmarshal(raw, &before)
	json.Unmarshal(delivery.Body, &after)
	if delivery.Topic != "cvd.telemetry.v1" || delivery.Equipment != id {
		t.Fatal(delivery)
	}
	for _, key := range []string{"payload", "sourceTime", "sourceEpoch", "sequence"} {
		if !reflect.DeepEqual(before[key], after[key]) {
			t.Fatal("lost", key)
		}
	}
}
