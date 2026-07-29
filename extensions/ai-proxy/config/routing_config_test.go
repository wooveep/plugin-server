package config

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

type protocolModeReader interface {
	GetProtocolMode() string
}

type pathScopeReader interface {
	GetPathScope() string
}

func TestPluginConfigRoutingParserContract(t *testing.T) {
	const legacyProvider = `"provider":{
		"type":"generic",
		"genericHost":"http://127.0.0.1:8080",
		"apiTokens":["token"]
	}`

	t.Run("missing_protocol_mode_defaults_to_auto", func(t *testing.T) {
		var pluginConfig PluginConfig
		pluginConfig.FromJson(gjson.Parse(`{` + legacyProvider + `}`))
		if err := pluginConfig.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}

		reader, ok := any(&pluginConfig).(protocolModeReader)
		if !ok {
			t.Fatal("*PluginConfig does not implement GetProtocolMode")
		}
		if got := reader.GetProtocolMode(); got != "auto" {
			t.Fatalf("GetProtocolMode() = %q, want %q", got, "auto")
		}
		if pluginConfig.GetProviderConfig() == nil {
			t.Fatal("legacy provider selection was not retained")
		}
	})

	t.Run("passthrough_is_accepted", func(t *testing.T) {
		var pluginConfig PluginConfig
		pluginConfig.FromJson(gjson.Parse(`{` + legacyProvider + `,"protocol_mode":"passthrough"}`))
		if err := pluginConfig.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}

		reader, ok := any(&pluginConfig).(protocolModeReader)
		if !ok {
			t.Fatal("*PluginConfig does not implement GetProtocolMode")
		}
		if got := reader.GetProtocolMode(); got != "passthrough" {
			t.Fatalf("GetProtocolMode() = %q, want %q", got, "passthrough")
		}
	})

	t.Run("omitted_override_inherits_global_protocol_mode", func(t *testing.T) {
		var global PluginConfig
		global.FromJson(gjson.Parse(`{` + legacyProvider + `,"protocol_mode":"passthrough"}`))
		if err := global.Validate(); err != nil {
			t.Fatalf("global Validate() = %v, want nil", err)
		}

		rule := global
		rule.FromJson(gjson.Parse(`{}`))
		if err := rule.Validate(); err != nil {
			t.Fatalf("rule Validate() = %v, want nil", err)
		}

		reader, ok := any(&rule).(protocolModeReader)
		if !ok {
			t.Fatal("*PluginConfig does not implement GetProtocolMode")
		}
		if got := reader.GetProtocolMode(); got != "passthrough" {
			t.Fatalf("inherited GetProtocolMode() = %q, want %q", got, "passthrough")
		}
	})

	t.Run("invalid_protocol_mode_is_rejected", func(t *testing.T) {
		var pluginConfig PluginConfig
		pluginConfig.FromJson(gjson.Parse(`{` + legacyProvider + `,"protocol_mode":"invalid"}`))

		err := pluginConfig.Validate()
		if err == nil {
			t.Fatal("Validate() error = nil, want protocol_mode rejection")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "protocol_mode") {
			t.Fatalf("Validate() error = %q, want protocol_mode context", err)
		}
		if len(err.Error()) > 256 {
			t.Fatalf("Validate() error length = %d, want at most 256", len(err.Error()))
		}
	})

	t.Run("missing_path_scope_preserves_legacy_suffixes", func(t *testing.T) {
		var pluginConfig PluginConfig
		pluginConfig.FromJson(gjson.Parse(`{` + legacyProvider + `}`))
		if err := pluginConfig.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}

		reader, ok := any(&pluginConfig).(pathScopeReader)
		if !ok {
			t.Fatal("*PluginConfig does not implement GetPathScope")
		}
		if got := reader.GetPathScope(); got != "legacy_suffixes" {
			t.Fatalf("GetPathScope() = %q, want %q", got, "legacy_suffixes")
		}
		if pluginConfig.GetProviderConfig() == nil {
			t.Fatal("legacy provider selection was not retained")
		}
	})
}
