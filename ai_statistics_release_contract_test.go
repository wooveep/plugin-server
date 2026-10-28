package wasmgo_test

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAIStatisticsReleaseSchemaAndDocumentationContract(t *testing.T) {
	t.Parallel()

	assertClosedObject := func(t *testing.T, schema map[string]any, name string) {
		t.Helper()
		value, exists := schema["additionalProperties"]
		if !exists {
			t.Errorf("%s.additionalProperties is absent, want false", name)
			return
		}
		closed, ok := value.(bool)
		if !ok || closed {
			t.Errorf("%s.additionalProperties = %#v, want false", name, value)
		}
	}
	assertExactProperties := func(t *testing.T, properties map[string]any, expected []string) {
		t.Helper()
		expectedSet := make(map[string]struct{}, len(expected))
		for _, name := range expected {
			expectedSet[name] = struct{}{}
			if _, exists := properties[name]; !exists {
				t.Errorf("property %s is absent", name)
			}
		}
		for name := range properties {
			if _, exists := expectedSet[name]; !exists {
				t.Errorf("unexpected property %s", name)
			}
		}
	}
	assertIntegerDefault := func(t *testing.T, properties map[string]any, name string, expected int) {
		t.Helper()
		field := nestedMapping(t, properties, name)
		value, exists := field["default"]
		if !exists {
			t.Errorf("%s.default is absent, want %d", name, expected)
			return
		}
		if value != expected {
			t.Errorf("%s.default = %#v, want %d", name, value, expected)
		}
	}
	assertBoolDefault := func(t *testing.T, properties map[string]any, name string, expected bool) {
		t.Helper()
		field := nestedMapping(t, properties, name)
		value, exists := field["default"]
		if !exists {
			t.Errorf("%s.default is absent, want %t", name, expected)
			return
		}
		if value != expected {
			t.Errorf("%s.default = %#v, want %t", name, value, expected)
		}
	}
	requireFragments := func(t *testing.T, body string, fragments []string) {
		t.Helper()
		for _, fragment := range fragments {
			if !strings.Contains(strings.ToLower(body), strings.ToLower(fragment)) {
				t.Errorf("missing required contract fragment %q", fragment)
			}
		}
	}
	forbidFragments := func(t *testing.T, body string, fragments []string) {
		t.Helper()
		for _, fragment := range fragments {
			if strings.Contains(strings.ToLower(body), strings.ToLower(fragment)) {
				t.Errorf("found prohibited contract fragment %q", fragment)
			}
		}
	}
	markdownSection := func(t *testing.T, body, heading string) string {
		t.Helper()
		start := strings.Index(body, heading)
		if start < 0 {
			t.Errorf("missing required section %q", heading)
			return ""
		}
		section := body[start+len(heading):]
		if end := strings.Index(section, "\n## "); end >= 0 {
			section = section[:end]
		}
		return section
	}

	t.Run("parser fixture", func(t *testing.T) {
		var fixture map[string]any
		err := yaml.Unmarshal([]byte(`
schema:
  additionalProperties: false
  properties:
    path_scope:
      type: string
      default: legacy_suffixes
    attributes:
      type: array
      items:
        type: object
        additionalProperties: false
        properties:
          key:
            type: string
`), &fixture)
		if err != nil {
			t.Fatalf("parse fixture YAML: %v", err)
		}
		schema := nestedMapping(t, fixture, "schema")
		assertClosedObject(t, schema, "schema")
		properties := nestedMapping(t, schema, "properties")
		assertExactProperties(t, properties, []string{"path_scope", "attributes"})
		assertStringValue(t, nestedMapping(t, properties, "path_scope"), "default", "legacy_suffixes")
		assertClosedObject(t, nestedMapping(t, properties, "attributes", "items"), "attributes.items")
	})

	t.Run("closed schema and exact supported properties", func(t *testing.T) {
		resource := loadYAMLMapping(t, "resources/plugins/ai-statistics/spec.yaml")
		globalSchema := nestedMapping(t, resource, "spec", "configSchema", "openAPIV3Schema")
		routeSchema := nestedMapping(t, resource, "spec", "routeConfigSchema", "openAPIV3Schema")
		globalProperties := nestedMapping(t, globalSchema, "properties")
		routeProperties := nestedMapping(t, routeSchema, "properties")

		assertClosedObject(t, globalSchema, "global schema")
		assertClosedObject(t, routeSchema, "route schema")
		assertExactProperties(t, globalProperties, []string{
			"use_default_attributes",
			"use_default_response_attributes",
			"attributes",
			"disable_openai_usage",
			"value_length_limit",
			"path_scope",
			"enable_path_suffixes",
			"enable_methods",
			"enable_content_types",
			"session_id_header",
		})
		assertExactProperties(t, routeProperties, []string{
			"use_default_attributes",
			"use_default_response_attributes",
			"attributes",
			"disable_openai_usage",
			"value_length_limit",
			"path_scope",
			"enable_path_suffixes",
			"enable_methods",
			"enable_content_types",
			"session_id_header",
		})

		attributeItems := nestedMapping(t, globalProperties, "attributes", "items")
		assertClosedObject(t, attributeItems, "attributes.items")
		assertStringSlice(t, attributeItems, "required", []string{"key"})
		assertExactProperties(t, nestedMapping(t, attributeItems, "properties"), []string{
			"key",
			"value_source",
			"value",
			"default_value",
			"rule",
			"apply_to_log",
			"apply_to_span",
			"trace_span_key",
			"as_separate_log_field",
		})

		assertBoolDefault(t, globalProperties, "use_default_attributes", false)
		assertBoolDefault(t, globalProperties, "use_default_response_attributes", false)
		assertBoolDefault(t, globalProperties, "disable_openai_usage", false)
		assertIntegerDefault(t, globalProperties, "value_length_limit", 32000)
		assertStringValue(t, nestedMapping(t, globalProperties, "path_scope"), "default", "legacy_suffixes")

		// Routing overrides inherit the initialized global values. Complete
		// rule-only configurations also expose the ordinary attribute settings.
		for _, name := range []string{"path_scope", "enable_path_suffixes", "enable_methods"} {
			raw := routeProperties[name]
			field, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("route property %s is %T, want mapping", name, raw)
			}
			assertKeyAbsent(t, field, "default")
		}
	})

	t.Run("legacy global and unfiltered bound-target examples", func(t *testing.T) {
		resource := loadYAMLMapping(t, "resources/plugins/ai-statistics/spec.yaml")
		globalExample := nestedMapping(
			t,
			resource,
			"spec",
			"configSchema",
			"openAPIV3Schema",
			"example",
		)
		routeExample := nestedMapping(
			t,
			resource,
			"spec",
			"routeConfigSchema",
			"openAPIV3Schema",
			"example",
		)

		assertStringValue(t, globalExample, "path_scope", "legacy_suffixes")
		assertStringSlice(t, globalExample, "enable_path_suffixes", []string{
			"/completions",
			"/messages",
		})
		if value := globalExample["value_length_limit"]; value != 4000 {
			t.Errorf("lightweight global example value_length_limit = %#v, want 4000", value)
		}

		if actual := sortedMappingKeys(routeExample); !reflect.DeepEqual(actual, []string{"path_scope"}) {
			t.Errorf("route example properties = %#v, want only path_scope", actual)
		}
		assertStringValue(t, routeExample, "path_scope", "bound_target")
		assertKeyAbsent(t, routeExample, "enable_path_suffixes")
		assertKeyAbsent(t, routeExample, "enable_methods")
	})

	t.Run("independent bilingual safe observability documentation", func(t *testing.T) {
		type languageContract struct {
			name             string
			extensionPath    string
			resourcePath     string
			safeHeading      string
			safeFragments    []string
			configFragments  []string
			prohibitedClaims []string
		}
		contracts := []languageContract{
			{
				name:          "Chinese",
				extensionPath: "extensions/ai-statistics/README.md",
				resourcePath:  "resources/plugins/ai-statistics/README.md",
				safeHeading:   "## 安全基础可观测契约",
				safeFragments: []string{
					"`route`", "`path`", "`method`", "`status_code`", "`latency_ms`",
					"`error_class`", "`request_count`", "`request_duration_ms`",
					"规范化且有界的 `method`", "正值 `latency_ms`",
					"与 model、逻辑 operation、token 或 usage 提取无关",
					"响应头只捕获有界状态和错误投影", "非流式响应体 EOS", "流式 EOS",
					"`wrapper.ProcessStreamDone`", "恰好一次", "同一个终态时间戳",
					"恰好三位 ASCII 数字", "`100..599`", "`status_code=0`",
					"`status_unknown`", "`none`", "`http_4xx`", "`http_5xx`",
					"`route_name`", "有效 UTF-8", "128 字节", "`route_unknown`",
					"`matched:true`", "schema v2", "`operation_template`",
					"恰好一个前导 `/`", "`/ . _ - ~ { }`",
					"`provider_native_unknown`", "`path_unknown`",
					"原始路径、query、请求头、凭据、请求体或响应体",
					"动态请求片段", "原始路径的哈希", "无界 provider 错误文本", "指标属性",
					"逻辑 `normalized_operation`", "`normalized_operation` 必须非空、不超过 128 字节",
					"不能以 `/` 开头", "`/ . _ - ~`",
					"query、fragment、authority 或控制字符", "不会作为路径模板",
					"schema v1 只能保留", "不能信任 adapter、adapter version、provider、capability 或 usage profile",
					"unknown、malformed 或 unmatched 元数据不贡献任何可选 enrichment",
					"opaque", "可选维度会省略而不是猜测",
					"`usage_missing` 是 ai-billing 的事件字段",
					"不是 ai-statistics 的事实或指标维度",
					"不会创建合成的 `usage_missing` 指标标签",
					"ai-proxy schema-v2 元数据只是可选的可信增强",
				},
				configFragments: []string{
					"插件执行优先级：`900`",
					"`use_default_attributes`",
					"`use_default_response_attributes`",
					"`32000`（普通/自定义配置）",
					"`10485760`（完整默认属性）",
					"`4000`（轻量默认响应属性）",
					"省略列表时匹配所有 Content-Type",
					"自定义属性需要；内置属性可省略",
				},
				prohibitedClaims: []string{
					"ai-statistics 输出 `usage_missing`",
					"ai-statistics 生成 `usage_missing`",
				},
			},
			{
				name:          "English",
				extensionPath: "extensions/ai-statistics/README_EN.md",
				resourcePath:  "resources/plugins/ai-statistics/README_EN.md",
				safeHeading:   "## Safe base observability contract",
				safeFragments: []string{
					"`route`", "`path`", "`method`", "`status_code`", "`latency_ms`",
					"`error_class`", "`request_count`", "`request_duration_ms`",
					"normalized and bounded `method`", "positive `latency_ms`",
					"independent of model, logical operation, token, or usage extraction",
					"response headers capture only the bounded status and error projection",
					"non-streaming response-body EOS", "streaming EOS",
					"`wrapper.ProcessStreamDone`", "exactly once", "same terminal timestamp",
					"exactly three ASCII digits", "`100..599`", "`status_code=0`",
					"`status_unknown`", "`none`", "`http_4xx`", "`http_5xx`",
					"`route_name`", "valid UTF-8", "128 bytes", "`route_unknown`",
					"`matched:true`", "schema v2", "`operation_template`",
					"exactly one leading `/`", "`/ . _ - ~ { }`",
					"`provider_native_unknown`", "`path_unknown`",
					"raw path, query, request headers, credentials, request body, or response body",
					"dynamic request segments", "hash of the raw path",
					"unbounded provider error text", "metric attributes",
					"logical `normalized_operation`", "`normalized_operation` must be non-empty, at most 128 bytes",
					"must not start with `/`", "`/ . _ - ~`",
					"query, fragment, authority, or control character", "never a path template",
					"schema v1 may retain only", "does not trust adapter, adapter version, provider, capability, or usage profile",
					"unknown, malformed, or unmatched metadata contributes no optional enrichment",
					"opaque", "optional dimensions are omitted rather than guessed",
					"`usage_missing` is an ai-billing event field",
					"not an ai-statistics fact or metric dimension",
					"does not create a synthetic `usage_missing` metric label",
					"ai-proxy schema-v2 metadata is optional trusted enrichment",
				},
				configFragments: []string{
					"Plugin Priority: `900`",
					"`use_default_attributes`",
					"`use_default_response_attributes`",
					"`32000` (plain/custom configuration)",
					"`10485760` (full default attributes)",
					"`4000` (lightweight default response attributes)",
					"an omitted list matches all content types",
					"Required for custom attributes; optional for built-in attributes",
				},
				prohibitedClaims: []string{
					"ai-statistics emits `usage_missing`",
					"ai-statistics produces `usage_missing`",
				},
			},
		}

		for _, contract := range contracts {
			contract := contract
			t.Run(contract.name, func(t *testing.T) {
				extension, err := os.ReadFile(contract.extensionPath)
				if err != nil {
					t.Fatalf("read %s: %v", contract.extensionPath, err)
				}
				resource, err := os.ReadFile(contract.resourcePath)
				if err != nil {
					t.Fatalf("read %s: %v", contract.resourcePath, err)
				}
				if string(extension) != string(resource) {
					t.Errorf("%s extension and resource documentation differ", contract.name)
				}
				body := string(extension)
				requireFragments(t, markdownSection(t, body, contract.safeHeading), contract.safeFragments)
				requireFragments(t, body, contract.configFragments)
				forbidFragments(t, body, contract.prohibitedClaims)
			})
		}
	})
}

func sortedMappingKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
