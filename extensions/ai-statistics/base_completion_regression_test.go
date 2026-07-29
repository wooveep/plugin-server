package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatisticsBaseCompletionSeamsRegression(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		makeConfig := func(t *testing.T, enabledContentTypes []string) json.RawMessage {
			t.Helper()
			config, err := json.Marshal(map[string]interface{}{
				"path_scope":             "bound_target",
				"enable_path_suffixes":   []string{},
				"enable_methods":         []string{},
				"attributes":             []map[string]interface{}{},
				"enable_content_types":   enabledContentTypes,
				"disable_openai_usage":   true,
				"use_default_attributes": false,
			})
			require.NoError(t, err)
			return config
		}

		metricPrefix := func(status int, errorClass string) string {
			return fmt.Sprintf(
				"route.statistics-completion-route.path.provider_native_unknown.method.POST.status.%d.error.%s.metric.",
				status,
				errorClass,
			)
		}

		assertTerminalFacts := func(
			t *testing.T,
			host test.TestHost,
			prefix string,
			wantStatus int,
			wantErrorClass string,
		) uint64 {
			t.Helper()
			count, err := host.GetCounterMetric(prefix + "request_count")
			require.NoError(t, err)
			require.Equal(t, uint64(1), count)
			duration, err := host.GetCounterMetric(prefix + "request_duration_ms")
			require.NoError(t, err)
			require.Greater(t, duration, uint64(0))

			rawLog, err := host.GetProperty([]string{wrapper.AILogKey})
			require.NoError(t, err)
			decodedLog, err := strconv.Unquote(`"` + string(rawLog) + `"`)
			require.NoError(t, err)
			facts := map[string]interface{}{}
			require.NoError(t, json.Unmarshal([]byte(decodedLog), &facts))
			require.Equal(t, float64(wantStatus), facts["status_code"])
			require.Equal(t, wantErrorClass, facts["error_class"])
			require.Equal(t, float64(duration), facts["latency_ms"],
				"duration counter and terminal AILog must use the same timestamp")
			return duration
		}

		t.Run("status boundary accepts only exact three digit ASCII 100 through 599", func(t *testing.T) {
			testCases := []struct {
				name           string
				rawStatus      string
				includeStatus  bool
				wantStatus     int
				wantErrorClass string
			}{
				{name: "099", rawStatus: "099", includeStatus: true, wantStatus: 0, wantErrorClass: "status_unknown"},
				{name: "100", rawStatus: "100", includeStatus: true, wantStatus: 100, wantErrorClass: "status_unknown"},
				{name: "599", rawStatus: "599", includeStatus: true, wantStatus: 599, wantErrorClass: "http_5xx"},
				{name: "600", rawStatus: "600", includeStatus: true, wantStatus: 0, wantErrorClass: "status_unknown"},
				{name: "999", rawStatus: "999", includeStatus: true, wantStatus: 0, wantErrorClass: "status_unknown"},
				{name: "signed", rawStatus: "+200", includeStatus: true, wantStatus: 0, wantErrorClass: "status_unknown"},
				{name: "four digits", rawStatus: "0200", includeStatus: true, wantStatus: 0, wantErrorClass: "status_unknown"},
				{name: "empty", rawStatus: "", includeStatus: true, wantStatus: 0, wantErrorClass: "status_unknown"},
				{name: "missing", wantStatus: 0, wantErrorClass: "status_unknown"},
				{name: "non numeric", rawStatus: "2O0", includeStatus: true, wantStatus: 0, wantErrorClass: "status_unknown"},
			}

			for _, testCase := range testCases {
				testCase := testCase
				t.Run(testCase.name, func(t *testing.T) {
					host, status := test.NewTestHost(makeConfig(t, []string{"application/json"}))
					t.Cleanup(host.Reset)
					require.Equal(t, types.OnPluginStartStatusOK, status)
					require.NoError(t, host.SetRouteName("statistics-completion-route"))
					require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(
						[][2]string{
							{":authority", "statistics-client.example"},
							{":method", "POST"},
							{":path", "/provider/native/completion"},
						},
						test.WithEndOfStream(true),
					))

					headers := [][2]string{{"content-type", "text/plain"}}
					if testCase.includeStatus {
						headers = append([][2]string{{":status", testCase.rawStatus}}, headers...)
					}
					require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders(
						headers,
						test.WithEndOfStream(true),
					))

					prefix := metricPrefix(testCase.wantStatus, testCase.wantErrorClass)
					_, err := host.GetCounterMetric(prefix + "request_count")
					assert.Error(t, err, "response headers must not finalize completed-request metrics")

					host.CompleteHttp()
					assertTerminalFacts(t, host, prefix, testCase.wantStatus, testCase.wantErrorClass)
					require.Equal(t, headers, host.GetResponseHeaders())
				})
			}
		})

		t.Run("duration includes delayed non streaming body EOS and remains exact once", func(t *testing.T) {
			host, status := test.NewTestHost(makeConfig(t, []string{"application/json"}))
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("statistics-completion-route"))
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(
				[][2]string{
					{":authority", "statistics-client.example"},
					{":method", "POST"},
					{":path", "/provider/native/completion"},
				},
				test.WithEndOfStream(true),
			))

			headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders(headers))
			prefix := metricPrefix(200, "none")
			_, err := host.GetCounterMetric(prefix + "request_count")
			assert.Error(t, err, "response headers must not finalize completed-request metrics")

			time.Sleep(25 * time.Millisecond)
			body := []byte(`{"opaque":"terminal-body"}`)
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody(body))
			duration := assertTerminalFacts(t, host, prefix, 200, "none")
			require.GreaterOrEqual(t, duration, uint64(25),
				"duration must include observable response-body delay")

			host.CompleteHttp()
			countAfterDone, err := host.GetCounterMetric(prefix + "request_count")
			require.NoError(t, err)
			require.Equal(t, uint64(1), countAfterDone)
			durationAfterDone, err := host.GetCounterMetric(prefix + "request_duration_ms")
			require.NoError(t, err)
			require.Equal(t, duration, durationAfterDone)
			require.Equal(t, headers, host.GetResponseHeaders())
			require.Equal(t, body, host.GetResponseBody())
		})

		t.Run("unsupported parser content finalizes through stream done fallback", func(t *testing.T) {
			host, status := test.NewTestHost(makeConfig(t, []string{"application/json"}))
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("statistics-completion-route"))
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(
				[][2]string{
					{":authority", "statistics-client.example"},
					{":method", "POST"},
					{":path", "/provider/native/completion"},
				},
				test.WithEndOfStream(true),
			))

			headers := [][2]string{{":status", "202"}, {"content-type", "text/plain"}}
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders(headers))
			prefix := metricPrefix(202, "none")
			_, err := host.GetCounterMetric(prefix + "request_count")
			assert.Error(t, err)

			time.Sleep(10 * time.Millisecond)
			host.CompleteHttp()
			assertTerminalFacts(t, host, prefix, 202, "none")
			require.Equal(t, headers, host.GetResponseHeaders())
		})

		t.Run("streaming EOS finalizes before stream done without duplication", func(t *testing.T) {
			host, status := test.NewTestHost(makeConfig(t, []string{"text/event-stream"}))
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("statistics-completion-route"))
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(
				[][2]string{
					{":authority", "statistics-client.example"},
					{":method", "POST"},
					{":path", "/provider/native/completion"},
				},
				test.WithEndOfStream(true),
			))

			headers := [][2]string{{":status", "503"}, {"content-type", "text/event-stream"}}
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders(headers))
			prefix := metricPrefix(503, "http_5xx")
			_, err := host.GetCounterMetric(prefix + "request_count")
			assert.Error(t, err)

			time.Sleep(10 * time.Millisecond)
			chunk := []byte("data: opaque-terminal-stream\n\n")
			require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingResponseBody(chunk, true))
			duration := assertTerminalFacts(t, host, prefix, 503, "http_5xx")

			host.CompleteHttp()
			countAfterDone, err := host.GetCounterMetric(prefix + "request_count")
			require.NoError(t, err)
			require.Equal(t, uint64(1), countAfterDone)
			durationAfterDone, err := host.GetCounterMetric(prefix + "request_duration_ms")
			require.NoError(t, err)
			require.Equal(t, duration, durationAfterDone)
			require.Equal(t, headers, host.GetResponseHeaders())
			require.Equal(t, chunk, host.GetResponseBody())
		})
	})
}
