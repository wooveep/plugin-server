package clientresponse

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
)

// History reconstructs supported completed text/thinking content for the existing
// history eligibility gate. It does not turn pending tools into completed text.
func History(protocol string, body []byte) ([]json.RawMessage, error) {
	events, err := Events(body)
	if err != nil {
		return nil, err
	}
	message := map[string]json.RawMessage{"role": json.RawMessage(`"assistant"`)}
	blocks := map[int]map[string]json.RawMessage{}
	appendText := func(object map[string]json.RawMessage, key string, raw json.RawMessage) error {
		var text, previous string
		if json.Unmarshal(raw, &text) != nil {
			return errors.New("continuation text is not a string")
		}
		if len(object[key]) > 0 {
			_ = json.Unmarshal(object[key], &previous)
		}
		object[key], _ = json.Marshal(previous + text)
		return nil
	}
	for _, e := range events {
		if bytes.Equal(e.Data, []byte("[DONE]")) {
			continue
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(e.Data, &object) != nil || object == nil {
			return nil, errors.New("invalid continuation event")
		}
		switch protocol {
		case Chat:
			var choices []struct {
				Delta map[string]json.RawMessage `json:"delta"`
			}
			if json.Unmarshal(object["choices"], &choices) != nil || len(choices) == 0 {
				continue
			}
			for _, key := range []string{"content", "reasoning_content", "reasoning", "refusal", "reasoning_signature", "reasoning_redacted_content"} {
				if raw, exists := choices[0].Delta[key]; exists && !bytes.Equal(raw, []byte("null")) {
					if err := appendText(message, key, raw); err != nil {
						return nil, err
					}
				}
			}
		case Responses:
			var kind string
			_ = json.Unmarshal(object["type"], &kind)
			if kind == "response.completed" || kind == "response.incomplete" {
				var response struct {
					Output []json.RawMessage `json:"output"`
				}
				if json.Unmarshal(object["response"], &response) == nil && len(response.Output) > 0 {
					return response.Output, nil
				}
			}
		case Messages:
			var kind string
			var index int
			_ = json.Unmarshal(object["type"], &kind)
			_ = json.Unmarshal(object["index"], &index)
			if kind == "content_block_start" {
				var block map[string]json.RawMessage
				if json.Unmarshal(object["content_block"], &block) != nil || block == nil {
					return nil, errors.New("invalid continuation block")
				}
				blocks[index] = block
			} else if kind == "content_block_delta" {
				block := blocks[index]
				if block == nil {
					return nil, errors.New("continuation delta without its block")
				}
				var delta map[string]json.RawMessage
				_ = json.Unmarshal(object["delta"], &delta)
				for _, key := range []string{"text", "thinking", "signature"} {
					if raw, ok := delta[key]; ok {
						if err := appendText(block, key, raw); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}
	if protocol == Responses {
		return nil, errors.New("Responses continuation has no complete output")
	}
	if protocol == Messages {
		indices := make([]int, 0, len(blocks))
		for index := range blocks {
			indices = append(indices, index)
		}
		sort.Ints(indices)
		content := make([]map[string]json.RawMessage, 0, len(indices))
		for _, index := range indices {
			content = append(content, blocks[index])
		}
		message["content"], _ = json.Marshal(content)
	}
	raw, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	return []json.RawMessage{raw}, nil
}
