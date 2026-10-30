package executionfacts

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/serviceidentity"
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
	Identity             serviceidentity.Snapshot
}

func ReadInternalCallReference(headers [][2]string) (InternalCallReference, error) {
	identity, err := serviceidentity.Read(headers)
	if err != nil {
		return InternalCallReference{}, err
	}
	var call string
	found := false
	for _, pair := range headers {
		if !strings.EqualFold(pair[0], InternalCallReferenceHeader) {
			continue
		}
		if found {
			return InternalCallReference{}, errors.New("duplicate original internal call reference")
		}
		found, call = true, pair[1]
	}
	if !found || !balanceidentity.ValidID(call) {
		return InternalCallReference{}, errors.New("original server-prepared internal call is required")
	}
	return InternalCallReference{CallID: call, APIKeyID: identity.APIKeyID, CredentialGeneration: identity.CredentialGeneration, Identity: identity}, nil
}
