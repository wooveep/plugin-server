package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	memoryNativeRouteID    = "55555555-5555-4555-8555-555555555555"
	memoryNativePolicyID   = "66666666-6666-4666-8666-666666666666"
	memoryNativeStaticJSON = `{"release_id":"original/release:opaque","target_id":"original/target:opaque","route_id":"55555555-5555-4555-8555-555555555555","policy_id":"66666666-6666-4666-8666-666666666666","policy_version":7}`
)

func TestManagedMemoryNativeIdentityConfigBoundary(t *testing.T) {
	for _, raw := range []string{
		`null`, `{}`, strings.Replace(memoryNativeStaticJSON, `"policy_version":7`, `"policy_version":"7"`, 1),
		strings.Replace(memoryNativeStaticJSON, `"policy_version":7`, `"policy_version":1.0`, 1),
		strings.Replace(memoryNativeStaticJSON, `"policy_version":7`, `"policy_version":1e0`, 1),
		strings.Replace(memoryNativeStaticJSON, `"policy_version":7`, `"policy_version":0`, 1),
		strings.Replace(memoryNativeStaticJSON, `"policy_version":7`, `"policy_version":9223372036854775808`, 1),
		strings.Replace(memoryNativeStaticJSON, memoryNativeRouteID, "00000000-0000-0000-0000-000000000000", 1),
		strings.Replace(memoryNativeStaticJSON, `"target_id":"original/target:opaque"`, `"target_id":""`, 1),
		strings.Replace(memoryNativeStaticJSON, `"release_id":"original/release:opaque"`, `"release_id":"`+strings.Repeat("界", 86)+`"`, 1),
		strings.TrimSuffix(memoryNativeStaticJSON, "}") + `,"caller":{"account_id":"configured-caller-is-forbidden"}}`,
		strings.TrimSuffix(memoryNativeStaticJSON, "}") + `,"release_id":"duplicate"}`,
	} {
		var c config.PluginConfig
		c.FromJson(gjson.Parse(`{"redis_stream":{"service_name":"redis.memory.svc.cluster.local"},"path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"text_generate","native_identity":`+raw+`}`), memoryCaptureNoopLog{})
		require.Error(t, c.Validate(), raw)
	}
	for _, version := range []string{`"07"`, `"8"`, `7`} {
		var c config.PluginConfig
		c.FromJson(gjson.Parse(`{"redis_stream":{"service_name":"redis.memory.svc.cluster.local"},"path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"text_generate","policy_version":`+version+`,"native_identity":`+memoryNativeStaticJSON+`}`), memoryCaptureNoopLog{})
		require.Error(t, c.Validate())
	}
	var c config.PluginConfig
	c.FromJson(gjson.Parse(`{"redis_stream":{"service_name":"redis.memory.svc.cluster.local"},"path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"text_generate","native_identity":`+strings.Replace(memoryNativeStaticJSON, `"policy_version":7`, `"policy_version":9223372036854775807`, 1)+`}`), memoryCaptureNoopLog{})
	require.NoError(t, c.Validate())
	require.Equal(t, "9223372036854775807", c.Route.PolicyVersion)
	ctx := newMemoryCaptureTestContext()
	ctx.SetContext(memoryNativeCallerContextKey, memoryNativeCallerFixture())
	body, err := buildMemoryAssembleRequestBody(ctx, c)
	require.NoError(t, err)
	require.Equal(t, "9223372036854775807", gjson.GetBytes(body, "native_identity.policy_version").Raw)
	require.Equal(t, `"9007199254740993"`, gjson.GetBytes(body, "native_identity.caller.credential_generation").Raw)
}

func TestManagedMemoryNativeIdentityRoundTrip(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		caller := memoryNativeCallerFixture()
		host := startNativeMemoryHeaders(t, memoryNativeConfiguration(t, "semantic", false), caller.Headers())
		defer host.Reset()
		body := []byte(`{"model":"public.model","messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA"},"vendor_sequence":9007199254740993}]}]}`)
		require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
		facts := requireMemoryAssembleCall(t, host)
		require.Equal(t, caller.TenantID, facts["tenant"])
		require.Equal(t, caller.PrincipalID, facts["consumer"])
		require.Equal(t, map[string]interface{}{"id": memoryNativeRouteID, "name": "memory-route"}, facts["route"])
		require.Equal(t, map[string]interface{}{"name": "public.model"}, facts["model"])
		require.Equal(t, "7", facts["policy_version"])
		native, ok := facts["native_identity"].(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, "original/release:opaque", native["release_id"])
		require.Equal(t, "original/target:opaque", native["target_id"])
		require.Equal(t, memoryNativeRouteID, native["route_id"])
		require.Equal(t, memoryNativePolicyID, native["policy_id"])
		var expectedCaller map[string]interface{}
		encoded, err := json.Marshal(caller)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(encoded, &expectedCaller))
		require.Equal(t, expectedCaller, native["caller"])
		host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{"role": "system", "content": "retained memory"}, nil))
		require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
		require.Contains(t, string(host.GetRequestBody()), "retained memory")
		require.Contains(t, string(host.GetRequestBody()), "9007199254740993")
		originalBasis, err := bodybasis.Capture(body)
		require.NoError(t, err)
		retainedBasis, err := bodybasis.Read(host.GetRequestHeaders())
		require.NoError(t, err)
		require.Equal(t, originalBasis, retainedBasis)
		// The later proxy removes private identity headers before provider I/O.
		// Response events must retain this request's authenticated snapshot.
		for _, name := range balanceidentity.HeaderNames() {
			require.NoError(t, proxywasm.RemoveHttpRequestHeader(name))
		}
		host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
		host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`))
		event := requireMemoryResponseCaptureEvent(t, host)
		require.Equal(t, native, event["native_identity"])
		require.Equal(t, caller.TenantID, event["tenant"])
		require.Equal(t, caller.PrincipalID, event["consumer"])
		require.Equal(t, facts["route"], event["route"])
		require.Equal(t, facts["model"], event["model"])
	})
}

func TestManagedMemoryNativeCallerCannotDowngradeToLegacy(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		caller := memoryNativeCallerFixture()
		legacyBalance := caller
		legacyBalance.TenantID, legacyBalance.PrincipalID = "", ""
		badGeneration := caller
		badGeneration.CredentialGeneration = "01"
		for _, headers := range [][][2]string{
			nil,
			legacyBalance.Headers(),
			badGeneration.Headers(),
			append(caller.Headers(), [2]string{strings.ToUpper(balanceidentity.GenerationHeader), "2"}),
		} {
			host := startNativeMemoryHeaders(t, memoryNativeConfiguration(t, "semantic", false), headers)
			body := []byte(`{"model":"public.model","messages":[{"role":"user","content":"question"}]}`)
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(body))
			require.Nil(t, host.GetLocalResponse(), "missing native caller bypasses memory without blocking the model response")
			require.Empty(t, host.GetHttpCalloutAttributes())
			require.Empty(t, host.GetRedisCalloutAttributes())
			_, err := bodybasis.Read(host.GetRequestHeaders())
			require.NoError(t, err, "memory bypass must retain the separately configured original basis")
			host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
			host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"answer"}}]}`))
			require.Empty(t, host.GetRedisCalloutAttributes(), "an unauthenticated native event must not fall through to legacy labels")
			host.Reset()
		}
	})
}

func TestManagedMemoryNativeRecentKeyUsesConsolePrincipalComponent(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		caller := memoryNativeCallerFixture()
		host := startNativeMemoryHeaders(t, memoryNativeConfiguration(t, "recent-only", true), caller.Headers())
		defer host.Reset()
		require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(`{"model":"public.model","messages":[{"role":"user","content":"question"}]}`)))
		calls := host.GetRedisCalloutAttributes()
		require.Len(t, calls, 1)
		command := requireMemoryRecentRedisCommand(t, calls[0].Query)
		require.Equal(t, "ZREVRANGE", strings.ToUpper(command[0]))
		require.Equal(t, "memory:recent:"+caller.TenantID+":consumer-name-sha256:f1d11edf5c5cbe84bc60d6c543d0d5938a5bd3a833381499af7549ddc4933a23", command[1])
		host.CallOnRedisCall(0, test.CreateRedisRespNull())
		require.Empty(t, host.GetHttpCalloutAttributes())
		host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
		host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}]}`))
		event := requireMemoryResponseCaptureEvent(t, host)
		require.Equal(t, caller.PrincipalID, event["consumer"], "only the Redis component is hashed")
	})
}

func memoryNativeConfiguration(t *testing.T, mode string, recent bool) json.RawMessage {
	t.Helper()
	rule := map[string]interface{}{
		"_match_route_": []string{"memory-route"}, "_match_service_": []string{"mft-memory.dns"},
		"path_scope": "bound_target", "client_protocol": "openai_chat", "operation_kind": "text_generate",
		"native_identity": json.RawMessage(memoryNativeStaticJSON), "capture_body_basis": true,
		"memory_mode": mode, "recent_window_turns": 4, "capture_response": true,
		"redis_stream":     map[string]interface{}{"service_name": "redis.memory.svc.cluster.local", "stream": memoryEventStreamName},
		"console_internal": map[string]interface{}{"service_name": memoryConsoleService, "service_port": 8080, "assemble_path": memoryConsolePath},
		"tenant_header":    "x-client-tenant", "consumer_header": "x-client-consumer",
	}
	if recent {
		rule["recent_cache"] = map[string]interface{}{"service_name": "redis.memory.svc.cluster.local", "key_prefix": "memory:recent"}
	}
	// The original string policy version is derived once from the static integer.
	return mustMemoryConfig(t, map[string]interface{}{"_rules_": []map[string]interface{}{rule}})
}

func memoryNativeCallerFixture() balanceidentity.Snapshot {
	return balanceidentity.Snapshot{
		AccountID: "11111111-1111-4111-8111-111111111111", APIKeyID: "22222222-2222-4222-8222-222222222222",
		CredentialGeneration: "9007199254740993", TenantID: "33333333-3333-4333-8333-333333333333", PrincipalID: "44444444-4444-4444-8444-444444444444",
	}
}

func startNativeMemoryHeaders(t *testing.T, cfg json.RawMessage, authenticatedHeaders [][2]string) test.TestHost {
	t.Helper()
	host, status := newMemoryConfigTestHost(cfg)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	require.NoError(t, host.SetClusterName("outbound|80||mft-memory.dns"))
	headers := applyMemoryRequestGatingHeaderOverrides(memoryConsoleAssembleHeaders(), [][2]string{{":path", "/private/answer"}, {"x-client-tenant", "spoofed-legacy-tenant"}, {"x-client-consumer", "spoofed-legacy-consumer"}})
	// These fields emulate the original key-auth output, not client self-report.
	headers = append(headers, authenticatedHeaders...)
	require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
	return host
}

// The original admitted call is captured before later filters clear the private
// headers. A client-style id alone or contradictory metadata cannot fill it.
func TestManagedMemoryRetainsOnlyCompleteOriginalAdmissionParent(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		callID := "99999999-9999-4999-8999-999999999999"
		original := [][2]string{
			{executionfacts.AdmissionCallHeader, callID},
			{executionfacts.AdmissionReleaseHeader, "original/release:opaque"},
			{executionfacts.AdmissionOccurredHeader, time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)},
			{executionfacts.AdmissionDeadlineHeader, time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano)},
		}
		wrongRelease := append([][2]string(nil), original...)
		wrongRelease[1][1] = "foreign-release"
		for _, scenario := range []struct {
			name     string
			headers  [][2]string
			expected string
		}{
			{"original", original, callID},
			{"single-client-id", original[:1], ""},
			{"duplicate", append(append([][2]string(nil), original...), original[0]), ""},
			{"foreign-release", wrongRelease, ""},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				headers := append(memoryNativeCallerFixture().Headers(), scenario.headers...)
				host := startNativeMemoryHeaders(t, memoryNativeConfiguration(t, "recent-only", true), headers)
				defer host.Reset()
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(`{"model":"public.model","messages":[{"role":"user","content":"question"}]}`)))
				host.CallOnRedisCall(0, test.CreateRedisRespNull())
				for _, header := range original {
					require.NoError(t, proxywasm.RemoveHttpRequestHeader(header[0]))
				}
				host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
				host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}]}`))
				event := requireMemoryResponseCaptureEvent(t, host)
				if scenario.expected == "" {
					require.NotContains(t, event, "call_id")
				} else {
					require.Equal(t, scenario.expected, event["call_id"])
				}
			})
		}
	})
}
