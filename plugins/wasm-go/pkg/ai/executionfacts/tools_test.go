package executionfacts

import (
	"bytes"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
)

func TestToolBridgePreservesKnownUnknownAndExactCounts(t *testing.T) {
	e := deliveryFixture()
	e.SchemaVersion, e.ServedFrom = ToolSchemaVersion, "upstream"
	e.ToolUsage = map[fixedtools.Tool]ToolMeasurement{
		fixedtools.WebSearch:       {Status: "reported", Count: "9007199254740993", Source: "provider"},
		fixedtools.WebExtractor:    {Status: "missing", Source: "none", ReasonCode: "terminal_count_not_observed"},
		fixedtools.CodeInterpreter: {Status: "known_zero", Count: "0", Source: "provider"},
	}
	snapshot, err := Freeze(e)
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := EncodeToolDeliveryBridge(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeToolDeliveryBridge(bridge)
	if err != nil || !bytes.Equal(snapshot.Bytes(), decoded.Bytes()) {
		t.Fatalf("tool bridge changed evidence: %v", err)
	}
	if _, err := DecodeDeliveryBridge(bridge); err == nil {
		t.Fatal("legacy decoder accepted v5")
	}
	if _, err := DecodeEmbeddingDeliveryBridge(bridge); err == nil {
		t.Fatal("embedding decoder accepted tool observations")
	}
	e.ToolUsage[fixedtools.WebSearch] = ToolMeasurement{Status: "reported", Count: "1", Source: "provider"}
	if snapshot.Envelope().ToolUsage[fixedtools.WebSearch].Count != "9007199254740993" {
		t.Fatal("source mutated a frozen tool snapshot")
	}
	for _, version := range []string{SchemaVersion, ConnectionSchemaVersion, CustomerEmbeddingSchemaVersion} {
		e.SchemaVersion = version
		if _, err := FreezeEmbedding(e); err == nil {
			t.Fatalf("legacy version %s accepted new tool fields", version)
		}
	}
}

func TestToolAttemptKeepsItsVersionAcrossOriginalOutcomeCorrection(t *testing.T) {
	e := attemptFixture()
	e.SchemaVersion, e.ProviderAccountID = ToolSchemaVersion, ""
	a, err := NewAttempt(e)
	if err != nil {
		t.Fatal(err)
	}
	outcome := AttemptOutcome{EvidenceRefs: e.EvidenceRefs, ResponseBodyDigest: e.OutboundBodyDigest, ObservedAt: e.ObservedAt, SendState: "observed_sent", Acceptance: "accepted", ExecutionResult: "succeeded", HTTPStatus: statusFact(200), HeadersComplete: true, BodyComplete: true, Usage: &Usage{Status: "reported", Source: "provider", Unit: "token", Quantities: map[string]string{"input": "10", "output": "2"}}, ToolUsage: map[fixedtools.Tool]ToolMeasurement{fixedtools.WebSearch: {Status: "missing", Source: "none", ReasonCode: "terminal_not_observed"}}}
	first, err := a.Finish(outcome)
	if err != nil {
		t.Fatal(err)
	}
	outcome.ToolUsage[fixedtools.WebSearch] = ToolMeasurement{Status: "reported", Count: "2", Source: "provider"}
	last, err := a.Finish(outcome)
	if err != nil {
		t.Fatal(err)
	}
	corrected, err := Correct(first, last.Envelope())
	if err != nil {
		t.Fatal(err)
	}
	if corrected.Envelope().SchemaVersion != ToolSchemaVersion || corrected.Revision() != "2" || corrected.SourceFactID() != first.SourceFactID() {
		t.Fatal("tool correction changed the original source")
	}
}
