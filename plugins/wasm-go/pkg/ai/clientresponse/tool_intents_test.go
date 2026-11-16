package clientresponse

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestOriginalToolIntentsSurviveChatJSONAndTerminalUsageWithoutBecomingCharges(t *testing.T) {
	usage := `{"prompt_tokens":17,"completion_tokens":5,"total_tokens":22,"x_tools":{"search":{"count":1,"strategy":"turbo","private_ref":"unregistered-secret"},"web_extractor":{"count":0},"private_tool":{"count":100}}}`
	jsonBody := []byte(`{"id":"r","choices":[{"index":0,"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}],"usage":` + usage + `}`)
	stream := []byte("data: {\"id\":\"r\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"r\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":" + usage + "}\n\ndata: [DONE]\n\n")
	for _, profile := range []Profile{DeepSeek, Qwen, OpenAI} {
		for _, body := range [][]byte{jsonBody, stream} {
			streaming := bytes.HasPrefix(body, []byte("data:"))
			out, err := Project(Policy{Protocol: Chat, Profile: profile, PublicModel: "sale", IncludeUsage: true}, body, streaming, ReadUsage(body, Chat, streaming))
			if err != nil || strings.Contains(string(out), "unregistered-secret") || strings.Contains(string(out), "private_tool") {
				t.Fatal("tool intent allowlist failed", profile, err, string(out))
			}
			frames := []Event{{Data: out}}
			if streaming {
				frames, _ = Events(out)
			}
			count := 0
			for _, frame := range frames {
				if !gjson.GetBytes(frame.Data, "usage.prompt_tokens").Exists() {
					continue
				}
				count++
				search := gjson.GetBytes(frame.Data, "usage.x_tools.search")
				if search.Exists() != (profile != OpenAI) || profile != OpenAI && (search.Get("count").Int() != 1 || search.Get("strategy").String() != "turbo" || gjson.GetBytes(frame.Data, "usage.x_tools.web_extractor.count").Raw != "0") {
					t.Fatal("original counters or strict profile changed", profile, string(frame.Data))
				}
			}
			if count != 1 {
				t.Fatal("usage was duplicated or dropped", string(out))
			}
		}
	}
}

func TestInvalidOptionalToolIntentDoesNotDiscardKnownTokens(t *testing.T) {
	u := ReadUsage([]byte(`{"usage":{"prompt_tokens":17,"completion_tokens":5,"total_tokens":22,"x_tools":{"search":{"count":-1},"web_search":{"count":"1"}}}}`), Chat, false)
	out := u.wireProfile(Chat, DeepSeek)
	if gjson.GetBytes(out, "prompt_tokens").Int() != 17 || gjson.GetBytes(out, "x_tools").Exists() {
		t.Fatal("unknown tool intent changed known tokens", string(out))
	}
}
