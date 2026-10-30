package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/stretchr/testify/require"
)

func TestSharedProtocolModelRepresentsAIMemoryEventFacts(t *testing.T) {
	event := MemoryEvent{
		MemoryEventIdentity: MemoryEventIdentity{
			Tenant:    "tenant-a",
			Consumer:  "consumer-a",
			SessionID: "session-a",
		},
		MemoryEventRequest: MemoryEventRequest{
			RequestID:     "request-a",
			RequestPath:   "/v1/chat/completions",
			RequestDigest: "request-digest-a",
		},
		Route:             &MemoryEventRoute{Name: "route-a"},
		Model:             &MemoryEventModel{Name: "qwen-turbo"},
		MemoryEventPolicy: MemoryEventPolicy{PolicyVersion: "7", MemoryMode: "semantic"},
		MemoryEventStatus: MemoryEventStatus{StatusCode: 200},
		MemoryEventStream: MemoryEventStream{IsStream: true},
		MemoryEventToolCall: MemoryEventToolCall{
			ContainsToolCalls: false,
		},
		Usage:             &MemoryEventUsage{Unit: "token", Input: 12, Output: 5, Total: 17},
		MemoryEventFinish: MemoryEventFinish{FinishReason: "stop"},
		UserContent:       "what did we decide?",
		AssistantContent:  "we decided to ship",
	}

	normalized := protocol.NormalizedExchange{
		Request: protocol.RequestFacts{
			Protocol:  protocol.DetectProtocolKind(event.RequestPath),
			Path:      event.RequestPath,
			Route:     event.Route.Name,
			Tenant:    event.Tenant,
			Consumer:  event.Consumer,
			SessionID: event.SessionID,
			RequestID: event.RequestID,
			Model:     event.Model.Name,
			Stream:    event.IsStream,
		},
		CurrentUserPrompt: protocol.CurrentUserPrompt{Text: event.UserContent},
		InjectableContext: protocol.InjectableContext{
			Text:      "remember project launch context",
			Placement: protocol.InjectionSystem,
			Messages:  []protocol.Message{{Role: "system", Content: "remember project launch context"}},
		},
		CacheDigest: protocol.CacheDigestInput{
			Protocol:              protocol.ProtocolChatCompletions,
			Model:                 event.Model.Name,
			Fields:                map[string]json.RawMessage{"messages": json.RawMessage(`[{"role":"user","content":"what did we decide?"}]`)},
			ExcludedFields:        []string{"stream"},
			ContextPolicyVersion:  event.PolicyVersion,
			InjectedContextDigest: "assembled-memory-digest-a",
		},
		Response: protocol.ResponseText{
			Text:              event.AssistantContent,
			FinishReason:      event.FinishReason,
			ContainsToolCalls: event.ContainsToolCalls,
		},
		Usage: protocol.Usage{
			InputTokens:  event.Usage.Input,
			OutputTokens: event.Usage.Output,
			TotalTokens:  event.Usage.Total,
		},
	}

	require.Equal(t, protocol.ProtocolChatCompletions, normalized.Request.Protocol)
	require.Equal(t, "7", normalized.CacheDigest.ContextPolicyVersion)
	require.Equal(t, "assembled-memory-digest-a", normalized.CacheDigest.InjectedContextDigest)
	require.Equal(t, "we decided to ship", normalized.Response.Text)
	require.Equal(t, 17, normalized.Usage.TotalTokens)
}
