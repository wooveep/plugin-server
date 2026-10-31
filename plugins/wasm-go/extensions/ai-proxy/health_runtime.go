package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

type managedHealthSettings struct {
	RuntimeConfigID string                           `json:"runtime_config_id"`
	ConfigRevision  upstreamhealth.Revision          `json:"config_revision"`
	Snapshot        managedHealthSnapshot            `json:"snapshot"`
	Target          upstreamhealth.TargetReference   `json:"target"`
	Targets         []upstreamhealth.TargetReference `json:"targets,omitempty"`
}

type managedHealthSnapshot struct {
	Prefix          string                  `json:"prefix"`
	AuthorityID     string                  `json:"authority_id"`
	MinimumRevision upstreamhealth.Revision `json:"minimum_revision"`
}

type managedHealthRuntime struct {
	settings managedHealthSettings
	owner    string
	runtime  *upstreamhealth.Runtime
}

type managedHealthAttempt struct {
	scopeRevisions     map[string]upstreamhealth.Revision
	rateLimitScopes    []string
	claimScopes        []string
	claimID            *string
	claimDeadline      time.Time
	retryAfterEvidence *string
}

type managedHealthMember struct {
	credential managedCredential
	target     upstreamhealth.TargetReference
}

type managedHealthChoice struct {
	member   managedHealthMember
	decision upstreamhealth.Decision
}

type managedHealthDecider interface {
	DecideNewCall(upstreamhealth.ReadResult, upstreamhealth.Candidate, time.Time) upstreamhealth.Decision
}

func eligibleManagedHealthMembers(client managedHealthDecider, read upstreamhealth.ReadResult, members []managedHealthMember, now time.Time) ([]managedHealthChoice, *uint64) {
	var eligible []managedHealthChoice
	var retryAfter *uint64
	for _, member := range members {
		decision := client.DecideNewCall(read, upstreamhealth.Candidate{Target: member.target}, now)
		if decision.Kind == upstreamhealth.Eligible || decision.Kind == upstreamhealth.RecoveryRequired {
			eligible = append(eligible, managedHealthChoice{member, decision})
		} else if decision.RetryAfterSeconds != nil && (retryAfter == nil || *decision.RetryAfterSeconds < *retryAfter) {
			value := *decision.RetryAfterSeconds
			retryAfter = &value
		}
	}
	return eligible, retryAfter
}

// A v2 directory covers every original member/declared operation exactly once.
// This is static source binding, not a choice or a substitute for a fresh view.
func managedHealthMembers(config *managedExecutionConfig) ([]managedHealthMember, error) {
	settings := config.settings.UpstreamHealth
	if settings == nil || settings.RuntimeConfigID != config.release.Descriptor.ReleaseID || settings.ConfigRevision != "1" {
		return nil, errors.New("managed health runtime differs from the retained release registration")
	}
	members, err := config.credentialMembers()
	if err != nil {
		return nil, err
	}
	targets := settings.Targets
	if config.release.SchemaVersion == "modelfusion-execution.v1" {
		if len(targets) != 0 {
			return nil, errors.New("managed health v1 does not accept a target pool")
		}
		targets = []upstreamhealth.TargetReference{settings.Target}
	} else {
		t := settings.Target
		if t.TargetID != "" || t.BindingID != "" || t.ProviderAccountID != "" || t.RegionEndpointRef != "" || t.UpstreamModel != "" || t.OperationKind != "" ||
			t.CredentialRef != "" || t.CredentialGeneration != "" || len(t.ScopeIDs) != 0 || len(targets) != len(members)*len(config.release.Descriptor.Capabilities) {
			return nil, errors.New("managed health requires the complete unambiguous target directory")
		}
	}
	byRef := make(map[string]managedCredential, len(members))
	for _, member := range members {
		byRef[member.CredentialRef] = member
	}
	d := config.release.Descriptor
	seen := make(map[[2]string]bool, len(targets))
	result := make([]managedHealthMember, 0, len(targets))
	for _, t := range targets {
		member, exists := byRef[t.CredentialRef]
		_, declared := d.Capabilities[t.OperationKind]
		key := [2]string{t.CredentialRef, t.OperationKind}
		if !exists || !declared || seen[key] || t.TargetID != d.TargetID || t.BindingID != d.BindingID || t.ProviderAccountID != d.ProviderAccountID ||
			t.RegionEndpointRef != d.RegionEndpointRef || t.UpstreamModel != d.UpstreamModel || string(t.CredentialGeneration) != member.CredentialRevision ||
			len(t.ScopeIDs) == 0 || len(t.ScopeIDs) > 128 {
			return nil, errors.New("managed health target differs from the original frozen member")
		}
		if _, err := upstreamhealth.ParseRevision(string(t.CredentialGeneration)); err != nil {
			return nil, err
		}
		seen[key] = true
		scopes := make(map[string]bool, len(t.ScopeIDs))
		for _, scope := range t.ScopeIDs {
			if !managedID(scope) || scopes[scope] {
				return nil, errors.New("managed health scopes are invalid")
			}
			scopes[scope] = true
		}
		t.ScopeIDs = append([]string(nil), t.ScopeIDs...)
		if t.OperationKind == managedOperation(config) {
			result = append(result, managedHealthMember{member, t})
		}
	}
	if config.settings.MediaDispatchMode != "managed_operation" && len(result) != len(members) {
		return nil, errors.New("managed health text member directory is incomplete")
	}
	return result, nil
}

// Freeze one selected tuple into private request copies. The original complete
// configuration, its arrays and proxy_digest remain unchanged for all requests.
func freezeManagedHealthMember(state *managedExecutionState, member managedHealthMember) error {
	if state.credentialSelected || state.config == nil || state.config.health == nil {
		return errors.New("managed health credential was already selected or is unavailable")
	}
	original := state.config
	retained := *original
	retained.release.CredentialRef, retained.release.CredentialRevision = member.credential.CredentialRef, member.credential.CredentialRevision
	retained.settings.CredentialValue = member.credential.CredentialValue
	health := *original.health
	health.settings.Target = member.target
	health.settings.Target.ScopeIDs = append([]string(nil), member.target.ScopeIDs...)
	retained.health = &health
	state.config, state.credentialSelected = &retained, true
	return nil
}

// This is one plugin VM's existing startup state, not a cross-VM context or a
// new recovery controller. Reload retains pending outcomes and read watermarks.
type managedHealthShared struct {
	snapshotDigest string
	runtime        *upstreamhealth.Runtime
}

var managedHealthStartup = struct {
	sync.Mutex
	owner       string
	byAuthority map[string]*managedHealthShared
}{byAuthority: map[string]*managedHealthShared{}}

// managedGatewayHealthRuntime shares one original Redis state and pending veto
// across text/media configurations in the same actual host/VM startup.
func managedGatewayHealthRuntime(settings *managedHealthSettings, redis upstreamhealth.RuntimeRedis, stream, producerID string) (*upstreamhealth.Runtime, string, error) {
	managedHealthStartup.Lock()
	defer managedHealthStartup.Unlock()
	owner := managedHealthStartup.owner
	if owner == "" || settings == nil {
		return nil, "", errors.New("managed health requires the actual host node and VM startup identity")
	}
	projection := upstreamhealth.Config{Prefix: settings.Snapshot.Prefix, AuthorityID: settings.Snapshot.AuthorityID, MinimumRevision: "0"}
	if _, err := projection.SnapshotKey(); err != nil {
		return nil, "", err
	}
	raw, _ := json.Marshal([]string{projection.Prefix, stream, producerID})
	digest := bodybasis.DigestBytes(raw)
	key := owner + "/" + projection.AuthorityID
	if shared, ok := managedHealthStartup.byAuthority[key]; ok {
		if shared.snapshotDigest != digest {
			return nil, "", errors.New("managed health authority changed its original Redis/Stream during reload")
		}
		return shared.runtime, owner, nil
	}
	runtime, err := upstreamhealth.NewRuntime(redis, upstreamhealth.RuntimeConfig{Projection: projection, Stream: stream,
		ProducerID: producerID, OwnerInstance: owner, Policy: upstreamhealth.DefaultRuntimePolicy()})
	if err != nil {
		return nil, "", err
	}
	managedHealthStartup.byAuthority[key] = &managedHealthShared{snapshotDigest: digest, runtime: runtime}
	return runtime, owner, nil
}

// Shared by managed text and media leaves; both use exactly the startup value
// captured by the one host/VM hook rather than inventing another epoch.
func managedGatewayOwnerInstance() string {
	managedHealthStartup.Lock()
	defer managedHealthStartup.Unlock()
	return managedHealthStartup.owner
}

func healthHeaderValue(value string) bool {
	if len(value) == 0 || len(value) > 8192 {
		return false
	}
	for _, char := range value {
		if char <= 0x20 || char >= 0x7f {
			return false
		}
	}
	return true
}

func initializeManagedHealthRuntime(ctx wrapper.PluginContext) error {
	instance, ok := ctx.(interface{ RuntimeInstanceID() string })
	nodeID, err := proxywasm.GetProperty([]string{"node", "id"})
	owner := ""
	if ok && err == nil && managedID(string(nodeID)) && managedID(instance.RuntimeInstanceID()) {
		owner = string(nodeID) + "/" + instance.RuntimeInstanceID()
		// D0's original Id budget applies to this complete source-derived value.
		if len(owner) > 200 || !healthHeaderValue(owner) {
			owner = ""
		}
	}
	managedHealthStartup.Lock()
	managedHealthStartup.owner = owner
	managedHealthStartup.Unlock()
	// Standalone routes do not need a health owner. A managed health declaration
	// is rejected below if either real source could not be read; no fallback ID.
	return nil
}

func configureManagedHealth(config *managedExecutionConfig) (*managedHealthRuntime, error) {
	settings := config.settings.UpstreamHealth
	if settings == nil {
		return nil, nil
	}
	if settings.RuntimeConfigID != config.release.Descriptor.ReleaseID || settings.ConfigRevision != "1" {
		return nil, errors.New("managed health runtime differs from the retained release registration")
	}
	projection := upstreamhealth.Config{Prefix: settings.Snapshot.Prefix, AuthorityID: settings.Snapshot.AuthorityID, MinimumRevision: settings.Snapshot.MinimumRevision}
	if _, err := projection.SnapshotKey(); err != nil {
		return nil, err
	}
	if _, err := managedHealthMembers(config); err != nil {
		return nil, err
	}
	redis, ok := config.redis.(upstreamhealth.RuntimeRedis)
	if !ok {
		return nil, errors.New("managed health requires the existing foreground and root event Redis client")
	}
	runtime, owner, err := managedGatewayHealthRuntime(settings, redis, config.settings.PrivateStream, config.settings.ProducerID)
	if err != nil {
		return nil, err
	}
	// Own all slice fields independently of the config parser's temporary object.
	retained := *settings
	retained.Target.ScopeIDs = append([]string(nil), settings.Target.ScopeIDs...)
	retained.Targets = append([]upstreamhealth.TargetReference(nil), settings.Targets...)
	for index := range retained.Targets {
		retained.Targets[index].ScopeIDs = append([]string(nil), settings.Targets[index].ScopeIDs...)
	}
	return &managedHealthRuntime{settings: retained, owner: owner, runtime: runtime}, nil
}

func authorizeManagedHealth(ctx wrapper.HttpContext, state *managedExecutionState, basis bodybasis.Snapshot, body []byte) {
	health := state.config.health
	state.healthInFlight = true
	members, err := managedHealthMembers(state.config)
	if err != nil {
		rejectManagedHealth(ctx, state, "health_member_directory_invalid", nil)
		return
	}
	members = managedTextUntried(state, members)
	if len(members) == 0 {
		rejectManagedHealth(ctx, state, "upstream_health_unavailable", nil)
		return
	}
	// The asynchronous snapshot callback owns these exact mapped bytes; it does
	// not re-read or retain a caller-mutable body buffer after qualification.
	body = append([]byte(nil), body...)
	lookup := func() {
		health.runtime.Lookup(func(read upstreamhealth.ReadResult) {
			if managedTextStopped(state) || state.dispatchStarted || state.credentialSelected {
				return
			}
			if read.Status == upstreamhealth.ReadReady && read.Snapshot != nil {
				actual, minimum := string(read.Snapshot.Metadata().Revision), string(health.settings.Snapshot.MinimumRevision)
				if len(actual) < len(minimum) || len(actual) == len(minimum) && actual < minimum {
					rejectManagedHealth(ctx, state, "health_revision_below_registered_minimum", nil)
					return
				}
			}
			// One original snapshot qualifies the complete pool. Pending local veto,
			// disabled controls and old generations remain in the original decider.
			now := time.Now()
			eligible, retryAfter := eligibleManagedHealthMembers(health.runtime, read, members, now)
			if len(eligible) == 0 {
				rejectManagedHealth(ctx, state, "upstream_health_unavailable", retryAfter)
				return
			}
			index, err := managedUniformMember(len(eligible))
			if err != nil || freezeManagedHealthMember(state, eligible[index].member) != nil {
				rejectManagedHealth(ctx, state, "health_member_choice_unavailable", nil)
				return
			}
			selected := eligible[index]
			decision := selected.decision
			request, valid := prepareManagedSelectedRequest(ctx, state, body)
			if !valid {
				return
			}
			state.health = &managedHealthAttempt{scopeRevisions: map[string]upstreamhealth.Revision{}}
			for scope, version := range decision.ScopeVersions {
				state.health.scopeRevisions[scope] = version.Revision
			}
			modelScopes, _, err := read.Snapshot.ModelOperationScopes(selected.member.target, now)
			if err != nil {
				rejectManagedHealth(ctx, state, "health_rate_limit_scope_unknown", nil)
				return
			}
			state.health.rateLimitScopes = append([]string(nil), modelScopes...)
			if decision.Kind == upstreamhealth.Eligible {
				dispatchManagedPreparedRequest(ctx, state, basis, request)
				return
			}
			candidate, err := upstreamhealth.NewRecoveryCandidate(stateSelection(state, basis), health.settings.RuntimeConfigID, health.settings.ConfigRevision, nil)
			if err != nil {
				rejectManagedHealth(ctx, state, "health_candidate_invalid", nil)
				return
			}
			claimRequest := upstreamhealth.ClaimRequest{ClaimRequestID: "health:" + state.attemptID, OwnerInstance: health.owner,
				ExpectedEpochs: decision.ScopeEpochs, Candidate: candidate, CandidateDigest: candidate.Digest()}
			err = health.runtime.ClaimConsumed(claimRequest, selected.member.target, func(consumed upstreamhealth.GatewayResult[upstreamhealth.RecoveryClaim]) {
				if managedTextStopped(state) || state.dispatchStarted {
					return
				}
				if consumed.State != upstreamhealth.GatewayAcknowledged || consumed.Value == nil {
					rejectManagedHealth(ctx, state, "health_consume_unconfirmed", nil)
					return
				}
				deadline, err := time.Parse(time.RFC3339Nano, consumed.Value.ExpiresAt)
				if err != nil {
					rejectManagedHealth(ctx, state, "health_consume_unconfirmed", nil)
					return
				}
				id := consumed.Value.ClaimID
				state.health.claimID, state.health.claimDeadline = &id, deadline
				for _, scope := range consumed.Value.Scopes {
					state.health.claimScopes = append(state.health.claimScopes, scope.ScopeID)
				}
				state.frozenSelection = append([]byte(nil), consumed.Value.ConsumedSelection...)
				dispatchManagedPreparedRequest(ctx, state, basis, request)
			})
			if err != nil {
				rejectManagedHealth(ctx, state, "health_claim_invalid", nil)
			}

		})
	}
	// Root-owned retries retain exact health records. Current requests perform
	// their own Redis lookup without waiting for Console or an outcome ACK.
	health.runtime.FlushPending()
	lookup()
}

func rejectManagedHealth(ctx wrapper.HttpContext, state *managedExecutionState, reason string, retryAfter *uint64) {
	if managedTextStopped(state) || state.dispatchStarted {
		return
	}
	if state.healthUnavailable != nil {
		unavailable := state.healthUnavailable
		state.healthUnavailable = nil
		unavailable(reason)
		return
	}
	recordManagedAccessFailure(ctx, reason, false)
	ctx.SetUserAttribute("managed_execution_rejection", reason)
	headers := [][2]string{{"content-type", "application/json"}}
	if retryAfter != nil {
		headers = append(headers, [2]string{"retry-after", strconv.FormatUint(*retryAfter, 10)})
	}
	_ = proxywasm.SendHttpResponseWithDetail(503, "ai-proxy.managed_execution", headers, managedErrorBody(reason), -1)
}

func reportManagedHealthOutcome(state *managedExecutionState, snapshot executionfacts.Snapshot) {
	health := state.config.health
	if health == nil || state.health == nil {
		return
	}
	e := snapshot.Envelope()
	if state.health.claimID == nil && (e.HTTPStatus == nil || *e.HTTPStatus != 429) {
		return
	}
	affectedScopes := state.health.claimScopes
	if e.HTTPStatus != nil && *e.HTTPStatus == 429 {
		affectedScopes = state.health.rateLimitScopes
	}
	selection := upstreamhealth.AttemptSelection{RuntimeConfigID: health.settings.RuntimeConfigID, ConfigRevision: health.settings.ConfigRevision,
		TargetID: state.original.TargetID, CredentialRef: state.original.CredentialRef, CredentialGeneration: upstreamhealth.Revision(state.original.CredentialRevision),
		OperationKind: state.original.OperationKind, CallID: state.callID, AttemptID: state.attemptID, SelectionDigest: state.original.SelectionDigest}
	request, err := upstreamhealth.OutcomeFromAttempt(snapshot, selection, affectedScopes, state.health.scopeRevisions,
		"fact:"+snapshot.EventID()+":"+snapshot.Digest(), state.health.claimID)
	if err != nil {
		log.Warn("upstream health original outcome could not be retained")
		return
	}
	if snapshot.Envelope().HTTPStatus == nil {
		request.ObservationPhase = "transport_outcome"
	}
	if request.Effect.Kind == "rate_limited" && state.health.retryAfterEvidence != nil {
		evidence := *state.health.retryAfterEvidence
		request.Effect.RetryAfterEvidence = &evidence
	}
	err = health.runtime.ReportOutcome(request, health.settings.Target, func(result upstreamhealth.GatewayResult[upstreamhealth.OutcomeReceipt]) {
		// Captured client state only: an original customer context may be deleted.
		if result.State != upstreamhealth.GatewayAcknowledged {
			log.Warn("upstream health original outcome remains unacknowledged")
		}
	})
	if err != nil {
		log.Warn("upstream health original outcome dispatch unavailable")
	}
}

// The raw HTTP value is parsed once at the actual response boundary. IF03's
// retry_after remains its original absolute UTC deadline. Valid raw evidence is
// retained by the health DTO; malformed input is a sanitized missing deadline.
func managedProviderDeadline(raw, observedAt string) (string, *string) {
	invalid := "provider_retry_after_invalid"
	observed, err := time.Parse(time.RFC3339Nano, observedAt)
	if err != nil || len(raw) == 0 || len(raw) > 128 {
		return "", &invalid
	}
	original := raw
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", &invalid
	}
	digits := true
	for _, char := range raw {
		if char < '0' || char > '9' {
			digits = false
			break
		}
	}
	var deadline time.Time
	if digits {
		seconds, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || seconds > uint64((1<<63-1)/int64(time.Second)) {
			return "", &invalid
		}
		deadline = observed.Add(time.Duration(seconds) * time.Second)
	} else {
		parsed, err := http.ParseTime(raw)
		if err != nil {
			return "", &invalid
		}
		deadline = parsed.UTC()
	}
	value := deadline.UTC().Format(time.RFC3339Nano)
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		return "", &invalid
	}
	evidence := original
	return value, &evidence
}
