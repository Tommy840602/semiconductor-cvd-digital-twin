package main

import (
	"context"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

type Item struct {
	EquipmentID string `json:"equipmentId"`
	EventID     string `json:"eventId"`
	Sequence    int64  `json:"sequence"`
	Kind        string `json:"kind"`
}
type Spool struct {
	Acked   int64                      `json:"acked"`
	Pending map[string]json.RawMessage `json:"pending"`
	Done    map[string]bool            `json:"done"`
}

func equipment() string { return env("EQUIPMENT_ID", "TEL-CVDTIN-08A-01") }
func main() {
	if env("CVD_PROTOCOL", "opcua") == "secs" {
		runSECS()
		return
	}
	ctx := context.Background()
	dir := env("DATA_DIR", ".runtime")
	os.MkdirAll(dir, 0700)
	s := Spool{Pending: map[string]json.RawMessage{}, Done: map[string]bool{}}
	if b, err := os.ReadFile(filepath.Join(dir, "spool.json")); err == nil {
		if err = json.Unmarshal(b, &s); err != nil {
			log.Fatal(err)
		}
	}
	var mu sync.Mutex
	save := func() {
		b, _ := json.Marshal(s)
		f, err := os.OpenFile(dir+"/spool.tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatal(err)
		}
		if _, err = f.Write(b); err != nil {
			log.Fatal(err)
		}
		if err = f.Sync(); err != nil {
			log.Fatal(err)
		}
		f.Close()
		if err = os.Rename(dir+"/spool.tmp", dir+"/spool.json"); err != nil {
			log.Fatal(err)
		}
	}
	commands := make(chan []byte, 128)
	opts := mqtt.NewClientOptions().AddBroker(env("MQTT_URL", "tcp://localhost:1883")).SetClientID("cvd-gateway-" + equipment()).SetAutoReconnect(true).SetCleanSession(false)
	opts.OnConnect = func(c mqtt.Client) {
		c.Subscribe("cvd/"+equipment()+"/commands", 1, func(_ mqtt.Client, m mqtt.Message) {
			b := append([]byte(nil), m.Payload()...)
			var command struct {
				EquipmentID string `json:"equipmentId"`
			}
			if json.Unmarshal(b, &command) != nil || command.EquipmentID != equipment() {
				log.Print("rejected mismatched command")
				return
			}
			select {
			case commands <- b:
			default:
				log.Print("command queue full; retry same ID")
			}
		})
		c.Subscribe("cvd/"+equipment()+"/acks", 1, func(_ mqtt.Client, m mqtt.Message) {
			mu.Lock()
			defer mu.Unlock()
			id := string(m.Payload())
			if _, exists := s.Pending[id]; exists {
				s.Done[id] = true
				delete(s.Pending, id)
			}
			// The ticker checkpoints ACK progress in batches.
			// A crash here only replays an already durable event.
		})
	}
	mq := mqtt.NewClient(opts)
	for {
		t := mq.Connect()
		t.Wait()
		if t.Error() == nil {
			break
		}
		log.Print(t.Error())
		time.Sleep(time.Second)
	}
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if !mq.IsConnected() {
			w.WriteHeader(503)
		}
		fmt.Fprint(w, "gateway")
	})
	go http.ListenAndServe(":8082", nil)
	for {
		if err := session(ctx, mq, commands, &mu, &s, save); err != nil {
			log.Print(err)
		}
		time.Sleep(time.Second)
	}
}
func session(ctx context.Context, mq mqtt.Client, commands chan []byte, mu *sync.Mutex, s *Spool, save func()) error {
	c, err := newClient(ctx, env("OPC_ENDPOINT", "opc.tcp://localhost:4840"))
	if err != nil {
		return err
	}
	if err = c.Connect(ctx); err != nil {
		return err
	}
	defer c.Close(ctx)
	ns, err := c.FindNamespace(ctx, "urn:cvd:model")
	if err != nil {
		return err
	}
	call := func(method string, arg any) (*ua.CallMethodResult, error) {
		return c.Call(ctx, &ua.CallMethodRequest{ObjectID: ua.NewNumericNodeID(0, 85), MethodID: ua.NewStringNodeID(ns, method), InputArguments: []*ua.Variant{ua.MustVariant(arg)}})
	}
	notify := make(chan *opcua.PublishNotificationData, 32)
	sub, err := c.Subscribe(ctx, &opcua.SubscriptionParameters{Interval: 250 * time.Millisecond}, notify)
	if err != nil {
		return err
	}
	defer sub.Cancel(ctx)
	res, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, opcua.NewMonitoredItemCreateRequestWithDefaults(ua.NewStringNodeID(ns, "Snapshot"), ua.AttributeIDValue, 1), opcua.NewMonitoredItemCreateRequestWithDefaults(ua.NewStringNodeID(ns, "Events"), ua.AttributeIDValue, 2))
	if err != nil {
		return err
	}
	for _, r := range res.Results {
		if r.StatusCode != ua.StatusOK {
			return r.StatusCode
		}
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var source []json.RawMessage
	inflight := map[string]mqtt.Token{}
	lastSent := map[string]time.Time{}
	for {
		select {
		case b := <-commands:
			r, err := call("Command", string(b))
			if err != nil {
				return err
			}
			if r.StatusCode != ua.StatusOK {
				log.Print(r.StatusCode)
			}
		case n := <-notify:
			if n.Error != nil {
				return n.Error
			}
			if change, ok := n.Value.(*ua.DataChangeNotification); ok {
				for _, v := range change.MonitoredItems {
					if v.Value.Status != ua.StatusOK {
						continue
					}
					raw, ok := v.Value.Value.Value().(string)
					if !ok {
						continue
					}
					if v.ClientHandle == 1 {
						var snapshot map[string]any
						if json.Unmarshal([]byte(raw), &snapshot) != nil {
							continue
						}
						if snapshot["equipmentId"] != equipment() {
							return fmt.Errorf("OPC equipment mismatch")
						}
						delete(snapshot, "events")
						delete(snapshot, "commands")
						b, _ := json.Marshal(snapshot)
						if mq.IsConnected() {
							mq.Publish("cvd/"+equipment()+"/snapshot", 1, false, b)
						}
					} else {
						if json.Unmarshal([]byte(raw), &source) != nil {
							continue
						}
						mu.Lock()
						for _, b := range source {
							var x Item
							if json.Unmarshal(b, &x) == nil && (x.EquipmentID == equipment() || (x.EquipmentID == "CVD-01" && equipment() == "TEL-CVDTIN-08A-01")) && x.EventID != "" && x.Sequence > s.Acked && !s.Done[x.EventID] {
								s.Pending[x.EventID] = b
							}
						}
						save()
						mu.Unlock()
					}
				}
			}
		case <-ticker.C:
			mu.Lock()
			type outgoing struct {
				raw  json.RawMessage
				item Item
			}
			list := make([]outgoing, 0, len(s.Pending))
			for _, b := range s.Pending {
				var item Item
				json.Unmarshal(b, &item)
				list = append(list, outgoing{b, item})
			}
			mu.Unlock()
			sort.Slice(list, func(i, j int) bool { return list[i].item.Sequence < list[j].item.Sequence })
			if mq.IsConnected() {
				for _, entry := range list {
					b, x := entry.raw, entry.item
					if time.Since(lastSent[x.EventID]) < 5*time.Second {
						continue
					}
					topic := "events"
					if x.Kind == "telemetry" {
						topic = "telemetry"
					}
					if x.Kind == "command-result" {
						topic = "command-results"
					}
					if token := inflight[x.EventID]; token == nil || token.WaitTimeout(0) {
						lastSent[x.EventID] = time.Now()
						inflight[x.EventID] = mq.Publish("cvd/"+equipment()+"/"+topic, 1, false, []byte(b))
					}
				}
			}
			max := int64(0)
			mu.Lock()
			for _, b := range source {
				var x Item
				json.Unmarshal(b, &x)
				if x.Sequence > s.Acked && !s.Done[x.EventID] {
					break
				}
				max = x.Sequence
			}
			mu.Unlock()
			if max > s.Acked {
				r, err := call("Ack", max)
				if err != nil {
					return err
				}
				if r.StatusCode != ua.StatusOK {
					return r.StatusCode
				}
				mu.Lock()
				s.Acked = max
				for _, b := range source {
					var x Item
					json.Unmarshal(b, &x)
					if x.Sequence <= max {
						delete(s.Done, x.EventID)
						delete(s.Pending, x.EventID)
						delete(inflight, x.EventID)
						delete(lastSent, x.EventID)
					}
				}
				save()
				mu.Unlock()
			}
		}
	}
}

func newClient(ctx context.Context, endpoint string) (*opcua.Client, error) {
	eps, err := opcua.GetEndpoints(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	ep, err := opcua.SelectEndpoint(eps, "None", ua.MessageSecurityModeNone)
	if err != nil {
		return nil, err
	}
	return opcua.NewClient(endpoint, opcua.SecurityPolicy("None"), opcua.SecurityMode(ua.MessageSecurityModeNone), opcua.AuthAnonymous(), opcua.SecurityFromEndpoint(ep, ua.UserTokenTypeAnonymous))
}
