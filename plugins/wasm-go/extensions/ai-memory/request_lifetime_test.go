package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

func TestMemoryAssemblyBuffersReleasedBeforeSlowStreamingResponse(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		previous := memoryLoadRecentMemory
		var requestContext wrapper.HttpContext
		memoryLoadRecentMemory = func(ctx wrapper.HttpContext, cfg config.PluginConfig, logger log.Log) error {
			requestContext = ctx
			return previous(ctx, cfg, logger)
		}
		t.Cleanup(func() { memoryLoadRecentMemory = previous })
		largeContext := strings.Repeat("large context ", 1<<16)
		body, err := json.Marshal(map[string]interface{}{
			"model": "qwen-turbo", "stream": true,
			"messages": []interface{}{
				map[string]interface{}{"role": "system", "content": largeContext},
				map[string]interface{}{"role": "user", "content": []interface{}{
					map[string]interface{}{"type": "text", "text": "current question"},
					map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "data:image/png;base64,fixture"}},
				}},
			},
		})
		require.NoError(t, err)
		host := startMemoryTrustedDigestRequest(t, body, "")
		completeMemoryTrustedDigestInjection(t, host, "durable preference")
		require.NotNil(t, requestContext)
		require.NotEmpty(t, requestContext.GetContext(memoryOriginalBodyContextKey))
		require.NotEmpty(t, requestContext.GetContext(memoryCurrentMessagesContextKey))
		host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "text/event-stream"}})
		require.Nil(t, requestContext.GetContext(memoryOriginalBodyContextKey), "full body must not remain live for the slow stream")
		require.Nil(t, requestContext.GetContext(memoryCurrentMessagesContextKey), "decoded large messages must be released after assembly")
		require.Contains(t, string(host.GetRequestBody()), largeContext)
		require.Contains(t, string(host.GetRequestBody()), "data:image/png;base64,fixture")
		require.Contains(t, string(host.GetRequestBody()), "durable preference")
		callMemoryStreamingResponseChunks(t, host,
			memoryStreamingSSEFrame("\n", `{"model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"captured answer"},"finish_reason":null}]}`),
			memoryStreamingSSEFrame("\n", `{"model":"qwen-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":5,"total_tokens":17}}`)+memoryStreamingSSEFrame("\n", `[DONE]`),
		)
		event := requireMemoryResponseCaptureEvent(t, host)
		require.Equal(t, "current question", event["user_content"])
		require.Equal(t, "captured answer", event["assistant_content"])
		require.Equal(t, "stop", event["finish_reason"])
		requireMemoryResponseCaptureUsage(t, event, 12, 5, 17)
	})
}
