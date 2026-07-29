package config

import (
	"strings"

	"github.com/tidwall/gjson"
)

const (
	ProtocolModeAuto        = "auto"
	ProtocolModePassthrough = "passthrough"
	PathScopeLegacySuffixes = "legacy_suffixes"
	PathScopeBoundTarget    = "bound_target"

	maxRoutingFilterEntries = 64
	maxPathSuffixBytes      = 256
	maxHTTPMethodBytes      = 32
)

func (c *PluginConfig) parseRoutingConfig(json gjson.Result) {
	c.routingConfigError = ""
	c.enablePathSuffixes = cloneStrings(c.enablePathSuffixes)
	c.enableMethods = cloneStrings(c.enableMethods)

	if value := json.Get("protocol_mode"); value.Exists() {
		c.protocolModeConfigured = true
		if value.Type != gjson.String {
			c.setRoutingConfigError("protocol_mode must be auto or passthrough")
		} else {
			c.protocolMode = strings.TrimSpace(value.String())
		}
	}

	if value := json.Get("path_scope"); value.Exists() {
		c.pathScopeConfigured = true
		if value.Type != gjson.String {
			c.setRoutingConfigError("path_scope must be legacy_suffixes or bound_target")
		} else {
			c.pathScope = strings.TrimSpace(value.String())
		}
	}

	if value := json.Get("enable_path_suffixes"); value.Exists() {
		suffixes, ok := parseRoutingStringList(value, maxPathSuffixBytes, false)
		if !ok {
			c.setRoutingConfigError("enable_path_suffixes must be an array of bounded strings")
		} else {
			c.enablePathSuffixes = suffixes
		}
	}

	if value := json.Get("enable_methods"); value.Exists() {
		methods, ok := parseRoutingStringList(value, maxHTTPMethodBytes, true)
		if !ok {
			c.setRoutingConfigError("enable_methods must be an array of bounded HTTP method strings")
		} else {
			c.enableMethods = methods
		}
	}
}

func (c *PluginConfig) GetProtocolMode() string {
	if !c.protocolModeConfigured {
		return ProtocolModeAuto
	}
	return c.protocolMode
}

func (c *PluginConfig) GetPathScope() string {
	if !c.pathScopeConfigured {
		return PathScopeLegacySuffixes
	}
	return c.pathScope
}

func (c *PluginConfig) GetEnablePathSuffixes() []string {
	return cloneStrings(c.enablePathSuffixes)
}

func (c *PluginConfig) GetEnableMethods() []string {
	return cloneStrings(c.enableMethods)
}

func (c *PluginConfig) MatchesBoundTargetFilters(method string, normalizedPath string) bool {
	if c.GetPathScope() != PathScopeBoundTarget {
		return true
	}
	if len(c.enablePathSuffixes) > 0 && !matchesSuffixFilter(c.enablePathSuffixes, normalizedPath) {
		return false
	}
	if len(c.enableMethods) > 0 && !matchesMethodFilter(c.enableMethods, method) {
		return false
	}
	return true
}

func (c *PluginConfig) setRoutingConfigError(message string) {
	if c.routingConfigError == "" {
		c.routingConfigError = message
	}
}

func parseRoutingStringList(value gjson.Result, maxEntryBytes int, normalizeMethod bool) ([]string, bool) {
	if !value.IsArray() {
		return nil, false
	}

	values := value.Array()
	if len(values) > maxRoutingFilterEntries {
		return nil, false
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, item := range values {
		if item.Type != gjson.String {
			return nil, false
		}
		entry := strings.TrimSpace(item.String())
		if entry == "" {
			continue
		}
		if len(entry) > maxEntryBytes {
			return nil, false
		}
		if normalizeMethod {
			entry = strings.ToUpper(entry)
			if !isHTTPToken(entry) {
				return nil, false
			}
		}
		if _, exists := seen[entry]; exists {
			continue
		}
		seen[entry] = struct{}{}
		normalized = append(normalized, entry)
	}
	return normalized, true
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}

func matchesSuffixFilter(suffixes []string, normalizedPath string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(normalizedPath, suffix) {
			return true
		}
	}
	return false
}

func matchesMethodFilter(methods []string, method string) bool {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	for _, configuredMethod := range methods {
		if normalizedMethod == configuredMethod {
			return true
		}
	}
	return false
}

func isHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') {
			continue
		}
		switch character {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}
