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
		extensionReadme := quotaSurfaceRead(t, extensionRoot, "README.md")
		extensionReadmeEN := quotaSurfaceRead(t, extensionRoot, "README_EN.md")
		pluginText := quotaSurfaceRead(t, extensionRoot, "plugin.yaml")
		resourceReadme := quotaSurfaceRead(t, resourceRoot, "README.md")
		resourceReadmeEN := quotaSurfaceRead(t, resourceRoot, "README_EN.md")
		specText := quotaSurfaceRead(t, resourceRoot, "spec.yaml")
		spec := quotaSurfaceYAML(t, specText)
		plugin := quotaSurfaceYAML(t, pluginText)

		t.Run("schema exposes only admission configuration", func(t *testing.T) {
			globalSchema := quotaSurfaceMap(t, spec, "spec", "configSchema", "openAPIV3Schema")
			routeSchema := quotaSurfaceMap(t, spec, "spec", "routeConfigSchema", "openAPIV3Schema")
			globalProperties := quotaSurfaceMap(t, globalSchema, "properties")
			routeProperties := quotaSurfaceMap(t, routeSchema, "properties")

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
			require.Equal(t, PathScopeLegacySuffixes, globalExample["path_scope"])
			require.Contains(t, globalExample, "enable_path_suffixes")
			require.Equal(t, PathScopeBoundTarget, routeExample["path_scope"])
			require.NotContains(t, routeExample, "enable_path_suffixes")
			require.NotContains(t, routeExample, "enable_methods")

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
			for label, text := range map[string]string{
				"extension README.md":    extensionReadme,
				"extension README_EN.md": extensionReadmeEN,
				"resource README.md":     resourceReadme,
				"resource README_EN.md":  resourceReadmeEN,
			} {
				normalized := strings.ToLower(strings.Join(strings.Fields(text), " "))
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
