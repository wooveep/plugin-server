package memorycache

import "testing"

func TestIsCacheHitResponseCodeDetails(t *testing.T) {
	tests := []struct {
		name    string
		details string
		want    bool
	}{
		{name: "versioned projected plugin", details: "via_wasm::higress-system.ai-cache-1.0.1::ai-cache.hit", want: true},
		{name: "internal projected plugin", details: "via_wasm::higress-system.ai-cache.internal::ai-cache.hit", want: true},
		{name: "bare plugin name", details: "via_wasm::ai-cache::ai-cache.hit", want: true},
		{name: "raw detail", details: "ai-cache.hit"},
		{name: "different plugin", details: "via_wasm::other-plugin::ai-cache.hit"},
		{name: "empty plugin", details: "via_wasm::::ai-cache.hit"},
		{name: "extended detail", details: "via_wasm::higress-system.ai-cache-1.0.1::ai-cache.hit.extra"},
		{name: "extra segment", details: "via_wasm::higress-system.ai-cache-1.0.1::ai-cache.hit::extra"},
		{name: "upstream", details: "via_upstream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCacheHitResponseCodeDetails(tt.details); got != tt.want {
				t.Fatalf("IsCacheHitResponseCodeDetails(%q) = %v, want %v", tt.details, got, tt.want)
			}
		})
	}
}
