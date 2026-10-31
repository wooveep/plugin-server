package upstreamhealth

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

const RuntimeSchema = "upstream-health-runtime.v1"
const runtimeFieldPrefix = "runtime:"

// RuntimeRedis uses the existing foreground commands for send qualification.
// Command retains outcome callbacks in the existing root event dispatcher.
type RuntimeRedis interface {
	RedisReader
	Eval(string, int, []interface{}, []interface{}, wrapper.RedisResponseCallback) error
	Command([]interface{}, wrapper.RedisResponseCallback) error
}

// RuntimePolicy mirrors the existing HEALTH timing policy, not a new probe loop.
type RuntimePolicy struct {
	BackoffSeconds          []uint32 `json:"backoff_seconds"`
	ClaimLeaseSeconds       uint32   `json:"claim_lease_seconds"`
	InconclusiveWaitSeconds uint32   `json:"inconclusive_wait_seconds"`
	ClockToleranceSeconds   uint32   `json:"clock_tolerance_seconds"`
}

func DefaultRuntimePolicy() RuntimePolicy {
	return RuntimePolicy{BackoffSeconds: []uint32{30, 60, 120, 240, 300}, ClaimLeaseSeconds: 10,
		InconclusiveWaitSeconds: 30, ClockToleranceSeconds: 30}
}

// RuntimeScope changes only natural-traffic state under an unchanged PG control.
// A new control revision fences ownership without clearing temporary cooldown.
type RuntimeScope struct {
	ControlRevision          Revision   `json:"control_revision"`
	ControlFailureGeneration Revision   `json:"control_failure_generation"`
	State                    ScopeState `json:"state"`
	ClaimExpiresAt           *string    `json:"claim_expires_at"`
}

// RuntimeEvent uses the original private execution Stream for archival/rebuild.
// It is not a source of provider usage, pricing, or a second business send.
type RuntimeEvent struct {
	SchemaVersion   string          `json:"schema_version"`
	AuthorityID     string          `json:"authority_id"`
	ProducerID      string          `json:"producer_id"`
	EventID         string          `json:"event_id"`
	Kind            string          `json:"kind"`
	Revision        Revision        `json:"revision"`
	ControlRevision Revision        `json:"control_revision"`
	RequestDigest   string          `json:"request_digest"`
	RecordedAt      string          `json:"recorded_at"`
	Scopes          []RuntimeScope  `json:"scopes"`
	Claim           *RecoveryClaim  `json:"claim,omitempty"`
	Outcome         *OutcomeRequest `json:"outcome,omitempty"`
}

type RuntimeConfig struct {
	Projection    Config
	Stream        string
	ProducerID    string
	OwnerInstance string
	Policy        RuntimePolicy
}

// Runtime keeps PG controls separate from Redis cooldown/one-shot recovery.
// The atomic mutation appends its original record to the existing event Stream.
type Runtime struct {
	redis   RuntimeRedis
	reader  *Reader
	config  RuntimeConfig
	mu      sync.Mutex
	pending map[string]runtimePending
	claims  map[string]RecoveryClaim
}

type runtimePending struct {
	body   []byte
	target TargetReference
}

func NewRuntime(redis RuntimeRedis, config RuntimeConfig) (*Runtime, error) {
	if redis == nil || !identity(config.Stream) || !identity(config.ProducerID) || !identity(config.OwnerInstance) ||
		len(config.Policy.BackoffSeconds) == 0 || len(config.Policy.BackoffSeconds) > 32 || config.Policy.ClaimLeaseSeconds == 0 ||
		config.Policy.InconclusiveWaitSeconds == 0 {
		return nil, errors.New("health runtime requires its existing Redis, Stream, startup, and policy")
	}
	for i, seconds := range config.Policy.BackoffSeconds {
		if seconds == 0 || i > 0 && seconds < config.Policy.BackoffSeconds[i-1] {
			return nil, errors.New("health runtime backoff policy is invalid")
		}
	}
	reader, err := NewReader(redis, config.Projection)
	if err != nil {
		return nil, err
	}
	config.Policy.BackoffSeconds = append([]uint32(nil), config.Policy.BackoffSeconds...)
	return &Runtime{redis: redis, reader: reader, config: config, pending: map[string]runtimePending{}, claims: map[string]RecoveryClaim{}}, nil
}

type runtimeView struct {
	read     ReadResult
	revision Revision
	scopes   map[string]RuntimeScope
	now      time.Time
}

func (r *Runtime) decodeAll(value resp.Value, now time.Time) runtimeView {
	failure := func(status ReadStatus) runtimeView { return runtimeView{read: ReadResult{Status: status}} }
	if value.Error() != nil {
		return failure(ReadRedisUnavailable)
	}
	if value.Type() != resp.Array || len(value.Array())%2 != 0 {
		return failure(ReadInvalid)
	}
	fields := map[string]string{}
	for i := 0; i < len(value.Array()); i += 2 {
		key, value := value.Array()[i], value.Array()[i+1]
		if key.Type() != resp.BulkString || value.Type() != resp.BulkString {
			return failure(ReadInvalid)
		}
		if _, duplicate := fields[key.String()]; duplicate {
			return failure(ReadInvalid)
		}
		fields[key.String()] = value.String()
	}
	if len(fields) == 0 {
		return failure(ReadMissing)
	}
	control := make([]string, len(HashFields()))
	for i, field := range HashFields() {
		value, found := fields[field]
		if !found {
			return failure(ReadIncomplete)
		}
		control[i] = value
	}
	read := r.reader.acceptSnapshot(decodeHash(control, r.reader.config, now))
	if read.Status != ReadReady {
		return runtimeView{read: read}
	}
	revision := Revision("0")
	if raw, found := fields["runtime_revision"]; found {
		var err error
		revision, err = ParseRevision(raw)
		if err != nil {
			return failure(ReadInvalid)
		}
	}
	read.Snapshot.runtimeNow = now
	view := runtimeView{read: read, revision: revision, scopes: map[string]RuntimeScope{}, now: now}
	for index, original := range read.Snapshot.wire.Scopes {
		current := RuntimeScope{ControlRevision: original.Revision, ControlFailureGeneration: original.FailureGeneration, State: original}
		if raw, found := fields[runtimeFieldPrefix+original.Identity.ScopeID]; found {
			var delivered RuntimeScope
			if gatewayJSON([]byte(raw), &delivered) != nil {
				return failure(ReadInvalid)
			}
			if !sameJSON(delivered.State.Identity, original.Identity) ||
				(delivered.State.ActiveClaim == nil) != (delivered.ClaimExpiresAt == nil) ||
				delivered.ClaimExpiresAt != nil && !validTime(*delivered.ClaimExpiresAt) {
				return failure(ReadInvalid)
			}
			var err error
			if delivered.ControlRevision != original.Revision {
				// Explicit controls invalidate ownership, while preserving temporary
				// cooldown and its progress. Clearing a permanent denial is not a
				// successful recovery of an earlier provider 429.
				delivered.State.Revision, err = rebaseRuntimeRevision(delivered.State.Revision, delivered.ControlRevision, original.Revision)
				if err != nil {
					return failure(ReadInvalid)
				}
				delivered.State.FailureGeneration, err = rebaseRuntimeRevision(delivered.State.FailureGeneration, delivered.ControlFailureGeneration, original.FailureGeneration)
				if err != nil {
					return failure(ReadInvalid)
				}
				if compareRevision(original.RecoveryEpoch, delivered.State.RecoveryEpoch) > 0 {
					delivered.State.RecoveryEpoch = original.RecoveryEpoch
				}
				delivered.ControlRevision = original.Revision
				delivered.ControlFailureGeneration = original.FailureGeneration
				delivered.State.PermanentReason = original.PermanentReason
				delivered.State.ActiveClaim = nil
				delivered.ClaimExpiresAt = nil
				if original.BlockedUntil != nil {
					incoming, _ := utc(*original.BlockedUntil)
					old := time.Time{}
					if delivered.State.BlockedUntil != nil {
						old, _ = utc(*delivered.State.BlockedUntil)
					}
					if incoming.After(old) {
						delivered.State.BlockedUntil = original.BlockedUntil
					}
				}
				if original.BackoffStep > delivered.State.BackoffStep {
					delivered.State.BackoffStep = original.BackoffStep
				}
			} else if delivered.ControlFailureGeneration != original.FailureGeneration || !sameJSON(delivered.State.PermanentReason, original.PermanentReason) {
				return failure(ReadInvalid)
			}
			current = delivered
		}
		view.scopes[original.Identity.ScopeID] = current
		read.Snapshot.wire.Scopes[index] = current.State
	}
	if !validateWire(read.Snapshot.wire) {
		return failure(ReadInvalid)
	}
	return view
}

func (r *Runtime) load(background bool, callback func(runtimeView)) {
	var once sync.Once
	// Lease decisions and the state they inspect use one Redis clock observation.
	// A different gateway wall clock cannot steal another VM's live exclusion.
	const read = `return {redis.call('HGETALL',KEYS[1]),redis.call('TIME')}`
	finish := func(value resp.Value) {
		once.Do(func() {
			failed := runtimeView{read: ReadResult{Status: ReadRedisUnavailable}}
			if value.Error() != nil || value.Type() != resp.Array || len(value.Array()) != 2 {
				callback(failed)
				return
			}
			clock := value.Array()[1]
			if clock.Type() != resp.Array || len(clock.Array()) != 2 {
				callback(failed)
				return
			}
			seconds, err := strconv.ParseInt(clock.Array()[0].String(), 10, 64)
			micros, microErr := strconv.ParseInt(clock.Array()[1].String(), 10, 64)
			if err != nil || microErr != nil || micros < 0 || micros >= 1000000 {
				callback(failed)
				return
			}
			callback(r.decodeAll(value.Array()[0], time.Unix(seconds, micros*1000).UTC()))
		})
	}
	var err error
	if background {
		err = r.redis.Command([]interface{}{"EVAL", read, 1, r.reader.key}, finish)
	} else {
		err = r.redis.Eval(read, 1, []interface{}{r.reader.key}, nil, finish)
	}
	if err != nil {
		once.Do(func() { callback(runtimeView{read: ReadResult{Status: ReadRedisUnavailable}}) })
	}
}

func (r *Runtime) Lookup(callback func(ReadResult)) {
	r.load(false, func(view runtimeView) { callback(view.read) })
}

func (r *Runtime) PendingOutcomes(target TargetReference) []OutcomeRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	var requests []OutcomeRequest
	for _, pending := range r.pending {
		var request OutcomeRequest
		if gatewayJSON(pending.body, &request) == nil && outcomeApplies(request, target) {
			requests = append(requests, request)
		}
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].EventID < requests[j].EventID })
	return requests
}

// FlushPending resends original health state only, from the root event context.
// It never resumes a customer request or creates provider traffic.
func (r *Runtime) FlushPending() {
	r.mu.Lock()
	packets := make([]runtimePending, 0, len(r.pending))
	for _, pending := range r.pending {
		packets = append(packets, pending)
	}
	r.mu.Unlock()
	for _, pending := range packets {
		var request OutcomeRequest
		if gatewayJSON(pending.body, &request) == nil {
			_ = r.ReportOutcome(request, pending.target, func(GatewayResult[OutcomeReceipt]) {})
		}
	}
}

func (r *Runtime) DecideNewCall(read ReadResult, candidate Candidate, now time.Time) Decision {
	if read.Snapshot != nil && !read.Snapshot.runtimeNow.IsZero() {
		now = read.Snapshot.runtimeNow
	}
	for _, request := range r.PendingOutcomes(candidate.Target) {
		if request.Effect.Kind == "rate_limited" {
			candidate.PendingLocalVeto = true
		}
	}
	return read.DecideNewCall(candidate, now)
}

func nextRuntimeRevision(value Revision) (Revision, error) {
	parsed, err := strconv.ParseInt(string(value), 10, 64)
	if err != nil || parsed < 0 || parsed == math.MaxInt64 {
		return "", errors.New("health revision cannot advance")
	}
	return Revision(strconv.FormatInt(parsed+1, 10)), nil
}

func rebaseRuntimeRevision(value, oldControl, newControl Revision) (Revision, error) {
	var numbers [3]int64
	for i, value := range []Revision{value, oldControl, newControl} {
		parsed, err := ParseRevision(string(value))
		if err != nil {
			return "", err
		}
		numbers[i], _ = strconv.ParseInt(string(parsed), 10, 64)
	}
	if numbers[2] < numbers[1] || numbers[0] < numbers[1] || numbers[0] > math.MaxInt64-(numbers[2]-numbers[1]) {
		return "", errors.New("health runtime control revision cannot move backward or overflow")
	}
	return Revision(strconv.FormatInt(numbers[0]+numbers[2]-numbers[1], 10)), nil
}

// A replayed consume never returns a new send grant. Outcome replay returns its
// original receipt. Lua checks all control/runtime fences before changing state.
const mutateRuntime = `
local old=redis.call('GET',KEYS[2])
if old then
    local prefix=ARGV[7]..'\n'
    if string.sub(old,1,#prefix)~=prefix then return {'conflict',''} end
    return {ARGV[4]=='claim_consumed' and 'duplicate' or 'replayed',string.sub(old,#prefix+1)}
end
if redis.call('HGET',KEYS[1],'authority_id')~=ARGV[1]
    or redis.call('HGET',KEYS[1],'state_digest')~=ARGV[2]
    or (redis.call('HGET',KEYS[1],'runtime_revision') or '0')~=ARGV[3] then return {'stale',''} end
if ARGV[4]=='claim_consumed' then
    local now=redis.call('TIME')
    local millis=tonumber(now[1])*1000+math.floor(tonumber(now[2])/1000)
    if tonumber(ARGV[10])<=millis then return {'expired',''} end
end
local stream=redis.call('TYPE',KEYS[3]).ok
if stream~='none' and stream~='stream' then return {'unavailable',''} end
local entries=cjson.decode(ARGV[8])
redis.call('XADD',KEYS[3],'*','message_type','health_runtime','event',ARGV[5])
for _,entry in ipairs(entries) do redis.call('HSET',KEYS[1],entry[1],entry[2]) end
redis.call('HSET',KEYS[1],'runtime_revision',ARGV[9])
redis.call('SET',KEYS[2],ARGV[7]..'\n'..ARGV[6])
return {'committed',ARGV[6]}
`

func (r *Runtime) commit(view runtimeView, event RuntimeEvent, original, receipt []byte, background bool, callback func(string, []byte)) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	raw, err = mfjson.Canonicalize(raw)
	if err != nil {
		return err
	}
	updates := make([][2]string, 0, len(event.Scopes))
	for _, scope := range event.Scopes {
		raw, err := json.Marshal(scope)
		if err != nil {
			return err
		}
		updates = append(updates, [2]string{runtimeFieldPrefix + scope.State.Identity.ScopeID, string(raw)})
	}
	encoded, err := json.Marshal(updates)
	if err != nil {
		return err
	}
	// One original attempt cannot be consumed again under another command key;
	// one original observation phase cannot apply twice under another event ID.
	identity := ""
	deadline := int64(0)
	if event.Claim != nil {
		var candidate struct {
			AttemptID string `json:"attempt_id"`
		}
		if json.Unmarshal(event.Claim.Request.Candidate.raw, &candidate) != nil {
			return errors.New("health claim has no original attempt")
		}
		identity = "attempt:" + candidate.AttemptID
		expires, _ := utc(event.Claim.ExpiresAt)
		deadline = expires.UnixMilli()
	} else {
		identity = "outcome:" + event.Outcome.Selection.AttemptID + ":" + event.Outcome.ObservationPhase
	}
	key := r.reader.key + ":event:" + strings.TrimPrefix(bodybasis.DigestBytes([]byte(identity)), "sha256:")
	keys := []interface{}{r.reader.key, key, r.config.Stream}
	args := []interface{}{r.config.Projection.AuthorityID, view.read.Snapshot.Metadata().StateDigest, string(view.revision), event.Kind,
		string(raw), string(receipt), bodybasis.DigestBytes(original), string(encoded), string(event.Revision), strconv.FormatInt(deadline, 10)}
	var once sync.Once
	deliver := func(status string, body []byte) { once.Do(func() { callback(status, body) }) }
	finish := func(value resp.Value) {
		if value.Error() != nil || value.Type() != resp.Array || len(value.Array()) != 2 {
			deliver("unavailable", nil)
			return
		}
		deliver(value.Array()[0].String(), []byte(value.Array()[1].String()))
	}
	if background {
		command := []interface{}{"EVAL", mutateRuntime, len(keys)}
		command = append(command, keys...)
		command = append(command, args...)
		err = r.redis.Command(command, finish)
	} else {
		err = r.redis.Eval(mutateRuntime, len(keys), keys, args, finish)
	}
	if err != nil {
		deliver("unavailable", nil)
	}
	return nil
}

// ClaimConsumed atomically takes all required scopes for this one real attempt.
// Its finite lease and final selection enter the original Stream before a send.
// Unknown and duplicate results never authorize dispatch or renew the lease.
func (r *Runtime) ClaimConsumed(request ClaimRequest, target TargetReference, callback func(GatewayResult[RecoveryClaim])) error {
	if err := request.validate(r.config.OwnerInstance); err != nil {
		return err
	}
	original, err := json.Marshal(request)
	if err != nil {
		return err
	}
	original, err = mfjson.Canonicalize(original)
	if err != nil {
		return err
	}
	var frozen ClaimRequest
	if err = gatewayJSON(original, &frozen); err != nil {
		return err
	}
	request = frozen
	var selected struct {
		TargetID           string   `json:"target_id"`
		BindingID          string   `json:"binding_id"`
		ProviderAccountID  string   `json:"provider_account_id"`
		UpstreamModel      string   `json:"upstream_model"`
		OperationKind      string   `json:"operation_kind"`
		CredentialRef      string   `json:"credential_ref"`
		CredentialRevision Revision `json:"credential_revision"`
	}
	if json.Unmarshal(request.Candidate.raw, &selected) != nil || !validTarget(target) || selected.TargetID != target.TargetID || selected.BindingID != target.BindingID ||
		selected.ProviderAccountID != target.ProviderAccountID || selected.UpstreamModel != target.UpstreamModel || selected.OperationKind != target.OperationKind ||
		selected.CredentialRef != target.CredentialRef || selected.CredentialRevision != target.CredentialGeneration {
		return errors.New("health recovery candidate differs from its frozen target")
	}
	deny := func(reason string) { callback(GatewayResult[RecoveryClaim]{State: GatewayRejected, Reason: reason}) }
	r.load(false, func(view runtimeView) {
		now := view.now
		decision := r.DecideNewCall(view.read, Candidate{Target: target}, now)
		if decision.Kind != RecoveryRequired || !sameJSON(decision.ScopeEpochs, request.ExpectedEpochs) {
			deny("health_recovery_changed")
			return
		}
		id, err := uuid.NewRandom()
		if err != nil {
			deny("health_claim_identity_unavailable")
			return
		}
		final, err := request.Candidate.FinalSelection(id.String())
		if err != nil {
			deny("health_final_selection_invalid")
			return
		}
		expires := now.Add(time.Duration(r.config.Policy.ClaimLeaseSeconds) * time.Second).UTC().Format(time.RFC3339Nano)
		claim := RecoveryClaim{ClaimID: id.String(), Request: request, Actor: "service/gateway/" + r.config.ProducerID,
			CredentialGeneration: target.CredentialGeneration, CredentialControlRevision: decision.CredentialControlRevision, CredentialRef: target.CredentialRef,
			Scopes: []ClaimedScope{}, CreatedAt: now.UTC().Format(time.RFC3339Nano), ExpiresAt: expires, Status: "consumed", ConsumedSelection: final}
		var scopes []RuntimeScope
		for _, state := range view.read.Snapshot.wire.Scopes {
			if _, required := decision.ScopeEpochs[state.Identity.ScopeID]; !required {
				continue
			}
			current := view.scopes[state.Identity.ScopeID]
			if current.ClaimExpiresAt != nil {
				deadline, _ := utc(*current.ClaimExpiresAt)
				if deadline.After(now) {
					deny("health_scope_owned")
					return
				}
			}
			current.State.RecoveryEpoch, err = nextRuntimeRevision(current.State.RecoveryEpoch)
			if err != nil {
				deny("health_epoch_unavailable")
				return
			}
			current.State.Revision, err = nextRuntimeRevision(current.State.Revision)
			if err != nil {
				deny("health_revision_unavailable")
				return
			}
			current.State.ActiveClaim = &claim.ClaimID
			current.ClaimExpiresAt = &expires
			claim.Scopes = append(claim.Scopes, ClaimedScope{ScopeID: state.Identity.ScopeID, Epoch: current.State.RecoveryEpoch, FailureGeneration: current.State.FailureGeneration})
			scopes = append(scopes, current)
		}
		revision, err := nextRuntimeRevision(view.revision)
		if err != nil {
			deny("health_revision_unavailable")
			return
		}
		event := RuntimeEvent{SchemaVersion: RuntimeSchema, AuthorityID: r.config.Projection.AuthorityID, ProducerID: r.config.ProducerID,
			EventID: "consume:" + request.ClaimRequestID, Kind: "claim_consumed", Revision: revision, ControlRevision: view.read.Snapshot.Metadata().Revision,
			RequestDigest: bodybasis.DigestBytes(original), RecordedAt: claim.CreatedAt, Scopes: scopes, Claim: &claim}
		receipt, err := json.Marshal(claim)
		if err != nil {
			deny("health_receipt_invalid")
			return
		}
		err = r.commit(view, event, original, receipt, false, func(status string, body []byte) {
			if status != "committed" {
				callback(GatewayResult[RecoveryClaim]{State: GatewayUnknown, Reason: "health_consume_unconfirmed"})
				return
			}
			var stored RecoveryClaim
			if gatewayJSON(body, &stored) != nil || !bytes.Equal(body, receipt) {
				deny("health_receipt_invalid")
				return
			}
			r.mu.Lock()
			for id, old := range r.claims {
				end, _ := utc(old.ExpiresAt)
				if !end.After(now) {
					delete(r.claims, id)
				}
			}
			r.claims[stored.ClaimID] = stored
			r.mu.Unlock()
			callback(GatewayResult[RecoveryClaim]{State: GatewayAcknowledged, Value: &stored})
		})
		if err != nil {
			callback(GatewayResult[RecoveryClaim]{State: GatewayUnknown, Reason: "health_consume_unconfirmed"})
		}
	})
	return nil
}

func (r *Runtime) ReportOutcome(request OutcomeRequest, target TargetReference, callback func(GatewayResult[OutcomeReceipt])) error {
	if err := request.validate(); err != nil {
		return err
	}
	if request.Effect.Kind != "rate_limited" && request.Effect.Kind != "recovery_succeeded" && request.Effect.Kind != "recovery_accepted" && request.Effect.Kind != "neutral" {
		return errors.New("natural traffic cannot change permanent health or credential controls")
	}
	if !validTarget(target) || request.Selection.TargetID != target.TargetID || request.Selection.CredentialRef != target.CredentialRef ||
		request.Selection.CredentialGeneration != target.CredentialGeneration || request.Selection.OperationKind != target.OperationKind {
		return errors.New("health outcome differs from its original frozen target")
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
	if err = gatewayJSON(raw, &frozen); err != nil {
		return err
	}
	request = frozen
	r.mu.Lock()
	if old, exists := r.pending[request.EventID]; exists && (!bytes.Equal(old.body, raw) || !sameJSON(old.target, target)) {
		r.mu.Unlock()
		return errors.New("health event content changed")
	}
	var held *RecoveryClaim
	if request.ClaimID != nil {
		if claim, found := r.claims[*request.ClaimID]; found {
			held = &claim
		}
	}
	if held != nil {
		var selected struct {
			CallID          string `json:"call_id"`
			AttemptID       string `json:"attempt_id"`
			SelectionDigest string `json:"selection_digest"`
		}
		if json.Unmarshal(held.ConsumedSelection, &selected) != nil || selected.CallID != request.Selection.CallID ||
			selected.AttemptID != request.Selection.AttemptID || selected.SelectionDigest != request.Selection.SelectionDigest {
			r.mu.Unlock()
			return errors.New("health outcome differs from its original consumed selection")
		}
	}
	// Boundary rejection cannot install a pending failure veto.
	target.ScopeIDs = append([]string(nil), target.ScopeIDs...)
	r.pending[request.EventID] = runtimePending{body: append([]byte(nil), raw...), target: target}
	r.mu.Unlock()
	unknown := func() {
		callback(GatewayResult[OutcomeReceipt]{State: GatewayUnknown, Reason: "health_outcome_unconfirmed"})
	}
	r.load(true, func(view runtimeView) {
		if view.read.Status != ReadReady {
			unknown()
			return
		}
		now := view.now
		occurred, _ := utc(request.OccurredAt)
		clockValid := !occurred.After(now.Add(time.Duration(r.config.Policy.ClockToleranceSeconds) * time.Second))
		if held != nil {
			created, _ := utc(held.CreatedAt)
			clockValid = clockValid && !occurred.Before(created.Add(-time.Duration(r.config.Policy.ClockToleranceSeconds)*time.Second))
		}
		control, controlled := view.read.Snapshot.Credential(target.CredentialRef)
		currentCredential := controlled && control.Enabled && control.Generation == target.CredentialGeneration
		var scopes []RuntimeScope
		for _, id := range request.AffectedScopeIDs {
			current, found := view.scopes[id]
			if !found || !scopeApplies(current.State.Identity, target) {
				unknown()
				return
			}
			matching := false
			if held != nil {
				expires, _ := utc(held.ExpiresAt)
				control, controlled := view.read.Snapshot.Credential(held.CredentialRef)
				for _, scope := range held.Scopes {
					if scope.ScopeID == id && current.State.ActiveClaim != nil && *current.State.ActiveClaim == held.ClaimID &&
						current.State.RecoveryEpoch == scope.Epoch && current.State.FailureGeneration == scope.FailureGeneration && expires.After(now) &&
						clockValid && controlled && control.Enabled && control.Generation == held.CredentialGeneration && control.Revision == held.CredentialControlRevision {
						matching = true
					}
				}
			}
			effect := request.Effect.Kind
			if !currentCredential {
				effect = "historical"
			}
			switch effect {
			case "rate_limited":
				if matching && current.State.BackoffStep < uint64(len(r.config.Policy.BackoffSeconds)-1) {
					current.State.BackoffStep++
				}
				step := current.State.BackoffStep
				if step >= uint64(len(r.config.Policy.BackoffSeconds)) {
					step = uint64(len(r.config.Policy.BackoffSeconds) - 1)
				}
				basis := occurred
				if !clockValid {
					basis = now
				}
				deadline := basis.Add(time.Duration(r.config.Policy.BackoffSeconds[step]) * time.Second)
				if clockValid && request.Effect.RetryAfter != nil {
					provider, _ := utc(*request.Effect.RetryAfter)
					if provider.After(deadline) {
						deadline = provider
					}
				}
				if current.State.BlockedUntil != nil {
					old, _ := utc(*current.State.BlockedUntil)
					if old.After(deadline) {
						deadline = old
					}
				}
				until := deadline.UTC().Format(time.RFC3339Nano)
				current.State.BlockedUntil = &until
				current.State.FailureGeneration, err = nextRuntimeRevision(current.State.FailureGeneration)
				if err != nil {
					unknown()
					return
				}
				current.State.LastEventID = &request.EventID
				if matching {
					current.State.ActiveClaim = nil
					current.ClaimExpiresAt = nil
				}
			case "recovery_succeeded", "recovery_accepted":
				if matching && current.State.PermanentReason == nil {
					current.State.BlockedUntil = nil
					current.State.BackoffStep = 0
					current.State.ActiveClaim = nil
					current.ClaimExpiresAt = nil
					current.State.RecoveredAt = &request.OccurredAt
					current.State.LastEventID = &request.EventID
				}
			case "neutral":
				if matching {
					deadline := now.Add(time.Duration(r.config.Policy.InconclusiveWaitSeconds) * time.Second)
					if current.State.BlockedUntil != nil {
						old, _ := utc(*current.State.BlockedUntil)
						if old.After(deadline) {
							deadline = old
						}
					}
					until := deadline.UTC().Format(time.RFC3339Nano)
					current.State.BlockedUntil = &until
					current.State.ActiveClaim = nil
					current.ClaimExpiresAt = nil
					current.State.LastEventID = &request.EventID
				}
			}
			current.State.Revision, err = nextRuntimeRevision(current.State.Revision)
			if err != nil {
				unknown()
				return
			}
			scopes = append(scopes, current)
		}
		revision, err := nextRuntimeRevision(view.revision)
		if err != nil {
			unknown()
			return
		}
		event := RuntimeEvent{SchemaVersion: RuntimeSchema, AuthorityID: r.config.Projection.AuthorityID, ProducerID: r.config.ProducerID, EventID: request.EventID,
			Kind: "outcome", Revision: revision, ControlRevision: view.read.Snapshot.Metadata().Revision,
			RequestDigest: bodybasis.DigestBytes(raw), RecordedAt: now.UTC().Format(time.RFC3339Nano), Scopes: scopes, Outcome: &request}
		receipt := OutcomeReceipt{EventID: request.EventID, Revision: view.read.Snapshot.Metadata().Revision, Decision: "runtime_committed", Scopes: []ScopeState{}, ReceivedAt: now.UTC().Format(time.RFC3339Nano)}
		for _, scope := range scopes {
			receipt.Scopes = append(receipt.Scopes, scope.State)
		}
		encoded, err := json.Marshal(receipt)
		if err != nil {
			unknown()
			return
		}
		err = r.commit(view, event, raw, encoded, true, func(status string, body []byte) {
			if status != "committed" && status != "replayed" {
				unknown()
				return
			}
			var stored OutcomeReceipt
			if gatewayJSON(body, &stored) != nil {
				unknown()
				return
			}
			r.mu.Lock()
			if bytes.Equal(r.pending[request.EventID].body, raw) {
				delete(r.pending, request.EventID)
			}
			if request.ClaimID != nil {
				delete(r.claims, *request.ClaimID)
			}
			r.mu.Unlock()
			callback(GatewayResult[OutcomeReceipt]{State: GatewayAcknowledged, Value: &stored})
		})
		if err != nil {
			unknown()
		}
	})
	return nil
}
