package contract

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestTopicsAndControls(t *testing.T) {
	key := bytes.Repeat([]byte{42}, 32)
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	for n := 1; n <= 10; n++ {
		id := fmt.Sprintf("TEL-CVDTIN-08A-%02d", n)
		for _, kind := range []string{"NBIRTH", "NDEATH", "NDATA", "NCMD", "DBIRTH", "DDEATH", "DDATA", "DCMD"} {
			k, i, e := ParseTopic(Topic(kind, id))
			if e != nil || k != kind || i != id {
				t.Fatal(k, i, e)
			}
		}
		p := []byte(fmt.Sprintf(`{"equipmentId":%q,"commandId":"same-local-id","expiresAt":"2026-09-18T00:00:00Z"}`, id))
		raw, e := Sign(p, key)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = Verify(raw, key, id, now); e != nil {
			t.Fatal(e)
		}
		if _, e = Verify(raw, key, "wrong", now); e == nil {
			t.Fatal("wrong tool accepted")
		}
		if _, e = Verify(raw, key, id, now.Add(48*time.Hour)); e == nil {
			t.Fatal("expired accepted")
		}
		if _, e = Verify(raw, bytes.Repeat([]byte{43}, 32), id, now); e == nil {
			t.Fatal("forgery accepted")
		}
	}
	if _, _, e := ParseTopic("spBv1.0/cvd-fab/DDATA/edge-TEL-CVDTIN-08A-01/TEL-CVDTIN-08A-02"); e == nil {
		t.Fatal("topic mismatch")
	}
}

func TestMetrologyIsNotCVD(t *testing.T) {
	if ValidEquipment("KLA-SIM-01") || !ValidDevice("KLA-SIM-01") {
		t.Fatal("fleet identity")
	}
	if _, id, e := ParseTopic(Topic("DDATA", "KLA-SIM-01")); e != nil || id != "KLA-SIM-01" {
		t.Fatal(id, e)
	}
}
