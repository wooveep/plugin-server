package clientresponse

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestPlatformResourceIdentityIsIndependentAndStreamStable(t *testing.T) {
	for _, protocol := range []string{Chat, Responses, Messages} {
		for _, raw := range []string{"qwen-private-123", "deepseek-private-456", "123456789", "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"} {
			body := []byte(`{"id":"` + raw + `","model":"actual","object":"response","status":"completed","output":[],"choices":[],"content":[]}`)
			a, err := Project(Policy{Protocol: protocol, PublicModel: "sales", RequestID: "trace-1"}, body, false, Usage{})
			if err != nil {
				t.Fatal(err)
			}
			b, err := Project(Policy{Protocol: protocol, PublicModel: "sales", RequestID: "trace-1"}, body, false, Usage{})
			id := gjson.GetBytes(a, "id").String()
			if err != nil || id == "" || id == raw || id == "trace-1" || id == gjson.GetBytes(b, "id").String() || strings.Contains(string(a), raw) {
				t.Fatalf("public identity retained cross-system linkage: %s", a)
			}
		}
	}
	raw := []byte("data: {\"id\":\"deepseek-private-456\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"deepseek-private-456\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	body, err := Project(Policy{Protocol: Chat, PublicModel: "sales", RequestID: "trace-1"}, raw, true, Usage{})
	if err != nil {
		t.Fatal(err)
	}
	events, _ := Events(body)
	if gjson.GetBytes(events[0].Data, "id").String() != gjson.GetBytes(events[1].Data, "id").String() || strings.Contains(string(body), "deepseek-private") {
		t.Fatalf("public stream identity was not stable: %s", body)
	}
}

func TestPlatformItemAndFunctionAssociationsDoNotRewriteContent(t *testing.T) {
	raw := []byte("event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"supplier-response\",\"object\":\"response\",\"status\":\"in_progress\",\"output\":[]}}\n\nevent: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"supplier-item\",\"type\":\"function_call\",\"call_id\":\"supplier-call\",\"name\":\"lookup\",\"arguments\":\"\"}}\n\nevent: response.function_call_arguments.delta\ndata: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"supplier-item\",\"output_index\":0,\"delta\":\"{}\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"supplier-response\",\"object\":\"response\",\"status\":\"completed\",\"output\":[{\"id\":\"supplier-item\",\"type\":\"function_call\",\"call_id\":\"supplier-call\",\"name\":\"lookup\",\"arguments\":\"{}\"},{\"id\":\"supplier-text\",\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"supplier-item is quoted content\"}]}]}}\n\n")
	body, err := Project(Policy{Protocol: Responses, PublicModel: "sales"}, raw, true, Usage{})
	if err != nil {
		t.Fatal(err)
	}
	events, _ := Events(body)
	first := gjson.GetBytes(events[1].Data, "item.id").String()
	if first == "supplier-item" || first != gjson.GetBytes(events[2].Data, "item_id").String() || first != gjson.GetBytes(events[3].Data, "response.output.0.id").String() || gjson.GetBytes(events[1].Data, "item.call_id").String() == "supplier-call" || gjson.GetBytes(events[1].Data, "item.call_id").String() != gjson.GetBytes(events[3].Data, "response.output.0.call_id").String() || gjson.GetBytes(events[3].Data, "response.output.1.content.0.text").String() != "supplier-item is quoted content" {
		t.Fatalf("associations or user output changed: %s", body)
	}
}

func TestChatToolDeltaWithoutIDDoesNotCreateAnotherCall(t *testing.T) {
	raw := []byte("data: {\"id\":\"raw\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"a\",\"type\":\"function\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{\"}},{\"index\":1,\"id\":\"b\",\"type\":\"function\",\"function\":{\"name\":\"other\",\"arguments\":\"{\"}}]}}]}\n\ndata: {\"id\":\"raw\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"}\"}},{\"index\":1,\"function\":{\"arguments\":\"}\"}}]}}]}\n\ndata: [DONE]\n\n")
	body, err := Project(Policy{Protocol: Chat, PublicModel: "sales"}, raw, true, Usage{})
	if err != nil {
		t.Fatal(err)
	}
	events, _ := Events(body)
	if gjson.GetBytes(events[1].Data, "choices.0.delta.tool_calls.0.id").Exists() || gjson.GetBytes(events[1].Data, "choices.0.delta.tool_calls.1.id").Exists() || gjson.GetBytes(events[0].Data, "choices.0.delta.tool_calls.0.id").String() == gjson.GetBytes(events[0].Data, "choices.0.delta.tool_calls.1.id").String() {
		t.Fatal("tool argument deltas changed parallel call identity")
	}
}
