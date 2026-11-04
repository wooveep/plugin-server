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
	Aliases                                         *ResourceAliases
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
	if policy.Aliases == nil {
		policy.Aliases = &ResourceAliases{}
	}
	if !streaming {
		encoded, err := projectObject(policy, body, usage, false)
		if err != nil {
			return nil, err
		}
		return policy.Aliases.rewrite(policy.Protocol, encoded, false)
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
		raw, err = policy.Aliases.rewrite(policy.Protocol, raw, true)
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
			} else {
				if policy.IncludeUsage && lastChat != nil {
					if wire := usage.wire(Chat); wire != nil {
						lastChat["choices"], lastChat["usage"] = json.RawMessage("[]"), wire
						counts, e := json.Marshal(lastChat)
						if e != nil {
							return nil, e
						}
						AppendEvent(&output, Event{Data: counts})
					}
				}
				lastChat = nil
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
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(sanitized["error"], &fields)
		object["error"], _ = json.Marshal(retain(fields, "code message"))
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
			object = retain(object, "type sequence_number response output_index content_index summary_index item_id item part delta text refusal arguments annotation annotation_index logprobs")
		}
		for _, key := range []string{"item", "part", "content_block", "delta"} {
			if raw, ok := object[key]; ok && len(raw) > 0 && raw[0] == '{' {
				if key == "delta" && kind == "message_delta" {
					var delta map[string]json.RawMessage
					if json.Unmarshal(raw, &delta) != nil {
						return nil, errors.New("invalid public message delta")
					}
					object[key], _ = json.Marshal(retain(delta, "stop_reason stop_sequence"))
					continue
				}
				projected, err := projectBlock(raw)
				if err != nil {
					return nil, err
				}
				object[key] = projected
			}
		}
		if raw, ok := object["annotation"]; ok {
			var annotation map[string]json.RawMessage
			if json.Unmarshal(raw, &annotation) != nil || annotation == nil {
				return nil, errors.New("invalid public annotation")
			}
			object["annotation"], _ = json.Marshal(projectCitation(annotation))
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
				if raw := choices[i]["logprobs"]; len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
					delete(choices[i], "logprobs")
				}
				for _, key := range []string{"message", "delta"} {
					var message map[string]json.RawMessage
					if json.Unmarshal(choice[key], &message) == nil && message != nil {
						// Real reasoning and opaque continuation fields are content,
						// not supplier diagnostic metadata.
						var err error
						choices[i][key], err = projectChatMessage(message)
						if err != nil {
							return nil, err
						}
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
		if output, ok := result["output"]; ok {
			var err error
			result["output"], err = projectBlockList(output)
			if err != nil {
				return nil, err
			}
		}
		result["usage"] = json.RawMessage("null")
		if wire := usage.wire(Responses); wire != nil {
			result["usage"] = wire
		}
	case Messages:
		result = retain(object, "id type role model content stop_reason stop_sequence usage container")
		result["type"] = json.RawMessage(`"message"`)
		if content, ok := result["content"]; ok {
			var err error
			result["content"], err = projectBlockList(content)
			if err != nil {
				return nil, err
			}
		}
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
	return ClassifyFailure(FailureInput{Body: body, Client: policy.Protocol}).Body(policy)
}

// Headers emits public transport metadata. The billing bridge is installed later
// by its original owner and never travels through this public allowlist.
func Headers(policy Policy, headers [][2]string, streaming bool) [][2]string {
	result := make([][2]string, 0, 5)
	for _, h := range headers {
		switch strings.ToLower(h[0]) {
		case "cache-control", "content-language":
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
