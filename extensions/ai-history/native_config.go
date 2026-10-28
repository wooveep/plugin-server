package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/tidwall/gjson"
)

// These values are frozen by the publisher, never inferred from a URL, header,
// model prefix, or another Wasm VM's private context.
type historyNativeConfig struct {
	ReleaseID     string `json:"release_id"`
	TargetID      string `json:"target_id"`
	RouteID       string `json:"route_id"`
	PolicyID      string `json:"policy_id"`
	PolicyVersion int64  `json:"policy_version"`
}

type historyBinding struct {
	Identity         historyNativeConfig
	Protocol         protocol.ProtocolKind
	PublicModel      string
	Bypass           string
	CaptureBodyBasis bool
}

func parseHistoryBinding(input gjson.Result, c *PluginConfig) error {
	c.historyEnabled = true
	c.historyBinding = nil
	if enabled := input.Get("enabled"); enabled.Exists() {
		if enabled.Type != gjson.True && enabled.Type != gjson.False {
			return fmt.Errorf("enabled must be a boolean")
		}
		c.historyEnabled = enabled.Bool()
	}
	declared := input.Get("native_identity").Exists() || input.Get("client_protocol").Exists() ||
		input.Get("operation_kind").Exists() || input.Get("path_scope").Exists() || input.Get("public_model").Exists()
	capture := input.Get("capture_body_basis")
	if capture.Exists() && capture.Type != gjson.True && capture.Type != gjson.False {
		return fmt.Errorf("capture_body_basis must be a boolean")
	}
	if capture.Bool() && !declared {
		return fmt.Errorf("capture_body_basis requires a complete managed history binding")
	}
	if !declared {
		return nil
	}
	for _, field := range []string{"path_scope", "client_protocol", "operation_kind", "public_model"} {
		value := input.Get(field)
		if value.Type != gjson.String || strings.TrimSpace(value.String()) == "" {
			return fmt.Errorf("managed history requires nonempty %s", field)
		}
	}
	if input.Get("path_scope").String() != "bound_target" {
		return fmt.Errorf("managed history path_scope must be bound_target")
	}
	identity := input.Get("native_identity")
	if !identity.IsObject() {
		return fmt.Errorf("managed history requires native_identity")
	}
	var binding historyBinding
	binding.CaptureBodyBasis = capture.Bool()
	decoder := json.NewDecoder(strings.NewReader(identity.Raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&binding.Identity); err != nil {
		return fmt.Errorf("native_identity: %w", err)
	}
	for _, value := range []string{binding.Identity.ReleaseID, binding.Identity.TargetID} {
		if !utf8.ValidString(value) || strings.TrimSpace(value) == "" || len(value) > 256 {
			return fmt.Errorf("native release_id and target_id must be nonempty strings of at most 256 UTF-8 bytes")
		}
	}
	if !balanceidentity.ValidID(binding.Identity.RouteID) || !balanceidentity.ValidID(binding.Identity.PolicyID) {
		return fmt.Errorf("native route_id and policy_id must be canonical non-nil UUIDs")
	}
	if binding.Identity.PolicyVersion <= 0 {
		return fmt.Errorf("native policy_version must be a positive int64 integer")
	}
	binding.PublicModel = input.Get("public_model").String()
	if err := bodybasis.ValidateModel(binding.PublicModel); err != nil {
		return fmt.Errorf("public_model: %w", err)
	}
	kind, err := protocol.ParsePublishedTextProtocol(input.Get("client_protocol").String(), input.Get("operation_kind").String())
	binding.Protocol = kind
	if err != nil {
		if binding.CaptureBodyBasis {
			return fmt.Errorf("capture_body_basis requires a supported published text protocol")
		}
		binding.Bypass = "unsupported-published-operation-or-protocol"
	}
	c.historyBinding = &binding
	return nil
}
