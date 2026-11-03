package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestSharedRouteCacheUsesOnlyTheAdmittedTenantPolicy(t *testing.T) {
	const secondTenant = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	base := nativeCacheExecutionConfig(t, true)
	var err error
	// Exercise the actual accountless descriptor, preserving its original digest.
	base, err = sjson.DeleteBytes(base, "_modelfusion_execution.descriptor.provider_account_id")
	require.NoError(t, err)
	base, err = sjson.DeleteBytes(base, "_modelfusion_execution.descriptor.descriptor_digest")
	require.NoError(t, err)
	digest, err := mfjson.Digest([]byte(gjson.GetBytes(base, "_modelfusion_execution.descriptor").Raw))
	require.NoError(t, err)
	base, err = sjson.SetBytes(base, "_modelfusion_execution.descriptor.descriptor_digest", digest)
	require.NoError(t, err)
	first, err := sjson.DeleteBytes(base, "_modelfusion_execution")
	require.NoError(t, err)
	second, err := sjson.SetBytes(first, "native_identity.policy_version", 9)
	require.NoError(t, err)
	second, err = sjson.SetBytes(second, "cache_policy_version", "9")
	require.NoError(t, err)
	second, err = sjson.SetBytes(second, "materialized_lookup.redis.key_prefix", "tenant-b-cache:")
	require.NoError(t, err)
	route, err := json.Marshal(map[string]string{"release_id": "opaque-release-without-prefix-semantics", "target_id": "original-target", "route_id": nativeCacheTestRoute})
	require.NoError(t, err)
	policies, err := json.Marshal(map[string]json.RawMessage{nativeCacheTestCaller().TenantID: first, secondTenant: second})
	require.NoError(t, err)
	root := map[string]json.RawMessage{"path_scope": json.RawMessage(`"bound_target"`), "public_model": json.RawMessage(`"public-alias"`), "native_route": route, "tenant_policies": policies}
	for _, field := range []string{"client_protocol", "operation_kind", "capture_body_basis", "consume_body_basis", "native_execution", "_modelfusion_execution"} {
		root[field] = json.RawMessage(gjson.GetBytes(base, field).Raw)
	}
	raw, err := json.Marshal(root)
	require.NoError(t, err)
	for _, tenant := range []string{nativeCacheTestCaller().TenantID, secondTenant, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"} {
		t.Run(tenant, func(t *testing.T) {
			test.RunTest(t, func(t *testing.T) {
				host, status := test.NewTestHost(raw)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				headers := nativeCacheExecutionHeaders(true)
				for index := range headers {
					if headers[index][0] == balanceidentity.TenantHeader {
						headers[index][1] = tenant
					}
				}
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
				action := host.CallOnHttpRequestBody([]byte(nativeCacheTestBody))
				basis, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, "public-alias", basis.PublicModel())
				if tenant == "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" {
					require.Equal(t, types.ActionContinue, action)
					require.Empty(t, host.GetRedisCalloutAttributes())
					require.Empty(t, host.GetHttpCalloutAttributes())
				} else {
					require.Equal(t, types.ActionPause, action)
					calls := host.GetHttpCalloutAttributes()
					require.Len(t, calls, 1)
					require.Empty(t, host.GetRedisCalloutAttributes())
					require.Equal(t, tenant, gjson.GetBytes(calls[0].Body, "native_identity.caller.tenant_id").String())
					require.Equal(t, tenant, gjson.GetBytes(calls[0].Body, "tenant").String())
				}
			})
		})
	}
}
