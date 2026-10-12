package gemlink

import (
	"context"
	"cvd/protocol/contract"
	"encoding/json"
	"fmt"
	"github.com/arloliu/go-secs/v2/hsms"
	"github.com/arloliu/go-secs/v2/secs2"
)

type Host struct {
	Conn hsms.Connection
	ID   string
}

func (h *Host) request(ctx context.Context, s, f byte, item secs2.Item) (secs2.Item, error) {
	r, e := h.Conn.SendDataMessage(ctx, s, f, true, item)
	if e != nil {
		return nil, e
	}
	return r.Item()
}
func (h *Host) Initialize(ctx context.Context) error {
	h.Conn.AddDataMessageHandler(func(m *hsms.DataMessage, ep hsms.SECS2Endpoint) {
		if (m.Stream() == 6 && (m.Function() == 1 || m.Function() == 11)) || (m.Stream() == 5 && m.Function() == 1) {
			_ = ep.ReplyDataMessage(context.Background(), m, secs2.B(0))
		}
	})
	r, e := h.request(ctx, 1, 13, secs2.L())
	if e != nil {
		return e
	}
	ack, e := secs2.NewCursor(r).At(0).Binary()
	if e != nil || len(ack) != 1 || ack[0] != 0 {
		return fmt.Errorf("communication rejected")
	}
	r, e = h.request(ctx, 1, 3, secs2.L(secs2.U4(1)))
	if e != nil {
		return e
	}
	id, e := secs2.NewCursor(r).At(0).ASCII()
	if e != nil || id != h.ID {
		return fmt.Errorf("equipment mismatch")
	}
	r, e = h.request(ctx, 1, 17, nil)
	if e != nil {
		return e
	}
	ack, e = r.ToBinary()
	if e != nil || len(ack) != 1 || ack[0] != 0 {
		return fmt.Errorf("online rejected")
	}
	for _, q := range []struct {
		f byte
		i secs2.Item
	}{
		{23, secs2.L(secs2.U4(1), secs2.A("000001"), secs2.U4(4294967295), secs2.U4(1), secs2.L(secs2.U4(101), secs2.U4(102), secs2.U4(103), secs2.U4(104)))},
		{33, secs2.L(secs2.U4(1), secs2.L(secs2.L(secs2.U4(1), secs2.L(secs2.U4(1001)))))},
		{35, secs2.L(secs2.U4(1), secs2.L(secs2.L(secs2.U4(1), secs2.L(secs2.U4(1)))))},
		{37, secs2.L(secs2.BOOLEAN(true), secs2.L(secs2.U4(1)))},
	} {
		r, e = h.request(ctx, 2, q.f, q.i)
		if e != nil {
			return e
		}
		a, e := r.ToBinary()
		if e != nil || len(a) != 1 || a[0] != 0 {
			return fmt.Errorf("report configuration rejected")
		}
	}
	return nil
}
func (h *Host) Read(ctx context.Context) (snapshot, events []byte, err error) {
	r, e := h.request(ctx, 1, 3, secs2.L(secs2.U4(1000), secs2.U4(1001)))
	if e != nil {
		return nil, nil, e
	}
	snapshot, e = secs2.NewCursor(r).At(0).Binary()
	if e != nil {
		return nil, nil, e
	}
	events, e = secs2.NewCursor(r).At(1).Binary()
	var s struct {
		ID string `json:"equipmentId"`
	}
	if json.Unmarshal(snapshot, &s) != nil || s.ID != h.ID {
		return nil, nil, fmt.Errorf("equipment mismatch")
	}
	return snapshot, events, e
}
func (h *Host) Control(ctx context.Context, signed []byte) error {
	var signedCommand contract.SignedCommand
	var command struct {
		Type       string         `json:"type"`
		Parameters map[string]any `json:"parameters"`
	}
	if json.Unmarshal(signed, &signedCommand) != nil || json.Unmarshal(signedCommand.Payload, &command) != nil {
		return fmt.Errorf("invalid control envelope")
	}
	if command.Type == "configure" {
		id, _ := command.Parameters["recipeVersion"].(string)
		body, e := json.Marshal(command.Parameters)
		if e != nil {
			return e
		}
		r, e := h.request(ctx, 7, 3, secs2.L(secs2.A(id), secs2.B(signed)))
		if e != nil {
			return e
		}
		ack, e := r.ToBinary()
		if e != nil || len(ack) != 1 || ack[0] != 0 {
			return fmt.Errorf("program download rejected")
		}
		r, e = h.request(ctx, 7, 5, secs2.A(id))
		if e != nil {
			return e
		}
		copy, e := secs2.NewCursor(r).At(1).Binary()
		if e != nil || string(copy) != string(body) {
			return fmt.Errorf("program readback mismatch")
		}
	}
	return h.command(ctx, "CVD_CONTROL", "ENVELOPE", secs2.B(signed))
}
func (h *Host) Ack(ctx context.Context, n uint64) error {
	return h.command(ctx, "CVD_ACK", "SEQUENCE", secs2.U8(n))
}
func (h *Host) command(ctx context.Context, name, param string, value secs2.Item) error {
	r, e := h.request(ctx, 2, 41, secs2.L(secs2.A(name), secs2.L(secs2.L(secs2.A(param), value))))
	if e != nil {
		return e
	}
	ack, e := secs2.NewCursor(r).At(0).Binary()
	if e != nil || len(ack) != 1 || ack[0] != 0 {
		return fmt.Errorf("remote command rejected")
	}
	return nil
}
