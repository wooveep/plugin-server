package clientresponse

import (
	"bytes"
	"encoding/json"
	"math/big"
)

// Usage is presentation data. It never extends the financial meter vocabulary.
type Usage struct {
	Input, Output, Total json.RawMessage
	MessagesInput        json.RawMessage
	InputDetails         map[string]json.RawMessage
	OutputDetails        map[string]json.RawMessage
	ServerTools          json.RawMessage
}

func number(raw json.RawMessage) (*big.Int, bool) {
	if len(raw) == 0 || len(raw) > 39 {
		return nil, false
	}
	for _, b := range raw {
		if b < '0' || b > '9' {
			return nil, false
		}
	}
	n, ok := new(big.Int).SetString(string(raw), 10)
	return n, ok && n.BitLen() <= 128 && n.String() == string(raw)
}

func readDetails(raw json.RawMessage, keys string) map[string]json.RawMessage {
	var source map[string]json.RawMessage
	_ = json.Unmarshal(raw, &source)
	result := map[string]json.RawMessage{}
	for _, key := range splitKeys(keys) {
		if _, ok := number(source[key]); ok {
			result[key] = source[key]
		}
	}
	return result
}

func readUsageObject(raw json.RawMessage, format string) Usage {
	var o map[string]json.RawMessage
	_ = json.Unmarshal(raw, &o)
	u := Usage{Input: o["prompt_tokens"], Output: o["completion_tokens"], Total: o["total_tokens"], InputDetails: readDetails(o["prompt_tokens_details"], "cached_tokens cache_write_tokens audio_tokens image_tokens text_tokens"), OutputDetails: readDetails(o["completion_tokens_details"], "reasoning_tokens audio_tokens accepted_prediction_tokens rejected_prediction_tokens text_tokens")}
	if format == Responses || format == Messages || format == "qwen" {
		u.Input, u.Output = o["input_tokens"], o["output_tokens"]
		if format == Responses {
			u.InputDetails = readDetails(o["input_tokens_details"], "cached_tokens cache_write_tokens")
			u.OutputDetails = readDetails(o["output_tokens_details"], "reasoning_tokens")
			var details map[string]json.RawMessage
			_ = json.Unmarshal(o["input_tokens_details"], &details)
			ttl := readDetails(details["cache_write_tokens_details"], "ttl_30m_tokens ttl_12h_tokens")
			if a, ok := number(ttl["ttl_30m_tokens"]); ok {
				if b, ok := number(ttl["ttl_12h_tokens"]); ok {
					if total, ok := number(u.InputDetails["cache_write_tokens"]); ok && new(big.Int).Add(a, b).Cmp(total) == 0 {
						u.InputDetails["cache_write_tokens_details"], _ = json.Marshal(ttl)
					}
				}
			}
		} else if format == "qwen" {
			if _, ok := u.InputDetails["cached_tokens"]; !ok {
				if _, ok := number(o["cached_tokens"]); ok {
					u.InputDetails["cached_tokens"] = o["cached_tokens"]
				}
			}
			if len(u.OutputDetails) == 0 {
				u.OutputDetails = readDetails(o["output_tokens_details"], "reasoning_tokens")
			}
		} else {
			if tools := readDetails(o["server_tool_use"], "web_search_requests web_fetch_requests"); len(tools) > 0 {
				u.ServerTools, _ = json.Marshal(tools)
			}
			u.InputDetails = map[string]json.RawMessage{}
			u.MessagesInput, u.Total = u.Input, nil
			input, inputOK := number(u.Input)
			completeCache := true
			for source, target := range map[string]string{"cache_read_input_tokens": "cached_tokens", "cache_creation_input_tokens": "cache_write_tokens"} {
				if n, ok := number(o[source]); ok {
					u.InputDetails[target] = o[source]
					if inputOK {
						input.Add(input, n)
					}
				} else {
					completeCache = false
				}
			}
			if inputOK && completeCache && input.BitLen() <= 128 {
				u.Input = json.RawMessage(input.String())
			} else {
				// Ordinary Messages input is still known. A different protocol's
				// full input is unknown without its complete cache decomposition.
				u.Input = nil
			}
		}
	}
	// DeepSeek's duplicate counters must agree; a conflict is not resolved by
	// silently choosing the field that happens to be inspected last.
	_, hasHit := o["prompt_cache_hit_tokens"]
	_, hasMiss := o["prompt_cache_miss_tokens"]
	if hasHit || hasMiss {
		hit, hitOK := number(o["prompt_cache_hit_tokens"])
		miss, missOK := number(o["prompt_cache_miss_tokens"])
		input, inputOK := number(u.Input)
		if !hitOK || !missOK || !inputOK || new(big.Int).Add(hit, miss).Cmp(input) != 0 {
			delete(u.InputDetails, "cached_tokens")
		} else if prior, exists := u.InputDetails["cached_tokens"]; exists && !bytes.Equal(prior, o["prompt_cache_hit_tokens"]) {
			delete(u.InputDetails, "cached_tokens")
		} else {
			u.InputDetails["cached_tokens"] = o["prompt_cache_hit_tokens"]
		}
	}
	for _, detail := range []struct {
		values map[string]json.RawMessage
		total  json.RawMessage
	}{{u.InputDetails, u.Input}, {u.OutputDetails, u.Output}} {
		if total, ok := number(detail.total); ok {
			for key, raw := range detail.values {
				if value, numeric := number(raw); numeric && value.Cmp(total) > 0 {
					delete(detail.values, key)
				}
			}
		}
	}
	if input, ok := number(u.Input); ok {
		read, readOK := number(u.InputDetails["cached_tokens"])
		write, writeOK := number(u.InputDetails["cache_write_tokens"])
		if readOK && writeOK && new(big.Int).Add(read, write).Cmp(input) > 0 {
			delete(u.InputDetails, "cached_tokens")
			delete(u.InputDetails, "cache_write_tokens")
		}
	}
	if _, ok := number(u.InputDetails["cache_write_tokens"]); !ok {
		delete(u.InputDetails, "cache_write_tokens_details")
	}
	return u
}

// ReadUsage reads the final original supplier measurement, never sums cumulative
// stream frames and never treats an absent detail as a reported zero.
func ReadUsage(body []byte, format string, stream bool) Usage {
	if !stream {
		var o map[string]json.RawMessage
		_ = json.Unmarshal(body, &o)
		return readUsageObject(o["usage"], format)
	}
	frames, err := Events(body)
	if err != nil {
		return Usage{}
	}
	var latest map[string]json.RawMessage
	for _, frame := range frames {
		var o map[string]json.RawMessage
		if json.Unmarshal(frame.Data, &o) != nil {
			continue
		}
		if format == Responses {
			var response map[string]json.RawMessage
			_ = json.Unmarshal(o["response"], &response)
			o = response
		} else if format == Messages && bytes.Equal(o["type"], []byte(`"message_start"`)) {
			var message map[string]json.RawMessage
			_ = json.Unmarshal(o["message"], &message)
			o = message
		}
		var update map[string]json.RawMessage
		if json.Unmarshal(o["usage"], &update) == nil && update != nil {
			if latest == nil {
				latest = map[string]json.RawMessage{}
			}
			for k, v := range update {
				latest[k] = v
			}
		}
	}
	raw, _ := json.Marshal(latest)
	return readUsageObject(raw, format)
}

func (u Usage) wire(protocol string) json.RawMessage {
	if protocol == Messages && len(u.MessagesInput) > 0 {
		if _, ok := number(u.MessagesInput); !ok {
			return nil
		}
		if _, ok := number(u.Output); !ok {
			return nil
		}
		result := map[string]json.RawMessage{"input_tokens": u.MessagesInput, "output_tokens": u.Output}
		for source, target := range map[string]string{"cached_tokens": "cache_read_input_tokens", "cache_write_tokens": "cache_creation_input_tokens"} {
			if raw, exists := u.InputDetails[source]; exists {
				result[target] = raw
			}
		}
		if len(u.ServerTools) > 0 {
			result["server_tool_use"] = u.ServerTools
		}
		raw, _ := json.Marshal(result)
		return raw
	}
	i, iOK := number(u.Input)
	o, oOK := number(u.Output)
	if !iOK || !oOK {
		return nil
	}
	total := new(big.Int).Add(i, o)
	if total.BitLen() > 128 {
		return nil
	}
	if len(u.Total) > 0 {
		if t, ok := number(u.Total); !ok || t.Cmp(total) != 0 {
			return nil
		}
	}
	result := map[string]json.RawMessage{}
	if protocol == Chat {
		result["prompt_tokens"], result["completion_tokens"] = u.Input, u.Output
	} else {
		result["input_tokens"], result["output_tokens"] = u.Input, u.Output
	}
	if protocol == Messages {
		// Messages separates cache counts from ordinary input. Unknown optional
		// cache measurements stay absent rather than claiming a fabricated zero.
		input := new(big.Int).Set(i)
		for source, target := range map[string]string{"cached_tokens": "cache_read_input_tokens", "cache_write_tokens": "cache_creation_input_tokens"} {
			if n, ok := number(u.InputDetails[source]); ok {
				input.Sub(input, n)
				result[target] = u.InputDetails[source]
			}
		}
		if input.Sign() < 0 {
			return nil
		}
		result["input_tokens"] = json.RawMessage(input.String())
	} else {
		result["total_tokens"] = json.RawMessage(total.String())
		inputKey, outputKey := "prompt_tokens_details", "completion_tokens_details"
		if protocol == Responses {
			inputKey, outputKey = "input_tokens_details", "output_tokens_details"
		}
		input := map[string]json.RawMessage{}
		for key, value := range u.InputDetails {
			if protocol == Responses && key != "cached_tokens" && key != "cache_write_tokens" && key != "cache_write_tokens_details" || protocol == Chat && key == "cache_write_tokens_details" {
				continue
			}
			input[key] = value
		}
		if len(input) > 0 {
			result[inputKey], _ = json.Marshal(input)
		}
		output := map[string]json.RawMessage{}
		for key, value := range u.OutputDetails {
			if protocol == Chat || key == "reasoning_tokens" {
				output[key] = value
			}
		}
		if len(output) > 0 {
			result[outputKey], _ = json.Marshal(output)
		}
	}
	if protocol == Messages && len(u.ServerTools) > 0 && string(u.ServerTools) != "null" {
		result["server_tool_use"] = u.ServerTools
	}
	raw, _ := json.Marshal(result)
	return raw
}
