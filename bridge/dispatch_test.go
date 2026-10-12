package main

import (
	"cvd/protocol/contract"
	"testing"
	"time"
)

func TestSlowEquipmentDoesNotBlockAnotherAndOrderIsPreserved(t *testing.T) {
	gate := make(chan struct{})
	seen := make(chan string, 4)
	dispatch := equipmentDispatcher(func(topic string, raw []byte) {
		_, id, _ := contract.ParseTopic(topic)
		if id == "TEL-CVDTIN-08A-01" {
			<-gate
		}
		seen <- string(raw)
	})
	dispatch(contract.Topic("NBIRTH", "TEL-CVDTIN-08A-01"), []byte("first"))
	raw := []byte("second")
	dispatch(contract.Topic("DBIRTH", "TEL-CVDTIN-08A-01"), raw)
	raw[0] = 'X'
	dispatch(contract.Topic("NBIRTH", "TEL-CVDTIN-08A-02"), []byte("independent"))
	select {
	case got := <-seen:
		if got != "independent" {
			t.Fatal(got)
		}
	case <-time.After(time.Second):
		t.Fatal("other equipment blocked")
	}
	close(gate)
	for _, want := range []string{"first", "second"} {
		select {
		case got := <-seen:
			if got != want {
				t.Fatalf("%s != %s", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("ordered delivery timed out")
		}
	}
}
