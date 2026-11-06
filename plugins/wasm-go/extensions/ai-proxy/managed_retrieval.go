package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/tidwall/gjson"
)

func validateManagedRetrieval(configuration *managedExecutionConfig) error {
	d := configuration.release.Descriptor
	s := configuration.settings
	operation, path, adapter := executionfacts.RetrievalProtocol(s.InputProtocol)
	capability, exists := d.Capabilities[operation]
	if !exists || len(d.Capabilities) != 1 || s.CallIdentitySource != executionfacts.CallIdentitySourceAdmission ||
		s.InputProtocol != s.UpstreamProtocol || s.UpstreamPath != path || d.AdapterRef != adapter || d.Transport.PathAdapterRef != adapter ||
		s.TextExecution != nil || s.ToolPolicy != nil || s.MediaDispatchMode == "managed_operation" || s.UpstreamHealth == nil ||
		capability.ResponseClassifierRef != executionfacts.RetrievalClassifier || capability.SendMode != "single_send" ||
		capability.MaxAttempts != 1 || capability.MaxFallbackHops != 0 || capability.QueryCapabilityRef != "" || capability.ProviderIdempotencyRef != "" ||
		!managedRetrievalModel(s.InputProtocol, d.UpstreamModel) {
		return errors.New("retrieval requires its exact customer single-send supplier protocol")
	}
	return nil
}

func managedRetrievalModel(protocol, model string) bool {
	switch protocol {
	case "dashscope_multimodal_embedding":
		return model == "qwen3-vl-embedding" || model == "qwen2.5-vl-embedding" ||
			model == "multimodal-embedding-v1" || strings.HasPrefix(model, "tongyi-embedding-vision-")
	case "dashscope_rerank":
		return model == "qwen3-rerank" || model == "qwen3.7-text-rerank" || model == "qwen3-vl-rerank" ||
			model == "gte-rerank" || model == "gte-rerank-v2"
	case "bailian_rerank":
		return model == "qwen3-rerank" || model == "qwen3.7-text-rerank"
	default:
		return false
	}
}

func prepareManagedRetrievalRequest(body []byte, model, credential string) (managedProtocolRequest, error) {
	var object map[string]json.RawMessage
	var actualModel string
	if decodeManagedJSON(body, &object, false) != nil || json.Unmarshal(object["model"], &actualModel) != nil ||
		actualModel != model || !validManagedCredentialValue(credential) {
		return managedProtocolRequest{}, errors.New("retrieval request differs from the frozen model or credential")
	}
	return managedProtocolRequest{
		Body:    bytes.Clone(body),
		Headers: [][2]string{{"content-type", "application/json"}, {"accept", "application/json"}, {"accept-encoding", "identity"}, {"authorization", "Bearer " + credential}},
	}, nil
}

func managedRetrievalObservation(protocol, model string, status uint16, headers [][2]string, body []byte, complete bool) managedProtocolObservation {
	o := managedProtocolObservation{Usage: executionfacts.MissingTokenUsage("provider_usage_not_observed"), ExecutionResult: "unknown", Acceptance: "unknown", BodyComplete: complete}
	if status >= 400 {
		o.ExecutionResult = "failed"
	}
	stream, err := managedProtocolContentType(headers)
	if !complete || err != nil || stream {
		return o
	}
	var object, usage map[string]json.RawMessage
	if decodeManagedJSON(body, &object, false) != nil || object == nil {
		return o
	}
	root := gjson.ParseBytes(body)
	code := root.Get("code").String()
	if (root.Get("error").Exists() && root.Get("error").Type != gjson.Null) || code != "" && code != "200" {
		o.ProviderError, o.ExecutionResult = true, "failed"
	}
	if !managedProtocolMergeModel(&o.ResponseModel, object["model"]) {
		return o
	}
	if status >= 200 && status < 300 && !o.ProviderError {
		resultPath := "output.results"
		if protocol == "dashscope_multimodal_embedding" {
			resultPath = "output.embeddings"
		} else if protocol == "bailian_rerank" {
			resultPath = "results"
		}
		if results := root.Get(resultPath); results.IsArray() {
			o.ExecutionResult, o.Acceptance = "succeeded", "accepted"
		}
	}
	if json.Unmarshal(object["usage"], &usage) != nil || usage == nil {
		return o
	}
	quantities := make(map[string]string, 2)
	if protocol != "dashscope_multimodal_embedding" {
		input, valid := managedCount(usage["total_tokens"])
		if !valid {
			return o
		}
		if prompt, exists := usage["prompt_tokens"]; exists {
			count, valid := managedCount(prompt)
			if !valid || count.Cmp(input) != 0 {
				return o
			}
		}
		quantities["input"] = input.String()
	} else {
		input, valid := managedCount(usage["input_tokens"])
		if !valid {
			return o
		}
		if strings.HasPrefix(model, "tongyi-embedding-vision-") {
			var details map[string]json.RawMessage
			if raw, exists := usage["input_tokens_details"]; exists && json.Unmarshal(raw, &details) != nil {
				return o
			}
			if visualRaw, exists := details["image_tokens"]; exists {
				visual, valid := managedCount(visualRaw)
				if !valid || visual.Cmp(input) > 0 {
					return o
				}
				if textRaw, exists := details["text_tokens"]; exists {
					text, valid := managedCount(textRaw)
					if !valid || new(big.Int).Add(text, visual).Cmp(input) != 0 {
						return o
					}
				}
				quantities["visual_input"] = visual.String()
			}
		} else {
			if raw, exists := usage["image_tokens"]; exists {
				visual, valid := managedCount(raw)
				if !valid {
					return o
				}
				input.Add(input, visual)
				quantities["visual_input"] = visual.String()
			}
			if raw, exists := usage["total_tokens"]; exists {
				total, valid := managedCount(raw)
				if !valid || total.Cmp(input) != 0 {
					return o
				}
			}
		}
		if input.BitLen() > 128 {
			return o
		}
		quantities["input"] = input.String()
	}
	o.Usage = &executionfacts.Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: quantities}
	return o
}
