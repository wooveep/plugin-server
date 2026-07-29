package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatisticsOpaqueBaseFactsContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		const (
			aiLogPropertyKey = "ai_log"

			routeFactKey   = "route"
			pathFactKey    = "path"
			methodFactKey  = "method"
			statusFactKey  = "status_code"
			latencyFactKey = "latency_ms"
			errorFactKey   = "error_class"

			unknownProviderPath = "provider_native_unknown"
			requestCountMetric  = "request_count"
			requestTimeMetric   = "request_duration_ms"
		)

		type lifecycleCase struct {
			name               string
			disableOpenAIUsage bool
			method             string
			path               string
			status             int
			contentType        string
			responseBody       []byte
			streaming          bool
			errorClass         string
			sensitiveMarker    string
		}

		testCases := []lifecycleCase{
			{
				name:               "opaque success emits base facts when OpenAI usage is disabled",
				disableOpenAIUsage: true,
				method:             "PATCH",
				path:               "/vendor/jobs/job-42",
				status:             202,
				contentType:        "application/json",
				responseBody:       []byte("opaque-success-response"),
				errorClass:         "none",
				sensitiveMarker:    "opaque-success-response",
			},
			{
				name:               "opaque success without usage still emits count and duration",
				disableOpenAIUsage: false,
				method:             "POST",
				path:               "/vendor/native/models",
				status:             201,
				contentType:        "application/json",
				responseBody:       []byte(`{"result":"opaque-without-usage"}`),
				errorClass:         "none",
				sensitiveMarker:    "opaque-without-usage",
			},
			{
				name:               "opaque streaming error emits bounded classification at end of stream",
				disableOpenAIUsage: false,
				method:             "DELETE",
				path:               "/vendor/jobs/job-99",
				status:             503,
				contentType:        "text/event-stream",
				responseBody:       []byte("data: provider-secret-error-detail\n\n"),
				streaming:          true,
				errorClass:         "http_5xx",
				sensitiveMarker:    "provider-secret-error-detail",
			},
		}

		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				config, err := json.Marshal(map[string]interface{}{
					"path_scope":             "bound_target",
					"enable_path_suffixes":   []string{},
					"enable_methods":         []string{},
					"attributes":             []map[string]interface{}{},
					"disable_openai_usage":   testCase.disableOpenAIUsage,
					"enable_content_types":   []string{},
					"use_default_attributes": false,
				})
				require.NoError(t, err)

				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				const routeName = "statistics-base-facts-route"
				require.NoError(t, host.SetRouteName(routeName))

				requestHeaders := [][2]string{
					{":authority", "statistics-client.example"},
					{":method", testCase.method},
					{":path", testCase.path},
					{"authorization", "Bearer request-secret-must-not-be-observed"},
				}
				require.Equal(
					t,
					types.ActionContinue,
					host.CallOnHttpRequestHeaders(requestHeaders, test.WithEndOfStream(true)),
				)

				time.Sleep(5 * time.Millisecond)

				responseHeaders := [][2]string{
					{":status", strconv.Itoa(testCase.status)},
					{"content-type", testCase.contentType},
					{"x-provider-trace", "opaque-provider-trace"},
				}
				require.Equal(
					t,
					types.ActionContinue,
					host.CallOnHttpResponseHeaders(responseHeaders),
				)
				require.Equal(t, responseHeaders, host.GetResponseHeaders())

				if testCase.streaming {
					require.Equal(
						t,
						types.ActionContinue,
						host.CallOnHttpStreamingResponseBody(testCase.responseBody, true),
					)
				} else {
					require.Equal(
						t,
						types.ActionContinue,
						host.CallOnHttpResponseBody(testCase.responseBody),
					)
				}
				require.Equal(t, testCase.responseBody, host.GetResponseBody())
				require.Equal(t, responseHeaders, host.GetResponseHeaders())

				rawLog, err := host.GetProperty([]string{aiLogPropertyKey})
				require.NoError(t, err)
				decodedLog, err := strconv.Unquote(`"` + string(rawLog) + `"`)
				require.NoError(t, err)

				var facts map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(decodedLog), &facts))

				assert.Equal(t, routeName, facts[routeFactKey])
				assert.Equal(t, unknownProviderPath, facts[pathFactKey])
				assert.Equal(t, testCase.method, facts[methodFactKey])
				assert.Equal(t, float64(testCase.status), facts[statusFactKey])
				assert.Equal(t, testCase.errorClass, facts[errorFactKey])
				if assert.Contains(t, facts, latencyFactKey) {
					latency, ok := facts[latencyFactKey].(float64)
					if assert.True(t, ok, "latency must be a numeric millisecond fact") {
						assert.Greater(t, latency, float64(0))
					}
				}

				for _, optionalFact := range []string{
					"api",
					"model",
					"operation",
					"input_token",
					"output_token",
					"total_token",
				} {
					assert.NotContains(
						t,
						facts,
						optionalFact,
						"unavailable optional dimensions must remain absent",
					)
				}

				assert.NotContains(t, decodedLog, testCase.sensitiveMarker)
				assert.NotContains(t, decodedLog, "request-secret-must-not-be-observed")
				assert.NotContains(t, decodedLog, "opaque-provider-trace")
				assert.NotContains(t, decodedLog, testCase.path)

				allLogEntries := make([]string, 0)
				allLogEntries = append(allLogEntries, host.GetTraceLogs()...)
				allLogEntries = append(allLogEntries, host.GetDebugLogs()...)
				allLogEntries = append(allLogEntries, host.GetInfoLogs()...)
				allLogEntries = append(allLogEntries, host.GetWarnLogs()...)
				allLogEntries = append(allLogEntries, host.GetErrorLogs()...)
				allLogEntries = append(allLogEntries, host.GetCriticalLogs()...)
				allLogs := strings.Join(allLogEntries, "\n")
				assert.NotContains(t, allLogs, testCase.sensitiveMarker)
				assert.NotContains(t, allLogs, "request-secret-must-not-be-observed")

				metricPrefix := fmt.Sprintf(
					"route.%s.path.%s.method.%s.status.%d.error.%s.metric.",
					routeName,
					unknownProviderPath,
					testCase.method,
					testCase.status,
					testCase.errorClass,
				)
				requestCount, countErr := host.GetCounterMetric(metricPrefix + requestCountMetric)
				assert.NoError(t, countErr)
				assert.Equal(t, uint64(1), requestCount)

				requestDuration, durationErr := host.GetCounterMetric(metricPrefix + requestTimeMetric)
				assert.NoError(t, durationErr)
				assert.Greater(t, requestDuration, uint64(0))

				for _, metricName := range []string{
					metricPrefix + requestCountMetric,
					metricPrefix + requestTimeMetric,
				} {
					assert.False(t, strings.Contains(metricName, testCase.sensitiveMarker))
					assert.False(t, strings.Contains(metricName, "request-secret-must-not-be-observed"))
					assert.False(t, strings.Contains(metricName, testCase.path))
				}

				host.CompleteHttp()
			})
		}
	})
}
