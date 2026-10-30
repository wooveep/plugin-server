package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatisticsTerminalLifecycleGoWasmContract(t *testing.T) {
	goldenMetadata, err := os.ReadFile(filepath.Join(
		"..",
		"ai-proxy",
		"testdata",
		"adapter_metadata_v2_qwen_responses.yaml",
	))
	require.NoError(t, err)
	goldenMetadata = []byte(strings.TrimSpace(string(goldenMetadata)))

	makeConfig := func(t *testing.T, contentTypes []string) json.RawMessage {
		t.Helper()
		config, err := json.Marshal(map[string]interface{}{
			"path_scope":             "bound_target",
			"enable_path_suffixes":   []string{},
			"enable_methods":         []string{},
			"attributes":             []map[string]interface{}{},
			"enable_content_types":   contentTypes,
			"disable_openai_usage":   true,
			"use_default_attributes": false,
		})
		require.NoError(t, err)
		return config
	}

	readFacts := func(t *testing.T, host test.TestHost) map[string]interface{} {
		t.Helper()
		rawLog, err := host.GetProperty([]string{wrapper.AILogKey})
		require.NoError(t, err)
		decodedLog, err := strconv.Unquote(`"` + string(rawLog) + `"`)
		require.NoError(t, err)
		facts := map[string]interface{}{}
		require.NoError(t, json.Unmarshal([]byte(decodedLog), &facts))
		return facts
	}

	allLogs := func(host test.TestHost) string {
		entries := append([]string{}, host.GetTraceLogs()...)
		entries = append(entries, host.GetDebugLogs()...)
		entries = append(entries, host.GetInfoLogs()...)
		entries = append(entries, host.GetWarnLogs()...)
		entries = append(entries, host.GetErrorLogs()...)
		entries = append(entries, host.GetCriticalLogs()...)
		return strings.Join(entries, "\n")
	}

	assertTerminal := func(
		t *testing.T,
		host test.TestHost,
		metricPrefix string,
		wantStatus int,
		wantErrorClass string,
	) uint64 {
		t.Helper()
		count, err := host.GetCounterMetric(metricPrefix + "request_count")
		require.NoError(t, err)
		require.Equal(t, uint64(1), count)
		duration, err := host.GetCounterMetric(metricPrefix + "request_duration_ms")
		require.NoError(t, err)
		require.Greater(t, duration, uint64(0))

		facts := readFacts(t, host)
		require.Equal(t, "statistics-v2-lifecycle-route", facts["route"])
		require.Equal(t, "/v1/responses", facts["path"])
		require.Equal(t, "openai/v1/responses", facts["operation"])
		require.Equal(t, "POST", facts["method"])
		require.Equal(t, float64(wantStatus), facts["status_code"])
		require.Equal(t, wantErrorClass, facts["error_class"])
		require.Equal(t, float64(duration), facts["latency_ms"])
		require.NotEqual(t, facts["operation"], facts["path"])
		return duration
	}

	test.RunTest(t, func(t *testing.T) {
		t.Run("non_streaming_body_EOS_owns_v2_terminal_facts_exactly_once", func(t *testing.T) {
			host, status := test.NewTestHost(makeConfig(t, []string{"application/json"}))
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("statistics-v2-lifecycle-route"))
			require.NoError(t, host.SetProperty([]string{adapterMetadataProperty}, goldenMetadata))

			rawPath := "/v1/responses?api_key=terminal-secret"
			requestHeaders := [][2]string{
				{":authority", "statistics-client.example"},
				{":method", "POST"},
				{":path", rawPath},
				{"authorization", "Bearer terminal-request-secret"},
			}
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(
				requestHeaders,
				test.WithEndOfStream(true),
			))

			responseHeaders := [][2]string{
				{":status", "202"},
				{"content-type", "application/json"},
				{"x-provider-trace", "terminal-trace-secret"},
			}
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders(responseHeaders))
			metricPrefix := "route.statistics-v2-lifecycle-route.path./v1/responses.method.POST.status.202.error.none.metric."
			_, err := host.GetCounterMetric(metricPrefix + "request_count")
			assert.Error(t, err, "headers must not emit terminal metrics")

			time.Sleep(15 * time.Millisecond)
			responseBody := []byte(`{"opaque":"terminal-response-secret"}`)
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody(responseBody))
			duration := assertTerminal(t, host, metricPrefix, 202, "none")
			require.GreaterOrEqual(t, duration, uint64(15))

			host.CompleteHttp()
			countAfterDone, err := host.GetCounterMetric(metricPrefix + "request_count")
			require.NoError(t, err)
			require.Equal(t, uint64(1), countAfterDone)
			durationAfterDone, err := host.GetCounterMetric(metricPrefix + "request_duration_ms")
			require.NoError(t, err)
			require.Equal(t, duration, durationAfterDone)
			require.Equal(t, responseHeaders, host.GetResponseHeaders())
			require.Equal(t, responseBody, host.GetResponseBody())

			for _, forbidden := range []string{
				rawPath,
				"api_key=terminal-secret",
				"terminal-request-secret",
				"terminal-trace-secret",
				"terminal-response-secret",
			} {
				require.NotContains(t, fmt.Sprint(readFacts(t, host)), forbidden)
				require.NotContains(t, metricPrefix, forbidden)
				require.NotContains(t, allLogs(host), forbidden)
			}
		})

		t.Run("unsupported_content_finalizes_only_at_stream_done", func(t *testing.T) {
			host, status := test.NewTestHost(makeConfig(t, []string{"application/json"}))
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("statistics-v2-lifecycle-route"))
			require.NoError(t, host.SetProperty([]string{adapterMetadataProperty}, goldenMetadata))
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(
				[][2]string{
					{":authority", "statistics-client.example"},
					{":method", "POST"},
					{":path", "/v1/responses"},
				},
				test.WithEndOfStream(true),
			))

			responseHeaders := [][2]string{{":status", "204"}, {"content-type", "application/octet-stream"}}
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders(
				responseHeaders,
				test.WithEndOfStream(true),
			))
			metricPrefix := "route.statistics-v2-lifecycle-route.path./v1/responses.method.POST.status.204.error.none.metric."
			_, err := host.GetCounterMetric(metricPrefix + "request_count")
			assert.Error(t, err)

			time.Sleep(10 * time.Millisecond)
			host.CompleteHttp()
			assertTerminal(t, host, metricPrefix, 204, "none")
			require.Equal(t, responseHeaders, host.GetResponseHeaders())
			require.Empty(t, host.GetResponseBody())
		})

		t.Run("streaming_EOS_then_stream_done_does_not_duplicate", func(t *testing.T) {
			host, status := test.NewTestHost(makeConfig(t, []string{"text/event-stream"}))
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("statistics-v2-lifecycle-route"))
			require.NoError(t, host.SetProperty([]string{adapterMetadataProperty}, goldenMetadata))
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(
				[][2]string{
					{":authority", "statistics-client.example"},
					{":method", "POST"},
					{":path", "/v1/responses"},
				},
				test.WithEndOfStream(true),
			))

			responseHeaders := [][2]string{{":status", "503"}, {"content-type", "text/event-stream"}}
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders(responseHeaders))
			time.Sleep(10 * time.Millisecond)
			chunk := []byte("data: terminal-stream-secret\n\n")
			require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingResponseBody(chunk, true))

			metricPrefix := "route.statistics-v2-lifecycle-route.path./v1/responses.method.POST.status.503.error.http_5xx.metric."
			duration := assertTerminal(t, host, metricPrefix, 503, "http_5xx")
			host.CompleteHttp()
			countAfterDone, err := host.GetCounterMetric(metricPrefix + "request_count")
			require.NoError(t, err)
			require.Equal(t, uint64(1), countAfterDone)
			durationAfterDone, err := host.GetCounterMetric(metricPrefix + "request_duration_ms")
			require.NoError(t, err)
			require.Equal(t, duration, durationAfterDone)
			require.Equal(t, responseHeaders, host.GetResponseHeaders())
			require.Equal(t, chunk, host.GetResponseBody())
			require.NotContains(t, fmt.Sprint(readFacts(t, host)), "terminal-stream-secret")
			require.NotContains(t, allLogs(host), "terminal-stream-secret")
		})
	})
}
