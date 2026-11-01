package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestSharedRouteMemoryUsesOnlyTheAdmittedTenantPolicy(t *testing.T) {
	const secondTenant = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	first := []byte(gjson.GetBytes(memoryNativeConfiguration(t, "semantic", false), "_rules_.0").Raw)
	second, err := sjson.SetBytes(first, "native_identity.policy_version", 9)
	require.NoError(t, err)
	route, err := json.Marshal(map[string]string{"release_id": "original/release:opaque", "target_id": "original/target:opaque", "route_id": memoryNativeRouteID})
	require.NoError(t, err)
	policies, err := json.Marshal(map[string]json.RawMessage{memoryNativeCallerFixture().TenantID: first, secondTenant: second})
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]json.RawMessage{"path_scope": json.RawMessage(`"bound_target"`), "public_model": json.RawMessage(`"public.model"`),
		"client_protocol": json.RawMessage(`"openai_chat"`), "operation_kind": json.RawMessage(`"text_generate"`), "capture_body_basis": json.RawMessage(`true`), "native_route": route, "tenant_policies": policies})
	require.NoError(t, err)
	for _, tenant := range []string{memoryNativeCallerFixture().TenantID, secondTenant, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"} {
		t.Run(tenant, func(t *testing.T) {
			test.RunTest(t, func(t *testing.T) {
				caller := memoryNativeCallerFixture()
				caller.TenantID = tenant
				headers := append(caller.Headers(), [2]string{executionfacts.AdmissionCallHeader, "99999999-9999-4999-8999-999999999999"},
					[2]string{executionfacts.AdmissionReleaseHeader, "original/release:opaque"},
					[2]string{executionfacts.AdmissionOccurredHeader, time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)})
				host := startNativeMemoryHeaders(t, raw, headers)
				defer host.Reset()
				body := []byte(`{"model":"public.model","messages":[{"role":"user","content":"hello"}]}`)
				action := host.CallOnHttpRequestBody(body)
				basis, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, "public.model", basis.PublicModel())
				if tenant == "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" {
					require.Equal(t, types.ActionContinue, action)
					require.Empty(t, host.GetRedisCalloutAttributes())
					require.Empty(t, host.GetHttpCalloutAttributes())
				} else {
					require.Equal(t, types.ActionPause, action)
					facts := requireMemoryAssembleCall(t, host)
					require.Equal(t, tenant, facts["tenant"])
					if tenant == secondTenant {
						require.Equal(t, "9", facts["policy_version"])
					} else {
						require.Equal(t, "7", facts["policy_version"])
					}
				}
			})
		})
	}
}
