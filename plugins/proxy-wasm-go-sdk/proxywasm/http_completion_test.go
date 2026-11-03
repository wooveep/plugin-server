package proxywasm_test

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/stretchr/testify/require"
)

func TestHTTPAccountingCompletionSurvivesRequestDeletion(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "deleted"}[deleted], func(t *testing.T) {
			var retainedID, ordinaryID uint32
			retained, ordinary := 0, 0
			plugin := &redisLifecyclePlugin{onRequest: func() {
				var err error
				retainedID, err = proxywasm.DispatchHttpCallWithCompletion("provider", [][2]string{{":method", "POST"}, {":path", "/"}}, []byte("input"), nil, 1000, func(_, size, _ int, alive bool) {
					retained++
					require.Equal(t, !deleted, alive)
					body, err := proxywasm.GetHttpCallResponseBody(0, size)
					require.NoError(t, err)
					require.Equal(t, "provider output", string(body))
				})
				require.NoError(t, err)
				ordinaryID, err = proxywasm.DispatchHttpCall("provider", [][2]string{{":method", "POST"}, {":path", "/"}}, nil, nil, 1000, func(int, int, int) { ordinary++ })
				require.NoError(t, err)
			}}
			host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(&redisLifecycleVM{plugin: plugin}))
			defer reset()
			require.Equal(t, types.OnVMStartStatusOK, host.StartVM())
			require.Equal(t, types.OnPluginStartStatusOK, host.StartPlugin())
			id := host.InitializeHttpContext()
			host.CallOnRequestHeaders(id, nil, true)
			if deleted {
				host.CompleteHttpContext(id)
			}
			host.CallOnHttpCallResponse(retainedID, [][2]string{{":status", "200"}}, nil, []byte("provider output"))
			host.CallOnHttpCallResponse(ordinaryID, [][2]string{{":status", "200"}}, nil, []byte("provider output"))
			require.Equal(t, 1, retained)
			if deleted {
				require.Zero(t, ordinary)
			} else {
				require.Equal(t, 1, ordinary)
			}
		})
	}
}
