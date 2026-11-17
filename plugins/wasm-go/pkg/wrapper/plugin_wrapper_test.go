package wrapper

import (
	"encoding/binary"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/stretchr/testify/require"
)

func TestCommonHttpCtxEnableRequestBodyIsExplicitOptIn(t *testing.T) {
	ctx := &CommonHttpCtx[struct{}]{}
	require.False(t, ctx.needRequestBody, "the conservative default must remain unchanged")

	ctx.EnableRequestBody()
	require.True(t, ctx.needRequestBody, "explicit opt-in must make body callbacks eligible")

	ctx.DontReadRequestBody()
	require.False(t, ctx.needRequestBody, "the existing opt-out must remain authoritative")
}

func TestCompletedEmptyBodyReachesRegisteredValidator(t *testing.T) {
	calls := 0
	config := struct{}{}
	vm := &CommonVmCtx[struct{}]{
		onHttpRequestBody: func(_ HttpContext, _ struct{}, body []byte) types.Action {
			calls++
			require.Empty(t, body)
			return types.ActionPause
		},
	}
	ctx := &CommonHttpCtx[struct{}]{
		config:          &config,
		needRequestBody: true,
		plugin:          &CommonPluginCtx[struct{}]{vm: vm},
	}
	require.Equal(t, types.ActionPause, ctx.OnHttpRequestBody(0, false))
	require.Zero(t, calls, "a validator must wait for the complete body")
	require.Equal(t, types.ActionPause, ctx.OnHttpRequestBody(0, true))
	require.Equal(t, 1, calls, "zero-byte EOS must not bypass the validator")
}

func TestRequiredBodyReadFailureRejectsInsteadOfForwarding(t *testing.T) {
	for _, required := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "required"}[required], func(t *testing.T) {
			vm := NewCommonVmCtx[struct{}]("required-body")
			vm.onHttpRequestBody = func(HttpContext, struct{}, []byte) types.Action {
				t.Fatal("a failed host read cannot reach the body consumer")
				return types.ActionContinue
			}
			host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm))
			t.Cleanup(reset)
			host.RegisterForeignFunction("declare_property", func([]byte) []byte { return []byte{0} })
			host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
			require.Equal(t, types.OnVMStartStatusOK, host.StartVM())
			require.Equal(t, types.OnPluginStartStatusOK, host.StartPlugin())
			contextID := host.InitializeHttpContext()
			host.CallOnRequestHeaders(contextID, nil, false)

			config := struct{}{}
			ctx := &CommonHttpCtx[struct{}]{config: &config, plugin: &CommonPluginCtx[struct{}]{vm: vm}, userContext: map[string]interface{}{}, needRequestBody: true}
			if required {
				require.True(t, RequireRequestBody(ctx))
			}
			// The host has no buffer although its callback advertised nonempty
			// input. This exercises the real SDK NotFound result.
			action := ctx.OnHttpRequestBody(4, true)
			if required {
				require.Equal(t, types.ActionPause, action)
				require.NotNil(t, host.GetSentLocalResponse(contextID))
				require.Equal(t, uint32(500), host.GetSentLocalResponse(contextID).StatusCode)
			} else {
				require.Equal(t, types.ActionContinue, action)
				require.Nil(t, host.GetSentLocalResponse(contextID))
			}
		})
	}
}

func TestWithRebuildMaxMemBytes(t *testing.T) {
	original := proxywasm.GetVMContext()
	order := []string{}
	vm := NewCommonVmCtx[struct{}](
		"rebuild-mem-test",
		PrePluginStartOrReload[struct{}](func(PluginContext) error {
			order = append(order, "pre")
			return nil
		}),
		ParseRawConfig(func([]byte, *struct{}) error {
			order = append(order, "parse")
			return nil
		}),
		WithRebuildMaxMemBytes[struct{}](64*1024*1024),
	)
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration([]byte(`{}`)))
	t.Cleanup(func() {
		reset()
		proxywasm.SetVMContext(original)
	})
	host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	host.RegisterForeignFunction("declare_property", func([]byte) []byte { return []byte{0} })
	require.Equal(t, types.OnVMStartStatusOK, host.StartVM())
	require.Equal(t, types.OnPluginStartStatusOK, host.StartPlugin())
	require.Equal(t, []string{"pre", "parse"}, order)

	mem := make([]byte, 8)
	binary.LittleEndian.PutUint64(mem, 32*1024*1024)
	require.NoError(t, host.SetProperty([]string{"plugin_vm_memory"}, mem))
	belowID := host.InitializeHttpContext()
	host.CallOnRequestHeaders(belowID, [][2]string{{":method", "POST"}, {":path", "/v1/chat/completions"}, {":authority", "gateway.example"}}, true)
	_, err := host.GetProperty([]string{"wasm_need_rebuild"})
	require.Error(t, err)

	binary.LittleEndian.PutUint64(mem, 64*1024*1024)
	require.NoError(t, host.SetProperty([]string{"plugin_vm_memory"}, mem))
	atID := host.InitializeHttpContext()
	host.CallOnRequestHeaders(atID, [][2]string{{":method", "POST"}, {":path", "/v1/chat/completions"}, {":authority", "gateway.example"}}, true)
	flag, err := host.GetProperty([]string{"wasm_need_rebuild"})
	require.NoError(t, err)
	require.Equal(t, "true", string(flag))
}
