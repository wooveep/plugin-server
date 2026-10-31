package mediaexecution

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/tidwall/resp"
)

// PrepareRedisCreate uses the first original-ticket Redis consumption. It
// preserves the same frozen transport/body and seals only the actual live
// invocation. Replayed/lost Redis replies cannot grant a provider creation.
func (e *Executor) PrepareRedisCreate(reply resp.Value, actualSelection []byte, live Invocation, now time.Time) (Request, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.createTaken {
		return Request{}, ErrReplay
	}
	e.createTaken = true
	if !firstRedisPermission(reply) {
		return Request{}, ErrNoGrant
	}
	selection, err := e.selection(actualSelection, false)
	if err != nil || field(selection, "gateway_boot_id") != e.gateway.BootID ||
		field(selection, "invocation_id") != live.ID || field(selection, "context_generation") != live.Generation ||
		field(selection, "dispatch_id") != e.original.Dispatch.ID || field(selection, "attempt_id") != e.original.Dispatch.AttemptID {
		return Request{}, ErrIdentity
	}
	if now.IsZero() || !now.Before(e.original.Dispatch.Deadline) {
		return Request{}, ErrExpired
	}
	origin := e.original.Procurement
	history, err := object(e.original.History)
	if err != nil {
		return Request{}, ErrIdentity
	}
	attempt := map[string]json.RawMessage{
		"procurement":       origin.Identity,
		"parent_attempt_id": json.RawMessage("null"),
		"provider_task_id":  json.RawMessage("null"),
		"pricing_asset_id":  origin.PricingAssetID,
		"cost_owner":        origin.CostOwner,
	}
	if len(attempt["pricing_asset_id"]) == 0 {
		attempt["pricing_asset_id"] = json.RawMessage("null")
	}
	for name, value := range map[string]string{
		"attempt_id": e.original.Dispatch.AttemptID, "target_id": field(e.descriptor, "target_id"),
		"selection_digest": field(selection, "selection_digest"), "send_transport": "http_call", "recording_owner": "ai_proxy", "mechanism": "router",
		"credential_ref": field(e.candidate, "credential_ref"), "credential_revision": field(e.candidate, "credential_revision"),
	} {
		attempt[name], _ = json.Marshal(value)
	}
	attempt["writer"], _ = json.Marshal(origin.Writer)
	history["attempt"], err = json.Marshal(attempt)
	if err != nil {
		return Request{}, ErrIdentity
	}
	sealed, err := json.Marshal(history)
	if err != nil || e.checkHistory(sealed, field(selection, "selection_digest")) != nil {
		return Request{}, ErrIdentity
	}
	return e.request("POST", e.profile.path, e.original.Dispatch.AttemptID, "", sealed, actualSelection), nil
}

// PrepareRedisQuery uses the original-account read ticket independently of
// generation balance and never changes the generation's sealed liability.
func (e *Executor) PrepareRedisQuery(reply resp.Value, actualSelection []byte, expiresAt, now time.Time) (Request, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !firstRedisPermission(reply) || !e.profile.asynchronous {
		return Request{}, ErrNoGrant
	}
	selection, err := e.selection(actualSelection, true)
	if err != nil || field(selection, "gateway_boot_id") != e.gateway.BootID || e.history.Attempt == nil ||
		!taskID(e.history.Attempt.TaskID) || e.checkHistory(e.original.History, "") != nil {
		return Request{}, ErrIdentity
	}
	attempt := field(selection, "attempt_id")
	if _, exists := e.queries[attempt]; exists {
		return Request{}, ErrReplay
	}
	if now.IsZero() || !now.Before(expiresAt) {
		return Request{}, ErrExpired
	}
	if len(e.queries) >= 10_000 {
		return Request{}, ErrCapacity
	}
	e.queries[attempt] = struct{}{}
	return e.request("GET", "/api/v1/tasks/"+e.history.Attempt.TaskID, attempt, e.history.Attempt.TaskID, e.original.History, actualSelection), nil
}

func firstRedisPermission(value resp.Value) bool {
	if value.Error() != nil || value.Type() != resp.Array {
		return false
	}
	parts := value.Array()
	if len(parts) != 2 || parts[0].String() != "allow" {
		return false
	}
	first, second, found := strings.Cut(parts[1].String(), "-")
	if !found || first == "" || second == "" {
		return false
	}
	_, firstError := strconv.ParseUint(first, 10, 64)
	_, secondError := strconv.ParseUint(second, 10, 64)
	return firstError == nil && secondError == nil
}

func identityMatchesProcurement(procurement, descriptor map[string]json.RawMessage) bool {
	return field(procurement, "binding_id") == field(descriptor, "binding_id") &&
		field(procurement, "provider_id") == field(descriptor, "provider_id") &&
		field(procurement, "purchase_account_id") == field(descriptor, "provider_account_id") &&
		field(procurement, "upstream_model_name") == field(descriptor, "upstream_model") && identity(field(procurement, "region"))
}

// ConsumeUnknown permanently disables creation in this live executor. A lost
// Consume/COMMIT response must be resolved by reading the original dispatch;
// a later consumed state is not another send grant. It creates no provider fact.
func (e *Executor) ConsumeUnknown() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.createTaken = true
}

// PrepareCreate accepts only the first committed private Consume response for
// the actual live Context. It does not call Consume. Once returned, the actual
// AiProxy sender must mark handoff before its host call, retain unknown failures,
// and never reconstruct the executor/permission to retry generation.
func (e *Executor) PrepareCreate(committedReply, actualSelection []byte, live Invocation, now time.Time) (Request, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.createTaken {
		return Request{}, ErrReplay
	}
	var reply struct {
		Dispatch dispatchWire    `json:"dispatch"`
		Grant    json.RawMessage `json:"grant"`
		History  json.RawMessage `json:"history"`
	}
	if decode(committedReply, &reply) != nil {
		e.createTaken = true
		return Request{}, ErrNoGrant
	}
	grant, err := object(reply.Grant)
	if err != nil {
		e.createTaken = true
		return Request{}, ErrNoGrant
	}
	selection, err := e.selection(actualSelection, false)
	if err != nil {
		return Request{}, err
	}
	if reply.Dispatch.ID != e.original.Dispatch.ID || reply.Dispatch.AttemptID != e.original.Dispatch.AttemptID ||
		reply.Dispatch.OperationID != e.history.OperationID || reply.Dispatch.State != "consumed" ||
		!reply.Dispatch.Deadline.Equal(e.original.Dispatch.Deadline) || !equalJSON(reply.Dispatch.Candidate, e.original.Dispatch.Candidate) ||
		!decimalID(reply.Dispatch.Revision) || field(grant, "schema_version") != "dispatch-grant.v1" ||
		!identity(field(grant, "grant_id")) || field(grant, "dispatch_id") != e.original.Dispatch.ID ||
		field(grant, "attempt_id") != e.original.Dispatch.AttemptID || field(grant, "release_id") != e.history.ReleaseID ||
		field(selection, "dispatch_id") != e.original.Dispatch.ID || field(selection, "attempt_id") != e.original.Dispatch.AttemptID ||
		field(grant, "selection_digest") != field(selection, "selection_digest") ||
		field(grant, "original_body_digest") != e.basis.Digest() ||
		!identity(live.ID) || !decimalID(live.Generation) ||
		field(selection, "invocation_id") != live.ID || field(grant, "invocation_id") != live.ID ||
		field(selection, "context_generation") != live.Generation || field(grant, "context_generation") != live.Generation ||
		field(selection, "gateway_boot_id") != e.gateway.BootID || field(grant, "gateway_boot_id") != e.gateway.BootID {
		return Request{}, ErrIdentity
	}
	expiry, err := time.Parse(time.RFC3339Nano, field(grant, "not_after"))
	if err != nil || now.IsZero() || !now.Before(expiry) || expiry.After(e.original.Dispatch.Deadline) {
		return Request{}, ErrExpired
	}
	if e.checkHistory(reply.History, field(selection, "selection_digest")) != nil {
		return Request{}, ErrIdentity
	}
	e.createTaken = true
	return e.request("POST", e.profile.path, e.original.Dispatch.AttemptID, "", reply.History, actualSelection), nil
}

// PrepareQuery accepts the first committed original-query receipt, including
// owner refresh already authorized by Console. It performs no new-call balance
// check, never accepts a create grant, and never changes original procurement.
func (e *Executor) PrepareQuery(committedReply []byte, now time.Time) (Request, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.profile.asynchronous {
		return Request{}, ErrUnsupported
	}
	var reply struct {
		Receipt struct {
			AttemptID   string          `json:"query_attempt_id"`
			OperationID string          `json:"operation_id"`
			ExpiresAt   time.Time       `json:"expires_at"`
			Selection   json.RawMessage `json:"selection"`
		} `json:"receipt"`
		History json.RawMessage `json:"history"`
	}
	if decode(committedReply, &reply) != nil {
		return Request{}, ErrNoGrant
	}
	var query struct {
		AttemptID          string          `json:"query_attempt_id"`
		OperationID        string          `json:"operation_id"`
		TaskID             string          `json:"provider_task_id"`
		Method             string          `json:"method"`
		Path               string          `json:"path"`
		Descriptor         json.RawMessage `json:"descriptor"`
		Profile            string          `json:"profile"`
		Parameters         json.RawMessage `json:"effective_parameters"`
		Selection          json.RawMessage `json:"execution_selection"`
		CredentialRef      string          `json:"credential_ref"`
		CredentialRevision string          `json:"credential_revision"`
		CapabilityRef      string          `json:"capability_ref"`
		OwnerInstance      string          `json:"owner_instance"`
		OwnerActor         string          `json:"owner_actor"`
		ExpiresAt          time.Time       `json:"expires_at"`
	}
	if string(reply.Receipt.Selection) == "null" || len(reply.Receipt.Selection) == 0 {
		return Request{}, ErrNoGrant
	}
	if decode(reply.Receipt.Selection, &query) != nil {
		return Request{}, ErrIdentity
	}
	if _, taken := e.queries[query.AttemptID]; taken {
		return Request{}, ErrReplay
	}
	selection, err := e.selection(query.Selection, true)
	if err != nil {
		return Request{}, err
	}
	var history historyWire
	if decode(reply.History, &history) != nil || e.checkHistory(reply.History, "") != nil || history.Attempt == nil ||
		history.Attempt.TaskID != query.TaskID || !taskID(query.TaskID) || !identity(query.AttemptID) ||
		query.AttemptID == e.original.Dispatch.AttemptID || query.AttemptID != reply.Receipt.AttemptID ||
		query.OperationID != e.history.OperationID || query.OperationID != reply.Receipt.OperationID ||
		query.Method != "GET" || query.Path != "/api/v1/tasks/"+query.TaskID || query.Profile != e.profile.name ||
		!equalJSON(query.Descriptor, e.original.Descriptor) ||
		query.CredentialRef != field(e.candidate, "credential_ref") || query.CredentialRevision != field(e.candidate, "credential_revision") ||
		query.OwnerActor != e.gateway.Actor || query.OwnerInstance != e.gateway.BootID ||
		field(selection, "gateway_boot_id") != e.gateway.BootID || field(selection, "attempt_id") != query.AttemptID ||
		!query.ExpiresAt.Equal(reply.Receipt.ExpiresAt) {
		return Request{}, ErrIdentity
	}
	caps, _ := object(e.descriptor["capabilities"])
	cap, _ := object(caps["task_query"])
	if query.CapabilityRef != field(cap, "query_capability_ref") {
		return Request{}, ErrUnsupported
	}
	parameters, _ := object([]byte(e.original.Body))
	delete(parameters, "model")
	parameterBytes, err := json.Marshal(parameters)
	if err != nil || !equalJSON(query.Parameters, parameterBytes) {
		return Request{}, ErrIdentity
	}
	if now.IsZero() || !now.Before(query.ExpiresAt) {
		return Request{}, ErrExpired
	}
	// Bounded local retention never evicts an old permit and allows its replay.
	// On exhaustion read original state; do not invent another query attempt.
	if len(e.queries) >= 10_000 {
		return Request{}, ErrCapacity
	}
	e.queries[query.AttemptID] = struct{}{}
	return e.request("GET", query.Path, query.AttemptID, query.TaskID, reply.History, query.Selection), nil
}

func (e *Executor) selection(raw []byte, query bool) (map[string]json.RawMessage, error) {
	s, err := object(raw)
	if err != nil || !hasDigest(raw, "selection_digest") || field(s, "selection_canonicalization") != "selection-json.v1" ||
		field(s, "call_id") != e.history.CallID || field(s, "operation_id") != e.history.OperationID ||
		field(s, "send_transport") != "http_call" || field(s, "recording_owner") != "ai_proxy" ||
		field(s, "credential_ref") != field(e.candidate, "credential_ref") ||
		field(s, "credential_revision") != field(e.candidate, "credential_revision") ||
		field(s, "selection_generation") != e.original.Revision {
		return nil, ErrIdentity
	}
	for _, key := range []string{"release_id", "target_id", "binding_id", "provider_account_id", "upstream_model", "cluster_ref", "descriptor_digest", "mapper_digest", "proxy_digest"} {
		if field(s, key) != field(e.descriptor, key) {
			return nil, ErrIdentity
		}
	}
	if query {
		if _, hasDispatch := s["dispatch_id"]; hasDispatch {
			return nil, ErrIdentity
		}
		if field(s, "operation_kind") != "task_query" || field(s, "mechanism") != "task_query" ||
			field(s, "original_body_digest") != bodybasis.DigestBytes(nil) {
			return nil, ErrIdentity
		}
	} else if field(s, "operation_kind") != e.profile.operation || field(s, "mechanism") != "router" ||
		field(s, "original_body_digest") != e.basis.Digest() {
		return nil, ErrIdentity
	}
	return s, nil
}

func (e *Executor) checkHistory(raw []byte, digest string) error {
	var h historyWire
	if decode(raw, &h) != nil || h.CallID != e.history.CallID || h.OperationID != e.history.OperationID ||
		h.ReleaseID != e.history.ReleaseID || h.RouteID != e.history.RouteID || h.PublicModel != e.history.PublicModel ||
		h.Kind != e.profile.operation || !equalJSON(h.Caller, e.history.Caller) || h.Attempt == nil {
		return ErrIdentity
	}
	oldCall, oldErr := object(e.original.History)
	newCall, newErr := object(raw)
	delete(oldCall, "attempt")
	delete(newCall, "attempt")
	oldBytes, _ := json.Marshal(oldCall)
	newBytes, _ := json.Marshal(newCall)
	if oldErr != nil || newErr != nil || !equalJSON(oldBytes, newBytes) {
		return ErrIdentity
	}
	a := h.Attempt
	if a.ID != e.original.Dispatch.AttemptID || a.TargetID != field(e.descriptor, "target_id") ||
		a.Writer.ProducerID != e.gateway.ProducerID || a.Writer.Role != "ai_proxy" ||
		a.Transport != "http_call" || a.Owner != "ai_proxy" || a.Mechanism != "router" ||
		a.CredentialRef != field(e.candidate, "credential_ref") || a.CredentialRevision != field(e.candidate, "credential_revision") ||
		a.Procurement.BindingID != field(e.descriptor, "binding_id") || a.Procurement.ProviderID != field(e.descriptor, "provider_id") ||
		a.Procurement.Account != field(e.descriptor, "provider_account_id") || a.Procurement.Model != e.profile.model || !identity(a.Procurement.Region) ||
		(digest != "" && a.SelectionDigest != digest) {
		return ErrIdentity
	}
	if e.history.Attempt != nil && (a.SelectionDigest != e.history.Attempt.SelectionDigest ||
		a.Procurement.Region != e.history.Attempt.Procurement.Region ||
		(e.history.Attempt.TaskID != "" && a.TaskID != e.history.Attempt.TaskID)) {
		return ErrIdentity
	}
	return nil
}

func (e *Executor) request(method, path, attempt, task string, history, selection []byte) Request {
	r := Request{method: method, endpoint: e.original.Endpoint, path: path, authority: e.authority, secret: e.secret,
		operationID: e.history.OperationID, callID: e.history.CallID, attemptID: attempt,
		createAttemptID: e.original.Dispatch.AttemptID, taskID: task,
		history: append([]byte(nil), history...), selection: append([]byte(nil), selection...), query: method == "GET"}
	r.descriptor = append([]byte(nil), e.original.Descriptor...)
	r.timeoutMS = e.timeoutMS
	r.profile = e.profile
	r.estimation = e.estimation
	r.outputContract, r.createClassifier = e.outputContract, e.createClassifier
	if !r.query {
		r.body = append([]byte(nil), e.body...)
	}
	return r
}

func taskID(value string) bool {
	if !identity(value) || value == "." || value == ".." {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
