package main

import (
	"bytes"
	"encoding/json"
	"math/big"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/tidwall/gjson"
)

const managedCacheWriteConflict = "provider_cache_write_conflict_or_invalid"

type managedCacheMode string

const (
	managedCacheImplicit managedCacheMode = "0"
	managedCacheExplicit managedCacheMode = "1"
)

func hasManagedCacheControl(request gjson.Result, protocol string) bool {
	if protocol == managedProtocolMessages {
		if request.Get("cache_control.type").String() == "ephemeral" {
			return true
		}
		for _, path := range []string{"system", "tools"} {
			for _, item := range request.Get(path).Array() {
				if item.Get("cache_control.type").String() == "ephemeral" {
					return true
				}
			}
		}
	}
	if protocol == managedProtocolResponses {
		if request.Get("cache_control.type").String() == "ephemeral" {
			return true
		}
		if input := request.Get("input"); input.IsArray() {
			for _, item := range input.Array() {
				if item.Get("cache_control.type").String() == "ephemeral" {
					return true
				}
				if content := item.Get("content"); content.IsArray() {
					for _, block := range content.Array() {
						if block.Get("cache_control.type").String() == "ephemeral" {
							return true
						}
					}
				}
			}
		}
	}
	for _, path := range []string{"messages", "input.messages"} {
		for _, message := range request.Get(path).Array() {
			for _, content := range message.Get("content").Array() {
				if content.Get("cache_control.type").String() == "ephemeral" {
					return true
				}
				if protocol == managedProtocolMessages {
					if nested := content.Get("content"); nested.IsArray() {
						for _, sub := range nested.Array() {
							if sub.Get("cache_control.type").String() == "ephemeral" {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

// Both aliases describe the same write. A nil count with present=true means
// the supplier supplied an invalid or contradictory measurement.
func managedCacheWriteCount(fields map[string]json.RawMessage) (count *big.Int, present bool) {
	creation := fields["cache_creation_input_tokens"]
	write := fields["cache_write_tokens"]
	hasCreation, hasWrite := !managedProtocolNull(creation), !managedProtocolNull(write)
	if !hasCreation && !hasWrite {
		return nil, false
	}
	if !hasWrite {
		write = creation
	}
	count, ok := managedCount(write)
	if !ok {
		return nil, true
	}
	if hasCreation && hasWrite {
		other, ok := managedCount(creation)
		if !ok || other.Cmp(count) != 0 {
			return nil, true
		}
	}
	return count, true
}

// A write is a subset of the legacy cache-miss counter, which defaults to
// the full input when the supplier omits its read counter.
func managedCacheWrite(quantities map[string]string, write *big.Int, creation, writeDetails json.RawMessage) {
	input := quantities["cache_miss"]
	if input == "" {
		input = quantities["input"]
	}
	miss, valid := managedInteger(input)
	if !valid || write.Cmp(miss) > 0 {
		return
	}
	quantities["cache_write"] = write.String()
	if managedProtocolNull(creation) && managedProtocolNull(writeDetails) {
		return
	}
	sum := new(big.Int)
	buckets := make(map[string]string)
	for _, breakdown := range [...]json.RawMessage{creation, writeDetails} {
		if managedProtocolNull(breakdown) {
			continue
		}
		var details map[string]json.RawMessage
		if json.Unmarshal(breakdown, &details) != nil {
			return
		}
		for field, meter := range map[string]string{
			"ephemeral_5m_input_tokens": "cache_write_5m",
			"ephemeral_1h_input_tokens": "cache_write_1h",
			"ttl_30m_tokens":            "cache_write_30m",
			"ttl_12h_tokens":            "cache_write_12h",
		} {
			value, exists := details[field]
			if !exists {
				continue
			}
			count, ok := managedCount(value)
			if !ok {
				return
			}
			quantity := count.String()
			if prior, exists := buckets[meter]; exists {
				if prior != quantity {
					return
				}
				continue
			}
			sum.Add(sum, count)
			buckets[meter] = quantity
		}
	}
	if sum.Cmp(write) == 0 {
		for meter, count := range buckets {
			quantities[meter] = count
		}
	}
}

// Qwen reports thinking in the original message or delta, before projection
// can hide it. An intact terminal with no evidence is the non-thinking mode.
func recordManagedQwenThinking(s *managedExecutionState, usage *executionfacts.Usage, headers [][2]string, body []byte, terminal bool) {
	if s.config.profile.supplier != "qwen" || usage.Status != "reported" || !terminal {
		return
	}
	protocol := s.config.profile.UpstreamProtocol
	switch protocol {
	case managedProtocolChat, managedProtocolMessages, managedProtocolResponses, managedProtocolDashScopeText, managedProtocolDashScopeMultimodal:
	default:
		return
	}
	thinking := false
	if stream, _ := managedProtocolContentType(headers); stream {
		events, err := managedProtocolEvents(body)
		if err != nil {
			return
		}
		for _, event := range events {
			if managedQwenEventThinking(protocol, event) {
				thinking = true
				break
			}
		}
	} else {
		switch protocol {
		case managedProtocolMessages:
			thinking = managedMessagesThinking(body)
		case managedProtocolResponses:
			thinking = managedResponsesThinking(body)
		default:
			thinking = managedQwenReasoning(body)
		}
	}
	applyManagedQwenThinking(s, usage, thinking)
}

func managedQwenEventThinking(protocol string, event managedProtocolEvent) bool {
	switch protocol {
	case managedProtocolMessages:
		if bytes.Equal(bytes.TrimSpace(event.Data), []byte("[DONE]")) {
			return false
		}
		object, kind, err := managedProtocolEventObject(event)
		return err == nil && managedMessagesEventThinking(object, kind)
	case managedProtocolResponses:
		if bytes.Equal(bytes.TrimSpace(event.Data), []byte("[DONE]")) {
			return false
		}
		object, kind, err := managedProtocolEventObject(event)
		return err == nil && managedResponsesEventThinking(object, kind)
	default:
		return managedQwenReasoning(event.Data)
	}
}

func managedMessagesThinking(payload []byte) bool {
	for _, item := range gjson.GetBytes(payload, "content").Array() {
		switch item.Get("type").String() {
		case "thinking", "redacted_thinking":
			return true
		}
	}
	return false
}

func managedMessagesEventThinking(object map[string]json.RawMessage, kind string) bool {
	switch kind {
	case "message_start":
		return managedMessagesThinking(object["message"])
	case "content_block_start":
		switch gjson.GetBytes(object["content_block"], "type").String() {
		case "thinking", "redacted_thinking":
			return true
		}
	case "content_block_delta":
		switch gjson.GetBytes(object["delta"], "type").String() {
		case "thinking_delta", "signature_delta":
			return true
		}
	}
	return false
}

func managedResponsesThinking(payload []byte) bool {
	for _, item := range gjson.GetBytes(payload, "output").Array() {
		if item.Get("type").String() == "reasoning" {
			return true
		}
	}
	return false
}

func managedResponsesEventThinking(object map[string]json.RawMessage, kind string) bool {
	switch kind {
	case "response.output_item.added", "response.output_item.done":
		if gjson.GetBytes(object["item"], "type").String() == "reasoning" {
			return true
		}
	case "response.reasoning_text.delta", "response.reasoning_text.done",
		"response.reasoning_summary_text.delta", "response.reasoning_summary_text.done",
		"response.reasoning_summary_part.added", "response.reasoning_summary_part.done":
		return true
	}
	if raw, exists := object["response"]; exists {
		return managedResponsesThinking(raw)
	}
	return false
}

func managedQwenReasoning(payload []byte) bool {
	for _, path := range []string{"choices.#.delta.reasoning_content", "choices.#.message.reasoning_content", "output.choices.#.message.reasoning_content"} {
		for _, item := range gjson.GetBytes(payload, path).Array() {
			if item.Type == gjson.String && item.String() != "" {
				return true
			}
		}
	}
	return false
}

func applyManagedQwenThinking(s *managedExecutionState, usage *executionfacts.Usage, thinking bool) {
	if s.cacheMode == managedCacheImplicit {
		if usage.Quantities["cache_write"] == "" && (usage.Quantities["cache_read_mode"] == "" || s.config.profile.UpstreamProtocol == managedProtocolMessages) {
			usage.Quantities["cache_read_mode"] = string(managedCacheImplicit)
			usage.Quantities["cache_write_inapplicable"] = "1"
		} else if s.config.profile.UpstreamProtocol == managedProtocolMessages && usage.Quantities["cache_write"] == "0" {
			usage.Quantities["cache_read_mode"] = string(managedCacheImplicit)
		}
	}
	if count, exists := usage.Quantities["reasoning_tokens"]; exists {
		if count == "0" && thinking {
			return
		}
		thinking = count != "0"
	}
	if thinking {
		usage.Quantities["thinking_mode"] = "1"
	} else {
		usage.Quantities["thinking_mode"] = "0"
	}
}
