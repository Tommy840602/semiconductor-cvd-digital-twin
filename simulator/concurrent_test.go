package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func three(t *testing.T, seconds int) *Engine {
	e := NewEngine()
	e.ProcessSeconds = seconds
	for i := 1; i <= 3; i++ {
		start(e, i)
	}
	return e
}
func completeAll(t *testing.T, e *Engine) {
	t.Helper()
	for n := 0; n < 2400; n++ {
		tick(t, e)
		done := true
		for _, l := range e.Lots {
			if l.Phase != "complete" {
				done = false
			}
		}
		if done {
			return
		}
	}
	t.Fatal("batches failed to drain")
}
func TestConcurrentShortRecipeAndPause(t *testing.T) {
	for _, seconds := range []int{5, 30} {
		e := three(t, seconds)
		cmd(e, "pause", "", "L1", nil)
		for n := 0; n < 100; n++ {
			tick(t, e)
		}
		for _, w := range e.Lots[0].Wafers {
			if w.Attempts != 0 {
				t.Fatal("paused batch dispatched")
			}
		}
		for _, l := range e.Lots[1:] {
			seen := false
			for _, w := range l.Wafers {
				seen = seen || w.Attempts > 0
			}
			if !seen {
				t.Fatal("other lot blocked", l.ID)
			}
		}
		cmd(e, "resume", "", "L1", nil)
		completeAll(t, e)
		for _, l := range e.Lots {
			for _, w := range l.Wafers {
				if w.Attempts != 1 || w.Location != l.LP || w.Result != "success" {
					t.Fatal(w)
				}
			}
		}
	}
}
func TestConcurrentReadyLotsRotate(t *testing.T) {
	e := three(t, 30)
	for _, l := range e.Lots {
		w := l.Wafers[0]
		w.Location = l.LL
		ll := e.lock(l.LL)
		ll.Phase = "vacuum"
	}
	for want := 1; want <= 3; want++ {
		for n := 0; n < 10 && e.LastDispatchOrder != want; n++ {
			tick(t, e)
		}
		if e.LastDispatchOrder != want {
			t.Fatal("round robin order", want, e.LastDispatchOrder)
		}
	}
	completeAll(t, e)
}
func TestConcurrentCheckpointAndOldOverbooking(t *testing.T) {
	e := three(t, 30)
	for n := 0; n < 50; n++ {
		tick(t, e)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	e.Save(path)
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Paused || r.LastDispatchOrder != e.LastDispatchOrder || r.NextLoadLock != e.NextLoadLock {
		t.Fatal("cursor lost")
	}
	r.Paused = false
	for n := 0; n < 100; n++ {
		tick(t, e)
		tick(t, r)
		if !reflect.DeepEqual(e.Chambers, r.Chambers) || !reflect.DeepEqual(e.Lots, r.Lots) || e.LastDispatchOrder != r.LastDispatchOrder {
			t.Fatal("restart diverged")
		}
	}
	// A v3 checkpoint may have every PM and every LL occupied.
	old := three(t, 30)
	old.ModelVersion = 3
	for i, p := range old.Chambers {
		w := old.Lots[0].Wafers[i]
		w.Location = p.ID
		w.Result = "success"
		w.Attempts = 1
		p.WaferID = w.ID
	}
	for _, l := range old.Lots {
		for _, w := range l.Wafers {
			if w.Result == "pending" {
				w.Location = l.LL
				break
			}
		}
		old.lock(l.LL).Phase = "vacuum"
	}
	old.Save(path)
	restored, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ModelVersion != 6 || !restored.Paused {
		t.Fatal("migration")
	}
	restored.Paused = false
	completeAll(t, restored)
}
func TestConcurrentFaultDoesNotPauseUnrelatedLot(t *testing.T) {
	e := three(t, 30)
	var target *Chamber
	for n := 0; n < 100 && target == nil; n++ {
		tick(t, e)
		for _, p := range e.Chambers {
			if p.WaferID != "" && e.wafer(p.WaferID).LotID == "L2" {
				target = p
				break
			}
		}
	}
	if target == nil {
		t.Fatal("L2 never started")
	}
	cmd(e, "fault", target.ID, "L2", nil)
	if !e.DispatchPaused || !e.Lots[1].Paused || e.Lots[0].Paused {
		t.Fatal("wrong fault pause scope")
	}
	for n := 0; n < 50; n++ {
		tick(t, e)
	}
	cmd(e, "clear", target.ID, "L2", nil)
	for n := 0; n < 5; n++ {
		tick(t, e)
	}
	if !e.DispatchPaused || e.wafer(target.WaferID).Result != "exception" {
		t.Fatal("auto resumed")
	}
	if result := cmd(e, "scrap", target.ID, "L2", nil); result.Status != "rejected" {
		t.Fatal("scrap bypass")
	}
	if result := cmd(e, "resume", "", "L2", nil); result.Status != "rejected" {
		t.Fatal("release bypass")
	}
	if !e.DispositionHold {
		t.Fatal("tool hold missing")
	}
}
