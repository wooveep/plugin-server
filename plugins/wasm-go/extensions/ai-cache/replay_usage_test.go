package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNativeReplayEstimatesCurrentInputAndCachedOutputForEveryProtocol(t *testing.T) {
	for _, tc := range []struct {
		kind                  protocol.ProtocolKind
		response              string
		chunks                []string
		inputPath, outputPath string
	}{
		{protocol.ProtocolChatCompletions, `{"choices":[{"message":{"content":"cached"}}],"usage":{"prompt_tokens":9999,"completion_tokens":4444}}`, []string{`{"choices":[{"delta":{"content":"cached"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9999,"completion_tokens":4444}}`}, "prompt_tokens", "completion_tokens"},
		{protocol.ProtocolMessages, `{"content":[{"type":"text","text":"cached"}],"usage":{"input_tokens":9999,"output_tokens":4444}}`, []string{`{"type":"message_start","message":{"content":[],"usage":{"input_tokens":9999,"output_tokens":0,"cache_read_input_tokens":9999}}}`, `{"type":"content_block_delta","delta":{"type":"text_delta","text":"cached"}}`, `{"type":"message_delta","usage":{"output_tokens":4444}}`}, "input_tokens", "output_tokens"},
		{protocol.ProtocolResponses, `{"output":[{"type":"message","content":[{"type":"output_text","text":"cached"}]}],"usage":{"input_tokens":9999,"output_tokens":4444}}`, []string{`{"type":"response.created","response":{"usage":{"input_tokens":9999,"output_tokens":0}}}`, `{"type":"response.output_text.delta","delta":"cached"}`, `{"type":"response.output_text.done","text":"cached"}`, `{"type":"response.completed","response":{"output":[{"type":"message","content":[{"type":"output_text","text":"cached"}]}],"usage":{"input_tokens":9999,"output_tokens":4444}}}`}, "input_tokens", "output_tokens"},
	} {
		for _, stream := range []bool{false, true} {
			t.Run(string(tc.kind)+map[bool]string{false: "/json", true: "/sse"}[stream], func(t *testing.T) {
				state := &nativeCacheDeliveryState{config: &nativeCacheDeliveryConfig{tokenizerModel: "gpt-4o"}, requestText: "hello"}
				record := MaterializedReplayRecord{Protocol: tc.kind, Response: json.RawMessage(tc.response), Usage: json.RawMessage(`{}`)}
				for _, raw := range tc.chunks {
					record.StreamChunks = append(record.StreamChunks, json.RawMessage(raw))
				}
				original, err := json.Marshal(record)
				require.NoError(t, err)
				usage, payload, err := estimateNativeCacheReplay(state, record, thinReplayPayload(tc.kind, record, stream))
				require.NoError(t, err)
				require.Equal(t, "estimated", usage.Status)
				require.Equal(t, "estimator", usage.Source)
				require.Equal(t, map[string]string{"input": "1", "output": "1", "cache_hit": "0", "cache_miss": "1"}, usage.Quantities)
				after, err := json.Marshal(record)
				require.NoError(t, err)
				require.Equal(t, original, after, "stored response and historical provider meter must stay immutable")
				if !stream {
					require.Equal(t, int64(1), gjson.GetBytes(payload.Body, "usage."+tc.inputPath).Int())
					require.Equal(t, int64(1), gjson.GetBytes(payload.Body, "usage."+tc.outputPath).Int())
					return
				}
				switch tc.kind {
				case protocol.ProtocolChatCompletions:
					require.Equal(t, int64(1), gjson.GetBytes(payload.StreamChunks[0].Data, "usage.prompt_tokens").Int())
					require.Equal(t, int64(1), gjson.GetBytes(payload.StreamChunks[0].Data, "usage.completion_tokens").Int())
				case protocol.ProtocolMessages:
					require.Equal(t, int64(1), gjson.GetBytes(payload.StreamChunks[0].Data, "message.usage.input_tokens").Int())
					require.Zero(t, gjson.GetBytes(payload.StreamChunks[0].Data, "message.usage.cache_read_input_tokens").Int())
					require.Equal(t, int64(1), gjson.GetBytes(payload.StreamChunks[2].Data, "usage.output_tokens").Int())
				case protocol.ProtocolResponses:
					last := payload.StreamChunks[len(payload.StreamChunks)-1]
					require.Equal(t, int64(1), gjson.GetBytes(last.Data, "response.usage.input_tokens").Int())
					require.Equal(t, int64(1), gjson.GetBytes(last.Data, "response.usage.output_tokens").Int())
				}
			})
		}
	}
}
