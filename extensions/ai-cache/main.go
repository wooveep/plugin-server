package main

import (
	"strconv"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/memorycache"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	PLUGIN_NAME                = "ai-cache"
	CACHE_GATE_CONTEXT_KEY     = "cacheGate"
	STREAM_CONTEXT_KEY         = "stream"
	SKIP_CACHE_HEADER          = "x-higress-skip-ai-cache"
	CACHE_TENANT_CONTEXT_KEY   = "cacheTenant"
	CACHE_CONSUMER_CONTEXT_KEY = "cacheConsumer"
	CACHE_SESSION_CONTEXT_KEY  = "cacheSession"
	CACHE_PATH_CONTEXT_KEY     = "cacheRequestPath"
	CACHE_ROUTE_CONTEXT_KEY    = "cacheRoute"
	CACHE_MODEL_CONTEXT_KEY    = "cacheModel"
	CACHE_DIGEST_CONTEXT_KEY   = "cacheRequestDigest"
	CACHE_MATERIALIZED_KEY     = "cacheMaterializedKey"
	CACHE_USER_CONTENT_KEY     = "cacheUserContent"
	CACHE_REQUEST_ID_KEY       = "cacheRequestID"
	CACHE_EVENT_STARTED_AT_KEY = "cacheEventStartedAt"
	CACHE_RESPONSE_STATUS_KEY  = "cacheResponseStatus"
	CACHE_RESPONSE_NOSTORE_KEY = "cacheResponseNoStore"
	CACHE_SENSITIVE_KEY        = "cacheSensitive"
	CACHE_STREAM_CAPTURE_KEY   = "cacheStreamCapture"
	CACHE_RESPONSE_CAPTURE_KEY = "cacheResponseCapture"
	CACHE_MEMORY_DIGEST_KEY    = "cacheMemoryDigest"
	CACHE_POLICY_VERSION_KEY   = "cacheEffectivePolicyVersion"
	CACHE_NATIVE_IDENTITY_KEY  = "cacheNativeIdentity"
	CACHE_SEMANTIC_CONTEXT_KEY = "cacheSemanticContextDigest"

	DEFAULT_MAX_BODY_BYTES uint32 = 100 * 1024 * 1024
)

func main() {}

func init() {
	// CreateClient()
	wrapper.SetCtx(
		PLUGIN_NAME,
		wrapper.ParseOverrideConfigBy(parseConfig, parseOverrideConfig),
		wrapper.ProcessRequestHeadersBy(onHttpRequestHeaders),
		wrapper.ProcessRequestBodyBy(onHttpRequestBody),
		wrapper.ProcessResponseHeadersBy(onHttpResponseHeaders),
		wrapper.ProcessStreamingResponseBodyBy(onHttpResponseBody),
		wrapper.ProcessStreamDoneBy(onNativeCacheStreamDone),
	)
}

func parseConfig(json gjson.Result, c *config.PluginConfig, log log.Log) error {
	c.FromJson(json, log)
	if err := configureNativeCacheDelivery(json, c); err != nil {
		return err
	}
	if err := c.Validate(); err != nil {
		return err
	}
	// Note that initializing the client during the parseConfig phase may cause errors, such as Redis not being usable in Docker Compose.
	if err := c.Complete(log); err != nil {
		log.Errorf("complete config failed: %v", err)
		return err
	}
	return nil
}

func parseOverrideConfig(json gjson.Result, global config.PluginConfig, c *config.PluginConfig, log log.Log) error {
	c.FromJsonWithGlobal(json, global, log)
	if err := configureNativeCacheDelivery(json, c); err != nil {
		return err
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if err := c.Complete(log); err != nil {
		log.Errorf("complete override config failed: %v", err)
		return err
	}
	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) (action types.Action) {
	ctx.DisableReroute()
	if c.NativeExecution != nil {
		if !prepareNativeCacheHeaders(ctx, c) {
			return types.ActionPause
		}
		// Even a user-requested cache bypass must retain the SDK-selected body
		// basis stage. Later mapper/proxy must not receive client basis headers.
		defer func() {
			ctx.SetRequestBodyBufferLimit(DEFAULT_MAX_BODY_BYTES)
			if !wrapper.RequireRequestBody(ctx) {
				rejectNativeCache(ctx, nil, 500, "body_context_unavailable")
				action = types.ActionPause
				return
			}
			action = types.HeaderStopIteration
		}()
	}
	_ = proxywasm.RemoveHttpRequestHeader(memorycache.DeprecatedDigestHeader)
	skipCache, _ := proxywasm.GetHttpRequestHeader(SKIP_CACHE_HEADER)
	if isTruthyHeaderValue(skipCache) {
		ctx.SetContext(SKIP_CACHE_HEADER, struct{}{})
		markCacheGate(ctx, "skip-header")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	contentType, _ := proxywasm.GetHttpRequestHeader("content-type")
	// The request does not have a body.
	if contentType == "" {
		return types.ActionContinue
	}
	if !isJSONContentType(contentType) {
		log.Warnf("content is not json, can't process: %s", contentType)
		markCacheGate(ctx, "unsupported-content-type")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	if c.HasThinConfig() {
		if requestHasNoStore() {
			markCacheGate(ctx, "no-store")
			ctx.DontReadRequestBody()
			return types.ActionContinue
		}
		if c.RoutePolicy.EnableBypass {
			markCacheGate(ctx, "route-bypass")
			ctx.DontReadRequestBody()
			return types.ActionContinue
		}
		if c.ProtocolBypassReason != "" {
			markCacheGate(ctx, "unsupported-published-protocol")
			log.Debugf("[onHttpRequestHeaders] text cache bypass: %s", c.ProtocolBypassReason)
			ctx.DontReadRequestBody()
			return types.ActionContinue
		}
		path := requestPath(ctx)
		if !pathMatchesSuffixes(path, c.RoutePolicy.EnabledPathSuffixes) {
			markCacheGate(ctx, "unsupported-path")
			ctx.DontReadRequestBody()
			return types.ActionContinue
		}
		var tenant, consumer string
		if c.NativeIdentity != nil {
			headers, err := proxywasm.GetHttpRequestHeaders()
			if err != nil {
				markCacheGate(ctx, "native-caller-unavailable")
				ctx.DontReadRequestBody()
				return types.ActionContinue
			}
			caller, err := balanceidentity.Read(headers)
			if err != nil || caller.TenantID == "" || caller.PrincipalID == "" {
				markCacheGate(ctx, "native-caller-unavailable")
				ctx.DontReadRequestBody()
				return types.ActionContinue
			}
			// Native key authentication cleared client fields and installed this
			// credential's five facts. Retain one snapshot for lookup and response.
			ctx.SetContext(CACHE_NATIVE_IDENTITY_KEY, &nativeCacheProcessingIdentity{
				NativeIdentityConfig: *c.NativeIdentity, Caller: caller,
			})
			tenant, consumer = caller.TenantID, caller.PrincipalID
		} else {
			tenant, _ = proxywasm.GetHttpRequestHeader(c.TenantHeader)
			if strings.TrimSpace(tenant) == "" {
				markCacheGate(ctx, "missing-tenant")
				ctx.DontReadRequestBody()
				return types.ActionContinue
			}
			consumer, _ = proxywasm.GetHttpRequestHeader(c.ConsumerHeader)
			if c.CacheScope == config.CACHE_SCOPE_CONSUMER && strings.TrimSpace(consumer) == "" {
				markCacheGate(ctx, "missing-consumer")
				ctx.DontReadRequestBody()
				return types.ActionContinue
			}
		}
		ctx.SetContext(CACHE_TENANT_CONTEXT_KEY, tenant)
		ctx.SetContext(CACHE_CONSUMER_CONTEXT_KEY, consumer)
		session, _ := proxywasm.GetHttpRequestHeader(c.SessionHeader)
		ctx.SetContext(CACHE_SESSION_CONTEXT_KEY, session)
		ctx.SetContext(CACHE_PATH_CONTEXT_KEY, path)
		sensitive, _ := proxywasm.GetHttpRequestHeader("x-mse-cache-sensitive")
		ctx.SetContext(CACHE_SENSITIVE_KEY, isTruthyHeaderValue(sensitive))
	}
	ctx.SetRequestBodyBufferLimit(DEFAULT_MAX_BODY_BYTES)
	_ = proxywasm.RemoveHttpRequestHeader("Accept-Encoding")
	// The request has a body and requires delaying the header transmission until a cache miss occurs,
	// at which point the header should be sent.
	return types.HeaderStopIteration
}

func onHttpRequestBody(ctx wrapper.HttpContext, c config.PluginConfig, body []byte, log log.Log) types.Action {
	if c.NativeExecution != nil && !prepareNativeCacheBody(ctx, c, body) {
		return types.ActionPause
	}
	consumeThinMemoryDigest(ctx, c)
	if c.NativeIdentity != nil && cacheNativeIdentity(ctx) == nil {
		markCacheGate(ctx, "native-caller-unavailable")
	}
	if cacheGateReason(ctx) != "" {
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	bodyJson := gjson.ParseBytes(body)
	// TODO: It may be necessary to support stream mode determination for different LLM providers.
	stream := false
	if bodyJson.Get("stream").Bool() {
		stream = true
		ctx.SetContext(STREAM_CONTEXT_KEY, struct{}{})
	}

	if !c.HasThinConfig() {
		log.Warn("[onHttpRequestBody] thin cache config is disabled, fail open")
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	return onThinHttpRequestBody(ctx, c, body, log, stream)
}

func onThinHttpRequestBody(ctx wrapper.HttpContext, c config.PluginConfig, body []byte, log log.Log, stream bool) types.Action {
	if c.RoutePolicy.Memory.Enabled && c.RoutePolicy.Memory.CacheMode == config.MEMORY_CACHE_MODE_BYPASS {
		markCacheGate(ctx, "memory-bypass")
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	path := ctx.GetStringContext(CACHE_PATH_CONTEXT_KEY, "")
	if path == "" {
		path = requestPath(ctx)
		ctx.SetContext(CACHE_PATH_CONTEXT_KEY, path)
	}
	kind := c.ProtocolForPath(path)
	adapter, ok := thinProtocolAdapterForKind(kind)
	if !ok {
		markCacheGate(ctx, "unsupported-path")
		log.Warnf("[onThinHttpRequestBody] unsupported request path for protocol adapter: %s", path)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	userPrompt, err := thinCurrentUserPrompt(kind, body)
	if err != nil {
		markCacheGate(ctx, "unsupported-content")
		log.Debugf("[onThinHttpRequestBody] text cache bypass: %v", err)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	method, _ := proxywasm.GetHttpRequestHeader(":method")
	digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{
		Method: method,
		Path:   path,
		Body:   body,
	})
	if err != nil {
		log.Warnf("[onThinHttpRequestBody] build protocol request digest failed, fail open: %v", err)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	cachePolicyVersion, ok := thinEffectiveCachePolicyVersion(ctx, c, log)
	if !ok {
		markCacheGate(ctx, "memory-missing-digest")
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	route := requestRoute()
	if identity := cacheNativeIdentity(ctx); identity != nil {
		route = identity.RouteID
	}
	material, err := BuildScopedCacheKeyMaterial(ScopedCacheKeyInput{
		KeyPrefix:          c.MaterializedLookup.Redis.KeyPrefix,
		Tenant:             ctx.GetStringContext(CACHE_TENANT_CONTEXT_KEY, ""),
		Consumer:           ctx.GetStringContext(CACHE_CONSUMER_CONTEXT_KEY, ""),
		CacheScope:         c.CacheScope,
		Route:              route,
		Model:              digest.Input.Model,
		Protocol:           kind,
		RequestDigest:      digest.Digest,
		CachePolicyVersion: cachePolicyVersion,
	})
	if err != nil {
		log.Warnf("[onThinHttpRequestBody] build materialized cache key failed, fail open: %v", err)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	ctx.SetContext(CACHE_ROUTE_CONTEXT_KEY, material.Route)
	ctx.SetContext(CACHE_MODEL_CONTEXT_KEY, material.Model)
	ctx.SetContext(CACHE_DIGEST_CONTEXT_KEY, material.RequestDigest)
	ctx.SetContext(CACHE_MATERIALIZED_KEY, material.RedisKey)
	ctx.SetContext(CACHE_POLICY_VERSION_KEY, material.CachePolicyVersion)
	ctx.SetContext(CACHE_EVENT_STARTED_AT_KEY, nowMillis())
	ctx.SetContext(CACHE_REQUEST_ID_KEY, currentRequestID())
	ctx.SetContext(CACHE_USER_CONTENT_KEY, userPrompt.Text)
	semanticContext, err := buildThinSemanticContextDigest(kind, body, userPrompt)
	if err != nil {
		// Exact lookup still uses the full original request. An unprovable
		// semantic context only disables semantic fallback for this request.
		log.Debugf("[onThinHttpRequestBody] semantic context unavailable, exact lookup only: %v", err)
	}
	ctx.SetContext(CACHE_SEMANTIC_CONTEXT_KEY, semanticContext)

	if !c.MaterializedLookup.Redis.Enabled || !c.RoutePolicy.EnableRedisLookup {
		log.Debug("[onThinHttpRequestBody] materialized Redis lookup is disabled, fail open")
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	if err := CheckMaterializedCacheForKey(material, ctx, c, log, stream); err != nil {
		log.Errorf("[onThinHttpRequestBody] materialized Redis lookup failed for key: %s, error: %v", material.RedisKey, err)
		return types.ActionContinue
	}

	return types.ActionPause
}

func thinEffectiveCachePolicyVersion(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) (string, bool) {
	if !c.RoutePolicy.Memory.Enabled || c.RoutePolicy.Memory.CacheMode != config.MEMORY_CACHE_MODE_POLICY_DIGEST {
		return c.CachePolicyVersion, true
	}
	digest := ctx.GetStringContext(CACHE_MEMORY_DIGEST_KEY, "")
	if digest == "" {
		log.Warnf("[thinEffectiveCachePolicyVersion] trusted memory digest property is missing, fail open")
		return "", false
	}
	version := c.CachePolicyVersion
	if index := strings.LastIndex(version, "-v"); index >= 0 {
		version = version[index+2:]
	}
	parsed, err := strconv.ParseInt(version, 10, 64)
	if err != nil || parsed <= 0 {
		log.Warn("memory-aware cache requires a positive numeric policy version or final -v<integer> suffix")
		return "", false
	}
	return strings.Join([]string{
		c.CachePolicyVersion,
		"memory",
		c.RoutePolicy.Memory.PolicyVersion,
		digest,
		"cache-v" + strconv.FormatInt(parsed, 10),
	}, "|"), true
}

func consumeThinMemoryDigest(ctx wrapper.HttpContext, c config.PluginConfig) {
	if !c.RoutePolicy.Memory.Enabled {
		return
	}
	digest, _ := proxywasm.GetProperty([]string{memorycache.TrustedDigestProperty})
	ctx.SetContext(CACHE_MEMORY_DIGEST_KEY, strings.TrimSpace(string(digest)))
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) types.Action {
	if cacheGateReason(ctx) != "" || ctx.GetContext(SKIP_CACHE_HEADER) != nil {
		ctx.SetUserAttribute("cache_status", "skip")
		ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	if c.HasThinConfig() && ctx.GetContext(CACHE_MATERIALIZED_KEY) != nil {
		captureThinResponseHeaders(ctx, log)
	}
	contentType, _ := proxywasm.GetHttpResponseHeader("content-type")
	if strings.Contains(contentType, "text/event-stream") {
		ctx.SetContext(STREAM_CONTEXT_KEY, struct{}{})
	} else {
		ctx.SetResponseBodyBufferLimit(DEFAULT_MAX_BODY_BYTES)
	}

	return types.ActionContinue
}

func markCacheGate(ctx wrapper.HttpContext, reason string) {
	ctx.SetContext(CACHE_GATE_CONTEXT_KEY, reason)
}

func cacheGateReason(ctx wrapper.HttpContext) string {
	return ctx.GetStringContext(CACHE_GATE_CONTEXT_KEY, "")
}

func requestPath(ctx wrapper.HttpContext) string {
	if path := ctx.Path(); path != "" {
		return path
	}
	path, _ := proxywasm.GetHttpRequestHeader(":path")
	return path
}

func requestRoute() string {
	routeName, err := proxywasm.GetProperty([]string{"route_name"})
	if err != nil || len(routeName) == 0 {
		return "-"
	}
	return string(routeName)
}

func isJSONContentType(contentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func pathMatchesSuffixes(path string, suffixes []string) bool {
	if len(suffixes) == 0 {
		return true
	}
	pathOnly := strings.Split(path, "?")[0]
	for _, suffix := range suffixes {
		if suffix == "" {
			continue
		}
		if strings.HasSuffix(pathOnly, suffix) {
			return true
		}
	}
	return false
}

func requestHasNoStore() bool {
	cacheControl, _ := proxywasm.GetHttpRequestHeader("cache-control")
	for _, directive := range strings.Split(cacheControl, ",") {
		if strings.EqualFold(strings.TrimSpace(directive), "no-store") {
			return true
		}
	}
	return false
}

func isTruthyHeaderValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "on", "true", "yes":
		return true
	default:
		return false
	}
}

func onHttpResponseBody(ctx wrapper.HttpContext, c config.PluginConfig, chunk []byte, isLastChunk bool, log log.Log) []byte {
	log.Debugf("[onHttpResponseBody] is last chunk: %v", isLastChunk)
	log.Debugf("[onHttpResponseBody] chunk: %s", string(chunk))

	if c.HasThinConfig() && ctx.GetContext(CACHE_MATERIALIZED_KEY) != nil {
		handleThinResponseBody(ctx, c, chunk, isLastChunk, log)
	}
	return chunk
}
