package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
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
	require.True(t, hasManagedCacheControl(gjson.Parse(`{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":"out","cache_control":{"type":"ephemeral"}}]}]}]}`), managedProtocolMessages))
	require.True(t, hasManagedCacheControl(gjson.Parse(`{"input":[{"role":"user","content":[{"type":"input_text","text":"ctx","cache_control":{"type":"ephemeral"}}]}]}`), managedProtocolResponses))
	require.True(t, hasManagedCacheControl(gjson.Parse(`{"input":[{"role":"user","content":"ctx","cache_control":{"type":"ephemeral"}}]}`), managedProtocolResponses))
	require.False(t, hasManagedCacheControl(gjson.Parse(`{"input":"cache_control","tools":[{"type":"function","name":"f","parameters":{"properties":{"cache_control":{"type":"ephemeral"}}}}]}`), managedProtocolResponses))
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

func newQwenNativeProtocolHost(t *testing.T, protocol, transport string) *managedHost {
	t.Helper()
	fixture := managedConfigFixture()
	fixture["provider"] = json.RawMessage(`{"type":"qwen","apiTokens":["controlled-upstream-key"],"qwenEnableCompatible":true,"qwenEnableSearch":false}`)
	release := fixture["_modelfusion_execution"].(managedRelease)
	release.Descriptor.UpstreamModel = "qwen-plus"
	release.Descriptor.AdapterRef = "ai-proxy.native-text.v1:" + protocol + ":" + protocol
	release.Descriptor.Transport.PathAdapterRef = release.Descriptor.AdapterRef
	release.Descriptor.Transport.Host = "dashscope.aliyuncs.com"
	release.Descriptor.Transport.SNI = "dashscope.aliyuncs.com"
	descRaw, err := json.Marshal(release.Descriptor)
	require.NoError(t, err)
	release.Descriptor.DescriptorDigest, err = managedDescriptorDigest(descRaw)
	require.NoError(t, err)
	fixture["_modelfusion_execution"] = release

	settings := fixture["managed_execution"].(managedSettings)
	settings.Authority = "dashscope.aliyuncs.com"
	settings.InputProtocol = protocol
	settings.UpstreamProtocol = protocol
	if protocol == managedProtocolMessages {
		settings.UpstreamPath = "/apps/anthropic/v1/messages"
	} else {
		settings.UpstreamPath = "/compatible-mode/v1/responses"
	}
	settings.ResponseProfile = clientresponse.Qwen
	if transport == "router" {
		settings.SendTransport = "router"
		settings.StreamProjection = true
		settings.StreamProtocol = protocol
	}
	fixture["managed_execution"] = settings
	fixture["protocol_mode"] = "auto"

	h := newManagedHost(t, fixture)
	if transport == "router" {
		require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(release.Descriptor.ClusterRef)))
	}
	return h
}

func TestQwenNativeMessagesAndResponsesThinkingAndCacheMatrix(t *testing.T) {
	const reqID = "a8a7295d-0699-991f-849a-97f767160077"
	type cacheCase struct {
		name               string
		explicit           bool
		messagesStartUsage string
		messagesDeltaUsage string
		messagesJSONUsage  string
		responsesUsage     string
		wantQuantities     map[string]string
		wantAbsent         []string
		wantCacheEvidence  string
	}
	cacheCases := []cacheCase{
		{
			name:               "implicit_plain",
			explicit:           false,
			messagesStartUsage: `{"input_tokens":100,"output_tokens":0}`,
			messagesDeltaUsage: `{"input_tokens":100,"output_tokens":25}`,
			messagesJSONUsage:  `{"input_tokens":100,"output_tokens":25}`,
			responsesUsage:     `{"input_tokens":100,"output_tokens":25,"total_tokens":125}`,
			wantQuantities: map[string]string{
				"input":                    "100",
				"output":                   "25",
				"cache_read_mode":          "0",
				"cache_write_inapplicable": "1",
			},
			wantAbsent:        []string{"cache_write", "cache_write_5m", "cache_hit", "cache_miss"},
			wantCacheEvidence: "bailian-cache:implicit",
		},
		{
			name:               "implicit_hit_without_write_counter",
			explicit:           false,
			messagesStartUsage: `{"input_tokens":2867,"output_tokens":0,"cache_read_input_tokens":6350}`,
			messagesDeltaUsage: `{"input_tokens":2867,"output_tokens":25,"cache_read_input_tokens":6350}`,
			messagesJSONUsage:  `{"input_tokens":2867,"output_tokens":25,"cache_read_input_tokens":6350}`,
			responsesUsage:     `{"input_tokens":9217,"output_tokens":25,"total_tokens":9242,"input_tokens_details":{"cached_tokens":6350}}`,
			wantQuantities: map[string]string{
				"input":                    "9217",
				"output":                   "25",
				"cache_hit":                "6350",
				"cache_miss":               "2867",
				"cache_read_mode":          "0",
				"cache_write_inapplicable": "1",
			},
			wantAbsent:        []string{"cache_write", "cache_write_5m"},
			wantCacheEvidence: "bailian-cache:implicit",
		},
		{
			name:               "explicit_zero_write_repro_105",
			explicit:           true,
			messagesStartUsage: `{"input_tokens":2867,"output_tokens":0,"cache_read_input_tokens":6350,"cache_creation_input_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":0}}`,
			messagesDeltaUsage: `{"input_tokens":2867,"output_tokens":1023,"cache_read_input_tokens":6350,"cache_creation_input_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":0}}`,
			messagesJSONUsage:  `{"input_tokens":2867,"output_tokens":1023,"cache_read_input_tokens":6350,"cache_creation_input_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":0}}`,
			responsesUsage:     `{"input_tokens":9217,"output_tokens":1023,"total_tokens":10240,"input_tokens_details":{"cached_tokens":6350,"cache_type":"ephemeral","cache_creation_input_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":0}}}`,
			wantQuantities: map[string]string{
				"input":           "9217",
				"output":          "1023",
				"cache_hit":       "6350",
				"cache_miss":      "2867",
				"cache_write":     "0",
				"cache_write_5m":  "0",
				"cache_read_mode": "1",
			},
			wantAbsent:        []string{"cache_write_inapplicable"},
			wantCacheEvidence: "bailian-cache:explicit",
		},
		{
			name:               "explicit_nonzero_write",
			explicit:           true,
			messagesStartUsage: `{"input_tokens":300,"output_tokens":0,"cache_read_input_tokens":400,"cache_creation_input_tokens":300,"cache_creation":{"ephemeral_5m_input_tokens":300}}`,
			messagesDeltaUsage: `{"input_tokens":300,"output_tokens":40,"cache_read_input_tokens":400,"cache_creation_input_tokens":300,"cache_creation":{"ephemeral_5m_input_tokens":300}}`,
			messagesJSONUsage:  `{"input_tokens":300,"output_tokens":40,"cache_read_input_tokens":400,"cache_creation_input_tokens":300,"cache_creation":{"ephemeral_5m_input_tokens":300}}`,
			responsesUsage:     `{"input_tokens":1000,"output_tokens":40,"total_tokens":1040,"input_tokens_details":{"cached_tokens":400,"cache_type":"ephemeral","cache_creation_input_tokens":300,"cache_creation":{"ephemeral_5m_input_tokens":300}}}`,
			wantQuantities: map[string]string{
				"input":           "1000",
				"output":          "40",
				"cache_hit":       "400",
				"cache_miss":      "600",
				"cache_write":     "300",
				"cache_write_5m":  "300",
				"cache_read_mode": "1",
			},
			wantAbsent:        []string{"cache_write_inapplicable"},
			wantCacheEvidence: "bailian-cache:explicit",
		},
	}

	for _, protocol := range []string{managedProtocolMessages, managedProtocolResponses} {
		for _, transport := range []string{"router", "http_call"} {
			for _, stream := range []bool{false, true} {
				for _, thoughtMode := range []string{"disabled", "enabled", "variant"} {
					for _, cc := range cacheCases {
						name := fmt.Sprintf("%s/%s/stream=%t/thinking=%s/%s", protocol, transport, stream, thoughtMode, cc.name)
						t.Run(name, func(t *testing.T) {
							h := newQwenNativeProtocolHost(t, protocol, transport)
							var injected bytes.Buffer
							origInject := injectManagedRouterTerminal
							t.Cleanup(func() { injectManagedRouterTerminal = origInject })
							injectManagedRouterTerminal = func(body []byte, end bool) error {
								injected.Write(body)
								return nil
							}

							wantThinking := "0"
							wantThinkingEvidence := "bailian-thinking:disabled"
							if thoughtMode != "disabled" {
								wantThinking = "1"
								wantThinkingEvidence = "bailian-thinking:enabled"
							}

							cacheMarker := ""
							if cc.explicit {
								cacheMarker = `,"cache_control":{"type":"ephemeral"}`
							}

							var publicBody, upstreamPayload []byte
							if protocol == managedProtocolMessages {
								thinkingReq := `{"type":"disabled"}`
								if thoughtMode != "disabled" {
									thinkingReq = `{"type":"enabled","budget_tokens":1024}`
								}
								publicBody = []byte(fmt.Sprintf(`{"model":"public-model","stream":%t,"max_tokens":128,"thinking":%s,"messages":[{"role":"user","content":[{"type":"text","text":"Explain cache."%s}]}]}`, stream, thinkingReq, cacheMarker))
								if !stream {
									contentJSON := `[{"type":"text","text":"Direct answer."}]`
									switch thoughtMode {
									case "enabled":
										contentJSON = `[{"type":"thinking","thinking":"Step-by-step analysis.","signature":"sig_abc"},{"type":"text","text":"Direct answer."}]`
									case "variant":
										contentJSON = `[{"type":"redacted_thinking","data":"opaque-redacted"},{"type":"text","text":"Direct answer."}]`
									}
									upstreamPayload = []byte(fmt.Sprintf(`{"id":"msg_1","type":"message","role":"assistant","model":"qwen-plus","content":%s,"stop_reason":"end_turn","stop_sequence":null,"usage":%s}`, contentJSON, cc.messagesJSONUsage))
								} else {
									blocks := "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
										"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Direct answer.\"}}\n\n" +
										"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"
									switch thoughtMode {
									case "enabled":
										blocks = "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n" +
											"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"Step-by-step \"}}\n\n" +
											"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"signature_delta\",\"signature\":\"sig_abc\"}}\n\n" +
											"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
											"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
											"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"Direct answer.\"}}\n\n" +
											"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n"
									case "variant":
										blocks = "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"redacted_thinking\",\"data\":\"opaque-redacted\"}}\n\n" +
											"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
											"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
											"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"Direct answer.\"}}\n\n" +
											"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n"
									}
									upstreamPayload = []byte(fmt.Sprintf(
										"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"qwen-plus\",\"content\":[],\"stop_reason\":null,\"stop_sequence\":null,\"usage\":%s}}\n\n%s"+
											"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":%s}\n\n"+
											"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
										cc.messagesStartUsage, blocks, cc.messagesDeltaUsage))
								}
							} else {
								reasoningReq := `{"effort":"none"}`
								if thoughtMode != "disabled" {
									reasoningReq = `{"effort":"medium"}`
								}
								publicBody = []byte(fmt.Sprintf(`{"model":"public-model","stream":%t,"max_output_tokens":128,"reasoning":%s,"input":[{"role":"user","content":[{"type":"input_text","text":"Explain cache."%s}]}]}`, stream, reasoningReq, cacheMarker))
								respUsage := cc.responsesUsage
								if thoughtMode == "enabled" {
									respUsage = respUsage[:len(respUsage)-1] + `,"output_tokens_details":{"reasoning_tokens":12}}`
								} else if thoughtMode == "disabled" {
									respUsage = respUsage[:len(respUsage)-1] + `,"output_tokens_details":{"reasoning_tokens":0}}`
								}
								if !stream {
									outputJSON := `[{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Direct answer.","annotations":[]}]}]`
									switch thoughtMode {
									case "enabled":
										outputJSON = `[{"id":"rs_1","type":"reasoning","content":[{"type":"reasoning_text","text":"Step-by-step."}],"summary":[]},{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Direct answer.","annotations":[]}]}]`
									case "variant":
										outputJSON = `[{"id":"rs_1","type":"reasoning","encrypted_content":"opaque-enc","summary":[{"type":"summary_text","text":"Brief summary."}]},{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Direct answer.","annotations":[]}]}]`
									}
									upstreamPayload = []byte(fmt.Sprintf(`{"id":"resp_1","object":"response","status":"completed","model":"qwen-plus","output":%s,"usage":%s}`, outputJSON, respUsage))
								} else {
									events := ""
									completedOutput := `[{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Direct answer.","annotations":[]}]}]`
									switch thoughtMode {
									case "enabled":
										events = "event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"sequence_number\":1,\"output_index\":0,\"item\":{\"id\":\"rs_1\",\"type\":\"reasoning\",\"summary\":[]}}\n\n" +
											"event: response.reasoning_text.delta\ndata: {\"type\":\"response.reasoning_text.delta\",\"sequence_number\":2,\"item_id\":\"rs_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"Step-by-step.\"}\n\n" +
											"event: response.reasoning_text.done\ndata: {\"type\":\"response.reasoning_text.done\",\"sequence_number\":3,\"item_id\":\"rs_1\",\"output_index\":0,\"content_index\":0,\"text\":\"Step-by-step.\"}\n\n" +
											"event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"sequence_number\":4,\"output_index\":0,\"item\":{\"id\":\"rs_1\",\"type\":\"reasoning\",\"content\":[{\"type\":\"reasoning_text\",\"text\":\"Step-by-step.\"}],\"summary\":[]}}\n\n"
										completedOutput = `[{"id":"rs_1","type":"reasoning","content":[{"type":"reasoning_text","text":"Step-by-step."}],"summary":[]},{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Direct answer.","annotations":[]}]}]`
									case "variant":
										events = "event: response.reasoning_summary_part.added\ndata: {\"type\":\"response.reasoning_summary_part.added\",\"sequence_number\":1,\"item_id\":\"rs_1\",\"output_index\":0,\"summary_index\":0,\"part\":{\"type\":\"summary_text\",\"text\":\"\"}}\n\n" +
											"event: response.reasoning_summary_text.delta\ndata: {\"type\":\"response.reasoning_summary_text.delta\",\"sequence_number\":2,\"item_id\":\"rs_1\",\"output_index\":0,\"summary_index\":0,\"delta\":\"Brief summary.\"}\n\n" +
											"event: response.reasoning_summary_text.done\ndata: {\"type\":\"response.reasoning_summary_text.done\",\"sequence_number\":3,\"item_id\":\"rs_1\",\"output_index\":0,\"summary_index\":0,\"text\":\"Brief summary.\"}\n\n" +
											"event: response.reasoning_summary_part.done\ndata: {\"type\":\"response.reasoning_summary_part.done\",\"sequence_number\":4,\"item_id\":\"rs_1\",\"output_index\":0,\"summary_index\":0,\"part\":{\"type\":\"summary_text\",\"text\":\"Brief summary.\"}}\n\n"
									}
									events += "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"sequence_number\":5,\"item_id\":\"msg_1\",\"output_index\":1,\"content_index\":0,\"delta\":\"Direct answer.\"}\n\n"
									upstreamPayload = []byte(fmt.Sprintf(
										"event: response.created\ndata: {\"type\":\"response.created\",\"sequence_number\":0,\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"model\":\"qwen-plus\",\"status\":\"in_progress\",\"output\":[]}}\n\n%s"+
											"event: response.completed\ndata: {\"type\":\"response.completed\",\"sequence_number\":6,\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"model\":\"qwen-plus\",\"status\":\"completed\",\"output\":%s,\"usage\":%s}}\n\n"+
											"data: [DONE]\n\n",
										events, completedOutput, respUsage))
								}
							}

							mappedBody := bytes.Replace(publicBody, []byte(`"public-model"`), []byte(`"qwen-plus"`), 1)
							h.start(t, publicBody, mappedBody)

							contentType := "application/json"
							if stream {
								contentType = "text/event-stream"
							}
							respHeaders := [][2]string{
								{":status", "200"},
								{"content-type", contentType},
								{"x-request-id", reqID},
							}
							if transport == "router" {
								require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
								if stream {
									require.Equal(t, types.ActionContinue, h.host.CallOnResponseHeaders(h.id, respHeaders, false))
									mid := len(upstreamPayload) / 2
									h.host.CallOnResponseBody(h.id, upstreamPayload[:mid], false)
									h.host.CallOnResponseBody(h.id, upstreamPayload[mid:], true)
								} else {
									require.Equal(t, types.HeaderStopIteration, h.host.CallOnResponseHeaders(h.id, respHeaders, false))
									h.host.CallOnResponseBody(h.id, upstreamPayload, true)
								}
								drainDashScopeRedis(h)
							} else {
								h.response(t, respHeaders, upstreamPayload)
							}

							state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
							require.NotNil(t, state.lastAttempt)
							require.NotNil(t, state.lastDelivery)

							for _, fact := range []executionfacts.Envelope{state.lastAttempt.Envelope(), state.lastDelivery.Envelope()} {
								require.Equal(t, "reported", fact.Usage.Status)
								require.Equal(t, wantThinking, fact.Usage.Quantities["thinking_mode"])
								for key, want := range cc.wantQuantities {
									require.Equal(t, want, fact.Usage.Quantities[key], "key=%s", key)
								}
								for _, absent := range cc.wantAbsent {
									require.NotContains(t, fact.Usage.Quantities, absent, "unexpected meter=%s", absent)
								}
								if protocol == managedProtocolResponses && thoughtMode == "enabled" {
									require.Equal(t, "12", fact.Usage.Quantities["reasoning_tokens"])
								}
								require.True(t, slices.Contains(fact.EvidenceRefs, "bailian-request:"+reqID), "refs=%v", fact.EvidenceRefs)
								require.True(t, slices.Contains(fact.EvidenceRefs, wantThinkingEvidence), "refs=%v", fact.EvidenceRefs)
								require.True(t, slices.Contains(fact.EvidenceRefs, cc.wantCacheEvidence), "refs=%v", fact.EvidenceRefs)
							}
							h.host.CompleteHttpContext(h.id)
						})
					}
				}
			}
		}
	}
}

func TestQwenMessagesImplicitZeroWriteAndResponsesReasoningConflict(t *testing.T) {
	t.Run("messages_implicit_with_zero_write_counter_reports_implicit_read_mode", func(t *testing.T) {
		state := managedExecutionState{
			cacheMode: managedCacheImplicit,
			config: &managedExecutionConfig{
				profile: managedProtocolProfile{
					supplier:         "qwen",
					InputProtocol:    managedProtocolMessages,
					UpstreamProtocol: managedProtocolMessages,
				},
			},
		}
		headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
		body := []byte(`{"id":"msg_1","type":"message","role":"assistant","model":"qwen-plus","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":500,"output_tokens":20,"cache_read_input_tokens":1024,"cache_creation_input_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":0}}}`)
		outcome := managedResponseOutcome(&state, headers, nil, body, nil)
		require.Equal(t, "reported", outcome.Usage.Status)
		require.Equal(t, "1524", outcome.Usage.Quantities["input"])
		require.Equal(t, "1024", outcome.Usage.Quantities["cache_hit"])
		require.Equal(t, "500", outcome.Usage.Quantities["cache_miss"])
		require.Equal(t, "0", outcome.Usage.Quantities["cache_write"])
		require.Equal(t, "0", outcome.Usage.Quantities["cache_write_5m"])
		require.Equal(t, "0", outcome.Usage.Quantities["cache_read_mode"])
		require.Equal(t, "0", outcome.Usage.Quantities["thinking_mode"])
		require.NotContains(t, outcome.Usage.Quantities, "cache_write_inapplicable")
	})

	t.Run("responses_reasoning_output_with_zero_reasoning_tokens_leaves_thinking_mode_unset", func(t *testing.T) {
		state := managedExecutionState{
			cacheMode: managedCacheImplicit,
			config: &managedExecutionConfig{
				profile: managedProtocolProfile{
					supplier:         "qwen",
					InputProtocol:    managedProtocolResponses,
					UpstreamProtocol: managedProtocolResponses,
				},
			},
		}
		headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
		body := []byte(`{"id":"resp_1","object":"response","status":"completed","model":"qwen-plus","output":[{"id":"rs_1","type":"reasoning","content":[{"type":"reasoning_text","text":"evidence"}],"summary":[]},{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"answer","annotations":[]}]}],"usage":{"input_tokens":100,"output_tokens":20,"total_tokens":120,"output_tokens_details":{"reasoning_tokens":0}}}`)
		outcome := managedResponseOutcome(&state, headers, nil, body, nil)
		require.Equal(t, "reported", outcome.Usage.Status)
		require.Equal(t, "0", outcome.Usage.Quantities["cache_read_mode"])
		require.Equal(t, "1", outcome.Usage.Quantities["cache_write_inapplicable"])
		require.NotContains(t, outcome.Usage.Quantities, "thinking_mode")
	})

	t.Run("interrupted_native_streams_preserve_implicit_cache_without_inventing_thinking_mode", func(t *testing.T) {
		for _, tc := range []struct {
			protocol string
			public   string
			partial  string
		}{
			{
				managedProtocolMessages,
				`{"model":"public-model","stream":true,"max_tokens":128,"messages":[{"role":"user","content":"hello"}]}`,
				"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"m1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"qwen-plus\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":12,\"output_tokens\":0}}}\n\n" +
					"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n" +
					"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"half thought\"}}\n\n",
			},
			{
				managedProtocolResponses,
				`{"model":"public-model","stream":true,"max_output_tokens":128,"input":"hello"}`,
				"event: response.created\ndata: {\"type\":\"response.created\",\"sequence_number\":0,\"response\":{\"id\":\"r1\",\"object\":\"response\",\"model\":\"qwen-plus\",\"status\":\"in_progress\",\"output\":[]}}\n\n" +
					"event: response.reasoning_text.delta\ndata: {\"type\":\"response.reasoning_text.delta\",\"sequence_number\":1,\"delta\":\"half thought\"}\n\n",
			},
		} {
			t.Run(tc.protocol, func(t *testing.T) {
				h := newQwenNativeProtocolHost(t, tc.protocol, "router")
				mapped := bytes.Replace([]byte(tc.public), []byte(`"public-model"`), []byte(`"qwen-plus"`), 1)
				h.start(t, []byte(tc.public), mapped)
				require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
				require.Equal(t, types.ActionContinue, h.host.CallOnResponseHeaders(h.id, [][2]string{
					{":status", "200"},
					{"content-type", "text/event-stream"},
					{"x-request-id", "a8a7295d-0699-991f-849a-97f767160077"},
				}, false))
				h.host.CallOnResponseBody(h.id, []byte(tc.partial), true)
				drainDashScopeRedis(h)

				state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
				require.NotNil(t, state.lastDelivery)
				deliv := state.lastDelivery.Envelope()
				require.Equal(t, "estimated", deliv.Usage.Status)
				require.Equal(t, "0", deliv.Usage.Quantities["cache_write"])
				require.Equal(t, "0", deliv.Usage.Quantities["cache_read_mode"])
				require.NotContains(t, deliv.Usage.Quantities, "thinking_mode")
				require.Contains(t, deliv.EvidenceRefs, "bailian-cache:implicit")
				h.host.CompleteHttpContext(h.id)
			})
		}
	})
}
