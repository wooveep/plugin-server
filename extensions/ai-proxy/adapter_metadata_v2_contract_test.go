package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestAdapterMetadataV2GoWasmContract(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "adapter_metadata_v2_qwen_responses.yaml"))
	require.NoError(t, err)

	type metadataEnvelope struct {
		SchemaVersion       int    `json:"schema_version"`
		Matched             bool   `json:"matched"`
		Mode                string `json:"mode"`
		AdapterID           string `json:"adapter_id"`
		Version             string `json:"version"`
		NormalizedOperation string `json:"normalized_operation"`
		OperationTemplate   string `json:"operation_template"`
		ProviderMatch       string `json:"provider_match"`
		CapabilityMatch     string `json:"capability_match"`
		UsageProfile        string `json:"usage_profile"`
	}

	readMetadata := func(t *testing.T, host wasmhost.TestHost) ([]byte, metadataEnvelope) {
		t.Helper()
		raw, err := host.GetProperty([]string{adapterMetadataProperty})
		require.NoError(t, err)
		require.NotEmpty(t, raw)
		require.LessOrEqual(t, len(raw), maxAdapterMetadataBytes)
		var metadata metadataEnvelope
		require.NoError(t, json.Unmarshal(raw, &metadata))
		return raw, metadata
	}

	wasmhost.RunTest(t, func(t *testing.T) {
		t.Run("matched_responses_adapter_publishes_exact_v2_golden", func(t *testing.T) {
			host, status := wasmhost.NewTestHost(responsesFallbackQwenConfig)
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "dashscope.aliyuncs.com"},
				{":path", "/v1/responses"},
				{":method", "POST"},
				{"content-type", "application/json"},
			}))

			raw, metadata := readMetadata(t, host)
			require.Equal(t, bytes.TrimSpace(golden), raw,
				"producer bytes must exactly match the shared consumer golden")
			require.Equal(t, 2, metadata.SchemaVersion)
			require.True(t, metadata.Matched)
			require.Equal(t, "auto", metadata.Mode)
			require.Equal(t, "openai/v1/responses", metadata.NormalizedOperation)
			require.Equal(t, "/v1/responses", metadata.OperationTemplate)
		})

		t.Run("unmatched_request_publishes_no_adapter_or_operation_claim", func(t *testing.T) {
			host, status := wasmhost.NewTestHost(responsesFallbackQwenConfig)
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "dashscope.aliyuncs.com"},
				{":path", "/v1/responses"},
				{":method", "GET"},
				{"content-type", "application/json"},
			}))

			_, metadata := readMetadata(t, host)
			require.Equal(t, 2, metadata.SchemaVersion)
			require.False(t, metadata.Matched)
			require.Empty(t, metadata.AdapterID)
			require.Empty(t, metadata.Version)
			require.Empty(t, metadata.NormalizedOperation)
			require.Empty(t, metadata.OperationTemplate)
			require.Empty(t, metadata.ProviderMatch)
			require.Empty(t, metadata.CapabilityMatch)
			require.Empty(t, metadata.UsageProfile)
		})

		t.Run("forced_passthrough_publishes_no_skipped_adapter_claim", func(t *testing.T) {
			config := json.RawMessage(`{
				"protocol_mode":"passthrough",
				"provider":{
					"type":"qwen",
					"apiTokens":["provider-passthrough-token"],
					"qwenEnableCompatible":true,
					"capabilities":{"openai/v1/responses":""}
				}
			}`)
			host, status := wasmhost.NewTestHost(config)
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "dashscope.aliyuncs.com"},
				{":path", "/v1/responses"},
				{":method", "POST"},
				{"content-type", "application/json"},
			}))

			_, metadata := readMetadata(t, host)
			require.Equal(t, 2, metadata.SchemaVersion)
			require.False(t, metadata.Matched)
			require.Equal(t, "passthrough", metadata.Mode)
			require.Empty(t, metadata.AdapterID)
			require.Empty(t, metadata.NormalizedOperation)
			require.Empty(t, metadata.OperationTemplate)
			require.Empty(t, metadata.UsageProfile)
		})
	})
}
