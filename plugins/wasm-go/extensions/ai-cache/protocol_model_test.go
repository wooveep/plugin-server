package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/stretchr/testify/require"
)

func TestSharedProtocolModelRepresentsAICacheReplayRecord(t *testing.T) {
	streamUsage := protocol.Usage{InputTokens: 8, OutputTokens: 3, TotalTokens: 11}
	record := MaterializedReplayRecord{
		SchemaVersion:      materializedRecordSchemaVersion,
		Tenant:             "tenant-a",
		Consumer:           "consumer-a",
		Route:              "route-a",
		Model:              "gpt-4.1",
		Protocol:           protocol.ProtocolResponses,
		CacheScope:         config.CACHE_SCOPE_CONSUMER,
		CachePolicyVersion: "cache-policy-v1|context|context-policy-v1|context-digest-a",
		RequestDigest:      "request-digest-a",
		SoftExpiresAt:      time.Now().Add(time.Minute).Unix(),
		HardExpiresAt:      time.Now().Add(time.Hour).Unix(),
		Response:           json.RawMessage(`{"output_text":"cached response"}`),
		Usage:              json.RawMessage(`{"input_tokens":8,"output_tokens":3,"total_tokens":11}`),
		FinishReason:       "completed",
		StreamReplayable:   true,
		StreamChunks: []json.RawMessage{
			json.RawMessage(`{"type":"response.output_text.delta","delta":"cached response"}`),
		},
	}

	normalized := protocol.NormalizedExchange{
		Request: protocol.RequestFacts{
			Protocol: protocol.ProtocolResponses,
			Path:     "/v1/responses",
			Route:    record.Route,
			Tenant:   record.Tenant,
			Consumer: record.Consumer,
			Model:    record.Model,
			Stream:   true,
		},
		CacheDigest: protocol.CacheDigestInput{
			Protocol:              protocol.ProtocolResponses,
			Model:                 record.Model,
			Fields:                map[string]json.RawMessage{"input": json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"draft reply"}]}]`)},
			ExcludedFields:        []string{"stream"},
			ContextPolicyVersion:  "context-policy-v1",
			InjectedContextDigest: "context-digest-a",
		},
		Response: protocol.ResponseText{
			Text:         "cached response",
			FinishReason: record.FinishReason,
		},
		Usage: streamUsage,
		StreamChunks: []protocol.StreamChunk{{
			Protocol:   protocol.ProtocolResponses,
			Event:      "response.output_text.delta",
			Data:       record.StreamChunks[0],
			TextDelta:  "cached response",
			Usage:      &streamUsage,
			Replayable: true,
		}},
		Replay: protocol.ReplayPayload{
			Protocol:    protocol.ProtocolResponses,
			StatusCode:  200,
			ContentType: "application/json",
			Body:        record.Response,
			StreamChunks: []protocol.StreamChunk{{
				Protocol:   protocol.ProtocolResponses,
				Event:      "response.output_text.delta",
				Data:       record.StreamChunks[0],
				TextDelta:  "cached response",
				Replayable: true,
			}},
		},
	}

	require.Equal(t, protocol.ProtocolResponses, protocol.DetectProtocolKind(normalized.Request.Path))
	require.NoError(t, normalized.Replay.Validate(true))
	require.Equal(t, "context-policy-v1", normalized.CacheDigest.ContextPolicyVersion)
	require.Equal(t, "context-digest-a", normalized.CacheDigest.InjectedContextDigest)
	require.Equal(t, streamUsage, *normalized.StreamChunks[0].Usage)
	require.JSONEq(t, `{"output_text":"cached response"}`, string(normalized.Replay.Body))
}
