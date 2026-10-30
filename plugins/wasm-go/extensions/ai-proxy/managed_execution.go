package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"mime"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

const (
	managedExecutionContextKey            = "ai-proxy-managed-execution.v1"
	managedCallHeader                     = "x-modelfusion-call-id"
	managedAttemptHeader                  = "x-modelfusion-attempt-id"
	managedAdmissionReleaseHeader         = "x-modelfusion-admission-release-id"
	managedAdmissionDeadlineHeader        = "x-modelfusion-admission-valid-until"
	managedAdmissionOccurredHeader        = executionfacts.AdmissionOccurredHeader
	managedDefaultTimeoutMS        uint32 = 2 * 60 * 1000
)

// These private structures are the SDK's reserved release envelope and the
// semantic proxy configuration. The SDK checks the semantic proxy digest before
// installing this envelope. A body field is never a producer grant or history.
type managedRelease struct {
	SchemaVersion      string                      `json:"schema_version"`
	Descriptor         managedDescriptor           `json:"descriptor"`
	RouteID            string                      `json:"route_id"`
	SalesAssetID       string                      `json:"sales_asset_id"`
	PublicModel        string                      `json:"public_model"`
	CredentialRef      string                      `json:"credential_ref,omitempty"`
	CredentialRevision string                      `json:"credential_revision,omitempty"`
	Credentials        []managedCredentialIdentity `json:"credentials,omitempty"`
}

type managedCredentialIdentity struct {
	CredentialRef      string `json:"credential_ref"`
	CredentialRevision string `json:"credential_revision"`
}

type managedCredential struct {
	CredentialRef      string `json:"credential_ref"`
	CredentialRevision string `json:"credential_revision"`
	CredentialValue    string `json:"credential_value"`
}

type managedDescriptor struct {
	ReleaseID              string                       `json:"release_id"`
	TargetID               string                       `json:"target_id"`
	BindingID              string                       `json:"binding_id"`
	ProviderID             string                       `json:"provider_id"`
	ProviderAccountID      string                       `json:"provider_account_id"`
	RegionEndpointRef      string                       `json:"region_endpoint_ref"`
	ServiceRef             string                       `json:"service_ref"`
	ClusterRef             string                       `json:"cluster_ref"`
	CredentialPoolRef      string                       `json:"credential_pool_ref"`
	AdapterRef             string                       `json:"adapter_ref"`
	CredentialPoolRevision string                       `json:"credential_pool_revision"`
	AdapterRevision        string                       `json:"adapter_revision"`
	MapperDigest           string                       `json:"mapper_digest"`
	ProxyDigest            string                       `json:"proxy_digest"`
	DescriptorDigest       string                       `json:"descriptor_digest"`
	UpstreamModel          string                       `json:"upstream_model"`
	ConfigCanonicalization string                       `json:"config_canonicalization"`
	Transport              managedTransport             `json:"transport_contract"`
	Capabilities           map[string]managedCapability `json:"capabilities"`
}

type managedTransport struct {
	Host           string `json:"host"`
	SNI            string `json:"sni"`
	PathAdapterRef string `json:"path_adapter_ref"`
}

type managedCapability struct {
	SendMode               string `json:"send_mode"`
	ResponseClassifierRef  string `json:"response_classifier_ref"`
	QueryCapabilityRef     string `json:"query_capability_ref,omitempty"`
	ProviderIdempotencyRef string `json:"provider_idempotency_ref,omitempty"`
	MaxAttempts            uint32 `json:"max_attempts"`
	MaxFallbackHops        uint32 `json:"max_fallback_hops"`
}

type managedRedisSettings struct {
	ServiceName string `json:"service_name"`
	ServicePort int    `json:"service_port,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Database    int    `json:"database,omitempty"`
	Timeout     int64  `json:"timeout,omitempty"`
}

type managedSettings struct {
	MediaDispatchMode  string                 `json:"media_dispatch_mode,omitempty"`
	ProducerID         string                 `json:"producer_id"`
	CallIdentitySource string                 `json:"call_identity_source"`
	PrivateStream      string                 `json:"private_stream"`
	Redis              managedRedisSettings   `json:"redis"`
	UpstreamPath       string                 `json:"upstream_path"`
	Authority          string                 `json:"authority"`
	CredentialValue    string                 `json:"credential_value,omitempty"`
	Credentials        []managedCredential    `json:"credentials,omitempty"`
	TimeoutMS          uint32                 `json:"timeout_ms,omitempty"`
	InputProtocol      string                 `json:"input_protocol,omitempty"`
	UpstreamProtocol   string                 `json:"upstream_protocol,omitempty"`
	UpstreamHealth     *managedHealthSettings `json:"upstream_health,omitempty"`
	TextExecution      *managedTextSettings   `json:"text_execution,omitempty"`
}

type managedRedisClient interface {
	Command([]interface{}, wrapper.RedisResponseCallback) error
}

type managedExecutionConfig struct {
	settings managedSettings
	release  managedRelease
	redis    managedRedisClient
	profile  managedProtocolProfile
	health   *managedHealthRuntime
	targets  map[string]*managedExecutionConfig
	mapper   json.RawMessage
}

// configureManagedExecution is called for every global and rule parse. Clearing
// the inherited pointer is essential: a different rule must supply its own
// complete release envelope and semantic configuration in this very JSON value.
func configureManagedExecution(value gjson.Result, cfg *config.PluginConfig) error {
	cfg.ManagedExecution = nil
	settingsValue := value.Get("managed_execution")
	releaseValue := value.Get("_modelfusion_execution")
	if !settingsValue.Exists() && !releaseValue.Exists() {
		return nil
	}
	if !settingsValue.IsObject() || !releaseValue.IsObject() {
		return errors.New("managed execution requires its complete SDK release envelope")
	}
	if err := validateManagedJSON([]byte(value.Raw), executionfacts.MaxEnvelopeBytes); err != nil {
		return errors.New("managed execution configuration is ambiguous or exceeds its budget")
	}
	if value.Get("path_scope").String() != "bound_target" {
		return errors.New("managed execution requires route-bound target scope")
	}
	for _, field := range []string{"enable_methods", "enable_path_suffixes"} {
		if filter := value.Get(field); filter.Exists() && (!filter.IsArray() || len(filter.Array()) != 0) {
			return errors.New("managed execution does not accept manual URL or method filters")
		}
	}
	providerConfig, activeProvider := cfg.GetProviderConfig(), cfg.GetProvider()
	if providerConfig == nil || activeProvider == nil {
		return errors.New("managed execution requires a configured text provider")
	}
	if !managedRequiredFields(releaseValue, "schema_version descriptor route_id sales_asset_id public_model") || !managedRequiredFields(releaseValue.Get("descriptor"), "release_id target_id binding_id provider_id provider_account_id region_endpoint_ref service_ref cluster_ref credential_pool_ref adapter_ref credential_pool_revision adapter_revision mapper_digest proxy_digest descriptor_digest upstream_model config_canonicalization transport_contract capabilities") || !managedRequiredFields(releaseValue.Get("descriptor.transport_contract"), "host sni path_adapter_ref") {
		return errors.New("managed execution requires the complete released descriptor")
	}
	if err := validateManagedCredentialShape(settingsValue, releaseValue); err != nil {
		return err
	}
	for _, capability := range releaseValue.Get("descriptor.capabilities").Map() {
		if !managedRequiredFields(capability, "send_mode response_classifier_ref max_attempts max_fallback_hops") {
			return errors.New("managed execution requires complete released capabilities")
		}
	}
	var parsed managedExecutionConfig
	if err := decodeManagedJSON([]byte(settingsValue.Raw), &parsed.settings, true); err != nil {
		return errors.New("invalid managed execution settings")
	}
	if err := decodeManagedJSON([]byte(releaseValue.Raw), &parsed.release, true); err != nil {
		return errors.New("invalid managed execution release descriptor")
	}
	if err := parsed.validate(); err != nil {
		return err
	}
	expected, err := managedDescriptorDigest([]byte(releaseValue.Get("descriptor").Raw))
	if err != nil || expected != parsed.release.Descriptor.DescriptorDigest {
		return errors.New("managed execution target descriptor digest differs")
	}
	if parsed.settings.MediaDispatchMode == "managed_operation" {
		if err := validateManagedMediaRelease(&parsed, value, cfg); err != nil {
			return err
		}
		cfg.ManagedExecution = &parsed
		return nil
	}
	profile, err := newManagedProtocolProfile(parsed.settings.InputProtocol, parsed.settings.UpstreamProtocol)
	if err != nil {
		return errors.New("unsupported managed execution protocol profile")
	}
	parsed.profile = profile
	providerType := activeProvider.GetProviderType()
	if providerType != "openai" && providerType != "generic" &&
		(providerType != "claude" || profile.UpstreamProtocol != managedProtocolMessages) {
		return errors.New("managed execution provider does not support its frozen protocol profile")
	}
	redis := parsed.settings.Redis
	client := wrapper.NewRedisEventClient(wrapper.FQDNCluster{FQDN: redis.ServiceName, Port: int64(redis.ServicePort)}, proxywasm.DispatchRedisCallInBackground)
	if err := client.Init(redis.Username, redis.Password, redis.Timeout, wrapper.WithDataBase(redis.Database)); err != nil {
		return errors.New("managed execution Redis initialization failed")
	}
	parsed.redis = client
	parsed.health, err = configureManagedHealth(&parsed)
	if err != nil {
		return err
	}
	if err := configureManagedTextDirectory(value, &parsed); err != nil {
		return err
	}
	cfg.ManagedExecution = &parsed
	return nil
}

func (c *managedExecutionConfig) validate() error {
	d := c.release.Descriptor
	if c.settings.CallIdentitySource != executionfacts.CallIdentitySourceAdmission && c.settings.CallIdentitySource != executionfacts.CallIdentitySourceInternal {
		return errors.New("managed execution requires the original admission call identity capability")
	}
	if (c.release.SchemaVersion != "modelfusion-execution.v1" && c.release.SchemaVersion != "modelfusion-execution.v2") || d.ConfigCanonicalization != "mf-json.v1" {
		return errors.New("unsupported managed execution release schema")
	}
	for _, field := range []string{c.settings.ProducerID, c.settings.PrivateStream, c.release.RouteID, c.release.SalesAssetID, c.release.PublicModel, d.ReleaseID, d.TargetID, d.BindingID, d.ProviderID, d.ProviderAccountID, d.RegionEndpointRef, d.ServiceRef, d.ClusterRef, d.CredentialPoolRef, d.AdapterRef, d.UpstreamModel, d.Transport.PathAdapterRef} {
		if !managedID(field) {
			return errors.New("managed execution requires complete frozen identities")
		}
	}
	for _, revision := range []string{d.CredentialPoolRevision, d.AdapterRevision} {
		n, ok := managedInteger(revision)
		if !ok || n.Sign() == 0 {
			return errors.New("managed execution requires exact positive revisions")
		}
	}
	for _, digest := range []string{d.MapperDigest, d.ProxyDigest, d.DescriptorDigest} {
		if !managedDigest(digest) {
			return errors.New("managed execution requires frozen descriptor/config digests")
		}
	}
	if !managedAuthority(c.settings.Authority) || c.settings.Authority != d.Transport.Host || (d.Transport.SNI != "" && !managedAuthority(d.Transport.SNI)) {
		return errors.New("managed execution authority differs from the released transport")
	}
	_, err := validateSafeOriginFormTarget(c.settings.UpstreamPath)
	if err != nil {
		return errors.New("managed execution requires a safe text upstream path")
	}
	if _, err := c.credentialMembers(); err != nil {
		return err
	}
	if c.release.SchemaVersion == "modelfusion-execution.v2" {
		if _, err := managedHealthMembers(c); err != nil {
			return err
		}
	}
	if c.settings.MediaDispatchMode != "" && c.settings.MediaDispatchMode != "disabled" && c.settings.MediaDispatchMode != "managed_operation" {
		return errors.New("managed execution media dispatch mode is unsupported")
	}
	if c.settings.CallIdentitySource == executionfacts.CallIdentitySourceInternal && (c.settings.UpstreamHealth == nil || c.settings.MediaDispatchMode == "managed_operation") {
		return errors.New("internal execution requires its registered service health endpoint and text operation")
	}
	if managedOperation(c) == executionfacts.NativeEmbeddingOperation {
		if err := validateManagedEmbedding(c); err != nil {
			return err
		}
	} else if c.settings.MediaDispatchMode != "managed_operation" {
		text, ok := d.Capabilities["text_generate"]
		if !ok || !managedID(text.ResponseClassifierRef) || text.MaxAttempts < 1 || (text.SendMode != "single_send" && text.SendMode != "observable_retry") {
			return errors.New("managed execution descriptor has no text capability")
		}
	}
	for operation, capability := range d.Capabilities {
		if operation != "text_generate" && operation != "image_generate" && operation != "video_generate" && operation != "task_query" && (operation != executionfacts.NativeEmbeddingOperation || managedOperation(c) != executionfacts.NativeEmbeddingOperation) {
			return errors.New("managed execution descriptor has an undeclared capability")
		}
		if !managedID(capability.ResponseClassifierRef) || capability.MaxAttempts < 1 || (capability.SendMode != "single_send" && capability.SendMode != "observable_retry") {
			return errors.New("managed execution descriptor has an invalid capability")
		}
	}
	if err := validateManagedTextSettings(c); err != nil {
		return err
	}
	if c.settings.TimeoutMS == 0 {
		c.settings.TimeoutMS = managedDefaultTimeoutMS
	}
	if c.settings.TimeoutMS > 10*60*1000 {
		return errors.New("managed execution timeout exceeds the supported callout boundary")
	}
	r := &c.settings.Redis
	if !managedID(r.ServiceName) || strings.IndexAny(r.ServiceName, " /:@?#[]") >= 0 || r.Database < 0 || r.Timeout < 0 {
		return errors.New("invalid managed execution Redis endpoint")
	}
	if r.ServicePort == 0 {
		r.ServicePort = 6379
		if strings.HasSuffix(r.ServiceName, ".static") {
			r.ServicePort = 80
		}
	}
	if r.ServicePort < 1 || r.ServicePort > 65535 {
		return errors.New("invalid managed execution Redis port")
	}
	if r.Timeout == 0 {
		r.Timeout = 1000
	}
	return nil
}

// Property presence is checked before typed decoding: null or mixed shapes must
// not be normalized into a scalar seed or an empty pool.
func validateManagedCredentialShape(settings, release gjson.Result) error {
	switch release.Get("schema_version").String() {
	case "modelfusion-execution.v1":
		if !managedRequiredFields(release, "credential_ref credential_revision") || !managedRequiredFields(settings, "credential_value") ||
			release.Get("credentials").Exists() || settings.Get("credentials").Exists() || settings.Get("upstream_health.targets").Exists() {
			return errors.New("managed execution v1 requires its original scalar credential")
		}
	case "modelfusion-execution.v2":
		if !release.Get("credentials").IsArray() || len(release.Get("credentials").Array()) == 0 ||
			!settings.Get("credentials").IsArray() || len(settings.Get("credentials").Array()) == 0 ||
			release.Get("credential_ref").Exists() || release.Get("credential_revision").Exists() || settings.Get("credential_value").Exists() ||
			!settings.Get("upstream_health.targets").IsArray() || len(settings.Get("upstream_health.targets").Array()) == 0 || settings.Get("upstream_health.target").Exists() {
			return errors.New("managed execution v2 requires the complete credential and health directories")
		}
	default:
		return errors.New("unsupported managed execution release schema")
	}
	return nil
}

func validManagedCredential(member managedCredential) bool {
	revision, ok := managedInteger(member.CredentialRevision)
	return managedID(member.CredentialRef) && ok && revision.Sign() > 0 && validManagedCredentialValue(member.CredentialValue)
}

func validManagedCredentialValue(value string) bool {
	return len(value) > 0 && len(value) <= 8192 && strings.IndexFunc(value, func(r rune) bool { return r <= 32 || r >= 127 }) < 0
}

// The envelope is a complete static identity directory, never a selected member.
// Return copies in that original directory order; secret-array position has no
// authority and no caller can add a ref or generation to this set.
func (c *managedExecutionConfig) credentialMembers() ([]managedCredential, error) {
	if c.release.SchemaVersion == "modelfusion-execution.v1" {
		member := managedCredential{c.release.CredentialRef, c.release.CredentialRevision, c.settings.CredentialValue}
		if len(c.release.Credentials) != 0 || len(c.settings.Credentials) != 0 || !validManagedCredential(member) {
			return nil, errors.New("managed execution v1 scalar credential is invalid")
		}
		return []managedCredential{member}, nil
	}
	if c.release.SchemaVersion != "modelfusion-execution.v2" || c.release.CredentialRef != "" || c.release.CredentialRevision != "" || c.settings.CredentialValue != "" ||
		len(c.release.Credentials) == 0 || len(c.release.Credentials) != len(c.settings.Credentials) {
		return nil, errors.New("managed execution credential directories differ")
	}
	byRef := make(map[string]managedCredential, len(c.settings.Credentials))
	for _, member := range c.settings.Credentials {
		if !validManagedCredential(member) {
			return nil, errors.New("managed execution contains an invalid credential member")
		}
		if _, exists := byRef[member.CredentialRef]; exists {
			return nil, errors.New("managed execution contains a duplicate credential member")
		}
		byRef[member.CredentialRef] = member
	}
	result := make([]managedCredential, 0, len(c.release.Credentials))
	for _, identity := range c.release.Credentials {
		member, exists := byRef[identity.CredentialRef]
		if !exists || member.CredentialRevision != identity.CredentialRevision {
			return nil, errors.New("managed execution credential directories differ")
		}
		delete(byRef, identity.CredentialRef)
		result = append(result, member)
	}
	return result, nil
}

func managedUniformMember(count int) (int, error) {
	if count < 1 {
		return 0, errors.New("managed execution has no eligible credential")
	}
	if count == 1 {
		return 0, nil
	}
	index, err := rand.Int(rand.Reader, big.NewInt(int64(count)))
	if err != nil {
		return 0, errors.New("managed execution credential choice is unavailable")
	}
	return int(index.Int64()), nil
}

// All request state belongs to this ai-proxy Wasm context. It does not assume
// that a context map, Go pointer or private envelope crosses a Wasm boundary.
type managedExecutionState struct {
	config                                                     *managedExecutionConfig
	sourceConfig                                               *managedExecutionConfig
	credentialSelected                                         bool
	identity                                                   balanceidentity.Snapshot
	callID, attemptID, businessUTC                             string
	streaming                                                  bool
	prepared, dispatchStarted, handedOff, callbackSeen, closed bool
	original                                                   executionfacts.Envelope
	attempt                                                    executionfacts.Attempt
	embeddingAttempt                                           executionfacts.EmbeddingAttempt
	expectedEmbeddings                                         int
	ledger                                                     *executionfacts.Ledger
	deliveries                                                 map[string]*executionfacts.Delivery
	lastAttempt, lastDelivery                                  *executionfacts.Snapshot
	admissionDeadline                                          time.Time
	healthInFlight                                             bool
	health                                                     *managedHealthAttempt
	frozenSelection                                            json.RawMessage
	text                                                       *managedTextCall
	mechanism, parentAttemptID                                 string
	selectionGeneration                                        uint32
	healthUnavailable                                          func(string)
	internal                                                   *managedInternalCall
}

func onManagedRequestHeaders(ctx wrapper.HttpContext, cfg config.PluginConfig) (types.Action, bool) {
	managed, ok := cfg.ManagedExecution.(*managedExecutionConfig)
	if !ok || managed == nil {
		return types.ActionContinue, false
	}
	beginManagedAccessLog(ctx, managed.release.Descriptor.ReleaseID)
	if managed.settings.MediaDispatchMode == "managed_operation" {
		ctx.SetContext(ctxSkipAIProxyLifecycle, true)
		return rejectManagedRequest(ctx, 409, "media_operation_api_required"), true
	}
	ctx.DisableReroute()
	ctx.SetContext(ctxSkipAIProxyLifecycle, true)
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return rejectManagedRequest(ctx, 403, "authenticated_identity_missing"), true
	}
	var origin executionfacts.AdmittedCall
	var identity balanceidentity.Snapshot
	var internal *managedInternalCall
	if managed.settings.CallIdentitySource == executionfacts.CallIdentitySourceInternal {
		internal, err = readManagedInternalCall(headers)
		if err != nil {
			return rejectManagedRequest(ctx, 403, "registered_internal_service_call_required"), true
		}
	} else {
		origin, err = executionfacts.ReadAdmittedCall(headers, managed.release.Descriptor.ReleaseID, time.Now())
		if err != nil {
			return rejectManagedRequest(ctx, 403, "current_execution_admission_required"), true
		}
		identity, err = balanceidentity.Read(headers)
		if err != nil || identity.TenantID == "" || identity.PrincipalID == "" {
			return rejectManagedRequest(ctx, 403, "authenticated_identity_missing"), true
		}
	}
	clean := make([][2]string, 0, len(headers))
	for _, header := range headers {
		if strings.EqualFold(header[0], managedCallHeader) || strings.EqualFold(header[0], managedAttemptHeader) ||
			strings.EqualFold(header[0], managedAdmissionReleaseHeader) || strings.EqualFold(header[0], managedAdmissionDeadlineHeader) ||
			strings.EqualFold(header[0], managedAdmissionOccurredHeader) || strings.EqualFold(header[0], managedInternalCallHeader) {
			continue
		}
		clean = append(clean, header)
	}
	if proxywasm.ReplaceHttpRequestHeaders(clean) != nil {
		return rejectManagedRequest(ctx, 500, "identity_header_cleanup_failed"), true
	}
	headers = clean
	if ctx.GetContext(managedExecutionContextKey) != nil {
		return rejectManagedRequest(ctx, 409, "execution_context_reentry"), true
	}
	_, err = validateSafeOriginFormTarget(ctx.Path())
	if err != nil || ctx.Method() != "POST" {
		return rejectManagedRequest(ctx, 400, "unsupported_text_request"), true
	}
	contentType, err := managedSingleHeader(headers, "content-type")
	mediaType, _, mediaErr := mime.ParseMediaType(contentType)
	if err != nil || mediaErr != nil || mediaType != "application/json" || !ctx.HasRequestBody() {
		return rejectManagedRequest(ctx, 400, "json_body_required"), true
	}
	state := &managedExecutionState{config: managed, sourceConfig: managed, identity: identity, internal: internal, callID: origin.CallID, admissionDeadline: origin.ValidUntil, businessUTC: origin.OccurredAt, ledger: executionfacts.NewLedger(128), deliveries: map[string]*executionfacts.Delivery{}}
	ctx.SetContext(managedExecutionContextKey, state)
	if state.internal == nil && proxywasm.ReplaceHttpRequestHeader(managedCallHeader, state.callID) != nil {
		return rejectManagedRequest(ctx, 500, "execution_identity_install_failed"), true
	}
	ctx.SetUserAttribute("managed_execution_transport", "http_call")
	ctx.SetUserAttribute("managed_execution_response_mode", "buffered_http_call")
	ctx.SetUserAttribute("managed_call_id", state.callID)
	ctx.SetRequestBodyBufferLimit(defaultMaxBodyBytes)
	if !wrapper.RequireRequestBody(ctx) {
		return rejectManagedRequest(ctx, 500, "body_context_unavailable"), true
	}
	return types.HeaderStopIteration, true
}

func onManagedRequestBody(ctx wrapper.HttpContext, cfg config.PluginConfig, body []byte) (types.Action, bool) {
	managed, enabled := cfg.ManagedExecution.(*managedExecutionConfig)
	if !enabled || managed == nil {
		return types.ActionContinue, false
	}
	state, ok := ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	if !ok || state == nil || state.sourceConfig != managed {
		return rejectManagedRequest(ctx, 409, "original_execution_missing"), true
	}
	if state.dispatchStarted || state.closed || state.healthInFlight {
		return types.ActionPause, true
	}
	if state.internal != nil && state.internal.checkStarted {
		return types.ActionPause, true
	}
	if state.internal == nil && !time.Now().Before(state.admissionDeadline) {
		return rejectManagedRequest(ctx, 403, "execution_admission_expired"), true
	}
	if len(body) == 0 || uint64(len(body)) > uint64(defaultMaxBodyBytes) {
		return rejectManagedRequest(ctx, 413, "request_body_out_of_bounds"), true
	}
	var object map[string]json.RawMessage
	if err := decodeManagedJSON(body, &object, false); err != nil || object == nil {
		return rejectManagedRequest(ctx, 400, "invalid_json_body"), true
	}
	var model string
	if json.Unmarshal(object["model"], &model) != nil || model != managed.release.Descriptor.UpstreamModel {
		return rejectManagedRequest(ctx, 400, "mapped_model_differs_from_selection"), true
	}
	if stream, exists := object["stream"]; exists {
		if json.Unmarshal(stream, &state.streaming) != nil || bytes.Equal(bytes.TrimSpace(stream), []byte("null")) {
			return rejectManagedRequest(ctx, 400, "invalid_stream_flag"), true
		}
	}
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return rejectManagedRequest(ctx, 400, "original_body_basis_missing"), true
	}
	basis, err := bodybasis.Read(headers)
	if err != nil || basis.PublicModel() != managed.release.PublicModel {
		return rejectManagedRequest(ctx, 400, "original_body_basis_missing"), true
	}
	if err := retainManagedTextBody(state, basis, body); err != nil {
		return rejectManagedRequest(ctx, 400, "text_execution_body_invalid"), true
	}
	// The RPC callback owns these original mapped bytes. It does not trust an
	// internal-cost header, allocate another call or reread a mutable body.
	body = append([]byte(nil), body...)
	ready := func() {
		if managed.health != nil {
			authorizeManagedHealth(ctx, state, basis, body)
		} else if request, ok := prepareManagedSelectedRequest(ctx, state, body); ok {
			dispatchManagedPreparedRequest(ctx, state, basis, request)
		}
	}
	if state.internal != nil {
		checkManagedInternalCall(ctx, state, basis, managedOperation(managed), ready)
	} else {
		ready()
	}
	return types.ActionPause, true
}

// Credential choice has already frozen this request's config before protocol
// conversion installs its Authorization. No callback can choose another member.
func prepareManagedSelectedRequest(ctx wrapper.HttpContext, state *managedExecutionState, body []byte) (managedProtocolRequest, bool) {
	managed := state.config
	request, err := managed.profile.PrepareRequest(body, managed.release.Descriptor.UpstreamModel, managed.settings.CredentialValue)
	if err != nil {
		rejectManagedPreparation(ctx, state, 400, "unsupported_or_invalid_protocol_request")
		return managedProtocolRequest{}, false
	}
	if len(request.Body) == 0 || uint64(len(request.Body)) > uint64(defaultMaxBodyBytes) {
		rejectManagedPreparation(ctx, state, 413, "mapped_request_body_out_of_bounds")
		return managedProtocolRequest{}, false
	}
	var outboundObject map[string]json.RawMessage
	var model string
	if decodeManagedJSON(request.Body, &outboundObject, false) != nil || json.Unmarshal(outboundObject["model"], &model) != nil || model != managed.release.Descriptor.UpstreamModel {
		rejectManagedPreparation(ctx, state, 500, "protocol_conversion_changed_selection")
		return managedProtocolRequest{}, false
	}
	state.streaming, state.expectedEmbeddings = request.Streaming, request.ExpectedEmbeddings
	if !time.Now().Before(state.admissionDeadline) {
		rejectManagedPreparation(ctx, state, 403, "execution_admission_expired")
		return managedProtocolRequest{}, false
	}
	attemptID, err := uuid.NewRandom()
	if err != nil {
		rejectManagedPreparation(ctx, state, 500, "execution_identity_unavailable")
		return managedProtocolRequest{}, false
	}
	state.attemptID = attemptID.String()
	if proxywasm.ReplaceHttpRequestHeader(managedAttemptHeader, state.attemptID) != nil {
		rejectManagedPreparation(ctx, state, 500, "execution_identity_install_failed")
		return managedProtocolRequest{}, false
	}
	ctx.SetUserAttribute("managed_attempt_id", state.attemptID)
	return request, true
}

// Remains the sole provider send. Health callbacks can enter it once only after
// an eligible current view or the first certainly committed recovery Consume.
func dispatchManagedPreparedRequest(ctx wrapper.HttpContext, state *managedExecutionState, basis bodybasis.Snapshot, request managedProtocolRequest) {
	if managedTextStopped(state) || state.dispatchStarted {
		return
	}
	if !time.Now().Before(state.admissionDeadline) || state.health != nil && state.health.claimID != nil && !time.Now().Before(state.health.claimDeadline) {
		rejectManagedPreparation(ctx, state, 503, "health_or_admission_expired")
		return
	}
	managed := state.config
	body := request.Body
	if err := prepareManagedExecution(state, basis, body); err != nil {
		managedFactGap(ctx, "original_selection_invalid")
		rejectManagedPreparation(ctx, state, 500, "original_selection_invalid")
		return
	}
	registration, err := managedRegistration(state, basis)
	if err != nil {
		managedFactGap(ctx, "registration_invalid")
	} else {
		dispatchManagedSnapshot(ctx, state, registration)
	}

	// This is the only business send. Redis callbacks never resume the router or
	// invoke it again. A DispatchHttpCall return is handoff, not wire proof.
	state.dispatchStarted = true
	markManagedTextAttempt(state)
	recordManagedAccessAttempt(ctx, state)
	outbound := [][2]string{{":method", "POST"}, {":path", managed.settings.UpstreamPath}, {":authority", managed.settings.Authority}, {managedCallHeader, state.callID}, {managedAttemptHeader, state.attemptID}}
	outbound = append(outbound, request.Headers...)
	_, err = proxywasm.DispatchHttpCall(managed.release.Descriptor.ClusterRef, outbound, body, nil, managed.settings.TimeoutMS, func(numHeaders, bodySize, numTrailers int) {
		onManagedHTTPResponse(ctx, state, numHeaders, bodySize)
	})
	if err != nil && !state.callbackSeen {
		state.callbackSeen = true
		outcome := executionfacts.AttemptOutcome{ObservedAt: managedNow(), SendState: "not_sent", Acceptance: "not_sent", ExecutionResult: "failed", TransportError: "http_call_dispatch_failed", Usage: executionfacts.MissingTokenUsage("request_not_sent")}
		recordManagedAttempt(ctx, state, outcome)
		if !state.closed {
			sendManagedFinal(ctx, state, outcome, "failed", 502, [][2]string{{"content-type", "application/json"}}, managedErrorBody("upstream_dispatch_failed"))
		}
	} else if err == nil {
		state.handedOff = true
	}
}

func prepareManagedExecution(s *managedExecutionState, basis bodybasis.Snapshot, body []byte) error {
	selection := stateSelection(s, basis)
	digest, err := managedSelectionDigest(s.config, selection)
	if err != nil {
		return err
	}
	d := s.config.release.Descriptor
	s.original = executionfacts.Envelope{
		ProducerID: s.config.settings.ProducerID, CallID: s.callID, AttemptID: s.attemptID,
		ReleaseID: d.ReleaseID, RouteID: s.config.release.RouteID, SalesAssetID: s.config.release.SalesAssetID,
		TenantID: s.identity.TenantID, PrincipalID: s.identity.PrincipalID, AccountID: s.identity.AccountID,
		PrincipalKind: "user", BillingMode: "customer", OperationKind: managedOperation(s.config), OccurredAt: s.businessUTC,
		TargetID: d.TargetID, BindingID: d.BindingID, ProviderID: d.ProviderID, ProviderAccountID: d.ProviderAccountID,
		CredentialRef: s.config.release.CredentialRef, CredentialRevision: s.config.release.CredentialRevision,
		SelectionDigest: digest, UpstreamModel: d.UpstreamModel, OutboundBodyDigest: managedBodyDigest(body),
		SendTransport: "http_call", RecordingOwner: "ai_proxy", Mechanism: managedTextMechanism(s), ParentAttemptID: s.parentAttemptID,
	}
	if err := setManagedOriginalCaller(s, &s.original); err != nil {
		return err
	}
	if managedOperation(s.config) == executionfacts.NativeEmbeddingOperation {
		s.embeddingAttempt, err = executionfacts.NewEmbeddingAttempt(s.original)
	} else {
		s.attempt, err = executionfacts.NewAttempt(s.original)
	}
	if err != nil {
		return err
	}
	s.prepared = true
	return nil
}

func stateSelection(s *managedExecutionState, basis bodybasis.Snapshot) []byte {
	if len(s.frozenSelection) != 0 {
		return append([]byte(nil), s.frozenSelection...)
	}
	d := s.config.release.Descriptor
	selection := map[string]string{
		"call_id": s.callID, "release_id": d.ReleaseID, "target_id": d.TargetID, "binding_id": d.BindingID, "attempt_id": s.attemptID,
		"operation_kind": managedOperation(s.config), "provider_account_id": d.ProviderAccountID, "upstream_model": d.UpstreamModel, "cluster_ref": d.ClusterRef,
		"credential_ref": s.config.release.CredentialRef, "credential_revision": s.config.release.CredentialRevision, "selection_generation": "1",
		"selection_canonicalization": "selection-json.v1", "original_body_digest": basis.Digest(), "mapper_digest": d.MapperDigest, "proxy_digest": d.ProxyDigest, "descriptor_digest": d.DescriptorDigest,
		"mechanism": managedTextMechanism(s), "send_transport": "http_call", "recording_owner": "ai_proxy",
	}
	if s.selectionGeneration != 0 {
		selection["selection_generation"] = strconv.FormatUint(uint64(s.selectionGeneration), 10)
	}
	if s.parentAttemptID != "" {
		selection["parent_attempt_id"] = s.parentAttemptID
	}
	if s.config.health != nil {
		selection["gateway_boot_id"] = s.config.health.owner
	}
	if s.original.SelectionDigest != "" {
		selection["selection_digest"] = s.original.SelectionDigest
	}
	raw, _ := json.Marshal(selection) // The closed string-only shape cannot fail.
	return raw
}

func onManagedHTTPResponse(ctx wrapper.HttpContext, s *managedExecutionState, numHeaders, bodySize int) {
	if s.callbackSeen {
		return
	}
	s.callbackSeen = true
	headers, headerErr := proxywasm.GetHttpCallResponseHeaders()
	var body []byte
	var bodyErr error
	if bodySize < 0 || uint64(bodySize) > uint64(defaultMaxBodyBytes) {
		bodyErr = errors.New("response body out of bounds")
	} else if bodySize > 0 {
		body, bodyErr = proxywasm.GetHttpCallResponseBody(0, bodySize)
		if len(body) != bodySize && bodyErr == nil {
			bodyErr = errors.New("response body incomplete")
		}
	}
	outcome := managedResponseOutcome(s, headers, headerErr, body, bodyErr)
	if numHeaders != len(headers) || headerErr != nil {
		outcome.HeadersComplete = false
	}
	recordManagedAttempt(ctx, s, outcome)
	if managedTextStopped(s) {
		// A late provider callback can correct the attempt. It cannot fabricate
		// a final client delivery after this HTTP stream has already closed.
		return
	}
	finish := func() { finishManagedHTTPResponse(ctx, s, outcome, headers, body, bodyErr) }
	if continueManagedText(ctx, s, outcome, headers, body, finish) {
		return
	}
	finish()
}

func finishManagedHTTPResponse(ctx wrapper.HttpContext, s *managedExecutionState, outcome executionfacts.AttemptOutcome, headers [][2]string, body []byte, bodyErr error) {
	if managedTextStopped(s) {
		return
	}
	if outcome.HTTPStatus == nil || bodyErr != nil {
		// The local 502 is a response policy, never a fabricated upstream status.
		sendManagedFinal(ctx, s, outcome, "failed", 502, [][2]string{{"content-type", "application/json"}}, managedErrorBody("upstream_response_unavailable"))
		return
	}
	deliveryState := "unknown"
	if *outcome.HTTPStatus >= 400 {
		deliveryState = "failed"
	}
	if *outcome.HTTPStatus >= 200 && *outcome.HTTPStatus < 300 {
		deliveryState = outcome.ExecutionResult
		if deliveryState != "succeeded" && deliveryState != "partial" && deliveryState != "failed" {
			deliveryState = "unknown"
		}
	}
	clientResponse, err := s.config.profile.ConvertResponse(ctx, *outcome.HTTPStatus, headers, body)
	if err != nil {
		managedFactGap(ctx, "protocol_response_conversion_failed")
		sendManagedFinal(ctx, s, outcome, "failed", 502, [][2]string{{"content-type", "application/json"}}, managedErrorBody("upstream_protocol_response_incomplete"))
		return
	}
	sendManagedFinal(ctx, s, outcome, deliveryState, uint32(*outcome.HTTPStatus), clientResponse.Headers, clientResponse.Body)
}

// ai-billing is the only delivery producer on this route. It receives precisely
// this frozen snapshot through the protected response carrier, validates it
// against its own release/caller configuration, and appends those same bytes.
// An upstream carrier is always removed, including when local encoding fails.
func sendManagedFinal(ctx wrapper.HttpContext, s *managedExecutionState, outcome executionfacts.AttemptOutcome, deliveryState string, status uint32, headers [][2]string, body []byte) {
	if !beginManagedTextFinal(s) {
		return
	}
	responseHeaders := managedResponseHeaders(headers)
	snapshot, err := managedDeliverySnapshot(s, outcome, deliveryState)
	if err != nil {
		managedFactGap(ctx, "delivery_fact_invalid")
	} else {
		encoded, err := encodeManagedDeliveryBridge(snapshot)
		if err != nil {
			managedFactGap(ctx, "delivery_bridge_invalid")
		} else {
			s.lastDelivery = &snapshot
			responseHeaders = append(responseHeaders, [2]string{executionfacts.DeliveryBridgeHeader, encoded})
		}
	}
	if proxywasm.SendHttpResponse(status, responseHeaders, body, -1) != nil {
		managedFactGap(ctx, "local_response_not_acknowledged")
	}
}

func managedResponseOutcome(s *managedExecutionState, headers [][2]string, headerErr error, body []byte, bodyErr error) executionfacts.AttemptOutcome {
	o := executionfacts.AttemptOutcome{ObservedAt: managedNow(), SendState: "handed_off", Acceptance: "unknown", ExecutionResult: "unknown", Usage: executionfacts.MissingTokenUsage("provider_usage_not_observed")}
	statusText, statusErr := managedSingleHeader(headers, ":status")
	status, parseErr := strconv.ParseUint(statusText, 10, 16)
	if headerErr == nil && statusErr == nil && parseErr == nil && status >= 100 && status <= 599 && strconv.FormatUint(status, 10) == statusText {
		actual := uint16(status)
		o.HTTPStatus = &actual
		o.HeadersComplete = true
		o.SendState = "observed_sent"
		o.EvidenceRefs = append(o.EvidenceRefs, "http-call:"+s.attemptID+":status:"+statusText)
	} else {
		o.TransportError = "upstream_status_not_observed"
	}
	if bodyErr == nil {
		o.BodyComplete = true
		o.ResponseBodyDigest = managedBodyDigest(body)
		o.EvidenceRefs = append(o.EvidenceRefs, "response-body:"+o.ResponseBodyDigest)
	} else {
		o.TransportError = "upstream_body_not_observed"
	}
	if retryAfter, err := managedSingleHeader(headers, "retry-after"); err == nil {
		deadline, evidence := managedProviderDeadline(retryAfter, o.ObservedAt)
		o.RetryAfter = deadline
		if s.health != nil {
			s.health.retryAfterEvidence = evidence
		}
	}
	if o.HTTPStatus == nil {
		o.Usage = executionfacts.MissingTokenUsage("upstream_status_not_observed")
		return o
	}
	if bodyErr != nil {
		o.Usage = executionfacts.MissingTokenUsage("upstream_body_not_observed")
		return o
	}
	var observed managedProtocolObservation
	if managedOperation(s.config) == executionfacts.NativeEmbeddingOperation {
		observed = managedEmbeddingObservation(*o.HTTPStatus, headers, body, o.BodyComplete, s.expectedEmbeddings)
	} else {
		observed = s.config.profile.ObserveResponse(*o.HTTPStatus, headers, body, o.BodyComplete)
	}
	o.Usage = observed.Usage
	if o.Usage == nil {
		o.Usage = executionfacts.MissingTokenUsage("provider_usage_not_observed")
	}
	o.ExecutionResult = observed.ExecutionResult
	o.Acceptance = observed.Acceptance
	o.BodyComplete = observed.BodyComplete
	o.ResponseModel = observed.ResponseModel
	if !managedID(o.ResponseModel) {
		o.ResponseModel = ""
	}
	if observed.ProviderError {
		o.ProviderError = "provider_error_response"
	}
	if *o.HTTPStatus >= 400 {
		o.ErrorClass = "upstream_http_error"
	}

	return o
}

func recordManagedAttempt(ctx wrapper.HttpContext, s *managedExecutionState, outcome executionfacts.AttemptOutcome) {
	var attempt executionfacts.Snapshot
	var err error
	if managedOperation(s.config) == executionfacts.NativeEmbeddingOperation {
		attempt, err = s.embeddingAttempt.Finish(outcome)
		if err == nil && s.lastAttempt != nil {
			attempt, err = executionfacts.CorrectEmbedding(*s.lastAttempt, attempt.Envelope())
		}
	} else {
		attempt, err = s.attempt.Finish(outcome)
		if err == nil && s.lastAttempt != nil {
			attempt, err = executionfacts.Correct(*s.lastAttempt, attempt.Envelope())
		}
	}
	if err != nil {
		managedFactGap(ctx, "attempt_fact_invalid")
		return
	}
	s.lastAttempt = &attempt
	dispatchManagedSnapshot(ctx, s, attempt)
	reportManagedHealthOutcome(s, attempt)
}

func managedDeliverySnapshot(s *managedExecutionState, outcome executionfacts.AttemptOutcome, deliveryState string) (executionfacts.Snapshot, error) {
	if outcome.Usage == nil {
		outcome.Usage = executionfacts.MissingTokenUsage("provider_usage_not_observed")
	}
	delivery := executionfacts.Envelope{
		ProducerID: s.config.settings.ProducerID, Kind: "call_delivery", CallID: s.callID,
		ReleaseID: s.original.ReleaseID, RouteID: s.original.RouteID, SalesAssetID: s.original.SalesAssetID,
		TenantID: s.identity.TenantID, PrincipalID: s.identity.PrincipalID, AccountID: s.identity.AccountID,
		PrincipalKind: "user", BillingMode: "customer", OperationKind: managedOperation(s.config),
		OccurredAt: s.businessUTC, ObservedAt: outcome.ObservedAt, EvidenceRefs: outcome.EvidenceRefs,
		DeliveryState: deliveryState, ServedFrom: "upstream", PublicModel: s.config.release.PublicModel,
		ResponseModel: outcome.ResponseModel, Usage: outcome.Usage, ErrorClass: outcome.ErrorClass,
	}
	if err := setManagedOriginalCaller(s, &delivery); err != nil {
		return executionfacts.Snapshot{}, err
	}
	if managedOperation(s.config) == executionfacts.NativeEmbeddingOperation {
		return executionfacts.FreezeEmbedding(delivery)
	}
	return executionfacts.Freeze(delivery)
}

func dispatchManagedSnapshot(ctx wrapper.HttpContext, s *managedExecutionState, snapshot executionfacts.Snapshot) {
	retained, err := s.ledger.Retain(snapshot)
	if err != nil {
		managedFactGap(ctx, "fact_retention_failed")
		return
	}
	delivery := s.deliveries[retained.EventID()]
	if delivery == nil {
		delivery, err = executionfacts.NewDelivery(retained)
		if err != nil {
			managedFactGap(ctx, "fact_snapshot_invalid")
			return
		}
		s.deliveries[retained.EventID()] = delivery
	}
	var appendToStream executionfacts.AppendFunc
	if s.config.redis != nil && s.config.settings.PrivateStream != "" {
		appendToStream = func(body []byte, complete func(executionfacts.AppendResult)) error {
			return s.config.redis.Command([]interface{}{"XADD", s.config.settings.PrivateStream, "*", "message_type", retained.MessageType(), "event", string(body)}, func(response resp.Value) {
				if response.Error() != nil {
					complete(executionfacts.AppendResult{Err: errors.New("private stream response failed")})
					return
				}
				complete(executionfacts.AppendResult{StreamID: response.String()})
			})
		}
	}
	if err := delivery.Dispatch(appendToStream, managedDeliveryObserver(ctx)); err != nil {
		managedFactGap(ctx, "fact_dispatch_failed")
	}
}

func managedDeliveryObserver(ctx wrapper.HttpContext) func(executionfacts.DeliveryStatus) {
	return func(status executionfacts.DeliveryStatus) {
		ctx.SetUserAttribute("execution_fact_delivery", status.State)
		if status.State == "gap" || status.State == "unknown" {
			managedFactGap(ctx, status.Reason)
		}
	}
}

func managedFactGap(ctx wrapper.HttpContext, reason string) {
	recordManagedAccessFailure(ctx, reason, true)
	ctx.SetUserAttribute("execution_fact_gap", true)
	ctx.SetUserAttribute("execution_fact_gap_reason", reason)
	// Every reason is an internal fixed code. Never log request/response bodies,
	// provider error text, configured credentials or a raw Redis error.
	callID, attemptID := "", ""
	if state, ok := ctx.GetContext(managedExecutionContextKey).(*managedExecutionState); ok && state != nil {
		callID, attemptID = state.callID, state.attemptID
	}
	log.Warnf("ai-proxy managed execution fact gap fail open, call_id:%s attempt_id:%s reason:%s", callID, attemptID, reason)
}

func onManagedStreamDone(ctx wrapper.HttpContext, cfg config.PluginConfig) {
	defer closeManagedAccessLog(ctx)
	s, ok := ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	if !ok || s == nil || s.closed {
		return
	}
	if s.text != nil {
		s.text.closed = true
		for _, attempt := range s.text.attempts {
			attempt.closed = true
		}
		s = s.text.current
	}
	s.closed = true
	if s.prepared && s.dispatchStarted && !s.callbackSeen {
		managedFactGap(ctx, "upstream_callback_missing")
		outcome := executionfacts.AttemptOutcome{ObservedAt: managedNow(), SendState: "unknown", Acceptance: "unknown", ExecutionResult: "unknown", TransportError: "upstream_callback_missing", Usage: executionfacts.MissingTokenUsage("upstream_callback_missing")}
		if s.handedOff {
			outcome.SendState = "handed_off"
		}
		recordManagedAttempt(ctx, s, outcome)
	}
	// Event ACKs belong to the plugin context and may arrive after this HTTP
	// context is deleted. Only the Redis result/timeout decides delivery failure;
	// completing the business response is not an ACK deadline.
}

func rejectManagedRequest(ctx wrapper.HttpContext, status uint32, reason string) types.Action {
	recordManagedAccessFailure(ctx, reason, false)
	ctx.SetUserAttribute("managed_execution_rejection", reason)
	_ = sendManagedError(status, reason)
	return types.ActionPause
}

func managedErrorBody(reason string) []byte {
	body, _ := json.Marshal(map[string]interface{}{"error": map[string]string{"type": "managed_execution_error", "code": reason}})
	return body
}

func sendManagedError(status uint32, reason string) error {
	return proxywasm.SendHttpResponseWithDetail(status, "ai-proxy.managed_execution", [][2]string{{"content-type", "application/json"}}, managedErrorBody(reason), -1)
}

func managedResponseHeaders(headers [][2]string) [][2]string {
	result := make([][2]string, 0, len(headers))
	for _, header := range headers {
		name := strings.ToLower(header[0])
		if strings.HasPrefix(name, ":") || strings.EqualFold(name, executionfacts.DeliveryBridgeHeader) {
			continue
		}
		switch name {
		case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade", "content-length":
			continue
		}
		result = append(result, header)
	}
	return result
}

func managedRequiredFields(object gjson.Result, fields string) bool {
	if !object.IsObject() {
		return false
	}
	for _, field := range strings.Fields(fields) {
		value := object.Get(field)
		if !value.Exists() || value.Type == gjson.Null {
			return false
		}
	}
	return true
}

func managedSingleHeader(headers [][2]string, name string) (string, error) {
	value := ""
	found := false
	for _, header := range headers {
		if !strings.EqualFold(header[0], name) {
			continue
		}
		if found {
			return "", errors.New("duplicate managed execution header")
		}
		found = true
		value = header[1]
	}
	if !found {
		return "", errors.New("managed execution header missing")
	}
	return value, nil
}

func managedID(value string) bool {
	return value != "" && len(value) <= 256 && utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) < 0
}

func managedAuthority(value string) bool {
	if !managedID(value) || strings.IndexAny(value, " /\\@?#\t") >= 0 {
		return false
	}
	parsed, err := url.Parse("https://" + value)
	return err == nil && parsed.Host == value && parsed.Hostname() != "" && parsed.User == nil
}

func managedDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, r := range value[7:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func managedDescriptorDigest(raw []byte) (string, error) {
	// Validate the original complete object before removing its self reference.
	canonical, err := mfjson.Canonicalize(raw)
	if err != nil {
		return "", err
	}
	var descriptor map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &descriptor); err != nil || descriptor == nil {
		return "", errors.New("invalid managed execution descriptor object")
	}
	delete(descriptor, "descriptor_digest")
	projection, err := json.Marshal(descriptor)
	if err != nil {
		return "", err
	}
	return mfjson.Digest(projection)
}

func managedBodyDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func managedNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func managedInteger(value string) (*big.Int, bool) {
	if value == "" || len(value) > 39 || (len(value) > 1 && value[0] == '0') {
		return nil, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return nil, false
		}
	}
	n, ok := new(big.Int).SetString(value, 10)
	return n, ok && n.BitLen() <= 128
}

// Only observed exact provider counts become reported usage. Missing, negative,
// fractional, exponent, overflowing or incomplete counts never become zeroes.
func managedOpenAIUsage(object map[string]json.RawMessage) *executionfacts.Usage {
	missing := func() *executionfacts.Usage { return executionfacts.MissingTokenUsage("provider_usage_not_observed") }
	var usage map[string]json.RawMessage
	if raw, exists := object["usage"]; !exists || json.Unmarshal(raw, &usage) != nil || usage == nil {
		return missing()
	}
	input, inOK := managedCount(usage["prompt_tokens"])
	output, outOK := managedCount(usage["completion_tokens"])
	if !inOK || !outOK {
		return executionfacts.MissingTokenUsage("provider_usage_incomplete_or_invalid")
	}
	if totalRaw, exists := usage["total_tokens"]; exists {
		total, ok := managedCount(totalRaw)
		if !ok || total.Cmp(new(big.Int).Add(new(big.Int).Set(input), output)) != 0 {
			return executionfacts.MissingTokenUsage("provider_usage_total_conflict")
		}
	}
	quantities := map[string]string{"input": input.String(), "output": output.String()}
	var details map[string]json.RawMessage
	if raw, exists := usage["prompt_tokens_details"]; exists && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if json.Unmarshal(raw, &details) != nil {
			return executionfacts.MissingTokenUsage("provider_cache_split_invalid")
		}
	}
	if cached, exists := details["cached_tokens"]; exists {
		hit, ok := managedCount(cached)
		if !ok || hit.Cmp(input) > 0 {
			return executionfacts.MissingTokenUsage("provider_cache_split_invalid")
		}
		quantities["cache_hit"] = hit.String()
		quantities["cache_miss"] = new(big.Int).Sub(new(big.Int).Set(input), hit).String()
	}
	hitRaw, hitExists := usage["prompt_cache_hit_tokens"]
	missRaw, missExists := usage["prompt_cache_miss_tokens"]
	if hitExists || missExists {
		hit, hOK := managedCount(hitRaw)
		miss, mOK := managedCount(missRaw)
		if !hitExists || !missExists || !hOK || !mOK || new(big.Int).Add(new(big.Int).Set(hit), miss).Cmp(input) != 0 {
			return executionfacts.MissingTokenUsage("provider_cache_split_invalid")
		}
		if prior, exists := quantities["cache_hit"]; exists && prior != hit.String() {
			return executionfacts.MissingTokenUsage("provider_cache_split_conflict")
		}
		quantities["cache_hit"], quantities["cache_miss"] = hit.String(), miss.String()
	}
	return &executionfacts.Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: quantities}
}

func managedCount(raw json.RawMessage) (*big.Int, bool) {
	// OpenAI usage is a JSON number. Do not accept a string or pass through f64.
	return managedInteger(string(bytes.TrimSpace(raw)))
}

// This parses an aggregated HTTP-call response, not real-time frame delivery.
// A final usage frame alone does not prove an intact SSE terminal boundary.
func managedSSEObservation(body []byte) (*executionfacts.Usage, string, bool) {
	missing := executionfacts.MissingTokenUsage("stream_terminal_not_observed")
	if !utf8.Valid(body) {
		return missing, "unknown", false
	}
	body = bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf})
	var data []byte
	var eventName string
	hasData, done, invalid, providerError, partial := false, false, false, false, false
	finished := map[string]bool{}
	var finalUsage *executionfacts.Usage
	allFinished := func() bool {
		if len(finished) == 0 {
			return false
		}
		for _, ended := range finished {
			if !ended {
				return false
			}
		}
		return true
	}
	consume := func() {
		if !hasData {
			eventName = ""
			return
		}
		defer func() { data = nil; hasData = false; eventName = "" }()
		if done {
			invalid = true
			return
		}
		if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
			done = true
			return
		}
		var object map[string]json.RawMessage
		if decodeManagedJSON(data, &object, false) != nil || object == nil {
			invalid = true
			return
		}
		if value, exists := object["error"]; eventName == "error" || exists && !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			providerError = true
		}
		var choices []map[string]json.RawMessage
		if raw, exists := object["choices"]; exists {
			if json.Unmarshal(raw, &choices) != nil {
				invalid = true
				return
			}
			for _, choice := range choices {
				index, ok := managedCount(choice["index"])
				if !ok || index.BitLen() > 32 {
					invalid = true
					return
				}
				key := index.String()
				if finished[key] {
					invalid = true
					return
				}
				finished[key] = false
				if raw, exists := choice["finish_reason"]; exists && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
					var reason string
					if json.Unmarshal(raw, &reason) != nil || reason == "" {
						invalid = true
						return
					}
					finished[key] = true
					if reason == "length" || reason == "content_filter" {
						partial = true
					}
				}
			}
		}
		if raw, exists := object["usage"]; exists && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) && allFinished() {
			finalUsage = managedOpenAIUsage(object)
		}
	}
	for start := 0; start < len(body); {
		end := start
		for end < len(body) && body[end] != '\r' && body[end] != '\n' {
			end++
		}
		if end == len(body) {
			invalid = true
			break
		}
		line := body[start:end]
		start = end + 1
		if body[end] == '\r' && start < len(body) && body[start] == '\n' {
			start++
		}
		if len(line) == 0 {
			consume()
			continue
		}
		if line[0] == ':' {
			continue
		}
		field, value, hasColon := bytes.Cut(line, []byte(":"))
		if !hasColon {
			value = nil
		}
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}
		switch string(field) {
		case "data":
			if len(data)+len(value)+1 > executionfacts.MaxEnvelopeBytes {
				invalid = true
				continue
			}
			if hasData {
				data = append(data, '\n')
			}
			data = append(data, value...)
			hasData = true
		case "event":
			eventName = string(value)
		}
	}
	if hasData || invalid || !done || !allFinished() {
		return missing, "unknown", providerError
	}
	if providerError {
		return executionfacts.MissingTokenUsage("stream_provider_error"), "failed", true
	}
	if finalUsage == nil {
		finalUsage = executionfacts.MissingTokenUsage("stream_usage_not_observed")
	}
	if partial {
		return finalUsage, "partial", false
	}
	return finalUsage, "succeeded", false
}

func decodeManagedJSON(raw []byte, target interface{}, strict bool) error {
	if err := validateManagedJSON(raw, int(defaultMaxBodyBytes)); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if strict {
		decoder.DisallowUnknownFields()
	}
	return decoder.Decode(target)
}

// No canonicalizer is duplicated here: selection-json.v1 is exclusively owned
// by executionfacts. This bounded input check only rejects ambiguous JSON before
// typed configuration/body decoding, including duplicate names and bad UTF-16.
func validateManagedJSON(raw []byte, limit int) error {
	if len(raw) == 0 || len(raw) > limit || !utf8.Valid(raw) {
		return errors.New("invalid managed JSON input")
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return errors.New("invalid JSON escape")
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return errors.New("invalid Unicode escape")
		}
		n, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return err
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return errors.New("unpaired Unicode surrogate")
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return errors.New("unpaired Unicode surrogate")
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return errors.New("unpaired Unicode surrogate")
			}
			i += 6
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 128 {
			return errors.New("managed JSON nesting limit exceeded")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			keys := map[string]bool{}
			for decoder.More() {
				token, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := token.(string)
				if !ok || keys[key] {
					return errors.New("duplicate managed JSON member")
				}
				keys[key] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return errors.New("invalid managed JSON delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("trailing managed JSON input")
	}
	return nil
}
