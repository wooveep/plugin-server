package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func cacheSafetyRequest(t *testing.T, published string, body string) (test.TestHost, types.Action) {
	t.Helper()
	cfg := publishedCacheConfig(t, thinResponseCaptureConfig(t), published, "text_generate")
	host, status := test.NewTestHost(cfg)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-default"))
	require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"}, {":path", "/controlled/custom-answer"}, {":method", "POST"},
		{"content-type", "application/json"}, {"x-mse-tenant", "tenant-a"}, {"x-mse-consumer", "consumer-a"},
	}))
	return host, host.CallOnHttpRequestBody([]byte(body))
}

func cacheSafetyEvent(t *testing.T, host test.TestHost) map[string]json.RawMessage {
	t.Helper()
	var result map[string]json.RawMessage
	for _, call := range host.GetRedisCalloutAttributes() {
		command, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(command) < 5 || !strings.EqualFold(command[0], "xadd") {
			continue
		}
		for i := thinResponseCaptureXADDFieldStart(t, command); i+1 < len(command); i += 2 {
			if command[i] == "event" {
				require.Nil(t, result, "one actual response produces at most one CacheEvent")
				require.NoError(t, json.Unmarshal([]byte(command[i+1]), &result))
			}
		}
	}
	return result
}

func TestCacheSafetyUsagePresence(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		protocols := []struct {
			published                        string
			kind                             protocol.ProtocolKind
			request, response, input, output string
		}{
			{"openai_chat", protocol.ProtocolChatCompletions, `{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}]}`, `{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}]}`, "prompt_tokens", "completion_tokens"},
			{"anthropic_messages", protocol.ProtocolMessages, `{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}]}`, `{"type":"message","content":[{"type":"text","text":"answer"}],"stop_reason":"end_turn"}`, "input_tokens", "output_tokens"},
			{"openai_responses", protocol.ProtocolResponses, `{"model":"qwen-turbo","input":"hello"}`, `{"output_text":"answer","status":"completed"}`, "input_tokens", "output_tokens"},
		}
		for _, p := range protocols {
			for _, counts := range []struct{ name, input, output string }{
				{"missing", "", ""}, {"only_output", "", "5"}, {"explicit_zero", "0", "0"},
				{"exact_integer", "9007199254740993", "0"},
			} {
				for _, stream := range []bool{false, true} {
					t.Run(p.published+"/"+counts.name+"/"+strconv.FormatBool(stream), func(t *testing.T) {
						request, err := sjson.SetBytes([]byte(p.request), "stream", stream)
						require.NoError(t, err)
						host, action := cacheSafetyRequest(t, p.published, string(request))
						defer host.Reset()
						require.Equal(t, types.ActionPause, action)
						host.CallOnRedisCall(0, test.CreateRedisRespNull())
						require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
						usage := map[string]json.RawMessage{}
						if counts.input != "" {
							usage[p.input] = json.RawMessage(counts.input)
						}
						if counts.output != "" {
							usage[p.output] = json.RawMessage(counts.output)
						}
						response := []byte(p.response)
						if !stream {
							if counts.name != "missing" {
								raw, err := json.Marshal(usage)
								require.NoError(t, err)
								response, err = sjson.SetRawBytes(response, "usage", raw)
								require.NoError(t, err)
							}
							host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
							require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody(response))
							require.Equal(t, response, host.GetResponseBody())
						} else {
							host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "text/event-stream"}})
							switch p.kind {
							case protocol.ProtocolChatCompletions:
								response = []byte(`{"choices":[{"delta":{"content":"answer"},"finish_reason":"stop"}]}`)
								if counts.name != "missing" {
									raw, _ := json.Marshal(usage)
									response, err = sjson.SetRawBytes(response, "usage", raw)
									require.NoError(t, err)
								}
								callThinResponseStreamChunks(t, host, "data: "+string(response)+"\n\n", "data: [DONE]\n\n")
							case protocol.ProtocolMessages:
								start := `{"type":"message_start","message":{"content":[],"usage":{"output_tokens":0}}}`
								if counts.input != "" {
									raw, e := sjson.SetRawBytes([]byte(start), "message.usage.input_tokens", []byte(counts.input))
									require.NoError(t, e)
									start = string(raw)
								}
								final := `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`
								if counts.output != "" {
									raw, e := sjson.SetRawBytes([]byte(final), "usage.output_tokens", []byte(counts.output))
									require.NoError(t, e)
									final = string(raw)
								}
								callThinResponseStreamChunks(t, host, "event: message_start\ndata: "+start+"\n\n",
									"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"answer\"}}\n\n",
									"event: message_delta\ndata: "+final+"\n\n", "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
							case protocol.ProtocolResponses:
								final := `{"type":"response.completed","response":{"status":"completed"}}`
								if counts.name != "missing" {
									raw, _ := json.Marshal(usage)
									response, err = sjson.SetRawBytes([]byte(final), "response.usage", raw)
									require.NoError(t, err)
									final = string(response)
								}
								callThinResponseStreamChunks(t, host,
									"event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\n",
									"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\n",
									"event: response.completed\ndata: "+final+"\n\n")
							}
						}
						event := cacheSafetyEvent(t, host)
						require.NotNil(t, event)
						if counts.name == "missing" {
							require.NotContains(t, event, "usage")
							return
						}
						var retained map[string]json.RawMessage
						require.NoError(t, json.Unmarshal(event["usage"], &retained))
						require.Equal(t, usage, retained, "only actually reported fields survive; no synthesized total or zero")
						interpreted := nativeCacheReplayUsage(p.kind, event["usage"])
						if counts.input == "" {
							require.Equal(t, "missing", interpreted.Status)
						} else {
							require.Equal(t, "reported", interpreted.Status)
							require.Equal(t, counts.input, interpreted.Quantities["input"])
							require.Equal(t, counts.output, interpreted.Quantities["output"])
						}
					})
				}
			}
		}
	})
}

func TestCacheSafetyRequestOutputBoundary(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name, protocol, body string
			allowed              bool
		}{
			{"chat_audio_output", "openai_chat", `{"model":"qwen-turbo","modalities":["text","audio"],"messages":[{"role":"user","content":"hello"}]}`, false},
			{"response_background", "openai_responses", `{"model":"qwen-turbo","input":"hello","background":true}`, false},
			{"chat_mixed_input", "openai_chat", `{"model":"qwen-turbo","modalities":["text"],"messages":[{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image_url","image_url":{"url":"data:image/png;base64,fixture"}},{"type":"input_audio","input_audio":{"data":"fixture","format":"wav"}}]}]}`, true},
			{"responses_mixed_input", "openai_responses", `{"model":"qwen-turbo","background":false,"input":[{"role":"user","content":[{"type":"input_text","text":"hello"},{"type":"input_image","image_url":"data:image/png;base64,fixture"}]}]}`, true},
			{"messages_mixed_input", "anthropic_messages", `{"model":"qwen-turbo","messages":[{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"fixture"}}]}]}`, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				host, action := cacheSafetyRequest(t, tc.protocol, tc.body)
				defer host.Reset()
				require.Equal(t, []byte(tc.body), host.GetRequestBody())
				if tc.allowed {
					require.Equal(t, types.ActionPause, action)
					require.Len(t, host.GetRedisCalloutAttributes(), 1)
					return
				}
				require.Equal(t, types.ActionContinue, action)
				host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
				host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"text"},"finish_reason":"stop"}]}`))
				require.Empty(t, host.GetRedisCalloutAttributes(), "unsupported output cannot lookup, replay, or emit")
				require.Empty(t, host.GetHttpCalloutAttributes())
				require.Nil(t, host.GetLocalResponse(), "auxiliary bypass must not reject the business call")
			})
		}
		t.Run("native_bypass_keeps_original_first_body_basis", func(t *testing.T) {
			host, status := test.NewTestHost(nativeCacheExecutionConfig(t, true))
			require.Equal(t, types.OnPluginStartStatusOK, status)
			defer host.Reset()
			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheExecutionHeaders(true)))
			body, err := sjson.SetBytes([]byte(nativeCacheTestBody), "modalities", []string{"text", "audio"})
			require.NoError(t, err)
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(body))
			expected, err := bodybasis.Capture(body)
			require.NoError(t, err)
			actual, err := bodybasis.Read(host.GetRequestHeaders())
			require.NoError(t, err)
			require.Equal(t, expected, actual)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			require.Nil(t, host.GetLocalResponse())
		})
	})
}

func TestCacheSafetyResponseOutputBoundary(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct{ protocol, request, response, stream string }{
			{"openai_chat", `{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}]}`, `{"choices":[{"message":{"content":"answer","audio":{"data":"actual-audio"}},"finish_reason":"stop"}]}`, "data: {\"choices\":[{\"delta\":{\"content\":\"answer\",\"audio\":{\"data\":\"actual-audio\"}},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"},
			{"anthropic_messages", `{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}]}`, `{"content":[{"type":"text","text":"answer"},{"type":"audio","data":"actual-audio"}],"stop_reason":"end_turn"}`, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"audio\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"answer\"}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n"},
			{"openai_responses", `{"model":"qwen-turbo","input":"hello"}`, `{"output":[{"type":"message","content":[{"type":"output_text","text":"answer"},{"type":"output_audio","data":"actual-audio"}]}],"status":"completed"}`, "event: response.audio.delta\ndata: {\"type\":\"response.audio.delta\",\"delta\":\"actual-audio\"}\n\nevent: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"},
		} {
			for _, stream := range []bool{false, true} {
				t.Run(tc.protocol+"/"+strconv.FormatBool(stream), func(t *testing.T) {
					body, err := sjson.SetBytes([]byte(tc.request), "stream", stream)
					require.NoError(t, err)
					host, action := cacheSafetyRequest(t, tc.protocol, string(body))
					defer host.Reset()
					require.Equal(t, types.ActionPause, action)
					host.CallOnRedisCall(0, test.CreateRedisRespNull())
					if stream {
						host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "text/event-stream"}})
						callThinResponseStreamChunks(t, host, tc.stream)
					} else {
						host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
						host.CallOnHttpResponseBody([]byte(tc.response))
						require.Equal(t, []byte(tc.response), host.GetResponseBody())
					}
					require.Nil(t, cacheSafetyEvent(t, host), "actual non-text output cannot become a text-only cache event")
					require.Nil(t, host.GetLocalResponse())
				})
			}
		}
	})
}

func TestCacheSafetyMaterializedOutputAndMissingUsage(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		now := time.Now().Unix()
		material := ScopedCacheKeyMaterial{Tenant: "tenant-a", Consumer: "consumer-a", Route: "route-a", Model: "model-a", Protocol: protocol.ProtocolChatCompletions, CacheScope: "consumer", CachePolicyVersion: "1", RequestDigest: "digest"}
		record := MaterializedReplayRecord{SchemaVersion: materializedRecordSchemaVersion, Tenant: material.Tenant, Consumer: material.Consumer, Route: material.Route, Model: material.Model, Protocol: material.Protocol, CacheScope: material.CacheScope, CachePolicyVersion: material.CachePolicyVersion, RequestDigest: material.RequestDigest, SoftExpiresAt: now + 60, HardExpiresAt: now + 600, FinishReason: "stop", Usage: json.RawMessage(`{}`), Response: json.RawMessage(`{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}]}`)}
		require.NoError(t, validateMaterializedRecord(record, material, false, now), "an empty usage object remains a legal missing-usage replay")
		require.Equal(t, "missing", nativeCacheReplayUsage(record.Protocol, record.Usage).Status)
		record.Response = json.RawMessage(`{"choices":[{"message":{"content":"answer","audio":{"data":"actual-audio"}},"finish_reason":"stop"}]}`)
		require.Error(t, validateMaterializedRecord(record, material, false, now))
		record.Response = json.RawMessage(`{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}]}`)
		for _, usage := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`5`)} {
			record.Usage = usage
			require.Error(t, validateMaterializedRecord(record, material, false, now), "do not expand the original materialized wire contract")
		}
	})
}

func TestCacheSafetyNativeCallerSnapshot(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(nativeCacheExecutionConfig(t, true))
		require.Equal(t, types.OnPluginStartStatusOK, status)
		defer host.Reset()
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheExecutionHeaders(true)))
		// A later stage can remove transient headers. Lookup and FACT retain
		// the one authenticated snapshot already installed at the boundary.
		for _, name := range balanceidentity.HeaderNames() {
			require.NoError(t, proxywasm.RemoveHttpRequestHeader(name))
		}
		body := []byte(nativeCacheTestBody)
		require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
		host.CallOnRedisCall(0, test.CreateRedisRespString(nativeCacheExecutionRecord(t, body, false)))
		response := host.GetLocalResponse()
		require.NotNil(t, response)
		snapshot, err := executionfacts.DecodeDeliveryBridge(thinReplayPayloadHeader(response.Headers, executionfacts.DeliveryBridgeHeader))
		require.NoError(t, err)
		fact := snapshot.Envelope()
		caller := nativeCacheTestCaller()
		require.Equal(t, caller.TenantID, fact.TenantID)
		require.Equal(t, caller.PrincipalID, fact.PrincipalID)
		require.Equal(t, caller.AccountID, fact.AccountID)
		require.Equal(t, nativeCacheOriginalCall, fact.CallID)
		require.Empty(t, host.GetHttpCalloutAttributes())
	})
}
