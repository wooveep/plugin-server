package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func managedProtocolFixture(t *testing.T, input, upstream string) managedProtocolProfile {
	t.Helper()
	profile, err := newManagedProtocolProfile(input, upstream)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

var managedProtocolJSONHeaders = [][2]string{{"content-type", "application/json"}}
var managedProtocolSSEHeaders = [][2]string{{"content-type", "text/event-stream; charset=utf-8"}}

const managedProtocolHuge = "340282366920938463463374607431768211454"
const managedProtocolChatReply = `{"id":"reply-1","model":"frozen-upstream","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9007199254740993,"completion_tokens":2,"total_tokens":9007199254740995,"prompt_tokens_details":{"cached_tokens":1}}}`

func managedProtocolChatStream(input, output string) []byte {
	return []byte("data: " + `{"id":"reply-1","model":"frozen-upstream","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"},"finish_reason":null}]}` + "\n\n" +
		"data: " + `{"id":"reply-1","model":"frozen-upstream","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
		"data: " + `{"id":"reply-1","model":"frozen-upstream","choices":[],"usage":{"prompt_tokens":` + input + `,"completion_tokens":` + output + `}}` + "\n\n" +
		"data: [DONE]\n\n")
}

func TestManagedProtocolProfilesAreConfiguredAndBounded(t *testing.T) {
	for _, pair := range [][2]string{{"", ""}, {managedProtocolChat, managedProtocolChat}, {managedProtocolMessages, managedProtocolMessages}, {managedProtocolResponses, managedProtocolResponses}, {managedProtocolMessages, managedProtocolChat}, {managedProtocolResponses, managedProtocolChat}} {
		if _, err := newManagedProtocolProfile(pair[0], pair[1]); err != nil {
			t.Fatalf("supported profile %v rejected: %v", pair, err)
		}
	}
	for _, pair := range [][2]string{{managedProtocolChat, managedProtocolMessages}, {managedProtocolChat, managedProtocolResponses}, {managedProtocolMessages, managedProtocolResponses}, {managedProtocolResponses, managedProtocolMessages}, {"openai", managedProtocolChat}, {" openai_chat", managedProtocolChat}} {
		if _, err := newManagedProtocolProfile(pair[0], pair[1]); err == nil {
			t.Fatalf("unsupported profile %v accepted", pair)
		}
	}
}

func TestManagedProtocolNativeRequestPreservesMappedBytesAndControlledAuthentication(t *testing.T) {
	body := []byte("{ \"model\" : \"frozen-upstream\", \"messages\" : [{\"role\":\"user\",\"content\":\"hello\"}], \"stream\":false }")
	for _, protocol := range []string{managedProtocolChat, managedProtocolMessages, managedProtocolResponses} {
		profile := managedProtocolFixture(t, protocol, protocol)
		request, err := profile.PrepareRequest(body, "frozen-upstream", "server-key")
		if err != nil || !bytes.Equal(request.Body, body) || request.Streaming {
			t.Fatalf("native %s changed mapped request: %v", protocol, err)
		}
		if value, err := managedSingleHeader(request.Headers, "content-type"); err != nil || value != "application/json" {
			t.Fatal("controlled request content type missing")
		}
		if protocol == managedProtocolMessages {
			if value, _ := managedSingleHeader(request.Headers, "x-api-key"); value != "server-key" {
				t.Fatal("native Messages did not select controlled credential")
			}
			if _, err := managedSingleHeader(request.Headers, "authorization"); err == nil {
				t.Fatal("native Messages sent a second authorization mode")
			}
			if version, _ := managedSingleHeader(request.Headers, "anthropic-version"); version != "2023-06-01" {
				t.Fatal("native Messages version was not controlled")
			}
		} else if value, _ := managedSingleHeader(request.Headers, "authorization"); value != "Bearer server-key" {
			t.Fatal("OpenAI native authentication missing")
		}
		request.Body[0] = '['
		if body[0] != '{' {
			t.Fatal("request aliases caller's frozen input bytes")
		}
	}
}

func TestManagedProtocolFallbackRequestsReuseConvertersAndKeepFrozenModel(t *testing.T) {
	for _, test := range []struct {
		protocol string
		body     string
	}{
		{managedProtocolResponses, `{"model":"frozen-upstream","input":"hello","instructions":"be concise","stream":true}`},
		{managedProtocolMessages, `{"model":"frozen-upstream","max_tokens":10,"system":"be concise","messages":[{"role":"user","content":"hello"}],"stream":true}`},
	} {
		profile := managedProtocolFixture(t, test.protocol, managedProtocolChat)
		request, err := profile.PrepareRequest([]byte(test.body), "frozen-upstream", "server-key")
		if err != nil {
			t.Fatal(err)
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(request.Body, &object) != nil || string(object["model"]) != `"frozen-upstream"` || !request.Streaming || !bytes.Contains(request.Body, []byte(`"messages"`)) {
			t.Fatalf("fallback lost mapper model/stream/messages: %s", request.Body)
		}
		if value, _ := managedSingleHeader(request.Headers, "authorization"); value != "Bearer server-key" {
			t.Fatal("fallback used client/native Messages authentication upstream")
		}
		var options struct {
			IncludeUsage bool `json:"include_usage"`
		}
		if json.Unmarshal(object["stream_options"], &options) != nil || !options.IncludeUsage {
			t.Fatal("Chat fallback did not request provider usage")
		}
	}
	profile := managedProtocolFixture(t, managedProtocolResponses, managedProtocolChat)
	if _, err := profile.PrepareRequest([]byte(`{"model":"frozen-upstream","input":"hello","previous_response_id":"old"}`), "frozen-upstream", "server-key"); err == nil {
		t.Fatal("stateful Responses silently used stateless chat fallback")
	}
}

func TestManagedProtocolChatStreamRequestsProviderUsageBeforeFreezingOutboundBody(t *testing.T) {
	profile := managedProtocolFixture(t, managedProtocolChat, managedProtocolChat)
	for _, body := range []string{
		`{"model":"frozen-upstream","stream":true,"messages":[]}`,
		`{"model":"frozen-upstream","stream":true,"stream_options":{"include_usage":false},"messages":[]}`,
		`{"model":"frozen-upstream","stream":true,"stream_options":{"include_usage":true},"messages":[]}`,
	} {
		request, err := profile.PrepareRequest([]byte(body), "frozen-upstream", "server-key")
		if err != nil {
			t.Fatal(err)
		}
		var actual struct {
			StreamOptions struct {
				IncludeUsage bool `json:"include_usage"`
			} `json:"stream_options"`
		}
		if json.Unmarshal(request.Body, &actual) != nil || !actual.StreamOptions.IncludeUsage {
			t.Fatalf("managed Chat stream did not request its original billable usage: %s", request.Body)
		}
	}
}

func TestManagedProtocolRejectsAmbiguousRequestWithoutChangingSelection(t *testing.T) {
	profile := managedProtocolFixture(t, "", "")
	for _, body := range []string{
		`{"model":"other-model","messages":[]}`,
		`{"model":"frozen-upstream","model":"other-model"}`,
		`{"model":"frozen-upstream","stream":null}`,
		`{"model":"frozen-upstream","stream":"true"}`,
		`{"model":"frozen-upstream","messages":"\ud800"}`,
	} {
		if _, err := profile.PrepareRequest([]byte(body), "frozen-upstream", "server-key"); err == nil {
			t.Fatalf("ambiguous request accepted: %s", body)
		}
	}
	if _, err := profile.PrepareRequest([]byte(`{"model":"frozen-upstream"}`), "frozen-upstream", "server-key\r\nx-client: attacker"); err == nil {
		t.Fatal("upstream credential allowed header injection")
	}
}

func TestManagedProtocolNativeUsageIsExactAndHTTPErrorDoesNotEraseIt(t *testing.T) {
	for _, test := range []struct {
		protocol, body, input, output, hit, miss string
	}{
		{managedProtocolChat, managedProtocolChatReply, "9007199254740993", "2", "1", "9007199254740992"},
		{managedProtocolResponses, `{"object":"response","status":"completed","model":"frozen-upstream","usage":{"input_tokens":` + managedProtocolHuge + `,"output_tokens":0,"total_tokens":` + managedProtocolHuge + `,"input_tokens_details":{"cached_tokens":1}}}`, managedProtocolHuge, "0", "1", "340282366920938463463374607431768211453"},
		{managedProtocolMessages, `{"type":"message","stop_reason":"end_turn","model":"frozen-upstream","usage":{"input_tokens":9007199254740993,"output_tokens":0,"cache_read_input_tokens":2,"cache_creation_input_tokens":3}}`, "9007199254740998", "0", "2", "9007199254740996"},
	} {
		profile := managedProtocolFixture(t, test.protocol, test.protocol)
		for _, status := range []uint16{200, 503} {
			observed := profile.ObserveResponse(status, managedProtocolJSONHeaders, []byte(test.body), true)
			if observed.Usage.Status != "reported" || observed.Usage.Quantities["input"] != test.input || observed.Usage.Quantities["output"] != test.output || observed.Usage.Quantities["cache_hit"] != test.hit || observed.Usage.Quantities["cache_miss"] != test.miss {
				t.Fatalf("%s/%d lost exact original usage: %#v", test.protocol, status, observed.Usage)
			}
			if status == 200 && (observed.ExecutionResult != "succeeded" || observed.Acceptance != "accepted") {
				t.Fatal("real terminal response was not accepted")
			}
			if status == 503 && (observed.ExecutionResult != "failed" || observed.Acceptance != "unknown") {
				t.Fatal("HTTP failure invented pre-acceptance rejection or erased provider usage")
			}
		}
	}
}

func TestManagedProtocolMissingInvalidAndExplicitZeroUsageStayDistinct(t *testing.T) {
	profile := managedProtocolFixture(t, managedProtocolResponses, managedProtocolResponses)
	for _, raw := range []string{`null`, `{}`, `{"input_tokens":1}`, `{"input_tokens":1e3,"output_tokens":0}`, `{"input_tokens":"1","output_tokens":0}`, `{"input_tokens":1,"output_tokens":0,"total_tokens":2}`, `{"input_tokens":340282366920938463463374607431768211456,"output_tokens":0}`} {
		observed := profile.ObserveResponse(200, managedProtocolJSONHeaders, []byte(`{"status":"completed","usage":`+raw+`}`), true)
		if observed.Usage.Status != "missing" || observed.Usage.Quantities != nil {
			t.Fatalf("unobserved/invalid usage became free or rounded: %s", raw)
		}
	}
	zero := profile.ObserveResponse(200, managedProtocolJSONHeaders, []byte(`{"status":"completed","usage":{"input_tokens":0,"output_tokens":0}}`), true)
	if zero.Usage.Status != "reported" || zero.Usage.Quantities["input"] != "0" || zero.Usage.Quantities["output"] != "0" {
		t.Fatal("explicit observed zero was treated as missing")
	}
	messages := managedProtocolFixture(t, managedProtocolMessages, managedProtocolMessages)
	for _, raw := range []string{`{"input_tokens":1,"output_tokens":0,"cache_read_input_tokens":1}`, `{"input_tokens":340282366920938463463374607431768211455,"output_tokens":0,"cache_read_input_tokens":1,"cache_creation_input_tokens":0}`} {
		observed := messages.ObserveResponse(200, managedProtocolJSONHeaders, []byte(`{"stop_reason":"end_turn","usage":`+raw+`}`), true)
		if observed.Usage.Status != "missing" || observed.Usage.Quantities != nil {
			t.Fatal("incomplete/overflowing Messages cache usage became a billable number")
		}
	}
}

func TestManagedProtocolChatAggregatedStreamRequiresRealTerminal(t *testing.T) {
	profile := managedProtocolFixture(t, "", "")
	body := managedProtocolChatStream(managedProtocolHuge, "0")
	observed := profile.ObserveResponse(200, managedProtocolSSEHeaders, body, true)
	if observed.Usage.Status != "reported" || observed.Usage.Quantities["input"] != managedProtocolHuge || observed.ExecutionResult != "succeeded" || observed.Acceptance != "accepted" {
		t.Fatalf("complete buffered chat stream lost actual usage/terminal: %#v", observed)
	}
	for _, truncated := range [][]byte{bytes.TrimSuffix(body, []byte("data: [DONE]\n\n")), bytes.TrimSuffix(body, []byte("\n")), body[:len(body)-8]} {
		observed := profile.ObserveResponse(200, managedProtocolSSEHeaders, truncated, true)
		if observed.ExecutionResult != "unknown" || observed.Usage.Status != "missing" || observed.Acceptance != "unknown" {
			t.Fatal("buffer exhaustion or final usage alone fabricated a terminal")
		}
	}
	conflict := bytes.Replace(body, []byte("data: [DONE]"), []byte(`data: {"choices":[],"usage":{"prompt_tokens":2,"completion_tokens":0}}`+"\n\ndata: [DONE]"), 1)
	if observed := profile.ObserveResponse(200, managedProtocolSSEHeaders, conflict, true); observed.Usage.Status != "missing" || observed.Usage.Quantities != nil {
		t.Fatal("conflicting actual stream usage silently selected the last value")
	}
	providerError := bytes.Replace(body, []byte("data: [DONE]"), []byte(`event: error`+"\ndata: "+`{"error":{"type":"provider_error"}}`+"\n\ndata: [DONE]"), 1)
	if observed := profile.ObserveResponse(200, managedProtocolSSEHeaders, providerError, true); observed.ExecutionResult != "failed" || observed.Acceptance != "unknown" || observed.Usage.Status != "reported" || observed.Usage.Quantities["input"] != managedProtocolHuge {
		t.Fatal("explicit complete usage was erased by a later provider error")
	}
}

func TestManagedProtocolResponsesAggregatedStreamUsesOriginalTerminalEnvelope(t *testing.T) {
	profile := managedProtocolFixture(t, managedProtocolResponses, managedProtocolResponses)
	body := []byte("event: response.created\ndata: " + `{"type":"response.created","response":{"model":"frozen-upstream","status":"in_progress"}}` + "\n\n" +
		"event: response.completed\ndata: " + `{"type":"response.completed","response":{"model":"frozen-upstream","status":"completed","usage":{"input_tokens":9007199254740993,"output_tokens":0,"total_tokens":9007199254740993}}}` + "\n\n")
	observed := profile.ObserveResponse(200, managedProtocolSSEHeaders, body, true)
	if observed.ExecutionResult != "succeeded" || observed.Acceptance != "accepted" || observed.Usage.Quantities["input"] != "9007199254740993" {
		t.Fatalf("native Responses terminal not captured exactly: %#v", observed)
	}
	for _, invalid := range [][]byte{
		bytes.Replace(body, []byte(`"status":"completed"`), []byte(`"status":"in_progress"`), 1),
		bytes.Replace(body, []byte("event: response.completed"), []byte("event: response.failed"), 1),
		append(bytes.Clone(body), body...),
		bytes.TrimSuffix(body, []byte("\n")),
	} {
		if observed := profile.ObserveResponse(200, managedProtocolSSEHeaders, invalid, true); observed.ExecutionResult != "unknown" || observed.Usage.Status != "missing" {
			t.Fatal("conflicting/truncated Responses events became successful terminal usage")
		}
	}
}

func TestManagedProtocolMessagesAggregatedStreamRequiresFinalCumulativeUsage(t *testing.T) {
	profile := managedProtocolFixture(t, managedProtocolMessages, managedProtocolMessages)
	start := "event: message_start\ndata: " + `{"type":"message_start","message":{"type":"message","model":"frozen-upstream","usage":{"input_tokens":9007199254740993,"output_tokens":0}}}` + "\n\n"
	finish := "event: message_delta\ndata: " + `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}` + "\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	observed := profile.ObserveResponse(200, managedProtocolSSEHeaders, []byte(start+finish), true)
	if observed.ExecutionResult != "succeeded" || observed.Usage.Status != "reported" || observed.Usage.Quantities["output"] != "2" || observed.Usage.Quantities["input"] != "9007199254740993" {
		t.Fatalf("native Messages final usage not captured: %#v", observed)
	}
	progress := "event: message_delta\ndata: " + `{"type":"message_delta","delta":{},"usage":{"output_tokens":1}}` + "\n\n"
	noFinalUsage := strings.Replace(finish, `,"usage":{"output_tokens":2}`, "", 1)
	observed = profile.ObserveResponse(200, managedProtocolSSEHeaders, []byte(start+progress+noFinalUsage), true)
	if observed.ExecutionResult != "succeeded" || observed.Usage.Status != "missing" {
		t.Fatal("intermediate count was promoted to final cumulative usage")
	}
	invalid := strings.Replace(finish, `"output_tokens":2`, `"output_tokens":2,"input_tokens":1`, 1)
	if observed := profile.ObserveResponse(200, managedProtocolSSEHeaders, []byte(start+invalid), true); observed.ExecutionResult != "unknown" || observed.Usage.Status != "missing" {
		t.Fatal("Messages source count replacement was silently accepted")
	}
}

func TestManagedProtocolFallbackResponseReusesConverterAndPreservesExactUsage(t *testing.T) {
	for _, input := range []string{managedProtocolResponses, managedProtocolMessages} {
		profile := managedProtocolFixture(t, input, managedProtocolChat)
		body := []byte(strings.Replace(managedProtocolChatReply, "9007199254740993", managedProtocolHuge, 1))
		body = bytes.Replace(body, []byte(`"completion_tokens":2`), []byte(`"completion_tokens":0`), 1)
		// This exact count is beyond native machine int even in amd64 builds.
		body = bytes.Replace(body, []byte(`,"total_tokens":9007199254740995`), nil, 1)
		original := bytes.Clone(body)
		observed := profile.ObserveResponse(200, managedProtocolJSONHeaders, body, true)
		converted, err := profile.ConvertResponse(nil, 200, append(managedProtocolJSONHeaders, [2]string{"content-length", "12"}, [2]string{"etag", "original-etag"}), body)
		if err != nil {
			t.Fatalf("%s fallback: %v", input, err)
		}
		if !bytes.Equal(body, original) || observed.Usage.Quantities["input"] != managedProtocolHuge {
			t.Fatal("client conversion altered actual upstream bytes/usage")
		}
		if !bytes.Contains(converted.Body, []byte(`"hello"`)) || !bytes.Contains(converted.Body, []byte(`"model":"frozen-upstream"`)) {
			t.Fatalf("existing converter lost content/model: %s", converted.Body)
		}
		// Consume the converted wire with the corresponding native observer:
		// exact input/output and the cache partition must round-trip together.
		native := managedProtocolFixture(t, input, input)
		roundTrip := native.ObserveResponse(200, converted.Headers, converted.Body, true)
		if !managedProtocolSameUsage(observed.Usage, roundTrip.Usage) || roundTrip.ExecutionResult != "succeeded" {
			t.Fatalf("exact usage was truncated or cache split changed: %s => %#v", converted.Body, roundTrip)
		}
		if _, err := managedSingleHeader(converted.Headers, "content-length"); err == nil {
			t.Fatal("converted body retained obsolete content length")
		}
		if _, err := managedSingleHeader(converted.Headers, "etag"); err == nil {
			t.Fatal("converted body retained original representation validator")
		}
	}
}

func TestManagedProtocolFallbackAggregatedSSEDoesNotInventFinalityOrRoundCounts(t *testing.T) {
	for _, input := range []string{managedProtocolResponses, managedProtocolMessages} {
		profile := managedProtocolFixture(t, input, managedProtocolChat)
		body := managedProtocolChatStream(managedProtocolHuge, "0")
		converted, err := profile.ConvertResponse(nil, 200, managedProtocolSSEHeaders, body)
		if err != nil {
			t.Fatalf("%s buffered fallback: %v", input, err)
		}
		native := managedProtocolFixture(t, input, input)
		observed := native.ObserveResponse(200, converted.Headers, converted.Body, true)
		if observed.ExecutionResult != "succeeded" || observed.Usage.Status != "reported" || observed.Usage.Quantities["input"] != managedProtocolHuge || observed.Usage.Quantities["output"] != "0" {
			t.Fatalf("converted buffered stream has no exact terminal: %s => %#v", converted.Body, observed)
		}
		if _, err := profile.ConvertResponse(nil, 200, managedProtocolSSEHeaders, bytes.TrimSuffix(body, []byte("data: [DONE]\n\n"))); err == nil {
			t.Fatal("provider converter synthesized DONE for an unverified original stream")
		}
	}
}

func TestManagedProtocolMissingUsageIsNotReplacedByConverterDefaults(t *testing.T) {
	body := []byte(`{"id":"reply-1","model":"frozen-upstream","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
	for _, input := range []string{managedProtocolResponses, managedProtocolMessages} {
		profile := managedProtocolFixture(t, input, managedProtocolChat)
		converted, err := profile.ConvertResponse(nil, 200, managedProtocolJSONHeaders, body)
		if err != nil {
			t.Fatal(err)
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(converted.Body, &object) != nil {
			t.Fatal("converted non-stream response is not JSON")
		}
		if _, exists := object["usage"]; exists {
			t.Fatal("provider converter default became fabricated free usage")
		}
	}
}

func TestManagedProtocolConversionErrorsPreserveActualProviderFacts(t *testing.T) {
	profile := managedProtocolFixture(t, managedProtocolResponses, managedProtocolChat)
	bad := []byte(`{"choices":[{"index":0,"message":null,"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":0}}`)
	original := bytes.Clone(bad)
	observation := profile.ObserveResponse(200, managedProtocolJSONHeaders, bad, true)
	if _, err := profile.ConvertResponse(nil, 200, managedProtocolJSONHeaders, bad); err == nil {
		t.Fatal("invalid converter input was accepted")
	}
	if observation.Usage.Status != "reported" || observation.Usage.Quantities["input"] != "1" || !bytes.Equal(original, bad) {
		t.Fatal("local conversion error erased observed provider cost or source bytes")
	}
	errorBody := []byte(`{"error":{"message":"provider unavailable","type":"upstream_error"},"usage":{"prompt_tokens":3,"completion_tokens":0}}`)
	errorObservation := profile.ObserveResponse(503, managedProtocolJSONHeaders, errorBody, true)
	converted, err := profile.ConvertResponse(nil, 503, managedProtocolJSONHeaders, errorBody)
	if err != nil || !bytes.Equal(converted.Body, errorBody) || errorObservation.ExecutionResult != "failed" || errorObservation.Acceptance != "unknown" || errorObservation.Usage.Status != "reported" {
		t.Fatal("actual upstream error was rewritten to a success/pre-acceptance proof")
	}
	// Individual exact counters fit the fact contract, but the Responses total
	// cannot be represented by that profile's u128 count. Keep the upstream
	// observation and fail conversion rather than emit a rounded client total.
	overflow := []byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":340282366920938463463374607431768211455,"completion_tokens":1}}`)
	before := profile.ObserveResponse(200, managedProtocolJSONHeaders, overflow, true)
	if _, err := profile.ConvertResponse(nil, 200, managedProtocolJSONHeaders, overflow); err == nil || before.Usage.Status != "reported" || before.Usage.Quantities["output"] != "1" {
		t.Fatal("unrepresentable client total erased or rounded actual usage")
	}
}

func TestManagedProtocolUnreadableResponseNeverInventsUsage(t *testing.T) {
	profile := managedProtocolFixture(t, "", "")
	for _, headers := range [][][2]string{
		{},
		{{"content-type", "application/json"}, {"Content-Type", "text/event-stream"}},
		{{"content-type", "application/json"}, {"content-encoding", "gzip"}},
		{{"content-type", "text/html"}},
	} {
		observed := profile.ObserveResponse(200, headers, []byte(managedProtocolChatReply), true)
		if observed.ExecutionResult != "unknown" || observed.Usage.Status != "missing" || observed.Acceptance != "unknown" {
			t.Fatal("unreadable body was counted or accepted")
		}
	}
	observed := profile.ObserveResponse(200, managedProtocolJSONHeaders, []byte(managedProtocolChatReply), false)
	if observed.Usage.Status != "missing" || observed.BodyComplete || observed.ExecutionResult != "unknown" {
		t.Fatal("incomplete host body read became success")
	}
}
