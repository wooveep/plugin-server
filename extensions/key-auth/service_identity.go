package main

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/serviceidentity"
	"github.com/tidwall/gjson"
)

type internalServiceCredentialBinding struct {
	Credential string `json:"credential"`
	serviceidentity.Snapshot
}

// Customer wallet material and accountless services are explicit alternatives.
// A service consumer still covers every actual configured credential exactly.
func parseServiceCredentialBindings(item gjson.Result, credentials []string) (map[string]serviceidentity.Snapshot, error) {
	value := item.Get("internal_service_bindings")
	if !value.Exists() {
		return nil, nil
	}
	if _, err := mfjson.Canonicalize([]byte(item.Raw)); err != nil {
		return nil, errors.New("invalid native service consumer JSON")
	}
	if item.Get("credential_bindings").Exists() || !value.IsArray() || len(value.Array()) == 0 || item.Get("tenant").String() == "" {
		return nil, errors.New("internal_service_bindings requires a retained tenant and excludes customer wallet bindings")
	}
	bindings := make(map[string]serviceidentity.Snapshot, len(credentials))
	for _, entry := range value.Array() {
		exact, err := mfjson.Canonicalize([]byte(entry.Raw))
		if err != nil {
			return nil, errors.New("invalid native service binding JSON")
		}
		var binding internalServiceCredentialBinding
		decoder := json.NewDecoder(strings.NewReader(string(exact)))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&binding) != nil || binding.Snapshot.Validate() != nil {
			return nil, errors.New("invalid accountless native service binding")
		}
		if !contains(credentials, binding.Credential) {
			return nil, errors.New("native service binding must reference an actual configured credential")
		}
		if _, exists := bindings[binding.Credential]; exists {
			return nil, errors.New("duplicate native service credential binding")
		}
		bindings[binding.Credential] = binding.Snapshot
	}
	if len(bindings) != len(credentials) {
		return nil, errors.New("native service bindings must cover every configured credential")
	}
	return bindings, nil
}

func installServiceIdentityHeaders(identity *serviceidentity.Snapshot) error {
	if err := removeBalanceIdentityHeaders(); err != nil {
		return err
	}
	if identity == nil || identity.Validate() != nil {
		return errors.New("native service identity is invalid")
	}
	for _, pair := range identity.Headers() {
		if err := proxywasm.AddHttpRequestHeader(pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}

// Publishing follows the existing allow decision. A denied credential never
// installs trusted service state, even if its configured headers were resolved.
func authenticatedIdentity(identity credentialIdentity, realm string) types.Action {
	if identity.Service != nil {
		if identity.Balance != nil || serviceidentity.Publish(*identity.Service) != nil {
			return deniedUnauthorizedConsumer(realm)
		}
	}
	return authenticated(identity.Name)
}
