package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func mustMemoryConfig(t *testing.T, value map[string]interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}

func newMemoryConfigTestHost(config json.RawMessage) (test.TestHost, types.OnPluginStartStatus) {
	return test.NewTestHost(config)
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
		require.Equalf(t, reflect.Struct, current.Kind(), "failed to resolve %q as struct; config shape missing ai-memory fields", strings.Join(fieldPath, "."))
		current = current.FieldByName(fieldName)
		require.Truef(t, current.IsValid(), "field %q is not present in config", strings.Join(fieldPath, "."))
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

func TestParseConfig(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("global external targets, identity headers, suffixes, fail policy, and route behavior", func(t *testing.T) {
			host, status := newMemoryConfigTestHost(mustMemoryConfig(t, map[string]interface{}{
				"redis_stream": map[string]interface{}{
					"service_name": "redis.memory.svc.cluster.local",
					"service_port": 6380,
					"database":     3,
					"timeout":      750,
					"stream":       "memory:events",
				},
				"recent_cache": map[string]interface{}{
					"service_name": "redis.recent.svc.cluster.local",
					"service_port": 6381,
					"database":     4,
					"timeout":      60,
					"key_prefix":   "memory:recent:v1",
				},
				"console_internal": map[string]interface{}{
					"service_name":  "modelfusion-console.higress-system.svc.cluster.local",
					"service_port":  8080,
					"assemble_path": "/internal/memory/assemble",
					"timeout_ms":    120,
					"auth_token":    "placeholder-internal-token",
				},
				"tenant_header":        "x-tenant-id",
				"consumer_header":      "x-consumer-id",
				"session_header":       "x-session-id",
				"request_id_header":    "x-request-id",
				"enable_path_suffixes": []string{"/v1/chat/completions", "/v1/messages"},
				"fail_policy":          "open",
				"_rules_": []map[string]interface{}{
					{
						"_match_route_":       []string{"memory-semantic"},
						"memory_mode":         "semantic",
						"recent_window_turns": 6,
						"memory_token_budget": 1500,
						"assemble_timeout_ms": 80,
						"inject_role":         "system",
						"semantic_top_k":      3,
						"capture_response":    true,
						"no_store_header":     "x-memory-no-store",
						"question_from":       `messages.@reverse.#(role=="user").content`,
						"response_value_from": "choices.0.message.content",
						"stream_value_from":   "choices.0.delta.content",
						"tool_calls_from": []string{
							"choices.0.delta.tool_calls",
							"choices.0.delta.content.tool_calls",
						},
						"policy_version": "7",
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("memory-semantic"))
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			require.Equal(t, "redis.memory.svc.cluster.local", mustStringField(t, config, "RedisStream", "ServiceName"))
			require.Equal(t, int64(6380), mustIntField(t, config, "RedisStream", "ServicePort"))
			require.Equal(t, int64(3), mustIntField(t, config, "RedisStream", "Database"))
			require.Equal(t, int64(750), mustIntField(t, config, "RedisStream", "Timeout"))
			require.Equal(t, "memory:events", mustStringField(t, config, "RedisStream", "Stream"))

			require.Equal(t, "redis.recent.svc.cluster.local", mustStringField(t, config, "RecentCache", "ServiceName"))
			require.Equal(t, int64(6381), mustIntField(t, config, "RecentCache", "ServicePort"))
			require.Equal(t, int64(4), mustIntField(t, config, "RecentCache", "Database"))
			require.Equal(t, int64(60), mustIntField(t, config, "RecentCache", "Timeout"))
			require.Equal(t, "memory:recent:v1", mustStringField(t, config, "RecentCache", "KeyPrefix"))

			require.Equal(t, "modelfusion-console.higress-system.svc.cluster.local", mustStringField(t, config, "ConsoleInternal", "ServiceName"))
			require.Equal(t, int64(8080), mustIntField(t, config, "ConsoleInternal", "ServicePort"))
			require.Equal(t, "/internal/memory/assemble", mustStringField(t, config, "ConsoleInternal", "AssemblePath"))
			require.Equal(t, int64(120), mustIntField(t, config, "ConsoleInternal", "TimeoutMS"))
			require.Equal(t, "placeholder-internal-token", mustStringField(t, config, "ConsoleInternal", "AuthToken"))

			require.Equal(t, "x-tenant-id", mustStringField(t, config, "TenantHeader"))
			require.Equal(t, "x-consumer-id", mustStringField(t, config, "ConsumerHeader"))
			require.Equal(t, "x-session-id", mustStringField(t, config, "SessionHeader"))
			require.Equal(t, "x-request-id", mustStringField(t, config, "RequestIDHeader"))
			require.Equal(t, []string{"/v1/chat/completions", "/v1/messages"}, mustSliceField(t, config, "EnablePathSuffixes"))
			require.Equal(t, "open", mustStringField(t, config, "FailPolicy"))

			require.Equal(t, "semantic", mustStringField(t, config, "Route", "MemoryMode"))
			require.Equal(t, int64(6), mustIntField(t, config, "Route", "RecentWindowTurns"))
			require.Equal(t, int64(1500), mustIntField(t, config, "Route", "MemoryTokenBudget"))
			require.Equal(t, int64(80), mustIntField(t, config, "Route", "AssembleTimeoutMS"))
			require.Equal(t, "system", mustStringField(t, config, "Route", "InjectRole"))
			require.Equal(t, int64(3), mustIntField(t, config, "Route", "SemanticTopK"))
			require.True(t, mustBoolField(t, config, "Route", "CaptureResponse"))
			require.Equal(t, "x-memory-no-store", mustStringField(t, config, "Route", "NoStoreHeader"))
			require.Equal(t, `messages.@reverse.#(role=="user").content`, mustStringField(t, config, "Route", "QuestionFrom"))
			require.Equal(t, "choices.0.message.content", mustStringField(t, config, "Route", "ResponseValueFrom"))
			require.Equal(t, "choices.0.delta.content", mustStringField(t, config, "Route", "StreamValueFrom"))
			require.Equal(t, []string{"choices.0.delta.tool_calls", "choices.0.delta.content.tool_calls"}, mustSliceField(t, config, "Route", "ToolCallsFrom"))
			require.Equal(t, "7", mustStringField(t, config, "Route", "PolicyVersion"))
		})

		t.Run("global defaults are inherited when route only configures memory behavior", func(t *testing.T) {
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
						"_match_route_":       []string{"memory-digest"},
						"memory_mode":         "digest",
						"recent_window_turns": 4,
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("memory-digest"))
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			require.Equal(t, "redis.memory.svc.cluster.local", mustStringField(t, config, "RedisStream", "ServiceName"))
			require.Equal(t, int64(6379), mustIntField(t, config, "RedisStream", "ServicePort"))
			require.Equal(t, int64(0), mustIntField(t, config, "RedisStream", "Database"))
			require.Equal(t, int64(500), mustIntField(t, config, "RedisStream", "Timeout"))
			require.Equal(t, "memory:events", mustStringField(t, config, "RedisStream", "Stream"))

			require.Equal(t, "redis.recent.svc.cluster.local", mustStringField(t, config, "RecentCache", "ServiceName"))
			require.Equal(t, int64(6379), mustIntField(t, config, "RecentCache", "ServicePort"))
			require.Equal(t, int64(0), mustIntField(t, config, "RecentCache", "Database"))
			require.Equal(t, int64(50), mustIntField(t, config, "RecentCache", "Timeout"))
			require.Equal(t, "memory:recent", mustStringField(t, config, "RecentCache", "KeyPrefix"))

			require.Equal(t, "console.internal.svc.cluster.local", mustStringField(t, config, "ConsoleInternal", "ServiceName"))
			require.Equal(t, int64(80), mustIntField(t, config, "ConsoleInternal", "ServicePort"))
			require.Equal(t, "/internal/memory/assemble", mustStringField(t, config, "ConsoleInternal", "AssemblePath"))
			require.Equal(t, int64(100), mustIntField(t, config, "ConsoleInternal", "TimeoutMS"))
			require.Empty(t, mustStringField(t, config, "ConsoleInternal", "AuthToken"))

			require.Equal(t, "x-mse-tenant", mustStringField(t, config, "TenantHeader"))
			require.Equal(t, "x-mse-consumer", mustStringField(t, config, "ConsumerHeader"))
			require.Equal(t, "x-mse-session", mustStringField(t, config, "SessionHeader"))
			require.Equal(t, "x-request-id", mustStringField(t, config, "RequestIDHeader"))
			require.Equal(t, []string{"/v1/chat/completions", "/v1/messages", "/v1/responses"}, mustSliceField(t, config, "EnablePathSuffixes"))
			require.Equal(t, "open", mustStringField(t, config, "FailPolicy"))
			require.Equal(t, "digest", mustStringField(t, config, "Route", "MemoryMode"))
			require.Equal(t, int64(4), mustIntField(t, config, "Route", "RecentWindowTurns"))
			require.Equal(t, "system", mustStringField(t, config, "Route", "InjectRole"))
			require.True(t, mustBoolField(t, config, "Route", "CaptureResponse"))
		})
	})
}
