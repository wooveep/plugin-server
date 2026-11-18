package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newDashScopeConversionHost(t *testing.T, inputProtocol, upstreamProtocol string, upstreamModel string) *managedHost {
	t.Helper()
	fixture := managedConfigFixture()
	fixture["provider"] = json.RawMessage(`{"type":"qwen","apiTokens":["controlled-upstream-key"],"qwenEnableCompatible":false,"qwenEnableSearch":false}`)
	release := fixture["_modelfusion_execution"].(managedRelease)
	release.Descriptor.UpstreamModel = upstreamModel
	release.Descriptor.AdapterRef = "ai-proxy.native-text.v1:" + inputProtocol + ":" + upstreamProtocol
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
	settings.InputProtocol = inputProtocol
	settings.UpstreamProtocol = upstreamProtocol
	settings.UpstreamPath = "/api/v1/services/aigc/text-generation/generation"
	if upstreamProtocol == managedProtocolDashScopeMultimodal {
		settings.UpstreamPath = "/api/v1/services/aigc/multimodal-generation/generation"
	}
	settings.SendTransport = "router"
	settings.StreamProjection = true
	if inputProtocol != managedProtocolChat {
		settings.StreamProtocol = inputProtocol
	}
	settings.ResponseProfile = clientresponse.Qwen
	fixture["managed_execution"] = settings
	fixture["protocol_mode"] = "auto"

	h := newManagedHost(t, fixture)
	require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(release.Descriptor.ClusterRef)))
	return h
}

func TestDashScopeCrossProtocolSixCellMultiTurnMatrix(t *testing.T) {
	upstreams := []struct {
		protocol string
		path     string
		model    string
	}{
		{managedProtocolDashScopeText, "/api/v1/services/aigc/text-generation/generation", "qwen-flash"},
		{managedProtocolDashScopeMultimodal, "/api/v1/services/aigc/multimodal-generation/generation", "qwen3.6-plus"},
	}
	inputs := []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages}

	for _, up := range upstreams {
		for _, in := range inputs {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s->%s/stream=%t/turn1", in, up.protocol, stream), func(t *testing.T) {
					// --- Turn 1: Request with system/developer + tools + thinking + max tokens -> Tool Call response ---
					h1 := newDashScopeConversionHost(t, in, up.protocol, up.model)
					var turn1Public []byte
					switch in {
					case managedProtocolChat:
						turn1Public = []byte(fmt.Sprintf(`{
							"model":"public-model",
							"stream":%t,
							"stream_options":{"include_usage":true},
							"max_completion_tokens":128,
							"enable_thinking":true,
							"thinking_budget":1024,
							"messages":[
								{"role":"system","content":"Be concise."},
								{"role":"user","content":"Weather in Hangzhou?"}
							],
							"tools":[{"type":"function","function":{"name":"get_weather","description":"Get weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}],
							"tool_choice":"auto"
						}`, stream))
					case managedProtocolResponses:
						turn1Public = []byte(fmt.Sprintf(`{
							"model":"public-model",
							"stream":%t,
							"instructions":"Be concise.",
							"max_output_tokens":128,
							"reasoning":{"effort":"low"},
							"input":[
								{"role":"developer","content":"Use metric units."},
								{"role":"user","content":[{"type":"input_text","text":"Weather in Hangzhou?"}]}
							],
							"tools":[{"type":"function","name":"get_weather","description":"Get weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}],
							"tool_choice":"auto"
						}`, stream))
					case managedProtocolMessages:
						turn1Public = []byte(fmt.Sprintf(`{
							"model":"public-model",
							"stream":%t,
							"max_tokens":128,
							"thinking":{"type":"enabled","budget_tokens":2048},
							"system":[{"type":"text","text":"Be concise."}],
							"messages":[
								{"role":"user","content":[{"type":"text","text":"Weather in Hangzhou?"}]}
							],
							"tools":[{"name":"get_weather","description":"Get weather","input_schema":{"type":"object","properties":{"city":{"type":"string"}}}}],
							"tool_choice":{"type":"auto"}
						}`, stream))
					}
					turn1Mapped := bytes.Replace(turn1Public, []byte(`"public-model"`), []byte(`"`+up.model+`"`), 1)
					h1.start(t, turn1Public, turn1Mapped)

					reqHeaders := h1.host.GetCurrentRequestHeaders(h1.id)
					gotPath, err := managedSingleHeader(reqHeaders, ":path")
					require.NoError(t, err)
					require.Equal(t, up.path, gotPath)
					sseHeader, sseErr := managedSingleHeader(reqHeaders, "x-dashscope-sse")
					acceptHeader, acceptErr := managedSingleHeader(reqHeaders, "accept")
					require.NoError(t, acceptErr)
					if stream {
						require.NoError(t, sseErr)
						require.Equal(t, "enable", sseHeader)
						require.Equal(t, "text/event-stream", acceptHeader)
					} else {
						require.Error(t, sseErr)
						require.Equal(t, "application/json", acceptHeader)
					}

					outbound1 := h1.host.GetCurrentRequestBody(h1.id)
					require.Equal(t, up.model, gjson.GetBytes(outbound1, "model").String())
					require.Equal(t, int64(128), gjson.GetBytes(outbound1, "parameters.max_tokens").Int())
					require.True(t, gjson.GetBytes(outbound1, "parameters.enable_thinking").Bool(), string(outbound1))
					require.False(t, gjson.GetBytes(outbound1, "parameters.top_p").Exists(), "omitted top_p must not clamp to 0.000001")
					require.Equal(t, "get_weather", gjson.GetBytes(outbound1, "parameters.tools.0.function.name").String())
					require.Equal(t, "auto", gjson.GetBytes(outbound1, "parameters.tool_choice").String())
					for _, msg := range gjson.GetBytes(outbound1, "input.messages").Array() {
						require.NotEqual(t, "developer", msg.Get("role").String(), "developer role must be normalized to system for DashScope")
						if up.protocol == managedProtocolDashScopeMultimodal && (msg.Get("role").String() == "system" || msg.Get("role").String() == "user") {
							require.True(t, msg.Get("content").IsArray(), "multimodal message content should be array: %s", msg.Raw)
						}
						if up.protocol == managedProtocolDashScopeText && msg.Get("role").String() == "system" {
							require.Equal(t, gjson.String, msg.Get("content").Type, "text-generation system content must be string: %s", msg.Raw)
						}
					}

					var injected1 bytes.Buffer
					origInject1 := injectManagedRouterTerminal
					t.Cleanup(func() { injectManagedRouterTerminal = origInject1 })
					injectManagedRouterTerminal = func(body []byte, end bool) error {
						injected1.Write(body)
						return nil
					}

					require.NoError(t, h1.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
					if stream {
						require.Equal(t, types.ActionContinue, h1.host.CallOnResponseHeaders(h1.id, [][2]string{
							{":status", "200"},
							{"content-type", "text/event-stream"},
							{"x-request-id", "a8a7295d-0699-991f-849a-97f767160077"},
						}, false))
						// Cumulative (incremental_output=false) DashScope SSE frames with reasoning + split tool_calls arguments.
						sseTurn1 := "event: result\ndata: " + `{"request_id":"a8a7295d-0699-991f-849a-97f767160077","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","reasoning_content":"Checking Hangzhou weather..."}}]},"usage":{"input_tokens":84,"output_tokens":5,"total_tokens":89}}` + "\n\n" +
							"event: result\ndata: " + `{"request_id":"a8a7295d-0699-991f-849a-97f767160077","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","reasoning_content":"Checking Hangzhou weather...","tool_calls":[{"index":0,"id":"call_hz_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":"}}]}}]},"usage":{"input_tokens":84,"output_tokens":12,"total_tokens":96}}` + "\n\n" +
							"event: result\ndata: " + `{"request_id":"a8a7295d-0699-991f-849a-97f767160077","output":{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","reasoning_content":"Checking Hangzhou weather...","tool_calls":[{"index":0,"id":"call_hz_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Hangzhou\"}"}}]}}]},"usage":{"input_tokens":84,"output_tokens":18,"total_tokens":102,"prompt_tokens_details":{"cached_tokens":32},"output_tokens_details":{"reasoning_tokens":6}}}` + "\n\n"
						h1.host.CallOnResponseBody(h1.id, []byte(sseTurn1), true)
					} else {
						require.Equal(t, types.HeaderStopIteration, h1.host.CallOnResponseHeaders(h1.id, [][2]string{
							{":status", "200"},
							{"content-type", "application/json"},
							{"x-request-id", "a8a7295d-0699-991f-849a-97f767160077"},
						}, false))
						jsonTurn1 := `{"request_id":"a8a7295d-0699-991f-849a-97f767160077","output":{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","reasoning_content":"Checking Hangzhou weather...","tool_calls":[{"index":0,"id":"call_hz_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Hangzhou\"}"}}]}}]},"usage":{"input_tokens":84,"output_tokens":18,"total_tokens":102,"prompt_tokens_details":{"cached_tokens":32},"output_tokens_details":{"reasoning_tokens":6}}}`
						h1.host.CallOnResponseBody(h1.id, []byte(jsonTurn1), true)
					}
					drainDashScopeRedis(h1)

					resp1 := string(h1.host.GetCurrentResponseBody(h1.id)) + injected1.String()
					require.NotContains(t, resp1, "response_incomplete")
					require.NotContains(t, resp1, "event: result")
					switch in {
					case managedProtocolChat:
						if stream {
							require.Contains(t, resp1, "data: [DONE]\n\n")
							require.Equal(t, 1, strings.Count(resp1, `"id":"call_pf_`), "tool_call id must only appear on initial delta chunk: %s", resp1)
							require.Contains(t, resp1, `"prompt_tokens":84`)
						} else {
							require.True(t, strings.HasPrefix(gjson.Get(resp1, "choices.0.message.tool_calls.0.id").String(), "call_pf_"), resp1)
							require.Equal(t, `{"city":"Hangzhou"}`, gjson.Get(resp1, "choices.0.message.tool_calls.0.function.arguments").String())
							require.Equal(t, "Checking Hangzhou weather...", gjson.Get(resp1, "choices.0.message.reasoning_content").String())
							require.Equal(t, int64(84), gjson.Get(resp1, "usage.prompt_tokens").Int())
							require.Equal(t, int64(32), gjson.Get(resp1, "usage.prompt_tokens_details.cached_tokens").Int())
						}
					case managedProtocolResponses:
						if stream {
							require.Contains(t, resp1, `"type":"response.reasoning_text.delta"`)
							require.Contains(t, resp1, `"type":"response.function_call_arguments.delta"`)
							require.Contains(t, resp1, `"type":"response.completed"`)
							require.Contains(t, resp1, `"call_id":"call_pf_`)
						} else {
							require.Equal(t, "completed", gjson.Get(resp1, "status").String())
							require.Equal(t, "reasoning", gjson.Get(resp1, "output.0.type").String())
							require.Equal(t, "function_call", gjson.Get(resp1, "output.1.type").String())
							require.True(t, strings.HasPrefix(gjson.Get(resp1, "output.1.call_id").String(), "call_pf_"), resp1)
							require.Equal(t, `{"city":"Hangzhou"}`, gjson.Get(resp1, "output.1.arguments").String())
							require.Equal(t, int64(84), gjson.Get(resp1, "usage.input_tokens").Int())
						}
					case managedProtocolMessages:
						if stream {
							require.Contains(t, resp1, `"type":"thinking_delta"`)
							require.Contains(t, resp1, `"type":"input_json_delta"`)
							require.Contains(t, resp1, `"stop_reason":"tool_use"`)
							require.Contains(t, resp1, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
						} else {
							require.Equal(t, "tool_use", gjson.Get(resp1, "stop_reason").String())
							require.Equal(t, "thinking", gjson.Get(resp1, "content.0.type").String())
							require.Equal(t, "tool_use", gjson.Get(resp1, "content.1.type").String())
							require.True(t, strings.HasPrefix(gjson.Get(resp1, "content.1.id").String(), "call_pf_"), resp1)
							require.Equal(t, "Hangzhou", gjson.Get(resp1, "content.1.input.city").String())
						}
					}

					state1 := h1.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
					require.NotNil(t, state1.lastDelivery)
					deliv1 := state1.lastDelivery.Envelope()
					require.Equal(t, "succeeded", deliv1.DeliveryState)
					require.Equal(t, "reported", deliv1.Usage.Status)
					require.Equal(t, "84", deliv1.Usage.Quantities["input"])
					require.Equal(t, "18", deliv1.Usage.Quantities["output"])
					require.Equal(t, "32", deliv1.Usage.Quantities["cache_hit"])
					require.Equal(t, "6", deliv1.Usage.Quantities["reasoning_tokens"])
					h1.host.CompleteHttpContext(h1.id)
				})

				t.Run(fmt.Sprintf("%s->%s/stream=%t/turn2", in, up.protocol, stream), func(t *testing.T) {
					// --- Turn 2: Assistant tool_calls + reasoning history + Tool output -> Final Text response ---
					h2 := newDashScopeConversionHost(t, in, up.protocol, up.model)
					var turn2Public []byte
					switch in {
					case managedProtocolChat:
						turn2Public = []byte(fmt.Sprintf(`{
							"model":"public-model",
							"stream":%t,
							"messages":[
								{"role":"user","content":"Weather in Hangzhou?"},
								{"role":"assistant","reasoning_content":"Checking Hangzhou weather...","tool_calls":[{"id":"call_hz_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Hangzhou\"}"}}]},
								{"role":"tool","tool_call_id":"call_hz_1","content":"Sunny, 24C"}
							],
							"tools":[{"type":"function","function":{"name":"get_weather","description":"Get weather","parameters":{"type":"object"}}}]
						}`, stream))
					case managedProtocolResponses:
						turn2Public = []byte(fmt.Sprintf(`{
							"model":"public-model",
							"stream":%t,
							"input":[
								{"role":"user","content":"Weather in Hangzhou?"},
								{"type":"reasoning","content":[{"type":"reasoning_text","text":"Checking Hangzhou weather..."}]},
								{"type":"function_call","call_id":"call_hz_1","name":"get_weather","arguments":"{\"city\":\"Hangzhou\"}"},
								{"type":"function_call_output","call_id":"call_hz_1","output":[{"type":"input_text","text":"Sunny, 24C"}]}
							],
							"tools":[{"type":"function","name":"get_weather","description":"Get weather","parameters":{"type":"object"}}]
						}`, stream))
					case managedProtocolMessages:
						turn2Public = []byte(fmt.Sprintf(`{
							"model":"public-model",
							"stream":%t,
							"max_tokens":128,
							"messages":[
								{"role":"user","content":"Weather in Hangzhou?"},
								{"role":"assistant","content":[
									{"type":"thinking","thinking":"Checking Hangzhou weather...","signature":"sig_1"},
									{"type":"tool_use","id":"call_hz_1","name":"get_weather","input":{"city":"Hangzhou"}}
								]},
								{"role":"user","content":[
									{"type":"tool_result","tool_use_id":"call_hz_1","content":"Sunny, 24C"}
								]}
							],
							"tools":[{"name":"get_weather","description":"Get weather","input_schema":{"type":"object"}}]
						}`, stream))
					}
					turn2Mapped := bytes.Replace(turn2Public, []byte(`"public-model"`), []byte(`"`+up.model+`"`), 1)
					h2.start(t, turn2Public, turn2Mapped)

					outbound2 := h2.host.GetCurrentRequestBody(h2.id)
					require.Equal(t, "assistant", gjson.GetBytes(outbound2, "input.messages.1.role").String())
					require.Equal(t, "Checking Hangzhou weather...", gjson.GetBytes(outbound2, "input.messages.1.reasoning_content").String())
					require.Equal(t, "call_hz_1", gjson.GetBytes(outbound2, "input.messages.1.tool_calls.0.id").String())
					require.Equal(t, gjson.String, gjson.GetBytes(outbound2, "input.messages.1.content").Type, "assistant tool_calls message content must be string, never null: %s", string(outbound2))
					require.Equal(t, "tool", gjson.GetBytes(outbound2, "input.messages.2.role").String())
					require.Equal(t, "call_hz_1", gjson.GetBytes(outbound2, "input.messages.2.tool_call_id").String())
					require.Equal(t, gjson.String, gjson.GetBytes(outbound2, "input.messages.2.content").Type)
					require.Equal(t, "Sunny, 24C", gjson.GetBytes(outbound2, "input.messages.2.content").String())
					if up.model == "qwen3.6-plus" {
						require.True(t, gjson.GetBytes(outbound2, "parameters.preserve_thinking").Bool())
					}

					contentPayload := `"It is sunny and 24C in Hangzhou."`
					contentDelta1 := `"It is sunny "`
					if up.protocol == managedProtocolDashScopeMultimodal {
						contentPayload = `[{"text":"It is sunny and 24C in Hangzhou."}]`
						contentDelta1 = `[{"text":"It is sunny "}]`
					}

					var injected2 bytes.Buffer
					origInject2 := injectManagedRouterTerminal
					t.Cleanup(func() { injectManagedRouterTerminal = origInject2 })
					injectManagedRouterTerminal = func(body []byte, end bool) error {
						injected2.Write(body)
						return nil
					}

					require.NoError(t, h2.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
					if stream {
						require.Equal(t, types.ActionContinue, h2.host.CallOnResponseHeaders(h2.id, [][2]string{
							{":status", "200"},
							{"content-type", "text/event-stream"},
						}, false))
						sseTurn2 := "event: result\ndata: " + `{"request_id":"b9b8396e-0699-991f-849a-97f767160088","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":` + contentDelta1 + `}}]},"usage":{"input_tokens":110,"output_tokens":4,"total_tokens":114}}` + "\n\n" +
							"event: result\ndata: " + `{"request_id":"b9b8396e-0699-991f-849a-97f767160088","output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":` + contentPayload + `}}]},"usage":{"input_tokens":110,"output_tokens":9,"total_tokens":119}}` + "\n\n"
						h2.host.CallOnResponseBody(h2.id, []byte(sseTurn2), true)
					} else {
						require.Equal(t, types.HeaderStopIteration, h2.host.CallOnResponseHeaders(h2.id, [][2]string{
							{":status", "200"},
							{"content-type", "application/json"},
						}, false))
						jsonTurn2 := `{"request_id":"b9b8396e-0699-991f-849a-97f767160088","output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":` + contentPayload + `}}]},"usage":{"input_tokens":110,"output_tokens":9,"total_tokens":119}}`
						h2.host.CallOnResponseBody(h2.id, []byte(jsonTurn2), true)
					}
					drainDashScopeRedis(h2)

					resp2 := string(h2.host.GetCurrentResponseBody(h2.id)) + injected2.String()
					require.NotContains(t, resp2, "response_incomplete")
					require.Contains(t, resp2, "24C in Hangzhou.")
					state2 := h2.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
					require.NotNil(t, state2.lastDelivery)
					require.Equal(t, "succeeded", state2.lastDelivery.Envelope().DeliveryState)
					require.Equal(t, "110", state2.lastDelivery.Envelope().Usage.Quantities["input"])
					require.Equal(t, "9", state2.lastDelivery.Envelope().Usage.Quantities["output"])
					h2.host.CompleteHttpContext(h2.id)
				})
			}
		}
	}
}

func TestResponsesToChatMapsDeveloperRoleToSystemForNonOpenAISuppliers(t *testing.T) {
	body := []byte(`{
		"model":"frozen-model",
		"instructions":"Follow platform policy.",
		"input":[
			{"role":"developer","content":"Respond in JSON."},
			{"role":"user","content":"ping"}
		]
	}`)
	for _, tc := range []struct {
		supplier string
		wantRole string
	}{
		{"qwen", "system"},
		{"deepseek", "system"},
		{"zhipuai", "system"},
		{"openai", "developer"},
		{"azure", "developer"},
	} {
		t.Run(tc.supplier, func(t *testing.T) {
			p, err := newManagedProtocolProfile(managedProtocolResponses, managedProtocolChat)
			require.NoError(t, err)
			p.supplier = tc.supplier
			req, err := p.PrepareRequest(body, "frozen-model", "fixture-key")
			require.NoError(t, err)
			require.Equal(t, tc.wantRole, gjson.GetBytes(req.Body, "messages.0.role").String())
			require.Equal(t, tc.wantRole, gjson.GetBytes(req.Body, "messages.1.role").String())
			require.Equal(t, "user", gjson.GetBytes(req.Body, "messages.2.role").String())
		})
	}
}

func TestDashScopeCrossProtocolErrorClassificationAndPathGuards(t *testing.T) {
	// Path/supplier mismatch must fail closed at config load.
	qwenCfg := managedConfigFixture()
	qwenCfg["provider"] = json.RawMessage(`{"type":"qwen","apiTokens":["controlled-upstream-key"]}`)
	badSettings := qwenCfg["managed_execution"].(managedSettings)
	badSettings.InputProtocol = managedProtocolChat
	badSettings.UpstreamProtocol = managedProtocolDashScopeText
	badSettings.UpstreamPath = "/api/v1/services/aigc/multimodal-generation/generation"
	qwenCfg["managed_execution"] = badSettings
	rawBad, err := json.Marshal(qwenCfg)
	require.NoError(t, err)
	var parsedCfg managedExecutionConfig
	require.Error(t, newManagedSupplierAdapterErrorCheck(rawBad, &parsedCfg))

	// HTTP 429 Throttling.RateQuota and SSE DataInspectionFailed must classify cleanly across all 3 client protocols.
	for _, in := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
		t.Run("http_429/"+in, func(t *testing.T) {
			h := newDashScopeConversionHost(t, in, managedProtocolDashScopeText, "qwen-flash")
			pub := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hi"}]}`)
			if in == managedProtocolResponses {
				pub = []byte(`{"model":"public-model","input":"hi"}`)
			} else if in == managedProtocolMessages {
				pub = []byte(`{"model":"public-model","max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`)
			}
			h.start(t, pub, bytes.Replace(pub, []byte("public-model"), []byte("qwen-flash"), 1))
			require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
			require.Equal(t, types.HeaderStopIteration, h.host.CallOnResponseHeaders(h.id, [][2]string{
				{":status", "429"},
				{"content-type", "application/json"},
			}, false))
			dashErr := []byte(`{"code":"Throttling.RateQuota","message":"Requests rate limit exceeded","request_id":"c1c2c3c4-0699-991f-849a-97f767160099"}`)
			require.Equal(t, types.ActionContinue, h.host.CallOnResponseBody(h.id, dashErr, true))
			respBody := string(h.host.GetCurrentResponseBody(h.id))
			require.Equal(t, "rate_limit_exceeded", gjson.Get(respBody, "error.code").String(), respBody)
			require.NotContains(t, respBody, "Throttling.RateQuota")
			h.host.CompleteHttpContext(h.id)
		})

		t.Run("sse_policy_error/"+in, func(t *testing.T) {
			h := newDashScopeConversionHost(t, in, managedProtocolDashScopeText, "qwen-flash")
			pub := []byte(`{"model":"public-model","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
			if in == managedProtocolResponses {
				pub = []byte(`{"model":"public-model","stream":true,"input":"hi"}`)
			} else if in == managedProtocolMessages {
				pub = []byte(`{"model":"public-model","stream":true,"max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`)
			}
			h.start(t, pub, bytes.Replace(pub, []byte("public-model"), []byte("qwen-flash"), 1))
			require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
			require.Equal(t, types.ActionContinue, h.host.CallOnResponseHeaders(h.id, [][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			}, false))
			sseErr := "event: error\ndata: {\"code\":\"DataInspectionFailed\",\"message\":\"Input data may contain inappropriate content.\",\"request_id\":\"d1d2d3d4-0699-991f-849a-97f767160099\"}\n\n"
			h.host.CallOnResponseBody(h.id, []byte(sseErr), true)
			respBody := string(h.host.GetCurrentResponseBody(h.id))
			require.Contains(t, respBody, "content_policy_violation", respBody)
			require.NotContains(t, respBody, "DataInspectionFailed")
			require.NotContains(t, respBody, "response_incomplete")
			h.host.CompleteHttpContext(h.id)
		})
	}
}

func drainDashScopeRedis(h *managedHost) {
	seen := map[uint32]bool{}
	for {
		progressed := false
		for _, call := range h.host.GetRedisCalloutAttributesFromContext(h.id) {
			if seen[call.CalloutID] {
				continue
			}
			seen[call.CalloutID] = true
			progressed = true
			h.host.CallOnRedisCallResponse(call.CalloutID, 0, []byte(":1\r\n"))
		}
		if !progressed {
			return
		}
	}
}

func TestDashScopeCrossProtocolBufferedHttpCallConversion(t *testing.T) {
	for _, up := range []struct {
		protocol string
		path     string
	}{
		{managedProtocolDashScopeText, "/api/v1/services/aigc/text-generation/generation"},
		{managedProtocolDashScopeMultimodal, "/api/v1/services/aigc/multimodal-generation/generation"},
	} {
		for _, in := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s->%s/stream=%t", in, up.protocol, stream), func(t *testing.T) {
					fixture := managedConfigFixture()
					fixture["provider"] = json.RawMessage(`{"type":"qwen","apiTokens":["controlled-upstream-key"],"qwenEnableCompatible":false}`)
					release := fixture["_modelfusion_execution"].(managedRelease)
					release.Descriptor.UpstreamModel = "qwen-flash"
					release.Descriptor.AdapterRef = "ai-proxy.native-text.v1:" + in + ":" + up.protocol
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
					settings.InputProtocol = in
					settings.UpstreamProtocol = up.protocol
					settings.UpstreamPath = up.path
					settings.ResponseProfile = clientresponse.Qwen
					fixture["managed_execution"] = settings
					fixture["protocol_mode"] = "auto"

					h := newManagedHost(t, fixture)
					var pub []byte
					switch in {
					case managedProtocolChat:
						pub = []byte(fmt.Sprintf(`{"model":"public-model","stream":%t,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"ping"}]}`, stream))
					case managedProtocolResponses:
						pub = []byte(fmt.Sprintf(`{"model":"public-model","stream":%t,"input":"ping"}`, stream))
					case managedProtocolMessages:
						pub = []byte(fmt.Sprintf(`{"model":"public-model","stream":%t,"max_tokens":32,"messages":[{"role":"user","content":"ping"}]}`, stream))
					}
					h.start(t, pub, bytes.Replace(pub, []byte("public-model"), []byte("qwen-flash"), 1))
					calls := h.host.GetCalloutAttributesFromContext(h.id)
					require.Len(t, calls, 1)
					gotPath, _ := managedSingleHeader(calls[0].Headers, ":path")
					require.Equal(t, up.path, gotPath)

					if stream {
						sseBody := []byte("event: result\ndata: {\"request_id\":\"e1e2e3e4-0699-991f-849a-97f767160099\",\"output\":{\"choices\":[{\"finish_reason\":\"stop\",\"message\":{\"role\":\"assistant\",\"content\":\"pong\"}}]},\"usage\":{\"input_tokens\":14,\"output_tokens\":2,\"total_tokens\":16}}\n\n")
						h.rawResponse(t, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, sseBody)
					} else {
						jsonBody := []byte(`{"request_id":"e1e2e3e4-0699-991f-849a-97f767160099","output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"pong"}}]},"usage":{"input_tokens":14,"output_tokens":2,"total_tokens":16}}`)
						h.rawResponse(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, jsonBody)
					}
					drainDashScopeRedis(h)

					sent := h.host.GetSentLocalResponse(h.id)
					require.NotNil(t, sent)
					require.EqualValues(t, 200, sent.StatusCode)
					require.Contains(t, string(sent.Data), "pong")
					require.NotContains(t, string(sent.Data), "response_incomplete")
					h.host.CompleteHttpContext(h.id)
				})
			}
		}
	}
}

func newManagedSupplierAdapterErrorCheck(raw []byte, _ *managedExecutionConfig) error {
	value := gjson.ParseBytes(raw)
	var cfg config.PluginConfig
	cfg.FromJson(value)
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := cfg.Complete(); err != nil {
		return err
	}
	return configureManagedExecution(value, &cfg)
}
