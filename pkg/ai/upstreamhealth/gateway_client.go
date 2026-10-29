package upstreamhealth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
)

const (
	OutcomePath            = "/internal/upstream-health/outcomes"
	RecoveryClaimPath      = "/internal/upstream-health/recovery-claims"
	MaxGatewayBodyBytes    = 1 << 20
	MaxGatewayCommandBytes = 64 << 10
)

// GatewayResponse keeps transport completeness separate from a real HTTP code.
// A host read failure must not be converted into a synthetic upstream status.
type GatewayResponse struct {
	StatusCode      int
	HeadersComplete bool
	BodyComplete    bool
	Body            []byte
}

// GatewayCall is supplied by the existing trusted service transport. It must
// preserve target's escaped path and never follow client-provided destinations.
type GatewayCall func(method, target string, headers [][2]string, body []byte, timeoutMS uint32, callback func(GatewayResponse)) error

type GatewayState string

const (
	GatewayAcknowledged GatewayState = "acknowledged"
	GatewayRejected     GatewayState = "rejected"
	GatewayUnknown      GatewayState = "unknown"
	GatewayNotFound     GatewayState = "not_found"
)

// Acknowledged means the particular server operation committed. A ClaimReceipt
// still only allocates recovery; only a Consume response can authorize its first
// original attempt. No GET, timeout or unknown response creates a send grant.
type GatewayResult[T any] struct {
	State      GatewayState
	HTTPStatus int
	Value      *T
	Reason     string
}

type GatewayClient struct {
	call     GatewayCall
	headers  [][2]string
	owner    string
	timeout  uint32
	mu       sync.Mutex
	pending  map[string][]byte
	revision Revision
	digest   string
}

// NewGatewayClient accepts only deployment-authenticated producer/startup headers,
// never the external HTTP request's headers. Server registration remains the
// authority for the actor and owner; the request body cannot assign either role.
func NewGatewayClient(call GatewayCall, trustedHeaders [][2]string, ownerInstance string, timeoutMS uint32) (*GatewayClient, error) {
	if call == nil || !identity(ownerInstance) || timeoutMS == 0 || timeoutMS > 60000 {
		return nil, errors.New("health gateway requires its registered transport and startup owner")
	}
	auth, instance := 0, 0
	for _, header := range trustedHeaders {
		if header[0] == "" || strings.HasPrefix(header[0], ":") || strings.ContainsAny(header[0]+header[1], "\r\n\x00") {
			return nil, errors.New("health registered headers are invalid")
		}
		if strings.EqualFold(header[0], "authorization") {
			auth++
			if !strings.HasPrefix(header[1], "Bearer ") || strings.TrimSpace(strings.TrimPrefix(header[1], "Bearer ")) == "" {
				return nil, errors.New("health gateway requires server authentication")
			}
		}
		if strings.EqualFold(header[0], "x-modelfusion-gateway-instance") {
			instance++
			if header[1] != ownerInstance {
				return nil, errors.New("health gateway startup differs from its registered owner")
			}
		}
	}
	if auth != 1 || instance != 1 {
		return nil, errors.New("health gateway requires one registered authorization and startup")
	}
	return &GatewayClient{call: call, headers: append([][2]string(nil), trustedHeaders...), owner: ownerInstance, timeout: timeoutMS, pending: map[string][]byte{}, revision: "0"}, nil
}

func gatewayJSON(raw []byte, target interface{}) error {
	canonical, err := mfjson.Canonicalize(raw)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func callGateway[T any](c *GatewayClient, method, path string, value interface{}, callback func(GatewayResult[T]), decode func([]byte) (*T, error), missing bool) error {
	return callGatewayWithBudget(c, method, path, value, callback, decode, missing, MaxGatewayBodyBytes)
}

func callGatewayWithBudget[T any](c *GatewayClient, method, path string, value interface{}, callback func(GatewayResult[T]), decode func([]byte) (*T, error), missing bool, maxResponseBytes int) error {
	if c == nil || c.call == nil || callback == nil {
		return errors.New("health gateway callback is required")
	}
	var body []byte
	if value != nil {
		var err error
		body, err = json.Marshal(value)
		if err != nil {
			return err
		}
		body, err = mfjson.Canonicalize(body)
		if err != nil {
			return err
		}
		if len(body) > MaxGatewayCommandBytes {
			return errors.New("health command exceeds the original service budget")
		}
	}
	var once sync.Once
	finish := func(result GatewayResult[T]) { once.Do(func() { callback(result) }) }
	err := c.call(method, path, append([][2]string(nil), c.headers...), body, c.timeout, func(response GatewayResponse) {
		result := GatewayResult[T]{State: GatewayUnknown, HTTPStatus: response.StatusCode, Reason: "health_response_unconfirmed"}
		if !response.HeadersComplete || !response.BodyComplete || response.StatusCode < 100 || response.StatusCode > 599 || len(response.Body) > maxResponseBytes {
			finish(result)
			return
		}
		if missing && response.StatusCode == 404 {
			result.State, result.Reason = GatewayNotFound, "health_original_claim_absent"
			finish(result)
			return
		}
		if response.StatusCode != 200 {
			if response.StatusCode >= 400 && response.StatusCode < 500 && response.StatusCode != 408 {
				result.State, result.Reason = GatewayRejected, "health_request_rejected"
			}
			finish(result)
			return
		}
		if missing && bytes.Equal(bytes.TrimSpace(response.Body), []byte("null")) {
			result.State, result.Reason = GatewayNotFound, "health_original_claim_absent"
			finish(result)
			return
		}
		parsed, err := decode(append([]byte(nil), response.Body...))
		if err == nil {
			result.State, result.Reason, result.Value = GatewayAcknowledged, "", parsed
		}
		finish(result)
	})
	if err != nil {
		finish(GatewayResult[T]{State: GatewayUnknown, Reason: "health_transport_unconfirmed"})
	}
	return nil
}

// ReportOutcome retains every original until its exact durable ACK, including
// a recovery success whose history has not arrived yet. Failures additionally
// veto local sends before dispatch; a newer Redis revision cannot clear them.
// The caller may retry this same event on natural traffic; there is no timer,
// provider call, worker, credential lookup or business retry in this client.
func (c *GatewayClient) ReportOutcome(request OutcomeRequest, callback func(GatewayResult[OutcomeReceipt])) error {
	if err := request.validate(); err != nil {
		return err
	}
	if c == nil || callback == nil {
		return errors.New("health outcome client and callback are required")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	raw, err = mfjson.Canonicalize(raw)
	if err != nil {
		return err
	}
	var frozen OutcomeRequest
	if gatewayJSON(raw, &frozen) != nil {
		return errors.New("health outcome could not be retained")
	}
	c.mu.Lock()
	old, exists := c.pending[frozen.EventID]
	if exists && !bytes.Equal(old, raw) {
		c.mu.Unlock()
		return errors.New("health event has different original content")
	}
	c.pending[frozen.EventID] = append([]byte(nil), raw...)
	c.mu.Unlock()
	return callGateway(c, "POST", OutcomePath, frozen, func(result GatewayResult[OutcomeReceipt]) {
		if result.State == GatewayAcknowledged {
			c.mu.Lock()
			if compareRevision(result.Value.Revision, c.revision) > 0 {
				c.revision = result.Value.Revision
				c.digest = ""
			}
			if bytes.Equal(c.pending[frozen.EventID], raw) {
				delete(c.pending, frozen.EventID)
			}
			c.mu.Unlock()
		}
		callback(result)
	}, func(body []byte) (*OutcomeReceipt, error) {
		var receipt OutcomeReceipt
		if err := gatewayJSON(body, &receipt); err != nil {
			return nil, err
		}
		if receipt.EventID != frozen.EventID || !identity(receipt.Decision) || receipt.Scopes == nil || !validTime(receipt.ReceivedAt) {
			return nil, errors.New("health ACK differs from the original event")
		}
		if _, err := ParseRevision(string(receipt.Revision)); err != nil {
			return nil, err
		}
		for _, scope := range receipt.Scopes {
			if !validScope(scope.Identity) {
				return nil, errors.New("health ACK contains an invalid scope")
			}
			for _, revision := range []Revision{scope.Revision, scope.FailureGeneration, scope.RecoveryEpoch} {
				if _, err := ParseRevision(string(revision)); err != nil {
					return nil, err
				}
			}
		}
		return &receipt, nil
	}, false)
}

// PendingOutcomes supplies original unacknowledged events for an explicit same-event
// retry. It neither changes their occurrence time nor elects another target.
func (c *GatewayClient) PendingOutcomes(target TargetReference) []OutcomeRequest {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []OutcomeRequest
	for _, raw := range c.pending {
		var request OutcomeRequest
		if gatewayJSON(raw, &request) == nil && outcomeApplies(request, target) {
			out = append(out, request)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EventID < out[j].EventID })
	return out
}

func (c *GatewayClient) Vetoes(target TargetReference) bool {
	for _, request := range c.PendingOutcomes(target) {
		if request.Effect.Kind == "rate_limited" || request.Effect.Kind == "credential_invalid" || request.Effect.Kind == "permanently_blocked" {
			return true
		}
	}
	return false
}

// DecideNewCall joins the original snapshot decision with this observer's local
// failure and durable-ACK watermark. Clearing a pending event cannot reopen an
// older Redis view while the already committed outcome is still being published.
// Reuse the registered client for its startup/authority across managed targets.
func (c *GatewayClient) DecideNewCall(result ReadResult, candidate Candidate, now time.Time) Decision {
	if c == nil {
		return deny("health_control_unavailable", ReadRedisUnavailable)
	}
	if result.Status == ReadReady && result.Snapshot != nil && result.Snapshot.current(now) == ReadReady {
		metadata := result.Snapshot.Metadata()
		c.mu.Lock()
		order := compareRevision(metadata.Revision, c.revision)
		if order < 0 {
			c.mu.Unlock()
			return deny("health_control_unpublished", ReadStale)
		}
		if order == 0 && c.digest != "" && c.digest != metadata.StateDigest {
			c.mu.Unlock()
			return deny("health_control_conflict", ReadConflict)
		}
		c.revision, c.digest = metadata.Revision, metadata.StateDigest
		c.mu.Unlock()
	}
	candidate.PendingLocalVeto = candidate.PendingLocalVeto || c.Vetoes(candidate.Target)
	return result.DecideNewCall(candidate, now)
}

func outcomeApplies(request OutcomeRequest, target TargetReference) bool {
	if request.Effect.Kind == "credential_invalid" {
		return request.Selection.CredentialRef == target.CredentialRef && request.Selection.CredentialGeneration == target.CredentialGeneration
	}
	for _, left := range request.AffectedScopeIDs {
		for _, right := range target.ScopeIDs {
			if left == right {
				return true
			}
		}
	}
	return false
}

func decodeClaim(body []byte, request ClaimRequest) (*ClaimReceipt, error) {
	var receipt ClaimReceipt
	if err := gatewayJSON(body, &receipt); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	switch receipt.Decision {
	case "granted":
		if len(fields) != 2 || fields["claim"] == nil || receipt.Claim == nil || receipt.Reason != nil || receipt.Claim.Status != "allocated" || len(receipt.Claim.ConsumedSelection) != 0 {
			return nil, errors.New("health allocation is invalid")
		}
		if err := receipt.Claim.validate(request); err != nil {
			return nil, err
		}
	case "not_required":
		if len(fields) != 1 || receipt.Claim != nil || receipt.Reason != nil {
			return nil, errors.New("health not-required response contains a claim")
		}
	case "denied":
		if len(fields) != 2 || fields["reason"] == nil || receipt.Claim != nil || receipt.Reason == nil || !identity(*receipt.Reason) {
			return nil, errors.New("health denial is invalid")
		}
	default:
		return nil, errors.New("health claim decision is unknown")
	}
	return &receipt, nil
}

func (c *GatewayClient) Claim(request ClaimRequest, callback func(GatewayResult[ClaimReceipt])) error {
	if c == nil {
		return errors.New("health gateway is absent")
	}
	if err := request.validate(c.owner); err != nil {
		return err
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	var original ClaimRequest
	if err = gatewayJSON(raw, &original); err != nil {
		return err
	}
	return callGateway(c, "POST", RecoveryClaimPath, original, callback, func(body []byte) (*ClaimReceipt, error) { return decodeClaim(body, original) }, false)
}

// GetClaim only retrieves the original allocation receipt for this startup.
// A lost Consume response must never be recovered into permission to send.
func (c *GatewayClient) GetClaim(original ClaimRequest, callback func(GatewayResult[ClaimReceipt])) error {
	if c == nil {
		return errors.New("health gateway is absent")
	}
	if err := original.validate(c.owner); err != nil {
		return err
	}
	raw, err := json.Marshal(original)
	if err != nil {
		return err
	}
	if err = gatewayJSON(raw, &original); err != nil {
		return err
	}
	return callGateway(c, "GET", RecoveryClaimPath+"/"+url.PathEscape(original.ClaimRequestID), nil, callback, func(body []byte) (*ClaimReceipt, error) { return decodeClaim(body, original) }, true)
}

// Consume returns acknowledged only for the original first committed consume.
// The caller must retain its final selection and use its existing single-send
// gate. A duplicate/unknown/replayed allocation can never authorize another send.
func (c *GatewayClient) Consume(allocation RecoveryClaim, finalSelection []byte, callback func(GatewayResult[RecoveryClaim])) error {
	if c == nil {
		return errors.New("health gateway is absent")
	}
	if err := allocation.Request.validate(c.owner); err != nil {
		return err
	}
	if err := allocation.validate(allocation.Request); err != nil {
		return err
	}
	if allocation.Status != "allocated" || len(allocation.ConsumedSelection) != 0 {
		return errors.New("health recovery was already consumed")
	}
	raw, err := json.Marshal(allocation)
	if err != nil {
		return err
	}
	if err = gatewayJSON(raw, &allocation); err != nil {
		return err
	}
	expected, err := allocation.Request.Candidate.FinalSelection(allocation.ClaimID)
	if err != nil {
		return err
	}
	final, err := executionfacts.CanonicalSelection(finalSelection)
	if err != nil {
		return err
	}
	if !bytes.Equal(expected, final) {
		return errors.New("health final selection changed the original candidate or claim")
	}
	request := struct {
		Request   ClaimRequest    `json:"request"`
		Selection json.RawMessage `json:"selection"`
	}{allocation.Request, append([]byte(nil), final...)}
	return callGateway(c, "POST", RecoveryClaimPath+"/"+allocation.ClaimID+"/consume", request, callback, func(body []byte) (*RecoveryClaim, error) {
		var consumed RecoveryClaim
		if err := gatewayJSON(body, &consumed); err != nil {
			return nil, err
		}
		if err := consumed.validate(allocation.Request); err != nil {
			return nil, err
		}
		stored, err := executionfacts.CanonicalSelection(consumed.ConsumedSelection)
		expires, _ := utc(consumed.ExpiresAt)
		if err != nil || consumed.Status != "consumed" || consumed.ClaimID != allocation.ClaimID || !bytes.Equal(stored, final) ||
			consumed.Actor != allocation.Actor || consumed.CreatedAt != allocation.CreatedAt || consumed.ExpiresAt != allocation.ExpiresAt ||
			!sameJSON(consumed.Scopes, allocation.Scopes) || consumed.CredentialControlRevision != allocation.CredentialControlRevision || !expires.After(time.Now()) {
			return nil, errors.New("health consumption did not confirm the original live selection")
		}
		return &consumed, nil
	}, false)
}
