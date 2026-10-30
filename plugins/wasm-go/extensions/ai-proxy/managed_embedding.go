package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strconv"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

const managedProtocolEmbeddings = "openai_embeddings"
const managedEmbeddingAdapter = "openai-embeddings.v1"
const managedEmbeddingClassifier = "ai-proxy.native-embedding.v1"

func managedOperation(configuration *managedExecutionConfig) string {
	if configuration.settings.InputProtocol == managedProtocolEmbeddings {
		return executionfacts.NativeEmbeddingOperation
	}
	return "text_generate"
}

func validateManagedEmbedding(configuration *managedExecutionConfig) error {
	d := configuration.release.Descriptor
	capability, exists := d.Capabilities[executionfacts.NativeEmbeddingOperation]
	if !exists || len(d.Capabilities) != 1 || configuration.settings.CallIdentitySource != executionfacts.CallIdentitySourceInternal ||
		configuration.settings.InputProtocol != managedProtocolEmbeddings || configuration.settings.UpstreamProtocol != managedProtocolEmbeddings ||
		configuration.settings.TextExecution != nil || configuration.settings.MediaDispatchMode == "managed_operation" || configuration.settings.UpstreamHealth == nil ||
		d.AdapterRef != managedEmbeddingAdapter || d.Transport.PathAdapterRef != managedEmbeddingAdapter || capability.ResponseClassifierRef != managedEmbeddingClassifier ||
		capability.SendMode != "single_send" || capability.MaxAttempts != 1 || capability.MaxFallbackHops != 0 || capability.QueryCapabilityRef != "" || capability.ProviderIdempotencyRef != "" {
		return errors.New("embedding requires its registered internal single-send adapter and health capability")
	}
	return nil
}

func prepareManagedEmbeddingRequest(body []byte, model, credential string) (managedProtocolRequest, error) {
	var object map[string]json.RawMessage
	var actualModel string
	if decodeManagedJSON(body, &object, false) != nil || object == nil || json.Unmarshal(object["model"], &actualModel) != nil || actualModel != model {
		return managedProtocolRequest{}, errors.New("embedding request differs from the frozen model")
	}
	for _, field := range []string{"stream", "background"} {
		if raw, exists := object[field]; exists {
			var enabled bool
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &enabled) != nil || enabled {
				return managedProtocolRequest{}, errors.New("embedding supports only synchronous JSON responses")
			}
		}
	}
	count := managedEmbeddingInputCount(object["input"])
	if count == 0 || !validManagedCredentialValue(credential) {
		return managedProtocolRequest{}, errors.New("embedding requires text/token input and its selected credential")
	}
	return managedProtocolRequest{Body: bytes.Clone(body), Headers: [][2]string{{"content-type", "application/json"}, {"accept", "application/json"}, {"accept-encoding", "identity"}, {"authorization", "Bearer " + credential}}, ExpectedEmbeddings: count}, nil
}

// Input modality is not inferred from a URL, vector dimension or output usage.
// The original OpenAI embedding input is text, a text batch or exact token IDs.
func managedEmbeddingInputCount(raw json.RawMessage) int {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		if text != "" {
			return 1
		}
		return 0
	}
	var batch []json.RawMessage
	if json.Unmarshal(raw, &batch) != nil || len(batch) == 0 {
		return 0
	}
	allTokens := true
	for _, item := range batch {
		if _, ok := managedCount(item); !ok {
			allTokens = false
			break
		}
	}
	if allTokens {
		return 1
	}
	kind := ""
	for _, item := range batch {
		if json.Unmarshal(item, &text) == nil {
			if text == "" || kind == "tokens" {
				return 0
			}
			kind = "text"
			continue
		}
		var tokens []json.RawMessage
		if json.Unmarshal(item, &tokens) != nil || len(tokens) == 0 || kind == "text" {
			return 0
		}
		for _, token := range tokens {
			if _, ok := managedCount(token); !ok {
				return 0
			}
		}
		kind = "tokens"
	}
	return len(batch)
}

func managedEmbeddingUsage(object map[string]json.RawMessage) *executionfacts.Usage {
	var usage map[string]json.RawMessage
	if json.Unmarshal(object["usage"], &usage) != nil || usage == nil {
		return executionfacts.MissingTokenUsage("provider_usage_not_observed")
	}
	input, ok := managedCount(usage["prompt_tokens"])
	if !ok {
		return executionfacts.MissingTokenUsage("provider_input_usage_invalid")
	}
	if raw, exists := usage["total_tokens"]; exists {
		total, valid := managedCount(raw)
		if !valid || total.Cmp(input) != 0 {
			return executionfacts.MissingTokenUsage("provider_embedding_usage_total_conflict")
		}
	}
	return &executionfacts.Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: map[string]string{"input": input.String()}}
}

func managedEmbeddingObservation(status uint16, headers [][2]string, body []byte, complete bool, expected int) managedProtocolObservation {
	observation := managedProtocolObservation{Usage: executionfacts.MissingTokenUsage("provider_usage_not_observed"), ExecutionResult: "unknown", Acceptance: "unknown", BodyComplete: complete}
	if status >= 400 && status <= 599 {
		observation.ExecutionResult = "failed"
	}
	stream, err := managedProtocolContentType(headers)
	if !complete || err != nil || stream {
		return observation
	}
	var object map[string]json.RawMessage
	if decodeManagedJSON(body, &object, false) != nil || object == nil {
		return observation
	}
	observation.Usage = managedEmbeddingUsage(object)
	if !managedProtocolMergeModel(&observation.ResponseModel, object["model"]) {
		return observation
	}
	if raw, exists := object["error"]; exists && !managedProtocolNull(raw) {
		observation.ProviderError, observation.ExecutionResult = true, "failed"
		return observation
	}
	if status < 200 || status >= 300 {
		return observation
	}
	var data []map[string]json.RawMessage
	if json.Unmarshal(object["data"], &data) != nil || len(data) == 0 || expected > 0 && len(data) != expected {
		return observation
	}
	seen := make(map[int]bool, len(data))
	for _, item := range data {
		index, err := strconv.Atoi(string(item["index"]))
		if err != nil || index < 0 || index >= len(data) || strconv.Itoa(index) != string(item["index"]) || seen[index] || !managedEmbeddingVector(item["embedding"]) {
			return observation
		}
		seen[index] = true
	}
	observation.ExecutionResult, observation.Acceptance = "succeeded", "accepted"
	return observation
}

func managedEmbeddingVector(raw json.RawMessage) bool {
	var encoded string
	if json.Unmarshal(raw, &encoded) == nil {
		decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
		return err == nil && len(decoded) > 0 && len(decoded)%4 == 0 && base64.StdEncoding.EncodeToString(decoded) == encoded
	}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) != nil || len(values) == 0 {
		return false
	}
	for _, value := range values {
		number, err := strconv.ParseFloat(string(value), 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return false
		}
	}
	return true
}

func managedSelectionDigest(configuration *managedExecutionConfig, raw []byte) (string, error) {
	if managedOperation(configuration) == executionfacts.NativeEmbeddingOperation {
		return executionfacts.EmbeddingSelectionDigest(raw)
	}
	return executionfacts.SelectionDigest(raw)
}

func encodeManagedDeliveryBridge(snapshot executionfacts.Snapshot) (string, error) {
	if snapshot.Envelope().OperationKind == executionfacts.NativeEmbeddingOperation {
		return executionfacts.EncodeEmbeddingDeliveryBridge(snapshot)
	}
	return executionfacts.EncodeDeliveryBridge(snapshot)
}
