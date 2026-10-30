package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func nativeMediaBillingConfiguration(t *testing.T, capabilities map[string]nativeBillingCapability) []byte {
	t.Helper()
	cfg, _ := nativeBillingConfigFixture(t)
	release := cfg.NativeExecution.(*nativeBillingConfig).release
	release.SchemaVersion = "modelfusion-execution.v2"
	release.CredentialRef, release.CredentialRevision = "", ""
	release.Credentials = []nativeBillingCredential{{CredentialRef: "released-media-credential", CredentialRevision: "1"}}
	release.Descriptor.Capabilities = capabilities
	rawDescriptor, err := json.Marshal(release.Descriptor)
	require.NoError(t, err)
	release.Descriptor.DescriptorDigest, err = nativeBillingDescriptorDigest(rawDescriptor)
	require.NoError(t, err)
	rawRelease, err := json.Marshal(release)
	require.NoError(t, err)
	for _, key := range []string{"credential_ref", "credential_revision"} {
		rawRelease, err = sjson.DeleteBytes(rawRelease, key)
		require.NoError(t, err)
	}
	raw, err := sjson.SetRawBytes([]byte(`{"_rules_":[{"_match_route_":["released-media-route"],"path_scope":"bound_target","redis_stream":{"service_name":"redis.static","service_port":6379,"stream":"private-native-media"},"native_execution":{"producer_id":"native-media-producer","call_identity_source":"admission"}}]}`), "_rules_.0._modelfusion_execution", rawRelease)
	require.NoError(t, err)
	return raw
}

func TestNativeMediaBindingInitializesWithoutAnotherBillingProducer(t *testing.T) {
	for _, operation := range []string{"image_generate", "video_generate"} {
		t.Run(operation, func(t *testing.T) {
			capabilities := map[string]nativeBillingCapability{operation: {SendMode: "single_send", ResponseClassifierRef: "native-media-result", MaxAttempts: 1}}
			if operation == "video_generate" {
				capabilities["task_query"] = nativeBillingCapability{SendMode: "single_send", ResponseClassifierRef: "native-media-query", MaxAttempts: 1}
			}
			configuration := nativeMediaBillingConfiguration(t, capabilities)
			test.RunTest(t, func(t *testing.T) {
				host, status := test.NewTestHost(configuration)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("released-media-route"))
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{{":authority", "media.fixture.test"}, {":method", "POST"}, {":path", "/native/media"}, {"content-type", "application/json"}}))
				require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders([][2]string{{":status", "202"}, {"content-type", "application/json"}}))
				host.CallOnHttpResponseBody([]byte(`{"output":{"task_id":"original-task"},"usage":{"input_tokens":4,"output_tokens":2}}`))
				host.CompleteHttp()
				require.Nil(t, host.GetLocalResponse())
				require.Empty(t, host.GetRedisCalloutAttributes(), "the ai-proxy media owner must remain the sole upstream fact producer")
			})
		})
	}
}

func TestNativeMediaBindingRejectsMissingAmbiguousOrRepeatedCreation(t *testing.T) {
	single := nativeBillingCapability{SendMode: "single_send", ResponseClassifierRef: "native-media-result", MaxAttempts: 1}
	for name, capabilities := range map[string]map[string]nativeBillingCapability{
		"query without creation": {"task_query": single},
		"ambiguous creation":     {"image_generate": single, "video_generate": single},
		"repeated creation":      {"image_generate": {SendMode: "observable_retry", ResponseClassifierRef: "native-media-result", MaxAttempts: 2}},
	} {
		t.Run(name, func(t *testing.T) {
			configuration := nativeMediaBillingConfiguration(t, capabilities)
			test.RunTest(t, func(t *testing.T) {
				host, status := test.NewTestHost(configuration)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		})
	}
}
