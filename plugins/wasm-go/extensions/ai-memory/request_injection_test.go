package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

const (
	memoryConsoleService  = "console.internal.svc.cluster.local"
	memoryConsoleUpstream = "outbound|8080||console.internal.svc.cluster.local"
	memoryConsolePath     = "/internal/memory/assemble"
)

func TestMemoryRequestInjection(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("assemble facts use latest user and stable digest", func(t *testing.T) {
			firstHost, firstFacts := startMemoryInjectionAndRequireAssembleWithoutCleanup(t, []byte(`{
				"model": "qwen-turbo",
				"stream": false,
				"temperature": 0.2,
				"messages": [
					{"role": "system", "content": "answer tersely"},
					{"role": "user", "content": "older question"},
					{"role": "assistant", "content": "older answer"},
					{"role": "user", "content": "latest memory intent"}
				],
				"metadata": {"b": 2, "a": 1}
			}`))
			firstHost.Reset()

			_, secondFacts := startMemoryInjectionAndRequireAssemble(t, []byte(`{"metadata":{"a":1,"b":2},"messages":[{"content":"answer tersely","role":"system"},{"content":"older question","role":"user"},{"content":"older answer","role":"assistant"},{"content":"latest memory intent","role":"user"}],"temperature":0.2,"stream":false,"model":"qwen-turbo"}`))

			require.Equal(t, "latest memory intent", firstFacts["current_question"])
			require.Equal(t, "latest memory intent", secondFacts["current_question"])
			require.Equal(t, "semantic", firstFacts["memory_mode"])
			require.Equal(t, "7", firstFacts["policy_version"])
			require.Equal(t, "tenant-a", firstFacts["tenant"])
			require.Equal(t, "consumer-a", firstFacts["consumer"])
			require.Equal(t, "session-a", firstFacts["session_id"])
			require.Equal(t, "request-injection-1", firstFacts["request_id"])
			require.Equal(t, "/v1/chat/completions", firstFacts["request_path"])
			require.Equal(t, map[string]interface{}{"name": "memory-route"}, firstFacts["route"])
			require.Equal(t, map[string]interface{}{"name": "qwen-turbo"}, firstFacts["model"])
			require.EqualValues(t, 6, firstFacts["recent_window_turns"])
			require.EqualValues(t, 1500, firstFacts["memory_token_budget"])
			require.EqualValues(t, 3, firstFacts["semantic_top_k"])

			firstDigest, ok := firstFacts["messages_digest"].(string)
			require.True(t, ok, "assemble facts must include a string messages_digest")
			require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{64}$`), firstDigest)
			require.Equal(t, firstDigest, secondFacts["messages_digest"])
		})

		t.Run("injected body preserves high-priority messages before memory and recent messages", func(t *testing.T) {
			host, _ := startMemoryInjectionAndRequireAssemble(t, []byte(`{
				"model": "qwen-turbo",
				"messages": [
					{"role": "system", "content": "client system"},
					{"role": "developer", "content": "client developer"},
					{"role": "user", "content": "current question"}
				],
				"metadata": {"trace": "keep"}
			}`))

			host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{
				"role":    "system",
				"content": "Console memory context",
			}, []map[string]interface{}{
				{"role": "user", "content": "recent user"},
				{"role": "assistant", "content": "recent assistant"},
			}))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			messages := requireMemoryRequestMessages(t, host)
			require.Len(t, messages, 6)
			requireMemoryMessage(t, messages[0], "system", "client system")
			requireMemoryMessage(t, messages[1], "developer", "client developer")
			requireMemoryMessage(t, messages[2], "system", "Console memory context")
			requireMemoryMessage(t, messages[3], "user", "recent user")
			requireMemoryMessage(t, messages[4], "assistant", "recent assistant")
			requireMemoryMessage(t, messages[5], "user", "current question")
			require.JSONEq(t, `{"trace":"keep"}`, string(requireMemoryRequestField(t, host.GetRequestBody(), "metadata")))
		})

		t.Run("multi-user-turn current request avoids duplicate recent injection", func(t *testing.T) {
			host, _ := startMemoryInjectionAndRequireAssemble(t, []byte(`{
				"model": "qwen-turbo",
				"messages": [
					{"role": "system", "content": "client system"},
					{"role": "user", "content": "already present user"},
					{"role": "assistant", "content": "already present assistant"},
					{"role": "user", "content": "latest follow up"}
				]
			}`))

			host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{
				"role":    "system",
				"content": "Console memory context",
			}, []map[string]interface{}{
				{"role": "user", "content": "already present user"},
				{"role": "assistant", "content": "already present assistant"},
			}))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			messages := requireMemoryRequestMessages(t, host)
			require.Len(t, messages, 5)
			requireMemoryMessage(t, messages[0], "system", "client system")
			requireMemoryMessage(t, messages[1], "system", "Console memory context")
			requireMemoryMessage(t, messages[2], "user", "already present user")
			requireMemoryMessage(t, messages[3], "assistant", "already present assistant")
			requireMemoryMessage(t, messages[4], "user", "latest follow up")
		})

		t.Run("messages protocol injects memory context into system blocks", func(t *testing.T) {
			host, _ := startMemoryProtocolInjectionAndRequireAssemble(t, "/v1/messages", []byte(`{
				"model": "deepseek-v4-flash",
				"messages": [{"role": "user", "content": [{"type":"text","text":"messages current question"}]}]
			}`))

			host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{
				"role":    "system",
				"content": "Console memory context",
			}, []map[string]interface{}{
				{"role": "user", "content": "recent user"},
				{"role": "assistant", "content": "recent assistant"},
			}))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			systemText := requireMemoryMessagesSystemText(t, host)
			require.Contains(t, systemText, "system: Console memory context")
			require.Contains(t, systemText, "user: recent user")
			require.Contains(t, systemText, "assistant: recent assistant")
			require.JSONEq(t, `[{"role":"user","content":[{"type":"text","text":"messages current question"}]}]`, string(requireMemoryRequestField(t, host.GetRequestBody(), "messages")))
		})

		t.Run("responses protocol injects memory context into instructions", func(t *testing.T) {
			host, _ := startMemoryProtocolInjectionAndRequireAssemble(t, "/v1/responses", []byte(`{
				"model": "deepseek-v4-flash",
				"instructions": "client instructions",
				"input": [{"role":"user","content":[{"type":"input_text","text":"responses current question"}]}]
			}`))

			host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{
				"role":    "system",
				"content": "Console memory context",
			}, []map[string]interface{}{
				{"role": "user", "content": "recent user"},
				{"role": "assistant", "content": "recent assistant"},
			}))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			instructions := requireMemoryRequestStringField(t, host.GetRequestBody(), "instructions")
			require.Contains(t, instructions, "client instructions")
			require.Contains(t, instructions, "system: Console memory context")
			require.Contains(t, instructions, "user: recent user")
			require.Contains(t, instructions, "assistant: recent assistant")
			require.JSONEq(t, `[{"role":"user","content":[{"type":"input_text","text":"responses current question"}]}]`, string(requireMemoryRequestField(t, host.GetRequestBody(), "input")))
		})
	})
}

func memoryInjectionConfig(t *testing.T) json.RawMessage {
	t.Helper()
	return memoryInjectionConfigForPath(t, "/v1/chat/completions")
}

func memoryInjectionConfigForPath(t *testing.T, path string) json.RawMessage {
	t.Helper()
	return mustMemoryConfig(t, map[string]interface{}{
		"redis_stream": map[string]interface{}{
			"service_name": "redis.memory.svc.cluster.local",
		},
		"recent_cache": map[string]interface{}{
			"service_name": "redis.recent.svc.cluster.local",
		},
		"console_internal": map[string]interface{}{
			"service_name":  memoryConsoleService,
			"service_port":  8080,
			"assemble_path": memoryConsolePath,
			"timeout_ms":    120,
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"session_header":       "x-mse-session",
		"request_id_header":    "x-request-id",
		"enable_path_suffixes": []string{path},
		"fail_policy":          "open",
		"_rules_": []map[string]interface{}{
			{
				"_match_route_":       []string{"memory-route"},
				"memory_mode":         "semantic",
				"recent_window_turns": 6,
				"memory_token_budget": 1500,
				"assemble_timeout_ms": 80,
				"inject_role":         "system",
				"semantic_top_k":      3,
				"capture_response":    true,
				"policy_version":      "7",
			},
		},
	})
}

func startMemoryInjectionAndRequireAssemble(t *testing.T, body []byte) (test.TestHost, map[string]interface{}) {
	t.Helper()
	host, facts := startMemoryInjectionAndRequireAssembleWithoutCleanup(t, body)
	t.Cleanup(host.Reset)
	return host, facts
}

func startMemoryInjectionAndRequireAssembleWithoutCleanup(t *testing.T, body []byte) (test.TestHost, map[string]interface{}) {
	t.Helper()
	return startMemoryProtocolInjectionAndRequireAssembleWithoutCleanup(t, "/v1/chat/completions", body)
}

func startMemoryProtocolInjectionAndRequireAssemble(t *testing.T, path string, body []byte) (test.TestHost, map[string]interface{}) {
	t.Helper()
	host, facts := startMemoryProtocolInjectionAndRequireAssembleWithoutCleanup(t, path, body)
	t.Cleanup(host.Reset)
	return host, facts
}

func startMemoryProtocolInjectionAndRequireAssembleWithoutCleanup(t *testing.T, path string, body []byte) (test.TestHost, map[string]interface{}) {
	t.Helper()
	host, status := newMemoryConfigTestHost(memoryInjectionConfigForPath(t, path))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	require.NoError(t, host.SetRequestId("property-request-id"))

	headerAction := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", path},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
		{"x-mse-session", "session-a"},
		{"x-request-id", "request-injection-1"},
	})
	require.Equal(t, types.HeaderStopIteration, headerAction)

	bodyAction := host.CallOnHttpRequestBody(body)
	require.Equal(t, types.ActionPause, bodyAction)
	if len(host.GetRedisCalloutAttributes()) > 0 {
		host.CallOnRedisCall(0, test.CreateRedisRespNull())
	}
	return host, requireMemoryAssembleCall(t, host)
}

func requireMemoryAssembleCall(t *testing.T, host test.TestHost) map[string]interface{} {
	t.Helper()
	callouts := host.GetHttpCalloutAttributes()
	require.Len(t, callouts, 1, "memory-enabled request should issue one Console assemble callout")
	callout := callouts[0]
	require.Equal(t, memoryConsoleUpstream, callout.Upstream)
	require.True(t, test.HasHeaderWithValue(callout.Headers, ":method", "POST"))
	require.True(t, test.HasHeaderWithValue(callout.Headers, ":path", memoryConsolePath))
	require.True(t, test.HasHeaderWithValue(callout.Headers, "content-type", "application/json"))

	var facts map[string]interface{}
	require.NoErrorf(t, json.Unmarshal(callout.Body, &facts), "Console assemble body must be JSON; body=%s", string(callout.Body))
	return facts
}

func memoryAssembleHeaders() [][2]string {
	return [][2]string{
		{":status", "200"},
		{"content-type", "application/json"},
	}
}

func memoryAssembleResponse(t *testing.T, decision string, memoryMessage map[string]interface{}, recentMessages []map[string]interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{
		"schema_version":  1,
		"decision":        decision,
		"memory_message":  memoryMessage,
		"recent_messages": recentMessages,
		"trace": map[string]interface{}{
			"policy_version": "7",
			"recent_source":  "redis",
			"raw_trace":      "Console trace raw value",
		},
		"diagnostics": map[string]interface{}{
			"raw_diagnostic": "Console diagnostic raw value",
		},
	})
	require.NoError(t, err)
	return data
}

func requireMemoryRequestMessages(t *testing.T, host test.TestHost) []map[string]interface{} {
	t.Helper()
	var request struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	require.NoErrorf(t, json.Unmarshal(host.GetRequestBody(), &request), "request body must be JSON after memory injection; body=%s", string(host.GetRequestBody()))
	return request.Messages
}

func requireMemoryMessage(t *testing.T, message map[string]interface{}, role, content string) {
	t.Helper()
	require.Equal(t, role, message["role"])
	require.Equal(t, content, message["content"])
}

func requireMemoryRequestField(t *testing.T, body []byte, field string) json.RawMessage {
	t.Helper()
	var request map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &request))
	value, ok := request[field]
	require.Truef(t, ok, "request body should preserve field %q", field)
	return value
}

func requireMemoryRequestStringField(t *testing.T, body []byte, field string) string {
	t.Helper()
	raw := requireMemoryRequestField(t, body, field)
	var value string
	require.NoError(t, json.Unmarshal(raw, &value))
	return value
}

func requireMemoryMessagesSystemText(t *testing.T, host test.TestHost) string {
	t.Helper()
	var request struct {
		System []map[string]interface{} `json:"system"`
	}
	require.NoErrorf(t, json.Unmarshal(host.GetRequestBody(), &request), "request body must be JSON after memory injection; body=%s", string(host.GetRequestBody()))
	var parts []string
	for _, block := range request.System {
		if text, _ := block["text"].(string); text != "" {
			parts = append(parts, text)
		}
	}
	require.NotEmptyf(t, parts, "messages request should contain injected system text blocks; body=%s", string(host.GetRequestBody()))
	return strings.Join(parts, "\n")
}
