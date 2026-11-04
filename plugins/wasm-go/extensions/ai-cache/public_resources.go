package main

import (
	"github.com/higress-group/wasm-go/pkg/ai/resourceref"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

func reissueNativeCacheResources(ctx wrapper.HttpContext, state *nativeCacheDeliveryState, done func(bool)) {
	if state.references == nil {
		done(false)
		return
	}
	scope := resourceref.Scope{Tenant: state.caller.TenantID, Principal: state.caller.PrincipalID, SalesAsset: state.config.salesAssetID}
	resourceref.Reissue(state.references, scope, state.publicAliases.Records(), func(err error) {
		if err != nil {
			ctx.SetUserAttribute("cache_reference_status", "unavailable")
		}
		done(err == nil)
	})
}
