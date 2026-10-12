package main

import (
	"context"
	"cvd/protocol/contract"
	"cvd/protocol/gemlink"
	model "cvd/protocol/metrology"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/arloliu/go-secs/v2/hsms"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Job struct {
	ID     string        `json:"measurementId"`
	Source model.Source  `json:"source"`
	State  string        `json:"state"`
	At     time.Time     `json:"at"`
	Result *model.Result `json:"result,omitempty"`
}
type Event struct {
	Equipment string    `json:"equipmentId"`
	ID        string    `json:"eventId"`
	Sequence  uint64    `json:"sequence"`
	Kind      string    `json:"kind"`
	Time      time.Time `json:"sourceTime"`
	Payload   any       `json:"payload"`
}
type State struct {
	Equipment string          `json:"equipmentId"`
	Type      string          `json:"equipmentType"`
	Epoch     string          `json:"epoch"`
	Version   uint64          `json:"version"`
	Sequence  uint64          `json:"sequence"`
	Time      time.Time       `json:"sourceTime"`
	Jobs      map[string]*Job `json:"jobs"`
	Queue     []string        `json:"queue"`
	Events    []Event         `json:"events"`
}
type Adapter struct {
	mu   sync.Mutex
	s    State
	path string
	key  []byte
}

func load(path string, key []byte) (*Adapter, error) {
	a := &Adapter{path: path, key: key, s: State{Equipment: model.Station, Type: "metrology", Epoch: fmt.Sprint(time.Now().UnixNano()), Jobs: map[string]*Job{}, Queue: []string{}, Events: []Event{}}}
	b, e := os.ReadFile(path)
	if e == nil {
		e = json.Unmarshal(b, &a.s)
		if a.s.Equipment != model.Station || a.s.Jobs == nil {
			e = errors.New("invalid checkpoint")
		}
	} else if os.IsNotExist(e) {
		e = nil
	}
	return a, e
}
func (a *Adapter) save() error {
	b, e := json.Marshal(a.s)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(a.path), 0700); e != nil {
		return e
	}
	f, e := os.OpenFile(a.path+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	f.Close()
	if e != nil {
		return e
	}
	if e = os.Rename(a.path+".tmp", a.path); e != nil {
		return e
	}
	d, e := os.Open(filepath.Dir(a.path))
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}
func (a *Adapter) Snapshot() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.s
	s.Jobs = nil
	s.Queue = nil
	if len(s.Events) > 10 {
		s.Events = s.Events[:10]
	}
	b, _ := json.Marshal(s)
	return b
}
func (a *Adapter) emit(j *Job, now time.Time) {
	a.s.Sequence++
	a.s.Events = append(a.s.Events, Event{model.Station, fmt.Sprintf("%s-%d", a.s.Epoch, a.s.Sequence), a.s.Sequence, "measurement-state", now, *j})
}
func (a *Adapter) Execute(raw []byte) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, e := contract.Verify(raw, a.key, model.Station, time.Now())
	if e != nil {
		return nil, e
	}
	var c struct {
		ID         string `json:"commandId"`
		Type       string `json:"type"`
		Parameters struct {
			ID     string       `json:"measurementId"`
			Source model.Source `json:"source"`
		} `json:"parameters"`
	}
	if json.Unmarshal(b, &c) != nil || c.Type != "measure" || c.ID != c.Parameters.ID {
		return nil, errors.New("invalid measurement command")
	}
	if e = c.Parameters.Source.Validate(); e != nil {
		return nil, e
	}
	if old := a.s.Jobs[c.ID]; old != nil {
		before, _ := json.Marshal(old.Source)
		after, _ := json.Marshal(c.Parameters.Source)
		if string(before) != string(after) {
			return nil, errors.New("immutable source")
		}
		return []byte(`{"accepted":true}`), nil
	}
	j := &Job{ID: c.ID, Source: c.Parameters.Source, State: "queued", At: time.Now().UTC()}
	a.s.Jobs[j.ID] = j
	a.s.Queue = append(a.s.Queue, j.ID)
	a.emit(j, j.At)
	if e = a.save(); e != nil {
		log.Fatal(e)
	}
	return []byte(`{"accepted":true}`), nil
}
func (a *Adapter) tick(now time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.s.Version++
	a.s.Time = now.UTC()
	if len(a.s.Queue) > 0 {
		j := a.s.Jobs[a.s.Queue[0]]
		if j.State == "queued" {
			j.State = "measuring"
			j.At = now
			a.emit(j, now)
		} else if now.Sub(j.At) >= 3*time.Second {
			r, e := model.Generate(j.Source)
			j.State = "completed"
			if e != nil {
				j.State = "awaiting-remeasure"
			} else {
				j.Result = &r
			}
			a.emit(j, now)
			a.s.Queue = a.s.Queue[1:]
		}
	}
	return a.save()
}
func (a *Adapter) Ack(n uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if n > a.s.Sequence {
		return errors.New("invalid ACK")
	}
	keep := []Event{}
	for _, e := range a.s.Events {
		if e.Sequence > n {
			keep = append(keep, e)
		}
	}
	a.s.Events = keep
	return a.save()
}
func (a *Adapter) SaveProgram(string, []byte) error { return errors.New("unsupported") }
func (a *Adapter) Program(string) ([]byte, error)   { return nil, errors.New("unsupported") }
func env(k, d string) string {
	if s := os.Getenv(k); s != "" {
		return s
	}
	return d
}
func main() {
	key, e := os.ReadFile(env("CVD_CONTROL_KEY_FILE", "/run/secrets/control_key"))
	if e != nil || len(key) < 32 {
		log.Fatal("missing control key")
	}
	a, e := load(env("DATA_DIR", "/data")+"/state.json", key)
	if e != nil {
		log.Fatal(e)
	}
	c, e := gemlink.New(env("HSMS_ENDPOINT", "0.0.0.0:5000"), false)
	if e != nil {
		log.Fatal(e)
	}
	gemlink.NewDevice(model.Station, a, c)
	if e = c.Open(context.Background(), hsms.OpenBackground); e != nil {
		log.Fatal(e)
	}
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "KLA synthetic metrology") })
	go http.ListenAndServe(":8081", nil)
	for now := range time.NewTicker(time.Second).C {
		if e = a.tick(now); e != nil {
			log.Fatal(e)
		}
	}
}
