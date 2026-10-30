package main

import (
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

// These requests provide only a client consumer header, without an authenticated
// native identity. Keep route/model/usage assertions while labeling them anonymous.
func TestStatisticsLegacyUsageRouteCardinalityRegression(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(basicConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		unsafeRoute := strings.Repeat("r", maxAttributionBytes+1)
		require.NoError(t, host.SetRouteName(unsafeRoute))
		require.NoError(t, host.SetClusterName("cluster-1"))

		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/api/chat"},
			{":method", "POST"},
			{"x-mse-consumer", "user1"},
		}))
		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody([]byte(`{
			"model": "gpt-3.5-turbo",
			"messages": [{"role": "user", "content": "Hello"}]
		}`)))
		require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders([][2]string{
			{":status", "200"},
			{"content-type", "application/json"},
		}))
		require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte(`{
			"choices": [{"message": {"content": "Hello"}}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 8, "total_tokens": 13},
			"model": "gpt-3.5-turbo"
		}`)))
		host.CompleteHttp()

		safeInputMetric := "route.route_unknown.upstream.cluster-1.model.gpt-3.5-turbo.consumer.none.metric.input_token"
		inputTokens, err := host.GetCounterMetric(safeInputMetric)
		require.NoError(t, err)
		require.Equal(t, uint64(5), inputTokens)

		matchedConfig, err := host.GetMatchConfig()
		require.NoError(t, err)
		statisticsConfig, ok := matchedConfig.(*AIStatisticsConfig)
		require.True(t, ok)
		for metricName := range statisticsConfig.counterMetrics {
			require.NotContains(t, metricName, unsafeRoute)
		}
	})
}
