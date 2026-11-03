package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

type managedToolSettings struct {
	Allowed   []fixedtools.Tool `json:"allowed"`
	Supported []fixedtools.Tool `json:"supported"`
	Defaults  []fixedtools.Tool `json:"defaults"`
	Supplier  string            `json:"supplier"`
	PriceKey  string            `json:"price_key"`
}

func validateManagedTools(c *managedExecutionConfig) error {
	p := c.settings.ToolPolicy
	if p == nil {
		return nil
	}
	_, text := c.release.Descriptor.Capabilities["text_generate"]
	if !text || p.PriceKey != "modelfusion:tool-prices:"+c.release.SalesAssetID || !managedID(p.Supplier) {
		return errors.New("invalid model tool policy identity")
	}
	for _, tools := range [][]fixedtools.Tool{p.Allowed, p.Supported, p.Defaults} {
		seen := map[fixedtools.Tool]bool{}
		for _, tool := range tools {
			if !fixedtools.Valid(tool) || seen[tool] {
				return errors.New("invalid model tool policy catalog")
			}
			seen[tool] = true
		}
	}
	return nil
}

func containsManagedTool(tools []fixedtools.Tool, requested fixedtools.Tool) bool {
	for _, tool := range tools {
		if tool == requested {
			return true
		}
	}
	return false
}

// Only tool requests read price availability. Ordinary admission remains the
// existing local identity/balance flow, with no synchronous Console call.
func authorizeManagedTools(ctx wrapper.HttpContext, s *managedExecutionState, body []byte, ready func()) {
	p := s.config.settings.ToolPolicy
	var defaults []fixedtools.Tool
	if p != nil {
		defaults = p.Defaults
	}
	requested, err := fixedtools.Inspect(body, s.config.profile.InputProtocol, defaults)
	if err != nil {
		rejectManagedPreparation(ctx, s, 400, "invalid_fixed_tools")
		return
	}
	s.requestedTools, s.removedTools = requested, map[fixedtools.Tool]bool{}
	if len(requested) == 0 {
		ready()
		return
	}
	for tool := range requested {
		if p == nil || !containsManagedTool(p.Allowed, tool) {
			rejectManagedPreparation(ctx, s, 403, "tool_not_open_for_model_protocol_or_price")
			return
		}
	}
	if err := s.config.redis.Command([]interface{}{"GET", p.PriceKey}, func(value resp.Value) {
		if value.Error() != nil || value.IsNull() {
			rejectManagedPreparation(ctx, s, 503, "tool_price_unavailable")
			return
		}
		priced := "," + value.String() + ","
		for tool := range requested {
			if !strings.Contains(priced, ","+string(tool)+",") {
				rejectManagedPreparation(ctx, s, 403, "tool_sales_price_missing")
				return
			}
		}
		ready()
	}); err != nil {
		rejectManagedPreparation(ctx, s, 503, "tool_price_unavailable")
	}
}

func prepareManagedTools(ctx wrapper.HttpContext, s *managedExecutionState, body []byte) ([]byte, error) {
	p := s.config.settings.ToolPolicy
	if p == nil {
		return body, nil
	}
	removed := map[fixedtools.Tool]bool{}
	for tool := range s.removedTools {
		removed[tool] = true
	}
	for tool := range s.requestedTools {
		if !containsManagedTool(p.Allowed, tool) {
			return nil, errors.New("fallback tool permission differs")
		}
		if !containsManagedTool(p.Supported, tool) {
			removed[tool], s.removedTools[tool] = true, true
		}
	}
	for _, tool := range p.Defaults {
		if !s.requestedTools[tool] {
			removed[tool] = true
		}
	}
	if len(s.removedTools) > 0 {
		var names []string
		for _, tool := range p.Allowed {
			if s.removedTools[tool] {
				names = append(names, string(tool))
			}
		}
		ctx.SetUserAttribute("fixed_tools_removed", strings.Join(names, ","))
	}
	return fixedtools.Filter(body, s.config.profile.InputProtocol, p.Defaults, removed)
}

// Tool counts come from the original terminal usage, never output item counts.
func managedToolUsage(s *managedExecutionState, body []byte, outcome executionfacts.AttemptOutcome, headers [][2]string) map[fixedtools.Tool]executionfacts.ToolMeasurement {
	p := s.config.settings.ToolPolicy
	if p == nil {
		return nil
	}
	result := map[fixedtools.Tool]executionfacts.ToolMeasurement{}
	usage := gjson.Result{}
	complete := outcome.BodyComplete && outcome.ExecutionResult != "unknown"
	if stream, err := managedProtocolContentType(headers); err == nil && stream {
		if events, err := managedProtocolEvents(body); err == nil {
			for _, event := range events {
				if bytes.Equal(event.Data, []byte("[DONE]")) {
					continue
				}
				object := gjson.ParseBytes(event.Data)
				kind := object.Get("type").String()
				switch s.config.profile.UpstreamProtocol {
				case managedProtocolResponses:
					if kind == "response.completed" || kind == "response.incomplete" || kind == "response.failed" {
						usage = object.Get("response.usage")
					}
				case managedProtocolMessages:
					if kind == "message_delta" && object.Get("usage").IsObject() {
						usage = object.Get("usage")
					}
				default:
					if object.Get("usage").IsObject() {
						usage = object.Get("usage")
					}
				}
			}
		} else {
			complete = false
		}
	} else {
		usage = gjson.GetBytes(body, "usage")
	}
	for _, tool := range p.Allowed {
		zero := executionfacts.ToolMeasurement{Status: "known_zero", Count: "0", Source: "protocol_zero", ReasonCode: "tool_not_requested"}
		if s.removedTools[tool] {
			zero.ReasonCode = "outlet_unsupported"
			result[tool] = zero
			continue
		}
		if !s.requestedTools[tool] || outcome.SendState == "not_sent" {
			result[tool] = zero
			continue
		}
		missing := executionfacts.ToolMeasurement{Status: "missing", Source: "none", ReasonCode: "tool_usage_not_observed"}
		result[tool] = missing
		if !complete || !usage.IsObject() {
			continue
		}
		var count gjson.Result
		protocol := s.config.profile.UpstreamProtocol
		switch {
		case p.Supplier == "qwen" && protocol == managedProtocolResponses:
			count = usage.Get("x_tools." + string(tool) + ".count")
		case p.Supplier == "qwen" && protocol == managedProtocolMessages:
			field := map[fixedtools.Tool]string{fixedtools.WebSearch: "web_search_requests", fixedtools.WebExtractor: "web_fetch_requests"}[tool]
			if field != "" {
				count = usage.Get("server_tool_use." + field)
			}
		case p.Supplier == "deepseek" && (protocol == managedProtocolResponses || protocol == managedProtocolMessages):
			field := map[fixedtools.Tool]string{fixedtools.WebSearch: "tool_search", fixedtools.WebExtractor: "tool_fetch"}[tool]
			if field != "" {
				count = usage.Get("server_tool_use." + field)
			}
			if protocol == managedProtocolResponses && !usage.Get("server_tool_use").Exists() && outcome.Usage != nil && outcome.Usage.Status == "reported" {
				zero.Source, zero.ReasonCode = "provider", "complete_usage_reports_no_tools"
				result[tool] = zero
				continue
			}
		default:
			missing.ReasonCode = "supplier_reports_intent_only"
			result[tool] = missing
		}
		if !count.Exists() {
			continue
		}
		var raw json.RawMessage = []byte(count.Raw)
		if value, ok := managedCount(raw); ok {
			status := "reported"
			if value.Sign() == 0 {
				status = "known_zero"
			}
			result[tool] = executionfacts.ToolMeasurement{Status: status, Count: value.String(), Source: "provider"}
		}
	}
	return result
}
