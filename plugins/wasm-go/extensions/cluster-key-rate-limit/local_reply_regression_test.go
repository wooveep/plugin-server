package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/resp"
)

func rpmEvalResp(threshold, current, ttl int) []byte {
	raw, _ := resp.ArrayValue([]resp.Value{
		resp.ArrayValue([]resp.Value{
			resp.IntegerValue(threshold),
			resp.IntegerValue(current),
			resp.IntegerValue(ttl),
		}),
	}).MarshalRESP()
	return raw
}

// F06 / U04 / U07: cluster-key-rate-limit emits "cluster-key-rate-limit.rejected",
// which is intentionally outside test-sp003 release_responses.rs mappers.
// Verify static RPM positive thresholds, zero rejection at config boundary,
// unmatched consumer pass-through, and non-AI plaintext rejection preservation.
func TestClusterKeyRateLimitLocalReplyRegressionU04U07(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("zero static RPM threshold is rejected at config boundary", func(t *testing.T) {
			zeroCfg, err := json.Marshal(map[string]any{
				"rule_name": "rpm-zero",
				"global_threshold": map[string]any{
					"query_per_minute": 0,
				},
				"redis": map[string]any{
					"service_name": "redis.static",
					"service_port": 6379,
				},
			})
			require.NoError(t, err)
			host, status := test.NewTestHost(zeroCfg)
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})
	})

	test.RunTest(t, func(t *testing.T) {
		cfg, err := json.Marshal(map[string]any{
			"_rules_": []map[string]any{
				{
					"_match_route_": []string{"non-ai-api"},
					"rule_name":     "route-rpm-limit",
					"rule_items": []map[string]any{
						{
							"limit_by_consumer": "",
							"limit_keys": []map[string]any{
								{
									"key":              "limited-consumer",
									"query_per_minute": 10,
								},
							},
						},
					},
					"redis": map[string]any{
						"service_name": "redis.static",
						"service_port": 6379,
					},
					"show_limit_quota_header": true,
					"rejected_code":           429,
					"rejected_msg":            "Too many gateway requests",
				},
			},
		})
		require.NoError(t, err)

		t.Run("unmatched consumer passes through without Redis call", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("non-ai-api")

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "gateway.example.com"},
				{":path", "/api/v1/orders"},
				{":method", http.MethodGet},
				{"x-mse-consumer", "unlimited-consumer"},
			})
			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Nil(t, host.GetLocalResponse())
		})

		t.Run("positive static RPM preserves plaintext rejection and X-RateLimit-Reset", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("non-ai-api")

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "gateway.example.com"},
				{":path", "/api/v1/orders"},
				{":method", http.MethodPost},
				{"x-mse-consumer", "limited-consumer"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)
			calls := host.GetRedisCalloutAttributes()
			require.Len(t, calls, 1)
			require.Contains(t, string(calls[0].Query), "higress-cluster-key-rate-limit:{route-rpm-limit}:limit_by_consumer:60:x-mse-consumer:limited-consumer")

			host.CallOnRedisCall(0, rpmEvalResp(10, 11, 25))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(429), resp.StatusCode)
			require.Equal(t, "cluster-key-rate-limit.rejected", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"X-RateLimit-Reset", "25"})
			require.Equal(t, "Too many gateway requests", string(resp.Data))
		})
	})
}
