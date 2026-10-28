package main

import (
	"errors"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

func replaceMemoryRequestBody(ctx wrapper.HttpContext, input memoryMessageAssemblyInput, log log.Log) bool {
	original := ctx.GetByteSliceContext(memoryOriginalBodyContextKey, nil)
	if len(original) == 0 {
		log.Warnf("[ai-memory] original request body unavailable, fail open")
		return false
	}
	if memoryRequestInjectionUsesProtocolAdapter(ctx) {
		return replaceProtocolMemoryRequestBody(ctx, original, input, log)
	}
	current, err := currentMemoryMessages(ctx)
	if err != nil {
		log.Warnf("[ai-memory] current request messages unavailable, fail open")
		return false
	}
	input.Current = current
	if !hasEffectiveMemoryInjection(input) {
		return false
	}
	replaced, err := sessionctx.ReplaceOpenAIChatMessages(original, assembleFinalMemoryMessages(input))
	if err != nil {
		log.Warnf("[ai-memory] request body assembly failed open: %v", err)
		return false
	}
	if err := memoryReplaceHTTPRequestBody(replaced); err != nil {
		log.Warnf("[ai-memory] request body replacement failed open: %v", err)
		return false
	}
	if err := writeMemoryCacheDigest(input); err != nil {
		log.Warnf("[ai-memory] trusted cache digest unavailable after injection: %v", err)
	}
	return true
}

func memoryRequestInjectionUsesProtocolAdapter(ctx wrapper.HttpContext) bool {
	return protocol.ProtocolKind(ctx.GetStringContext(memoryProtocolContextKey, "")) != protocol.ProtocolChatCompletions
}

func replaceProtocolMemoryRequestBody(ctx wrapper.HttpContext, original []byte, input memoryMessageAssemblyInput, log log.Log) bool {
	if !hasEffectiveProtocolMemoryInjection(input) {
		return false
	}
	adapter := memoryProtocolAdapterForKind(protocol.ProtocolKind(ctx.GetStringContext(memoryProtocolContextKey, "")))
	replaced, err := adapter.InjectContext(original, memoryProtocolInjectableContext(input))
	if err != nil {
		log.Warnf("[ai-memory] request body protocol assembly failed open: %v", err)
		return false
	}
	if err := memoryReplaceHTTPRequestBody(replaced); err != nil {
		log.Warnf("[ai-memory] request body replacement failed open: %v", err)
		return false
	}
	if err := writeMemoryCacheDigest(input); err != nil {
		log.Warnf("[ai-memory] trusted cache digest unavailable after protocol injection: %v", err)
	}
	return true
}

func hasEffectiveProtocolMemoryInjection(input memoryMessageAssemblyInput) bool {
	return input.MemoryMessage != nil || len(input.Recent) > 0
}

func replaceMemoryRequestBodyWithRecentFallback(ctx wrapper.HttpContext, log log.Log) bool {
	recent := recentMemoryMessages(ctx)
	if len(recent) == 0 {
		return false
	}
	return replaceMemoryRequestBody(ctx, memoryMessageAssemblyInput{Recent: recent}, log)
}

func currentMemoryMessages(ctx wrapper.HttpContext) ([]sessionctx.OpenAIMessage, error) {
	messages, ok := ctx.GetContext(memoryCurrentMessagesContextKey).([]sessionctx.OpenAIMessage)
	if !ok {
		return nil, errors.New("current messages missing")
	}
	return append([]sessionctx.OpenAIMessage(nil), messages...), nil
}

func recentMemoryMessages(ctx wrapper.HttpContext) []sessionctx.OpenAIMessage {
	messages, ok := ctx.GetContext(memoryRecentMessagesContextKey).([]sessionctx.OpenAIMessage)
	if !ok {
		return nil
	}
	return append([]sessionctx.OpenAIMessage(nil), messages...)
}
