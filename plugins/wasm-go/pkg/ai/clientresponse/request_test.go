package clientresponse

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestResponsesEchoesAcceptedClientFieldsWithoutInternalInstructions(t *testing.T) {
	request := []byte(`{"model":"sales","instructions":"client instruction","metadata":{"trace":"mine"},"temperature":0,"parallel_tool_calls":false}`)
	body := []byte(`{"id":"r","object":"response","model":"upstream","status":"completed","output":[],"instructions":"internal supplier prompt","metadata":{"binding":"private"},"tools":[{"type":"internal"}]}`)
	out, err := Project(Policy{Protocol: Responses, PublicModel: "sales", RequestFields: RequestFields(request)}, body, false, Usage{})
	if err != nil || gjson.GetBytes(out, "instructions").String() != "client instruction" || gjson.GetBytes(out, "metadata.trace").String() != "mine" || gjson.GetBytes(out, "metadata.binding").Exists() || gjson.GetBytes(out, "tools").Exists() || gjson.GetBytes(out, "temperature").Raw != "0" || gjson.GetBytes(out, "parallel_tool_calls").Raw != "false" {
		t.Fatal("request echoes leaked internal state or lost explicit zero", string(out), err)
	}
}
