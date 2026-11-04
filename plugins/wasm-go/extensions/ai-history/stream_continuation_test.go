package main

import (
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHistoryStreamKeepsNativeContinuationAfterEligibility(t *testing.T) {
	for _, tc := range []struct {
		kind                  protocol.ProtocolKind
		body, field, expected string
	}{
		{protocol.ProtocolChatCompletions, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"real reasoning\",\"content\":\"answer\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", "reasoning_content", "real reasoning"},
		{protocol.ProtocolMessages, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"real\",\"signature\":\"original-signature\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"answer\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", "content.0.signature", "original-signature"},
	} {
		output, reason := historyResponseOutput(tc.kind, 200, true, [][]byte{[]byte(tc.body)})
		require.Empty(t, reason)
		require.Len(t, output, 1)
		require.Equal(t, tc.expected, gjson.GetBytes(output[0], tc.field).String())
	}
}
