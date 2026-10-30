package config

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type noopLogger struct{}

func (l noopLogger) Trace(msg string)                             {}
func (l noopLogger) Tracef(format string, args ...interface{})    {}
func (l noopLogger) Debug(msg string)                             {}
func (l noopLogger) Debugf(format string, args ...interface{})    {}
func (l noopLogger) Info(msg string)                              {}
func (l noopLogger) Infof(format string, args ...interface{})     {}
func (l noopLogger) Warn(msg string)                              {}
func (l noopLogger) Warnf(format string, args ...interface{})     {}
func (l noopLogger) Error(msg string)                             {}
func (l noopLogger) Errorf(format string, args ...interface{})    {}
func (l noopLogger) Critical(msg string)                          {}
func (l noopLogger) Criticalf(format string, args ...interface{}) {}
func (l noopLogger) ResetID(pluginID string)                      {}

func parseThinConfig(t *testing.T, raw []byte) PluginConfig {
	t.Helper()
	var cfg PluginConfig
	cfg.FromJson(gjson.ParseBytes(raw), noopLogger{})
	return cfg
}

func mustField(t *testing.T, cfg any, fieldPath ...string) reflect.Value {
	t.Helper()
	current := reflect.ValueOf(cfg)
	for current.Kind() == reflect.Pointer || current.Kind() == reflect.Interface {
		if current.IsNil() {
			require.Fail(t, "configuration pointer is nil while resolving field path")
		}
		current = current.Elem()
	}
	for _, fieldName := range fieldPath {
		for current.Kind() == reflect.Pointer || current.Kind() == reflect.Interface {
			if current.IsNil() {
				require.Fail(t, "field is nil", strings.Join(fieldPath, "."))
			}
			current = current.Elem()
		}
		require.Equalf(t, reflect.Struct, current.Kind(), "failed to resolve %q as struct; config shape missing thin config fields", strings.Join(fieldPath, "."))
		current = current.FieldByName(fieldName)
		require.Truef(t, current.IsValid(), "field %q is not present in config (thin config model not implemented yet)", strings.Join(append([]string{}, fieldPath...), "."))
	}
	for current.Kind() == reflect.Pointer || current.Kind() == reflect.Interface {
		if current.IsNil() {
			require.Fail(t, "field is nil", strings.Join(fieldPath, "."))
		}
		current = current.Elem()
	}
	return current
}

func mustStringField(t *testing.T, cfg any, fieldPath ...string) string {
	t.Helper()
	value := mustField(t, cfg, fieldPath...)
	require.Equalf(t, reflect.String, value.Kind(), "field %q must be a string", strings.Join(fieldPath, "."))
	return value.String()
}

func mustBoolField(t *testing.T, cfg any, fieldPath ...string) bool {
	t.Helper()
	value := mustField(t, cfg, fieldPath...)
	require.Equalf(t, reflect.Bool, value.Kind(), "field %q must be a bool", strings.Join(fieldPath, "."))
	return value.Bool()
}

func mustIntField(t *testing.T, cfg any, fieldPath ...string) int64 {
	t.Helper()
	value := mustField(t, cfg, fieldPath...)
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(value.Uint())
	default:
		require.Failf(t, "field is not integer", "field %q kind is %s", strings.Join(fieldPath, "."), value.Kind())
	}
	return 0
}

func mustSliceField(t *testing.T, cfg any, fieldPath ...string) []string {
	t.Helper()
	value := mustField(t, cfg, fieldPath...)
	require.Equalf(t, reflect.Slice, value.Kind(), "field %q must be a slice", strings.Join(fieldPath, "."))
	var out []string
	for i := 0; i < value.Len(); i++ {
		element := value.Index(i)
		require.Equalf(t, reflect.String, element.Kind(), "field %q[%d] must be a string", strings.Join(fieldPath, "."), i)
		out = append(out, element.String())
	}
	return out
}

func TestPluginConfig_ThinPluginConfigParsing(t *testing.T) {
	rawConfig := map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis-stack-server.higress-system.svc.cluster.local",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"console_lookup": map[string]interface{}{
			"enabled":      true,
			"service_name": "modelfusion-console.higress-system.svc.cluster.local",
			"service_port": 8080,
			"path":         "/internal/cache/lookup",
			"bearer_token": "console-service-token",
			"timeout":      50,
		},
		"redis_stream": map[string]interface{}{
			"enabled":      true,
			"service_name": "redis-stack-server.higress-system.svc.cluster.local",
			"service_port": 6379,
			"stream":       "cache:events",
			"field":        "event",
			"timeout":      120,
		},
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": true,
			"enable_replay":         true,
			"enable_bypass":         false,
			"enabled_path_suffixes": []string{
				"/v1/chat/completions",
				"/v1/messages",
			},
		},
		"tenant_header":        "x-tenant-id",
		"consumer_header":      "x-consumer-id",
		"session_header":       "x-session-id",
		"cache_scope":          "consumer",
		"cache_policy_version": "policy-v1",
	}
	cfgBytes, err := json.Marshal(rawConfig)
	require.NoError(t, err)

	cfg := parseThinConfig(t, cfgBytes)
	require.Equal(t, true, mustBoolField(t, cfg, "MaterializedLookup", "Redis", "Enabled"))
	require.Equal(t, "redis-stack-server.higress-system.svc.cluster.local", mustStringField(t, cfg, "MaterializedLookup", "Redis", "ServiceName"))
	require.Equal(t, "cache:materialized:", mustStringField(t, cfg, "MaterializedLookup", "Redis", "KeyPrefix"))
	require.Equal(t, int64(80), mustIntField(t, cfg, "MaterializedLookup", "Redis", "Timeout"))

	require.Equal(t, true, mustBoolField(t, cfg, "ConsoleLookup", "Enabled"))
	require.Equal(t, "modelfusion-console.higress-system.svc.cluster.local", mustStringField(t, cfg, "ConsoleLookup", "ServiceName"))
	require.Equal(t, "/internal/cache/lookup", mustStringField(t, cfg, "ConsoleLookup", "Path"))
	require.Equal(t, "console-service-token", mustStringField(t, cfg, "ConsoleLookup", "BearerToken"))
	require.Equal(t, int64(50), mustIntField(t, cfg, "ConsoleLookup", "Timeout"))

	require.Equal(t, true, mustBoolField(t, cfg, "Event", "RedisStream", "Enabled"))
	require.Equal(t, "cache:events", mustStringField(t, cfg, "Event", "RedisStream", "Stream"))
	require.Equal(t, "event", mustStringField(t, cfg, "Event", "RedisStream", "Field"))
	require.Equal(t, int64(120), mustIntField(t, cfg, "Event", "RedisStream", "Timeout"))

	require.Equal(t, true, mustBoolField(t, cfg, "RoutePolicy", "EnableRedisLookup"))
	require.Equal(t, true, mustBoolField(t, cfg, "RoutePolicy", "EnableConsoleLookup"))
	require.Equal(t, true, mustBoolField(t, cfg, "RoutePolicy", "EnableReplay"))
	require.Equal(t, false, mustBoolField(t, cfg, "RoutePolicy", "EnableBypass"))
	require.Equal(t, []string{"/v1/chat/completions", "/v1/messages"}, mustSliceField(t, cfg, "RoutePolicy", "EnabledPathSuffixes"))

	require.Equal(t, "x-tenant-id", mustStringField(t, cfg, "TenantHeader"))
	require.Equal(t, "x-consumer-id", mustStringField(t, cfg, "ConsumerHeader"))
	require.Equal(t, "x-session-id", mustStringField(t, cfg, "SessionHeader"))
	require.Equal(t, "consumer", mustStringField(t, cfg, "CacheScope"))
	require.Equal(t, "policy-v1", mustStringField(t, cfg, "CachePolicyVersion"))
}

func TestPluginConfig_ThinPluginConfigDefaults(t *testing.T) {
	rawConfig := map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled": true,
			},
		},
		"tenant_header":   "",
		"consumer_header": "",
	}
	cfgBytes, err := json.Marshal(rawConfig)
	require.NoError(t, err)

	cfg := parseThinConfig(t, cfgBytes)
	require.Equal(t, "open", mustStringField(t, cfg, "FailPolicy"))
	require.Equal(t, "x-mse-tenant", mustStringField(t, cfg, "TenantHeader"))
	require.Equal(t, "x-mse-consumer", mustStringField(t, cfg, "ConsumerHeader"))
	require.Equal(t, "x-openclaw-session-key", mustStringField(t, cfg, "SessionHeader"))
	require.Equal(t, false, mustBoolField(t, cfg, "ConsoleLookup", "Enabled"))
	require.Equal(t, "cache:events", mustStringField(t, cfg, "Event", "RedisStream", "Stream"))
}

func TestPluginConfig_ThinPluginConfigValidation(t *testing.T) {
	validConfig := map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis-stack-server.higress-system.svc.cluster.local",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"cache_policy_version": "policy-v1",
	}
	cfgBytes, err := json.Marshal(validConfig)
	require.NoError(t, err)
	cfg := parseThinConfig(t, cfgBytes)
	require.NoError(t, cfg.Validate())

	metadataOnly, err := json.Marshal(map[string]interface{}{
		"cache_policy_version": "policy-v1",
	})
	require.NoError(t, err)
	cfg = parseThinConfig(t, metadataOnly)
	require.Error(t, cfg.Validate())

	missingPolicyVersion, err := json.Marshal(map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis-stack-server.higress-system.svc.cluster.local",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
	})
	require.NoError(t, err)
	cfg = parseThinConfig(t, missingPolicyVersion)
	require.Error(t, cfg.Validate())

	missingRedisTarget, err := json.Marshal(map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled": true,
			},
		},
	})
	require.NoError(t, err)
	cfg = parseThinConfig(t, missingRedisTarget)
	require.Error(t, cfg.Validate())

	missingConsoleTarget, err := json.Marshal(map[string]interface{}{
		"console_lookup": map[string]interface{}{
			"enabled": true,
		},
	})
	require.NoError(t, err)
	cfg = parseThinConfig(t, missingConsoleTarget)
	require.Error(t, cfg.Validate())

	missingStreamTarget, err := json.Marshal(map[string]interface{}{
		"redis_stream": map[string]interface{}{
			"enabled": true,
		},
	})
	require.NoError(t, err)
	cfg = parseThinConfig(t, missingStreamTarget)
	require.Error(t, cfg.Validate())
}

func TestPluginConfig_RejectsLegacyOnlineCacheConfig(t *testing.T) {
	validThinConfig := map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis-stack-server.higress-system.svc.cluster.local",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"cache_policy_version": "policy-v1",
	}

	for name, configPatch := range map[string]map[string]interface{}{
		"cache provider": {
			"cache": map[string]interface{}{
				"type":        "redis",
				"serviceName": "redis.static",
			},
		},
		"embedding provider": {
			"embedding": map[string]interface{}{
				"type":        "dashscope",
				"serviceName": "dashscope.static",
				"apiKey":      "placeholder",
			},
		},
		"vector provider": {
			"vector": map[string]interface{}{
				"type":         "dashvector",
				"serviceName":  "dashvector.static",
				"serviceHost":  "dashvector.example.com",
				"apiKey":       "placeholder",
				"collectionID": "collection-a",
			},
		},
		"legacy extraction keys": {
			"cacheKeyStrategy":     "lastQuestion",
			"cacheKeyFrom":         "messages.@reverse.0.content",
			"cacheValueFrom":       "choices.0.message.content",
			"cacheStreamValueFrom": "choices.0.delta.content",
			"cacheToolCallsFrom":   "choices.0.delta.tool_calls",
		},
		"legacy response templates": {
			"responseTemplate":       `{"choices":[{"message":{"content":"%s"}}]}`,
			"streamResponseTemplate": "data:%s\n\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			rawConfig := map[string]interface{}{}
			for key, value := range validThinConfig {
				rawConfig[key] = value
			}
			for key, value := range configPatch {
				rawConfig[key] = value
			}
			cfgBytes, err := json.Marshal(rawConfig)
			require.NoError(t, err)

			cfg := parseThinConfig(t, cfgBytes)
			require.ErrorContains(t, cfg.Validate(), "legacy online cache configuration is not supported")
		})
	}
}

func TestPluginConfig_ThinPluginRouteOverrideInheritsExternalTargets(t *testing.T) {
	globalConfig := map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis-stack-server.higress-system.svc.cluster.local",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"console_lookup": map[string]interface{}{
			"enabled":      true,
			"service_name": "modelfusion-console.higress-system.svc.cluster.local",
			"service_port": 8080,
			"path":         "/internal/cache/lookup",
			"bearer_token": "global-console-token",
			"timeout":      50,
		},
		"redis_stream": map[string]interface{}{
			"enabled":      true,
			"service_name": "redis-stack-server.higress-system.svc.cluster.local",
			"service_port": 6379,
			"stream":       "cache:events",
			"field":        "event",
			"timeout":      120,
		},
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": true,
			"enable_replay":         true,
			"enabled_path_suffixes": []string{"/v1/chat/completions"},
		},
		"cache_policy_version": "policy-v1",
	}
	globalBytes, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	global := parseThinConfig(t, globalBytes)
	require.NoError(t, global.Validate())

	routeConfig := map[string]interface{}{
		"_match_route_": []string{"route-a"},
		"route_policy": map[string]interface{}{
			"enable_console_lookup": false,
			"enable_replay":         false,
			"enable_bypass":         true,
			"enabled_path_suffixes": []string{"/v1/messages"},
		},
		"cache_scope": "consumer",
	}
	routeBytes, err := json.Marshal(routeConfig)
	require.NoError(t, err)

	var route PluginConfig
	route.FromJsonWithGlobal(gjson.ParseBytes(routeBytes), global, noopLogger{})
	require.NoError(t, route.Validate())

	require.Equal(t, global.MaterializedLookup.Redis, route.MaterializedLookup.Redis)
	require.Equal(t, global.ConsoleLookup.ServiceName, route.ConsoleLookup.ServiceName)
	require.Equal(t, global.ConsoleLookup.ServicePort, route.ConsoleLookup.ServicePort)
	require.Equal(t, global.ConsoleLookup.Path, route.ConsoleLookup.Path)
	require.Equal(t, global.ConsoleLookup.BearerToken, route.ConsoleLookup.BearerToken)
	require.Equal(t, global.ConsoleLookup.Timeout, route.ConsoleLookup.Timeout)
	require.Equal(t, global.Event.RedisStream, route.Event.RedisStream)
	require.Equal(t, false, route.RoutePolicy.EnableConsoleLookup)
	require.Equal(t, false, route.RoutePolicy.EnableReplay)
	require.Equal(t, true, route.RoutePolicy.EnableBypass)
	require.Equal(t, []string{"/v1/messages"}, route.RoutePolicy.EnabledPathSuffixes)
	require.Equal(t, "consumer", route.CacheScope)
	require.Equal(t, "policy-v1", route.CachePolicyVersion)
}

func TestPluginConfig_ThinPluginRuleOnlyConfigInitializesProviderConfigs(t *testing.T) {
	routeConfig := map[string]interface{}{
		"_match_route_": []string{"route-only"},
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis-stack-server.higress-system.svc.cluster.local",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"cache_policy_version": "policy-v1",
	}
	routeBytes, err := json.Marshal(routeConfig)
	require.NoError(t, err)

	var route PluginConfig
	var validateErr error
	require.NotPanics(t, func() {
		route.FromJsonWithGlobal(gjson.ParseBytes(routeBytes), PluginConfig{}, noopLogger{})
		validateErr = route.Validate()
	})
	require.NoError(t, validateErr)
	require.Equal(t, true, route.MaterializedLookup.Redis.Enabled)
	require.Equal(t, CACHE_SCOPE_TENANT, route.CacheScope)
	require.Equal(t, FAIL_POLICY_OPEN, route.FailPolicy)
}

func TestPluginConfig_ThinPluginRouteOverrideRejectsEmptyPolicyVersion(t *testing.T) {
	globalConfig := map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis-stack-server.higress-system.svc.cluster.local",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"cache_policy_version": "policy-v1",
	}
	globalBytes, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	global := parseThinConfig(t, globalBytes)
	require.NoError(t, global.Validate())

	routeConfig := map[string]interface{}{
		"_match_route_":        []string{"route-a"},
		"cache_policy_version": "",
	}
	routeBytes, err := json.Marshal(routeConfig)
	require.NoError(t, err)

	var route PluginConfig
	route.FromJsonWithGlobal(gjson.ParseBytes(routeBytes), global, noopLogger{})
	require.ErrorContains(t, route.Validate(), "cache_policy_version")
}

func TestPluginConfig_ThinPluginRouteConsoleLookupOverrideUpdatesPolicyDefault(t *testing.T) {
	globalConfig := map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis-stack-server.higress-system.svc.cluster.local",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"cache_policy_version": "policy-v1",
	}
	globalBytes, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	global := parseThinConfig(t, globalBytes)
	require.NoError(t, global.Validate())
	require.Equal(t, false, global.RoutePolicy.EnableConsoleLookup)

	enableRoute := map[string]interface{}{
		"_match_route_": []string{"route-a"},
		"console_lookup": map[string]interface{}{
			"enabled":      true,
			"service_name": "modelfusion-console.higress-system.svc.cluster.local",
			"service_port": 8080,
			"timeout":      50,
		},
	}
	enableRouteBytes, err := json.Marshal(enableRoute)
	require.NoError(t, err)

	var enabled PluginConfig
	enabled.FromJsonWithGlobal(gjson.ParseBytes(enableRouteBytes), global, noopLogger{})
	require.NoError(t, enabled.Validate())
	require.Equal(t, true, enabled.ConsoleLookup.Enabled)
	require.Equal(t, true, enabled.RoutePolicy.EnableConsoleLookup)

	globalWithConsoleConfig := map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis-stack-server.higress-system.svc.cluster.local",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"console_lookup": map[string]interface{}{
			"enabled":      true,
			"service_name": "modelfusion-console.higress-system.svc.cluster.local",
			"service_port": 8080,
			"timeout":      50,
		},
		"cache_policy_version": "policy-v1",
	}
	globalWithConsoleBytes, err := json.Marshal(globalWithConsoleConfig)
	require.NoError(t, err)
	globalWithConsole := parseThinConfig(t, globalWithConsoleBytes)
	require.NoError(t, globalWithConsole.Validate())
	require.Equal(t, true, globalWithConsole.RoutePolicy.EnableConsoleLookup)

	disableRoute := map[string]interface{}{
		"_match_route_": []string{"route-b"},
		"console_lookup": map[string]interface{}{
			"enabled": false,
		},
	}
	disableRouteBytes, err := json.Marshal(disableRoute)
	require.NoError(t, err)

	var disabled PluginConfig
	disabled.FromJsonWithGlobal(gjson.ParseBytes(disableRouteBytes), globalWithConsole, noopLogger{})
	require.NoError(t, disabled.Validate())
	require.Equal(t, false, disabled.ConsoleLookup.Enabled)
	require.Equal(t, false, disabled.RoutePolicy.EnableConsoleLookup)
}
