package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/ai/resourceref"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

func TestNativeResponsesCacheReplayReissuesResourcesWithoutAliasChains(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		cfg := publishedCacheConfig(t, nativeCacheExecutionConfig(t, true), "openai_responses", "text_generate")
		host, status := test.NewTestHost(cfg)
		require.Equal(t, types.OnPluginStartStatusOK, status)
		defer host.Reset()
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheExecutionHeaders(true)))
		body := []byte(`{"model":"public-alias","input":"hello"}`)
		require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
		completeNativeCacheRegistration(t, host)
		adapter, _ := thinProtocolAdapterForKind(protocol.ProtocolResponses)
		digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{Method: "POST", Path: "/native/custom-answer", Body: body})
		require.NoError(t, err)
		const old = "resp_pf_33333333333333333333333333333333"
		const item = "item_pf_44444444444444444444444444444444"
		record := MaterializedReplayRecord{SchemaVersion: materializedRecordSchemaVersion,
			Tenant: nativeCacheTestCaller().TenantID, Consumer: nativeCacheTestCaller().PrincipalID, Route: nativeCacheTestRoute, Model: "public-alias", Protocol: protocol.ProtocolResponses, CacheScope: "consumer", CachePolicyVersion: "23", RequestDigest: digest.Digest,
			SoftExpiresAt: time.Now().Unix() + 60, HardExpiresAt: time.Now().Unix() + 3600, FinishReason: "completed", Usage: json.RawMessage(`{"input_tokens":17,"output_tokens":5,"total_tokens":22}`),
			Response: json.RawMessage(`{"id":"` + old + `","object":"response","status":"completed","output":[{"id":"` + item + `","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"cached answer","annotations":[]}]}]}`)}
		record.CachePolicyVersion = strconv.FormatInt(nativeCacheTestVersion, 10)
		raw, err := json.Marshal(record)
		require.NoError(t, err)
		host.CallOnRedisCall(0, test.CreateRedisRespString(string(raw)))
		require.Nil(t, host.GetLocalResponse(), "resource aliases must be restored and saved before cache delivery")
		calls := host.GetRedisCalloutAttributes()
		require.Len(t, calls, 1)
		query, _, err := resp.NewReader(bytes.NewReader(calls[0].Query)).ReadValue()
		require.NoError(t, err)
		require.Equal(t, "MGET", query.Array()[0].String())
		owner := resourceref.Owner{Provider: "native-source", Account: "native-account", Region: "native-region", Model: "native-model", Protocol: clientresponse.Responses}
		values := []resp.Value{}
		for _, key := range query.Array()[1:] {
			kind, public, original := "response", old, "private-native-response"
			if bytes.HasSuffix([]byte(key.String()), []byte(item)) {
				kind, public, original = "item", item, "private-native-item"
			}
			value, e := json.Marshal(resourceref.NewRecord(clientresponse.ResourceAlias{Kind: kind, Public: public, Original: original}, owner, true))
			require.NoError(t, e)
			values = append(values, resp.StringValue(string(value)))
		}
		var reply bytes.Buffer
		require.NoError(t, resp.NewWriter(&reply).WriteArray(values))
		host.CallOnRedisCall(0, reply.Bytes())
		require.Nil(t, host.GetLocalResponse())
		calls = host.GetRedisCalloutAttributes()
		require.Len(t, calls, 1)
		require.Contains(t, string(calls[0].Query), "modelfusion-public-resource-write.v1")
		require.Contains(t, string(calls[0].Query), "private-native-response")
		require.NotContains(t, string(calls[0].Query), old, "replay must not create an alias chain")
		host.CallOnRedisCall(0, []byte(":1\r\n"))
		response := host.GetLocalResponse()
		require.NotNil(t, response)
		require.NotEqual(t, old, gjson.GetBytes(response.Data, "id").String())
		require.NotContains(t, string(response.Data), "private-native")
		bridge := thinReplayPayloadHeader(response.Headers, executionfacts.DeliveryBridgeHeader)
		snapshot, err := executionfacts.DecodeDeliveryBridge(bridge)
		require.NoError(t, err)
		require.Equal(t, "cache", snapshot.Envelope().ServedFrom)
		require.Equal(t, "estimated", snapshot.Envelope().Usage.Status)
	})
}
