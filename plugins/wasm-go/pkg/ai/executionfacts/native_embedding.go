package executionfacts

import "errors"

// NativeEmbeddingSchemaVersion is the separately registered native:v2 wire.
// The original v1 factory and decoder continue to reject this operation.
const NativeEmbeddingSchemaVersion = "billing-facts.v2"

// CustomerEmbeddingSchemaVersion adds wallet-backed input-only observations.
const CustomerEmbeddingSchemaVersion = "billing-facts.v4"
const CustomerEmbeddingRegistrationVersion = "execution-registration.v3"
const NativeEmbeddingOperation = "text_embedding"

const RetrievalSchemaVersion = "billing-facts.v6"
const RetrievalRegistrationVersion = "execution-registration.v4"
const MultimodalEmbeddingOperation = "multimodal_embedding"
const RerankOperation = "rerank"
const RetrievalClassifier = "ai-proxy.native-retrieval.v1"

// RetrievalProtocol identifies one registered, unconverted supplier wire API.
func RetrievalProtocol(protocol string) (operation, path, adapter string) {
	switch protocol {
	case "dashscope_multimodal_embedding":
		return MultimodalEmbeddingOperation, "/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding", "dashscope-multimodal-embedding.v1"
	case "dashscope_rerank":
		return RerankOperation, "/api/v1/services/rerank/text-rerank/text-rerank", "dashscope-rerank.v1"
	case "bailian_rerank":
		return RerankOperation, "/compatible-api/v1/reranks", "bailian-rerank.v1"
	default:
		return "", "", ""
	}
}

func IsRetrieval(operation string) bool {
	return one(operation, MultimodalEmbeddingOperation, RerankOperation)
}

func IsInputOnly(operation string) bool {
	return operation == NativeEmbeddingOperation || IsRetrieval(operation)
}

// EmbeddingAttempt retains the original customer or service selection before one send.
type EmbeddingAttempt struct{ original Envelope }

func NewEmbeddingAttempt(original Envelope) (EmbeddingAttempt, error) {
	original.Kind = "upstream_attempt"
	version := NativeEmbeddingSchemaVersion
	if original.ProviderAccountID == "" {
		version = ConnectionSchemaVersion
	}
	if original.BillingMode == "customer" || original.SchemaVersion == CustomerEmbeddingSchemaVersion {
		version = CustomerEmbeddingSchemaVersion
	}
	if IsRetrieval(original.OperationKind) {
		version = RetrievalSchemaVersion
	}
	if original.SchemaVersion != "" && original.SchemaVersion != version ||
		original.PayloadCanonicalization != "" && original.PayloadCanonicalization != Canonicalization {
		return EmbeddingAttempt{}, errorsForVersion()
	}
	original.SchemaVersion, original.PayloadCanonicalization = version, Canonicalization
	if err := validateEmbeddingIdentity(original); err != nil {
		return EmbeddingAttempt{}, err
	}
	if err := validateAttemptIdentity(original); err != nil {
		return EmbeddingAttempt{}, err
	}
	if original.Mechanism != "router" || !(original.SendTransport == "router" && original.RecordingOwner == "router_observer" || original.SendTransport == "http_call" && original.RecordingOwner == "ai_proxy") {
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
	version := NativeEmbeddingSchemaVersion
	if input.SchemaVersion == ConnectionSchemaVersion {
		version = ConnectionSchemaVersion
	}
	if input.BillingMode == "customer" || input.SchemaVersion == CustomerEmbeddingSchemaVersion {
		version = CustomerEmbeddingSchemaVersion
	}
	usage := validateEmbeddingUsage
	if version == CustomerEmbeddingSchemaVersion {
		usage = validateCustomerEmbeddingUsage
	}
	if IsRetrieval(input.OperationKind) {
		version = RetrievalSchemaVersion
		usage = func(u Usage) error { return validateRetrievalUsage(input.OperationKind, u) }
	}
	return freezeObservation(input, version, usage, validateEmbeddingEnvelope)
}

func CorrectEmbedding(previous Snapshot, replacement Envelope) (Snapshot, error) {
	version := NativeEmbeddingSchemaVersion
	if previous.envelope.SchemaVersion == ConnectionSchemaVersion {
		version = ConnectionSchemaVersion
	}
	if one(previous.envelope.SchemaVersion, CustomerEmbeddingSchemaVersion, RetrievalSchemaVersion) {
		version = previous.envelope.SchemaVersion
	}
	return correctObservation(previous, replacement, version, FreezeEmbedding)
}

func validateEmbeddingIdentity(e Envelope) error {
	validOperation := e.OperationKind == NativeEmbeddingOperation
	if e.SchemaVersion == RetrievalSchemaVersion {
		validOperation = IsRetrieval(e.OperationKind) && e.BillingMode == "customer"
	}
	if !validOperation || e.OperationID != "" || e.ProviderTaskID != "" || e.ParentAttemptID != "" {
		return errors.New("embedding requires its original synchronous call without media identity")
	}
	for _, value := range []string{e.ProducerID, e.CallID, e.PrincipalID} {
		if !id(value) {
			return errors.New("embedding requires its original caller identity")
		}
	}
	if e.BillingMode == "customer" && one(e.SchemaVersion, CustomerEmbeddingSchemaVersion, RetrievalSchemaVersion) {
		if e.PrincipalKind != "user" || e.CostOwner != nil || !id(e.TenantID) || !id(e.AccountID) || !id(e.SalesAssetID) {
			return errors.New("customer embedding requires its original user and sale wallet")
		}
	} else if e.BillingMode == "internal_cost" {
		if e.PrincipalKind != "service" || e.CostOwner == nil || e.AccountID != "" || e.SalesAssetID != "" ||
			!one(e.CostOwner.Kind, "platform", "tenant") || !id(e.CostOwner.OwnerID) || !id(e.CostOwner.SourceJobID) {
			return errors.New("internal embedding requires its original service job owner")
		}
	} else {
		return errors.New("embedding caller carrier does not match its registered version")
	}
	if !timestamp(e.OccurredAt) || (e.ReleaseID == "") != (e.RouteID == "") {
		return errors.New("embedding has invalid original ownership or occurrence time")
	}
	return nil
}

func validateEmbeddingEnvelope(e Envelope) error {
	if !one(e.SchemaVersion, NativeEmbeddingSchemaVersion, ConnectionSchemaVersion, CustomerEmbeddingSchemaVersion, RetrievalSchemaVersion) || e.PayloadCanonicalization != Canonicalization {
		return errorsForVersion()
	}
	if err := validateEmbeddingIdentity(e); err != nil {
		return err
	}
	if (e.SendTransport != "" || e.RecordingOwner != "") && !(e.SendTransport == "router" && e.RecordingOwner == "router_observer" || e.SendTransport == "http_call" && e.RecordingOwner == "ai_proxy") || e.Mechanism != "" && e.Mechanism != "router" {
		return errors.New("embedding cannot declare another sender or retry mechanism")
	}
	usage := validateEmbeddingUsage
	if e.SchemaVersion == CustomerEmbeddingSchemaVersion {
		usage = validateCustomerEmbeddingUsage
	}
	if e.SchemaVersion == RetrievalSchemaVersion {
		usage = func(u Usage) error { return validateRetrievalUsage(e.OperationKind, u) }
	}
	return validateObservationFields(e, usage)
}

func validateCustomerEmbeddingUsage(u Usage) error {
	if u.Status != "estimated" {
		return validateEmbeddingUsage(u)
	}
	if u.Unit != "token" || u.Source != "estimator" || !id(u.EstimatorVersion) || len(u.Quantities) != 1 {
		return errors.New("estimated embedding usage requires its input estimator")
	}
	value, exists := u.Quantities["input"]
	if !exists {
		return errors.New("embedding cannot invent output or cache quantities")
	}
	_, err := integer(value, 128)
	return err
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

func validateRetrievalUsage(operation string, u Usage) error {
	if u.Status == "reported" && operation == MultimodalEmbeddingOperation && len(u.Quantities) == 2 {
		input, inputErr := integer(u.Quantities["input"], 128)
		visual, visualErr := integer(u.Quantities["visual_input"], 128)
		if u.Unit != "token" || u.Source != "provider" || u.EstimatorVersion != "" || inputErr != nil || visualErr != nil || visual.Cmp(input) > 0 {
			return errors.New("multimodal usage requires provider input and its visual subset")
		}
		return nil
	}
	return validateEmbeddingUsage(u)
}

func decodeInputOnlySelection(raw []byte, retrieval bool) (map[string]interface{}, error) {
	object, err := decodeSelectionFields(raw, "invocation_id gateway_boot_id context_generation recovery_claim_refs")
	if err != nil {
		return nil, err
	}
	operation, _ := object["operation_kind"].(string)
	if object["selection_canonicalization"] != "selection-json.v1" || (!retrieval && operation != NativeEmbeddingOperation || retrieval && !IsRetrieval(operation)) ||
		object["mechanism"] != "router" || !(object["send_transport"] == "router" && object["recording_owner"] == "router_observer" || object["send_transport"] == "http_call" && object["recording_owner"] == "ai_proxy") {
		return nil, errors.New("native:v2 embedding selection requires its original single HTTP sender")
	}
	return object, nil
}

func decodeEmbeddingSelection(raw []byte) (map[string]interface{}, error) {
	return decodeInputOnlySelection(raw, false)
}

func decodeRetrievalSelection(raw []byte) (map[string]interface{}, error) {
	return decodeInputOnlySelection(raw, true)
}

func RetrievalSelectionDigest(raw []byte) (string, error) {
	return selectionDigest(raw, decodeRetrievalSelection)
}

// EmbeddingSelectionDigest uses the same mf-json selection projection under the
// new native:v2 schema. It never temporarily relabels an embedding as text.
func EmbeddingSelectionDigest(raw []byte) (string, error) {
	return selectionDigest(raw, decodeEmbeddingSelection)
}

func CanonicalEmbeddingSelection(raw []byte) ([]byte, error) {
	return canonicalSelection(raw, decodeEmbeddingSelection)
}
