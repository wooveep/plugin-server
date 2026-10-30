package main

import (
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/memorycache"
)

func memoryResponseIsCacheReplay() bool {
	detail, err := proxywasm.GetProperty([]string{"response", "code_details"})
	return err == nil && memorycache.IsCacheHitResponseCodeDetails(string(detail))
}
