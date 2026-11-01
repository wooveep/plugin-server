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
	"github.com/tidwall/sjson"
)

func TestSharedRouteHistoryUsesOnlyTheAdmittedTenantPolicy(t *testing.T) {
	const secondTenant = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	first, err := sjson.SetBytes([]byte(nativeHistoryConfig), "capture_body_basis", true)
	require.NoError(t, err)
	second, err := sjson.SetBytes(first, "native_identity.policy_version", 9)
	require.NoError(t, err)
	second, err = sjson.SetBytes(second, "cacheKeyPrefix", "tenant-b-history:")
	require.NoError(t, err)
	route, err := json.Marshal(map[string]string{"release_id": "original-release", "target_id": "original-target", "route_id": "00000000-0000-4000-8000-000000000001"})
	require.NoError(t, err)
	policies, err := json.Marshal(map[string]json.RawMessage{historyTestCaller().TenantID: first, secondTenant: second})
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]json.RawMessage{"path_scope": json.RawMessage(`"bound_target"`), "public_model": json.RawMessage(`"public-model"`),
		"client_protocol": json.RawMessage(`"openai_chat"`), "operation_kind": json.RawMessage(`"text_generate"`), "capture_body_basis": json.RawMessage(`true`), "native_route": route, "tenant_policies": policies})
	require.NoError(t, err)
	for _, tenant := range []string{historyTestCaller().TenantID, secondTenant, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"} {
		t.Run(tenant, func(t *testing.T) {
			test.RunTest(t, func(t *testing.T) {
				host, status := test.NewTestHost(raw)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				caller := historyTestCaller()
				caller.TenantID = tenant
				body := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hello"}]}`)
				headers := append(historyTestHeaders(t, body, caller), [2]string{executionfacts.AdmissionCallHeader, "99999999-9999-4999-8999-999999999999"},
					[2]string{executionfacts.AdmissionReleaseHeader, "original-release"},
					[2]string{executionfacts.AdmissionOccurredHeader, time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)})
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
				action := host.CallOnHttpRequestBody(body)
				basis, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, "public-model", basis.PublicModel())
				if tenant == "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" {
					require.Equal(t, types.ActionContinue, action)
					require.Empty(t, host.GetRedisCalloutAttributes())
				} else {
					require.Equal(t, types.ActionPause, action)
					require.Len(t, host.GetRedisCalloutAttributes(), 1)
					if tenant == secondTenant {
						require.Contains(t, historyTestCommand(t, host.GetRedisCalloutAttributes()[0].Query)[1], "tenant-b-history:")
					}
				}
			})
		})
	}
}
