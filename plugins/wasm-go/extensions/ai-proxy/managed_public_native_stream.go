package main

import (
	"bytes"
	"encoding/json"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/tidwall/gjson"
)

func publicNativeIncompleteStream(s *managedExecutionState, headers [][2]string, original []byte) (managedProtocolClientResponse, uint32, bool, error) {
	normalized := bytes.ReplaceAll(original, []byte("\r\n"), []byte("\n"))
	last := bytes.LastIndex(normalized, []byte("\n\n"))
	if last < 0 {
		return managedProtocolClientResponse{}, 0, false, nil
	}
	events, err := clientresponse.Events(normalized[:last+2])
	if err != nil || len(events) == 0 {
		return managedProtocolClientResponse{}, 0, false, nil
	}
	policy := managedPublicPolicy(s)
	terminal := false
	sequence := int64(0)
	for _, event := range events {
		kind := gjson.GetBytes(event.Data, "type").String()
		if event.Name == "error" || kind == "error" {
			// Explicit errors keep their original known category through the
			// shared public codec; they are not relabeled as transport EOF.
			return managedProtocolClientResponse{}, 0, false, nil
		}
		if value := gjson.GetBytes(event.Data, "sequence_number").Int(); value >= sequence {
			sequence = value + 1
		}
		if policy.Protocol == clientresponse.Messages {
			terminal = terminal || kind == "message_stop"
		} else {
			terminal = terminal || kind == "response.completed" || kind == "response.failed" || kind == "response.incomplete"
		}
	}
	if terminal && last+2 == len(normalized) {
		return managedProtocolClientResponse{}, 0, false, nil
	}
	failure := clientresponse.Failure{Kind: clientresponse.FailureIncomplete}
	var output bytes.Buffer
	output.Write(normalized[:last+2])
	errorBody := failure.Body(policy)
	if policy.Protocol == clientresponse.Responses {
		var envelope map[string]json.RawMessage
		_ = json.Unmarshal(errorBody, &envelope)
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(envelope["error"], &fields)
		fields["type"] = json.RawMessage(`"error"`)
		fields["sequence_number"], _ = json.Marshal(sequence)
		errorBody, _ = json.Marshal(fields)
	}
	clientresponse.AppendEvent(&output, clientresponse.Event{Name: "error", Data: errorBody})
	body, err := clientresponse.Project(policy, output.Bytes(), true, clientresponse.ReadUsage(original, policy.Protocol, true))
	return managedProtocolClientResponse{Body: body, Headers: failure.Headers(policy, headers, true, true)}, 200, true, err
}
