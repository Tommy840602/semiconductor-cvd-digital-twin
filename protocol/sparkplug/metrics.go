package sparkplug

import (
	"encoding/json"
	"fmt"
	"google.golang.org/protobuf/proto"
	"sort"
	"strings"
	"time"
)

func Metric(name string, v any, at time.Time) *Payload_Metric {
	m := &Payload_Metric{Name: proto.String(name), Timestamp: proto.Uint64(uint64(at.UnixMilli()))}
	switch x := v.(type) {
	case bool:
		m.Datatype = proto.Uint32(11)
		m.Value = &Payload_Metric_BooleanValue{BooleanValue: x}
	case string:
		m.Datatype = proto.Uint32(12)
		m.Value = &Payload_Metric_StringValue{StringValue: x}
	case uint64:
		m.Datatype = proto.Uint32(8)
		m.Value = &Payload_Metric_LongValue{LongValue: x}
	case []byte:
		m.Datatype = proto.Uint32(17)
		m.Value = &Payload_Metric_BytesValue{BytesValue: x}
	case float64:
		m.Datatype = proto.Uint32(10)
		m.Value = &Payload_Metric_DoubleValue{DoubleValue: x}
	default:
		m.Datatype = proto.Uint32(12)
		m.IsNull = proto.Bool(true)
	}
	return m
}

// SnapshotMetrics exposes ordinary process values individually; the Bytes extension preserves detailed wafer topology.
func SnapshotMetrics(raw []byte, at time.Time) []*Payload_Metric {
	var state map[string]any
	if json.Unmarshal(raw, &state) != nil {
		return nil
	}
	delete(state, "events")
	delete(state, "commands")
	clean, _ := json.Marshal(state)
	ms := []*Payload_Metric{Metric("CVD/State", clean, at)}
	for _, k := range []string{"equipmentId", "version", "epoch", "paused", "sourceTime", "recipeVersion", "processSeconds", "dispositionHold", "holdGeneration", "dispatchPaused"} {
		ms = append(ms, Metric("Equipment/"+k, state[k], at))
	}
	for _, group := range []string{"chambers", "locks", "lots"} {
		items, _ := state[group].([]any)
		for _, item := range items {
			m, _ := item.(map[string]any)
			id, _ := m["id"].(string)
			if group == "lots" {
				continue
			}
			keys := []string{}
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				switch m[k].(type) {
				case bool, float64, string:
					ms = append(ms, Metric(id+"/"+k, m[k], at))
				}
			}
			if p, ok := m["processTelemetry"].(map[string]any); ok {
				keys = nil
				for k := range p {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					switch p[k].(type) {
					case bool, float64, string:
						ms = append(ms, Metric(id+"/Process/"+k, p[k], at))
					}
				}
			}
		}
	}
	for _, m := range ms {
		unit := ""
		if strings.HasSuffix(m.GetName(), "/temperature") {
			unit = "degC"
		}
		if strings.HasSuffix(m.GetName(), "/pressure") {
			unit = "mbar"
		}
		if unit != "" {
			m.Properties = &Payload_PropertySet{Keys: []string{"engUnit"}, Values: []*Payload_PropertyValue{{Type: proto.Uint32(12), Value: &Payload_PropertyValue_StringValue{StringValue: unit}}}}
		}
	}
	return ms
}
func Encode(seq uint64, ms []*Payload_Metric, at time.Time) ([]byte, error) {
	return proto.Marshal(&Payload{Timestamp: proto.Uint64(uint64(at.UnixMilli())), Seq: proto.Uint64(seq % 256), Metrics: ms})
}
func Decode(raw []byte) (*Payload, error) {
	if len(raw) > 16*1024*1024 {
		return nil, fmt.Errorf("payload too large")
	}
	p := &Payload{}
	e := proto.Unmarshal(raw, p)
	if e != nil {
		return nil, e
	}
	return p, nil
}

type Dictionary map[uint64]*Payload_Metric

func (d Dictionary) Birth(p *Payload) error {
	next := Dictionary{}
	names := map[string]bool{}
	for _, m := range p.Metrics {
		if m.GetName() == "" || m.Datatype == nil || names[m.GetName()] {
			return fmt.Errorf("invalid birth metric")
		}
		names[m.GetName()] = true
		if m.Alias != nil {
			if _, ok := next[m.GetAlias()]; ok {
				return fmt.Errorf("duplicate alias")
			}
			next[m.GetAlias()] = proto.Clone(m).(*Payload_Metric)
		}
	}
	clear(d)
	for a, m := range next {
		d[a] = m
	}
	return nil
}
func (d Dictionary) Expand(p *Payload) error {
	for _, m := range p.Metrics {
		if m.GetName() == "" {
			def, ok := d[m.GetAlias()]
			if m.Alias == nil || !ok {
				return fmt.Errorf("unknown alias")
			}
			m.Name = proto.String(def.GetName())
			if m.Datatype == nil {
				m.Datatype = proto.Uint32(def.GetDatatype())
			}
			if m.GetDatatype() != def.GetDatatype() {
				return fmt.Errorf("alias datatype changed")
			}
		}
	}
	return nil
}

// Registry keeps a stable schema when optional process fields disappear during idle.
type Registry map[string]*Payload_Metric

func (r Registry) Birth(current []*Payload_Metric, at time.Time) []*Payload_Metric {
	present := map[string]bool{}
	out := append([]*Payload_Metric(nil), current...)
	for _, m := range current {
		present[m.GetName()] = true
		r[m.GetName()] = proto.Clone(m).(*Payload_Metric)
	}
	names := []string{}
	for name := range r {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !present[name] {
			m := proto.Clone(r[name]).(*Payload_Metric)
			m.IsNull = proto.Bool(true)
			m.Value = nil
			m.Timestamp = proto.Uint64(uint64(at.UnixMilli()))
			out = append(out, m)
		}
	}
	return out
}
