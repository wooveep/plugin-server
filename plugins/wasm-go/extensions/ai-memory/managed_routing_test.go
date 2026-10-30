package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestManagedMemoryConfigBoundary(t *testing.T) {
	var global config.PluginConfig
	global.FromJson(gjson.Parse(`{"redis_stream":{"service_name":"redis.memory.svc.cluster.local"},"enable_path_suffixes":["/legacy-only"]}`), memoryCaptureNoopLog{})
	require.NoError(t, global.Validate())

	t.Run("bound route clears inherited filters without mutating global", func(t *testing.T) {
		var route config.PluginConfig
		route.FromJsonWithGlobal(gjson.Parse(`{"path_scope":"bound_target","client_protocol":"openai_responses","operation_kind":"text_generate","memory_mode":"semantic"}`), global, memoryCaptureNoopLog{})
		require.NoError(t, route.Validate())
		require.Empty(t, route.EnablePathSuffixes)
		require.Equal(t, []string{"/legacy-only"}, global.EnablePathSuffixes)
		require.Equal(t, protocol.ProtocolResponses, route.ProtocolForPath("/private/answer"))
		require.Equal(t, protocol.ProtocolResponses, route.ProtocolForPath("/v1/chat/completions"))
	})
	t.Run("legacy rules keep their original filters and protocol selection", func(t *testing.T) {
		var route config.PluginConfig
		route.FromJsonWithGlobal(gjson.Parse(`{"memory_mode":"digest"}`), global, memoryCaptureNoopLog{})
		require.NoError(t, route.Validate())
		require.Equal(t, []string{"/legacy-only"}, route.EnablePathSuffixes)
		require.Equal(t, protocol.ProtocolMessages, route.ProtocolForPath("/v1/messages"))
		require.Equal(t, protocol.ProtocolUnknown, route.ProtocolForPath("/private/answer"))
	})
	for _, raw := range []string{
		`{"path_scope":"bound_target"}`,
		`{"path_scope":"bound_target","client_protocol":"openai_chat"}`,
		`{"client_protocol":"openai_chat","operation_kind":true}`,
		`{"client_protocol":null,"operation_kind":"text_generate"}`,
		`{"client_protocol":"","operation_kind":"text_generate"}`,
		`{"client_protocol":"openai_chat","operation_kind":""}`,
		`{"path_scope":null}`,
		`{"path_scope":"guess"}`,
		`{"path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"text_generate","enable_path_suffixes":["/v1/chat/completions"]}`,
		`{"path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"text_generate","enable_path_suffixes":[""]}`,
		`{"path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"text_generate","enable_methods":["POST"]}`,
		`{"path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"text_generate","enable_methods":null}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var route config.PluginConfig
			route.FromJsonWithGlobal(gjson.Parse(raw), global, memoryCaptureNoopLog{})
			require.Error(t, route.Validate())
		})
	}
	t.Run("complete unsupported operation never falls back to a known suffix", func(t *testing.T) {
		var route config.PluginConfig
		route.FromJsonWithGlobal(gjson.Parse(`{"path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"task_query","enable_path_suffixes":[],"enable_methods":[]}`), global, memoryCaptureNoopLog{})
		require.NoError(t, route.Validate())
		require.Equal(t, protocol.ProtocolUnknown, route.ProtocolForPath("/v1/chat/completions"))
	})
}

func TestManagedMemoryCompleteRuleCarrier(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		rule := map[string]interface{}{
			"_match_route_": []string{"memory-route"}, "_match_service_": []string{"mft-memory.dns"},
			"path_scope": "bound_target", "client_protocol": "openai_chat", "operation_kind": "text_generate",
			"enable_path_suffixes": []string{}, "enable_methods": []string{}, "memory_mode": "semantic", "policy_version": "7",
			"recent_window_turns": 4, "capture_response": true,
			"redis_stream":     map[string]interface{}{"service_name": "redis.memory.svc.cluster.local", "stream": memoryEventStreamName},
			"recent_cache":     map[string]interface{}{"service_name": "redis.memory.svc.cluster.local", "key_prefix": "memory:recent"},
			"console_internal": map[string]interface{}{"service_name": memoryConsoleService, "service_port": 8080, "assemble_path": memoryConsolePath, "auth_token": "memory-test-auth"},
			"tenant_header":    "x-native-tenant", "consumer_header": "x-native-consumer", "session_header": "x-native-session", "request_id_header": "x-native-request",
		}
		carrier := mustMemoryConfig(t, map[string]interface{}{"_rules_": []map[string]interface{}{rule}})
		t.Run("root stays dormant without creating dependency clients", func(t *testing.T) {
			var root config.PluginConfig
			require.NoError(t, root.ParseRoot(gjson.ParseBytes(carrier), memoryCaptureNoopLog{}))
			require.Equal(t, config.MemoryModeOff, root.Route.MemoryMode)
			require.Nil(t, root.GetRecentRedisClient())
			require.Nil(t, root.GetEventRedisClient())
		})
		for _, tc := range []struct{ name, route, cluster string }{
			{"matches original route and service", "memory-route", "outbound|80||mft-memory.dns"},
			{"different route stays unbound", "other-route", "outbound|80||mft-memory.dns"},
			{"different service stays unbound", "memory-route", "outbound|80||other-target.dns"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				host, status := newMemoryConfigTestHost(carrier)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName(tc.route))
				require.NoError(t, host.SetClusterName(tc.cluster))
				body := []byte(`{"model":"public.model","messages":[{"role":"user","content":"standalone question"}]}`)
				headers := applyMemoryRequestGatingHeaderOverrides(memoryConsoleAssembleHeaders(), [][2]string{
					{":path", "/tenant/standalone"}, {"x-native-tenant", "standalone-tenant"}, {"x-native-consumer", "standalone-consumer"},
					{"x-native-session", "standalone-session"}, {"x-native-request", "standalone-request"},
				})
				headerAction := host.CallOnHttpRequestHeaders(headers)
				bodyAction := host.CallOnHttpRequestBody(body)
				if tc.route != "memory-route" || tc.cluster != "outbound|80||mft-memory.dns" {
					require.Equal(t, types.ActionContinue, headerAction)
					require.Equal(t, types.ActionContinue, bodyAction)
					require.Empty(t, host.GetRedisCalloutAttributes())
					require.Empty(t, host.GetHttpCalloutAttributes())
					require.JSONEq(t, string(body), string(host.GetRequestBody()))
					return
				}
				require.Equal(t, types.HeaderStopIteration, headerAction)
				require.Equal(t, types.ActionPause, bodyAction)
				require.Len(t, host.GetRedisCalloutAttributes(), 1)
				host.CallOnRedisCall(0, test.CreateRedisRespNull())
				facts := requireMemoryAssembleCall(t, host)
				require.Equal(t, "semantic", facts["memory_mode"])
				require.Equal(t, "standalone question", facts["current_question"])
				require.Equal(t, "standalone-tenant", facts["tenant"])
				require.Equal(t, "standalone-consumer", facts["consumer"])
				require.Equal(t, "standalone-session", facts["session_id"])
				require.Equal(t, "standalone-request", facts["request_id"])
				require.True(t, test.HasHeaderWithValue(host.GetHttpCalloutAttributes()[0].Headers, "authorization", "Bearer memory-test-auth"))
				host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{"role": "system", "content": "standalone memory"}, nil))
				require.Contains(t, string(host.GetRequestBody()), "standalone memory")
				host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
				host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"standalone answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`))
				event := requireMemoryResponseCaptureEvent(t, host)
				require.Equal(t, "standalone-tenant", event["tenant"])
				require.Equal(t, "standalone-consumer", event["consumer"])
				require.Equal(t, "standalone-session", event["session_id"])
				require.Equal(t, "standalone answer", event["assistant_content"])
			})
		}
		t.Run("invalid nonempty root is not reinterpreted as a standalone rule carrier", func(t *testing.T) {
			invalid := mustMemoryConfig(t, map[string]interface{}{"redis_stream": map[string]interface{}{}, "_rules_": []map[string]interface{}{rule}})
			host, status := newMemoryConfigTestHost(invalid)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})
	})
}

func TestManagedMemoryAliasInjectionAndCapture(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		cases := []struct {
			clientProtocol string
			request        string
			response       string
			stream         string
		}{
			{
				clientProtocol: "openai_chat",
				request:        `{"model":"public.model","messages":[{"role":"user","content":"current question"}],"stream":%t,"tools":[{"type":"function","function":{"name":"lookup"}}]}`,
				response:       `{"choices":[{"message":{"role":"assistant","content":"alias answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`,
				stream:         "data: {\"choices\":[{\"delta\":{\"content\":\"alias answer\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":4,\"total_tokens\":13}}\n\ndata: [DONE]\n\n",
			},
			{
				clientProtocol: "anthropic_messages",
				request:        `{"model":"public.model","messages":[{"role":"user","content":[{"type":"text","text":"current question"}]}],"stream":%t,"tools":[{"name":"lookup","input_schema":{"type":"object"}}]}`,
				response:       `{"type":"message","content":[{"type":"text","text":"alias answer"}],"stop_reason":"end_turn","usage":{"input_tokens":9,"output_tokens":4}}`,
				stream:         "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":9,\"output_tokens\":0}}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"alias answer\"}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":4}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
			},
			{
				clientProtocol: "openai_responses",
				request:        `{"model":"public.model","instructions":"original instructions","input":[{"role":"user","content":[{"type":"input_text","text":"current question"}]}],"stream":%t,"tools":[{"type":"function","name":"lookup"}]}`,
				response:       `{"object":"response","output_text":"alias answer","status":"completed","usage":{"input_tokens":9,"output_tokens":4,"total_tokens":13}}`,
				stream:         "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"alias answer\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":9,\"output_tokens\":4,\"total_tokens\":13}}}\n\n",
			},
		}
		for _, tc := range cases {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/stream=%t", tc.clientProtocol, stream), func(t *testing.T) {
					alias := "/tenant/answer/" + tc.clientProtocol + "?trace=1"
					body := []byte(fmt.Sprintf(tc.request, stream))
					noStore := tc.clientProtocol == "openai_responses" && stream
					headers := [][2]string{{"x-client-protocol", "forged"}, {"x-modelfusion-operation-kind", "task_query"}}
					if noStore {
						headers = append(headers, [2]string{memoryNoStoreHeader, "true"})
					}
					host, facts := startManagedMemoryRequest(t, managedMemoryConfig(t, tc.clientProtocol, "text_generate", "semantic"), alias, body, headers)
					defer host.Reset()
					require.Equal(t, "current question", facts["current_question"])
					require.Equal(t, alias, facts["request_path"])
					host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{"role": "system", "content": "retained memory"}, nil))
					require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
					switch tc.clientProtocol {
					case "openai_chat":
						require.Contains(t, string(requireMemoryRequestField(t, host.GetRequestBody(), "messages")), "retained memory")
					case "anthropic_messages":
						require.Contains(t, requireMemoryMessagesSystemText(t, host), "retained memory")
					case "openai_responses":
						require.Contains(t, requireMemoryRequestStringField(t, host.GetRequestBody(), "instructions"), "retained memory")
					}
					require.JSONEq(t, gjson.GetBytes(body, "tools").Raw, gjson.GetBytes(host.GetRequestBody(), "tools").Raw)
					contentType := "application/json"
					if stream {
						contentType = "text/event-stream"
					}
					host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", contentType}})
					if stream {
						middle := len(tc.stream) / 2
						host.CallOnHttpStreamingResponseBody([]byte(tc.stream[:middle]), false)
						host.CallOnHttpStreamingResponseBody([]byte(tc.stream[middle:]), true)
					} else {
						host.CallOnHttpResponseBody([]byte(tc.response))
					}
					event := requireMemoryResponseCaptureEvent(t, host)
					require.Equal(t, alias, event["request_path"])
					require.Equal(t, "tenant-a", event["tenant"])
					require.Equal(t, "consumer-a", event["consumer"])
					require.Equal(t, "session-a", event["session_id"])
					require.Equal(t, stream, event["is_stream"])
					requireMemoryResponseCaptureUsage(t, event, 9, 4, 13)
					if noStore {
						requireMemoryEventOmitsField(t, event, "user_content")
						requireMemoryEventOmitsField(t, event, "assistant_content")
					} else {
						require.Equal(t, "current question", event["user_content"])
						require.Equal(t, "alias answer", event["assistant_content"])
					}
				})
			}
		}
	})
}

func TestManagedMemoryUnsupportedAndOffPassThrough(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct{ clientProtocol, operation, mode, body string }{
			{"openai_chat", "embedding", "semantic", `{"model":"public.model","input":[1,2]}`},
			{"openai_chat", "image_generate", "semantic", `{"model":"public.model","prompt":"a landscape"}`},
			{"openai_chat", "task_query", "semantic", `{"task_id":"original-task"}`},
			{"unknown", "text_generate", "semantic", `{"model":"public.model","messages":[{"role":"user","content":"do not guess"}]}`},
			{"openai_chat", "text_generate", "off", `{"model":"public.model","messages":[{"role":"user","content":"off"}]}`},
			{"openai_chat", "text_generate", "semantic", `{"model":"public.model","input":[1,2]}`},
		} {
			t.Run(tc.clientProtocol+"/"+tc.operation+"/"+tc.mode+"/"+tc.body, func(t *testing.T) {
				host, status := newMemoryConfigTestHost(managedMemoryConfig(t, tc.clientProtocol, tc.operation, tc.mode))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("memory-route"))
				// A recognizable suffix must not rescue an unsupported published binding.
				host.CallOnHttpRequestHeaders(memoryConsoleAssembleHeaders())
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody([]byte(tc.body)))
				require.JSONEq(t, tc.body, string(host.GetRequestBody()))
				require.Empty(t, host.GetRedisCalloutAttributes())
				require.Empty(t, host.GetHttpCalloutAttributes())
				require.Nil(t, host.GetLocalResponse())
			})
		}
	})
}

func TestManagedMemoryAliasKeepsIdentityScopedDigest(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		seen := map[string]bool{}
		for _, headers := range [][][2]string{nil, {{"x-mse-tenant", "tenant-b"}}, {{"x-mse-consumer", "consumer-b"}}, {{"x-mse-session", "session-b"}}} {
			host, facts := startManagedMemoryRequest(t, managedMemoryConfig(t, "openai_chat", "text_generate", "semantic"), "/tenant/answer", []byte(`{"model":"public.model","messages":[{"role":"user","content":"current question"}]}`), headers)
			digest, ok := facts["messages_digest"].(string)
			require.True(t, ok)
			require.NotEmpty(t, digest)
			require.False(t, seen[digest], "tenant, consumer, and session must each isolate the same alias request")
			seen[digest] = true
			host.Reset()
		}
	})
}

func TestManagedMemoryToolContinuationStillCaptures(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		body := []byte(`{"model":"public.model","messages":[{"role":"assistant","content":null,"tool_calls":[{"id":"original-tool","type":"function","function":{"name":"lookup","arguments":"{}"}}]},{"role":"tool","tool_call_id":"original-tool","content":"42"}]}`)
		host, _ := startManagedMemoryRequest(t, managedMemoryConfig(t, "openai_chat", "text_generate", "semantic"), "/tenant/answer", body, nil)
		defer host.Reset()
		host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "bypass", nil, nil))
		require.JSONEq(t, string(body), string(host.GetRequestBody()))
		host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
		host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":null,"tool_calls":[{"id":"next-tool","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`))
		event := requireMemoryResponseCaptureEvent(t, host)
		require.Equal(t, true, event["contains_tool_calls"])
		requireMemoryResponseCaptureUsage(t, event, 9, 4, 13)
	})
}

func managedMemoryConfig(t *testing.T, clientProtocol, operation, mode string) json.RawMessage {
	t.Helper()
	return mustMemoryConfig(t, map[string]interface{}{
		"redis_stream":         map[string]interface{}{"service_name": "redis.memory.svc.cluster.local", "stream": memoryEventStreamName},
		"console_internal":     map[string]interface{}{"service_name": memoryConsoleService, "service_port": 8080, "assemble_path": memoryConsolePath},
		"enable_path_suffixes": []string{"/legacy-only"},
		"_rules_": []map[string]interface{}{{
			"_match_route_": []string{"memory-route"}, "path_scope": "bound_target", "client_protocol": clientProtocol, "operation_kind": operation,
			"memory_mode": mode, "policy_version": "7", "capture_response": true,
		}},
	})
}

func startManagedMemoryRequest(t *testing.T, cfg json.RawMessage, path string, body []byte, overrides [][2]string) (test.TestHost, map[string]interface{}) {
	t.Helper()
	host, status := newMemoryConfigTestHost(cfg)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	headers := applyMemoryRequestGatingHeaderOverrides(memoryConsoleAssembleHeaders(), append([][2]string{{":path", path}}, overrides...))
	require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
	require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
	return host, requireMemoryAssembleCall(t, host)
}
