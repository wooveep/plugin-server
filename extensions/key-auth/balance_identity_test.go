package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

var firstBalanceIdentity = balanceidentity.Snapshot{AccountID: "00000000-0000-0000-0000-000000000002", APIKeyID: "00000000-0000-0000-0000-000000000003", CredentialGeneration: "9007199254740993"}
var secondBalanceIdentity = balanceidentity.Snapshot{AccountID: firstBalanceIdentity.AccountID, APIKeyID: "00000000-0000-0000-0000-000000000004", CredentialGeneration: firstBalanceIdentity.CredentialGeneration}

func nativeKeyAuthFixture(bindings bool) json.RawMessage {
	consumer := map[string]any{"name": "retained-owner", "tenant": "retained-tenant", "credentials": []string{"synthetic-key-one", "synthetic-key-two"}}
	if bindings {
		consumer["credential_bindings"] = []balanceCredentialBinding{
			{Credential: "synthetic-key-one", Snapshot: firstBalanceIdentity},
			{Credential: "synthetic-key-two", Snapshot: secondBalanceIdentity},
		}
	}
	raw, _ := json.Marshal(map[string]any{"global_auth": true, "keys": []string{"Authorization"}, "in_header": true, "consumers": []any{consumer}})
	return raw
}

func TestAuthenticatedCredentialCarriesItsOwnOriginalBalanceIdentity(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			key      string
			identity balanceidentity.Snapshot
		}{
			{"synthetic-key-one", firstBalanceIdentity},
			{"synthetic-key-two", secondBalanceIdentity},
		} {
			t.Run(tc.key, func(t *testing.T) {
				host, status := test.NewTestHost(nativeKeyAuthFixture(true))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				headers := append([][2]string{{":authority", "example.test"}, {":path", "/v1/chat/completions"}, {":method", "POST"}, {"Authorization", "Bearer " + tc.key}}, secondBalanceIdentity.Headers()...)
				headers = append(headers, [2]string{balanceidentity.AccountHeader, "client-selected-account"})
				action := host.CallOnHttpRequestHeaders(headers)
				require.Equal(t, types.ActionContinue, action)
				require.Nil(t, host.GetLocalResponse())
				actual, err := balanceidentity.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, tc.identity, actual)
				host.CompleteHttp()
			})
		}
	})
}

func TestNoAuthenticationShortcutCanRetainClientBalanceIdentity(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		public, _ := json.Marshal(map[string]any{"global_auth": false, "keys": []string{"Authorization"}, "in_header": true, "consumers": []any{map[string]any{"name": "retained-owner", "credential": "synthetic-key-one"}}})
		authOnly, _ := json.Marshal(map[string]any{"global_auth": true, "keys": []string{"Authorization"}, "in_header": true, "credentials": []string{"synthetic-key-one"}})
		for name, config := range map[string]json.RawMessage{"public rule": public, "auth only": authOnly, "legacy consumer": nativeKeyAuthFixture(false)} {
			t.Run(name, func(t *testing.T) {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				headers := append([][2]string{{":authority", "example.test"}, {":path", "/v1/chat/completions"}, {":method", "POST"}, {"Authorization", "Bearer synthetic-key-one"}}, firstBalanceIdentity.Headers()...)
				host.CallOnHttpRequestHeaders(headers)
				_, err := balanceidentity.Read(host.GetRequestHeaders())
				require.Error(t, err)
				host.CompleteHttp()
			})
		}
	})
}

func TestNativeBalanceBindingsRejectUnknownIncompleteAndAmbiguousCredentials(t *testing.T) {
	var valid map[string]any
	require.NoError(t, json.Unmarshal(nativeKeyAuthFixture(true), &valid))
	consumer := valid["consumers"].([]any)[0].(map[string]any)
	bindings := consumer["credential_bindings"].([]any)
	for name, change := range map[string]func(map[string]any){
		"unknown credential":      func(row map[string]any) { row["credential"] = "unconfigured-key" },
		"missing account":         func(row map[string]any) { delete(row, "account_id") },
		"wrong key ID":            func(row map[string]any) { row["api_key_id"] = "client-alias" },
		"numeric generation":      func(row map[string]any) { row["credential_generation"] = 1 },
		"noncanonical generation": func(row map[string]any) { row["credential_generation"] = "01" },
		"unknown field":           func(row map[string]any) { row["enabled"] = true },
	} {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(bindings[0])
			var row map[string]any
			require.NoError(t, json.Unmarshal(raw, &row))
			change(row)
			data, _ := json.Marshal(map[string]any{"tenant": "retained-tenant", "credential_bindings": []any{row, bindings[1]}})
			_, err := parseBalanceCredentialBindings(gjson.ParseBytes(data), []string{"synthetic-key-one", "synthetic-key-two"})
			require.Error(t, err)
		})
	}
	for name, rows := range map[string][]any{"incomplete": {bindings[0]}, "duplicate": {bindings[0], bindings[0]}} {
		t.Run(name, func(t *testing.T) {
			data, _ := json.Marshal(map[string]any{"tenant": "retained-tenant", "credential_bindings": rows})
			_, err := parseBalanceCredentialBindings(gjson.ParseBytes(data), []string{"synthetic-key-one", "synthetic-key-two"})
			require.Error(t, err)
		})
	}
}

func TestUnconfiguredCredentialDoesNotInstallAnyBalanceIdentity(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(nativeKeyAuthFixture(true))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		headers := append([][2]string{{":authority", "example.test"}, {":path", "/v1/chat/completions"}, {":method", "POST"}, {"Authorization", "Bearer unknown-synthetic-key"}}, firstBalanceIdentity.Headers()...)
		host.CallOnHttpRequestHeaders(headers)
		require.Equal(t, uint32(http.StatusUnauthorized), host.GetLocalResponse().StatusCode)
		_, err := balanceidentity.Read(host.GetRequestHeaders())
		require.Error(t, err)
		host.CompleteHttp()
	})
}
