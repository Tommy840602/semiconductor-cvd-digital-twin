package main

import (
	"context"
	"cvd/protocol/contract"
	"cvd/protocol/gemlink"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/arloliu/go-secs/v2/hsms"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type secsAdapter struct {
	mu         sync.Mutex
	e          *Engine
	dir        string
	key        []byte
	lastSample time.Time
}

func (a *secsAdapter) sample(now time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.e.EnforceBuffer(now)
	a.e.Tick(now)
	if !a.lastSample.IsZero() && now.Sub(a.lastSample) > 1500*time.Millisecond {
		a.e.emit("data-gap", "", "", map[string]any{"reason": "sampling delayed", "from": a.lastSample, "to": now, "missedSeconds": int(now.Sub(a.lastSample)/time.Second) - 1})
	}
	a.lastSample = now
	return a.e.Save(a.dir + "/state.json")
}
func (a *secsAdapter) Snapshot() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, _ := json.Marshal(a.e)
	return b
}
func (a *secsAdapter) Execute(raw []byte) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, e := contract.Verify(raw, a.key, a.e.EquipmentID, time.Now())
	if e != nil {
		return nil, e
	}
	var c Command
	if e = json.Unmarshal(b, &c); e != nil {
		return nil, e
	}
	if c.Type == "vacuum-service-channel" || c.Type == "vacuum-recover" || c.Type == "vacuum-test" || (c.Type == "maintenance-request" && c.Parameters["selectedPumpIds"] != nil) {
		var envelope struct {
			Actor struct {
				Role string `json:"role"`
			} `json:"actor"`
		}
		json.Unmarshal(b, &envelope)
		if envelope.Actor.Role != "Engineer" && envelope.Actor.Role != "Leader" {
			return nil, errors.New("vacuum control requires Engineer or Leader")
		}
	}
	r := a.e.Execute(c, time.Now())
	if e = a.e.Save(a.dir + "/state.json"); e != nil {
		log.Fatal(e)
	}
	return json.Marshal(r)
}
func (a *secsAdapter) Ack(seq uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if seq > uint64(a.e.Sequence) {
		return errors.New("invalid ack")
	}
	keep := a.e.Events[:0]
	for _, e := range a.e.Events {
		if uint64(e.Sequence) > seq {
			keep = append(keep, e)
		}
	}
	a.e.Events = keep
	return a.e.Save(a.dir + "/state.json")
}
func (a *secsAdapter) programPath(id string) (string, error) {
	if !strings.HasPrefix(id, a.e.EquipmentID+"-recipe-") || strings.ContainsAny(id, "/\\.") {
		return "", errors.New("invalid PPID")
	}
	return filepath.Join(a.dir, "programs", id+".json"), nil
}
func (a *secsAdapter) SaveProgram(id string, b []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, e := a.programPath(id)
	if e != nil {
		return e
	}
	raw, e := contract.Verify(b, a.key, a.e.EquipmentID, time.Now())
	if e != nil {
		return e
	}
	var command struct {
		Type       string         `json:"type"`
		Parameters map[string]any `json:"parameters"`
	}
	if json.Unmarshal(raw, &command) != nil || command.Type != "configure" {
		return errors.New("invalid program control")
	}
	settings := command.Parameters
	b, e = json.Marshal(settings)
	if e != nil {
		return e
	}
	steps, ok := settings["stepSeconds"].(map[string]any)
	if !ok || len(steps) != 4 || settings["recipeVersion"] != id {
		return errors.New("invalid recipe")
	}
	total := 0
	for _, k := range []string{"preheat", "stabilize", "deposition", "purge"} {
		v, ok := steps[k].(float64)
		if !ok || v < 1 || v > 3600 || v != float64(int(v)) {
			return errors.New("invalid step")
		}
		total += int(v)
	}
	if total > 3600 {
		return errors.New("recipe too long")
	}
	if old, e := os.ReadFile(p); e == nil {
		if string(old) != string(b) {
			return errors.New("immutable PPID")
		}
		return nil
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	f, e := os.OpenFile(p+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
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
	return os.Rename(p+".tmp", p)
}
func (a *secsAdapter) Program(id string) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, e := a.programPath(id)
	if e != nil {
		return nil, e
	}
	return os.ReadFile(p)
}
func runSECS() {
	dir := env("DATA_DIR", ".runtime")
	if e := os.MkdirAll(dir, 0700); e != nil {
		log.Fatal(e)
	}
	engine, e := Load(dir + "/state.json")
	if e != nil {
		log.Fatal(e)
	}
	id := env("EQUIPMENT_ID", "TEL-CVDTIN-08A-01")
	if !validEquipment(id) || (engine.EquipmentID != "" && engine.EquipmentID != id) {
		log.Fatal("equipment mismatch")
	}
	engine.EquipmentID = id
	key, e := os.ReadFile(env("CVD_CONTROL_KEY_FILE", "/run/secrets/control_key"))
	if e != nil || len(key) < 32 {
		log.Fatal("control key unavailable")
	}
	a := &secsAdapter{e: engine, dir: dir, key: key, lastSample: engine.Now}
	c, e := gemlink.New(env("HSMS_ENDPOINT", "0.0.0.0:5000"), false)
	if e != nil {
		log.Fatal(e)
	}
	d := gemlink.NewDevice(id, a, c)
	if e = c.Open(context.Background(), hsms.OpenBackground); e != nil {
		log.Fatal(e)
	}
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "hsms simulator") })
	go func() { log.Print(http.ListenAndServe(":8081", nil)) }()
	trace := make(chan time.Time, 1)
	go func() {
		for now := range trace {
			d.Trace(now)
		}
	}()
	for now := range time.NewTicker(time.Second).C {
		if e = a.sample(now); e != nil {
			log.Fatal(e)
		}
		select {
		case trace <- now:
		default:
		}
	}
}
