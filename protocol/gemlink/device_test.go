package gemlink

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/arloliu/go-secs/v2/hsms"
	"io"
	"net"
	"testing"
	"time"
)

type fakeAdapter struct{ calls int }

func (f *fakeAdapter) Snapshot() []byte {
	return []byte(`{"equipmentId":"TEL-CVDTIN-08A-01","chambers":[{"temperature":400}]}`)
}
func (f *fakeAdapter) Execute([]byte) ([]byte, error) {
	f.calls++
	return []byte(`{"status":"succeeded"}`), nil
}
func (f *fakeAdapter) SaveProgram(string, []byte) error { return nil }
func (f *fakeAdapter) Program(string) ([]byte, error)   { return nil, nil }
func (f *fakeAdapter) Ack(uint64) error                 { return nil }

// Frames below are independently constructed from the HSMS 10-byte header and SECS wire format.
func frame(s, f, typ byte, id uint32, body []byte) []byte {
	b := make([]byte, 14+len(body))
	binary.BigEndian.PutUint32(b, uint32(10+len(body)))
	if typ != 0 {
		b[4] = 255
		b[5] = 255
	} else {
		b[5] = 1
	}
	b[6] = s
	b[7] = f
	b[9] = typ
	binary.BigEndian.PutUint32(b[10:], id)
	copy(b[14:], body)
	return b
}
func readFrame(t *testing.T, c net.Conn) []byte {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var prefix [4]byte
	if _, e := io.ReadFull(c, prefix[:]); e != nil {
		t.Fatal(e)
	}
	b := make([]byte, binary.BigEndian.Uint32(prefix[:]))
	if _, e := io.ReadFull(c, b); e != nil {
		t.Fatal(e)
	}
	return b
}
func TestIndependentWirePeer(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := l.Addr().String()
	l.Close()
	conn, e := New(addr, false)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	a := &fakeAdapter{}
	d := NewDevice("TEL-CVDTIN-08A-01", a, conn)
	if e = conn.Open(context.Background(), hsms.OpenBackground); e != nil {
		t.Fatal(e)
	}
	var c net.Conn
	for n := 0; n < 100; n++ {
		c, e = net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if e == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	selectFrame := frame(0, 0, 1, 7, nil)
	c.Write(selectFrame[:3])
	c.Write(selectFrame[3:])
	r := readFrame(t, c)
	if r[5] != 2 || binary.BigEndian.Uint32(r[6:]) != 7 {
		t.Fatalf("bad select %x", r)
	}
	// Coalesced frames exercise stream framing independently from our client library.
	c.Write(append(frame(0, 0, 5, 8, nil), frame(0x81, 13, 0, 9, []byte{1, 0})...))
	r = readFrame(t, c)
	if r[5] != 6 {
		t.Fatal("linktest")
	}
	r = readFrame(t, c)
	if r[3] != 14 {
		t.Fatalf("communication %x", r)
	}
	c.Write(frame(0x81, 3, 0, 10, []byte{1, 1, 0xb1, 4, 0, 0, 0, 1}))
	r = readFrame(t, c)
	expected := append([]byte{1, 1, 0x41, 17}, []byte("TEL-CVDTIN-08A-01")...)
	if !bytes.Equal(r[10:], expected) {
		t.Fatalf("identity wire %x != %x", r[10:], expected)
	}
	d.SetRemote(false)
	c.Write(frame(0x81, 17, 0, 11, nil))
	readFrame(t, c)
	c.Write(frame(0x81, 3, 0, 12, []byte{1, 1, 0xb1, 4, 0, 0, 0, 4}))
	r = readFrame(t, c)
	if !bytes.Equal(r[10:], []byte{1, 1, 0xa5, 1, 4}) {
		t.Fatalf("local state %x", r)
	}
	command := append([]byte{1, 2, 0x41, 11}, []byte("CVD_CONTROL")...)
	command = append(command, 1, 1, 1, 2, 0x41, 8)
	command = append(command, []byte("ENVELOPE")...)
	command = append(command, 0x21, 0)
	c.Write(frame(0x82, 41, 0, 14, command))
	r = readFrame(t, c)
	if !bytes.Equal(r[10:], []byte{1, 2, 0x21, 1, 2, 1, 0}) {
		t.Fatalf("Local command not rejected: %x", r)
	}
	c.Write(frame(0xe3, 1, 0, 15, nil))
	r = readFrame(t, c)
	if r[2] != 9 || r[3] != 5 {
		t.Fatalf("unsupported function accepted: %x", r)
	}
	if a.calls != 0 {
		t.Fatal("read changed equipment")
	}
	c.Write(frame(0, 0, 3, 16, nil))
	r = readFrame(t, c)
	if r[5] != 4 {
		t.Fatal("deselect response")
	}
	deadline := time.Now().Add(time.Second)
	for {
		comm, online, _ := d.Status()
		if !comm && !online {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("GEM state survived deselect")
		}
		time.Sleep(time.Millisecond)
	}
	c.Write(frame(0, 0, 9, 13, nil))
}
func TestHSMSFailureBounds(t *testing.T) {
	for _, name := range []string{"T7-unselected", "T8-partial", "oversized"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			l, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			addr := l.Addr().String()
			l.Close()
			server, e := New(addr, false)
			if e != nil {
				t.Fatal(e)
			}
			defer server.Close()
			NewDevice("TEL-CVDTIN-08A-01", &fakeAdapter{}, server)
			if e = server.Open(context.Background(), hsms.OpenBackground); e != nil {
				t.Fatal(e)
			}
			var c net.Conn
			for n := 0; n < 100; n++ {
				c, e = net.DialTimeout("tcp", addr, 100*time.Millisecond)
				if e == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if e != nil {
				t.Fatal(e)
			}
			defer c.Close()
			if name != "T7-unselected" {
				c.Write(frame(0, 0, 1, 1, nil))
				readFrame(t, c)
			}
			switch name {
			case "T8-partial":
				partial := frame(0x81, 3, 0, 2, []byte{1, 1, 0xb1, 4, 0, 0, 0, 1})
				c.Write(partial[:len(partial)-1])
			case "oversized":
				c.Write([]byte{1, 0, 0, 0})
			}
			c.SetReadDeadline(time.Now().Add(12 * time.Second))
			var buf [32]byte
			_, e = c.Read(buf[:])
			if e == nil {
				t.Fatal("invalid peer remained usable")
			}
			if ne, ok := e.(net.Error); ok && ne.Timeout() {
				t.Fatal("transport failed to enforce timer")
			}
		})
	}
}
func TestT3NoReply(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		c, e := l.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		var f [14]byte
		if _, e = io.ReadFull(c, f[:]); e != nil {
			return
		}
		sys := binary.BigEndian.Uint32(f[10:])
		c.Write(frame(0, 0, 2, sys, nil))
		<-done
	}()
	c, e := New(l.Addr().String(), true)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e = c.Open(ctx, hsms.OpenWaitSelected); e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	_, e = c.SendDataMessage(ctx, 1, 1, true, nil)
	if e == nil || time.Since(start) < 4*time.Second || time.Since(start) > 8*time.Second {
		t.Fatalf("T3 not enforced: %v %v", time.Since(start), e)
	}
}
