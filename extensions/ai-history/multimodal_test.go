package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestHistoryInjectionPreservesMultimodalContent(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, _ := test.NewTestHost(basicRedisConfig)
		defer host.Reset()
		host.CallOnHttpRequestHeaders([][2]string{{":authority", "example.com"}, {":method", "POST"},
			{":path", "/v1/chat/completions"}, {"content-type", "application/json"}, {"authorization", "synthetic-user"}})
		host.CallOnHttpRequestBody([]byte(`{"messages":[{"role":"system","content":"context"},{"role":"user","content":[{"type":"text","text":"current question"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AA=="}}]}]}`))
		host.CallOnRedisCall(0, test.CreateRedisRespString(`[{"role":"user","content":"previous"},{"role":"assistant","content":"answer"}]`))
		var body struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(host.GetRequestBody(), &body))
		require.Len(t, body.Messages, 4)
		require.Equal(t, "system", body.Messages[0].Role)
		require.JSONEq(t, `[{"type":"text","text":"current question"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AA=="}}]`, string(body.Messages[3].Content))
	})
}
