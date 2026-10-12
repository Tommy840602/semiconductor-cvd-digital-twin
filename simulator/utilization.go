package main

import "time"

// Utilization describes one completed wall-clock second, never a simulated duration.
type Utilization struct {
	Version       int             `json:"version"`
	IntervalStart time.Time       `json:"intervalStart"`
	IntervalEnd   time.Time       `json:"intervalEnd"`
	PM            map[string]bool `json:"pm"`
}

func (e *Engine) telemetryPayload(start, end time.Time, active map[string]bool) map[string]any {
	p := map[string]any{"chambers": e.copyChambers(), "locks": e.copyLocks()}
	if !start.IsZero() && end.Sub(start) == time.Second {
		p["utilization"] = Utilization{1, start, end, active}
		p["statistics"] = e.statisticsPayload(active)
	}
	return p
}
