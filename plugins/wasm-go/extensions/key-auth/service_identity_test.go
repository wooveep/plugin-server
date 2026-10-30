package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/serviceidentity"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func internalServiceIdentity() serviceidentity.Snapshot {
	return serviceidentity.Snapshot{Service: "ai-memory", APIKeyID: "20000000-0000-4000-8000-000000000009", CredentialGeneration: "9007199254740993", TenantID: "30000000-0000-4000-8000-000000000009", PrincipalID: "40000000-0000-4000-8000-000000000009"}
}
func internalServiceConfig() map[string]interface{} {
	return map[string]interface{}{"global_auth": true, "keys": []string{"Authorization"}, "in_header": true, "consumers": []interface{}{map[string]interface{}{"name": "real-service", "tenant": "retained-tenant", "credential": "synthetic-service-key", "internal_service_bindings": []internalServiceCredentialBinding{{Credential: "synthetic-service-key", Snapshot: internalServiceIdentity()}}}}}
}

func TestAccountlessServiceAuthenticationInstallsTypedStateAndNeverCustomerWallet(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		raw, _ := json.Marshal(internalServiceConfig())
		host, status := test.NewTestHost(raw)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		headers := append([][2]string{{":method", "POST"}, {":path", "/internal-route"}, {":authority", "gateway.example"}, {"Authorization", "Bearer synthetic-service-key"}, {executionfacts.InternalCallReferenceHeader, "50000000-0000-4000-8000-000000000009"}}, firstBalanceIdentity.Headers()...)
		headers = append(headers, [2]string{serviceidentity.ServiceHeader, "ai-cache"})
		host.CallOnHttpRequestHeaders(headers)
		require.Nil(t, host.GetLocalResponse())
		actual, err := serviceidentity.Read(host.GetRequestHeaders())
		require.NoError(t, err)
		require.Equal(t, internalServiceIdentity(), actual)
		_, err = balanceidentity.Read(host.GetRequestHeaders())
		require.Error(t, err)
		ref, err := executionfacts.ReadInternalCallReference(host.GetRequestHeaders())
		require.NoError(t, err)
		require.Equal(t, actual.APIKeyID, ref.APIKeyID)
		host.CompleteHttp()
	})
}

func TestCustomerPublicAndDeniedRulesCannotPublishInternalServiceState(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, mode := range []string{"customer", "public", "denied"} {
			t.Run(mode, func(t *testing.T) {
				config := internalServiceConfig()
				if mode == "public" {
					config["global_auth"] = false
				}
				if mode == "denied" {
					config["allow"] = []string{}
				}
				raw, _ := json.Marshal(config)
				if mode == "customer" {
					raw = nativeKeyAuthFixture(true)
				}
				host, status := test.NewTestHost(raw)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, serviceidentity.Publish(internalServiceIdentity())) // Prior result must not survive this authentication entry.
				credential := "synthetic-service-key"
				if mode == "customer" {
					credential = "synthetic-key-one"
				}
				headers := append([][2]string{{":method", "POST"}, {":path", "/internal-route"}, {":authority", "gateway.example"}, {"Authorization", "Bearer " + credential}, {executionfacts.InternalCallReferenceHeader, "50000000-0000-4000-8000-000000000009"}}, internalServiceIdentity().Headers()...)
				host.CallOnHttpRequestHeaders(headers)
				_, err := executionfacts.ReadInternalCallReference(host.GetRequestHeaders())
				require.Error(t, err)
				_, err = serviceidentity.Read(internalServiceIdentity().Headers())
				require.Error(t, err, "previous authenticated state must not survive any non-service path")
				if mode == "customer" {
					actual, err := balanceidentity.Read(host.GetRequestHeaders())
					require.NoError(t, err)
					require.Equal(t, firstBalanceIdentity, actual)
				}
				if mode == "denied" {
					require.Equal(t, uint32(http.StatusForbidden), host.GetLocalResponse().StatusCode)
				}
				host.CompleteHttp()
			})
		}
	})
}

func TestServiceBindingsAreClosedCompleteAndMutuallyExclusive(t *testing.T) {
	raw, _ := json.Marshal(internalServiceCredentialBinding{Credential: "synthetic-service-key", Snapshot: internalServiceIdentity()})
	for _, field := range []string{"service", "api_key_id", "credential_generation", "tenant_id", "principal_id"} {
		var row map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &row))
		delete(row, field)
		data, _ := json.Marshal(map[string]interface{}{"tenant": "retained-tenant", "internal_service_bindings": []interface{}{row}})
		_, err := parseServiceCredentialBindings(gjson.ParseBytes(data), []string{"synthetic-service-key"})
		require.Error(t, err)
	}
	for _, mode := range []string{"wallet_field", "both", "duplicate", "missing_coverage", "null_customer"} {
		var row map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &row))
		rows := []interface{}{row}
		item := map[string]interface{}{"tenant": "retained-tenant", "internal_service_bindings": rows}
		credentials := []string{"synthetic-service-key"}
		switch mode {
		case "wallet_field":
			row["account_id"] = nil
		case "both":
			item["credential_bindings"] = []interface{}{}
		case "null_customer":
			item["credential_bindings"] = nil
		case "duplicate":
			item["internal_service_bindings"] = []interface{}{row, row}
		case "missing_coverage":
			credentials = append(credentials, "another-service-key")
		}
		data, _ := json.Marshal(item)
		_, err := parseServiceCredentialBindings(gjson.ParseBytes(data), credentials)
		require.Error(t, err)
	}
}
