package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func publishedCacheConfig(t *testing.T, raw json.RawMessage, clientProtocol, operationKind string) json.RawMessage {
	t.Helper()
	var err error
	raw, err = sjson.SetBytes(raw, "client_protocol", clientProtocol)
	require.NoError(t, err)
	raw, err = sjson.SetBytes(raw, "operation_kind", operationKind)
	require.NoError(t, err)
	raw, err = sjson.SetRawBytes(raw, "route_policy.enabled_path_suffixes", []byte(`[]`))
	require.NoError(t, err)
	return raw
}

func TestPublishedCacheCustomPathReplayAndCapture(t *testing.T) {
	for _, tc := range []struct {
		clientProtocol string
		kind           protocol.ProtocolKind
		request        string
		response       string
		streamChunk    string
	}{
		{
			"openai_chat", protocol.ProtocolChatCompletions,
			`{"model":"qwen-turbo","messages":[{"role":"user","content":"weather?"}]}`,
			`{"choices":[{"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`,
			`{"choices":[{"delta":{"content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`,
		},
		{
			"openai_responses", protocol.ProtocolResponses,
			`{"model":"qwen-turbo","input":[{"role":"user","content":[{"type":"input_text","text":"weather?"}]}]}`,
			`{"output_text":"answer","status":"completed","usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}}`,
			`{"type":"response.completed","response":{"status":"completed","output_text":"answer","usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}}}`,
		},
		{
			"anthropic_messages", protocol.ProtocolMessages,
			`{"model":"qwen-turbo","messages":[{"role":"user","content":[{"type":"text","text":"weather?"}]}]}`,
			`{"type":"message","role":"assistant","content":[{"type":"text","text":"answer"}],"stop_reason":"end_turn","usage":{"input_tokens":7,"output_tokens":3}}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
		},
	} {
		test.RunTest(t, func(t *testing.T) {
			t.Run(tc.clientProtocol, func(t *testing.T) {
				for _, stream := range []bool{false, true} {
					t.Run(map[bool]string{false: "json", true: "sse"}[stream], func(t *testing.T) {
						cfg := publishedCacheConfig(t, thinResponseCaptureConfig(t), tc.clientProtocol, "text_generate")
						host, status := test.NewTestHost(cfg)
						require.Equal(t, types.OnPluginStartStatusOK, status)
						defer host.Reset()
						require.NoError(t, host.SetRouteName("test-route-default"))
						// The path intentionally resembles a different protocol. Only the
						// trusted published protocol may choose the adapter.
						path := "/tenant/custom/v1/messages?alias=1"
						action := host.CallOnHttpRequestHeaders([][2]string{
							{":authority", "example.com"}, {":path", path}, {":method", "POST"},
							{"content-type", "application/json"}, {"x-mse-tenant", "tenant-a"},
							{"x-mse-consumer", "consumer-a"}, {"x-client-protocol", "untrusted"},
						})
						require.Equal(t, types.HeaderStopIteration, action)
						body, err := sjson.SetBytes([]byte(tc.request), "stream", stream)
						require.NoError(t, err)
						require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
						require.Len(t, host.GetRedisCalloutAttributes(), 1)
						adapter, ok := thinProtocolAdapterForKind(tc.kind)
						require.True(t, ok)
						digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{Method: "POST", Path: path, Body: body})
						require.NoError(t, err)
						now := time.Now().Unix()
						record := MaterializedReplayRecord{
							SchemaVersion: materializedRecordSchemaVersion, Tenant: "tenant-a", Consumer: "consumer-a",
							Route: "test-route-default", Model: "qwen-turbo", Protocol: tc.kind,
							CacheScope: "consumer", CachePolicyVersion: "policy-v1", RequestDigest: digest.Digest,
							SoftExpiresAt: now + 60, HardExpiresAt: now + 3600,
							Response: json.RawMessage(tc.response), Usage: json.RawMessage(`{"input_tokens":7,"output_tokens":3}`),
							FinishReason: "stop", StreamReplayable: stream,
						}
						if stream {
							record.StreamChunks = []json.RawMessage{json.RawMessage(tc.streamChunk)}
						}
						raw, err := json.Marshal(record)
						require.NoError(t, err)
						host.CallOnRedisCall(0, test.CreateRedisRespString(string(raw)))
						response := host.GetLocalResponse()
						require.NotNil(t, response, "custom paths must replay the selected protocol")
						require.Equal(t, uint32(200), response.StatusCode)
						if stream {
							require.Contains(t, thinReplayPayloadHeader(response.Headers, "content-type"), "text/event-stream")
							require.Contains(t, string(response.Data), tc.streamChunk)
						} else {
							require.JSONEq(t, tc.response, string(response.Data))
						}
						require.Empty(t, host.GetHttpCalloutAttributes())
					})
				}

				t.Run("miss captures original protocol", func(t *testing.T) {
					cfg := publishedCacheConfig(t, thinResponseCaptureConfig(t), tc.clientProtocol, "text_generate")
					host, status := test.NewTestHost(cfg)
					require.Equal(t, types.OnPluginStartStatusOK, status)
					defer host.Reset()
					require.NoError(t, host.SetRouteName("test-route-default"))
					host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "example.com"}, {":path", "/customer/answer"}, {":method", "POST"},
						{"content-type", "application/json"}, {"x-mse-tenant", "tenant-a"}, {"x-mse-consumer", "consumer-a"},
					})
					require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(tc.request)))
					host.CallOnRedisCall(0, test.CreateRedisRespNull())
					host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
					host.CallOnHttpResponseBody([]byte(tc.response))
					event, found := thinResponseCaptureEvent(t, host)
					require.True(t, found, "cache miss must capture the declared client protocol on a custom path")
					require.Equal(t, string(tc.kind), event["protocol"])
					require.Equal(t, "/customer/answer", event["request_path"])
					require.Equal(t, "weather?", event["user_content"])
					require.Equal(t, "answer", event["assistant_content"])
				})
			})
		})
	}
}

func TestPublishedCacheGatesAndConsoleLookup(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			field   string
			value   string
			headers [][2]string
		}{
			{"no store", "", "", [][2]string{{"cache-control", "no-store"}}},
			{"explicit bypass", "route_policy.enable_bypass", "true", nil},
			{"consumer isolation", "", "", [][2]string{{"x-mse-consumer", ""}}},
			{"explicit path intersection", "route_policy.enabled_path_suffixes", `["/only-this-path"]`, nil},
			{"media operation", "operation_kind", `"image_generate"`, nil},
			{"query operation", "operation_kind", `"query"`, nil},
			{"unknown protocol", "client_protocol", `"unknown"`, nil},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cfg := publishedCacheConfig(t, thinRequestGatingConfig(t, "consumer", false), "openai_chat", "text_generate")
				if tc.field != "" {
					var err error
					cfg, err = sjson.SetRawBytes(cfg, tc.field, []byte(tc.value))
					require.NoError(t, err)
				}
				host, _, _ := startThinRequestGatingRequest(t, cfg, tc.headers)
				defer host.Reset()
				require.Empty(t, host.GetRedisCalloutAttributes())
				require.Empty(t, host.GetHttpCalloutAttributes())
			})
		}
		t.Run("Console miss receives selected protocol on custom path", func(t *testing.T) {
			cfg := publishedCacheConfig(t, thinConsoleLookupConfig(t, true), "openai_chat", "text_generate")
			host, _, bodyAction := startThinRequestGatingRequest(t, cfg, [][2]string{
				{":path", "/customer/custom-answer"},
				{"x-mse-session", "session-a"},
			})
			defer host.Reset()
			require.Equal(t, types.ActionPause, bodyAction)
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			request := requireThinConsoleLookupCall(t, host)
			require.Equal(t, "chat_completions", request["protocol"])
			require.Equal(t, "weather?", request["semantic_query_text"])
		})
	})
}

func TestPublishedCacheRuleOnlyControllerConfig(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		rule := publishedCacheConfig(t, thinResponseCaptureConfig(t), "openai_responses", "text_generate")
		rule, err := sjson.SetRawBytes(rule, "_match_route_", []byte(`["bound-cache-route"]`))
		require.NoError(t, err)
		controllerConfig := append([]byte(`{"_rules_":[`), rule...)
		controllerConfig = append(controllerConfig, []byte(`]}`)...)
		for _, route := range []string{"bound-cache-route", "unrelated-route"} {
			t.Run(route, func(t *testing.T) {
				host, status := test.NewTestHost(controllerConfig)
				require.Equal(t, types.OnPluginStartStatusOK, status,
					"the matcher parses complete rules without enabling a global config")
				defer host.Reset()
				require.NoError(t, host.SetRouteName(route))
				body := []byte(`{"model":"qwen-turbo","input":"weather?"}`)
				headerAction := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"}, {":path", "/customer/custom-answer"}, {":method", "POST"},
					{"content-type", "application/json"}, {"x-mse-tenant", "tenant-a"},
					{"x-mse-consumer", "consumer-a"}, {"accept-encoding", "gzip"},
				})
				bodyAction := host.CallOnHttpRequestBody(body)
				if route == "unrelated-route" {
					require.Equal(t, types.ActionContinue, headerAction)
					require.Equal(t, types.ActionContinue, bodyAction)
					require.Empty(t, host.GetRedisCalloutAttributes())
					require.Empty(t, host.GetHttpCalloutAttributes())
					require.True(t, test.HasHeaderWithValue(host.GetRequestHeaders(), "accept-encoding", "gzip"))
					require.Equal(t, body, host.GetRequestBody())
					return
				}
				require.Equal(t, types.HeaderStopIteration, headerAction)
				require.Equal(t, types.ActionPause, bodyAction)
				require.Len(t, host.GetRedisCalloutAttributes(), 1)
				host.CallOnRedisCall(0, test.CreateRedisRespNull())
				host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
				host.CallOnHttpResponseBody([]byte(`{"output_text":"answer","status":"completed","usage":{"input_tokens":7,"output_tokens":3}}`))
				event, found := thinResponseCaptureEvent(t, host)
				require.True(t, found)
				require.Equal(t, "responses", event["protocol"])
				require.Equal(t, "bound-cache-route", event["route"])
				require.Equal(t, "policy-v1", event["cache_policy_version"])
				require.Equal(t, "tenant-a", event["tenant"])
				require.Equal(t, "consumer-a", event["consumer"])
				require.Equal(t, "weather?", event["user_content"])
				require.Equal(t, "answer", event["assistant_content"])
			})
		}
		t.Run("empty global config remains invalid", func(t *testing.T) {
			host, status := test.NewTestHost([]byte(`{}`))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})
	})
}

func TestPublishedCacheMultimodalInputPreservesAttachmentIdentity(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			clientProtocol string
			kind           protocol.ProtocolKind
			field          string
			pure           string
			mixedA         string
			mixedB         string
			response       string
		}{
			{
				"openai_chat", protocol.ProtocolChatCompletions, "messages",
				`{"model":"vision-model","messages":[{"role":"user","content":"weather?"}]}`,
				`{"model":"vision-model","messages":[{"role":"user","content":[{"type":"text","text":"weather?"},{"type":"image_url","image_url":{"url":"https://fixture.invalid/a.png","detail":"high"}}]}]}`,
				`{"model":"vision-model","messages":[{"role":"user","content":[{"type":"text","text":"weather?"},{"type":"image_url","image_url":{"url":"https://fixture.invalid/b.png","detail":"high"}}]}]}`,
				`{"choices":[{"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3}}`,
			},
			{
				"openai_responses", protocol.ProtocolResponses, "input",
				`{"model":"vision-model","input":"weather?"}`,
				`{"model":"vision-model","input":[{"role":"user","content":[{"type":"input_text","text":"weather?"},{"type":"input_image","file_id":"file-a","detail":"high"}]}]}`,
				`{"model":"vision-model","input":[{"role":"user","content":[{"type":"input_text","text":"weather?"},{"type":"input_image","file_id":"file-b","detail":"high"}]}]}`,
				`{"output_text":"answer","status":"completed","usage":{"input_tokens":7,"output_tokens":3}}`,
			},
			{
				"anthropic_messages", protocol.ProtocolMessages, "messages",
				`{"model":"vision-model","messages":[{"role":"user","content":"weather?"}]}`,
				`{"model":"vision-model","messages":[{"role":"user","content":[{"type":"text","text":"weather?"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"YQ=="}}]}]}`,
				`{"model":"vision-model","messages":[{"role":"user","content":[{"type":"text","text":"weather?"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"Yg=="}}]}]}`,
				`{"type":"message","content":[{"type":"text","text":"answer"}],"stop_reason":"end_turn","usage":{"input_tokens":7,"output_tokens":3}}`,
			},
		} {
			t.Run(tc.clientProtocol, func(t *testing.T) {
				keys := make(map[string]string)
				for _, variant := range []struct{ name, body string }{
					{"pure", tc.pure}, {"mixed-a", tc.mixedA}, {"mixed-b", tc.mixedB},
				} {
					t.Run(variant.name, func(t *testing.T) {
						cfg := publishedCacheConfig(t, thinResponseCaptureConfig(t), tc.clientProtocol, "text_generate")
						host, status := test.NewTestHost(cfg)
						require.Equal(t, types.OnPluginStartStatusOK, status)
						defer host.Reset()
						require.NoError(t, host.SetRouteName("multimodal-text-route"))
						host.CallOnHttpRequestHeaders([][2]string{
							{":authority", "example.com"}, {":path", "/custom/text-answer"}, {":method", "POST"},
							{"content-type", "application/json"}, {"x-mse-tenant", "tenant-a"}, {"x-mse-consumer", "consumer-a"},
						})
						body := []byte(variant.body)
						require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
						require.Equal(t, body, host.GetRequestBody(), "attachments must reach upstream unchanged")
						calls := host.GetRedisCalloutAttributes()
						require.Len(t, calls, 1)
						command, ok := thinResponseCaptureCommand(t, calls[0].Query)
						require.True(t, ok)
						require.Len(t, command, 2)
						require.Equal(t, "get", command[0])
						keys[variant.name] = command[1]
						adapter, ok := thinProtocolAdapterForKind(tc.kind)
						require.True(t, ok)
						digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{Method: "POST", Path: "/custom/text-answer", Body: body})
						require.NoError(t, err)
						require.JSONEq(t, gjson.GetBytes(body, tc.field).Raw, string(digest.Input.Fields[tc.field]))
						host.CallOnRedisCall(0, test.CreateRedisRespNull())
						host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
						host.CallOnHttpResponseBody([]byte(tc.response))
						event, found := thinResponseCaptureEvent(t, host)
						require.True(t, found, "supported multimodal input must not blanket-bypass text-output cache")
						require.Equal(t, digest.Digest, event["request_digest"])
						require.Equal(t, "weather?", event["user_content"])
						require.Equal(t, "answer", event["assistant_content"])
					})
				}
				require.NotEmpty(t, keys["pure"])
				require.NotEmpty(t, keys["mixed-a"])
				require.NotEmpty(t, keys["mixed-b"])
				require.NotEqual(t, keys["pure"], keys["mixed-a"])
				require.NotEqual(t, keys["mixed-a"], keys["mixed-b"], "equal prompt text must not collapse distinct attachments")
			})
		}
	})
}
