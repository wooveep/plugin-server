package main

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

const (
	memoryEventSchemaVersion = 1
	memoryEventKind          = "memory_event"
	memoryEventField         = "event"

	memoryEventDeliveredContextKey = "memoryEventDelivered"
)

//go:embed VERSION
var memoryEventReleaseVersion string

var memoryEventPluginVersion = strings.TrimSpace(memoryEventReleaseVersion)

type MemoryEvent struct {
	SchemaVersion  int                   `json:"schema_version"`
	NativeIdentity *memoryNativeIdentity `json:"native_identity,omitempty"`

	MemoryEventEnvelope
	MemoryEventIdentity
	MemoryEventRequest

	Route *MemoryEventRoute `json:"route,omitempty"`
	Model *MemoryEventModel `json:"model,omitempty"`
	Usage *MemoryEventUsage `json:"usage,omitempty"`

	MemoryEventPolicy
	MemoryEventStatus
	MemoryEventStream
	MemoryEventToolCall
	MemoryEventFinish
	MemoryEventTiming

	PluginVersion string `json:"plugin_version"`
	NoStore       bool   `json:"no_store"`

	UserContent      string `json:"user_content,omitempty"`
	AssistantContent string `json:"assistant_content,omitempty"`
}

type MemoryEventEnvelope struct {
	EventID        string `json:"event_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

type MemoryEventIdentity struct {
	Tenant    string `json:"tenant"`
	Consumer  string `json:"consumer"`
	SessionID string `json:"session_id,omitempty"`
}

type MemoryEventRequest struct {
	CallID        string `json:"call_id,omitempty"`
	RequestID     string `json:"request_id"`
	RequestPath   string `json:"request_path"`
	RequestDigest string `json:"request_digest"`
}

type MemoryEventRoute struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}

type MemoryEventModel struct {
	Name string `json:"name"`
}

type MemoryEventPolicy struct {
	PolicyVersion string `json:"policy_version,omitempty"`
	MemoryMode    string `json:"memory_mode"`
}

type MemoryEventStatus struct {
	StatusCode int `json:"status_code"`
}

type MemoryEventStream struct {
	IsStream bool `json:"is_stream"`
}

type MemoryEventToolCall struct {
	ContainsToolCalls bool `json:"contains_tool_calls"`
}

type MemoryEventUsage struct {
	Unit   string `json:"unit"`
	Input  int    `json:"input"`
	Output int    `json:"output"`
	Total  int    `json:"total"`
}

type MemoryEventFinish struct {
	FinishReason string `json:"finish_reason,omitempty"`
}

type MemoryEventTiming struct {
	StartedAtMS int64 `json:"started_at_ms"`
	EndedAtMS   int64 `json:"ended_at_ms"`
}

func memoryBuildEvent(ctx wrapper.HttpContext, c config.PluginConfig, capture memoryResponseCapture, endedAtMS int64) MemoryEvent {
	startedAtMS := memoryInt64Context(ctx, memoryStartedAtContextKey)
	tenant := ctx.GetStringContext(memoryTenantContextKey, "")
	consumer := ctx.GetStringContext(memoryConsumerContextKey, "")
	requestID := ctx.GetStringContext(memoryRequestIDContextKey, "")
	requestDigest := ctx.GetStringContext(memoryRequestDigestContextKey, "")
	envelope := sessionctx.NewEventEnvelope(sessionctx.EventEnvelopeInput{
		EventKind:     memoryEventKind,
		Tenant:        tenant,
		Consumer:      consumer,
		RequestID:     requestID,
		RequestDigest: requestDigest,
		StartedAtMS:   startedAtMS,
		EndedAtMS:     endedAtMS,
	})

	event := MemoryEvent{
		SchemaVersion:  memoryEventSchemaVersion,
		NativeIdentity: nativeMemoryIdentity(ctx, c),
		MemoryEventEnvelope: MemoryEventEnvelope{
			EventID:        envelope.EventID,
			IdempotencyKey: envelope.IdempotencyKey,
		},
		MemoryEventIdentity: MemoryEventIdentity{
			Tenant:    tenant,
			Consumer:  consumer,
			SessionID: ctx.GetStringContext(memorySessionContextKey, ""),
		},
		MemoryEventRequest: MemoryEventRequest{
			RequestID:     requestID,
			RequestPath:   ctx.GetStringContext(memoryRequestPathContextKey, ""),
			RequestDigest: requestDigest,
		},
		MemoryEventPolicy: MemoryEventPolicy{
			PolicyVersion: ctx.GetStringContext(memoryPolicyVersionContextKey, c.Route.PolicyVersion),
			MemoryMode:    ctx.GetStringContext(memoryModeContextKey, c.Route.MemoryMode),
		},
		MemoryEventStatus:   MemoryEventStatus{StatusCode: capture.StatusCode},
		MemoryEventStream:   MemoryEventStream{IsStream: capture.IsStream},
		MemoryEventToolCall: MemoryEventToolCall{ContainsToolCalls: capture.ContainsToolCalls},
		MemoryEventFinish:   MemoryEventFinish{FinishReason: capture.FinishReason},
		MemoryEventTiming:   MemoryEventTiming{StartedAtMS: startedAtMS, EndedAtMS: endedAtMS},
		PluginVersion:       memoryEventPluginVersion,
		NoStore:             ctx.GetBoolContext(memoryNoStoreContextKey, false),
		Usage:               memoryEventUsage(capture.Usage),
	}
	if route := ctx.GetStringContext(memoryRouteContextKey, ""); route != "" {
		event.Route = &MemoryEventRoute{Name: route}
	}
	if c.NativeIdentity != nil {
		event.Route = &MemoryEventRoute{ID: c.NativeIdentity.RouteID, Name: ctx.GetStringContext(memoryRouteNameContextKey, "")}
		if admitted, ok := ctx.GetContext(memoryNativeCallContextKey).(executionfacts.AdmittedCall); ok {
			event.CallID = admitted.CallID
		}
	}
	if model := ctx.GetStringContext(memoryModelContextKey, ""); model != "" {
		event.Model = &MemoryEventModel{Name: model}
	}
	if memoryEventRawContentAllowed(ctx, c, capture) {
		event.UserContent = ctx.GetStringContext(memoryUserContentContextKey, "")
		event.AssistantContent = capture.AssistantContent
	}
	return event
}

func memoryEventRawContentAllowed(ctx wrapper.HttpContext, c config.PluginConfig, capture memoryResponseCapture) bool {
	return memoryRawContentAllowed(ctx, c) &&
		!capture.ParseFailed &&
		!capture.UnsafeContent &&
		memoryEventStatusEligible(capture.StatusCode)
}

func memoryEventStatusEligible(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

func memoryEventUsage(usage sessionctx.Usage) *MemoryEventUsage {
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0 {
		return nil
	}
	return &MemoryEventUsage{
		Unit:   "token",
		Input:  usage.PromptTokens,
		Output: usage.CompletionTokens,
		Total:  usage.TotalTokens,
	}
}

func memoryEmitEventAfterResponseCompletion(ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log) {
	if memoryGateReason(ctx) != "" || ctx.GetBoolContext(memoryEventDeliveredContextKey, false) {
		return
	}
	capture, ok := ctx.GetContext(memoryResponseCaptureContextKey).(memoryResponseCapture)
	if !ok {
		return
	}
	ctx.SetContext(memoryEventDeliveredContextKey, true)

	event := memoryBuildEvent(ctx, c, capture, nowMillis())
	command, err := sessionctx.BuildRedisStreamXADD(sessionctx.RedisStreamXADDInput{
		Stream: c.RedisStream.Stream,
		Field:  memoryEventField,
		Event:  event,
	})
	if err != nil {
		log.Warnf("[ai-memory] Redis Stream event build failed open, request_id:%s stream:%s err:%v", event.RequestID, c.RedisStream.Stream, err)
		return
	}
	if err := memoryDispatchEventXADD(c, event, command, log); err != nil {
		log.Warnf("[ai-memory] Redis Stream dispatch failed open, request_id:%s stream:%s err:%v", event.RequestID, c.RedisStream.Stream, err)
	}
}

func memoryDispatchEventXADD(c config.PluginConfig, event MemoryEvent, command []string, log logs.Log) error {
	client := c.GetEventRedisClient()
	if client == nil {
		return fmt.Errorf("memory event Redis client is not configured")
	}
	return client.Command(command, func(value resp.Value) {
		if err := value.Error(); err != nil {
			log.Warnf("[ai-memory] Redis Stream delivery failed open, request_id:%s stream:%s reason:redis_response_error", event.RequestID, c.RedisStream.Stream)
			return
		}
		log.Debugf("[ai-memory] Redis Stream delivery accepted, request_id:%s stream:%s id:%s", event.RequestID, c.RedisStream.Stream, value.String())
	})
}

func memoryInt64Context(ctx wrapper.HttpContext, key string) int64 {
	switch value := ctx.GetContext(key).(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case int32:
		return int64(value)
	case float64:
		return int64(value)
	default:
		return 0
	}
}
