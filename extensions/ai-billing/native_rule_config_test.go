package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func independentNativeBillingRule(t *testing.T, target, redisService, stream string) (BillingConfig, map[string]interface{}) {
	t.Helper()
	config, value := nativeBillingConfigFixture(t)
	release := value["_modelfusion_execution"].(map[string]interface{})
	descriptor := release["descriptor"].(map[string]interface{})
	descriptor["release_id"], descriptor["target_id"] = "release-"+target, "target-"+target
	descriptor["service_ref"], descriptor["cluster_ref"] = target+".dns", "outbound|8080||"+target+".dns"
	delete(descriptor, "descriptor_digest")
	raw, err := json.Marshal(descriptor)
	require.NoError(t, err)
	descriptor["descriptor_digest"], err = nativeBillingDescriptorDigest(raw)
	require.NoError(t, err)
	value["enable_methods"], value["enable_path_suffixes"] = []string{}, []string{}
	value["_match_route_"], value["_match_service_"] = []string{"route-" + target}, []string{target + ".dns"}
	value["redis_stream"] = map[string]interface{}{
		"service_name": redisService, "service_port": 6379, "stream": stream, "timeout": 1000,
	}
	config.RedisStream = RedisStream{ServiceName: redisService, ServicePort: 6379, Stream: stream, Timeout: 1000}
	require.NoError(t, configureNativeBilling(gjson.ParseBytes(mustBillingConfig(t, value)), &config))
	return config, value
}

func independentNativeBillingRoot(rules ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"_rules_": rules}
}

func independentNativeBillingDelivery(t *testing.T, config BillingConfig, target string) executionfacts.Snapshot {
	t.Helper()
	envelope := nativeBillingDeliveryFixture(t, config).Envelope()
	envelope.CallID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(target)).String()
	envelope.PayloadDigest = ""
	snapshot, err := executionfacts.Freeze(envelope)
	require.NoError(t, err)
	return snapshot
}

// The real init-registered parser and response callback run in both Go and Wasm
// hosts. Redis replies are simulated; this does not claim a live Stream grant.
func TestNativeBillingRuleOnlyTargetsDeliverToTheirOwnRedisAndStream(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		firstConfig, first := independentNativeBillingRule(t, "native-a", "redis-native-a.static", "private-native-a")
		secondConfig, second := independentNativeBillingRule(t, "native-b", "redis-native-b.static", "private-native-b")
		for _, tc := range []struct {
			target string
			config BillingConfig
		}{
			{"native-a", firstConfig}, {"native-b", secondConfig},
		} {
			for _, dependencyError := range []bool{false, true} {
				host, status := test.NewTestHost(mustBillingConfig(t, independentNativeBillingRoot(first, second)))
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("route-"+tc.target))
				require.NoError(t, host.SetClusterName("outbound|8080||"+tc.target+".dns"))
				headers := append([][2]string{
					{":authority", "example.test"}, {":path", "/custom/customer/alias"}, {":method", http.MethodPost},
				}, nativeBillingAdmittedHeaders(tc.config, uuid.NewSHA1(uuid.NameSpaceURL, []byte(tc.target)).String())...)
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(headers))
				require.Empty(t, host.GetRedisCalloutAttributes())
				require.Empty(t, host.GetHttpCalloutAttributes())
				snapshot := independentNativeBillingDelivery(t, tc.config, tc.target)
				require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders([][2]string{
					{":status", "200"}, {"content-type", "application/json"},
					{executionfacts.DeliveryBridgeHeader, nativeBillingBridgeFixture(t, snapshot)},
				}))
				calls := host.GetRedisCalloutAttributes()
				require.Len(t, calls, 1)
				require.Contains(t, calls[0].Upstream, tc.config.RedisStream.ServiceName)
				require.Contains(t, string(calls[0].Query), tc.config.RedisStream.Stream)
				require.Contains(t, string(calls[0].Query), "XADD")
				require.Contains(t, string(calls[0].Query), "observation")
				require.Equal(t, 1, bytes.Count(calls[0].Query, snapshot.Bytes()))
				for _, header := range host.GetResponseHeaders() {
					require.NotEqual(t, executionfacts.DeliveryBridgeHeader, header[0])
				}
				if dependencyError {
					host.CallOnRedisCall(0, test.CreateRedisRespError("controlled dependency failure"))
				} else {
					host.CallOnRedisCall(0, test.CreateRedisRespString("20-0"))
				}
				require.Nil(t, host.GetLocalResponse())
				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte(`{"model":"unmodified-client-response"}`)))
				host.CompleteHttp()
				require.Empty(t, host.GetRedisCalloutAttributes())
				require.Empty(t, host.GetHttpCalloutAttributes())
				host.Reset()
			}
		}
	})
}

func TestNativeBillingScopedRuleStillInheritsAnInitializedGlobalRedis(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		config, rule := independentNativeBillingRule(t, "native-inherited", "redis-global.static", "private-native-global")
		redis := rule["redis_stream"]
		delete(rule, "redis_stream")
		root := map[string]interface{}{
			"redis_stream": redis, "enable_methods": []string{http.MethodDelete},
			"enable_path_suffixes": []string{"/legacy-only"}, "_rules_": []map[string]interface{}{rule},
		}
		host, status := test.NewTestHost(mustBillingConfig(t, root))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		require.NoError(t, host.SetRouteName("route-native-inherited"))
		require.NoError(t, host.SetClusterName("outbound|8080||native-inherited.dns"))
		headers := append([][2]string{{":authority", "example.test"}, {":path", "/custom/native/alias"}, {":method", http.MethodPost}}, nativeBillingAdmittedHeaders(config, uuid.NewSHA1(uuid.NameSpaceURL, []byte("native-inherited")).String())...)
		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(headers))
		snapshot := independentNativeBillingDelivery(t, config, "native-inherited")
		require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders([][2]string{
			{":status", "200"}, {"content-type", "application/json"},
			{executionfacts.DeliveryBridgeHeader, nativeBillingBridgeFixture(t, snapshot)},
		}))
		calls := host.GetRedisCalloutAttributes()
		require.Len(t, calls, 1)
		require.Contains(t, calls[0].Upstream, "redis-global.static")
		require.Contains(t, string(calls[0].Query), "private-native-global")
		require.Equal(t, 1, bytes.Count(calls[0].Query, snapshot.Bytes()))
		host.CallOnRedisCall(0, test.CreateRedisRespString("21-0"))
		host.CompleteHttp()
		require.Empty(t, host.GetRedisCalloutAttributes())
	})
}

func TestNativeBillingInitializedGlobalCannotBeOverriddenByACompleteRule(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		_, rule := independentNativeBillingRule(t, "native-a", "redis-route.static", "private-route")
		for _, redis := range []interface{}{
			map[string]interface{}{"service_name": "redis-global.static", "stream": "private-global"},
			rule["redis_stream"],
		} {
			root := map[string]interface{}{"redis_stream": redis, "_rules_": []map[string]interface{}{rule}}
			host, status := test.NewTestHost(mustBillingConfig(t, root))
			require.Equal(t, types.OnPluginStartStatusFailed, status)
			host.Reset()
		}
	})
}

func TestNativeBillingBadGlobalCannotUseAnIndependentRuleAsFallback(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for name, root := range map[string]map[string]interface{}{
			"unknown root":        {"unknown": true},
			"missing connection":  {"provider": "openai"},
			"invalid connection":  {"redis_stream": map[string]interface{}{"service_name": ""}},
			"invalid fail policy": {"fail_policy": "close"},
			"invalid event kind":  {"event_kind": "invoice"},
			"invalid path scope":  {"path_scope": "future-scope"},
			"invalid methods":     {"enable_methods": 42},
			"removed service":     {"billing_service": map[string]interface{}{"service_name": "old-http-service"}},
		} {
			t.Run(name, func(t *testing.T) {
				_, rule := independentNativeBillingRule(t, "native-a", "redis-route.static", "private-route")
				root["_rules_"] = []map[string]interface{}{rule}
				host, status := test.NewTestHost(mustBillingConfig(t, root))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
	})
}

func TestNativeBillingCannotBeInstalledInTheGlobalRoot(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, withRule := range []bool{false, true} {
			_, root := independentNativeBillingRule(t, "native-global", "redis-global.static", "private-global")
			delete(root, "_match_route_")
			delete(root, "_match_service_")
			if withRule {
				_, rule := independentNativeBillingRule(t, "native-a", "redis-route.static", "private-route")
				root["_rules_"] = []map[string]interface{}{rule}
			}
			host, status := test.NewTestHost(mustBillingConfig(t, root))
			require.Equal(t, types.OnPluginStartStatusFailed, status)
			host.Reset()
		}
	})
}

func TestNativeBillingIndependentRulesRequireCompletePairedConfiguration(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, missing := range []string{"redis_stream", "native_execution", "_modelfusion_execution"} {
			_, rule := independentNativeBillingRule(t, "native-a", "redis-route.static", "private-route")
			delete(rule, missing)
			host, status := test.NewTestHost(mustBillingConfig(t, independentNativeBillingRoot(rule)))
			require.Equal(t, types.OnPluginStartStatusFailed, status, missing)
			host.Reset()
		}
		_, rule := independentNativeBillingRule(t, "native-a", "redis-route.static", "private-route")
		rule["_rules_"] = []map[string]interface{}{{"_match_route_": []string{"nested"}}}
		host, status := test.NewTestHost(mustBillingConfig(t, independentNativeBillingRoot(rule)))
		require.Equal(t, types.OnPluginStartStatusFailed, status)
		host.Reset()
	})
}

func TestNativeBillingUnmatchedRuleDoesNotAccessAnyDeliveryBackend(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, tc := range []struct{ route, service string }{
			{"other-route", "native-a.dns"}, {"route-native-a", "other-target.dns"},
		} {
			_, rule := independentNativeBillingRule(t, "native-a", "redis-route.static", "private-route")
			host, status := test.NewTestHost(mustBillingConfig(t, independentNativeBillingRoot(rule)))
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName(tc.route))
			require.NoError(t, host.SetClusterName("outbound|8080||"+tc.service))
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.test"}, {":path", "/unrelated/business"}, {":method", http.MethodPost},
			}))
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}}))
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte(`{"usage":{"prompt_tokens":10,"completion_tokens":20}}`)))
			host.CompleteHttp()
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			require.Nil(t, host.GetLocalResponse())
			host.Reset()
		}
	})
}
