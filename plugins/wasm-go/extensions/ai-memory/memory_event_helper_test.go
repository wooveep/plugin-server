package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/stretchr/testify/require"
)

func TestMemoryEventPluginVersionMatchesVersionFile(t *testing.T) {
	version, err := os.ReadFile("VERSION")
	require.NoError(t, err)
	require.Equal(t, strings.TrimSpace(string(version)), memoryEventPluginVersion)
}

func TestMemoryEventStructJSONShape(t *testing.T) {
	t.Run("marshals flat required facts with nested route model and normalized usage", func(t *testing.T) {
		event := MemoryEvent{
			SchemaVersion:       memoryEventSchemaVersion,
			MemoryEventEnvelope: MemoryEventEnvelope{EventID: "event-1", IdempotencyKey: "idem-1"},
			MemoryEventIdentity: MemoryEventIdentity{Tenant: "tenant-a", Consumer: "consumer-a", SessionID: "session-a"},
			MemoryEventRequest:  MemoryEventRequest{RequestID: "request-1", RequestPath: "/v1/chat/completions", RequestDigest: "sha256:abc"},
			Route:               &MemoryEventRoute{Name: "memory-route"},
			Model:               &MemoryEventModel{Name: "qwen-turbo"},
			MemoryEventPolicy:   MemoryEventPolicy{PolicyVersion: "policy-v1", MemoryMode: "semantic"},
			MemoryEventStatus:   MemoryEventStatus{StatusCode: 200},
			MemoryEventStream:   MemoryEventStream{IsStream: true},
			MemoryEventToolCall: MemoryEventToolCall{ContainsToolCalls: true},
			Usage:               &MemoryEventUsage{Unit: "token", Input: 12, Output: 5, Total: 17},
			MemoryEventFinish:   MemoryEventFinish{FinishReason: "stop"},
			MemoryEventTiming:   MemoryEventTiming{StartedAtMS: 1782420000000, EndedAtMS: 1782420000500},
			PluginVersion:       memoryEventPluginVersion,
			UserContent:         "safe user",
			AssistantContent:    "safe assistant",
		}

		body, err := json.Marshal(event)
		require.NoError(t, err)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &payload))

		for _, field := range []string{
			"schema_version",
			"event_id",
			"idempotency_key",
			"tenant",
			"consumer",
			"session_id",
			"request_id",
			"request_path",
			"request_digest",
			"route",
			"model",
			"policy_version",
			"memory_mode",
			"status_code",
			"is_stream",
			"contains_tool_calls",
			"usage",
			"finish_reason",
			"started_at_ms",
			"ended_at_ms",
			"plugin_version",
			"user_content",
			"assistant_content",
		} {
			require.Containsf(t, payload, field, "missing MemoryEvent field %q; payload=%s", field, string(body))
		}
		require.EqualValues(t, 1, payload["schema_version"])
		require.Equal(t, map[string]interface{}{"name": "memory-route"}, payload["route"])
		require.Equal(t, map[string]interface{}{"name": "qwen-turbo"}, payload["model"])
		require.Equal(t, map[string]interface{}{
			"unit":   "token",
			"input":  float64(12),
			"output": float64(5),
			"total":  float64(17),
		}, payload["usage"])
		require.Equal(t, memoryEventPluginVersion, payload["plugin_version"])
	})

	t.Run("omits optional empty fields", func(t *testing.T) {
		event := MemoryEvent{
			SchemaVersion:       memoryEventSchemaVersion,
			MemoryEventEnvelope: MemoryEventEnvelope{EventID: "event-2", IdempotencyKey: "idem-2"},
			MemoryEventIdentity: MemoryEventIdentity{Tenant: "tenant-a", Consumer: "consumer-a"},
			MemoryEventRequest:  MemoryEventRequest{RequestID: "request-2", RequestPath: "/v1/chat/completions", RequestDigest: "sha256:def"},
			MemoryEventPolicy:   MemoryEventPolicy{MemoryMode: "recent-only"},
			MemoryEventStatus:   MemoryEventStatus{StatusCode: 204},
			MemoryEventToolCall: MemoryEventToolCall{},
			MemoryEventTiming:   MemoryEventTiming{StartedAtMS: 1782420000000, EndedAtMS: 1782420000500},
			PluginVersion:       memoryEventPluginVersion,
		}

		body, err := json.Marshal(event)
		require.NoError(t, err)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &payload))

		for _, field := range []string{
			"session_id",
			"route",
			"model",
			"policy_version",
			"usage",
			"finish_reason",
			"user_content",
			"assistant_content",
		} {
			require.NotContainsf(t, payload, field, "expected empty optional field %q to be omitted; payload=%s", field, string(body))
		}
		require.Contains(t, payload, "no_store")
		require.Equal(t, false, payload["no_store"])
		require.Contains(t, payload, "contains_tool_calls")
		require.Equal(t, false, payload["contains_tool_calls"])
		require.Contains(t, payload, "is_stream")
		require.Equal(t, false, payload["is_stream"])
	})
}

func TestMemoryBuildEventRawContentRules(t *testing.T) {
	tests := []struct {
		name            string
		captureResponse bool
		noStore         bool
		capture         memoryResponseCapture
		wantNoStore     bool
		wantRaw         bool
	}{
		{
			name:            "disabled capture omits raw and preserves safe facts",
			captureResponse: false,
			capture:         memoryBuildEventTestCapture(200, "disabled capture assistant must be omitted"),
		},
		{
			name:            "no-store omits raw and preserves safe facts",
			captureResponse: true,
			noStore:         true,
			wantNoStore:     true,
			capture:         memoryBuildEventTestCapture(200, "no-store assistant must be omitted"),
		},
		{
			name:            "parse failure omits raw and preserves safe facts",
			captureResponse: true,
			capture: func() memoryResponseCapture {
				capture := memoryBuildEventTestCapture(200, "parse failure assistant must be omitted")
				capture.ParseFailed = true
				return capture
			}(),
		},
		{
			name:            "unsafe content omits raw and preserves safe facts",
			captureResponse: true,
			capture: func() memoryResponseCapture {
				capture := memoryBuildEventTestCapture(200, "unsafe assistant must be omitted")
				capture.UnsafeContent = true
				return capture
			}(),
		},
		{
			name:            "ineligible status omits raw and preserves safe facts",
			captureResponse: true,
			capture:         memoryBuildEventTestCapture(500, "failed status assistant must be omitted"),
		},
		{
			name:            "eligible safe content includes raw",
			captureResponse: true,
			capture:         memoryBuildEventTestCapture(201, "eligible assistant content"),
			wantRaw:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := memoryBuildEventTestContext(tt.noStore)
			cfg := memoryBuildEventTestConfig(tt.captureResponse)

			event := memoryBuildEvent(ctx, cfg, tt.capture, 1782420000500)

			requireMemoryBuildEventSafeFacts(t, event, tt.capture.StatusCode)
			require.Equal(t, tt.wantNoStore, event.NoStore)
			require.Equal(t, tt.capture.FinishReason, event.FinishReason)
			require.Equal(t, tt.capture.ContainsToolCalls, event.ContainsToolCalls)
			require.Equal(t, tt.capture.IsStream, event.IsStream)
			require.NotNil(t, event.Usage)
			require.Equal(t, "token", event.Usage.Unit)
			require.Equal(t, tt.capture.Usage.PromptTokens, event.Usage.Input)
			require.Equal(t, tt.capture.Usage.CompletionTokens, event.Usage.Output)
			require.Equal(t, tt.capture.Usage.TotalTokens, event.Usage.Total)

			payload := memoryBuildEventJSONPayload(t, event)
			if tt.wantRaw {
				require.Equal(t, "event helper user content", payload["user_content"])
				require.Equal(t, tt.capture.AssistantContent, payload["assistant_content"])
				return
			}
			require.NotContains(t, payload, "user_content")
			require.NotContains(t, payload, "assistant_content")
		})
	}
}

func TestMemoryBuildEventOmitsZeroUsage(t *testing.T) {
	ctx := memoryBuildEventTestContext(false)
	cfg := memoryBuildEventTestConfig(true)
	capture := memoryBuildEventTestCapture(200, "assistant without usage")
	capture.Usage = sessionctx.Usage{}

	event := memoryBuildEvent(ctx, cfg, capture, 1782420000500)

	require.Nil(t, event.Usage)
	payload := memoryBuildEventJSONPayload(t, event)
	require.NotContains(t, payload, "usage")
	require.Equal(t, "event helper user content", payload["user_content"])
	require.Equal(t, "assistant without usage", payload["assistant_content"])
}

func memoryBuildEventTestConfig(captureResponse bool) config.PluginConfig {
	return config.PluginConfig{
		Route: config.RouteConfig{
			MemoryMode:      config.MemoryModeSemantic,
			CaptureResponse: captureResponse,
			PolicyVersion:   "7",
		},
	}
}

func memoryBuildEventTestContext(noStore bool) *memoryCaptureTestContext {
	ctx := newMemoryCaptureTestContext()
	ctx.SetContext(memoryTenantContextKey, "tenant-a")
	ctx.SetContext(memoryConsumerContextKey, "consumer-a")
	ctx.SetContext(memorySessionContextKey, "session-a")
	ctx.SetContext(memoryRequestIDContextKey, "request-a")
	ctx.SetContext(memoryRequestPathContextKey, "/v1/chat/completions")
	ctx.SetContext(memoryRequestDigestContextKey, "sha256:request")
	ctx.SetContext(memoryModeContextKey, config.MemoryModeSemantic)
	ctx.SetContext(memoryPolicyVersionContextKey, "7")
	ctx.SetContext(memoryRouteContextKey, "memory-route")
	ctx.SetContext(memoryModelContextKey, "qwen-turbo")
	ctx.SetContext(memoryStartedAtContextKey, int64(1782420000000))
	ctx.SetContext(memoryUserContentContextKey, "event helper user content")
	ctx.SetContext(memoryNoStoreContextKey, noStore)
	return ctx
}

func memoryBuildEventTestCapture(statusCode int, assistantContent string) memoryResponseCapture {
	return memoryResponseCapture{
		AssistantContent:  assistantContent,
		FinishReason:      "stop",
		Usage:             sessionctx.Usage{PromptTokens: 7, CompletionTokens: 5, TotalTokens: 12},
		StatusCode:        statusCode,
		ContainsToolCalls: true,
		IsStream:          true,
	}
}

func requireMemoryBuildEventSafeFacts(t *testing.T, event MemoryEvent, statusCode int) {
	t.Helper()
	require.Equal(t, memoryEventSchemaVersion, event.SchemaVersion)
	require.NotEmpty(t, event.EventID)
	require.NotEmpty(t, event.IdempotencyKey)
	require.Equal(t, "tenant-a", event.Tenant)
	require.Equal(t, "consumer-a", event.Consumer)
	require.Equal(t, "session-a", event.SessionID)
	require.Equal(t, "request-a", event.RequestID)
	require.Equal(t, "/v1/chat/completions", event.RequestPath)
	require.Equal(t, "sha256:request", event.RequestDigest)
	require.Equal(t, "memory-route", event.Route.Name)
	require.Equal(t, "qwen-turbo", event.Model.Name)
	require.Equal(t, "7", event.PolicyVersion)
	require.Equal(t, config.MemoryModeSemantic, event.MemoryMode)
	require.Equal(t, statusCode, event.StatusCode)
	require.Equal(t, int64(1782420000000), event.StartedAtMS)
	require.Equal(t, int64(1782420000500), event.EndedAtMS)
	require.Equal(t, memoryEventPluginVersion, event.PluginVersion)

	repeatedContext := memoryBuildEventTestContext(event.NoStore)
	repeated := memoryBuildEvent(repeatedContext, memoryBuildEventTestConfig(true), memoryBuildEventTestCapture(statusCode, "repeat"), event.EndedAtMS+1)
	require.Equal(t, event.IdempotencyKey, repeated.IdempotencyKey)
}

func memoryBuildEventJSONPayload(t *testing.T, event MemoryEvent) map[string]interface{} {
	t.Helper()
	body, err := json.Marshal(event)
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &payload))
	return payload
}
