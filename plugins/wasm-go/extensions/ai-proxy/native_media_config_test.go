package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func nativeMediaProxyConfiguration(t *testing.T, profile string, misplacedEstimate bool) []byte {
	t.Helper()
	fixture := managedPoolFixture(t)
	configuration, err := decodeManagedPoolFixture(t, fixture)
	require.NoError(t, err)
	protocol, err := mediaexecution.LookupProfile(profile)
	require.NoError(t, err)
	configuration.settings.MediaDispatchMode = "managed_operation"
	configuration.settings.InputProtocol, configuration.settings.UpstreamProtocol = profile, profile
	configuration.settings.UpstreamPath = protocol.CreatePath
	descriptor := &configuration.release.Descriptor
	descriptor.UpstreamModel = protocol.Model
	capability := managedCapability{SendMode: "single_send", ResponseClassifierRef: "native-media-result", MaxAttempts: 1}
	if protocol.Asynchronous {
		capability.QueryCapabilityRef = "native-media-query"
	}
	descriptor.Capabilities = map[string]managedCapability{protocol.OperationKind: capability}
	if protocol.Asynchronous {
		descriptor.Capabilities["task_query"] = capability
	}
	configuration.settings.UpstreamHealth.Targets = nil
	for _, credential := range configuration.release.Credentials {
		for operation := range descriptor.Capabilities {
			configuration.settings.UpstreamHealth.Targets = append(configuration.settings.UpstreamHealth.Targets, upstreamhealth.TargetReference{
				TargetID: descriptor.TargetID, BindingID: descriptor.BindingID, ProviderAccountID: descriptor.ProviderAccountID,
				RegionEndpointRef: descriptor.RegionEndpointRef, UpstreamModel: descriptor.UpstreamModel,
				OperationKind: operation, CredentialRef: credential.CredentialRef,
				CredentialGeneration: upstreamhealth.Revision(credential.CredentialRevision), ScopeIDs: []string{"media-model-scope"},
			})
		}
	}
	rawDescriptor, err := json.Marshal(descriptor)
	require.NoError(t, err)
	descriptor.DescriptorDigest, err = managedDescriptorDigest(rawDescriptor)
	require.NoError(t, err)
	fixture["managed_execution"], fixture["_modelfusion_execution"] = configuration.settings, configuration.release
	raw := managedFixtureJSON(t, fixture)
	for _, path := range []string{"provider", "managed_execution.upstream_health.target"} {
		raw, err = sjson.DeleteBytes(raw, path)
		require.NoError(t, err)
	}
	raw, err = sjson.SetRawBytes(raw, "providers", []byte(`[{"id":"media-provider","type":"generic","apiTokens":["second-controlled-key","first-controlled-key"]}]`))
	require.NoError(t, err)
	raw, err = sjson.SetBytes(raw, "activeProviderId", "media-provider")
	require.NoError(t, err)
	raw, err = sjson.SetBytes(raw, "_match_route_", []string{"released-media-route"})
	require.NoError(t, err)
	if misplacedEstimate {
		raw, err = sjson.SetBytes(raw, "managed_execution.usage_estimation", "disabled")
		require.NoError(t, err)
	}
	result, err := json.Marshal(struct {
		Rules []json.RawMessage `json:"_rules_"`
	}{Rules: []json.RawMessage{raw}})
	require.NoError(t, err)
	return result
}

func TestNativeMediaScopedConfigurationInitializesTheRealPlugin(t *testing.T) {
	for _, profile := range []string{mediaexecution.QwenImage30NativeSync, mediaexecution.Profile, mediaexecution.Wan30Text} {
		t.Run(profile, func(t *testing.T) {
			configuration := nativeMediaProxyConfiguration(t, profile, false)
			test.RunTest(t, func(t *testing.T) {
				host, status := test.NewTestHost(configuration)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("released-media-route"))
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestHeaders([][2]string{{":authority", "media.fixture.test"}, {":method", "POST"}, {":path", "/native/media"}, {"content-type", "application/json"}}))
				require.NotNil(t, host.GetLocalResponse())
				require.Equal(t, uint32(409), host.GetLocalResponse().StatusCode, "native media dispatch stays with the original Console operation API")
			})
		})
	}
}

func TestNativeMediaRuntimeRejectsConsoleOnlyEstimationPolicy(t *testing.T) {
	configuration := nativeMediaProxyConfiguration(t, mediaexecution.QwenImage30NativeSync, true)
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configuration)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusFailed, status)
	})
}
