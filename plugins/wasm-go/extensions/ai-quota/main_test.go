package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

var monetaryConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]any{
		"balance_scope_id": testScope, "cache_generation": testGeneration, "projection_schema": "balance.v1",
		"missing_balance_policy": "deny", "enable_path_suffixes": []string{"/v1/chat/completions", "/v1/messages"},
		"redis": map[string]any{"service_name": "redis.static", "service_port": 6379, "timeout": 1000, "database": 0},
	})
	return data
}()

func quotaConfigWith(overrides map[string]interface{}) json.RawMessage {
	var config map[string]interface{}
	_ = json.Unmarshal(monetaryConfig, &config)
	for key, value := range overrides {
		config[key] = value
	}
	data, _ := json.Marshal(config)
	return data
}

func nativeAdmissionHeaders(path, method string) [][2]string {
	return append([][2]string{{":authority", "example.test"}, {":path", path}, {":method", method}}, testIdentity.Headers()...)
}

// The simulated host tests transport/action handling; the Lua decision itself is
// separately exercised against actual Redis in balance_projection_test.go.
func admissionReply(reason string) []byte {
	result := balanceAdmission{Decision: "deny", ReasonCode: reason}
	if reason == "ok" {
		result.Decision = "allow"
	}
	if reason == "ok" || reason == "insufficient_balance" {
		result.ObservedAccountVersion = "9007199254740993"
		result.ObservedGeneration = testGeneration
	}
	raw, _ := json.Marshal(result)
	return test.CreateRedisRespString(string(raw))
}

func TestParseConfig(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(monetaryConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		match, err := host.GetMatchConfig()
		require.NoError(t, err)
		config := match.(*QuotaConfig)
		require.Equal(t, testNamespace, config.namespace)
		require.Equal(t, []string{"/v1/chat/completions", "/v1/messages"}, config.EnablePathSuffixes)
	})
}

func TestRemovedSettlementConfigIsRejected(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		removedFields := map[string]interface{}{
			"price_key_template":   "billing:effective_price:{tenant}:{provider}:{model}:{token_type}",
			"amount_scale":         1000000,
			"price_unit_tokens":    1000000,
			"missing_price_policy": "skip",
			"missing_usage_policy": "skip",
		}

		for field, value := range removedFields {
			t.Run("global rejects "+field, func(t *testing.T) {
				host, status := test.NewTestHost(quotaConfigWith(map[string]interface{}{
					field: value,
				}))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})

			t.Run("rule rejects "+field, func(t *testing.T) {
				host, status := test.NewTestHost(quotaConfigWith(map[string]interface{}{
					"_rules_": []map[string]interface{}{
						{
							"_match_route_": []string{"route-removed-settlement"},
							field:           value,
						},
					},
				}))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
	})
}

func TestRequestAdmission(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			reason string
			status int
		}{
			{"ok", 0}, {"insufficient_balance", 403}, {"balance_missing", 403},
			{"authorization_stale", 403}, {"route_authorization_missing", 403}, {"account_mismatch", 403},
			{"projection_unready", 503}, {"wrong_generation", 503},
			{"projection_schema_mismatch", 503}, {"invalid_amount", 503}, {"dependency_unavailable", 503},
		} {
			t.Run(tc.reason, func(t *testing.T) {
				host, status := test.NewTestHost(monetaryConfig)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				action := host.CallOnHttpRequestHeaders(nativeAdmissionHeaders("/v1/chat/completions", http.MethodPost))
				require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)
				calls := host.GetRedisCalloutAttributes()
				require.Len(t, calls, 1)
				require.Contains(t, string(calls[0].Query), testAccountKey())
				require.Contains(t, string(calls[0].Query), ":principal:key:"+testAPIKey)
				host.CallOnRedisCall(0, admissionReply(tc.reason))
				if tc.status == 0 {
					require.Nil(t, host.GetLocalResponse())
					require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				} else {
					require.Equal(t, uint32(tc.status), host.GetLocalResponse().StatusCode)
					require.NotEqual(t, types.ActionContinue, host.GetHttpStreamAction())
				}
				require.Empty(t, host.GetRedisCalloutAttributes())
				host.CompleteHttp()
			})
		}
	})
}

func TestMalformedRedisResultCannotResumeAModelRequest(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for name, response := range map[string][]byte{
			"null": test.CreateRedisRespNull(), "old integer": test.CreateRedisRespInt(100),
			"old string":           test.CreateRedisRespString("100"),
			"missing observations": test.CreateRedisRespString(`{"decision":"allow","reason_code":"ok"}`),
			"wrong generation":     test.CreateRedisRespString(`{"decision":"allow","reason_code":"ok","observed_account_version":"1","observed_generation":"old"}`),
			"unknown reason":       test.CreateRedisRespString(`{"decision":"deny","reason_code":"allow_missing"}`),
			"dependency failure":   test.CreateRedisRespError("owned test dependency failure"),
		} {
			t.Run(name, func(t *testing.T) {
				host, status := test.NewTestHost(monetaryConfig)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders(nativeAdmissionHeaders("/v1/chat/completions", http.MethodPost))
				host.CallOnRedisCall(0, response)
				require.Equal(t, uint32(http.StatusServiceUnavailable), host.GetLocalResponse().StatusCode)
				require.NotEqual(t, types.ActionContinue, host.GetHttpStreamAction())
				host.CompleteHttp()
			})
		}
	})
}

func TestClientAliasDoesNotSupplyTheNativeWalletIdentity(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(monetaryConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		host.CallOnHttpRequestHeaders([][2]string{{":authority", "example.test"}, {":path", "/v1/chat/completions"}, {":method", "POST"}, {"x-mse-tenant", "client-tenant"}, {"x-mse-consumer", "client-owner"}})
		require.Equal(t, uint32(http.StatusForbidden), host.GetLocalResponse().StatusCode)
		require.Empty(t, host.GetRedisCalloutAttributes())
		host.CompleteHttp()
	})
}

func TestAdmissionOnlyDoesNotDeduct(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(monetaryConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		host.CallOnHttpRequestHeaders(nativeAdmissionHeaders("/v1/chat/completions", http.MethodPost))
		host.CallOnRedisCall(0, admissionReply("ok"))
		body := []byte(`{"usage":{"prompt_tokens":1001,"completion_tokens":2000,"total_tokens":3001}}`)
		require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingResponseBody(body, true))
		require.Empty(t, host.GetRedisCalloutAttributes())
		require.Nil(t, host.GetLocalResponse())
		host.CompleteHttp()
	})
}

func TestKeyBuilders(t *testing.T) {
	keys := testNamespace.keys(testIdentity)
	require.Equal(t, keys, (balanceNamespace{ScopeID: testScope, Generation: testAPIKey}).keys(testIdentity))
	require.Equal(t, "mf:balance:{"+testScope+"}:control", keys[0])
	require.Equal(t, "mf:balance:{"+testScope+"}:account:"+testAccount, keys[1])
	require.Equal(t, keys[1].(string)+":principal:key:"+testAPIKey, keys[2])
	require.False(t, isAIPathEnabled("/v1/chat/completions/quota", []string{"/v1/chat/completions"}))
	require.True(t, isAIPathEnabled("/proxy/v1/messages?debug=true", []string{"/v1/messages"}))
}
