package executionfacts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
)

// DeliveryBridgeHeader is private response metadata between the released proxy
// and billing filters. It is removed before returning the response to a client.
const DeliveryBridgeHeader = "x-modelfusion-delivery-observation"

// MaxDeliveryBridgeBytes bounds raw response metadata, not the frozen IF03 schema.
// Its base64url representation leaves room within the managed header-map budget.
const MaxDeliveryBridgeBytes = 32 * 1024
const MaxDeliveryBridgeHeaderBytes = (MaxDeliveryBridgeBytes*8 + 5) / 6

// EncodeDeliveryBridge carries the exact frozen delivery without creating another
// event, source revision, timestamp, or usage measurement. Encoding is not trust:
// the released proxy overwrites upstream headers and billing verifies ownership.
func EncodeDeliveryBridge(snapshot Snapshot) (string, error) {
	if snapshot.MessageType() != "observation" || snapshot.envelope.Kind != "call_delivery" {
		return "", errors.New("delivery bridge requires a frozen call delivery")
	}
	raw := snapshot.Bytes()
	checked, err := decodeDeliveryBridgeBytes(raw)
	if err != nil || !bytes.Equal(snapshot.BusinessBytes(), checked.BusinessBytes()) {
		return "", errors.New("delivery bridge snapshot is invalid or oversized")
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// DecodeDeliveryBridge accepts one canonical base64url value and reconstructs the
// same immutable snapshot. Callers must independently check their configured
// producer, original request identity, and released route/model ownership.
func DecodeDeliveryBridge(value string) (Snapshot, error) {
	if value == "" || len(value) > MaxDeliveryBridgeHeaderBytes {
		return Snapshot{}, errors.New("delivery bridge is empty or oversized")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(raw) != value {
		return Snapshot{}, errors.New("delivery bridge requires canonical unpadded base64url")
	}
	return decodeDeliveryBridgeBytes(raw)
}

func decodeDeliveryBridgeBytes(raw []byte) (Snapshot, error) {
	return decodeVersionedDeliveryBridge(raw, SchemaVersion, Freeze)
}

func decodeVersionedDeliveryBridge(raw []byte, version string, freeze func(Envelope) (Snapshot, error)) (Snapshot, error) {
	if len(raw) == 0 || len(raw) > MaxDeliveryBridgeBytes {
		return Snapshot{}, errors.New("delivery bridge observation exceeds its transport budget")
	}
	// Exact decoding rejects duplicate keys and unpaired Unicode surrogates before
	// encoding/json can erase or replace the original evidence.
	if _, err := decodeExact(raw); err != nil {
		return Snapshot{}, errors.New("delivery bridge has invalid exact JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var envelope Envelope
	if err := decoder.Decode(&envelope); err != nil {
		return Snapshot{}, errors.New("delivery bridge has invalid observation fields")
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return Snapshot{}, errors.New("delivery bridge has trailing observation bytes")
	}
	if envelope.Kind != "call_delivery" || envelope.SchemaVersion != version || envelope.PayloadCanonicalization != Canonicalization {
		return Snapshot{}, errors.New("delivery bridge has a different frozen observation kind or version")
	}
	snapshot, err := freeze(envelope)
	if err != nil || !bytes.Equal(snapshot.Bytes(), raw) {
		// Freeze may default absent source keys or normalize an empty collection.
		// Byte equality prevents such reconstruction from being accepted as a bridge.
		return Snapshot{}, errors.New("delivery bridge must preserve complete canonical observation bytes")
	}
	return snapshot, nil
}

// Embedding bridges require the separately registered native:v2 codec. The
// original v1 decoder remains strict and cannot accept this new operation.
func EncodeEmbeddingDeliveryBridge(snapshot Snapshot) (string, error) {
	if snapshot.MessageType() != "observation" || snapshot.envelope.Kind != "call_delivery" {
		return "", errors.New("embedding bridge requires a frozen call delivery")
	}
	raw := snapshot.Bytes()
	checked, err := decodeVersionedDeliveryBridge(raw, NativeEmbeddingSchemaVersion, FreezeEmbedding)
	if err != nil || !bytes.Equal(snapshot.BusinessBytes(), checked.BusinessBytes()) {
		return "", errors.New("embedding delivery bridge snapshot is invalid or oversized")
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeEmbeddingDeliveryBridge(value string) (Snapshot, error) {
	if value == "" || len(value) > MaxDeliveryBridgeHeaderBytes {
		return Snapshot{}, errors.New("embedding delivery bridge is empty or oversized")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(raw) != value {
		return Snapshot{}, errors.New("embedding bridge requires canonical unpadded base64url")
	}
	return decodeVersionedDeliveryBridge(raw, NativeEmbeddingSchemaVersion, FreezeEmbedding)
}
