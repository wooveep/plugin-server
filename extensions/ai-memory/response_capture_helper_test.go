package main

import (
	"reflect"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryCaptureNonStreamingResponseFacts(t *testing.T) {
	t.Run("captures assistant content status usage finish reason and configured tool calls", func(t *testing.T) {
		capture := memoryCaptureNonStreamingResponse([]byte(`{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "captured helper answer",
					"custom_tool_calls": [{"id":"custom-call-1","name":"lookup"}]
				},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 9, "completion_tokens": 4, "total_tokens": 13}
		}`), 202, "choices.0.message.content", []string{"choices.0.message.custom_tool_calls"})

		require.Equal(t, "captured helper answer", capture.AssistantContent)
		require.Equal(t, "stop", capture.FinishReason)
		require.Equal(t, 202, capture.StatusCode)
		require.False(t, capture.IsStream)
		require.True(t, capture.ContainsToolCalls)
		require.False(t, capture.ParseFailed)
		require.Equal(t, 9, capture.Usage.PromptTokens)
		require.Equal(t, 4, capture.Usage.CompletionTokens)
		require.Equal(t, 13, capture.Usage.TotalTokens)
	})

	t.Run("uses configured response value path for assistant content", func(t *testing.T) {
		capture := memoryCaptureNonStreamingResponse([]byte(`{
			"choices": [{
				"message": {"role": "assistant"},
				"finish_reason": "stop"
			}],
			"output": {"message": {"text": "configured path assistant answer"}},
			"usage": {"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5}
		}`), 200, "output.message.text", nil)

		require.Equal(t, "configured path assistant answer", capture.AssistantContent)
		require.Equal(t, "stop", capture.FinishReason)
		require.False(t, capture.ParseFailed)
		require.Equal(t, 3, capture.Usage.PromptTokens)
		require.Equal(t, 2, capture.Usage.CompletionTokens)
		require.Equal(t, 5, capture.Usage.TotalTokens)
	})

	t.Run("parse failure preserves status and records failure without content", func(t *testing.T) {
		capture := memoryCaptureNonStreamingResponse([]byte(`{not-json`), 502, "choices.0.message.content", nil)

		require.Equal(t, 502, capture.StatusCode)
		require.False(t, capture.IsStream)
		require.True(t, capture.ParseFailed)
		require.Empty(t, capture.AssistantContent)
		require.Empty(t, capture.FinishReason)
		require.False(t, capture.ContainsToolCalls)
		require.Zero(t, capture.Usage)
	})
}

func TestMemoryRawContentEligibilityGatesNonStreamingRawFields(t *testing.T) {
	tests := []struct {
		name            string
		captureResponse bool
		noStore         bool
	}{
		{name: "disabled capture", captureResponse: false},
		{name: "request no-store", captureResponse: true, noStore: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMemoryCaptureTestContext()
			ctx.SetContext(memoryResponseStatusContextKey, 207)
			ctx.SetContext(memoryUserContentContextKey, "raw helper user content")
			if tt.noStore {
				ctx.SetContext(memoryNoStoreContextKey, true)
			}
			cfg := config.PluginConfig{
				Route: config.RouteConfig{
					CaptureResponse:   tt.captureResponse,
					ResponseValueFrom: "choices.0.message.content",
					ToolCallsFrom:     []string{"choices.0.message.custom_tool_calls"},
				},
			}

			memoryCaptureNonStreamingResponseChunk(ctx, cfg, []byte(`{
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "raw helper assistant content must be omitted",
						"custom_tool_calls": [{"id":"tool-1","name":"lookup"}]
					},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 14, "completion_tokens": 6, "total_tokens": 20}
			}`), true, memoryCaptureNoopLog{})

			capture, ok := ctx.GetContext(memoryResponseCaptureContextKey).(memoryResponseCapture)
			require.True(t, ok)
			require.Equal(t, 207, capture.StatusCode)
			require.False(t, capture.IsStream)
			require.False(t, capture.ParseFailed)
			require.Empty(t, capture.AssistantContent)
			require.Equal(t, "stop", capture.FinishReason)
			require.True(t, capture.ContainsToolCalls)
			require.Equal(t, 14, capture.Usage.PromptTokens)
			require.Equal(t, 6, capture.Usage.CompletionTokens)
			require.Equal(t, 20, capture.Usage.TotalTokens)
			require.Empty(t, memoryEventUserContent(ctx, cfg))
			require.Equal(t, "raw helper user content", ctx.GetStringContext(memoryUserContentContextKey, ""))
		})
	}
}

func TestMemoryEventUserContentReturnsStoredContentOnlyWhenRawCaptureAllowed(t *testing.T) {
	tests := []struct {
		name            string
		captureResponse bool
		noStore         bool
		want            string
	}{
		{name: "allowed", captureResponse: true, want: "event-facing user content"},
		{name: "disabled capture", captureResponse: false},
		{name: "request no-store", captureResponse: true, noStore: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMemoryCaptureTestContext()
			ctx.SetContext(memoryUserContentContextKey, "event-facing user content")
			if tt.noStore {
				ctx.SetContext(memoryNoStoreContextKey, true)
			}
			cfg := config.PluginConfig{
				Route: config.RouteConfig{
					CaptureResponse: tt.captureResponse,
				},
			}

			require.Equal(t, tt.want, memoryEventUserContent(ctx, cfg))
			require.Equal(t, "event-facing user content", ctx.GetStringContext(memoryUserContentContextKey, ""))
		})
	}
}

func TestMemoryResponseHookSkipsBodyForGatedAndAllowsStreamRequests(t *testing.T) {
	t.Run("gated request disables response body processing", func(t *testing.T) {
		ctx := newMemoryCaptureTestContext()
		markMemoryGate(ctx, "missing-tenant")

		action := onHttpResponseHeaders(ctx, config.PluginConfig{}, memoryCaptureNoopLog{})

		require.Equal(t, types.ActionContinue, action)
		require.True(t, ctx.dontReadResponseBody)
	})

	t.Run("stream request keeps response body processing enabled", func(t *testing.T) {
		test.RunGoTest(t, func(t *testing.T) {
			host := startMemoryStreamingResponseCaptureRequest(t)

			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})

			require.Equal(t, types.ActionContinue, action)
			requireMemoryHostNeedsResponseBody(t, host)
		})
	})
}

func TestMemoryCaptureNonStreamingResponseChunkLimitsBuffer(t *testing.T) {
	t.Run("non-final chunks fail open when combined buffer is too large", func(t *testing.T) {
		ctx := newMemoryCaptureTestContext()
		ctx.SetContext(memoryResponseStatusContextKey, 200)

		memoryCaptureNonStreamingResponseChunkWithLimit(ctx, config.PluginConfig{}, []byte("12345"), false, 8, memoryCaptureNoopLog{})
		memoryCaptureNonStreamingResponseChunkWithLimit(ctx, config.PluginConfig{}, []byte("6789"), false, 8, memoryCaptureNoopLog{})

		capture, ok := ctx.GetContext(memoryResponseCaptureContextKey).(memoryResponseCapture)
		require.True(t, ok)
		require.Equal(t, 200, capture.StatusCode)
		require.True(t, capture.ParseFailed)
		require.Empty(t, ctx.GetByteSliceContext(memoryResponseBodyBufferContextKey, nil))
	})

	t.Run("single final chunk fails open when too large", func(t *testing.T) {
		ctx := newMemoryCaptureTestContext()
		ctx.SetContext(memoryResponseStatusContextKey, 200)

		memoryCaptureNonStreamingResponseChunkWithLimit(ctx, config.PluginConfig{}, []byte(`{"choices":[{"message":{"content":"too large"},"finish_reason":"stop"}]}`), true, 8, memoryCaptureNoopLog{})

		capture, ok := ctx.GetContext(memoryResponseCaptureContextKey).(memoryResponseCapture)
		require.True(t, ok)
		require.Equal(t, 200, capture.StatusCode)
		require.True(t, capture.ParseFailed)
		require.Empty(t, capture.AssistantContent)
	})
}

func TestMemoryCaptureStreamingResponseChunkStoresFinalCaptureFacts(t *testing.T) {
	ctx := newMemoryCaptureTestContext()
	ctx.SetContext(memoryStreamContextKey, true)
	ctx.SetContext(memoryResponseStatusContextKey, 200)
	cfg := config.PluginConfig{
		Route: config.RouteConfig{
			CaptureResponse: true,
			StreamValueFrom: "choices.0.delta.content",
			ToolCallsFrom:   []string{"choices.0.delta.vendor_tool_calls"},
		},
	}

	chunks := [][]byte{
		[]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\r\n\r\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"crlf \"}}]}\r\n\r\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"split"),
		[]byte(" cr \"}}]}\r" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"lf done\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":5,\"total_tokens\":17}}\n" +
			"data: [DONE]"),
	}
	for i, chunk := range chunks {
		got := onHttpResponseBody(ctx, cfg, chunk, i == len(chunks)-1, memoryCaptureNoopLog{})
		require.Equal(t, chunk, got)
	}

	capture, ok := ctx.GetContext(memoryResponseCaptureContextKey).(memoryResponseCapture)
	require.True(t, ok)
	require.True(t, capture.IsStream)
	require.False(t, capture.ParseFailed)
	require.Equal(t, 200, capture.StatusCode)
	require.Equal(t, "crlf split cr lf done", capture.AssistantContent)
	require.Equal(t, "stop", capture.FinishReason)
	require.False(t, capture.ContainsToolCalls)
	require.Equal(t, 12, capture.Usage.PromptTokens)
	require.Equal(t, 5, capture.Usage.CompletionTokens)
	require.Equal(t, 17, capture.Usage.TotalTokens)
}

func TestMemoryCaptureStreamingResponseChunkUsesConfiguredStreamValuePath(t *testing.T) {
	ctx := newMemoryCaptureTestContext()
	ctx.SetContext(memoryStreamContextKey, true)
	ctx.SetContext(memoryResponseStatusContextKey, 200)
	cfg := config.PluginConfig{
		Route: config.RouteConfig{
			CaptureResponse: true,
			StreamValueFrom: "vendor.delta.text",
		},
	}

	chunks := [][]byte{
		[]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
			"data: {\"vendor\":{\"delta\":{\"text\":\"configured \"}},\"choices\":[{\"delta\":{}}]}\n\n"),
		[]byte("data: {\"vendor\":{\"delta\":{\"text\":\"stream path\"}},\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":6,\"completion_tokens\":4,\"total_tokens\":10}}\n\n" +
			"data: [DONE]\n\n"),
	}
	for i, chunk := range chunks {
		got := onHttpResponseBody(ctx, cfg, chunk, i == len(chunks)-1, memoryCaptureNoopLog{})
		require.Equal(t, chunk, got)
	}

	capture, ok := ctx.GetContext(memoryResponseCaptureContextKey).(memoryResponseCapture)
	require.True(t, ok)
	require.True(t, capture.IsStream)
	require.False(t, capture.ParseFailed)
	require.Equal(t, 200, capture.StatusCode)
	require.Equal(t, "configured stream path", capture.AssistantContent)
	require.Equal(t, "stop", capture.FinishReason)
	require.False(t, capture.ContainsToolCalls)
	require.Equal(t, 6, capture.Usage.PromptTokens)
	require.Equal(t, 4, capture.Usage.CompletionTokens)
	require.Equal(t, 10, capture.Usage.TotalTokens)
}

func TestMemoryCaptureStreamingResponseChunkDetectsToolCalls(t *testing.T) {
	tests := []struct {
		name         string
		toolPaths    []string
		payload      string
		wantContent  string
		wantFinish   string
		wantPrompt   int
		wantComplete int
		wantTotal    int
	}{
		{
			name:         "canonical tool-call delta",
			payload:      `{"choices":[{"delta":{"content":"canonical preface","tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`,
			wantContent:  "canonical preface",
			wantFinish:   "stop",
			wantPrompt:   7,
			wantComplete: 3,
			wantTotal:    10,
		},
		{
			name:         "configured tool-call path",
			toolPaths:    []string{"choices.0.delta.vendor_tool_calls"},
			payload:      `{"choices":[{"delta":{"content":"configured preface","vendor_tool_calls":[{"id":"custom-call-1","name":"lookup_memory"}]},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":3,"total_tokens":11}}`,
			wantContent:  "configured preface",
			wantFinish:   "stop",
			wantPrompt:   8,
			wantComplete: 3,
			wantTotal:    11,
		},
		{
			name:         "finish reason only",
			payload:      `{"choices":[{"delta":{"content":"finish reason preface"},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":9,"completion_tokens":3,"total_tokens":12}}`,
			wantContent:  "finish reason preface",
			wantFinish:   "tool_calls",
			wantPrompt:   9,
			wantComplete: 3,
			wantTotal:    12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMemoryCaptureTestContext()
			ctx.SetContext(memoryStreamContextKey, true)
			ctx.SetContext(memoryResponseStatusContextKey, 201)
			cfg := config.PluginConfig{
				Route: config.RouteConfig{
					CaptureResponse: true,
					StreamValueFrom: "choices.0.delta.content",
					ToolCallsFrom:   tt.toolPaths,
				},
			}

			chunk := []byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
				"data: " + tt.payload + "\n\n" +
				"data: [DONE]\n\n")
			got := onHttpResponseBody(ctx, cfg, chunk, true, memoryCaptureNoopLog{})
			require.Equal(t, chunk, got)

			capture, ok := ctx.GetContext(memoryResponseCaptureContextKey).(memoryResponseCapture)
			require.True(t, ok)
			require.True(t, capture.IsStream)
			require.False(t, capture.ParseFailed)
			require.Equal(t, 201, capture.StatusCode)
			require.Equal(t, tt.wantContent, capture.AssistantContent)
			require.Equal(t, tt.wantFinish, capture.FinishReason)
			require.True(t, capture.ContainsToolCalls)
			require.Equal(t, tt.wantPrompt, capture.Usage.PromptTokens)
			require.Equal(t, tt.wantComplete, capture.Usage.CompletionTokens)
			require.Equal(t, tt.wantTotal, capture.Usage.TotalTokens)
		})
	}
}

func TestMemoryCaptureStreamingResponseChunkParseFailureFailsOpen(t *testing.T) {
	ctx := newMemoryCaptureTestContext()
	ctx.SetContext(memoryStreamContextKey, true)
	ctx.SetContext(memoryResponseStatusContextKey, 502)
	chunk := []byte("data: {not-json}\n\n")

	got := onHttpResponseBody(ctx, config.PluginConfig{}, chunk, true, memoryCaptureNoopLog{})

	require.Equal(t, chunk, got)
	capture, ok := ctx.GetContext(memoryResponseCaptureContextKey).(memoryResponseCapture)
	require.True(t, ok)
	require.True(t, capture.IsStream)
	require.True(t, capture.ParseFailed)
	require.Equal(t, 502, capture.StatusCode)
	require.Empty(t, capture.AssistantContent)
	require.Empty(t, capture.FinishReason)
	require.False(t, capture.ContainsToolCalls)
	require.Zero(t, capture.Usage)
}

func TestMemoryRawContentEligibilityGatesStreamingAssistantContent(t *testing.T) {
	ctx := newMemoryCaptureTestContext()
	ctx.SetContext(memoryStreamContextKey, true)
	ctx.SetContext(memoryResponseStatusContextKey, 206)
	ctx.SetContext(memoryUserContentContextKey, "stream raw user content")
	cfg := config.PluginConfig{
		Route: config.RouteConfig{
			CaptureResponse: false,
			StreamValueFrom: "choices.0.delta.content",
		},
	}

	chunk := []byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"stream raw assistant content must be omitted\",\"tool_calls\":[{\"index\":0,\"id\":\"tool-1\",\"type\":\"function\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":15,\"completion_tokens\":7,\"total_tokens\":22}}\n\n" +
		"data: [DONE]\n\n")

	got := onHttpResponseBody(ctx, cfg, chunk, true, memoryCaptureNoopLog{})

	require.Equal(t, chunk, got)
	capture, ok := ctx.GetContext(memoryResponseCaptureContextKey).(memoryResponseCapture)
	require.True(t, ok)
	require.True(t, capture.IsStream)
	require.False(t, capture.ParseFailed)
	require.Equal(t, 206, capture.StatusCode)
	require.Empty(t, capture.AssistantContent)
	require.Equal(t, "stop", capture.FinishReason)
	require.True(t, capture.ContainsToolCalls)
	require.Equal(t, 15, capture.Usage.PromptTokens)
	require.Equal(t, 7, capture.Usage.CompletionTokens)
	require.Equal(t, 22, capture.Usage.TotalTokens)
	require.Empty(t, memoryEventUserContent(ctx, cfg))
	require.Equal(t, "stream raw user content", ctx.GetStringContext(memoryUserContentContextKey, ""))
}

type memoryCaptureTestContext struct {
	values               map[string]interface{}
	dontReadResponseBody bool
}

func newMemoryCaptureTestContext() *memoryCaptureTestContext {
	return &memoryCaptureTestContext{values: map[string]interface{}{memoryProtocolContextKey: "chat_completions"}}
}

func requireMemoryHostNeedsResponseBody(t *testing.T, host test.TestHost) {
	t.Helper()
	hostValue := reflect.ValueOf(host)
	for hostValue.Kind() == reflect.Interface || hostValue.Kind() == reflect.Pointer {
		hostValue = hostValue.Elem()
	}
	contextID := uint32(hostValue.FieldByName("currentContextID").Uint())
	httpContext := proxywasm.GetHttpContext(contextID)
	contextValue := reflect.ValueOf(httpContext)
	for contextValue.Kind() == reflect.Interface || contextValue.Kind() == reflect.Pointer {
		contextValue = contextValue.Elem()
	}
	field := contextValue.FieldByName("needResponseBody")
	require.True(t, field.IsValid(), "test host HTTP context must expose needResponseBody")
	require.True(t, field.Bool(), "stream response headers must not disable response body processing")
}

func (m *memoryCaptureTestContext) Scheme() string { return "" }
func (m *memoryCaptureTestContext) Host() string   { return "" }
func (m *memoryCaptureTestContext) Path() string   { return "" }
func (m *memoryCaptureTestContext) Method() string { return "" }
func (m *memoryCaptureTestContext) SetContext(key string, value interface{}) {
	m.values[key] = value
}
func (m *memoryCaptureTestContext) GetContext(key string) interface{} { return m.values[key] }
func (m *memoryCaptureTestContext) GetBoolContext(key string, defaultValue bool) bool {
	if value, ok := m.values[key].(bool); ok {
		return value
	}
	return defaultValue
}
func (m *memoryCaptureTestContext) GetStringContext(key, defaultValue string) string {
	if value, ok := m.values[key].(string); ok {
		return value
	}
	return defaultValue
}
func (m *memoryCaptureTestContext) GetByteSliceContext(key string, defaultValue []byte) []byte {
	if value, ok := m.values[key].([]byte); ok {
		return value
	}
	return defaultValue
}
func (m *memoryCaptureTestContext) GetUserAttribute(key string) interface{} { return nil }
func (m *memoryCaptureTestContext) SetUserAttribute(key string, value interface{}) {
}
func (m *memoryCaptureTestContext) SetUserAttributeMap(kvmap map[string]interface{}) {}
func (m *memoryCaptureTestContext) GetUserAttributeMap() map[string]interface{} {
	return nil
}
func (m *memoryCaptureTestContext) WriteUserAttributeToLog() error                  { return nil }
func (m *memoryCaptureTestContext) WriteUserAttributeToLogWithKey(key string) error { return nil }
func (m *memoryCaptureTestContext) WriteUserAttributeToTrace() error                { return nil }
func (m *memoryCaptureTestContext) DontReadRequestBody()                            {}
func (m *memoryCaptureTestContext) DontReadResponseBody() {
	m.dontReadResponseBody = true
}
func (m *memoryCaptureTestContext) BufferRequestBody()                         {}
func (m *memoryCaptureTestContext) BufferResponseBody()                        {}
func (m *memoryCaptureTestContext) NeedPauseStreamingResponse()                {}
func (m *memoryCaptureTestContext) PushBuffer(buffer []byte)                   {}
func (m *memoryCaptureTestContext) PopBuffer() []byte                          { return nil }
func (m *memoryCaptureTestContext) BufferQueueSize() int                       { return 0 }
func (m *memoryCaptureTestContext) DisableReroute()                            {}
func (m *memoryCaptureTestContext) SetRequestBodyBufferLimit(byteSize uint32)  {}
func (m *memoryCaptureTestContext) SetResponseBodyBufferLimit(byteSize uint32) {}
func (m *memoryCaptureTestContext) RouteCall(method, url string, headers [][2]string, body []byte, callback iface.RouteResponseCallback) error {
	return nil
}
func (m *memoryCaptureTestContext) GetExecutionPhase() iface.HTTPExecutionPhase {
	return iface.EncodeData
}
func (m *memoryCaptureTestContext) HasRequestBody() bool       { return false }
func (m *memoryCaptureTestContext) HasResponseBody() bool      { return true }
func (m *memoryCaptureTestContext) IsWebsocket() bool          { return false }
func (m *memoryCaptureTestContext) IsBinaryRequestBody() bool  { return false }
func (m *memoryCaptureTestContext) IsBinaryResponseBody() bool { return false }

type memoryCaptureNoopLog struct{}

func (memoryCaptureNoopLog) Trace(msg string)                          {}
func (memoryCaptureNoopLog) Tracef(format string, args ...interface{}) {}
func (memoryCaptureNoopLog) Debug(msg string)                          {}
func (memoryCaptureNoopLog) Debugf(format string, args ...interface{}) {}
func (memoryCaptureNoopLog) Info(msg string)                           {}
func (memoryCaptureNoopLog) Infof(format string, args ...interface{})  {}
func (memoryCaptureNoopLog) Warn(msg string)                           {}
func (memoryCaptureNoopLog) Warnf(format string, args ...interface{})  {}
func (memoryCaptureNoopLog) Error(msg string)                          {}
func (memoryCaptureNoopLog) Errorf(format string, args ...interface{}) {}
func (memoryCaptureNoopLog) Critical(msg string)                       {}
func (memoryCaptureNoopLog) Criticalf(format string, args ...interface{}) {
}
func (memoryCaptureNoopLog) ResetID(pluginID string) {}
