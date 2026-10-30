package main

import (
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const (
	memoryNativeCallerContextKey = "memoryNativeCaller"
	memoryNativeCallContextKey   = "memoryNativeAdmittedCall"
	memoryRouteNameContextKey    = "memoryGatewayRouteName"
)

type memoryNativeIdentity struct {
	config.NativeIdentityConfig
	Caller balanceidentity.Snapshot `json:"caller"`
}

// nativeMemoryIdentity reuses the request-time authenticated snapshot for both
// assembly and response events. A configured native binding always stays
// explicit on the wire; missing state cannot silently select legacy identity.
func nativeMemoryIdentity(ctx wrapper.HttpContext, c config.PluginConfig) *memoryNativeIdentity {
	if c.NativeIdentity == nil {
		return nil
	}
	caller, _ := ctx.GetContext(memoryNativeCallerContextKey).(balanceidentity.Snapshot)
	return &memoryNativeIdentity{NativeIdentityConfig: *c.NativeIdentity, Caller: caller}
}
