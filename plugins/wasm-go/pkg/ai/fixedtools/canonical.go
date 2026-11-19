package fixedtools

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
)

// InspectCanonical resolves requested fixed server tools from a decoded canonical
// Request over supplier defaults. Client functions (ToolKindFunction) never match
// fixed server tools even when sharing the same name.
func InspectCanonical(req canonical.Request, defaults []Tool) (map[Tool]bool, error) {
	requested := map[Tool]bool{}
	for _, field := range []struct {
		flag canonical.Optional[bool]
		tool Tool
	}{
		{req.FixedTools.EnableSearch, WebSearch},
		{req.FixedTools.EnableCodeInterpreter, CodeInterpreter},
	} {
		if field.flag.IsNull() {
			return nil, errors.New("invalid tool flag")
		}
		enabled := slices.Contains(defaults, field.tool)
		if field.flag.IsPresent() {
			enabled = field.flag.Val
		}
		if enabled {
			requested[field.tool] = true
		}
	}

	if requested[WebSearch] {
		if req.FixedTools.SearchStrategy.IsPresent() && req.FixedTools.SearchStrategy.Val == "agent_max" {
			requested[WebExtractor] = true
		}
		if raw := bytes.TrimSpace(req.FixedTools.SearchOptionsRaw); len(raw) > 0 {
			var options struct {
				Strategy string `json:"strategy"`
			}
			if err := json.Unmarshal(raw, &options); err != nil {
				return nil, errors.New("invalid search options")
			}
			if options.Strategy == "agent_max" {
				requested[WebExtractor] = true
			}
		}
	}

	for _, item := range req.Tools {
		if item.Kind != canonical.ToolKindFixedServer {
			continue
		}
		if tool, ok := Kind(item.FixedType); ok {
			requested[tool] = true
		}
	}
	return requested, nil
}

// FilterCanonical returns a deep-cloned copy of req with removed fixed server tools
// stripped, explicit false switches written for removed boolean flags so supplier
// defaults cannot re-enable them, and any resulting dangling ToolChoice cleared.
// It never mutates req or its shared maps/slices.
func FilterCanonical(req canonical.Request, defaults []Tool, removed map[Tool]bool) canonical.Request {
	out := req.CloneForAttempt()

	if removed[WebSearch] {
		out.FixedTools.EnableSearch = canonical.Some(false)
		out.FixedTools.WebSearchOptionsRaw = nil
	} else if slices.Contains(defaults, WebSearch) && out.FixedTools.EnableSearch.IsAbsent() {
		out.FixedTools.EnableSearch = canonical.Some(true)
	}

	if removed[CodeInterpreter] {
		out.FixedTools.EnableCodeInterpreter = canonical.Some(false)
	} else if slices.Contains(defaults, CodeInterpreter) && out.FixedTools.EnableCodeInterpreter.IsAbsent() {
		out.FixedTools.EnableCodeInterpreter = canonical.Some(true)
	}

	if removed[WebExtractor] {
		if out.FixedTools.SearchStrategy.IsPresent() && out.FixedTools.SearchStrategy.Val == "agent_max" {
			out.FixedTools.SearchStrategy = canonical.AbsentOf[string]()
		}
		if len(out.FixedTools.SearchOptionsRaw) > 0 {
			var options map[string]json.RawMessage
			if json.Unmarshal(out.FixedTools.SearchOptionsRaw, &options) == nil && string(options["strategy"]) == `"agent_max"` {
				delete(options, "strategy")
				out.FixedTools.SearchOptionsRaw, _ = json.Marshal(options)
			}
		}
	}

	if len(out.Tools) > 0 {
		kept := make([]canonical.ToolDefinition, 0, len(out.Tools))
		removedNames, keptNames := map[string]bool{}, map[string]bool{}
		for _, item := range out.Tools {
			if item.Kind == canonical.ToolKindFixedServer {
				if tool, fixed := Kind(item.FixedType); fixed && removed[tool] {
					if item.FixedType != "" {
						removedNames[item.FixedType] = true
					}
					if item.Name != "" {
						removedNames[item.Name] = true
					}
					removedNames[string(tool)] = true
					continue
				}
				if item.FixedType != "" {
					keptNames[item.FixedType] = true
				}
				if item.Name != "" {
					keptNames[item.Name] = true
				}
				kept = append(kept, item)
				continue
			}
			if item.Name != "" {
				keptNames[item.Name] = true
			}
			kept = append(kept, item)
		}

		if len(kept) != len(out.Tools) {
			if len(kept) == 0 {
				out.Tools = nil
			} else {
				out.Tools = kept
			}
			dangling := false
			switch out.ToolChoice.Mode {
			case canonical.ToolChoiceRequired, canonical.ToolChoiceAny:
				dangling = len(kept) == 0
			case canonical.ToolChoiceFixed:
				target := out.ToolChoice.FixedRaw
				if target == "" {
					target = out.ToolChoice.Name
				}
				if tool, fixed := Kind(target); fixed && removed[tool] {
					dangling = true
				} else if target != "" && removedNames[target] && !keptNames[target] {
					dangling = true
				}
			case canonical.ToolChoiceFunction:
				if out.ToolChoice.Name != "" && removedNames[out.ToolChoice.Name] && !keptNames[out.ToolChoice.Name] {
					dangling = true
				}
			}
			if dangling {
				out.ToolChoice = canonical.ToolChoice{}
			}
		}
	}

	return out
}
