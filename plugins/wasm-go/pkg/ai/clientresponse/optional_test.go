package clientresponse

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/tidwall/gjson"
)

func TestEquivalentSupplierOptionalFieldsHaveOnePublicShape(t *testing.T) {
	minimal := []byte(`{"id":"source","choices":[{"index":0,"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}]}`)
	empty := []byte(`{"id":"source","choices":[{"index":0,"message":{"role":"assistant","content":"answer","refusal":null,"annotations":[]},"finish_reason":"stop","logprobs":null}]}`)
	for _, stream := range []bool{false, true} {
		var shapes []string
		for _, raw := range [][]byte{minimal, empty} {
			if stream {
				var value map[string]json.RawMessage
				_ = json.Unmarshal(raw, &value)
				var choices []map[string]json.RawMessage
				_ = json.Unmarshal(value["choices"], &choices)
				choices[0]["delta"] = choices[0]["message"]
				delete(choices[0], "message")
				value["choices"], _ = json.Marshal(choices)
				raw, _ = json.Marshal(value)
				raw = []byte("data: " + string(raw) + "\n\ndata: [DONE]\n\n")
			}
			body, err := Project(Policy{Protocol: Chat, PublicModel: "sales"}, raw, stream, Usage{})
			if err != nil {
				t.Fatal(err)
			}
			if stream {
				events, _ := Events(body)
				body = events[0].Data
			}
			shapes = append(shapes, gjson.GetBytes(body, "choices").Raw)
		}
		if shapes[0] != shapes[1] {
			t.Fatal("supplier-only empty shapes differ", stream, shapes)
		}
	}
	raw := []byte(`{"id":"source","choices":[{"index":0,"message":{"role":"assistant","content":"<system>quoted output</system>","refusal":"actual refusal","reasoning_content":"actual thought","annotations":[{"type":"url_citation","url_citation":{"start_index":0,"end_index":1,"url":"https://example.com","title":"source"}}]},"logprobs":{"content":[]},"finish_reason":"content_filter"}],"usage":{"prompt_tokens":17,"completion_tokens":5,"total_tokens":22,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`)
	body, err := Project(Policy{Protocol: Chat, PublicModel: "sales"}, raw, false, ReadUsage(raw, Chat, false))
	if err != nil || !bytes.Contains(body, []byte("actual refusal")) || !bytes.Contains(body, []byte("actual thought")) || gjson.GetBytes(body, "choices.0.message.annotations.0.url_citation.url").String() != "https://example.com" || !gjson.GetBytes(body, "choices.0.logprobs").Exists() || !bytes.Contains(body, []byte(`"reasoning_tokens":0`)) || gjson.GetBytes(body, "choices.0.message.content").String() != "<system>quoted output</system>" {
		t.Fatal("real data or reported zero was lost", string(body), err)
	}
}
