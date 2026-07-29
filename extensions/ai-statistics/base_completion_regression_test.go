package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

func TestStatisticsBaseCompletionSeamsRegression(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		testCases := []struct {
			name                string
			responseHeaders     [][2]string
			enabledContentTypes []string
			headerEndOfStream   bool
			sendEmptyBody       bool
			wantStatus          int
			wantErrorClass      string
		}{
			{
				name: "response headers end stream without body",
				responseHeaders: [][2]string{
					{":status", "204"},
					{"content-type", "application/octet-stream"},
				},
				headerEndOfStream: true,
				wantStatus:        204,
				wantErrorClass:    "none",
			},
			{
				name: "unsupported parser content type retains base facts",
				responseHeaders: [][2]string{
					{":status", "202"},
					{"content-type", "text/plain"},
				},
				enabledContentTypes: []string{"application/json"},
				headerEndOfStream:   true,
				wantStatus:          202,
				wantErrorClass:      "none",
			},
			{
				name: "empty body does not duplicate base metrics",
				responseHeaders: [][2]string{
					{":status", "200"},
					{"content-type", "application/json"},
				},
				sendEmptyBody:  true,
				wantStatus:     200,
				wantErrorClass: "none",
			},
			{
				name: "malformed status uses bounded fallback",
				responseHeaders: [][2]string{
					{":status", "not-a-status"},
					{"content-type", "application/octet-stream"},
				},
				headerEndOfStream: true,
				wantStatus:        0,
				wantErrorClass:    "status_unknown",
			},
			{
				name: "missing status uses bounded fallback",
				responseHeaders: [][2]string{
					{"content-type", "application/octet-stream"},
				},
				headerEndOfStream: true,
				wantStatus:        0,
				wantErrorClass:    "status_unknown",
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
					"enable_content_types":   testCase.enabledContentTypes,
					"disable_openai_usage":   true,
					"use_default_attributes": false,
				})
				require.NoError(t, err)

				host, status := test.NewTestHost(config)
				defer host.Reset()
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
				require.Equal(
					t,
					types.ActionContinue,
					host.CallOnHttpResponseHeaders(
						testCase.responseHeaders,
						test.WithEndOfStream(testCase.headerEndOfStream),
					),
				)
				require.Equal(t, testCase.responseHeaders, host.GetResponseHeaders())

				metricPrefix := fmt.Sprintf(
					"route.statistics-completion-route.path.provider_native_unknown.method.POST.status.%d.error.%s.metric.",
					testCase.wantStatus,
					testCase.wantErrorClass,
				)
				countBeforeBody, err := host.GetCounterMetric(metricPrefix + "request_count")
				require.NoError(t, err)
				require.Equal(t, uint64(1), countBeforeBody)
				durationBeforeBody, err := host.GetCounterMetric(metricPrefix + "request_duration_ms")
				require.NoError(t, err)
				require.Greater(t, durationBeforeBody, uint64(0))

				if testCase.sendEmptyBody {
					require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte{}))
					countAfterBody, err := host.GetCounterMetric(metricPrefix + "request_count")
					require.NoError(t, err)
					require.Equal(t, countBeforeBody, countAfterBody)
					durationAfterBody, err := host.GetCounterMetric(metricPrefix + "request_duration_ms")
					require.NoError(t, err)
					require.Equal(t, durationBeforeBody, durationAfterBody)
				}
				require.Empty(t, host.GetResponseBody())
				require.Equal(t, testCase.responseHeaders, host.GetResponseHeaders())

				rawLog, err := host.GetProperty([]string{wrapper.AILogKey})
				require.NoError(t, err)
				decodedLog, err := strconv.Unquote(`"` + string(rawLog) + `"`)
				require.NoError(t, err)
				facts := map[string]interface{}{}
				require.NoError(t, json.Unmarshal([]byte(decodedLog), &facts))
				require.Equal(t, float64(testCase.wantStatus), facts["status_code"])
				require.Equal(t, testCase.wantErrorClass, facts["error_class"])

				matchedConfig, err := host.GetMatchConfig()
				require.NoError(t, err)
				statisticsConfig, ok := matchedConfig.(*AIStatisticsConfig)
				require.True(t, ok)
				metricNames := make([]string, 0, len(statisticsConfig.counterMetrics))
				for metricName := range statisticsConfig.counterMetrics {
					metricNames = append(metricNames, metricName)
				}
				sort.Strings(metricNames)
				require.Equal(t, []string{
					metricPrefix + "request_count",
					metricPrefix + "request_duration_ms",
				}, metricNames)
			})
		}
	})
}
