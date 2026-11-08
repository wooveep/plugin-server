package wrapper

import (
	"strconv"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/stretchr/testify/require"
)

func TestRouterContinuationRetainsObserverWithoutRepeatingRequestWork(t *testing.T) {
	for _, oldDoneFirst := range []bool{false, true} {
		t.Run(strconv.FormatBool(oldDoneFirst), func(t *testing.T) {
			requests, bodies, responses, completions := 0, 0, 0, 0
			vm := NewCommonVmCtx[struct{}]("observer", ProcessRequestHeaders(func(ctx HttpContext, _ struct{}) types.Action {
				requests++
				ctx.SetContext("retained", "original processed request")
				return types.HeaderStopIteration
			}), ProcessRequestBody(func(HttpContext, struct{}, []byte) types.Action {
				bodies++
				return types.ActionContinue
			}), ProcessStreamingResponseBody(func(ctx HttpContext, _ struct{}, data []byte, _ bool) []byte {
				require.Equal(t, "original processed request", ctx.GetStringContext("retained", ""))
				responses++
				return data
			}), ProcessStreamDone(func(HttpContext, struct{}) { completions++ }), WithRouterResponseContinuation(func(struct{}) bool { return true }))
			host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration([]byte(`{}`)))
			t.Cleanup(reset)
			host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
			host.RegisterForeignFunction("declare_property", func(wire []byte) []byte {
				require.Equal(t, []byte{0x28, 0x01}, wire[len(wire)-2:])
				return []byte{0}
			})
			require.Equal(t, types.OnVMStartStatusOK, host.StartVM())
			require.Equal(t, types.OnPluginStartStatusOK, host.StartPlugin())
			old := host.InitializeHttpContext()
			headers := [][2]string{{":method", "POST"}, {":path", "/v1/chat/completions"}, {"content-type", "application/json"}, {":authority", "example.test"}}
			host.CallOnRequestHeaders(old, headers, false)
			host.CallOnRequestBody(old, []byte(`{"model":"public"}`), true)
			require.NoError(t, host.SetProperty([]string{RouterContinuationProperty}, []byte("pending-request")))
			host.CallOnResponseHeaders(old, [][2]string{{":status", "429"}, {"content-type", "application/json"}}, false)
			host.CallOnResponseBody(old, []byte(`{"error":{"code":"rate_limit"}}`), true)
			require.Zero(t, responses, "an intermediate response cannot become the cached or recorded final response")
			if oldDoneFirst {
				host.CompleteHttpContext(old)
			}
			next := host.InitializeHttpContext()
			headers[1][1] = RouterContinuationPathPrefix + "target"
			require.Equal(t, types.ActionContinue, host.CallOnRequestHeaders(next, headers, false))
			host.CallOnRequestBody(next, []byte(`{"model":"original"}`), true)
			require.NoError(t, host.SetProperty([]string{RouterContinuationProperty}, []byte("-")))
			if !oldDoneFirst {
				host.CompleteHttpContext(old)
			}
			host.CallOnResponseHeaders(next, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false)
			host.CallOnResponseBody(next, []byte("data: {}\n\n"), true)
			host.CompleteHttpContext(next)
			require.Equal(t, 1, requests)
			require.Equal(t, 1, bodies)
			require.Equal(t, 1, responses)
			require.Equal(t, 1, completions)
		})
	}
}
