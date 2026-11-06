package main

import (
	"bytes"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/tokenestimate"
	"github.com/stretchr/testify/require"
)

func TestRouterDashScopeStreamObservesFinalUsageAcrossFragments(t *testing.T) {
	first := []byte("event: result\r\ndata:{\"output\":{\"choices\":[{\"message\":{\"reasoning_content\":\"计算\"},\"finish_reason\":\"null\"}]},\"usage\":{\"input_tokens\":2000,\"output_tokens\":1,\"total_tokens\":2001}}\r\n\r\n")
	text := []byte("event: result\ndata: {\"output\":{\"choices\":[{\"message\":{\"content\":[{\"text\":\"结果\"}]},\"finish_reason\":\"null\"}]}}\n\n")
	final := []byte("event: result\ndata:{\"output\":{\"choices\":[{\"finish_reason\":\"stop\"}]},\"usage\":{\"input_tokens\":2000,\"output_tokens\":9,\"total_tokens\":2009,\"prompt_tokens_details\":{\"cached_tokens\":0,\"cache_type\":\"ephemeral\",\"cache_write_tokens\":1920,\"cache_creation_input_tokens\":1920},\"output_tokens_details\":{\"reasoning_tokens\":6}}}\n\n")
	for _, protocol := range []string{managedProtocolDashScopeText, managedProtocolDashScopeMultimodal} {
		t.Run(protocol, func(t *testing.T) {
			stream := managedDashScopeStream{protocol: protocol, incremental: true}
			for _, part := range [][]byte{first, text} {
				for i := range part {
					stream.push(part[i : i+1])
				}
			}
			partial := stream.observation(200, managedProtocolSSEHeaders)
			require.Equal(t, "unknown", partial.ExecutionResult)
			require.Equal(t, "missing", partial.Usage.Status)
			require.Equal(t, "结果", tokenestimate.ResponseText(stream.estimateBody()))
			require.True(t, stream.thinking)
			stream.push(final)
			actual := stream.observation(200, managedProtocolSSEHeaders)
			want := managedQwenObservation(protocol, 200, managedProtocolSSEHeaders, append(append(first, text...), final...), true, 0)
			require.Equal(t, want, actual)
			require.True(t, actual.BodyComplete)
			require.Equal(t, "reported", actual.Usage.Status)
			require.Equal(t, "1920", actual.Usage.Quantities["cache_write"])
			require.Equal(t, "6", actual.Usage.Quantities["reasoning_tokens"])
			applyManagedQwenThinking(&managedExecutionState{}, actual.Usage, stream.thinking)
			require.Equal(t, "1", actual.Usage.Quantities["thinking_mode"])
			stream.push([]byte("data: {\"output\":"))
			require.Equal(t, "missing", stream.observation(200, managedProtocolSSEHeaders).Usage.Status)
		})
	}
}

func TestRouterDashScopeStreamKeepsCumulativeTextOnceForEstimation(t *testing.T) {
	var stream managedDashScopeStream
	stream.push([]byte("data:{\"output\":{\"text\":\"Hello\"}}\n\n"))
	stream.push([]byte("data:{\"output\":{\"text\":\"Hello world\"}}\n\n"))
	require.Equal(t, "Hello world", tokenestimate.ResponseText(stream.estimateBody()))
}

func TestInterruptedDashScopeUsageEstimatesOnlyBaseTokens(t *testing.T) {
	for _, tc := range []struct{ name, marker string }{
		{name: "implicit"},
		{name: "explicit", marker: `,"cache_control":{"type":"ephemeral"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := managedConfigFixture()
			state := &managedExecutionState{
				config: &managedExecutionConfig{
					settings: fixture["managed_execution"].(managedSettings),
					release:  fixture["_modelfusion_execution"].(managedRelease),
					profile:  managedProtocolProfile{supplier: "qwen", UpstreamProtocol: managedProtocolDashScopeText},
				},
				callID:      "50000000-0000-0000-0000-000000000001",
				attemptID:   "original-attempt",
				businessUTC: "2026-11-06T00:00:00Z",
				streaming:   true,
				identity: balanceidentity.Snapshot{
					AccountID:   "10000000-0000-0000-0000-000000000001",
					TenantID:    "30000000-0000-0000-0000-000000000001",
					PrincipalID: "40000000-0000-0000-0000-000000000001",
				},
			}
			body := []byte(`{"model":"upstream-model","input":{"messages":[{"role":"user","content":[{"text":"one two three"` + tc.marker + `}]}]}}`)
			basis, err := bodybasis.Capture(bytes.Replace(body, []byte("upstream-model"), []byte("public-model"), 1))
			require.NoError(t, err)
			require.NoError(t, prepareManagedExecution(state, basis, body))
			stream := managedDashScopeStream{protocol: managedProtocolDashScopeText, incremental: true}
			stream.push([]byte("data: {\"output\":{\"text\":\"Hello world\",\"finish_reason\":\"null\"}}\n\n"))
			observed := stream.observation(200, managedProtocolSSEHeaders)
			require.Equal(t, "missing", observed.Usage.Status)
			usage := availableManagedUsage(state, executionfacts.AttemptOutcome{
				SendState: "observed_sent",
				Usage:     observed.Usage,
			}, stream.estimateBody())
			require.Equal(t, "estimated", usage.Status)
			require.Equal(t, "3", usage.Quantities["input"])
			require.Equal(t, "2", usage.Quantities["output"])
			require.Equal(t, map[string]string{"input": "3", "output": "2"}, usage.Quantities)
		})
	}
}
