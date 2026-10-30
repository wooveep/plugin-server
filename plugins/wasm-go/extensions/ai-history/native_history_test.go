package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

const nativeHistoryConfig = `{"redis":{"serviceName":"redis.dns"},"path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"text_generate","public_model":"public-model","native_identity":{"release_id":"original-release","target_id":"original-target","route_id":"00000000-0000-4000-8000-000000000001","policy_id":"00000000-0000-4000-8000-000000000002","policy_version":9007199254740993}}`

func historyTestCaller() balanceidentity.Snapshot {
	return balanceidentity.Snapshot{
		AccountID: "00000000-0000-4000-8000-000000000003", APIKeyID: "00000000-0000-4000-8000-000000000004",
		CredentialGeneration: "9007199254740993", TenantID: "00000000-0000-4000-8000-000000000005",
		PrincipalID: "00000000-0000-4000-8000-000000000006",
	}
}

func historyTestHeaders(t *testing.T, body []byte, caller balanceidentity.Snapshot) [][2]string {
	t.Helper()
	basis, err := bodybasis.Capture(body)
	require.NoError(t, err)
	headers := [][2]string{{":authority", "example.com"}, {":method", "POST"}, {":path", "/alias/answer"}, {"content-type", "application/json"}}
	headers = append(headers, caller.Headers()...)
	return append(headers, basis.Headers()...)
}

func historyTestCommand(t *testing.T, raw []byte) []string {
	t.Helper()
	value, _, err := resp.NewReader(bytes.NewReader(raw)).ReadValue()
	require.NoError(t, err)
	var command []string
	for _, item := range value.Array() {
		command = append(command, item.String())
	}
	return command
}

func TestNativeHistoryConfigurationBoundary(t *testing.T) {
	var config PluginConfig
	require.NoError(t, parseHistoryBinding(gjson.Parse(nativeHistoryConfig), &config))
	require.Equal(t, int64(9007199254740993), config.historyBinding.Identity.PolicyVersion)
	for name, raw := range map[string]string{
		"partial":          `{"path_scope":"bound_target"}`,
		"client caller":    strings.Replace(nativeHistoryConfig, `"release_id":`, `"caller":{},"release_id":`, 1),
		"version string":   strings.Replace(nativeHistoryConfig, `9007199254740993`, `"9007199254740993"`, 1),
		"version fraction": strings.Replace(nativeHistoryConfig, `9007199254740993`, `1.5`, 1),
		"version overflow": strings.Replace(nativeHistoryConfig, `9007199254740993`, `9223372036854775808`, 1),
		"nil route":        strings.Replace(nativeHistoryConfig, `00000000-0000-4000-8000-000000000001`, `00000000-0000-0000-0000-000000000000`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			var bad PluginConfig
			require.Error(t, parseHistoryBinding(gjson.Parse(raw), &bad))
		})
	}
}

func TestNativeHistoryThreeProtocolsPreserveConversation(t *testing.T) {
	for _, tc := range []struct {
		Client, Container, Request, Response string
		Kind                                 protocol.ProtocolKind
	}{
		{"openai_chat", "messages", `{"model":"public-model","seed":9007199254740993,"tools":[{"type":"function","function":{"name":"f","parameters":{"type":"object"}}}],"messages":[{"role":"system","content":"current policy"},{"role":"user","content":[{"type":"text","text":"look"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AA==","detail":"high"}}]}]}`, `{"choices":[{"message":{"role":"assistant","content":"answer","annotations":[{"opaque":9007199254740993}]},"finish_reason":"stop"}]}`, protocol.ProtocolChatCompletions},
		{"anthropic_messages", "messages", `{"model":"public-model","seed":9007199254740993,"system":[{"type":"text","text":"current policy","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":[{"type":"text","text":"look"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AA=="}}]}]}`, `{"content":[{"type":"text","text":"answer","citations":[{"opaque":9007199254740993}]}],"stop_reason":"end_turn"}`, protocol.ProtocolMessages},
		{"openai_responses", "input", `{"model":"public-model","seed":9007199254740993,"instructions":"current policy","input":[{"role":"user","content":[{"type":"input_text","text":"look"},{"type":"input_image","image_url":"data:image/png;base64,AA==","detail":"high"}]}]}`, `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer","annotations":[{"opaque":9007199254740993}]}]}]}`, protocol.ProtocolResponses},
	} {
		t.Run(tc.Client, func(t *testing.T) {
			test.RunGoTest(t, func(t *testing.T) {
				configRaw := strings.Replace(nativeHistoryConfig, "openai_chat", tc.Client, 1)
				host, status := test.NewTestHost([]byte(configRaw))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				body := []byte(tc.Request)
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(historyTestHeaders(t, body, historyTestCaller())))
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
				get := historyTestCommand(t, host.GetRedisCalloutAttributes()[0].Query)
				require.Equal(t, "get", strings.ToLower(get[0]))
				require.NotContains(t, get[1], "Authorization")
				var config PluginConfig
				require.NoError(t, parseHistoryBinding(gjson.Parse(configRaw), &config))
				stored := nativeHistoryDocument{SchemaVersion: "history.native.v1", Identity: historyNativeIdentity{config.historyBinding.Identity, historyTestCaller()}, Protocol: tc.Kind, PublicModel: "public-model", Turns: []historyTurn{{
					Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"previous"}`)}, Output: []json.RawMessage{json.RawMessage(`{"role":"assistant","content":"previous answer"}`)},
				}}}
				encoded, err := json.Marshal(stored)
				require.NoError(t, err)
				host.CallOnRedisCall(0, test.CreateRedisRespString(string(encoded)))
				actual := host.GetRequestBody()
				require.Equal(t, "9007199254740993", gjson.GetBytes(actual, "seed").Raw)
				require.Equal(t, "public-model", gjson.GetBytes(actual, "model").String())
				var before, after map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(body, &before))
				require.NoError(t, json.Unmarshal(actual, &after))
				for _, name := range []string{"tools", "system", "instructions"} {
					if len(before[name]) > 0 {
						require.JSONEq(t, string(before[name]), string(after[name]))
					}
				}
				var current, injected []json.RawMessage
				require.NoError(t, json.Unmarshal(before[tc.Container], &current))
				require.NoError(t, json.Unmarshal(after[tc.Container], &injected))
				require.Len(t, injected, len(current)+2)
				require.JSONEq(t, string(current[len(current)-1]), string(injected[len(injected)-1]))
				host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
				host.CallOnHttpStreamingResponseBody([]byte(tc.Response), true)
				calls := host.GetRedisCalloutAttributes()
				require.Len(t, calls, 1)
				set := historyTestCommand(t, calls[0].Query)
				require.Equal(t, "set", strings.ToLower(set[0]))
				require.Equal(t, get[1], set[1])
				var saved nativeHistoryDocument
				require.NoError(t, json.Unmarshal([]byte(set[2]), &saved))
				require.Len(t, saved.Turns, 2)
				require.JSONEq(t, string(current[len(current)-1]), string(saved.Turns[1].Input[0]))
				require.Contains(t, string(saved.Turns[1].Output[0]), "9007199254740993")
				require.Equal(t, historyTestCaller(), saved.Identity.Caller)
			})
		})
	}
}

func TestNativeHistoryBypassesWithoutInventingLegacyIdentity(t *testing.T) {
	body := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hello"}]}`)
	for _, name := range []string{"caller", "basis", "no-store", "off", "async", "unknown protocol"} {
		t.Run(name, func(t *testing.T) {
			test.RunGoTest(t, func(t *testing.T) {
				raw := nativeHistoryConfig
				if name == "off" {
					raw = strings.Replace(raw, `"redis":`, `"enabled":false,"redis":`, 1)
				}
				if name == "async" {
					raw = strings.Replace(raw, "text_generate", "video_generate", 1)
				}
				if name == "unknown protocol" {
					raw = strings.Replace(raw, "openai_chat", "unknown", 1)
				}
				host, status := test.NewTestHost([]byte(raw))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				headers := historyTestHeaders(t, body, historyTestCaller())
				if name == "caller" {
					headers = append(headers[:4], headers[len(headers)-2:]...)
				}
				if name == "basis" {
					headers = headers[:len(headers)-2]
				}
				headers = append(headers, [2]string{"authorization", "must-not-be-a-legacy-key"})
				if name == "no-store" {
					headers = append(headers, [2]string{"cache-control", "private, no-store"})
				}
				host.CallOnHttpRequestHeaders(headers)
				host.CallOnHttpRequestBody(body)
				require.Empty(t, host.GetRedisCalloutAttributes())
			})
		})
	}
}

func TestNativeHistoryStreamCaptureAndIncompleteOutput(t *testing.T) {
	for _, tc := range []struct {
		Kind protocol.ProtocolKind
		Body string
	}{
		{protocol.ProtocolChatCompletions, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"},
		{protocol.ProtocolMessages, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"},
		{protocol.ProtocolResponses, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"},
	} {
		t.Run(string(tc.Kind), func(t *testing.T) {
			for split := 1; split < len(tc.Body); split++ {
				output, reason := historyResponseOutput(tc.Kind, 200, true, [][]byte{[]byte(tc.Body[:split]), []byte(tc.Body[split:])})
				require.Empty(t, reason, fmt.Sprint(split))
				require.Len(t, output, 1)
				require.Contains(t, string(output[0]), "hello")
			}
		})
	}
	for name, body := range map[string]string{
		"truncated": "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n",
		"tool":      "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"id\":\"call1\"}]},\"finish_reason\":\"tool_calls\"}]}\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			output, reason := historyResponseOutput(protocol.ProtocolChatCompletions, 200, true, [][]byte{[]byte(body)})
			require.Empty(t, output)
			require.NotEmpty(t, reason)
		})
	}
}

func TestNativeHistoryPreservesToolContextAndPreciseNumbers(t *testing.T) {
	body := []byte(`{"model":"public-model","messages":[{"role":"system","content":"policy"},{"role":"user","content":[{"type":"image_url","image_url":{"url":"image-A"}}]},{"role":"assistant","content":null,"tool_calls":[{"id":"t","function":{"name":"f","arguments":"{}"}}]},{"role":"tool","tool_call_id":"t","content":"result"}],"metadata":{"Number":9007199254740993,"RawValue":0.12345678901234567890123456789}}`)
	request, err := parseHistoryRequest(protocol.ProtocolChatCompletions, body, "public-model")
	require.NoError(t, err)
	injected, err := request.inject([]historyTurn{{Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"before"}`)}, Output: []json.RawMessage{json.RawMessage(`{"role":"assistant","content":"answer"}`)}}}, 3, body)
	require.NoError(t, err)
	require.Equal(t, "9007199254740993", gjson.GetBytes(injected, "metadata.Number").Raw)
	require.Equal(t, "0.12345678901234567890123456789", gjson.GetBytes(injected, "metadata.RawValue").Raw)
	require.Equal(t, "policy", gjson.GetBytes(injected, "messages.0.content").String())
	require.Equal(t, "image-A", gjson.GetBytes(injected, "messages.3.content.0.image_url.url").String())
	require.Equal(t, "t", gjson.GetBytes(injected, "messages.4.tool_calls.0.id").String())
	require.Equal(t, "t", gjson.GetBytes(injected, "messages.5.tool_call_id").String())
	require.Len(t, request.Turn, 3)
}

func TestNativeHistoryRuleOnlyScope(t *testing.T) {
	rule := nativeHistoryConfig[:len(nativeHistoryConfig)-1] + `,"_match_route_":["published-history-route"]}`
	for _, route := range []string{"published-history-route", "other-route"} {
		t.Run(route, func(t *testing.T) {
			test.RunGoTest(t, func(t *testing.T) {
				host, status := test.NewTestHost([]byte(`{"_rules_":[` + rule + `]}`))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName(route))
				body := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hello"}]}`)
				headers := historyTestHeaders(t, body, historyTestCaller())
				headers = append(headers, [2]string{"accept-encoding", "gzip"})
				host.CallOnHttpRequestHeaders(headers)
				action := host.CallOnHttpRequestBody(body)
				if route == "other-route" {
					require.Equal(t, types.ActionContinue, action)
					require.Empty(t, host.GetRedisCalloutAttributes())
					require.Equal(t, body, host.GetRequestBody())
					require.True(t, test.HasHeaderWithValue(host.GetRequestHeaders(), "accept-encoding", "gzip"))
				} else {
					require.Equal(t, types.ActionPause, action)
					require.Len(t, host.GetRedisCalloutAttributes(), 1)
				}
			})
		})
	}
}

func TestNativeHistoryAllIdentityComponentsIsolateStorage(t *testing.T) {
	body := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hello"}]}`)
	var keys []string
	for index := 0; index < 11; index++ {
		test.RunGoTest(t, func(t *testing.T) {
			raw := nativeHistoryConfig
			caller := historyTestCaller()
			switch index {
			case 1:
				caller.AccountID = "00000000-0000-4000-8000-000000000010"
			case 2:
				caller.APIKeyID = "00000000-0000-4000-8000-000000000010"
			case 3:
				caller.CredentialGeneration = "9007199254740994"
			case 4:
				caller.TenantID = "00000000-0000-4000-8000-000000000010"
			case 5:
				caller.PrincipalID = "00000000-0000-4000-8000-000000000010"
			case 6:
				raw = strings.Replace(raw, "original-release", "another-release", 1)
			case 7:
				raw = strings.Replace(raw, "original-target", "another-target", 1)
			case 8:
				raw = strings.Replace(raw, "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000010", 1)
			case 9:
				raw = strings.Replace(raw, "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000010", 1)
			case 10:
				raw = strings.Replace(raw, "9007199254740993", "9007199254740994", 1)
			}
			host, status := test.NewTestHost([]byte(raw))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.CallOnHttpRequestHeaders(historyTestHeaders(t, body, caller))
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
			command := historyTestCommand(t, host.GetRedisCalloutAttributes()[0].Query)
			for _, key := range keys {
				require.NotEqual(t, key, command[1])
			}
			keys = append(keys, command[1])
		})
	}
}

func TestNativeHistoryReadFailureDoesNotOverwriteUnseenHistory(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		test.RunGoTest(t, func(t *testing.T) {
			host, _ := test.NewTestHost([]byte(nativeHistoryConfig))
			defer host.Reset()
			body := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hello"}]}`)
			host.CallOnHttpRequestHeaders(historyTestHeaders(t, body, historyTestCaller()))
			host.CallOnHttpRequestBody(body)
			if corrupt {
				host.CallOnRedisCall(0, test.CreateRedisRespString(`{"schema_version":"history.native.v1","native_identity":{}}`))
			} else {
				host.CallOnRedisCall(0, []byte("-ERR unavailable\r\n"))
			}
			require.Equal(t, body, host.GetRequestBody())
			host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
			host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}]}`))
			require.Empty(t, host.GetRedisCalloutAttributes())
		})
	}
}

func TestNativeHistoryNonTextOutputDoesNotBecomeTextHistory(t *testing.T) {
	for _, tc := range []struct {
		Kind protocol.ProtocolKind
		Body string
	}{
		{protocol.ProtocolChatCompletions, `{"choices":[{"message":{"role":"assistant","content":"caption","audio":{"data":"AA=="}},"finish_reason":"stop"}]}`},
		{protocol.ProtocolMessages, `{"content":[{"type":"text","text":"caption"},{"type":"image","source":{"data":"AA=="}}],"stop_reason":"end_turn"}`},
		{protocol.ProtocolResponses, `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"caption"}]},{"type":"image_generation_call","result":"AA=="}]}`},
	} {
		output, reason := historyResponseOutput(tc.Kind, 200, false, [][]byte{[]byte(tc.Body)})
		require.Empty(t, output)
		require.Equal(t, "non-text-output", reason)
	}
}

func TestNativeHistoryMultiTurnAttachmentsDoNotInheritSingleFactBudget(t *testing.T) {
	var config PluginConfig
	require.NoError(t, parseHistoryBinding(gjson.Parse(nativeHistoryConfig), &config))
	requestBody := []byte(`{"model":"public-model","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,` + strings.Repeat("A", 400_000) + `"}}]}]}`)
	request, err := parseHistoryRequest(protocol.ProtocolChatCompletions, requestBody, "public-model")
	require.NoError(t, err)
	document := nativeHistoryDocument{SchemaVersion: "history.native.v1", Identity: historyNativeIdentity{config.historyBinding.Identity, historyTestCaller()}, Protocol: protocol.ProtocolChatCompletions, PublicModel: "public-model"}
	for i := 0; i < 3; i++ {
		document.Turns = append(document.Turns, historyTurn{Input: request.Turn, Output: []json.RawMessage{json.RawMessage(`{"role":"assistant","content":"answer"}`)}})
	}
	raw, err := json.Marshal(document)
	require.NoError(t, err)
	require.Greater(t, len(raw), 1_048_576)
	stored, err := readNativeHistory(raw, document)
	require.NoError(t, err)
	injected, err := request.inject(stored.Turns, 3, requestBody)
	require.NoError(t, err)
	require.Equal(t, 4, strings.Count(string(injected), "data:image/png;base64,"))
	_, err = parseHistoryRequest(protocol.ProtocolChatCompletions, injected, "public-model")
	require.NoError(t, err)
}

func TestNativeHistoryTTLAndResponseNoStore(t *testing.T) {
	for _, noStore := range []bool{false, true} {
		test.RunGoTest(t, func(t *testing.T) {
			raw := strings.Replace(nativeHistoryConfig, `"redis":`, `"cacheTTL":60,"redis":`, 1)
			host, _ := test.NewTestHost([]byte(raw))
			defer host.Reset()
			body := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hello"}]}`)
			host.CallOnHttpRequestHeaders(historyTestHeaders(t, body, historyTestCaller()))
			host.CallOnHttpRequestBody(body)
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
			if noStore {
				headers = append(headers, [2]string{"cache-control", "no-store"})
			}
			host.CallOnHttpResponseHeaders(headers)
			host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}]}`))
			calls := host.GetRedisCalloutAttributes()
			if noStore {
				require.Empty(t, calls)
				return
			}
			require.Len(t, calls, 1)
			command := historyTestCommand(t, calls[0].Query)
			require.Len(t, command, 5)
			require.Equal(t, []string{"EX", "60"}, command[3:])
			host.CallOnRedisCall(0, []byte("+OK\r\n"))
			require.Empty(t, host.GetRedisCalloutAttributes(), "TTL is part of the same write")
		})
	}
}

func TestNativeHistoryAsynchronousAndNonTextRequestsBypass(t *testing.T) {
	for _, tc := range []struct{ Client, Body string }{
		{"openai_chat", `{"model":"public-model","modalities":["text","audio"],"messages":[{"role":"user","content":"hello"}]}`},
		{"openai_responses", `{"model":"public-model","background":true,"input":"hello"}`},
	} {
		test.RunGoTest(t, func(t *testing.T) {
			host, _ := test.NewTestHost([]byte(strings.Replace(nativeHistoryConfig, "openai_chat", tc.Client, 1)))
			defer host.Reset()
			host.CallOnHttpRequestHeaders(historyTestHeaders(t, []byte(tc.Body), historyTestCaller()))
			host.CallOnHttpRequestBody([]byte(tc.Body))
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Equal(t, []byte(tc.Body), host.GetRequestBody())
		})
	}
}
