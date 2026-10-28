package wrapper

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

func TestRedisEventReauthenticationKeepsEventDispatcher(t *testing.T) {
	dispatches := 0
	client := NewRedisEventClient(FQDNCluster{FQDN: "redis.static", Port: 6379}, func(cluster string, query []byte, callback func(int, int)) (uint32, error) {
		dispatches++
		return proxywasm.DispatchRedisCall(cluster, query, callback)
	})
	callbacks := 0
	vm := NewCommonVmCtx("redis-event-test",
		ParseConfig(func(gjson.Result, *struct{}) error { return client.Init("", "", 500) }),
		ProcessRequestHeaders(func(HttpContext, struct{}) types.Action {
			require.NoError(t, client.Command([]interface{}{"XADD", "events", "*", "event", "controlled"}, func(reply resp.Value) {
				callbacks++
				require.NoError(t, reply.Error())
				require.Equal(t, "30-0", reply.String())
			}))
			return types.ActionContinue
		}),
	)
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration([]byte(`{}`)))
	defer reset()
	host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	require.Equal(t, types.OnPluginStartStatusOK, host.StartPlugin())
	id := host.InitializeHttpContext()
	require.Equal(t, types.ActionContinue, host.CallOnRequestHeaders(id, [][2]string{{":authority", "example.test"}}, true))
	calls := host.GetRedisCalloutAttributesFromContext(id)
	require.Len(t, calls, 1)
	original := string(calls[0].Query)
	host.CallOnRedisCallResponse(calls[0].CalloutID, 0, []byte("-NOAUTH Authentication required\r\n"))
	require.Zero(t, callbacks, "the existing one-time authentication retry owns completion")
	calls = host.GetRedisCalloutAttributesFromContext(id)
	require.Len(t, calls, 1)
	require.Equal(t, 2, dispatches, "authentication retry must retain the event dispatch path")
	require.Equal(t, original, string(calls[0].Query), "reauthentication must preserve the exact event bytes")
	host.CallOnRedisCallResponse(calls[0].CalloutID, 0, []byte("$4\r\n30-0\r\n"))
	require.Equal(t, 1, callbacks)
	host.CompleteHttpContext(id)
}
