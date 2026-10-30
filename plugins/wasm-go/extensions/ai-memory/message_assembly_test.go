package main

import (
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/stretchr/testify/require"
)

func TestMemoryFinalMessageAssemblyOrder(t *testing.T) {
	memoryMessage := sessionctx.OpenAIMessage{Role: "system", Content: "Console memory context"}
	messages := assembleFinalMemoryMessages(memoryMessageAssemblyInput{
		Current: []sessionctx.OpenAIMessage{
			{Role: "system", Content: "client system"},
			{Role: "developer", Content: "client developer"},
			{Role: "user", Content: "current question"},
		},
		MemoryMessage: &memoryMessage,
		Recent: []sessionctx.OpenAIMessage{
			{Role: "user", Content: "recent user"},
			{Role: "assistant", Content: "recent assistant"},
		},
	})

	requireOpenAIMessages(t, messages, []memoryExpectedMessage{
		{role: "system", content: "client system"},
		{role: "developer", content: "client developer"},
		{role: "system", content: "Console memory context"},
		{role: "user", content: "recent user"},
		{role: "assistant", content: "recent assistant"},
		{role: "user", content: "current question"},
	})
}

func TestMemoryFinalMessageAssemblySkipsRecentForMultiUserCurrentRequest(t *testing.T) {
	memoryMessage := sessionctx.OpenAIMessage{Role: "system", Content: "Console memory context"}
	messages := assembleFinalMemoryMessages(memoryMessageAssemblyInput{
		Current: []sessionctx.OpenAIMessage{
			{Role: "system", Content: "client system"},
			{Role: "user", Content: "opening question"},
			{Role: "assistant", Content: "opening answer"},
			{Role: "user", Content: "current question"},
		},
		MemoryMessage: &memoryMessage,
		Recent: []sessionctx.OpenAIMessage{
			{Role: "user", Content: "recent user"},
			{Role: "assistant", Content: "recent assistant"},
		},
	})

	requireOpenAIMessages(t, messages, []memoryExpectedMessage{
		{role: "system", content: "client system"},
		{role: "system", content: "Console memory context"},
		{role: "user", content: "opening question"},
		{role: "assistant", content: "opening answer"},
		{role: "user", content: "current question"},
	})
}

func TestMemoryAssemblyInputFromResponseDecision(t *testing.T) {
	memoryMessage := &memoryAssembleMessage{Role: "system", Content: "Console memory context"}
	recentMessages := []memoryAssembleMessage{
		{Role: "user", Content: "recent user"},
		{Role: "assistant", Content: "recent assistant"},
	}

	tests := []struct {
		name       string
		response   memoryConsoleAssembleResponse
		wantMemory bool
		wantRecent int
	}{
		{
			name: "inject includes memory and recent",
			response: memoryConsoleAssembleResponse{
				Decision:       memoryAssembleDecisionInject,
				MemoryMessage:  memoryMessage,
				RecentMessages: recentMessages,
			},
			wantMemory: true,
			wantRecent: len(recentMessages),
		},
		{
			name: "recent only excludes memory",
			response: memoryConsoleAssembleResponse{
				Decision:       memoryAssembleDecisionRecentOnly,
				MemoryMessage:  memoryMessage,
				RecentMessages: recentMessages,
			},
			wantRecent: len(recentMessages),
		},
		{
			name:     "skip excludes memory and recent",
			response: memoryConsoleAssembleResponse{Decision: memoryAssembleDecisionSkip},
		},
		{
			name:     "bypass excludes memory and recent",
			response: memoryConsoleAssembleResponse{Decision: memoryAssembleDecisionBypass},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := memoryAssemblyInputFromResponse(tt.response, config.InjectRoleSystem)

			require.Equal(t, tt.wantMemory, input.MemoryMessage != nil)
			require.Len(t, input.Recent, tt.wantRecent)
		})
	}
}

func TestMemoryAssemblyInputFromResponseUsesConfiguredInjectRole(t *testing.T) {
	tests := []struct {
		name        string
		consoleRole string
		injectRole  string
		wantRole    string
	}{
		{
			name:        "system route overrides Console developer message",
			consoleRole: config.InjectRoleDeveloper,
			injectRole:  config.InjectRoleSystem,
			wantRole:    config.InjectRoleSystem,
		},
		{
			name:        "developer route upgrades Console system message",
			consoleRole: config.InjectRoleSystem,
			injectRole:  config.InjectRoleDeveloper,
			wantRole:    config.InjectRoleDeveloper,
		},
		{
			name:        "empty route role defaults to system",
			consoleRole: config.InjectRoleDeveloper,
			wantRole:    config.InjectRoleSystem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := memoryAssemblyInputFromResponse(memoryConsoleAssembleResponse{
				Decision: memoryAssembleDecisionInject,
				MemoryMessage: &memoryAssembleMessage{
					Role:    tt.consoleRole,
					Content: "Console memory context",
				},
			}, tt.injectRole)

			require.NotNil(t, input.MemoryMessage)
			require.Equal(t, tt.wantRole, input.MemoryMessage.Role)
			require.Equal(t, "Console memory context", input.MemoryMessage.Content)
		})
	}
}

func requireOpenAIMessages(t *testing.T, messages []sessionctx.OpenAIMessage, expected []memoryExpectedMessage) {
	t.Helper()
	require.Len(t, messages, len(expected))
	for i, want := range expected {
		require.Equal(t, want.role, messages[i].Role)
		require.Equal(t, want.content, messages[i].Content)
	}
}
