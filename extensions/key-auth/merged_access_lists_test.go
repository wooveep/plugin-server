package main

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestKeyAuthMergedAccessLists(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			config     string
			credential string
			wantDenied bool
		}{
			{
				name:       "last consumer revoked in merged rule",
				config:     `{"global_auth":false,"keys":["Authorization"],"in_header":true,"consumers":[],"allow":[]}`,
				credential: "revoked-token", wantDenied: true,
			},
			{
				name:       "empty merged rule rejects missing header",
				config:     `{"global_auth":false,"keys":["Authorization"],"in_header":true,"consumers":[],"allow":[]}`,
				wantDenied: true,
			},
			{
				name:       "empty merged rule rejects another valid consumer",
				config:     `{"global_auth":false,"keys":["Authorization"],"in_header":true,"consumers":[{"name":"other","credential":"valid-token"}],"allow":[]}`,
				credential: "valid-token", wantDenied: true,
			},
			{
				name:       "merged rule rejects unknown credential",
				config:     `{"global_auth":false,"keys":["Authorization"],"in_header":true,"consumers":[{"name":"other","credential":"valid-token"}],"allow":["other"]}`,
				credential: "unknown-token", wantDenied: true,
			},
			{
				name:       "merged rule accepts allowed consumer",
				config:     `{"global_auth":false,"keys":["Authorization"],"in_header":true,"consumers":[{"name":"other","credential":"valid-token"}],"allow":["other"]}`,
				credential: "valid-token",
			},
			{
				name:   "unmatched configuration stays public",
				config: `{"global_auth":false,"keys":["Authorization"],"in_header":true,"consumers":[]}`,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				host, status := test.NewTestHost([]byte(tc.config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				headers := requestHeaders()
				if tc.credential != "" {
					headers = append(headers, [2]string{"Authorization", "Bearer " + tc.credential})
				}
				host.CallOnHttpRequestHeaders(headers)
				if tc.wantDenied {
					response := host.GetLocalResponse()
					require.NotNil(t, response)
					require.Contains(t, []uint32{401, 403}, response.StatusCode)
				} else {
					require.Nil(t, host.GetLocalResponse())
				}
				host.CompleteHttp()
			})
		}
	})
}
