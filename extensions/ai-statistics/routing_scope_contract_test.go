package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

func TestStatisticsRoutingScopeContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		mustConfig := func(t *testing.T, config map[string]interface{}) json.RawMessage {
			t.Helper()
			data, err := json.Marshal(config)
			require.NoError(t, err)
			return data
		}
		fixedMarkerConfig := func(marker string) map[string]interface{} {
			return map[string]interface{}{
				"attributes": []map[string]interface{}{
					{
						"key":          "routing_scope_marker",
						"value_source": "fixed_value",
						"value":        marker,
						"apply_to_log": true,
					},
				},
				"disable_openai_usage": true,
			}
		}
		defaultAttributesConfig := func() map[string]interface{} {
			return map[string]interface{}{
				"use_default_attributes": true,
				"disable_openai_usage":   true,
			}
		}
		requireObserved := func(
			t *testing.T,
			config map[string]interface{},
			routeName, method, path, marker string,
			wantObserved bool,
		) {
			t.Helper()
			host, status := test.NewTestHost(mustConfig(t, config))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			if routeName != "" {
				require.NoError(t, host.SetRouteName(routeName))
			}

			headerAction := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "statistics-client.example"},
				{":method", method},
				{":path", path},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, headerAction)
			bodyAction := host.CallOnHttpRequestBody([]byte(`{
				"model":"contract-model",
				"messages":[{"role":"user","content":"routing contract"}]
			}`))
			require.Equal(t, types.ActionContinue, bodyAction)

			raw, err := host.GetProperty([]string{wrapper.AILogKey})
			if !wantObserved {
				require.Error(t, err, "out-of-scope request must not reach the statistics body lifecycle")
				return
			}
			require.NoError(t, err, "eligible request must reach the statistics body lifecycle")
			if marker != "" {
				require.Contains(t, string(raw), marker, "route-effective statistics config must retain inherited attributes")
			}
		}

		t.Run("omitted path scope retains legacy default suffixes", func(t *testing.T) {
			requireObserved(
				t, defaultAttributesConfig(), "", http.MethodPost,
				"/v1/chat/completions", "", true,
			)
			requireObserved(
				t, defaultAttributesConfig(), "", http.MethodPost,
				"/vendor/jobs/job-42", "", false,
			)
		})

		t.Run("explicit legacy scope retains explicit suffixes", func(t *testing.T) {
			config := fixedMarkerConfig("explicit-legacy-marker")
			config["path_scope"] = "legacy_suffixes"
			config["enable_path_suffixes"] = []string{"/vendor/allowed"}

			requireObserved(
				t, config, "", http.MethodPost,
				"/v2/vendor/allowed", "explicit-legacy-marker", true,
			)
			requireObserved(
				t, config, "", http.MethodPost,
				"/v1/chat/completions", "", false,
			)
		})

		t.Run("bound target without filters observes provider native path without synthesized suffixes", func(t *testing.T) {
			config := defaultAttributesConfig()
			config["path_scope"] = "bound_target"

			requireObserved(
				t, config, "", http.MethodPatch,
				"/vendor/jobs/job-42:cancel", "", true,
			)
		})

		t.Run("bound target literal empty filters do not narrow provider native path", func(t *testing.T) {
			config := defaultAttributesConfig()
			config["path_scope"] = "bound_target"
			config["enable_path_suffixes"] = []string{}
			config["enable_methods"] = []string{}

			requireObserved(
				t, config, "", http.MethodDelete,
				"/native/files/file-42", "", true,
			)
		})

		for _, testCase := range []struct {
			name         string
			method       string
			path         string
			wantObserved bool
		}{
			{
				name:         "both filters match",
				method:       "MiXeD",
				path:         "/v2/vendor/jobs",
				wantObserved: true,
			},
			{
				name:         "method case differs in lowercase",
				method:       "mixed",
				path:         "/v2/vendor/jobs",
				wantObserved: false,
			},
			{
				name:         "method case differs in uppercase",
				method:       "MIXED",
				path:         "/v2/vendor/jobs",
				wantObserved: false,
			},
			{
				name:         "suffix does not match",
				method:       "MiXeD",
				path:         "/v2/vendor/models",
				wantObserved: false,
			},
			{
				name:         "neither filter matches",
				method:       http.MethodGet,
				path:         "/v2/vendor/models",
				wantObserved: false,
			},
		} {
			t.Run("nonempty suffix and exact-case method intersect/"+testCase.name, func(t *testing.T) {
				config := fixedMarkerConfig("intersection-marker")
				config["path_scope"] = "bound_target"
				config["enable_path_suffixes"] = []string{"/vendor/jobs"}
				config["enable_methods"] = []string{"MiXeD"}

				requireObserved(
					t, config, "", testCase.method, testCase.path,
					"intersection-marker", testCase.wantObserved,
				)
			})
		}

		t.Run("rule omitted scope inherits global legacy suffix and attributes", func(t *testing.T) {
			config := fixedMarkerConfig("rule-inherited-legacy-marker")
			config["enable_path_suffixes"] = []string{"/global/allowed"}
			config["_rules_"] = []map[string]interface{}{
				{
					"_match_route_": []string{"statistics-route-inherit"},
				},
			}

			requireObserved(
				t, config, "statistics-route-inherit", http.MethodPost,
				"/v2/global/allowed", "rule-inherited-legacy-marker", true,
			)
		})

		t.Run("rule explicit legacy suffix overrides inherited suffix and retains attributes", func(t *testing.T) {
			config := fixedMarkerConfig("rule-explicit-legacy-marker")
			config["enable_path_suffixes"] = []string{"/global/allowed"}
			config["_rules_"] = []map[string]interface{}{
				{
					"_match_route_":        []string{"statistics-route-legacy"},
					"path_scope":           "legacy_suffixes",
					"enable_path_suffixes": []string{"/route/allowed"},
				},
			}

			requireObserved(
				t, config, "statistics-route-legacy", http.MethodPost,
				"/v2/route/allowed", "rule-explicit-legacy-marker", true,
			)
		})

		t.Run("rule bound target clears inherited legacy suffix without losing attributes", func(t *testing.T) {
			config := fixedMarkerConfig("rule-bound-target-marker")
			config["enable_path_suffixes"] = []string{"/v1/chat/completions"}
			config["_rules_"] = []map[string]interface{}{
				{
					"_match_route_": []string{"statistics-route-native"},
					"path_scope":    "bound_target",
				},
			}

			requireObserved(
				t, config, "statistics-route-native", http.MethodPut,
				"/native/messages/batches/batch-42", "rule-bound-target-marker", true,
			)
		})

		t.Run("rule bound target literal empty filters retain inherited attributes", func(t *testing.T) {
			config := fixedMarkerConfig("rule-empty-filter-marker")
			config["enable_path_suffixes"] = []string{"/v1/chat/completions"}
			config["_rules_"] = []map[string]interface{}{
				{
					"_match_route_":        []string{"statistics-route-empty"},
					"path_scope":           "bound_target",
					"enable_path_suffixes": []string{},
					"enable_methods":       []string{},
				},
			}

			requireObserved(
				t, config, "statistics-route-empty", http.MethodDelete,
				"/native/files/file-42", "rule-empty-filter-marker", true,
			)
		})

		for _, testCase := range []struct {
			name         string
			method       string
			path         string
			wantObserved bool
		}{
			{
				name:         "both rule filters match",
				method:       "RuLe",
				path:         "/v3/native/rule/jobs",
				wantObserved: true,
			},
			{
				name:         "rule method case differs",
				method:       "rule",
				path:         "/v3/native/rule/jobs",
				wantObserved: false,
			},
			{
				name:         "rule suffix does not match",
				method:       "RuLe",
				path:         "/v3/native/rule/models",
				wantObserved: false,
			},
		} {
			t.Run("rule nonempty suffix and exact-case method intersect/"+testCase.name, func(t *testing.T) {
				config := fixedMarkerConfig("rule-intersection-marker")
				config["enable_path_suffixes"] = []string{"/v1/chat/completions"}
				config["_rules_"] = []map[string]interface{}{
					{
						"_match_route_":        []string{"statistics-route-intersection"},
						"path_scope":           "bound_target",
						"enable_path_suffixes": []string{"/native/rule/jobs"},
						"enable_methods":       []string{"RuLe"},
					},
				}

				requireObserved(
					t, config, "statistics-route-intersection", testCase.method, testCase.path,
					"rule-intersection-marker", testCase.wantObserved,
				)
			})
		}

		for _, testCase := range []struct {
			name   string
			mutate func(map[string]interface{})
		}{
			{
				name: "global unknown path scope",
				mutate: func(config map[string]interface{}) {
					config["path_scope"] = "all_paths"
				},
			},
			{
				name: "rule unknown path scope",
				mutate: func(config map[string]interface{}) {
					config["_rules_"] = []map[string]interface{}{
						{
							"_match_route_": []string{"statistics-route-invalid"},
							"path_scope":    "all_paths",
						},
					}
				},
			},
			{
				name: "global blank-only suffix filter",
				mutate: func(config map[string]interface{}) {
					config["path_scope"] = "bound_target"
					config["enable_path_suffixes"] = []string{"   "}
				},
			},
			{
				name: "global blank-only method filter",
				mutate: func(config map[string]interface{}) {
					config["path_scope"] = "bound_target"
					config["enable_methods"] = []string{"   "}
				},
			},
			{
				name: "rule blank-only suffix filter",
				mutate: func(config map[string]interface{}) {
					config["_rules_"] = []map[string]interface{}{
						{
							"_match_route_":        []string{"statistics-route-blank-suffix"},
							"path_scope":           "bound_target",
							"enable_path_suffixes": []string{"   "},
						},
					}
				},
			},
			{
				name: "rule blank-only method filter",
				mutate: func(config map[string]interface{}) {
					config["_rules_"] = []map[string]interface{}{
						{
							"_match_route_":  []string{"statistics-route-blank-method"},
							"path_scope":     "bound_target",
							"enable_methods": []string{"   "},
						},
					}
				},
			},
		} {
			t.Run("invalid routing configuration fails closed/"+testCase.name, func(t *testing.T) {
				config := fixedMarkerConfig("invalid-config-marker")
				testCase.mutate(config)

				host, status := test.NewTestHost(mustConfig(t, config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
	})
}
