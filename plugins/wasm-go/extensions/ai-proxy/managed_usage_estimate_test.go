package main

import (
	"slices"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/stretchr/testify/require"
)

func TestNativeBailianBodyRequestIDRemainsAvailableForSLSWithoutHeaders(t *testing.T) {
	s := &managedExecutionState{attemptID: "attempt-1", config: &managedExecutionConfig{settings: managedSettings{Authority: "dashscope.aliyuncs.com"}, profile: managedProtocolProfile{supplier: "qwen"}}}
	const id = "a8a7295d-0699-991f-849a-97f767160077"
	recordManagedBailianRequestID(s, id)
	require.Equal(t, id, s.upstreamRequestID)
	require.Contains(t, appendManagedBailianEvidence(s, nil, nil), "bailian-request:"+id)
	require.Empty(t, appendManagedBailianEvidence(s, [][2]string{{"x-request-id", "c652718f-433b-9413-a8dd-c81cb014ff54"}}, nil))
}

func TestManagedResponseHeaderIDPrecedesNativeStreamBodyID(t *testing.T) {
	for _, name := range []string{"x-request-id", "request-id", "x-dashscope-request-id"} {
		t.Run(name, func(t *testing.T) {
			s := &managedExecutionState{config: &managedExecutionConfig{profile: managedProtocolProfile{supplier: "qwen"}}}
			captureManagedUpstreamID(s, [][2]string{{name, "original-header-id"}})
			recordManagedBailianRequestID(s, "a8a7295d-0699-991f-849a-97f767160077")
			require.Equal(t, "original-header-id", s.upstreamRequestID)
		})
	}
}

func TestBailianThinkingEvidenceRetainsOnlyExplicitRequestModes(t *testing.T) {
	for _, tc := range []struct{ protocol, body, want string }{
		{managedProtocolChat, `{}`, ""},
		{managedProtocolChat, `{"enable_thinking":false}`, "bailian-thinking:disabled"},
		{managedProtocolChat, `{"enable_thinking":true}`, "bailian-thinking:enabled"},
		{managedProtocolDashScopeText, `{"parameters":{"enable_thinking":false}}`, "bailian-thinking:disabled"},
	} {
		s := &managedExecutionState{attemptID: "attempt-1", publicRequest: []byte(tc.body),
			config: &managedExecutionConfig{settings: managedSettings{Authority: "dashscope.aliyuncs.com"},
				profile: managedProtocolProfile{supplier: "qwen", UpstreamProtocol: tc.protocol}}}
		refs := appendManagedBailianEvidence(s, [][2]string{{"x-request-id", "a8a7295d-0699-991f-849a-97f767160077"}}, nil)
		if tc.want != "" && !slices.Contains(refs, tc.want) || tc.want == "" && len(refs) != 2 {
			t.Fatal("explicit mode evidence differs", tc, refs)
		}
	}
}

func TestManagedAvailableUsageRespectsProviderZeroAndEstimatesMissingParts(t *testing.T) {
	state := &managedExecutionState{config: &managedExecutionConfig{}, requestText: "hello input"}
	state.config.release.Descriptor.UpstreamModel = "unknown"
	outcome := executionfacts.AttemptOutcome{SendState: "observed_sent", Usage: executionfacts.MissingTokenUsage("omitted")}
	usage := availableManagedUsage(state, outcome, []byte(`{"usage":{"prompt_tokens":0},"choices":[{"message":{"content":"hello output"}}]}`))
	if usage.Status != "estimated" || usage.Quantities["input"] != "0" || usage.Quantities["output"] == "0" || usage.EstimatorVersion == "" {
		t.Fatal("available provider input was overwritten", usage)
	}
	usage = availableManagedUsage(state, outcome, []byte(`{"error":{"message":"unavailable"}}`))
	if usage.Quantities["input"] == "0" || usage.Quantities["output"] != "0" {
		t.Fatal("failed send did not retain available input-only charge", usage)
	}
	for _, send := range []string{"not_sent", "handed_off", "unknown"} {
		outcome.SendState = send
		if usage := availableManagedUsage(state, outcome, nil); usage.Status != "missing" {
			t.Fatal("unconfirmed send became billable", send)
		}
	}
}

func TestManagedReportedUsageStaysExact(t *testing.T) {
	state := &managedExecutionState{config: &managedExecutionConfig{}}
	reported := &executionfacts.Usage{Status: "reported", Source: "provider", Unit: "token", Quantities: map[string]string{"input": "9007199254740993", "output": "0"}}
	actual := availableManagedUsage(state, executionfacts.AttemptOutcome{SendState: "observed_sent", Usage: reported}, nil)
	if actual != reported {
		t.Fatal("exact reported usage was replaced")
	}
}

func TestImplicitCacheEstimateDoesNotPriceImpossibleExplicitWrites(t *testing.T) {
	state := &managedExecutionState{config: &managedExecutionConfig{}, requestText: "available input", cacheMode: managedCacheImplicit}
	usage := availableManagedUsage(state, executionfacts.AttemptOutcome{SendState: "observed_sent", Usage: executionfacts.MissingTokenUsage("omitted")}, nil)
	if usage.Quantities["cache_write"] != "0" || usage.Quantities["cache_read_mode"] != "0" || usage.Source != "estimator" {
		t.Fatal("known implicit cache constraints were lost", usage)
	}
	if _, exists := usage.Quantities["thinking_mode"]; exists {
		t.Fatal("estimation invented an actual thinking mode", usage)
	}
}

func TestExplicitMessagesEstimateUsesTotalInput(t *testing.T) {
	state := &managedExecutionState{
		config: &managedExecutionConfig{profile: managedProtocolProfile{
			UpstreamProtocol: managedProtocolMessages,
		}},
		cacheMode:   managedCacheExplicit,
		requestText: "one two three",
	}
	outcome := executionfacts.AttemptOutcome{
		SendState: "observed_sent",
		Usage:     executionfacts.MissingTokenUsage("omitted"),
	}
	usage := availableManagedUsage(state, outcome, []byte(`{"usage":{"input_tokens":1,"cache_read_input_tokens":2}}`))
	if usage.Quantities["input"] != "3" || usage.Quantities["cache_hit"] != "2" || usage.Quantities["cache_miss"] != "1" {
		t.Fatal("estimate did not retain the total input and reported hit", usage)
	}
	for _, key := range []string{"cache_write", "cache_read_mode", "thinking_mode"} {
		if _, exists := usage.Quantities[key]; exists {
			t.Fatal("estimate invented a conditional meter", key, usage)
		}
	}
}
