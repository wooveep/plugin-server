package main

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

// Exercises the deployed global parser and HTTP callbacks, including ordinary
// traffic sharing the same listener with the private media notice worker.
func TestManagedMediaGlobalStartupPreservesOrdinaryTrafficAndAuthenticatesNotices(t *testing.T) {
	previous, retained := managedMediaActive, managedMediaRetained
	managedMediaActive, managedMediaRetained = nil, nil
	t.Cleanup(func() { managedMediaActive, managedMediaRetained = previous, retained })
	settings := managedMediaSettings{
		ProducerID: "original-media-producer", QueueCapacity: 64, MaxResponseBytes: 4 << 20,
		ServiceAuthToken: "original-private-console-token", PrivateStream: "original-execution-stream",
		Redis: managedRedisSettings{ServiceName: "redis.static", ServicePort: 6379, Timeout: 1000},
		Profiles: []managedMediaProfile{{Profile: mediaexecution.QwenImage30NativeSync,
			AdapterRef: "original-native-adapter", AdapterRevision: "1", CreateClassifierRef: "original-native-classifier",
			OutputIdentityContract: mediaexecution.NativeImageChoicesContent}},
	}
	vm := wrapper.NewCommonVmCtx("media-notice-startup",
		wrapper.PrePluginStartOrReload[config.PluginConfig](initializeManagedHealthRuntime),
		wrapper.ParseOverrideConfig(parseGlobalConfig, parseOverrideRuleConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeader),
		wrapper.ProcessRequestBody(onHttpRequestBody),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
		wrapper.ProcessResponseBody(onHttpResponseBody),
		wrapper.ProcessStreamDone(onManagedStreamDone))
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).
		WithPluginConfiguration(managedFixtureJSON(t, map[string]interface{}{"managed_media": settings})))
	t.Cleanup(reset)
	host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	if err := host.SetProperty([]string{"node", "id"}, []byte("original-media-gateway")); err != nil {
		t.Fatal(err)
	}
	if host.StartVM() != types.OnVMStartStatusOK || host.StartPlugin() != types.OnPluginStartStatusOK {
		t.Fatal("actual global media configuration did not start")
	}
	id := host.InitializeHttpContext()
	headers := [][2]string{{":method", "POST"}, {":path", "/v1/chat/completions"},
		{":authority", "ordinary.example"}, {"content-type", "application/json"}, {"authorization", "Bearer customer-key"}}
	body := []byte(`{"model":"ordinary-model","messages":[]}`)
	if host.CallOnRequestHeaders(id, headers, false) != types.ActionContinue ||
		host.CallOnRequestBody(id, body, true) != types.ActionContinue {
		t.Fatal("media notice worker intercepted ordinary traffic")
	}
	if host.GetSentLocalResponse(id) != nil || !reflect.DeepEqual(host.GetCurrentRequestHeaders(id), headers) ||
		!bytes.Equal(host.GetCurrentRequestBody(id), body) || len(host.GetCalloutAttributesFromContext(id)) != 0 ||
		len(host.GetRedisCalloutAttributesFromContext(id)) != 0 {
		t.Fatal("ordinary request was rewritten, rejected, or produced private work")
	}
	for _, path := range []string{managedMediaDispatchPath, managedMediaQueryPath} {
		id := host.InitializeHttpContext()
		request := [][2]string{{":method", "POST"}, {":path", path}, {":authority", "ordinary.example"},
			{"content-type", "application/json"}, {"authorization", "Bearer customer-key"}}
		host.CallOnRequestHeaders(id, request, false)
		response := host.GetSentLocalResponse(id)
		if response == nil || response.StatusCode != 403 || len(host.GetCalloutAttributesFromContext(id)) != 0 {
			t.Fatal("customer credential crossed the private media notice boundary")
		}
	}
}
