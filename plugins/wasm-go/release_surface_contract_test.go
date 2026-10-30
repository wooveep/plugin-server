package wasmgo_test

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAIRoutePluginSharedRoutingSchemaVocabulary(t *testing.T) {
	t.Parallel()

	plugins := []struct {
		name             string
		supportsProtocol bool
	}{
		{name: "ai-proxy", supportsProtocol: true},
		{name: "ai-billing"},
		{name: "ai-quota"},
		{name: "ai-statistics"},
	}

	for _, plugin := range plugins {
		plugin := plugin
		t.Run(plugin.name, func(t *testing.T) {
			t.Parallel()

			resource := loadYAMLMapping(t, "resources/plugins/"+plugin.name+"/spec.yaml")
			globalSchema := nestedMapping(t, resource, "spec", "configSchema", "openAPIV3Schema")
			routeSchema := nestedMapping(t, resource, "spec", "routeConfigSchema", "openAPIV3Schema")
			globalProperties := nestedMapping(t, globalSchema, "properties")
			routeProperties := nestedMapping(t, routeSchema, "properties")

			assertScopeSchema(t, globalProperties, "path_scope", true)
			assertScopeSchema(t, routeProperties, "path_scope", false)
			assertFilterSchema(t, globalProperties, "enable_path_suffixes", "/v1/chat/completions")
			assertFilterSchema(t, routeProperties, "enable_path_suffixes", "/v1/chat/completions")
			assertFilterSchema(t, globalProperties, "enable_methods", "POST")
			assertFilterSchema(t, routeProperties, "enable_methods", "POST")

			if plugin.supportsProtocol {
				assertProtocolSchema(t, globalProperties, true)
				assertProtocolSchema(t, routeProperties, false)
			} else {
				if _, exists := globalProperties["protocol_mode"]; exists {
					t.Error("protocol_mode must remain ai-proxy-only")
				}
				if _, exists := routeProperties["protocol_mode"]; exists {
					t.Error("route protocol_mode must remain ai-proxy-only")
				}
			}
		})
	}
}

func TestAIRoutePluginExamplesMatchRoutingVocabulary(t *testing.T) {
	t.Parallel()

	plugins := []struct {
		name             string
		legacySuffixes   []string
		supportsProtocol bool
	}{
		{name: "ai-proxy", supportsProtocol: true},
		{name: "ai-billing", legacySuffixes: []string{"/v1/chat/completions", "/v1/messages"}},
		{name: "ai-quota", legacySuffixes: []string{"/v1/chat/completions", "/v1/messages"}},
		{name: "ai-statistics", legacySuffixes: []string{"/completions", "/messages"}},
	}

	for _, plugin := range plugins {
		plugin := plugin
		t.Run(plugin.name+"/resource", func(t *testing.T) {
			t.Parallel()

			resource := loadYAMLMapping(t, "resources/plugins/"+plugin.name+"/spec.yaml")
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
			assertStringValue(t, routeExample, "path_scope", "bound_target")
			assertKeyAbsent(t, routeExample, "enable_path_suffixes")
			assertKeyAbsent(t, routeExample, "enable_methods")
			if plugin.supportsProtocol {
				assertStringValue(t, globalExample, "protocol_mode", "auto")
				assertKeyAbsent(t, routeExample, "protocol_mode")
			}
			if plugin.legacySuffixes != nil {
				assertStringSlice(t, globalExample, "enable_path_suffixes", plugin.legacySuffixes)
			}
			assertExampleMethodsUppercase(t, globalExample)
			assertExampleMethodsUppercase(t, routeExample)
		})
	}

	for _, plugin := range []struct {
		name           string
		legacySuffixes []string
	}{
		{name: "ai-billing", legacySuffixes: []string{"/v1/chat/completions", "/v1/messages"}},
		{name: "ai-quota", legacySuffixes: []string{"/v1/chat/completions", "/v1/messages"}},
	} {
		plugin := plugin
		t.Run(plugin.name+"/extension", func(t *testing.T) {
			t.Parallel()

			manifest := loadYAMLMapping(t, "extensions/"+plugin.name+"/plugin.yaml")
			defaultConfig := nestedMapping(t, manifest, "spec", "defaultConfig")
			assertStringValue(t, defaultConfig, "path_scope", "legacy_suffixes")
			if suffixes, exists := defaultConfig["enable_path_suffixes"]; exists {
				assertStringSliceValue(t, suffixes, plugin.legacySuffixes)
			}
			assertExampleMethodsUppercase(t, defaultConfig)

			rules := nestedSlice(t, manifest, "spec", "matchRules")
			foundBoundTarget := false
			for i, rawRule := range rules {
				rule, ok := rawRule.(map[string]any)
				if !ok {
					t.Fatalf("matchRules[%d] is %T, want mapping", i, rawRule)
				}
				config, ok := rule["config"].(map[string]any)
				if !ok {
					t.Fatalf("matchRules[%d].config is %T, want mapping", i, rule["config"])
				}
				assertExampleMethodsUppercase(t, config)
				if config["path_scope"] != "bound_target" {
					continue
				}
				foundBoundTarget = true
				assertKeyAbsent(t, config, "enable_path_suffixes")
				assertKeyAbsent(t, config, "enable_methods")
			}
			if !foundBoundTarget {
				t.Error("extension example must include an unfiltered bound_target rule")
			}
		})
	}

	t.Run("ai-proxy/extension", func(t *testing.T) {
		t.Parallel()

		example := loadGoAnnotationExample(t, "extensions/ai-proxy/config/config.go")
		assertStringValue(t, example, "protocol_mode", "auto")
		assertStringValue(t, example, "path_scope", "legacy_suffixes")
		assertExampleMethodsUppercase(t, example)
	})
}

func assertScopeSchema(t *testing.T, properties map[string]any, field string, global bool) {
	t.Helper()

	schema := nestedMapping(t, properties, field)
	assertStringValue(t, schema, "type", "string")
	assertStringSlice(t, schema, "enum", []string{"legacy_suffixes", "bound_target"})
	if global {
		assertStringValue(t, schema, "default", "legacy_suffixes")
		return
	}
	assertKeyAbsent(t, schema, "default")
}

func assertProtocolSchema(t *testing.T, properties map[string]any, global bool) {
	t.Helper()

	schema := nestedMapping(t, properties, "protocol_mode")
	assertStringValue(t, schema, "type", "string")
	assertStringSlice(t, schema, "enum", []string{"auto", "passthrough"})
	if global {
		assertStringValue(t, schema, "default", "auto")
		return
	}
	assertKeyAbsent(t, schema, "default")
}

func assertFilterSchema(t *testing.T, properties map[string]any, field string, validValue string) {
	t.Helper()

	schema := nestedMapping(t, properties, field)
	assertStringValue(t, schema, "type", "array")
	assertKeyAbsent(t, schema, "default")
	if minItems, exists := schema["minItems"]; exists {
		value, ok := minItems.(int)
		if !ok || value > 0 {
			t.Errorf("%s.minItems = %#v, want absent or zero so literal [] is valid", field, minItems)
		}
	}

	items := nestedMapping(t, schema, "items")
	assertStringValue(t, items, "type", "string")
	patternValue, exists := items["pattern"]
	if !exists {
		t.Errorf("%s.items.pattern is absent; blank-only entries would pass the schema", field)
		return
	}
	pattern, ok := patternValue.(string)
	if !ok {
		t.Errorf("%s.items.pattern is %T, want string", field, patternValue)
		return
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		t.Errorf("%s.items.pattern %q does not compile: %v", field, pattern, err)
		return
	}
	for _, blank := range []string{"", " ", "\t", " \t "} {
		if compiled.MatchString(blank) {
			t.Errorf("%s.items.pattern %q accepts blank-only value %q", field, pattern, blank)
		}
	}
	if !compiled.MatchString(validValue) {
		t.Errorf("%s.items.pattern %q rejects valid value %q", field, pattern, validValue)
	}
}

func loadYAMLMapping(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var value map[string]any
	if err := yaml.Unmarshal(data, &value); err != nil {
		t.Fatalf("parse %s as YAML: %v", path, err)
	}
	return value
}

func loadGoAnnotationExample(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "// @Example" {
			continue
		}
		for _, exampleLine := range lines[i+1:] {
			trimmed := strings.TrimSpace(exampleLine)
			if trimmed == "// @End" {
				break
			}
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
			if trimmed == "" {
				continue
			}
			var value map[string]any
			if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
				t.Fatalf("parse %s @Example as JSON: %v", path, err)
			}
			return value
		}
	}
	t.Fatalf("%s has no JSON @Example annotation", path)
	return nil
}

func nestedMapping(t *testing.T, root map[string]any, path ...string) map[string]any {
	t.Helper()

	current := root
	for _, key := range path {
		raw, exists := current[key]
		if !exists {
			t.Fatalf("%s is absent", strings.Join(path, "."))
		}
		next, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("%s is %T, want mapping", strings.Join(path, "."), raw)
		}
		current = next
	}
	return current
}

func nestedSlice(t *testing.T, root map[string]any, path ...string) []any {
	t.Helper()

	current := root
	for _, key := range path[:len(path)-1] {
		current = nestedMapping(t, current, key)
	}
	raw, exists := current[path[len(path)-1]]
	if !exists {
		t.Fatalf("%s is absent", strings.Join(path, "."))
	}
	value, ok := raw.([]any)
	if !ok {
		t.Fatalf("%s is %T, want array", strings.Join(path, "."), raw)
	}
	return value
}

func assertStringValue(t *testing.T, values map[string]any, key string, expected string) {
	t.Helper()

	actual, exists := values[key]
	if !exists {
		t.Errorf("%s is absent, want %q", key, expected)
		return
	}
	if actual != expected {
		t.Errorf("%s = %#v, want %q", key, actual, expected)
	}
}

func assertStringSlice(t *testing.T, values map[string]any, key string, expected []string) {
	t.Helper()

	raw, exists := values[key]
	if !exists {
		t.Errorf("%s is absent, want %#v", key, expected)
		return
	}
	assertStringSliceValue(t, raw, expected)
}

func assertStringSliceValue(t *testing.T, raw any, expected []string) {
	t.Helper()

	items, ok := raw.([]any)
	if !ok {
		t.Errorf("value is %T, want array %#v", raw, expected)
		return
	}
	actual := make([]string, len(items))
	for i, item := range items {
		value, ok := item.(string)
		if !ok {
			t.Errorf("array item %d is %T, want string", i, item)
			return
		}
		actual[i] = value
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("array = %#v, want %#v", actual, expected)
	}
}

func assertKeyAbsent(t *testing.T, values map[string]any, key string) {
	t.Helper()

	if value, exists := values[key]; exists {
		t.Errorf("%s = %#v, want absent", key, value)
	}
}

func assertExampleMethodsUppercase(t *testing.T, values map[string]any) {
	t.Helper()

	raw, exists := values["enable_methods"]
	if !exists {
		return
	}
	items, ok := raw.([]any)
	if !ok {
		t.Errorf("enable_methods is %T, want array", raw)
		return
	}
	for i, item := range items {
		method, ok := item.(string)
		if !ok {
			t.Errorf("enable_methods[%d] is %T, want string", i, item)
			continue
		}
		if method != strings.ToUpper(method) {
			t.Errorf("enable_methods[%d] = %q, want canonical uppercase", i, method)
		}
	}
}
