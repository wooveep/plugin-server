package main

import (
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/observedusage"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestTPMUsesValidatedDeliveryIndependentlyOfPublicUsage(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, body := range []string{`{"model":"sales","usage":null}`, `{"model":"sales","usage":{"input_tokens":999,"output_tokens":999,"total_tokens":1998}}`} {
			host := startResponseTest(t, globalThresholdConfig, "application/json")
			require.NoError(t, host.SetProperty([]string{observedusage.Property}, []byte(`{"version":"1","call_id":"one-sale","model":"sales","source":"provider","input":"17","output":"5"}`)))
			host.CallOnHttpStreamingResponseBody([]byte(body), true)
			assertResponseIncrement(t, host, 22, 1)
			require.Equal(t, []byte(body), host.GetResponseBody())
			host.CompleteHttp()
			host.Reset()
		}
	})
}
