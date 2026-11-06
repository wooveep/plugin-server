package main

import (
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

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
	if usage.Quantities["input"] != "3" || usage.Quantities["cache_write"] != "3" || usage.Quantities["cache_hit"] != "0" {
		t.Fatal("partial uncached input replaced the full explicit-cache estimate", usage)
	}
}
