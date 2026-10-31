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
	"github.com/tidwall/resp"
)

const (
	managedMediaDispatchPath = "/__modelfusion/media/dispatch"
	managedMediaQueryPath    = "/__modelfusion/media/query"
	managedMediaContextKey   = "modelfusion.private_media_notice"
	managedMediaNoticeLimit  = 32 << 20
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

// This global Redis/Stream registration is deployment-owned. Original target transport,
// credential and all accounting identity still come from the frozen REL archive.
type managedMediaSettings struct {
	ProducerID       string                `json:"producer_id"`
	ServiceAuthToken string                `json:"service_auth_token"`
	PrivateStream    string                `json:"private_stream"`
	Redis            managedRedisSettings  `json:"redis"`
	Profiles         []managedMediaProfile `json:"profiles"`
	QueueCapacity    int                   `json:"queue_capacity"`
	MaxResponseBytes int                   `json:"max_response_bytes"`
}

type managedMediaAdmission struct {
	ScopeID              string  `json:"scope_id"`
	Generation           string  `json:"generation"`
	AccountID            string  `json:"account_id"`
	AdmissionRevision    string  `json:"admission_revision"`
	MappingRevision      string  `json:"mapping_revision"`
	CredentialGeneration string  `json:"credential_generation"`
	RouteID              string  `json:"route_id"`
	KeyID                *string `json:"api_key_id"`
}
type managedMediaNotice struct {
	SchemaVersion string                `json:"schema_version"`
	DispatchID    string                `json:"dispatch_id"`
	OperationID   string                `json:"operation_id"`
	RequestID     string                `json:"request_id"`
	TicketKey     string                `json:"ticket_key"`
	TicketDigest  string                `json:"ticket_digest"`
	ExpiresAt     time.Time             `json:"expires_at"`
	Admission     managedMediaAdmission `json:"admission"`
	Archive       json.RawMessage       `json:"archive"`
}
type managedMediaACK struct {
	SchemaVersion string `json:"schema_version"`
	OperationID   string `json:"operation_id"`
	RequestID     string `json:"request_id"`
	TicketDigest  string `json:"ticket_digest"`
	Accepted      bool   `json:"accepted"`
}
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
	redis    upstreamhealth.RuntimeRedis
	provider managedMediaProviderCall
	now      func() time.Time
}

type managedMediaJob struct {
	runtime              *managedMediaRuntime
	notice               managedMediaNotice
	key, stage           string
	live                 mediaexecution.Invocation
	deadline, retryAt    time.Time
	reportDeadline       time.Time
	executor             *mediaexecution.Executor
	archive              managedMediaArchive
	selection            json.RawMessage
	request              mediaexecution.Request
	attempt              executionfacts.Attempt
	actual, generation   *executionfacts.Snapshot
	delivery             *executionfacts.Snapshot
	healthAfterFact      *executionfacts.Snapshot
	manifest             *mediaexecution.OutputManifest
	health               *upstreamhealth.Runtime
	healthTarget         upstreamhealth.TargetReference
	healthDecision       upstreamhealth.Decision
	claimID              *string
	claimScopes          []string
	modelScopes          []string
	scopeRevisions       map[string]upstreamhealth.Revision
	reports              []managedMediaReport
	reportInFlight       bool
	providerCallbackSeen bool
}

// These fields are private originals carried by the authenticated frozen notice.
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
	if len(settings.ServiceAuthToken) == 0 || len(settings.ServiceAuthToken) > 8192 || !managedID(settings.PrivateStream) || !managedID(settings.Redis.ServiceName) ||
		settings.Redis.ServicePort < 1 || settings.Redis.ServicePort > 65535 || settings.Redis.Timeout < 1 || settings.Redis.Timeout > 60000 || settings.Redis.Database < 0 {
		return errors.New("managed_media requires existing Redis, Stream and service authentication")
	}
	for _, character := range settings.ServiceAuthToken {
		if character < 33 || character > 126 {
			return errors.New("managed_media service authentication is not a valid Bearer credential")
		}
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
	redis := settings.Redis
	client := wrapper.NewRedisEventClient(wrapper.FQDNCluster{FQDN: redis.ServiceName, Port: int64(redis.ServicePort)}, proxywasm.DispatchRedisCallInBackground)
	if err := client.Init(redis.Username, redis.Password, redis.Timeout, wrapper.WithDataBase(redis.Database)); err != nil {
		return errors.New("managed_media Redis initialization failed")
	}
	runtime.redis = client
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
	if err != nil || !authorizeManagedMediaNotice(headers, runtime.settings.ServiceAuthToken) {
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
	if !managedID(notice.DispatchID) || !managedID(notice.OperationID) || !managedID(notice.RequestID) ||
		!strings.HasPrefix(notice.TicketKey, "mf:media:ticket:") || !managedID(notice.TicketKey) || notice.ExpiresAt.IsZero() || len(notice.Archive) == 0 ||
		!managedID(notice.Admission.ScopeID) || !managedID(notice.Admission.AccountID) {
		return managedMediaACK{}, errors.New("invalid original media ticket")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return managedMediaACK{}, errors.New("invalid media ticket")
	}
	delete(fields, "ticket_digest")
	original, err := json.Marshal(fields)
	if err != nil {
		return managedMediaACK{}, err
	}
	digest, err := mfjson.Digest(original)
	if err != nil || digest != notice.TicketDigest {
		return managedMediaACK{}, errors.New("media ticket differs from original payload")
	}
	key := notice.TicketKey
	ack := managedMediaACK{OperationID: notice.OperationID, RequestID: notice.RequestID, TicketDigest: notice.TicketDigest, Accepted: true}
	switch path {
	case managedMediaDispatchPath:
		if notice.SchemaVersion != "media-dispatch-notice.v2" || notice.RequestID != notice.DispatchID {
			return managedMediaACK{}, errors.New("invalid create notice")
		}
		ack.SchemaVersion = "media-dispatch-ack.v2"
	case managedMediaQueryPath:
		if notice.SchemaVersion != "media-query-notice.v2" {
			return managedMediaACK{}, errors.New("invalid query notice")
		}
		ack.SchemaVersion = "media-query-ack.v2"
	default:
		return managedMediaACK{}, errors.New("unknown media control path")
	}
	if original, duplicate := r.jobs[key]; duplicate {
		if original.notice.TicketDigest != notice.TicketDigest {
			return managedMediaACK{}, errors.New("media ticket conflict")
		}
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
		if (job.stage == "waiting_health" || job.stage == "waiting_admission") && !job.retryAt.After(now) && job.notice.ExpiresAt.After(now) {
			job.health.FlushPending()
			job.checkHealth()
		}
		if len(job.reports) > 0 && !job.reportInFlight && !job.retryAt.After(now) {
			job.report()
		}
		if job.reportInFlight && !job.reportDeadline.After(now) {
			job.reportInFlight = false
		}
		if job.health != nil && !job.retryAt.After(now) {
			job.health.FlushPending()
		}
	}
}

func (j *managedMediaJob) fetch() {
	r := j.runtime
	j.stage, j.deadline = "fetching", j.notice.ExpiresAt
	raw := j.notice.Archive
	if !j.notice.ExpiresAt.After(r.now()) || json.Unmarshal(raw, &j.archive) != nil {
		j.stage = "done"
		return
	}
	binding, supported := r.profiles[j.archive.Profile]
	if !supported {
		j.stage = "done"
		return
	}
	var err error
	j.executor, err = mediaexecution.FromPrivateArchive(raw, mediaexecution.GatewayIdentity{ProducerID: r.settings.ProducerID, Actor: "service/gateway/" + r.settings.ProducerID, BootID: r.owner}, binding)
	if err != nil || j.executor.DispatchID() != j.notice.DispatchID || j.executor.OperationID() != j.notice.OperationID || !j.isQuery() && j.executor.DispatchState() != "prepared" {
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
	if j.isQuery() && j.generation == nil {
		j.stage = "done"
		return
	}
	j.qualify()
}
func (j *managedMediaJob) isQuery() bool { return j.notice.SchemaVersion == "media-query-notice.v2" }

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
	if j.isQuery() {
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
	client, owner, err := managedGatewayHealthRuntime(settings, j.runtime.redis, j.runtime.settings.PrivateStream, j.runtime.settings.ProducerID)
	if err != nil || owner != j.runtime.owner {
		j.stage = "done"
		return
	}
	j.health = client
	j.checkHealth()
}

// A queued original remains in its live gateway while health is unavailable.
// The existing root tick retries only reads; it cannot renew a ticket or replay
// an uncertain claim/consumption, and it does not need the notifying Console.
func (j *managedMediaJob) checkHealth() {
	client, target := j.health, j.healthTarget
	j.stage, j.deadline = "qualifying", j.notice.ExpiresAt
	client.Lookup(func(read upstreamhealth.ReadResult) {
		if j.stage != "qualifying" {
			return
		}
		decision := client.DecideNewCall(read, upstreamhealth.Candidate{Target: target}, j.runtime.now())
		if (decision.Kind != upstreamhealth.Eligible && decision.Kind != upstreamhealth.RecoveryRequired) || read.Snapshot == nil {
			j.stage = "waiting_health"
			j.retryAt = j.runtime.now().Add(time.Second)
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
}

func (j *managedMediaJob) claim() {
	var task *string
	if j.isQuery() {
		var history struct {
			Attempt *struct {
				Task *string `json:"provider_task_id"`
			} `json:"attempt"`
		}
		if json.Unmarshal(j.executor.OriginalHistory(), &history) != nil || history.Attempt == nil {
			j.stage = "done"
			return
		}
		task = history.Attempt.Task
	}
	candidate, err := upstreamhealth.NewRecoveryCandidate(j.selection, j.executor.RuntimeConfigID(), upstreamhealth.Revision(j.executor.ConfigRevision()), task)
	if err != nil {
		j.stage = "done"
		return
	}
	request := upstreamhealth.ClaimRequest{ClaimRequestID: "media-health:" + j.key, OwnerInstance: j.runtime.owner, ExpectedEpochs: j.healthDecision.ScopeEpochs, Candidate: candidate, CandidateDigest: candidate.Digest()}
	j.stage, j.deadline = "claiming", j.notice.ExpiresAt
	err = j.health.ClaimConsumed(request, j.healthTarget, func(result upstreamhealth.GatewayResult[upstreamhealth.RecoveryClaim]) {
		if j.stage != "claiming" {
			return
		}
		if result.State != upstreamhealth.GatewayAcknowledged || result.Value == nil {
			j.stage = "done"
			return
		}
		claim := result.Value
		expires, err := time.Parse(time.RFC3339Nano, claim.ExpiresAt)
		if err != nil || !expires.After(j.runtime.now()) {
			j.stage = "done"
			return
		}
		j.selection, j.claimID = append([]byte(nil), claim.ConsumedSelection...), &claim.ClaimID
		for _, scope := range claim.Scopes {
			j.claimScopes = append(j.claimScopes, scope.ScopeID)
		}
		j.consume()
	})
	if err != nil {
		j.stage = "done"
	}
}
func (j *managedMediaJob) consume() {
	if j.stage == "sending" || j.stage == "done" {
		return
	}
	body, err := json.Marshal(struct {
		Schema       string `json:"schema_version"`
		ProducerID   string `json:"producer_id"`
		OperationID  string `json:"operation_id"`
		RequestID    string `json:"request_id"`
		TicketDigest string `json:"ticket_digest"`
		Selection    string `json:"selection"`
	}{"media-dispatch-observed.v1", j.runtime.settings.ProducerID, j.executor.OperationID(), j.notice.RequestID, j.notice.TicketDigest, string(j.selection)})
	if err != nil {
		j.stage = "done"
		return
	}
	admission := j.notice.Admission
	prefix := "mf:balance:{" + admission.ScopeID + "}"
	account := prefix + ":account:" + admission.AccountID
	mapping := account + ":principal:key:"
	if admission.KeyID != nil {
		mapping += *admission.KeyID
	}
	j.stage, j.deadline = "consuming", j.notice.ExpiresAt
	err = j.runtime.redis.Command([]interface{}{"EVAL", mediaexecution.DispatchConsumeScript, 5, j.notice.TicketKey, prefix + ":control", account, mapping, j.runtime.settings.PrivateStream, j.notice.TicketDigest, string(body)}, func(reply resp.Value) {
		if j.stage != "consuming" {
			return
		}
		var err error
		if !j.isQuery() {
			j.request, err = j.executor.PrepareRedisCreate(reply, j.selection, j.live, j.runtime.now())
		} else {
			j.request, err = j.executor.PrepareRedisQuery(reply, j.selection, j.notice.ExpiresAt, j.runtime.now())
		}
		if errors.Is(err, mediaexecution.ErrProjectionUnready) {
			// This definite denial did not consume the original ticket.
			j.stage, j.retryAt = "waiting_admission", j.runtime.now().Add(time.Second)
			return
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
		j.executor.ConsumeUnknown()
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
		SchemaVersion string                 `json:"schema_version"`
		ProducerID    string                 `json:"producer_id"`
		OperationID   string                 `json:"operation_id"`
		Observation   string                 `json:"observation"`
		Projection    managedMediaProjection `json:"projection"`
	}{SchemaVersion: "media-observation.v1", ProducerID: j.runtime.settings.ProducerID, OperationID: j.request.OperationID(), Observation: string(snapshot.Bytes()), Projection: projection})
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
	if err = j.health.ReportOutcome(request, j.healthTarget, func(upstreamhealth.GatewayResult[upstreamhealth.OutcomeReceipt]) {}); err != nil {
		log.Warn("media health outcome remains unacknowledged")
	}
}

func (j *managedMediaJob) report() {
	j.reportInFlight = true
	j.retryAt = j.runtime.now().Add(time.Second)
	j.reportDeadline = j.runtime.now().Add(time.Duration(j.runtime.settings.Redis.Timeout)*time.Millisecond + time.Second)
	report := j.reports[0]
	wire := append([]byte(nil), report.Wire...)
	err := j.runtime.redis.Command([]interface{}{"XADD", j.runtime.settings.PrivateStream, "*", "message_type", "media_observation", "event", string(wire)}, func(reply resp.Value) {
		j.reportInFlight = false
		if reply.Error() != nil || reply.Type() != resp.BulkString || !strings.Contains(reply.String(), "-") {
			return
		}
		if len(j.reports) == 0 || !bytes.Equal(j.reports[0].Wire, wire) {
			return
		}
		j.reports = j.reports[1:]
		if j.healthAfterFact != nil && report.EventID == j.healthAfterFact.EventID() {
			snapshot := *j.healthAfterFact
			j.healthAfterFact = nil
			j.reportHealth(snapshot, true)
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
