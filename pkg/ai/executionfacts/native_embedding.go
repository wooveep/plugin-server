package executionfacts

import "errors"

// NativeEmbeddingSchemaVersion is the separately registered native:v2 wire.
// The original v1 factory and decoder continue to reject this operation.
const NativeEmbeddingSchemaVersion = "billing-facts.v2"
const NativeEmbeddingOperation = "text_embedding"

// EmbeddingAttempt retains the original service/job selection before one send.
// It cannot be constructed from a customer sale or a media/query attempt.
type EmbeddingAttempt struct{ original Envelope }

func NewEmbeddingAttempt(original Envelope) (EmbeddingAttempt, error) {
	original.Kind = "upstream_attempt"
	if original.SchemaVersion != "" && original.SchemaVersion != NativeEmbeddingSchemaVersion ||
		original.PayloadCanonicalization != "" && original.PayloadCanonicalization != Canonicalization {
		return EmbeddingAttempt{}, errorsForVersion()
	}
	original.SchemaVersion, original.PayloadCanonicalization = NativeEmbeddingSchemaVersion, Canonicalization
	if err := validateEmbeddingIdentity(original); err != nil {
		return EmbeddingAttempt{}, err
	}
	if err := validateAttemptIdentity(original); err != nil {
		return EmbeddingAttempt{}, err
	}
	if original.Mechanism != "router" || original.SendTransport != "http_call" || original.RecordingOwner != "ai_proxy" {
		return EmbeddingAttempt{}, errors.New("embedding supports only the original single HTTP attempt")
	}
	copied, err := cloneEnvelope(original)
	return EmbeddingAttempt{original: copied}, err
}

func (a EmbeddingAttempt) Finish(outcome AttemptOutcome) (Snapshot, error) {
	return finishAttempt(a.original, outcome, FreezeEmbedding)
}

// FreezeEmbedding retains the unchanged billing-json.v1 business projection,
// with the native:v2 operation and input-only provider usage constraints.
func FreezeEmbedding(input Envelope) (Snapshot, error) {
	return freezeObservation(input, NativeEmbeddingSchemaVersion, validateEmbeddingUsage, validateEmbeddingEnvelope)
}

func CorrectEmbedding(previous Snapshot, replacement Envelope) (Snapshot, error) {
	return correctObservation(previous, replacement, NativeEmbeddingSchemaVersion, FreezeEmbedding)
}

func validateEmbeddingIdentity(e Envelope) error {
	if e.OperationKind != NativeEmbeddingOperation || e.PrincipalKind != "service" || e.BillingMode != "internal_cost" || e.CostOwner == nil ||
		e.AccountID != "" || e.SalesAssetID != "" || e.OperationID != "" || e.ProviderTaskID != "" || e.ParentAttemptID != "" {
		return errors.New("embedding requires its original internal service call without sale or media identity")
	}
	for _, value := range []string{e.ProducerID, e.CallID, e.PrincipalID, e.CostOwner.OwnerID, e.CostOwner.SourceJobID} {
		if !id(value) {
			return errors.New("embedding requires the original service job owner")
		}
	}
	if !one(e.CostOwner.Kind, "platform", "tenant") || !timestamp(e.OccurredAt) || (e.ReleaseID == "") != (e.RouteID == "") {
		return errors.New("embedding has invalid original ownership or occurrence time")
	}
	return nil
}

func validateEmbeddingEnvelope(e Envelope) error {
	if e.SchemaVersion != NativeEmbeddingSchemaVersion || e.PayloadCanonicalization != Canonicalization {
		return errorsForVersion()
	}
	if err := validateEmbeddingIdentity(e); err != nil {
		return err
	}
	if e.SendTransport != "" && e.SendTransport != "http_call" || e.RecordingOwner != "" && e.RecordingOwner != "ai_proxy" || e.Mechanism != "" && e.Mechanism != "router" {
		return errors.New("embedding cannot declare another sender or retry mechanism")
	}
	return validateObservationFields(e, validateEmbeddingUsage)
}

func validateEmbeddingUsage(u Usage) error {
	if u.Unit != "token" || u.EstimatorVersion != "" {
		return errors.New("embedding usage is provider input tokens only")
	}
	switch u.Status {
	case "reported":
		if u.Source != "provider" || len(u.Quantities) != 1 {
			return errors.New("reported embedding usage requires actual provider input")
		}
		value, exists := u.Quantities["input"]
		if !exists {
			return errors.New("embedding cannot invent output or cache quantities")
		}
		_, err := integer(value, 128)
		return err
	case "missing":
		if u.Source != "none" || u.Quantities != nil || !id(u.ReasonCode) {
			return errors.New("missing embedding usage cannot fabricate zero")
		}
		return nil
	default:
		return errors.New("embedding usage cannot be estimated or protocol zero")
	}
}

func decodeEmbeddingSelection(raw []byte) (map[string]interface{}, error) {
	object, err := decodeSelectionFields(raw, "invocation_id gateway_boot_id context_generation recovery_claim_refs")
	if err != nil {
		return nil, err
	}
	if object["selection_canonicalization"] != "selection-json.v1" || object["operation_kind"] != NativeEmbeddingOperation ||
		object["mechanism"] != "router" || object["send_transport"] != "http_call" || object["recording_owner"] != "ai_proxy" {
		return nil, errors.New("native:v2 embedding selection requires its original single HTTP sender")
	}
	return object, nil
}

// EmbeddingSelectionDigest uses the same mf-json selection projection under the
// new native:v2 schema. It never temporarily relabels an embedding as text.
func EmbeddingSelectionDigest(raw []byte) (string, error) {
	return selectionDigest(raw, decodeEmbeddingSelection)
}

func CanonicalEmbeddingSelection(raw []byte) ([]byte, error) {
	return canonicalSelection(raw, decodeEmbeddingSelection)
}
