package mediaexecution

import (
	"encoding/json"
	"math/big"
	"net/url"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

type nativeImageChoice struct {
	FinishReason string `json:"finish_reason"`
	Message      struct {
		Content []struct {
			Image string `json:"image"`
			Code  string `json:"code"`
		} `json:"content"`
	} `json:"message"`
}

func (p profileDefinition) unit() string {
	if p.operation == "video_generate" {
		return "millisecond"
	}
	return "image"
}

func outputReference(position int, rawURL, code string) OutputEvidence {
	o := OutputEvidence{Position: position}
	if identity(code) {
		o.ProviderError = code
	}
	u, err := url.Parse(rawURL)
	if err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil {
		o.URL, o.Successful = rawURL, code == ""
	}
	return o
}

func (r Request) observeSynchronous(response Response, o Observation) Observation {
	var wire struct {
		RequestID string `json:"request_id"`
		Code      string `json:"code"`
		Error     struct {
			Code json.RawMessage `json:"code"`
		} `json:"error"`
		Data []struct {
			URL  string `json:"url"`
			Code string `json:"code"`
		} `json:"data"`
		Output struct {
			Choices []nativeImageChoice `json:"choices"`
		} `json:"output"`
		Usage map[string]json.RawMessage `json:"usage"`
	}
	if decode(response.Body, &wire) != nil {
		o.ReasonCode = "media_response_invalid"
		return o
	}
	if r.profile.compatible {
		// This protocol supplies request identity in the actual header, never
		// a native-looking body field or the platform request identifier.
		var values []string
		for _, header := range response.Headers {
			if strings.EqualFold(header[0], "x-request-id") {
				values = append(values, header[1])
			}
		}
		if len(values) == 1 {
			o.captureProviderRequestID(values[0])
		}
	} else {
		o.captureProviderRequestID(wire.RequestID)
	}
	code := wire.Code
	if r.profile.compatible {
		_ = json.Unmarshal(wire.Error.Code, &code)
	}
	status := *response.HTTPStatus
	if code != "" {
		if identity(code) {
			o.ActualAttempt.ProviderError = code
		}
		if status == 400 || status == 401 || status == 403 || status == 404 || status == 405 || status == 413 || status == 415 || status == 422 {
			o.GenerationAcceptance, o.GenerationResult, o.DeliveryState, o.Terminal = "rejected_before_acceptance", "failed", "failed", true
			o.ActualAttempt.Acceptance, o.ActualAttempt.ExecutionResult = "rejected_before_acceptance", "failed"
			o.ReasonCode = "media_provider_rejection"
		}
		return o
	}
	if status != 200 {
		o.ReasonCode = "media_response_unsupported"
		return o
	}
	var successes, failures int
	if r.profile.compatible {
		for _, result := range wire.Data {
			evidence := outputReference(len(o.Outputs), result.URL, result.Code)
			o.Outputs = append(o.Outputs, evidence)
			if evidence.Successful {
				successes++
			} else {
				failures++
			}
		}
	} else {
		for _, choice := range wire.Output.Choices {
			for _, content := range choice.Message.Content {
				evidence := outputReference(len(o.Outputs), content.Image, content.Code)
				o.Outputs = append(o.Outputs, evidence)
				if evidence.Successful {
					successes++
				} else {
					failures++
				}
			}
		}
	}
	o.GenerationUsage, o.AggregateSpecification = r.profile.usage(wire.Usage)
	if successes > 0 {
		o.GenerationAcceptance, o.GenerationResult, o.DeliveryState, o.Terminal = "accepted", "succeeded", "succeeded", true
		if failures > 0 {
			o.GenerationResult, o.DeliveryState = "partial", "partial"
		}
		o.ActualAttempt.Acceptance, o.ActualAttempt.ExecutionResult = o.GenerationAcceptance, o.GenerationResult
		o.ActualAttempt.Usage = o.GenerationUsage
	}
	// Returned URL positions cannot safely establish a stable per-output billing
	// key. Preserve real aggregates and references without inventing such keys.
	o.ReasonCode = "media_per_output_measurement_pending"
	return o
}

func (p profileDefinition) usage(values map[string]json.RawMessage) (*executionfacts.Usage, map[string]string) {
	missing := &executionfacts.Usage{Status: "missing", Unit: p.unit(), Source: "none", ReasonCode: "media_usage_missing"}
	quantities, specifications := make(map[string]string), make(map[string]string)
	if p.operation == "video_generate" {
		for _, pair := range [][2]string{{"input_video_duration", "input_video_duration_ms"}, {"output_video_duration", "output_video_duration_ms"}} {
			if raw, present := values[pair[0]]; present {
				value, ok := milliseconds(raw)
				if !ok {
					return missing, specifications
				}
				quantities[pair[1]] = value
			}
		}
		if raw, present := values["video_count"]; present {
			value, ok := integer(raw)
			if !ok {
				return missing, specifications
			}
			quantities["video_count"] = value
		}
		// The provider's duration can include input reference video. It is never
		// substituted for missing output_video_duration, even for T2V requests.
		for _, key := range []string{"fps", "SR"} {
			if value, ok := integer(values[key]); ok && value != "0" {
				specifications[key] = value
			}
		}
		if sr := specifications["SR"]; sr != "" {
			specifications["resolution"] = sr + "p"
		}
		if value := rawText(values["ratio"]); identity(value) {
			specifications["ratio"] = value
		}
		if quantities["output_video_duration_ms"] == "" {
			return missing, specifications
		}
	} else {
		for _, key := range []string{"image_count", "input_image_count", "output_image_count"} {
			if raw, present := values[key]; present {
				value, ok := integer(raw)
				if !ok {
					return missing, specifications
				}
				quantities[key] = value
			}
		}
		if count, present := quantities["image_count"]; present && quantities["output_image_count"] != "" && quantities["output_image_count"] != count {
			return missing, specifications
		}
		for _, key := range []string{"width", "height", "output_width", "output_height"} {
			if value, ok := integer(values[key]); ok && value != "0" {
				specifications[key] = value
			}
		}
		for _, key := range []string{"input_image_type", "output_image_type"} {
			if value := rawText(values[key]); identity(value) {
				specifications[key] = value
			}
		}
		for _, pair := range [][2]string{{"output_width", "width"}, {"output_height", "height"}} {
			if specifications[pair[1]] == "" && specifications[pair[0]] != "" {
				specifications[pair[1]] = specifications[pair[0]]
			}
		}
	}
	if len(quantities) == 0 {
		return missing, specifications
	}
	return &executionfacts.Usage{Status: "reported", Unit: p.unit(), Source: "provider", Quantities: quantities}, specifications
}

func rawText(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

// The official video wire uses decimal seconds. Accept only quantities exactly
// representable as milliseconds; no float, truncation, or requested duration is
// allowed to manufacture a physical output measurement.
func milliseconds(raw json.RawMessage) (string, bool) {
	value, ok := new(big.Rat).SetString(string(raw))
	if !ok || value.Sign() < 0 {
		return "", false
	}
	value.Mul(value, big.NewRat(1000, 1))
	if !value.IsInt() || value.Num().BitLen() > 128 {
		return "", false
	}
	return value.Num().String(), true
}
