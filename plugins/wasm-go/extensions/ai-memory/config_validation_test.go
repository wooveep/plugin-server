package main

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestParseConfigValidation(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		validExternalTargets := map[string]interface{}{
			"redis_stream": map[string]interface{}{
				"service_name": "redis.memory.svc.cluster.local",
			},
			"recent_cache": map[string]interface{}{
				"service_name": "redis.recent.svc.cluster.local",
			},
			"console_internal": map[string]interface{}{
				"service_name": "console.internal.svc.cluster.local",
			},
			"_rules_": []map[string]interface{}{
				{
					"_match_route_": []string{"memory-route"},
					"memory_mode":   "digest",
				},
			},
		}

		tests := []struct {
			name   string
			config map[string]interface{}
		}{
			{
				name: "unsupported fail policy",
				config: withGlobalConfig(validExternalTargets, map[string]interface{}{
					"fail_policy": "closed",
				}),
			},
			{
				name: "invalid memory mode",
				config: map[string]interface{}{
					"redis_stream": map[string]interface{}{
						"service_name": "redis.memory.svc.cluster.local",
					},
					"recent_cache": map[string]interface{}{
						"service_name": "redis.recent.svc.cluster.local",
					},
					"console_internal": map[string]interface{}{
						"service_name": "console.internal.svc.cluster.local",
					},
					"_rules_": []map[string]interface{}{
						{
							"_match_route_": []string{"memory-route"},
							"memory_mode":   "vector",
						},
					},
				},
			},
			{
				name: "route redis stream target",
				config: withRouteConfig(validExternalTargets, map[string]interface{}{
					"redis_stream": map[string]interface{}{
						"service_name": "route.redis.memory.svc.cluster.local",
					},
				}),
			},
			{
				name: "route recent cache target",
				config: withRouteConfig(validExternalTargets, map[string]interface{}{
					"recent_cache": map[string]interface{}{
						"service_name": "route.redis.recent.svc.cluster.local",
					},
				}),
			},
			{
				name: "route console internal target",
				config: withRouteConfig(validExternalTargets, map[string]interface{}{
					"console_internal": map[string]interface{}{
						"service_name": "route.console.internal.svc.cluster.local",
					},
				}),
			},
			{
				name: "symbolic policy version",
				config: withRouteConfig(validExternalTargets, map[string]interface{}{
					"policy_version": "memory-policy-v1",
				}),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				host, status := newMemoryConfigTestHost(mustMemoryConfig(t, tt.config))
				defer host.Reset()

				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
	})
}

func TestParseConfigOptionalBackends(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("recent cache and Console service names are optional", func(t *testing.T) {
			host, status := newMemoryConfigTestHost(mustMemoryConfig(t, map[string]interface{}{
				"redis_stream": map[string]interface{}{
					"service_name": "redis.memory.svc.cluster.local",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"memory-route"},
						"memory_mode":   "digest",
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
		})

		t.Run("Redis Stream service name remains required", func(t *testing.T) {
			host, status := newMemoryConfigTestHost(mustMemoryConfig(t, map[string]interface{}{
				"recent_cache": map[string]interface{}{
					"service_name": "redis.recent.svc.cluster.local",
				},
				"console_internal": map[string]interface{}{
					"service_name": "console.internal.svc.cluster.local",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"memory-route"},
						"memory_mode":   "digest",
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})
	})
}

func TestParseConfigSharedRedisConnectionValidation(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		base := map[string]interface{}{
			"redis_stream": map[string]interface{}{
				"service_name": "redis.shared",
				"database":     2,
				"username":     "memory-worker",
				"password":     "shared-secret",
			},
			"recent_cache": map[string]interface{}{
				"service_name": "redis.shared",
				"database":     2,
				"username":     "memory-worker",
				"password":     "shared-secret",
			},
			"fail_policy": "open",
			"_rules_": []map[string]interface{}{
				{
					"_match_route_": []string{"memory-route"},
					"memory_mode":   "recent-only",
				},
			},
		}

		t.Run("matching database and credentials are accepted", func(t *testing.T) {
			host, status := newMemoryConfigTestHost(mustMemoryConfig(t, base))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
		})

		for _, field := range []string{"database", "username", "password"} {
			t.Run("mismatched "+field+" is rejected", func(t *testing.T) {
				config := withGlobalConfig(base, map[string]interface{}{
					"recent_cache": map[string]interface{}{
						"service_name": "redis.shared",
						"database":     2,
						"username":     "memory-worker",
						"password":     "shared-secret",
					},
				})
				recent := config["recent_cache"].(map[string]interface{})
				switch field {
				case "database":
					recent[field] = 3
				default:
					recent[field] = "different"
				}

				host, status := newMemoryConfigTestHost(mustMemoryConfig(t, config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
	})
}

func TestParseConfigInjectRoleValidation(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		validExternalTargets := map[string]interface{}{
			"redis_stream": map[string]interface{}{
				"service_name": "redis.memory.svc.cluster.local",
			},
			"recent_cache": map[string]interface{}{
				"service_name": "redis.recent.svc.cluster.local",
			},
			"console_internal": map[string]interface{}{
				"service_name": "console.internal.svc.cluster.local",
			},
			"_rules_": []map[string]interface{}{
				{
					"_match_route_": []string{"memory-route"},
					"memory_mode":   "digest",
				},
			},
		}

		tests := []struct {
			name   string
			config map[string]interface{}
		}{
			{
				name: "invalid inject role",
				config: withRouteConfig(validExternalTargets, map[string]interface{}{
					"inject_role": "assistant",
				}),
			},
			{
				name: "developer inject role without compatibility",
				config: withRouteConfig(validExternalTargets, map[string]interface{}{
					"inject_role": "developer",
				}),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				host, status := newMemoryConfigTestHost(mustMemoryConfig(t, tt.config))
				defer host.Reset()

				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
	})
}

func TestParseConfigDeveloperCompatibleRoute(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		host, status := newMemoryConfigTestHost(mustMemoryConfig(t, map[string]interface{}{
			"redis_stream": map[string]interface{}{
				"service_name": "redis.memory.svc.cluster.local",
			},
			"recent_cache": map[string]interface{}{
				"service_name": "redis.recent.svc.cluster.local",
			},
			"console_internal": map[string]interface{}{
				"service_name": "console.internal.svc.cluster.local",
			},
			"_rules_": []map[string]interface{}{
				{
					"_match_route_":        []string{"memory-developer"},
					"memory_mode":          "digest",
					"inject_role":          "developer",
					"developer_compatible": true,
				},
			},
		}))
		defer host.Reset()

		require.Equal(t, types.OnPluginStartStatusOK, status)
		require.NoError(t, host.SetRouteName("memory-developer"))
		config, err := host.GetMatchConfig()
		require.NoError(t, err)

		require.Equal(t, "developer", mustStringField(t, config, "Route", "InjectRole"))
		require.True(t, mustBoolField(t, config, "Route", "DeveloperCompatible"))
	})
}

func withGlobalConfig(base map[string]interface{}, overrides map[string]interface{}) map[string]interface{} {
	config := make(map[string]interface{}, len(base)+len(overrides))
	for key, value := range base {
		config[key] = value
	}
	for key, value := range overrides {
		config[key] = value
	}
	return config
}

func withRouteConfig(base map[string]interface{}, routeOverrides map[string]interface{}) map[string]interface{} {
	config := make(map[string]interface{}, len(base))
	for key, value := range base {
		config[key] = value
	}
	route := map[string]interface{}{
		"_match_route_": []string{"memory-route"},
		"memory_mode":   "digest",
	}
	for key, value := range routeOverrides {
		route[key] = value
	}
	config["_rules_"] = []map[string]interface{}{route}
	return config
}
