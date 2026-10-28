package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryTextOutputRequestBypass(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct{ name, protocol, body string }{
			{"chat audio output", "openai_chat", `{"model":"public.model","modalities":["text","audio"],"audio":{"voice":"alloy","format":"wav"},"messages":[{"role":"user","content":"question"}]}`},
			{"chat malformed modalities", "openai_chat", `{"model":"public.model","modalities":"text","messages":[{"role":"user","content":"question"}]}`},
			{"chat malformed modality item", "openai_chat", `{"model":"public.model","modalities":["text",7],"messages":[{"role":"user","content":"question"}]}`},
			{"responses background", "openai_responses", `{"model":"public.model","background":true,"input":"question"}`},
			{"responses malformed background", "openai_responses", `{"model":"public.model","background":"true","input":"question"}`},
			{"responses nontext modalities", "openai_responses", `{"model":"public.model","modalities":["audio"],"input":"question"}`},
		} {
			for _, recent := range []bool{false, true} {
				name := tc.name + "/console"
				if recent {
					name = tc.name + "/recent"
				}
				t.Run(name, func(t *testing.T) {
					host := startNativeMemoryHeaders(t, memoryTextOutputConfiguration(t, tc.protocol, recent), memoryNativeCallerFixture().Headers())
					t.Cleanup(host.Reset)
					body := []byte(tc.body)
					require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(body))
					require.Equal(t, body, host.GetRequestBody(), "memory must not inject into a bypassed request")
					require.Empty(t, host.GetHttpCalloutAttributes(), "no Console lookup")
					require.Empty(t, host.GetRedisCalloutAttributes(), "no recent lookup")
					basis, err := bodybasis.Read(host.GetRequestHeaders())
					require.NoError(t, err)
					expected, err := bodybasis.Capture(body)
					require.NoError(t, err)
					require.Equal(t, expected, basis, "the separately assigned first-body responsibility remains active")
					host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
					host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"not a memory answer"},"finish_reason":"stop"}]}`))
					require.Empty(t, host.GetRedisCalloutAttributes(), "bypassed requests emit no memory capture")
					require.Nil(t, host.GetLocalResponse(), "memory bypass leaves the model request available")
				})
			}
		}
	})
}

func TestMemoryTextOutputRetainsMultimodalInput(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct{ protocol, body, response string }{
			{"openai_chat", `{"model":"public.model","modalities":["text"],"messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA"}},{"type":"input_audio","input_audio":{"data":"BBB","format":"wav"}}]}]}`, `{"choices":[{"index":0,"message":{"content":"text answer","audio":null},"finish_reason":"stop"}]}`},
			{"anthropic_messages", `{"model":"public.model","messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAA"}}]}]}`, `{"content":[{"type":"text","text":"text answer"}],"stop_reason":"end_turn"}`},
			{"openai_responses", `{"model":"public.model","background":false,"input":[{"role":"user","content":[{"type":"input_text","text":"describe"},{"type":"input_image","image_url":"data:image/png;base64,AAA"},{"type":"input_file","filename":"sample.pdf","file_data":"BBB"}]}]}`, `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"text answer"}]}]}`},
		} {
			t.Run(tc.protocol, func(t *testing.T) {
				host := startNativeMemoryHeaders(t, memoryTextOutputConfiguration(t, tc.protocol, false), memoryNativeCallerFixture().Headers())
				t.Cleanup(host.Reset)
				body := []byte(tc.body)
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
				requireMemoryAssembleCall(t, host)
				host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{"role": "system", "content": "retained memory"}, nil))
				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				require.Contains(t, string(host.GetRequestBody()), "retained memory")
				for _, marker := range []string{"AAA", "BBB"} {
					if strings.Contains(tc.body, marker) {
						require.Contains(t, string(host.GetRequestBody()), marker, "input attachment must survive injection")
					}
				}
				basis, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				expected, err := bodybasis.Capture(body)
				require.NoError(t, err)
				require.Equal(t, expected, basis)
				host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
				require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte(tc.response)))
				event := requireMemoryResponseCaptureEvent(t, host)
				require.Equal(t, "text answer", event["assistant_content"])
			})
		}
	})
}

func TestMemoryTextOutputResponseBypass(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct{ name, protocol, response string }{
			{"chat audio", "openai_chat", `{"choices":[{"index":0,"message":{"content":"caption","audio":{"data":"AA=="}},"finish_reason":"stop"}]}`},
			{"chat mixed content", "openai_chat", `{"choices":[{"index":0,"message":{"content":[{"type":"text","text":"caption"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA"}}]},"finish_reason":"stop"}]}`},
			{"messages mixed content", "anthropic_messages", `{"content":[{"type":"text","text":"caption"},{"type":"image","source":{"data":"AAA"}}],"stop_reason":"end_turn"}`},
			{"responses image output", "openai_responses", `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"caption"}]},{"type":"image_generation_call","result":"AAA"}]}`},
			{"responses mixed content", "openai_responses", `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"caption"},{"type":"output_audio","data":"AAA"}]}]}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				host := memoryTextOutputResponseHost(t, tc.protocol, false)
				host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
				response := []byte(tc.response)
				require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody(response))
				require.Equal(t, response, host.GetResponseBody(), "nontext output remains available to the client")
				require.Empty(t, host.GetRedisCalloutAttributes(), "mixed output must not create a text-memory event")
			})
		}
	})
}

func TestMemoryTextOutputStreaming(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		chatText := memoryStreamingSSEFrame("\n", `{"choices":[{"index":0,"delta":{"content":"caption"},"finish_reason":"stop"}]}`)
		messagesText := memoryStreamingSSEEventFrame("\n", "content_block_delta", `{"type":"content_block_delta","delta":{"type":"text_delta","text":"caption"}}`)
		messagesEnd := memoryStreamingSSEEventFrame("\n", "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`)
		responsesText := memoryStreamingSSEEventFrame("\n", "response.output_text.delta", `{"type":"response.output_text.delta","delta":"caption"}`)
		responsesEnd := memoryStreamingSSEEventFrame("\n", "response.completed", `{"type":"response.completed","response":{"status":"completed"}}`)
		for _, tc := range []struct {
			name, protocol string
			chunks         []string
			capture        bool
		}{
			{"chat text", "openai_chat", []string{chatText, memoryStreamingSSEFrame("\n", "[DONE]")}, true},
			{"chat split audio delta", "openai_chat", []string{chatText, `data: {"choices":[{"index":0,"delta":{"au`, `dio":{"data":"AAA"}},"finish_reason":"stop"}]}` + "\n\n", memoryStreamingSSEFrame("\n", "[DONE]")}, false},
			{"messages text", "anthropic_messages", []string{messagesText, messagesEnd}, true},
			{"messages image block", "anthropic_messages", []string{messagesText, memoryStreamingSSEEventFrame("\n", "content_block_start", `{"type":"content_block_start","content_block":{"type":"image","source":{"data":"AAA"}}}`), messagesEnd}, false},
			{"responses text", "openai_responses", []string{responsesText, responsesEnd}, true},
			{"responses audio delta", "openai_responses", []string{responsesText, memoryStreamingSSEEventFrame("\n", "response.output_audio.delta", `{"type":"response.output_audio.delta","delta":"AAA"}`), responsesEnd}, false},
			{"responses image item", "openai_responses", []string{responsesText, memoryStreamingSSEEventFrame("\n", "response.output_item.added", `{"type":"response.output_item.added","item":{"type":"image_generation_call"}}`), responsesEnd}, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				host := memoryTextOutputResponseHost(t, tc.protocol, true)
				host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "text/event-stream"}})
				callMemoryStreamingResponseChunks(t, host, tc.chunks...)
				if tc.capture {
					event := requireMemoryResponseCaptureEvent(t, host)
					require.Equal(t, "caption", event["assistant_content"])
				} else {
					require.Empty(t, host.GetRedisCalloutAttributes(), "media frames must suppress text-memory capture")
				}
			})
		}
	})
}

func memoryTextOutputConfiguration(t *testing.T, protocol string, recent bool) json.RawMessage {
	t.Helper()
	mode := "semantic"
	if recent {
		mode = "recent-only"
	}
	var fields map[string]interface{}
	require.NoError(t, json.Unmarshal(memoryNativeConfiguration(t, mode, recent), &fields))
	fields["_rules_"].([]interface{})[0].(map[string]interface{})["client_protocol"] = protocol
	return mustMemoryConfig(t, fields)
}

func memoryTextOutputResponseHost(t *testing.T, protocol string, stream bool) test.TestHost {
	t.Helper()
	host := startNativeMemoryHeaders(t, memoryTextOutputConfiguration(t, protocol, false), memoryNativeCallerFixture().Headers())
	t.Cleanup(host.Reset)
	container := `"messages":[{"role":"user","content":"question"}]`
	if protocol == "openai_responses" {
		container = `"input":"question"`
	}
	streamField := "false"
	if stream {
		streamField = "true"
	}
	body := []byte(`{"model":"public.model",` + container + `,"stream":` + streamField + `}`)
	require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
	requireMemoryAssembleCall(t, host)
	host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "bypass", nil, nil))
	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
	return host
}
