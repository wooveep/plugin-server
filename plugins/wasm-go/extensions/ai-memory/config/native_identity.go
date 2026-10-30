package config

import (
	"fmt"
	"strconv"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/tidwall/gjson"
)

// NativeIdentityConfig binds memory to its original publication and policy.
// The authenticated caller is captured per request and is never configuration.
type NativeIdentityConfig struct {
	ReleaseID     string `json:"release_id"`
	TargetID      string `json:"target_id"`
	RouteID       string `json:"route_id"`
	PolicyID      string `json:"policy_id"`
	PolicyVersion int64  `json:"policy_version"`
}

func parseNativeIdentity(raw gjson.Result) (*NativeIdentityConfig, error) {
	if !raw.IsObject() {
		return nil, fmt.Errorf("native_identity must be an object")
	}
	if _, err := mfjson.Canonicalize([]byte(raw.Raw)); err != nil {
		return nil, fmt.Errorf("invalid native_identity JSON: %w", err)
	}
	fields := raw.Map()
	if len(fields) != 5 {
		return nil, fmt.Errorf("native_identity requires exactly release_id, target_id, route_id, policy_id, and policy_version")
	}
	for _, name := range []string{"release_id", "target_id", "route_id", "policy_id"} {
		if fields[name].Type != gjson.String {
			return nil, fmt.Errorf("native_identity.%s must be a string", name)
		}
	}
	version, err := strconv.ParseInt(fields["policy_version"].Raw, 10, 64)
	if fields["policy_version"].Type != gjson.Number || err != nil || version < 1 {
		return nil, fmt.Errorf("native_identity.policy_version must be a positive i64 JSON integer")
	}
	identity := NativeIdentityConfig{
		ReleaseID: fields["release_id"].Str, TargetID: fields["target_id"].Str,
		RouteID: fields["route_id"].Str, PolicyID: fields["policy_id"].Str,
		PolicyVersion: version,
	}
	if len(identity.ReleaseID) == 0 || len(identity.ReleaseID) > 256 || len(identity.TargetID) == 0 || len(identity.TargetID) > 256 {
		return nil, fmt.Errorf("native_identity release_id and target_id must contain 1 to 256 UTF-8 bytes")
	}
	if !balanceidentity.ValidID(identity.RouteID) || !balanceidentity.ValidID(identity.PolicyID) {
		return nil, fmt.Errorf("native_identity route_id and policy_id must be canonical non-nil UUIDs")
	}
	return &identity, nil
}
