package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const t25GoogleUsageJSON = `{"promptTokenCount":9007199254740993,"toolUsePromptTokenCount":4,"candidatesTokenCount":5,"thoughtsTokenCount":3,"totalTokenCount":9007199254741005,"cachedContentTokenCount":2}`

func newT25CrossWireHost(
	t *testing.T,
	providerRaw string,
	clientProtocol string,
	profile clientresponse.Profile,
) *managedHost {
	t.Helper()
	fixture := managedConfigFixture()
	fixture["provider"] = json.RawMessage(providerRaw)
	settings := fixture["managed_execution"].(managedSettings)
	settings.InputProtocol = clientProtocol
	settings.UpstreamProtocol = managedProtocolChat
	settings.ResponseProfile = profile
	fixture["managed_execution"] = settings
	return newManagedHost(t, fixture)
}

func runT25HostCase(
	t *testing.T,
	caseName string,
	providerRaw string,
	clientProtocol string,
	profile clientresponse.Profile,
	publicReq []byte,
	upstreamHeaders [][2]string,
	upstreamBody []byte,
) []byte {
	t.Helper()
	var out []byte
	t.Run(caseName, func(t *testing.T) {
		h := newT25CrossWireHost(t, providerRaw, clientProtocol, profile)
		mappedReq := bytes.Replace(publicReq, []byte("public-model"), []byte("upstream-model"), 1)
		h.start(t, publicReq, mappedReq)

		state, ok := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
		require.True(t, ok && state != nil)
		require.True(t, managedUsesCanonicalIR(state.config), "T25 case must execute through Canonical IR C pipeline")

		calls := h.host.GetCalloutAttributesFromContext(h.id)
		require.Len(t, calls, 1, "sole sender invariant")

		h.response(t, upstreamHeaders, upstreamBody)
		reply := h.host.GetSentLocalResponse(h.id)
		require.NotNil(t, reply, "downstream response missing")
		require.Equal(t, uint32(200), reply.StatusCode)
		out = bytes.Clone(reply.Data)
	})
	return out
}

// TestCanonicalSDKExport generates representative real-pipeline Canonical IR C
// downstream outputs across OpenAI Chat, OpenAI Responses (completed, incomplete,
// failed), and Anthropic Messages (JSON + SSE with signature_delta & M04).
// When MODELFUSION_CANONICAL_SDK_EVIDENCE_DIR is unset, it validates in-memory
// without writing any files to the repository.
func TestCanonicalSDKExport(t *testing.T) {
	restoreEntropy := clientresponse.SetIdentityEntropySource(newBenchDeterministicRand())
	t.Cleanup(restoreEntropy)

	geminiProvider := `{"type":"gemini","apiTokens":["controlled-upstream-key"],"apiVersion":"v1beta"}`
	qwenNativeProvider := `{"type":"qwen","protocol":"original","apiTokens":["controlled-upstream-key"],"qwenEnableCompatible":false,"qwenEnableSearch":false}`

	// 1. OpenAI Chat Completions — Non-Streaming JSON (Gemini -> openai_chat)
	chatJSONReq := []byte(`{
		"model":"public-model",
		"stream":false,
		"tools":[
			{"type":"function","function":{"name":"first_tool","parameters":{"type":"object"}}},
			{"type":"function","function":{"name":"second_tool","parameters":{"type":"object"}}}
		],
		"messages":[{"role":"user","content":"Execute parallel tools"}]
	}`)
	chatJSONUpBody := []byte(`{
		"responseId":"gem_chat_json_1",
		"candidates":[{
			"index":0,
			"finishReason":"STOP",
			"content":{
				"role":"model",
				"parts":[
					{"thought":true,"text":"Analyzing parallel parameters."},
					{"functionCall":{"id":"gcall_1","name":"first_tool","args":{"value":9007199254740993}}},
					{"functionCall":{"id":"gcall_2","name":"second_tool","args":{"city":"Shanghai"}}}
				]
			}
		}],
		"usageMetadata":` + t25GoogleUsageJSON + `
	}`)
	chatJSONOut := runT25HostCase(
		t, "chat_json", geminiProvider, managedProtocolChat, clientresponse.OpenAI,
		chatJSONReq, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, chatJSONUpBody,
	)
	assert.Equal(t, "chat.completion", gjson.GetBytes(chatJSONOut, "object").String())
	assert.Equal(t, gjson.Null, gjson.GetBytes(chatJSONOut, "choices.0.message.content").Type)
	assert.Equal(t, `{"value":9007199254740993}`, gjson.GetBytes(chatJSONOut, "choices.0.message.tool_calls.0.function.arguments").String())

	// 2. OpenAI Chat Completions — Streaming SSE (DashScope native -> openai_chat with include_usage)
	chatStreamReq := []byte(`{
		"model":"public-model",
		"stream":true,
		"stream_options":{"include_usage":true},
		"enable_thinking":true,
		"tools":[
			{"type":"function","function":{"name":"first_tool","parameters":{"type":"object"}}},
			{"type":"function","function":{"name":"second_tool","parameters":{"type":"object"}}}
		],
		"messages":[{"role":"user","content":"Stream parallel tools"}]
	}`)
	chatStreamUpBody := []byte(strings.Join([]string{
		`data: {"request_id":"qwen_cs_1","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","reasoning_content":"Step 1: ","content":""}}]}}`,
		`data: {"request_id":"qwen_cs_1","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","reasoning_content":"dispatching.","tool_calls":[{"index":0,"id":"qcall_0","type":"function","function":{"name":"first_tool","arguments":"{\"value\":"}},{"index":1,"id":"qcall_1","type":"function","function":{"name":"second_tool","arguments":"{\"city\":\"Shanghai\"}"}}]}}]}}`,
		`data: {"request_id":"qwen_cs_1","output":{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"index":0,"id":"qcall_0","type":"function","function":{"name":"first_tool","arguments":"9007199254740993}"}}]}}]},"usage":{"input_tokens":100,"output_tokens":12,"total_tokens":112,"output_tokens_details":{"reasoning_tokens":4,"text_tokens":8}}}`,
	}, "\n\n") + "\n\n")
	chatStreamOut := runT25HostCase(
		t, "chat_stream", qwenNativeProvider, managedProtocolChat, clientresponse.Qwen,
		chatStreamReq, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, chatStreamUpBody,
	)
	assert.Contains(t, string(chatStreamOut), "data: [DONE]\n\n")
	assert.Contains(t, string(chatStreamOut), "9007199254740993")

	// 3. OpenAI Responses — Non-Streaming JSON (Gemini -> openai_responses)
	respJSONReq := []byte(`{
		"model":"public-model",
		"stream":false,
		"instructions":"Keep precision exact.",
		"metadata":{"suite":"t25"},
		"tools":[
			{"type":"function","name":"first_tool","parameters":{"type":"object"}},
			{"type":"function","name":"second_tool","parameters":{"type":"object"}}
		],
		"input":"Run audit"
	}`)
	respJSONUpBody := []byte(`{
		"responseId":"gem_resp_json_1",
		"candidates":[{
			"index":0,
			"finishReason":"STOP",
			"content":{
				"role":"model",
				"parts":[
					{"thought":true,"text":"Deliberating before tool calls."},
					{"text":"Checking both systems."},
					{"functionCall":{"id":"rcall_1","name":"first_tool","args":{"value":9007199254740993}}},
					{"functionCall":{"id":"rcall_2","name":"second_tool","args":{"city":"Shanghai"}}}
				]
			}
		}],
		"usageMetadata":` + t25GoogleUsageJSON + `
	}`)
	respJSONOut := runT25HostCase(
		t, "responses_json", geminiProvider, managedProtocolResponses, clientresponse.OpenAI,
		respJSONReq, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, respJSONUpBody,
	)
	assert.Equal(t, "completed", gjson.GetBytes(respJSONOut, "status").String())
	assert.Equal(t, 4, len(gjson.GetBytes(respJSONOut, "output").Array()))
	assert.Equal(t, `{"value":9007199254740993}`, gjson.GetBytes(respJSONOut, "output.2.arguments").String())

	// 4. OpenAI Responses — Streaming SSE completed (Gemini -> openai_responses)
	respStreamReq := []byte(`{
		"model":"public-model",
		"stream":true,
		"instructions":"Keep precision exact.",
		"tools":[
			{"type":"function","name":"first_tool","parameters":{"type":"object"}},
			{"type":"function","name":"second_tool","parameters":{"type":"object"}}
		],
		"input":"Run streaming audit"
	}`)
	respStreamCompletedUp := []byte(strings.Join([]string{
		`data: {"responseId":"gem_rs_ok","candidates":[{"index":0,"content":{"role":"model","parts":[{"thought":true,"text":"Thinking through steps."}]}}]}`,
		`data: {"responseId":"gem_rs_ok","candidates":[{"index":0,"content":{"role":"model","parts":[{"functionCall":{"id":"rscall_1","name":"first_tool","args":{"value":9007199254740993}}},{"text":"checking"},{"functionCall":{"id":"rscall_2","name":"second_tool","args":{"city":"Shanghai"}}}]}}]}`,
		`data: {"responseId":"gem_rs_ok","candidates":[{"index":0,"finishReason":"STOP"}],"usageMetadata":` + t25GoogleUsageJSON + `}`,
	}, "\n\n") + "\n\n")
	respStreamCompletedOut := runT25HostCase(
		t, "responses_stream_completed", geminiProvider, managedProtocolResponses, clientresponse.OpenAI,
		respStreamReq, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, respStreamCompletedUp,
	)
	assert.Contains(t, string(respStreamCompletedOut), "event: response.reasoning_text.delta\n")
	assert.Contains(t, string(respStreamCompletedOut), "event: response.completed\n")

	// 5. OpenAI Responses — Streaming SSE incomplete (Gemini MAX_TOKENS -> openai_responses)
	respStreamIncompleteUp := []byte(strings.Join([]string{
		`data: {"responseId":"gem_rs_inc","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Partial answer cut off by token ceiling"}]}}]}`,
		`data: {"responseId":"gem_rs_inc","candidates":[{"index":0,"finishReason":"MAX_TOKENS"}],"usageMetadata":` + t25GoogleUsageJSON + `}`,
	}, "\n\n") + "\n\n")
	respStreamIncompleteOut := runT25HostCase(
		t, "responses_stream_incomplete", geminiProvider, managedProtocolResponses, clientresponse.OpenAI,
		respStreamReq, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, respStreamIncompleteUp,
	)
	assert.Contains(t, string(respStreamIncompleteOut), "event: response.incomplete\n")

	// 6. OpenAI Responses — Streaming SSE failed with incomplete open items (DashScope mid-stream error -> openai_responses)
	respStreamFailedUp := []byte(strings.Join([]string{
		`data: {"request_id":"qwen_rs_fail","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","reasoning_content":"Partial reasoning","content":"Partial text"}}]}}`,
		`data: {"request_id":"qwen_rs_fail","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","tool_calls":[{"index":0,"id":"fcall_0","type":"function","function":{"name":"first_tool","arguments":"{\"value\":9007199"}}]}}]},"usage":{"input_tokens":40,"output_tokens":9,"total_tokens":49}}`,
		`event: error` + "\n" + `data: {"code":"InternalError","message":"upstream worker crashed"}`,
	}, "\n\n") + "\n\n")
	respStreamFailedOut := runT25HostCase(
		t, "responses_stream_failed", qwenNativeProvider, managedProtocolResponses, clientresponse.OpenAI,
		respStreamReq, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, respStreamFailedUp,
	)
	assert.Contains(t, string(respStreamFailedOut), "event: response.failed\n")
	assert.NotContains(t, string(respStreamFailedOut), "event: response.completed\n")

	// 7. Anthropic Messages — Non-Streaming JSON (Gemini with thoughtSignature + parallel functionCalls -> anthropic_messages)
	msgJSONReq := []byte(`{
		"model":"public-model",
		"max_tokens":1024,
		"stream":false,
		"tools":[
			{"name":"first_tool","input_schema":{"type":"object"}},
			{"name":"second_tool","input_schema":{"type":"object"}}
		],
		"messages":[{"role":"user","content":"Inspect and call tools"}]
	}`)
	msgJSONUpBody := []byte(`{
		"responseId":"gem_msg_json_1",
		"candidates":[{
			"index":0,
			"finishReason":"STOP",
			"content":{
				"role":"model",
				"parts":[
					{"thought":true,"text":"Verifying ledger invariants.","thoughtSignature":"sig_anth_compat_01"},
					{"text":"Executing both ledger tools."},
					{"functionCall":{"id":"mcall_1","name":"first_tool","args":{"value":9007199254740993}}},
					{"functionCall":{"id":"mcall_2","name":"second_tool","args":{"city":"Shanghai"}}}
				]
			}
		}],
		"usageMetadata":` + t25GoogleUsageJSON + `
	}`)
	msgJSONOut := runT25HostCase(
		t, "messages_json", geminiProvider, managedProtocolMessages, clientresponse.DeepSeek,
		msgJSONReq, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, msgJSONUpBody,
	)
	assert.Equal(t, "message", gjson.GetBytes(msgJSONOut, "type").String())
	assert.Equal(t, "tool_use", gjson.GetBytes(msgJSONOut, "stop_reason").String())
	assert.Equal(t, "sig_anth_compat_01", gjson.GetBytes(msgJSONOut, "content.0.signature").String())
	assert.Equal(t, "9007199254740993", gjson.GetBytes(msgJSONOut, "content.2.input.value").Raw)

	// 8. Anthropic Messages — Streaming SSE (Gemini with thought + thoughtSignature + parallel functionCalls -> anthropic_messages Qwen M04 profile)
	msgStreamReq := []byte(`{
		"model":"public-model",
		"max_tokens":1024,
		"stream":true,
		"tools":[
			{"name":"first_tool","input_schema":{"type":"object"}},
			{"name":"second_tool","input_schema":{"type":"object"}}
		],
		"messages":[{"role":"user","content":"Stream thinking and tools"}]
	}`)
	msgStreamUpBody := []byte(strings.Join([]string{
		`data: {"responseId":"gem_ms_1","candidates":[{"index":0,"content":{"role":"model","parts":[{"thought":true,"text":"Step-by-step reasoning.","thoughtSignature":"sig_stream_an04"}]}}],"usageMetadata":` + t25GoogleUsageJSON + `}`,
		`data: {"responseId":"gem_ms_1","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Running tools now."},{"functionCall":{"id":"mscall_1","name":"first_tool","args":{"value":9007199254740993}}},{"functionCall":{"id":"mscall_2","name":"second_tool","args":{"city":"Shanghai"}}}]}}]}`,
		`data: {"responseId":"gem_ms_1","candidates":[{"index":0,"finishReason":"STOP"}],"usageMetadata":` + t25GoogleUsageJSON + `}`,
	}, "\n\n") + "\n\n")
	msgStreamOut := runT25HostCase(
		t, "messages_stream", geminiProvider, managedProtocolMessages, clientresponse.Qwen,
		msgStreamReq, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, msgStreamUpBody,
	)
	assert.Contains(t, string(msgStreamOut), `"type":"signature_delta"`)
	assert.Contains(t, string(msgStreamOut), "event: message_stop\n")

	if exportDir := strings.TrimSpace(os.Getenv("MODELFUSION_CANONICAL_SDK_EVIDENCE_DIR")); exportDir != "" {
		require.NoError(t, os.MkdirAll(exportDir, 0o700))
		exports := map[string][]byte{
			"chat-json.json":                  chatJSONOut,
			"chat-stream.sse":                 chatStreamOut,
			"responses-json.json":             respJSONOut,
			"responses-stream-completed.sse":  respStreamCompletedOut,
			"responses-stream-incomplete.sse": respStreamIncompleteOut,
			"responses-stream-failed.sse":     respStreamFailedOut,
			"messages-json.json":              msgJSONOut,
			"messages-stream.sse":             msgStreamOut,
		}
		for name, payload := range exports {
			require.NoError(t, os.WriteFile(filepath.Join(exportDir, name), payload, 0o600))
		}
	}
}
