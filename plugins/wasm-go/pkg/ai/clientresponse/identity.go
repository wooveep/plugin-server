package clientresponse

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// ResourceAlias is internal restoration data, never part of a public body.
type ResourceAlias struct {
	Kind     string `json:"kind"`
	Public   string `json:"public"`
	Original string `json:"original"`
}

// ResourceAliases belongs to one delivery, including all of its stream frames.
// Callers may retain its records to persist native resource continuation.
type ResourceAliases struct {
	values  map[[2]string]string
	records []ResourceAlias
	native  bool
}

// ObserveNativeResources records ownership without changing delivered bytes.
func ObserveNativeResources(protocol string, body []byte, stream bool) (*ResourceAliases, error) {
	aliases := &ResourceAliases{native: true}
	if !stream {
		_, err := aliases.rewrite(protocol, body, false)
		return aliases, err
	}
	events, err := Events(body)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		if string(event.Data) == "[DONE]" {
			continue
		}
		if _, err := aliases.rewrite(protocol, event.Data, true); err != nil {
			return nil, err
		}
	}
	return aliases, nil
}

func (a *ResourceAliases) Records() []ResourceAlias {
	return append([]ResourceAlias(nil), a.records...)
}

func (a *ResourceAliases) Seed(record ResourceAlias) {
	if a.values == nil {
		a.values = map[[2]string]string{}
	}
	a.values[[2]string{record.Kind, record.Original}] = record.Public
}

func (a *ResourceAliases) alias(kind string, raw json.RawMessage) (json.RawMessage, error) {
	var original string
	if len(raw) != 0 && string(raw) != "null" && json.Unmarshal(raw, &original) != nil {
		return nil, errors.New("resource identifier must be a string")
	}
	if a.values == nil {
		a.values = map[[2]string]string{}
	}
	key := [2]string{kind, original}
	public, exists := a.values[key]
	if a.native && !exists {
		public = original
		a.values[key] = public
		if original != "" {
			a.records = append(a.records, ResourceAlias{Kind: kind, Public: public, Original: original})
		}
		exists = true
	}
	if !exists {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, errors.New("public resource identity unavailable")
		}
		prefix := map[string]string{"chat": "chatcmpl", "response": "resp", "message": "msg", "item": "item", "call": "call", "conversation": "conv"}[kind]
		if prefix == "" {
			return nil, errors.New("unknown resource identity kind")
		}
		public = prefix + "_pf_" + hex.EncodeToString(random[:])
		a.values[key] = public
		a.records = append(a.records, ResourceAlias{Kind: kind, Public: public, Original: original})
	}
	return json.Marshal(public)
}

func (a *ResourceAliases) rewrite(protocol string, raw []byte, stream bool) ([]byte, error) {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return nil, errors.New("invalid public resource envelope")
	}
	set := func(key, kind string, required bool) error {
		value, exists := object[key]
		if !exists && !required || string(value) == "null" {
			return nil
		}
		var err error
		object[key], err = a.alias(kind, value)
		return err
	}
	if stream && protocol != Chat {
		for _, key := range []string{"response", "message"} {
			if nested, ok := object[key]; ok {
				// A Responses error's message is text, not a nested message
				// resource. Resource projection never rewrites error prose.
				if len(nested) == 0 || nested[0] != '{' {
					continue
				}
				value, err := a.rewrite(protocol, nested, false)
				if err != nil {
					return nil, err
				}
				object[key] = value
			}
		}
		if protocol == Responses {
			if err := set("item_id", "item", false); err != nil {
				return nil, err
			}
		}
		for _, key := range []string{"item", "content_block"} {
			if nested, ok := object[key]; ok {
				value, err := a.rewriteItem(nested, protocol)
				if err != nil {
					return nil, err
				}
				object[key] = value
			}
		}
		return json.Marshal(object)
	}
	// Error-only envelopes have no resource identity to mint.
	if _, ok := object["id"]; !ok {
		if _, failed := object["error"]; failed {
			return json.Marshal(object)
		}
	}
	kind := map[string]string{Chat: "chat", Responses: "response", Messages: "message"}[protocol]
	if err := set("id", kind, true); err != nil {
		return nil, err
	}
	if protocol == Chat {
		var choices []map[string]json.RawMessage
		if json.Unmarshal(object["choices"], &choices) == nil {
			for _, choice := range choices {
				for _, key := range []string{"message", "delta"} {
					if nested, ok := choice[key]; ok {
						value, err := a.rewriteItem(nested, protocol)
						if err != nil {
							return nil, err
						}
						choice[key] = value
					}
				}
			}
			object["choices"], _ = json.Marshal(choices)
		}
	} else {
		key := "content"
		if protocol == Responses {
			key = "output"
			if err := set("previous_response_id", "response", false); err != nil {
				return nil, err
			}
			if conversation, ok := object["conversation"]; ok && string(conversation) != "null" {
				if len(conversation) > 0 && conversation[0] == '"' {
					if err := set("conversation", "conversation", false); err != nil {
						return nil, err
					}
				} else {
					var container map[string]json.RawMessage
					if json.Unmarshal(conversation, &container) != nil {
						return nil, errors.New("invalid response conversation")
					}
					var err error
					container["id"], err = a.alias("conversation", container["id"])
					if err != nil {
						return nil, err
					}
					object["conversation"], _ = json.Marshal(container)
				}
			}
		}
		var items []json.RawMessage
		if json.Unmarshal(object[key], &items) == nil && items != nil {
			for i := range items {
				var err error
				items[i], err = a.rewriteItem(items[i], protocol)
				if err != nil {
					return nil, err
				}
			}
			object[key], _ = json.Marshal(items)
		}
	}
	return json.Marshal(object)
}

func (a *ResourceAliases) rewriteItem(raw json.RawMessage, protocol string) (json.RawMessage, error) {
	var item map[string]json.RawMessage
	if json.Unmarshal(raw, &item) != nil || item == nil {
		return nil, errors.New("invalid public output item")
	}
	kind := "item"
	if protocol == Messages {
		kind = "call"
	} else if protocol == Chat {
		kind = "message"
	}
	for key, keyKind := range map[string]string{"id": kind, "call_id": "call", "tool_use_id": "call", "tool_call_id": "call"} {
		if value, exists := item[key]; exists && string(value) != "null" {
			var err error
			item[key], err = a.alias(keyKind, value)
			if err != nil {
				return nil, err
			}
		}
	}
	var calls []map[string]json.RawMessage
	if json.Unmarshal(item["tool_calls"], &calls) == nil && calls != nil {
		for _, call := range calls {
			if _, present := call["id"]; !present {
				continue
			}
			var err error
			call["id"], err = a.alias("call", call["id"])
			if err != nil {
				return nil, err
			}
		}
		item["tool_calls"], _ = json.Marshal(calls)
	}
	return json.Marshal(item)
}
