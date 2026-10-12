package main

import (
	"context"
	"cvd/protocol/contract"
	"cvd/protocol/gemlink"
	sp "cvd/protocol/sparkplug"
	"encoding/json"
	"fmt"
	"github.com/arloliu/go-secs/v2/hsms"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"google.golang.org/protobuf/proto"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func secret(path string) string {
	b, e := os.ReadFile(path)
	if e != nil {
		log.Fatal(e)
	}
	return string(b)
}
func runSECS() {
	id := equipment()
	if !contract.ValidDevice(id) {
		log.Fatal("invalid equipment")
	}
	dir := env("DATA_DIR", ".runtime")
	os.MkdirAll(dir, 0700)
	key := []byte(secret(env("CVD_CONTROL_KEY_FILE", "/run/secrets/control_key")))
	spool := Spool{Pending: map[string]json.RawMessage{}, Done: map[string]bool{}}
	if b, e := os.ReadFile(dir + "/spool.json"); e == nil {
		if json.Unmarshal(b, &spool) != nil {
			log.Fatal("invalid spool")
		}
	}
	var pending atomic.Int64
	save := func() {
		previous := pending.Swap(int64(len(spool.Pending)))
		if len(spool.Pending) > 5000 && previous <= 5000 {
			log.Print("ALARM gateway backlog above 5000 records; durable spool retained")
		}
		b, _ := json.Marshal(spool)
		f, e := os.OpenFile(dir+"/spool.tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if e != nil {
			log.Fatal(e)
		}
		if _, e = f.Write(b); e == nil {
			e = f.Sync()
		}
		f.Close()
		if e != nil {
			log.Fatal(e)
		}
		if e = os.Rename(dir+"/spool.tmp", dir+"/spool.json"); e != nil {
			log.Fatal(e)
		}
	}
	var bd uint64
	if b, e := os.ReadFile(dir + "/bdseq.json"); e == nil {
		json.Unmarshal(b, &bd)
	}
	bd = (bd + 1) % 256
	b, _ := json.Marshal(bd)
	if e := os.WriteFile(dir+"/bdseq.json", b, 0600); e != nil {
		log.Fatal(e)
	}
	commands := make(chan []byte, 128)
	acks := make(chan []string, 128)
	rebirth := make(chan bool, 1)
	var healthMu sync.Mutex
	ready := false
	death, _ := proto.Marshal(&sp.Payload{Timestamp: proto.Uint64(uint64(time.Now().UnixMilli())), Metrics: []*sp.Payload_Metric{sp.Metric("bdSeq", bd, time.Now())}})
	opts := mqtt.NewClientOptions().AddBroker(env("MQTT_URL", "tcp://mqtt:1883")).SetClientID(contract.Node(id)).SetCleanSession(true).SetAutoReconnect(false).SetUsername(id).SetPassword(secret(env("MQTT_PASSWORD_FILE", "/run/secrets/mqtt_password"))).SetBinaryWill(contract.Topic("NDEATH", id), death, 1, false)
	opts.OnConnectionLost = func(_ mqtt.Client, e error) {
		healthMu.Lock()
		ready = false
		healthMu.Unlock()
		log.Printf("MQTT connection lost: %v", e)
	}
	opts.OnConnect = func(c mqtt.Client) {
		handler := func(_ mqtt.Client, m mqtt.Message) {
			kind, equipment, e := contract.ParseTopic(m.Topic())
			if e != nil || equipment != id {
				return
			}
			p, e := sp.Decode(m.Payload())
			if e != nil {
				return
			}
			for _, metric := range p.Metrics {
				switch metric.GetName() {
				case "Node Control/Rebirth":
					if kind == "NCMD" && metric.GetBooleanValue() {
						select {
						case rebirth <- true:
						default:
						}
					}
				case "CVD/Control":
					if kind == "DCMD" {
						raw := metric.GetBytesValue()
						if _, e = contract.Verify(raw, key, id, time.Now()); e != nil {
							log.Print(e)
							continue
						}
						select {
						case commands <- raw:
						default:
							log.Print("command queue full; awaiting retry")
						}
					}
				case "CVD/Ack":
					var ids []string
					if kind == "DCMD" && json.Unmarshal(metric.GetBytesValue(), &ids) == nil {
						select {
						case acks <- ids:
						default:
						}
					}
				}
			}
		}
		for _, kind := range []string{"NCMD", "DCMD"} {
			t := c.Subscribe(contract.Topic(kind, id), 1, handler)
			if !t.WaitTimeout(5*time.Second) || t.Error() != nil {
				log.Fatal("command subscription failed")
			}
		}
		t := c.Subscribe("spBv1.0/STATE/"+contract.Host, 1, func(_ mqtt.Client, m mqtt.Message) {
			var state struct {
				Online bool `json:"online"`
			}
			if json.Unmarshal(m.Payload(), &state) == nil && state.Online {
				select {
				case rebirth <- true:
				default:
				}
			}
		})
		t.WaitTimeout(5 * time.Second)
	}
	mq := mqtt.NewClient(opts)
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"equipmentId": id, "pendingRecords": pending.Load()})
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		healthMu.Lock()
		ok := ready
		healthMu.Unlock()
		if !ok || pending.Load() > 5000 {
			w.WriteHeader(503)
		}
		fmt.Fprint(w, "SECS/GEM Sparkplug gateway")
	})
	go http.ListenAndServe(":8082", nil)
	// Reconnect through a process restart so every MQTT session gets a new persisted bdSeq and matching Will.
	for {
		t := mq.Connect()
		if t.WaitTimeout(10*time.Second) && t.Error() == nil {
			break
		}
		time.Sleep(time.Second)
	}
	var seq uint64
	publish := func(kind string, ms []*sp.Payload_Metric) error {
		at := time.Now()
		raw, e := sp.Encode(seq, ms, at)
		if e != nil {
			return e
		}
		seq = (seq + 1) % 256
		t := mq.Publish(contract.Topic(kind, id), 0, false, raw)
		if !t.WaitTimeout(5 * time.Second) {
			return fmt.Errorf("MQTT publish timeout")
		}
		return t.Error()
	}
	definitions := sp.Registry{}
	birth := func(state []byte) error {
		seq = 0
		now := time.Now()
		if e := publish("NBIRTH", []*sp.Payload_Metric{sp.Metric("bdSeq", bd, now), sp.Metric("Node Control/Rebirth", false, now)}); e != nil {
			return e
		}
		ms := definitions.Birth(sp.SnapshotMetrics(state, now), now)
		ms = append(ms, sp.Metric("CVD/Event", []byte{}, now), sp.Metric("CVD/Control", []byte{}, now), sp.Metric("CVD/Ack", []byte{}, now), sp.Metric("CVD/Accepted", []byte{}, now))
		return publish("DBIRTH", ms)
	}
	for {
		c, e := gemlink.New(env("HSMS_ENDPOINT", "simulator:5000"), true)
		if e != nil {
			log.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		e = c.Open(ctx, hsms.OpenWaitSelected)
		h := &gemlink.Host{Conn: c, ID: id}
		if e == nil {
			e = h.Initialize(ctx)
		}
		cancel()
		if e != nil {
			c.Close()
			log.Print(e)
			time.Sleep(time.Second)
			continue
		}
		snapshot, _, e := h.Read(context.Background())
		if e == nil {
			e = birth(snapshot)
		}
		if e != nil {
			c.Close()
			time.Sleep(time.Second)
			continue
		}
		healthMu.Lock()
		ready = true
		healthMu.Unlock()
		tick := time.NewTicker(time.Second)
		sent := map[string]time.Time{}
		known := map[string]bool{}
		for _, m := range sp.SnapshotMetrics(snapshot, time.Now()) {
			known[m.GetName()] = true
		}
		session := func() error {
			for {
				select {
				case <-rebirth:
					if e := birth(snapshot); e != nil {
						return e
					}
				case raw := <-commands:
					if e := h.Control(context.Background(), raw); e != nil {
						log.Print("control outcome requires confirmation: ", e)
					} else {
						var wrapper contract.SignedCommand
						json.Unmarshal(raw, &wrapper)
						var cmd struct {
							ID string `json:"commandId"`
						}
						json.Unmarshal(wrapper.Payload, &cmd)
						accepted, _ := json.Marshal(map[string]any{"equipmentId": id, "commandId": cmd.ID, "sourceTime": time.Now().UTC().Format(time.RFC3339Nano)})
						if e = publish("DDATA", []*sp.Payload_Metric{sp.Metric("CVD/Accepted", accepted, time.Now())}); e != nil {
							return e
						}
					}
				case ids := <-acks:
					for _, eid := range ids {
						if _, ok := spool.Pending[eid]; ok {
							spool.Done[eid] = true
							delete(spool.Pending, eid)
						}
					}
					save()
				case <-tick.C:
					if !mq.IsConnected() {
						return fmt.Errorf("MQTT disconnected")
					}
					var events []byte
					var e error
					snapshot, events, e = h.Read(context.Background())
					if e != nil {
						return e
					}
					var source []json.RawMessage
					if json.Unmarshal(events, &source) != nil {
						return fmt.Errorf("invalid events")
					}
					for _, raw := range source {
						var x Item
						if json.Unmarshal(raw, &x) == nil && (x.EquipmentID == id || (x.EquipmentID == "CVD-01" && id == "TEL-CVDTIN-08A-01")) && x.Sequence > spool.Acked && !spool.Done[x.EventID] {
							spool.Pending[x.EventID] = raw
						}
					}
					save()
					ms := sp.SnapshotMetrics(snapshot, time.Now())
					newMetric := false
					for _, m := range ms {
						if !known[m.GetName()] {
							newMetric = true
							known[m.GetName()] = true
						}
					}
					if newMetric {
						if e = birth(snapshot); e != nil {
							return e
						}
					} else if e = publish("DDATA", ms); e != nil {
						return e
					}
					list := []Item{}
					for _, raw := range spool.Pending {
						var x Item
						if json.Unmarshal(raw, &x) == nil {
							list = append(list, x)
						}
					}
					sort.Slice(list, func(i, j int) bool { return list[i].Sequence < list[j].Sequence })
					budget := 4 // Ten gateways share the broker; leave capacity for live snapshots and acknowledgements.
					for _, x := range list {
						if budget == 0 {
							break
						}
						if time.Since(sent[x.EventID]) < 30*time.Second {
							continue
						}
						raw := spool.Pending[x.EventID]
						var event struct {
							Time time.Time `json:"sourceTime"`
						}
						json.Unmarshal(raw, &event)
						m := sp.Metric("CVD/Event", []byte(raw), event.Time)
						m.IsHistorical = proto.Bool(time.Since(event.Time) > 3*time.Second)
						if e = publish("DDATA", []*sp.Payload_Metric{m}); e != nil {
							return e
						}
						sent[x.EventID] = time.Now()
						budget--
					}
					max := spool.Acked
					for _, raw := range source {
						var x Item
						json.Unmarshal(raw, &x)
						if x.Sequence > spool.Acked && !spool.Done[x.EventID] {
							break
						}
						if x.Sequence > max {
							max = x.Sequence
						}
					}
					if max > spool.Acked {
						if e = h.Ack(context.Background(), uint64(max)); e != nil {
							return e
						}
						spool.Acked = max
						for _, raw := range source {
							var x Item
							json.Unmarshal(raw, &x)
							if x.Sequence <= max {
								delete(spool.Done, x.EventID)
								delete(sent, x.EventID)
							}
						}
						save()
					}
				}
			}
		}
		e = session()
		tick.Stop()
		c.Close()
		healthMu.Lock()
		ready = false
		healthMu.Unlock()
		if mq.IsConnected() {
			_ = publish("DDEATH", nil)
		} else {
			log.Fatal(e)
		}
		log.Print(e)
		time.Sleep(time.Second)
	}
}
