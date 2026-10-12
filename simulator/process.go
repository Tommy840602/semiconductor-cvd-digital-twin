package main

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
)

const modelRevision = "synthetic-tin-v1"

type Gas struct {
	Target float64 `json:"target"`
	Actual float64 `json:"actual"`
}
type ProcessTelemetry struct {
	RecipeVersion        string     `json:"recipeVersion"`
	SourceRunID          string     `json:"sourceProcessRunId,omitempty"`
	RecoveryRequestID    string     `json:"recoveryRequestId,omitempty"`
	RecoveryRevision     int        `json:"recoveryRevision,omitempty"`
	ExecutionID          string     `json:"executionId,omitempty"`
	RootRate             float64    `json:"rootDepositionRateNmPerSecond"`
	BaselineThickness    float64    `json:"baselineThicknessNm"`
	DeltaThickness       float64    `json:"deltaThicknessNm"`
	StepSeconds          [4]int     `json:"stepSeconds"`
	CompletedTick        int64      `json:"completedTick"`
	TelemetryRevision    int        `json:"telemetryRevision"`
	ModelRevision        string     `json:"modelRevision"`
	ProcessRunID         string     `json:"processRunId"`
	Duration             int        `json:"durationSeconds"`
	Elapsed              int        `json:"elapsedSeconds"`
	Phase                string     `json:"phase"`
	PhaseElapsed         int        `json:"phaseElapsedSeconds"`
	PhaseDuration        int        `json:"phaseDurationSeconds"`
	PhaseProgress        float64    `json:"phaseProgress"`
	RunProgress          float64    `json:"runProgress"`
	HeaterPower          float64    `json:"heaterPowerPct"`
	TemperatureDeviation float64    `json:"temperatureDeviationC"`
	Thickness            float64    `json:"thicknessNm"`
	Rate                 float64    `json:"depositionRateNmPerSecond"`
	Uniformity           *float64   `json:"uniformityPct"`
	Quality              string     `json:"quality"`
	TargetThickness      float64    `json:"targetThicknessNm"`
	ThicknessTolerance   float64    `json:"thicknessTolerancePct"`
	UniformityLimit      float64    `json:"uniformityLimitPct"`
	SampleThickness      [5]float64 `json:"sampleThicknessNm"`
}

func phaseDurations(total int) [4]int {
	weights := []float64{.20, .20, .45, .15}
	result := [4]int{1, 1, 1, 1}
	remainder := total - 4
	fractions := make([]float64, 4)
	used := 0
	for i, w := range weights {
		value := float64(remainder) * w
		n := int(value)
		result[i] += n
		used += n
		fractions[i] = value - float64(n)
	}
	order := []int{0, 1, 2, 3}
	sort.SliceStable(order, func(i, j int) bool { return fractions[order[i]] > fractions[order[j]] })
	for i := 0; i < remainder-used; i++ {
		result[order[i]]++
	}
	return result
}
func noise(seed uint64, key string) float64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d/%s", seed, key)
	return float64(h.Sum64()%100000)/49999.5 - 1
}
func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func (e *Engine) beginProcess(p *Chamber, w *Wafer) {
	p.Process = &ProcessTelemetry{RecipeVersion: e.RecipeVersion, TelemetryRevision: 1, ModelRevision: modelRevision, ProcessRunID: fmt.Sprintf("%s/%d/%s", w.ID, w.Attempts, modelRevision), Duration: e.ProcessSeconds, Quality: "not-evaluated", TargetThickness: 10, ThicknessTolerance: 10, UniformityLimit: 5}
	p.Process.StepSeconds = phaseDurations(e.ProcessSeconds)
	if e.RecipeSteps != nil {
		p.Process.StepSeconds = *e.RecipeSteps
	}
	p.Process.RootRate = processRootRate(p.Process)
	p.GasChannels = map[string]Gas{}
	p.ProcessRunID = p.Process.ProcessRunID
}
func (e *Engine) stepProcess(p *Chamber) {
	t := p.Process
	durations := t.StepSeconds
	if durations == [4]int{} {
		durations = phaseDurations(t.Duration)
		t.StepSeconds = durations
	}
	names := []string{"preheat", "stabilize", "deposition", "purge"}
	offset := 0
	idx := 0
	for idx < 3 && t.Elapsed >= offset+durations[idx] {
		offset += durations[idx]
		idx++
	}
	w := e.wafer(p.WaferID)
	if t.Phase != names[idx] {
		old := t.Phase
		t.Phase = names[idx]
		e.emit("process-phase-changed", w.LotID, w.ID, map[string]any{"pm": p.ID, "processRunId": t.ProcessRunID, "from": old, "phase": t.Phase, "durationSeconds": durations[idx]})
	}
	t.Elapsed++
	t.PhaseElapsed = t.Elapsed - offset
	t.PhaseDuration = durations[idx]
	t.PhaseProgress = float64(t.PhaseElapsed) / float64(t.PhaseDuration)
	t.RunProgress = float64(t.Elapsed) / float64(t.Duration)
	p.Step = t.Phase
	p.Remaining = t.Duration - t.Elapsed
	perturb := noise(e.ModelSeed, fmt.Sprintf("%s/%s/%d", p.ID, t.ProcessRunID, e.TickNumber))
	bias := .02 * noise(e.ModelSeed, p.ID)
	p.TargetTemperature = 400
	t.HeaterPower = clamp((p.TargetTemperature-p.Temperature)*.35+15, 0, 100)
	p.Temperature += (p.TargetTemperature-p.Temperature)*.18 + perturb*.15
	t.TemperatureDeviation = p.Temperature - p.TargetTemperature
	targets := map[string]float64{"precursor": 0, "reactant": 0, "carrier": 30, "purge": 0}
	if idx == 2 {
		targets["precursor"] = 40
		targets["reactant"] = 70
	}
	if idx == 3 {
		targets["carrier"] = 0
		targets["purge"] = 100
	}
	p.Flow = 0
	p.TargetFlow = 0
	for _, id := range []string{"precursor", "reactant", "carrier", "purge"} {
		target := targets[id]
		actual := target * (1 + bias + perturb*.01)
		p.GasChannels[id] = Gas{target, actual}
		p.Flow += actual
		p.TargetFlow += target
	}
	p.TargetPressure = 2
	p.Pressure = math.Max(.001, p.Pressure+(2-p.Pressure)*.4+perturb*.005)
	p.PumpState = "holding"
	p.ValveOpening = clamp(45+(p.Pressure-2)*10+p.Flow*.03, 0, 100)
	t.Rate = 0
	if idx == 2 {
		t.Rate = processRootRate(t) * (p.GasChannels["precursor"].Actual / 40)
		for i := range t.SampleThickness {
			t.SampleThickness[i] += t.Rate * (1 + .02*noise(e.ModelSeed, fmt.Sprintf("%s/%d", p.ID, i)))
		}
	}
	t.Thickness = 0
	min, max := math.Inf(1), 0.0
	for _, v := range t.SampleThickness {
		t.Thickness += v / 5
		min = math.Min(min, v)
		max = math.Max(max, v)
	}
	if t.Thickness > 0 {
		u := (max - min) / 2 / t.Thickness * 100
		t.Uniformity = &u
	}
	t.DeltaThickness = t.Thickness - t.BaselineThickness
	if p.Remaining == 0 {
		t.CompletedTick = e.TickNumber
		w.Result = "success"
		t.Quality = "review"
		if math.Abs(t.Thickness-t.TargetThickness)/t.TargetThickness*100 <= t.ThicknessTolerance && t.Uniformity != nil && *t.Uniformity <= t.UniformityLimit {
			t.Quality = "pass"
		}
		if t.ExecutionID != "" {
			if t.Quality != "pass" {
				w.Result = "exception"
				w.ExceptionVersion++
				e.DispatchPaused = true
				e.lot(w.LotID).Paused = true
				e.endRecovery(p, "failed", "膜厚或均勻度未達容差，保留晶圓待新一輪簽核")
			} else {
				w.Result = "awaiting_result"
				e.endRecovery(p, "awaiting_result", "原 PM 留片，等待 PE 結果確認")
			}
		}
		p.Step = "complete"
		e.emit("process-phase-changed", w.LotID, w.ID, map[string]any{"pm": p.ID, "processRunId": t.ProcessRunID, "from": t.Phase, "phase": "complete"})
		e.processResult(p, w, true, "completed")
		e.emit("process-complete", w.LotID, w.ID, map[string]any{"pm": p.ID, "attempt": w.Attempts, "processRunId": t.ProcessRunID})
	}
}
func (e *Engine) processResult(p *Chamber, w *Wafer, complete bool, reason string) {
	if p.Process == nil {
		return
	}
	copy := *p.Process
	w.MetrologyRunID = copy.ProcessRunID
	if !complete {
		copy.Quality = "not-evaluated"
	}
	e.qualificationResult(p, w, complete, copy)
	e.emit("process-result", w.LotID, w.ID, map[string]any{"pm": p.ID, "processRunId": copy.ProcessRunID, "complete": complete, "reason": reason, "result": copy, "slot": w.Slot, "purpose": w.Purpose})
}
func (e *Engine) vacuumTelemetry(ll *Lock) {
	ll.WaferID = ""
	if w := e.lockWafer(ll.ID); w != nil {
		ll.WaferID = w.ID
	}
	ll.PumpState = "idle"
	ll.ValveOpening = 0
	if ll.Phase == "pumping" {
		ll.PumpState = "pumping"
		ll.ValveOpening = 100
		if ll.Pump == nil || ll.Pump.Legacy {
			ll.Pressure = math.Exp(math.Log(1013) + (math.Log(.01)-math.Log(1013))*float64(3-ll.Remaining)/3)
		}
	}
	if ll.Phase == "venting" {
		ll.PumpState = "venting"
		ll.ValveOpening = 100
		ll.Pressure = math.Exp(math.Log(.01) + (math.Log(1013)-math.Log(.01))*float64(3-ll.Remaining)/3)
	}
	if ll.Phase == "vacuum" {
		ll.PumpState = "holding"
		ll.Pressure = .01
	}
	if ll.Phase == "atmosphere" || ll.Phase == "empty" {
		ll.Pressure = 1013
	}
}
