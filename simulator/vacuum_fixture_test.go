package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteVacuumIntegrationFixture(t *testing.T) {
	dir := os.Getenv("PHM_FIXTURE_DIR")
	if dir == "" {
		return
	}
	e := NewEngine()
	e.EquipmentID = "TEL-CVDTIN-08A-01"
	e.Now = time.Now().UTC()
	e.initVacuum()
	p := e.Locks[0].Pump
	p.SinceService = 832
	p.Lifetime = 832
	p.Wear = .832
	p.refresh()
	if err := e.Save(filepath.Join(dir, "state.json")); err != nil {
		t.Fatal(err)
	}
}
