package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestStaticUnlimitedOverridePrecedesDefault(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, subject := range []string{"unlimited", "inherited"} {
			t.Run(subject, func(t *testing.T) {
				cfg := json.RawMessage(`{"rule_name":"static-user-limit","redis":{"service_name":"redis.static","service_port":6379},
                    "rule_items":[{"limit_by_per_header":"x-modelfusion-principal-id","limit_keys":[
                        {"key":"regexp:^unlimited$","token_per_minute":0},
                        {"key":"*","token_per_minute":10}]}]}`)
				host, status := test.NewTestHost(cfg)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"}, {":path", "/v1/chat/completions"},
					{":method", "POST"}, {"x-modelfusion-principal-id", subject},
				})
				if subject == "unlimited" {
					require.Equal(t, types.ActionContinue, action, "zero overrides the wildcard without a Redis call")
					require.Empty(t, host.GetRedisCalloutAttributes())
				} else {
					require.Equal(t, types.HeaderStopAllIterationAndWatermark, action, "other users retain the default counter")
					require.Len(t, host.GetRedisCalloutAttributes(), 1)
				}
			})
		}
	})
}
