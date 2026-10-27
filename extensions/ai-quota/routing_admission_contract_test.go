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

		for _, field := range []string{"redis", "balance_scope_id", "cache_generation", "projection_schema", "missing_balance_policy"} {
			t.Run("rule cannot replace "+field, func(t *testing.T) {
				config := quotaRoutingContractConfig()
				config["_rules_"] = []map[string]any{{"_match_route_": []string{"route-native"}, field: config[field]}}
				host, status := test.NewTestHost(mustQuotaRoutingConfig(t, config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}

	})
}

func TestQuotaRoutingAdmissionContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("omitted path scope retains legacy default eligibility", func(t *testing.T) {
			config := quotaRoutingContractConfig()

			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/v1/chat/completions",
				100, true, false, testAccountKey(), "redis.static",
			)
			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/vendor/native-generate",
				100, false, false, "", "",
			)
		})

		t.Run("explicit legacy scope retains explicit suffix eligibility", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["path_scope"] = "legacy_suffixes"
			config["enable_path_suffixes"] = []string{"/vendor/allowed"}

			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/v2/vendor/allowed",
				100, true, false, testAccountKey(), "redis.static",
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
				t, config, "", http.MethodPatch, "/vendor/native-generate",
				100, true, false, testAccountKey(), "redis.static",
			)
			runQuotaRoutingAdmission(
				t, config, "", http.MethodPatch, "/vendor/native-generate",
				0, true, true, testAccountKey(), "redis.static",
			)
			runQuotaRoutingAdmission(
				t, config, "", http.MethodPatch, "/vendor/native-generate",
				-1, true, true, testAccountKey(), "redis.static",
			)
		})

		t.Run("global literal-empty bound-target filters do not narrow", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["path_scope"] = "bound_target"
			config["enable_path_suffixes"] = []string{}
			config["enable_methods"] = []string{}

			runQuotaRoutingAdmission(
				t, config, "", http.MethodDelete, "/native/files/file-42",
				100, true, false, testAccountKey(), "redis.static",
			)
		})

		t.Run("rule bound target inherits global redis without legacy suffix gating", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["enable_path_suffixes"] = []string{"/v1/chat/completions"}
			config["redis"] = map[string]interface{}{
				"service_name": "redis.global",
				"service_port": 6382,
			}
			config["_rules_"] = []map[string]interface{}{
				{
					"_match_route_": []string{"route-native-omitted-filters"},
					"path_scope":    "bound_target",
				},
			}

			runQuotaRoutingAdmission(
				t, config, "route-native-omitted-filters", http.MethodPut,
				"/native/messages/batches/batch-42", -1, true, true,
				testAccountKey(), "redis.global",
			)
		})

		t.Run("rule literal-empty bound-target filters do not narrow", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["enable_path_suffixes"] = []string{"/v1/chat/completions"}
			config["_rules_"] = []map[string]interface{}{
				{
					"_match_route_":        []string{"route-native-empty-filters"},
					"path_scope":           "bound_target",
					"enable_path_suffixes": []string{},
					"enable_methods":       []string{},
				},
			}

			runQuotaRoutingAdmission(
				t, config, "route-native-empty-filters", http.MethodDelete,
				"/native/files/file-42", 100, true, false,
				testAccountKey(), "redis.static",
			)
		})

		t.Run("nonempty suffix and method filters intersect bound target", func(t *testing.T) {
			config := quotaRoutingContractConfig()
			config["path_scope"] = "bound_target"
			config["enable_path_suffixes"] = []string{"/vendor/jobs"}
			config["enable_methods"] = []string{http.MethodPost}

			runQuotaRoutingAdmission(
				t, config, "", http.MethodPost, "/v2/vendor/jobs",
				100, true, false, testAccountKey(), "redis.static",
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
				100, true, false, testAccountKey(), "redis.static",
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

func TestBoundTargetReadMethodsRequireAdmission(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method, func(t *testing.T) {
				config := quotaRoutingContractConfig()
				config["path_scope"] = "bound_target"
				config["enable_methods"] = []string{method}
				runQuotaRoutingAdmission(t, config, "", method, "/native/generate", 100, true, false, testAccountKey(), "redis.static")
				runQuotaRoutingAdmission(t, config, "", method, "/native/generate", 0, true, true, testAccountKey(), "redis.static")

				host, status := test.NewTestHost(mustQuotaRoutingConfig(t, config))
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders([][2]string{{":authority", "example.test"}, {":path", "/native/generate"}, {":method", method}})
				response := host.GetLocalResponse()
				require.NotNil(t, response)
				require.Equal(t, uint32(http.StatusForbidden), response.StatusCode)
				require.Equal(t, "ai-quota.missing_identity", response.StatusCodeDetail)
				require.Empty(t, host.GetRedisCalloutAttributes())
				host.CompleteHttp()
				host.Reset()

				config["enable_methods"] = []string{http.MethodPost}
				runQuotaRoutingAdmission(t, config, "", method, "/native/tasks/task-42", 0, false, false, "", "")
			})
		}
	})
}

func quotaRoutingContractConfig() map[string]interface{} {
	var config map[string]interface{}
	_ = json.Unmarshal(monetaryConfig, &config)
	delete(config, "enable_path_suffixes")
	return config
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

	action := host.CallOnHttpRequestHeaders(nativeAdmissionHeaders(path, method))
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

	reason := "ok"
	if balance <= 0 {
		reason = "insufficient_balance"
	}
	host.CallOnRedisCall(0, admissionReply(reason))
	if wantDenied {
		response := host.GetLocalResponse()
		require.NotNil(t, response)
		require.Equal(t, uint32(http.StatusForbidden), response.StatusCode)
		require.Equal(t, "ai-quota.no_balance", response.StatusCodeDetail)
		require.Contains(t, string(response.Data), "No monetary balance left")
		require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.GetHttpStreamAction())
		require.NotEqual(t, types.ActionContinue, host.GetHttpStreamAction())
	} else {
		require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
		require.Nil(t, host.GetLocalResponse())
	}
	host.CompleteHttp()
}
