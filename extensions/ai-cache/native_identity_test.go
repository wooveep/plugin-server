package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/memorycache"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

const (
	nativeCacheTestRoute         = "11111111-1111-4111-8111-111111111111"
	nativeCacheTestPolicy        = "22222222-2222-4222-8222-222222222222"
	nativeCacheTestVersion int64 = 9007199254740993
	nativeCacheTestBody          = `{"model":"public-alias","messages":[{"role":"user","content":"weather?"}]}`
)

func nativeCacheTestCaller() balanceidentity.Snapshot {
	return balanceidentity.Snapshot{
		AccountID:            "33333333-3333-4333-8333-333333333333",
		APIKeyID:             "44444444-4444-4444-8444-444444444444",
		CredentialGeneration: "9007199254740993",
		TenantID:             "55555555-5555-4555-8555-555555555555",
		PrincipalID:          "66666666-6666-4666-8666-666666666666",
	}
}

func nativeCacheTestConfig(t *testing.T) json.RawMessage {
	t.Helper()
	raw := publishedCacheConfig(t, thinResponseCaptureConfig(t), "openai_chat", "text_generate")
	var err error
	for field, value := range map[string]interface{}{
		"native_identity": map[string]interface{}{
			"release_id": "opaque-release-without-prefix-semantics",
			"target_id":  "original-target", "route_id": nativeCacheTestRoute,
			"policy_id": nativeCacheTestPolicy, "policy_version": nativeCacheTestVersion,
		},
		"cache_policy_version": strconv.FormatInt(nativeCacheTestVersion, 10),
		"console_lookup": map[string]interface{}{
			"enabled": true, "service_name": thinConsoleService, "service_port": 8080,
			"path": thinConsolePath, "bearer_token": thinConsoleToken, "timeout": 50,
		},
		"route_policy.enable_console_lookup": true,
	} {
		raw, err = sjson.SetBytes(raw, field, value)
		require.NoError(t, err)
	}
	return raw
}

func nativeCacheTestHeaders(caller balanceidentity.Snapshot) [][2]string {
	headers := [][2]string{
		{":authority", "example.com"}, {":path", "/native/custom-answer"}, {":method", "POST"},
		{"content-type", "application/json"}, {"x-mse-tenant", "untrusted-tenant-label"},
		{"x-mse-consumer", "untrusted-consumer-label"}, {"x-mse-session", "session-a"},
		{"x-request-id", "native-cache-request"},
	}
	return append(headers, caller.Headers()...)
}

func TestNativeCacheIdentityLookupAndEventShareOriginalCaller(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		caller := nativeCacheTestCaller()
		host, status := test.NewTestHost(nativeCacheTestConfig(t))
		require.Equal(t, types.OnPluginStartStatusOK, status)
		defer host.Reset()
		require.NoError(t, host.SetRouteName("rel-diagnostic-route-name-is-not-an-identity"))
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheTestHeaders(caller)))
		body := []byte(nativeCacheTestBody)
		require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
		require.Equal(t, body, host.GetRequestBody(), "cache must retain the original public model and body")
		calls := host.GetRedisCalloutAttributes()
		require.Len(t, calls, 1)
		command, ok := thinResponseCaptureCommand(t, calls[0].Query)
		require.True(t, ok)
		require.Len(t, command, 2)
		adapter, ok := thinProtocolAdapterForKind(protocol.ProtocolChatCompletions)
		require.True(t, ok)
		digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{Method: "POST", Path: "/native/custom-answer", Body: body})
		require.NoError(t, err)
		expected, err := BuildScopedCacheKeyMaterial(ScopedCacheKeyInput{
			KeyPrefix: "cache:materialized:", Tenant: caller.TenantID, Consumer: caller.PrincipalID,
			CacheScope: "consumer", Route: nativeCacheTestRoute, Model: "public-alias",
			Protocol: protocol.ProtocolChatCompletions, RequestDigest: digest.Digest,
			CachePolicyVersion: strconv.FormatInt(nativeCacheTestVersion, 10),
		})
		require.NoError(t, err)
		require.Equal(t, expected.RedisKey, command[1])

		host.CallOnRedisCall(0, test.CreateRedisRespNull())
		httpCalls := host.GetHttpCalloutAttributes()
		require.Len(t, httpCalls, 1)
		require.True(t, test.HasHeaderWithValue(httpCalls[0].Headers, "authorization", "Bearer "+thinConsoleToken))
		lookup := nativeCacheDecodeObject(t, httpCalls[0].Body)
		require.Equal(t, caller.TenantID, lookup["tenant"])
		require.Equal(t, caller.PrincipalID, lookup["consumer"])
		require.Equal(t, nativeCacheTestRoute, lookup["route"])
		require.Equal(t, "public-alias", lookup["model"])
		require.Equal(t, strconv.FormatInt(nativeCacheTestVersion, 10), lookup["cache_policy_version"])
		native, ok := lookup["native_identity"].(map[string]interface{})
		require.True(t, ok, "Console receives the explicit native identity rather than legacy label lookup")
		require.Equal(t, "opaque-release-without-prefix-semantics", native["release_id"])
		require.Equal(t, "original-target", native["target_id"])
		require.Equal(t, nativeCacheTestRoute, native["route_id"])
		require.Equal(t, nativeCacheTestPolicy, native["policy_id"])
		require.Equal(t, json.Number("9007199254740993"), native["policy_version"])
		encodedCaller, err := json.Marshal(caller)
		require.NoError(t, err)
		require.Equal(t, nativeCacheDecodeObject(t, encodedCaller), native["caller"])

		host.CallOnHttpCall([][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(`{"data":{"decision":"miss"}}`))
		// The original caller was captured before a later filter replaces or removes
		// these headers. Response events must never read a later request identity.
		for _, header := range balanceidentity.HeaderNames() {
			require.NoError(t, proxywasm.RemoveHttpRequestHeader(header))
		}
		host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
		host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3}}`))
		var event map[string]interface{}
		for _, call := range host.GetRedisCalloutAttributes() {
			command, ok := thinResponseCaptureCommand(t, call.Query)
			if !ok || len(command) < 5 || !strings.EqualFold(command[0], "xadd") {
				continue
			}
			require.Nil(t, event, "one response emits one cache event")
			fieldStart := thinResponseCaptureXADDFieldStart(t, command)
			for i := fieldStart; i+1 < len(command); i += 2 {
				if command[i] == "event" {
					event = nativeCacheDecodeObject(t, []byte(command[i+1]))
				}
			}
		}
		require.NotNil(t, event)
		require.Equal(t, native, event["native_identity"], "lookup and event retain the same exact caller and integer version")
		for _, label := range []string{"tenant", "consumer", "route", "model", "request_digest", "cache_policy_version"} {
			require.Equal(t, lookup[label], event[label], label)
		}
	})
}

func TestNativeCacheIdentityMissingCallerNeverFallsBack(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		caller := nativeCacheTestCaller()
		identityHeaders := balanceidentity.HeaderNames()
		for _, tc := range []struct {
			name   string
			remove []string
			extra  [][2]string
		}{
			{"all missing", identityHeaders[:], nil},
			{"legacy three fields", []string{balanceidentity.TenantHeader, balanceidentity.PrincipalHeader}, nil},
			{"missing generation", []string{balanceidentity.GenerationHeader}, nil},
			{"noncanonical generation", []string{balanceidentity.GenerationHeader}, [][2]string{{balanceidentity.GenerationHeader, "01"}}},
			{"duplicate key", nil, [][2]string{{strings.ToUpper(balanceidentity.APIKeyHeader), caller.APIKeyID}}},
			{"nil principal", []string{balanceidentity.PrincipalHeader}, [][2]string{{balanceidentity.PrincipalHeader, "00000000-0000-0000-0000-000000000000"}}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				headers := nativeCacheTestHeaders(caller)
				for _, name := range tc.remove {
					filtered := headers[:0]
					for _, header := range headers {
						if !strings.EqualFold(header[0], name) {
							filtered = append(filtered, header)
						}
					}
					headers = filtered
				}
				headers = append(headers, tc.extra...)
				host, status := test.NewTestHost(nativeCacheTestConfig(t))
				require.Equal(t, types.OnPluginStartStatusOK, status)
				defer host.Reset()
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(headers))
				body := []byte(nativeCacheTestBody)
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(body))
				host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
				host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}]}`))
				require.Empty(t, host.GetRedisCalloutAttributes(), "no lookup or event under unbound native identity")
				require.Empty(t, host.GetHttpCalloutAttributes())
				require.Nil(t, host.GetLocalResponse(), "an auxiliary cache bypass must not reject the business call")
				require.Equal(t, body, host.GetRequestBody())
			})
		}
	})
}

func TestNativeCacheIdentityExactReplayAndScopeIsolation(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		caller := nativeCacheTestCaller()
		keys := make(map[string]string)
		for _, variant := range []string{"original", "other tenant", "other principal", "other route", "other model"} {
			t.Run(variant, func(t *testing.T) {
				cfg := nativeCacheTestConfig(t)
				actualCaller := caller
				body := []byte(nativeCacheTestBody)
				var err error
				switch variant {
				case "other tenant":
					actualCaller.TenantID = "77777777-7777-4777-8777-777777777777"
				case "other principal":
					actualCaller.PrincipalID = "88888888-8888-4888-8888-888888888888"
				case "other route":
					cfg, err = sjson.SetBytes(cfg, "native_identity.route_id", "99999999-9999-4999-8999-999999999999")
					require.NoError(t, err)
				case "other model":
					body, err = sjson.SetBytes(body, "model", "another-public-model")
					require.NoError(t, err)
				}
				host, status := test.NewTestHost(cfg)
				require.Equal(t, types.OnPluginStartStatusOK, status)
				defer host.Reset()
				require.NoError(t, host.SetRouteName("same-runtime-route-name"))
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheTestHeaders(actualCaller)))
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
				calls := host.GetRedisCalloutAttributes()
				require.Len(t, calls, 1)
				command, ok := thinResponseCaptureCommand(t, calls[0].Query)
				require.True(t, ok)
				require.Len(t, command, 2)
				keys[variant] = command[1]
				// A record accidentally returned for another namespace must be rejected
				// even if it has the same full request digest.
				adapter, _ := thinProtocolAdapterForKind(protocol.ProtocolChatCompletions)
				digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{Method: "POST", Path: "/native/custom-answer", Body: []byte(nativeCacheTestBody)})
				require.NoError(t, err)
				now := time.Now().Unix()
				record := MaterializedReplayRecord{
					SchemaVersion: materializedRecordSchemaVersion, Tenant: caller.TenantID, Consumer: caller.PrincipalID,
					Route: nativeCacheTestRoute, Model: "public-alias", Protocol: protocol.ProtocolChatCompletions,
					CacheScope: "consumer", CachePolicyVersion: strconv.FormatInt(nativeCacheTestVersion, 10), RequestDigest: digest.Digest,
					SoftExpiresAt: now + 60, HardExpiresAt: now + 3600,
					Response: json.RawMessage(`{"choices":[{"message":{"content":"cached answer"},"finish_reason":"stop"}]}`),
					Usage:    json.RawMessage(`{"input_tokens":7,"output_tokens":3}`), FinishReason: "stop",
				}
				encoded, err := json.Marshal(record)
				require.NoError(t, err)
				host.CallOnRedisCall(0, test.CreateRedisRespString(string(encoded)))
				if variant == "original" {
					require.NotNil(t, host.GetLocalResponse())
					require.Equal(t, uint32(200), host.GetLocalResponse().StatusCode)
					require.Empty(t, host.GetHttpCalloutAttributes())
				} else {
					require.Nil(t, host.GetLocalResponse(), "a mismatched replay must not become a hit")
				}
			})
		}
		require.Len(t, keys, 5)
		seen := make(map[string]string)
		for variant, key := range keys {
			require.NotContains(t, seen, key, "tenant/principal/route/public-model isolation: %s and %s", seen[key], variant)
			seen[key] = variant
		}
	})
}

func nativeCacheDecodeObject(t *testing.T, raw []byte) map[string]interface{} {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value map[string]interface{}
	require.NoError(t, decoder.Decode(&value))
	return value
}

func TestNativeCacheIdentityRetainsTheEffectiveMemoryPolicyFingerprint(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, digest := range []string{"", strings.Repeat("a", 64)} {
			t.Run(map[bool]string{true: "missing memory digest", false: "original trusted memory digest"}[digest == ""], func(t *testing.T) {
				cfg := nativeCacheTestConfig(t)
				var err error
				cfg, err = sjson.SetRawBytes(cfg, "route_policy.memory", []byte(`{"enabled":true,"cache_mode":"policy_digest","policy_version":"2"}`))
				require.NoError(t, err)
				host, status := test.NewTestHost(cfg)
				require.Equal(t, types.OnPluginStartStatusOK, status)
				defer host.Reset()
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheTestHeaders(nativeCacheTestCaller())))
				if digest != "" {
					require.NoError(t, host.SetProperty([]string{memorycache.TrustedDigestProperty}, []byte(digest)))
				}
				action := host.CallOnHttpRequestBody([]byte(nativeCacheTestBody))
				if digest == "" {
					require.Equal(t, types.ActionContinue, action)
					require.Empty(t, host.GetRedisCalloutAttributes())
					require.Empty(t, host.GetHttpCalloutAttributes())
					return
				}
				require.Equal(t, types.ActionPause, action)
				host.CallOnRedisCall(0, test.CreateRedisRespNull())
				calls := host.GetHttpCalloutAttributes()
				require.Len(t, calls, 1)
				lookup := nativeCacheDecodeObject(t, calls[0].Body)
				require.Equal(t, "9007199254740993|memory|2|"+digest+"|cache-v9007199254740993", lookup["cache_policy_version"])
				native := lookup["native_identity"].(map[string]interface{})
				require.Equal(t, json.Number("9007199254740993"), native["policy_version"], "the frozen policy revision stays separate from its effective memory fingerprint")
			})
		}
	})
}
