// 这个文件中主要将OnHttpRequestHeaders、OnHttpRequestBody、OnHttpResponseHeaders、OnHttpResponseBody这四个函数实现
// 其中的缓存思路调用cache.go中的逻辑，然后cache.go中的逻辑会调用textEmbeddingProvider和vectorStoreProvider中的逻辑（实例）
package main

import (
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	PLUGIN_NAME                              = "ai-cache"
	CACHE_KEY_CONTEXT_KEY                    = "cacheKey"
	CACHE_KEY_MATERIAL_CONTEXT_KEY           = "cacheKeyMaterial"
	CACHE_KEY_EMBEDDING_KEY                  = "cacheKeyEmbedding"
	CACHE_CONTENT_CONTEXT_KEY                = "cacheContent"
	CACHE_RESPONSE_BODY_CONTEXT_KEY          = "cacheResponseBody"
	CACHE_STREAM_FACTS_CONTEXT_KEY           = "cacheStreamFacts"
	CACHE_REQUEST_MODE_CONTEXT_KEY           = "cacheRequestMode"
	CACHE_BYPASS_REASON_CONTEXT_KEY          = "cacheBypassReason"
	CACHE_DISABLE_LEGACY_CONTEXT_KEY         = "cacheDisableLegacyFallback"
	CACHE_REQUEST_COLLAPSE_LEASE_CONTEXT_KEY = "cacheRequestCollapseLease"
	CACHE_WRITE_PENDING_CONTEXT_KEY          = "cacheWritePending"
	PARTIAL_MESSAGE_CONTEXT_KEY              = "partialMessage"
	TOOL_CALLS_CONTEXT_KEY                   = "toolCalls"
	STREAM_CONTEXT_KEY                       = "stream"
	SKIP_CACHE_HEADER                        = "x-higress-skip-ai-cache"
	CACHE_MODE_HEADER                        = "x-higress-ai-cache-mode"
	ERROR_PARTIAL_MESSAGE_KEY                = "errorPartialMessage"

	DEFAULT_MAX_BODY_BYTES uint32 = 100 * 1024 * 1024
)

func main() {}

func init() {
	// CreateClient()
	wrapper.SetCtx(
		PLUGIN_NAME,
		wrapper.ParseConfigBy(parseConfig),
		wrapper.ProcessRequestHeadersBy(onHttpRequestHeaders),
		wrapper.ProcessRequestBodyBy(onHttpRequestBody),
		wrapper.ProcessResponseHeadersBy(onHttpResponseHeaders),
		wrapper.ProcessStreamingResponseBodyBy(onHttpResponseBody),
		wrapper.ProcessStreamDoneBy(onHttpStreamDone),
	)
}

func parseConfig(json gjson.Result, c *config.PluginConfig, log log.Log) error {
	// config.EmbeddingProviderConfig.FromJson(json.Get("embeddingProvider"))
	// config.VectorDatabaseProviderConfig.FromJson(json.Get("vectorBaseProvider"))
	// config.RedisConfig.FromJson(json.Get("redis"))
	c.FromJson(json, log)
	if err := c.Validate(); err != nil {
		return err
	}
	// Note that initializing the client during the parseConfig phase may cause errors, such as Redis not being usable in Docker Compose.
	if err := c.Complete(log); err != nil {
		log.Errorf("complete config failed: %v", err)
		return err
	}
	if c.L1Cache.Enabled {
		c.SetL1CacheStore(newStructuredL1Cache(c.L1Cache.MaxEntries, c.L1Cache.TTLSeconds))
	}
	if c.RequestCollapse.Enabled {
		c.SetRequestCollapseStore(newRequestCollapseStore())
	}
	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) types.Action {
	ctx.DisableReroute()
	skipCache, _ := proxywasm.GetHttpRequestHeader(SKIP_CACHE_HEADER)
	requestModeHeader, _ := proxywasm.GetHttpRequestHeader(CACHE_MODE_HEADER)
	requestMode := normalizeRequestCacheMode(requestModeHeader)
	if requestMode != "" {
		ctx.SetContext(CACHE_REQUEST_MODE_CONTEXT_KEY, requestMode)
	}
	if skipCache == "on" || requestMode == config.CACHE_MODE_SKIP {
		ctx.SetContext(SKIP_CACHE_HEADER, struct{}{})
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	contentType, _ := proxywasm.GetHttpRequestHeader("content-type")
	// The request does not have a body.
	if contentType == "" {
		return types.ActionContinue
	}
	if !strings.Contains(contentType, "application/json") {
		log.Warnf("content is not json, can't process: %s", contentType)
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	ctx.SetRequestBodyBufferLimit(DEFAULT_MAX_BODY_BYTES)
	_ = proxywasm.RemoveHttpRequestHeader("Accept-Encoding")
	// The request has a body and requires delaying the header transmission until a cache miss occurs,
	// at which point the header should be sent.
	return types.HeaderStopIteration
}

func onHttpRequestBody(ctx wrapper.HttpContext, c config.PluginConfig, body []byte, log log.Log) types.Action {
	bodyJson := gjson.ParseBytes(body)
	// TODO: It may be necessary to support stream mode determination for different LLM providers.
	stream := false
	if bodyJson.Get("stream").Bool() {
		stream = true
		ctx.SetContext(STREAM_CONTEXT_KEY, struct{}{})
	}

	cacheMode := effectiveCacheMode(ctx, c)
	if cacheMode == config.CACHE_MODE_OFF || cacheMode == config.CACHE_MODE_SKIP {
		ctx.SetUserAttribute("cache_status", "skip")
		ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	var key string
	if c.CacheKeyStrategy == config.CACHE_KEY_STRATEGY_LAST_QUESTION {
		log.Debugf("[onHttpRequestBody] cache key strategy is last question, cache key from: %s", c.CacheKeyFrom)
		key = bodyJson.Get(c.CacheKeyFrom).String()
	} else if c.CacheKeyStrategy == config.CACHE_KEY_STRATEGY_ALL_QUESTIONS {
		log.Debugf("[onHttpRequestBody] cache key strategy is all questions, cache key from: messages")
		messages := bodyJson.Get("messages").Array()
		var userMessages []string
		for _, msg := range messages {
			if msg.Get("role").String() == "user" {
				userMessages = append(userMessages, msg.Get("content").String())
			}
		}
		key = strings.Join(userMessages, "\n")
	} else if c.CacheKeyStrategy == config.CACHE_KEY_STRATEGY_DISABLED {
		log.Info("[onHttpRequestBody] cache key strategy is disabled")
		ctx.DontReadResponseBody()
		return types.ActionContinue
	} else {
		log.Warnf("[onHttpRequestBody] unknown cache key strategy: %s", c.CacheKeyStrategy)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	ctx.SetContext(CACHE_KEY_CONTEXT_KEY, key)
	log.Debugf("[onHttpRequestBody] parsed cache key, cache_id=%s", cacheLogID(key))
	if key == "" {
		log.Debug("[onHttpRequestBody] parse key from request body failed")
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	eligibility := evaluateRequestCacheEligibility(bodyJson, key)
	if !eligibility.Cacheable {
		log.Infof("[onHttpRequestBody] bypass cache: %s", eligibility.Reason)
		ctx.SetContext(CACHE_KEY_CONTEXT_KEY, nil)
		ctx.SetContext(CACHE_BYPASS_REASON_CONTEXT_KEY, eligibility.Reason)
		ctx.SetUserAttribute("cache_status", "skip")
		ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	if !eligibility.SemanticAllowed && cacheMode == config.CACHE_MODE_SEMANTIC {
		log.Infof("[onHttpRequestBody] semantic cache bypass: %s", eligibility.Reason)
		ctx.SetContext(CACHE_DISABLE_LEGACY_CONTEXT_KEY, true)
		cacheMode = config.CACHE_MODE_EXACT_ONLY
	}

	keyMaterial := BuildScopedCacheKeyMaterial(c, bodyJson, key)
	if !keyMaterial.Valid {
		log.Warnf("[onHttpRequestBody] bypass cache: %s", keyMaterial.BypassReason)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	ctx.SetContext(CACHE_KEY_MATERIAL_CONTEXT_KEY, keyMaterial)
	log.Debugf("[onHttpRequestBody] scoped cache key ready, cache_id=%s", cacheLogID(keyMaterial.EntryKey))

	if cacheMode == config.CACHE_MODE_SHADOW {
		ctx.SetUserAttribute("cache_status", "shadow")
		ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
		return types.ActionContinue
	}

	if err := CheckCacheForKey(key, ctx, c, log, stream, shouldUseSimilaritySearch(cacheMode, c)); err != nil {
		log.Errorf("[onHttpRequestBody] check cache failed, cache_id=%s, error: %v", cacheLogID(keyMaterial.EntryKey), err)
		return types.ActionContinue
	}

	return types.ActionPause
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) types.Action {
	skipCache := ctx.GetContext(SKIP_CACHE_HEADER)
	if skipCache != nil {
		ctx.SetUserAttribute("cache_status", "skip")
		ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	if ctx.GetContext(CACHE_KEY_CONTEXT_KEY) != nil {
		status, err := proxywasm.GetHttpResponseHeader(":status")
		if err != nil || !isSuccessfulHTTPStatus(status) {
			reason := "response status not cacheable"
			log.Infof("[onHttpResponseHeaders] bypass cache write: %s, status: %s", reason, status)
			ctx.SetContext(CACHE_KEY_CONTEXT_KEY, nil)
			ctx.SetContext(CACHE_BYPASS_REASON_CONTEXT_KEY, reason)
			ctx.SetUserAttribute("cache_status", "skip")
			ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
			ctx.DontReadResponseBody()
			releaseRequestCollapse(ctx, c, log)
			return types.ActionContinue
		}
		ctx.SetUserAttribute("cache_status", "miss")
		ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
	}
	contentType, _ := proxywasm.GetHttpResponseHeader("content-type")
	if strings.Contains(contentType, "text/event-stream") {
		ctx.SetContext(STREAM_CONTEXT_KEY, struct{}{})
	} else {
		ctx.SetResponseBodyBufferLimit(DEFAULT_MAX_BODY_BYTES)
	}

	return types.ActionContinue
}

func onHttpResponseBody(ctx wrapper.HttpContext, c config.PluginConfig, chunk []byte, isLastChunk bool, log log.Log) []byte {
	log.Debugf("[onHttpResponseBody] is last chunk: %v", isLastChunk)
	log.Debugf("[onHttpResponseBody] chunk: %s", string(chunk))

	if ctx.GetContext(TOOL_CALLS_CONTEXT_KEY) != nil || ctx.GetContext(ERROR_PARTIAL_MESSAGE_KEY) != nil {
		releaseRequestCollapse(ctx, c, log)
		return chunk
	}

	key := ctx.GetContext(CACHE_KEY_CONTEXT_KEY)
	if key == nil {
		log.Debug("[onHttpResponseBody] key is nil, skip cache")
		return chunk
	}

	stream := ctx.GetContext(STREAM_CONTEXT_KEY)
	var err error
	if !isLastChunk {
		if stream == nil {
			err = handleNonStreamChunk(ctx, c, chunk, log)
		} else {
			err = handleStreamChunk(ctx, c, unifySSEChunk(chunk), log)
		}
		if err != nil {
			log.Errorf("[onHttpResponseBody] handle non last chunk failed, error: %v", err)
			// Set an empty struct in the context to indicate an error in processing the partial message
			ctx.SetContext(ERROR_PARTIAL_MESSAGE_KEY, struct{}{})
		}
		return chunk
	}
	var value string
	if stream == nil {
		value, err = processNonStreamLastChunk(ctx, c, chunk, log)
	} else {
		value, err = processStreamLastChunk(ctx, c, unifySSEChunk(chunk), log)
	}

	if err != nil {
		log.Errorf("[onHttpResponseBody] process last chunk failed, error: %v", err)
		releaseRequestCollapse(ctx, c, log)
		return chunk
	}

	if ctx.GetContext(TOOL_CALLS_CONTEXT_KEY) != nil {
		log.Warnf("[onHttpResponseBody] bypass cache write: response contains tool calls")
		ctx.SetContext(CACHE_BYPASS_REASON_CONTEXT_KEY, "response contains tool calls")
		releaseRequestCollapse(ctx, c, log)
		return chunk
	}
	if shouldBypassCacheWrite(ctx, c) {
		releaseRequestCollapse(ctx, c, log)
		return chunk
	}
	if strings.TrimSpace(value) == "" {
		log.Warnf("[onHttpResponseBody] bypass cache write: missing final assistant content")
		ctx.SetContext(CACHE_BYPASS_REASON_CONTEXT_KEY, "missing final assistant content")
		releaseRequestCollapse(ctx, c, log)
		return chunk
	}

	cacheWritePending := cacheResponse(ctx, c, key.(string), value, log, func(success bool) {
		ctx.SetContext(CACHE_WRITE_PENDING_CONTEXT_KEY, nil)
		releaseRequestCollapseWithNotify(ctx, c, log, success)
	})
	if !cacheWritePending {
		releaseRequestCollapseWithNotify(ctx, c, log, false)
	}
	uploadEmbeddingAndAnswer(ctx, c, key.(string), value, log)
	return chunk
}

func onHttpStreamDone(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) {
	if ctx.GetBoolContext(CACHE_WRITE_PENDING_CONTEXT_KEY, false) {
		ctx.SetContext(CACHE_WRITE_PENDING_CONTEXT_KEY, nil)
		releaseRequestCollapse(ctx, c, log)
		return
	}
	releaseRequestCollapse(ctx, c, log)
}

func normalizeRequestCacheMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case config.CACHE_MODE_SKIP:
		return config.CACHE_MODE_SKIP
	case config.CACHE_MODE_NO_STORE:
		return config.CACHE_MODE_NO_STORE
	case config.CACHE_MODE_EXACT_ONLY:
		return config.CACHE_MODE_EXACT_ONLY
	default:
		return ""
	}
}

func requestCacheMode(ctx wrapper.HttpContext) string {
	if mode, ok := ctx.GetContext(CACHE_REQUEST_MODE_CONTEXT_KEY).(string); ok {
		return mode
	}
	return ""
}

func effectiveCacheMode(ctx wrapper.HttpContext, c config.PluginConfig) string {
	if c.CacheMode == config.CACHE_MODE_OFF {
		return config.CACHE_MODE_OFF
	}
	switch requestCacheMode(ctx) {
	case config.CACHE_MODE_SKIP:
		return config.CACHE_MODE_SKIP
	case config.CACHE_MODE_EXACT_ONLY:
		return config.CACHE_MODE_EXACT_ONLY
	default:
		return c.CacheMode
	}
}

func shouldUseSimilaritySearch(cacheMode string, c config.PluginConfig) bool {
	return cacheMode == config.CACHE_MODE_SEMANTIC && c.EnableSemanticCache
}

func shouldBypassCacheWrite(ctx wrapper.HttpContext, c config.PluginConfig) bool {
	cacheMode := effectiveCacheMode(ctx, c)
	return cacheMode == config.CACHE_MODE_OFF ||
		cacheMode == config.CACHE_MODE_SKIP ||
		requestCacheMode(ctx) == config.CACHE_MODE_NO_STORE
}
