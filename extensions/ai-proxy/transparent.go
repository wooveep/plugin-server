package main

import (
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const ctxKeyTransparentLifecycle = "ai_proxy_transparent_lifecycle"

func initializeProtocolLifecycle(ctx wrapper.HttpContext, mode string) {
	ctx.SetContext(ctxSkipAIProxyLifecycle, false)
	ctx.SetContext(ctxKeyTransparentLifecycle, false)
	ctx.SetContext("needClaudeResponseConversion", false)
	ctx.SetContext(provider.CtxKeyNeedResponsesResponseConversion, false)
	publishNoAdapterMetadata(mode)
}

func markTransparentLifecycle(ctx wrapper.HttpContext) {
	ctx.SetContext(ctxKeyTransparentLifecycle, true)
}

func isTransparentLifecycle(ctx wrapper.HttpContext) bool {
	return ctx.GetBoolContext(ctxKeyTransparentLifecycle, false)
}
