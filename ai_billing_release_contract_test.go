package wasmgo_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAIBillingReleaseSchemaAndDocumentationContract(t *testing.T) {
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
	requireFragments := func(t *testing.T, body string, fragments []string) {
		t.Helper()
		for _, fragment := range fragments {
			if !strings.Contains(strings.ToLower(body), strings.ToLower(fragment)) {
				t.Errorf("missing required contract fragment %q", fragment)
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
  required: [service_name]
  properties:
    service_name:
      type: string
    timeout:
      type: integer
      default: 500
`), &fixture)
		if err != nil {
			t.Fatalf("parse fixture YAML: %v", err)
		}
		schema := nestedMapping(t, fixture, "schema")
		assertClosedObject(t, schema, "schema")
		assertStringSlice(t, schema, "required", []string{"service_name"})
		properties := nestedMapping(t, schema, "properties")
		assertExactProperties(t, properties, []string{"service_name", "timeout"})
		assertIntegerDefault(t, properties, "timeout", 500)
	})

	t.Run("closed schema and supported fields", func(t *testing.T) {
		resource := loadYAMLMapping(t, "resources/plugins/ai-billing/spec.yaml")
		globalSchema := nestedMapping(t, resource, "spec", "configSchema", "openAPIV3Schema")
		routeSchema := nestedMapping(t, resource, "spec", "routeConfigSchema", "openAPIV3Schema")
		globalProperties := nestedMapping(t, globalSchema, "properties")
		routeProperties := nestedMapping(t, routeSchema, "properties")

		assertClosedObject(t, globalSchema, "global schema")
		assertClosedObject(t, routeSchema, "route schema")
		assertKeyAbsent(t, globalSchema, "required")
		assertExactProperties(t, globalProperties, []string{
			"redis_stream",
			"event_kind",
			"quota_scope",
			"provider",
			"cost_source",
			"worker_kind",
			"bill_customer",
			"tenant_header",
			"consumer_header",
			"path_scope",
			"enable_path_suffixes",
			"enable_methods",
			"fail_policy",
		})
		assertExactProperties(t, routeProperties, []string{
			"redis_stream",
			"event_kind",
			"quota_scope",
			"provider",
			"cost_source",
			"worker_kind",
			"bill_customer",
			"tenant_header",
			"consumer_header",
			"path_scope",
			"enable_path_suffixes",
			"enable_methods",
			"fail_policy",
		})
		assertKeyAbsent(t, globalProperties, "billing_service")
		assertKeyAbsent(t, routeProperties, "billing_service")

		for name, properties := range map[string]map[string]any{
			"global redis_stream": globalProperties,
			"route redis_stream":  routeProperties,
		} {
			redisSchema := nestedMapping(t, properties, "redis_stream")
			redisProperties := nestedMapping(t, redisSchema, "properties")
			assertClosedObject(t, redisSchema, name)
			assertStringSlice(t, redisSchema, "required", []string{"service_name"})
			assertExactProperties(t, redisProperties, []string{
				"service_name",
				"service_port",
				"username",
				"password",
				"database",
				"timeout",
				"stream",
			})
			assertKeyAbsent(t, nestedMapping(t, redisProperties, "service_name"), "default")
			assertIntegerDefault(t, redisProperties, "service_port", 6379)
			assertIntegerDefault(t, redisProperties, "database", 0)
			assertIntegerDefault(t, redisProperties, "timeout", 500)
			assertStringValue(t, nestedMapping(t, redisProperties, "stream"), "default", "billing:events")
		}

		for _, properties := range []map[string]any{globalProperties, routeProperties} {
			eventKind := nestedMapping(t, properties, "event_kind")
			assertStringSlice(t, eventKind, "enum", []string{"usage", "internal_cost", "customer_usage"})
			requireFragments(t, eventKind["description"].(string), []string{"usage", "internal_cost", "customer_usage"})
			eventKindI18n := nestedMapping(t, eventKind, "x-description-i18n")
			requireFragments(t, eventKindI18n["en-US"].(string), []string{"usage", "internal_cost", "customer_usage"})
			assertStringValue(t, nestedMapping(t, properties, "bill_customer"), "type", "boolean")
		}
		assertStringValue(t, nestedMapping(t, globalProperties, "event_kind"), "default", "usage")
		for name, raw := range routeProperties {
			field, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("route property %s is %T, want mapping", name, raw)
			}
			assertKeyAbsent(t, field, "default")
		}
	})

	t.Run("global Redis and route examples", func(t *testing.T) {
		resource := loadYAMLMapping(t, "resources/plugins/ai-billing/spec.yaml")
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
		resourceRedis := nestedMapping(t, globalExample, "redis_stream")
		assertStringValue(t, resourceRedis, "username", "<redis-username>")
		assertStringValue(t, resourceRedis, "password", "<redis-password>")
		assertStringValue(t, globalExample, "event_kind", "usage")
		assertStringValue(t, routeExample, "event_kind", "internal_cost")
		assertStringValue(t, routeExample, "path_scope", "bound_target")
		assertKeyAbsent(t, routeExample, "enable_path_suffixes")
		assertKeyAbsent(t, routeExample, "enable_methods")
		routeRedis := nestedMapping(t, routeExample, "redis_stream")
		assertStringValue(t, routeRedis, "service_name", "redis-stack-server.dns")
		assertStringValue(t, routeRedis, "username", "<redis-username>")
		assertStringValue(t, routeRedis, "password", "<redis-password>")
		assertKeyAbsent(t, routeExample, "billing_service")
		for _, name := range []string{"cost_source", "worker_kind"} {
			value, exists := routeExample[name]
			if !exists || strings.TrimSpace(value.(string)) == "" {
				t.Errorf("route example %s = %#v, want non-empty internal attribution", name, value)
			}
		}
		if value, exists := routeExample["bill_customer"]; !exists || value != false {
			t.Errorf("route example bill_customer = %#v, want false", value)
		}

		manifest := loadYAMLMapping(t, "extensions/ai-billing/plugin.yaml")
		defaultConfig := nestedMapping(t, manifest, "spec", "defaultConfig")
		manifestRedis := nestedMapping(t, defaultConfig, "redis_stream")
		assertStringValue(t, manifestRedis, "username", "<redis-username>")
		assertStringValue(t, manifestRedis, "password", "<redis-password>")
		assertStringValue(t, defaultConfig, "event_kind", "usage")
		assertStringSlice(t, defaultConfig, "enable_path_suffixes", []string{
			"/v1/chat/completions",
			"/v1/messages",
		})

		foundUnfilteredBoundTarget := false
		foundInternalCost := false
		for i, rawRule := range nestedSlice(t, manifest, "spec", "matchRules") {
			rule, ok := rawRule.(map[string]any)
			if !ok {
				t.Fatalf("matchRules[%d] is %T, want mapping", i, rawRule)
			}
			config := nestedMapping(t, rule, "config")
			assertKeyAbsent(t, config, "redis_stream")
			assertKeyAbsent(t, config, "billing_service")
			if config["path_scope"] == "bound_target" {
				foundUnfilteredBoundTarget = true
				assertKeyAbsent(t, config, "enable_path_suffixes")
				assertKeyAbsent(t, config, "enable_methods")
			}
			if config["event_kind"] == "internal_cost" {
				foundInternalCost = true
				for _, name := range []string{"cost_source", "worker_kind"} {
					value, exists := config[name]
					if !exists || strings.TrimSpace(value.(string)) == "" {
						t.Errorf("internal-cost rule %s = %#v, want non-empty attribution", name, value)
					}
				}
				if value, exists := config["bill_customer"]; !exists || value != false {
					t.Errorf("internal-cost rule bill_customer = %#v, want false", value)
				}
			}
		}
		if !foundUnfilteredBoundTarget {
			t.Error("plugin example has no unfiltered bound_target rule")
		}
		if !foundInternalCost {
			t.Error("plugin example has no internal_cost rule")
		}
	})

	t.Run("independent bilingual event trust correlation and migration docs", func(t *testing.T) {
		type languageContract struct {
			name                string
			extensionPath       string
			resourcePath        string
			eventHeading        string
			trustHeading        string
			correlationHeading  string
			configHeading       string
			independenceHeading string
			eventFragments      []string
			trustFragments      []string
			correlationParts    []string
			configFragments     []string
			independenceParts   []string
		}
		contracts := []languageContract{
			{
				name:                "Chinese",
				extensionPath:       "extensions/ai-billing/README.md",
				resourcePath:        "resources/plugins/ai-billing/README.md",
				eventHeading:        "## 账单事件契约",
				trustHeading:        "## 可信内部成本归因",
				correlationHeading:  "## 响应关联契约",
				configHeading:       "## 配置与迁移",
				independenceHeading: "## 独立部署边界",
				eventFragments: []string{
					"`event_id`", "`event_kind`", "`cost_source`", "`worker_kind`",
					"`source_job`", "`bill_customer`", "`idempotency_key`", "`request_id`",
					"`tenant`", "`consumer`", "`route`", "`provider`", "`quota_scope`",
					"`model`", "`request_path`", "`status_code`", "`usage`",
					"`usage_missing`", "`usage_source`", "`start_time_ms`", "`end_time_ms`",
					"`is_stream`", "`cluster`", "`price_version`", "`upstream_invoked`",
					"`customer_usage`", "兼容别名", "归一化为 `usage`",
					"普通 `usage` 事件会省略", "可信 provider usage", "安全估算",
					"`usage_source=estimated` 和 `usage_missing=false`",
					"`usage_source=missing`", "不会虚构 model、provider usage、price、cost 或 settlement",
					"可信网关生命周期事实 `upstream_invoked`",
					"请求、响应或用户 payload 不能伪造",
					"不提供顶层 `input_tokens`", "不包含原始 API key 或真实 Redis 凭据",
				},
				trustFragments: []string{
					"仅已配置的 `internal_cost`", "完整的一对", "覆盖已配置的回退值",
					"两者都缺失", "部分、空、超长、非法字符或重复", "只抑制 billing event",
					"模型流量继续", "接受和拒绝的提交值都不会写入日志",
					"客户路由忽略", "不能启用 `internal_cost`", "不能设置 `bill_customer`",
				},
				correlationParts: []string{
					"`x-ocmf-gateway-request-id`", "替换而不是追加", "事件中的 `request_id`",
					"非流式", "流式", "网关本地", "缓存",
					"流式结束不会追加第二个 `x-ocmf-gateway-request-id` 值",
					"缺少规范请求上下文", "删除上游值且不会虚构", "不能证明 Redis 已持久化",
				},
				configFragments: []string{
					"全局投递目标", "`redis_stream.service_name`", "`6379`", "`0`", "`500`",
					"`billing:events`", "`<redis-username>`", "`<redis-password>`",
					"全局和规则级 `billing_service` 都会被拒绝",
					"规则级 `redis_stream` 仅可用于休眠的空基础配置",
					"`defaultConfigDisable: true`", "继承全局值", "精确交集",
					"`/v1/chat/completions`", "`/v1/messages`", "`/v1/responses`", "`POST`",
				},
				independenceParts: []string{
					"`ai-billing`、`ai-quota` 和 `ai-statistics` 可以独立部署",
					"不依赖或共享彼此的私有运行时状态",
				},
			},
			{
				name:                "English",
				extensionPath:       "extensions/ai-billing/README_EN.md",
				resourcePath:        "resources/plugins/ai-billing/README_EN.md",
				eventHeading:        "## Billing event contract",
				trustHeading:        "## Trusted internal-cost attribution",
				correlationHeading:  "## Response correlation contract",
				configHeading:       "## Configuration and migration",
				independenceHeading: "## Independent deployment boundary",
				eventFragments: []string{
					"`event_id`", "`event_kind`", "`cost_source`", "`worker_kind`",
					"`source_job`", "`bill_customer`", "`idempotency_key`", "`request_id`",
					"`tenant`", "`consumer`", "`route`", "`provider`", "`quota_scope`",
					"`model`", "`request_path`", "`status_code`", "`usage`",
					"`usage_missing`", "`usage_source`", "`start_time_ms`", "`end_time_ms`",
					"`is_stream`", "`cluster`", "`price_version`", "`upstream_invoked`",
					"`customer_usage`", "compatibility alias", "normalizes to `usage`",
					"ordinary `usage` events omit", "trustworthy provider usage", "safe estimation",
					"`usage_source=estimated` with `usage_missing=false`",
					"`usage_source=missing`", "does not fabricate model, provider usage, price, cost, or settlement",
					"trusted gateway lifecycle fact `upstream_invoked`",
					"request, response, or user payload cannot spoof",
					"no top-level `input_tokens`", "no raw API keys or real Redis credentials",
				},
				trustFragments: []string{
					"only configured `internal_cost`", "complete pair", "overrides the configured fallback",
					"both are absent", "partial, empty, oversized, invalid-character, or duplicate",
					"suppresses only billing-event delivery", "model traffic continues",
					"accepted and rejected submitted values are never logged", "customer routes ignore",
					"cannot enable `internal_cost`", "cannot set `bill_customer`",
				},
				correlationParts: []string{
					"`x-ocmf-gateway-request-id`", "replaces rather than appends",
					"event `request_id`", "non-streaming", "streaming", "gateway-local", "cache",
					"stream completion does not append a second `x-ocmf-gateway-request-id` value",
					"canonical request context is missing",
					"removes the upstream value without inventing one", "does not prove Redis persistence",
				},
				configFragments: []string{
					"a global delivery target", "`redis_stream.service_name`", "`6379`", "`0`", "`500`",
					"`billing:events`", "`<redis-username>`", "`<redis-password>`",
					"global and rule-level `billing_service` are rejected",
					"rule-level `redis_stream` is accepted only with a dormant empty base",
					"`defaultConfigDisable: true`", "inherit global values", "exact intersection",
					"`/v1/chat/completions`", "`/v1/messages`", "`/v1/responses`", "`POST`",
				},
				independenceParts: []string{
					"`ai-billing`, `ai-quota`, and `ai-statistics` can be deployed independently",
					"do not depend on or share one another's private runtime state",
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
				requireFragments(t, markdownSection(t, body, contract.eventHeading), contract.eventFragments)
				requireFragments(t, markdownSection(t, body, contract.trustHeading), contract.trustFragments)
				requireFragments(t, markdownSection(t, body, contract.correlationHeading), contract.correlationParts)
				requireFragments(t, markdownSection(t, body, contract.configHeading), contract.configFragments)
				requireFragments(t, markdownSection(t, body, contract.independenceHeading), contract.independenceParts)
			})
		}
	})
}
