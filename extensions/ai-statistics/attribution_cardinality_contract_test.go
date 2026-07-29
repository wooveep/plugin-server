package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatisticsAttributionCardinalityContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		const (
			adapterMetadataProperty = "mse_ai_proxy_adapter_metadata"
			aiLogPropertyKey        = "ai_log"
			requestUUID             = "550e8400-e29b-41d4-a716-446655440000"
			rawRequestPath          = "/v1/files/" + requestUUID + "?api_key=secret"
			callerHeaderMarker      = "caller-header-metadata-marker"
			callerBodyMarker        = "caller-body-metadata-marker"
			attackerSegment         = "attacker-controlled-job-4242"
			trustedRouteName        = "statistics-trusted-route"
			unknownRoute            = "route_unknown"
			unknownPath             = "path_unknown"
			providerNativeUnknown   = "provider_native_unknown"
		)

		type observation struct {
			facts       map[string]interface{}
			metricNames []string
			allLogs     string
		}

		config := json.RawMessage(`{
			"path_scope":"bound_target",
			"enable_path_suffixes":[],
			"enable_methods":[],
			"attributes":[],
			"use_default_attributes":false,
			"disable_openai_usage":false
		}`)

		counterMetricNames := func(config *AIStatisticsConfig) []string {
			names := make([]string, 0, len(config.counterMetrics))
			for name := range config.counterMetrics {
				names = append(names, name)
			}
			sort.Strings(names)
			return names
		}

		metricNames := func(t *testing.T, host test.TestHost) []string {
			t.Helper()
			matched, err := host.GetMatchConfig()
			require.NoError(t, err)
			statisticsConfig, ok := matched.(*AIStatisticsConfig)
			require.True(t, ok, "same-package match config must expose *AIStatisticsConfig")
			return counterMetricNames(statisticsConfig)
		}

		sha256Hex := func(value string) string {
			return fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
		}

		exercise := func(t *testing.T, routeName, requestPath, metadata string) observation {
			t.Helper()
			host, status := test.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NotEqual(t, "<missing>", routeName, "missing route uses the raw-host control")
			require.NoError(t, host.SetRouteName(routeName))
			if metadata != "" {
				require.NoError(t, host.SetProperty([]string{adapterMetadataProperty}, []byte(metadata)))
				stored, err := host.GetProperty([]string{adapterMetadataProperty})
				require.NoError(t, err, "test control must set trusted metadata property")
				require.Equal(t, metadata, string(stored), "test control must retrieve trusted metadata property unchanged")
			}

			requestBody := []byte(fmt.Sprintf(`{"%s":{"normalized_operation":"/spoofed/%s"},"marker":"%s"}`,
				adapterMetadataProperty, attackerSegment, callerBodyMarker))
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "statistics-client.example"},
				{":method", "POST"},
				{":path", requestPath},
				{"authorization", "Bearer api_key=secret"},
				{adapterMetadataProperty, callerHeaderMarker},
				{"x-route-name", "spoofed-" + attackerSegment},
			}))
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(requestBody))

			responseHeaders := [][2]string{{":status", "200"}, {"content-type", "application/octet-stream"}}
			responseBody := []byte("opaque-response-" + callerBodyMarker)
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders(responseHeaders))
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody(responseBody))
			require.Equal(t, responseHeaders, host.GetResponseHeaders(), "response headers must remain transparent")
			require.Equal(t, responseBody, host.GetResponseBody(), "opaque response body must remain transparent")

			rawLog, err := host.GetProperty([]string{aiLogPropertyKey})
			require.NoError(t, err)
			decodedLog, err := strconv.Unquote(`"` + string(rawLog) + `"`)
			require.NoError(t, err)
			facts := map[string]interface{}{}
			require.NoError(t, json.Unmarshal([]byte(decodedLog), &facts))

			host.CompleteHttp()
			allLogEntries := append([]string{}, host.GetTraceLogs()...)
			allLogEntries = append(allLogEntries, host.GetDebugLogs()...)
			allLogEntries = append(allLogEntries, host.GetInfoLogs()...)
			allLogEntries = append(allLogEntries, host.GetWarnLogs()...)
			allLogEntries = append(allLogEntries, host.GetErrorLogs()...)
			allLogEntries = append(allLogEntries, host.GetCriticalLogs()...)

			return observation{
				facts:       facts,
				metricNames: metricNames(t, host),
				allLogs:     strings.Join(allLogEntries, "\n"),
			}
		}

		exerciseMissingRoute := func(t *testing.T, requestPath, metadata string) observation {
			t.Helper()
			bootstrapHost, bootstrapStatus := test.NewTestHost(config)
			require.Equal(t, types.OnPluginStartStatusOK, bootstrapStatus)
			testVM := proxywasm.GetVMContext()
			bootstrapHost.Reset()

			host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().
				WithPluginConfiguration(config).
				WithVMContext(testVM))
			defer reset()
			host.RegisterForeignFunction("get_log_level", func([]byte) []byte {
				result := make([]byte, 4)
				binary.LittleEndian.PutUint32(result, 0)
				return result
			})
			require.Equal(t, types.OnPluginStartStatusOK, host.StartPlugin())
			_, err := host.GetProperty([]string{"route_name"})
			require.Error(t, err, "raw HostEmulator control must leave route_name absent")
			if metadata != "" {
				require.NoError(t, host.SetProperty([]string{adapterMetadataProperty}, []byte(metadata)))
			}

			contextID := host.InitializeHttpContext()
			require.Equal(t, types.ActionContinue, host.CallOnRequestHeaders(contextID, [][2]string{
				{":authority", "statistics-client.example"},
				{":method", "POST"},
				{":path", requestPath},
				{"authorization", "Bearer api_key=secret"},
				{adapterMetadataProperty, callerHeaderMarker},
				{"x-route-name", "spoofed-" + attackerSegment},
			}, false))
			requestBody := []byte(fmt.Sprintf(`{"%s":{"normalized_operation":"/spoofed/%s"},"marker":"%s"}`,
				adapterMetadataProperty, attackerSegment, callerBodyMarker))
			require.Equal(t, types.ActionContinue, host.CallOnRequestBody(contextID, requestBody, true))

			responseHeaders := [][2]string{{":status", "200"}, {"content-type", "application/octet-stream"}}
			responseBody := []byte("opaque-response-" + callerBodyMarker)
			require.Equal(t, types.ActionContinue, host.CallOnResponseHeaders(contextID, responseHeaders, false))
			require.Equal(t, types.ActionContinue, host.CallOnResponseBody(contextID, responseBody, true))
			require.Equal(t, responseHeaders, host.GetCurrentResponseHeaders(contextID))
			require.Equal(t, responseBody, host.GetCurrentResponseBody(contextID))

			rawLog, err := host.GetProperty([]string{aiLogPropertyKey})
			require.NoError(t, err)
			decodedLog, err := strconv.Unquote(`"` + string(rawLog) + `"`)
			require.NoError(t, err)
			facts := map[string]interface{}{}
			require.NoError(t, json.Unmarshal([]byte(decodedLog), &facts))

			matched, ok := proxywasm.GetHttpContext(contextID).(interface {
				GetMatchConfig() (*AIStatisticsConfig, error)
			})
			require.True(t, ok, "raw HostEmulator context must expose the existing match config")
			statisticsConfig, err := matched.GetMatchConfig()
			require.NoError(t, err)
			host.CompleteHttpContext(contextID)
			allLogEntries := append([]string{}, host.GetTraceLogs()...)
			allLogEntries = append(allLogEntries, host.GetDebugLogs()...)
			allLogEntries = append(allLogEntries, host.GetInfoLogs()...)
			allLogEntries = append(allLogEntries, host.GetWarnLogs()...)
			allLogEntries = append(allLogEntries, host.GetErrorLogs()...)
			allLogEntries = append(allLogEntries, host.GetCriticalLogs()...)

			return observation{
				facts:       facts,
				metricNames: counterMetricNames(statisticsConfig),
				allLogs:     strings.Join(allLogEntries, "\n"),
			}
		}

		assertSafe := func(t *testing.T, got observation, wantRoute, wantPath string) {
			t.Helper()
			assert.Equal(t, wantRoute, got.facts["route"])
			assert.Equal(t, wantPath, got.facts["path"])
			assert.Equal(t, "POST", got.facts["method"])
			assert.Equal(t, float64(200), got.facts["status_code"])
			assert.Equal(t, "none", got.facts["error_class"])

			metricPrefix := fmt.Sprintf("route.%s.path.%s.method.POST.status.200.error.none.metric.", wantRoute, wantPath)
			assert.Equal(t, []string{metricPrefix + "request_count", metricPrefix + "request_duration_ms"}, got.metricNames,
				"only bounded base request metrics may be emitted")
			for _, optionalMetric := range []string{"input_token", "output_token", "total_token", "first_token_duration"} {
				assert.NotContains(t, got.metricNames, optionalMetric)
			}

			for _, forbidden := range []string{
				requestUUID,
				"api_key=secret",
				rawRequestPath,
				callerHeaderMarker,
				callerBodyMarker,
				attackerSegment,
				sha256Hex(requestUUID),
				sha256Hex(rawRequestPath),
			} {
				assert.NotContains(t, fmt.Sprint(got.facts), forbidden, "decoded AILog must exclude untrusted data")
				assert.NotContains(t, strings.Join(got.metricNames, "\n"), forbidden, "metric names must exclude untrusted data")
				assert.NotContains(t, got.allLogs, forbidden, "logs must exclude untrusted data")
			}
		}

		adapterMetadata := func(matched bool, normalizedOperation string) string {
			return fmt.Sprintf(`{"schema_version":1,"matched":%t,"mode":"auto","adapter_id":"openai.files.get","version":"v1","normalized_operation":%q,"usage_profile":"opaque"}`,
				matched, normalizedOperation)
		}
		trustedMetadata := adapterMetadata(true, "/v1/files/{file_id}")
		t.Run("trusted normalized template replaces raw UUID query and credential", func(t *testing.T) {
			assertSafe(t, exercise(t, trustedRouteName, rawRequestPath, trustedMetadata), trustedRouteName, "/v1/files/{file_id}")
		})

		t.Run("opaque provider native request without trusted metadata uses fixed fallback", func(t *testing.T) {
			assertSafe(t, exercise(t, trustedRouteName, "/vendor/jobs/"+attackerSegment, ""), trustedRouteName, providerNativeUnknown)
		})

		t.Run("unmatched adapter metadata cannot contribute a normalized template", func(t *testing.T) {
			assertSafe(t, exercise(t, trustedRouteName, "/vendor/jobs/"+attackerSegment, adapterMetadata(false, "/v1/files/{file_id}")), trustedRouteName, providerNativeUnknown)
		})

		for _, testCase := range []struct {
			name     string
			template string
		}{
			{name: "non-origin form", template: "files/{file_id}"},
			{name: "double slash", template: "//v1/files/{file_id}"},
			{name: "authority", template: "http://attacker.example/v1/files/{file_id}"},
			{name: "query", template: "/v1/files/{file_id}?api_key=secret"},
			{name: "fragment", template: "/v1/files/{file_id}#fragment"},
			{name: "control character", template: "/v1/files/{file_id}\n"},
			{name: "illegal ASCII", template: "/v1/files/{file id}"},
			{name: "Unicode", template: "/v1/files/é"},
			{name: "over 128 bytes", template: "/" + strings.Repeat("a", 129)},
		} {
			testCase := testCase
			t.Run("invalid trusted template "+testCase.name+" uses fixed fallback", func(t *testing.T) {
				metadata := adapterMetadata(true, testCase.template)
				assertSafe(t, exercise(t, trustedRouteName, rawRequestPath, metadata), trustedRouteName, unknownPath)
			})
		}

		for _, testCase := range []struct {
			name      string
			routeName string
			wantRoute string
		}{
			{name: "valid trusted route name is retained", routeName: trustedRouteName, wantRoute: trustedRouteName},
			{name: "missing trusted route name uses fixed fallback", routeName: "<missing>", wantRoute: unknownRoute},
			{name: "exact 128 byte trusted route name is retained", routeName: strings.Repeat("r", 128), wantRoute: strings.Repeat("r", 128)},
			{name: "control route name uses fixed fallback", routeName: "route\ncontrol", wantRoute: unknownRoute},
			{name: "oversized route name uses fixed fallback", routeName: strings.Repeat("r", 129), wantRoute: unknownRoute},
		} {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				if testCase.routeName == "<missing>" {
					assertSafe(t, exerciseMissingRoute(t, rawRequestPath, trustedMetadata), testCase.wantRoute, "/v1/files/{file_id}")
					return
				}
				assertSafe(t, exercise(t, testCase.routeName, rawRequestPath, trustedMetadata), testCase.wantRoute, "/v1/files/{file_id}")
			})
		}

		t.Run("repeated missing metadata fallback is deterministic", func(t *testing.T) {
			first := exercise(t, trustedRouteName, "/vendor/jobs/"+attackerSegment, "")
			time.Sleep(time.Millisecond)
			second := exercise(t, trustedRouteName, "/vendor/jobs/"+attackerSegment, "")
			assertSafe(t, first, trustedRouteName, providerNativeUnknown)
			assertSafe(t, second, trustedRouteName, providerNativeUnknown)
			assert.Equal(t, first.facts["route"], second.facts["route"])
			assert.Equal(t, first.facts["path"], second.facts["path"])
			assert.Equal(t, first.metricNames, second.metricNames)
		})

		t.Run("caller spoofed metadata headers route headers and body are ignored", func(t *testing.T) {
			assertSafe(t, exercise(t, trustedRouteName, "/vendor/jobs/"+attackerSegment, ""), trustedRouteName, providerNativeUnknown)
		})
	})
}
