package main

import (
	"bytes"
	"context"
	"cvd/protocol/gemlink"
	m "cvd/protocol/metrology"
	"encoding/json"
	"github.com/arloliu/go-secs/v2/hsms"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestHSMSMeasurementRoundTrip(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := l.Addr().String()
	l.Close()
	key := bytes.Repeat([]byte{1}, 32)
	a, e := load(filepath.Join(t.TempDir(), "state.json"), key)
	if e != nil {
		t.Fatal(e)
	}
	device, e := gemlink.New(addr, false)
	if e != nil {
		t.Fatal(e)
	}
	gemlink.NewDevice(m.Station, a, device)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e = device.Open(ctx, hsms.OpenBackground); e != nil {
		t.Fatal(e)
	}
	defer device.Close()
	client, e := gemlink.New(addr, true)
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	if e = client.Open(ctx, hsms.OpenWaitSelected); e != nil {
		t.Fatal(e)
	}
	h := gemlink.Host{Conn: client, ID: m.Station}
	if e = h.Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	if e = h.Control(ctx, command(t, "wire-wafer", key)); e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	a.tick(now)
	a.tick(now.Add(4 * time.Second))
	_, raw, e := h.Read(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var events []Event
	if e = json.Unmarshal(raw, &events); e != nil || len(events) != 3 {
		t.Fatal(string(raw), e)
	}
	if e = h.Ack(ctx, events[2].Sequence); e != nil {
		t.Fatal(e)
	}
	_, raw, e = h.Read(ctx)
	if e != nil || string(raw) != "[]" {
		t.Fatal(string(raw), e)
	}
}
