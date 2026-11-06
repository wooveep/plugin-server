package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestN01ActualThinkingIgnoresTheRequestedMode(t *testing.T) {
	headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
	streamHeaders := [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}
	for _, tc := range []struct {
		name, requested, want string
		headers               [][2]string
		body                  string
	}{
		{"disabled", "false", "0", headers, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"answer"}}],"usage":{"prompt_tokens":1000,"completion_tokens":100}}`},
		{"requested but no reasoning", "true", "0", headers, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"answer"}}],"usage":{"prompt_tokens":1000,"completion_tokens":100,"completion_tokens_details":{"reasoning_tokens":0}}}`},
		{"actual thought", "true", "1", headers, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"answer","reasoning_content":"evidence"}}],"usage":{"prompt_tokens":1000,"completion_tokens":100,"completion_tokens_details":{"reasoning_tokens":60}}}`},
		{"conflict", "true", "", headers, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"answer","reasoning_content":"evidence"}}],"usage":{"prompt_tokens":1000,"completion_tokens":100,"completion_tokens_details":{"reasoning_tokens":0}}}`},
		{"SSE actual thought", "false", "1", streamHeaders, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"evidence\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":1000,\"completion_tokens\":100}}\n\ndata: [DONE]\n\n"},
		{"SSE incomplete", "true", "", streamHeaders, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"evidence\"},\"finish_reason\":null}]}\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := managedExecutionState{
				config:        &managedExecutionConfig{profile: managedProtocolProfile{supplier: "qwen", InputProtocol: managedProtocolChat, UpstreamProtocol: managedProtocolChat}},
				publicRequest: []byte(`{"enable_thinking":` + tc.requested + `}`),
			}
			outcome := managedResponseOutcome(&state, tc.headers, nil, []byte(tc.body), nil)
			require.Equal(t, tc.want, outcome.Usage.Quantities["thinking_mode"])
			if tc.want != "" {
				require.Equal(t, "100", outcome.Usage.Quantities["output"])
			}
		})
	}
}

func TestN02N03NormalizeProviderCacheWritesAndTTL(t *testing.T) {
	chat := func(raw string) map[string]string {
		return managedOpenAIUsage(map[string]json.RawMessage{"usage": json.RawMessage(raw)}).Quantities
	}
	for _, quantities := range []map[string]string{
		chat(`{"prompt_tokens":1000,"completion_tokens":100,"prompt_tokens_details":{"cached_tokens":400,"cache_type":"ephemeral","cache_creation_input_tokens":300,"cache_creation":{"ephemeral_5m_input_tokens":300}}}`),
		managedResponsesUsage(json.RawMessage(`{"input_tokens":1000,"output_tokens":100,"input_tokens_details":{"cached_tokens":400,"cache_write_tokens":300,"cache_write_tokens_details":{"ttl_30m_tokens":100,"ttl_12h_tokens":200}}}`)).Quantities,
		managedMessagesUsage(json.RawMessage(`{"input_tokens":300,"output_tokens":100,"cache_read_input_tokens":400,"cache_creation_input_tokens":300,"cache_creation":{"ephemeral_5m_input_tokens":100,"ephemeral_1h_input_tokens":200}}`)).Quantities,
		managedMessagesUsage(json.RawMessage(`{"input_tokens":300,"output_tokens":100,"cache_read_input_tokens":400,"cache_creation_input_tokens":300,"cache_write_tokens":300}`)).Quantities,
	} {
		for key, want := range map[string]string{"input": "1000", "output": "100", "cache_hit": "400", "cache_miss": "600", "cache_write": "300"} {
			require.Equal(t, want, quantities[key], key)
		}
	}
	for _, tc := range []struct{ detail, write, short, long string }{
		{`"cached_tokens":400`, "", "", ""},
		{`"cached_tokens":400,"cache_write_tokens":0`, "0", "", ""},
		{`"cached_tokens":0,"cache_write_tokens":300,"cache_write_tokens_details":{"ttl_30m_tokens":100,"ttl_12h_tokens":200}`, "300", "100", "200"},
		{`"cached_tokens":400,"cache_write_tokens":300,"cache_write_tokens_details":{"ttl_30m_tokens":100,"ttl_12h_tokens":201}`, "300", "", ""},
		{`"cached_tokens":400,"cache_write_tokens":601`, "", "", ""},
	} {
		quantities := managedResponsesUsage(json.RawMessage(`{"input_tokens":1000,"output_tokens":100,"input_tokens_details":{` + tc.detail + `}}`)).Quantities
		require.Equal(t, tc.write, quantities["cache_write"])
		require.Equal(t, tc.short, quantities["cache_write_30m"])
		require.Equal(t, tc.long, quantities["cache_write_12h"])
		require.Equal(t, "1000", quantities["input"])
	}
	require.True(t, hasManagedCacheControl(gjson.Parse(`{"messages":[{"content":[{"cache_\u0063ontrol":{"type":"ephemeral"}}]}]}`), managedProtocolChat))
	require.False(t, hasManagedCacheControl(gjson.Parse(`{"messages":[{"content":"cache_control"}]}`), managedProtocolChat))
	require.False(t, hasManagedCacheControl(gjson.Parse(`{"tools":[{"function":{"parameters":{"properties":{"cache_control":{"type":"ephemeral"}}}}}]}`), managedProtocolChat))
	require.True(t, hasManagedCacheControl(gjson.Parse(`{"cache_control":{"type":"ephemeral"}}`), managedProtocolMessages))
}

func TestCacheWriteWithoutReadCounter(t *testing.T) {
	for _, usage := range []*executionfacts.Usage{
		managedOpenAIUsage(map[string]json.RawMessage{"usage": json.RawMessage(`{"prompt_tokens":1000,"completion_tokens":100,"prompt_tokens_details":{"cache_creation_input_tokens":300}}`)}),
		managedResponsesUsage(json.RawMessage(`{"input_tokens":1000,"output_tokens":100,"input_tokens_details":{"cache_write_tokens":300}}`)),
		managedMessagesUsage(json.RawMessage(`{"input_tokens":700,"output_tokens":100,"cache_creation_input_tokens":300}`)),
	} {
		require.Equal(t, "reported", usage.Status)
		require.Equal(t, "1000", usage.Quantities["input"])
		require.Equal(t, "300", usage.Quantities["cache_write"])
	}
	readOnly := managedMessagesUsage(json.RawMessage(`{"input_tokens":700,"output_tokens":100,"cache_read_input_tokens":300}`))
	require.Equal(t, "reported", readOnly.Status)
	require.Equal(t, "1000", readOnly.Quantities["input"])
	require.Equal(t, "300", readOnly.Quantities["cache_hit"])
	require.NotContains(t, readOnly.Quantities, "cache_write")
}

func TestCacheWriteAliasesUseOneObservedQuantity(t *testing.T) {
	for _, tc := range []struct{ name, detail, write, short, long string }{
		{"same nonzero", `"cache_creation_input_tokens":300,"cache_write_tokens":300,"cache_creation":{"ttl_30m_tokens":100},"cache_write_tokens_details":{"ttl_30m_tokens":100,"ttl_12h_tokens":200}`, "300", "100", "200"},
		{"same zero", `"cache_creation_input_tokens":0,"cache_write_tokens":0`, "0", "", ""},
		{"null alias", `"cache_creation_input_tokens":null,"cache_write_tokens":300`, "300", "", ""},
		{"different totals", `"cache_creation_input_tokens":300,"cache_write_tokens":250`, "", "", ""},
		{"invalid alias", `"cache_creation_input_tokens":"300","cache_write_tokens":300`, "", "", ""},
		{"different TTL splits", `"cache_creation_input_tokens":300,"cache_write_tokens":300,"cache_creation":{"ttl_30m_tokens":300},"cache_write_tokens_details":{"ttl_30m_tokens":100,"ttl_12h_tokens":200}`, "300", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			details := `{"cached_tokens":400,` + tc.detail + `}`
			chat := json.RawMessage(`{"prompt_tokens":1000,"completion_tokens":100,"prompt_tokens_details":` + details + `}`)
			responses := json.RawMessage(`{"input_tokens":1000,"output_tokens":100,"input_tokens_details":` + details + `}`)
			for _, usage := range []*executionfacts.Usage{
				managedOpenAIUsage(map[string]json.RawMessage{"usage": chat}),
				managedResponsesUsage(responses),
			} {
				if tc.write == "" {
					require.Equal(t, "missing", usage.Status)
					require.Equal(t, "provider_cache_write_conflict_or_invalid", usage.ReasonCode)
					state := &managedExecutionState{config: &managedExecutionConfig{}, requestText: "cache prefix"}
					outcome := executionfacts.AttemptOutcome{SendState: "observed_sent", Usage: usage}
					require.Same(t, usage, availableManagedUsage(state, outcome, nil))
					continue
				}
				require.Equal(t, "reported", usage.Status)
				require.Equal(t, "600", usage.Quantities["cache_miss"])
				require.Equal(t, tc.write, usage.Quantities["cache_write"])
				require.Equal(t, tc.short, usage.Quantities["cache_write_30m"])
				require.Equal(t, tc.long, usage.Quantities["cache_write_12h"])
			}
		})
	}
}

func TestN04N05RetrievalKeepsTheWireBodyAndMeasuresOnlyInputs(t *testing.T) {
	for _, tc := range []struct{ protocol, model, usage, result, input, visual string }{
		{"dashscope_multimodal_embedding", "qwen3-vl-embedding", `{"input_tokens":300,"image_tokens":700,"total_tokens":1000}`, `"output":{"embeddings":[{"index":0,"embedding":[0.1,0.2]}]}`, "1000", "700"},
		{"dashscope_multimodal_embedding", "qwen2.5-vl-embedding", `{"input_tokens":300,"image_tokens":700}`, `"output":{"embeddings":[{"index":0,"embedding":[0.1]}]}`, "1000", "700"},
		{"dashscope_multimodal_embedding", "tongyi-embedding-vision-plus", `{"input_tokens":1000,"input_tokens_details":{"text_tokens":300,"image_tokens":700},"output_tokens":89,"total_tokens":1089}`, `"output":{"embeddings":[{"index":0,"embedding":[0.1]}]}`, "1000", "700"},
		{"dashscope_rerank", "qwen3-rerank", `{"total_tokens":1200}`, `"output":{"results":[{"index":0,"relevance_score":0.8}]}`, "1200", ""},
		{"bailian_rerank", "qwen3-rerank", `{"total_tokens":1200,"prompt_tokens":1200}`, `"results":[{"index":0,"relevance_score":0.8},{"index":1,"relevance_score":0.5}]`, "1200", ""},
		{"dashscope_rerank", "qwen3.7-text-rerank", `{"total_tokens":45,"prompt_tokens":45}`, `"output":{"results":[{"index":0,"relevance_score":0.8}]}`, "45", ""},
		{"bailian_rerank", "qwen3.7-text-rerank", `{"total_tokens":45,"prompt_tokens":45}`, `"results":[{"index":0,"relevance_score":0.8}]`, "45", ""},
	} {
		t.Run(tc.protocol+"/"+tc.model, func(t *testing.T) {
			require.True(t, managedRetrievalModel(tc.protocol, tc.model))
			profile, err := newManagedProtocolProfile(tc.protocol, tc.protocol)
			require.NoError(t, err)
			body := []byte(`{ "model" : "` + tc.model + `", "input":{"contents":[]},"parameters": {"dimension":128,"top_n":1} }`)
			request, err := profile.PrepareRequest(body, tc.model, "test-key")
			require.NoError(t, err)
			require.True(t, bytes.Equal(body, request.Body))
			reply := []byte(`{` + tc.result + `,"usage":` + tc.usage + `}`)
			outcome := managedRetrievalObservation(tc.protocol, tc.model, 200, [][2]string{{"content-type", "application/json"}}, reply, true)
			require.Equal(t, "succeeded", outcome.ExecutionResult)
			require.Equal(t, "reported", outcome.Usage.Status)
			require.Equal(t, tc.input, outcome.Usage.Quantities["input"])
			require.Equal(t, tc.visual, outcome.Usage.Quantities["visual_input"])
			require.NotContains(t, outcome.Usage.Quantities, "output")
		})
	}
	require.False(t, managedRetrievalModel("bailian_rerank", "qwen3-vl-rerank"))
}
