package executionfacts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestDeliveryBridgePreservesFrozenBytesKeysAndExactCounts(t *testing.T) {
	envelope := deliveryFixture()
	envelope.Usage.Quantities = map[string]string{"input": "9007199254740993", "output": "0"}
	snapshot := mustFreeze(t, envelope)
	header, err := EncodeDeliveryBridge(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeDeliveryBridge(header)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.Bytes(), snapshot.Bytes()) || !bytes.Equal(decoded.BusinessBytes(), snapshot.BusinessBytes()) || decoded.EventID() != snapshot.EventID() || decoded.SourceFactID() != snapshot.SourceFactID() || decoded.Revision() != snapshot.Revision() || decoded.Digest() != snapshot.Digest() {
		t.Fatal("bridge changed the original delivery identity or evidence")
	}
	if decoded.Envelope().Usage.Quantities["input"] != "9007199254740993" || strings.Contains(header, "=") {
		t.Fatal("bridge rounded counts or used padded base64")
	}
	if _, err := NewDelivery(decoded); err != nil {
		t.Fatal("decoded bridge cannot enter the existing ACK ledger")
	}
}

func TestDeliveryBridgeRejectsOtherKindsNoncanonicalAndAmbiguousEvidence(t *testing.T) {
	if _, err := EncodeDeliveryBridge(mustFreeze(t, attemptFixture())); err == nil {
		t.Fatal("attempt was accepted as final delivery")
	}
	snapshot := mustFreeze(t, deliveryFixture())
	header, err := EncodeDeliveryBridge(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", header + "=", header + "\n", " " + header, strings.Repeat("a", MaxDeliveryBridgeHeaderBytes+1)} {
		if _, err := DecodeDeliveryBridge(invalid); err == nil {
			t.Fatal("invalid bridge encoding was accepted")
		}
	}
	for _, raw := range [][]byte{
		append([]byte(" "), snapshot.Bytes()...),
		bytes.Replace(snapshot.Bytes(), []byte(`"kind":"call_delivery"`), []byte(`"kind":"call_delivery","kind":"call_delivery"`), 1),
		bytes.Replace(snapshot.Bytes(), []byte(`"call_id":"call-1"`), []byte(`"call_id":"another-call"`), 1),
		append(snapshot.Bytes(), []byte(`{}`)...),
	} {
		if _, err := DecodeDeliveryBridge(base64.RawURLEncoding.EncodeToString(raw)); err == nil {
			t.Fatal("noncanonical, duplicate, changed-digest, or trailing bridge was accepted")
		}
	}
	var body map[string]interface{}
	if err := json.Unmarshal(snapshot.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	delete(body, "event_id")
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeDeliveryBridge(base64.RawURLEncoding.EncodeToString(raw)); err == nil {
		t.Fatal("bridge reconstructed a missing original event identity")
	}
}

func TestOversizedDeliveryBridgeIsAnExplicitTransportGap(t *testing.T) {
	envelope := deliveryFixture()
	envelope.EvidenceRefs = make([]string, 200)
	for index := range envelope.EvidenceRefs {
		envelope.EvidenceRefs[index] = strings.Repeat("e", 200) + strconv.Itoa(index)
	}
	snapshot := mustFreeze(t, envelope)
	if _, err := EncodeDeliveryBridge(snapshot); err == nil {
		t.Fatal("oversized response metadata was marked delivered")
	}
	if _, err := DecodeDeliveryBridge(base64.RawURLEncoding.EncodeToString(snapshot.Bytes())); err == nil {
		t.Fatal("oversized raw delivery escaped the transport budget")
	}
}
