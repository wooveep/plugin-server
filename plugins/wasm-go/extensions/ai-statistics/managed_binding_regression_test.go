package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func statisticsManagedFacts(t *testing.T, host test.TestHost) map[string]json.RawMessage {
	t.Helper()
	raw, err := host.GetProperty([]string{wrapper.AILogKey})
	require.NoError(t, err)
	decoded, err := strconv.Unquote("\"" + string(raw) + "\"")
	require.NoError(t, err)
	var facts map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(decoded), &facts))
	return facts
}

func TestStatisticsManagedBindingConfiguration(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, field := range []string{"enable_path_suffixes", "enable_methods"} {
			for _, values := range []string{`["/v1/chat/completions"]`, `["POST"]`, `["*"]`} {
				t.Run(field+"/"+values, func(t *testing.T) {
					host, status := test.NewTestHost(json.RawMessage(fmt.Sprintf(`{"path_scope":"bound_target",%q:%s}`, field, values)))
					defer host.Reset()
					require.Equal(t, types.OnPluginStartStatusFailed, status)
				})
			}
		}
		for _, match := range []string{"route", "service"} {
			t.Run("rule only complete configuration/"+match, func(t *testing.T) {
				rule := map[string]interface{}{
					"path_scope":           "bound_target",
					"disable_openai_usage": true,
					"enable_content_types": []string{"application/json"},
					"attributes":           []map[string]interface{}{{"key": "configured_rule_marker", "value_source": "fixed_value", "value": "from-complete-rule", "apply_to_log": true}},
				}
				if match == "route" {
					rule["_match_route_"] = []string{"managed-statistics-route"}
				} else {
					rule["_match_service_"] = []string{"managed-statistics.dns"}
				}
				config, err := json.Marshal(map[string]interface{}{"_rules_": []interface{}{rule}})
				require.NoError(t, err)
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("managed-statistics-route"))
				require.NoError(t, host.SetClusterName("outbound|443||managed-statistics.dns"))
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{{":method", "GET"}, {":path", "/tenant/custom/generate"}, {":authority", "statistics.example"}}, test.WithEndOfStream(true)))
				require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders([][2]string{{":status", "403"}, {"content-type", "text/plain"}}, test.WithEndOfStream(true)))
				host.CompleteHttp()
				facts := statisticsManagedFacts(t, host)
				require.JSONEq(t, `"from-complete-rule"`, string(facts["configured_rule_marker"]))
				require.JSONEq(t, "403", string(facts["status_code"]))
				count, err := host.GetCounterMetric("route.managed-statistics-route.path.provider_native_unknown.method.GET.status.403.error.http_4xx.metric.request_count")
				require.NoError(t, err)
				require.Equal(t, uint64(1), count)
			})
		}
		t.Run("bad global cannot become a valid isolated rule", func(t *testing.T) {
			host, status := test.NewTestHost(json.RawMessage(`{"path_scope":"invalid","_rules_":[{"_match_route_":["managed-statistics-route"],"path_scope":"bound_target"}]}`))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})
		t.Run("initialized global permits routing overrides only", func(t *testing.T) {
			for _, field := range []string{"use_default_attributes", "use_default_response_attributes", "attributes", "disable_openai_usage", "value_length_limit", "enable_content_types", "session_id_header"} {
				t.Run(field, func(t *testing.T) {
					rule := map[string]interface{}{"_match_route_": []string{"managed-statistics-route"}, "path_scope": "bound_target", field: nil}
					config, err := json.Marshal(map[string]interface{}{"disable_openai_usage": true, "_rules_": []interface{}{rule}})
					require.NoError(t, err)
					host, status := test.NewTestHost(config)
					defer host.Reset()
					require.Equal(t, types.OnPluginStartStatusFailed, status)
				})
			}
		})
		t.Run("legacy without default attributes remains unrestricted", func(t *testing.T) {
			host, status := test.NewTestHost(json.RawMessage(`{}`))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("managed-statistics-route"))
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{{":method", "HEAD"}, {":path", "/not-a-default-suffix"}, {":authority", "statistics.example"}}, test.WithEndOfStream(true)))
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders([][2]string{{":status", "204"}}, test.WithEndOfStream(true)))
			host.CompleteHttp()
			require.JSONEq(t, "204", string(statisticsManagedFacts(t, host)["status_code"]))
		})
	})
}

func TestStatisticsManagedUsagePreservesMissingFields(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		cases := []struct {
			name, body string
			want       map[string]int64
		}{
			{"empty usage", `{"model":"provider-model","usage":{}}`, nil},
			{"only input", `{"model":"provider-model","usage":{"prompt_tokens":5}}`, map[string]int64{"input_token": 5}},
			{"only total", `{"model":"provider-model","usage":{"total_tokens":7}}`, map[string]int64{"total_token": 7}},
			{"complete chat", `{"model":"provider-model","usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`, map[string]int64{"input_token": 2, "output_token": 3, "total_token": 5}},
			{"actual zero", `{"model":"provider-model","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`, map[string]int64{"input_token": 0, "output_token": 0, "total_token": 0}},
			{"native responses", `{"model":"provider-model","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}`, map[string]int64{"input_token": 4, "output_token": 2, "total_token": 6}},
			{"native claude no total", `{"model":"provider-model","usage":{"input_tokens":4,"output_tokens":2}}`, map[string]int64{"input_token": 4, "output_token": 2}},
			{"invalid optional counters", `{"model":"provider-model","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3,"prompt_tokens_details":{"cached_tokens":null},"completion_tokens_details":{"reasoning_tokens":"bad"}}}`, map[string]int64{"input_token": 1, "output_token": 2, "total_token": 3}},
			{"actual optional zero", `{"model":"provider-model","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`, map[string]int64{"input_token": 0, "output_token": 0, "total_token": 0}},
			{"null and invalid counters", `{"model":"provider-model","usage":{"prompt_tokens":null,"completion_tokens":"bad","total_tokens":-1}}`, nil},
			{"unknown json", `{"opaque":"not-usage"}`, nil},
			{"invalid json", `{"usage":`, nil},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				host, status := test.NewTestHost(json.RawMessage(`{"path_scope":"bound_target","use_default_response_attributes":true}`))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("managed-statistics-route"))
				require.NoError(t, host.SetClusterName("controlled-cluster"))
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{{":method", "POST"}, {":path", "/alias/independent-of-protocol"}, {":authority", "statistics.example"}, {"content-type", "application/json"}}))
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody([]byte(`{"model":"public-model","messages":[]}`)))
				headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
				require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders(headers))
				require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte(tc.body)))
				host.CompleteHttp()
				facts := statisticsManagedFacts(t, host)
				if tc.name == "invalid optional counters" || tc.name == "actual optional zero" {
					for _, key := range []string{"cached_tokens", "reasoning_tokens"} {
						if tc.name == "actual optional zero" {
							require.JSONEq(t, "0", string(facts[key]))
						} else {
							require.True(t, len(facts[key]) == 0 || string(facts[key]) == "null", "invalid optional counter %s must be unknown", key)
						}
					}
				}
				for _, key := range []string{"input_token", "output_token", "total_token"} {
					expected, known := tc.want[key]
					if known {
						require.JSONEq(t, strconv.FormatInt(expected, 10), string(facts[key]), key)
					} else {
						require.True(t, len(facts[key]) == 0 || string(facts[key]) == "null", "missing %s must not be invented as %s", key, facts[key])
					}
					metric := "route.managed-statistics-route.upstream.controlled-cluster.model.provider-model.consumer.none.metric." + key
					count, err := host.GetCounterMetric(metric)
					if known && expected > 0 {
						require.NoError(t, err, key)
						require.Equal(t, uint64(expected), count, key)
					} else {
						require.Error(t, err, "missing/zero metric must not add a counter: %s", key)
					}
				}
				count, err := host.GetCounterMetric("route.managed-statistics-route.path.provider_native_unknown.method.POST.status.200.error.none.metric.request_count")
				require.NoError(t, err)
				require.Equal(t, uint64(1), count)
				require.Equal(t, headers, host.GetResponseHeaders())
				require.Equal(t, []byte(tc.body), host.GetResponseBody())
			})
		}
	})
}

func TestStatisticsManagedStreamingAndShortCircuitBasics(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			status int
			chunks []string
			eos    bool
		}{
			{"rejected before body", 403, nil, false},
			{"empty response", 204, nil, false},
			{"stream interrupted", 200, []string{"data: {\"opaque\":true}\n\n"}, false},
			{"claude fragmented frames", 200, []string{"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"provider-model\",\"usage\":{\"input_", "tokens\":4}}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":", "2}}\n\n"}, true},
			{"claude split usage", 200, []string{"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"provider-model\",\"usage\":{\"input_tokens\":4}}}\n\n", "event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":2}}\n\n"}, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				host, status := test.NewTestHost(json.RawMessage(`{"path_scope":"bound_target","use_default_response_attributes":true}`))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("managed-statistics-route"))
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{{":method", "PATCH"}, {":path", "/arbitrary/ai-route"}, {":authority", "statistics.example"}}, test.WithEndOfStream(true)))
				contentType := "text/event-stream"
				if len(tc.chunks) == 0 {
					contentType = "text/plain"
				}
				require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders([][2]string{{":status", strconv.Itoa(tc.status)}, {"content-type", contentType}}))
				for i, chunk := range tc.chunks {
					require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingResponseBody([]byte(chunk), tc.eos && i == len(tc.chunks)-1))
				}
				host.CompleteHttp()
				facts := statisticsManagedFacts(t, host)
				require.JSONEq(t, strconv.Itoa(tc.status), string(facts["status_code"]))
				errorClass := "none"
				if tc.status == 403 {
					errorClass = "http_4xx"
				}
				prefix := fmt.Sprintf("route.managed-statistics-route.path.provider_native_unknown.method.PATCH.status.%d.error.%s.metric.", tc.status, errorClass)
				count, err := host.GetCounterMetric(prefix + "request_count")
				require.NoError(t, err)
				require.Equal(t, uint64(1), count)
				duration, err := host.GetCounterMetric(prefix + "request_duration_ms")
				require.NoError(t, err)
				require.Greater(t, duration, uint64(0))
				if tc.name == "claude split usage" || tc.name == "claude fragmented frames" {
					require.JSONEq(t, "4", string(facts["input_token"]))
					require.JSONEq(t, "2", string(facts["output_token"]))
					require.True(t, len(facts["total_token"]) == 0 || string(facts["total_token"]) == "null")
				} else {
					for _, key := range []string{"input_token", "output_token", "total_token"} {
						require.True(t, len(facts[key]) == 0 || string(facts[key]) == "null", key)
					}
				}
			})
		}
	})
}

func TestStatisticsManagedSchemaDeclaresFullScopedConfiguration(t *testing.T) {
	raw, err := os.ReadFile("../../resources/plugins/ai-statistics/spec.yaml")
	require.NoError(t, err)
	var document map[string]interface{}
	require.NoError(t, yaml.Unmarshal(raw, &document))
	spec := document["spec"].(map[string]interface{})
	for _, scope := range []string{"configSchema", "routeConfigSchema"} {
		schema := spec[scope].(map[string]interface{})["openAPIV3Schema"].(map[string]interface{})
		properties := schema["properties"].(map[string]interface{})
		for _, key := range []string{"attributes", "use_default_attributes", "use_default_response_attributes", "disable_openai_usage", "enable_content_types"} {
			require.Contains(t, properties, key)
		}
		conditions := schema["allOf"].([]interface{})
		constraint := conditions[0].(map[string]interface{})["then"].(map[string]interface{})["properties"].(map[string]interface{})
		for _, key := range []string{"enable_path_suffixes", "enable_methods"} {
			require.Equal(t, 0, constraint[key].(map[string]interface{})["maxItems"])
		}
		if scope == "routeConfigSchema" {
			require.NotContains(t, properties["path_scope"].(map[string]interface{}), "default", "omitted route scope must still inherit global scope")
		}
	}
}
