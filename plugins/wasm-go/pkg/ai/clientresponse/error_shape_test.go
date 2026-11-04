package clientresponse

import (
	"github.com/tidwall/gjson"
	"testing"
)

func TestResponsesResourceErrorUsesResourceFields(t *testing.T) {
	raw := []byte(`{"id":"raw","object":"response","status":"failed","output":[],"error":{"code":"server_error","message":"private diagnostic"}}`)
	body, err := Project(Policy{Protocol: Responses, PublicModel: "sales"}, raw, false, Usage{})
	if err != nil || gjson.GetBytes(body, "error.type").Exists() || gjson.GetBytes(body, "error.code").String() != "server_error" || gjson.GetBytes(body, "error.message").String() == "private diagnostic" {
		t.Fatal("resource error became a foreign error envelope", string(body), err)
	}
}
