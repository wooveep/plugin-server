package main

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryPrivacyAndIsolation(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("no-store excludes raw prompts answers and credentials from event redis and logs", func(t *testing.T) {
			const (
				rawUserContent      = "privacy raw prompt must not leak through gated capture"
				rawAssistantContent = "privacy raw answer must not leak through gated capture"
			)
			headers := append(memoryPrivacyCredentialHeaders(), [2]string{memoryNoStoreHeader, "true"})
			body := memoryEventRequestBody(t, rawUserContent, map[string]interface{}{
				"authorization":           "request-body-authorization-secret",
				"redis_password":          "request-body-redis-password-secret",
				"internal_bearer_token":   "request-body-internal-bearer-secret",
				"unpermitted_cache_field": "cache-field-secret",
			})

			host := startMemoryEventRequestWithBody(t, true, body, headers...)
			callMemoryEventResponse(t, host, 200, memoryEventResponseBody(t, rawAssistantContent, "stop", 6, 4, 10))

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryEventRequiredFields(t, event)
			requireMemoryResponseSafeFacts(t, event, 200)
			require.Equal(t, true, event["no_store"])
			requireMemoryEventOmitsField(t, event, "user_content")
			requireMemoryEventOmitsField(t, event, "assistant_content")

			forbidden := memoryPrivacySecretValues(rawUserContent, rawAssistantContent, "cache-field-secret")
			requireMemoryPrivacyNoForbiddenValues(t, "MemoryEvent", memoryEventJSON(t, event), forbidden...)
			requireMemoryPrivacyNoForbiddenValues(t, "Redis commands", memoryPrivacyRedisCommands(t, host), forbidden...)
			requireMemoryPrivacyNoForbiddenValues(t, "plugin logs", memoryPrivacyLogs(host), forbidden...)

			combinedLower := strings.ToLower(memoryEventJSON(t, event) + "\n" + memoryPrivacyRedisCommands(t, host) + "\n" + memoryPrivacyLogs(host))
			requireMemoryPrivacyNoForbiddenValues(t, "observable privacy surface", combinedLower, memoryPrivacyCredentialHeaderNames()...)
		})

		t.Run("raw event payload is not mirrored into plugin logs", func(t *testing.T) {
			const (
				rawUserContent      = "privacy raw prompt allowed in event only"
				rawAssistantContent = "privacy raw answer allowed in event only"
			)
			host := startMemoryEventRequestWithBody(t, true, memoryEventRequestBody(t, rawUserContent, nil))
			callMemoryEventResponse(t, host, 200, memoryEventResponseBody(t, rawAssistantContent, "stop", 6, 4, 10))

			event := requireMemoryResponseCaptureEvent(t, host)
			require.Equal(t, rawUserContent, event["user_content"])
			require.Equal(t, rawAssistantContent, event["assistant_content"])

			decodedLogs := memoryPrivacyDecodedBase64LogFragments(memoryPrivacyLogs(host))
			requireMemoryPrivacyNoForbiddenValues(t, "decoded plugin logs", decodedLogs, rawUserContent, rawAssistantContent)
		})

		t.Run("redis stream failure logs only safe diagnostics", func(t *testing.T) {
			const (
				rawUserContent      = "privacy raw prompt inside redis failure"
				rawAssistantContent = "privacy raw answer inside redis failure"
				redisSecret         = "redis-password-secret"
			)
			responseBody := memoryEventResponseBody(t, rawAssistantContent, "stop", 6, 4, 10)
			host := startMemoryEventRequestWithBody(t, true, memoryEventRequestBody(t, rawUserContent, nil))
			callMemoryEventResponse(t, host, 200, responseBody)
			event := requireMemoryResponseCaptureEvent(t, host)
			require.Equal(t, rawUserContent, event["user_content"])
			require.Equal(t, rawAssistantContent, event["assistant_content"])
			requireMemoryEventXADDCommand(t, host)

			host.CallOnRedisCall(0, test.CreateRedisRespError(rawUserContent+" "+rawAssistantContent+" "+redisSecret))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			require.Nil(t, host.GetLocalResponse())
			require.Equal(t, string(responseBody), string(host.GetResponseBody()))
			requireMemoryPrivacyNoForbiddenValues(t, "plugin logs", memoryPrivacyLogs(host), rawUserContent, rawAssistantContent, redisSecret)
		})

		t.Run("inbound credentials are not forwarded to Console assemble or logs", func(t *testing.T) {
			host := startMemoryPrivacyAssembleRequest(t, memoryEventRequestBody(t, "privacy assemble prompt", nil), memoryPrivacyCredentialHeaders()...)
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			facts := requireMemoryAssembleCall(t, host)
			require.Equal(t, "privacy assemble prompt", facts["current_question"])

			callout := host.GetHttpCalloutAttributes()[0]
			secrets := memoryPrivacySecretValues()
			requireMemoryPrivacyNoForbiddenValues(t, "Console assemble body", memoryEventJSON(t, facts), secrets...)
			requireMemoryPrivacyNoForbiddenValues(t, "Console assemble headers", memoryPrivacyHeaders(callout.Headers), secrets...)
			requireMemoryPrivacyNoForbiddenValues(t, "plugin logs", memoryPrivacyLogs(host), secrets...)

			headerTextLower := strings.ToLower(memoryPrivacyHeaders(callout.Headers))
			requireMemoryPrivacyNoForbiddenValues(t, "Console assemble headers", headerTextLower, memoryPrivacyCredentialHeaderNames()...)
		})

		t.Run("Console assemble response content is not logged on success or failure", func(t *testing.T) {
			tests := []struct {
				name     string
				response []byte
				wantBody []memoryExpectedMessage
			}{
				{
					name: "successful assemble redacts returned memory trace and diagnostics",
					response: memoryAssembleResponse(t, "inject", map[string]interface{}{
						"role":    "system",
						"content": "privacy Console memory message",
					}, []map[string]interface{}{
						{"role": "user", "content": "privacy Console recent user"},
						{"role": "assistant", "content": "privacy Console recent assistant"},
					}),
					wantBody: []memoryExpectedMessage{
						{role: "system", content: "privacy Console memory message"},
						{role: "user", content: "privacy Console recent user"},
						{role: "assistant", content: "privacy Console recent assistant"},
						{role: "user", content: "privacy assemble prompt"},
					},
				},
				{
					name:     "invalid assemble response redacts raw error body",
					response: []byte(`{"schema_version":1,"decision":"inject","memory_message":{"role":"system","content":"privacy Console invalid memory"},"recent_messages":[{"role":"user","content":"privacy invalid recent user"}],"trace":{"raw_trace":"privacy invalid trace"},"diagnostics":{"raw_error":"privacy invalid diagnostic"},"error":"privacy internal bearer error"}`),
					wantBody: []memoryExpectedMessage{
						{role: "user", content: "recent safe user"},
						{role: "assistant", content: "recent safe assistant"},
						{role: "user", content: "privacy assemble prompt"},
					},
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					host := startMemoryPrivacyAssembleRequest(t, memoryEventRequestBody(t, "privacy assemble prompt", nil))
					host.CallOnRedisCall(0, validMemoryRecentZSetResponse(t))
					requireMemoryAssembleCall(t, host)
					host.CallOnHttpCall(memoryAssembleHeaders(), tt.response)

					require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
					requireMemoryExpectedMessages(t, requireMemoryRequestMessages(t, host), tt.wantBody)
					requireMemoryPrivacyNoForbiddenValues(t, "plugin logs", memoryPrivacyLogs(host),
						"privacy Console memory message",
						"privacy Console recent user",
						"privacy Console recent assistant",
						"Console trace raw value",
						"Console diagnostic raw value",
						"privacy Console invalid memory",
						"privacy invalid recent user",
						"privacy invalid trace",
						"privacy invalid diagnostic",
						"privacy internal bearer error",
					)
				})
			}
		})

		t.Run("Redis usage and MemoryEvent schema stay isolated from ai-cache", func(t *testing.T) {
			host := startMemoryEventRequestWithBody(t, true, memoryEventRequestBody(t, "cache isolation user question", nil))
			callMemoryEventResponse(t, host, 200, memoryEventResponseBody(t, "cache isolation assistant answer", "stop", 5, 4, 9))

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryPrivacyUsesMemoryEventStream(t, host)
			requireMemoryPrivacyNoForbiddenValues(t, "MemoryEvent", memoryEventJSON(t, event), memoryPrivacyCacheArtifacts()...)
			requireMemoryPrivacyNoForbiddenValues(t, "Redis commands", memoryPrivacyRedisCommands(t, host), memoryPrivacyCacheArtifacts()...)
		})
	})
}

func startMemoryPrivacyAssembleRequest(t *testing.T, body []byte, extraHeaders ...[2]string) test.TestHost {
	t.Helper()
	host, status := newMemoryConfigTestHost(memoryConsoleAssembleConfig(t, "semantic"))
	t.Cleanup(host.Reset)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	require.NoError(t, host.SetRequestId("property-request-id"))

	headers := applyMemoryRequestGatingHeaderOverrides(memoryConsoleAssembleHeaders(), [][2]string{
		{"x-request-id", "request-privacy-assemble-1"},
	})
	headers = append(headers, extraHeaders...)
	action := host.CallOnHttpRequestHeaders(headers)
	require.Equal(t, types.HeaderStopIteration, action)

	action = host.CallOnHttpRequestBody(body)
	require.Equal(t, types.ActionPause, action)
	requireMemoryRecentRedisLookup(t, host)
	return host
}

func requireMemoryPrivacyUsesMemoryEventStream(t *testing.T, host test.TestHost) {
	t.Helper()
	var xadds [][]string
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := memoryResponseCaptureCommand(t, call.Query)
		require.Truef(t, ok, "memory Redis call must be valid RESP; calls=%s", memoryResponseCaptureCallSummary(t, host))
		require.NotEmpty(t, cmd)
		if strings.EqualFold(cmd[0], "xadd") {
			require.GreaterOrEqual(t, len(cmd), 2, "MemoryEvent XADD must include stream")
			xadds = append(xadds, cmd)
		}
	}
	require.Lenf(t, xadds, 1, "memory-enabled response should emit exactly one pending MemoryEvent XADD; redis calls=%s", memoryResponseCaptureCallSummary(t, host))
	require.Equal(t, memoryEventStreamName, xadds[0][1])
}

func requireMemoryPrivacyNoForbiddenValues(t *testing.T, label, text string, forbidden ...string) {
	t.Helper()
	for _, value := range forbidden {
		if value == "" {
			continue
		}
		require.NotContainsf(t, text, value, "%s leaked forbidden value %q", label, value)
	}
}

func memoryPrivacyCredentialHeaders() [][2]string {
	return [][2]string{
		{"authorization", "Bearer request-authorization-secret"},
		{"x-api-key", "sk-request-api-key-secret"},
		{"x-internal-bearer", "Bearer gateway-internal-bearer-secret"},
		{"x-redis-password", "redis-password-secret"},
		{"x-provider-api-key", "provider-api-key-secret"},
	}
}

func memoryPrivacyCredentialHeaderNames() []string {
	return []string{
		"authorization",
		"x-api-key",
		"x-internal-bearer",
		"x-redis-password",
		"x-provider-api-key",
	}
}

func memoryPrivacySecretValues(extra ...string) []string {
	values := []string{
		"Bearer request-authorization-secret",
		"sk-request-api-key-secret",
		"Bearer gateway-internal-bearer-secret",
		"redis-password-secret",
		"provider-api-key-secret",
		"request-body-authorization-secret",
		"request-body-redis-password-secret",
		"request-body-internal-bearer-secret",
	}
	return append(values, extra...)
}

func memoryPrivacyCacheArtifacts() []string {
	return []string{
		"cache:events",
		"higress-ai-cache:",
		"cache:materialized:",
		`"cache_scope"`,
		`"cache_policy_version"`,
		`"cache_key"`,
		`"cache_value"`,
		`"cache_hit"`,
		`"cache_event"`,
		`"event_kind":"cache"`,
	}
}

func memoryPrivacyRedisCommands(t *testing.T, host test.TestHost) string {
	t.Helper()
	var commands []string
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := memoryResponseCaptureCommand(t, call.Query)
		if !ok {
			commands = append(commands, string(call.Query))
			continue
		}
		commands = append(commands, strings.Join(cmd, " "))
	}
	return strings.Join(commands, "\n")
}

func memoryPrivacyHeaders(headers [][2]string) string {
	var values []string
	for _, header := range headers {
		values = append(values, header[0]+": "+header[1])
	}
	return strings.Join(values, "\n")
}

func memoryPrivacyLogs(host test.TestHost) string {
	logs := append([]string{}, host.GetTraceLogs()...)
	logs = append(logs, host.GetDebugLogs()...)
	logs = append(logs, host.GetInfoLogs()...)
	logs = append(logs, host.GetWarnLogs()...)
	logs = append(logs, host.GetErrorLogs()...)
	logs = append(logs, host.GetCriticalLogs()...)
	return strings.Join(logs, "\n")
}

func memoryPrivacyDecodedBase64LogFragments(logs string) string {
	var decoded []string
	for _, token := range strings.Fields(logs) {
		token = strings.Trim(token, ",")
		body, err := base64.StdEncoding.DecodeString(token)
		if err == nil {
			decoded = append(decoded, string(body))
		}
	}
	return strings.Join(decoded, "\n")
}
