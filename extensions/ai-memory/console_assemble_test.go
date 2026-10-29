package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryConsoleAssemble(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("off mode skips Console assemble and memory behavior", func(t *testing.T) {
			host, status := newMemoryConfigTestHost(memoryConsoleAssembleConfig(t, "off"))
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("memory-route"))

			headerAction := host.CallOnHttpRequestHeaders(memoryConsoleAssembleHeaders())
			bodyAction := host.CallOnHttpRequestBody(memoryConsoleAssembleRequestBody())

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
		})

		t.Run("recent-only mode skips Console assemble and injects Redis recent messages", func(t *testing.T) {
			host := startMemoryConsoleAssembleRequest(t, "recent-only")
			host.CallOnRedisCall(0, validMemoryRecentZSetResponse(t))

			require.Empty(t, host.GetHttpCalloutAttributes(), "recent-only mode must not call Console assemble")
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			messages := requireMemoryRequestMessages(t, host)
			require.Len(t, messages, 3)
			requireMemoryMessage(t, messages[0], "user", "recent safe user")
			requireMemoryMessage(t, messages[1], "assistant", "recent safe assistant")
			requireMemoryMessage(t, messages[2], "user", "current question")
		})

		for _, mode := range []string{"digest", "semantic", "full"} {
			t.Run(mode+" mode calls Console assemble after Redis miss", func(t *testing.T) {
				host := startMemoryConsoleAssembleRequest(t, mode)
				host.CallOnRedisCall(0, test.CreateRedisRespNull())

				facts := requireMemoryAssembleCall(t, host)
				require.Equal(t, mode, facts["memory_mode"])
				require.Equal(t, "current question", facts["current_question"])
			})
		}

		t.Run("question_from path drives Console current_question with latest user fallback", func(t *testing.T) {
			facts := memoryConsoleAssembleFactsWithConfig(t, memoryConsoleAssembleCustomConfig(t, "semantic", map[string]interface{}{
				"question_from": "metadata.extracted_question",
			}, true, true), []byte(`{
				"model": "qwen-turbo",
				"metadata": {"extracted_question": "path selected question"},
				"messages": [
					{"role": "user", "content": "latest user fallback"}
				],
				"stream": false
			}`), memoryConsoleAssembleHeaders())
			require.Equal(t, "path selected question", facts["current_question"])

			fallbackFacts := memoryConsoleAssembleFactsWithConfig(t, memoryConsoleAssembleCustomConfig(t, "semantic", map[string]interface{}{
				"question_from": "metadata.missing_question",
			}, true, true), []byte(`{
				"model": "qwen-turbo",
				"messages": [
					{"role": "user", "content": "latest user fallback"}
				],
				"stream": false
			}`), memoryConsoleAssembleHeaders())
			require.Equal(t, "latest user fallback", fallbackFacts["current_question"])
		})

		t.Run("messages digest is scoped and stable across request id retries", func(t *testing.T) {
			body := memoryConsoleAssembleRequestBody()
			factsA := memoryConsoleAssembleFactsWithConfig(t, memoryConsoleAssembleCustomConfig(t, "semantic", nil, false, true), body, memoryConsoleAssembleHeaders())

			factsRetry := memoryConsoleAssembleFactsWithConfig(t, memoryConsoleAssembleCustomConfig(t, "semantic", nil, false, true), body, memoryConsoleAssembleHeadersWith(
				[2]string{"x-request-id", "request-console-assemble-retry"},
			))
			require.Equal(t, factsA["messages_digest"], factsRetry["messages_digest"], "request_id must not affect scoped digest")

			factsOtherTenant := memoryConsoleAssembleFactsWithConfig(t, memoryConsoleAssembleCustomConfig(t, "semantic", nil, false, true), body, memoryConsoleAssembleHeadersWith(
				[2]string{"x-mse-tenant", "tenant-b"},
			))
			require.NotEqual(t, factsA["messages_digest"], factsOtherTenant["messages_digest"], "tenant must scope messages_digest")
		})

		t.Run("omitted recent cache service skips Redis lookup", func(t *testing.T) {
			host := startMemoryConsoleAssembleRequestWithConfig(t, memoryConsoleAssembleCustomConfig(t, "semantic", nil, false, true), memoryConsoleAssembleRequestBody(), memoryConsoleAssembleHeaders())

			require.Empty(t, host.GetRedisCalloutAttributes())
			requireMemoryAssembleCall(t, host)
		})

		t.Run("omitted Console service skips assemble and continues without memory", func(t *testing.T) {
			host, status := newMemoryConfigTestHost(memoryConsoleAssembleCustomConfig(t, "semantic", nil, false, false))
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("memory-route"))
			require.NoError(t, host.SetRequestId("property-request-id"))

			headerAction := host.CallOnHttpRequestHeaders(memoryConsoleAssembleHeaders())
			require.Equal(t, types.HeaderStopIteration, headerAction)
			bodyAction := host.CallOnHttpRequestBody(memoryConsoleAssembleRequestBody())

			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
		})

		tests := []struct {
			name             string
			decision         string
			memoryMessage    map[string]interface{}
			responseRecent   []map[string]interface{}
			expectedMessages []memoryExpectedMessage
		}{
			{
				name:     "inject decision inserts memory and response recent messages",
				decision: "inject",
				memoryMessage: map[string]interface{}{
					"role":    "system",
					"content": "Console memory context",
				},
				responseRecent: []map[string]interface{}{
					{"role": "user", "content": "console recent user"},
					{"role": "assistant", "content": "console recent assistant"},
				},
				expectedMessages: []memoryExpectedMessage{
					{role: "system", content: "Console memory context"},
					{role: "user", content: "console recent user"},
					{role: "assistant", content: "console recent assistant"},
					{role: "user", content: "current question"},
				},
			},
			{
				name:     "recent_only decision uses Console recent messages without memory message",
				decision: "recent_only",
				responseRecent: []map[string]interface{}{
					{"role": "user", "content": "console recent user"},
					{"role": "assistant", "content": "console recent assistant"},
				},
				expectedMessages: []memoryExpectedMessage{
					{role: "user", content: "console recent user"},
					{role: "assistant", content: "console recent assistant"},
					{role: "user", content: "current question"},
				},
			},
			{
				name:     "skip decision keeps current request messages only",
				decision: "skip",
				expectedMessages: []memoryExpectedMessage{
					{role: "user", content: "current question"},
				},
			},
			{
				name:     "bypass decision keeps current request messages only",
				decision: "bypass",
				expectedMessages: []memoryExpectedMessage{
					{role: "user", content: "current question"},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				host := startMemoryConsoleAssembleRequest(t, "semantic")
				host.CallOnRedisCall(0, validMemoryRecentZSetResponse(t))
				requireMemoryAssembleCall(t, host)

				host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, tt.decision, tt.memoryMessage, tt.responseRecent))

				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				requireMemoryExpectedMessages(t, requireMemoryRequestMessages(t, host), tt.expectedMessages)
				requireMemoryConsoleSafeLogs(t,
					host,
					"Console memory context",
					"console recent user",
					"console recent assistant",
					"Console trace raw value",
					"Console diagnostic raw value",
				)
			})
		}

		t.Run("request no-store still allows Console memory injection", func(t *testing.T) {
			host := startMemoryConsoleAssembleRequestWithConfig(
				t,
				memoryConsoleAssembleConfig(t, "semantic"),
				memoryConsoleAssembleRequestBody(),
				memoryConsoleAssembleHeadersWith([2]string{memoryNoStoreHeader, "true"}),
			)
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireMemoryAssembleCall(t, host)

			host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{
				"role":    "system",
				"content": "no-store still injects safe memory",
			}, nil))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			requireMemoryExpectedMessages(t, requireMemoryRequestMessages(t, host), []memoryExpectedMessage{
				{role: "system", content: "no-store still injects safe memory"},
				{role: "user", content: "current question"},
			})
		})

		t.Run("skip decision preserves original request body without empty rewrite", func(t *testing.T) {
			original := []byte(`{"model":"qwen-turbo","messages":[{"role":"user","content":"opening question"},{"role":"system","content":"late client system"},{"role":"user","content":"current question"}],"metadata":{"trace":"keep"}}`)
			host := startMemoryConsoleAssembleRequestWithBody(t, "semantic", original)
			host.CallOnRedisCall(0, validMemoryRecentZSetResponse(t))
			requireMemoryAssembleCall(t, host)

			host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "skip", nil, nil))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			require.Equal(t, string(original), string(host.GetRequestBody()))
		})

		t.Run("timeout falls back to Redis recent memory", func(t *testing.T) {
			host := startMemoryConsoleAssembleRequest(t, "semantic")
			host.CallOnRedisCall(0, validMemoryRecentZSetResponse(t))
			requireMemoryAssembleCall(t, host)

			host.CallOnHttpCall(nil, nil)

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			requireMemoryExpectedMessages(t, requireMemoryRequestMessages(t, host), []memoryExpectedMessage{
				{role: "user", content: "recent safe user"},
				{role: "assistant", content: "recent safe assistant"},
				{role: "user", content: "current question"},
			})
		})

		t.Run("invalid response falls back to Redis recent memory and does not leak raw memory into logs", func(t *testing.T) {
			host := startMemoryConsoleAssembleRequest(t, "semantic")
			host.CallOnRedisCall(0, validMemoryRecentZSetResponse(t))
			requireMemoryAssembleCall(t, host)

			host.CallOnHttpCall(memoryAssembleHeaders(), []byte(`{"schema_version":1,"decision":"inject","memory_message":{"role":"system","content":"raw Console memory must not be logged"}`))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			requireMemoryExpectedMessages(t, requireMemoryRequestMessages(t, host), []memoryExpectedMessage{
				{role: "user", content: "recent safe user"},
				{role: "assistant", content: "recent safe assistant"},
				{role: "user", content: "current question"},
			})
			requireMemoryConsoleSafeLogs(t, host, "raw Console memory must not be logged", "recent safe user", "recent safe assistant")
		})

		t.Run("invalid response with multi-user request preserves original body", func(t *testing.T) {
			original := []byte(`{"model":"qwen-turbo","messages":[{"role":"user","content":"opening question"},{"role":"system","content":"late client system"},{"role":"user","content":"current question"}],"metadata":{"trace":"keep"}}`)
			host := startMemoryConsoleAssembleRequestWithBody(t, "semantic", original)
			host.CallOnRedisCall(0, validMemoryRecentZSetResponse(t))
			requireMemoryAssembleCall(t, host)

			host.CallOnHttpCall(memoryAssembleHeaders(), []byte(`{"schema_version":1,"decision":"inject","memory_message":{"role":"system","content":"raw Console memory must not be logged"}`))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			require.Equal(t, string(original), string(host.GetRequestBody()))
			requireMemoryConsoleSafeLogs(t, host, "raw Console memory must not be logged", "recent safe user", "recent safe assistant")
		})

		t.Run("invalid decision and role values are not logged raw", func(t *testing.T) {
			tests := []struct {
				name     string
				response []byte
				forbid   string
			}{
				{
					name:     "invalid decision",
					response: []byte(`{"schema_version":1,"decision":"secret-invalid-decision"}`),
					forbid:   "secret-invalid-decision",
				},
				{
					name:     "invalid role",
					response: []byte(`{"schema_version":1,"decision":"inject","memory_message":{"role":"secret-invalid-role","content":"safe reject content"}}`),
					forbid:   "secret-invalid-role",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					host := startMemoryConsoleAssembleRequest(t, "semantic")
					host.CallOnRedisCall(0, validMemoryRecentZSetResponse(t))
					requireMemoryAssembleCall(t, host)

					host.CallOnHttpCall(memoryAssembleHeaders(), tt.response)

					require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
					requireMemoryConsoleSafeLogs(t, host, tt.forbid)
				})
			}
		})

		t.Run("oversized assemble request skips Console callout and fails open", func(t *testing.T) {
			host := startMemoryConsoleAssembleRequestWithBody(t, "semantic", []byte(`{
				"model": "qwen-turbo",
				"messages": [
					{"role": "user", "content": "`+strings.Repeat("oversized-question ", 20_000)+`"}
				],
				"stream": false
			}`))
			host.CallOnRedisCall(0, test.CreateRedisRespNull())

			require.Empty(t, host.GetHttpCalloutAttributes())
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			requireMemoryConsoleSafeLogs(t, host, "oversized-question")
		})
	})
}

func TestParseMemoryAssembleResponseAcceptsConsoleAPIEnvelope(t *testing.T) {
	body := []byte(`{"data":{"decision":"inject","reason":"memory_available","memory_messages":[{"role":"system","content":"Console memory context"}],"recent_messages":[{"role":"user","content":"recent user"},{"role":"assistant","content":"recent assistant"}],"diagnostics":{"source":"console"}}}`)

	response, err := parseMemoryAssembleResponse(body)

	require.NoError(t, err)
	require.Equal(t, memoryAssembleDecisionInject, response.Decision)
	require.NotNil(t, response.MemoryMessage)
	require.Equal(t, "system", response.MemoryMessage.Role)
	require.Equal(t, "Console memory context", response.MemoryMessage.Content)
	require.Len(t, response.RecentMessages, 2)
	require.Equal(t, "recent user", response.RecentMessages[0].Content)
	require.Equal(t, "recent assistant", response.RecentMessages[1].Content)
}

func TestMemoryConsoleAssembleBounds(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		body := []byte(`{"schema_version":1,"decision":"skip","trace":{"raw":"` + strings.Repeat("oversized-response ", 20_000) + `"}}`)

		_, err := parseMemoryAssembleResponse(body)

		require.Error(t, err)
		require.NotContains(t, err.Error(), "oversized-response")
	})
}

type memoryExpectedMessage struct {
	role    string
	content string
}

func memoryConsoleAssembleConfig(t *testing.T, memoryMode string) json.RawMessage {
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
		"enable_path_suffixes": []string{"/v1/chat/completions"},
		"fail_policy":          "open",
		"_rules_": []map[string]interface{}{
			{
				"_match_route_":       []string{"memory-route"},
				"memory_mode":         memoryMode,
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

func startMemoryConsoleAssembleRequest(t *testing.T, memoryMode string) test.TestHost {
	t.Helper()
	return startMemoryConsoleAssembleRequestWithBody(t, memoryMode, memoryConsoleAssembleRequestBody())
}

func startMemoryConsoleAssembleRequestWithBody(t *testing.T, memoryMode string, body []byte) test.TestHost {
	t.Helper()
	return startMemoryConsoleAssembleRequestWithConfig(t, memoryConsoleAssembleConfig(t, memoryMode), body, memoryConsoleAssembleHeaders())
}

func startMemoryConsoleAssembleRequestWithConfig(t *testing.T, config json.RawMessage, body []byte, headers [][2]string) test.TestHost {
	t.Helper()
	return startMemoryConsoleAssembleRequestWithConfigCleanup(t, config, body, headers, true)
}

func startMemoryConsoleAssembleRequestWithConfigNoCleanup(t *testing.T, config json.RawMessage, body []byte, headers [][2]string) test.TestHost {
	t.Helper()
	return startMemoryConsoleAssembleRequestWithConfigCleanup(t, config, body, headers, false)
}

func memoryConsoleAssembleFactsWithConfig(t *testing.T, config json.RawMessage, body []byte, headers [][2]string) map[string]interface{} {
	t.Helper()
	host := startMemoryConsoleAssembleRequestWithConfigNoCleanup(t, config, body, headers)
	defer host.Reset()
	if len(host.GetRedisCalloutAttributes()) > 0 {
		host.CallOnRedisCall(0, test.CreateRedisRespNull())
	}
	return requireMemoryAssembleCall(t, host)
}

func startMemoryConsoleAssembleRequestWithConfigCleanup(t *testing.T, config json.RawMessage, body []byte, headers [][2]string, cleanup bool) test.TestHost {
	t.Helper()
	host, status := newMemoryConfigTestHost(config)
	if cleanup {
		t.Cleanup(host.Reset)
	}
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	require.NoError(t, host.SetRequestId("property-request-id"))

	headerAction := host.CallOnHttpRequestHeaders(headers)
	require.Equal(t, types.HeaderStopIteration, headerAction)

	bodyAction := host.CallOnHttpRequestBody(body)
	require.Equal(t, types.ActionPause, bodyAction)
	if len(host.GetRedisCalloutAttributes()) > 0 {
		requireMemoryRecentRedisLookup(t, host)
	}
	return host
}

func memoryConsoleAssembleHeaders() [][2]string {
	return [][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
		{"x-mse-session", "session-a"},
		{"x-request-id", "request-console-assemble-1"},
	}
}

func memoryConsoleAssembleRequestBody() []byte {
	return []byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "user", "content": "current question"}
		],
		"stream": false
	}`)
}

func memoryConsoleAssembleHeadersWith(overrides ...[2]string) [][2]string {
	return applyMemoryRequestGatingHeaderOverrides(memoryConsoleAssembleHeaders(), overrides)
}

func memoryConsoleAssembleCustomConfig(t *testing.T, memoryMode string, routeOverrides map[string]interface{}, includeRecent bool, includeConsole bool) json.RawMessage {
	t.Helper()
	cfg := map[string]interface{}{
		"redis_stream": map[string]interface{}{
			"service_name": "redis.memory.svc.cluster.local",
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"session_header":       "x-mse-session",
		"request_id_header":    "x-request-id",
		"enable_path_suffixes": []string{"/v1/chat/completions"},
		"fail_policy":          "open",
	}
	if includeRecent {
		cfg["recent_cache"] = map[string]interface{}{
			"service_name": "redis.recent.svc.cluster.local",
		}
	}
	if includeConsole {
		cfg["console_internal"] = map[string]interface{}{
			"service_name":  memoryConsoleService,
			"service_port":  8080,
			"assemble_path": memoryConsolePath,
			"timeout_ms":    120,
		}
	}
	route := map[string]interface{}{
		"_match_route_":       []string{"memory-route"},
		"memory_mode":         memoryMode,
		"recent_window_turns": 6,
		"memory_token_budget": 1500,
		"assemble_timeout_ms": 80,
		"inject_role":         "system",
		"semantic_top_k":      3,
		"capture_response":    true,
		"policy_version":      "7",
	}
	for key, value := range routeOverrides {
		route[key] = value
	}
	cfg["_rules_"] = []map[string]interface{}{route}
	return mustMemoryConfig(t, cfg)
}

func requireMemoryExpectedMessages(t *testing.T, messages []map[string]interface{}, expected []memoryExpectedMessage) {
	t.Helper()
	require.Len(t, messages, len(expected))
	for i, want := range expected {
		requireMemoryMessage(t, messages[i], want.role, want.content)
	}
}

func requireMemoryConsoleSafeLogs(t *testing.T, host test.TestHost, forbidden ...string) {
	t.Helper()
	logs := strings.ToLower(strings.Join(append(append(append(host.GetDebugLogs(), host.GetInfoLogs()...), host.GetWarnLogs()...), host.GetErrorLogs()...), "\n"))
	for _, value := range forbidden {
		require.NotContains(t, logs, strings.ToLower(value))
	}
}
