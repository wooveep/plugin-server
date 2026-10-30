package main

import (
	"bytes"
	"encoding/json"
	stdlog "log"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestThinCacheEventRedisDisablesBuffering(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("shared endpoint reuses one immediate-dispatch client", func(t *testing.T) {
			logs := thinCacheRedisStartupLogs(t, thinResponseCaptureConfig(t))
			require.Equal(t, 1, strings.Count(logs, "[redis init] upstream:"), logs)
			require.Contains(t, logs, "[redis init] upstream: outbound|6379||redis.static?buffer_flush_timeout=0&max_buffer_size_before_flush=0, username: , timeout: 80")
			require.NotContains(t, logs, "buffer_flush_timeout=3&max_buffer_size_before_flush=1024")
		})

		t.Run("distinct endpoints keep materialized defaults and make events immediate", func(t *testing.T) {
			logs := thinCacheRedisStartupLogs(t, thinDistinctRedisResponseCaptureConfig(t))
			require.Equal(t, 2, strings.Count(logs, "[redis init] upstream:"), logs)
			require.Contains(t, logs, "[redis init] upstream: outbound|6379||redis.static?buffer_flush_timeout=3&max_buffer_size_before_flush=1024, username: , timeout: 80")
			require.Contains(t, logs, "[redis init] upstream: outbound|6379||redis.events?buffer_flush_timeout=0&max_buffer_size_before_flush=0, username: , timeout: 120")
		})
	})
}

func thinCacheRedisStartupLogs(t *testing.T, rawConfig []byte) string {
	t.Helper()
	var logs bytes.Buffer
	previousWriter := stdlog.Writer()
	stdlog.SetOutput(&logs)
	defer stdlog.SetOutput(previousWriter)

	host, status := test.NewTestHost(rawConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)
	return logs.String()
}

func thinDistinctRedisResponseCaptureConfig(t *testing.T) []byte {
	t.Helper()
	var value map[string]interface{}
	require.NoError(t, json.Unmarshal(thinResponseCaptureConfig(t), &value))
	value["redis_stream"].(map[string]interface{})["service_name"] = "redis.events"
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
