package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestCacheUsageDeltaActualProtocolCapture(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			published                string
			kind                     protocol.ProtocolKind
			request, response, usage string
		}{
			{"openai_chat", protocol.ProtocolChatCompletions, `{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}]}`, `{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}]}`, `{"prompt_tokens":17,"completion_tokens":5,"total_tokens":22,"prompt_tokens_details":{"cached_tokens":17,"original_exact_detail":9007199254740993},"completion_tokens_details":{"reasoning_tokens":0}}`},
			{"anthropic_messages", protocol.ProtocolMessages, `{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}]}`, `{"type":"message","content":[{"type":"text","text":"answer"}],"stop_reason":"end_turn"}`, `{"input_tokens":0,"output_tokens":5,"total_tokens":22,"cache_read_input_tokens":17,"cache_creation_input_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":0,"original_exact_detail":9007199254740993}}`},
			{"openai_responses", protocol.ProtocolResponses, `{"model":"qwen-turbo","input":"hello"}`, `{"output":[{"type":"message","content":[{"type":"output_text","text":"answer"}]}],"status":"completed"}`, `{"input_tokens":17,"output_tokens":5,"total_tokens":22,"input_tokens_details":{"cached_tokens":17,"original_exact_detail":9007199254740993},"output_tokens_details":{"reasoning_tokens":0}}`},
		} {
			for _, stream := range []bool{false, true} {
				t.Run(tc.published+"/"+strconv.FormatBool(stream), func(t *testing.T) {
					body, err := sjson.SetBytes([]byte(tc.request), "stream", stream)
					require.NoError(t, err)
					host, action := cacheSafetyRequest(t, tc.published, string(body))
					defer host.Reset()
					require.Equal(t, types.ActionPause, action)
					host.CallOnRedisCall(0, test.CreateRedisRespNull())
					if !stream {
						response, err := sjson.SetRawBytes([]byte(tc.response), "usage", []byte(tc.usage))
						require.NoError(t, err)
						host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
						host.CallOnHttpResponseBody(response)
						require.Equal(t, response, host.GetResponseBody())
					} else {
						host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "text/event-stream"}})
						switch tc.kind {
						case protocol.ProtocolChatCompletions:
							callThinResponseStreamChunks(t, host,
								"data: {\"choices\":[{\"delta\":{\"content\":\"answer\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":0,\"completion_tokens\":0,\"prior_only\":true}}\n\n",
								"data: {\"choices\":[],\"usage\":"+tc.usage+"}\n\n", "data: [DONE]\n\n")
						case protocol.ProtocolMessages:
							// The actual start cache fields survive, but its output/total
							// zeros cannot stand in for the final report.
							start, err := sjson.SetRawBytes([]byte(tc.usage), "output_tokens", []byte("0"))
							require.NoError(t, err)
							start, err = sjson.SetRawBytes(start, "total_tokens", []byte("0"))
							require.NoError(t, err)
							callThinResponseStreamChunks(t, host,
								"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"content\":[],\"usage\":"+string(start)+"}}\n\n",
								"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"answer\"}}\n\n",
								"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5,\"total_tokens\":22}}\n\n",
								"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
						case protocol.ProtocolResponses:
							callThinResponseStreamChunks(t, host,
								"event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"usage\":{\"input_tokens\":0,\"output_tokens\":0,\"prior_only\":true}}}\n\n",
								"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\n",
								"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":"+tc.usage+"}}\n\n")
						}
					}
					event := cacheSafetyEvent(t, host)
					require.NotNil(t, event)
					var original, retained map[string]json.RawMessage
					require.NoError(t, json.Unmarshal([]byte(tc.usage), &original))
					require.NoError(t, json.Unmarshal(event["usage"], &retained))
					require.Equal(t, original, retained, "full original objects and exact nested integer lexemes must survive")
					meter := nativeCacheReplayUsage(tc.kind, event["usage"])
					require.Equal(t, "reported", meter.Status)
					require.Equal(t, "meter", meter.Source)
					require.Equal(t, map[string]string{"input": "17", "output": "5", "cache_hit": "17", "cache_miss": "0"}, meter.Quantities)
				})
			}
		}
	})
}

func TestCacheUsageDeltaNativeDeliveryUsesActualProviderMeter(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			published             string
			kind                  protocol.ProtocolKind
			body, response, usage string
		}{
			{"openai_chat", protocol.ProtocolChatCompletions, `{"model":"public-alias","messages":[{"role":"user","content":"hello"}]}`, `{"choices":[{"message":{"content":"cached"},"finish_reason":"stop"}]}`, `{"prompt_tokens":17,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":17}}`},
			{"anthropic_messages", protocol.ProtocolMessages, `{"model":"public-alias","messages":[{"role":"user","content":"hello"}]}`, `{"content":[{"type":"text","text":"cached"}],"stop_reason":"end_turn"}`, `{"input_tokens":0,"output_tokens":5,"cache_read_input_tokens":17,"cache_creation_input_tokens":0}`},
			{"openai_responses", protocol.ProtocolResponses, `{"model":"public-alias","input":"hello"}`, `{"output":[{"type":"message","content":[{"type":"output_text","text":"cached"}]}],"status":"completed"}`, `{"input_tokens":17,"output_tokens":5,"input_tokens_details":{"cached_tokens":17}}`},
		} {
			t.Run(tc.published, func(t *testing.T) {
				cfg, err := sjson.SetBytes(nativeCacheExecutionConfig(t, true), "client_protocol", tc.published)
				require.NoError(t, err)
				host, status := test.NewTestHost(cfg)
				require.Equal(t, types.OnPluginStartStatusOK, status)
				defer host.Reset()
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheExecutionHeaders(true)))
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(tc.body)))
				adapter, ok := thinProtocolAdapterForKind(tc.kind)
				require.True(t, ok)
				digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{Method: "POST", Path: "/native/custom-answer", Body: []byte(tc.body)})
				require.NoError(t, err)
				record := MaterializedReplayRecord{
					SchemaVersion: materializedRecordSchemaVersion, Tenant: nativeCacheTestCaller().TenantID, Consumer: nativeCacheTestCaller().PrincipalID,
					Route: nativeCacheTestRoute, Model: "public-alias", Protocol: tc.kind, CacheScope: "consumer",
					CachePolicyVersion: strconv.FormatInt(nativeCacheTestVersion, 10), RequestDigest: digest.Digest,
					SoftExpiresAt: time.Now().Unix() + 60, HardExpiresAt: time.Now().Unix() + 3600, FinishReason: "stop",
					Usage: json.RawMessage(tc.usage), Response: json.RawMessage(tc.response),
				}
				raw, err := json.Marshal(record)
				require.NoError(t, err)
				host.CallOnRedisCall(0, test.CreateRedisRespString(string(raw)))
				response := host.GetLocalResponse()
				require.NotNil(t, response)
				snapshot, err := executionfacts.DecodeDeliveryBridge(thinReplayPayloadHeader(response.Headers, executionfacts.DeliveryBridgeHeader))
				require.NoError(t, err)
				fact := snapshot.Envelope()
				require.Equal(t, "cache", fact.ServedFrom)
				require.Equal(t, nativeCacheOriginalCall, fact.CallID)
				require.Equal(t, "meter", fact.Usage.Source)
				require.Equal(t, map[string]string{"input": "17", "output": "5", "cache_hit": "17", "cache_miss": "0"}, fact.Usage.Quantities)
				require.Empty(t, host.GetHttpCalloutAttributes())
			})
		}
	})
}

func TestCacheUsageDeltaExactCountsAndMissing(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		max := "340282366920938463463374607431768211455"
		for _, tc := range []struct {
			name     string
			kind     protocol.ProtocolKind
			raw      string
			expected map[string]string
		}{
			{"messages_u128", protocol.ProtocolMessages, `{"input_tokens":0,"output_tokens":5,"cache_read_input_tokens":` + max + `,"cache_creation_input_tokens":0}`, map[string]string{"input": max, "output": "5", "cache_hit": max, "cache_miss": "0"}},
			{"messages_sum_overflow", protocol.ProtocolMessages, `{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":` + max + `,"cache_creation_input_tokens":0}`, nil},
			{"messages_creation_sum_overflow", protocol.ProtocolMessages, `{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":` + max + `}`, nil},
			{"messages_missing_creation", protocol.ProtocolMessages, `{"input_tokens":0,"output_tokens":5,"cache_read_input_tokens":17}`, nil},
			{"messages_missing_output", protocol.ProtocolMessages, `{"input_tokens":0,"cache_read_input_tokens":17,"cache_creation_input_tokens":0}`, nil},
			{"messages_creation_plus_input", protocol.ProtocolMessages, `{"input_tokens":3,"output_tokens":5,"cache_read_input_tokens":17,"cache_creation_input_tokens":2}`, map[string]string{"input": "22", "output": "5", "cache_hit": "17", "cache_miss": "5"}},
			{"chat_nested_u128", protocol.ProtocolChatCompletions, `{"prompt_tokens":` + max + `,"completion_tokens":0,"prompt_tokens_details":{"cached_tokens":` + max + `}}`, map[string]string{"input": max, "output": "0", "cache_hit": max, "cache_miss": "0"}},
			{"responses_nested_u128", protocol.ProtocolResponses, `{"input_tokens":` + max + `,"output_tokens":0,"input_tokens_details":{"cached_tokens":` + max + `}}`, map[string]string{"input": max, "output": "0", "cache_hit": max, "cache_miss": "0"}},
			{"chat_nested_exceeds_input", protocol.ProtocolChatCompletions, `{"prompt_tokens":16,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":17}}`, nil},
			{"chat_nested_string", protocol.ProtocolChatCompletions, `{"prompt_tokens":17,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":"17"}}`, nil},
			{"responses_nested_fraction", protocol.ProtocolResponses, `{"input_tokens":17,"output_tokens":5,"input_tokens_details":{"cached_tokens":17.0}}`, nil},
			{"responses_nested_overflow", protocol.ProtocolResponses, `{"input_tokens":17,"output_tokens":5,"input_tokens_details":{"cached_tokens":340282366920938463463374607431768211456}}`, nil},
			{"chat_total_conflict", protocol.ProtocolChatCompletions, `{"prompt_tokens":17,"completion_tokens":5,"total_tokens":23,"prompt_tokens_details":{"cached_tokens":17}}`, nil},
			{"chat_original_splits", protocol.ProtocolChatCompletions, `{"prompt_tokens":17,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":17},"prompt_cache_hit_tokens":17,"prompt_cache_miss_tokens":0}`, map[string]string{"input": "17", "output": "5", "cache_hit": "17", "cache_miss": "0"}},
			{"chat_original_split_conflict", protocol.ProtocolChatCompletions, `{"prompt_tokens":17,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":17},"prompt_cache_hit_tokens":16,"prompt_cache_miss_tokens":1}`, nil},
			{"explicit_zero_nested", protocol.ProtocolResponses, `{"input_tokens":0,"output_tokens":0,"input_tokens_details":{"cached_tokens":0}}`, map[string]string{"input": "0", "output": "0", "cache_hit": "0", "cache_miss": "0"}},
			{"no_split_from_platform_cache", protocol.ProtocolChatCompletions, `{"prompt_tokens":17,"completion_tokens":5}`, map[string]string{"input": "17", "output": "5"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				usage := nativeCacheReplayUsage(tc.kind, json.RawMessage(tc.raw))
				if tc.expected == nil {
					require.Equal(t, "missing", usage.Status)
					require.Nil(t, usage.Quantities)
					return
				}
				require.Equal(t, "reported", usage.Status)
				require.Equal(t, "meter", usage.Source)
				require.Equal(t, tc.expected, usage.Quantities)
			})
		}
	})
}

func TestCacheToolDeltaMetadataAndMedia(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			published                                                      string
			kind                                                           protocol.ProtocolKind
			request, toolResponse, mediaResponse, streamTools, streamMedia string
		}{
			{"openai_chat", protocol.ProtocolChatCompletions, `{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}]}`, `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"original","type":"function","function":{"name":"test","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":17,"completion_tokens":5}}`, `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"original","type":"function","function":{"name":"test","arguments":"{}"}}],"audio":{"data":"actual-media"}},"finish_reason":"tool_calls"}]}`,
				"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"original\",\"type\":\"function\",\"function\":{\"name\":\"test\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n",
				"data: {\"choices\":[{\"delta\":{\"audio\":{\"data\":\"actual-media\"}}}]}\n\n"},
			{"anthropic_messages", protocol.ProtocolMessages, `{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}]}`, `{"content":[{"type":"tool_use","id":"original","name":"test","input":{}}],"stop_reason":"tool_use","usage":{"input_tokens":0,"output_tokens":5,"cache_read_input_tokens":17,"cache_creation_input_tokens":0}}`, `{"content":[{"type":"tool_use","id":"original","name":"test","input":{}},{"type":"audio","data":"actual-media"}],"stop_reason":"tool_use"}`,
				"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"tool_use\",\"id\":\"original\",\"name\":\"test\",\"input\":{}}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"}}\n\n",
				"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"audio\",\"data\":\"actual-media\"}}\n\n"},
			{"openai_responses", protocol.ProtocolResponses, `{"model":"qwen-turbo","input":"hello"}`, `{"output":[{"type":"function_call","name":"test","call_id":"original","arguments":"{}"}],"status":"completed","usage":{"input_tokens":17,"output_tokens":5}}`, `{"output":[{"type":"function_call","name":"test","call_id":"original","arguments":"{}"},{"type":"message","content":[{"type":"output_audio","data":"actual-media"}]}],"status":"completed"}`,
				"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"function_call\",\"name\":\"test\",\"call_id\":\"original\",\"arguments\":\"{}\"}],\"status\":\"completed\",\"usage\":{\"input_tokens\":17,\"output_tokens\":5}}}\n\n",
				"event: response.audio.delta\ndata: {\"type\":\"response.audio.delta\",\"delta\":\"actual-media\"}\n\n"},
		} {
			for _, stream := range []bool{false, true} {
				for _, media := range []bool{false, true} {
					t.Run(tc.published+"/stream_"+strconv.FormatBool(stream)+"/media_"+strconv.FormatBool(media), func(t *testing.T) {
						body, err := sjson.SetBytes([]byte(tc.request), "stream", stream)
						require.NoError(t, err)
						host, action := cacheSafetyRequest(t, tc.published, string(body))
						defer host.Reset()
						require.Equal(t, types.ActionPause, action)
						host.CallOnRedisCall(0, test.CreateRedisRespNull())
						if stream {
							host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "text/event-stream"}})
							frames := tc.streamTools
							if media {
								frames += tc.streamMedia
							}
							if tc.kind == protocol.ProtocolChatCompletions {
								frames += "data: [DONE]\n\n"
							}
							callThinResponseStreamChunks(t, host, frames)
						} else {
							host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
							response := tc.toolResponse
							if media {
								response = tc.mediaResponse
							}
							host.CallOnHttpResponseBody([]byte(response))
							require.Equal(t, []byte(response), host.GetResponseBody())
						}
						event := cacheSafetyEvent(t, host)
						if media {
							require.Nil(t, event, "tools must not hide media in later output items or frames")
						} else {
							require.NotNil(t, event, "tool metadata still emits the original gated auxiliary event")
							require.Equal(t, "true", string(event["contains_tool_calls"]))
							require.NotContains(t, event, "assistant_content", "tool results cannot become materialized text")
						}
						require.Nil(t, host.GetLocalResponse())
					})
				}
			}
			t.Run(tc.published+"/replay_rejects_tools", func(t *testing.T) {
				now := time.Now().Unix()
				material := ScopedCacheKeyMaterial{Tenant: "tenant-a", Consumer: "consumer-a", Route: "route-a", Model: "model-a", Protocol: tc.kind, CacheScope: "consumer", CachePolicyVersion: "1", RequestDigest: "digest"}
				record := MaterializedReplayRecord{SchemaVersion: materializedRecordSchemaVersion, Tenant: material.Tenant, Consumer: material.Consumer, Route: material.Route, Model: material.Model, Protocol: tc.kind, CacheScope: material.CacheScope, CachePolicyVersion: material.CachePolicyVersion, RequestDigest: material.RequestDigest, SoftExpiresAt: now + 60, HardExpiresAt: now + 600, FinishReason: "completed", Usage: json.RawMessage(`{}`), Response: json.RawMessage(tc.toolResponse)}
				require.ErrorContains(t, validateMaterializedRecord(record, material, false, now), "tool calls")
				for _, frame := range strings.Split(tc.streamTools, "\n\n") {
					for _, line := range strings.Split(frame, "\n") {
						if strings.HasPrefix(line, "data: ") {
							record.StreamChunks = append(record.StreamChunks, json.RawMessage(strings.TrimPrefix(line, "data: ")))
						}
					}
				}
				record.StreamReplayable = true
				require.ErrorContains(t, validateMaterializedRecord(record, material, true, now), "tool calls")
			})
		}
	})
}
