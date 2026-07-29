package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestQuotaRoutingValidationRegression(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		globalCases := []struct {
			name  string
			field string
			value interface{}
		}{
			{name: "blank path scope", field: "path_scope", value: "   "},
			{name: "unknown path scope", field: "path_scope", value: "all_paths"},
			{name: "non-string path scope", field: "path_scope", value: 7},
			{name: "non-array suffix filter", field: "enable_path_suffixes", value: "/v1/chat/completions"},
			{name: "mixed-type suffix filter", field: "enable_path_suffixes", value: []interface{}{"/v1/chat/completions", 7}},
			{name: "non-array method filter", field: "enable_methods", value: http.MethodPost},
			{name: "mixed-type method filter", field: "enable_methods", value: []interface{}{http.MethodPost, 7}},
			{name: "invalid HTTP method token", field: "enable_methods", value: []string{"POST GET"}},
			{name: "explicit empty legacy suffix filter", field: "enable_path_suffixes", value: []string{}},
		}

		for _, testCase := range globalCases {
			t.Run("global rejects "+testCase.name, func(t *testing.T) {
				config := quotaRoutingContractConfig()
				config[testCase.field] = testCase.value

				host, status := test.NewTestHost(mustQuotaRoutingConfig(t, config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}

		ruleCases := []struct {
			name  string
			field string
			value interface{}
		}{
			{name: "blank path scope", field: "path_scope", value: "   "},
			{name: "unknown path scope", field: "path_scope", value: "all_paths"},
			{name: "non-string path scope", field: "path_scope", value: false},
			{name: "mixed-type suffix filter", field: "enable_path_suffixes", value: []interface{}{"/v1/chat/completions", false}},
			{name: "mixed-type method filter", field: "enable_methods", value: []interface{}{http.MethodPost, false}},
			{name: "invalid HTTP method token", field: "enable_methods", value: []string{"POST GET"}},
			{name: "explicit empty legacy suffix filter", field: "enable_path_suffixes", value: []string{}},
		}

		for _, testCase := range ruleCases {
			t.Run("rule rejects "+testCase.name, func(t *testing.T) {
				config := quotaRoutingContractConfig()
				config["_rules_"] = []map[string]interface{}{
					{
						"_match_route_": []string{"route-invalid"},
						testCase.field:  testCase.value,
					},
				}

				host, status := test.NewTestHost(mustQuotaRoutingConfig(t, config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
	})
}

func TestQuotaRawRoutingGateFailsClosed(t *testing.T) {
	require.False(t, isAIPathEnabled("/vendor/jobs/job-42", []string{""}))
	require.False(t, isAIPathEnabled("/vendor/jobs/job-42", []string{"   ", "/allowed"}))
	require.True(t, isAIPathEnabled("/v2/allowed?next=/ignored", []string{"/allowed"}))
	require.False(t, isAIPathEnabled("/v2/denied?next=/allowed", []string{"/allowed"}))
}

func TestQuotaRoutingFiltersHaveNoUndocumentedCaps(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("sixty-five legacy suffixes are accepted and gated", func(t *testing.T) {
			suffixes := make([]string, 65)
			for index := range suffixes {
				suffixes[index] = "/allowed-" + strings.Repeat("x", index+1)
			}
			config := quotaRoutingContractConfig()
			config["enable_path_suffixes"] = suffixes

			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/v2"+suffixes[64],
				100, true, false, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
		})

		t.Run("suffix longer than 256 bytes is accepted and gated", func(t *testing.T) {
			suffix := "/" + strings.Repeat("s", 300)
			config := quotaRoutingContractConfig()
			config["enable_path_suffixes"] = []string{suffix}

			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/prefix"+suffix,
				100, true, false, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
		})

		t.Run("valid method longer than 32 bytes is accepted and gated exactly", func(t *testing.T) {
			method := strings.Repeat("M", 40)
			config := quotaRoutingContractConfig()
			config["path_scope"] = "bound_target"
			config["enable_methods"] = []string{method}

			runQuotaRoutingAdmission(
				t, config, "", method, "/vendor/jobs/job-42",
				100, true, false, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/vendor/jobs/job-42",
				100, false, false, "", "",
			)
		})
	})
}

func TestQuotaRuleOverrideIsolationRegression(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		config := quotaRoutingContractConfig()
		config["path_scope"] = "bound_target"
		config["enable_path_suffixes"] = []string{"/global"}
		config["enable_methods"] = []string{http.MethodPost}
		config["tenant_header"] = "x-global-tenant"
		config["consumer_header"] = "x-global-consumer"
		config["balance_key_template"] = "global:{tenant}:{quota_scope}:{consumer}"
		config["missing_balance_policy"] = "deny"
		config["redis"] = map[string]interface{}{
			"service_name": "redis.global",
			"service_port": 6382,
			"timeout":      901,
			"database":     5,
		}
		config["_rules_"] = []map[string]interface{}{
			{
				"_match_route_":          []string{"route-isolated"},
				"quota_scope":            "route-scope",
				"provider":               "anthropic",
				"enable_path_suffixes":   []string{"/rule"},
				"tenant_header":          "x-rule-tenant",
				"consumer_header":        "x-rule-consumer",
				"balance_key_template":   "rule:{tenant}:{quota_scope}:{consumer}",
				"missing_balance_policy": "allow",
				"redis": map[string]interface{}{
					"service_name": "redis.rule",
					"service_port": 6399,
					"timeout":      222,
					"database":     9,
				},
			},
		}

		host, status := test.NewTestHost(mustQuotaRoutingConfig(t, config))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		require.NoError(t, host.SetRouteName("route-isolated"))

		matchConfig, err := host.GetMatchConfig()
		require.NoError(t, err)
		ruleConfig := matchConfig.(*QuotaConfig)
		require.Equal(t, "route-scope", ruleConfig.QuotaScope)
		require.Equal(t, "anthropic", ruleConfig.Provider)
		require.Equal(t, "x-global-tenant", ruleConfig.TenantHeader)
		require.Equal(t, "x-global-consumer", ruleConfig.ConsumerHeader)
		require.Equal(t, "global:{tenant}:{quota_scope}:{consumer}", ruleConfig.BalanceKeyTemplate)
		require.Equal(t, MissingPolicyDeny, ruleConfig.MissingBalancePolicy)
		require.Equal(t, "redis.global", ruleConfig.redisInfo.ServiceName)
		require.Equal(t, 6382, ruleConfig.redisInfo.ServicePort)
		require.Equal(t, 901, ruleConfig.redisInfo.Timeout)
		require.Equal(t, 5, ruleConfig.redisInfo.Database)
		require.Equal(t, []string{"/rule"}, ruleConfig.EnablePathSuffixes)

		ruleConfig.EnablePathSuffixes[0] = "/mutated"
		require.NoError(t, host.SetRouteName("route-global"))
		globalMatch, err := host.GetMatchConfig()
		require.NoError(t, err)
		globalConfig := globalMatch.(*QuotaConfig)
		require.Equal(t, []string{"/global"}, globalConfig.EnablePathSuffixes)
	})
}

func TestQuotaRoutingQueryAndScopeSwitchRegression(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("query content does not create a suffix match", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["enable_path_suffixes"] = []string{"/v1/chat/completions"}

			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost,
				"/unrelated?next=/v1/chat/completions",
				100, false, false, "", "",
			)
			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost,
				"/v1/chat/completions?next=/unrelated",
				100, true, false, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
		})

		t.Run("rule switching back to legacy restores legacy defaults", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["path_scope"] = "bound_target"
			config["_rules_"] = []map[string]interface{}{
				{
					"_match_route_": []string{"route-legacy"},
					"path_scope":    "legacy_suffixes",
				},
			}

			runQuotaRoutingAdmission(
				t, config, "route-legacy", http.MethodPost,
				"/v1/chat/completions",
				100, true, false, "billing:balance:tenant-a:global:consumer-a", "redis.static",
			)
			runQuotaRoutingAdmission(
				t, config, "route-legacy", http.MethodPost,
				"/vendor/jobs/job-42",
				100, false, false, "", "",
			)
		})
	})
}
