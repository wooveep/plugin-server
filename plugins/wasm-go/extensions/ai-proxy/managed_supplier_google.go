package main

import (
	"bytes"
	"encoding/json"
	"math/big"
	"strconv"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

// Observe Google's original counters before the SDK's content conversion.
// Both reasoning output and tool-use prompts are separately reported tokens:
// https://ai.google.dev/api/generate-content#UsageMetadata
func managedGoogleUsage(raw json.RawMessage) map[string]json.RawMessage {
	var native map[string]json.RawMessage
	if json.Unmarshal(raw, &native) != nil || native == nil {
		return nil
	}
	wire := map[string]json.RawMessage{
		"prompt_tokens": native["promptTokenCount"], "completion_tokens": native["candidatesTokenCount"],
	}
	if total, exists := native["totalTokenCount"]; exists {
		wire["total_tokens"] = total
	}
	for source, destination := range map[string]string{"toolUsePromptTokenCount": "prompt_tokens", "thoughtsTokenCount": "completion_tokens"} {
		if additional, exists := native[source]; exists {
			base, baseOK := managedCount(wire[destination])
			extra, extraOK := managedCount(additional)
			if !baseOK || !extraOK {
				wire[destination] = json.RawMessage("null")
			} else {
				wire[destination] = json.RawMessage(new(big.Int).Add(base, extra).String())
			}
		}
	}
	if cached, exists := native["cachedContentTokenCount"]; exists {
		wire["prompt_tokens_details"], _ = json.Marshal(map[string]json.RawMessage{"cached_tokens": cached})
	}
	return wire
}

func managedGoogleObservation(protocol string, status uint16, headers [][2]string, body []byte, complete bool, expectedEmbeddings int) managedProtocolObservation {
	observation := managedProtocolObservation{Usage: executionfacts.MissingTokenUsage("provider_usage_not_observed"), ExecutionResult: "unknown", Acceptance: "unknown", BodyComplete: complete}
	if status >= 400 {
		observation.ExecutionResult = "failed"
	}
	stream, err := managedProtocolContentType(headers)
	if !complete || err != nil {
		return observation
	}
	events := []managedProtocolEvent{{Data: body}}
	if stream {
		events, err = managedProtocolEvents(body)
		if err != nil || protocol == managedProtocolEmbeddings {
			observation.BodyComplete = false
			return observation
		}
	}
	finished := make(map[string]string)
	for _, event := range events {
		var object map[string]json.RawMessage
		if decodeManagedJSON(event.Data, &object, false) != nil || object == nil {
			observation.BodyComplete = false
			observation.Usage = executionfacts.MissingTokenUsage("invalid_native_google_response")
			return observation
		}
		observation.ProviderError = observation.ProviderError || event.Name == "error"
		if providerError, exists := object["error"]; exists && !managedProtocolNull(providerError) {
			observation.ProviderError = true
		}
		managedProtocolMergeModel(&observation.ResponseModel, object["modelVersion"])
		if raw, exists := object["usageMetadata"]; exists {
			wire, _ := json.Marshal(managedGoogleUsage(raw))
			observation.Usage = managedOpenAIUsage(map[string]json.RawMessage{"usage": wire})
		}
		if protocol == managedProtocolEmbeddings {
			var embeddings []struct {
				Values json.RawMessage `json:"values"`
			}
			if json.Unmarshal(object["embeddings"], &embeddings) != nil {
				return observation
			}
			data := make([]map[string]json.RawMessage, 0, len(embeddings))
			for index, embedding := range embeddings {
				data = append(data, map[string]json.RawMessage{"index": json.RawMessage(strconv.Itoa(index)), "embedding": embedding.Values})
			}
			wire, _ := json.Marshal(managedGoogleUsage(object["usageMetadata"]))
			encoded, _ := json.Marshal(data)
			view, _ := json.Marshal(map[string]json.RawMessage{"usage": wire, "data": encoded})
			observed := managedEmbeddingObservation(status, headers, view, complete, expectedEmbeddings)
			observed.ResponseModel = observation.ResponseModel
			if observation.ProviderError {
				observed.ProviderError, observed.ExecutionResult, observed.Acceptance = true, "failed", "unknown"
			}
			return observed
		}
		var candidates []struct {
			Index        *uint32 `json:"index"`
			FinishReason string  `json:"finishReason"`
		}
		if raw, exists := object["candidates"]; exists {
			if json.Unmarshal(raw, &candidates) != nil {
				observation.BodyComplete = false
				return observation
			}
		}
		for ordinal, candidate := range candidates {
			index := strconv.Itoa(ordinal)
			if candidate.Index != nil {
				index = strconv.FormatUint(uint64(*candidate.Index), 10)
			}
			if candidate.FinishReason != "" {
				finished[index] = candidate.FinishReason
			} else if _, seen := finished[index]; !seen {
				finished[index] = ""
			}
		}
		var feedback struct {
			BlockReason string `json:"blockReason"`
		}
		_ = json.Unmarshal(object["promptFeedback"], &feedback)
		observation.ProviderError = observation.ProviderError || feedback.BlockReason != "" && feedback.BlockReason != "BLOCK_REASON_UNSPECIFIED"
	}
	if len(finished) != 0 {
		observation.ExecutionResult = "succeeded"
		for _, reason := range finished {
			switch reason {
			case "STOP":
			case "MAX_TOKENS", "SAFETY", "RECITATION", "LANGUAGE", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "IMAGE_SAFETY", "IMAGE_PROHIBITED_CONTENT", "IMAGE_RECITATION":
				if observation.ExecutionResult != "unknown" {
					observation.ExecutionResult = "partial"
				}
			default:
				observation.ExecutionResult = "unknown"
			}
		}
	}
	if observation.ProviderError || status >= 400 {
		observation.ExecutionResult = "failed"
	}
	if stream && observation.ExecutionResult == "unknown" {
		observation.BodyComplete = false
		observation.Usage = executionfacts.MissingTokenUsage("google_stream_terminal_not_observed")
	}
	if status >= 200 && status < 300 && (observation.ExecutionResult == "succeeded" || observation.ExecutionResult == "partial") {
		observation.Acceptance = "accepted"
	}
	return observation
}

// Moonshot's SDK moves choices[0].usage to the OpenAI root. Project that same
// original RawMessage for observation; do not parse counters through SDK ints.
func managedMoonshotObservation(profile managedProtocolProfile, status uint16, headers [][2]string, body []byte, complete bool) managedProtocolObservation {
	stream, err := managedProtocolContentType(headers)
	if !stream || err != nil || !complete {
		return profile.ObserveResponse(status, headers, body, complete)
	}
	events, err := managedProtocolEvents(body)
	if err != nil {
		return profile.ObserveResponse(status, headers, body, complete)
	}
	var view bytes.Buffer
	for _, event := range events {
		if bytes.Equal(event.Data, []byte("[DONE]")) {
			managedProtocolWriteEvent(&view, event.Name, event.Data)
			continue
		}
		var object map[string]json.RawMessage
		if decodeManagedJSON(event.Data, &object, false) != nil || object == nil {
			return profile.ObserveResponse(status, headers, body, complete)
		}
		var choices []map[string]json.RawMessage
		_ = json.Unmarshal(object["choices"], &choices)
		if len(choices) != 0 {
			if usage, exists := choices[0]["usage"]; exists {
				object["usage"] = usage
			}
		}
		encoded, _ := json.Marshal(object)
		managedProtocolWriteEvent(&view, event.Name, encoded)
	}
	return profile.ObserveResponse(status, headers, view.Bytes(), complete)
}
