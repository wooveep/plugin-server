package main

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestKeyAuthEmptyAccessLists(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			globalAuth bool
			consumers  []map[string]interface{}
			rule       bool
			matched    bool
			credential string
			wantDenied bool
		}{
			{name: "last credential revoked", rule: true, matched: true, credential: "revoked-token", wantDenied: true},
			{name: "no credentials without header", rule: true, matched: true, wantDenied: true},
			{name: "empty global consumers", globalAuth: true, credential: "revoked-token", wantDenied: true},
			{name: "empty allow denies another valid consumer", consumers: []map[string]interface{}{{"name": "other", "credential": "valid-token"}}, rule: true, matched: true, credential: "valid-token", wantDenied: true},
			{name: "empty allow with global auth", globalAuth: true, consumers: []map[string]interface{}{{"name": "other", "credential": "valid-token"}}, rule: true, matched: true, credential: "valid-token", wantDenied: true},
			{name: "unmatched route stays public", consumers: []map[string]interface{}{{"name": "other", "credential": "valid-token"}}, rule: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				consumers := tc.consumers
				if consumers == nil {
					consumers = []map[string]interface{}{}
				}
				config := map[string]interface{}{
					"global_auth": tc.globalAuth,
					"keys":        []string{"Authorization"},
					"in_header":   true,
					"consumers":   consumers,
				}
				if tc.rule {
					config["_rules_"] = []map[string]interface{}{{
						"_match_route_": []string{"protected"},
						"allow":         []string{},
					}}
				}
				host, status := test.NewTestHost(mustJSON(config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				if tc.matched {
					require.NoError(t, host.SetRouteName("protected"))
				}
				headers := requestHeaders()
				if tc.credential != "" {
					headers = append(headers, [2]string{"Authorization", "Bearer " + tc.credential})
				}
				host.CallOnHttpRequestHeaders(headers)
				if tc.wantDenied {
					response := host.GetLocalResponse()
					require.NotNil(t, response, "an explicitly empty access list must reject requests")
					require.Contains(t, []uint32{401, 403}, response.StatusCode)
				} else {
					require.Nil(t, host.GetLocalResponse())
				}
				host.CompleteHttp()
			})
		}
	})
}
