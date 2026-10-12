package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func dispositionSend(e *Engine, kind string, a DispositionAction) Result {
	raw, _ := json.Marshal(a)
	key, _ := os.ReadFile(os.Getenv("CVD_RECOVERY_KEY_FILE"))
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(kind + "\n" + string(raw)))
	return cmd(e, kind, "", "", map[string]any{"payload": string(raw), "signature": hex.EncodeToString(mac.Sum(nil))})
}
func completeSupplement(t *testing.T) (*Engine, *Recovery, *Incident) {
	e, p, r := recoveryFixture(t)
	r.Target = p.Process.Thickness + r.RootRate*2
	if x := recoverySend(e, "wafer-supplement", r); x.Status != "succeeded" {
		t.Fatal(x)
	}
	cmd(e, "clear", p.ID, "", nil)
	tick(t, e)
	tick(t, e)
	if e.Recoveries[r.ExecutionID].Status != "awaiting_result" {
		t.Fatal("not waiting")
	}
	return e, r, e.Incidents[r.IncidentID]
}
func TestDispositionSupplementHoldConfirmRelease(t *testing.T) {
	e, r, i := completeSupplement(t)
	before := e.chamber(r.PMID).Process.Thickness
	for n := 0; n < 40; n++ {
		tick(t, e)
	}
	if e.wafer(r.WaferID).Location != r.PMID || !e.DispositionHold || e.chamber(r.PMID).Process.Thickness != before {
		t.Fatal("automatic return/growth")
	}
	path := filepath.Join(t.TempDir(), "held.json")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e = loaded
	i = e.Incidents[r.IncidentID]
	if !e.DispositionHold || cmd(e, "resume", "", "", nil).Status != "rejected" || cmd(e, "start", "", "new", map[string]any{"lp": "LP2", "ll": "LL2"}).Status != "rejected" {
		t.Fatal("restart bypass")
	}
	cmd(e, "recovery-clock", "", "", nil)
	a := DispositionAction{IncidentID: i.ID, ExecutionID: r.ExecutionID, ResultVersion: i.ResultVersion, AuthorizationID: "confirm-1"}
	bad := a
	bad.ResultVersion--
	if dispositionSend(e, "disposition-confirm", bad).Status != "rejected" {
		t.Fatal("stale result")
	}
	bad = a
	bad.EquipmentID = "other"
	if dispositionSend(e, "disposition-confirm", bad).Status != "rejected" {
		t.Fatal("cross tool")
	}
	if x := dispositionSend(e, "disposition-confirm", a); x.Status != "succeeded" {
		t.Fatal(x)
	}
	for n := 0; n < 60 && !i.Returned; n++ {
		tick(t, e)
	}
	if !i.Returned || i.Status != "resolved" || e.wafer(r.WaferID).Location != e.lot(i.LotID).LP || !e.DispositionHold {
		t.Fatal("return or lock")
	}
	if dispositionSend(e, "disposition-confirm", a).Status != "succeeded" {
		t.Fatal("retry")
	}
	release := DispositionAction{ReleaseID: "release-1", Generation: e.HoldGeneration, Evidence: e.releaseEvidence()}
	if x := dispositionSend(e, "equipment-release", release); x.Status != "succeeded" {
		t.Fatal(x)
	}
	if e.DispositionHold || e.DispatchPaused {
		t.Fatal("not released")
	}
	cmd(e, "fault", "PM4", "", nil)
	if dispositionSend(e, "equipment-release", release).Status != "succeeded" || !e.DispositionHold || !e.DispatchPaused {
		t.Fatal("replay unlocked new fault")
	}
}
func TestDispositionScrapAndEmptyPM(t *testing.T) {
	e, p, r := recoveryFixture(t)
	r.DispositionType = "scrap"
	r.StepSeconds = [4]int{}
	r.Target = 0
	r.RootRate = 0
	if recoverySend(e, "wafer-scrap", r).Status != "succeeded" {
		t.Fatal("scrap approval")
	}
	cmd(e, "clear", p.ID, "", nil)
	i := e.Incidents[r.IncidentID]
	for n := 0; n < 60 && !i.Returned; n++ {
		tick(t, e)
	}
	if !i.Returned || e.wafer(r.WaferID).Result != "scrap" || i.Status != "awaiting_result" {
		t.Fatal("scrap not returned")
	}
	a := DispositionAction{IncidentID: i.ID, ExecutionID: r.ExecutionID, ResultVersion: i.ResultVersion, AuthorizationID: "scrap-confirm"}
	if x := dispositionSend(e, "disposition-confirm", a); x.Status != "succeeded" {
		t.Fatal(x)
	}
	cmd(e, "fault", "PM4", "", nil)
	empty := e.activeIncident("PM4", "")
	if empty == nil {
		t.Fatal("empty PM missing")
	}
	release := DispositionAction{ReleaseID: "blocked", Generation: e.HoldGeneration, Evidence: e.releaseEvidence()}
	if dispositionSend(e, "equipment-release", release).Status != "rejected" {
		t.Fatal("other PM bypass")
	}
	er := &Recovery{IncidentID: empty.ID, DispositionType: "equipment", ExecutionID: "equipment-check", RequestID: "empty", Revision: 1, PMID: "PM4"}
	if x := recoverySend(e, "equipment-check", er); x.Status != "succeeded" {
		t.Fatal(x)
	}
	cmd(e, "clear", "PM4", "", nil)
	tick(t, e)
	a = DispositionAction{IncidentID: empty.ID, ExecutionID: er.ExecutionID, ResultVersion: empty.ResultVersion, AuthorizationID: "empty-confirm"}
	if x := dispositionSend(e, "disposition-confirm", a); x.Status != "succeeded" {
		t.Fatal(x)
	}
	if dispositionSend(e, "equipment-release", release).Status != "rejected" {
		t.Fatal("reused failed release")
	}
	release.ReleaseID = "new-release"
	release.Evidence = e.releaseEvidence()
	if x := dispositionSend(e, "equipment-release", release); x.Status != "succeeded" {
		t.Fatal(x)
	}
	if e.wafer(r.WaferID).Result != "scrap" {
		t.Fatal("scrap turned success")
	}
}
func TestDispositionNewFaultRejectsOldConfirmation(t *testing.T) {
	e, r, i := completeSupplement(t)
	a := DispositionAction{IncidentID: i.ID, ExecutionID: r.ExecutionID, ResultVersion: i.ResultVersion, AuthorizationID: "stale"}
	cmd(e, "fault", r.PMID, "", nil)
	cmd(e, "clear", r.PMID, "", nil)
	if dispositionSend(e, "disposition-confirm", a).Status != "rejected" {
		t.Fatal("old confirmation")
	}
	if i.Status != "awaiting_ee" || e.wafer(r.WaferID).Result != "exception" {
		t.Fatal("not held")
	}
}
func TestDispositionRejectPreservesThickness(t *testing.T) {
	e, r, i := completeSupplement(t)
	before := e.chamber(r.PMID).Process.Thickness
	a := DispositionAction{IncidentID: i.ID, ExecutionID: r.ExecutionID, ResultVersion: i.ResultVersion, AuthorizationID: "reject", Reason: "PM 故障"}
	if x := dispositionSend(e, "disposition-reject", a); x.Status != "succeeded" {
		t.Fatal(x)
	}
	if i.Status != "awaiting_ee" || e.chamber(r.PMID).Process.Thickness != before || e.wafer(r.WaferID).Result != "exception" {
		t.Fatal("reject altered baseline")
	}
	if e.Recoveries[r.ExecutionID].Reason != a.Reason || e.Recoveries[r.ExecutionID].ReasonCode != "" {
		t.Fatal("human reason was interpreted as a system code")
	}
}

func TestDispositionMigrationStates(t *testing.T) {
	for _, state := range []string{"approved_waiting", "executing", "held-complete", "returned-complete"} {
		t.Run(state, func(t *testing.T) {
			e, p, r := recoveryFixture(t)
			r.Target = p.Process.Thickness + r.RootRate*2
			recoverySend(e, "wafer-supplement", r)
			if state != "approved_waiting" {
				cmd(e, "clear", p.ID, "", nil)
				tick(t, e)
			}
			if state == "held-complete" || state == "returned-complete" {
				tick(t, e)
				e.Recoveries[r.ExecutionID].Status = "completed"
				e.wafer(r.WaferID).Result = "success"
			}
			if state == "returned-complete" {
				for n := 0; n < 50; n++ {
					tick(t, e)
				}
			}
			if state == "returned-complete" {
				e.Recoveries[r.ExecutionID].Status = "completed"
			}
			original := e.wafer(r.WaferID).Location
			attempts := e.wafer(r.WaferID).Attempts
			e.ModelVersion = 5
			e.DispositionHold = false
			e.Incidents = nil
			e.Recoveries[r.ExecutionID].IncidentID = ""
			e.Recoveries[r.ExecutionID].DispositionType = ""
			path := filepath.Join(t.TempDir(), "v5.json")
			e.Save(path)
			loaded, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if !loaded.DispositionHold || !loaded.DispatchPaused || loaded.wafer(r.WaferID).Location != original || loaded.wafer(r.WaferID).Attempts != attempts {
				t.Fatal("migration physical change")
			}
			rr := loaded.Recoveries[r.ExecutionID]
			if state == "approved_waiting" && rr.Status != "cancelled" {
				t.Fatal("old queued execution unfenced")
			}
			if state == "executing" && rr.Status != "executing" {
				t.Fatal("lost execution")
			}
			if (state == "held-complete" || state == "returned-complete") && rr.Status != "awaiting_result" {
				t.Fatal("missing result review")
			}
			for _, i := range loaded.Incidents {
				if i.ConfirmationID != "" || i.Released {
					t.Fatal("fabricated signature")
				}
			}
			loaded.Save(path)
			twice, err := Load(path)
			if err != nil || twice.HoldGeneration != loaded.HoldGeneration || len(twice.Incidents) != len(loaded.Incidents) {
				t.Fatal("migration not idempotent")
			}
		})
	}
}
func TestDispositionReleaseGenerationRace(t *testing.T) {
	e, r, i := completeSupplement(t)
	a := DispositionAction{IncidentID: i.ID, ExecutionID: r.ExecutionID, ResultVersion: i.ResultVersion, AuthorizationID: "confirmation"}
	dispositionSend(e, "disposition-confirm", a)
	for n := 0; n < 60 && !i.Returned; n++ {
		tick(t, e)
	}
	release := DispositionAction{ReleaseID: "old-intent", Generation: e.HoldGeneration, Evidence: e.releaseEvidence()}
	cmd(e, "fault", "PM4", "", nil)
	if dispositionSend(e, "equipment-release", release).Status != "rejected" || !e.DispositionHold {
		t.Fatal("new fault lost race")
	}
}

func TestScrapCancellationRace(t *testing.T) {
	for _, cancelFirst := range []bool{true, false} {
		e, p, r := recoveryFixture(t)
		r.DispositionType = "scrap"
		r.StepSeconds = [4]int{}
		if cancelFirst {
			recoverySend(e, "cancel-supplement", r)
		}
		recoverySend(e, "wafer-scrap", r)
		cmd(e, "clear", p.ID, "", nil)
		tick(t, e)
		result := recoverySend(e, "cancel-supplement", r)
		if cancelFirst {
			if e.wafer(r.WaferID).Result != "exception" || e.Recoveries[r.ExecutionID].Status != "cancelled" {
				t.Fatal("cancel lost")
			}
		} else {
			if result.Status != "rejected" || e.wafer(r.WaferID).Result != "scrap" {
				t.Fatal("execution cancelled after start")
			}
		}
	}
}

func TestProductionCheckpointMigrationReadOnly(t *testing.T) {
	dir := os.Getenv("CVD_CHECKPOINT_DIR")
	if dir == "" {
		t.Skip("set CVD_CHECKPOINT_DIR to backup states-original for migration rehearsal")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) != 10 {
		t.Fatal("expected ten backup checkpoints")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var old Engine
			if json.Unmarshal(raw, &old) != nil {
				t.Fatal("bad checkpoint")
			}
			next, err := Load(file)
			if err != nil {
				t.Fatal(err)
			}
			if next.EquipmentID != old.EquipmentID || len(next.Lots) != len(old.Lots) || len(next.Recoveries) != len(old.Recoveries) {
				t.Fatal("identity/count changed")
			}
			for _, l := range old.Lots {
				for _, w := range l.Wafers {
					n := next.wafer(w.ID)
					if n == nil || n.Location != w.Location || n.Attempts != w.Attempts || n.Slot != w.Slot || n.LotID != w.LotID {
						t.Fatal("wafer altered", w.ID)
					}
				}
			}
			for _, p := range old.Chambers {
				q := next.chamber(p.ID)
				if p.Process != nil && (q.Process == nil || q.Process.Thickness != p.Process.Thickness || q.Process.SampleThickness != p.Process.SampleThickness || q.Process.Elapsed != p.Process.Elapsed) {
					t.Fatal("process altered", p.ID)
				}
			}
			for id, r := range old.Recoveries {
				n := next.Recoveries[id]
				if n.ExecutionID != r.ExecutionID || n.RequestID != r.RequestID || n.Revision != r.Revision {
					t.Fatal("execution altered")
				}
			}
			for _, i := range next.Incidents {
				if i.ConfirmationID != "" || i.Released {
					t.Fatal("migration invented approval")
				}
			}
		})
	}
}

func TestLegacyDirectScrapRequiresReviewWithoutReload(t *testing.T) {
	e, p, r := recoveryFixture(t)
	w := e.wafer(r.WaferID)
	l := e.lot(w.LotID)
	w.Result = "scrap"
	w.Location = l.LP
	p.WaferID = ""
	p.Process = nil
	p.ProcessRunID = ""
	p.Remaining = 0
	p.Fault = false
	e.ModelVersion = 5
	e.DispositionMigrationVersion = 0
	e.Incidents = nil
	e.Recoveries = nil
	e.DispositionHold = false
	e.DispatchPaused = true
	l.Paused = true
	path := filepath.Join(t.TempDir(), "old-scrap.json")
	e.Save(path)
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e = loaded
	if !e.DispositionHold || len(e.Incidents) != 1 {
		t.Fatal("old direct scrap bypassed review")
	}
	var i *Incident
	for _, incident := range e.Incidents {
		i = incident
	}
	if !i.Legacy || !i.Returned || i.Status != "awaiting_ee" || i.ExecutionID != "" {
		t.Fatal("fabricated old execution")
	}
	if cmd(e, "resume", "", "", nil).Status != "rejected" {
		t.Fatal("legacy hold bypass")
	}
	r = &Recovery{IncidentID: i.ID, DispositionType: "legacy_return", LegacyResult: "scrap", ExecutionID: "legacy-evidence-check", RequestID: "legacy-ee", Revision: 1, WaferID: w.ID, ExceptionVersion: w.ExceptionVersion}
	if x := recoverySend(e, "legacy-return-check", r); x.Status != "succeeded" {
		t.Fatal(x)
	}
	cmd(e, "recovery-clock", "", "", nil)
	tick(t, e)
	if i.Status != "awaiting_result" || e.wafer(w.ID).Location != l.LP || e.wafer(w.ID).Result != "scrap" || e.wafer(w.ID).Attempts != w.Attempts {
		t.Fatal("legacy check reprocessed wafer")
	}
	a := DispositionAction{IncidentID: i.ID, ExecutionID: r.ExecutionID, ResultVersion: i.ResultVersion, AuthorizationID: "legacy-pe-confirm"}
	if x := dispositionSend(e, "disposition-confirm", a); x.Status != "succeeded" {
		t.Fatal(x)
	}
	a = DispositionAction{ReleaseID: "legacy-release", Generation: e.HoldGeneration, Evidence: e.releaseEvidence()}
	if x := dispositionSend(e, "equipment-release", a); x.Status != "succeeded" {
		t.Fatal(x)
	}
	if e.wafer(w.ID).Result != "scrap" {
		t.Fatal("legacy scrap changed")
	}
}
