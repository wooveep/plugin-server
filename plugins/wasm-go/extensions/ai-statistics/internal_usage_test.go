package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/observedusage"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestStatisticsUsesValidatedDeliveryWhenPublicUsageIsNull(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		host, _ := test.NewTestHost(json.RawMessage(`{"path_scope":"bound_target","use_default_response_attributes":true}`))
		defer host.Reset()
		require.NoError(t, host.SetRouteName("managed-statistics-route"))
		host.CallOnHttpRequestHeaders([][2]string{{":method", "POST"}, {":path", "/v1/responses"}, {":authority", "statistics.example"}, {"content-type", "application/json"}})
		host.CallOnHttpRequestBody([]byte(`{"model":"sales-model","input":"hello"}`))
		require.NoError(t, host.SetProperty([]string{observedusage.Property}, []byte(`{"version":"1","call_id":"one-sale","model":"sales-model","source":"provider","input":"17","output":"5","cached":"2"}`)))
		host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
		body := []byte(`{"id":"r","model":"sales-model","output":[],"usage":null}`)
		host.CallOnHttpResponseBody(body)
		host.CompleteHttp()
		facts := statisticsManagedFacts(t, host)
		require.JSONEq(t, "17", string(facts["input_token"]))
		require.JSONEq(t, "5", string(facts["output_token"]))
		require.JSONEq(t, "22", string(facts["total_token"]))
		require.Equal(t, body, host.GetResponseBody())
	})
}
