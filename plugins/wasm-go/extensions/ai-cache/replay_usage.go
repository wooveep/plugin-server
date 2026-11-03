package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/ai/tokenestimate"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// A cache replay is a new sale. Its original provider meter is evidence about
// the stored response, never the current request's usage or procurement cost.
func estimateNativeCacheReplay(state *nativeCacheDeliveryState, record MaterializedReplayRecord, payload protocol.ReplayPayload) (*executionfacts.Usage, protocol.ReplayPayload, error) {
	input, err := tokenestimate.Count(state.config.tokenizerModel, state.requestText)
	if err != nil {
		return nil, payload, fmt.Errorf("estimate current cache input: %w", err)
	}
	outputBody := record.Response
	if len(payload.StreamChunks) > 0 {
		var body bytes.Buffer
		for _, chunk := range payload.StreamChunks {
			body.WriteString("data: ")
			body.Write(chunk.Data)
			body.WriteString("\n\n")
		}
		outputBody = body.Bytes()
	}
	output, err := tokenestimate.Count(state.config.tokenizerModel, tokenestimate.ResponseText(outputBody))
	if err != nil {
		return nil, payload, fmt.Errorf("estimate cached output: %w", err)
	}
	usage := &executionfacts.Usage{Status: "estimated", Unit: "token", Source: "estimator", EstimatorVersion: "tiktoken-go.v0.7.0/" + string(tokenestimate.EncodingForModel(state.config.tokenizerModel)) + "/cache-replay-text.v1", Quantities: map[string]string{"input": strconv.FormatInt(input, 10), "output": strconv.FormatInt(output, 10), "cache_hit": "0", "cache_miss": strconv.FormatInt(input, 10)}}
	var wire []byte
	switch record.Protocol {
	case protocol.ProtocolChatCompletions:
		wire, err = json.Marshal(struct {
			Input   int64            `json:"prompt_tokens"`
			Output  int64            `json:"completion_tokens"`
			Total   int64            `json:"total_tokens"`
			Details map[string]int64 `json:"prompt_tokens_details"`
		}{input, output, input + output, map[string]int64{"cached_tokens": 0}})
	case protocol.ProtocolMessages:
		wire, err = json.Marshal(struct {
			Input  int64 `json:"input_tokens"`
			Output int64 `json:"output_tokens"`
			Read   int64 `json:"cache_read_input_tokens"`
			Write  int64 `json:"cache_creation_input_tokens"`
		}{Input: input, Output: output})
	case protocol.ProtocolResponses:
		wire, err = json.Marshal(struct {
			Input   int64            `json:"input_tokens"`
			Output  int64            `json:"output_tokens"`
			Total   int64            `json:"total_tokens"`
			Details map[string]int64 `json:"input_tokens_details"`
		}{input, output, input + output, map[string]int64{"cached_tokens": 0}})
	}
	if err != nil {
		return nil, payload, fmt.Errorf("encode cache usage: %w", err)
	}
	if len(payload.Body) > 0 {
		payload.Body, err = sjson.SetRawBytes(payload.Body, "usage", wire)
	} else {
		foundChatUsage := false
		for i := range payload.StreamChunks {
			chunk := &payload.StreamChunks[i]
			path := ""
			value := wire
			switch record.Protocol {
			case protocol.ProtocolChatCompletions:
				if gjson.GetBytes(chunk.Data, "usage").Exists() {
					path = "usage"
					foundChatUsage = true
				}
			case protocol.ProtocolMessages:
				switch chunk.Event {
				case "message_start":
					path = "message.usage"
					value, err = sjson.SetRawBytes(wire, "output_tokens", []byte("0"))
				case "message_delta":
					path = "usage"
					value = []byte(`{"output_tokens":` + usage.Quantities["output"] + `}`)
				}
			case protocol.ProtocolResponses:
				if gjson.GetBytes(chunk.Data, "response").IsObject() {
					path = "response.usage"
					if chunk.Event == "response.created" {
						value, err = sjson.SetRawBytes(wire, "output_tokens", []byte("0"))
						if err == nil {
							value, err = sjson.SetRawBytes(value, "total_tokens", []byte(usage.Quantities["input"]))
						}
					}
				}
			}
			if err == nil && path != "" {
				chunk.Data, err = sjson.SetRawBytes(chunk.Data, path, value)
			}
			if err != nil {
				return nil, payload, fmt.Errorf("encode cache stream usage: %w", err)
			}
		}
		if record.Protocol == protocol.ProtocolChatCompletions && !foundChatUsage && len(payload.StreamChunks) > 0 {
			last := &payload.StreamChunks[len(payload.StreamChunks)-1]
			last.Data, err = sjson.SetRawBytes(last.Data, "usage", wire)
		}
	}
	if err != nil {
		return nil, payload, fmt.Errorf("encode cache replay usage: %w", err)
	}
	return usage, payload, nil
}
