// Package mediaexecution prepares exact original-account image and video HTTP work.
// It performs no network, Consume, authorization, fact delivery, or settlement.
// Private Console replies must already be authenticated and committed by the
// adapter. This package must never be mounted as a public body-selected sender.
package mediaexecution

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
)

const (
	Profile    = "qwen_image30_native_async"
	Model      = "qwen-image-3.0"
	CreatePath = "/api/v1/services/aigc/image-generation/generation"
)

var (
	ErrUnsupported = errors.New("media original native profile is unsupported")
	ErrIdentity    = errors.New("media original execution identity mismatch")
	ErrNoGrant     = errors.New("media first committed send permission is absent")
	// ErrProjectionUnready confirms that the original ticket remains unconsumed.
	ErrProjectionUnready = errors.New("media balance projection is not ready")
	ErrReplay            = errors.New("media send permission has already been taken")
	ErrExpired           = errors.New("media original send permission expired")
	ErrCapacity          = errors.New("media live query replay-guard capacity exceeded")
)

// GatewayIdentity comes from the authenticated service registration and actual
// live process, never a client header or the JSON being decoded. Construction of
// this value is not authentication; Console remains the Consume/query authority.
type GatewayIdentity struct{ ProducerID, Actor, BootID string }

// ProfileBinding names the exact retained adapter implementation supported by
// this leaf. The parent resolves these existing references from its retained
// capability directory, independently of the private reply being checked. Do
// not construct it by copying arbitrary descriptor revisions from a client.
// This is not a new configuration dialect or an authorization token.
type ProfileBinding struct {
	AdapterRef, AdapterRevision                                 string
	CreateClassifierRef, QueryClassifierRef, QueryCapabilityRef string
	OutputIdentityContract                                      string
}

// Invocation identifies the actual live Context that made Consume. It cannot
// be reconstructed on a restart to reuse an old grant.
type Invocation struct{ ID, Generation string }

type dispatchWire struct {
	ID          string          `json:"dispatch_id"`
	AttemptID   string          `json:"attempt_id"`
	OperationID string          `json:"operation_id"`
	Revision    string          `json:"revision"`
	State       string          `json:"state"`
	Deadline    time.Time       `json:"deadline"`
	Candidate   json.RawMessage `json:"candidate"`
}
type archiveWire struct {
	Dispatch               dispatchWire          `json:"dispatch"`
	Descriptor             json.RawMessage       `json:"descriptor"`
	Profile                string                `json:"profile"`
	Body                   string                `json:"request_body"`
	Endpoint               string                `json:"provider_endpoint"`
	Native                 json.RawMessage       `json:"native_execution"`
	History                json.RawMessage       `json:"history"`
	Runtime                string                `json:"runtime_config_id"`
	Revision               string                `json:"config_revision"`
	OutputIdentityContract string                `json:"output_identity_contract"`
	Estimation             UsageEstimationPolicy `json:"usage_estimation"`
	Procurement            procurementWire       `json:"procurement"`
}
type procurementWire struct {
	Identity       json.RawMessage `json:"identity"`
	Writer         writerWire      `json:"writer"`
	PricingAssetID json.RawMessage `json:"pricing_asset_id"`
	CostOwner      json.RawMessage `json:"cost_owner"`
}
type writerWire struct {
	ProducerID string `json:"producer_id"`
	Role       string `json:"role"`
}
type historyWire struct {
	CallID         string          `json:"call_id"`
	OperationID    string          `json:"operation_id"`
	ReleaseID      string          `json:"release_id"`
	RouteID        string          `json:"route_id"`
	Kind           string          `json:"operation_kind"`
	PublicModel    string          `json:"public_model"`
	OccurredAt     string          `json:"business_occurred_at"`
	Caller         json.RawMessage `json:"caller"`
	DeliveryWriter writerWire      `json:"delivery_writer"`
	Attempt        *attemptWire    `json:"attempt"`
}
type attemptWire struct {
	ID                 string     `json:"attempt_id"`
	TargetID           string     `json:"target_id"`
	SelectionDigest    string     `json:"selection_digest"`
	Transport          string     `json:"send_transport"`
	Owner              string     `json:"recording_owner"`
	Mechanism          string     `json:"mechanism"`
	CredentialRef      string     `json:"credential_ref"`
	CredentialRevision string     `json:"credential_revision"`
	TaskID             string     `json:"provider_task_id"`
	Writer             writerWire `json:"writer"`
	Procurement        struct {
		BindingID  string `json:"binding_id"`
		ProviderID string `json:"provider_id"`
		Account    string `json:"purchase_account_id"`
		Region     string `json:"region"`
		Model      string `json:"upstream_model_name"`
	} `json:"procurement"`
}

// Executor owns one original dispatch's local handoff guards. Keep one instance
// for its live Context; a restart must read original Console state, whose replay
// has no grant. Local guards do not replace the durable server authority.
type Executor struct {
	mu                               sync.Mutex
	original                         archiveWire
	descriptor, candidate            map[string]json.RawMessage
	history                          historyWire
	gateway                          GatewayIdentity
	basis                            bodybasis.Snapshot
	body                             []byte
	secret, authority                string
	timeoutMS                        uint32
	profile                          profileDefinition
	estimation                       UsageEstimationPolicy
	createTaken                      bool
	outputContract, createClassifier string
	queries                          map[string]struct{}
}

// Request is an immutable prepared handoff. Only the actual AiProxy instrumentor
// may send it, exactly once. It carries the original create history separately
// from the actual HTTP attempt, including for read-only queries.
type Request struct {
	outputContract, createClassifier                        string
	method, endpoint, path, authority, secret               string
	body                                                    []byte
	operationID, callID, attemptID, createAttemptID, taskID string
	history, selection                                      []byte
	descriptor                                              []byte
	timeoutMS                                               uint32
	query                                                   bool
	profile                                                 profileDefinition
	estimation                                              UsageEstimationPolicy
}

func (r Request) String() string              { return "mediaexecution.Request(redacted)" }
func (e *Executor) String() string            { return "mediaexecution.Executor(redacted)" }
func (r Request) Method() string              { return r.method }
func (r Request) Endpoint() string            { return r.endpoint }
func (r Request) Path() string                { return r.path }
func (r Request) Body() []byte                { return append([]byte(nil), r.body...) }
func (r Request) AttemptID() string           { return r.attemptID }
func (r Request) CreateAttemptID() string     { return r.createAttemptID }
func (r Request) OperationID() string         { return r.operationID }
func (r Request) CallID() string              { return r.callID }
func (r Request) IsQuery() bool               { return r.query }
func (r Request) Profile() string             { return r.profile.name }
func (r Request) OriginalHistory() []byte     { return append([]byte(nil), r.history...) }
func (r Request) Selection() []byte           { return append([]byte(nil), r.selection...) }
func (r Request) Descriptor() []byte          { return append([]byte(nil), r.descriptor...) }
func (r Request) TimeoutMilliseconds() uint32 { return r.timeoutMS }
func (r Request) OutboundBodyDigest() string  { return bodybasis.DigestBytes(r.body) }
func (r Request) Headers() [][2]string {
	headers := [][2]string{{"Authorization", "Bearer " + r.secret}, {"Host", r.authority}, {"Accept", "application/json"}}
	if !r.query {
		headers = append(headers, [2]string{"Content-Type", "application/json"})
		if r.profile.asynchronous {
			headers = append(headers, [2]string{"X-DashScope-Async", "enable"})
		}
	}
	return headers
}

// FromPrivateArchive reads the existing GatewayExecutionPayload, not a new
// configuration dialect. Each image/video protocol requires its separately
// retained profile binding. Current provider lookups and text proxies are never
// substituted. Oversized private wires fail explicitly.
func FromPrivateArchive(raw []byte, gateway GatewayIdentity, supported ProfileBinding) (*Executor, error) {
	var original archiveWire
	if decode(raw, &original) != nil || !identity(gateway.ProducerID) ||
		!strings.HasPrefix(gateway.Actor, "service/") || !identity(gateway.Actor) || !identity(gateway.BootID) ||
		!identity(original.Runtime) || !decimalID(original.Revision) {
		return nil, ErrUnsupported
	}
	origin := original.Procurement
	procurement, err := object(origin.Identity)
	descriptor, descriptorError := object(original.Descriptor)
	var asset string
	if err != nil || descriptorError != nil || origin.Writer.ProducerID != gateway.ProducerID || origin.Writer.Role != "ai_proxy" ||
		!identityMatchesProcurement(procurement, descriptor) || json.Unmarshal(origin.PricingAssetID, &asset) != nil || !identity(asset) || len(origin.CostOwner) == 0 {
		return nil, ErrIdentity
	}
	profile, err := registeredProfile(original.Profile)
	if err != nil {
		return nil, err
	}
	if supported.OutputIdentityContract != "" && (supported.OutputIdentityContract != profile.outputIdentityContract() || original.OutputIdentityContract != supported.OutputIdentityContract) {
		return nil, ErrUnsupported
	}
	d, err := object(original.Descriptor)
	if err != nil || field(d, "config_canonicalization") != "mf-json.v1" || field(d, "upstream_model") != profile.model ||
		!hasDigest(original.Descriptor, "descriptor_digest") {
		return nil, ErrUnsupported
	}
	if !identity(supported.AdapterRef) || !identity(supported.AdapterRevision) ||
		!identity(supported.CreateClassifierRef) || (profile.asynchronous && (!identity(supported.QueryClassifierRef) || !identity(supported.QueryCapabilityRef))) ||
		field(d, "adapter_ref") != supported.AdapterRef || field(d, "adapter_revision") != supported.AdapterRevision {
		return nil, ErrUnsupported
	}
	candidate, err := object(original.Dispatch.Candidate)
	if err != nil || !identity(original.Dispatch.ID) || !identity(original.Dispatch.AttemptID) ||
		!identity(original.Dispatch.OperationID) || !decimalID(original.Dispatch.Revision) ||
		(original.Dispatch.State != "prepared" && original.Dispatch.State != "consumed") || original.Dispatch.Deadline.IsZero() {
		return nil, ErrIdentity
	}
	var native struct {
		ProducerID       string                `json:"producer_id"`
		Authority        string                `json:"authority"`
		Path             string                `json:"upstream_path"`
		TimeoutMS        uint32                `json:"timeout_ms"`
		InputProtocol    string                `json:"input_protocol"`
		UpstreamProtocol string                `json:"upstream_protocol"`
		Estimation       UsageEstimationPolicy `json:"usage_estimation"`
	}
	var h historyWire
	if decode(original.Native, &native) != nil || decode(original.History, &h) != nil ||
		native.ProducerID != gateway.ProducerID || native.Path != profile.path || native.TimeoutMS == 0 ||
		(native.InputProtocol != "" && native.InputProtocol != profile.name) || (native.UpstreamProtocol != "" && native.UpstreamProtocol != profile.name) ||
		!identity(native.Authority) {
		return nil, ErrUnsupported
	}
	original.Estimation = original.Estimation.resolved()
	if original.Estimation != native.Estimation.resolved() || original.Estimation.validate(profile) != nil {
		return nil, ErrUnsupported
	}
	transport, err := object(d["transport_contract"])
	u, endpointErr := url.Parse(original.Endpoint)
	if err != nil || endpointErr != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") ||
		field(transport, "host") != native.Authority ||
		(u.Scheme == "https" && field(transport, "sni") != u.Hostname()) ||
		(u.Scheme == "http" && field(transport, "sni") != "") {
		return nil, ErrUnsupported
	}
	if h.Kind != profile.operation || h.OperationID != original.Dispatch.OperationID ||
		h.ReleaseID != field(d, "release_id") || !identity(h.CallID) || !identity(h.RouteID) ||
		!identity(h.PublicModel) || !identity(field(d, "provider_account_id")) || !identity(field(d, "region_endpoint_ref")) {
		return nil, ErrIdentity
	}
	caller, err := object(h.Caller)
	if err != nil || field(caller, "billing_mode") != "customer" {
		return nil, ErrUnsupported
	}
	for _, key := range []string{"tenant_id", "principal_id", "account_id", "sales_asset_id"} {
		if !identity(field(caller, key)) {
			return nil, ErrIdentity
		}
	}
	for _, key := range []string{"release_id", "target_id", "binding_id", "provider_id", "provider_account_id", "upstream_model", "cluster_ref", "descriptor_digest", "mapper_digest", "proxy_digest"} {
		if !identity(field(d, key)) {
			return nil, ErrIdentity
		}
	}
	for _, key := range []string{"descriptor_digest", "mapper_digest", "proxy_digest"} {
		if !digestSyntax(field(d, key)) {
			return nil, ErrIdentity
		}
	}
	for _, key := range []string{"release_id", "target_id", "binding_id", "provider_account_id", "upstream_model", "cluster_ref", "mapper_digest", "proxy_digest"} {
		if field(candidate, key) != field(d, key) {
			return nil, ErrIdentity
		}
	}
	if !identity(field(candidate, "credential_ref")) || !decimalID(field(candidate, "credential_revision")) {
		return nil, ErrIdentity
	}
	secret, err := credentialForSelection(original.Native, field(candidate, "credential_ref"), field(candidate, "credential_revision"))
	if err != nil {
		return nil, err
	}
	capabilities, err := object(d["capabilities"])
	if err != nil {
		return nil, ErrUnsupported
	}
	var queryRef string
	kinds := []string{profile.operation}
	if profile.asynchronous {
		kinds = append(kinds, "task_query")
	} else if supported.QueryClassifierRef != "" || supported.QueryCapabilityRef != "" {
		return nil, ErrUnsupported
	}
	for _, kind := range kinds {
		cap, err := object(capabilities[kind])
		expectedClassifier := supported.CreateClassifierRef
		if kind == "task_query" {
			expectedClassifier = supported.QueryClassifierRef
		}
		if err != nil || field(cap, "send_mode") != "single_send" || string(cap["max_attempts"]) != "1" ||
			string(cap["max_fallback_hops"]) != "0" || field(cap, "response_classifier_ref") != expectedClassifier ||
			(profile.asynchronous && field(cap, "query_capability_ref") != supported.QueryCapabilityRef) {
			return nil, ErrUnsupported
		}
		if queryRef != "" && queryRef != field(cap, "query_capability_ref") {
			return nil, ErrIdentity
		}
		queryRef = field(cap, "query_capability_ref")
	}
	// The basis always describes the original platform bytes. A profile-specific
	// native body is frozen separately and becomes the actual outbound digest.
	body := []byte(original.Body)
	basis, err := bodybasis.Capture(body)
	if err != nil || basis.PublicModel() != h.PublicModel {
		return nil, ErrIdentity
	}
	if field(candidate, "original_body_digest") != basis.Digest() || field(candidate, "call_id") != h.CallID ||
		field(candidate, "operation_id") != h.OperationID || field(candidate, "operation_kind") != profile.operation ||
		field(candidate, "selection_generation") != original.Revision || field(candidate, "send_transport") != "http_call" ||
		field(candidate, "recording_owner") != "ai_proxy" || field(candidate, "mechanism") != "router" {
		return nil, ErrIdentity
	}
	rewritten, err := profile.prepareBody(body)
	if err != nil {
		return nil, ErrUnsupported
	}
	e := &Executor{original: original, descriptor: d, candidate: candidate, history: h, gateway: gateway,
		basis: basis, body: rewritten, secret: secret, authority: native.Authority,
		timeoutMS: native.TimeoutMS,
		profile:   profile, estimation: original.Estimation,
		outputContract: supported.OutputIdentityContract, createClassifier: supported.CreateClassifierRef,
		createTaken: original.Dispatch.State == "consumed", queries: make(map[string]struct{})}
	if h.Attempt != nil && e.checkHistory(original.History, "") != nil {
		return nil, ErrIdentity
	}
	return e, nil
}

// retainedCredential is a private member of the original compiled target pool.
// It is never a random-selection input or a public transport identity.
type retainedCredential struct {
	reference, revision, value string
}

func validCredentialValue(value string) bool {
	return value != "" && len(value) <= 8192 && strings.IndexFunc(value, func(r rune) bool { return r < 0x21 || r > 0x7e }) < 0
}

func credentialPool(raw []byte) ([]retainedCredential, error) {
	native, err := object(raw)
	if err != nil {
		return nil, ErrUnsupported
	}
	pool, plural := native["credentials"]
	legacy, scalar := native["credential_value"]
	if plural {
		if scalar {
			return nil, ErrUnsupported
		}
		var entries []json.RawMessage
		if decode(pool, &entries) != nil || len(entries) == 0 {
			return nil, ErrUnsupported
		}
		result := make([]retainedCredential, 0, len(entries))
		seen := make(map[string]bool, len(entries))
		for _, raw := range entries {
			entry, err := object(raw)
			member := retainedCredential{field(entry, "credential_ref"), field(entry, "credential_revision"), field(entry, "credential_value")}
			if err != nil || len(entry) != 3 || !identity(member.reference) || !decimalID(member.revision) || len(member.revision) > 200 || !validCredentialValue(member.value) || seen[member.reference] {
				return nil, ErrUnsupported
			}
			seen[member.reference] = true
			result = append(result, member)
		}
		return result, nil
	}
	var value string
	if !scalar || json.Unmarshal(legacy, &value) != nil || !validCredentialValue(value) {
		return nil, ErrUnsupported
	}
	// Original singleton archives contain one already-bound secret. They never
	// supply a seed/fallback for an explicit multiple-member target directory.
	return []retainedCredential{{value: value}}, nil
}

func credentialForSelection(raw []byte, reference, revision string) (string, error) {
	if !identity(reference) || !decimalID(revision) || len(revision) > 200 {
		return "", ErrIdentity
	}
	pool, err := credentialPool(raw)
	if err != nil {
		return "", err
	}
	if len(pool) == 1 && pool[0].reference == "" {
		return pool[0].value, nil
	}
	for _, member := range pool {
		if member.reference == reference && member.revision == revision {
			return member.value, nil
		}
	}
	return "", ErrIdentity
}

// ValidateCredentialConfiguration binds the original pool to its provider tokens.
// The caller supplies both from the same immutable compiled rule. This checks
// the full multiset, never picks a member or installs credentials, and returns
// no secret. Per-request sending resolves only the prepared ref/revision.
func ValidateCredentialConfiguration(raw []byte, providerTokens []string) error {
	pool, err := credentialPool(raw)
	if err != nil || len(pool) != len(providerTokens) {
		return ErrUnsupported
	}
	counts := make(map[string]int, len(pool))
	for _, member := range pool {
		counts[member.value]++
	}
	for _, value := range providerTokens {
		if counts[value] == 0 {
			return ErrUnsupported
		}
		counts[value]--
	}
	return nil
}

func decode(raw []byte, value any) error {
	// The RPC wraps an original body plus retained FACT bytes, so its transport
	// budget is larger than descriptor/body canonicalization. Reuse strict JSON
	// validation without changing any frozen mf-json digest or body basis budget.
	if err := mfjson.ValidateRuntimeBody(raw, 32<<20); err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}
func object(raw []byte) (map[string]json.RawMessage, error) {
	var value map[string]json.RawMessage
	if err := decode(raw, &value); err != nil || value == nil {
		return nil, ErrIdentity
	}
	return value, nil
}
func field(m map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(m[key], &value)
	return value
}
func identity(value string) bool {
	return value != "" && len(value) <= 256 && strings.TrimSpace(value) == value &&
		strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) < 0
}
func decimalID(value string) bool {
	return value != "" && value[0] >= '1' && value[0] <= '9' &&
		strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) < 0
}
func digestSyntax(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, c := range value[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func hasDigest(raw []byte, key string) bool {
	m, err := object(raw)
	if err != nil {
		return false
	}
	expected := field(m, key)
	delete(m, key)
	projection, err := json.Marshal(m)
	if err != nil {
		return false
	}
	digest, err := mfjson.Digest(projection)
	return err == nil && expected == digest
}
func equalJSON(a, b []byte) bool {
	aCanon, aErr := mfjson.Canonicalize(a)
	bCanon, bErr := mfjson.Canonicalize(b)
	return aErr == nil && bErr == nil && string(aCanon) == string(bCanon)
}
