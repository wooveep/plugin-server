package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMemoryResourceCatalog(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		root := memoryResourceCatalogRoot(t)
		readme := memoryResourceText(t, root, "README.md")
		readmeEN := memoryResourceText(t, root, "README_EN.md")
		specText := memoryResourceText(t, root, "spec.yaml")
		spec := memoryResourceYAML(t, specText)

		t.Run("resource files exist and docs describe the thin plugin contract", func(t *testing.T) {
			requireMemoryResourceReadmeTopics(t, "README.md", readme)
			requireMemoryResourceReadmeTopics(t, "README_EN.md", readmeEN)
			requireMemoryResourcePrivacyBoundary(t, "README.md", readme)
			requireMemoryResourcePrivacyBoundary(t, "README_EN.md", readmeEN)
		})

		t.Run("spec schema exposes global targets and route memory behavior", func(t *testing.T) {
			require.Equal(t, "ai-memory", memoryResourceNestedMap(t, spec, "info")["name"])

			globalSchema := memoryResourceNestedMap(t, spec, "spec", "configSchema", "openAPIV3Schema")
			routeSchema := memoryResourceNestedMap(t, spec, "spec", "routeConfigSchema", "openAPIV3Schema")
			globalProps := memoryResourceNestedMap(t, globalSchema, "properties")
			routeProps := memoryResourceNestedMap(t, routeSchema, "properties")

			requireMemoryResourceKeys(t, globalProps, []string{
				"native_identity",
				"capture_body_basis",
				"redis_stream",
				"recent_cache",
				"console_internal",
				"tenant_header",
				"consumer_header",
				"session_header",
				"request_id_header",
				"enable_path_suffixes",
				"fail_policy",
			})
			requireMemoryResourceKeys(t, routeProps, []string{
				"native_identity",
				"capture_body_basis",
				"memory_mode",
				"recent_window_turns",
				"memory_token_budget",
				"assemble_timeout_ms",
				"inject_role",
				"developer_compatible",
				"semantic_top_k",
				"capture_response",
				"no_store_header",
				"question_from",
				"response_value_from",
				"stream_value_from",
				"tool_calls_from",
				"policy_version",
			})
			for _, key := range []string{"redis_stream", "recent_cache", "console_internal", "tenant_header", "consumer_header", "session_header", "request_id_header", "fail_policy", "native_identity"} {
				require.Equal(t, globalProps[key], routeProps[key], "complete standalone rules must expose the original %s shape", key)
			}
			for _, properties := range []map[string]interface{}{globalProps, routeProps} {
				native := memoryResourceNestedMap(t, properties, "native_identity")
				require.Equal(t, false, native["additionalProperties"])
				require.ElementsMatch(t, []interface{}{"release_id", "target_id", "route_id", "policy_id", "policy_version"}, native["required"])
				nativeProps := memoryResourceNestedMap(t, native, "properties")
				require.Len(t, nativeProps, 5)
				require.NotContains(t, nativeProps, "caller", "authenticated caller must never be rule configuration")
				require.Equal(t, "integer", memoryResourceNestedMap(t, nativeProps, "policy_version")["type"])
				require.Equal(t, "boolean", memoryResourceNestedMap(t, properties, "capture_body_basis")["type"])
			}
			requireMemoryResourceAdditionalPropertiesFalse(t, routeSchema)
			requireMemoryResourceEnum(t, routeProps["memory_mode"], []string{"off", "recent-only", "digest", "semantic"})
			requireMemoryResourceEnum(t, globalProps["fail_policy"], []string{"open"})
		})

		t.Run("schema examples omit credential values and preserve sensitive field metadata", func(t *testing.T) {
			globalExample := memoryResourceNestedMap(t, spec, "spec", "configSchema", "openAPIV3Schema", "example")
			routeExample := memoryResourceNestedMap(t, spec, "spec", "routeConfigSchema", "openAPIV3Schema", "example")

			requireMemoryResourceKeys(t, globalExample, []string{
				"redis_stream",
				"recent_cache",
				"console_internal",
				"tenant_header",
				"consumer_header",
				"session_header",
				"request_id_header",
				"enable_path_suffixes",
				"fail_policy",
			})
			requireMemoryResourceKeys(t, routeExample, []string{
				"memory_mode",
				"recent_window_turns",
				"memory_token_budget",
				"assemble_timeout_ms",
				"inject_role",
				"developer_compatible",
				"semantic_top_k",
				"capture_response",
				"no_store_header",
				"question_from",
				"response_value_from",
				"stream_value_from",
				"tool_calls_from",
				"policy_version",
			})
			requireMemoryResourceAbsentKeys(t, routeExample, []string{"redis_stream", "recent_cache", "console_internal"})
			requireMemoryResourceAbsentKeys(t, routeExample, []string{
				"tenant_header",
				"consumer_header",
				"session_header",
				"request_id_header",
				"enable_path_suffixes",
				"fail_policy",
			})
			for _, credential := range []struct{ section, field string }{{"redis_stream", "password"}, {"recent_cache", "password"}, {"console_internal", "auth_token"}} {
				require.NotContains(t, memoryResourceNestedMap(t, globalExample, credential.section), credential.field)
				for _, schema := range []string{"configSchema", "routeConfigSchema"} {
					field := memoryResourceNestedMap(t, spec, "spec", schema, "openAPIV3Schema", "properties", credential.section, "properties", credential.field)
					require.Equal(t, "password", field["format"])
					require.Equal(t, true, field["writeOnly"])
					require.Equal(t, true, field["x-sensitive"])
					for _, value := range []string{"default", "example", "examples"} {
						require.NotContains(t, field, value)
					}
				}
			}
			requireMemoryResourceNoSecretLiterals(t, "spec.yaml", specText)
			requireMemoryResourceSecretPlaceholders(t, "README.md", readme, 1)
			requireMemoryResourceNoSecretLiterals(t, "README.md", readme)
			requireMemoryResourceSecretPlaceholders(t, "README_EN.md", readmeEN, 1)
			requireMemoryResourceNoSecretLiterals(t, "README_EN.md", readmeEN)
		})
	})
}

func memoryResourceCatalogRoot(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok, "resource catalog test must resolve source path")
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "resources", "plugins", "ai-memory"))
}

func memoryResourceText(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	body, err := os.ReadFile(path)
	require.NoErrorf(t, err, "resource catalog file %s must exist", path)
	require.NotEmptyf(t, body, "resource catalog file %s must not be empty", path)
	return string(body)
}

func memoryResourceYAML(t *testing.T, text string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(text), &out), "resource spec.yaml must parse as YAML")
	return out
}

func memoryResourceNestedMap(t *testing.T, root map[string]interface{}, path ...string) map[string]interface{} {
	t.Helper()
	var current interface{} = root
	for _, segment := range path {
		m, ok := current.(map[string]interface{})
		require.Truef(t, ok, "resource spec path %s must be an object", strings.Join(path, "."))
		value, ok := m[segment]
		require.Truef(t, ok, "resource spec path %s missing segment %q", strings.Join(path, "."), segment)
		current = value
	}
	m, ok := current.(map[string]interface{})
	require.Truef(t, ok, "resource spec path %s must resolve to an object", strings.Join(path, "."))
	return m
}

type memoryResourceReadmeTopic struct {
	name         string
	alternatives []string
}

func requireMemoryResourceReadmeTopics(t *testing.T, label, text string) {
	t.Helper()
	topics := []memoryResourceReadmeTopic{
		{name: "plugin identity", alternatives: []string{"ai-memory"}},
		{name: "request gating", alternatives: []string{"request gating", "path suffix", "content-type", "enable_path_suffixes", "请求门控", "路径后缀"}},
		{name: "tenant identity", alternatives: []string{"tenant_header", "x-mse-tenant", "tenant identity", "租户"}},
		{name: "consumer identity", alternatives: []string{"consumer_header", "x-mse-consumer", "consumer identity", "consumer"}},
		{name: "Redis recent-memory fallback", alternatives: []string{"recent_cache", "memory:recent", "Redis recent", "recent memory", "最近记忆"}},
		{name: "Console assemble", alternatives: []string{"console_internal", "/internal/memory/assemble", "Console assemble", "memory assemble"}},
		{name: "request body injection", alternatives: []string{"request body", "inject", "memory message", "请求体", "注入"}},
		{name: "response capture", alternatives: []string{"response capture", "capture_response", "assistant response", "响应捕获"}},
		{name: "Redis Stream MemoryEvent emission", alternatives: []string{"MemoryEvent", "memory:events", "Redis Stream", "XADD"}},
		{name: "fail-open behavior", alternatives: []string{"fail-open", "fail open", "fail_policy", "失败开放"}},
		{name: "plugin ordering", alternatives: []string{"plugin ordering", "ai-quota", "ai-cache", "ai-proxy", "插件顺序"}},
		{name: "Console PostgreSQL ownership", alternatives: []string{"PostgreSQL", "Console owns", "Console 负责"}},
		{name: "Console recent materialization ownership", alternatives: []string{"recent-window materialization", "recent window materialization", "recent-memory materialization", "最近窗口"}},
		{name: "Console digest ownership", alternatives: []string{"daily digest", "digests", "digest generation", "摘要"}},
		{name: "Console semantic ownership", alternatives: []string{"semantic recall", "semantic", "语义召回"}},
		{name: "Console embedding ownership", alternatives: []string{"embedding", "embeddings", "向量"}},
		{name: "Console vector ownership", alternatives: []string{"vector indexing", "vector index", "vector", "向量索引"}},
		{name: "Console deletion ownership", alternatives: []string{"deletion", "delete", "删除"}},
		{name: "Console repair ownership", alternatives: []string{"repair", "修复"}},
		{name: "Console management API ownership", alternatives: []string{"management API", "management APIs", "管理 API"}},
		{name: "Console backend model cost ownership", alternatives: []string{"backend model cost", "cost attribution", "model cost", "模型成本"}},
		{name: "raw prompt privacy", alternatives: []string{"raw prompts", "raw prompt", "prompts", "原始 prompt"}},
		{name: "raw answer privacy", alternatives: []string{"raw answers", "raw answer", "answers", "原始回答"}},
		{name: "credential privacy", alternatives: []string{"credentials", "credential", "secret", "密钥"}},
		{name: "authorization header privacy", alternatives: []string{"authorization headers", "Authorization", "鉴权"}},
		{name: "Redis password privacy", alternatives: []string{"Redis passwords", "redis password", "redis_password"}},
		{name: "internal bearer token privacy", alternatives: []string{"internal bearer tokens", "internal bearer", "Bearer"}},
		{name: "defaultConfig example", alternatives: []string{"defaultConfig"}},
		{name: "matchRules example", alternatives: []string{"matchRules"}},
	}
	for _, topic := range topics {
		requireMemoryResourceContainsAny(t, label, text, topic)
	}
}

func requireMemoryResourcePrivacyBoundary(t *testing.T, label, text string) {
	t.Helper()
	normalized := strings.ToLower(strings.Join(strings.Fields(text), " "))
	requireMemoryResourceContainsAny(t, label, normalized, memoryResourceReadmeTopic{
		name: "privacy non-logging statement",
		alternatives: []string{
			"does not log",
			"must not log",
			"never logs",
			"not log",
			"不会记录",
			"不得记录",
			"不记录",
			"不会写入日志",
			"不得写入日志",
		},
	})
	for _, topic := range []memoryResourceReadmeTopic{
		{name: "raw prompt privacy", alternatives: []string{"raw prompts", "raw prompt", "prompts", "原始 prompt", "原始提示"}},
		{name: "raw answer privacy", alternatives: []string{"raw answers", "raw answer", "answers", "原始回答"}},
		{name: "credential privacy", alternatives: []string{"credentials", "credential", "secrets", "密钥", "凭据"}},
		{name: "authorization header privacy", alternatives: []string{"authorization headers", "authorization header", "authorization", "鉴权请求头", "授权头"}},
		{name: "Redis password privacy", alternatives: []string{"redis passwords", "redis password", "redis_password", "redis 密码"}},
		{name: "internal bearer token privacy", alternatives: []string{"internal bearer tokens", "internal bearer token", "internal bearer", "bearer token", "内部 bearer"}},
	} {
		requireMemoryResourceContainsAny(t, label, normalized, topic)
	}
}

func requireMemoryResourceContainsAny(t *testing.T, label, text string, topic memoryResourceReadmeTopic) {
	t.Helper()
	lowerText := strings.ToLower(text)
	for _, alternative := range topic.alternatives {
		if strings.Contains(lowerText, strings.ToLower(alternative)) {
			return
		}
	}
	require.Failf(t, "missing resource documentation topic", "%s missing %q; expected one of %v", label, topic.name, topic.alternatives)
}

func requireMemoryResourceKeys(t *testing.T, m map[string]interface{}, keys []string) {
	t.Helper()
	for _, key := range keys {
		require.Containsf(t, m, key, "resource schema missing key %q", key)
	}
}

func requireMemoryResourceAbsentKeys(t *testing.T, m map[string]interface{}, keys []string) {
	t.Helper()
	for _, key := range keys {
		require.NotContainsf(t, m, key, "resource route schema/example must not include external target override %q", key)
	}
}

func requireMemoryResourceAdditionalPropertiesFalse(t *testing.T, schema map[string]interface{}) {
	t.Helper()
	value, ok := schema["additionalProperties"]
	require.True(t, ok, "resource route config schema must explicitly reject undeclared fields")
	require.Equal(t, false, value, "resource route config schema must set additionalProperties: false")
}

func requireMemoryResourceEnum(t *testing.T, field interface{}, expected []string) {
	t.Helper()
	m, ok := field.(map[string]interface{})
	require.True(t, ok, "schema field must be an object")
	rawEnum, ok := m["enum"].([]interface{})
	require.True(t, ok, "schema field must define enum values")
	values := make([]string, 0, len(rawEnum))
	for _, item := range rawEnum {
		value, ok := item.(string)
		require.True(t, ok, "enum value must be a string")
		values = append(values, value)
	}
	require.ElementsMatch(t, expected, values)
}

func requireMemoryResourceSecretPlaceholders(t *testing.T, label, text string, minMatches int) {
	t.Helper()
	pattern := regexp.MustCompile(`(?im)^[ \t]*(auth_token|password|redis_password|bearer_token|service_auth)[ \t]*:[ \t]*([^ \t\r\n#]+)`)
	matches := pattern.FindAllStringSubmatch(text, -1)
	require.GreaterOrEqualf(t, len(matches), minMatches, "%s must include at least %d secret placeholder example(s)", label, minMatches)
	for _, match := range matches {
		value := strings.Trim(match[2], `"'`)
		require.Truef(t, strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">"), "%s example field %s must use a placeholder, got %q", label, match[1], value)
	}
}

func requireMemoryResourceNoSecretLiterals(t *testing.T, label, text string) {
	t.Helper()
	secretPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?im)^[ \t]*authorization[ \t]*:[ \t]*bearer[ \t]+["']?[^<"' \t\r\n#][^ \t\r\n#]*`),
		regexp.MustCompile(`(?im)^[ \t]*(authorization|auth[_-]?token|bearer[_-]?token|service[_-]?auth)[ \t:=]+bearer[ \t]+["']?(sk-[a-z0-9]|[a-z0-9._-]*[0-9._-][a-z0-9._-]{11,})`),
		regexp.MustCompile(`(?im)^[ \t]*(api[_-]?key|auth[_-]?token|bearer[_-]?token|redis[_-]?password|password|service[_-]?auth)[ \t]*:[ \t]*["']?[^<"' \t\r\n#][^ \t\r\n#]*`),
		regexp.MustCompile(`(?im)^[ \t]*(secret|token|password)[ \t]*=[ \t]*["']?[^<"' \t\r\n#][^ \t\r\n#]*`),
	}
	for _, pattern := range secretPatterns {
		require.Falsef(t, pattern.MatchString(text), "%s contains a secret-looking literal matching %s; use placeholders instead", label, pattern.String())
	}
}
