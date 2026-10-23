package main

import (
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	pluginName = "ai-memory"

	memoryGateContextKey            = "memoryGate"
	memoryTenantContextKey          = "memoryTenant"
	memoryConsumerContextKey        = "memoryConsumer"
	memorySessionContextKey         = "memorySession"
	memoryRequestIDContextKey       = "memoryRequestID"
	memoryRouteContextKey           = "memoryRoute"
	memoryModelContextKey           = "memoryModel"
	memoryRequestPathContextKey     = "memoryRequestPath"
	memoryModeContextKey            = "memoryMode"
	memoryPolicyVersionContextKey   = "memoryPolicyVersion"
	memoryStartedAtContextKey       = "memoryStartedAt"
	memoryStreamContextKey          = "memoryStream"
	memoryNoStoreContextKey         = "memoryNoStore"
	memoryRequestDigestContextKey   = "memoryRequestDigest"
	memoryUserContentContextKey     = "memoryUserContent"
	memoryOriginalBodyContextKey    = "memoryOriginalBody"
	memoryCurrentMessagesContextKey = "memoryCurrentMessages"
	memoryRecentMessagesContextKey  = "memoryRecentMessages"

	maxRequestBodyBytes  = 100 * 1024 * 1024
	maxResponseBodyBytes = 100 * 1024 * 1024
)

func main() {}

func init() {
	wrapper.SetCtx(
		pluginName,
		wrapper.ParseOverrideConfigBy(parseConfig, parseOverrideConfig),
		wrapper.ProcessRequestHeadersBy(onHttpRequestHeaders),
		wrapper.ProcessRequestBodyBy(onHttpRequestBody),
		wrapper.ProcessResponseHeadersBy(onHttpResponseHeaders),
		wrapper.ProcessStreamingResponseBodyBy(onHttpResponseBody),
	)
}

func parseConfig(json gjson.Result, c *config.PluginConfig, log log.Log) error {
	c.FromJson(json, log)
	if err := c.Validate(); err != nil {
		return err
	}
	return c.Complete(log)
}

func parseOverrideConfig(json gjson.Result, global config.PluginConfig, c *config.PluginConfig, log log.Log) error {
	c.FromJsonWithGlobal(json, global, log)
	if err := c.Validate(); err != nil {
		return err
	}
	return c.Complete(log)
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) types.Action {
	ctx.DisableReroute()
	removeCallerMemoryCacheDigest()
	if c.Route.MemoryMode == config.MemoryModeOff {
		markMemoryGate(ctx, "memory-off")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	contentType, _ := proxywasm.GetHttpRequestHeader("content-type")
	if contentType == "" || !isJSONContentType(contentType) {
		markMemoryGate(ctx, "unsupported-content-type")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	path := requestPath(ctx)
	if !pathMatchesSuffixes(path, c.EnablePathSuffixes) {
		markMemoryGate(ctx, "unsupported-path")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	tenant, _ := proxywasm.GetHttpRequestHeader(c.TenantHeader)
	tenant = strings.TrimSpace(tenant)
	if tenant == "" {
		markMemoryGate(ctx, "missing-tenant")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	consumer, _ := proxywasm.GetHttpRequestHeader(c.ConsumerHeader)
	consumer = strings.TrimSpace(consumer)
	if consumer == "" {
		markMemoryGate(ctx, "missing-consumer")
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}
	session, _ := proxywasm.GetHttpRequestHeader(c.SessionHeader)
	requestID := currentRequestID(c.RequestIDHeader)
	ctx.SetContext(memoryTenantContextKey, tenant)
	ctx.SetContext(memoryConsumerContextKey, consumer)
	ctx.SetContext(memorySessionContextKey, strings.TrimSpace(session))
	ctx.SetContext(memoryRequestIDContextKey, requestID)
	ctx.SetContext(memoryRouteContextKey, requestRoute())
	ctx.SetContext(memoryRequestPathContextKey, path)
	ctx.SetContext(memoryModeContextKey, c.Route.MemoryMode)
	ctx.SetContext(memoryPolicyVersionContextKey, c.Route.PolicyVersion)
	ctx.SetContext(memoryStartedAtContextKey, nowMillis())
	ctx.SetContext(memoryNoStoreContextKey, requestHasNoStore(c.Route.NoStoreHeader))
	ctx.SetRequestBodyBufferLimit(maxRequestBodyBytes)
	_ = proxywasm.RemoveHttpRequestHeader("Accept-Encoding")
	_ = proxywasm.RemoveHttpRequestHeader("Content-Length")
	return types.HeaderStopIteration
}

func onHttpRequestBody(ctx wrapper.HttpContext, c config.PluginConfig, body []byte, log log.Log) types.Action {
	if memoryGateReason(ctx) != "" {
		return types.ActionContinue
	}
	path := ctx.GetStringContext(memoryRequestPathContextKey, "")
	kind, adapter, ok := memoryProtocolAdapterForPath(path)
	if !ok {
		markMemoryGate(ctx, "unsupported-protocol")
		return types.ActionContinue
	}
	method, _ := proxywasm.GetHttpRequestHeader(":method")
	bodyDigest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{
		Method:               method,
		Path:                 path,
		Body:                 body,
		ContextPolicyVersion: c.Route.PolicyVersion,
	})
	if err != nil {
		markMemoryGate(ctx, "request-digest-failed")
		return types.ActionContinue
	}
	digest, err := sessionctx.BuildScopedRequestDigest(sessionctx.ScopedRequestDigestInput{
		Tenant:      ctx.GetStringContext(memoryTenantContextKey, ""),
		Consumer:    ctx.GetStringContext(memoryConsumerContextKey, ""),
		SessionID:   ctx.GetStringContext(memorySessionContextKey, ""),
		Route:       ctx.GetStringContext(memoryRouteContextKey, ""),
		RequestPath: path,
		Model:       bodyDigest.Input.Model,
		BodyDigest:  bodyDigest.Digest,
	})
	if err != nil {
		markMemoryGate(ctx, "request-digest-failed")
		return types.ActionContinue
	}
	currentQuestion, err := memoryCurrentQuestion(body, path, c.Route.QuestionFrom)
	if err != nil {
		markMemoryGate(ctx, "request-parse-failed")
		return types.ActionContinue
	}
	ctx.SetContext(memoryModelContextKey, bodyDigest.Input.Model)
	ctx.SetContext(memoryStreamContextKey, gjson.GetBytes(body, "stream").Bool())
	ctx.SetContext(memoryRequestDigestContextKey, digest)
	ctx.SetContext(memoryUserContentContextKey, currentQuestion)
	ctx.SetContext(memoryOriginalBodyContextKey, append([]byte(nil), body...))
	if kind == protocol.ProtocolChatCompletions {
		request, err := sessionctx.ParseOpenAIChatRequest(body)
		if err != nil {
			markMemoryGate(ctx, "request-parse-failed")
			return types.ActionContinue
		}
		ctx.SetContext(memoryCurrentMessagesContextKey, append([]sessionctx.OpenAIMessage(nil), request.Messages...))
	} else {
		ctx.SetContext(memoryCurrentMessagesContextKey, nil)
	}
	ctx.SetContext(memoryRecentMessagesContextKey, nil)
	if memoryRecentCacheConfigured(c) {
		if err := memoryLoadRecentMemory(ctx, c, log); err != nil {
			log.Warnf("[ai-memory] recent memory lookup failed open: %v", err)
			if memoryConsoleAssembleConfigured(c) {
				if err := dispatchMemoryAssemble(ctx, c, log); err != nil {
					log.Warnf("[ai-memory] Console assemble dispatch failed open: %v", err)
					return types.ActionContinue
				}
				return types.ActionPause
			}
			return types.ActionContinue
		}
		return types.ActionPause
	}
	if memoryConsoleAssembleConfigured(c) {
		if err := dispatchMemoryAssemble(ctx, c, log); err != nil {
			log.Warnf("[ai-memory] Console assemble dispatch failed open: %v", err)
			return types.ActionContinue
		}
		return types.ActionPause
	}
	return types.ActionContinue
}

func memoryCurrentQuestion(body []byte, requestPath string, questionFrom string) (string, error) {
	if questionFrom = strings.TrimSpace(questionFrom); questionFrom != "" {
		result := gjson.GetBytes(body, questionFrom)
		if result.Exists() {
			if question := strings.TrimSpace(memoryTextFromGJSON(result)); question != "" {
				return question, nil
			}
		}
	}
	return memoryProtocolCurrentUserPromptForPath(requestPath, body)
}

func memoryRecentCacheConfigured(c config.PluginConfig) bool {
	return strings.TrimSpace(c.RecentCache.ServiceName) != ""
}

func memoryConsoleInternalConfigured(c config.PluginConfig) bool {
	return strings.TrimSpace(c.ConsoleInternal.ServiceName) != ""
}

func memoryConsoleAssembleConfigured(c config.PluginConfig) bool {
	return shouldUseMemoryAssemble(c) && memoryConsoleInternalConfigured(c)
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) types.Action {
	// Assembly is complete before upstream response headers arrive. Keep only
	// the small event facts while the model's streaming response stays open.
	ctx.SetContext(memoryUserContentContextKey, strings.Clone(ctx.GetStringContext(memoryUserContentContextKey, "")))
	ctx.SetContext(memoryModelContextKey, strings.Clone(ctx.GetStringContext(memoryModelContextKey, "")))
	ctx.SetContext(memoryOriginalBodyContextKey, nil)
	ctx.SetContext(memoryCurrentMessagesContextKey, nil)
	if memoryResponseIsCacheReplay() {
		markMemoryGate(ctx, "cache-replay")
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	if memoryGateReason(ctx) != "" {
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	memoryCaptureResponseStatus(ctx, log)
	if ctx.GetBoolContext(memoryStreamContextKey, false) {
		return types.ActionContinue
	}
	ctx.SetResponseBodyBufferLimit(maxResponseBodyBytes)
	return types.ActionContinue
}

func onHttpResponseBody(ctx wrapper.HttpContext, c config.PluginConfig, chunk []byte, isLastChunk bool, log log.Log) []byte {
	if ctx.GetBoolContext(memoryStreamContextKey, false) {
		memoryCaptureStreamingResponseChunk(ctx, c, chunk, isLastChunk, log)
		if isLastChunk {
			memoryEmitEventAfterResponseCompletion(ctx, c, log)
		}
		return chunk
	}
	memoryCaptureNonStreamingResponseChunk(ctx, c, chunk, isLastChunk, log)
	if isLastChunk {
		memoryEmitEventAfterResponseCompletion(ctx, c, log)
	}
	return chunk
}

func markMemoryGate(ctx wrapper.HttpContext, reason string) {
	ctx.SetContext(memoryGateContextKey, reason)
}

func memoryGateReason(ctx wrapper.HttpContext) string {
	return ctx.GetStringContext(memoryGateContextKey, "")
}

func memoryRawContentAllowed(ctx wrapper.HttpContext, c config.PluginConfig) bool {
	return c.Route.CaptureResponse && !ctx.GetBoolContext(memoryNoStoreContextKey, false)
}

func memoryEventUserContent(ctx wrapper.HttpContext, c config.PluginConfig) string {
	if !memoryRawContentAllowed(ctx, c) {
		return ""
	}
	return ctx.GetStringContext(memoryUserContentContextKey, "")
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

func currentRequestID(header string) string {
	if requestID, _ := proxywasm.GetHttpRequestHeader(header); strings.TrimSpace(requestID) != "" {
		return strings.TrimSpace(requestID)
	}
	property, _ := proxywasm.GetProperty([]string{"x_request_id"})
	return string(property)
}

func requestHasNoStore(header string) bool {
	if value, _ := proxywasm.GetHttpRequestHeader(header); isTruthyHeaderValue(value) {
		return true
	}
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

func nowMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
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
