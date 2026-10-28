package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const nativeBillingContextKey = "ai-billing-native-execution.v1"

type nativeBillingSettings struct {
	ProducerID         string `json:"producer_id"`
	CallIdentitySource string `json:"call_identity_source"`
}

// The SDK supplies this complete reserved object after verifying the released
// semantic plugin configuration. An incoming HTTP header cannot install it.
type nativeBillingRelease struct {
	SchemaVersion      string                  `json:"schema_version"`
	Descriptor         nativeBillingDescriptor `json:"descriptor"`
	RouteID            string                  `json:"route_id"`
	SalesAssetID       string                  `json:"sales_asset_id"`
	PublicModel        string                  `json:"public_model"`
	CredentialRef      string                  `json:"credential_ref"`
	CredentialRevision string                  `json:"credential_revision"`
}

type nativeBillingDescriptor struct {
	ReleaseID              string                             `json:"release_id"`
	TargetID               string                             `json:"target_id"`
	BindingID              string                             `json:"binding_id"`
	ProviderID             string                             `json:"provider_id"`
	ProviderAccountID      string                             `json:"provider_account_id"`
	RegionEndpointRef      string                             `json:"region_endpoint_ref"`
	ServiceRef             string                             `json:"service_ref"`
	ClusterRef             string                             `json:"cluster_ref"`
	CredentialPoolRef      string                             `json:"credential_pool_ref"`
	CredentialPoolRevision string                             `json:"credential_pool_revision"`
	AdapterRef             string                             `json:"adapter_ref"`
	AdapterRevision        string                             `json:"adapter_revision"`
	MapperDigest           string                             `json:"mapper_digest"`
	ProxyDigest            string                             `json:"proxy_digest"`
	DescriptorDigest       string                             `json:"descriptor_digest"`
	UpstreamModel          string                             `json:"upstream_model"`
	ConfigCanonicalization string                             `json:"config_canonicalization"`
	Transport              nativeBillingTransport             `json:"transport_contract"`
	Capabilities           map[string]nativeBillingCapability `json:"capabilities"`
}

type nativeBillingTransport struct {
	Host           string `json:"host"`
	SNI            string `json:"sni"`
	PathAdapterRef string `json:"path_adapter_ref"`
}

type nativeBillingCapability struct {
	SendMode               string `json:"send_mode"`
	ResponseClassifierRef  string `json:"response_classifier_ref"`
	QueryCapabilityRef     string `json:"query_capability_ref,omitempty"`
	ProviderIdempotencyRef string `json:"provider_idempotency_ref,omitempty"`
	MaxAttempts            uint32 `json:"max_attempts"`
	MaxFallbackHops        uint32 `json:"max_fallback_hops"`
}

type nativeBillingConfig struct {
	producerID string
	release    nativeBillingRelease
}

type nativeBillingState struct {
	config                         *nativeBillingConfig
	identity                       balanceidentity.Snapshot
	origin                         executionfacts.AdmittedCall
	valid, bridgeSeen, gap, closed bool
	snapshot                       executionfacts.Snapshot
	delivery                       *executionfacts.Delivery
}

// configureNativeBilling clears inherited authority and accepts only a complete,
// route-bound SDK envelope paired with explicit semantic producer configuration.
// Redis initialization and inheritance belong to the existing configuration
// parser. A complete independent rule owns its connection and private Stream.
func configureNativeBilling(value gjson.Result, cfg *BillingConfig) error {
	cfg.NativeExecution = nil
	settingsValue, releaseValue := value.Get("native_execution"), value.Get("_modelfusion_execution")
	if !settingsValue.Exists() && !releaseValue.Exists() {
		return nil
	}
	if !settingsValue.IsObject() || !releaseValue.IsObject() || value.Get("path_scope").Type != gjson.String || value.Get("path_scope").String() != PathScopeBoundTarget {
		return errors.New("native billing requires paired route-bound execution configuration")
	}
	if err := validateNativeBillingJSON([]byte(value.Raw)); err != nil {
		return errors.New("native billing configuration is ambiguous or exceeds its budget")
	}
	for _, field := range []string{"enable_methods", "enable_path_suffixes"} {
		filter := value.Get(field)
		if filter.Exists() && (!filter.IsArray() || len(filter.Array()) != 0) {
			return errors.New("native billing does not accept URL suffix or method filters")
		}
	}
	var settings nativeBillingSettings
	var release nativeBillingRelease
	if decodeNativeBillingJSON([]byte(settingsValue.Raw), &settings) != nil || decodeNativeBillingJSON([]byte(releaseValue.Raw), &release) != nil {
		return errors.New("native billing requires exact frozen execution fields")
	}
	if !nativeBillingID(settings.ProducerID) || cfg.RedisStream.Stream == "" {
		return errors.New("native billing requires a configured producer and private Stream")
	}
	if settings.CallIdentitySource != executionfacts.CallIdentitySourceAdmission {
		return errors.New("native billing requires the original admission call identity capability")
	}
	if err := validateNativeBillingRelease(releaseValue, release); err != nil {
		return err
	}
	cfg.NativeExecution = &nativeBillingConfig{producerID: settings.ProducerID, release: release}
	cfg.PathScope = PathScopeBoundTarget
	cfg.EnableMethods = nil
	cfg.EnablePathSuffixes = nil
	cfg.dormant = false
	return nil
}

func validateNativeBillingRelease(value gjson.Result, release nativeBillingRelease) error {
	if release.SchemaVersion != "modelfusion-execution.v1" || release.Descriptor.ConfigCanonicalization != "mf-json.v1" || !balanceidentity.ValidID(release.RouteID) || !balanceidentity.ValidID(release.SalesAssetID) || !nativeBillingID(release.PublicModel) || !nativeBillingID(release.CredentialRef) || !nativeBillingRevision(release.CredentialRevision) {
		return errors.New("native billing has invalid released call ownership")
	}
	descriptor := value.Get("descriptor")
	for _, field := range strings.Fields("release_id target_id binding_id provider_id provider_account_id region_endpoint_ref service_ref cluster_ref credential_pool_ref credential_pool_revision adapter_ref adapter_revision mapper_digest proxy_digest descriptor_digest upstream_model config_canonicalization") {
		entry := descriptor.Get(field)
		if entry.Type != gjson.String || !nativeBillingID(entry.String()) {
			return errors.New("native billing requires a complete frozen target descriptor")
		}
	}
	if !nativeBillingRevision(release.Descriptor.CredentialPoolRevision) || !nativeBillingRevision(release.Descriptor.AdapterRevision) {
		return errors.New("native billing requires exact positive released revisions")
	}
	for _, digest := range []string{release.Descriptor.MapperDigest, release.Descriptor.ProxyDigest, release.Descriptor.DescriptorDigest} {
		if !nativeBillingDigest(digest) {
			return errors.New("native billing requires frozen target digests")
		}
	}
	transport := descriptor.Get("transport_contract")
	for _, field := range []string{"host", "sni", "path_adapter_ref"} {
		if transport.Get(field).Type != gjson.String {
			return errors.New("native billing requires a complete released transport")
		}
	}
	if !nativeBillingID(release.Descriptor.Transport.Host) || !nativeBillingID(release.Descriptor.Transport.PathAdapterRef) || (release.Descriptor.Transport.SNI != "" && !nativeBillingID(release.Descriptor.Transport.SNI)) {
		return errors.New("native billing has an invalid released transport")
	}
	capabilities := descriptor.Get("capabilities")
	if !capabilities.IsObject() || !capabilities.Get("text_generate").IsObject() {
		return errors.New("native billing requires the released text capability")
	}
	for operation, capability := range capabilities.Map() {
		if operation != "text_generate" && operation != "image_generate" && operation != "video_generate" && operation != "task_query" {
			return errors.New("native billing has an undeclared released operation")
		}
		parsed := release.Descriptor.Capabilities[operation]
		if !capability.IsObject() || !nativeBillingID(parsed.ResponseClassifierRef) || (parsed.SendMode != "single_send" && parsed.SendMode != "observable_retry") || parsed.MaxAttempts == 0 || capability.Get("max_attempts").Type != gjson.Number || capability.Get("max_fallback_hops").Type != gjson.Number {
			return errors.New("native billing has an invalid released capability")
		}
		for _, optional := range []string{"query_capability_ref", "provider_idempotency_ref"} {
			if entry := capability.Get(optional); entry.Exists() && (entry.Type != gjson.String || !nativeBillingID(entry.String())) {
				return errors.New("native billing has an invalid released capability reference")
			}
		}
	}
	expected, err := nativeBillingDescriptorDigest([]byte(descriptor.Raw))
	if err != nil || expected != release.Descriptor.DescriptorDigest {
		return errors.New("native billing target descriptor digest differs")
	}
	return nil
}

// Quota precedes billing and installs this call's committed admission identity.
// Proxy or cache later supplies the sole delivery bridge; attempt IDs stay separate.
func onNativeBillingRequestHeaders(ctx wrapper.HttpContext, cfg BillingConfig) (types.Action, bool) {
	if cfg.NativeExecution == nil {
		return types.ActionContinue, false
	}
	ctx.SetContext(ctxBillingEnabled, false)
	ctx.DontReadRequestBody()
	ctx.DontReadResponseBody()
	configured, ok := cfg.NativeExecution.(*nativeBillingConfig)
	if !ok || configured == nil {
		nativeBillingGap(ctx, nil, "native_configuration_missing")
		return types.ActionContinue, true
	}
	if previous := ctx.GetContext(nativeBillingContextKey); previous != nil {
		state, _ := previous.(*nativeBillingState)
		if state != nil {
			state.valid = false
		}
		nativeBillingGap(ctx, state, "native_request_reentry")
		return types.ActionContinue, true
	}
	state := &nativeBillingState{config: configured}
	ctx.SetContext(nativeBillingContextKey, state)
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		nativeBillingGap(ctx, state, "authenticated_identity_missing")
		return types.ActionContinue, true
	}
	captureNativeBillingIdentity(ctx, state, headers)
	return types.ActionContinue, true
}

func captureNativeBillingIdentity(ctx executionFactContext, state *nativeBillingState, headers [][2]string) {
	state.valid = false
	identity, err := balanceidentity.Read(headers)
	if err != nil || identity.TenantID == "" || identity.PrincipalID == "" {
		nativeBillingGap(ctx, state, "authenticated_identity_missing")
		return
	}
	origin, err := executionfacts.ReadAdmittedCall(headers, state.config.release.Descriptor.ReleaseID, time.Now())
	if err != nil {
		nativeBillingGap(ctx, state, "original_call_missing")
		return
	}
	state.identity, state.origin, state.valid = identity, origin, true
}

func onNativeBillingResponseHeaders(ctx wrapper.HttpContext, cfg BillingConfig) (types.Action, bool) {
	if cfg.NativeExecution == nil {
		return types.ActionContinue, false
	}
	headers, err := proxywasm.GetHttpResponseHeaders()
	if err != nil {
		_ = proxywasm.RemoveHttpResponseHeader(executionfacts.DeliveryBridgeHeader)
		nativeBillingGap(ctx, nil, "delivery_bridge_headers_unavailable")
		return types.ActionContinue, true
	}
	value, clean, found, bridgeErr := splitNativeDeliveryBridge(headers)
	if found {
		if err := proxywasm.ReplaceHttpResponseHeaders(clean); err != nil {
			nativeBillingGap(ctx, nil, "delivery_bridge_strip_failed")
			return types.ActionContinue, true
		}
	}
	state, _ := ctx.GetContext(nativeBillingContextKey).(*nativeBillingState)
	if bridgeErr != nil || !found {
		nativeBillingGap(ctx, state, "delivery_bridge_missing_or_duplicate")
		return types.ActionContinue, true
	}
	consumeNativeBillingBridge(ctx, cfg, state, value)
	return types.ActionContinue, true
}

func splitNativeDeliveryBridge(headers [][2]string) (string, [][2]string, bool, error) {
	value, count := "", 0
	clean := make([][2]string, 0, len(headers))
	for _, header := range headers {
		if strings.EqualFold(header[0], executionfacts.DeliveryBridgeHeader) {
			value = header[1]
			count++
		} else {
			clean = append(clean, header)
		}
	}
	if count != 1 {
		return "", clean, count != 0, errors.New("delivery bridge must occur exactly once")
	}
	return value, clean, true, nil
}

func consumeNativeBillingBridge(ctx executionFactContext, cfg BillingConfig, state *nativeBillingState, value string) {
	configured, ok := cfg.NativeExecution.(*nativeBillingConfig)
	if !ok || configured == nil || state == nil || !state.valid || state.config != configured || state.closed {
		nativeBillingGap(ctx, state, "original_native_identity_missing")
		return
	}
	state.bridgeSeen = true
	snapshot, err := executionfacts.DecodeDeliveryBridge(value)
	if err != nil {
		nativeBillingGap(ctx, state, "delivery_bridge_invalid")
		return
	}
	if err := validateNativeBillingDelivery(state, snapshot); err != nil {
		nativeBillingGap(ctx, state, "delivery_bridge_identity_mismatch")
		return
	}
	if state.delivery != nil {
		if !bytes.Equal(state.snapshot.Bytes(), snapshot.Bytes()) {
			nativeBillingGap(ctx, state, "delivery_bridge_changed")
		}
		return
	}
	// This preserves the finalizer's original event/source/revision and byte sequence.
	// No legacy BillingEvent, estimated usage, price lookup, or second attempt exists.
	state.snapshot = snapshot
	var client executionRedisClient
	if cfg.redisClient != nil {
		client = cfg.redisClient
	}
	state.delivery, err = dispatchExecutionFact(ctx, client, cfg.RedisStream.Stream, snapshot)
	if err != nil && state.delivery == nil {
		nativeBillingGap(ctx, state, "delivery_bridge_retention_failed")
	}
}

func validateNativeBillingDelivery(state *nativeBillingState, snapshot executionfacts.Snapshot) error {
	identity, release, fact := state.identity, state.config.release, snapshot.Envelope()
	if identity.Validate() != nil || identity.TenantID == "" || identity.PrincipalID == "" || snapshot.ProducerID() != state.config.producerID || fact.CallID != state.origin.CallID || fact.OccurredAt != state.origin.OccurredAt || fact.Kind != "call_delivery" || fact.PrincipalKind != "user" || fact.BillingMode != "customer" || fact.OperationKind != "text_generate" || fact.ReleaseID != release.Descriptor.ReleaseID || fact.RouteID != release.RouteID || fact.SalesAssetID != release.SalesAssetID || fact.PublicModel != release.PublicModel || fact.AccountID != identity.AccountID || fact.TenantID != identity.TenantID || fact.PrincipalID != identity.PrincipalID {
		return errors.New("delivery differs from original caller or released ownership")
	}
	// E1 bridges final delivery only. Procurement facts belong to the sender's
	// separately registered attempt and are never fabricated by billing.
	if fact.AttemptID != "" || fact.TargetID != "" || fact.BindingID != "" || fact.ProviderID != "" || fact.ProviderAccountID != "" || fact.SelectionDigest != "" || fact.CredentialRef != "" || fact.CredentialRevision != "" || fact.SendState != "" || fact.SendTransport != "" || fact.RecordingOwner != "" || fact.Mechanism != "" {
		return errors.New("final delivery bridge contains unsupported attempt attribution")
	}
	return nil
}

func onNativeBillingStreamDone(ctx wrapper.HttpContext, cfg BillingConfig) {
	if cfg.NativeExecution == nil {
		return
	}
	state, _ := ctx.GetContext(nativeBillingContextKey).(*nativeBillingState)
	if state == nil {
		nativeBillingGap(ctx, nil, "original_native_identity_missing")
		return
	}
	if state.closed {
		return
	}
	state.closed = true
	if state.delivery != nil {
		// The event client owns its ACK in the plugin context. Request completion
		// does not imply an append timeout; a later Redis failure remains visible.
		return
	}
	if !state.gap {
		nativeBillingGap(ctx, state, "delivery_bridge_missing")
	}
}

func nativeBillingGap(ctx executionFactContext, state *nativeBillingState, reason string) {
	if state != nil {
		state.gap = true
	}
	// All callers supply fixed internal reason codes. Never log bridge bytes,
	// provider payloads, configured credentials, or the original authentication key.
	ctx.SetUserAttribute("execution_fact_gap_reason", reason)
	executionDeliveryObserver(ctx)(executionfacts.DeliveryStatus{State: "gap", Reason: reason})
}

func nativeBillingID(value string) bool {
	return value != "" && len(value) <= 256 && value == strings.TrimSpace(value) && utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) < 0
}

func nativeBillingRevision(value string) bool {
	if value == "" || len(value) > 39 || (len(value) > 1 && value[0] == '0') {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	n, ok := new(big.Int).SetString(value, 10)
	return ok && n.Sign() > 0 && n.BitLen() <= 128
}

func nativeBillingDigest(value string) bool {
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

func decodeNativeBillingJSON(raw []byte, output interface{}) error {
	if err := validateNativeBillingJSON(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return errors.New("invalid native execution configuration fields")
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return errors.New("trailing native execution configuration")
	}
	return nil
}

func validateNativeBillingJSON(raw []byte) error {
	if len(raw) == 0 || len(raw) > executionfacts.MaxEnvelopeBytes || !utf8.Valid(raw) {
		return errors.New("native execution JSON exceeds its UTF-8 budget")
	}
	for index := 0; index < len(raw); index++ {
		if raw[index] != '\\' {
			continue
		}
		index++
		if index >= len(raw) {
			return errors.New("invalid native execution JSON escape")
		}
		if raw[index] != 'u' {
			continue
		}
		if index+4 >= len(raw) {
			return errors.New("invalid native execution Unicode escape")
		}
		value, err := strconv.ParseUint(string(raw[index+1:index+5]), 16, 16)
		if err != nil {
			return errors.New("invalid native execution Unicode escape")
		}
		index += 4
		if value >= 0xDC00 && value <= 0xDFFF {
			return errors.New("unpaired native execution Unicode surrogate")
		}
		if value >= 0xD800 && value <= 0xDBFF {
			if index+6 >= len(raw) || raw[index+1] != '\\' || raw[index+2] != 'u' {
				return errors.New("unpaired native execution Unicode surrogate")
			}
			low, err := strconv.ParseUint(string(raw[index+3:index+7]), 16, 16)
			if err != nil || low < 0xDC00 || low > 0xDFFF {
				return errors.New("unpaired native execution Unicode surrogate")
			}
			index += 6
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var visit func(int) error
	visit = func(depth int) error {
		if depth > 128 {
			return errors.New("native execution JSON depth exceeds budget")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		if token == nil {
			return errors.New("native execution configuration cannot use null")
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			keys := map[string]struct{}{}
			for decoder.More() {
				if len(keys) >= 10000 {
					return errors.New("native execution JSON member budget exceeded")
				}
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok {
					return errors.New("invalid native execution JSON key")
				}
				if _, duplicate := keys[name]; duplicate {
					return errors.New("duplicate native execution JSON key")
				}
				keys[name] = struct{}{}
				if err := visit(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for count := 0; decoder.More(); count++ {
				if count >= 10000 {
					return errors.New("native execution JSON array budget exceeded")
				}
				if err := visit(depth + 1); err != nil {
					return err
				}
			}
		default:
			return errors.New("invalid native execution JSON delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := visit(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("native execution JSON has trailing bytes")
	}
	return nil
}

// The complete original descriptor is hashed; typed decoding must not drop
// explicit optional fields or otherwise rebuild a different projection.
func nativeBillingDescriptorDigest(raw []byte) (string, error) {
	canonical, err := mfjson.Canonicalize(raw)
	if err != nil {
		return "", err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &object); err != nil || object == nil {
		return "", errors.New("invalid native descriptor object")
	}
	delete(object, "descriptor_digest")
	projection, err := json.Marshal(object)
	if err != nil {
		return "", err
	}
	return mfjson.Digest(projection)
}
