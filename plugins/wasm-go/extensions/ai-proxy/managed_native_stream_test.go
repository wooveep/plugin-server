package main

import (
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/tokenestimate"
	"github.com/stretchr/testify/require"
)

func TestManagedNativeStreamCollectsToolAndThinkingDeltasForFallbackEstimate(t *testing.T) {
	t.Run("messages truncated during thinking_delta and input_json_delta", func(t *testing.T) {
		stream := newManagedNativeStream(managedProtocolMessages, false, true)
		payload := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"m1\",\"type\":\"message\",\"model\":\"qwen3.8-flash\",\"usage\":{\"input_tokens\":6556,\"output_tokens\":0}}}\n\n" +
			"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n" +
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"Analyzing workspace layout. \"}}\n\n" +
			"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
			"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"read_file\",\"input\":{}}}\n\n" +
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\\\"src/main.rs\\\"}\"}}\n\n"
		stream.push([]byte(payload), nil)

		obs := stream.observation()
		require.Equal(t, "unknown", obs.ExecutionResult)
		require.Equal(t, "missing", obs.Usage.Status)

		extracted := tokenestimate.ResponseText(stream.estimateBody())
		require.Equal(t, `Analyzing workspace layout. {"path":"src/main.rs"}`, extracted)
		estimated, err := tokenestimate.Count("qwen3.8-flash", extracted)
		require.NoError(t, err)
		require.Greater(t, estimated, int64(0))
	})

	t.Run("responses truncated during reasoning_text.delta and reasoning_summary_text.delta", func(t *testing.T) {
		stream := newManagedNativeStream(managedProtocolResponses, false, true)
		payload := "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"r1\",\"model\":\"deepseek\",\"status\":\"in_progress\"}}\n\n" +
			"event: response.reasoning_text.delta\ndata: {\"type\":\"response.reasoning_text.delta\",\"delta\":\"Step 1: check config. \"}\n\n" +
			"event: response.reasoning_summary_text.delta\ndata: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"Summary: checked config. \"}\n\n" +
			"event: response.function_call_arguments.delta\ndata: {\"type\":\"response.function_call_arguments.delta\",\"delta\":\"{\\\"ok\\\":true}\"}\n\n"
		stream.push([]byte(payload), nil)

		obs := stream.observation()
		require.Equal(t, "unknown", obs.ExecutionResult)
		require.Equal(t, "missing", obs.Usage.Status)

		extracted := tokenestimate.ResponseText(stream.estimateBody())
		require.Equal(t, `Step 1: check config. Summary: checked config. {"ok":true}`, extracted)
		estimated, err := tokenestimate.Count("deepseek-v4.1-flash-1017", extracted)
		require.NoError(t, err)
		require.Greater(t, estimated, int64(0))
	})

	t.Run("responses tolerates trailing ping after response.completed", func(t *testing.T) {
		stream := newManagedNativeStream(managedProtocolResponses, false, false)
		payload := "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"r1\",\"model\":\"deepseek\",\"status\":\"in_progress\"}}\n\n" +
			"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"r1\",\"model\":\"deepseek\",\"status\":\"completed\",\"usage\":{\"input_tokens\":100,\"output_tokens\":20,\"total_tokens\":120}}}\n\n" +
			"event: ping\ndata: {\"type\":\"ping\"}\n\n" +
			"data: [DONE]\n\n"
		stream.push([]byte(payload), nil)
		obs := stream.observation()
		require.Equal(t, "succeeded", obs.ExecutionResult)
		require.True(t, obs.BodyComplete)
		require.Equal(t, "reported", obs.Usage.Status)
		require.Equal(t, "100", obs.Usage.Quantities["input"])
		require.Equal(t, "20", obs.Usage.Quantities["output"])
	})
}
