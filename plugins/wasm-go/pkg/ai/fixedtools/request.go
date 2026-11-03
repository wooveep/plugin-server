package fixedtools

import (
	"encoding/json"
	"errors"
	"strings"
)

// Kind recognizes built-in tool types, never the name of a client function.
func Kind(kind string) (Tool, bool) {
	switch {
	case kind == "web_search" || kind == "web_search_preview" || strings.HasPrefix(kind, "web_search_20"):
		return WebSearch, true
	case kind == "web_extractor" || kind == "web_fetch" || strings.HasPrefix(kind, "web_fetch_20"):
		return WebExtractor, true
	case kind == "code_interpreter":
		return CodeInterpreter, true
	case kind == "image_search":
		return ImageSearch, true
	case kind == "web_search_image":
		return WebSearchImage, true
	default:
		return "", false
	}
}

// Inspect resolves explicit request flags over supplier defaults.
func Inspect(body []byte, protocol string, defaults []Tool) (map[Tool]bool, error) {
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil || object == nil {
		return nil, errors.New("invalid tool request")
	}
	requested := map[Tool]bool{}
	parameters := object
	if strings.HasPrefix(protocol, "dashscope_") {
		parameters = map[string]json.RawMessage{}
		if raw, exists := object["parameters"]; exists && (json.Unmarshal(raw, &parameters) != nil || parameters == nil) {
			return nil, errors.New("invalid tool parameters")
		}
	}
	for _, field := range []struct {
		name string
		tool Tool
	}{{"enable_search", WebSearch}, {"enable_code_interpreter", CodeInterpreter}} {
		enabled := false
		for _, tool := range defaults {
			enabled = enabled || tool == field.tool
		}
		if raw, exists := parameters[field.name]; exists {
			if string(raw) == "null" || json.Unmarshal(raw, &enabled) != nil {
				return nil, errors.New("invalid tool flag")
			}
		}
		if enabled {
			requested[field.tool] = true
		}
	}
	if requested[WebSearch] {
		var options struct {
			Strategy string `json:"strategy"`
		}
		if raw, exists := parameters["search_options"]; exists && json.Unmarshal(raw, &options) != nil {
			return nil, errors.New("invalid search options")
		}
		if options.Strategy == "agent_max" {
			requested[WebExtractor] = true
		}
	}
	if raw, exists := parameters["tools"]; exists {
		var tools []struct {
			Type string `json:"type"`
		}
		if string(raw) == "null" || json.Unmarshal(raw, &tools) != nil {
			return nil, errors.New("invalid tool declarations")
		}
		for _, item := range tools {
			if tool, ok := Kind(item.Type); ok {
				requested[tool] = true
			}
		}
	}
	return requested, nil
}

// Filter removes only unsupported fixed tools and their dangling forced choices.
// Explicit false flags survive so a provider default cannot turn a tool back on.
func Filter(body []byte, protocol string, defaults []Tool, removed map[Tool]bool) ([]byte, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		return nil, err
	}
	parameters := object
	native := strings.HasPrefix(protocol, "dashscope_")
	if native {
		parameters = map[string]json.RawMessage{}
		if raw, exists := object["parameters"]; exists {
			if err := json.Unmarshal(raw, &parameters); err != nil {
				return nil, err
			}
		}
	}
	changed := false
	for _, field := range []struct {
		name string
		tool Tool
	}{{"enable_search", WebSearch}, {"enable_code_interpreter", CodeInterpreter}} {
		present := false
		for _, tool := range defaults {
			present = present || tool == field.tool
		}
		if removed[field.tool] {
			parameters[field.name] = json.RawMessage("false")
			changed = true
		} else if present {
			if _, exists := parameters[field.name]; !exists {
				parameters[field.name] = json.RawMessage("true")
				changed = true
			}
		}
	}
	if removed[WebExtractor] {
		if raw, exists := parameters["search_options"]; exists {
			var options map[string]json.RawMessage
			if json.Unmarshal(raw, &options) == nil && string(options["strategy"]) == `"agent_max"` {
				delete(options, "strategy")
				parameters["search_options"], _ = json.Marshal(options)
				changed = true
			}
		}
	}
	var tools []json.RawMessage
	if raw, exists := parameters["tools"]; exists {
		if err := json.Unmarshal(raw, &tools); err != nil {
			return nil, err
		}
		kept := make([]json.RawMessage, 0, len(tools))
		removedNames, keptNames := map[string]bool{}, map[string]bool{}
		for _, rawTool := range tools {
			var item struct {
				Type     string `json:"type"`
				Name     string `json:"name"`
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			if err := json.Unmarshal(rawTool, &item); err != nil {
				return nil, err
			}
			if tool, fixed := Kind(item.Type); fixed && removed[tool] {
				removedNames[item.Type], removedNames[item.Name], removedNames[string(tool)] = true, true, true
				changed = true
			} else {
				kept = append(kept, rawTool)
				keptNames[item.Name], keptNames[item.Function.Name], keptNames[item.Type] = true, true, true
			}
		}
		if len(kept) != len(tools) {
			if len(kept) == 0 {
				delete(parameters, "tools")
			} else {
				parameters["tools"], _ = json.Marshal(kept)
			}
			if raw, exists := parameters["tool_choice"]; exists {
				var choice string
				dangling := json.Unmarshal(raw, &choice) == nil && (choice == "required" || choice == "any") && len(kept) == 0
				var selected struct {
					Type string `json:"type"`
					Name string `json:"name"`
				}
				if json.Unmarshal(raw, &selected) == nil {
					dangling = dangling || (selected.Type == "any" || selected.Type == "required") && len(kept) == 0
					tool, fixed := Kind(selected.Type)
					dangling = dangling || fixed && removed[tool] || selected.Type == "tool" && selected.Name != "" && removedNames[selected.Name] && !keptNames[selected.Name]
				}
				if dangling {
					delete(parameters, "tool_choice")
				}
			}
		}
	}
	if !changed {
		return append([]byte(nil), body...), nil
	}
	if native {
		object["parameters"], _ = json.Marshal(parameters)
	}
	return json.Marshal(object)
}
