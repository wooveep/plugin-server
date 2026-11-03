package main

import (
	"encoding/json"
	"math/big"
	"strconv"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/tokenestimate"
)

// availableManagedUsage fills only unavailable text usage after an observed send.
// A failed response still consumed its available input; absent output costs zero.
func availableManagedUsage(s *managedExecutionState, outcome executionfacts.AttemptOutcome, body []byte) *executionfacts.Usage {
	if outcome.SendState != "observed_sent" || outcome.Usage != nil && outcome.Usage.Status != "missing" {
		return outcome.Usage
	}
	model := s.config.release.Descriptor.UpstreamModel
	if managedOperation(s.config) == executionfacts.NativeEmbeddingOperation {
		// Frozen internal v2/v3 facts retain their provider-only contract.
		if s.internal != nil {
			return outcome.Usage
		}
		usage, err := estimateManagedEmbeddingInput(model, s.embeddingInput)
		if err != nil {
			return outcome.Usage
		}
		return usage
	}
	if managedOperation(s.config) != "text_generate" {
		return outcome.Usage
	}
	input, err := tokenestimate.Count(model, s.requestText)
	if err != nil {
		return outcome.Usage
	}
	output, err := tokenestimate.Count(model, tokenestimate.ResponseText(body))
	if err != nil {
		return outcome.Usage
	}
	quantities := map[string]string{"input": strconv.FormatInt(input, 10), "output": strconv.FormatInt(output, 10)}
	for _, document := range tokenestimate.Documents(body) {
		for meter, paths := range map[string][]string{
			"input":     {"usage.prompt_tokens", "usage.input_tokens", "message.usage.input_tokens", "usageMetadata.promptTokenCount"},
			"output":    {"usage.completion_tokens", "usage.output_tokens", "usageMetadata.candidatesTokenCount"},
			"cache_hit": {"usage.prompt_tokens_details.cached_tokens", "usage.prompt_cache_hit_tokens", "usage.cache_read_input_tokens", "usageMetadata.cachedContentTokenCount"},
		} {
			for _, path := range paths {
				if count, ok := managedCount(json.RawMessage(document.Get(path).Raw)); ok {
					quantities[meter] = count.String()
					break
				}
			}
		}
	}
	if hit, ok := quantities["cache_hit"]; ok {
		inputCount, _ := new(big.Int).SetString(quantities["input"], 10)
		hitCount, _ := new(big.Int).SetString(hit, 10)
		if hitCount.Cmp(inputCount) <= 0 {
			quantities["cache_miss"] = new(big.Int).Sub(inputCount, hitCount).String()
		} else {
			delete(quantities, "cache_hit")
		}
	}
	return &executionfacts.Usage{Status: "estimated", Unit: "token", Source: "estimator", Quantities: quantities,
		EstimatorVersion: "tiktoken-go.v0.7.0/" + string(tokenestimate.EncodingForModel(model)) + "/available-text.v2"}
}
