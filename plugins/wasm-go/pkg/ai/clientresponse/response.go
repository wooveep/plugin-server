// Package clientresponse implements the platform's public text response contract.
// Supplier measurements remain separate from the unchanged financial snapshots.
package clientresponse

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

const (
	Chat      = "openai_chat"
	Responses = "openai_responses"
	Messages  = "anthropic_messages"
)

type Policy struct {
	Protocol, PublicModel, UpstreamModel, RequestID string
	IncludeUsage, ExposeServiceTier                 bool
	CreatedAt                                       int64
	RequestFields                                   map[string]json.RawMessage
}

// RequestFields keeps only client-owned response echoes, never supplier settings.
// The caller has already validated and accepted the complete request boundary.
func RequestFields(body []byte) map[string]json.RawMessage {
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(body, &fields)
	return retain(fields, "instructions metadata tools tool_choice parallel_tool_calls temperature top_p text reasoning max_output_tokens store background truncation prompt_cache_key prompt_cache_retention prompt_cache_options safety_identifier")
}

func splitKeys(value string) []string { return strings.Fields(value) }

func retain(object map[string]json.RawMessage, names string) map[string]json.RawMessage {
	result := map[string]json.RawMessage{}
	for _, name := range splitKeys(names) {
		if raw, ok := object[name]; ok {
			result[name] = raw
		}
	}
	return result
}

// Project normalizes a response already encoded in the client's protocol.
// usage must come from the original supplier payload, not a converter's defaults.
func Project(policy Policy, body []byte, streaming bool, usage Usage) ([]byte, error) {
	if policy.PublicModel == "" || policy.Protocol != Chat && policy.Protocol != Responses && policy.Protocol != Messages {
		return nil, errors.New("public response requires its frozen model and protocol")
	}
	if !streaming {
		return projectObject(policy, body, usage, false)
	}
	events, err := Events(body)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	var lastChat map[string]json.RawMessage
	for _, event := range events {
		if bytes.Equal(event.Data, []byte("[DONE]")) {
			if policy.Protocol == Chat && policy.IncludeUsage && lastChat != nil {
				if wire := usage.wire(Chat); wire != nil {
					lastChat["choices"], lastChat["usage"] = json.RawMessage("[]"), wire
					raw, err := json.Marshal(lastChat)
					if err != nil {
						return nil, err
					}
					AppendEvent(&output, Event{Data: raw})
				}
			}
			AppendEvent(&output, event)
			continue
		}
		raw, err := projectObject(policy, event.Data, usage, true)
		if err != nil {
			return nil, err
		}
		if policy.Protocol == Chat {
			var frame map[string]json.RawMessage
			_ = json.Unmarshal(raw, &frame)
			if _, failed := frame["error"]; !failed {
				lastChat = frame
				var choices []json.RawMessage
				_ = json.Unmarshal(frame["choices"], &choices)
				if len(choices) == 0 {
					continue
				}
			}
		}
		AppendEvent(&output, Event{Name: event.Name, Data: raw})
	}
	return output.Bytes(), nil
}

func projectObject(policy Policy, body []byte, usage Usage, stream bool) ([]byte, error) {
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil || object == nil {
		return nil, errors.New("invalid public response JSON")
	}
	if stream && bytes.Equal(object["type"], []byte(`"error"`)) {
		if policy.Protocol == Messages {
			return Error(policy, body), nil
		}
		var sanitized map[string]json.RawMessage
		_ = json.Unmarshal(Error(policy, body), &sanitized)
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(sanitized["error"], &fields)
		for _, key := range []string{"type", "sequence_number"} {
			if raw, ok := object[key]; ok {
				fields[key] = raw
			}
		}
		return json.Marshal(fields)
	}
	if raw, exists := object["error"]; exists && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if policy.Protocol != Responses || !bytes.Equal(object["object"], []byte(`"response"`)) {
			return Error(policy, body), nil
		}
		var sanitized map[string]json.RawMessage
		_ = json.Unmarshal(Error(policy, body), &sanitized)
		object["error"] = sanitized["error"]
	}
	model, _ := json.Marshal(policy.PublicModel)
	if stream && policy.Protocol != Chat {
		var kind string
		_ = json.Unmarshal(object["type"], &kind)
		for _, nested := range []string{"response", "message"} {
			if raw, exists := object[nested]; exists {
				current := usage
				if kind == "response.created" || kind == "response.in_progress" {
					current = Usage{}
				} else if kind == "message_start" {
					current.Output, current.Total = json.RawMessage("0"), nil
					current.OutputDetails = nil
				}
				projected, err := projectObject(policy, raw, current, false)
				if err != nil {
					return nil, err
				}
				object[nested] = projected
			}
		}
		if policy.Protocol == Messages {
			if kind == "message_delta" {
				current := usage
				var fields map[string]json.RawMessage
				_ = json.Unmarshal(object["usage"], &fields)
				if _, valid := number(fields["output_tokens"]); valid {
					current.Output, current.Total = fields["output_tokens"], nil
				}
				if wire := current.wire(Messages); wire != nil {
					object["usage"] = wire
				} else {
					delete(object, "usage")
				}
			}
			object = retain(object, "type index message content_block delta usage error")
		} else {
			object = retain(object, "type sequence_number response output_index content_index summary_index item_id item part delta text refusal arguments annotation annotation_index obfuscation logprobs")
		}
		return json.Marshal(object)
	}
	var result map[string]json.RawMessage
	switch policy.Protocol {
	case Chat:
		result = retain(object, "id object created model choices usage service_tier")
		objectName := `"chat.completion"`
		if stream {
			objectName = `"chat.completion.chunk"`
		}
		result["object"] = json.RawMessage(objectName)
		var choices []map[string]json.RawMessage
		if json.Unmarshal(object["choices"], &choices) == nil {
			for i, choice := range choices {
				choices[i] = retain(choice, "index message delta finish_reason logprobs")
				for _, key := range []string{"message", "delta"} {
					var message map[string]json.RawMessage
					if json.Unmarshal(choice[key], &message) == nil && message != nil {
						// Real reasoning and opaque continuation fields are content,
						// not supplier diagnostic metadata.
						choices[i][key], _ = json.Marshal(retain(message, "id role content refusal annotations tool_calls function_call audio reasoning_content reasoning reasoning_signature reasoning_redacted_content claude_content_blocks"))
					}
				}
			}
			result["choices"], _ = json.Marshal(choices)
		}
		if stream {
			delete(result, "usage")
			if policy.IncludeUsage {
				result["usage"] = json.RawMessage("null")
			}
		} else if wire := usage.wire(Chat); wire != nil {
			result["usage"] = wire
		} else {
			delete(result, "usage")
		}
	case Responses:
		result = retain(object, "id object created_at completed_at status model output error incomplete_details instructions metadata tools tool_choice parallel_tool_calls temperature top_p text reasoning max_output_tokens max_tool_calls previous_response_id conversation store background truncation usage service_tier safety_identifier prompt_cache_key prompt_cache_retention prompt_cache_options access_programs")
		for _, key := range []string{"instructions", "metadata", "tools"} {
			delete(result, key)
			if raw, ok := policy.RequestFields[key]; ok {
				result[key] = raw
			}
		}
		for key, raw := range policy.RequestFields {
			if _, present := result[key]; !present {
				result[key] = raw
			}
		}
		result["object"] = json.RawMessage(`"response"`)
		result["usage"] = json.RawMessage("null")
		if wire := usage.wire(Responses); wire != nil {
			result["usage"] = wire
		}
	case Messages:
		result = retain(object, "id type role model content stop_reason stop_sequence usage container")
		result["type"] = json.RawMessage(`"message"`)
		if wire := usage.wire(Messages); wire != nil {
			result["usage"] = wire
		}
	}
	if len(result["id"]) == 0 && policy.RequestID != "" {
		result["id"], _ = json.Marshal(policy.RequestID)
	}
	createdKey := "created"
	if policy.Protocol == Responses {
		createdKey = "created_at"
	}
	if policy.Protocol != Messages && len(result[createdKey]) == 0 && policy.CreatedAt > 0 {
		result[createdKey] = json.RawMessage(strconv.FormatInt(policy.CreatedAt, 10))
	}
	result["model"] = model
	if !policy.ExposeServiceTier {
		delete(result, "service_tier")
	}
	return json.Marshal(result)
}

// Error preserves a client-protocol error shape without exposing raw diagnostics.
func Error(policy Policy, body []byte) []byte {
	var object map[string]json.RawMessage
	_ = json.Unmarshal(body, &object)
	var source map[string]json.RawMessage
	_ = json.Unmarshal(object["error"], &source)
	if source == nil {
		source = object
	}
	message := "The upstream request could not be completed."
	if raw := source["message"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &message)
	}
	if policy.UpstreamModel != "" && policy.UpstreamModel != policy.PublicModel {
		message = strings.ReplaceAll(message, policy.UpstreamModel, policy.PublicModel)
	}
	// Unknown upstream messages may contain URLs, account data or echoed bodies.
	// Keep a stable public description; fixed error codes retain diagnostics.
	if len(message) > 512 || strings.Contains(message, "://") || strings.Contains(strings.ToLower(message), "api key") || strings.Contains(message, "Bearer ") {
		message = "The upstream request could not be completed."
	}
	errorObject := map[string]json.RawMessage{}
	for _, key := range []string{"type", "code", "param"} {
		if bytes.Equal(source[key], []byte("null")) {
			errorObject[key] = source[key]
			continue
		}
		var value string
		if json.Unmarshal(source[key], &value) != nil || len(value) > 128 {
			continue
		}
		valid := true
		for _, r := range value {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_.-/[]", r)) {
				valid = false
				break
			}
		}
		if valid {
			if policy.UpstreamModel != "" && policy.UpstreamModel != policy.PublicModel {
				value = strings.ReplaceAll(value, policy.UpstreamModel, policy.PublicModel)
			}
			errorObject[key], _ = json.Marshal(value)
		}
	}
	errorObject["message"], _ = json.Marshal(message)
	if _, ok := errorObject["type"]; !ok {
		errorObject["type"] = json.RawMessage(`"api_error"`)
	}
	result := map[string]json.RawMessage{}
	result["error"], _ = json.Marshal(errorObject)
	if policy.Protocol == Messages {
		result["type"] = json.RawMessage(`"error"`)
	}
	raw, _ := json.Marshal(result)
	return raw
}

// Headers emits public transport metadata. The billing bridge is installed later
// by its original owner and never travels through this public allowlist.
func Headers(policy Policy, headers [][2]string, streaming bool) [][2]string {
	result := make([][2]string, 0, 5)
	for _, h := range headers {
		switch strings.ToLower(h[0]) {
		case "cache-control", "retry-after", "content-language":
			result = append(result, h)
		}
	}
	contentType := "application/json"
	if streaming {
		contentType = "text/event-stream"
	}
	result = append(result, [2]string{"content-type", contentType})
	if policy.RequestID != "" {
		result = append(result, [2]string{"x-request-id", policy.RequestID})
	}
	return result
}
