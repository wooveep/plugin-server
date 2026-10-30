package mediaexecution

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
)

const (
	NativeImageChoicesContent = "native_image_choices_content_v1"
	CompatibleImageData       = "compatible_image_data_v1"
	TaskSingleVideo           = "task_single_video_v1"
)

// These types mirror the existing trusted MEDIA/price_revisions port. They
// carry actual measurements and references, without rates or wallet arithmetic.
type MeasuredQuantity struct {
	Unit     string `json:"unit"`
	Quantity string `json:"quantity"`
}
type UsageOutput struct {
	OutputID      string                      `json:"output_id"`
	Successful    bool                        `json:"successful"`
	Specification map[string]string           `json:"specification"`
	Metrics       map[string]MeasuredQuantity `json:"metrics"`
}
type OutputReference struct {
	OutputID  string  `json:"output_id"`
	URL       string  `json:"url"`
	ExpiresAt *string `json:"expires_at"`
}
type OutputPosition struct {
	Position []uint32 `json:"position"`
	OutputID string   `json:"output_id"`
}

// OutputManifest freezes the first complete terminal provider shape. URL
// signatures, usage arrival time and query attempts do not enter its identity.
type OutputManifest struct {
	SchemaVersion          string           `json:"schema_version"`
	AttemptID              string           `json:"attempt_id"`
	ProviderTaskID         *string          `json:"provider_task_id"`
	ResponseClassifierRef  string           `json:"response_classifier_ref"`
	OutputIdentityContract string           `json:"output_identity_contract"`
	Positions              []OutputPosition `json:"positions"`
	StructuralCounts       []uint32         `json:"structural_counts"`
}
type OutputProjection struct {
	Outputs    []UsageOutput
	References []OutputReference
	Manifest   *OutputManifest
	Usage      *executionfacts.Usage
}

func (p profileDefinition) outputIdentityContract() string {
	if p.operation == "video_generate" {
		return TaskSingleVideo
	}
	if p.compatible {
		return CompatibleImageData
	}
	return NativeImageChoicesContent
}

// Project uses the explicitly frozen adapter identity contract. A prior
// terminal manifest remains authoritative when signed URLs or usage change.
// Unknown, truncated and progress responses cannot freeze positional identity.
func (r Request) Project(response Response, observation Observation, previous *OutputManifest) (OutputProjection, error) {
	projection := OutputProjection{Outputs: []UsageOutput{}, References: []OutputReference{}}
	if !observation.Terminal || !response.HeadersComplete || !response.BodyObserved || !response.BodyComplete {
		return projection, nil
	}
	if observation.GenerationAcceptance == "not_sent" {
		return projection, nil
	}
	if r.outputContract == "" || r.outputContract != r.profile.outputIdentityContract() {
		return projection, ErrUnsupported
	}
	if observation.GenerationAcceptance == "rejected_before_acceptance" {
		projection.Manifest = &OutputManifest{SchemaVersion: "media-output-manifest.v1", AttemptID: r.createAttemptID,
			ResponseClassifierRef: r.createClassifier, OutputIdentityContract: r.outputContract, Positions: []OutputPosition{}, StructuralCounts: []uint32{}}
		return projection, nil
	}
	var task *string
	if r.profile.asynchronous {
		if !taskID(observation.ProviderTaskID) || (r.query && observation.ProviderTaskID != r.taskID) {
			return projection, ErrIdentity
		}
		id := observation.ProviderTaskID
		task = &id
	}
	manifest := OutputManifest{SchemaVersion: "media-output-manifest.v1", AttemptID: r.createAttemptID, ProviderTaskID: task,
		ResponseClassifierRef: r.createClassifier, OutputIdentityContract: r.outputContract, Positions: []OutputPosition{}, StructuralCounts: []uint32{}}
	root, err := object(response.Body)
	if err != nil {
		return projection, err
	}
	add := func(position []uint32, evidence OutputEvidence) error {
		if len(manifest.Positions) >= 4096 {
			return ErrCapacity
		}
		identityBytes, err := json.Marshal(struct {
			AttemptID  string   `json:"attempt_id"`
			TaskID     *string  `json:"provider_task_id,omitempty"`
			Classifier string   `json:"response_classifier_ref"`
			Contract   string   `json:"output_identity_contract"`
			Position   []uint32 `json:"position"`
		}{r.createAttemptID, task, r.createClassifier, r.outputContract, position})
		if err != nil {
			return err
		}
		digest, err := mfjson.Digest(identityBytes)
		if err != nil {
			return err
		}
		id := "media-output:" + strings.TrimPrefix(digest, "sha256:")
		manifest.Positions = append(manifest.Positions, OutputPosition{Position: position, OutputID: id})
		output := UsageOutput{OutputID: id, Successful: evidence.Successful, Specification: make(map[string]string), Metrics: make(map[string]MeasuredQuantity)}
		for key, value := range observation.AggregateSpecification {
			output.Specification[key] = value
		}
		if evidence.Successful {
			if r.profile.operation == "image_generate" {
				output.Metrics["image_output_count"] = MeasuredQuantity{Unit: "image", Quantity: "1"}
			} else if duration, measured := observation.GenerationUsage.Quantities["output_video_duration_ms"]; measured {
				output.Metrics["video_output_duration"] = MeasuredQuantity{Unit: "millisecond", Quantity: duration}
			}
			projection.References = append(projection.References, OutputReference{OutputID: id, URL: evidence.URL})
		}
		projection.Outputs = append(projection.Outputs, output)
		return nil
	}
	if r.profile.operation == "video_generate" {
		output, err := object(root["output"])
		if err != nil {
			return projection, err
		}
		if videoURL := field(output, "video_url"); videoURL != "" {
			evidence := outputReference(0, videoURL, field(output, "code"))
			if !evidence.Successful {
				return projection, ErrIdentity
			}
			manifest.StructuralCounts = []uint32{1}
			if err := add([]uint32{0}, evidence); err != nil {
				return projection, err
			}
		} else if observation.GenerationResult != "failed" {
			return projection, ErrIdentity
		}
	} else if r.profile.compatible {
		var data []map[string]json.RawMessage
		if decode(root["data"], &data) != nil || len(data) == 0 {
			return projection, ErrIdentity
		}
		manifest.StructuralCounts = []uint32{uint32(len(data))}
		for index, entry := range data {
			for key := range entry {
				if key != "url" && key != "code" && key != "message" && key != "revised_prompt" {
					return projection, ErrUnsupported
				}
			}
			evidence := outputReference(index, field(entry, "url"), field(entry, "code"))
			if !evidence.Successful && evidence.ProviderError == "" {
				return projection, ErrIdentity
			}
			if err := add([]uint32{uint32(index)}, evidence); err != nil {
				return projection, err
			}
		}
	} else {
		output, err := object(root["output"])
		if err != nil {
			return projection, err
		}
		var choices []map[string]json.RawMessage
		if output["choices"] == nil && observation.GenerationResult == "failed" {
			choices = []map[string]json.RawMessage{}
		} else if decode(output["choices"], &choices) != nil || len(choices) == 0 {
			return projection, ErrIdentity
		}
		if len(choices) > 0 {
			manifest.StructuralCounts = append(manifest.StructuralCounts, uint32(len(choices)))
		}
		for choiceIndex, choice := range choices {
			message, err := object(choice["message"])
			if err != nil {
				return projection, err
			}
			var content []map[string]json.RawMessage
			if decode(message["content"], &content) != nil || len(content) == 0 {
				return projection, ErrIdentity
			}
			manifest.StructuralCounts = append(manifest.StructuralCounts, uint32(len(content)))
			for contentIndex, entry := range content {
				for key := range entry {
					if key != "image" && key != "code" && key != "message" && key != "type" {
						return projection, ErrUnsupported
					}
				}
				if entry["type"] != nil && field(entry, "type") != "image" {
					return projection, ErrUnsupported
				}
				evidence := outputReference(contentIndex, field(entry, "image"), field(entry, "code"))
				if !evidence.Successful && evidence.ProviderError == "" {
					return projection, ErrIdentity
				}
				if err := add([]uint32{uint32(choiceIndex), uint32(contentIndex)}, evidence); err != nil {
					return projection, err
				}
			}
		}
	}
	projection.Manifest = &manifest
	if r.profile.operation == "image_generate" {
		successes := 0
		for _, output := range projection.Outputs {
			if output.Successful {
				successes++
			}
		}
		for _, key := range []string{"image_count", "output_image_count"} {
			if count, known := observation.GenerationUsage.Quantities[key]; known && count != strconv.Itoa(successes) {
				return projection, ErrIdentity
			}
		}
		if successes > 0 {
			source := "meter"
			if observation.GenerationUsage.Status == "reported" {
				source = "provider"
			}
			projection.Usage = &executionfacts.Usage{Status: "reported", Unit: "image", Source: source, Quantities: map[string]string{"image_output_count": strconv.Itoa(successes)}}
		}
	} else if duration, measured := observation.GenerationUsage.Quantities["output_video_duration_ms"]; measured {
		projection.Usage = &executionfacts.Usage{Status: "reported", Unit: "millisecond", Source: "provider", Quantities: map[string]string{"video_output_duration": duration}}
	}
	if previous != nil {
		old, _ := json.Marshal(previous)
		current, _ := json.Marshal(manifest)
		left, leftErr := mfjson.Digest(old)
		right, rightErr := mfjson.Digest(current)
		if leftErr != nil || rightErr != nil || left != right {
			return projection, ErrIdentity
		}
	}
	r.estimateWanOutput(root["usage"], &projection)
	return projection, nil
}
