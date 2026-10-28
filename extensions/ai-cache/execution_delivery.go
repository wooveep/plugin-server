package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"mime"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const nativeCacheDeliveryContext = "ai-cache.execution-delivery.v1"

type nativeCacheDeliveryConfig struct {
	producerID, releaseID, targetID, routeID, salesAssetID, publicModel string
	capture                                                             bool
}

type nativeCacheDeliveryState struct {
	config                               *nativeCacheDeliveryConfig
	caller                               balanceidentity.Snapshot
	origin                               executionfacts.AdmittedCall
	basis                                bodybasis.Snapshot
	bodyReady, replied, rejected, closed bool
	snapshot                             executionfacts.Snapshot
}

// The SDK supplies the original complete envelope after hashing semantic config.
// The parser checks its raw descriptor digest, never reinterpreting a URL/model
// or caller-configured static field as a sale, producer grant or attempt.
func configureNativeCacheDelivery(value gjson.Result, cfg *config.PluginConfig) error {
	cfg.NativeExecution = nil
	settings, release := value.Get("native_execution"), value.Get("_modelfusion_execution")
	if !settings.Exists() && !release.Exists() {
		if value.Get("capture_body_basis").Exists() || value.Get("consume_body_basis").Exists() {
			return errors.New("cache body basis responsibility requires native execution ownership")
		}
		return nil
	}
	if !settings.IsObject() || !release.IsObject() || cfg.NativeIdentity == nil ||
		!cfg.PublishedProtocolConfigured || !isSupportedThinProtocol(cfg.PublishedProtocol) ||
		len(cfg.RoutePolicy.EnabledPathSuffixes) != 0 || cfg.RoutePolicy.EnableBypass || !cfg.RoutePolicy.EnableReplay {
		return errors.New("native cache delivery requires complete unfiltered published target configuration")
	}
	if _, err := mfjson.Canonicalize([]byte(value.Raw)); err != nil {
		return errors.New("native cache delivery configuration is ambiguous or oversized")
	}
	parsed, err := nativeCacheObject([]byte(settings.Raw), "producer_id call_identity_source")
	if err != nil || !nativeCacheText(parsed["producer_id"]) {
		return errors.New("native cache delivery requires the configured producer")
	}
	if nativeCacheString(parsed["call_identity_source"]) != executionfacts.CallIdentitySourceAdmission {
		return errors.New("native cache delivery requires the original admission call identity capability")
	}
	envelope, err := nativeCacheObject([]byte(release.Raw), "schema_version descriptor route_id sales_asset_id public_model credential_ref credential_revision")
	if err != nil || nativeCacheString(envelope["schema_version"]) != "modelfusion-execution.v1" ||
		!nativeCacheText(envelope["public_model"]) || !nativeCacheText(envelope["credential_ref"]) ||
		!nativeCachePositive(envelope["credential_revision"]) {
		return errors.New("native cache delivery requires complete frozen call ownership")
	}
	descriptor, err := nativeCacheObject(envelope["descriptor"], "release_id target_id binding_id provider_id provider_account_id region_endpoint_ref service_ref cluster_ref credential_pool_ref credential_pool_revision adapter_ref adapter_revision mapper_digest proxy_digest descriptor_digest upstream_model config_canonicalization transport_contract capabilities")
	if err != nil {
		return errors.New("native cache delivery requires the original full descriptor")
	}
	for name, raw := range descriptor {
		if name != "transport_contract" && name != "capabilities" && !nativeCacheText(raw) {
			return errors.New("native cache descriptor has invalid identity fields")
		}
	}
	if nativeCacheString(descriptor["config_canonicalization"]) != "mf-json.v1" ||
		!nativeCachePositive(descriptor["credential_pool_revision"]) || !nativeCachePositive(descriptor["adapter_revision"]) {
		return errors.New("native cache descriptor has invalid version or canonicalization")
	}
	for _, name := range []string{"mapper_digest", "proxy_digest", "descriptor_digest"} {
		if !nativeCacheDigest(nativeCacheString(descriptor[name])) {
			return errors.New("native cache descriptor has invalid digests")
		}
	}
	expected := nativeCacheString(descriptor["descriptor_digest"])
	delete(descriptor, "descriptor_digest")
	raw, err := json.Marshal(descriptor)
	if err != nil {
		return errors.New("native cache descriptor cannot be encoded")
	}
	actual, err := mfjson.Digest(raw)
	if err != nil || actual != expected {
		return errors.New("native cache descriptor digest differs")
	}
	transport, err := nativeCacheObject(descriptor["transport_contract"], "host sni path_adapter_ref")
	if err != nil || !nativeCacheText(transport["host"]) || !nativeCacheText(transport["path_adapter_ref"]) {
		return errors.New("native cache requires the released transport identity")
	}
	var sni string
	if json.Unmarshal(transport["sni"], &sni) != nil {
		return errors.New("native cache transport SNI is invalid")
	}
	var capabilities map[string]json.RawMessage
	if json.Unmarshal(descriptor["capabilities"], &capabilities) != nil || len(capabilities) == 0 || len(capabilities["text_generate"]) == 0 {
		return errors.New("native cache requires the released text capability")
	}
	for operation, raw := range capabilities {
		if operation != "text_generate" && operation != "image_generate" && operation != "video_generate" && operation != "task_query" {
			return errors.New("native cache descriptor contains an unregistered operation")
		}
		var capability map[string]json.RawMessage
		if json.Unmarshal(raw, &capability) != nil || capability == nil {
			return errors.New("native cache capability is invalid")
		}
		for name, value := range capability {
			switch name {
			case "send_mode", "response_classifier_ref", "query_capability_ref", "provider_idempotency_ref":
				if !nativeCacheText(value) {
					return errors.New("native cache capability reference is invalid")
				}
			case "max_attempts", "max_fallback_hops":
				n, ok := nativeCacheCount(value)
				if !ok || n.BitLen() > 32 {
					return errors.New("native cache capability count is invalid")
				}
			default:
				return errors.New("native cache capability has an unknown field")
			}
		}
		mode := nativeCacheString(capability["send_mode"])
		attempts, attemptOK := nativeCacheCount(capability["max_attempts"])
		if (mode != "single_send" && mode != "observable_retry") || !nativeCacheText(capability["response_classifier_ref"]) ||
			!attemptOK || attempts.Sign() == 0 || capability["max_fallback_hops"] == nil {
			return errors.New("native cache capability is incomplete")
		}
	}
	var captures, consumes bool
	capture, consume := value.Get("capture_body_basis"), value.Get("consume_body_basis")
	if (capture.Type != gjson.True && capture.Type != gjson.False) || (consume.Type != gjson.True && consume.Type != gjson.False) {
		return errors.New("native cache requires explicit body basis responsibility")
	}
	captures, consumes = capture.Bool(), consume.Bool()
	if captures == consumes {
		return errors.New("native cache must either capture or consume the original body basis")
	}
	out := &nativeCacheDeliveryConfig{
		producerID: nativeCacheString(parsed["producer_id"]), releaseID: nativeCacheString(descriptor["release_id"]),
		targetID: nativeCacheString(descriptor["target_id"]), routeID: nativeCacheString(envelope["route_id"]),
		salesAssetID: nativeCacheString(envelope["sales_asset_id"]), publicModel: nativeCacheString(envelope["public_model"]), capture: captures,
	}
	if !balanceidentity.ValidID(out.routeID) || !balanceidentity.ValidID(out.salesAssetID) ||
		out.releaseID != cfg.NativeIdentity.ReleaseID || out.targetID != cfg.NativeIdentity.TargetID || out.routeID != cfg.NativeIdentity.RouteID {
		return errors.New("native cache static policy differs from its frozen execution ownership")
	}
	cfg.NativeExecution = out
	return nil
}

func nativeCacheObject(raw []byte, fields string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil || len(object) != len(strings.Fields(fields)) {
		return nil, errors.New("native cache object has unexpected fields")
	}
	for _, name := range strings.Fields(fields) {
		if _, ok := object[name]; !ok {
			return nil, errors.New("native cache object is incomplete")
		}
	}
	return object, nil
}
func nativeCacheString(raw json.RawMessage) string {
	var out string
	_ = json.Unmarshal(raw, &out)
	return out
}
func nativeCacheText(raw json.RawMessage) bool {
	value := nativeCacheString(raw)
	return value != "" && len(value) <= 256 && strings.TrimSpace(value) == value && strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) < 0
}
func nativeCachePositive(raw json.RawMessage) bool {
	value := nativeCacheString(raw)
	n, ok := nativeCacheCount([]byte(value))
	return ok && n.Sign() > 0
}
func nativeCacheDigest(value string) bool {
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

func prepareNativeCacheHeaders(ctx wrapper.HttpContext, cfg config.PluginConfig) bool {
	parsed, ok := cfg.NativeExecution.(*nativeCacheDeliveryConfig)
	if !ok || parsed == nil || ctx.GetContext(nativeCacheDeliveryContext) != nil {
		return rejectNativeCache(ctx, nil, 409, "original_call_missing")
	}
	state := &nativeCacheDeliveryState{config: parsed}
	ctx.SetContext(nativeCacheDeliveryContext, state)
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return rejectNativeCache(ctx, state, 500, "request_headers_unavailable")
	}
	state.caller, err = balanceidentity.Read(headers)
	if err != nil || state.caller.TenantID == "" || state.caller.PrincipalID == "" {
		return rejectNativeCache(ctx, state, 403, "authenticated_caller_missing")
	}
	state.origin, err = executionfacts.ReadAdmittedCall(headers, parsed.releaseID, time.Now())
	if err != nil {
		return rejectNativeCache(ctx, state, 403, "current_execution_admission_required")
	}
	for _, header := range headers {
		if strings.EqualFold(header[0], "x-modelfusion-attempt-id") {
			return rejectNativeCache(ctx, state, 409, "attempt_already_present")
		}
	}
	if parsed.capture {
		clean := make([][2]string, 0, len(headers))
		for _, header := range headers {
			if !strings.EqualFold(header[0], bodybasis.DigestHeader) && !strings.EqualFold(header[0], bodybasis.PublicModelHeader) {
				clean = append(clean, header)
			}
		}
		if proxywasm.ReplaceHttpRequestHeaders(clean) != nil {
			return rejectNativeCache(ctx, state, 500, "original_basis_clear_failed")
		}
	} else {
		state.basis, err = bodybasis.Read(headers)
		if err != nil || state.basis.PublicModel() != parsed.publicModel {
			return rejectNativeCache(ctx, state, 400, "original_basis_missing")
		}
	}
	contentType, err := nativeCacheOneHeader(headers, "content-type")
	media, _, mediaErr := mime.ParseMediaType(contentType)
	if ctx.Method() != "POST" || !ctx.HasRequestBody() || err != nil || mediaErr != nil || media != "application/json" {
		return rejectNativeCache(ctx, state, 400, "json_text_request_required")
	}
	if encoding, count := nativeCacheHeader(headers, "content-encoding"); count > 1 || count == 1 && encoding != "identity" {
		return rejectNativeCache(ctx, state, 415, "content_encoding_unsupported")
	}
	return true
}

func prepareNativeCacheBody(ctx wrapper.HttpContext, cfg config.PluginConfig, body []byte) bool {
	state, _ := ctx.GetContext(nativeCacheDeliveryContext).(*nativeCacheDeliveryState)
	if state == nil || state.config != cfg.NativeExecution {
		return rejectNativeCache(ctx, state, 409, "original_call_missing")
	}
	if state.rejected || state.closed {
		return false
	}
	if state.bodyReady {
		return false
	}
	if !time.Now().Before(state.origin.ValidUntil) {
		return rejectNativeCache(ctx, state, 403, "execution_admission_expired")
	}
	if state.config.capture {
		basis, err := bodybasis.Capture(body)
		if err != nil || basis.PublicModel() != state.config.publicModel {
			return rejectNativeCache(ctx, state, 400, "original_body_invalid")
		}
		headers, err := proxywasm.GetHttpRequestHeaders()
		if err != nil {
			return rejectNativeCache(ctx, state, 500, "request_headers_unavailable")
		}
		headers = append(headers, basis.Headers()...)
		if proxywasm.ReplaceHttpRequestHeaders(headers) != nil {
			return rejectNativeCache(ctx, state, 500, "original_basis_install_failed")
		}
		state.basis = basis
	} else {
		if mfjson.ValidateRuntimeBody(body, int(DEFAULT_MAX_BODY_BYTES)) != nil {
			return rejectNativeCache(ctx, state, 400, "current_body_invalid")
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(body, &object) != nil || nativeCacheString(object["model"]) != state.config.publicModel {
			return rejectNativeCache(ctx, state, 400, "original_model_differs")
		}
	}
	state.bodyReady = true
	return true
}

// nativeCacheBridge runs only at the proven local replay boundary, before any
// procurement send. It never appends Redis or turns a cache event into billing.
func nativeCacheBridge(ctx wrapper.HttpContext, record MaterializedReplayRecord) (string, bool) {
	state, _ := ctx.GetContext(nativeCacheDeliveryContext).(*nativeCacheDeliveryState)
	if state == nil {
		return "", true
	} // Explicit standalone cache keeps its existing behavior.
	if state.replied || state.closed {
		return "", false
	}
	if state.rejected || !state.bodyReady || !time.Now().Before(state.origin.ValidUntil) {
		rejectNativeCache(ctx, state, 403, "execution_admission_expired")
		return "", false
	}
	state.replied = true
	if state.basis.PublicModel() != state.config.publicModel || record.Model != state.config.publicModel || record.Tenant != state.caller.TenantID || record.Route != state.config.routeID || record.CacheScope == config.CACHE_SCOPE_CONSUMER && record.Consumer != state.caller.PrincipalID {
		rejectNativeCache(ctx, state, 409, "cache_call_ownership_differs")
		return "", false
	}
	raw, err := json.Marshal(record)
	if err != nil {
		nativeCacheFactGap(ctx, "cache_record_digest_unavailable")
		return "", true
	}
	recordDigest := bodybasis.DigestBytes(raw)
	snapshot, err := executionfacts.Freeze(executionfacts.Envelope{
		ProducerID: state.config.producerID, Kind: "call_delivery", CallID: state.origin.CallID,
		ReleaseID: state.config.releaseID, RouteID: state.config.routeID, SalesAssetID: state.config.salesAssetID,
		TenantID: state.caller.TenantID, PrincipalID: state.caller.PrincipalID, AccountID: state.caller.AccountID,
		PrincipalKind: "user", BillingMode: "customer", OperationKind: "text_generate",
		OccurredAt: state.origin.OccurredAt, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano),
		EvidenceRefs: []string{"cache-replay:" + recordDigest}, DeliveryState: nativeCacheDeliveryResult(record.FinishReason),
		ServedFrom: "cache", PublicModel: state.config.publicModel, Usage: nativeCacheReplayUsage(record.Protocol, record.Usage),
	})
	if err != nil {
		nativeCacheFactGap(ctx, "cache_delivery_invalid")
		return "", true
	}
	state.snapshot = snapshot
	encoded, err := executionfacts.EncodeDeliveryBridge(snapshot)
	if err != nil {
		nativeCacheFactGap(ctx, "cache_delivery_bridge_unavailable")
		return "", true
	}
	return encoded, true
}

func nativeCacheDeliveryResult(reason string) string {
	switch reason {
	case "stop", "end_turn", "completed", "tool_calls", "function_call", "stop_sequence":
		return "succeeded"
	case "length", "content_filter", "max_tokens", "incomplete":
		return "partial"
	default:
		return "unknown"
	}
}

// Replay quantities are the retained cache meter, not this call's provider
// measurement. Platform response caching never invents provider cache splits.
func nativeCacheReplayUsage(kind protocol.ProtocolKind, raw json.RawMessage) *executionfacts.Usage {
	missing := func() *executionfacts.Usage {
		return executionfacts.MissingTokenUsage("cache_replay_usage_incomplete_or_invalid")
	}
	if _, err := mfjson.Canonicalize(raw); err != nil {
		return missing()
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return missing()
	}
	inputName, outputName := "input_tokens", "output_tokens"
	if kind == protocol.ProtocolChatCompletions {
		inputName, outputName = "prompt_tokens", "completion_tokens"
	}
	input, inOK := nativeCacheCount(object[inputName])
	output, outOK := nativeCacheCount(object[outputName])
	if !inOK || !outOK {
		return missing()
	}
	if total, exists := object["total_tokens"]; exists {
		n, ok := nativeCacheCount(total)
		if !ok || n.Cmp(new(big.Int).Add(new(big.Int).Set(input), output)) != 0 {
			return missing()
		}
	}
	quantities := map[string]string{"input": input.String(), "output": output.String()}
	hitRaw, hasHit := object["input_cache_hit_tokens"]
	missRaw, hasMiss := object["input_cache_miss_tokens"]
	if hasHit || hasMiss {
		hit, hOK := nativeCacheCount(hitRaw)
		miss, mOK := nativeCacheCount(missRaw)
		if !hasHit || !hasMiss || !hOK || !mOK || new(big.Int).Add(new(big.Int).Set(hit), miss).Cmp(input) != 0 {
			return missing()
		}
		quantities["cache_hit"], quantities["cache_miss"] = hit.String(), miss.String()
	}
	return &executionfacts.Usage{Status: "reported", Unit: "token", Source: "meter", Quantities: quantities}
}
func nativeCacheCount(raw []byte) (*big.Int, bool) {
	value := string(bytes.TrimSpace(raw))
	if value == "" || len(value) > 39 || len(value) > 1 && value[0] == '0' {
		return nil, false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return nil, false
		}
	}
	n, ok := new(big.Int).SetString(value, 10)
	return n, ok && n.BitLen() <= 128
}
func nativeCacheHeader(headers [][2]string, name string) (string, int) {
	value, count := "", 0
	for _, h := range headers {
		if strings.EqualFold(h[0], name) {
			value = h[1]
			count++
		}
	}
	return value, count
}
func nativeCacheOneHeader(headers [][2]string, name string) (string, error) {
	value, count := nativeCacheHeader(headers, name)
	if count != 1 {
		return "", errors.New("native cache header missing or duplicated")
	}
	return value, nil
}
func rejectNativeCache(ctx wrapper.HttpContext, state *nativeCacheDeliveryState, status uint32, reason string) bool {
	if state != nil {
		if state.rejected || state.closed {
			return false
		}
		state.rejected = true
	}
	ctx.SetUserAttribute("execution_cache_rejection", reason)
	_ = proxywasm.SendHttpResponseWithDetail(status, "ai-cache.execution_delivery", [][2]string{{"content-type", "application/json"}}, []byte(`{"error":"native cache request unavailable"}`), -1)
	return false
}
func nativeCacheFactGap(ctx wrapper.HttpContext, reason string) {
	ctx.SetUserAttribute("execution_fact_gap", true)
	ctx.SetUserAttribute("execution_fact_gap_reason", reason)
}
func onNativeCacheStreamDone(ctx wrapper.HttpContext, _ config.PluginConfig, _ logs.Log) {
	if state, ok := ctx.GetContext(nativeCacheDeliveryContext).(*nativeCacheDeliveryState); ok && state != nil {
		state.closed = true
	}
}
