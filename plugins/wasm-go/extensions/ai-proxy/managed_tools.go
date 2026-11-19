package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
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
	if managedUsesCanonicalIR(s.config) {
		baseReq, err := ensureManagedBaseCanonicalRequest(s, body)
		if err == nil {
			authorizeManagedCanonicalTools(ctx, s, *baseReq, ready)
			return
		}
		var rej canonical.RejectionError
		if errors.As(err, &rej) && (rej.SafeCode() == "invalid_tool_flag" || rej.SafeCode() == "invalid_search_options") {
			rejectManagedPreparation(ctx, s, 400, "invalid_fixed_tools")
			return
		}
	}
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
	finishManagedToolAuthorization(ctx, s, requested, ready)
}

func authorizeManagedCanonicalTools(ctx wrapper.HttpContext, s *managedExecutionState, req canonical.Request, ready func()) {
	p := s.config.settings.ToolPolicy
	var defaults []fixedtools.Tool
	if p != nil {
		defaults = p.Defaults
	}
	requested, err := fixedtools.InspectCanonical(req, defaults)
	if err != nil {
		rejectManagedPreparation(ctx, s, 400, "invalid_fixed_tools")
		return
	}
	finishManagedToolAuthorization(ctx, s, requested, ready)
}

func finishManagedToolAuthorization(ctx wrapper.HttpContext, s *managedExecutionState, requested map[fixedtools.Tool]bool, ready func()) {
	p := s.config.settings.ToolPolicy
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
	if err := s.config.priceRedis.Command([]interface{}{"GET", p.PriceKey}, func(value resp.Value) {
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

func resolveManagedAttemptRemovedTools(ctx wrapper.HttpContext, s *managedExecutionState) (map[fixedtools.Tool]bool, []fixedtools.Tool, error) {
	s.removedTools = map[fixedtools.Tool]bool{}
	p := s.config.settings.ToolPolicy
	if p == nil {
		return nil, nil, nil
	}
	effectiveRemoved := make(map[fixedtools.Tool]bool, len(s.requestedTools)+len(p.Defaults))
	for tool := range s.requestedTools {
		if !containsManagedTool(p.Allowed, tool) {
			return nil, nil, errors.New("fallback tool permission differs")
		}
		if !containsManagedTool(p.Supported, tool) {
			effectiveRemoved[tool] = true
			s.removedTools[tool] = true
		}
	}
	for _, tool := range p.Defaults {
		if !s.requestedTools[tool] {
			effectiveRemoved[tool] = true
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
	return effectiveRemoved, p.Defaults, nil
}

func prepareManagedTools(ctx wrapper.HttpContext, s *managedExecutionState, body []byte) ([]byte, error) {
	if s.config.settings.ToolPolicy == nil {
		return body, nil
	}
	effectiveRemoved, defaults, err := resolveManagedAttemptRemovedTools(ctx, s)
	if err != nil {
		return nil, err
	}
	return fixedtools.Filter(body, s.config.profile.InputProtocol, defaults, effectiveRemoved)
}

// Tool counts come from the original terminal usage, never output item counts.
func managedToolUsage(s *managedExecutionState, body []byte, outcome executionfacts.AttemptOutcome, headers [][2]string) map[fixedtools.Tool]executionfacts.ToolMeasurement {
	if s.config.settings.ToolPolicy == nil {
		return nil
	}
	usage := gjson.Result{}
	if stream, err := managedProtocolContentType(headers); err == nil && stream {
		if events, err := managedProtocolEvents(body); err == nil {
			for _, event := range events {
				usage = managedToolEventUsage(s.config.profile.UpstreamProtocol, event.Data, usage)
			}
		}
	} else {
		usage = gjson.GetBytes(body, "usage")
	}
	return managedToolMeasurements(s, usage, outcome)
}

func managedToolEventUsage(protocol string, data []byte, previous gjson.Result) gjson.Result {
	object := gjson.ParseBytes(data)
	if protocol == managedProtocolChat {
		if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
			return previous
		}
		// Only the terminal Chat usage can establish this platform meter.
		// A preceding incremental count is not retained across later frames.
		return object.Get("usage")
	}
	kind := object.Get("type").String()
	if protocol == managedProtocolResponses {
		if kind == "response.completed" || kind == "response.incomplete" || kind == "response.failed" {
			return object.Get("response.usage")
		}
		return previous
	}
	if usage := object.Get("usage"); usage.IsObject() && (protocol != managedProtocolMessages || kind == "message_delta") {
		return usage
	}
	return previous
}

func managedToolMeasurements(s *managedExecutionState, usage gjson.Result, outcome executionfacts.AttemptOutcome) map[fixedtools.Tool]executionfacts.ToolMeasurement {
	p := s.config.settings.ToolPolicy
	if p == nil {
		return nil
	}
	result := map[fixedtools.Tool]executionfacts.ToolMeasurement{}
	complete := outcome.BodyComplete && outcome.ExecutionResult != "unknown"
	for _, tool := range [5]fixedtools.Tool{fixedtools.WebSearch, fixedtools.WebExtractor, fixedtools.CodeInterpreter, fixedtools.ImageSearch, fixedtools.WebSearchImage} {
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
		case p.Supplier == "qwen" && protocol == managedProtocolChat && tool == fixedtools.WebSearch:
			count = usage.Get("x_tools.search.count")
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
			measurement := executionfacts.ToolMeasurement{Status: status, Count: value.String(), Source: "provider"}
			if p.Supplier == "qwen" && protocol == managedProtocolChat && tool == fixedtools.WebSearch {
				// User-selected platform accounting rule, not a supplier invoice claim.
				measurement.ReasonCode = "platform_qwen_openai_chat_usage_x_tools_search_count"
			}
			result[tool] = measurement
		}
	}
	return result
}
