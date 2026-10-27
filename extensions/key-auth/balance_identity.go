package main

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/tidwall/gjson"
)

type balanceCredentialBinding struct {
	Credential string `json:"credential"`
	balanceidentity.Snapshot
}

// Native metadata is complete for this consumer's actual configured credentials;
// it is never selected by a client-supplied wallet, alias, or key ID.
func parseBalanceCredentialBindings(item gjson.Result, credentials []string) (map[string]balanceidentity.Snapshot, error) {
	value := item.Get("credential_bindings")
	if !value.Exists() {
		return nil, nil
	}
	if !value.IsArray() || item.Get("tenant").String() == "" {
		return nil, errors.New("credential_bindings requires an array and a retained consumer tenant")
	}
	bindings := make(map[string]balanceidentity.Snapshot, len(credentials))
	for _, value := range value.Array() {
		var binding balanceCredentialBinding
		decoder := json.NewDecoder(strings.NewReader(value.Raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&binding); err != nil {
			return nil, errors.New("invalid native credential binding fields")
		}
		if !contains(credentials, binding.Credential) {
			return nil, errors.New("native binding must reference an actual configured credential")
		}
		if _, exists := bindings[binding.Credential]; exists {
			return nil, errors.New("duplicate native credential binding")
		}
		if err := binding.Snapshot.Validate(); err != nil {
			return nil, err
		}
		bindings[binding.Credential] = binding.Snapshot
	}
	if len(bindings) != len(credentials) {
		return nil, errors.New("native bindings must cover every configured credential")
	}
	return bindings, nil
}

func removeBalanceIdentityHeaders() error {
	for _, name := range balanceidentity.HeaderNames() {
		if err := proxywasm.RemoveHttpRequestHeader(name); err != nil && !errors.Is(err, types.ErrorStatusNotFound) {
			return err
		}
	}
	return nil
}

func installBalanceIdentity(identity *balanceidentity.Snapshot) error {
	if err := removeBalanceIdentityHeaders(); err != nil {
		return err
	}
	if identity == nil {
		return nil
	}
	for _, pair := range identity.Headers() {
		if err := proxywasm.AddHttpRequestHeader(pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}
