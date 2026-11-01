// Package tenantpolicy selects frozen processing policy by the original admitted caller.
package tenantpolicy

import (
	"fmt"
	"strings"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/tidwall/gjson"
)

// Route owns common body capture, independent of any tenant's optional policy.
type Route struct {
	ReleaseID string
	TargetID  string
	RouteID   string
}

// Directory contains complete, independently parsed tenant policies with no fallback.
type Directory[T any] struct {
	Route            Route
	Policies         map[string]T
	CaptureBodyBasis bool
	Protocol         protocol.ProtocolKind
	PublicModel      string
}

// Parse validates the directory once and delegates each policy to its original parser.
func Parse[T any](input gjson.Result, parse func(gjson.Result) (T, error)) (*Directory[T], error) {
	policies := input.Get("tenant_policies")
	if !policies.Exists() {
		return nil, nil
	}
	if _, err := mfjson.Canonicalize([]byte(input.Raw)); err != nil {
		return nil, fmt.Errorf("tenant policy directory: %w", err)
	}
	if !policies.IsObject() || len(policies.Map()) == 0 || input.Get("path_scope").String() != "bound_target" || input.Get("native_identity").Exists() {
		return nil, fmt.Errorf("tenant_policies requires a nonempty directory on one bound target")
	}
	route := input.Get("native_route")
	identity := Route{ReleaseID: route.Get("release_id").String(), TargetID: route.Get("target_id").String(), RouteID: route.Get("route_id").String()}
	for _, value := range []string{identity.ReleaseID, identity.TargetID} {
		if strings.TrimSpace(value) == "" || len(value) > 256 {
			return nil, fmt.Errorf("native_route requires its original release and target")
		}
	}
	if !balanceidentity.ValidID(identity.RouteID) {
		return nil, fmt.Errorf("native_route requires a canonical route UUID")
	}
	capture := input.Get("capture_body_basis")
	if capture.Type != gjson.True && capture.Type != gjson.False {
		return nil, fmt.Errorf("tenant policy directory requires explicit body capture ownership")
	}
	kind, err := protocol.ParsePublishedTextProtocol(input.Get("client_protocol").String(), input.Get("operation_kind").String())
	if err != nil {
		return nil, fmt.Errorf("tenant policy directory protocol: %w", err)
	}
	model := input.Get("public_model").String()
	if err := bodybasis.ValidateModel(model); err != nil {
		return nil, fmt.Errorf("tenant policy directory public model: %w", err)
	}
	result := &Directory[T]{Route: identity, Policies: make(map[string]T), CaptureBodyBasis: capture.Bool(), Protocol: kind, PublicModel: model}
	for tenant, policy := range policies.Map() {
		if !balanceidentity.ValidID(tenant) || !policy.IsObject() || policy.Get("tenant_policies").Exists() || policy.Get("native_route").Exists() {
			return nil, fmt.Errorf("tenant policy directory contains an invalid tenant or nested directory")
		}
		original := policy.Get("native_identity")
		if original.Get("release_id").String() != identity.ReleaseID || original.Get("target_id").String() != identity.TargetID || original.Get("route_id").String() != identity.RouteID || policy.Get("capture_body_basis").Bool() != capture.Bool() {
			return nil, fmt.Errorf("tenant policy differs from its original route or body capture ownership")
		}
		for _, field := range []string{"client_protocol", "operation_kind"} {
			if policy.Get(field).String() != input.Get(field).String() {
				return nil, fmt.Errorf("tenant policy differs from its original published protocol")
			}
		}
		parsed, err := parse(policy)
		if err != nil {
			return nil, fmt.Errorf("tenant policy %s: %w", tenant, err)
		}
		result.Policies[tenant] = parsed
	}
	return result, nil
}

// Select reads only the authentication and quota facts installed before processing.
// A missing policy never borrows another tenant's configuration.
func (d *Directory[T]) Select(headers [][2]string) (T, bool) {
	var missing T
	caller, err := balanceidentity.Read(headers)
	if err != nil || caller.TenantID == "" || caller.PrincipalID == "" {
		return missing, false
	}
	if _, err := executionfacts.ReadAdmittedCall(headers, d.Route.ReleaseID, time.Now().UTC()); err != nil {
		return missing, false
	}
	policy, found := d.Policies[caller.TenantID]
	return policy, found
}
