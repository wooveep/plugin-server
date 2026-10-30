package sessionctx_test

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/stretchr/testify/require"
)

func TestRequestFactsExtraction(t *testing.T) {
	t.Run("custom identity and session headers override defaults", func(t *testing.T) {
		headers := [][2]string{
			{"X-Tenant-ID", "tenant-a"},
			{"x-consumer-id", "consumer-a"},
			{"x-session-id", "session-custom"},
			{"x-openclaw-session-key", "session-default"},
			{"x-request-id", "header-request-id"},
		}
		facts := sessionctx.ExtractRequestFacts(headers, sessionctx.RequestFactOptions{
			TenantHeader:    "x-tenant-id",
			ConsumerHeader:  "x-consumer-id",
			SessionHeader:   "x-session-id",
			RequestIDHeader: "x-request-id",
		}, "property-request-id")

		require.Equal(t, "tenant-a", facts.Tenant)
		require.Equal(t, "consumer-a", facts.Consumer)
		require.Equal(t, "session-custom", facts.SessionID)
		require.Equal(t, "header-request-id", facts.RequestID)
		require.Equal(t, "tenant-a", sessionctx.HeaderValue(headers, "x-tenant-id"))
	})

	t.Run("default headers and request property fallback are stable", func(t *testing.T) {
		facts := sessionctx.ExtractRequestFacts([][2]string{
			{"X-Mse-Tenant", "tenant-default"},
			{"X-Mse-Consumer", "consumer-default"},
			{"x-agent-session", "session-agent"},
			{"x-openclaw-session-key", "session-openclaw"},
		}, sessionctx.RequestFactOptions{}, "property-request-id")

		require.Equal(t, "tenant-default", facts.Tenant)
		require.Equal(t, "consumer-default", facts.Consumer)
		require.Equal(t, "session-openclaw", facts.SessionID)
		require.Equal(t, "property-request-id", facts.RequestID)
	})
}

func TestRequestGateHelpers(t *testing.T) {
	require.True(t, sessionctx.IsJSONContentType("application/json"))
	require.True(t, sessionctx.IsJSONContentType("application/json; charset=utf-8"))
	require.True(t, sessionctx.IsJSONContentType("application/problem+json"))
	require.False(t, sessionctx.IsJSONContentType("text/plain"))

	require.True(t, sessionctx.PathMatchesSuffixes("/v1/chat/completions", []string{"/chat/completions"}))
	require.True(t, sessionctx.PathMatchesSuffixes("/v1/chat/completions?api-version=2024-01-01", []string{"/chat/completions"}))
	require.True(t, sessionctx.PathMatchesSuffixes("/v1/chat/completions", nil))
	require.False(t, sessionctx.PathMatchesSuffixes("/v1/embeddings", []string{"/chat/completions"}))
	require.False(t, sessionctx.PathMatchesSuffixes("/v1/embeddings", []string{""}))
}

func TestOpenAIRequestParsingIntentAndDigest(t *testing.T) {
	body := fullOpenAIChatRequestBody()

	request, err := sessionctx.ParseOpenAIChatRequest(body)
	require.NoError(t, err)
	require.Equal(t, "qwen-turbo", request.Model)
	require.True(t, request.Stream)
	require.Len(t, request.Messages, 4)
	require.NotEmpty(t, request.Tools)
	require.NotEmpty(t, request.ToolChoice)
	require.NotEmpty(t, request.ResponseFormat)
	require.NotEmpty(t, request.Temperature)
	require.NotEmpty(t, request.TopP)
	require.NotEmpty(t, request.Seed)
	require.NotEmpty(t, request.MaxTokens)
	require.NotEmpty(t, request.MaxCompletion)
	require.NotEmpty(t, request.Stop)
	require.NotEmpty(t, request.N)
	require.NotEmpty(t, request.PresencePenalty)
	require.NotEmpty(t, request.FrequencyPenalty)
	require.NotEmpty(t, request.LogitBias)
	require.NotEmpty(t, request.Logprobs)
	require.NotEmpty(t, request.TopLogprobs)
	require.NotEmpty(t, request.ParallelToolCalls)
	require.Contains(t, request.Extra, "vendor_extension")
	require.Equal(t, "latest weather intent", sessionctx.CurrentUserIntent(request.Messages))

	digestA, err := sessionctx.BuildRequestDigest(openAIRequestDigestInput(request))
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{64}$`), digestA)

	sameRequest, err := sessionctx.ParseOpenAIChatRequest(fullOpenAIChatRequestReorderedBody())
	require.NoError(t, err)
	digestB, err := sessionctx.BuildRequestDigest(openAIRequestDigestInput(sameRequest))
	require.NoError(t, err)
	require.Equal(t, digestA, digestB)

	changedModel, err := sessionctx.BuildRequestDigest(sessionctx.RequestDigestInput{
		Model:    "qwen-plus",
		Messages: sameRequest.Messages,
	})
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedModel)

	changedToolChoiceInput := openAIRequestDigestInput(request)
	changedToolChoiceInput.ToolChoice = jsonRaw(`{"type":"function","function":{"name":"lookup_other"}}`)
	changedToolChoice, err := sessionctx.BuildRequestDigest(changedToolChoiceInput)
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedToolChoice)

	changedTemperatureInput := openAIRequestDigestInput(request)
	changedTemperatureInput.Temperature = jsonRaw(`0.7`)
	changedTemperature, err := sessionctx.BuildRequestDigest(changedTemperatureInput)
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedTemperature)

	changedPenaltyInput := openAIRequestDigestInput(request)
	changedPenaltyInput.PresencePenalty = jsonRaw(`0.6`)
	changedPenalty, err := sessionctx.BuildRequestDigest(changedPenaltyInput)
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedPenalty)

	changedVendorInput := openAIRequestDigestInput(request)
	changedVendorInput.Extra = map[string]json.RawMessage{"vendor_extension": jsonRaw(`{"a":2,"b":2}`)}
	changedVendor, err := sessionctx.BuildRequestDigest(changedVendorInput)
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedVendor)

	largeSeedInputA := openAIRequestDigestInput(request)
	largeSeedInputA.Seed = jsonRaw(`9007199254740993`)
	largeSeedInputB := openAIRequestDigestInput(request)
	largeSeedInputB.Seed = jsonRaw(`9007199254740992`)
	largeSeedDigestA, err := sessionctx.BuildRequestDigest(largeSeedInputA)
	require.NoError(t, err)
	largeSeedDigestB, err := sessionctx.BuildRequestDigest(largeSeedInputB)
	require.NoError(t, err)
	require.NotEqual(t, largeSeedDigestA, largeSeedDigestB)

	toolMessageDigestA, err := sessionctx.BuildRequestDigest(sessionctx.RequestDigestInput{
		Model: "qwen-turbo",
		Messages: []sessionctx.OpenAIMessage{{
			Role:  "assistant",
			Extra: map[string]json.RawMessage{"vendor_message": jsonRaw(`{"b":2,"a":1}`)},
			ToolCalls: jsonRaw(
				`[{"type":"function","id":"call-1","function":{"arguments":"{}","name":"lookup"}}]`,
			),
		}},
	})
	require.NoError(t, err)
	toolMessageDigestB, err := sessionctx.BuildRequestDigest(sessionctx.RequestDigestInput{
		Model: "qwen-turbo",
		Messages: []sessionctx.OpenAIMessage{{
			Role:  "assistant",
			Extra: map[string]json.RawMessage{"vendor_message": jsonRaw(`{"a":1,"b":2}`)},
			ToolCalls: jsonRaw(
				`[{"function":{"name":"lookup","arguments":"{}"},"id":"call-1","type":"function"}]`,
			),
		}},
	})
	require.NoError(t, err)
	require.Equal(t, toolMessageDigestA, toolMessageDigestB)
}

func TestOpenAIRequestDigestConvenience(t *testing.T) {
	bodyA := fullOpenAIChatRequestBody()
	bodyB := fullOpenAIChatRequestReorderedBody()

	requestA, err := sessionctx.ParseOpenAIChatRequest(bodyA)
	require.NoError(t, err)
	requestB, err := sessionctx.ParseOpenAIChatRequest(bodyB)
	require.NoError(t, err)
	requestB.Stream = false

	digestA, err := sessionctx.BuildOpenAIChatRequestDigest(requestA)
	require.NoError(t, err)
	digestB, err := sessionctx.BuildOpenAIChatRequestDigest(requestB)
	require.NoError(t, err)
	require.Equal(t, digestA, digestB, "streaming flag and JSON field order should not change the request digest")

	baselineDigest, err := sessionctx.BuildRequestDigest(openAIRequestDigestInput(requestA))
	require.NoError(t, err)
	require.Equal(t, baselineDigest, digestA, "convenience helper must preserve the full parsed request mapping")

	requestB.Model = "qwen-plus"
	changedDigest, err := sessionctx.BuildOpenAIChatRequestDigest(requestB)
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedDigest)
}

func TestScopedRequestDigest(t *testing.T) {
	request, err := sessionctx.ParseOpenAIChatRequest(fullOpenAIChatRequestBody())
	require.NoError(t, err)
	bodyDigest, err := sessionctx.BuildOpenAIChatRequestDigest(request)
	require.NoError(t, err)

	input := sessionctx.ScopedRequestDigestInput{
		Tenant:      "tenant-a",
		Consumer:    "consumer-a",
		SessionID:   "session-a",
		Route:       "route-a",
		RequestPath: "/v1/chat/completions",
		Model:       "qwen-turbo",
		BodyDigest:  bodyDigest,
	}
	digestA, err := sessionctx.BuildScopedRequestDigest(input)
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{64}$`), digestA)

	withRequestID := input
	withRequestID.RequestID = "request-a"
	digestB, err := sessionctx.BuildScopedRequestDigest(withRequestID)
	require.NoError(t, err)
	require.Equal(t, digestA, digestB, "request_id must not affect scoped request digest")

	withDifferentRequestID := input
	withDifferentRequestID.RequestID = "request-b"
	digestC, err := sessionctx.BuildScopedRequestDigest(withDifferentRequestID)
	require.NoError(t, err)
	require.Equal(t, digestA, digestC, "retries with identical request facts must keep the same digest")

	changedSession := input
	changedSession.SessionID = "session-b"
	sessionDigest, err := sessionctx.BuildScopedRequestDigest(changedSession)
	require.NoError(t, err)
	require.NotEqual(t, digestA, sessionDigest)

	changedPath := input
	changedPath.RequestPath = "/v1/messages"
	pathDigest, err := sessionctx.BuildScopedRequestDigest(changedPath)
	require.NoError(t, err)
	require.NotEqual(t, digestA, pathDigest)

	changedBody := input
	changedBody.BodyDigest = strings.Repeat("a", 64)
	changedBodyDigest, err := sessionctx.BuildScopedRequestDigest(changedBody)
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedBodyDigest)
}

func TestOpenAIRequestBodyReplacement(t *testing.T) {
	original := []byte(`{
		"model": "qwen-turbo",
		"stream": true,
		"metadata": {"trace": "keep"},
		"messages": [{"role": "user", "content": "original"}]
	}`)
	replaced, err := sessionctx.ReplaceOpenAIChatMessages(original, []sessionctx.OpenAIMessage{
		{Role: "system", Content: "memory context"},
		{
			Role:         "assistant",
			Content:      "calling tool",
			Name:         "assistant-name",
			ToolCallID:   "call-1",
			ToolCalls:    jsonRaw(`[{"id":"call-1","type":"function","function":{"name":"lookup","arguments":"{}"}}]`),
			FunctionCall: jsonRaw(`{"name":"legacy_lookup","arguments":"{}"}`),
			Refusal:      jsonRaw(`"no"`),
			Annotations:  jsonRaw(`[{"type":"url_citation","url":"https://example.invalid"}]`),
			Audio:        jsonRaw(`{"id":"audio-1"}`),
			Extra:        map[string]json.RawMessage{"vendor_message": jsonRaw(`{"b":2,"a":1}`)},
		},
	})
	require.NoError(t, err)

	request, err := sessionctx.ParseOpenAIChatRequest(replaced)
	require.NoError(t, err)
	require.Equal(t, "qwen-turbo", request.Model)
	require.True(t, request.Stream)
	require.Len(t, request.Messages, 2)
	require.Equal(t, "assistant-name", request.Messages[1].Name)
	require.Equal(t, "call-1", request.Messages[1].ToolCallID)
	require.NotEmpty(t, request.Messages[1].ToolCalls)
	require.NotEmpty(t, request.Messages[1].FunctionCall)
	require.NotEmpty(t, request.Messages[1].Refusal)
	require.NotEmpty(t, request.Messages[1].Annotations)
	require.NotEmpty(t, request.Messages[1].Audio)
	require.Contains(t, request.Messages[1].Extra, "vendor_message")
	require.JSONEq(t, `{"trace":"keep"}`, string(jsonField(t, replaced, "metadata")))

	preservedRequest, err := sessionctx.ParseOpenAIChatRequest([]byte(`{
		"messages": [{
			"role": "assistant",
			"content": null,
			"refusal": "policy",
			"annotations": [{"type": "url_citation", "url": "https://example.invalid"}],
			"audio": {"id": "audio-1"},
			"vendor_message": {"b": 2, "a": 1}
		}]
	}`))
	require.NoError(t, err)
	preserved, err := sessionctx.ReplaceOpenAIChatMessages(original, preservedRequest.Messages)
	require.NoError(t, err)
	require.Equal(t, "null", string(messageField(t, preserved, 0, "content")))
	require.JSONEq(t, `"policy"`, string(messageField(t, preserved, 0, "refusal")))
	require.JSONEq(t, `[{"type":"url_citation","url":"https://example.invalid"}]`, string(messageField(t, preserved, 0, "annotations")))
	require.JSONEq(t, `{"id":"audio-1"}`, string(messageField(t, preserved, 0, "audio")))
	require.JSONEq(t, `{"a":1,"b":2}`, string(messageField(t, preserved, 0, "vendor_message")))

	emptyMessages, err := sessionctx.ReplaceOpenAIChatMessages(original, nil)
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(jsonField(t, emptyMessages, "messages")))

	_, err = sessionctx.ReplaceOpenAIChatMessages([]byte(`null`), nil)
	require.Error(t, err)
	_, err = sessionctx.ReplaceOpenAIChatMessages([]byte(`[]`), nil)
	require.Error(t, err)
}

func TestOpenAIResponseParsing(t *testing.T) {
	response, err := sessionctx.ParseOpenAIChatResponse([]byte(`{
		"id": "chatcmpl-1",
		"model": "qwen-turbo",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "assistant answer"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 9, "completion_tokens": 4, "total_tokens": 13}
	}`))
	require.NoError(t, err)
	require.Equal(t, "assistant answer", response.AssistantContent)
	require.Equal(t, "stop", response.FinishReason)
	require.Equal(t, sessionctx.Usage{
		PromptTokens:     9,
		CompletionTokens: 4,
		TotalTokens:      13,
	}, response.Usage)
	require.False(t, response.ContainsToolCalls)

	toolResponse, err := sessionctx.ParseOpenAIChatResponse([]byte(`{
		"choices": [{
			"message": {
				"role": "assistant",
				"tool_calls": [{"id": "call-1", "type": "function", "function": {"name": "lookup", "arguments": "{}"}}]
			},
			"finish_reason": "tool_calls"
		}]
	}`))
	require.NoError(t, err)
	require.True(t, toolResponse.ContainsToolCalls)
	require.Equal(t, "tool_calls", toolResponse.FinishReason)

	functionResponse, err := sessionctx.ParseOpenAIChatResponse([]byte(`{
		"choices": [{
			"message": {
				"role": "assistant",
				"function_call": {"name": "legacy_lookup", "arguments": "{}"}
			},
			"finish_reason": "function_call"
		}]
	}`))
	require.NoError(t, err)
	require.True(t, functionResponse.ContainsToolCalls)

	require.True(t, sessionctx.ContainsToolUse(functionResponse))
	require.Equal(t, sessionctx.Usage{}, sessionctx.ResponseUsage(functionResponse))
	require.Equal(t, "function_call", sessionctx.ResponseFinishReason(functionResponse))

	multiChoiceResponse, err := sessionctx.ParseOpenAIChatResponse([]byte(`{
		"choices": [
			{
				"index": 0,
				"message": {"role": "assistant", "content": "first answer"},
				"finish_reason": "stop"
			},
			{
				"index": 1,
				"message": {
					"role": "assistant",
					"tool_calls": [{"id": "call-1", "type": "function", "function": {"name": "lookup", "arguments": "{}"}}]
				},
				"finish_reason": "tool_calls"
			}
		]
	}`))
	require.NoError(t, err)
	require.Equal(t, "first answer", multiChoiceResponse.AssistantContent)
	require.Equal(t, "stop", multiChoiceResponse.FinishReason)
	require.Equal(t, []string{"stop", "tool_calls"}, multiChoiceResponse.FinishReasons)
	require.True(t, sessionctx.ContainsToolUse(multiChoiceResponse))
	require.Equal(t, "stop,tool_calls", sessionctx.ResponseFinishReason(multiChoiceResponse))
}

func TestOpenAIResponseParsingWithAdditionalToolCallPaths(t *testing.T) {
	body := []byte(`{
		"choices": [
			{
				"message": {
					"role": "assistant",
					"content": "vendor tool path preface"
				},
				"finish_reason": "stop"
			},
			{
				"message": {
					"role": "assistant",
					"custom_tool_calls": [{"id": "custom-call-1", "name": "lookup_memory"}]
				},
				"finish_reason": "stop"
			}
		],
		"usage": {"prompt_tokens": 10, "completion_tokens": 3, "total_tokens": 13}
	}`)

	defaultResponse, err := sessionctx.ParseOpenAIChatResponse(body)
	require.NoError(t, err)
	require.False(t, defaultResponse.ContainsToolCalls)
	require.Equal(t, "vendor tool path preface", defaultResponse.AssistantContent)

	customResponse, err := sessionctx.ParseOpenAIChatResponseWithOptions(body, sessionctx.ResponseParseOptions{
		AdditionalToolCallPaths: []string{"choices.1.message.custom_tool_calls"},
	})
	require.NoError(t, err)
	require.True(t, customResponse.ContainsToolCalls)
	require.Equal(t, "vendor tool path preface", customResponse.AssistantContent)
	require.Equal(t, "stop", customResponse.FinishReason)
	require.Equal(t, sessionctx.Usage{
		PromptTokens:     10,
		CompletionTokens: 3,
		TotalTokens:      13,
	}, customResponse.Usage)

	canonicalResponse, err := sessionctx.ParseOpenAIChatResponseWithOptions([]byte(`{
		"choices": [
			{
				"message": {
					"role": "assistant",
					"content": "canonical tool path preface"
				},
				"finish_reason": "stop"
			},
			{
				"message": {
					"role": "assistant",
					"tool_calls": [{"id":"call-1","type":"function","function":{"name":"lookup","arguments":"{}"}}]
				},
				"finish_reason": "stop"
			}
		]
	}`), sessionctx.ResponseParseOptions{
		AdditionalToolCallPaths: []string{"choices.1.message.custom_tool_calls"},
	})
	require.NoError(t, err)
	require.True(t, canonicalResponse.ContainsToolCalls, "additional paths must not replace built-in OpenAI tool-call detection")

	for _, tc := range []struct {
		name  string
		value string
	}{
		{name: "false", value: `false`},
		{name: "zero", value: `0`},
		{name: "decimal zero", value: `0.0`},
		{name: "exponent zero", value: `0e0`},
		{name: "negative zero", value: `-0`},
		{name: "null", value: `null`},
		{name: "empty array", value: `[]`},
		{name: "empty object", value: `{}`},
		{name: "empty string", value: `""`},
	} {
		t.Run("does not treat "+tc.name+" configured path value as tool calls", func(t *testing.T) {
			response, err := sessionctx.ParseOpenAIChatResponseWithOptions([]byte(`{
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "non tool response",
						"custom_tool_calls": `+tc.value+`
					},
					"finish_reason": "stop"
				}]
			}`), sessionctx.ResponseParseOptions{
				AdditionalToolCallPaths: []string{"choices.0.message.custom_tool_calls"},
			})
			require.NoError(t, err)
			require.False(t, response.ContainsToolCalls)
		})
	}
}

func TestStreamCapture(t *testing.T) {
	capture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})

	require.NoError(t, capture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n")))
	require.NoError(t, capture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"world\"},\"finish_reason\":\"stop\"}]}\n\n")))
	require.NoError(t, capture.AppendSSE([]byte("data: [DONE]\n\n")))

	require.Equal(t, "hello world", capture.AssistantContent())
	require.Equal(t, "stop", capture.FinishReason())
	require.False(t, capture.ContainsToolCalls())

	toolCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})
	require.NoError(t, toolCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}]}}]}\n\n")))
	require.True(t, toolCapture.ContainsToolCalls())

	functionCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})
	require.NoError(t, functionCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"function_call\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}}]}\n\n")))
	require.True(t, functionCapture.ContainsToolCalls())

	contentToolCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})
	require.NoError(t, contentToolCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":{\"tool_calls\":[{\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}]}}}]}\n\n")))
	require.True(t, contentToolCapture.ContainsToolCalls())

	splitCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})
	require.NoError(t, splitCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"split")))
	require.NoError(t, splitCapture.AppendSSE([]byte(" chunk\"},\"finish_reason\":\"stop\"}]}\n\n")))
	require.Equal(t, "split chunk", splitCapture.AssistantContent())
	require.Equal(t, "stop", splitCapture.FinishReason())

	crCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})
	require.NoError(t, crCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"cr \"}}]}\rdata: {\"choices\":[{\"delta\":{\"content\":\"line\"},\"finish_reason\":\"stop\"}]}\rdata: [DONE]\r")))
	require.Equal(t, "cr line", crCapture.AssistantContent())
	require.Equal(t, "stop", crCapture.FinishReason())
}

func TestStreamCaptureUsageFinishReasonAndCustomToolPaths(t *testing.T) {
	finishReasonCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})
	require.NoError(t, finishReasonCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")))
	require.NoError(t, finishReasonCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"finish reason tool preface\"},\"finish_reason\":\"function_call\"}],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"total_tokens\":10}}\n\n")))

	require.Equal(t, "finish reason tool preface", finishReasonCapture.AssistantContent())
	require.Equal(t, "function_call", finishReasonCapture.FinishReason())
	require.True(t, finishReasonCapture.ContainsToolCalls())
	require.Equal(t, sessionctx.Usage{
		PromptTokens:     7,
		CompletionTokens: 3,
		TotalTokens:      10,
	}, finishReasonCapture.Usage())

	customPathCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{
		AdditionalToolCallPaths: []string{"choices.0.delta.vendor_tool_calls"},
	})
	require.NoError(t, customPathCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"custom path preface\",\"vendor_tool_calls\":[{\"id\":\"custom-call-1\",\"name\":\"lookup_memory\"}]},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":3,\"total_tokens\":11}}\r\n\r\n")))

	require.Equal(t, "custom path preface", customPathCapture.AssistantContent())
	require.Equal(t, "stop", customPathCapture.FinishReason())
	require.True(t, customPathCapture.ContainsToolCalls())
	require.Equal(t, sessionctx.Usage{
		PromptTokens:     8,
		CompletionTokens: 3,
		TotalTokens:      11,
	}, customPathCapture.Usage())

	toolCallsFinishCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})
	require.NoError(t, toolCallsFinishCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")))
	require.True(t, toolCallsFinishCapture.ContainsToolCalls())

	canonicalWithOptionsCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{
		AdditionalToolCallPaths: []string{"choices.0.delta.vendor_tool_calls"},
	})
	require.NoError(t, canonicalWithOptionsCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}]}}]}\n\n")))
	require.True(t, canonicalWithOptionsCapture.ContainsToolCalls(), "additional paths must not replace built-in stream tool-call detection")

	emptyPathCapture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{
		AdditionalToolCallPaths: []string{"choices.0.delta.vendor_tool_calls"},
	})
	require.NoError(t, emptyPathCapture.AppendSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"no tool\",\"vendor_tool_calls\":false},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\n")))
	require.False(t, emptyPathCapture.ContainsToolCalls())
	require.Equal(t, sessionctx.Usage{
		PromptTokens:     1,
		CompletionTokens: 1,
		TotalTokens:      2,
	}, emptyPathCapture.Usage())
}

func TestEventEnvelopeAndSafeLogRedaction(t *testing.T) {
	input := sessionctx.EventEnvelopeInput{
		EventKind:     "cache",
		Tenant:        "tenant-a",
		Consumer:      "consumer-a",
		RequestID:     "request-a",
		RequestDigest: "digest-a",
		StartedAtMS:   1000,
		EndedAtMS:     1500,
	}
	event := sessionctx.NewEventEnvelope(input)
	repeated := sessionctx.NewEventEnvelope(input)
	delayed := sessionctx.NewEventEnvelope(sessionctx.EventEnvelopeInput{
		EventKind:     "cache",
		Tenant:        "tenant-a",
		Consumer:      "consumer-a",
		RequestID:     "request-a",
		RequestDigest: "digest-a",
		StartedAtMS:   2000,
		EndedAtMS:     2500,
	})

	require.NotEmpty(t, event.EventID)
	require.Equal(t, "cache", event.EventKind)
	require.Equal(t, "tenant-a", event.Tenant)
	require.Equal(t, "consumer-a", event.Consumer)
	require.Equal(t, "request-a", event.RequestID)
	require.Equal(t, "digest-a", event.RequestDigest)
	require.EqualValues(t, 1000, event.StartedAtMS)
	require.EqualValues(t, 1500, event.EndedAtMS)
	require.Equal(t, event.IdempotencyKey, repeated.IdempotencyKey)
	require.Equal(t, event.IdempotencyKey, delayed.IdempotencyKey)

	redacted := sessionctx.RedactForLog(`authorization=Bearer credential-one x-api-key: credential-two "x-internal-bearer":"credential-three" x-redis-password=credential-four x-provider-api-key=credential-five route=chat`)
	for _, item := range []struct {
		label string
		value string
	}{
		{label: "authorization value", value: "credential-one"},
		{label: "api key value", value: "credential-two"},
		{label: "internal bearer value", value: "credential-three"},
		{label: "redis credential value", value: "credential-four"},
		{label: "provider credential value", value: "credential-five"},
	} {
		requireStringExcludes(t, redacted, item.label, item.value)
	}
	require.Contains(t, redacted, "route=chat")
	require.Contains(t, strings.ToLower(redacted), "redacted")

	sensitiveRedacted := sessionctx.RedactForLog(
		`authorization=Bearer credential-six user_content="raw user memory" assistant_content="raw assistant memory" route=chat`,
		"raw user memory",
		"raw assistant memory",
	)
	requireStringExcludes(t, sensitiveRedacted, "authorization value", "credential-six")
	requireStringExcludes(t, sensitiveRedacted, "raw user content", "raw user memory")
	requireStringExcludes(t, sensitiveRedacted, "raw assistant content", "raw assistant memory")
	require.Contains(t, sensitiveRedacted, "route=chat")

	spacedQuotedKeyRedacted := sessionctx.RedactForLog(`"authorization" : "Bearer credential-seven" route=chat`)
	requireStringExcludes(t, spacedQuotedKeyRedacted, "quoted authorization value", "credential-seven")
	require.Contains(t, spacedQuotedKeyRedacted, "route=chat")

	overlapRedacted := sessionctx.RedactForLog(
		`user_content="raw secret suffix" assistant_content="raw secret" route=chat`,
		"raw secret",
		"raw secret suffix",
	)
	requireStringExcludes(t, overlapRedacted, "long overlapping secret suffix", "suffix")
	requireStringExcludes(t, overlapRedacted, "short overlapping secret", "raw secret")
	require.Contains(t, overlapRedacted, "route=chat")
}

func TestRedisStreamEnvelopeAndFailOpenLogging(t *testing.T) {
	event := sessionctx.NewEventEnvelope(sessionctx.EventEnvelopeInput{
		EventKind:     "memory",
		Tenant:        "tenant-a",
		Consumer:      "consumer-a",
		RequestID:     "request-a",
		RequestDigest: "digest-a",
		StartedAtMS:   1000,
		EndedAtMS:     1500,
	})

	command, err := sessionctx.BuildRedisStreamXADD(sessionctx.RedisStreamXADDInput{
		Stream:         "memory:events",
		MaxLen:         1000,
		ApproximateMax: true,
		Event:          event,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"XADD", "memory:events", "MAXLEN", "~", "1000", "*", "event"}, command[:7])
	require.JSONEq(t, `{
		"event_id": "`+event.EventID+`",
		"idempotency_key": "`+event.IdempotencyKey+`",
		"event_kind": "memory",
		"tenant": "tenant-a",
		"consumer": "consumer-a",
		"request_id": "request-a",
		"request_digest": "digest-a",
		"started_at_ms": 1000,
		"ended_at_ms": 1500
	}`, command[7])

	command, err = sessionctx.BuildRedisStreamXADD(sessionctx.RedisStreamXADDInput{
		Stream: "cache:events",
		ID:     "123-0",
		Field:  "payload",
		Event:  json.RawMessage(`{"event_kind":"cache"}`),
	})
	require.NoError(t, err)
	require.Equal(t, []string{"XADD", "cache:events", "123-0", "payload", `{"event_kind":"cache"}`}, command)

	_, err = sessionctx.BuildRedisStreamXADD(sessionctx.RedisStreamXADDInput{Event: event})
	require.Error(t, err)
	_, err = sessionctx.BuildRedisStreamXADD(sessionctx.RedisStreamXADDInput{Stream: "memory:events"})
	require.Error(t, err)
	var nilRaw json.RawMessage
	_, err = sessionctx.BuildRedisStreamXADD(sessionctx.RedisStreamXADDInput{
		Stream: "memory:events",
		Event:  nilRaw,
	})
	require.Error(t, err)
	var nilEnvelope *sessionctx.EventEnvelope
	_, err = sessionctx.BuildRedisStreamXADD(sessionctx.RedisStreamXADDInput{
		Stream: "memory:events",
		Event:  nilEnvelope,
	})
	require.Error(t, err)
	_, err = sessionctx.BuildRedisStreamXADD(sessionctx.RedisStreamXADDInput{
		Stream: "memory:events",
		Event:  make(chan struct{}),
	})
	require.Error(t, err)

	fields := sessionctx.FailOpenLogFields(sessionctx.FailOpenLogInput{
		Operation:  "redis_stream_xadd",
		Reason:     "timeout",
		RequestID:  "request-a",
		Route:      "/v1/chat/completions",
		Model:      "qwen-turbo",
		Stream:     "cache:events",
		StatusCode: 504,
		Err:        errors.New("x-redis-password=credential-seven raw assistant secret"),
	}, "raw assistant secret")
	require.Equal(t, "true", logFieldValue(fields, "fail_open"))
	require.Equal(t, "redis_stream_xadd", logFieldValue(fields, "operation"))
	require.Equal(t, "timeout", logFieldValue(fields, "reason"))
	require.Equal(t, "request-a", logFieldValue(fields, "request_id"))
	require.Equal(t, "/v1/chat/completions", logFieldValue(fields, "route"))
	require.Equal(t, "qwen-turbo", logFieldValue(fields, "model"))
	require.Equal(t, "cache:events", logFieldValue(fields, "stream"))
	require.Equal(t, "504", logFieldValue(fields, "status_code"))
	requireStringExcludes(t, logFieldValue(fields, "error"), "redis credential", "credential-seven")
	requireStringExcludes(t, logFieldValue(fields, "error"), "assistant content", "raw assistant secret")
}

func requireStringExcludes(t *testing.T, text, label, value string) {
	t.Helper()
	require.Falsef(t, strings.Contains(text, value), "redacted output leaked %s", label)
}

func fullOpenAIChatRequestBody() []byte {
	return []byte(`{
		"model": "qwen-turbo",
		"stream": true,
		"tools": [{"type":"function","function":{"name":"lookup_weather","parameters":{"type":"object"}}}],
		"tool_choice": {"function":{"name":"lookup_weather"},"type":"function"},
		"response_format": {"type":"json_object"},
		"temperature": 0.2,
		"top_p": 0.9,
		"seed": 123,
		"max_tokens": 256,
		"max_completion_tokens": 512,
		"stop": ["END"],
		"n": 1,
		"presence_penalty": 0.3,
		"frequency_penalty": 0.4,
		"logit_bias": {"42": -2},
		"logprobs": true,
		"top_logprobs": 2,
		"parallel_tool_calls": false,
		"vendor_extension": {"b": 2, "a": 1},
		"messages": [
			{"role": "system", "content": "answer tersely"},
			{"role": "user", "content": "older question"},
			{"role": "assistant", "content": "older answer"},
			{"role": "user", "content": [
				{"type": "text", "text": "latest weather intent"},
				{"type": "image_url", "image_url": {"url": "https://example.invalid/image.png"}}
			]}
		]
	}`)
}

func fullOpenAIChatRequestReorderedBody() []byte {
	return []byte(`{"vendor_extension":{"a":1,"b":2},"parallel_tool_calls":false,"top_logprobs":2,"logprobs":true,"logit_bias":{"42":-2},"frequency_penalty":0.4,"presence_penalty":0.3,"stream":true,"n":1,"stop":["END"],"max_completion_tokens":512,"max_tokens":256,"seed":123,"top_p":0.9,"temperature":0.2,"response_format":{"type":"json_object"},"tool_choice":{"type":"function","function":{"name":"lookup_weather"}},"tools":[{"function":{"parameters":{"type":"object"},"name":"lookup_weather"},"type":"function"}],"messages":[{"content":"answer tersely","role":"system"},{"content":"older question","role":"user"},{"content":"older answer","role":"assistant"},{"content":[{"text":"latest weather intent","type":"text"},{"image_url":{"url":"https://example.invalid/image.png"},"type":"image_url"}],"role":"user"}],"model":"qwen-turbo"}`)
}

func jsonRaw(value string) json.RawMessage {
	return json.RawMessage(value)
}

func openAIRequestDigestInput(request sessionctx.OpenAIChatRequest) sessionctx.RequestDigestInput {
	return sessionctx.RequestDigestInput{
		Model:             request.Model,
		Messages:          request.Messages,
		Tools:             request.Tools,
		ToolChoice:        request.ToolChoice,
		ResponseFormat:    request.ResponseFormat,
		Temperature:       request.Temperature,
		TopP:              request.TopP,
		Seed:              request.Seed,
		MaxTokens:         request.MaxTokens,
		MaxCompletion:     request.MaxCompletion,
		Stop:              request.Stop,
		N:                 request.N,
		PresencePenalty:   request.PresencePenalty,
		FrequencyPenalty:  request.FrequencyPenalty,
		LogitBias:         request.LogitBias,
		Logprobs:          request.Logprobs,
		TopLogprobs:       request.TopLogprobs,
		ParallelToolCalls: request.ParallelToolCalls,
		Extra:             request.Extra,
	}
}

func jsonField(t *testing.T, body []byte, field string) json.RawMessage {
	t.Helper()
	var parsed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed[field]
}

func messageField(t *testing.T, body []byte, index int, field string) json.RawMessage {
	t.Helper()
	var parsed struct {
		Messages []map[string]json.RawMessage `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.Greater(t, len(parsed.Messages), index)
	return parsed.Messages[index][field]
}

func logFieldValue(fields [][2]string, name string) string {
	for _, field := range fields {
		if field[0] == name {
			return field[1]
		}
	}
	return ""
}
