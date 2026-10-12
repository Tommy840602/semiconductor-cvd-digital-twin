package gemlink

import (
 "bytes"
 "testing"
 "github.com/arloliu/go-secs/v2/secs2"
)
func TestSnapshotAndEventsPreserveExactIntegers(t *testing.T) {
 raw:=[]byte(`{"equipmentId":"TEL-CVDTIN-08A-01","sequence":9007199254740993,"events":[{"sequence":9007199254740993}],"chambers":[{"temperature":400}]}`)
 d:=&Device{ID:"TEL-CVDTIN-08A-01"}
 values:=d.valuesFrom([]uint64{1000,1001},raw)
 snapshot,err:=secs2.NewCursor(values).At(0).Binary();if err!=nil||!bytes.Equal(snapshot,raw){t.Fatalf("snapshot changed: %s %v",snapshot,err)}
 events,err:=secs2.NewCursor(values).At(1).Binary();if err!=nil||string(events)!=`[{"sequence":9007199254740993}]`{t.Fatalf("events lost precision: %s %v",events,err)}
}
