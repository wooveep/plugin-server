package memorycache

import "strings"

const (
	// TrustedDigestProperty is private per-request filter state shared by gateway plugins.
	TrustedDigestProperty = "mse_ai_memory_context_digest"
	// DeprecatedDigestHeader is stripped so callers cannot spoof the private handoff.
	DeprecatedDigestHeader = "x-mse-memory-digest"
	// CacheHitResponseDetail identifies a trusted local replay emitted by AI-Cache.
	CacheHitResponseDetail = "ai-cache.hit"
)

// IsCacheHitResponseCodeDetails accepts only Higress-wrapped local replies from
// an AI-Cache plugin instance. HTTP callers cannot set this StreamInfo fact.
func IsCacheHitResponseCodeDetails(details string) bool {
	parts := strings.Split(details, "::")
	return len(parts) == 3 &&
		parts[0] == "via_wasm" &&
		isAIcachePluginRuntimeName(parts[1]) &&
		parts[2] == CacheHitResponseDetail
}

func isAIcachePluginRuntimeName(name string) bool {
	if index := strings.LastIndex(name, ".ai-cache"); index >= 0 {
		name = name[index+1:]
	}
	if name == "ai-cache" || name == "ai-cache.internal" {
		return true
	}
	version, found := strings.CutPrefix(name, "ai-cache-")
	return found && version != "" && version[0] >= '0' && version[0] <= '9'
}
