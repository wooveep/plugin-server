package main

import (
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestBillingRoutingAndDeliveryConfigContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("global redis stream applies exact defaults", func(t *testing.T) {
			config := billingRoutingContractConfig()
			config["redis_stream"] = map[string]interface{}{
				"service_name": "redis.static",
			}

			host, status := test.NewTestHost(mustBillingConfig(t, config))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			matchConfig, err := host.GetMatchConfig()
			require.NoError(t, err)
			billingConfig := matchConfig.(*BillingConfig)
			require.Equal(t, "redis.static", billingConfig.RedisStream.ServiceName)
			require.Equal(t, defaultRedisPort, billingConfig.RedisStream.ServicePort)
			require.Empty(t, billingConfig.RedisStream.Username)
			require.Empty(t, billingConfig.RedisStream.Password)
			require.Zero(t, billingConfig.RedisStream.Database)
			require.Equal(t, defaultRedisTimeout, billingConfig.RedisStream.Timeout)
			require.Equal(t, defaultRedisStream, billingConfig.RedisStream.Stream)
		})

		for _, testCase := range []struct {
			name   string
			mutate func(map[string]interface{})
		}{
			{
				name: "global redis stream is required",
				mutate: func(config map[string]interface{}) {
					delete(config, "redis_stream")
				},
			},
			{
				name: "global redis service name is required",
				mutate: func(config map[string]interface{}) {
					config["redis_stream"] = map[string]interface{}{"service_name": ""}
				},
			},
			{
				name: "removed global billing service is rejected",
				mutate: func(config map[string]interface{}) {
					config["billing_service"] = map[string]interface{}{
						"service_name": "removed-http-service",
					}
				},
			},
			{
				name: "removed rule billing service is rejected",
				mutate: func(config map[string]interface{}) {
					config["_rules_"] = []map[string]interface{}{
						{
							"_match_route_": []string{"route-contract"},
							"billing_service": map[string]interface{}{
								"service_name": "removed-rule-service",
							},
						},
					}
				},
			},
			{
				name: "rule redis stream replacement is rejected",
				mutate: func(config map[string]interface{}) {
					config["_rules_"] = []map[string]interface{}{
						{
							"_match_route_": []string{"route-contract"},
							"redis_stream": map[string]interface{}{
								"service_name": "redis.route-controlled",
								"password":     "route-secret",
								"database":     9,
								"stream":       "route:events",
							},
						},
					}
				},
			},
			{
				name: "explicitly empty legacy suffix list remains invalid",
				mutate: func(config map[string]interface{}) {
					config["path_scope"] = "legacy_suffixes"
					config["enable_path_suffixes"] = []string{}
				},
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				config := billingRoutingContractConfig()
				testCase.mutate(config)

				host, status := test.NewTestHost(mustBillingConfig(t, config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}

		t.Run("rule inherits the complete global redis target", func(t *testing.T) {
			config := billingRoutingContractConfig()
			config["provider"] = "openai"
			config["quota_scope"] = "global-scope"
			config["redis_stream"] = map[string]interface{}{
				"service_name": "redis.global",
				"service_port": 6382,
				"database":     5,
				"timeout":      901,
				"stream":       "global:billing:events",
			}
			config["_rules_"] = []map[string]interface{}{
				{
					"_match_route_": []string{"route-contract"},
					"provider":      "anthropic",
					"quota_scope":   "route-scope",
				},
			}

			host, status := test.NewTestHost(mustBillingConfig(t, config))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-contract"))

			matchConfig, err := host.GetMatchConfig()
			require.NoError(t, err)
			billingConfig := matchConfig.(*BillingConfig)
			require.Equal(t, "redis.global", billingConfig.RedisStream.ServiceName)
			require.Equal(t, 6382, billingConfig.RedisStream.ServicePort)
			require.Empty(t, billingConfig.RedisStream.Username)
			require.Empty(t, billingConfig.RedisStream.Password)
			require.Equal(t, 5, billingConfig.RedisStream.Database)
			require.Equal(t, int64(901), billingConfig.RedisStream.Timeout)
			require.Equal(t, "global:billing:events", billingConfig.RedisStream.Stream)
			require.Equal(t, "anthropic", billingConfig.Provider)
			require.Equal(t, "route-scope", billingConfig.QuotaScope)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":method", http.MethodPost},
				{":path", "/v1/chat/completions"},
				{"x-request-id", "request-inherited-delivery"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseBody([]byte(
				`{"model":"provider-model","usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`,
			))
			require.Equal(t, types.ActionContinue, action)

			attributes := host.GetRedisCalloutAttributes()
			require.Len(t, attributes, 1)
			require.Regexp(t, `^outbound\|6382\|\|redis\.global(\?.*)?$`, attributes[0].Upstream)
			command := redisCommand(t, attributes[0].Query)
			require.Equal(t, []string{"xadd", "global:billing:events", "*", "event"}, command[:4])
			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("bound target accepts explicitly empty advanced filters", func(t *testing.T) {
			config := billingRoutingContractConfig()
			config["path_scope"] = "bound_target"
			config["enable_path_suffixes"] = []string{}
			config["enable_methods"] = []string{}

			host, status := test.NewTestHost(mustBillingConfig(t, config))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			runBillingRoutingRequest(
				t,
				config,
				"",
				http.MethodPatch,
				"/vendor/explicit-empty-filters/job-42",
				true,
			)
		})
	})
}

func TestBillingRequestGatingContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("omitted path scope retains legacy suffix gating", func(t *testing.T) {
			config := billingRoutingContractConfig()

			runBillingRoutingRequest(t, config, "", http.MethodPost, "/v1/chat/completions", true)
			runBillingRoutingRequest(t, config, "", http.MethodPost, "/vendor/jobs/job-42", false)
		})

		t.Run("explicit legacy scope retains explicit suffix gating", func(t *testing.T) {
			config := billingRoutingContractConfig()
			config["path_scope"] = "legacy_suffixes"
			config["enable_path_suffixes"] = []string{"/vendor/allowed"}

			runBillingRoutingRequest(t, config, "", http.MethodPost, "/v2/vendor/allowed", true)
			runBillingRoutingRequest(t, config, "", http.MethodPost, "/vendor/jobs/job-42", false)
		})

		t.Run("global bound target admits provider native path without suffixes", func(t *testing.T) {
			config := billingRoutingContractConfig()
			config["path_scope"] = "bound_target"

			runBillingRoutingRequest(t, config, "", http.MethodPatch, "/vendor/jobs/job-42:cancel", true)
		})

		t.Run("empty bound target filters do not narrow or widen ambiguously", func(t *testing.T) {
			config := billingRoutingContractConfig()
			config["path_scope"] = "bound_target"
			config["enable_methods"] = []string{}

			runBillingRoutingRequest(t, config, "", http.MethodDelete, "/vendor/files/file-42", true)
		})

		t.Run("rule bound target inherits global redis but not legacy suffix gating", func(t *testing.T) {
			omittedFiltersConfig := billingRoutingContractConfig()
			omittedFiltersConfig["enable_path_suffixes"] = []string{"/v1/chat/completions"}
			omittedFiltersConfig["_rules_"] = []map[string]interface{}{
				{
					"_match_route_": []string{"route-native-omitted-filters"},
					"path_scope":    "bound_target",
					"provider":      "anthropic",
				},
			}
			runBillingRoutingRequest(
				t,
				omittedFiltersConfig,
				"route-native-omitted-filters",
				http.MethodPut,
				"/native/messages/batches/batch-42",
				true,
			)

			explicitEmptyFiltersConfig := billingRoutingContractConfig()
			explicitEmptyFiltersConfig["enable_path_suffixes"] = []string{"/v1/chat/completions"}
			explicitEmptyFiltersConfig["_rules_"] = []map[string]interface{}{
				{
					"_match_route_":        []string{"route-native-empty-filters"},
					"path_scope":           "bound_target",
					"provider":             "anthropic",
					"enable_path_suffixes": []string{},
					"enable_methods":       []string{},
				},
			}
			runBillingRoutingRequest(
				t,
				explicitEmptyFiltersConfig,
				"route-native-empty-filters",
				http.MethodDelete,
				"/native/files/file-42",
				true,
			)
		})

		t.Run("nonempty suffix and method filters intersect bound target", func(t *testing.T) {
			config := billingRoutingContractConfig()
			config["path_scope"] = "bound_target"
			config["enable_path_suffixes"] = []string{"/vendor/jobs"}
			config["enable_methods"] = []string{http.MethodPost}

			runBillingRoutingRequest(t, config, "", http.MethodPost, "/v2/vendor/jobs", true)
			runBillingRoutingRequest(t, config, "", http.MethodGet, "/v2/vendor/jobs", false)
			runBillingRoutingRequest(t, config, "", http.MethodPost, "/v2/vendor/models", false)
		})
	})
}

func billingRoutingContractConfig() map[string]interface{} {
	return map[string]interface{}{
		"provider":        "openai",
		"tenant_header":   "x-tenant-id",
		"consumer_header": "x-consumer-id",
		"redis_stream": map[string]interface{}{
			"service_name": "redis.static",
		},
	}
}

func runBillingRoutingRequest(
	t *testing.T,
	config map[string]interface{},
	routeName string,
	method string,
	path string,
	wantDelivered bool,
) {
	t.Helper()
	host, status := test.NewTestHost(mustBillingConfig(t, config))
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)
	if routeName != "" {
		require.NoError(t, host.SetRouteName(routeName))
	}

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":method", method},
		{":path", path},
		{"x-request-id", "request-routing-contract"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	require.Equal(t, types.ActionContinue, action)
	action = host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "application/json"},
	})
	require.Equal(t, types.ActionContinue, action)
	action = host.CallOnHttpResponseBody([]byte(
		`{"model":"provider-native-model","usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`,
	))
	require.Equal(t, types.ActionContinue, action)

	if wantDelivered {
		event := requireRedisBillingEvent(t, host)
		require.Equal(t, path, event["request_path"])
		ackRedisBillingEvent(t, host)
	} else {
		require.Len(t, host.GetRedisCalloutAttributes(), 0)
	}
	host.CompleteHttp()
}
