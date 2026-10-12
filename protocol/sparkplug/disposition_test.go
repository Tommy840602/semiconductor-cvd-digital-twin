package sparkplug

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDispositionMetricsRoundTrip(t *testing.T) {
	raw := []byte(`{"equipmentId":"TEL-CVDTIN-08A-01","dispositionHold":true,"holdGeneration":7,"dispatchPaused":true,"incidents":{"i":{"status":"awaiting_result","resultVersion":2}},"releases":{"r":{"status":"invalidated"}}}`)
	at := time.Now()
	wire, err := Encode(1, SnapshotMetrics(raw, at), at)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Decode(wire)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, m := range p.Metrics {
		switch m.GetName() {
		case "Equipment/dispositionHold", "Equipment/dispatchPaused":
			if !m.GetBooleanValue() {
				t.Fatal("hold lost")
			}
			found[m.GetName()] = true
		case "Equipment/holdGeneration":
			if m.GetDoubleValue() != 7 {
				t.Fatal("generation lost")
			}
			found[m.GetName()] = true
		case "CVD/State":
			var s map[string]any
			if json.Unmarshal(m.GetBytesValue(), &s) != nil || s["incidents"] == nil || s["releases"] == nil {
				t.Fatal("evidence lost")
			}
			found[m.GetName()] = true
		}
	}
	if len(found) != 4 {
		t.Fatal(found)
	}
}
