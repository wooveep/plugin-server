package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

func TestDirectedConversionCannotRestoreDefaultSearchAfterExplicitOff(t *testing.T) {
	fixture := managedConfigFixture()
	fixture["provider"] = json.RawMessage(managedQwenFixtureProvider)
	settings := fixture["managed_execution"].(managedSettings)
	settings.InputProtocol, settings.UpstreamProtocol, settings.UpstreamPath = managedProtocolResponses, managedProtocolChat, "/api/v1/services/aigc/text-generation/generation"
	settings.ToolPolicy = &managedToolSettings{Allowed: []fixedtools.Tool{fixedtools.WebSearch}, Defaults: []fixedtools.Tool{fixedtools.WebSearch}, Supplier: "qwen", PriceKey: "modelfusion:tool-prices:asset-1"}
	fixture["managed_execution"] = settings
	h := newManagedHost(t, fixture)
	original := []byte(`{"model":"public-model","input":"hello","enable_search":false}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	calls := h.host.GetCalloutAttributesFromContext(h.id)
	if len(calls) != 1 || gjson.GetBytes(calls[0].Body, "parameters.enable_search").Raw != "false" {
		t.Fatal("converter or supplier restored default search")
	}
}

func TestManagedToolsUseOneSendAndOriginalV5Observation(t *testing.T) {
	fixture := managedConfigFixture()
	fixture["provider"] = map[string]interface{}{"type": "deepseek", "apiTokens": []string{"controlled-upstream-key"}}
	settings := fixture["managed_execution"].(managedSettings)
	settings.InputProtocol, settings.UpstreamProtocol, settings.UpstreamPath = "openai_responses", "openai_responses", "/responses"
	settings.ToolPolicy = &managedToolSettings{Allowed: []fixedtools.Tool{fixedtools.WebSearch, fixedtools.CodeInterpreter}, Supported: []fixedtools.Tool{fixedtools.WebSearch}, Supplier: "deepseek", PriceKey: "modelfusion:tool-prices:asset-1"}
	fixture["managed_execution"] = settings
	h := newManagedHost(t, fixture)
	configuration := h.cfg.ManagedExecution.(*managedExecutionConfig)
	originalRedis := configuration.redis
	configuration.priceRedis = managedMediaTestRedis{command: func(args []interface{}, callback wrapper.RedisResponseCallback) error {
		if args[0] == "GET" {
			callback(resp.StringValue("web_search,code_interpreter"))
			return nil
		}
		return originalRedis.Command(args, callback)
	}}
	original := []byte(`{"model":"public-model","input":"hello","tools":[{"type":"web_search"},{"type":"code_interpreter"},{"type":"function","name":"code_interpreter","parameters":{}}],"tool_choice":{"type":"code_interpreter"}}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	calls := h.host.GetCalloutAttributesFromContext(h.id)
	if len(calls) != 1 || len(gjson.GetBytes(calls[0].Body, "tools").Array()) != 2 || gjson.GetBytes(calls[0].Body, "tool_choice").Exists() {
		t.Fatal("Q6 did not retain the effective request in one send")
	}
	h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(`{"id":"resp-1","object":"response","status":"completed","model":"upstream-model","output":[],"usage":{"input_tokens":4,"output_tokens":2,"server_tool_use":{"tool_search":2}}}`))
	for _, record := range h.records(t) {
		if record.messageType == "execution_registration" {
			continue
		}
		fact := decodeManagedFact(t, record.raw)
		if fact.SchemaVersion != executionfacts.ToolSchemaVersion || fact.ToolUsage[fixedtools.WebSearch].Count != "2" || fact.ToolUsage[fixedtools.CodeInterpreter].ReasonCode != "outlet_unsupported" {
			t.Fatalf("original tool evidence lost: %#v", fact.ToolUsage)
		}
		if len(fact.ToolUsage) != 5 || fact.ToolUsage[fixedtools.ImageSearch].Status != "known_zero" {
			t.Fatal("unrequested tools must remain known zero across protocol-specific price rules")
		}
	}
	response := h.host.GetSentLocalResponse(h.id)
	bridge, err := managedSingleHeader(response.Headers, executionfacts.DeliveryBridgeHeader)
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := executionfacts.DecodeToolDeliveryBridge(bridge)
	if err != nil || delivery.Envelope().ToolUsage[fixedtools.WebSearch].Count != "2" {
		t.Fatalf("delivery did not retain tools: %v", err)
	}
}

func TestToolPriceAvailabilityReadRetainsOriginalHTTPContext(t *testing.T) {
	fixture := managedConfigFixture()
	fixture["provider"] = map[string]interface{}{"type": "deepseek", "apiTokens": []string{"controlled-upstream-key"}}
	settings := fixture["managed_execution"].(managedSettings)
	settings.InputProtocol, settings.UpstreamProtocol, settings.UpstreamPath = managedProtocolResponses, managedProtocolResponses, "/responses"
	settings.ToolPolicy = &managedToolSettings{Allowed: []fixedtools.Tool{fixedtools.WebSearch}, Supported: []fixedtools.Tool{fixedtools.WebSearch}, Supplier: "deepseek", PriceKey: "modelfusion:tool-prices:asset-1"}
	fixture["managed_execution"] = settings
	h := newManagedHost(t, fixture)
	original := []byte(`{"model":"public-model","input":"hello","tools":[{"type":"web_search"}]}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	lookups := h.host.GetRedisCalloutAttributesFromContext(h.id)
	if len(lookups) != 1 || len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatal("tool availability must read in the original HTTP context before the supplier send")
	}
	h.host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, []byte("$10\r\nweb_search\r\n"))
	if calls := h.host.GetCalloutAttributesFromContext(h.id); len(calls) != 1 {
		t.Fatal("the price callback did not continue the original single supplier send")
	}
}

func TestManagedToolMeteringUsesTerminalCountsAndKeepsUnknown(t *testing.T) {
	state := &managedExecutionState{config: &managedExecutionConfig{settings: managedSettings{ToolPolicy: &managedToolSettings{Supplier: "qwen", Allowed: []fixedtools.Tool{fixedtools.WebSearch, fixedtools.WebExtractor}}}, profile: managedProtocolProfile{UpstreamProtocol: managedProtocolResponses}}, requestedTools: map[fixedtools.Tool]bool{fixedtools.WebSearch: true, fixedtools.WebExtractor: true}}
	outcome := executionfacts.AttemptOutcome{SendState: "observed_sent", ExecutionResult: "succeeded", BodyComplete: true}
	body := []byte("event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"web_search_call\"}}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":4,\"output_tokens\":2,\"x_tools\":{\"web_search\":{\"count\":2}}}}}\n\n")
	usage := managedToolUsage(state, body, outcome, [][2]string{{"content-type", "text/event-stream"}})
	if usage[fixedtools.WebSearch].Count != "2" || usage[fixedtools.WebExtractor].Status != "missing" {
		t.Fatalf("terminal usage was inferred or summed twice: %#v", usage)
	}
	state.config.profile.UpstreamProtocol = managedProtocolChat
	usage = managedToolUsage(state, []byte(`{"usage":{"x_tools":{"search":{"count":99}}}}`), outcome, [][2]string{{"content-type", "application/json"}})
	if usage[fixedtools.WebSearch].Status != "missing" {
		t.Fatal("search intent was billed as actual use")
	}
	state.config.settings.ToolPolicy.Supplier = "deepseek"
	state.config.profile.UpstreamProtocol = managedProtocolResponses
	outcome.Usage = &executionfacts.Usage{Status: "reported"}
	usage = managedToolUsage(state, []byte(`{"usage":{"input_tokens":4,"output_tokens":2}}`), outcome, [][2]string{{"content-type", "application/json"}})
	if usage[fixedtools.WebSearch].Count != "0" || usage[fixedtools.WebSearch].Status != "known_zero" {
		t.Fatal("complete DeepSeek zero usage became unknown")
	}
	outcome.Usage = executionfacts.MissingTokenUsage("provider_usage_not_observed")
	if managedToolUsage(state, []byte(`{"usage":{}}`), outcome, nil)[fixedtools.WebSearch].Status != "missing" {
		t.Fatal("invalid or absent terminal token usage fabricated a zero tool count")
	}
	outcome.BodyComplete = false
	if managedToolUsage(state, []byte(`{"usage":{"input_tokens":4}}`), outcome, nil)[fixedtools.WebSearch].Status != "missing" {
		t.Fatal("interrupted usage fabricated a zero")
	}
}

func TestChatSSEExplicitErrorIsARecordedFailure(t *testing.T) {
	profile, _ := newManagedProtocolProfile(managedProtocolChat, managedProtocolChat)
	observed := profile.ObserveResponse(200, [][2]string{{"content-type", "text/event-stream"}}, []byte("event: error\n:HTTP_STATUS/400\ndata: {\"code\":\"InvalidParameter\",\"message\":\"url error\"}\n\n"), true)
	if observed.ExecutionResult != "failed" || !observed.ProviderError {
		t.Fatalf("explicit supplier error was lost: %#v", observed)
	}
}
