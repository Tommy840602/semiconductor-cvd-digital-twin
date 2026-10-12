package main

import (
	"context"
	"encoding/json"
	"github.com/gopcua/opcua/ua"
	"os"
	"testing"
	"time"
)

func TestOPCContract(t *testing.T) {
	endpoint := os.Getenv("OPC_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("OPC_TEST_ENDPOINT required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, err := newClient(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	ns, err := c.FindNamespace(ctx, "urn:cvd:model")
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.Read(ctx, &ua.ReadRequest{NodesToRead: []*ua.ReadValueID{{NodeID: ua.NewStringNodeID(ns, "Snapshot"), AttributeID: ua.AttributeIDValue}}})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err = json.Unmarshal([]byte(r.Results[0].Value.Value().(string)), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot["locks"].([]any)) != 3 {
		t.Fatal(snapshot)
	}
	command := map[string]any{"commandId": "opc-test-" + time.Now().Format("150405.000"), "type": "configure", "parameters": map[string]any{"processSeconds": 10}, "expiresAt": time.Now().Add(time.Minute)}
	b, _ := json.Marshal(command)
	out, err := c.Call(ctx, &ua.CallMethodRequest{ObjectID: ua.NewNumericNodeID(0, 85), MethodID: ua.NewStringNodeID(ns, "Command"), InputArguments: []*ua.Variant{ua.MustVariant(string(b))}})
	if err != nil || out.StatusCode != ua.StatusOK {
		t.Fatal(out, err)
	}
	if len(out.OutputArguments) != 1 {
		t.Fatal(out)
	}
	t.Log(out.OutputArguments[0].Value())
}
