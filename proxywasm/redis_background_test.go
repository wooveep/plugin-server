package proxywasm_test

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/stretchr/testify/require"
)

type redisLifecycleVM struct {
	types.DefaultVMContext
	plugin *redisLifecyclePlugin
}

func (v *redisLifecycleVM) NewPluginContext(uint32) types.PluginContext { return v.plugin }

type redisLifecyclePlugin struct {
	types.DefaultPluginContext
	onRequest func()
}

func (p *redisLifecyclePlugin) NewHttpContext(uint32) types.HttpContext {
	return &redisLifecycleHTTP{onRequest: p.onRequest}
}

type redisLifecycleHTTP struct {
	types.DefaultHttpContext
	onRequest func()
}

func (h *redisLifecycleHTTP) OnHttpRequestHeaders(int, bool) types.Action {
	h.onRequest()
	return types.ActionContinue
}

func TestBackgroundRedisCallbackSurvivesHTTPDeletion(t *testing.T) {
	for _, status := range []int32{0, 1} {
		t.Run(map[int32]string{0: "success", 1: "network_failure"}[status], func(t *testing.T) {
			var backgroundID, requestID uint32
			backgroundCallbacks, requestCallbacks := 0, 0
			plugin := &redisLifecyclePlugin{onRequest: func() {
				var err error
				backgroundID, err = proxywasm.DispatchRedisCallInBackground("redis", []byte("PING\r\n"), func(gotStatus, size int) {
					backgroundCallbacks++
					require.Equal(t, int(status), gotStatus)
					body, err := proxywasm.GetRedisCallResponse(0, size)
					require.NoError(t, err)
					require.Equal(t, "+PONG\r\n", string(body))
				})
				require.NoError(t, err)
				requestID, err = proxywasm.DispatchRedisCall("redis", []byte("PING\r\n"), func(int, int) {
					requestCallbacks++
				})
				require.NoError(t, err)
			}}
			host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(&redisLifecycleVM{plugin: plugin}))
			defer reset()
			require.Equal(t, types.OnVMStartStatusOK, host.StartVM())
			require.Equal(t, types.OnPluginStartStatusOK, host.StartPlugin())
			id := host.InitializeHttpContext()
			require.Equal(t, types.ActionContinue, host.CallOnRequestHeaders(id, nil, true))
			require.Len(t, host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID), 1)
			require.Len(t, host.GetRedisCalloutAttributesFromContext(id), 1, "dispatch restores the original caller")
			host.CompleteHttpContext(id)
			host.CallOnRedisCallResponse(backgroundID, status, []byte("+PONG\r\n"))
			host.CallOnRedisCallResponse(requestID, 0, []byte("+PONG\r\n"))
			require.Equal(t, 1, backgroundCallbacks, "event ACK must outlive the HTTP request")
			require.Zero(t, requestCallbacks, "request callbacks retain their existing cancellation semantics")
		})
	}
}
