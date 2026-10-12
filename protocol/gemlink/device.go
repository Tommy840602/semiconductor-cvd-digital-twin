package gemlink

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/arloliu/go-secs/v2/hsms"
	"github.com/arloliu/go-secs/v2/secs2"
	"sync"
	"time"
)

// Adapter methods must durably commit mutations before returning.
type Adapter interface {
	Snapshot() []byte
	Execute([]byte) ([]byte, error)
	Ack(uint64) error
	SaveProgram(string, []byte) error
	Program(string) ([]byte, error)
}
type Device struct {
	mu                            sync.Mutex
	ID                            string
	Adapter                       Adapter
	conn                          hsms.Connection
	communicating, online, remote bool
	traceIDs                      []uint64
	traceCount, traceLimit        uint64
	reports                       map[uint64][]uint64
	links                         map[uint64][]uint64
	enabled                       map[uint64]bool
	programs                      map[string][]byte
	lastEvent                     string
	alarm                         [4]bool
}

func NewDevice(id string, a Adapter, c hsms.Connection) *Device {
	d := &Device{ID: id, Adapter: a, conn: c, remote: true, reports: map[uint64][]uint64{}, links: map[uint64][]uint64{}, enabled: map[uint64]bool{}, programs: map[string][]byte{}}
	c.AddDataMessageHandler(d.handle)
	c.SubscribeLifecycle(func(e hsms.LifecycleEvent) {
		if e.Current != hsms.SelectedState {
			d.mu.Lock()
			d.communicating = false
			d.online = false
			d.traceIDs = nil
			d.mu.Unlock()
		}
	})
	return d
}
func (d *Device) SetRemote(remote bool) { d.mu.Lock(); defer d.mu.Unlock(); d.remote = remote }
func (d *Device) Status() (bool, bool, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.communicating, d.online, d.remote
}
func children(i secs2.Item) ([]secs2.Item, error) {
	if i == nil {
		return nil, fmt.Errorf("missing item")
	}
	return i.ToList()
}
func uints(i secs2.Item) ([]uint64, error) {
	xs, e := children(i)
	if e != nil {
		return nil, e
	}
	out := make([]uint64, len(xs))
	for j, x := range xs {
		v, e := secs2.NewCursor(x).Uint()
		if e != nil {
			return nil, e
		}
		out[j] = v
	}
	return out, nil
}
func (d *Device) value(id uint64, raw []byte, s map[string]json.RawMessage) secs2.Item {
	switch id {
	case 1:
		return secs2.A(d.ID)
	case 2:
		return secs2.A("CVD-SIM")
	case 3:
		return secs2.A("1.0")
	case 4:
		if !d.online {
			return secs2.U1(1)
		}
		if d.remote {
			return secs2.U1(5)
		}
		return secs2.U1(4)
	case 1000:
		return secs2.B(raw)
	case 1001:
		if v, ok := s["events"]; ok {
			return secs2.B([]byte(v))
		}
		return secs2.B([]byte("[]"))
	}
	if id >= 101 && id <= 104 {
		var chambers []struct {
			Temperature float64 `json:"temperature"`
		}
		if json.Unmarshal(s["chambers"], &chambers) == nil && int(id-101) < len(chambers) {
			return secs2.F8(chambers[id-101].Temperature)
		}
	}
	return secs2.L()
}
func (d *Device) values(ids []uint64) secs2.Item {
	return d.valuesFrom(ids, d.Adapter.Snapshot())
}
func (d *Device) valuesFrom(ids []uint64, raw []byte) secs2.Item {
	var s map[string]json.RawMessage
	json.Unmarshal(raw, &s)
	out := []secs2.Item{}
	for _, id := range ids {
		out = append(out, d.value(id, raw, s))
	}
	return secs2.L(out...)
}
func (d *Device) handle(m *hsms.DataMessage, ep hsms.SECS2Endpoint) {
	item, err := m.Item()
	if err != nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	reply := func(i secs2.Item) { _ = ep.ReplyDataMessage(context.Background(), m, i) }
	header := m.HeaderBytes()
	reject := func() { _ = ep.SendDataMessageAsync(context.Background(), 9, 7, false, secs2.B(header[:])) }
	key := int(m.Stream())*256 + int(m.Function())
	switch key {
	case 1*256 + 1:
		reply(secs2.L(secs2.A("CVD-SIM"), secs2.A("1.0")))
		return
	case 1*256 + 13:
		if xs, e := children(item); e != nil || len(xs) != 0 {
			reject()
			return
		}
		d.communicating = true
		reply(secs2.L(secs2.B(0), secs2.L(secs2.A("CVD-SIM"), secs2.A("1.0"))))
		return
	}
	if !d.communicating {
		reject()
		return
	}
	switch key {
	case 1*256 + 15:
		d.online = false
		reply(secs2.B(0))
	case 1*256 + 17:
		d.online = true
		reply(secs2.B(0))
	case 1*256 + 3:
		ids, e := uints(item)
		if e != nil {
			reject()
			return
		}
		if len(ids) == 0 {
			ids = []uint64{1, 2, 3, 4, 101, 102, 103, 104}
		}
		reply(d.values(ids))
	case 2*256 + 23:
		xs, e := children(item)
		if e != nil || len(xs) != 5 {
			reject()
			return
		}
		period, e := xs[1].ToASCII()
		limit, e2 := secs2.NewCursor(xs[2]).Uint()
		group, e3 := secs2.NewCursor(xs[3]).Uint()
		ids, e4 := uints(xs[4])
		if e != nil || e2 != nil || e3 != nil || e4 != nil || period != "000001" || group != 1 {
			reject()
			return
		}
		d.traceIDs = ids
		d.traceCount = 0
		d.traceLimit = limit
		reply(secs2.B(0))
	case 2*256 + 33:
		xs, e := children(item)
		if e != nil || len(xs) != 2 {
			reject()
			return
		}
		rs, e := children(xs[1])
		if e != nil {
			reject()
			return
		}
		next := map[uint64][]uint64{}
		for _, r := range rs {
			fields, e := children(r)
			if e != nil || len(fields) != 2 {
				reject()
				return
			}
			id, e := secs2.NewCursor(fields[0]).Uint()
			ids, e2 := uints(fields[1])
			if e != nil || e2 != nil {
				reject()
				return
			}
			next[id] = ids
		}
		d.reports = next
		reply(secs2.B(0))
	case 2*256 + 35:
		xs, e := children(item)
		if e != nil || len(xs) != 2 {
			reject()
			return
		}
		ls, e := children(xs[1])
		if e != nil {
			reject()
			return
		}
		next := map[uint64][]uint64{}
		for _, l := range ls {
			f, e := children(l)
			if e != nil || len(f) != 2 {
				reject()
				return
			}
			id, e := secs2.NewCursor(f[0]).Uint()
			ids, e2 := uints(f[1])
			if e != nil || e2 != nil {
				reject()
				return
			}
			for _, rid := range ids {
				if _, ok := d.reports[rid]; !ok {
					reject()
					return
				}
			}
			next[id] = ids
		}
		d.links = next
		reply(secs2.B(0))
	case 2*256 + 37:
		xs, e := children(item)
		if e != nil || len(xs) != 2 {
			reject()
			return
		}
		on, e := secs2.NewCursor(xs[0]).Bool()
		ids, e2 := uints(xs[1])
		if e != nil || e2 != nil {
			reject()
			return
		}
		for _, id := range ids {
			d.enabled[id] = on
		}
		reply(secs2.B(0))
	case 2*256 + 41:
		xs, e := children(item)
		if e != nil || len(xs) != 2 {
			reply(secs2.L(secs2.B(3), secs2.L()))
			return
		}
		cmd, e := xs[0].ToASCII()
		params, e2 := children(xs[1])
		if e != nil || e2 != nil || len(params) != 1 {
			reply(secs2.L(secs2.B(3), secs2.L()))
			return
		}
		f, e := children(params[0])
		if e != nil || len(f) != 2 {
			reply(secs2.L(secs2.B(3), secs2.L()))
			return
		}
		parameter, e := f[0].ToASCII()
		if e != nil || (cmd == "CVD_ACK" && parameter != "SEQUENCE") || (cmd == "CVD_CONTROL" && parameter != "ENVELOPE") {
			reply(secs2.L(secs2.B(3), secs2.L()))
			return
		}
		if cmd == "CVD_ACK" {
			n, e := secs2.NewCursor(f[1]).Uint()
			if e != nil || d.Adapter.Ack(n) != nil {
				reply(secs2.L(secs2.B(3), secs2.L()))
				return
			}
			reply(secs2.L(secs2.B(0), secs2.L()))
			return
		}
		if !d.online || !d.remote {
			reply(secs2.L(secs2.B(2), secs2.L()))
			return
		}
		if cmd != "CVD_CONTROL" {
			reply(secs2.L(secs2.B(1), secs2.L()))
			return
		}
		b, e := f[1].ToBinary()
		if e != nil {
			reply(secs2.L(secs2.B(3), secs2.L()))
			return
		}
		out, e := d.Adapter.Execute(b)
		if e != nil {
			reply(secs2.L(secs2.B(3), secs2.L()))
			return
		}
		var r struct {
			Status string `json:"status"`
		}
		json.Unmarshal(out, &r)
		ack := byte(0)
		if r.Status == "rejected" {
			ack = 2
		}
		reply(secs2.L(secs2.B(ack), secs2.L()))
	case 7*256 + 3:
		if !d.online || !d.remote {
			reject()
			return
		}
		xs, e := children(item)
		if e != nil || len(xs) != 2 {
			reject()
			return
		}
		id, e := xs[0].ToASCII()
		b, e2 := xs[1].ToBinary()
		if e != nil || e2 != nil || id == "" {
			reject()
			return
		}
		if previous, ok := d.programs[id]; ok && string(previous) != string(b) {
			reject()
			return
		}
		if d.Adapter.SaveProgram(id, b) != nil {
			reject()
			return
		}
		reply(secs2.B(0))
	case 7*256 + 5:
		id, e := item.ToASCII()
		if e != nil {
			reject()
			return
		}
		b, e := d.Adapter.Program(id)
		if e != nil {
			b = nil
		}
		reply(secs2.L(secs2.A(id), secs2.B(b)))
	default:
		// Unsupported primary is aborted in its stream, never acknowledged as success.
		_ = ep.SendDataMessageAsync(context.Background(), 9, 5, false, secs2.B(header[:]))
	}
}

// Trace is called once per equipment sample; disconnected samples remain in the adapter's durable events.
func (d *Device) Trace(now time.Time) {
	d.mu.Lock()
	if !d.online || len(d.traceIDs) == 0 || d.traceLimit == 0 || d.traceCount >= d.traceLimit {
		d.mu.Unlock()
		return
	}
	d.traceCount++
	body := secs2.L(secs2.U4(1), secs2.U4(d.traceCount), secs2.A(now.UTC().Format("20060102150405")), d.values(d.traceIDs))
	d.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = d.conn.SendDataMessage(ctx, 6, 1, true, body)
	d.mu.Lock()
	raw := d.Adapter.Snapshot()
	var state map[string]json.RawMessage
	json.Unmarshal(raw, &state)
	events := state["events"]
	reports := []secs2.Item{}
	if d.enabled[1] && string(events) != d.lastEvent {
		for _, rid := range d.links[1] {
			reports = append(reports, secs2.L(secs2.U4(rid), d.valuesFrom(d.reports[rid], raw)))
		}
		d.lastEvent = string(events)
	}
	alarms := []secs2.Item{}
	var cs []struct {
		Fault bool `json:"fault"`
	}
	if json.Unmarshal(state["chambers"], &cs) == nil {
		for n, c := range cs {
			if n >= 4 {
				break
			}
			fault := c.Fault
			if fault != d.alarm[n] {
				code := byte(0)
				if fault {
					code = 128
				}
				alarms = append(alarms, secs2.L(secs2.B(code), secs2.U4(n+1), secs2.A(fmt.Sprintf("PM%d simulated fault", n+1))))
				d.alarm[n] = fault
			}
		}
	}
	d.mu.Unlock()
	if len(reports) > 0 {
		_, _ = d.conn.SendDataMessage(ctx, 6, 11, true, secs2.L(secs2.U4(1), secs2.U4(1), secs2.L(reports...)))
	}
	for _, alarm := range alarms {
		_, _ = d.conn.SendDataMessage(ctx, 5, 1, true, alarm)
	}
}
