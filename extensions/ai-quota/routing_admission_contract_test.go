package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestQuotaRoutingConfigurationContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("bound target accepts explicitly empty advanced filters", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["path_scope"] = "bound_target"
			config["enable_path_suffixes"] = []string{}
			config["enable_methods"] = []string{}

			host, status := test.NewTestHost(mustQuotaRoutingConfig(t, config))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
		})

		for _, field := range []string{"enable_path_suffixes", "enable_methods"} {
			t.Run("global blank-only "+field+" fails closed", func(t *testing.T) {
				config := quotaRoutingContractConfig()
				config["path_scope"] = "bound_target"
				config[field] = []string{"   "}

				host, status := test.NewTestHost(mustQuotaRoutingConfig(t, config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})

			t.Run("rule blank-only "+field+" fails closed", func(t *testing.T) {
				config := quotaRoutingContractConfig()
				config["_rules_"] = []map[string]interface{}{
					{
						"_match_route_": []string{"route-blank-filter"},
						"path_scope":    "bound_target",
						field:           []string{"   "},
					},
				}

				host, status := test.NewTestHost(mustQuotaRoutingConfig(t, config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}

		t.Run("rule inherits global admission infrastructure and overrides route context", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["quota_scope"] = "global-scope"
			config["provider"] = "openai"
			config["tenant_header"] = "x-global-tenant"
			config["consumer_header"] = "x-global-consumer"
			config["missing_balance_policy"] = "allow"
			config["enable_path_suffixes"] = []string{"/v1/chat/completions"}
			config["redis"] = map[string]interface{}{
				"service_name": "redis.global",
				"service_port": 6382,
				"timeout":      901,
				"database":     5,
			}
			config["_rules_"] = []map[string]interface{}{
				{
					"_match_route_": []string{"route-native-inherited"},
					"path_scope":    "bound_target",
					"quota_scope":   "route-scope",
					"provider":      "anthropic",
				},
			}

			host, status := test.NewTestHost(mustQuotaRoutingConfig(t, config))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-native-inherited"))

			matchConfig, err := host.GetMatchConfig()
			require.NoError(t, err)
			quotaConfig := matchConfig.(*QuotaConfig)
			require.Equal(t, "route-scope", quotaConfig.QuotaScope)
			require.Equal(t, "anthropic", quotaConfig.Provider)
			require.Equal(t, "x-global-tenant", quotaConfig.TenantHeader)
			require.Equal(t, "x-global-consumer", quotaConfig.ConsumerHeader)
			require.Equal(t, "billing:balance:{tenant}:{quota_scope}:{consumer}", quotaConfig.BalanceKeyTemplate)
			require.Equal(t, MissingPolicyAllow, quotaConfig.MissingBalancePolicy)
			require.Equal(t, "redis.global", quotaConfig.redisInfo.ServiceName)
			require.Equal(t, 6382, quotaConfig.redisInfo.ServicePort)
			require.Equal(t, 901, quotaConfig.redisInfo.Timeout)
			require.Equal(t, 5, quotaConfig.redisInfo.Database)
		})
	})
}

func TestQuotaRoutingAdmissionContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("omitted path scope retains legacy default eligibility", func(t *testing.T) {
			config := quotaRoutingContractConfig()

			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/v1/chat/completions",
				100, true, false, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/vendor/jobs/job-42",
				100, false, false, "", "",
			)
		})

		t.Run("explicit legacy scope retains explicit suffix eligibility", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["path_scope"] = "legacy_suffixes"
			config["enable_path_suffixes"] = []string{"/vendor/allowed"}

			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/v2/vendor/allowed",
				100, true, false, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/v1/chat/completions",
				100, false, false, "", "",
			)
		})

		t.Run("global bound target admits provider-native paths without filters", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["path_scope"] = "bound_target"

			runQuotaRoutingAdmission(
				t, config, "", http.MethodPatch, "/vendor/jobs/job-42:cancel",
				100, true, false, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
			runQuotaRoutingAdmission(
				t, config, "", http.MethodPatch, "/vendor/jobs/job-42:cancel",
				-1, true, true, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
		})

		t.Run("global literal-empty bound-target filters do not narrow", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["path_scope"] = "bound_target"
			config["enable_path_suffixes"] = []string{}
			config["enable_methods"] = []string{}

			runQuotaRoutingAdmission(
				t, config, "", http.MethodDelete, "/native/files/file-42",
				100, true, false, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
		})

		t.Run("rule bound target inherits global redis without legacy suffix gating", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["quota_scope"] = "global-scope"
			config["enable_path_suffixes"] = []string{"/v1/chat/completions"}
			config["redis"] = map[string]interface{}{
				"service_name": "redis.global",
				"service_port": 6382,
			}
			config["_rules_"] = []map[string]interface{}{
				{
					"_match_route_": []string{"route-native-omitted-filters"},
					"path_scope":    "bound_target",
					"quota_scope":   "route-scope",
					"provider":      "anthropic",
				},
			}

			runQuotaRoutingAdmission(
				t, config, "route-native-omitted-filters", http.MethodPut,
				"/native/messages/batches/batch-42", -1, true, true,
				"billing:balance:tenant-a:route-scope:consumer-a", "redis.global",
			)
		})

		t.Run("rule literal-empty bound-target filters do not narrow", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["enable_path_suffixes"] = []string{"/v1/chat/completions"}
			config["_rules_"] = []map[string]interface{}{
				{
					"_match_route_":        []string{"route-native-empty-filters"},
					"path_scope":           "bound_target",
					"quota_scope":          "route-empty-scope",
					"enable_path_suffixes": []string{},
					"enable_methods":       []string{},
				},
			}

			runQuotaRoutingAdmission(
				t, config, "route-native-empty-filters", http.MethodDelete,
				"/native/files/file-42", 100, true, false,
				"billing:balance:tenant-a:route-empty-scope:consumer-a", "redis.static",
			)
		})

		t.Run("nonempty suffix and method filters intersect bound target", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["path_scope"] = "bound_target"
			config["enable_path_suffixes"] = []string{"/vendor/jobs"}
			config["enable_methods"] = []string{http.MethodPost}

			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/v2/vendor/jobs",
				100, true, false, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
			runQuotaRoutingAdmission(
				t, config, "", http.MethodGet, "/v2/vendor/jobs",
				100, false, false, "", "",
			)
			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/v2/vendor/models",
				100, false, false, "", "",
			)
		})

		t.Run("method filters preserve exact case", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["path_scope"] = "bound_target"
			config["enable_methods"] = []string{"MiXeD"}

			runQuotaRoutingAdmission(
				t, config, "", "MiXeD", "/v1/chat/completions",
				100, true, false, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
			runQuotaRoutingAdmission(
				t, config, "", "mixed", "/v1/chat/completions",
				100, false, false, "", "",
			)
			runQuotaRoutingAdmission(
				t, config, "", "MIXED", "/v1/chat/completions",
				100, false, false, "", "",
			)
		})
	})
}

func quotaRoutingContractConfig() map[string]interface{} {
	return map[string]interface{}{
		"quota_scope":            "global",
		"provider":               "openai",
		"tenant_header":          "x-tenant-id",
		"consumer_header":        "x-consumer-id",
		"balance_key_template":   "billing:balance:{tenant}:{quota_scope}:{consumer}",
		"missing_balance_policy": "deny",
		"redis": map[string]interface{}{
			"service_name": "redis.static",
			"service_port": 6379,
		},
	}
}

func mustQuotaRoutingConfig(t *testing.T, config map[string]interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(config)
	require.NoError(t, err)
	return data
}

func runQuotaRoutingAdmission(
	t *testing.T,
	config map[string]interface{},
	routeName string,
	method string,
	path string,
	balance int,
	wantAdmission bool,
	wantDenied bool,
	wantBalanceKey string,
	wantRedisService string,
) {
	t.Helper()
	host, status := test.NewTestHost(mustQuotaRoutingConfig(t, config))
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)
	if routeName != "" {
		require.NoError(t, host.SetRouteName(routeName))
	}

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":method", method},
		{":path", path},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	redisCalls := host.GetRedisCalloutAttributes()
	if !wantAdmission {
		require.Equal(t, types.ActionContinue, action)
		require.Empty(t, redisCalls)
		require.Nil(t, host.GetLocalResponse())
		host.CompleteHttp()
		return
	}

	require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)
	require.Len(t, redisCalls, 1)
	require.Contains(t, string(redisCalls[0].Query), wantBalanceKey)
	require.Contains(t, redisCalls[0].Upstream, wantRedisService)

	host.CallOnRedisCall(0, test.CreateRedisRespInt(balance))
	if wantDenied {
		response := host.GetLocalResponse()
		require.NotNil(t, response)
		require.Equal(t, uint32(http.StatusForbidden), response.StatusCode)
		require.Contains(t, string(response.Data), "No monetary balance left")
	} else {
		require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
		require.Nil(t, host.GetLocalResponse())
	}
	host.CompleteHttp()
}
