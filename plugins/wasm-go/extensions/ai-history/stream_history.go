package main

import (
	"bytes"
	"encoding/json"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
)

func nativeStreamHistoryOutput(kind protocol.ProtocolKind, chunks [][]byte) ([]json.RawMessage, string) {
	client := map[protocol.ProtocolKind]string{protocol.ProtocolChatCompletions: clientresponse.Chat, protocol.ProtocolResponses: clientresponse.Responses, protocol.ProtocolMessages: clientresponse.Messages}[kind]
	output, err := clientresponse.History(client, bytes.Join(chunks, nil))
	if err != nil {
		return nil, "continuation-output-incomplete"
	}
	return output, ""
}
