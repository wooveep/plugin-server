package main

import (
	"bytes"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRouterConvertedStreamUsesOnlyVerifiedChatTerminal(t *testing.T) {
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption())
	defer reset()
	host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	early := []byte("data:{\"id\":\"chat-q\",\"model\":\"upstream\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"early\"}}]}\r\n\r\n")
	usage := []byte("data:{\"choices\":[{\"index\":0,\"finish_reason\":\"stop\"}]}\n\ndata:{\"choices\":[],\"usage\":{\"prompt_tokens\":29,\"completion_tokens\":7,\"total_tokens\":36}}\n\n")
	for _, protocol := range []string{clientresponse.Responses, clientresponse.Messages} {
		for _, complete := range []bool{true, false} {
			t.Run(protocol+"/"+map[bool]string{true: "complete", false: "truncated"}[complete], func(t *testing.T) {
				s := &managedExecutionState{callID: "call-1", config: &managedExecutionConfig{
					profile: managedProtocolProfile{InputProtocol: protocol}, settings: managedSettings{ResponseProfile: clientresponse.Qwen},
				}}
				policy := managedPublicPolicy(s)
				policy.PublicModel = "sales-model"
				r := &managedRouterResponse{chat: &managedChatStream{}, projection: clientresponse.NewStreamProjection(policy)}
				s.router = r
				textEvent, terminal, failure, usagePath := "response.output_text.delta", "response.completed", "response.failed", "response.usage"
				if protocol == clientresponse.Responses {
					r.responsesConverter = provider.NewChatCompletionToResponsesStreamConverter()
				} else {
					r.messagesConverter = &provider.ClaudeToOpenAIConverter{}
					textEvent, terminal, failure, usagePath = "content_block_delta", "message_stop", "error", "usage"
				}
				require.Empty(t, projectManagedRouterChunk(nil, s, early[:len(early)-2], false))
				head := projectManagedRouterChunk(nil, s, early[len(early)-2:], false)
				require.Contains(t, string(head), "event: "+textEvent)
				require.Contains(t, string(head), "early")
				require.Contains(t, string(head), `"model":"sales-model"`)
				require.NotContains(t, string(head), `"input_tokens":0`)
				require.Empty(t, projectManagedRouterChunk(nil, s, usage, false))
				var tail []byte
				if complete {
					tail = []byte("data:[DONE]\n\n")
				}
				final := projectManagedRouterChunk(nil, s, tail, true)
				observed := r.streamObservation()
				require.NotContains(t, string(final), "[DONE]")
				if !complete {
					require.Contains(t, string(final), "event: "+failure)
					require.NotContains(t, string(final), "event: "+terminal)
					require.Equal(t, "missing", observed.Usage.Status)
					return
				}
				require.Equal(t, 1, bytes.Count(final, []byte("event: "+terminal)))
				require.Equal(t, "reported", observed.Usage.Status)
				require.Equal(t, "7", observed.Usage.Quantities["output"])
				events, err := clientresponse.Events(final)
				require.NoError(t, err)
				var found bool
				for _, event := range events {
					if counts := gjson.GetBytes(event.Data, usagePath); counts.IsObject() {
						require.Equal(t, int64(29), counts.Get("input_tokens").Int())
						require.Equal(t, int64(7), counts.Get("output_tokens").Int())
						found = true
					}
				}
				require.True(t, found)
			})
		}
	}
}

func TestRouterNativeStreamMatchesBufferedTerminalAcrossFragments(t *testing.T) {
	for _, tc := range []struct {
		protocol, prefix, tail, contentEvent, stopEvent string
	}{
		{clientresponse.Responses,
			"event: response.created\ndata:{\"type\":\"response.created\",\"sequence_number\":0,\"response\":{\"id\":\"resp-upstream\",\"model\":\"upstream\",\"status\":\"in_progress\"}}\n\nevent: response.output_text.delta\ndata:{\"type\":\"response.output_text.delta\",\"sequence_number\":1,\"delta\":\"首片段\"}\n\n",
			"event: response.completed\ndata:{\"type\":\"response.completed\",\"sequence_number\":2,\"response\":{\"id\":\"resp-upstream\",\"model\":\"upstream\",\"status\":\"completed\",\"usage\":{\"input_tokens\":29,\"output_tokens\":7,\"total_tokens\":36}}}\n\n",
			"response.output_text.delta", "response.completed"},
		{clientresponse.Messages,
			"event: message_start\ndata:{\"type\":\"message_start\",\"message\":{\"id\":\"msg-upstream\",\"model\":\"upstream\",\"usage\":{\"input_tokens\":4,\"output_tokens\":0}}}\n\nevent: content_block_delta\ndata:{\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"首片段\"}}\n\n",
			"event: message_delta\ndata:{\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":29,\"output_tokens\":7}}\n\nevent: message_stop\ndata:{\"type\":\"message_stop\"}\n\n",
			"content_block_delta", "message_stop"},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			r := &managedRouterResponse{native: newManagedNativeStream(tc.protocol, true, true),
				projection: clientresponse.NewStreamProjection(clientresponse.Policy{Protocol: tc.protocol, Profile: clientresponse.Qwen, PublicModel: "sales"})}
			s := &managedExecutionState{router: r}
			var prefix bytes.Buffer
			for i := range len(tc.prefix) {
				prefix.Write(projectManagedRouterChunk(nil, s, []byte(tc.prefix[i:i+1]), false))
			}
			require.Contains(t, prefix.String(), "event: "+tc.contentEvent)
			require.Contains(t, prefix.String(), "首片段")
			require.NotContains(t, prefix.String(), "event: "+tc.stopEvent)
			require.Equal(t, "unknown", r.streamObservation().ExecutionResult)
			ending := projectManagedRouterChunk(nil, s, []byte(tc.tail), false)
			require.NotContains(t, string(ending), "event: "+tc.stopEvent)
			final := projectManagedRouterChunk(nil, s, nil, true)
			require.Contains(t, string(final), "event: "+tc.stopEvent)
			wire := []byte(tc.prefix + tc.tail)
			want := managedResponsesStream(wire)
			if tc.protocol == clientresponse.Messages {
				want = managedMessagesStream(wire, true)
			}
			got := r.streamObservation()
			require.Equal(t, "succeeded", got.ExecutionResult)
			require.Equal(t, "29", got.Usage.Quantities["input"])
			require.Equal(t, "7", got.Usage.Quantities["output"])
			require.Equal(t, want, got)
		})
	}
}
