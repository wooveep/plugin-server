package clientresponse

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestMessagesKeepsKnownNativeInputWithoutInventingFullInput(t *testing.T) {
	u := ReadUsage([]byte(`{"usage":{"input_tokens":10,"output_tokens":4,"cache_read_input_tokens":2}}`), Messages, false)
	if u.wire(Responses) != nil || u.wire(Chat) != nil {
		t.Fatal("unknown cache creation was treated as zero in full input")
	}
	public := u.wire(Messages)
	if gjson.GetBytes(public, "input_tokens").Int() != 10 || gjson.GetBytes(public, "cache_read_input_tokens").Int() != 2 || gjson.GetBytes(public, "cache_creation_input_tokens").Exists() {
		t.Fatal("known native Messages usage was lost", string(public))
	}
}
