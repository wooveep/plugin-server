package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

type consoleLookupRequest struct {
	CallID                string                         `json:"call_id,omitempty"`
	NativeIdentity        *nativeCacheProcessingIdentity `json:"native_identity,omitempty"`
	Tenant                string                         `json:"tenant"`
	Consumer              string                         `json:"consumer,omitempty"`
	Session               string                         `json:"session,omitempty"`
	Route                 string                         `json:"route"`
	Model                 string                         `json:"model"`
	Protocol              protocol.ProtocolKind          `json:"protocol"`
	CacheScope            string                         `json:"cache_scope"`
	CachePolicyVersion    string                         `json:"cache_policy_version"`
	RequestDigest         string                         `json:"request_digest"`
	StreamMode            string                         `json:"stream_mode"`
	SemanticQueryText     string                         `json:"semantic_query_text,omitempty"`
	SemanticContextDigest string                         `json:"semantic_context_digest,omitempty"`
}

type consoleLookupResponseEnvelope struct {
	Data  *consoleLookupResponse `json:"data"`
	Error json.RawMessage        `json:"error,omitempty"`
}

type consoleLookupResponse struct {
	Decision        string                `json:"decision"`
	Reason          string                `json:"reason,omitempty"`
	MaterializedKey string                `json:"materialized_key,omitempty"`
	ReplayRecord    *consoleReplayRecord  `json:"replay_record,omitempty"`
	SemanticMatch   *consoleSemanticMatch `json:"semantic_match,omitempty"`
}

type consoleSemanticMatch struct {
	LookupRequestDigest   string `json:"lookup_request_digest"`
	SemanticContextDigest string `json:"semantic_context_digest"`
}

type consoleReplayRecord struct {
	GatewayTenant         string                `json:"gateway_tenant"`
	GatewayConsumer       string                `json:"gateway_consumer,omitempty"`
	GatewayRoute          string                `json:"gateway_route"`
	GatewayModel          string                `json:"gateway_model"`
	Protocol              protocol.ProtocolKind `json:"protocol"`
	CacheScope            string                `json:"cache_scope"`
	CachePolicyVersion    string                `json:"cache_policy_version"`
	RequestDigest         string                `json:"request_digest"`
	SemanticContextDigest string                `json:"semantic_context_digest,omitempty"`
	ReplayStatus          string                `json:"replay_status"`
	ResponseObject        json.RawMessage       `json:"response_object"`
	UsageSnapshot         json.RawMessage       `json:"usage_snapshot"`
	FinishReason          string                `json:"finish_reason,omitempty"`
	StreamMode            string                `json:"stream_mode"`
	SoftExpiresAt         string                `json:"soft_expires_at,omitempty"`
	HardExpiresAt         string                `json:"hard_expires_at"`
	SafeToReplay          bool                  `json:"safe_to_replay"`
}

func shouldUseConsoleLookup(c config.PluginConfig) bool {
	return c.ConsoleLookup.Enabled &&
		c.RoutePolicy.EnableConsoleLookup &&
		c.RoutePolicy.EnableReplay &&
		strings.TrimSpace(c.ConsoleLookup.BearerToken) != ""
}

func lookupMaterializedRecordFromConsole(material ScopedCacheKeyMaterial, ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log, stream bool) error {
	requestBody, err := buildConsoleLookupRequestBody(material, ctx, stream)
	if err != nil {
		return err
	}

	client := wrapper.NewClusterClient(wrapper.FQDNCluster{
		FQDN: c.ConsoleLookup.ServiceName,
		Port: int64(c.ConsoleLookup.ServicePort),
	})
	headers := [][2]string{{"content-type", "application/json"}}
	if token := strings.TrimSpace(c.ConsoleLookup.BearerToken); token != "" {
		headers = append(headers, [2]string{"authorization", "Bearer " + token})
	}
	return client.Post(c.ConsoleLookup.Path, headers, requestBody, func(statusCode int, _ http.Header, responseBody []byte) {
		handleConsoleLookupResponse(material, statusCode, responseBody, ctx, c, log, stream)
	}, uint32(c.ConsoleLookup.Timeout))
}

func buildConsoleLookupRequestBody(material ScopedCacheKeyMaterial, ctx wrapper.HttpContext, stream bool) ([]byte, error) {
	semanticContext := ctx.GetStringContext(CACHE_SEMANTIC_CONTEXT_KEY, "")
	semanticQuery := ""
	if semanticContext != "" {
		semanticQuery = strings.TrimSpace(ctx.GetStringContext(CACHE_USER_CONTENT_KEY, ""))
	}
	return json.Marshal(consoleLookupRequest{
		CallID:                cacheNativeParentCallID(ctx),
		NativeIdentity:        cacheNativeIdentity(ctx),
		Tenant:                material.Tenant,
		Consumer:              material.Consumer,
		Session:               ctx.GetStringContext(CACHE_SESSION_CONTEXT_KEY, ""),
		Route:                 material.Route,
		Model:                 material.Model,
		Protocol:              material.Protocol,
		CacheScope:            material.CacheScope,
		CachePolicyVersion:    material.CachePolicyVersion,
		RequestDigest:         material.RequestDigest,
		StreamMode:            consoleLookupStreamMode(stream),
		SemanticQueryText:     semanticQuery,
		SemanticContextDigest: semanticContext,
	})
}

func consoleLookupStreamMode(stream bool) string {
	if stream {
		return "stream"
	}
	return "non_stream"
}

func handleConsoleLookupResponse(material ScopedCacheKeyMaterial, statusCode int, responseBody []byte, ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log, stream bool) {
	if statusCode != http.StatusOK {
		log.Warnf("[%s] [handleConsoleLookupResponse] Console lookup returned status %d for key: %s", PLUGIN_NAME, statusCode, material.RedisKey)
		proxywasm.ResumeHttpRequest()
		return
	}

	var envelope consoleLookupResponseEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		log.Warnf("[%s] [handleConsoleLookupResponse] Console lookup returned malformed body for key: %s", PLUGIN_NAME, material.RedisKey)
		proxywasm.ResumeHttpRequest()
		return
	}
	if hasConsoleLookupErrorEnvelope(envelope.Error) {
		log.Warnf("[%s] [handleConsoleLookupResponse] Console lookup returned error envelope for key: %s", PLUGIN_NAME, material.RedisKey)
		proxywasm.ResumeHttpRequest()
		return
	}
	if envelope.Data == nil {
		log.Warnf("[%s] [handleConsoleLookupResponse] Console lookup returned no data for key: %s", PLUGIN_NAME, material.RedisKey)
		proxywasm.ResumeHttpRequest()
		return
	}
	if envelope.Data.Decision != "hit" {
		log.Infof("[%s] [handleConsoleLookupResponse] Console lookup decision %s for key: %s", PLUGIN_NAME, envelope.Data.Decision, material.RedisKey)
		proxywasm.ResumeHttpRequest()
		return
	}
	// Older Console versions marked semantic_replay while rewriting the source
	// exact digest to the current request. That response is not exact evidence.
	if envelope.Data.Reason == "semantic_replay" && envelope.Data.SemanticMatch == nil {
		log.Warnf("[%s] [handleConsoleLookupResponse] semantic hit omitted its context proof", PLUGIN_NAME)
		proxywasm.ResumeHttpRequest()
		return
	}
	if envelope.Data.SemanticMatch != nil && (envelope.Data.MaterializedKey != "" || envelope.Data.ReplayRecord == nil) {
		log.Warnf("[%s] [handleConsoleLookupResponse] semantic hit must carry a direct source replay record", PLUGIN_NAME)
		proxywasm.ResumeHttpRequest()
		return
	}
	if envelope.Data.MaterializedKey != "" {
		if err := fetchConsoleMaterializedKey(material, envelope.Data.MaterializedKey, ctx, c, log, stream); err != nil {
			log.Warnf("[%s] [handleConsoleLookupResponse] Console lookup materialized key fetch failed for key: %s, error: %v", PLUGIN_NAME, material.RedisKey, err)
			proxywasm.ResumeHttpRequest()
			return
		}
		return
	}
	if envelope.Data.ReplayRecord == nil {
		log.Warnf("[%s] [handleConsoleLookupResponse] Console lookup hit omitted materialized key for key: %s", PLUGIN_NAME, material.RedisKey)
		proxywasm.ResumeHttpRequest()
		return
	}
	if err := replayConsoleReplayRecord(*envelope.Data.ReplayRecord, envelope.Data.SemanticMatch, material, ctx, log, stream); err != nil {
		log.Warnf("[%s] [handleConsoleLookupResponse] Console lookup replay record rejected for key: %s, error: %v", PLUGIN_NAME, material.RedisKey, err)
		proxywasm.ResumeHttpRequest()
		return
	}
}

func hasConsoleLookupErrorEnvelope(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	return strings.TrimSpace(string(raw)) != "null"
}

func fetchConsoleMaterializedKey(material ScopedCacheKeyMaterial, materializedKey string, ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log, stream bool) error {
	if !c.RoutePolicy.EnableReplay {
		return errors.New("replay is disabled")
	}
	redisClient := c.GetMaterializedRedisClient()
	if redisClient == nil {
		return errors.New("materialized Redis client is not configured")
	}

	lookupMaterial := material
	lookupMaterial.RedisKey = materializedKey
	log.Debugf("[%s] [fetchConsoleMaterializedKey] querying Console-returned materialized key: %s", PLUGIN_NAME, lookupMaterial.RedisKey)
	return redisClient.Get(lookupMaterial.RedisKey, func(response resp.Value) {
		handleConsoleMaterializedKeyResponse(lookupMaterial, response, ctx, log, stream)
	})
}

func replayConsoleReplayRecord(replay consoleReplayRecord, semanticMatch *consoleSemanticMatch, material ScopedCacheKeyMaterial, ctx wrapper.HttpContext, log logs.Log, stream bool) error {
	if replay.StreamMode != consoleLookupStreamMode(stream) {
		return errors.New("console replay record stream mode mismatch")
	}
	record, err := materializedReplayRecordFromConsoleReplay(replay)
	if err != nil {
		return err
	}
	if semanticMatch == nil {
		if err := validateMaterializedRecord(record, material, stream, time.Now().Unix()); err != nil {
			return err
		}
	} else {
		contextDigest := ctx.GetStringContext(CACHE_SEMANTIC_CONTEXT_KEY, "")
		if contextDigest == "" || semanticMatch.LookupRequestDigest != material.RequestDigest ||
			semanticMatch.SemanticContextDigest != contextDigest || replay.SemanticContextDigest != contextDigest || record.RequestDigest == "" {
			return errors.New("console semantic match does not prove the current request context")
		}
		// Keep the source record's exact digest intact. Semantic matching only
		// substitutes that one equality check; original scope, expiry, safety,
		// stream and payload rules still apply independently in this plugin.
		if err := validateReplayRecordScopeAndPayload(record, material, stream, time.Now().Unix()); err != nil {
			return err
		}
	}
	return replayMaterializedRecord(record, stream, ctx, log)
}

func materializedReplayRecordFromConsoleReplay(replay consoleReplayRecord) (MaterializedReplayRecord, error) {
	if replay.ReplayStatus != "active" {
		return MaterializedReplayRecord{}, errors.New("console replay record is not active")
	}
	if !replay.SafeToReplay {
		return MaterializedReplayRecord{}, errors.New("console replay record is not safe to replay")
	}

	hardExpiresAt, err := parseConsoleReplayUnixSecond(replay.HardExpiresAt, "hard_expires_at")
	if err != nil {
		return MaterializedReplayRecord{}, err
	}
	softExpiresAt := hardExpiresAt
	if strings.TrimSpace(replay.SoftExpiresAt) != "" {
		softExpiresAt, err = parseConsoleReplayUnixSecond(replay.SoftExpiresAt, "soft_expires_at")
		if err != nil {
			return MaterializedReplayRecord{}, err
		}
	}

	record := MaterializedReplayRecord{
		SchemaVersion:      materializedRecordSchemaVersion,
		Tenant:             replay.GatewayTenant,
		Consumer:           replay.GatewayConsumer,
		Route:              replay.GatewayRoute,
		Model:              replay.GatewayModel,
		Protocol:           replay.Protocol,
		CacheScope:         replay.CacheScope,
		CachePolicyVersion: replay.CachePolicyVersion,
		RequestDigest:      replay.RequestDigest,
		SoftExpiresAt:      softExpiresAt,
		HardExpiresAt:      hardExpiresAt,
		Response:           replay.ResponseObject,
		Usage:              replay.UsageSnapshot,
		FinishReason:       replay.FinishReason,
		StreamReplayable:   replay.StreamMode == "stream",
	}
	if replay.StreamMode == "stream" {
		var payload struct {
			StreamReplayable bool              `json:"stream_replayable"`
			StreamChunks     []json.RawMessage `json:"stream_chunks"`
		}
		if err := json.Unmarshal(replay.ResponseObject, &payload); err != nil || !payload.StreamReplayable {
			return MaterializedReplayRecord{}, errors.New("console stream replay requires original replayable chunks")
		}
		// Preserve original chunks; the existing scope/output validator handles
		// every protocol chunk before any local delivery can occur.
		record.Response = nil
		record.StreamChunks = payload.StreamChunks
	}
	return record, nil
}

func parseConsoleReplayUnixSecond(raw string, field string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("console replay record %s is required", field)
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return 0, fmt.Errorf("console replay record %s is invalid", field)
	}
	return parsed.Unix(), nil
}

func handleConsoleMaterializedKeyResponse(material ScopedCacheKeyMaterial, response resp.Value, ctx wrapper.HttpContext, log logs.Log, stream bool) {
	if err := response.Error(); err != nil {
		log.Errorf("[%s] [handleConsoleMaterializedKeyResponse] error retrieving materialized key: %s, error: %v", PLUGIN_NAME, material.RedisKey, err)
		proxywasm.ResumeHttpRequest()
		return
	}
	if response.IsNull() {
		log.Infof("[%s] [handleConsoleMaterializedKeyResponse] Console materialized key miss: %s", PLUGIN_NAME, material.RedisKey)
		proxywasm.ResumeHttpRequest()
		return
	}

	record, err := LoadAndValidateMaterializedRecord(response.String(), material, stream)
	if err != nil {
		log.Warnf("[%s] [handleConsoleMaterializedKeyResponse] Console materialized record rejected for key: %s, reason: %v", PLUGIN_NAME, material.RedisKey, err)
		proxywasm.ResumeHttpRequest()
		return
	}
	if err := replayMaterializedRecord(record, stream, ctx, log); err != nil {
		log.Warnf("[%s] [handleConsoleMaterializedKeyResponse] Console materialized replay failed for key: %s, error: %v", PLUGIN_NAME, material.RedisKey, err)
		proxywasm.ResumeHttpRequest()
		return
	}
}
