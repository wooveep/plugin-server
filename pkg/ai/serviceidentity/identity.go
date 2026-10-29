// Package serviceidentity carries accountless identity installed by key-auth.
// Request headers are consistency checks, never the identity authority.
package serviceidentity

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
)

// Property is a request FilterState value written by the authentication filter.
// Envoy's Wasm host shares it across filters, not across client requests.
const Property = "modelfusion.key_auth.internal_service.v1"
const ServiceHeader = "x-modelfusion-internal-service"
const maxStateBytes = 1024 // Five bounded original identity fields, without a credential.

// Snapshot is the exact successfully authenticated service Key; it grants no
// route, cost-owner, operation or send permission and has no wallet identity.
type Snapshot struct {
	Service              string `json:"service"`
	APIKeyID             string `json:"api_key_id"`
	CredentialGeneration string `json:"credential_generation"`
	TenantID             string `json:"tenant_id"`
	PrincipalID          string `json:"principal_id"`
}

func (s Snapshot) Validate() error {
	if s.Service != "ai-memory" && s.Service != "ai-cache" {
		return errors.New("native internal identity requires a registered service")
	}
	if !balanceidentity.ValidID(s.APIKeyID) || !balanceidentity.ValidID(s.TenantID) || !balanceidentity.ValidID(s.PrincipalID) {
		return errors.New("native internal identity requires its original Key, tenant and principal")
	}
	generation, err := strconv.ParseInt(s.CredentialGeneration, 10, 64)
	if err != nil || generation < 1 || strconv.FormatInt(generation, 10) != s.CredentialGeneration {
		return errors.New("native internal identity requires its canonical positive Key generation")
	}
	return nil
}

func (s Snapshot) QualifiedPrincipal() string { return "service/" + s.Service + "/" + s.PrincipalID }

func (s Snapshot) Headers() [][2]string {
	return [][2]string{{ServiceHeader, s.Service}, {balanceidentity.APIKeyHeader, s.APIKeyID},
		{balanceidentity.GenerationHeader, s.CredentialGeneration}, {balanceidentity.TenantHeader, s.TenantID},
		{balanceidentity.PrincipalHeader, s.PrincipalID}}
}

// Clear replaces any previous authentication result with an invalid snapshot.
// The SDK requires nonempty data; Read rejects this object, and a later
// successful Publish can overwrite the same mutable request FilterState value.
func Clear() error { return proxywasm.SetProperty([]string{Property}, []byte("{}")) }

// Publish is called only by key-auth after the actual credential and allow rule
// succeed. Neither an inbound header nor an internal call reference creates it.
func Publish(s Snapshot) error {
	if err := s.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(s)
	if err != nil || len(raw) > maxStateBytes {
		return errors.New("native internal identity state is invalid")
	}
	return proxywasm.SetProperty([]string{Property}, raw)
}

// Read requires original typed FilterState and exactly matching authentication
// headers. A customer account or a header-only service assertion is rejected.
func Read(headers [][2]string) (Snapshot, error) {
	raw, err := proxywasm.GetProperty([]string{Property})
	if err != nil || len(raw) == 0 || len(raw) > maxStateBytes {
		return Snapshot{}, errors.New("authenticated native service state is unavailable")
	}
	exact, err := mfjson.Canonicalize(raw)
	if err != nil {
		return Snapshot{}, errors.New("authenticated native service state is invalid")
	}
	var snapshot Snapshot
	decoder := json.NewDecoder(bytes.NewReader(exact))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&snapshot) != nil || snapshot.Validate() != nil {
		return Snapshot{}, errors.New("authenticated native service state is invalid")
	}
	expected := map[string]string{}
	for _, pair := range snapshot.Headers() {
		expected[pair[0]] = pair[1]
	}
	seen := map[string]bool{}
	for _, pair := range headers {
		name := strings.ToLower(pair[0])
		if name == balanceidentity.AccountHeader {
			return Snapshot{}, errors.New("internal service identity cannot carry a customer account")
		}
		want, present := expected[name]
		if !present {
			continue
		}
		if seen[name] || pair[1] != want {
			return Snapshot{}, errors.New("request identity differs from authenticated native service state")
		}
		seen[name] = true
	}
	if len(seen) != len(expected) {
		return Snapshot{}, errors.New("authenticated native service headers are incomplete")
	}
	return snapshot, nil
}
