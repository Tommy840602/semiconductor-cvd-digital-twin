package main

import (
	"fmt"
	"testing"
	"time"
)

func TestFleetCommandIsolation(t *testing.T) {
	a, b := NewEngine(), NewEngine()
	a.EquipmentID = "TEL-CVDTIN-08A-01"
	b.EquipmentID = "TEL-CVDTIN-08A-02"
	c := Command{EquipmentID: a.EquipmentID, CommandID: "fault-1", Type: "fault", TargetID: "PM1", ExpiresAt: time.Now().Add(time.Hour)}
	if b.Execute(c, time.Now()).Status != "rejected" || b.Chambers[0].Fault || len(b.Commands) != 0 {
		t.Fatal("cross-tool mutation")
	}
	if a.Execute(c, time.Now()).Status != "succeeded" || !a.Chambers[0].Fault {
		t.Fatal("target tool did not apply")
	}
	if a.Events[len(a.Events)-1].EquipmentID != a.EquipmentID {
		t.Fatal("missing event identity")
	}
}

func TestTenToolsSeventyFiveWafersEach(t *testing.T) {
	for n := 1; n <= 10; n++ {
		e := NewEngine()
		e.ProcessSeconds = 7
		for i := 1; i <= 3; i++ {
			start(e, i)
		}
		e.EquipmentID = fmt.Sprintf("TEL-CVDTIN-08A-%02d", n)
		completeAll(t, e)
		count := 0
		for _, lot := range e.Lots {
			for _, w := range lot.Wafers {
				count++
				if w.Location != lot.LP {
					t.Fatal("wafer left original tool/port")
				}
			}
		}
		if count != 75 {
			t.Fatal(count)
		}
	}
}
