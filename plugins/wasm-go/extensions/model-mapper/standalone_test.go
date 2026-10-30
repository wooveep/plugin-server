package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

var basicConfig = json.RawMessage(`{"modelMapping":{"gpt-3.5-turbo":"gpt-4","gpt-*":"gpt-family","*":"fallback"}}`)

func TestStandaloneMappingPreservesUpstreamModes(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		config json.RawMessage
		body   string
		want   string
		model  string
	}{
		{"exact before prefix", basicConfig, `{"model":"gpt-3.5-turbo","messages":[]}`, `{"model":"gpt-4","messages":[]}`, "gpt-4"},
		{"prefix", basicConfig, `{"model":"gpt-4o","messages":[]}`, `{"model":"gpt-family","messages":[]}`, "gpt-family"},
		{"default", basicConfig, `{"model":"another","messages":[]}`, `{"model":"fallback","messages":[]}`, "fallback"},
		{"identity", json.RawMessage(`{}`), `{"model":"unchanged"}`, `{"model":"unchanged"}`, "unchanged"},
		{"nested model", json.RawMessage(`{"modelKey":"request.model","modelMapping":{"public":"provider"}}`), `{"request":{"model":"public"}}`, `{"request":{"model":"provider"}}`, "provider"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			test.RunTest(t, func(t *testing.T) {
				host, status := test.NewTestHost(scenario.config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				headers := mapperHeaders()
				for i := range headers {
					if headers[i][0] == ":path" {
						headers[i][1] = "/v1/chat/completions?trace=true"
					}
				}
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody([]byte(scenario.body)))
				require.Nil(t, host.GetLocalResponse())
				require.JSONEq(t, scenario.want, string(host.GetRequestBody()))
				require.Contains(t, host.GetRequestHeaders(), [2]string{"x-higress-llm-model-final", scenario.model})
				requireNoBasis(t, host.GetRequestHeaders())
				host.CompleteHttp()
			})
		})
	}
}
