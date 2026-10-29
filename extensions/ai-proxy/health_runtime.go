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
	RuntimeConfigID string                         `json:"runtime_config_id"`
	ConfigRevision  upstreamhealth.Revision        `json:"config_revision"`
	Snapshot        managedHealthSnapshot          `json:"snapshot"`
	Target          upstreamhealth.TargetReference `json:"target"`
	Console         managedHealthConsole           `json:"console_internal"`
}

type managedHealthSnapshot struct {
	Prefix          string                  `json:"prefix"`
	AuthorityID     string                  `json:"authority_id"`
	MinimumRevision upstreamhealth.Revision `json:"minimum_revision"`
}

type managedHealthConsole struct {
	ServiceName string `json:"service_name"`
	ServicePort uint32 `json:"service_port"`
	Host        string `json:"host"`
	AuthToken   string `json:"auth_token"`
	TimeoutMS   uint32 `json:"timeout_ms"`
}

type managedHealthRuntime struct {
	settings managedHealthSettings
	owner    string
	reader   *upstreamhealth.Reader
	client   *upstreamhealth.GatewayClient
}

type managedHealthAttempt struct {
	scopeRevisions     map[string]upstreamhealth.Revision
	rateLimitScopes    []string
	claimScopes        []string
	claimID            *string
	claimDeadline      time.Time
	retryAfterEvidence *string
}

// This is one plugin VM's existing startup state, not a cross-VM context or a
// new recovery controller. Reload retains pending outcomes and read watermarks.
type managedHealthShared struct {
	endpointDigest string
	snapshotDigest string
	reader         *upstreamhealth.Reader
	client         *upstreamhealth.GatewayClient
}

var managedHealthStartup = struct {
	sync.Mutex
	owner       string
	byAuthority map[string]*managedHealthShared
}{byAuthority: map[string]*managedHealthShared{}}

// managedGatewayHealthClient is shared by native text and media. Both retain
// one exact pending/veto/read/ACK state for their original startup and authority.
func managedGatewayHealthClient(authorityID string, endpoint healthGatewayEndpoint) (*upstreamhealth.GatewayClient, string, error) {
	managedHealthStartup.Lock()
	defer managedHealthStartup.Unlock()
	owner := managedHealthStartup.owner
	if owner == "" {
		return nil, "", errors.New("managed health requires the actual host node and VM startup identity")
	}
	raw, _ := json.Marshal(endpoint)
	digest := bodybasis.DigestBytes(raw)
	key := owner + "/" + authorityID
	if shared, ok := managedHealthStartup.byAuthority[key]; ok {
		if shared.endpointDigest != digest {
			return nil, "", errors.New("managed health authority changed its registered endpoint during reload")
		}
		return shared.client, owner, nil
	}
	client, err := newHealthGatewayClient(endpoint, owner)
	if err != nil {
		return nil, "", err
	}
	managedHealthStartup.byAuthority[key] = &managedHealthShared{endpointDigest: digest, client: client}
	return client, owner, nil
}

// Shared by managed text and media leaves; both use exactly the startup value
// captured by the one host/VM hook rather than inventing another epoch.
func managedGatewayOwnerInstance() string {
	managedHealthStartup.Lock()
	defer managedHealthStartup.Unlock()
	return managedHealthStartup.owner
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
	d, t := config.release.Descriptor, settings.Target
	if t.TargetID != d.TargetID || t.BindingID != d.BindingID || t.ProviderAccountID != d.ProviderAccountID || t.RegionEndpointRef != d.RegionEndpointRef ||
		t.UpstreamModel != d.UpstreamModel || t.OperationKind != "text_generate" || t.CredentialRef != config.release.CredentialRef ||
		string(t.CredentialGeneration) != config.release.CredentialRevision || len(t.ScopeIDs) == 0 || len(t.ScopeIDs) > 128 {
		return nil, errors.New("managed health target differs from the original frozen selection")
	}
	if _, err := upstreamhealth.ParseRevision(string(t.CredentialGeneration)); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, scope := range t.ScopeIDs {
		if !managedID(scope) || seen[scope] {
			return nil, errors.New("managed health scopes are invalid")
		}
		seen[scope] = true
	}
	endpoint := settings.Console
	client, owner, err := managedGatewayHealthClient(settings.Snapshot.AuthorityID, healthGatewayEndpoint{
		ServiceName: endpoint.ServiceName, ServicePort: endpoint.ServicePort, Host: endpoint.Host, AuthToken: endpoint.AuthToken, TimeoutMS: endpoint.TimeoutMS})
	if err != nil {
		return nil, err
	}
	managedHealthStartup.Lock()
	defer managedHealthStartup.Unlock()
	shared := managedHealthStartup.byAuthority[owner+"/"+settings.Snapshot.AuthorityID]
	transport, _ := json.Marshal([]interface{}{settings.Snapshot.Prefix, config.settings.Redis})
	transportDigest := bodybasis.DigestBytes(transport)
	if shared.reader != nil && shared.snapshotDigest != transportDigest {
		return nil, errors.New("managed health authority changed its original snapshot source during reload")
	}
	if shared.reader == nil {
		redis, ok := config.redis.(upstreamhealth.RedisReader)
		if !ok {
			return nil, errors.New("managed health requires the original Redis snapshot reader")
		}
		// HMGet is the embedded foreground RedisClusterClient method. Only Command
		// uses the existing event client's root-owned background dispatcher.
		projection.MinimumRevision = "0"
		reader, err := upstreamhealth.NewReader(redis, projection)
		if err != nil {
			return nil, err
		}
		shared.snapshotDigest, shared.reader = transportDigest, reader
	}
	// Own all slice fields independently of the config parser's temporary object.
	retained := *settings
	retained.Target.ScopeIDs = append([]string(nil), settings.Target.ScopeIDs...)
	return &managedHealthRuntime{settings: retained, owner: owner, reader: shared.reader, client: client}, nil
}

func authorizeManagedHealth(ctx wrapper.HttpContext, state *managedExecutionState, basis bodybasis.Snapshot, request managedProtocolRequest) {
	health := state.config.health
	state.healthInFlight = true
	lookup := func() {
		health.reader.Lookup(func(read upstreamhealth.ReadResult) {
			if state.closed || state.dispatchStarted {
				return
			}
			if read.Status == upstreamhealth.ReadReady && read.Snapshot != nil {
				actual, minimum := string(read.Snapshot.Metadata().Revision), string(health.settings.Snapshot.MinimumRevision)
				if len(actual) < len(minimum) || len(actual) == len(minimum) && actual < minimum {
					rejectManagedHealth(ctx, state, "health_revision_below_registered_minimum", nil)
					return
				}
			}
			decision := health.client.DecideNewCall(read, upstreamhealth.Candidate{Target: health.settings.Target}, time.Now())
			if decision.Kind != upstreamhealth.Eligible && decision.Kind != upstreamhealth.RecoveryRequired {
				rejectManagedHealth(ctx, state, "upstream_health_unavailable", decision.RetryAfterSeconds)
				return
			}
			state.health = &managedHealthAttempt{scopeRevisions: map[string]upstreamhealth.Revision{}}
			for scope, version := range decision.ScopeVersions {
				state.health.scopeRevisions[scope] = version.Revision
			}
			modelScopes, _, err := read.Snapshot.ModelOperationScopes(health.settings.Target, time.Now())
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
			err = health.client.Claim(claimRequest, func(result upstreamhealth.GatewayResult[upstreamhealth.ClaimReceipt]) {
				if state.closed || state.dispatchStarted {
					return
				}
				if result.State != upstreamhealth.GatewayAcknowledged || result.Value == nil || result.Value.Decision != "granted" || result.Value.Claim == nil {
					rejectManagedHealth(ctx, state, "health_recovery_not_granted", nil)
					return
				}
				allocated := *result.Value.Claim
				final, err := candidate.FinalSelection(allocated.ClaimID)
				if err != nil {
					rejectManagedHealth(ctx, state, "health_final_selection_invalid", nil)
					return
				}
				err = health.client.Consume(allocated, final, func(consumed upstreamhealth.GatewayResult[upstreamhealth.RecoveryClaim]) {
					if state.closed || state.dispatchStarted {
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
					state.frozenSelection = append([]byte(nil), final...)
					dispatchManagedPreparedRequest(ctx, state, basis, request)
				})
				if err != nil {
					rejectManagedHealth(ctx, state, "health_consume_invalid", nil)
				}
			})
			if err != nil {
				rejectManagedHealth(ctx, state, "health_claim_invalid", nil)
			}
		})
	}
	// Retry the exact pending packets first so a fast Redis read cannot close the
	// HTTP context before a durable outcome ACK callback arrives. This runs only
	// on natural traffic, never as a timer/probe or another business send.
	pending := health.client.PendingOutcomes(health.settings.Target)
	if len(pending) == 0 {
		lookup()
		return
	}
	remaining := len(pending)
	complete := func() {
		remaining--
		if remaining == 0 && !state.closed {
			lookup()
		}
	}
	for _, original := range pending {
		err := health.client.ReportOutcome(original, func(upstreamhealth.GatewayResult[upstreamhealth.OutcomeReceipt]) { complete() })
		if err != nil {
			complete()
		}
	}
}

func rejectManagedHealth(ctx wrapper.HttpContext, state *managedExecutionState, reason string, retryAfter *uint64) {
	if state.closed || state.dispatchStarted {
		return
	}
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
	err = health.client.ReportOutcome(request, func(result upstreamhealth.GatewayResult[upstreamhealth.OutcomeReceipt]) {
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
