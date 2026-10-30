package main

import (
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryCacheReplaySuppression(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, stream := range []bool{false, true} {
			name := "non-stream"
			if stream {
				name = "stream"
			}
			t.Run(name+" cache replay emits no MemoryEvent", func(t *testing.T) {
				requestBody := memoryEventRequestBody(t, "cache replay question", nil)
				if stream {
					requestBody = memoryStreamingResponseCaptureRequestBody()
				}
				host := startMemoryEventRequestWithBody(t, true, requestBody)
				require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_wasm::higress-system.ai-cache-1.0.1::ai-cache.hit")))

				contentType := "application/json"
				responseBody := memoryEventResponseBody(t, "cache replay answer", "stop", 4, 3, 7)
				if stream {
					contentType = "text/event-stream"
					responseBody = []byte(memoryStreamingSSEFrame("\n", `{"choices":[{"delta":{"content":"cache replay answer"},"finish_reason":"stop"}]}`))
				}
				action := host.CallOnHttpResponseHeaders([][2]string{
					{":status", "200"},
					{"content-type", contentType},
				})
				require.Equal(t, types.ActionContinue, action)
				if stream {
					action = host.CallOnHttpStreamingResponseBody(responseBody, true)
				} else {
					action = host.CallOnHttpResponseBody(responseBody)
				}
				require.Equal(t, types.ActionContinue, action)
				require.Equal(t, string(responseBody), string(host.GetResponseBody()))
				require.Zero(t, countMemoryEventXADDCallouts(t, host), "cache replay must not be learned again as memory")
			})
		}

		for _, detail := range []string{
			"via_upstream",
			"ai-cache.hit",
			"via_wasm::other-plugin::ai-cache.hit",
			"via_wasm::higress-system.ai-cache-1.0.1::ai-cache.hit.extra",
			"other.local.reply",
		} {
			t.Run("detail "+detail+" retains ordinary capture", func(t *testing.T) {
				host := startMemoryEventRequestWithBody(t, true, memoryEventRequestBody(t, "ordinary response question", nil))
				require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte(detail)))
				responseBody := memoryEventResponseBody(t, "ordinary response answer", "stop", 4, 3, 7)
				callMemoryEventResponse(t, host, 200, responseBody)

				require.Equal(t, string(responseBody), string(host.GetResponseBody()))
				require.Equal(t, 1, countMemoryEventXADDCallouts(t, host), "only the exact trusted cache-hit detail may suppress memory capture")
			})
		}
	})
}

func countMemoryEventXADDCallouts(t *testing.T, host test.TestHost) int {
	t.Helper()
	count := 0
	for _, call := range host.GetRedisCalloutAttributes() {
		command, ok := memoryResponseCaptureCommand(t, call.Query)
		if !ok || len(command) < 2 {
			continue
		}
		if strings.EqualFold(command[0], "XADD") && command[1] == memoryEventStreamName {
			count++
		}
	}
	return count
}
