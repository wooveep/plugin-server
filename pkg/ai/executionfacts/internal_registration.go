package executionfacts

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
)

const InternalRegistrationVersion = "execution-registration.v2"
const CallIdentitySourceInternal = "internal_execution"
const InternalCallReferenceHeader = "x-modelfusion-internal-call-ref"

// InternalRegistration is deliberately accountless. The receiver must already
// have the authenticated service/job CallContext under this original call_id.
// This encoder neither creates that authority nor fills a customer wallet.
type InternalRegistration struct {
	SchemaVersion        string          `json:"schema_version"`
	ProducerID           string          `json:"producer_id"`
	RegistrationID       string          `json:"registration_id"`
	CallID               string          `json:"call_id"`
	AttemptID            string          `json:"attempt_id"`
	ReleaseID            string          `json:"release_id"`
	RouteID              string          `json:"route_id"`
	TargetID             string          `json:"target_id"`
	APIKeyID             string          `json:"api_key_id"`
	CredentialGeneration string          `json:"credential_generation"`
	BusinessOccurredAt   string          `json:"business_occurred_at"`
	Selection            json.RawMessage `json:"selection"`
}

func FreezeInternalRegistration(input InternalRegistration) (Snapshot, error) {
	if err := validStrings(reflect.ValueOf(input)); err != nil {
		return Snapshot{}, err
	}
	r := input
	if r.SchemaVersion == "" {
		r.SchemaVersion = InternalRegistrationVersion
	}
	if r.SchemaVersion != InternalRegistrationVersion {
		return Snapshot{}, errors.New("internal registration requires its accountless v2 carrier")
	}
	for _, value := range []string{r.ProducerID, r.CallID, r.AttemptID, r.ReleaseID, r.RouteID, r.TargetID, r.APIKeyID} {
		if !id(value) {
			return Snapshot{}, errors.New("missing original internal registration identity")
		}
	}
	number, err := integer(r.CredentialGeneration, 63)
	if err != nil || number.Sign() == 0 || !timestamp(r.BusinessOccurredAt) {
		return Snapshot{}, errors.New("invalid original internal key generation or business time")
	}
	tree, err := decodeExact(r.Selection)
	if err != nil {
		return Snapshot{}, err
	}
	fields, ok := tree.(map[string]interface{})
	if !ok {
		return Snapshot{}, errors.New("internal registration requires its complete selection")
	}
	var canonicalSelection []byte
	switch fields["operation_kind"] {
	case "text_generate":
		canonicalSelection, err = CanonicalSelection(r.Selection)
	case NativeEmbeddingOperation:
		canonicalSelection, err = CanonicalEmbeddingSelection(r.Selection)
	default:
		err = errors.New("internal registration has no registered native operation")
	}
	if err != nil {
		return Snapshot{}, err
	}
	for field, want := range map[string]string{"call_id": r.CallID, "attempt_id": r.AttemptID, "release_id": r.ReleaseID, "target_id": r.TargetID} {
		if fields[field] != want {
			return Snapshot{}, errors.New("internal registration differs from the original selection")
		}
	}
	r.Selection = canonicalSelection
	if r.RegistrationID == "" {
		r.RegistrationID = stableID("registration", r.ProducerID, r.CallID, r.AttemptID)
	}
	if !id(r.RegistrationID) {
		return Snapshot{}, errors.New("invalid original internal registration key")
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return Snapshot{}, err
	}
	tree, err = decodeExact(raw)
	if err != nil {
		return Snapshot{}, err
	}
	wire, err := canonical(tree)
	if err != nil {
		return Snapshot{}, err
	}
	metadata := Envelope{ProducerID: r.ProducerID, EventID: r.RegistrationID, SourceFactID: r.RegistrationID, SourceRevision: "1", PayloadDigest: digestBytes(wire)}
	return Snapshot{envelope: metadata, wire: wire, business: wire, messageType: "execution_registration"}, nil
}

// InternalCallReference is request-time lookup material from the real key-auth
// filter plus the server-prepared job request. It is not a cost-owner assertion.
type InternalCallReference struct {
	CallID               string
	APIKeyID             string
	CredentialGeneration string
}

func ReadInternalCallReference(headers [][2]string) (InternalCallReference, error) {
	values := map[string]string{}
	for _, header := range headers {
		name := strings.ToLower(header[0])
		if name != InternalCallReferenceHeader && name != balanceidentity.APIKeyHeader && name != balanceidentity.GenerationHeader {
			continue
		}
		if _, duplicate := values[name]; duplicate {
			return InternalCallReference{}, errors.New("duplicate internal call reference or authenticated key header")
		}
		values[name] = header[1]
	}
	result := InternalCallReference{values[InternalCallReferenceHeader], values[balanceidentity.APIKeyHeader], values[balanceidentity.GenerationHeader]}
	number, err := strconv.ParseInt(result.CredentialGeneration, 10, 64)
	if !balanceidentity.ValidID(result.CallID) || !balanceidentity.ValidID(result.APIKeyID) || err != nil || number < 1 || strconv.FormatInt(number, 10) != result.CredentialGeneration {
		return InternalCallReference{}, errors.New("original internal call and authenticated service key are required")
	}
	return result, nil
}
