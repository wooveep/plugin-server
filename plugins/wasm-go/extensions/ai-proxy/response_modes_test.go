package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/resourceref"
	"github.com/tidwall/gjson"
)

func responseModeFixture(supplier, protocol, upstream, mode string, profile clientresponse.Profile) map[string]interface{} {
	fixture := managedConfigFixture()
	fixture["provider"] = map[string]interface{}{"type": supplier, "apiTokens": []string{"controlled-upstream-key"}, "qwenEnableCompatible": true}
	fixture["protocol_mode"] = mode
	settings := fixture["managed_execution"].(managedSettings)
	settings.InputProtocol, settings.UpstreamProtocol, settings.ResponseProfile = protocol, upstream, profile
	fixture["managed_execution"] = settings
	if mode == "passthrough" {
		release := fixture["_modelfusion_execution"].(managedRelease)
		release.PublicModel = release.Descriptor.UpstreamModel
		fixture["_modelfusion_execution"] = release
	}
	return fixture
}

func responseModePayload(protocol string, stream bool) []byte {
	chat := `{"id":"native-chat","model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello","reasoning_content":"thought"},"finish_reason":"stop"}],"usage":{"prompt_tokens":17,"completion_tokens":5,"total_tokens":22}}`
	response := `{"id":"native-response","object":"response","model":"upstream-model","status":"completed","output":[{"id":"native-item","type":"message","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[]}]}],"usage":{"input_tokens":17,"output_tokens":5,"total_tokens":22}}`
	message := `{"id":"native-message","type":"message","role":"assistant","model":"upstream-model","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":17,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`
	if !stream {
		return []byte(map[string]string{clientresponse.Chat: chat, clientresponse.Responses: response, clientresponse.Messages: message}[protocol])
	}
	switch protocol {
	case clientresponse.Chat:
		return managedProtocolChatStream("17", "5")
	case clientresponse.Responses:
		return []byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"native-item\",\"output_index\":0,\"content_index\":0,\"delta\":\"hello\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":" + response + "}\n\n")
	default:
		return []byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"native-message\",\"model\":\"upstream-model\",\"role\":\"assistant\",\"content\":[],\"usage\":{\"input_tokens\":17,\"output_tokens\":0,\"cache_read_input_tokens\":0,\"cache_creation_input_tokens\":0}}}\n\nevent: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	}
}

func responseModeRequest(protocol, model string, stream bool) []byte {
	if protocol == clientresponse.Responses {
		return []byte(fmt.Sprintf(`{"model":%q,"input":"hello","stream":%t}`, model, stream))
	}
	return []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hello"}],"max_tokens":16,"stream":%t}`, model, stream))
}

func TestResponseModesControlledValidationMatrix(t *testing.T) {
	type cell struct {
		id, supplier, client, upstream string
		profile                        clientresponse.Profile
		raw                            bool
	}
	var cells []cell
	for i, protocol := range []string{clientresponse.Chat, clientresponse.Responses, clientresponse.Messages} {
		cells = append(cells, cell{fmt.Sprintf("V-M%02d", i+1), "qwen", protocol, protocol, clientresponse.DeepSeek, false})
		cells = append(cells, cell{fmt.Sprintf("V-M%02d", i+4), "deepseek", protocol, protocol, clientresponse.Qwen, false})
		cells = append(cells, cell{fmt.Sprintf("V-R%02d", i+1), "deepseek", protocol, protocol, "", true})
		cells = append(cells, cell{fmt.Sprintf("V-R%02d", i+4), "qwen", protocol, protocol, "", true})
	}
	for i, protocol := range []string{clientresponse.Chat, clientresponse.Responses} {
		cells = append(cells, cell{fmt.Sprintf("V-M%02d", i+7), "deepseek", protocol, protocol, clientresponse.OpenAI, false})
		cells = append(cells, cell{fmt.Sprintf("V-R%02d", i+7), "openai", protocol, protocol, "", true})
	}
	cells = append(cells, cell{"V-M09", "openai", clientresponse.Messages, clientresponse.Chat, clientresponse.DeepSeek, false}, cell{"V-M10", "openai", clientresponse.Messages, clientresponse.Chat, clientresponse.Qwen, false})
	for _, cell := range cells {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", cell.id, stream), func(t *testing.T) {
				mode, public := "auto", "public-model"
				if cell.raw {
					mode, public = "passthrough", "upstream-model"
				}
				h := newManagedHost(t, responseModeFixture(cell.supplier, cell.client, cell.upstream, mode, cell.profile))
				h.start(t, responseModeRequest(cell.client, public, stream), responseModeRequest(cell.client, "upstream-model", stream))
				original := responseModePayload(cell.upstream, stream)
				contentType := "application/json"
				if stream {
					contentType = "text/event-stream"
				}
				h.response(t, [][2]string{{":status", "200"}, {"content-type", contentType}, {"x-request-id", "supplier-request"}, {"x-supplier-business", "one"}, {"x-supplier-business", "two"}}, original)
				reply := h.host.GetSentLocalResponse(h.id)
				if reply == nil || reply.StatusCode != 200 {
					t.Fatalf("delivery missing: %#v", reply)
				}
				if cell.raw {
					if !bytes.Equal(reply.Data, original) {
						t.Fatal("raw bytes changed", string(reply.Data))
					}
					var business []string
					for _, header := range reply.Headers {
						if header[0] == "x-supplier-business" {
							business = append(business, header[1])
						}
					}
					if strings.Join(business, ",") != "one,two" {
						t.Fatal("raw repeated business headers changed", business)
					}
				} else if !bytes.Contains(reply.Data, []byte("hello")) || bytes.Contains(reply.Data, []byte(`"model":"upstream-model"`)) {
					t.Fatal("mapped content or public identity changed", string(reply.Data))
				}
				if h.bridge(t).Envelope().Usage.Status != "reported" || len(h.records(t)) != 2 {
					t.Fatal("response mode changed original metering or attempts")
				}
			})
		}
	}
}

func TestPassthroughKeepsOpaqueErrorsWithoutInventingProcurementEvidence(t *testing.T) {
	for _, status := range []string{"200", "401", "429", "500", "503"} {
		t.Run(status, func(t *testing.T) {
			h := newManagedHost(t, responseModeFixture("openai", clientresponse.Chat, clientresponse.Chat, "passthrough", ""))
			h.start(t, responseModeRequest(clientresponse.Chat, "upstream-model", false), responseModeRequest(clientresponse.Chat, "upstream-model", false))
			body := []byte("  supplier maintenance\nopaque payload  ")
			h.response(t, [][2]string{{":status", status}, {"content-type", "text/plain"}, {"retry-after", "2"}}, body)
			reply := h.host.GetSentLocalResponse(h.id)
			if reply == nil || fmt.Sprint(reply.StatusCode) != status || !bytes.Equal(reply.Data, body) {
				t.Fatalf("opaque error rewritten: %#v", reply)
			}
			if status == "500" || status == "503" {
				for _, record := range h.records(t) {
					if record.messageType == "observation" {
						fact := decodeManagedFact(t, record.raw)
						if fact.HTTPStatus != nil || fact.SendState == "observed_sent" {
							t.Fatal("raw presentation fabricated send evidence")
						}
					}
				}
			}
		})
	}
}

func TestPassthroughRejectsModelProtocolProfileAndActualNativeFormatMismatch(t *testing.T) {
	newManagedHost(t)
	for _, cause := range []string{"model", "protocol", "profile", "native-format"} {
		t.Run(cause, func(t *testing.T) {
			fixture := responseModeFixture("openai", clientresponse.Chat, clientresponse.Chat, "passthrough", "")
			switch cause {
			case "model":
				release := fixture["_modelfusion_execution"].(managedRelease)
				release.PublicModel = "public-alias"
				fixture["_modelfusion_execution"] = release
			case "protocol":
				settings := fixture["managed_execution"].(managedSettings)
				settings.InputProtocol = clientresponse.Messages
				fixture["managed_execution"] = settings
			case "profile":
				settings := fixture["managed_execution"].(managedSettings)
				settings.ResponseProfile = clientresponse.Qwen
				fixture["managed_execution"] = settings
			case "native-format":
				fixture["provider"] = map[string]interface{}{"type": "qwen", "apiTokens": []string{"controlled-key"}, "qwenEnableCompatible": false}
			}
			var cfg config.PluginConfig
			value := gjson.ParseBytes(managedFixtureJSON(t, fixture))
			cfg.FromJson(value)
			if err := cfg.Validate(); err != nil {
				t.Fatal("fixture is invalid before response policy", err)
			}
			if err := cfg.Complete(); err != nil {
				t.Fatal("fixture failed before response policy", err)
			}
			err := configureManagedExecution(value, &cfg)
			if err == nil || !strings.Contains(err.Error(), "passthrough requires") || cfg.ManagedExecution != nil {
				t.Fatal("ineligible raw target retained execution authority", err)
			}
		})
	}
}

func TestPassthroughKeepsIncompleteSSEWithoutAddingATerminal(t *testing.T) {
	h := newManagedHost(t, responseModeFixture("openai", clientresponse.Chat, clientresponse.Chat, "passthrough", ""))
	request := responseModeRequest(clientresponse.Chat, "upstream-model", true)
	h.start(t, request, request)
	body := []byte("data: {\"id\":\"native\",\"choices\":[{\"delta\":{\"content\":\"partial\"},\"index\":0}]}\n\n")
	h.response(t, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, body)
	reply := h.host.GetSentLocalResponse(h.id)
	if reply == nil || reply.StatusCode != 200 || !bytes.Equal(reply.Data, body) {
		t.Fatal("raw partial stream was replaced or synthesized", reply)
	}
}

func TestPassthroughResourceOwnershipRestoresOriginalID(t *testing.T) {
	h := newManagedHost(t, responseModeFixture("openai", clientresponse.Responses, clientresponse.Responses, "passthrough", ""))
	original := responseModeRequest(clientresponse.Responses, "upstream-model", false)
	h.start(t, original, original)
	h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, responseModePayload(clientresponse.Responses, false))
	if h.host.GetSentLocalResponse(h.id) == nil {
		t.Fatal("resource delivery missing")
	}
	cfg := h.cfg.ManagedExecution.(*managedExecutionConfig)
	record := resourceref.NewRecord(clientresponse.ResourceAlias{Kind: "response", Public: "native-response", Original: "native-response"}, managedResourceOwner(cfg), true)
	record.Passthrough = true
	raw, _ := json.Marshal(record)
	h.id = h.host.InitializeHttpContext()
	request := []byte(`{"model":"upstream-model","input":"continue","previous_response_id":"native-response"}`)
	h.start(t, request, request)
	for _, call := range h.host.GetRedisCalloutAttributesFromContext(h.id) {
		if bytes.Contains(call.Query, []byte("MGET")) {
			h.host.CallOnRedisCallResponse(call.CalloutID, 0, []byte(fmt.Sprintf("*1\r\n$%d\r\n%s\r\n", len(raw), raw)))
		}
	}
	calls := h.host.GetCalloutAttributesFromContext(h.id)
	if len(calls) != 1 || gjson.GetBytes(calls[0].Body, "previous_response_id").String() != "native-response" {
		t.Fatal("native reference lost its original owner or ID", calls)
	}
}
