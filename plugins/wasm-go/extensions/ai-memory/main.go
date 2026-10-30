package main

import (
	"mime"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
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
	memoryProtocolContextKey        = "memoryProtocol"
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
	memoryBodyBasisContextKey       = "memoryBodyBasis"
	memoryBodyBasisRejectedKey      = "memoryBodyBasisRejected"

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
	return c.ParseRoot(json, log)
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
	if c.CaptureBodyBasis {
		headers, err := proxywasm.GetHttpRequestHeaders()
		if err != nil {
			return rejectMemoryBodyBasis(ctx, 500, "header_read_failed")
		}
		filtered := make([][2]string, 0, len(headers))
		var contentType, encoding string
		var contentTypes, encodings int
		for _, header := range headers {
			switch {
			case strings.EqualFold(header[0], bodybasis.DigestHeader), strings.EqualFold(header[0], bodybasis.PublicModelHeader):
				// This release designates memory as the first capture boundary.
				// Remove every client occurrence before any memory-policy gate.
				continue
			case strings.EqualFold(header[0], "content-type"):
				contentType = header[1]
				contentTypes++
			case strings.EqualFold(header[0], "content-encoding"):
				encoding = header[1]
				encodings++
			}
			filtered = append(filtered, header)
		}
		if err := proxywasm.ReplaceHttpRequestHeaders(filtered); err != nil {
			return rejectMemoryBodyBasis(ctx, 500, "basis_clear_failed")
		}
		mediaType, _, err := mime.ParseMediaType(contentType)
		if contentTypes != 1 || err != nil || !isJSONContentType(mediaType) {
			return rejectMemoryBodyBasis(ctx, 415, "json_required")
		}
		if encodings > 1 || encoding != "" && !strings.EqualFold(encoding, "identity") {
			return rejectMemoryBodyBasis(ctx, 415, "content_encoding_unsupported")
		}
		if !ctx.HasRequestBody() {
			return rejectMemoryBodyBasis(ctx, 400, "body_required")
		}
		ctx.SetRequestBodyBufferLimit(maxRequestBodyBytes)
		if !wrapper.RequireRequestBody(ctx) {
			return rejectMemoryBodyBasis(ctx, 500, "body_context_unavailable")
		}
	}
	removeCallerMemoryCacheDigest()
	if c.Route.MemoryMode == config.MemoryModeOff {
		return skipMemoryRequest(ctx, c, "memory-off")
	}
	contentType, _ := proxywasm.GetHttpRequestHeader("content-type")
	if contentType == "" || !isJSONContentType(contentType) {
		return skipMemoryRequest(ctx, c, "unsupported-content-type")
	}
	path := requestPath(ctx)
	if c.PathScope != config.PathScopeBoundTarget && !pathMatchesSuffixes(path, c.EnablePathSuffixes) {
		return skipMemoryRequest(ctx, c, "unsupported-path")
	}
	kind := c.ProtocolForPath(path)
	if kind == protocol.ProtocolUnknown {
		return skipMemoryRequest(ctx, c, "unsupported-protocol-or-operation")
	}
	if !ctx.HasRequestBody() {
		return skipMemoryRequest(ctx, c, "missing-request-body")
	}
	var tenant, consumer string
	route := requestRoute()
	if c.NativeIdentity != nil {
		headers, err := proxywasm.GetHttpRequestHeaders()
		if err != nil {
			return skipMemoryRequest(ctx, c, "native-caller-unavailable")
		}
		caller, err := balanceidentity.Read(headers)
		if err != nil || caller.TenantID == "" || caller.PrincipalID == "" {
			return skipMemoryRequest(ctx, c, "native-caller-unavailable")
		}
		tenant, consumer = caller.TenantID, caller.PrincipalID
		ctx.SetContext(memoryNativeCallerContextKey, caller)
		// Quota precedes managed memory and replaces client admission headers.
		// Missing/invalid original admission leaves parentage unknown; a later
		// proxy must not supply a replacement parent after this capture point.
		if admitted, err := executionfacts.ReadAdmittedCall(headers, c.NativeIdentity.ReleaseID, time.Now().UTC()); err == nil {
			ctx.SetContext(memoryNativeCallContextKey, admitted)
		}
		ctx.SetContext(memoryRouteNameContextKey, route)
		route = c.NativeIdentity.RouteID
	} else {
		tenant, _ = proxywasm.GetHttpRequestHeader(c.TenantHeader)
		tenant = strings.TrimSpace(tenant)
		if tenant == "" {
			return skipMemoryRequest(ctx, c, "missing-tenant")
		}
		consumer, _ = proxywasm.GetHttpRequestHeader(c.ConsumerHeader)
		consumer = strings.TrimSpace(consumer)
		if consumer == "" {
			return skipMemoryRequest(ctx, c, "missing-consumer")
		}
	}
	session, _ := proxywasm.GetHttpRequestHeader(c.SessionHeader)
	requestID := currentRequestID(c.RequestIDHeader)
	ctx.SetContext(memoryTenantContextKey, tenant)
	ctx.SetContext(memoryConsumerContextKey, consumer)
	ctx.SetContext(memorySessionContextKey, strings.TrimSpace(session))
	ctx.SetContext(memoryRequestIDContextKey, requestID)
	ctx.SetContext(memoryRouteContextKey, route)
	ctx.SetContext(memoryRequestPathContextKey, path)
	ctx.SetContext(memoryProtocolContextKey, string(kind))
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
	if ctx.GetBoolContext(memoryBodyBasisRejectedKey, false) {
		return types.ActionPause
	}
	if c.CaptureBodyBasis && ctx.GetContext(memoryBodyBasisContextKey) == nil {
		basis, err := bodybasis.Capture(body)
		if err != nil {
			return rejectMemoryBodyBasis(ctx, 400, "invalid_model_body")
		}
		headers, err := proxywasm.GetHttpRequestHeaders()
		if err != nil {
			return rejectMemoryBodyBasis(ctx, 500, "header_read_failed")
		}
		if err := proxywasm.ReplaceHttpRequestHeaders(append(headers, basis.Headers()...)); err != nil {
			return rejectMemoryBodyBasis(ctx, 500, "basis_write_failed")
		}
		ctx.SetContext(memoryBodyBasisContextKey, basis)
	}
	if memoryGateReason(ctx) != "" {
		return types.ActionContinue
	}
	if c.NativeIdentity != nil && !c.CaptureBodyBasis {
		if err := mfjson.ValidateRuntimeBody(body, maxRequestBodyBytes); err != nil {
			markMemoryGate(ctx, "invalid-runtime-body")
			return types.ActionContinue
		}
		headers, err := proxywasm.GetHttpRequestHeaders()
		if err != nil {
			markMemoryGate(ctx, "missing-trusted-body-basis")
			return types.ActionContinue
		}
		basis, err := bodybasis.Read(headers)
		model := gjson.GetBytes(body, "model")
		if err != nil || model.Type != gjson.String || model.Str != basis.PublicModel() {
			markMemoryGate(ctx, "missing-trusted-body-basis")
			return types.ActionContinue
		}
	}
	path := ctx.GetStringContext(memoryRequestPathContextKey, "")
	kind := protocol.ProtocolKind(ctx.GetStringContext(memoryProtocolContextKey, ""))
	if reason := memoryRequestOutputGate(kind, body); reason != "" {
		markMemoryGate(ctx, reason)
		return types.ActionContinue
	}
	adapter := memoryProtocolAdapterForKind(kind)
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
	content := gjson.ParseBytes(bodyDigest.Input.Fields["messages"])
	if kind == protocol.ProtocolResponses {
		content = gjson.ParseBytes(bodyDigest.Input.Fields["input"])
	}
	if !content.IsArray() && !(kind == protocol.ProtocolResponses && content.Type == gjson.String) {
		markMemoryGate(ctx, "unsupported-request-content")
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
	currentQuestion, err := memoryCurrentQuestion(body, kind, c.Route.QuestionFrom)
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
		if c.Route.MemoryMode == config.MemoryModeRecentOnly && countCurrentUserMessages(request.Messages) > 1 {
			// The original assembly rule excludes Recent when the client/History
			// already supplied multiple user turns. Its lookup cannot change this
			// effective empty context, including when that lookup fails.
			if err := writeMemoryCacheDigest(memoryMessageAssemblyInput{}); err != nil {
				log.Warnf("[ai-memory] trusted cache digest unavailable for complete current context: %v", err)
			}
		}
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

func skipMemoryRequest(ctx wrapper.HttpContext, c config.PluginConfig, reason string) types.Action {
	markMemoryGate(ctx, reason)
	if c.CaptureBodyBasis {
		// Memory can be off or bypassed while the release still needs the
		// pre-rewrite basis. Downstream headers must wait for that body callback.
		return types.HeaderStopIteration
	}
	ctx.DontReadRequestBody()
	return types.ActionContinue
}

func rejectMemoryBodyBasis(ctx wrapper.HttpContext, status uint32, code string) types.Action {
	ctx.SetContext(memoryBodyBasisRejectedKey, true)
	markMemoryGate(ctx, "body-basis-"+code)
	ctx.DontReadRequestBody()
	response := []byte(`{"error":{"code":"` + code + `","type":"memory_body_basis_error"}}`)
	if err := proxywasm.SendHttpResponse(status, [][2]string{{"content-type", "application/json"}}, response, -1); err != nil {
		proxywasm.LogCritical("ai-memory could not send body-basis rejection")
	}
	return types.ActionPause
}

func memoryCurrentQuestion(body []byte, kind protocol.ProtocolKind, questionFrom string) (string, error) {
	if questionFrom = strings.TrimSpace(questionFrom); questionFrom != "" {
		result := gjson.GetBytes(body, questionFrom)
		if result.Exists() {
			if question := strings.TrimSpace(memoryTextFromGJSON(result)); question != "" {
				return question, nil
			}
		}
	}
	return memoryProtocolCurrentUserPrompt(kind, body)
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
	ctx.SetUserAttribute("ai_memory_gate", reason)
	if err := ctx.WriteUserAttributeToLog(); err != nil {
		log.Warnf("[ai-memory] gate diagnostic unavailable: %v", err)
	}
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
