// Package executionfacts freezes original execution observations for IF-03.
// Encoding never authenticates a producer or creates historical execution proof.
package executionfacts

import "encoding/json"

// Envelope mirrors Console billing_facts::ObservationEnvelope, billing-facts.v1.
// It is an input value. Freeze copies all referenced data before returning.
// Counts and revisions are exact decimal strings, never floating-point values.
type Envelope struct {
	SchemaVersion           string     `json:"schema_version"`
	ProducerID              string     `json:"producer_id"`
	EventID                 string     `json:"event_id"`
	SourceFactID            string     `json:"source_fact_id"`
	SourceRevision          string     `json:"source_revision"`
	SupersedesEventID       string     `json:"supersedes_event_id,omitempty"`
	PayloadDigest           string     `json:"payload_digest"`
	PayloadCanonicalization string     `json:"payload_canonicalization"`
	Kind                    string     `json:"kind"`
	CallID                  string     `json:"call_id"`
	OperationID             string     `json:"operation_id,omitempty"`
	AttemptID               string     `json:"attempt_id,omitempty"`
	ReleaseID               string     `json:"release_id,omitempty"`
	RouteID                 string     `json:"route_id,omitempty"`
	SalesAssetID            string     `json:"sales_asset_id,omitempty"`
	TenantID                string     `json:"tenant_id,omitempty"`
	PrincipalID             string     `json:"principal_id,omitempty"`
	AccountID               string     `json:"account_id,omitempty"`
	PrincipalKind           string     `json:"principal_kind"`
	BillingMode             string     `json:"billing_mode"`
	CostOwner               *CostOwner `json:"cost_owner,omitempty"`
	OperationKind           string     `json:"operation_kind"`
	OccurredAt              string     `json:"occurred_at"`
	ObservedAt              string     `json:"observed_at"`
	RequestID               string     `json:"request_id,omitempty"`
	TraceID                 string     `json:"trace_id,omitempty"`
	EvidenceRefs            []string   `json:"evidence_refs"`
	TargetID                string     `json:"target_id,omitempty"`
	BindingID               string     `json:"binding_id,omitempty"`
	ProviderID              string     `json:"provider_id,omitempty"`
	ProviderAccountID       string     `json:"provider_account_id,omitempty"`
	ParentAttemptID         string     `json:"parent_attempt_id,omitempty"`
	CredentialRef           string     `json:"credential_ref,omitempty"`
	CredentialRevision      string     `json:"credential_revision,omitempty"`
	ProviderTaskID          string     `json:"provider_task_id,omitempty"`
	DeliveryState           string     `json:"delivery_state,omitempty"`
	ServedFrom              string     `json:"served_from,omitempty"`
	PublicModel             string     `json:"public_model,omitempty"`
	ResponseModel           string     `json:"response_model,omitempty"`
	Usage                   *Usage     `json:"usage,omitempty"`
	ErrorClass              string     `json:"error_class,omitempty"`
	SelectionDigest         string     `json:"selection_digest,omitempty"`
	SendTransport           string     `json:"send_transport,omitempty"`
	RecordingOwner          string     `json:"recording_owner,omitempty"`
	Mechanism               string     `json:"mechanism,omitempty"`
	SendState               string     `json:"send_state,omitempty"`
	Acceptance              string     `json:"acceptance,omitempty"`
	ExecutionResult         string     `json:"execution_result,omitempty"`
	HTTPStatus              *uint16    `json:"http_status,omitempty"`
	HeadersComplete         *bool      `json:"headers_complete,omitempty"`
	BodyComplete            *bool      `json:"body_complete,omitempty"`
	TransportError          string     `json:"transport_error,omitempty"`
	ProviderError           string     `json:"provider_error,omitempty"`
	OutboundBodyDigest      string     `json:"outbound_body_digest,omitempty"`
	ResponseBodyDigest      string     `json:"response_body_digest,omitempty"`
	RetryAfter              string     `json:"retry_after,omitempty"`
	UpstreamModel           string     `json:"upstream_model,omitempty"`
}

// CostOwner is an original registered internal job owner, not a customer wallet.
type CostOwner struct {
	Kind        string `json:"kind"`
	OwnerID     string `json:"owner_id"`
	SourceJobID string `json:"source_job_id"`
}

// Usage preserves the evidence status of this attempt or final delivery only.
type Usage struct {
	Status           string            `json:"status"`
	Unit             string            `json:"unit"`
	Quantities       map[string]string `json:"quantities,omitempty"`
	Source           string            `json:"source"`
	EstimatorVersion string            `json:"estimator_version,omitempty"`
	ReasonCode       string            `json:"reason_code,omitempty"`
}

// MissingTokenUsage explicitly records unavailable evidence without fake zeroes.
func MissingTokenUsage(reason string) *Usage {
	return &Usage{Status: "missing", Unit: "token", Source: "none", ReasonCode: reason}
}

// Attempt is the private original selection captured before a specific send.
// Outcome observations must be supplied independently for each Attempt.
type Attempt struct{ original Envelope }

// AttemptOutcome is supplied by the unique send observer, not inferred by codec.
// HTTPStatus stays nil when no upstream status was actually observed.
type AttemptOutcome struct {
	ObservedAt         string
	SendState          string
	Acceptance         string
	ExecutionResult    string
	HTTPStatus         *uint16
	HeadersComplete    bool
	BodyComplete       bool
	Usage              *Usage
	EvidenceRefs       []string
	TransportError     string
	ProviderError      string
	ErrorClass         string
	ProviderTaskID     string
	ResponseBodyDigest string
	ResponseModel      string
	RetryAfter         string
}

// NewAttempt freezes the exact selection and identity before handing off a send.
// It neither allocates a second purchase attempt nor claims the upstream sent.
func NewAttempt(original Envelope) (Attempt, error) {
	original.Kind = "upstream_attempt"
	if original.SchemaVersion != "" && original.SchemaVersion != SchemaVersion {
		return Attempt{}, errorsForVersion()
	}
	if original.PayloadCanonicalization != "" && original.PayloadCanonicalization != Canonicalization {
		return Attempt{}, errorsForVersion()
	}
	original.SchemaVersion = SchemaVersion
	original.PayloadCanonicalization = Canonicalization
	if err := validateIdentity(original); err != nil {
		return Attempt{}, err
	}
	if err := validateAttemptIdentity(original); err != nil {
		return Attempt{}, err
	}
	copied, err := cloneEnvelope(original)
	if err != nil {
		return Attempt{}, err
	}
	return Attempt{original: copied}, nil
}

// Finish captures this attempt's own result without consulting a final context.
func (a Attempt) Finish(outcome AttemptOutcome) (Snapshot, error) {
	return finishAttempt(a.original, outcome, Freeze)
}

func finishAttempt(original Envelope, outcome AttemptOutcome, freeze func(Envelope) (Snapshot, error)) (Snapshot, error) {
	e, err := cloneEnvelope(original)
	if err != nil {
		return Snapshot{}, err
	}
	e.ObservedAt = outcome.ObservedAt
	e.SendState = outcome.SendState
	e.Acceptance = outcome.Acceptance
	e.ExecutionResult = outcome.ExecutionResult
	e.HTTPStatus = outcome.HTTPStatus
	e.HeadersComplete = &outcome.HeadersComplete
	e.BodyComplete = &outcome.BodyComplete
	e.Usage = outcome.Usage
	if e.Usage == nil {
		e.Usage = MissingTokenUsage("usage_not_observed")
	}
	e.EvidenceRefs = outcome.EvidenceRefs
	e.TransportError = outcome.TransportError
	e.ProviderError = outcome.ProviderError
	e.ErrorClass = outcome.ErrorClass
	e.ProviderTaskID = outcome.ProviderTaskID
	e.ResponseBodyDigest = outcome.ResponseBodyDigest
	e.ResponseModel = outcome.ResponseModel
	e.RetryAfter = outcome.RetryAfter
	return freeze(e)
}

// Registration is a separate original-execution proposal. Console must verify
// its configured stream grant, key/wallet binding and immutable release target
// before creating HistoricalExecution. It is never authority from an observation.
type Registration struct {
	SchemaVersion        string          `json:"schema_version"`
	ProducerID           string          `json:"producer_id"`
	RegistrationID       string          `json:"registration_id"`
	CallID               string          `json:"call_id"`
	AttemptID            string          `json:"attempt_id"`
	ReleaseID            string          `json:"release_id"`
	RouteID              string          `json:"route_id"`
	TargetID             string          `json:"target_id"`
	AccountID            string          `json:"account_id"`
	APIKeyID             string          `json:"api_key_id"`
	CredentialGeneration string          `json:"credential_generation"`
	BusinessOccurredAt   string          `json:"business_occurred_at"`
	Selection            json.RawMessage `json:"selection"`
}
