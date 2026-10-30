package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/memorycache"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const materializedRecordSchemaVersion = "ai-cache.materialized.v1"

type MaterializedReplayRecord struct {
	SchemaVersion       string                `json:"schema_version"`
	Tenant              string                `json:"tenant"`
	Consumer            string                `json:"consumer,omitempty"`
	Route               string                `json:"route"`
	Model               string                `json:"model"`
	Protocol            protocol.ProtocolKind `json:"protocol"`
	CacheScope          string                `json:"cache_scope"`
	CachePolicyVersion  string                `json:"cache_policy_version"`
	RequestDigest       string                `json:"request_digest"`
	SoftExpiresAt       int64                 `json:"soft_expires_at"`
	HardExpiresAt       int64                 `json:"hard_expires_at"`
	Response            json.RawMessage       `json:"response"`
	Usage               json.RawMessage       `json:"usage,omitempty"`
	FinishReason        string                `json:"finish_reason,omitempty"`
	StreamReplayable    bool                  `json:"stream_replayable,omitempty"`
	StreamChunks        []json.RawMessage     `json:"stream_chunks,omitempty"`
	TrustedUpstreamFact *bool                 `json:"upstream_invoked,omitempty"`
}

func LoadAndValidateMaterializedRecord(raw string, material ScopedCacheKeyMaterial, stream bool) (MaterializedReplayRecord, error) {
	if strings.TrimSpace(raw) == "" {
		return MaterializedReplayRecord{}, errors.New("empty materialized record")
	}

	var record MaterializedReplayRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return MaterializedReplayRecord{}, errors.New("materialized record is not valid JSON")
	}
	if err := validateMaterializedRecord(record, material, stream, time.Now().Unix()); err != nil {
		return MaterializedReplayRecord{}, err
	}
	return record, nil
}

func validateMaterializedRecord(record MaterializedReplayRecord, material ScopedCacheKeyMaterial, stream bool, now int64) error {
	if record.RequestDigest != material.RequestDigest {
		return errors.New("materialized record request digest mismatch")
	}
	return validateReplayRecordScopeAndPayload(record, material, stream, now)
}

func validateReplayRecordScopeAndPayload(record MaterializedReplayRecord, material ScopedCacheKeyMaterial, stream bool, now int64) error {
	if record.SchemaVersion != materializedRecordSchemaVersion {
		return errors.New("unsupported materialized record schema version")
	}
	if !isSupportedThinProtocol(record.Protocol) {
		return errors.New("materialized record protocol is invalid")
	}
	if record.Protocol != material.Protocol {
		return errors.New("materialized record protocol mismatch")
	}
	if record.CacheScope != material.CacheScope {
		return errors.New("materialized record cache scope mismatch")
	}
	if record.Tenant != material.Tenant {
		return errors.New("materialized record tenant mismatch")
	}
	if material.CacheScope == config.CACHE_SCOPE_CONSUMER && record.Consumer != material.Consumer {
		return errors.New("materialized record consumer mismatch")
	}
	if record.Route != material.Route {
		return errors.New("materialized record route mismatch")
	}
	if record.Model != material.Model {
		return errors.New("materialized record model mismatch")
	}
	if record.CachePolicyVersion != material.CachePolicyVersion {
		return errors.New("materialized record policy version mismatch")
	}
	if record.SoftExpiresAt <= 0 || record.SoftExpiresAt <= now {
		return errors.New("materialized record soft expiration is expired")
	}
	if record.HardExpiresAt <= 0 || record.HardExpiresAt <= now {
		return errors.New("materialized record hard expiration is expired")
	}
	if record.FinishReason == "" {
		return errors.New("materialized record finish reason is required")
	}
	if err := requireJSONObject(record.Usage, "usage"); err != nil {
		return err
	}
	if stream {
		if !record.StreamReplayable {
			return errors.New("materialized record is not stream replayable")
		}
		if len(record.StreamChunks) == 0 {
			return errors.New("materialized record stream chunks are required")
		}
		for _, chunk := range record.StreamChunks {
			if err := requireJSONObject(chunk, "stream chunk"); err != nil {
				return err
			}
			output := thinOutputGate(record.Protocol, chunk)
			if output.blockedReason != "" {
				return errors.New(output.blockedReason)
			}
			if output.containsToolCalls {
				return errors.New("materialized record contains tool calls")
			}
		}
		return nil
	}
	if err := requireJSONObject(record.Response, "response"); err != nil {
		return err
	}
	output := thinOutputGate(record.Protocol, record.Response)
	if output.blockedReason != "" {
		return errors.New(output.blockedReason)
	}
	if output.containsToolCalls {
		return errors.New("materialized record contains tool calls")
	}
	return nil
}

func replayMaterializedRecord(record MaterializedReplayRecord, stream bool, ctx wrapper.HttpContext, log logs.Log) error {
	// Both Redis and Console callers validate the record against the original
	// request material before this point. Its protocol is that frozen selection.
	adapter, ok := thinProtocolAdapterForKind(record.Protocol)
	if !ok {
		return fmt.Errorf("unsupported protocol for materialized replay: %s", record.Protocol)
	}
	serialized, err := adapter.SerializeReplay(thinReplayPayload(record.Protocol, record, stream))
	if err != nil {
		return err
	}
	statusCode := serialized.StatusCode
	if statusCode == 0 {
		statusCode = 200
	}

	bridge, send := nativeCacheBridge(ctx, record)
	if !send {
		return nil
	}
	ctx.SetUserAttribute("cache_status", "hit")
	ctx.SetUserAttribute("upstream_invoked", false)
	ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
	headers := [][2]string{{"content-type", serialized.ContentType}}
	if bridge != "" {
		headers = append(headers, [2]string{executionfacts.DeliveryBridgeHeader, bridge})
	}
	if err := proxywasm.SendHttpResponseWithDetail(uint32(statusCode), memorycache.CacheHitResponseDetail, headers, serialized.Body, -1); err != nil {
		nativeCacheFactGap(ctx, "cache_local_response_outcome_unknown")
		// The local handoff may already have happened. Never resume into a new
		// upstream send after its result becomes unknown.
		return nil
	}
	return nil
}

func isSupportedThinProtocol(kind protocol.ProtocolKind) bool {
	switch kind {
	case protocol.ProtocolChatCompletions, protocol.ProtocolMessages, protocol.ProtocolResponses:
		return true
	default:
		return false
	}
}

func requireJSONObject(raw json.RawMessage, name string) error {
	if len(raw) == 0 {
		return fmt.Errorf("materialized record %s is required", name)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("materialized record %s is not valid JSON object", name)
	}
	if fields == nil {
		return fmt.Errorf("materialized record %s is not a JSON object", name)
	}
	return nil
}
