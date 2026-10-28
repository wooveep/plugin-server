package main

import (
	"encoding/json"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/tidwall/sjson"
)

// buildThinSemanticContextDigest masks only values selected by the existing
// query extractor. The complete request (including all unknown fields) remains
// in the projection. It never changes the real body or the exact cache digest.
func buildThinSemanticContextDigest(kind protocol.ProtocolKind, body []byte, query thinQueryTextSelection) (string, error) {
	if strings.TrimSpace(query.Text) == "" || len(query.Slots) == 0 {
		return "", nil
	}
	// Validate the original complete JSON before applying any mask: duplicate
	// keys, malformed Unicode and exact-number budget errors cannot be hidden
	// by query extraction or by a last-key-wins JSON parser.
	projected, err := mfjson.Canonicalize(body)
	if err != nil {
		return "", err
	}
	slots := make([]string, 0, len(query.Slots))
	for _, slot := range query.Slots {
		projected, err = sjson.SetRawBytes(projected, slot.path, []byte(`""`))
		if err != nil {
			return "", err
		}
		slots = append(slots, slot.pointer)
	}
	payload, err := json.Marshal(struct {
		SchemaVersion  string                `json:"schema_version"`
		Protocol       protocol.ProtocolKind `json:"protocol"`
		QueryTextSlots []string              `json:"query_text_slots"`
		Request        json.RawMessage       `json:"request"`
	}{"ai-cache-semantic-context.v1", kind, slots, projected})
	if err != nil {
		return "", err
	}
	return mfjson.Digest(payload)
}
