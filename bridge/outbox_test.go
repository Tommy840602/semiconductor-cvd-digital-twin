package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOutboxRestartOrderAndReplacement(t *testing.T) {
	b := &Bridge{dir: t.TempDir()}
	if err := os.Mkdir(filepath.Join(b.dir, "outbox"), 0700); err != nil {
		t.Fatal(err)
	}
	save := func(id, equipment, at string, version int) {
		body, _ := json.Marshal(map[string]any{"sourceTime": at, "version": version})
		if err := b.save(Delivery{ID: id, Equipment: equipment, Topic: "snapshot", Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	save("newer", "01", "2026-10-02T02:00:00Z", 2)
	save("older", "10", "2026-10-02T01:00:00Z", 1)
	batch, err := b.deliveryBatch(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch) != 1 || batch[0].d.ID != "older" {
		t.Fatal("old equipment data starved", batch)
	}
	for _, q := range b.outboxIndex {
		if q.d.Body != nil {
			t.Fatal("cache retained payload")
		}
	}
	// Replacement after send must survive acknowledgement of the old inode.
	save("older", "10", "2026-10-02T03:00:00Z", 3)
	if err = b.removeDelivered(batch[0]); err != nil {
		t.Fatal(err)
	}
	restarted := &Bridge{dir: b.dir}
	batch, err = restarted.deliveryBatch(250)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch) != 2 || batch[0].d.ID != "newer" || batch[1].d.ID != "older" {
		t.Fatal("restart lost replacement or ordering", batch)
	}
	var payload map[string]any
	json.Unmarshal(batch[1].d.Body, &payload)
	if payload["version"] != float64(3) {
		t.Fatal("stale replacement payload")
	}
	for _, q := range batch {
		if err = restarted.removeDelivered(q); err != nil {
			t.Fatal(err)
		}
	}
	batch, err = restarted.deliveryBatch(250)
	if err != nil || len(batch) != 0 || restarted.pending.Load() != 0 {
		t.Fatal("acknowledged records retained", err)
	}
}

func TestOutboxRetryAndCorruption(t *testing.T) {
	b := &Bridge{dir: t.TempDir()}
	os.Mkdir(filepath.Join(b.dir, "outbox"), 0700)
	d := Delivery{ID: "event", Equipment: "01", Topic: "events", Ack: true, Body: json.RawMessage(`{"sequence":1}`)}
	if err := b.save(d); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		batch, err := b.deliveryBatch(250)
		if err != nil || len(batch) != 1 {
			t.Fatal("unacknowledged retry lost", err)
		}
	}
	os.WriteFile(filepath.Join(b.dir, "outbox", "bad.json"), []byte("invalid"), 0600)
	if _, err := b.deliveryBatch(250); err == nil {
		t.Fatal("corruption silently dropped")
	}
}
