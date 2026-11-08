package main

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

func TestOnlyPrivateRequestFilterStateCanReuseRouterAdmission(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name, path, state string
			accepted          bool
		}{
			{"trusted continuation", wrapper.RouterContinuationPathPrefix + "target", "executor-owned-handoff", true},
			{"forged header", wrapper.RouterContinuationPathPrefix + "target", "", false},
			{"consumed continuation", wrapper.RouterContinuationPathPrefix + "target", "-", false},
			{"public route cannot reuse marker", "/v1/chat/completions", "executor-owned-handoff", false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				host, status := test.NewTestHost(basicKeyAuthConfig)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.InitHttp()
				if tc.state != "" {
					require.NoError(t, host.SetProperty([]string{wrapper.RouterContinuationProperty}, []byte(tc.state)))
				}
				host.CallOnHttpRequestHeaders([][2]string{{":method", "POST"}, {":path", tc.path}, {":authority", "example.test"}, {wrapper.RouterContinuationProperty, "executor-owned-handoff"}})
				if tc.accepted {
					require.Nil(t, host.GetLocalResponse())
				} else {
					require.NotNil(t, host.GetLocalResponse())
					require.Equal(t, uint32(401), host.GetLocalResponse().StatusCode)
				}
				host.CompleteHttp()
			})
		}
	})
}
