package mediaexecution

import (
	"encoding/json"
	"math/big"
	"net/url"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

// Response is evidence from the actual HTTP instrumentor. Status remains nil if
// no response headers were observed. BodyObserved distinguishes an observed empty
// body from no callback. Truncated bytes are hashed but never decoded as success.
type Response struct {
	HTTPStatus                                  *uint16
	HeadersComplete, BodyComplete, BodyObserved bool
	Body                                        []byte
	Headers                                     [][2]string
	SendState, TransportError, ObservedAt       string
}

// OutputEvidence is a returned native result, not a BILL UsageOutput. Position
// and signed URL are not stable output identities. Native results without such
// identity/actual per-output specifications remain pending rather than inventing
// a per-output minimum or using requested n/size as a physical measurement.
type OutputEvidence struct {
	Position           int
	URL, ProviderError string
	Successful         bool
}

func (o OutputEvidence) String() string { return "mediaexecution.OutputEvidence(redacted)" }

// Observation separates the real HTTP attempt from cumulative generation facts.
// A successful GET is not a second generated image or procurement ledger. Pass
// generation observations to the existing original-task/FACT authority, which
// owns ordering, late-result merge, source revision, and settlement policy.
type Observation struct {
	CallID, OperationID, CreateAttemptID, QueryAttemptID  string
	OriginalHistory                                       []byte
	ActualAttempt                                         executionfacts.AttemptOutcome
	ProviderTaskID, ProviderRequestID, TaskStatus         string
	GenerationAcceptance, GenerationResult, DeliveryState string
	Terminal                                              bool
	GenerationUsage                                       *executionfacts.Usage
	AggregateSpecification                                map[string]string
	Outputs                                               []OutputEvidence
	MeasurementsPending                                   bool
	ReasonCode                                            string
}

func (o Observation) String() string { return "mediaexecution.Observation(redacted)" }

// The supplier's actual protocol field is transport evidence, never a platform
// request ID or permission. Its digest also covers compatible header-only IDs.
func (o *Observation) captureProviderRequestID(value string) {
	if identity(value) {
		o.ProviderRequestID = value
		o.ActualAttempt.EvidenceRefs = append(o.ActualAttempt.EvidenceRefs, "provider-request-id:"+bodybasis.DigestBytes([]byte(value)))
	}
}

// Observe classifies only a prepared request and its actual observed response.
// It intentionally does not reject late evidence when the send grant has since
// expired. No response, malformed/truncated JSON, unknown status, and mismatched
// task identity preserve unknown. They never enable another create.
func (r Request) Observe(response Response) (Observation, error) {
	if !identity(r.attemptID) || !identity(r.createAttemptID) || !identity(r.operationID) {
		return Observation{}, ErrIdentity
	}
	if response.SendState != "not_sent" && response.SendState != "handed_off" && response.SendState != "observed_sent" && response.SendState != "unknown" {
		return Observation{}, ErrIdentity
	}
	if response.SendState == "not_sent" && (response.HTTPStatus != nil || response.BodyObserved) {
		return Observation{}, ErrIdentity
	}
	missing := func(reason string) *executionfacts.Usage {
		return &executionfacts.Usage{Status: "missing", Unit: r.profile.unit(), Source: "none", ReasonCode: reason}
	}
	o := Observation{CallID: r.callID, OperationID: r.operationID, CreateAttemptID: r.createAttemptID,
		OriginalHistory: r.OriginalHistory(), GenerationAcceptance: "unknown", GenerationResult: "unknown", DeliveryState: "unknown",
		GenerationUsage: missing("media_usage_not_observed"), MeasurementsPending: true, ReasonCode: "media_response_unknown"}
	if r.query {
		o.QueryAttemptID = r.attemptID
	}
	o.ActualAttempt = executionfacts.AttemptOutcome{ObservedAt: response.ObservedAt, SendState: response.SendState,
		Acceptance: "unknown", ExecutionResult: "unknown", HeadersComplete: response.HeadersComplete,
		BodyComplete: response.BodyComplete, Usage: missing("media_usage_not_observed"), TransportError: response.TransportError}
	if r.query {
		o.ActualAttempt.ProviderTaskID = r.taskID
	}
	if response.HTTPStatus != nil {
		if *response.HTTPStatus < 100 || *response.HTTPStatus > 599 {
			return Observation{}, ErrIdentity
		}
		status := *response.HTTPStatus
		o.ActualAttempt.HTTPStatus = &status
	}
	if response.BodyObserved {
		o.ActualAttempt.ResponseBodyDigest = bodybasis.DigestBytes(response.Body)
		o.ActualAttempt.EvidenceRefs = []string{"provider-response:" + o.ActualAttempt.ResponseBodyDigest}
	}
	if r.query {
		o.ActualAttempt.Usage = &executionfacts.Usage{Status: "not_applicable", Unit: r.profile.unit(), Source: "none", ReasonCode: "task_query_is_not_generation"}
	}
	if response.SendState == "not_sent" {
		o.ActualAttempt.Acceptance = "not_sent"
		o.ActualAttempt.ExecutionResult = "failed"
		if !r.query {
			o.GenerationAcceptance = "not_sent"
			o.GenerationResult, o.DeliveryState, o.Terminal = "failed", "failed", true
		}
		return o, nil
	}
	if response.HTTPStatus == nil || !response.HeadersComplete || !response.BodyObserved || !response.BodyComplete {
		return o, nil
	}
	if !r.profile.asynchronous {
		return r.observeSynchronous(response, o), nil
	}
	var wire struct {
		RequestID string `json:"request_id"`
		Code      string `json:"code"`
		Output    struct {
			TaskID   string              `json:"task_id"`
			Status   string              `json:"task_status"`
			Code     string              `json:"code"`
			VideoURL string              `json:"video_url"`
			Choices  []nativeImageChoice `json:"choices"`
			Results  []struct {
				URL  string `json:"url"`
				Code string `json:"code"`
			} `json:"results"`
			Metrics map[string]json.RawMessage `json:"task_metrics"`
		} `json:"output"`
		Usage map[string]json.RawMessage `json:"usage"`
	}
	if decode(response.Body, &wire) != nil {
		o.ReasonCode = "media_response_invalid"
		return o, nil
	}
	o.captureProviderRequestID(wire.RequestID)
	status := *response.HTTPStatus
	if wire.Code != "" {
		if identity(wire.Code) {
			o.ActualAttempt.ProviderError = wire.Code
		}
		// A complete explicit protocol rejection is different from a timeout/5xx.
		if wire.Output.TaskID == "" && (status == 400 || status == 401 || status == 403 || status == 404 || status == 405 || status == 413 || status == 415 || status == 422) {
			o.ActualAttempt.Acceptance, o.ActualAttempt.ExecutionResult = "rejected_before_acceptance", "failed"
			if !r.query {
				o.GenerationAcceptance, o.GenerationResult, o.DeliveryState, o.Terminal = "rejected_before_acceptance", "failed", "failed", true
			}
			o.ReasonCode = "media_provider_rejection"
		}
		return o, nil
	}
	if (r.query && status != 200) || (!r.query && status != 200 && status != 202) || !taskID(wire.Output.TaskID) {
		o.ReasonCode = "media_response_unsupported"
		return o, nil
	}
	if r.query && wire.Output.TaskID != r.taskID {
		o.ReasonCode = "media_provider_task_conflict"
		return o, nil
	}
	o.ProviderTaskID, o.TaskStatus = wire.Output.TaskID, wire.Output.Status
	if identity(wire.Output.Code) {
		o.ActualAttempt.ProviderError = wire.Output.Code
	}
	switch wire.Output.Status {
	case "PENDING", "RUNNING":
		o.GenerationAcceptance, o.GenerationResult = "accepted", "pending"
	case "SUCCEEDED":
		o.GenerationAcceptance, o.GenerationResult, o.Terminal = "accepted", "succeeded", true
	case "FAILED", "CANCELED":
		o.GenerationAcceptance, o.GenerationResult, o.Terminal = "accepted", "failed", true
	case "UNKNOWN":
		o.ReasonCode = "media_provider_task_unknown"
		return o, nil
	default:
		o.ReasonCode = "media_provider_status_unsupported"
		return o, nil
	}
	var successes, failures int
	for _, choice := range wire.Output.Choices {
		for _, content := range choice.Message.Content {
			evidence := outputReference(len(o.Outputs), content.Image, content.Code)
			if evidence.Successful {
				successes++
			} else {
				failures++
			}
			o.Outputs = append(o.Outputs, evidence)
		}
	}
	if r.profile.operation == "video_generate" && wire.Output.VideoURL != "" {
		evidence := outputReference(0, wire.Output.VideoURL, wire.Output.Code)
		o.Outputs = append(o.Outputs, evidence)
		if evidence.Successful {
			successes++
		} else {
			failures++
		}
	}
	for index, result := range wire.Output.Results {
		evidence := OutputEvidence{Position: index}
		if identity(result.Code) {
			evidence.ProviderError = result.Code
			failures++
		}
		u, err := url.Parse(result.URL)
		if err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil {
			evidence.URL = result.URL
			if result.Code == "" {
				evidence.Successful = true
				successes++
			}
		}
		o.Outputs = append(o.Outputs, evidence)
	}
	if failed, ok := integer(wire.Output.Metrics["FAILED"]); ok && failed != "0" {
		failures++
	}
	if o.Terminal {
		switch {
		case successes > 0 && (failures > 0 || wire.Output.Status != "SUCCEEDED"):
			o.GenerationResult, o.DeliveryState = "partial", "partial"
		case successes > 0:
			o.DeliveryState = "succeeded"
		case wire.Output.Status == "FAILED":
			o.DeliveryState = "failed"
		case wire.Output.Status == "CANCELED":
			o.DeliveryState = "cancelled"
		}
	}
	o.GenerationUsage, o.AggregateSpecification = r.profile.usage(wire.Usage)
	// This native result shape provides URL/code, not a stable per-output key or
	// per-output dimensions. Preserve provider aggregates without synthesizing IDs.
	o.ReasonCode = "media_per_output_measurement_pending"
	o.ActualAttempt.Acceptance, o.ActualAttempt.ExecutionResult = o.GenerationAcceptance, o.GenerationResult
	o.ActualAttempt.ProviderTaskID = o.ProviderTaskID
	if r.query {
		o.ActualAttempt.ExecutionResult = "succeeded" // The read succeeded; generation can still be pending.
	} else {
		o.ActualAttempt.Usage = o.GenerationUsage
	}
	return o, nil
}

func integer(raw json.RawMessage) (string, bool) {
	text := string(raw)
	if text == "" || strings.IndexFunc(text, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return "", false
	}
	value, ok := new(big.Int).SetString(text, 10)
	if !ok || value.Sign() < 0 || value.BitLen() > 128 {
		return "", false
	}
	return value.String(), true
}
