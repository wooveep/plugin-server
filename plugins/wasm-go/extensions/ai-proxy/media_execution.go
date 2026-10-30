package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	managedMediaDispatchPath = "/__modelfusion/media/dispatch"
	managedMediaQueryPath    = "/__modelfusion/media/query"
	managedMediaContextKey   = "modelfusion.private_media_notice"
	managedMediaNoticeLimit  = 4096
	managedMediaPrivateLimit = 32 << 20
)

type managedMediaProfile struct {
	Profile                string `json:"profile"`
	AdapterRef             string `json:"adapter_ref"`
	AdapterRevision        string `json:"adapter_revision"`
	CreateClassifierRef    string `json:"create_classifier_ref"`
	QueryClassifierRef     string `json:"query_classifier_ref,omitempty"`
	QueryCapabilityRef     string `json:"query_capability_ref,omitempty"`
	OutputIdentityContract string `json:"output_identity_contract"`
}

// This global control destination is deployment-owned. Original target transport,
// credential and all accounting identity still come from the frozen REL archive.
type managedMediaSettings struct {
	ProducerID       string                `json:"producer_id"`
	Console          managedHealthConsole  `json:"console_internal"`
	Profiles         []managedMediaProfile `json:"profiles"`
	QueueCapacity    int                   `json:"queue_capacity"`
	MaxResponseBytes int                   `json:"max_response_bytes"`
}

type managedMediaNotice struct {
	SchemaVersion string `json:"schema_version"`
	DispatchID    string `json:"dispatch_id,omitempty"`
	OperationID   string `json:"operation_id,omitempty"`
	RequestID     string `json:"request_id,omitempty"`
}

type managedMediaACK struct {
	SchemaVersion string `json:"schema_version"`
	DispatchID    string `json:"dispatch_id,omitempty"`
	OperationID   string `json:"operation_id,omitempty"`
	RequestID     string `json:"request_id,omitempty"`
	Accepted      bool   `json:"accepted"`
}

type managedMediaPrivateReply struct {
	Status                        int
	HeadersComplete, BodyComplete bool
	Body                          []byte
}

type managedMediaServiceCall func(method, path string, body []byte, callback func(managedMediaPrivateReply)) error
type managedMediaProviderCall func(mediaexecution.Request, func(mediaexecution.Response)) error

type managedMediaReport struct {
	Wire    []byte
	EventID string
}

type managedMediaProjection struct {
	Outputs          []mediaexecution.UsageOutput     `json:"outputs"`
	References       []mediaexecution.OutputReference `json:"references"`
	Terminal         bool                             `json:"terminal"`
	ProviderRevision *string                          `json:"provider_revision"`
	QueryAttemptID   *string                          `json:"query_attempt_id"`
	OutputManifest   *mediaexecution.OutputManifest   `json:"output_manifest"`
	QueryObservation *string                          `json:"query_observation"`
}

type managedMediaRuntime struct {
	settings managedMediaSettings
	owner    string
	digest   string
	profiles map[string]mediaexecution.ProfileBinding
	jobs     map[string]*managedMediaJob
	sequence uint64
	call     managedMediaServiceCall
	provider managedMediaProviderCall
	now      func() time.Time
}

type managedMediaJob struct {
	runtime                        *managedMediaRuntime
	notice                         managedMediaNotice
	key, stage                     string
	live                           mediaexecution.Invocation
	deadline, retryAt              time.Time
	reportDeadline, healthDeadline time.Time
	executor                       *mediaexecution.Executor
	archive                        managedMediaArchive
	selection                      json.RawMessage
	request                        mediaexecution.Request
	attempt                        executionfacts.Attempt
	actual, generation             *executionfacts.Snapshot
	delivery                       *executionfacts.Snapshot
	healthAfterFact                *executionfacts.Snapshot
	manifest                       *mediaexecution.OutputManifest
	health                         *upstreamhealth.GatewayClient
	healthTarget                   upstreamhealth.TargetReference
	healthDecision                 upstreamhealth.Decision
	claimID                        *string
	claimScopes                    []string
	modelScopes                    []string
	scopeRevisions                 map[string]upstreamhealth.Revision
	reports                        []managedMediaReport
	reportInFlight                 bool
	healthInFlight                 bool
	providerCallbackSeen           bool
}

// Both fields are private originals from Console, never notification inputs.
// generation_observation is the complete committed FACT predecessor, not a
// reconstructed revision counter. health_target resolves the actual operation.
type managedMediaArchive struct {
	Profile               string                         `json:"profile"`
	Native                managedMediaNative             `json:"native_execution"`
	HealthTarget          upstreamhealth.TargetReference `json:"health_target"`
	GenerationObservation *string                        `json:"generation_observation"`
	DeliveryObservation   *string                        `json:"delivery_observation"`
	OutputManifest        *mediaexecution.OutputManifest `json:"output_manifest"`
}

type managedMediaNative struct {
	Health *managedHealthSettings `json:"upstream_health"`
}

var managedMediaActive *managedMediaRuntime
var managedMediaRetained []*managedMediaRuntime

// A frozen media route can be loaded without pretending it is a text provider.
// It has no ordinary request execution path; the parent rejects customer hits
// and the only sender remains the authenticated persisted-operation notice VM.
func validateManagedMediaRelease(c *managedExecutionConfig, value gjson.Result, cfg *config.PluginConfig) error {
	protocol, err := mediaexecution.LookupProfile(c.settings.InputProtocol)
	if err != nil || c.settings.UpstreamProtocol != c.settings.InputProtocol || c.settings.UpstreamPath != protocol.CreatePath ||
		c.release.Descriptor.UpstreamModel != protocol.Model || value.Get("managed_execution.media_dispatch_mode").String() != "managed_operation" {
		return errors.New("managed media route differs from its frozen native protocol")
	}
	// Config.Complete has already validated the selected business provider.
	// Native media uses the frozen profile's codec, not its text adapter family.
	providerValue := value.Get("provider")
	if providers := value.Get("providers"); providers.Exists() {
		// Config.FromJson keeps this original JSON unchanged and resolves the
		// active Provider by ID. A legacy object would override that resolution;
		// never use it as a fallback for a missing or ambiguous array identity.
		active := value.Get("activeProviderId")
		if !providers.IsArray() || providerValue.Exists() || active.Type != gjson.String || !managedID(active.String()) {
			return errors.New("managed media route requires its exact active provider identity")
		}
		matches := 0
		for _, candidate := range providers.Array() {
			if !candidate.IsObject() || candidate.Get("id").Type != gjson.String {
				return errors.New("managed media route contains an invalid provider identity")
			}
			if candidate.Get("id").String() == active.String() {
				matches++
				providerValue = candidate
			}
		}
		selected := cfg.GetProviderConfig()
		if matches != 1 || selected == nil || selected.GetId() != active.String() {
			return errors.New("managed media route active provider is absent or ambiguous")
		}
	} else if c.release.SchemaVersion != "modelfusion-execution.v1" || value.Get("activeProviderId").Exists() || !providerValue.IsObject() {
		// Preserve only the original sole-v1 legacy form, not an array fallback.
		return errors.New("managed media route requires its original provider configuration")
	}
	tokens := providerValue.Get("apiTokens")
	if !tokens.IsArray() {
		return errors.New("managed media route credential differs from its frozen provider")
	}
	values := make([]string, 0, len(tokens.Array()))
	for _, token := range tokens.Array() {
		if token.Type != gjson.String {
			return errors.New("managed media route credential differs from its frozen provider")
		}
		values = append(values, token.String())
	}
	if mediaexecution.ValidateCredentialConfiguration([]byte(value.Get("managed_execution").Raw), values) != nil {
		return errors.New("managed media route credential differs from its frozen provider")
	}
	expected := map[string]bool{protocol.OperationKind: true}
	if protocol.Asynchronous {
		expected["task_query"] = true
	}
	if len(c.release.Descriptor.Capabilities) != len(expected) {
		return errors.New("managed media route has capabilities outside its exact profile")
	}
	var queryRef string
	for operation, capability := range c.release.Descriptor.Capabilities {
		if !expected[operation] || capability.SendMode != "single_send" || capability.MaxAttempts != 1 || capability.MaxFallbackHops != 0 ||
			!managedID(capability.ResponseClassifierRef) || (protocol.Asynchronous && !managedID(capability.QueryCapabilityRef)) ||
			(!protocol.Asynchronous && capability.QueryCapabilityRef != "") {
			return errors.New("managed media route requires its exact single-send capability")
		}
		if queryRef != "" && queryRef != capability.QueryCapabilityRef {
			return errors.New("managed media route query capability differs between operations")
		}
		queryRef = capability.QueryCapabilityRef
	}
	return nil
}

// configureManagedMedia is called only for global config, never a per-customer
// override. Reload cannot replace a live boot's original endpoint or profile
// pins, or discard unacknowledged immutable reports.
func configureManagedMedia(value gjson.Result) error {
	// The wrapper clears tick registrations before each global parse/reload.
	// This parser is called once globally, never for each customer match rule.
	wrapper.RegisterTickFunc(100, tickManagedMedia)
	if !value.Exists() {
		managedMediaActive = nil
		return nil
	}
	var settings managedMediaSettings
	if err := decodeManagedMediaJSON([]byte(value.Raw), &settings); err != nil {
		return errors.New("managed_media control configuration is invalid")
	}
	if !managedID(settings.ProducerID) || settings.QueueCapacity < 1 || settings.QueueCapacity > 256 ||
		settings.MaxResponseBytes < 1024 || settings.MaxResponseBytes > 4<<20 || len(settings.Profiles) == 0 || len(settings.Profiles) > 64 {
		return errors.New("managed_media requires bounded work and retained profile pins")
	}
	owner := managedGatewayOwnerInstance()
	if owner == "" {
		return errors.New("managed_media requires the original host node and VM startup")
	}
	endpoint := settings.Console
	if _, err := newHealthGatewayClient(healthGatewayEndpoint{ServiceName: endpoint.ServiceName, ServicePort: endpoint.ServicePort,
		Host: endpoint.Host, AuthToken: endpoint.AuthToken, TimeoutMS: endpoint.TimeoutMS}, owner); err != nil {
		return err
	}
	profiles := make(map[string]mediaexecution.ProfileBinding, len(settings.Profiles))
	for _, profile := range settings.Profiles {
		if !managedID(profile.AdapterRef) || !managedID(profile.AdapterRevision) || !managedID(profile.CreateClassifierRef) {
			return errors.New("managed_media profile requires retained adapter and classifier references")
		}
		async := false
		identityContract := mediaexecution.NativeImageChoicesContent
		switch profile.Profile {
		case mediaexecution.Profile:
			async = true
		case mediaexecution.Wan30Text, mediaexecution.Wan30FirstFrame:
			async, identityContract = true, mediaexecution.TaskSingleVideo
		case mediaexecution.QwenImage30Compatible:
			identityContract = mediaexecution.CompatibleImageData
		case mediaexecution.QwenImage20ProSync, mediaexecution.QwenImage30NativeSync:
		default:
			return errors.New("managed_media profile has no actual native codec")
		}
		if _, duplicate := profiles[profile.Profile]; duplicate || profile.OutputIdentityContract != identityContract || async && (!managedID(profile.QueryClassifierRef) || !managedID(profile.QueryCapabilityRef)) ||
			!async && (profile.QueryClassifierRef != "" || profile.QueryCapabilityRef != "") {
			return errors.New("managed_media profile query capability differs from its protocol")
		}
		profiles[profile.Profile] = mediaexecution.ProfileBinding{AdapterRef: profile.AdapterRef, AdapterRevision: profile.AdapterRevision,
			CreateClassifierRef: profile.CreateClassifierRef, QueryClassifierRef: profile.QueryClassifierRef, QueryCapabilityRef: profile.QueryCapabilityRef,
			OutputIdentityContract: profile.OutputIdentityContract}
	}
	raw, _ := json.Marshal(settings)
	digest, err := mfjson.Digest(raw)
	if err != nil {
		return err
	}
	for _, retained := range managedMediaRetained {
		if retained.owner == owner {
			if retained.digest != digest {
				return errors.New("managed_media cannot replace its original control registration during this startup")
			}
			managedMediaActive = retained
			return nil
		}
	}
	runtime := &managedMediaRuntime{settings: settings, owner: owner, digest: digest, profiles: profiles,
		jobs: make(map[string]*managedMediaJob), now: time.Now}
	runtime.call = runtime.consoleCall
	runtime.provider = runtime.providerCall
	managedMediaRetained = append(managedMediaRetained, runtime)
	managedMediaActive = runtime
	return nil
}

func onManagedMediaRequestHeaders(ctx wrapper.HttpContext) (types.Action, bool) {
	if ctx.Path() != managedMediaDispatchPath && ctx.Path() != managedMediaQueryPath {
		return types.ActionContinue, false
	}
	ctx.SetContext(ctxSkipAIProxyLifecycle, true)
	ctx.DisableReroute()
	runtime := managedMediaActive
	if runtime == nil || runtime.owner != managedGatewayOwnerInstance() {
		return rejectManagedRequest(ctx, 503, "media_control_unavailable"), true
	}
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil || !authorizeManagedMediaNotice(headers, runtime.settings.Console.AuthToken) {
		return rejectManagedRequest(ctx, 403, "media_control_authentication_required"), true
	}
	contentType, err := managedSingleHeader(headers, "content-type")
	mediaType, _, parseErr := mime.ParseMediaType(contentType)
	if err != nil || parseErr != nil || mediaType != "application/json" || ctx.Method() != "POST" || !ctx.HasRequestBody() {
		return rejectManagedRequest(ctx, 400, "media_notice_json_required"), true
	}
	ctx.SetContext(managedMediaContextKey, runtime)
	ctx.SetRequestBodyBufferLimit(managedMediaNoticeLimit)
	return types.ActionContinue, true
}

func authorizeManagedMediaNotice(headers [][2]string, secret string) bool {
	value, err := managedSingleHeader(headers, "authorization")
	return err == nil && secret != "" && subtle.ConstantTimeCompare([]byte(value), []byte("Bearer "+secret)) == 1
}

func onManagedMediaRequestBody(ctx wrapper.HttpContext, body []byte) (types.Action, bool) {
	runtime, ok := ctx.GetContext(managedMediaContextKey).(*managedMediaRuntime)
	if !ok {
		return types.ActionContinue, false
	}
	ack, err := runtime.enqueue(ctx.Path(), body)
	if err != nil {
		return rejectManagedRequest(ctx, 503, "media_notice_not_queued"), true
	}
	wire, _ := json.Marshal(ack)
	_ = proxywasm.SendHttpResponse(202, [][2]string{{"content-type", "application/json"}, {"cache-control", "no-store"}}, wire, -1)
	return types.ActionPause, true
}

func (r *managedMediaRuntime) enqueue(path string, body []byte) (managedMediaACK, error) {
	var notice managedMediaNotice
	if len(body) > managedMediaNoticeLimit || decodeManagedMediaJSON(body, &notice) != nil {
		return managedMediaACK{}, errors.New("invalid media notice")
	}
	var key string
	ack := managedMediaACK{Accepted: true}
	switch path {
	case managedMediaDispatchPath:
		if notice.SchemaVersion != "media-dispatch-notice.v1" || !managedID(notice.DispatchID) || notice.OperationID != "" || notice.RequestID != "" {
			return managedMediaACK{}, errors.New("invalid original media dispatch notice")
		}
		key = "dispatch/" + notice.DispatchID
		ack.SchemaVersion, ack.DispatchID = "media-dispatch-ack.v1", notice.DispatchID
	case managedMediaQueryPath:
		if notice.SchemaVersion != "media-query-notice.v1" || !managedID(notice.OperationID) || !managedID(notice.RequestID) || notice.DispatchID != "" {
			return managedMediaACK{}, errors.New("invalid original media query notice")
		}
		key = managedMediaQueryAttempt(notice.OperationID, notice.RequestID)
		ack.SchemaVersion, ack.OperationID, ack.RequestID = "media-query-ack.v1", notice.OperationID, notice.RequestID
	default:
		return managedMediaACK{}, errors.New("unknown media control path")
	}
	if _, duplicate := r.jobs[key]; duplicate {
		return ack, nil
	}
	if len(r.jobs) >= r.settings.QueueCapacity || r.sequence == ^uint64(0) {
		return managedMediaACK{}, errors.New("media control queue is full")
	}
	r.sequence++
	sequence := strconv.FormatUint(r.sequence, 10)
	r.jobs[key] = &managedMediaJob{runtime: r, notice: notice, key: key, stage: "queued",
		live: mediaexecution.Invocation{ID: "media-invocation:" + sequence, Generation: sequence}}
	return ack, nil
}

func managedMediaQueryAttempt(operationID, requestID string) string {
	digest := sha256.Sum256([]byte(operationID + "\x00" + requestID))
	return "media-query:" + hex.EncodeToString(digest[:])
}

// Register once on the existing plugin root. All asynchronous work begins here,
// after notice ACK, rather than retaining a deleted customer/notice context.
func tickManagedMedia() {
	for _, runtime := range managedMediaRetained {
		runtime.tick()
	}
}

func (r *managedMediaRuntime) tick() {
	now := r.now()
	for key, job := range r.jobs {
		if job.stage == "queued" {
			job.fetch()
			continue
		}
		pendingHealth := job.health != nil && len(job.health.PendingOutcomes(job.healthTarget)) > 0
		if job.stage == "done" && len(job.reports) == 0 && !pendingHealth && job.healthAfterFact == nil && (job.actual == nil || job.providerCallbackSeen) {
			delete(r.jobs, key)
			continue
		}
		if job.stage == "sending" && !job.deadline.After(now) && !job.providerCallbackSeen {
			job.capture(mediaexecution.Response{SendState: "unknown", TransportError: "media_http_callback_not_observed", ObservedAt: now.UTC().Format(time.RFC3339Nano)})
		}
		if job.stage != "queued" && job.stage != "sending" && job.stage != "done" && !job.deadline.After(now) {
			// No late RPC response can cross this stage guard into a provider call.
			if job.executor != nil {
				job.executor.ConsumeUnknown()
			}
			job.stage = "done"
		}
		if len(job.reports) > 0 && !job.reportInFlight && !job.retryAt.After(now) {
			job.report()
		}
		if job.reportInFlight && !job.reportDeadline.After(now) {
			job.reportInFlight = false
		}
		if job.healthInFlight && !job.healthDeadline.After(now) {
			job.healthInFlight = false
		}
		if job.health != nil && !job.healthInFlight && !job.retryAt.After(now) {
			pending := job.health.PendingOutcomes(job.healthTarget)
			if len(pending) > 0 {
				job.healthInFlight = true
				job.healthDeadline = now.Add(time.Duration(r.settings.Console.TimeoutMS)*time.Millisecond + time.Second)
				if err := job.health.ReportOutcome(pending[0], func(upstreamhealth.GatewayResult[upstreamhealth.OutcomeReceipt]) { job.healthInFlight = false }); err != nil {
					job.healthInFlight = false
				}
				job.retryAt = now.Add(time.Second)
			}
		}
	}
}

func (j *managedMediaJob) fetch() {
	r := j.runtime
	j.stage, j.deadline = "fetching", r.now().Add(time.Duration(r.settings.Console.TimeoutMS)*time.Millisecond+time.Second)
	path := "/internal/media/dispatches/" + url.PathEscape(j.notice.DispatchID) + "/execution"
	if j.notice.OperationID != "" {
		path = "/internal/media/operations/" + url.PathEscape(j.notice.OperationID) + "/execution"
	}
	err := r.call("GET", path, nil, func(reply managedMediaPrivateReply) {
		if j.stage != "fetching" {
			return
		}
		raw, err := managedMediaData(reply)
		if err != nil || json.Unmarshal(raw, &j.archive) != nil {
			j.stage = "done"
			return
		}
		binding, supported := r.profiles[j.archive.Profile]
		if !supported {
			j.stage = "done"
			return
		}
		j.executor, err = mediaexecution.FromPrivateArchive(raw, mediaexecution.GatewayIdentity{
			ProducerID: r.settings.ProducerID, Actor: "service/gateway/" + r.settings.ProducerID, BootID: r.owner}, binding)
		if err != nil || j.notice.DispatchID != "" && j.executor.DispatchID() != j.notice.DispatchID ||
			j.notice.OperationID != "" && j.executor.OperationID() != j.notice.OperationID {
			j.stage = "done"
			return
		}
		if j.notice.DispatchID != "" && j.executor.DispatchState() != "prepared" {
			j.stage = "done"
			return
		}
		if j.archive.GenerationObservation != nil {
			original, err := mediaexecution.DecodeOriginalObservation([]byte(*j.archive.GenerationObservation))
			if err != nil {
				j.stage = "done"
				return
			}
			j.generation = &original
		}
		if j.archive.DeliveryObservation != nil {
			original, err := mediaexecution.DecodeOriginalObservation([]byte(*j.archive.DeliveryObservation))
			if err != nil || original.Envelope().Kind != "call_delivery" {
				j.stage = "done"
				return
			}
			j.delivery = &original
		}
		j.manifest = j.archive.OutputManifest
		if j.notice.OperationID != "" && j.generation == nil {
			// Keep the original task unknown until its source cursor is available.
			j.stage = "done"
			return
		}
		j.qualify()
	})
	if err != nil {
		j.stage = "done"
	}
}

func (j *managedMediaJob) qualify() {
	settings := j.archive.Native.Health
	if settings == nil || settings.RuntimeConfigID != j.executor.RuntimeConfigID() || string(settings.ConfigRevision) != j.executor.ConfigRevision() {
		j.stage = "done"
		return
	}
	// The original operation-specific registration supplies these scope IDs.
	// A query never edits a generation scope into a guessed task_query scope.
	target := j.archive.HealthTarget
	var descriptor struct {
		TargetID          string `json:"target_id"`
		BindingID         string `json:"binding_id"`
		ProviderAccountID string `json:"provider_account_id"`
		RegionEndpointRef string `json:"region_endpoint_ref"`
		UpstreamModel     string `json:"upstream_model"`
	}
	if json.Unmarshal(j.executor.Descriptor(), &descriptor) != nil {
		j.stage = "done"
		return
	}
	queryAttempt := ""
	if j.notice.OperationID != "" {
		queryAttempt = managedMediaQueryAttempt(j.notice.OperationID, j.notice.RequestID)
	}
	selection, err := j.executor.Selection(j.live, queryAttempt, nil)
	var frame struct {
		CredentialRef      string `json:"credential_ref"`
		CredentialRevision string `json:"credential_revision"`
		OperationKind      string `json:"operation_kind"`
	}
	if err != nil || json.Unmarshal(selection, &frame) != nil || target.TargetID != descriptor.TargetID || target.BindingID != descriptor.BindingID ||
		target.ProviderAccountID != descriptor.ProviderAccountID || target.RegionEndpointRef != descriptor.RegionEndpointRef || target.UpstreamModel != descriptor.UpstreamModel ||
		target.CredentialRef != frame.CredentialRef || string(target.CredentialGeneration) != frame.CredentialRevision || target.OperationKind != frame.OperationKind {
		j.stage = "done"
		return
	}
	j.selection, j.healthTarget = selection, target
	endpoint := settings.Console
	client, owner, err := managedGatewayHealthClient(settings.Snapshot.AuthorityID, healthGatewayEndpoint{ServiceName: endpoint.ServiceName,
		ServicePort: endpoint.ServicePort, Host: endpoint.Host, AuthToken: endpoint.AuthToken, TimeoutMS: endpoint.TimeoutMS})
	if err != nil || owner != j.runtime.owner {
		j.stage = "done"
		return
	}
	j.health = client
	j.stage, j.deadline = "qualifying", j.runtime.now().Add(time.Duration(endpoint.TimeoutMS)*time.Millisecond+time.Second)
	config := upstreamhealth.Config{Prefix: settings.Snapshot.Prefix, AuthorityID: settings.Snapshot.AuthorityID, MinimumRevision: settings.Snapshot.MinimumRevision}
	err = client.Snapshot(config, func(read upstreamhealth.ReadResult) {
		if j.stage != "qualifying" {
			return
		}
		decision := client.DecideNewCall(read, upstreamhealth.Candidate{Target: target}, j.runtime.now())
		if (decision.Kind != upstreamhealth.Eligible && decision.Kind != upstreamhealth.RecoveryRequired) || read.Snapshot == nil {
			j.stage = "done"
			return
		}
		modelScopes, _, err := read.Snapshot.ModelOperationScopes(target, j.runtime.now())
		if err != nil {
			j.stage = "done"
			return
		}
		j.modelScopes, j.healthDecision = modelScopes, decision
		j.scopeRevisions = make(map[string]upstreamhealth.Revision, len(decision.ScopeVersions))
		for scope, version := range decision.ScopeVersions {
			j.scopeRevisions[scope] = version.Revision
		}
		if decision.Kind == upstreamhealth.RecoveryRequired {
			j.claim()
		} else {
			j.consume()
		}
	})
	if err != nil {
		j.stage = "done"
	}
}

func (j *managedMediaJob) command() ([]byte, string, error) {
	if j.notice.OperationID != "" {
		body, err := json.Marshal(struct {
			OperationID string                             `json:"operation_id"`
			RequestID   string                             `json:"query_request_id"`
			Selection   json.RawMessage                    `json:"selection"`
			Epochs      map[string]upstreamhealth.Revision `json:"expected_scope_epochs"`
		}{j.notice.OperationID, j.notice.RequestID, j.selection, j.healthDecision.ScopeEpochs})
		return body, "/internal/media/operations/" + url.PathEscape(j.notice.OperationID), err
	}
	body, err := json.Marshal(struct {
		Revision  string                             `json:"expected_dispatch_revision"`
		Selection json.RawMessage                    `json:"selection"`
		Epochs    map[string]upstreamhealth.Revision `json:"expected_scope_epochs"`
	}{j.executor.DispatchRevision(), j.selection, j.healthDecision.ScopeEpochs})
	return body, "/internal/media/dispatches/" + url.PathEscape(j.notice.DispatchID), err
}

func (j *managedMediaJob) claim() {
	body, path, err := j.command()
	if err != nil || len(j.healthDecision.ScopeEpochs) == 0 {
		j.stage = "done"
		return
	}
	path += "/claim"
	if j.notice.OperationID != "" {
		path = strings.TrimSuffix(path, "/claim") + "/query-claim"
	}
	j.stage, j.deadline = "claiming", j.runtime.now().Add(time.Duration(j.runtime.settings.Console.TimeoutMS)*time.Millisecond+time.Second)
	err = j.runtime.call("POST", path, body, func(reply managedMediaPrivateReply) {
		if j.stage != "claiming" {
			return
		}
		raw, err := managedMediaData(reply)
		var recovery struct {
			Receipt struct {
				Decision string `json:"decision"`
				Claim    *struct {
					ID        string                        `json:"claim_id"`
					Actor     string                        `json:"actor"`
					ExpiresAt time.Time                     `json:"expires_at"`
					Scopes    []upstreamhealth.ClaimedScope `json:"scopes"`
					Request   struct {
						Owner string `json:"owner_instance"`
					} `json:"request"`
				} `json:"claim"`
			} `json:"receipt"`
		}
		if err != nil || json.Unmarshal(raw, &recovery) != nil || recovery.Receipt.Decision != "granted" || recovery.Receipt.Claim == nil ||
			!managedID(recovery.Receipt.Claim.ID) || recovery.Receipt.Claim.Request.Owner != j.runtime.owner ||
			recovery.Receipt.Claim.Actor != "service/gateway/"+j.runtime.settings.ProducerID || !recovery.Receipt.Claim.ExpiresAt.After(j.runtime.now()) {
			j.stage = "done"
			return
		}
		claim := recovery.Receipt.Claim
		if len(claim.Scopes) != len(j.healthDecision.ScopeEpochs) {
			j.stage = "done"
			return
		}
		for _, scope := range claim.Scopes {
			if epoch, exists := j.healthDecision.ScopeEpochs[scope.ScopeID]; !exists || epoch != scope.Epoch {
				j.stage = "done"
				return
			}
			j.claimScopes = append(j.claimScopes, scope.ScopeID)
		}
		queryAttempt := ""
		if j.notice.OperationID != "" {
			queryAttempt = managedMediaQueryAttempt(j.notice.OperationID, j.notice.RequestID)
		}
		j.selection, err = j.executor.Selection(j.live, queryAttempt, []string{claim.ID})
		if err != nil {
			j.stage = "done"
			return
		}
		j.claimID = &claim.ID
		j.consume()
	})
	if err != nil {
		j.stage = "done"
	}
}

func (j *managedMediaJob) consume() {
	body, path, err := j.command()
	if err != nil {
		j.stage = "done"
		return
	}
	path += "/consume"
	if j.notice.OperationID != "" {
		path = strings.TrimSuffix(path, "/consume") + "/query"
	}
	j.stage, j.deadline = "consuming", j.runtime.now().Add(time.Duration(j.runtime.settings.Console.TimeoutMS)*time.Millisecond+time.Second)
	err = j.runtime.call("POST", path, body, func(reply managedMediaPrivateReply) {
		if j.stage != "consuming" {
			return
		}
		raw, err := managedMediaData(reply)
		if err != nil {
			j.resolveUnknownConsume()
			return
		}
		if j.notice.OperationID == "" {
			j.request, err = j.executor.PrepareCreate(raw, j.selection, j.live, j.runtime.now())
		} else {
			j.request, err = j.executor.PrepareQuery(raw, j.runtime.now())
			if err == nil {
				actual, actualErr := mfjson.Canonicalize(j.request.Selection())
				expected, expectedErr := mfjson.Canonicalize(j.selection)
				if actualErr != nil || expectedErr != nil || !bytes.Equal(actual, expected) {
					err = mediaexecution.ErrIdentity
				}
			}
		}
		if err != nil {
			j.stage = "done"
			return
		}
		j.attempt, err = j.request.NewAttempt(j.runtime.settings.ProducerID)
		if err != nil {
			j.stage = "done"
			return
		}
		// The one committed permission is already irreversibly taken. Mark the
		// actual handoff before the host call; every failure remains this attempt.
		j.stage = "sending"
		j.deadline = j.runtime.now().Add(time.Duration(j.request.TimeoutMilliseconds())*time.Millisecond + time.Second)
		err = j.runtime.provider(j.request, func(response mediaexecution.Response) {
			if j.providerCallbackSeen {
				return
			}
			j.providerCallbackSeen = true
			j.capture(response)
		})
		if err != nil && !j.providerCallbackSeen {
			j.providerCallbackSeen = true
			j.capture(mediaexecution.Response{SendState: "not_sent", TransportError: "media_http_dispatch_failed", ObservedAt: j.runtime.now().UTC().Format(time.RFC3339Nano)})
		}
	})
	if err != nil {
		j.resolveUnknownConsume()
	}
}

func (j *managedMediaJob) resolveUnknownConsume() {
	j.executor.ConsumeUnknown()
	j.stage = "resolving_unknown"
	j.deadline = j.runtime.now().Add(time.Duration(j.runtime.settings.Console.TimeoutMS)*time.Millisecond + time.Second)
	// This GET has no grant and is never converted back into permission. Even a
	// consumed read only resolves the original command's durable state.
	path := "/internal/media/dispatches/" + url.PathEscape(j.executor.DispatchID())
	if err := j.runtime.call("GET", path, nil, func(managedMediaPrivateReply) { j.stage = "done" }); err != nil {
		j.stage = "done"
	}
}

func (j *managedMediaJob) capture(response mediaexecution.Response) {
	observation, err := j.request.Observe(response)
	if err != nil {
		j.stage = "done"
		return
	}
	projection, projectionErr := j.request.Project(response, observation, j.manifest)
	if projectionErr != nil {
		observation.GenerationUsage = &executionfacts.Usage{Status: "missing", Unit: observation.GenerationUsage.Unit, Source: "none", ReasonCode: "media_output_identity_or_measurement_unknown"}
		observation.DeliveryState = "unknown"
		observation.ActualAttempt.EvidenceRefs = append(observation.ActualAttempt.EvidenceRefs, "media-output-contract-conflict")
	} else if projection.Usage != nil {
		observation.GenerationUsage = projection.Usage
	} else {
		observation.GenerationUsage = &executionfacts.Usage{Status: "missing", Unit: observation.GenerationUsage.Unit, Source: "none", ReasonCode: "media_output_measurements_pending"}
	}
	if projection.Manifest != nil {
		manifestBytes, _ := json.Marshal(projection.Manifest)
		if digest, err := mfjson.Digest(manifestBytes); err == nil {
			observation.ActualAttempt.EvidenceRefs = append(observation.ActualAttempt.EvidenceRefs, "media-output-manifest:"+digest)
		}
		if projectionErr == nil {
			j.manifest = projection.Manifest
		}
	}
	if !j.request.IsQuery() {
		observation.ActualAttempt.Usage = observation.GenerationUsage
	}
	if retryAfter, err := managedSingleHeader(response.Headers, "retry-after"); err == nil {
		observation.ActualAttempt.RetryAfter, _ = managedProviderDeadline(retryAfter, response.ObservedAt)
	}
	actual, err := j.attempt.Finish(observation.ActualAttempt)
	if err == nil && j.actual != nil {
		actual, err = executionfacts.Correct(*j.actual, actual.Envelope())
	}
	if err != nil {
		log.Warn("media actual observation could not be frozen")
		j.stage = "done"
		return
	}
	j.actual = &actual
	j.reportHealth(actual, false)
	var queryAttempt, queryObservation *string
	if j.request.IsQuery() {
		id, original := j.request.AttemptID(), string(actual.Bytes())
		queryAttempt, queryObservation = &id, &original
		// The actual GET is an independent immutable audit source. MEDIA keeps
		// this callback without FACT/BILL purchase or generation-state changes.
		j.retainObservation(actual, managedMediaProjection{Outputs: []mediaexecution.UsageOutput{}, References: []mediaexecution.OutputReference{},
			QueryAttemptID: queryAttempt, QueryObservation: queryObservation})
	}
	if j.generation != nil && mediaexecution.EstimatedWouldDowngrade(j.generation.Envelope().Usage, observation.GenerationUsage) {
		j.stage = "done"
		return
	}
	generation, err := j.request.GenerationObservation(actual, observation, j.generation)
	if err == nil {
		j.generation = &generation
		wireProjection := managedMediaProjection{Outputs: projection.Outputs, References: projection.References,
			Terminal: observation.Terminal, QueryAttemptID: queryAttempt, QueryObservation: queryObservation, OutputManifest: projection.Manifest}
		if wireProjection.Outputs == nil {
			wireProjection.Outputs = []mediaexecution.UsageOutput{}
		}
		if wireProjection.References == nil {
			wireProjection.References = []mediaexecution.OutputReference{}
		}
		j.retainObservation(generation, wireProjection)
		delivery, err := j.request.DeliveryObservation(generation, observation, j.delivery)
		if err == nil {
			j.delivery = &delivery
			j.retainObservation(delivery, wireProjection)
		} else {
			log.Warn("media original delivery writer or source predecessor is unavailable")
		}
	}
	j.stage = "done"
	if len(j.reports) > 0 && !j.reportInFlight {
		j.report()
	}
}

// One immutable original is retained until its committed MEDIA/FACT receipt.
// Retrying this wire cannot invoke Consume or the provider again.
func (j *managedMediaJob) retainObservation(snapshot executionfacts.Snapshot, projection managedMediaProjection) {
	body, err := json.Marshal(struct {
		Observation string                 `json:"observation"`
		Projection  managedMediaProjection `json:"projection"`
	}{Observation: string(snapshot.Bytes()), Projection: projection})
	if err != nil || len(body) > managedMediaPrivateLimit {
		log.Warn("media original FACT exceeds its private report budget")
		return
	}
	j.reports = append(j.reports, managedMediaReport{Wire: body, EventID: snapshot.EventID()})
}

func (j *managedMediaJob) reportHealth(snapshot executionfacts.Snapshot, mediaCommitted bool) {
	if j.health == nil {
		return
	}
	e := snapshot.Envelope()
	if j.claimID == nil && (e.HTTPStatus == nil || *e.HTTPStatus != 429) {
		return
	}
	asyncAccepted := !j.request.IsQuery() && e.ProviderTaskID != "" && e.SendState == "observed_sent" &&
		e.HTTPStatus != nil && *e.HTTPStatus >= 200 && *e.HTTPStatus < 300 && e.Acceptance == "accepted" &&
		e.HeadersComplete != nil && *e.HeadersComplete && e.BodyComplete != nil && *e.BodyComplete
	if asyncAccepted && !mediaCommitted {
		// RecoveryAccepted relies on the actual task association committed by
		// MEDIA/FACT T1, not merely an HTTP 202 or a notification ACK.
		j.healthAfterFact = &snapshot
		return
	}
	affected := j.claimScopes
	if e.HTTPStatus != nil && *e.HTTPStatus == 429 {
		affected = j.modelScopes
	}
	var task *string
	if e.ProviderTaskID != "" {
		value := e.ProviderTaskID
		task = &value
	}
	selection := upstreamhealth.AttemptSelection{RuntimeConfigID: j.executor.RuntimeConfigID(), ConfigRevision: upstreamhealth.Revision(j.executor.ConfigRevision()),
		TargetID: e.TargetID, CredentialRef: e.CredentialRef, CredentialGeneration: upstreamhealth.Revision(e.CredentialRevision), OperationKind: e.OperationKind,
		CallID: e.CallID, AttemptID: e.AttemptID, TaskID: task, SelectionDigest: e.SelectionDigest}
	request, err := upstreamhealth.OutcomeFromAttempt(snapshot, selection, affected, j.scopeRevisions, "fact:"+snapshot.EventID()+":"+snapshot.Digest(), j.claimID)
	if err != nil {
		log.Warn("media health outcome could not be retained")
		return
	}
	if !j.request.IsQuery() {
		// A create selection was made before its task existed. The original
		// D0/claim still has task_id=null; the learned task belongs to effect.
		request.Selection.TaskID = nil
	}
	if asyncAccepted {
		value := e.ProviderTaskID
		request.Effect = upstreamhealth.HealthEffect{Kind: "recovery_accepted", ProviderTaskID: &value}
	}
	if e.HTTPStatus == nil {
		request.ObservationPhase = "transport_outcome"
	}
	if err = j.health.ReportOutcome(request, func(upstreamhealth.GatewayResult[upstreamhealth.OutcomeReceipt]) {}); err != nil {
		log.Warn("media health outcome remains unacknowledged")
	}
}

func (j *managedMediaJob) report() {
	j.reportInFlight = true
	j.retryAt = j.runtime.now().Add(time.Second)
	j.reportDeadline = j.runtime.now().Add(time.Duration(j.runtime.settings.Console.TimeoutMS)*time.Millisecond + time.Second)
	path := "/internal/media/operations/" + url.PathEscape(j.request.OperationID()) + "/observations"
	report := j.reports[0]
	wire := append([]byte(nil), report.Wire...)
	err := j.runtime.call("POST", path, wire, func(reply managedMediaPrivateReply) {
		j.reportInFlight = false
		if data, err := managedMediaData(reply); err == nil && len(j.reports) > 0 && bytes.Equal(j.reports[0].Wire, wire) {
			var receipt struct {
				EventID string          `json:"event_id"`
				Reason  *string         `json:"reason_code"`
				Fact    json.RawMessage `json:"fact"`
			}
			if json.Unmarshal(data, &receipt) != nil || receipt.EventID != report.EventID {
				return
			}
			if j.healthAfterFact != nil && receipt.EventID == j.healthAfterFact.EventID() &&
				receipt.Reason == nil && len(receipt.Fact) > 0 && !bytes.Equal(receipt.Fact, []byte("null")) {
				snapshot := *j.healthAfterFact
				j.healthAfterFact = nil
				j.reportHealth(snapshot, true)
			}
			if receipt.Reason != nil {
				log.Warn("media original observation committed with a review reason")
			}
			j.reports = j.reports[1:]
		}
	})
	if err != nil {
		j.reportInFlight = false
	}
}

func decodeManagedMediaJSON(raw []byte, target interface{}) error {
	if len(raw) == 0 || len(raw) > managedMediaPrivateLimit {
		return errors.New("media private wire exceeds its original budget")
	}
	if err := mfjson.ValidateRuntimeBody(raw, managedMediaPrivateLimit); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func managedMediaData(reply managedMediaPrivateReply) ([]byte, error) {
	if reply.Status < 200 || reply.Status >= 300 || !reply.HeadersComplete || !reply.BodyComplete {
		return nil, errors.New("media Console commit response is unconfirmed")
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := decodeManagedMediaJSON(reply.Body, &envelope); err != nil || len(envelope.Data) == 0 || bytes.Equal(envelope.Data, []byte("null")) {
		return nil, errors.New("media Console reply has no committed data")
	}
	return append([]byte(nil), envelope.Data...), nil
}

func (r *managedMediaRuntime) consoleCall(method, path string, body []byte, callback func(managedMediaPrivateReply)) error {
	if !strings.HasPrefix(path, "/internal/media/") || strings.ContainsAny(path, "\r\n\x00?#") || len(body) > managedMediaPrivateLimit {
		return errors.New("media Console destination differs from its original registration")
	}
	e := r.settings.Console
	cluster := wrapper.FQDNCluster{FQDN: e.ServiceName, Port: int64(e.ServicePort), Host: e.Host}
	headers := [][2]string{{":method", method}, {":scheme", "http"}, {":authority", e.Host}, {":path", path},
		{"authorization", "Bearer " + e.AuthToken}, {"x-modelfusion-gateway-instance", r.owner}, {"content-type", "application/json"}, {"accept", "application/json"}}
	_, err := proxywasm.DispatchHttpCall(cluster.ClusterName(), headers, body, nil, e.TimeoutMS, func(numHeaders, bodySize, _ int) {
		response := readHealthGatewayResponse(numHeaders, bodySize, managedMediaPrivateLimit)
		callback(managedMediaPrivateReply{Status: response.StatusCode, HeadersComplete: response.HeadersComplete, BodyComplete: response.BodyComplete, Body: response.Body})
	})
	return err
}

func (r *managedMediaRuntime) providerCall(request mediaexecution.Request, callback func(mediaexecution.Response)) error {
	var descriptor struct {
		Cluster string `json:"cluster_ref"`
	}
	endpoint, err := url.Parse(request.Endpoint())
	if err != nil || json.Unmarshal(request.Descriptor(), &descriptor) != nil || descriptor.Cluster == "" {
		return errors.New("media original transport is absent")
	}
	headers := [][2]string{{":method", request.Method()}, {":scheme", endpoint.Scheme}, {":authority", endpoint.Host}, {":path", request.Path()}}
	for _, header := range request.Headers() {
		if !strings.EqualFold(header[0], "host") {
			headers = append(headers, header)
		} else {
			headers[2][1] = header[1]
		}
	}
	_, err = proxywasm.DispatchHttpCall(descriptor.Cluster, headers, request.Body(), nil, request.TimeoutMilliseconds(), func(numHeaders, bodySize, _ int) {
		callback(readManagedMediaResponse(numHeaders, bodySize, r.settings.MaxResponseBytes, r.now()))
	})
	return err
}

// This is the actual instrumentor boundary. Never manufacture a provider 502
// when response headers/body cannot be read or the host callback times out.
func readManagedMediaResponse(numHeaders, bodySize, limit int, now time.Time) mediaexecution.Response {
	response := mediaexecution.Response{ObservedAt: now.UTC().Format(time.RFC3339Nano), SendState: "unknown"}
	headers, err := proxywasm.GetHttpCallResponseHeaders()
	response.Headers = append([][2]string(nil), headers...)
	response.HeadersComplete = err == nil && len(headers) == numHeaders
	status, statusErr := managedSingleHeader(headers, ":status")
	if statusErr == nil && len(status) == 3 {
		parsed, err := strconv.ParseUint(status, 10, 16)
		if err == nil && parsed >= 100 && parsed <= 599 {
			value := uint16(parsed)
			response.HTTPStatus, response.SendState = &value, "observed_sent"
		}
	}
	if response.HTTPStatus == nil {
		response.HeadersComplete = false
		response.TransportError = "media_http_status_not_observed"
	}
	if bodySize < 0 || bodySize > limit {
		response.TransportError = "media_http_body_exceeds_budget"
		return response
	}
	if bodySize == 0 {
		response.BodyObserved, response.BodyComplete = true, true
		return response
	}
	body, err := proxywasm.GetHttpCallResponseBody(0, bodySize)
	response.Body = append([]byte(nil), body...)
	response.BodyObserved = err == nil || len(body) > 0
	response.BodyComplete = err == nil && len(body) == bodySize
	if !response.BodyComplete {
		response.TransportError = "media_http_body_not_observed"
	}
	return response
}

func (r *managedMediaRuntime) String() string {
	return fmt.Sprintf("managedMediaRuntime(jobs=%d)", len(r.jobs))
}
