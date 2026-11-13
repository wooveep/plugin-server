package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"

	"github.com/stretchr/testify/require"
)

// Regression: https://github.com/wooveep/plugin-server/issues/14.
func TestResponsesIncidentNativeOperationUsesTrustedProfile(t *testing.T) {
	for _, protocol := range []struct{ name, operation, path string }{
		{clientresponse.Chat, string(provider.ApiNameChatCompletion), provider.PathOpenAIChatCompletions},
		{clientresponse.Responses, string(provider.ApiNameResponses), provider.PathOpenAIResponses},
		{clientresponse.Messages, string(provider.ApiNameAnthropicMessages), provider.PathAnthropicMessages},
	} {
		for _, mode := range []string{"auto", "passthrough"} {
			t.Run(protocol.name+"/"+mode, func(t *testing.T) {
				h := incidentProtocolRouter(t, protocol.name, mode, true, nil)
				raw, err := h.host.GetProperty([]string{adapterMetadataProperty})
				require.NoError(t, err)
				var metadata adapterMetadata
				require.NoError(t, json.Unmarshal(raw, &metadata))
				require.Equal(t, 2, metadata.SchemaVersion)
				require.True(t, metadata.Matched)
				require.Equal(t, mode, metadata.Mode)
				require.Equal(t, protocol.path, metadata.OperationTemplate)
				require.Equal(t, protocol.operation, metadata.NormalizedOperation)
				require.Empty(t, metadata.AdapterID)
				require.NotContains(t, string(raw), "/custom/customer/")
				require.NotContains(t, string(raw), "controlled-upstream-key")
			})
		}
	}
	t.Run("embedding", func(t *testing.T) {
		h := newManagedTextHost(t, managedInternalFixture(t, true))
		request := []byte(`{"model":"public-model","input":"hello"}`)
		startManagedInternal(t, h, request, bytes.Replace(request, []byte("public-model"), []byte("upstream-model"), 1))
		raw, err := h.host.GetProperty([]string{adapterMetadataProperty})
		require.NoError(t, err)
		var metadata adapterMetadata
		require.NoError(t, json.Unmarshal(raw, &metadata))
		require.True(t, metadata.Matched)
		require.Equal(t, provider.PathOpenAIEmbeddings, metadata.OperationTemplate)
		require.Equal(t, string(provider.ApiNameEmbeddings), metadata.NormalizedOperation)
	})
}

func TestResponsesIncidentPreservesMatchedConversionMetadata(t *testing.T) {
	fixture := responseModeFixture("openai", clientresponse.Responses, clientresponse.Chat, "auto", clientresponse.OpenAI)
	settings := fixture["managed_execution"].(managedSettings)
	settings.SendTransport, settings.StreamProtocol, settings.StreamProjection = "router", clientresponse.Responses, true
	fixture["managed_execution"] = settings
	h := newManagedHost(t, fixture)
	release := fixture["_modelfusion_execution"].(managedRelease)
	require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(release.Descriptor.ClusterRef)))
	expected := adapterMetadata{SchemaVersion: 2, Matched: true, Mode: "auto", AdapterID: "responses.to_chat_completions", Version: "1", NormalizedOperation: "openai/v1/responses", OperationTemplate: "/v1/responses", ProviderMatch: "openai", CapabilityMatch: "openai/v1/chat/completions", UsageProfile: "openai.responses.v1"}
	raw, err := json.Marshal(expected)
	require.NoError(t, err)
	require.NoError(t, h.host.SetProperty([]string{adapterMetadataProperty}, raw))
	h.start(t, responseModeRequest(clientresponse.Responses, release.PublicModel, true), responseModeRequest(clientresponse.Responses, "upstream-model", true))
	observed, err := h.host.GetProperty([]string{adapterMetadataProperty})
	require.NoError(t, err)
	require.Equal(t, raw, observed)
}

func TestResponsesIncidentMismatchedMetadataRemainsUnknown(t *testing.T) {
	for _, mismatch := range []string{"operation", "schema"} {
		t.Run(mismatch, func(t *testing.T) {
			fixture := responseModeFixture("openai", clientresponse.Responses, clientresponse.Responses, "auto", clientresponse.OpenAI)
			h := newManagedHost(t, fixture)
			metadata := adapterMetadata{SchemaVersion: 2, Matched: true, Mode: "auto", NormalizedOperation: "openai/v1/files/get", OperationTemplate: "/v1/files/{file_id}"}
			if mismatch == "schema" {
				metadata.SchemaVersion = 99
			}
			raw, err := json.Marshal(metadata)
			require.NoError(t, err)
			require.NoError(t, h.host.SetProperty([]string{adapterMetadataProperty}, raw))
			request := responseModeRequest(clientresponse.Responses, "public-model", false)
			h.start(t, request, bytes.Replace(request, []byte("public-model"), []byte("upstream-model"), 1))
			raw, err = h.host.GetProperty([]string{adapterMetadataProperty})
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &metadata))
			require.False(t, metadata.Matched)
		})
	}
}
