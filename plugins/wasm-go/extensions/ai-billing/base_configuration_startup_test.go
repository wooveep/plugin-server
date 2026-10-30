package main

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

// A stored plugin name or filter entry cannot establish successful startup.
// Exercise OnPluginStart and unrelated listener traffic in Go and compiled Wasm.
func TestBaseConfigurationInitializesAndPreservesUnrelatedTraffic(t *testing.T) {
	for _, scenario := range []struct {
		name          string
		configuration []byte
	}{
		{"empty payload", nil},
		{"empty object", []byte(`{}`)},
		{"instance identity", []byte(`{"_plugin_id_":"base-startup-fixture"}`)},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			test.RunTest(t, func(t *testing.T) {
				host, status := test.NewTestHost(scenario.configuration)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				headers := [][2]string{{":authority", "admin.fixture.test"}, {":method", "GET"}, {":path", "/api/v1/plugin-instances"}}
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(headers))
				host.CompleteHttp()
				require.Nil(t, host.GetLocalResponse())
				require.Equal(t, headers, host.GetRequestHeaders())
				require.Empty(t, host.GetRedisCalloutAttributes())
			})
		})
	}
}

func TestEmptyRouteRuleListRejectsPluginStartup(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost([]byte(`{"_rules_":[]}`))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusFailed, status)
	})
}
