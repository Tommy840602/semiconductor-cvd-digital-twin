package main

import "time"
import "fmt"

// Statistics is an interval fact, not a last-observation carry-forward.
type Statistics struct {
	Version      int    `json:"version"`
	State        string `json:"state"`
	Flags        int    `json:"flags"`
	Era          string `json:"era"`
	ClosureCount int64  `json:"closureCount"`
}

var maintenancePhases = []string{"", "draining", "maintenance", "waiting", "qualifying", "failed", "passed", "ready", "released"}

func (e *Engine) statisticsInit() {
	if e.StatisticsEra == "" {
		e.StatisticsEra = fmt.Sprintf("%s-%d", e.EquipmentID, e.Now.UnixNano())
	}
}
func (e *Engine) statisticsFlags() int {
	flags := 0
	for n, p := range e.Chambers {
		if p.Fault {
			flags |= 1 << n
		}
	}
	if e.DispatchPaused {
		flags |= 16
	}
	if e.DispositionHold {
		flags |= 32
	}
	if e.Maintenance != nil {
		for n, p := range maintenancePhases {
			if p == e.Maintenance.Phase {
				flags |= n << 6
			}
		}
	}
	if e.Paused {
		flags |= 65536
	}
	return flags
}
func statisticsState(flags int, production, dummy bool) string {
	phase := (flags >> 6) & 15
	if flags&65536 != 0 {
		return "paused"
	}
	if phase == 2 {
		return "maintenance"
	}
	if dummy {
		return "dummy"
	}
	if production {
		return "processing"
	}
	if flags&47 != 0 {
		return "fault"
	}
	if phase == 4 {
		return "dummy"
	}
	if flags&16 != 0 || (phase > 0 && phase < 8) {
		return "paused"
	}
	return "idle"
}
func (e *Engine) statisticsPayload(active map[string]bool) Statistics {
	production, dummy := false, false
	for _, p := range e.Chambers {
		if active[p.ID] {
			if e.statisticsDummy[p.ID] {
				dummy = true
			} else {
				production = true
			}
		}
	}
	state := statisticsState(e.statisticsBefore, production, dummy)
	if e.statisticsDirty || e.statisticsBefore != e.statisticsFlags() {
		state = "unknown"
	}
	e.statisticsDirty = false
	return Statistics{1, state, e.statisticsBefore, e.StatisticsEra, e.StatisticsClosureCount}
}
func (e *Engine) closeWafers() {
	e.statisticsInit()
	for _, l := range e.Lots {
		for _, w := range l.Wafers {
			if w.ClosedAt != nil || !isTerminal(w) || w.Location != l.LP {
				continue
			}
			var incident *Incident
			for _, i := range e.Incidents {
				if i.WaferID == w.ID {
					incident = i
					break
				}
			}
			returned := w.LastReturnAt
			var confirmed *time.Time
			confirmation, id := "", ""
			if incident != nil {
				if incident.Status != "resolved" || !incident.Returned || incident.ConfirmationID == "" || incident.ConfirmedAt == nil {
					continue
				}
				if returned == nil {
					returned = incident.ReturnedAt
				}
				confirmed = incident.ConfirmedAt
				confirmation = incident.ConfirmationID
				id = incident.ID
			} else if w.Result == "scrap" || w.Attempts > 1 {
				continue
			}
			if returned == nil {
				continue
			} // A terminal legacy snapshot is not a timestamp.
			at := *returned
			if confirmed != nil && confirmed.After(at) {
				at = *confirmed
			}
			w.ClosedAt = &at
			e.StatisticsClosureCount++
			w.ClosureSequence = e.StatisticsClosureCount
			e.emit("wafer-closed", l.ID, w.ID, map[string]any{"version": 1, "era": e.StatisticsEra, "closureSequence": e.StatisticsClosureCount, "purpose": w.Purpose, "result": w.Result, "closedAt": at, "returnedAt": returned, "confirmedAt": confirmed, "confirmationId": confirmation, "incidentId": id})
		}
	}
}
