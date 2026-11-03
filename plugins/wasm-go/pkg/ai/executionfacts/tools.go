package executionfacts

import (
	"errors"

	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
)

// ToolSchemaVersion adds independent cumulative tools without changing token units.
const ToolSchemaVersion = "billing-facts.v5"

// ToolMeasurement distinguishes reported uses, known zero and unavailable counts.
type ToolMeasurement struct {
	Status     string `json:"status"`
	Count      string `json:"count,omitempty"`
	Source     string `json:"source"`
	ReasonCode string `json:"reason_code,omitempty"`
}

func validateToolUsage(e Envelope) error {
	if len(e.ToolUsage) > 5 || len(e.ToolUsage) > 0 && e.SchemaVersion != ToolSchemaVersion {
		return errors.New("tool measurements require the registered v5 catalog")
	}
	if e.SchemaVersion == ToolSchemaVersion && (e.OperationKind != "text_generate" || e.Usage != nil && e.Usage.Unit != "token") {
		return errors.New("tool observations require token-based text generation")
	}
	for tool, value := range e.ToolUsage {
		if !fixedtools.Valid(tool) {
			return errors.New("unknown fixed tool measurement")
		}
		switch value.Status {
		case "reported":
			count, err := integer(value.Count, 128)
			if err != nil || count.Sign() == 0 || value.Source != "provider" {
				return errors.New("reported tools require positive provider counts")
			}
		case "known_zero":
			if value.Count != "0" || !one(value.Source, "provider", "protocol_zero") {
				return errors.New("known-zero tools require an explicit zero source")
			}
		case "missing":
			if value.Count != "" || value.Source != "none" || value.ReasonCode == "" {
				return errors.New("unavailable tools cannot fabricate zero counts")
			}
		default:
			return errors.New("unsupported tool measurement status")
		}
		if len(value.ReasonCode) > 256 {
			return errors.New("tool measurement reason exceeds its budget")
		}
	}
	return nil
}
