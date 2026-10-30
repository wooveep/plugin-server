package main

import (
	"bytes"
	stdlog "log"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryRedisClientLifecycle(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("plugin startup initializes persistent immediate-dispatch clients", func(t *testing.T) {
			var logs bytes.Buffer
			previousWriter := stdlog.Writer()
			stdlog.SetOutput(&logs)
			defer stdlog.SetOutput(previousWriter)

			host, status := newMemoryConfigTestHost(memorySharedRedisLifecycleConfig(t))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			output := logs.String()
			require.Equal(t, 2, strings.Count(output, "[redis init] upstream:"), output)
			sharedInit := "[redis init] upstream: outbound|6379||redis.shared?buffer_flush_timeout=0&max_buffer_size_before_flush=0, username: , timeout: 50"
			require.Equal(t, 2, strings.Count(output, sharedInit), output)
			require.NotContains(t, output, "timeout: 500", output)
		})

		t.Run("request lookup and response event reuse startup clients", func(t *testing.T) {
			var logs bytes.Buffer
			previousWriter := stdlog.Writer()
			stdlog.SetOutput(&logs)
			defer stdlog.SetOutput(previousWriter)

			host, status := newMemoryConfigTestHost(memorySharedRedisLifecycleConfig(t))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("memory-route"))
			require.NoError(t, host.SetRequestId("property-request-id"))
			logs.Reset()

			action := host.CallOnHttpRequestHeaders(memoryConsoleAssembleHeaders())
			require.Equal(t, types.HeaderStopIteration, action)
			action = host.CallOnHttpRequestBody(memoryEventRequestBody(t, "persistent redis clients", nil))
			require.Equal(t, types.ActionPause, action)
			require.NotEmpty(t, host.GetRedisCalloutAttributes())
			require.Equal(t, "outbound|6379||redis.shared", host.GetRedisCalloutAttributes()[0].Upstream)
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())

			callMemoryEventResponse(t, host, 200, memoryEventResponseBody(t, "persistent redis response", "stop", 4, 3, 7))

			requireMemoryEventXADDCommand(t, host)
			requireMemoryEventXADDUpstream(t, host, "outbound|6379||redis.shared")
			require.NotContains(t, logs.String(), "[redis init]", logs.String())
		})
	})
}

func requireMemoryEventXADDUpstream(t *testing.T, host test.TestHost, expected string) {
	t.Helper()
	for _, call := range host.GetRedisCalloutAttributes() {
		command, ok := memoryResponseCaptureCommand(t, call.Query)
		if !ok || len(command) < 2 || !strings.EqualFold(command[0], "XADD") {
			continue
		}
		require.Equal(t, expected, call.Upstream)
		return
	}
	require.Fail(t, "missing MemoryEvent XADD callout")
}

func memorySharedRedisLifecycleConfig(t *testing.T) []byte {
	t.Helper()
	return mustMemoryConfig(t, map[string]interface{}{
		"redis_stream": map[string]interface{}{
			"service_name": "redis.shared",
		},
		"recent_cache": map[string]interface{}{
			"service_name": "redis.shared",
		},
		"console_internal": map[string]interface{}{
			"service_name": memoryConsoleService,
		},
		"fail_policy": "open",
		"_rules_": []map[string]interface{}{
			{
				"_match_route_":    []string{"memory-route"},
				"memory_mode":      "recent-only",
				"policy_version":   "7",
				"capture_response": true,
			},
		},
	})
}
