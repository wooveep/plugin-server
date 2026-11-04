package clientresponse

import (
	"encoding/json"
	"errors"
)

// Block field ownership stops at user payloads such as tool input/arguments.
// Those are content, while unregistered siblings are supplier diagnostics.
var publicBlockFields = map[string]string{
	"message":                      "id status role content",
	"output_text":                  "text annotations logprobs",
	"text":                         "text citations",
	"refusal":                      "refusal",
	"reasoning":                    "id status summary content encrypted_content",
	"reasoning_text":               "text",
	"summary_text":                 "text",
	"thinking":                     "thinking signature",
	"redacted_thinking":            "data",
	"function_call":                "id status call_id name namespace arguments",
	"function_call_output":         "id call_id output status",
	"custom_tool_call":             "id call_id name input status",
	"custom_tool_call_output":      "id call_id output",
	"tool_use":                     "id name input",
	"server_tool_use":              "id name input",
	"tool_result":                  "tool_use_id content is_error",
	"web_search_tool_result":       "tool_use_id content",
	"web_search_result":            "url title encrypted_content page_age",
	"web_search_tool_result_error": "error_code",
	"web_search_call":              "id status action",
	"file_search_call":             "id status queries results",
	"code_interpreter_call":        "id status code container_id outputs",
	"image_generation_call":        "id status result revised_prompt background output_format quality size",
	"computer_call":                "id status call_id action pending_safety_checks",
	"local_shell_call":             "id status call_id action",
	"shell_call":                   "id status call_id action environment",
	"shell_call_output":            "id call_id output max_output_chars",
	"mcp_call":                     "id status approval_request_id arguments error name output server_label",
	"mcp_list_tools":               "id tools server_label error",
	"mcp_approval_request":         "id name arguments server_label",
	"compaction":                   "id encrypted_content",
	"text_delta":                   "text",
	"thinking_delta":               "thinking",
	"signature_delta":              "signature",
	"input_json_delta":             "partial_json",
	"citations_delta":              "citation",
}

func projectBlock(raw json.RawMessage) (json.RawMessage, error) {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return nil, errors.New("invalid public content block")
	}
	var kind string
	_ = json.Unmarshal(object["type"], &kind)
	fields, known := publicBlockFields[kind]
	if !known {
		return nil, errors.New("unregistered public content block type")
	}
	object = retain(object, "type "+fields)
	for _, key := range []string{"content", "summary"} {
		if value, ok := object[key]; ok && len(value) > 0 && value[0] == '[' {
			var err error
			object[key], err = projectBlockList(value)
			if err != nil {
				return nil, err
			}
		}
	}
	for _, key := range []string{"annotations", "citations"} {
		var annotations []map[string]json.RawMessage
		if json.Unmarshal(object[key], &annotations) == nil && annotations != nil {
			for i := range annotations {
				annotations[i] = projectCitation(annotations[i])
			}
			object[key], _ = json.Marshal(annotations)
		}
	}
	if raw, ok := object["citation"]; ok {
		var citation map[string]json.RawMessage
		if json.Unmarshal(raw, &citation) != nil || citation == nil {
			return nil, errors.New("invalid public citation")
		}
		object["citation"], _ = json.Marshal(projectCitation(citation))
	}
	return json.Marshal(object)
}

func projectCitation(object map[string]json.RawMessage) map[string]json.RawMessage {
	result := retain(object, "type text cited_text start_index end_index url title file_id filename index document_index document_title start_char_index end_char_index start_page_number end_page_number start_block_index end_block_index encrypted_index search_result_index source")
	if raw, exists := object["url_citation"]; exists {
		var citation map[string]json.RawMessage
		if json.Unmarshal(raw, &citation) == nil && citation != nil {
			result["url_citation"], _ = json.Marshal(retain(citation, "start_index end_index url title"))
		}
	}
	return result
}

func projectBlockList(raw json.RawMessage) (json.RawMessage, error) {
	var blocks []json.RawMessage
	if json.Unmarshal(raw, &blocks) != nil {
		return nil, errors.New("invalid public content list")
	}
	for i := range blocks {
		var err error
		blocks[i], err = projectBlock(blocks[i])
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(blocks)
}

func projectChatMessage(message map[string]json.RawMessage) (json.RawMessage, error) {
	message = retain(message, "id role content refusal annotations tool_calls function_call audio reasoning_content reasoning reasoning_signature reasoning_redacted_content claude_content_blocks")
	if raw, exists := message["refusal"]; exists {
		var refusal string
		if json.Unmarshal(raw, &refusal) != nil {
			return nil, errors.New("invalid public refusal")
		}
		if refusal == "" {
			delete(message, "refusal")
		}
	}
	if blocks, ok := message["claude_content_blocks"]; ok && string(blocks) != "null" {
		var err error
		message["claude_content_blocks"], err = projectBlockList(blocks)
		if err != nil {
			return nil, err
		}
	}
	var calls []map[string]json.RawMessage
	if json.Unmarshal(message["tool_calls"], &calls) == nil && calls != nil {
		for i, call := range calls {
			calls[i] = retain(call, "index id type function")
			if function, ok := call["function"]; ok {
				var fields map[string]json.RawMessage
				if json.Unmarshal(function, &fields) != nil || fields == nil {
					return nil, errors.New("invalid public function payload")
				}
				calls[i]["function"], _ = json.Marshal(retain(fields, "name arguments"))
			}
		}
		message["tool_calls"], _ = json.Marshal(calls)
	}
	if function, ok := message["function_call"]; ok && string(function) != "null" {
		var fields map[string]json.RawMessage
		if json.Unmarshal(function, &fields) != nil || fields == nil {
			return nil, errors.New("invalid public legacy function payload")
		}
		message["function_call"], _ = json.Marshal(retain(fields, "name arguments"))
	}
	var annotations []map[string]json.RawMessage
	if raw, exists := message["annotations"]; exists {
		if json.Unmarshal(raw, &annotations) != nil {
			return nil, errors.New("invalid public annotations")
		}
		for i := range annotations {
			annotations[i] = projectCitation(annotations[i])
		}
		if len(annotations) == 0 {
			delete(message, "annotations")
		} else {
			message["annotations"], _ = json.Marshal(annotations)
		}
	}
	return json.Marshal(message)
}
