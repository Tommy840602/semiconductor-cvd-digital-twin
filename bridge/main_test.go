package main

import (
	"cvd/protocol/contract"
	sp "cvd/protocol/sparkplug"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBirthDeathAndIsolation(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "outbox"), 0700)
	b := &Bridge{nodes: map[string]*Node{}, dir: dir}
	id := "TEL-CVDTIN-08A-01"
	other := "TEL-CVDTIN-08A-02"
	send := func(kind, id string, seq uint64, ms ...*sp.Payload_Metric) error {
		raw, e := sp.Encode(seq, ms, time.Now())
		if e != nil {
			t.Fatal(e)
		}
		return b.receive(contract.Topic(kind, id), raw)
	}
	for _, id := range []string{id, other} {
		if e := send("NBIRTH", id, 0, sp.Metric("bdSeq", uint64(3), time.Now())); e != nil {
			t.Fatal(e)
		}
		if e := send("DBIRTH", id, 1, sp.Metric("PM1/temperature", 400.0, time.Now())); e != nil {
			t.Fatal(e)
		}
	}
	if e := send("DDATA", id, 3, sp.Metric("PM1/temperature", 401.0, time.Now())); e == nil {
		t.Fatal("sequence gap accepted")
	}
	if b.nodes[id].Device || !b.nodes[other].Device {
		t.Fatal("isolation failure")
	}
	if e := send("NDEATH", other, 0, sp.Metric("bdSeq", uint64(2), time.Now())); e != nil {
		t.Fatal(e)
	}
	if !b.nodes[other].Device {
		t.Fatal("old death invalidated current session")
	}
	if e := send("NDEATH", other, 0, sp.Metric("bdSeq", uint64(3), time.Now())); e != nil {
		t.Fatal(e)
	}
	if b.nodes[other].Device {
		t.Fatal("death ignored")
	}
	if e := send("NBIRTH", id, 0, sp.Metric("bdSeq", uint64(4), time.Now())); e != nil {
		t.Fatal(e)
	}
	m := sp.Metric("PM1/temperature", 400.0, time.Now())
	m.Alias = proto.Uint64(1)
	if e := send("DBIRTH", id, 1, m); e != nil {
		t.Fatal(e)
	}
	if e := send("DDATA", id, 2, &sp.Payload_Metric{Alias: proto.Uint64(2)}); e == nil {
		t.Fatal("unknown alias accepted")
	}
}

func TestDurableRetryKeepsFirstEvent(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "outbox"), 0700); err != nil {
		t.Fatal(err)
	}
	b := &Bridge{dir: dir}
	d := Delivery{ID: "original-event", Equipment: "TEL-CVDTIN-08A-01", Topic: "cvd.telemetry.v1", Body: []byte(`{"sourceTime":"2026-09-17T00:00:00Z"}`), Ack: true}
	if err := b.save(d); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "outbox", d.ID+".json")
	before, _ := os.Stat(p)
	raw, _ := os.ReadFile(p)
	restarted := &Bridge{dir: dir}
	time.Sleep(time.Millisecond)
	if err := restarted.save(d); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(p)
	again, _ := os.ReadFile(p)
	if !before.ModTime().Equal(after.ModTime()) || string(raw) != string(again) {
		t.Fatal("retry rewrote durable event")
	}
}
