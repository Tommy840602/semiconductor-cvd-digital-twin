package sparkplug

import (
	"encoding/hex"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
)

func TestIndependentProtobufFixture(t *testing.T) {
	// timestamp=1, metric name=x datatype=Double value=1.0, seq=0.
	raw, e := hex.DecodeString("0801120e0a0178200a69000000000000f03f1800")
	if e != nil {
		t.Fatal(e)
	}
	var p Payload
	if e = proto.Unmarshal(raw, &p); e != nil {
		t.Fatal(e)
	}
	if p.GetTimestamp() != 1 || len(p.Metrics) != 1 || p.Metrics[0].GetName() != "x" || p.Metrics[0].GetDatatype() != 10 || p.Metrics[0].GetDoubleValue() != 1 {
		t.Fatalf("bad fixture: %v", &p)
	}
}
func TestAliasLifetime(t *testing.T) {
	d := Dictionary{}
	p := &Payload{Metrics: []*Payload_Metric{{Name: proto.String("PM1/temperature"), Alias: proto.Uint64(1), Datatype: proto.Uint32(10)}}}
	if e := d.Birth(p); e != nil {
		t.Fatal(e)
	}
	data := &Payload{Metrics: []*Payload_Metric{{Alias: proto.Uint64(1)}}}
	if e := d.Expand(data); e != nil || data.Metrics[0].GetName() != "PM1/temperature" {
		t.Fatal(e)
	}
	if e := d.Expand(&Payload{Metrics: []*Payload_Metric{{Alias: proto.Uint64(2)}}}); e == nil {
		t.Fatal("unknown alias accepted")
	}
	if e := d.Birth(&Payload{Metrics: []*Payload_Metric{{Name: proto.String("PM2/temperature"), Alias: proto.Uint64(2), Datatype: proto.Uint32(10)}}}); e != nil {
		t.Fatal(e)
	}
	if e := d.Expand(&Payload{Metrics: []*Payload_Metric{{Alias: proto.Uint64(1)}}}); e == nil {
		t.Fatal("old alias leaked")
	}
}

func TestOptionalFieldsRemainInBirth(t *testing.T) {
	r := Registry{}
	at := time.Now()
	r.Birth([]*Payload_Metric{Metric("PM1/thickness", 2.0, at)}, at)
	idle := r.Birth([]*Payload_Metric{Metric("PM2/thickness", 3.0, at)}, at)
	d := Dictionary{}
	if e := d.Birth(&Payload{Metrics: idle}); e != nil {
		t.Fatal(e)
	}
	if len(idle) != 2 || idle[1].GetName() != "PM1/thickness" || !idle[1].GetIsNull() || idle[1].GetDatatype() != 10 {
		t.Fatal("optional metric disappeared or changed type")
	}
	active := r.Birth([]*Payload_Metric{Metric("PM1/thickness", 4.0, at)}, at)
	if active[0].GetIsNull() || active[0].GetDoubleValue() != 4 {
		t.Fatal("metric failed to recover")
	}
}
