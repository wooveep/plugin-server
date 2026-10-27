// Package balanceidentity carries the original wallet and API key selected by
// authentication. Clients cannot select these fields on a managed request.
package balanceidentity

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

const (
	AccountHeader    = "x-modelfusion-account-id"
	APIKeyHeader     = "x-modelfusion-api-key-id"
	GenerationHeader = "x-modelfusion-credential-generation"
	TenantHeader     = "x-modelfusion-tenant-id"
	PrincipalHeader  = "x-modelfusion-principal-id"
)

var canonicalID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Snapshot is configuration bound to the exact successfully authenticated key.
// It carries no credential and grants no permission by itself.
type Snapshot struct {
	AccountID            string `json:"account_id"`
	APIKeyID             string `json:"api_key_id"`
	CredentialGeneration string `json:"credential_generation"`
	TenantID             string `json:"tenant_id,omitempty"`
	PrincipalID          string `json:"principal_id,omitempty"`
}

// ValidID accepts the canonical, non-nil UUID representation used by native PG.
func ValidID(value string) bool {
	return canonicalID.MatchString(value) && value != "00000000-0000-0000-0000-000000000000"
}

// Validate checks the exact native identity without accepting numeric rounding.
func (s Snapshot) Validate() error {
	if !ValidID(s.AccountID) || !ValidID(s.APIKeyID) {
		return errors.New("balance identity requires canonical non-nil account and API key IDs")
	}
	value, err := strconv.ParseInt(s.CredentialGeneration, 10, 64)
	if err != nil || value < 1 || strconv.FormatInt(value, 10) != s.CredentialGeneration {
		return errors.New("balance identity requires a canonical positive credential generation")
	}
	if (s.TenantID != "" || s.PrincipalID != "") && (!ValidID(s.TenantID) || !ValidID(s.PrincipalID)) {
		return errors.New("native caller requires both original tenant and principal IDs")
	}
	return nil
}

// HeaderNames returns a fresh fixed-size list for removing untrusted inputs.
func HeaderNames() [5]string {
	return [5]string{AccountHeader, APIKeyHeader, GenerationHeader, TenantHeader, PrincipalHeader}
}

// Headers is installed only after authenticating the corresponding credential.
func (s Snapshot) Headers() [][2]string {
	headers := [][2]string{
		{AccountHeader, s.AccountID},
		{APIKeyHeader, s.APIKeyID},
		{GenerationHeader, s.CredentialGeneration},
	}
	if s.TenantID != "" && s.PrincipalID != "" {
		headers = append(headers, [2]string{TenantHeader, s.TenantID}, [2]string{PrincipalHeader, s.PrincipalID})
	}
	return headers
}

// Read accepts one of each field from the preceding managed authentication
// filter. It does not authenticate arbitrary incoming client headers.
func Read(headers [][2]string) (Snapshot, error) {
	values := make(map[string]string, 5)
	for _, pair := range headers {
		name := strings.ToLower(pair[0])
		if name != AccountHeader && name != APIKeyHeader && name != GenerationHeader && name != TenantHeader && name != PrincipalHeader {
			continue
		}
		if _, exists := values[name]; exists {
			return Snapshot{}, errors.New("duplicate balance identity header")
		}
		values[name] = pair[1]
	}
	snapshot := Snapshot{AccountID: values[AccountHeader], APIKeyID: values[APIKeyHeader], CredentialGeneration: values[GenerationHeader], TenantID: values[TenantHeader], PrincipalID: values[PrincipalHeader]}
	return snapshot, snapshot.Validate()
}
