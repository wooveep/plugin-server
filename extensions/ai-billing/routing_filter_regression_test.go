package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestBillingBlankOnlyBoundTargetFiltersAreRejected(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, field := range []string{"enable_path_suffixes", "enable_methods"} {
			t.Run("global "+field, func(t *testing.T) {
				config := billingRoutingContractConfig()
				config["path_scope"] = PathScopeBoundTarget
				config[field] = []string{"   "}

				host, status := test.NewTestHost(mustBillingConfig(t, config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})

			t.Run("rule "+field, func(t *testing.T) {
				config := billingRoutingContractConfig()
				config["_rules_"] = []map[string]interface{}{
					{
						"_match_route_": []string{"route-blank-filter"},
						"path_scope":    PathScopeBoundTarget,
						field:           []string{"   "},
					},
				}

				host, status := test.NewTestHost(mustBillingConfig(t, config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
	})

	require.False(t, isBillingRequestEnabled(
		"/vendor/jobs/job-42",
		http.MethodPost,
		BillingConfig{
			PathScope:          PathScopeBoundTarget,
			EnablePathSuffixes: []string{"   "},
		},
	))
	require.False(t, isBillingRequestEnabled(
		"/vendor/jobs/job-42",
		http.MethodPost,
		BillingConfig{
			PathScope:     PathScopeBoundTarget,
			EnableMethods: []string{"   "},
		},
	))
}

func TestBillingMethodFiltersPreserveExactCase(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("parser preserves exact-case tokens and deduplicates exact values", func(t *testing.T) {
			config := billingRoutingContractConfig()
			config["path_scope"] = PathScopeBoundTarget
			config["enable_methods"] = []string{"MiXeD", "mixed", "MiXeD"}

			host, status := test.NewTestHost(mustBillingConfig(t, config))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			matchConfig, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.Equal(
				t,
				[]string{"MiXeD", "mixed"},
				matchConfig.(*BillingConfig).EnableMethods,
			)
		})

		t.Run("runtime admits only the exact configured token", func(t *testing.T) {
			config := billingRoutingContractConfig()
			config["path_scope"] = PathScopeBoundTarget
			config["enable_methods"] = []string{"MiXeD"}

			runBillingRoutingRequest(t, config, "", "MiXeD", "/vendor/jobs/job-42", true)
			runBillingRoutingRequest(t, config, "", "mixed", "/vendor/jobs/job-42", false)
			runBillingRoutingRequest(t, config, "", "MIXED", "/vendor/jobs/job-42", false)
		})
	})
}

func TestBillingRoutingFiltersRetainUnboundedLegacyCompatibility(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("legacy accepts and gates sixty-five suffixes", func(t *testing.T) {
			suffixes := make([]string, 65)
			for index := range suffixes {
				suffixes[index] = fmt.Sprintf("/legacy-operation-%02d", index)
			}
			config := billingRoutingContractConfig()
			config["path_scope"] = PathScopeLegacySuffixes
			config["enable_path_suffixes"] = suffixes

			runBillingRoutingRequest(
				t,
				config,
				"",
				http.MethodPost,
				"/vendor/legacy-operation-64",
				true,
			)
			runBillingRoutingRequest(
				t,
				config,
				"",
				http.MethodPost,
				"/vendor/not-configured",
				false,
			)
		})

		t.Run("legacy accepts and gates a suffix longer than 256 bytes", func(t *testing.T) {
			suffix := "/" + strings.Repeat("a", 257)
			config := billingRoutingContractConfig()
			config["path_scope"] = PathScopeLegacySuffixes
			config["enable_path_suffixes"] = []string{suffix}

			runBillingRoutingRequest(
				t,
				config,
				"",
				http.MethodPost,
				"/vendor"+suffix,
				true,
			)
		})

		t.Run("bound target accepts and exactly gates a method longer than 32 bytes", func(t *testing.T) {
			method := strings.Repeat("M", 33)
			config := billingRoutingContractConfig()
			config["path_scope"] = PathScopeBoundTarget
			config["enable_methods"] = []string{method}

			runBillingRoutingRequest(t, config, "", method, "/vendor/jobs/job-42", true)
			runBillingRoutingRequest(
				t,
				config,
				"",
				strings.ToLower(method),
				"/vendor/jobs/job-42",
				false,
			)
		})
	})
}
