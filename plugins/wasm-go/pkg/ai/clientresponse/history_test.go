package clientresponse

import (
	"bytes"
	"testing"

	"github.com/tidwall/gjson"
)

func TestContinuationRetainsThinkingSignaturesAndExactContent(t *testing.T) {
	for _, tc := range []struct {
		protocol, body, path, expected string
	}{
		{Chat, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"think\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"reasoning_content\":\" again\",\"content\":\"answer\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", "reasoning_content", "think again"},
		{Messages, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\",\"signature\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"real thinking\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"signature_delta\",\"signature\":\"opaque-\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"signature_delta\",\"signature\":\"signature\"}}\n\n", "content.0.signature", "opaque-signature"},
		{Responses, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"reasoning\",\"id\":\"rs_1\",\"encrypted_content\":\"original-context\",\"summary\":[]}]}}\n\n", "encrypted_content", "original-context"},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			output, err := History(tc.protocol, []byte(tc.body))
			if err != nil || len(output) != 1 || gjson.GetBytes(output[0], tc.path).String() != tc.expected {
				t.Fatal("continuation was flattened or altered", output, err)
			}
		})
	}
}

func TestTruncatedEventsNeverAcquireATerminalDelimiter(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n")
	out, err := Project(Policy{Protocol: Chat, PublicModel: "sales"}, body, true, Usage{})
	if err == nil || bytes.Contains(out, []byte("[DONE]")) {
		t.Fatal("truncated upstream acquired a successful public terminal")
	}
}

func TestInvalidOptionalUsageDoesNotInvalidateKnownBase(t *testing.T) {
	for _, body := range []string{
		`{"usage":{"prompt_tokens":3,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":4,"cache_write_tokens":5},"completion_tokens_details":{"reasoning_tokens":3}}}`,
		`{"usage":{"prompt_tokens":3,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":2,"cache_write_tokens":2},"prompt_cache_hit_tokens":2,"prompt_cache_miss_tokens":2}}`,
	} {
		wire := ReadUsage([]byte(body), Chat, false).wire(Responses)
		if gjson.GetBytes(wire, "total_tokens").Int() != 5 || gjson.GetBytes(wire, "input_tokens_details.cached_tokens").Exists() || gjson.GetBytes(wire, "output_tokens_details.reasoning_tokens").Exists() {
			t.Fatal("invalid detail was exposed or base usage lost", string(wire))
		}
	}
}
