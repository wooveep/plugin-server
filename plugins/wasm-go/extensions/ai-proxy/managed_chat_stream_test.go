package main

import (
	"bytes"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/stretchr/testify/require"
)

func TestManagedChatStreamObservesOriginalUsageAcrossChunks(t *testing.T) {
	body := []byte("\ufeff: keep-alive\r\n\r\n")
	body = append(body, []byte("data: {\"model\":\"qwen-plus\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"答\",\"reasoning_content\":\"想\"},\"finish_reason\":null}]}\r\n\r\n")...)
	body = append(body, []byte("data: {\"choices\":[{\"index\":0,\"finish_reason\":\"stop\"}]}\r\n\r\n")...)
	body = append(body, []byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":21,\"completion_tokens\":3,\"total_tokens\":24}}\r\n\r\n")...)
	const done = "data: [DONE]\r\n\r\n"
	body = append(body, done...)

	for _, size := range []int{1, 2, 11, len(body)} {
		stream := managedChatStream{collectText: true}
		for offset := 0; offset < len(body); offset += size {
			stream.push(body[offset:min(offset+size, len(body))])
		}
		observed := stream.observation()
		require.Equal(t, "succeeded", observed.ExecutionResult, "chunk size %d", size)
		require.Equal(t, "reported", observed.Usage.Status)
		require.Equal(t, "21", observed.Usage.Quantities["input"])
		require.Equal(t, "3", observed.Usage.Quantities["output"])
		require.Equal(t, "qwen-plus", observed.ResponseModel)
		require.True(t, stream.thinking)
		require.Equal(t, "答", stream.text.String())
	}

	var interrupted managedChatStream
	interrupted.push(bytes.TrimSuffix(body, []byte(done)))
	observed := interrupted.observation()
	require.Equal(t, "unknown", observed.ExecutionResult)
	require.Equal(t, "missing", observed.Usage.Status)
}

func TestRouterChatProjectionEmitsContentBeforeTerminalUsage(t *testing.T) {
	policy := clientresponse.Policy{Protocol: clientresponse.Chat, Profile: clientresponse.Qwen,
		PublicModel: "sales-model", UpstreamModel: "upstream-model", IncludeUsage: true}
	s := &managedExecutionState{router: &managedRouterResponse{
		chat: &managedChatStream{}, projection: clientresponse.NewStreamProjection(policy),
	}}
	first := []byte("data: {\"id\":\"chatcmpl-demo\",\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"}}]}\n\n")
	require.Empty(t, projectManagedRouterChunk(nil, s, first[:len(first)-1], false))
	early := projectManagedRouterChunk(nil, s, first[len(first)-1:], false)
	require.Contains(t, string(early), `"content":"Hello"`)
	require.Contains(t, string(early), `"model":"sales-model"`)
	require.NotContains(t, string(early), `"prompt_tokens"`)
	require.NotContains(t, string(early), "[DONE]")

	tail := []byte("data: {\"id\":\"chatcmpl-demo\",\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"id\":\"chatcmpl-demo\",\"model\":\"upstream-model\",\"choices\":[],\"usage\":{\"prompt_tokens\":21,\"completion_tokens\":3,\"total_tokens\":24}}\n\n" +
		"data: [DONE]\n\n")
	output := projectManagedRouterChunk(nil, s, tail, false)
	require.NotContains(t, string(output), "[DONE]")
	final := projectManagedRouterChunk(nil, s, nil, true)
	require.Contains(t, string(final), `"prompt_tokens":21`)
	require.Equal(t, 1, bytes.Count(final, []byte("[DONE]")))
}
