package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var removedQuotaSettlementFields = []string{
	"price_key_template",
	"amount_scale",
	"price_unit_tokens",
	"missing_price_policy",
	"missing_usage_policy",
	"input_token_price",
	"output_token_price",
	"input_price",
	"output_price",
}

func TestQuotaAdmissionSurfaceResources(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		extensionRoot, resourceRoot := quotaSurfaceRoots(t)
		repositoryRoot := filepath.Clean(filepath.Join(extensionRoot, "..", ".."))
		extensionReadme := quotaSurfaceRead(t, extensionRoot, "README.md")
		extensionReadmeEN := quotaSurfaceRead(t, extensionRoot, "README_EN.md")
		pluginText := quotaSurfaceRead(t, extensionRoot, "plugin.yaml")
		resourceReadme := quotaSurfaceRead(t, resourceRoot, "README.md")
		resourceReadmeEN := quotaSurfaceRead(t, resourceRoot, "README_EN.md")
		currentQuotaDoc := quotaSurfaceRead(t, repositoryRoot, "docs/plugins/ai/api-consumer/ai-quota.md")
		jointRequirementsDoc := quotaSurfaceRead(t, repositoryRoot, "docs/plugins/ai/api-consumer/ai-quota-ai-billing-requirements.md")
		consoleBillingDoc := quotaSurfaceRead(t, repositoryRoot, "docs/plugins/ai/api-consumer/console-billing-service-api-design.md")
		specText := quotaSurfaceRead(t, resourceRoot, "spec.yaml")
		spec := quotaSurfaceYAML(t, specText)
		plugin := quotaSurfaceYAML(t, pluginText)

		t.Run("schema exposes only admission configuration", func(t *testing.T) {
			globalSchema := quotaSurfaceMap(t, spec, "spec", "configSchema", "openAPIV3Schema")
			routeSchema := quotaSurfaceMap(t, spec, "spec", "routeConfigSchema", "openAPIV3Schema")
			globalProperties := quotaSurfaceMap(t, globalSchema, "properties")
			routeProperties := quotaSurfaceMap(t, routeSchema, "properties")
			redisSchema := quotaSurfaceMap(t, globalProperties, "redis")
			redisProperties := quotaSurfaceMap(t, redisSchema, "properties")

			require.ElementsMatch(t, []string{
				"redis",
				"quota_scope",
				"provider",
				"tenant_header",
				"consumer_header",
				"balance_key_template",
				"path_scope",
				"enable_path_suffixes",
				"enable_methods",
				"missing_balance_policy",
			}, quotaSurfaceKeys(globalProperties))
			require.ElementsMatch(t, []string{
				"quota_scope",
				"provider",
				"path_scope",
				"enable_path_suffixes",
				"enable_methods",
			}, quotaSurfaceKeys(routeProperties))
			require.Equal(t, false, globalSchema["additionalProperties"])
			require.Equal(t, false, routeSchema["additionalProperties"])
			require.Equal(t, false, redisSchema["additionalProperties"])
			require.Equal(t, []interface{}{"redis"}, globalSchema["required"])
			require.Equal(t, []interface{}{"service_name"}, redisSchema["required"])
			require.ElementsMatch(t, []string{
				"service_name",
				"service_port",
				"username",
				"password",
				"timeout",
				"database",
			}, quotaSurfaceKeys(redisProperties))
			require.Equal(t, 6379, quotaSurfaceMap(t, redisProperties, "service_port")["default"])
			require.Equal(t, 1000, quotaSurfaceMap(t, redisProperties, "timeout")["default"])
			require.Equal(t, 0, quotaSurfaceMap(t, redisProperties, "database")["default"])
			require.Equal(t, "global", quotaSurfaceMap(t, globalProperties, "quota_scope")["default"])
			require.Equal(t, "default", quotaSurfaceMap(t, globalProperties, "provider")["default"])
			require.Equal(t, "x-mse-tenant", quotaSurfaceMap(t, globalProperties, "tenant_header")["default"])
			require.Equal(t, "x-mse-consumer", quotaSurfaceMap(t, globalProperties, "consumer_header")["default"])
			require.Equal(t, "billing:balance:{tenant}:{quota_scope}:{consumer}", quotaSurfaceMap(t, globalProperties, "balance_key_template")["default"])
			require.Equal(t, PathScopeLegacySuffixes, quotaSurfaceMap(t, globalProperties, "path_scope")["default"])
			require.Equal(t, "deny", quotaSurfaceMap(t, globalProperties, "missing_balance_policy")["default"])
			for name, field := range routeProperties {
				fieldMap, ok := field.(map[string]interface{})
				require.Truef(t, ok, "route field %q must be an object", name)
				require.NotContainsf(t, fieldMap, "default", "route field %q must inherit rather than publish a default", name)
			}
			for label, field := range map[string]interface{}{
				"global suffixes": globalProperties["enable_path_suffixes"],
				"global methods":  globalProperties["enable_methods"],
				"route suffixes":  routeProperties["enable_path_suffixes"],
				"route methods":   routeProperties["enable_methods"],
			} {
				fieldMap, ok := field.(map[string]interface{})
				require.Truef(t, ok, "%s must be an object", label)
				items := quotaSurfaceMap(t, fieldMap, "items")
				require.Equalf(t, `.*\S.*`, items["pattern"], "%s must reject blank-only entries", label)
			}
			quotaSurfaceRequireEnum(t, globalProperties["path_scope"], []string{
				PathScopeLegacySuffixes,
				PathScopeBoundTarget,
			})
			quotaSurfaceRequireEnum(t, routeProperties["path_scope"], []string{
				PathScopeLegacySuffixes,
				PathScopeBoundTarget,
			})
		})

		t.Run("examples preserve legacy scope and show unfiltered bound target", func(t *testing.T) {
			globalExample := quotaSurfaceMap(t, spec, "spec", "configSchema", "openAPIV3Schema", "example")
			routeExample := quotaSurfaceMap(t, spec, "spec", "routeConfigSchema", "openAPIV3Schema", "example")
			redisExample := quotaSurfaceMap(t, globalExample, "redis")
			require.Equal(t, PathScopeLegacySuffixes, globalExample["path_scope"])
			require.Equal(t, []interface{}{"/v1/chat/completions", "/v1/messages"}, globalExample["enable_path_suffixes"])
			require.Equal(t, "<redis-username>", redisExample["username"])
			require.Equal(t, "<redis-password>", redisExample["password"])
			require.Equal(t, 6379, redisExample["service_port"])
			require.Equal(t, 1000, redisExample["timeout"])
			require.Equal(t, 0, redisExample["database"])
			require.Equal(t, PathScopeBoundTarget, routeExample["path_scope"])
			require.NotContains(t, routeExample, "enable_path_suffixes")
			require.NotContains(t, routeExample, "enable_methods")
			for _, inherited := range []string{
				"redis",
				"tenant_header",
				"consumer_header",
				"balance_key_template",
				"missing_balance_policy",
			} {
				require.NotContainsf(t, routeExample, inherited, "route example must inherit %q", inherited)
			}

			defaultConfig := quotaSurfaceMap(t, plugin, "spec", "defaultConfig")
			require.Equal(t, PathScopeLegacySuffixes, defaultConfig["path_scope"])
			matchRules, ok := quotaSurfaceMap(t, plugin, "spec")["matchRules"].([]interface{})
			require.True(t, ok)
			require.NotEmpty(t, matchRules)
			foundBoundTarget := false
			for _, rawRule := range matchRules {
				rule, ok := rawRule.(map[string]interface{})
				require.True(t, ok)
				rawConfig, ok := rule["config"]
				if !ok {
					continue
				}
				config, ok := rawConfig.(map[string]interface{})
				require.True(t, ok)
				if config["path_scope"] == PathScopeBoundTarget {
					foundBoundTarget = true
					require.NotContains(t, config, "enable_path_suffixes")
					require.NotContains(t, config, "enable_methods")
				}
			}
			require.True(t, foundBoundTarget, "plugin example must include a bound-target route without synthesized filters")
		})

		t.Run("bilingual docs describe the final read-only admission action", func(t *testing.T) {
			for label, fixture := range map[string]struct {
				text          string
				versionMarker string
			}{
				"extension README.md": {
					text:          extensionReadme,
					versionMarker: "> 插件版本：`1.0.2`",
				},
				"extension README_EN.md": {
					text:          extensionReadmeEN,
					versionMarker: "> Plugin version: `1.0.2`",
				},
				"resource README.md": {
					text:          resourceReadme,
					versionMarker: "> 插件版本：`1.0.2`",
				},
				"resource README_EN.md": {
					text:          resourceReadmeEN,
					versionMarker: "> Plugin version: `1.0.2`",
				},
			} {
				normalized := strings.ToLower(strings.Join(strings.Fields(fixture.text), " "))
				require.Equalf(t, 1, strings.Count(fixture.text, fixture.versionMarker), "%s must identify release 1.0.2 exactly once", label)
				require.Contains(t, normalized, "ai-quota", label)
				require.Contains(t, normalized, "legacy_suffixes", label)
				require.Contains(t, normalized, "bound_target", label)
				require.True(
					t,
					strings.Contains(normalized, "intersection") || strings.Contains(normalized, "交集"),
					"%s must describe advanced-filter intersection semantics",
					label,
				)
				require.True(
					t,
					strings.Contains(normalized, "read-only") || strings.Contains(normalized, "只读"),
					"%s must describe Redis balance input as read-only",
					label,
				)
				require.True(
					t,
					strings.Contains(normalized, "final account enforcement action") ||
						strings.Contains(normalized, "最终账户执法动作"),
					"%s must identify request admission as the final account enforcement action",
					label,
				)
				require.True(
					t,
					strings.Contains(normalized, "greater than `0`") ||
						strings.Contains(normalized, "大于 `0`"),
					"%s must document the strict positive-balance boundary",
					label,
				)
				require.True(
					t,
					strings.Contains(normalized, "zero and negative balances") ||
						strings.Contains(normalized, "零余额和负余额"),
					"%s must document unconditional zero/negative denial",
					label,
				)
				require.True(
					t,
					strings.Contains(normalized, "missing key follows") ||
						strings.Contains(normalized, "余额 key 不存在时由"),
					"%s must apply policy only to a missing balance key",
					label,
				)
				require.True(
					t,
					strings.Contains(normalized, "does not mutate account state") ||
						strings.Contains(normalized, "不修改账户状态"),
					"%s must forbid account mutation",
					label,
				)
				require.True(
					t,
					strings.Contains(normalized, "recognizable, opaque, streaming, cache, local, error, and missing-usage responses") ||
						strings.Contains(normalized, "可识别、不透明、流式、缓存、本地、错误和 usage 缺失响应"),
					"%s must cover every response class without inspecting response facts",
					label,
				)
				require.True(
					t,
					strings.Contains(normalized, "do not trigger another quota decision") ||
						strings.Contains(normalized, "不会触发第二次配额判断"),
					"%s must forbid a second quota decision",
					label,
				)
				for _, ownerTerm := range []struct {
					english string
					chinese string
				}{
					{english: "authoritative accounts", chinese: "权威账户"},
					{english: "settlement", chinese: "结算"},
					{english: "statements", chinese: "账单"},
					{english: "reconciliation", chinese: "对账"},
					{english: "rebuild", chinese: "重建"},
					{english: "private runtime state", chinese: "私有运行时状态"},
				} {
					require.Truef(
						t,
						strings.Contains(normalized, ownerTerm.english) ||
							strings.Contains(normalized, ownerTerm.chinese),
						"%s must document account ownership term %q/%q",
						label,
						ownerTerm.english,
						ownerTerm.chinese,
					)
				}
				require.NotContains(t, normalized, "greater than or equal to zero", label)
				require.NotContains(t, normalized, "大于等于 0", label)
			}
		})

		t.Run("current schema examples and docs contain no removed settlement surface", func(t *testing.T) {
			for label, text := range map[string]string{
				"resource spec.yaml":     specText,
				"extension plugin.yaml":  pluginText,
				"extension README.md":    extensionReadme,
				"extension README_EN.md": extensionReadmeEN,
				"resource README.md":     resourceReadme,
				"resource README_EN.md":  resourceReadmeEN,
			} {
				normalized := strings.ToLower(text)
				for _, removed := range removedQuotaSettlementFields {
					require.NotContainsf(t, normalized, removed, "%s exposes removed field %q", label, removed)
				}
				for _, removedWording := range []string{
					"effective price key",
					"post-response",
					"after response",
					"响应期",
					"响应结束后",
				} {
					require.NotContainsf(t, normalized, removedWording, "%s retains removed response-settlement wording %q", label, removedWording)
				}
			}
		})

		t.Run("current quota docs contain no response settlement contract", func(t *testing.T) {
			normalized := strings.ToLower(strings.Join(strings.Fields(currentQuotaDoc), " "))
			for _, removed := range removedQuotaSettlementFields {
				require.NotContainsf(t, normalized, removed, "current ai-quota reference exposes removed field %q", removed)
			}
			for _, staleClaim := range []string{
				"响应后扣费",
				"响应完成后，插件",
				"pkg/tokenusage",
				"redis lua",
				"`eval`",
				"输入、输出 token 价格 key",
				"费用计算",
				"扣减余额",
			} {
				require.NotContainsf(t, normalized, staleClaim, "current ai-quota reference retains stale claim %q", staleClaim)
			}
			for _, required := range []string{
				"只读",
				"请求准入",
				"最终账户执法动作",
				"console",
				"billing-service",
				"结算",
				"bound_target",
				"交集",
			} {
				require.Containsf(t, normalized, required, "current ai-quota reference must describe %q", required)
			}
		})

		t.Run("historical joint requirements are explicitly retired", func(t *testing.T) {
			normalized := strings.ToLower(strings.Join(strings.Fields(jointRequirementsDoc), " "))
			require.Contains(t, normalized, "已废弃")
			require.Contains(t, normalized, "superseded")
			require.Contains(t, normalized, "ai-quota")
			require.Contains(t, normalized, "只读")
			require.Contains(t, normalized, "请求准入")
			require.Contains(t, normalized, "billing-service")
			require.Contains(t, normalized, "结算")
			for _, staleQuotaClaim := range []string{
				"`ai-quota` 响应结束后",
				"`ai-quota` 调用 tokenusage",
				"`ai-quota` 只读取 redis 中的生效价格",
				"| `ai-quota` | 金额余额准入、响应后",
				"`ai-quota` 按金额扣减余额",
				"ai-quota 从 redis 读取 tenant/provider/model",
			} {
				require.NotContainsf(t, normalized, staleQuotaClaim, "retired requirements retain active quota claim %q", staleQuotaClaim)
			}
		})

		t.Run("console design keeps settlement ownership outside quota", func(t *testing.T) {
			normalized := strings.ToLower(strings.Join(strings.Fields(consoleBillingDoc), " "))
			for _, staleQuotaClaim := range []string{
				"ai-quota 读取热余额和热价格",
				"redis 余额只用于网关快速判断和近实时扣减",
				"`ai-quota` 不理解折扣规则，只读取 `billing:effective_price:*`",
				"网关侧扣减金额与服务端计算金额不一致",
				"price 缺失导致的网关未扣费",
			} {
				require.NotContainsf(t, normalized, staleQuotaClaim, "console design retains stale quota claim %q", staleQuotaClaim)
			}
			for _, required := range []string{
				"ai-quota",
				"只读",
				"请求准入",
				"billing-service",
				"账户余额投影",
				"结算",
				"redis 热余额刷新",
			} {
				require.Containsf(t, normalized, required, "console design must retain %q ownership", required)
			}
		})
	})
}

func quotaSurfaceRoots(t *testing.T) (string, string) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	extensionRoot := filepath.Dir(source)
	resourceRoot := filepath.Clean(filepath.Join(extensionRoot, "..", "..", "resources", "plugins", "ai-quota"))
	return extensionRoot, resourceRoot
}

func quotaSurfaceRead(t *testing.T, root, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, name))
	require.NoError(t, err)
	require.NotEmpty(t, body)
	return string(body)
}

func quotaSurfaceYAML(t *testing.T, text string) map[string]interface{} {
	t.Helper()
	var value map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(text), &value))
	return value
}

func quotaSurfaceMap(t *testing.T, root map[string]interface{}, path ...string) map[string]interface{} {
	t.Helper()
	var current interface{} = root
	for _, segment := range path {
		object, ok := current.(map[string]interface{})
		require.Truef(t, ok, "path %s must be an object", strings.Join(path, "."))
		value, ok := object[segment]
		require.Truef(t, ok, "path %s is missing %q", strings.Join(path, "."), segment)
		current = value
	}
	object, ok := current.(map[string]interface{})
	require.Truef(t, ok, "path %s must resolve to an object", strings.Join(path, "."))
	return object
}

func quotaSurfaceKeys(value map[string]interface{}) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	return keys
}

func quotaSurfaceRequireEnum(t *testing.T, field interface{}, expected []string) {
	t.Helper()
	object, ok := field.(map[string]interface{})
	require.True(t, ok)
	rawValues, ok := object["enum"].([]interface{})
	require.True(t, ok)
	values := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		value, ok := rawValue.(string)
		require.True(t, ok)
		values = append(values, value)
	}
	require.ElementsMatch(t, expected, values)
}
