package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func quotaDormantTargetRule(route, service, redisService, scope, generation string) map[string]interface{} {
	rule := quotaRoutingContractConfig()
	for key, value := range executionAdmissionFixture("production") {
		rule[key] = value
	}
	rule["balance_scope_id"] = scope
	rule["cache_generation"] = generation
	rule["redis"] = map[string]interface{}{
		"service_name": redisService, "service_port": 6379, "timeout": 1000, "database": 0,
	}
	rule["_match_route_"] = []string{route}
	rule["_match_service_"] = []string{service}
	return rule
}

func quotaDormantRoot(rules ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"_rules_": rules}
}

func quotaDormantBalanceReply(t *testing.T, generation string) []byte {
	t.Helper()
	raw := executionAdmissionJSON(t, balanceAdmission{
		Decision: "allow", ReasonCode: "ok", ObservedAccountVersion: "1", ObservedGeneration: generation,
	})
	return test.CreateRedisRespString(string(raw))
}

type quotaDormantContext struct {
	wrapper.HttpContext
	rerouteCalls      int
	requestBodySkips  int
	responseBodySkips int
}

func (ctx *quotaDormantContext) DisableReroute()       { ctx.rerouteCalls++ }
func (ctx *quotaDormantContext) DontReadRequestBody()  { ctx.requestBodySkips++ }
func (ctx *quotaDormantContext) DontReadResponseBody() { ctx.responseBodySkips++ }

func TestQuotaExplicitDormantRootSkipsBodiesWithoutChangingReroute(t *testing.T) {
	config := QuotaConfig{ExecutionAdmission: "stale-authority"}
	require.NoError(t, parseConfig(gjson.Parse(`{"_rules_":[]}`), &config))
	require.True(t, config.dormant)
	require.Nil(t, config.invalidBase)
	require.Nil(t, config.redisClient)
	require.Nil(t, config.ExecutionAdmission)
	ctx := &quotaDormantContext{}
	require.Equal(t, types.ActionContinue, onHttpRequestHeaders(ctx, config))
	require.Zero(t, ctx.rerouteCalls)
	require.Equal(t, 1, ctx.requestBodySkips)
	require.Equal(t, 1, ctx.responseBodySkips)
	for _, raw := range []string{`{}`, `{"unknown":true}`, `{"_rules_":null}`, `{"_rules_":{},"unknown":true}`} {
		require.Error(t, parseConfig(gjson.Parse(raw), &config), raw)
		require.False(t, config.dormant, raw)
	}
}

func TestQuotaRuleOnlyRootParsesEachCompleteTargetIndependently(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		first := quotaDormantTargetRule("release-a", "mft-a.dns", "redis-a.static", testScope, testGeneration)
		secondScope := "60000000-0000-0000-0000-000000000002"
		secondGeneration := "70000000-0000-0000-0000-000000000002"
		second := quotaDormantTargetRule("release-b", "mft-b.dns", "redis-b.static", secondScope, secondGeneration)
		second["execution_admission"].(map[string]interface{})["release_id"] = "release-second"
		second["execution_admission"].(map[string]interface{})["route_id"] = "50000000-0000-0000-0000-000000000002"
		host, status := test.NewTestHost(mustQuotaRoutingConfig(t, quotaDormantRoot(first, second)))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		for _, tc := range []struct {
			route, service, redis, scope, generation, release string
		}{
			{"release-a", "mft-a.dns", "redis-a.static", testScope, testGeneration, "release-original"},
			{"release-b", "mft-b.dns", "redis-b.static", secondScope, secondGeneration, "release-second"},
		} {
			require.NoError(t, host.SetRouteName(tc.route))
			require.NoError(t, host.SetClusterName("outbound|8080||"+tc.service))
			matched, err := host.GetMatchConfig()
			require.NoError(t, err)
			config := matched.(*QuotaConfig)
			require.False(t, config.dormant)
			require.Nil(t, config.invalidBase)
			require.Equal(t, balanceNamespace{ScopeID: tc.scope, Generation: tc.generation}, config.namespace)
			require.Equal(t, tc.redis, config.redisInfo.ServiceName)
			require.Equal(t, PathScopeBoundTarget, config.PathScope)
			require.Empty(t, config.EnableMethods)
			require.Empty(t, config.EnablePathSuffixes)
			require.Equal(t, tc.release, config.ExecutionAdmission.(*executionAdmissionConfig).ReleaseID)
		}
	})
}

// These use the actual init-registered parser and request handler in Go and
// Wasm hosts. The mocked dependencies prove callback ordering, not live grants.
func TestQuotaRuleOnlyMatchedTargetRunsBalanceThenFreshCallAdmission(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		seen := map[string]bool{}
		for _, tc := range []struct {
			method, route, service, redis, scope, generation, release string
		}{
			{http.MethodPost, "release-a", "mft-a.dns", "redis-a.static", testScope, testGeneration, "release-original"},
			{http.MethodGet, "release-b", "mft-b.dns", "redis-b.static", "60000000-0000-0000-0000-000000000002", "70000000-0000-0000-0000-000000000002", "release-second"},
		} {
			t.Run(tc.method, func(t *testing.T) {
				first := quotaDormantTargetRule("release-a", "mft-a.dns", "redis-a.static", testScope, testGeneration)
				second := quotaDormantTargetRule("release-b", "mft-b.dns", "redis-b.static", "60000000-0000-0000-0000-000000000002", "70000000-0000-0000-0000-000000000002")
				second["execution_admission"].(map[string]interface{})["release_id"] = "release-second"
				second["execution_admission"].(map[string]interface{})["route_id"] = "50000000-0000-0000-0000-000000000002"
				host, status := test.NewTestHost(mustQuotaRoutingConfig(t, quotaDormantRoot(first, second)))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName(tc.route))
				require.NoError(t, host.SetClusterName("outbound|8080||"+tc.service))
				headers := append([][2]string{
					{":authority", "example.test"}, {":path", "/custom/customer/alias"}, {":method", tc.method},
					{executionAdmissionCallHeader, "forged-client-call"},
					{executionAdmissionAttemptHeader, "forged-client-attempt"},
					{executionAdmissionReleaseHeader, "forged-client-release"},
					{executionAdmissionDeadlineHeader, "forged-client-deadline"},
				}, executionAdmissionCaller.Headers()...)
				require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.CallOnHttpRequestHeaders(headers))
				redis := host.GetRedisCalloutAttributes()
				require.Len(t, redis, 1)
				require.Contains(t, redis[0].Upstream, tc.redis)
				require.Contains(t, string(redis[0].Query), "mf:balance:{"+tc.scope+"}")
				require.Contains(t, string(redis[0].Query), tc.generation)
				require.Empty(t, host.GetHttpCalloutAttributes())
				host.CallOnRedisCall(0, quotaDormantBalanceReply(t, tc.generation))
				require.Empty(t, host.GetRedisCalloutAttributes())
				require.Empty(t, host.GetHttpCalloutAttributes())
				origin, err := executionfacts.ReadAdmittedCall(host.GetRequestHeaders(), tc.release, time.Now())
				require.NoError(t, err)
				callID, err := uuid.Parse(origin.CallID)
				require.NoError(t, err)
				require.Equal(t, uuid.Version(4), callID.Version())
				require.NotEqual(t, "forged-client-call", origin.CallID)
				require.False(t, seen[origin.CallID])
				seen[origin.CallID] = true
				require.Nil(t, host.GetLocalResponse())
				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				require.Empty(t, host.GetHttpCalloutAttributes())
				require.Empty(t, host.GetRedisCalloutAttributes())
				callHeader, present := executionAdmissionSingleHeader(host.GetRequestHeaders(), executionAdmissionCallHeader)
				require.True(t, present)
				require.Equal(t, origin.CallID, callHeader)
				require.Zero(t, executionAdmissionHeaderCount(host.GetRequestHeaders(), executionAdmissionAttemptHeader))
				release, present := executionAdmissionSingleHeader(host.GetRequestHeaders(), executionAdmissionReleaseHeader)
				require.True(t, present)
				require.Equal(t, tc.release, release)
				host.CompleteHttp()
			})
		}
	})
}

func TestQuotaRuleOnlyUnmatchedRouteOrServiceDoesNotCallDependencies(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, tc := range []struct{ route, service string }{
			{"unrelated-route", "mft-a.dns"},
			{"release-a", "unrelated-target.dns"},
			{"unrelated-route", "unrelated-target.dns"},
		} {
			rule := quotaDormantTargetRule("release-a", "mft-a.dns", "redis-a.static", testScope, testGeneration)
			host, status := test.NewTestHost(mustQuotaRoutingConfig(t, quotaDormantRoot(rule)))
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName(tc.route))
			require.NoError(t, host.SetClusterName("outbound|8080||"+tc.service))
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.test"}, {":path", "/unrelated/business"}, {":method", http.MethodPost},
			}))
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			require.Nil(t, host.GetLocalResponse())
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody([]byte("unrelated opaque body")))
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			host.CompleteHttp()
			host.Reset()
		}
	})
}

func TestQuotaBadRootCannotBecomeDormantThroughACompleteRule(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for name, root := range map[string]map[string]interface{}{
			"unknown root":       {"unknown": true},
			"partial redis":      {"redis": map[string]interface{}{"service_name": "redis.partial"}},
			"partial namespace":  {"balance_scope_id": testScope},
			"removed settlement": {"amount_scale": 1000000},
			"legacy authority":   {"consumer_header": "client-owner"},
		} {
			t.Run(name, func(t *testing.T) {
				root["_rules_"] = []map[string]interface{}{quotaDormantTargetRule("release-a", "mft-a.dns", "redis-a.static", testScope, testGeneration)}
				host, status := test.NewTestHost(mustQuotaRoutingConfig(t, root))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
		for _, raw := range []json.RawMessage{[]byte(`{}`), []byte(`{"unknown":true}`), []byte(`{"_rules_":null}`), []byte(`{"unknown":true,"_rules_":[]}`)} {
			host, status := test.NewTestHost(raw)
			require.Equal(t, types.OnPluginStartStatusFailed, status)
			host.Reset()
		}
	})
}

func TestQuotaRuleOnlyTargetStillRequiresCompleteConfiguration(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, field := range []string{"redis", "balance_scope_id", "cache_generation", "projection_schema"} {
			rule := quotaDormantTargetRule("release-a", "mft-a.dns", "redis-a.static", testScope, testGeneration)
			delete(rule, field)
			host, status := test.NewTestHost(mustQuotaRoutingConfig(t, quotaDormantRoot(rule)))
			require.Equal(t, types.OnPluginStartStatusFailed, status, field)
			host.Reset()
		}
		rule := quotaDormantTargetRule("release-a", "mft-a.dns", "redis-a.static", testScope, testGeneration)
		rule["execution_admission"].(map[string]interface{})["release_id"] = ""
		host, status := test.NewTestHost(mustQuotaRoutingConfig(t, quotaDormantRoot(rule)))
		require.Equal(t, types.OnPluginStartStatusFailed, status)
		host.Reset()
		rule = quotaDormantTargetRule("release-a", "mft-a.dns", "redis-a.static", testScope, testGeneration)
		rule["_rules_"] = []map[string]interface{}{{"_match_route_": []string{"nested"}}}
		host, status = test.NewTestHost(mustQuotaRoutingConfig(t, quotaDormantRoot(rule)))
		require.Equal(t, types.OnPluginStartStatusFailed, status)
		host.Reset()
	})
}

func TestQuotaRealGlobalStillRejectsRedisAndGenerationOverrides(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, field := range []string{"redis", "balance_scope_id", "cache_generation", "projection_schema", "missing_balance_policy"} {
			root := quotaRoutingContractConfig()
			root["_rules_"] = []map[string]interface{}{{"_match_route_": []string{"release-a"}, field: root[field]}}
			host, status := test.NewTestHost(mustQuotaRoutingConfig(t, root))
			require.Equal(t, types.OnPluginStartStatusFailed, status, field)
			host.Reset()
		}
	})
}
