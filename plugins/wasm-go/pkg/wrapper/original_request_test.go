package wrapper

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/stretchr/testify/require"
)

func newOriginalRequestLogTestHost(t *testing.T, handler func(HttpContext)) proxytest.HostEmulator {
	t.Helper()
	original := proxywasm.GetVMContext()
	vm := NewCommonVmCtx[struct{}]("original-request-test", ProcessRequestHeaders(func(ctx HttpContext, _ struct{}) types.Action {
		PreserveOriginalRequestID(ctx)
		if handler != nil {
			handler(ctx)
		}
		return types.ActionContinue
	}))
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration([]byte(`{}`)))
	t.Cleanup(func() {
		reset()
		proxywasm.SetVMContext(original)
	})
	host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	host.RegisterForeignFunction("declare_property", func([]byte) []byte { return []byte{0} })
	require.Equal(t, types.OnVMStartStatusOK, host.StartVM())
	require.Equal(t, types.OnPluginStartStatusOK, host.StartPlugin())
	return host
}

func TestPreserveOriginalRequestLogMetadataSkipsEmptyHeadersWithoutWarning(t *testing.T) {
	host := newOriginalRequestLogTestHost(t, PreserveCurrentRequestLogID)
	id := host.InitializeHttpContext()
	action := host.CallOnRequestHeaders(id, [][2]string{
		{":method", "POST"},
		{":path", "/v1/responses"},
		{":authority", "gateway.example"},
		{"x-request-id", "req-original-123"},
	}, true)
	require.Equal(t, types.ActionContinue, action)

	for _, warning := range host.GetWarnLogs() {
		require.NotContains(t, warning, "data must not be empty")
	}

	captured, err := host.GetProperty([]string{originalLogCapturedProperty})
	require.NoError(t, err)
	require.Equal(t, "1", string(captured))

	origID, err := host.GetProperty([]string{originalLogIDProperty})
	require.NoError(t, err)
	require.Equal(t, "req-original-123", string(origID))

	_, err = host.GetProperty([]string{originalLogUserAgentProperty})
	require.Error(t, err)

	_, err = host.GetProperty([]string{originalLogForwardedProperty})
	require.Error(t, err)

	currCaptured, err := host.GetProperty([]string{currentLogCapturedProperty})
	require.NoError(t, err)
	require.Equal(t, "1", string(currCaptured))

	currID, err := host.GetProperty([]string{currentLogIDProperty})
	require.NoError(t, err)
	require.Equal(t, "req-original-123", string(currID))
}

func TestPreserveOriginalAndCurrentRequestLogSkipEmptyAndContinuationValues(t *testing.T) {
	t.Run("empty-and-dash-current-request-id", func(t *testing.T) {
		host := newOriginalRequestLogTestHost(t, PreserveCurrentRequestLogID)
		id := host.InitializeHttpContext()
		action := host.CallOnRequestHeaders(id, [][2]string{
			{":method", "POST"},
			{":path", "/v1/responses"},
			{":authority", "gateway.example"},
			{"x-request-id", "-"},
		}, true)
		require.Equal(t, types.ActionContinue, action)

		currCaptured, err := host.GetProperty([]string{currentLogCapturedProperty})
		require.NoError(t, err)
		require.Equal(t, "1", string(currCaptured))

		_, err = host.GetProperty([]string{currentLogIDProperty})
		require.Error(t, err)

		for _, warning := range host.GetWarnLogs() {
			require.NotContains(t, warning, "data must not be empty")
		}
	})

	t.Run("continuation-pending-skips-original-id-without-error", func(t *testing.T) {
		host := newOriginalRequestLogTestHost(t, nil)
		id := host.InitializeHttpContext()
		require.NoError(t, host.SetProperty([]string{RouterContinuationProperty}, []byte("pending")))
		action := host.CallOnRequestHeaders(id, [][2]string{
			{":method", "POST"},
			{":path", "/v1/responses"},
			{":authority", "gateway.example"},
			{"x-request-id", "orig-should-drop"},
		}, true)
		require.Equal(t, types.ActionContinue, action)

		captured, err := host.GetProperty([]string{originalLogCapturedProperty})
		require.NoError(t, err)
		require.Equal(t, "1", string(captured))

		_, err = host.GetProperty([]string{originalLogIDProperty})
		require.Error(t, err)

		for _, warning := range host.GetWarnLogs() {
			require.NotContains(t, warning, "data must not be empty")
		}
	})
}
