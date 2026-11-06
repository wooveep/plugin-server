package main

import (
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
	for _, path := range []string{"messages", "input.messages"} {
		for _, message := range request.Get(path).Array() {
			for _, content := range message.Get("content").Array() {
				if content.Get("cache_control.type").String() == "ephemeral" {
					return true
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
	switch s.config.profile.UpstreamProtocol {
	case managedProtocolChat, managedProtocolDashScopeText, managedProtocolDashScopeMultimodal:
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
			if managedQwenReasoning(event.Data) {
				thinking = true
				break
			}
		}
	} else {
		thinking = managedQwenReasoning(body)
	}
	applyManagedQwenThinking(s, usage, thinking)
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
	if s.cacheMode == managedCacheImplicit && usage.Quantities["cache_write"] == "" && usage.Quantities["cache_read_mode"] == "" {
		usage.Quantities["cache_read_mode"] = string(managedCacheImplicit)
		usage.Quantities["cache_write_inapplicable"] = "1"
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
