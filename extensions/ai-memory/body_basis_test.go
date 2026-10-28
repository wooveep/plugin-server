package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestManagedMemoryBodyBasisConfigBoundary(t *testing.T) {
	for _, raw := range []string{
		`{"capture_body_basis":"true","path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"text_generate"}`,
		`{"capture_body_basis":null,"path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"text_generate"}`,
		`{"capture_body_basis":true}`,
		`{"capture_body_basis":true,"path_scope":"bound_target","client_protocol":"openai_chat","operation_kind":"task_query"}`,
	} {
		var c config.PluginConfig
		c.FromJson(gjson.Parse(`{"redis_stream":{"service_name":"redis.memory.svc.cluster.local"}}`), memoryCaptureNoopLog{})
		c.FromJsonWithGlobal(gjson.Parse(raw), c, memoryCaptureNoopLog{})
		require.Error(t, c.Validate(), raw)
	}
	var c config.PluginConfig
	c.FromJson(gjson.Parse(`{"redis_stream":{"service_name":"redis.memory.svc.cluster.local"},"capture_body_basis":false}`), memoryCaptureNoopLog{})
	require.NoError(t, c.Validate())
	require.False(t, c.CaptureBodyBasis)
}

func TestManagedMemoryBodyBasisBeforeInjection(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct{ protocol, body string }{
			{"openai_chat", ` {"model":"public.model","messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA"},"vendor_sequence":9007199254740993}]}]} `},
			{"anthropic_messages", ` {"model":"public.model","messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAA"},"vendor_sequence":9007199254740993}]}]} `},
			{"openai_responses", ` {"model":"public.model","input":[{"role":"user","content":[{"type":"input_text","text":"describe"},{"type":"input_file","filename":"sample.pdf","file_data":"AAA","vendor_sequence":9007199254740993}]}]} `},
		} {
			t.Run(tc.protocol, func(t *testing.T) {
				host, status := newMemoryConfigTestHost(memoryBodyBasisConfig(t, tc.protocol, "semantic"))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("memory-route"))
				headers := append(memoryConsoleAssembleHeaders(), memorySpoofedBodyBasis()...)
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
				requireMemoryBodyBasisAbsent(t, host.GetRequestHeaders())
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(tc.body)))
				expected, err := bodybasis.Capture([]byte(tc.body))
				require.NoError(t, err)
				captured, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, expected, captured)
				requireMemoryAssembleCall(t, host)
				host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{"role": "system", "content": "retained memory"}, nil))
				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				require.Contains(t, string(host.GetRequestBody()), "retained memory")
				require.Contains(t, string(host.GetRequestBody()), "9007199254740993")
				after, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, captured, after, "memory injection must not replace the original basis")
				rewrittenBasis, err := bodybasis.Capture(host.GetRequestBody())
				require.NoError(t, err)
				require.NotEqual(t, rewrittenBasis.Digest(), after.Digest())
			})
		}
	})
}

func TestManagedMemoryBodyBasisSurvivesMemoryBypass(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name, mode string
			overrides  [][2]string
			assemble   bool
		}{
			{name: "memory off", mode: "off"},
			{name: "missing tenant", mode: "semantic", overrides: [][2]string{{"x-mse-tenant", ""}}},
			{name: "missing consumer", mode: "semantic", overrides: [][2]string{{"x-mse-consumer", ""}}},
			{name: "no store", mode: "semantic", overrides: [][2]string{{memoryNoStoreHeader, "true"}, {"content-encoding", "identity"}}, assemble: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				host, status := newMemoryConfigTestHost(memoryBodyBasisConfig(t, "openai_chat", tc.mode))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("memory-route"))
				headers := applyMemoryRequestGatingHeaderOverrides(memoryConsoleAssembleHeaders(), tc.overrides)
				headers = append(headers, memorySpoofedBodyBasis()...)
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
				requireMemoryBodyBasisAbsent(t, host.GetRequestHeaders())
				body := []byte(" {\n\"model\" : \"public.model\", \"messages\":[{\"role\":\"user\",\"content\":\"question\"}],\"amount\":9007199254740993.0000000001} ")
				action := host.CallOnHttpRequestBody(body)
				if tc.assemble {
					require.Equal(t, types.ActionPause, action)
					requireMemoryAssembleCall(t, host)
					host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "bypass", nil, nil))
				} else {
					require.Equal(t, types.ActionContinue, action)
					require.Empty(t, host.GetHttpCalloutAttributes())
				}
				require.Empty(t, host.GetRedisCalloutAttributes())
				require.Equal(t, string(body), string(host.GetRequestBody()), "capture must retain the actual original sending bytes")
				expected, err := bodybasis.Capture(body)
				require.NoError(t, err)
				actual, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, expected, actual)
			})
		}
	})
}

func TestManagedMemoryBodyBasisWaitsForCompleteBody(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		host, status := newMemoryConfigTestHost(memoryBodyBasisConfig(t, "openai_chat", "off"))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		require.NoError(t, host.SetRouteName("memory-route"))
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(append(memoryConsoleAssembleHeaders(), memorySpoofedBodyBasis()...)))
		body := []byte(`{"model":"public.model","messages":[{"role":"user","content":"chunked"}]}`)
		require.Equal(t, types.ActionPause, host.CallOnHttpStreamingRequestBody(body[:24], false))
		requireMemoryBodyBasisAbsent(t, host.GetRequestHeaders())
		require.Nil(t, host.GetLocalResponse())
		require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingRequestBody(body[24:], true))
		expected, err := bodybasis.Capture(body)
		require.NoError(t, err)
		actual, err := bodybasis.Read(host.GetRequestHeaders())
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		require.Equal(t, string(body), string(host.GetRequestBody()))
	})
}

func TestManagedMemoryBodyBasisRejectsInvalidOriginal(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, body := range []string{`{"messages":[]}`, `{"model":"public.model","model":"spoof"}`, `{"model":"public.model","data":{"x":1,"x":2}}`, `{"model":"public.model"`} {
			t.Run(body, func(t *testing.T) {
				host, status := newMemoryConfigTestHost(memoryBodyBasisConfig(t, "openai_chat", "off"))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("memory-route"))
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(append(memoryConsoleAssembleHeaders(), memorySpoofedBodyBasis()...)))
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(body)))
				require.NotNil(t, host.GetLocalResponse())
				require.Equal(t, uint32(400), host.GetLocalResponse().StatusCode)
				requireMemoryBodyBasisAbsent(t, host.GetRequestHeaders())
				require.Empty(t, host.GetHttpCalloutAttributes())
				require.Empty(t, host.GetRedisCalloutAttributes())
			})
		}
		for _, noBody := range []bool{false, true} {
			host, status := newMemoryConfigTestHost(memoryBodyBasisConfig(t, "openai_chat", "off"))
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("memory-route"))
			headers := append(memoryConsoleAssembleHeaders(), memorySpoofedBodyBasis()...)
			if !noBody {
				headers = append(headers, [2]string{"content-encoding", "gzip"})
			}
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestHeaders(headers, test.WithEndOfStream(noBody)))
			requireMemoryBodyBasisAbsent(t, host.GetRequestHeaders())
			require.NotNil(t, host.GetLocalResponse())
			if noBody {
				require.Equal(t, uint32(400), host.GetLocalResponse().StatusCode)
			} else {
				require.Equal(t, uint32(415), host.GetLocalResponse().StatusCode)
			}
			host.Reset()
		}
	})
}

func memoryBodyBasisConfig(t *testing.T, clientProtocol, mode string) json.RawMessage {
	t.Helper()
	value, err := sjson.SetBytes(managedMemoryConfig(t, clientProtocol, "text_generate", mode), "_rules_.0.capture_body_basis", true)
	require.NoError(t, err)
	return value
}

func memorySpoofedBodyBasis() [][2]string {
	return [][2]string{{bodybasis.DigestHeader, "client-spoof-1"}, {strings.ToUpper(bodybasis.DigestHeader), "client-spoof-2"}, {bodybasis.PublicModelHeader, "client-model-1"}, {strings.ToUpper(bodybasis.PublicModelHeader), "client-model-2"}}
}

func requireMemoryBodyBasisAbsent(t *testing.T, headers [][2]string) {
	t.Helper()
	for _, header := range headers {
		require.False(t, strings.EqualFold(header[0], bodybasis.DigestHeader))
		require.False(t, strings.EqualFold(header[0], bodybasis.PublicModelHeader))
	}
}
