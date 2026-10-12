package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/awcullen/opcua/server"
	"github.com/awcullen/opcua/ua"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func certificate(dir string) error {
	if _, err := os.Stat(dir + "/cert.pem"); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	u, _ := url.Parse("urn:cvd:simulator")
	cert := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "cvd-simulator"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(2, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, DNSNames: []string{"simulator", "localhost"}, URIs: []*url.URL{u}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		return err
	}
	if err = os.WriteFile(dir+"/cert.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		return err
	}
	return os.WriteFile(dir+"/key.pem", pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600)
}
func main() {
	if env("CVD_PROTOCOL", "opcua") == "secs" {
		runSECS()
		return
	}
	dir := env("DATA_DIR", ".runtime")
	if err := certificate(dir); err != nil {
		log.Fatal(err)
	}
	e, err := Load(dir + "/state.json")
	if err != nil {
		log.Fatal(err)
	}
	id := env("EQUIPMENT_ID", "TEL-CVDTIN-08A-01")
	if !validEquipment(id) {
		log.Fatal("invalid EQUIPMENT_ID")
	}
	if e.EquipmentID != "" && e.EquipmentID != id {
		log.Fatal("state belongs to another equipment")
	}
	e.EquipmentID = id
	var mu sync.Mutex
	srv, err := server.New(ua.ApplicationDescription{ApplicationURI: "urn:cvd:simulator", ApplicationName: ua.LocalizedText{Text: "TiN CVD Simulator"}, ApplicationType: ua.ApplicationTypeServer}, dir+"/cert.pem", dir+"/key.pem", env("OPC_ENDPOINT", "opc.tcp://0.0.0.0:4840"), server.WithSecurityPolicyNone(true), server.WithAnonymousIdentity(true))
	if err != nil {
		log.Fatal(err)
	}
	nm := srv.NamespaceManager()
	ns := nm.Add("urn:cvd:model")
	log.Printf("CVD namespace=%d", ns)
	refs := []ua.Reference{{ReferenceTypeID: ua.ReferenceTypeIDOrganizes, IsInverse: true, TargetID: ua.NewExpandedNodeID(ua.ObjectIDObjectsFolder)}}
	makeVar := func(name string) *server.VariableNode {
		n := server.NewVariableNode(srv, ua.NewNodeIDString(ns, name), ua.NewQualifiedName(ns, name), ua.LocalizedText{Text: name}, ua.LocalizedText{Text: name}, nil, refs, ua.DataValue{Value: "{}"}, ua.DataTypeIDString, ua.ValueRankScalar, nil, ua.AccessLevelsCurrentRead, 100, false, nil)
		if err := nm.AddNodes(n); err != nil {
			log.Fatal(err)
		}
		return n
	}
	snap := makeVar("Snapshot")
	events := makeVar("Events")
	// One JSON input/output keeps the command envelope identical across Java and Go.
	method := server.NewMethodNode(srv, ua.NewNodeIDString(ns, "Command"), ua.NewQualifiedName(ns, "Command"), ua.LocalizedText{Text: "Command"}, ua.LocalizedText{Text: "Execute simulation command"}, []ua.RolePermissionType{{RoleID: ua.ObjectIDWellKnownRoleAnonymous, Permissions: ua.PermissionTypeBrowse | ua.PermissionTypeCall}}, []ua.Reference{{ReferenceTypeID: ua.ReferenceTypeIDHasComponent, IsInverse: true, TargetID: ua.NewExpandedNodeID(ua.ObjectIDObjectsFolder)}}, true)
	method.SetCallMethodHandler(func(_ *server.Session, r ua.CallMethodRequest) ua.CallMethodResult {
		mu.Lock()
		defer mu.Unlock()
		if len(r.InputArguments) != 1 {
			return ua.CallMethodResult{StatusCode: ua.BadArgumentsMissing}
		}
		s, ok := r.InputArguments[0].(string)
		if !ok {
			return ua.CallMethodResult{StatusCode: ua.BadTypeMismatch}
		}
		var c Command
		if json.Unmarshal([]byte(s), &c) != nil {
			return ua.CallMethodResult{StatusCode: ua.BadInvalidArgument}
		}
		out := e.Execute(c, time.Now())
		if err := e.Save(dir + "/state.json"); err != nil {
			log.Fatal(err)
		}
		b, _ := json.Marshal(out)
		return ua.CallMethodResult{OutputArguments: []ua.Variant{string(b)}}
	})
	if err := nm.AddNodes(method); err != nil {
		log.Fatal(err)
	}
	ack := server.NewMethodNode(srv, ua.NewNodeIDString(ns, "Ack"), ua.NewQualifiedName(ns, "Ack"), ua.LocalizedText{Text: "Ack"}, ua.LocalizedText{}, []ua.RolePermissionType{{RoleID: ua.ObjectIDWellKnownRoleAnonymous, Permissions: ua.PermissionTypeBrowse | ua.PermissionTypeCall}}, []ua.Reference{{ReferenceTypeID: ua.ReferenceTypeIDHasComponent, IsInverse: true, TargetID: ua.NewExpandedNodeID(ua.ObjectIDObjectsFolder)}}, true)
	ack.SetCallMethodHandler(func(_ *server.Session, r ua.CallMethodRequest) ua.CallMethodResult {
		mu.Lock()
		defer mu.Unlock()
		if len(r.InputArguments) != 1 {
			return ua.CallMethodResult{StatusCode: ua.BadArgumentsMissing}
		}
		seq, ok := r.InputArguments[0].(int64)
		if !ok {
			return ua.CallMethodResult{StatusCode: ua.BadTypeMismatch}
		}
		kept := e.Events[:0]
		for _, v := range e.Events {
			if v.Sequence > seq {
				kept = append(kept, v)
			}
		}
		e.Events = kept
		if err := e.Save(dir + "/state.json"); err != nil {
			log.Fatal(err)
		}
		return ua.CallMethodResult{}
	})
	if err := nm.AddNodes(ack); err != nil {
		log.Fatal(err)
	}
	publish := func() {
		b, _ := json.Marshal(e)
		now := time.Now().UTC()
		snap.SetValue(ua.NewDataValue(string(b), 0, now, 0, now, 0))
		b, _ = json.Marshal(e.Events)
		events.SetValue(ua.NewDataValue(string(b), 0, now, 0, now, 0))
	}
	publish()
	go func() {
		for now := range time.NewTicker(time.Second).C {
			mu.Lock()
			e.EnforceBuffer(now)
			e.Tick(now)
			if err := e.Save(dir + "/state.json"); err != nil {
				log.Fatal(err)
			}
			publish()
			mu.Unlock()
		}
	}()
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") })
	go func() { log.Print(http.ListenAndServe(":8081", nil)) }()
	log.Fatal(srv.ListenAndServe())
}
