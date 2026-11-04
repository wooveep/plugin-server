package main

import (
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
)

func projectNativeReplayBody(state *nativeCacheDeliveryState, kind protocol.ProtocolKind, body []byte, stream bool) ([]byte, error) {
	client := map[protocol.ProtocolKind]string{protocol.ProtocolChatCompletions: clientresponse.Chat, protocol.ProtocolResponses: clientresponse.Responses, protocol.ProtocolMessages: clientresponse.Messages}[kind]
	created, _ := time.Parse(time.RFC3339Nano, state.origin.OccurredAt)
	policy := clientresponse.Policy{Protocol: client, PublicModel: state.config.publicModel, RequestID: state.origin.CallID, CreatedAt: created.Unix(), IncludeUsage: state.includeUsage, RequestFields: state.publicFields}
	// The replay estimator already replaced the historical supplier's usage.
	// Project current delivery counts, not the cached supplier's reasoning meter.
	return clientresponse.Project(policy, body, stream, clientresponse.ReadUsage(body, client, stream))
}
