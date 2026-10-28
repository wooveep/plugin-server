package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

const thinCacheEventKind = "cache_event"
const thinCachePluginVersion = "ai-cache-thin-v1"

// nativeCacheProcessingIdentity is captured once from the controlled rule and
// preceding native authentication. It is not decoded from client input.
type nativeCacheProcessingIdentity struct {
	config.NativeIdentityConfig
	Caller balanceidentity.Snapshot `json:"caller"`
}

func cacheNativeIdentity(ctx wrapper.HttpContext) *nativeCacheProcessingIdentity {
	identity, _ := ctx.GetContext(CACHE_NATIVE_IDENTITY_KEY).(*nativeCacheProcessingIdentity)
	return identity
}

type ThinCacheEvent struct {
	sessionctx.EventEnvelope
	NativeIdentity        *nativeCacheProcessingIdentity `json:"native_identity,omitempty"`
	SemanticContextDigest string                         `json:"semantic_context_digest,omitempty"`
	SessionID             string                         `json:"session_id,omitempty"`
	Route                 string                         `json:"route"`
	Model                 string                         `json:"model"`
	RequestPath           string                         `json:"request_path"`
	Protocol              protocol.ProtocolKind          `json:"protocol"`
	CacheScope            string                         `json:"cache_scope"`
	CachePolicyVersion    string                         `json:"cache_policy_version"`
	StatusCode            int                            `json:"status_code"`
	IsStream              bool                           `json:"is_stream"`
	ContainsToolCalls     bool                           `json:"contains_tool_calls"`
	NoStore               bool                           `json:"no_store"`
	Sensitive             bool                           `json:"sensitive"`
	PluginVersion         string                         `json:"plugin_version"`
	UserContent           string                         `json:"user_content,omitempty"`
	AssistantContent      string                         `json:"assistant_content,omitempty"`
	FinishReason          string                         `json:"finish_reason,omitempty"`
	Usage                 json.RawMessage                `json:"usage,omitempty"`
}

type thinResponseFacts struct {
	assistantContent  string
	finishReason      string
	usage             json.RawMessage
	containsToolCalls bool
	parseFailed       bool
}

type thinStreamCapture struct {
	chunks      [][]byte
	parseFailed bool
}

func captureThinResponseHeaders(ctx wrapper.HttpContext, log logs.Log) {
	statusHeader, _ := proxywasm.GetHttpResponseHeader(":status")
	statusCode, err := strconv.Atoi(statusHeader)
	if err != nil {
		log.Warnf("[%s] [captureThinResponseHeaders] invalid response status %q: %v", PLUGIN_NAME, statusHeader, err)
		statusCode = 0
	}
	ctx.SetContext(CACHE_RESPONSE_STATUS_KEY, statusCode)
	ctx.SetContext(CACHE_RESPONSE_NOSTORE_KEY, responseHasNoStore())
}

func handleThinResponseBody(ctx wrapper.HttpContext, c config.PluginConfig, chunk []byte, isLastChunk bool, log logs.Log) {
	if !c.Event.RedisStream.Enabled {
		return
	}

	path := ctx.GetStringContext(CACHE_PATH_CONTEXT_KEY, "")
	kind := c.ProtocolForPath(path)
	adapter, ok := thinProtocolAdapterForKind(kind)
	if !ok {
		log.Warnf("[%s] [handleThinResponseBody] unsupported request path for protocol adapter: %s", PLUGIN_NAME, path)
		return
	}
	statusCode := thinIntContext(ctx, CACHE_RESPONSE_STATUS_KEY, 0)
	stream := ctx.GetContext(STREAM_CONTEXT_KEY) != nil
	if stream {
		capture := getThinStreamCapture(ctx)
		capture.appendChunk(unifySSEChunk(chunk))
		if !isLastChunk {
			return
		}
		exchange, err := adapter.CaptureStream(protocol.ResponseStreamInput{
			StatusCode: statusCode,
			Chunks:     capture.chunks,
		})
		if err != nil {
			log.Warnf("[%s] [handleThinResponseBody] parse streaming response chunk failed: %v", PLUGIN_NAME, err)
			emitThinCacheEvent(ctx, c, kind, thinResponseFacts{parseFailed: true}, true, log)
			return
		}
		if reason := thinReplayOutputGate(kind, exchange.Replay); reason != "" {
			markCacheGate(ctx, reason)
			return
		}
		facts := thinResponseFactsFromExchange(kind, exchange)
		facts.parseFailed = facts.parseFailed || capture.parseFailed
		emitThinCacheEvent(ctx, c, kind, facts, true, log)
		return
	}

	body := appendThinResponseBodyChunk(ctx, chunk)
	if !isLastChunk {
		return
	}
	exchange, err := adapter.CaptureResponse(protocol.ResponseCaptureInput{
		StatusCode: statusCode,
		Body:       body,
	})
	if err != nil {
		log.Warnf("[%s] [handleThinResponseBody] parse non-streaming response failed: %v", PLUGIN_NAME, err)
		emitThinCacheEvent(ctx, c, kind, thinResponseFacts{parseFailed: true}, false, log)
		return
	}
	if reason := thinReplayOutputGate(kind, exchange.Replay); reason != "" {
		markCacheGate(ctx, reason)
		return
	}
	facts := thinResponseFactsFromExchange(kind, exchange)
	emitThinCacheEvent(ctx, c, kind, facts, false, log)
}

func appendThinResponseBodyChunk(ctx wrapper.HttpContext, chunk []byte) []byte {
	body, _ := ctx.GetContext(CACHE_RESPONSE_CAPTURE_KEY).([]byte)
	body = append(body, chunk...)
	ctx.SetContext(CACHE_RESPONSE_CAPTURE_KEY, body)
	return body
}

func getThinStreamCapture(ctx wrapper.HttpContext) *thinStreamCapture {
	if capture, ok := ctx.GetContext(CACHE_STREAM_CAPTURE_KEY).(*thinStreamCapture); ok {
		return capture
	}
	capture := &thinStreamCapture{}
	ctx.SetContext(CACHE_STREAM_CAPTURE_KEY, capture)
	return capture
}

func (c *thinStreamCapture) appendChunk(chunk []byte) {
	c.chunks = append(c.chunks, append([]byte(nil), chunk...))
}

func emitThinCacheEvent(ctx wrapper.HttpContext, c config.PluginConfig, kind protocol.ProtocolKind, facts thinResponseFacts, stream bool, log logs.Log) {
	event := buildThinCacheEvent(ctx, c, kind, facts, stream)
	redisClient := c.GetEventRedisClient()
	if redisClient == nil {
		log.Warnf("[%s] [emitThinCacheEvent] Redis Stream client is not configured", PLUGIN_NAME)
		return
	}

	command, err := sessionctx.BuildRedisStreamXADD(sessionctx.RedisStreamXADDInput{
		Stream: c.Event.RedisStream.Stream,
		Field:  c.Event.RedisStream.Field,
		Event:  event,
	})
	if err != nil {
		log.Warnf("[%s] [emitThinCacheEvent] build Redis Stream XADD failed: %v", PLUGIN_NAME, err)
		return
	}
	args := make([]interface{}, len(command))
	for i, arg := range command {
		args[i] = arg
	}
	if err := redisClient.Command(args, func(response resp.Value) {
		if err := response.Error(); err != nil {
			log.Warnf("[%s] [emitThinCacheEvent] Redis Stream XADD failed open: %v", PLUGIN_NAME, err)
		}
	}); err != nil {
		log.Warnf("[%s] [emitThinCacheEvent] Redis Stream XADD dispatch failed open: %v", PLUGIN_NAME, err)
	}
}

func buildThinCacheEvent(ctx wrapper.HttpContext, c config.PluginConfig, kind protocol.ProtocolKind, facts thinResponseFacts, stream bool) ThinCacheEvent {
	startedAt := thinInt64Context(ctx, CACHE_EVENT_STARTED_AT_KEY, 0)
	endedAt := nowMillis()
	if startedAt <= 0 {
		startedAt = endedAt
	}
	statusCode := thinIntContext(ctx, CACHE_RESPONSE_STATUS_KEY, 0)
	noStore := ctx.GetBoolContext(CACHE_RESPONSE_NOSTORE_KEY, false)
	sensitive := ctx.GetBoolContext(CACHE_SENSITIVE_KEY, false)
	gated := facts.parseFailed || statusCode < 200 || statusCode >= 300 || noStore || sensitive || facts.containsToolCalls

	event := ThinCacheEvent{
		EventEnvelope: sessionctx.NewEventEnvelope(sessionctx.EventEnvelopeInput{
			EventKind:     thinCacheEventKind,
			Tenant:        ctx.GetStringContext(CACHE_TENANT_CONTEXT_KEY, ""),
			Consumer:      ctx.GetStringContext(CACHE_CONSUMER_CONTEXT_KEY, ""),
			RequestID:     ctx.GetStringContext(CACHE_REQUEST_ID_KEY, ""),
			RequestDigest: ctx.GetStringContext(CACHE_DIGEST_CONTEXT_KEY, ""),
			StartedAtMS:   startedAt,
			EndedAtMS:     endedAt,
		}),
		NativeIdentity:        cacheNativeIdentity(ctx),
		SemanticContextDigest: ctx.GetStringContext(CACHE_SEMANTIC_CONTEXT_KEY, ""),
		SessionID:             ctx.GetStringContext(CACHE_SESSION_CONTEXT_KEY, ""),
		Route:                 ctx.GetStringContext(CACHE_ROUTE_CONTEXT_KEY, ""),
		Model:                 ctx.GetStringContext(CACHE_MODEL_CONTEXT_KEY, ""),
		RequestPath:           ctx.GetStringContext(CACHE_PATH_CONTEXT_KEY, ""),
		Protocol:              kind,
		CacheScope:            c.CacheScope,
		CachePolicyVersion:    ctx.GetStringContext(CACHE_POLICY_VERSION_KEY, c.CachePolicyVersion),
		StatusCode:            statusCode,
		IsStream:              stream,
		ContainsToolCalls:     facts.containsToolCalls,
		NoStore:               noStore,
		Sensitive:             sensitive,
		PluginVersion:         thinCachePluginVersion,
		FinishReason:          facts.finishReason,
	}
	if !thinUsageEmpty(facts.usage) {
		event.Usage = facts.usage
	}
	if !gated {
		event.UserContent = ctx.GetStringContext(CACHE_USER_CONTENT_KEY, "")
		event.AssistantContent = facts.assistantContent
	} else if !sensitive && !noStore {
		event.UserContent = ctx.GetStringContext(CACHE_USER_CONTENT_KEY, "")
	}
	return event
}

func currentRequestID() string {
	if requestID, _ := proxywasm.GetHttpRequestHeader("x-request-id"); requestID != "" {
		return requestID
	}
	property, _ := proxywasm.GetProperty([]string{"x_request_id"})
	return string(property)
}

func responseHasNoStore() bool {
	cacheControl, _ := proxywasm.GetHttpResponseHeader("cache-control")
	for _, directive := range strings.Split(cacheControl, ",") {
		if strings.EqualFold(strings.TrimSpace(directive), "no-store") {
			return true
		}
	}
	return false
}

func nowMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func thinIntContext(ctx wrapper.HttpContext, key string, fallback int) int {
	switch value := ctx.GetContext(key).(type) {
	case int:
		return value
	case int64:
		return int(value)
	default:
		return fallback
	}
}

func thinInt64Context(ctx wrapper.HttpContext, key string, fallback int64) int64 {
	switch value := ctx.GetContext(key).(type) {
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return fallback
	}
}

func thinUsageEmpty(usage json.RawMessage) bool {
	if len(usage) == 0 {
		return true
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(usage, &fields); err != nil {
		return false
	}
	return len(fields) == 0
}
