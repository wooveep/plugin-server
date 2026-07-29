package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestIncrementCounterInitializesNilCache(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(json.RawMessage(`{}`))
		t.Cleanup(host.Reset)
		require.Equal(t, types.OnPluginStartStatusOK, status)

		matched, err := host.GetMatchConfig()
		require.NoError(t, err)
		config, ok := matched.(*AIStatisticsConfig)
		require.True(t, ok)
		config.counterMetrics = nil
		const metricName = "route.regression.metric.request_count"

		require.NotPanics(t, func() {
			config.incrementCounter(metricName, 1)
			config.incrementCounter(metricName, 2)
		})
		require.Len(t, config.counterMetrics, 1)
		value, err := host.GetCounterMetric(metricName)
		require.NoError(t, err)
		require.Equal(t, uint64(3), value)
	})
}
