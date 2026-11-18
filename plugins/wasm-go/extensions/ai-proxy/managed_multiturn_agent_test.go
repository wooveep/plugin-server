package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/observedusage"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Regression for live modelfusion-dev call b06c474b-1c4d-449b-8d53-640d3cbe1886:
// DashScope /apps/anthropic/v1/messages (deepseek-v4.1-flash-1017, chat_round=4)
// emits pre-settlement message_start usage (input_tokens=10100, cache_read_input_tokens=0)
// and settles authoritative usage in message_delta (input_tokens=8862 < 10100,
// cache_read_input_tokens=1238, output_tokens=16, service_tier="standard").
func TestMessagesIncidentSettledCacheSplitAndMultiBlockAgentStream(t *testing.T) {
	h := incidentProtocolRouter(t, clientresponse.Messages, "auto", true, nil)
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false)

	stream := "event: ping\ndata: {\"type\":\"ping\"}\n\n" +
		"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_b06c474b\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"upstream-model\",\"content\":[],\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":10100,\"output_tokens\":0,\"cache_read_input_tokens\":0,\"cache_creation_input_tokens\":0}}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"Inspecting tool output...\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"signature_delta\",\"signature\":\"sig_round4\"}}\n\n" +
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_round4\",\"name\":\"read_file\",\"input\":{}}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\\\"README.md\\\"}\"}}\n\n" +
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\",\"stop_sequence\":null},\"usage\":{\"input_tokens\":8862,\"output_tokens\":16,\"cache_read_input_tokens\":1238,\"cache_creation_input_tokens\":0,\"cache_creation\":{\"ephemeral_5m_input_tokens\":0,\"ephemeral_1h_input_tokens\":0},\"service_tier\":\"standard\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n" +
		"event: ping\ndata: {\"type\":\"ping\"}\n\n"

	h.host.CallOnResponseBody(h.id, []byte(stream), true)
	for _, call := range h.host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID) {
		if bytes.Contains(call.Query, []byte(state.lastDelivery.EventID())) {
			h.host.CallOnRedisCallResponse(call.CalloutID, 0, []byte("$3\r\n1-0\r\n"))
			break
		}
	}

	wire := string(h.host.GetCurrentResponseBody(h.id))
	require.NotContains(t, wire, "response_incomplete")
	require.Contains(t, wire, `"type":"thinking_delta"`)
	require.Contains(t, wire, `"type":"signature_delta"`)
	require.Contains(t, wire, `"type":"input_json_delta"`)
	require.Contains(t, wire, `"stop_reason":"tool_use"`)
	require.Contains(t, wire, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")

	require.NotNil(t, state.lastDelivery)
	delivery := state.lastDelivery.Envelope()
	require.Equal(t, "succeeded", delivery.DeliveryState)
	require.Equal(t, "reported", delivery.Usage.Status)
	require.Equal(t, "10100", delivery.Usage.Quantities["input"])
	require.Equal(t, "1238", delivery.Usage.Quantities["cache_hit"])
	require.Equal(t, "8862", delivery.Usage.Quantities["cache_miss"])
	require.Equal(t, "16", delivery.Usage.Quantities["output"])

	raw, err := h.host.GetProperty([]string{observedusage.Property})
	require.NoError(t, err)
	require.Equal(t, "10100", gjson.GetBytes(raw, "input").String())
	require.Equal(t, "16", gjson.GetBytes(raw, "output").String())
	h.host.CompleteHttpContext(h.id)
}

func TestMessagesMultiDeltaStopReasonsAndNullCacheFields(t *testing.T) {
	newManagedHost(t)
	profile := managedProtocolFixture(t, managedProtocolMessages, managedProtocolMessages)

	// Split message_delta: stop_reason frame followed by usage frame
	split := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"type\":\"message\",\"model\":\"frozen-upstream\",\"usage\":{\"input_tokens\":500,\"output_tokens\":0}}}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"}}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"input_tokens\":420,\"output_tokens\":34,\"cache_read_input_tokens\":null}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	obs := profile.ObserveResponse(200, managedProtocolSSEHeaders, []byte(split), true)
	require.Equal(t, "succeeded", obs.ExecutionResult)
	require.Equal(t, "reported", obs.Usage.Status)
	require.Equal(t, "420", obs.Usage.Quantities["input"])
	require.Equal(t, "34", obs.Usage.Quantities["output"])

	// Conflicting stop_reason across two message_delta frames must still fail
	conflictStop := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"type\":\"message\",\"model\":\"frozen-upstream\",\"usage\":{\"input_tokens\":500,\"output_tokens\":0}}}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"}}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":34}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	obsConflict := profile.ObserveResponse(200, managedProtocolSSEHeaders, []byte(conflictStop), true)
	require.Equal(t, "unknown", obsConflict.ExecutionResult)

	// All valid stop_reason values + null cache_read_input_tokens on JSON Messages
	for reason, wantResult := range map[string]string{
		"end_turn":                      "succeeded",
		"tool_use":                      "succeeded",
		"stop_sequence":                 "succeeded",
		"refusal":                       "succeeded",
		"max_tokens":                    "partial",
		"pause_turn":                    "partial",
		"model_context_window_exceeded": "partial",
	} {
		doc := fmt.Sprintf(`{"id":"msg_1","type":"message","model":"frozen-upstream","stop_reason":%q,"usage":{"input_tokens":120,"output_tokens":15,"cache_read_input_tokens":null,"cache_creation_input_tokens":null}}`, reason)
		got := profile.ObserveResponse(200, managedProtocolJSONHeaders, []byte(doc), true)
		require.Equal(t, wantResult, got.ExecutionResult, "stop_reason=%s", reason)
		require.Equal(t, "reported", got.Usage.Status, "stop_reason=%s", reason)
		require.Equal(t, "120", got.Usage.Quantities["input"])
		require.Equal(t, "15", got.Usage.Quantities["output"])
	}
}

func TestMessagesToChatConversionPreservesReasoningHistoryAndStripsInternalFields(t *testing.T) {
	claudeMultiTurn := []byte(`{
		"model": "frozen",
		"max_tokens": 256,
		"thinking": {"type": "enabled", "budget_tokens": 2048},
		"output_config": {"effort": "high"},
		"anthropic_beta": ["interleaved-thinking-2025-05-14"],
		"messages": [
			{"role": "user", "content": "Check server status"},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Need to call check_status first.", "signature": "sig_1"},
				{"type": "tool_use", "id": "toolu_1", "name": "check_status", "input": {"host": "prod"}}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "toolu_1", "content": "OK"}
			]}
		]
	}`)
	for _, tc := range []struct {
		supplier      string
		wantReasoning bool
	}{
		{"deepseek", true},
		{"qwen", true},
		{"zhipuai", true},
		{"moonshot", true},
		{"openrouter", true},
		{"openai", false},
	} {
		t.Run(tc.supplier, func(t *testing.T) {
			p, err := newManagedProtocolProfile(managedProtocolMessages, managedProtocolChat)
			require.NoError(t, err)
			p.supplier = tc.supplier
			req, err := p.PrepareRequest(claudeMultiTurn, "frozen", "fixture-key")
			require.NoError(t, err)
			for _, internal := range []string{
				"claude_thinking",
				"claude_output_config",
				"claude_anthropic_beta",
				"messages.1.claude_content_blocks",
				"messages.2.claude_content_blocks",
			} {
				require.False(t, gjson.GetBytes(req.Body, internal).Exists(), "leaked internal field %s in %s", internal, string(req.Body))
			}
			gotReasoning := gjson.GetBytes(req.Body, "messages.1.reasoning_content").String()
			if tc.wantReasoning {
				require.Equal(t, "Need to call check_status first.", gotReasoning)
			} else {
				require.Empty(t, gotReasoning)
			}
		})
	}
}

func TestChatStreamEmptyFinishReasonAndResponsesReasoningFallback(t *testing.T) {
	newManagedHost(t)
	chatProfile := managedProtocolFixture(t, managedProtocolChat, managedProtocolChat)
	streamWithEmptyFinish := []byte(
		"data: {\"id\":\"c1\",\"model\":\"frozen-upstream\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"reasoning\":\"step 1 \",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"ls\",\"arguments\":\"{\"}}]},\"finish_reason\":\"\"}]}\n\n" +
			"data: {\"id\":\"c1\",\"model\":\"frozen-upstream\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"\",\"function\":{\"arguments\":\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n" +
			"data: {\"id\":\"c1\",\"model\":\"frozen-upstream\",\"choices\":[],\"usage\":{\"prompt_tokens\":30,\"completion_tokens\":10,\"total_tokens\":40}}\n\n" +
			"data: [DONE]\n\n",
	)
	obs := chatProfile.ObserveResponse(200, managedProtocolSSEHeaders, streamWithEmptyFinish, true)
	require.Equal(t, "succeeded", obs.ExecutionResult)
	require.Equal(t, "reported", obs.Usage.Status)

	respProfile := managedProtocolFixture(t, managedProtocolResponses, managedProtocolChat)
	converted, err := respProfile.ConvertResponse(nil, 200, managedProtocolSSEHeaders, streamWithEmptyFinish)
	require.NoError(t, err)
	require.Contains(t, string(converted.Body), "response.reasoning_text.delta")
	require.Contains(t, string(converted.Body), "step 1 ")
	require.Contains(t, string(converted.Body), "response.completed")
}
