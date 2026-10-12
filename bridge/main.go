package main

import (
	"context"
	"cvd/protocol/contract"
	sp "cvd/protocol/sparkplug"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/segmentio/kafka-go"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func env(k, d string) string {
	if s := os.Getenv(k); s != "" {
		return s
	}
	return d
}
func secret(p string) string {
	b, e := os.ReadFile(p)
	if e != nil {
		log.Fatal(e)
	}
	return string(b)
}

type Node struct {
	mu     sync.Mutex
	Birth  bool
	Device bool
	BD     uint64
	Seq    uint64
	Dict   sp.Dictionary
	Names  map[string]uint32
}
type Delivery struct {
	ID        string          `json:"id"`
	Equipment string          `json:"equipmentId"`
	Topic     string          `json:"topic"`
	Body      json.RawMessage `json:"body"`
	Ack       bool            `json:"ack"`
}
type Bridge struct {
	outboxMu         sync.Mutex
	outboxIndex      map[string]queuedDelivery
	pending          atomic.Int64
	lastWrite        atomic.Int64
	deliveryFailures atomic.Int64
	mu               sync.Mutex
	nodes            map[string]*Node
	mq               mqtt.Client
	dir              string
	writer           *kafka.Writer
}

func (b *Bridge) publish(kind, id string, ms []*sp.Payload_Metric) error {
	if b.mq == nil {
		return fmt.Errorf("MQTT unavailable")
	}
	raw, e := sp.Encode(0, ms, time.Now())
	if e != nil {
		return e
	}
	t := b.mq.Publish(contract.Topic(kind, id), 0, false, raw)
	if !t.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("MQTT timeout")
	}
	return t.Error()
}
func (b *Bridge) rebirth(id string) {
	_ = b.publish("NCMD", id, []*sp.Payload_Metric{sp.Metric("Node Control/Rebirth", true, time.Now())})
}
func (b *Bridge) save(d Delivery) error {
	b.outboxMu.Lock()
	defer b.outboxMu.Unlock()
	if strings.ContainsAny(d.ID, "/\\") || d.ID == "" {
		return fmt.Errorf("invalid ID")
	}
	p := filepath.Join(b.dir, "outbox", d.ID+".json")
	// Important event IDs are immutable. Keep their first durable copy while
	// Kafka is unavailable rather than rewriting every upstream retry.
	if d.Ack {
		if _, e := os.Stat(p); e == nil {
			return nil
		} else if !os.IsNotExist(e) {
			return e
		}
	}
	raw, e := json.Marshal(d)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(p+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(raw)
	if e == nil {
		e = f.Sync()
	}
	f.Close()
	if e != nil {
		return e
	}
	if e = os.Rename(p+".tmp", p); e != nil {
		return e
	}
	delete(b.outboxIndex, p)
	return nil
}
func (b *Bridge) quality(id, q string) {
	raw, _ := json.Marshal(map[string]any{"equipmentId": id, "quality": q, "sourceTime": time.Now().UTC().Format(time.RFC3339Nano)})
	if e := b.save(Delivery{ID: id + "-connectivity-" + fmt.Sprint(time.Now().UnixNano()), Equipment: id, Topic: deviceTopic(id, "connectivity"), Body: raw}); e != nil {
		log.Fatal(e)
	}
}
func (b *Bridge) receive(topic string, raw []byte) (err error) {
	kind, id, e := contract.ParseTopic(topic)
	if e != nil {
		return e
	}
	if kind == "NCMD" || kind == "DCMD" {
		return nil
	}
	p, e := sp.Decode(raw)
	if e != nil {
		return e
	}
	b.mu.Lock()
	n := b.nodes[id]
	if n == nil {
		n = &Node{Dict: sp.Dictionary{}, Names: map[string]uint32{}}
		b.nodes[id] = n
	}
	b.mu.Unlock()
	n.mu.Lock()
	defer n.mu.Unlock()
	defer func() {
		if err != nil {
			n.Device = false
			b.quality(id, "unknown")
		}
	}()
	if kind == "NBIRTH" {
		var bd *uint64
		for _, m := range p.Metrics {
			if m.GetName() == "bdSeq" {
				v := m.GetLongValue()
				bd = &v
			}
		}
		if bd == nil || p.GetSeq() != 0 {
			return fmt.Errorf("invalid node birth")
		}
		n.Birth = true
		n.Device = false
		n.BD = *bd
		n.Seq = 0
		n.Dict = sp.Dictionary{}
		n.Names = map[string]uint32{}
		b.quality(id, "unknown")
		return nil
	}
	if kind == "NDEATH" {
		for _, m := range p.Metrics {
			if m.GetName() == "bdSeq" && n.Birth && m.GetLongValue() == n.BD {
				n.Birth = false
				n.Device = false
				b.quality(id, "offline")
			}
		}
		return nil
	}
	if !n.Birth {
		go b.rebirth(id)
		return fmt.Errorf("node birth required")
	}
	if p.Seq == nil || p.GetSeq() != (n.Seq+1)%256 {
		n.Device = false
		b.quality(id, "unknown")
		go b.rebirth(id)
		return fmt.Errorf("sequence gap")
	}
	n.Seq = p.GetSeq()
	if kind == "DDEATH" {
		n.Device = false
		b.quality(id, "offline")
		return nil
	}
	if kind == "NDATA" {
		return nil
	}
	if kind == "DBIRTH" {
		if e = n.Dict.Birth(p); e != nil {
			return e
		}
		for _, m := range p.Metrics {
			n.Names[m.GetName()] = m.GetDatatype()
		}
		n.Device = true
		b.quality(id, "good")
	} else if !n.Device {
		go b.rebirth(id)
		return fmt.Errorf("device birth required")
	}
	if e = n.Dict.Expand(p); e != nil {
		n.Device = false
		go b.rebirth(id)
		return e
	}
	for _, m := range p.Metrics {
		if datatype, ok := n.Names[m.GetName()]; !ok || datatype != m.GetDatatype() {
			n.Device = false
			go b.rebirth(id)
			return fmt.Errorf("undefined metric %s datatype=%d", m.GetName(), m.GetDatatype())
		}
		if m.GetIsNull() {
			continue
		}
		switch m.GetName() {
		case "CVD/State":
			var state map[string]any
			if json.Unmarshal(m.GetBytesValue(), &state) != nil || state["equipmentId"] != id {
				return fmt.Errorf("snapshot mismatch")
			}
			if m.GetIsHistorical() {
				continue
			}
			state["quality"] = "good"
			state["protocol"] = "SECS/GEM + Sparkplug B"
			state["ingestTime"] = time.Now().UTC().Format(time.RFC3339Nano)
			body, _ := json.Marshal(state)
			version := fmt.Sprint(state["version"])
			epoch := fmt.Sprint(state["epoch"])
			if e = b.save(Delivery{ID: id + "-snapshot-" + epoch + "-" + version, Equipment: id, Topic: deviceTopic(id, "snapshot"), Body: body}); e != nil {
				return e
			}
		case "CVD/Accepted":
			if len(m.GetBytesValue()) == 0 {
				continue
			}
			var accepted struct {
				ID        string `json:"commandId"`
				Equipment string `json:"equipmentId"`
			}
			if json.Unmarshal(m.GetBytesValue(), &accepted) != nil || accepted.Equipment != id || accepted.ID == "" {
				return fmt.Errorf("acceptance mismatch")
			}
			if e = b.save(Delivery{ID: accepted.ID + "-accepted", Equipment: id, Topic: deviceTopic(id, "command-accepted"), Body: m.GetBytesValue()}); e != nil {
				return e
			}
		case "CVD/Event":
			if len(m.GetBytesValue()) == 0 {
				continue
			}
			var event map[string]any
			if json.Unmarshal(m.GetBytesValue(), &event) != nil {
				return fmt.Errorf("invalid event")
			}
			if event["equipmentId"] == "CVD-01" && id == "TEL-CVDTIN-08A-01" {
				event["equipmentId"] = id
			}
			if event["equipmentId"] != id {
				return fmt.Errorf("event mismatch")
			}
			eid, _ := event["eventId"].(string)
			topic := deviceTopic(id, "events")
			if event["kind"] == "telemetry" {
				topic = "cvd.telemetry.v1"
			}
			if event["kind"] == "command-result" {
				topic = "cvd.command-results.v1"
			}
			event["protocol"] = "SECS/GEM + Sparkplug B"
			event["ingestTime"] = time.Now().UTC().Format(time.RFC3339Nano)
			event["historical"] = m.GetIsHistorical()
			body, _ := json.Marshal(event)
			if e = b.save(Delivery{ID: eid, Equipment: id, Topic: topic, Body: body, Ack: true}); e != nil {
				return e
			}
		}
	}
	return nil
}
func (b *Bridge) deliver() {
	for range time.NewTicker(100 * time.Millisecond).C {
		list, e := b.deliveryBatch(250)
		if e != nil {
			log.Fatal(e)
		}
		if len(list) == 0 {
			continue
		}
		messages := make([]kafka.Message, len(list))
		for i, q := range list {
			messages[i] = kafka.Message{Topic: q.d.Topic, Key: []byte(q.d.Equipment), Value: q.d.Body}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = b.writer.WriteMessages(ctx, messages...)
		cancel()
		if e == nil {
			b.lastWrite.Store(time.Now().Unix())
		} else {
			b.deliveryFailures.Add(1)
		}
		if e != nil {
			continue
		}
		grouped := map[string][]string{}
		for _, q := range list {
			if q.d.Ack {
				grouped[q.d.Equipment] = append(grouped[q.d.Equipment], q.d.ID)
			}
		}
		confirmed := map[string]bool{}
		for id, ids := range grouped {
			raw, _ := json.Marshal(ids)
			confirmed[id] = b.publish("DCMD", id, []*sp.Payload_Metric{sp.Metric("CVD/Ack", raw, time.Now())}) == nil
		}
		for _, q := range list {
			if q.d.Ack && !confirmed[q.d.Equipment] {
				continue
			}
			if e = b.removeDelivered(q); e != nil && !os.IsNotExist(e) {
				log.Fatal(e)
			}
		}
	}
}

func ensureCommandTopic(broker string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, e := kafka.DialContext(ctx, "tcp", broker)
	if e != nil {
		return e
	}
	defer conn.Close()
	controller, e := conn.Controller()
	if e != nil {
		return e
	}
	admin, e := kafka.DialContext(ctx, "tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if e != nil {
		return e
	}
	defer admin.Close()
	admin.SetDeadline(time.Now().Add(10 * time.Second))
	return admin.CreateTopics(kafka.TopicConfig{Topic: "cvd.commands.v1", NumPartitions: 10, ReplicationFactor: 1})
}
func main() {
	for {
		if e := ensureCommandTopic(env("KAFKA_BOOTSTRAP", "kafka:9092")); e == nil {
			break
		} else {
			log.Print("prepare command topic: ", e)
			time.Sleep(time.Second)
		}
	}
	dir := env("DATA_DIR", "/data")
	if e := os.MkdirAll(filepath.Join(dir, "outbox"), 0700); e != nil {
		log.Fatal(e)
	}
	b := &Bridge{nodes: map[string]*Node{}, dir: dir, writer: &kafka.Writer{Addr: kafka.TCP(env("KAFKA_BOOTSTRAP", "kafka:9092")), Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll, Compression: kafka.Snappy, BatchTimeout: 10 * time.Millisecond, AllowAutoTopicCreation: true}}
	opts := mqtt.NewClientOptions().AddBroker(env("MQTT_URL", "tcp://mqtt:1883")).SetClientID(contract.Host).SetUsername("cvd-bridge").SetPassword(secret(env("MQTT_PASSWORD_FILE", "/run/secrets/mqtt_password"))).SetCleanSession(true).SetAutoReconnect(true).SetWill("spBv1.0/STATE/"+contract.Host, `{"online":false,"timestamp":0}`, 1, true)
	dispatch := equipmentDispatcher(func(topic string, raw []byte) {
		if e := b.receive(topic, raw); e != nil {
			log.Printf("quarantine %s: %v", topic, e)
		}
	})
	opts.OnConnect = func(c mqtt.Client) {
		t := c.Subscribe("spBv1.0/"+contract.Group+"/#", 0, func(_ mqtt.Client, m mqtt.Message) {
			dispatch(m.Topic(), m.Payload())
		})
		if !t.WaitTimeout(5*time.Second) || t.Error() != nil {
			log.Fatal("bridge subscription failed")
		}
		state, _ := json.Marshal(map[string]any{"online": true, "timestamp": time.Now().UnixMilli()})
		t = c.Publish("spBv1.0/STATE/"+contract.Host, 1, true, state)
		t.WaitTimeout(5 * time.Second)
	}
	b.mq = mqtt.NewClient(opts)
	for {
		t := b.mq.Connect()
		if t.WaitTimeout(10*time.Second) && t.Error() == nil {
			break
		}
		time.Sleep(time.Second)
	}
	go b.deliver()
	key := []byte(secret(env("CVD_CONTROL_KEY_FILE", "/run/secrets/control_key")))
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{env("KAFKA_BOOTSTRAP", "kafka:9092")}, GroupID: "cvd-sparkplug-control", Topic: "cvd.commands.v1", WatchPartitionChanges: true, PartitionWatchInterval: time.Second, ErrorLogger: log.Default()})
	go func() {
		for {
			m, e := reader.FetchMessage(context.Background())
			if e != nil {
				log.Print("command fetch: ", e)
				time.Sleep(time.Second)
				continue
			}
			id := string(m.Key)
			if _, e = contract.Verify(m.Value, key, id, time.Now()); e == nil {
				for {
					e = b.publish("DCMD", id, []*sp.Payload_Metric{sp.Metric("CVD/Control", m.Value, time.Now())})
					if e == nil {
						break
					}
					time.Sleep(time.Second)
				}
			} else {
				log.Printf("rejected control equipment=%s kafkaOffset=%d: %v", id, m.Offset, e)
			}
			if e = reader.CommitMessages(context.Background(), m); e != nil {
				log.Print(e)
			}
		}
	}()
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"pendingRecords": b.pending.Load(), "lastKafkaAck": b.lastWrite.Load(), "deliveryFailures": b.deliveryFailures.Load()})
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if !b.mq.IsConnected() || b.pending.Load() > 10000 || time.Since(time.Unix(b.lastWrite.Load(), 0)) > 15*time.Second {
			w.WriteHeader(503)
		}
		fmt.Fprint(w, "Sparkplug Kafka bridge")
	})
	log.Fatal(http.ListenAndServe(":8083", nil))
}

func deviceTopic(id, stream string) string {
	if id == "KLA-SIM-01" {
		return "metrology." + stream + ".v1"
	}
	return "cvd." + stream + ".v1"
}
