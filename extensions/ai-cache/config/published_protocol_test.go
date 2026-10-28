package config

import (
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPublishedCacheProtocolConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		kind protocol.ProtocolKind
	}{
		{"chat", `{"client_protocol":"openai_chat","operation_kind":"text_generate"}`, protocol.ProtocolChatCompletions},
		{"responses", `{"client_protocol":"openai_responses","operation_kind":"text_generate"}`, protocol.ProtocolResponses},
		{"messages", `{"client_protocol":"anthropic_messages","operation_kind":"text_generate"}`, protocol.ProtocolMessages},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cfg PluginConfig
			cfg.FromJson(gjson.Parse(tc.raw), noopLogger{})
			require.NoError(t, cfg.protocolConfigErr)
			require.True(t, cfg.PublishedProtocolConfigured)
			require.Equal(t, tc.kind, cfg.ProtocolForPath("/custom/action"))
			require.Equal(t, tc.kind, cfg.ProtocolForPath("/v1/messages"))
			require.Empty(t, cfg.ProtocolBypassReason)
			require.False(t, cfg.HasThinConfig(), "protocol metadata must not enable cache")
		})
	}

	for _, raw := range []string{
		`{"client_protocol":"openai_chat"}`,
		`{"operation_kind":"text_generate"}`,
		`{"client_protocol":null,"operation_kind":"text_generate"}`,
		`{"client_protocol":17,"operation_kind":"text_generate"}`,
		`{"client_protocol":"openai_chat","operation_kind":[]}`,
		`{"client_protocol":"","operation_kind":"text_generate"}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var cfg PluginConfig
			cfg.FromJson(gjson.Parse(raw), noopLogger{})
			require.ErrorContains(t, cfg.Validate(), "supplied together as non-empty strings")
		})
	}

	for _, raw := range []string{
		`{"client_protocol":"openai_chat","operation_kind":"image_generate"}`,
		`{"client_protocol":"openai_chat","operation_kind":"query"}`,
		`{"client_protocol":"unknown","operation_kind":"text_generate"}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var cfg PluginConfig
			cfg.FromJson(gjson.Parse(raw), noopLogger{})
			require.NoError(t, cfg.protocolConfigErr)
			require.NotEmpty(t, cfg.ProtocolBypassReason)
			require.Equal(t, protocol.ProtocolUnknown, cfg.ProtocolForPath("/v1/chat/completions"))
		})
	}
}

func TestPublishedCacheProtocolOverridesAreAtomic(t *testing.T) {
	var global PluginConfig
	global.FromJson(gjson.Parse(`{"client_protocol":"openai_chat","operation_kind":"text_generate"}`), noopLogger{})
	var route PluginConfig
	route.FromJsonWithGlobal(gjson.Parse(`{"cache_policy_version":"route-v1"}`), global, noopLogger{})
	require.Equal(t, protocol.ProtocolChatCompletions, route.ProtocolForPath("/custom"))

	route.FromJsonWithGlobal(gjson.Parse(`{"client_protocol":"openai_responses","operation_kind":"text_generate"}`), global, noopLogger{})
	require.NoError(t, route.protocolConfigErr)
	require.Equal(t, protocol.ProtocolResponses, route.ProtocolForPath("/v1/chat/completions"))
	route.FromJsonWithGlobal(gjson.Parse(`{"client_protocol":"openai_responses"}`), global, noopLogger{})
	require.ErrorContains(t, route.Validate(), "supplied together as non-empty strings")

	var legacy PluginConfig
	legacy.FromJson(gjson.Parse(`{}`), noopLogger{})
	require.False(t, legacy.PublishedProtocolConfigured)
	require.Equal(t, protocol.ProtocolResponses, legacy.ProtocolForPath("/v1/responses"))
	require.Equal(t, protocol.ProtocolUnknown, legacy.ProtocolForPath("/custom"))
}
