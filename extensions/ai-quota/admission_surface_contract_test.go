package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Check the real published schema example against the plugin parser rather than
// requiring prose phrases or a particular Redis command spelling.
func TestQuotaAdmissionSurfaceResources(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		_, source, _, ok := runtime.Caller(0)
		require.True(t, ok)
		path := filepath.Join(filepath.Dir(source), "..", "..", "resources", "plugins", "ai-quota", "spec.yaml")
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		var resource map[string]any
		require.NoError(t, yaml.Unmarshal(raw, &resource))
		spec := resource["spec"].(map[string]any)
		schema := spec["configSchema"].(map[string]any)["openAPIV3Schema"].(map[string]any)
		require.ElementsMatch(t, []any{"redis", "balance_scope_id", "cache_generation", "projection_schema"}, schema["required"])
		properties := schema["properties"].(map[string]any)
		require.Equal(t, []any{"deny"}, properties["missing_balance_policy"].(map[string]any)["enum"])
		example := schema["example"].(map[string]any)
		data, err := json.Marshal(example)
		require.NoError(t, err)
		host, status := test.NewTestHost(data)
		require.Equal(t, types.OnPluginStartStatusOK, status)
		host.Reset()
		for _, field := range []string{"balance_scope_id", "cache_generation", "projection_schema"} {
			t.Run("missing "+field, func(t *testing.T) {
				invalid := map[string]any{}
				for key, value := range example {
					if key != field {
						invalid[key] = value
					}
				}
				data, err := json.Marshal(invalid)
				require.NoError(t, err)
				host, status := test.NewTestHost(data)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
		for _, field := range []string{"balance_key_template", "tenant_header", "consumer_header", "quota_scope", "provider", "missing_balance_policy"} {
			t.Run("retired or permissive "+field, func(t *testing.T) {
				data := quotaConfigWith(map[string]interface{}{field: "allow"})
				host, status := test.NewTestHost(data)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
	})
}
