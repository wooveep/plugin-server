package clientresponse

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestResponseProfilesKeepDefaultAndExposeOnlyRegisteredMeasurements(t *testing.T) {
	chat := []byte(`{"id":"r","choices":[{"index":0,"message":{"role":"assistant","content":"answer","reasoning_content":"thought"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":5,"total_tokens":105,"prompt_tokens_details":{"cached_tokens":20,"cache_creation_input_tokens":30,"cache_creation":{"ephemeral_5m_input_tokens":30},"cache_type":"ephemeral","account":"private"}}}`)
	for _, profile := range []Profile{"", DeepSeek, Qwen, OpenAI} {
		out, err := Project(Policy{Protocol: Chat, Profile: profile, PublicModel: "sale"}, chat, false, ReadUsage(chat, Chat, false))
		if err != nil || gjson.GetBytes(out, "choices.0.message.reasoning_content").String() != "thought" {
			t.Fatal("content changed", err, string(out))
		}
		if gjson.GetBytes(out, "usage.prompt_tokens_details.cache_creation_input_tokens").Exists() != (profile == Qwen) || strings.Contains(string(out), "private") {
			t.Fatal("profile-specific fields crossed policies", profile, string(out))
		}
	}
	native := ReadUsage([]byte(`{"usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105,"output_tokens_details":{"text_tokens":5,"reasoning_tokens":2}}}`), "qwen", false)
	if gjson.GetBytes(native.wireProfile(Chat, Qwen), "completion_tokens_details.text_tokens").Int() != 5 || gjson.GetBytes(native.wireProfile(Chat, DeepSeek), "completion_tokens_details.text_tokens").Exists() {
		t.Fatal("native Qwen text measurement did not respect the presentation")
	}
}

func TestQwenMeasurementsRejectConflictsWithoutLosingKnownUsage(t *testing.T) {
	for _, fields := range []string{
		`"cache_creation_input_tokens":81,"cache_creation":{"ephemeral_5m_input_tokens":81}`,
		`"cache_creation_input_tokens":-1,"cache_creation":{"ephemeral_5m_input_tokens":0}`,
		`"cache_creation_input_tokens":"30","cache_creation":{"ephemeral_5m_input_tokens":30}`,
	} {
		u := ReadUsage([]byte(`{"usage":{"prompt_tokens":100,"completion_tokens":5,"total_tokens":105,"prompt_tokens_details":{"cached_tokens":20,`+fields+`}}}`), Chat, false)
		wire := u.wireProfile(Chat, Qwen)
		if gjson.GetBytes(wire, "prompt_tokens").Int() != 100 || gjson.GetBytes(wire, "prompt_tokens_details.cached_tokens").Int() != 20 || gjson.GetBytes(wire, "prompt_tokens_details.cache_creation").Exists() || gjson.GetBytes(wire, "prompt_tokens_details.cache_creation_input_tokens").Exists() {
			t.Fatal("invalid creation measurement changed valid usage", string(wire))
		}
	}
	zero := ReadUsage([]byte(`{"usage":{"prompt_tokens":100,"completion_tokens":5,"total_tokens":105,"prompt_tokens_details":{"cache_creation_input_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":0}}}}`), Chat, false).wireProfile(Chat, Qwen)
	if gjson.GetBytes(zero, "prompt_tokens_details.cache_creation.ephemeral_5m_input_tokens").Raw != "0" {
		t.Fatal("reported zero became unknown", string(zero))
	}
}

func TestQwenResponsesDetailsAreAnExactDisplayAllowlist(t *testing.T) {
	input := []byte(`{"usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105,"x_details":[{"input_tokens":100,"output_tokens":5,"total_tokens":105,"x_billing_type":"secret","input_tokens_details":{"text_tokens":100,"cached_tokens":20,"cache_creation_input_tokens":30,"cache_creation":{"ephemeral_5m_input_tokens":30},"cache_type":"ephemeral","account":"secret"},"output_tokens_details":{"text_tokens":5,"reasoning_tokens":2},"plugins":["secret"]},{"input_tokens":1,"output_tokens":2,"total_tokens":4}]}}`)
	u := ReadUsage(input, Responses, false)
	wire := u.wireProfile(Responses, Qwen)
	if strings.Contains(string(wire), "secret") || gjson.GetBytes(wire, "x_details.#").Int() != 1 || gjson.GetBytes(wire, "x_details.0.input_tokens_details.cache_creation.ephemeral_5m_input_tokens").Int() != 30 || gjson.GetBytes(u.wireProfile(Responses, DeepSeek), "x_details").Exists() {
		t.Fatal("Qwen details leaked diagnostics or accepted conflicting rows", string(wire))
	}
}

func TestQwenMessagesStartAndFinalUsageUseDifferentFieldSets(t *testing.T) {
	input := []byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"m\",\"role\":\"assistant\",\"content\":[],\"usage\":{\"input_tokens\":15,\"output_tokens\":0,\"cache_read_input_tokens\":3,\"cache_creation_input_tokens\":2}}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":8}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	out, err := Project(Policy{Protocol: Messages, Profile: Qwen, PublicModel: "sale"}, input, true, ReadUsage(input, Messages, true))
	if err != nil {
		t.Fatal(err)
	}
	events, _ := Events(out)
	if gjson.GetBytes(events[0].Data, "message.usage.cache_read_input_tokens").Exists() || gjson.GetBytes(events[0].Data, "message.usage.input_tokens").Int() != 15 || gjson.GetBytes(events[1].Data, "usage.cache_creation_input_tokens").Int() != 2 || gjson.GetBytes(events[1].Data, "usage.output_tokens").Int() != 8 {
		t.Fatal("Messages usage phases lost their contract", string(out))
	}
}

func TestResponseProfileValidationAndOpenAIOverload(t *testing.T) {
	for _, protocol := range []string{Chat, Responses, Messages} {
		for _, profile := range []string{"deepseek", "qwen", "openai"} {
			_, err := ParseProfile(profile, protocol)
			if (err != nil) != (protocol == Messages && profile == "openai") {
				t.Fatal(protocol, profile, err)
			}
		}
	}
	if _, err := ParseProfile("unknown", Chat); err == nil {
		t.Fatal("unknown profile accepted")
	}
	if _, err := ParseProfile("deepseek", "unknown"); err == nil {
		t.Fatal("unknown client protocol accepted")
	}
	failure := Failure{Kind: FailureOverloaded, Retryable: true}
	if gjson.GetBytes(failure.Body(Policy{Protocol: Chat, Profile: OpenAI}), "error.type").String() != "service_unavailable_error" || gjson.GetBytes(failure.Body(Policy{Protocol: Chat}), "error.type").String() != "overloaded_error" || failure.Status(Chat) != 503 {
		t.Fatal("presentation changed overload recovery or failed to select its HTTP type")
	}
}

func TestOpenAIHTTPOverloadTypeDoesNotAlterStreamingErrorEvents(t *testing.T) {
	body := []byte("event: error\ndata: {\"type\":\"error\",\"code\":\"server_is_overloaded\",\"message\":\"busy\"}\n\n")
	defaultBody, err := Project(Policy{Protocol: Responses, Profile: DeepSeek, PublicModel: "sale"}, body, true, Usage{})
	if err != nil {
		t.Fatal(err)
	}
	openaiBody, err := Project(Policy{Protocol: Responses, Profile: OpenAI, PublicModel: "sale"}, body, true, Usage{})
	if err != nil || !bytes.Equal(defaultBody, openaiBody) {
		t.Fatal("HTTP-only error type changed the SSE event", err, string(openaiBody))
	}
}
