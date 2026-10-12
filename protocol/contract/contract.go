// Package contract contains the versioned CVD application extension carried by SECS and Sparkplug.
package contract

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const Version = "cvd-secs-sparkplug-v1"
const Group = "cvd-fab"
const Host = "cvd-data-platform"

func ValidEquipment(id string) bool {
	for n := 1; n <= 10; n++ {
		if id == fmt.Sprintf("TEL-CVDTIN-08A-%02d", n) {
			return true
		}
	}
	return false
}
func ValidDevice(id string) bool { return ValidEquipment(id) || id == "KLA-SIM-01" }
func Node(id string) string      { return "edge-" + id }
func Topic(kind, id string) string {
	p := "spBv1.0/" + Group + "/" + kind + "/" + Node(id)
	if strings.HasPrefix(kind, "D") {
		p += "/" + id
	}
	return p
}
func ParseTopic(topic string) (kind, id string, err error) {
	p := strings.Split(topic, "/")
	if len(p) < 4 || p[0] != "spBv1.0" || p[1] != Group {
		return "", "", errors.New("invalid namespace")
	}
	kind = p[2]
	id = strings.TrimPrefix(p[3], "edge-")
	if !ValidDevice(id) || p[3] != Node(id) {
		return "", "", errors.New("invalid node")
	}
	switch kind {
	case "NBIRTH", "NDEATH", "NDATA", "NCMD":
		if len(p) != 4 {
			return "", "", errors.New("invalid node topic")
		}
	case "DBIRTH", "DDEATH", "DDATA", "DCMD":
		if len(p) != 5 || p[4] != id {
			return "", "", errors.New("device mismatch")
		}
	default:
		return "", "", errors.New("unknown message type")
	}
	return
}

type SignedCommand struct {
	Schema    string `json:"schemaVersion"`
	Payload   []byte `json:"payload"`
	Signature string `json:"signature"`
}

func Sign(payload []byte, key []byte) ([]byte, error) {
	if len(key) < 32 {
		return nil, errors.New("control key too short")
	}
	if !json.Valid(payload) {
		return nil, errors.New("invalid JSON payload")
	}
	h := hmac.New(sha256.New, key)
	h.Write([]byte(Version + "\n"))
	h.Write(payload)
	return json.Marshal(SignedCommand{Version, payload, hex.EncodeToString(h.Sum(nil))})
}
func Verify(raw, key []byte, id string, now time.Time) (json.RawMessage, error) {
	var c SignedCommand
	if len(key) < 32 || json.Unmarshal(raw, &c) != nil || c.Schema != Version {
		return nil, errors.New("invalid signed control")
	}
	h := hmac.New(sha256.New, key)
	h.Write([]byte(Version + "\n"))
	h.Write(c.Payload)
	sig, e := hex.DecodeString(c.Signature)
	if e != nil || !hmac.Equal(sig, h.Sum(nil)) {
		return nil, errors.New("invalid control signature")
	}
	var p struct {
		Equipment string    `json:"equipmentId"`
		ID        string    `json:"commandId"`
		Expires   time.Time `json:"expiresAt"`
	}
	if json.Unmarshal(c.Payload, &p) != nil || !ValidDevice(p.Equipment) || p.Equipment != id || p.ID == "" || !p.Expires.After(now) {
		return nil, errors.New("wrong equipment or expired command")
	}
	return c.Payload, nil
}
