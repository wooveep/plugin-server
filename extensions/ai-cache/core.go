package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/vector"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

// CheckCacheForKey checks if the key is in the cache, or triggers similarity search if not found.
func CheckCacheForKey(key string, ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log, stream bool, useSimilaritySearch bool) error {
	activeCacheProvider := c.GetCacheProvider()
	if activeCacheProvider == nil {
		if !useSimilaritySearch || !c.EnableSemanticCache {
			log.Debugf("[%s] [CheckCacheForKey] no cache provider configured and similarity search is disabled", PLUGIN_NAME)
			proxywasm.ResumeHttpRequest()
			return nil
		}
		log.Debugf("[%s] [CheckCacheForKey] no cache provider configured, performing similarity search", PLUGIN_NAME)
		return performSimilaritySearch(key, ctx, c, log, key, stream)
	}

	keyMaterial := ScopedCacheKeyMaterialForLookup(ctx, c, key)
	if !keyMaterial.Valid {
		log.Warnf("[%s] [CheckCacheForKey] bypass cache, cache_id=%s, reason: %s", PLUGIN_NAME, cacheLogID(key), keyMaterial.BypassReason)
		proxywasm.ResumeHttpRequest()
		return nil
	}
	queryKey := activeCacheProvider.GetCacheKeyPrefix() + keyMaterial.EntryKey
	entryID := cacheLogID(queryKey)
	log.Debugf("[%s] [CheckCacheForKey] querying cache, cache_id=%s", PLUGIN_NAME, entryID)

	if handleL1StructuredCacheHit(queryKey, key, ctx, log, stream, c) {
		return nil
	}

	err := activeCacheProvider.Get(queryKey, func(response resp.Value) {
		if handleCacheHitResponse(queryKey, key, response, ctx, log, stream, c) {
			return
		}
		if ctx.GetBoolContext(CACHE_DISABLE_LEGACY_CONTEXT_KEY, false) {
			log.Debugf("[%s] [CheckCacheForKey] scoped cache miss, legacy fallback disabled, cache_id=%s", PLUGIN_NAME, entryID)
			handleFinalCacheMiss(queryKey, activeCacheProvider.GetCacheKeyPrefix()+keyMaterial.LockKey, key, response, ctx, log, stream, c, useSimilaritySearch)
			return
		}
		legacyQueryKey := activeCacheProvider.GetCacheKeyPrefix() + key
		if legacyQueryKey != queryKey {
			log.Debugf("[%s] [CheckCacheForKey] scoped cache miss, querying legacy key, cache_id=%s, legacy_cache_id=%s", PLUGIN_NAME, entryID, cacheLogID(legacyQueryKey))
			if err := activeCacheProvider.Get(legacyQueryKey, func(legacyResponse resp.Value) {
				if handleCacheHitResponse("", key, legacyResponse, ctx, log, stream, c) {
					return
				}
				handleFinalCacheMiss(queryKey, activeCacheProvider.GetCacheKeyPrefix()+keyMaterial.LockKey, key, response, ctx, log, stream, c, useSimilaritySearch)
			}); err != nil {
				log.Errorf("[%s] [CheckCacheForKey] failed to retrieve legacy cache entry, legacy_cache_id=%s, error: %v", PLUGIN_NAME, cacheLogID(legacyQueryKey), err)
				handleFinalCacheMiss(queryKey, activeCacheProvider.GetCacheKeyPrefix()+keyMaterial.LockKey, key, response, ctx, log, stream, c, useSimilaritySearch)
			}
			return
		}
		handleFinalCacheMiss(queryKey, activeCacheProvider.GetCacheKeyPrefix()+keyMaterial.LockKey, key, response, ctx, log, stream, c, useSimilaritySearch)
	})
	if err != nil {
		log.Errorf("[%s] [CheckCacheForKey] failed to retrieve cache entry, cache_id=%s, error: %v", PLUGIN_NAME, entryID, err)
		return err
	}

	return nil
}

func handleFinalCacheMiss(cacheEntryKey string, lockKey string, key string, response resp.Value, ctx wrapper.HttpContext, log logs.Log, stream bool, c config.PluginConfig, useSimilaritySearch bool) {
	if useSimilaritySearch && c.EnableSemanticCache {
		handleCacheResponse(key, response, ctx, log, stream, c, useSimilaritySearch)
		return
	}
	if requestCacheMode(ctx) == config.CACHE_MODE_NO_STORE {
		handleCacheResponse(key, response, ctx, log, stream, c, useSimilaritySearch)
		return
	}
	if handleRequestCollapseMiss(cacheEntryKey, lockKey, key, response, ctx, log, stream, c) {
		return
	}
	handleCacheResponse(key, response, ctx, log, stream, c, useSimilaritySearch)
}

// handleCacheResponse processes cache response and handles cache hits and misses.
func handleCacheResponse(key string, response resp.Value, ctx wrapper.HttpContext, log logs.Log, stream bool, c config.PluginConfig, useSimilaritySearch bool) {
	if handleCacheHitResponse("", key, response, ctx, log, stream, c) {
		return
	}

	cacheID := cacheLogID(key)
	log.Infof("[%s] [handleCacheResponse] cache miss, cache_id=%s", PLUGIN_NAME, cacheID)
	if err := response.Error(); err != nil {
		log.Errorf("[%s] [handleCacheResponse] error retrieving cache entry, cache_id=%s, error: %v", PLUGIN_NAME, cacheID, err)
	}

	if useSimilaritySearch && c.EnableSemanticCache {
		if err := performSimilaritySearch(key, ctx, c, log, key, stream); err != nil {
			log.Errorf("[%s] [handleCacheResponse] failed to perform similarity search, cache_id=%s, error: %v", PLUGIN_NAME, cacheID, err)
			proxywasm.ResumeHttpRequest()
		}
	} else {
		proxywasm.ResumeHttpRequest()
	}
}

func handleCacheHitResponse(cacheEntryKey string, key string, response resp.Value, ctx wrapper.HttpContext, log logs.Log, stream bool, c config.PluginConfig) bool {
	if err := response.Error(); err != nil || response.IsNull() {
		return false
	}
	cacheID := cacheEntryLogID(cacheEntryKey, key)
	payload := redisCacheValueString(response)
	decoded, err := DecodeRedisCacheEntryPayload(payload)
	if err != nil {
		log.Warnf("[%s] [handleCacheResponse] invalid structured cache entry, cache_id=%s, error: %v", PLUGIN_NAME, cacheID, err)
		proxywasm.ResumeHttpRequest()
		return true
	}
	if decoded.IsAuthoritative() {
		freshness := StructuredCacheEntryFreshnessAt(*decoded.StructuredEntry, time.Now().Unix())
		if freshness == StructuredCacheFreshnessExpired {
			log.Infof("[%s] structured cache entry expired, cache_id=%s", PLUGIN_NAME, cacheID)
			return false
		}
		if freshness == StructuredCacheFreshnessStale && !c.StaleWhileRevalidateEnabled() {
			log.Infof("[%s] structured cache entry stale, cache_id=%s", PLUGIN_NAME, cacheID)
			return false
		}
		if freshness == StructuredCacheFreshnessStale {
			log.Infof("[%s] stale structured cache hit, cache_id=%s", PLUGIN_NAME, cacheID)
		} else {
			log.Infof("[%s] structured cache hit, cache_id=%s", PLUGIN_NAME, cacheID)
		}
		populateL1StructuredCache(cacheEntryKey, *decoded.StructuredEntry, c, log)
		if !stream {
			processStructuredCacheHit(key, *decoded.StructuredEntry, ctx, log)
			return true
		}
		processStructuredStreamingCacheHit(key, *decoded.StructuredEntry, ctx, log)
	} else {
		log.Infof("[%s] legacy cache hit, cache_id=%s", PLUGIN_NAME, cacheID)
		processCacheHit(key, decoded.LegacyValue, stream, ctx, c, log)
	}
	return true
}

func handleL1StructuredCacheHit(cacheEntryKey string, key string, ctx wrapper.HttpContext, log logs.Log, stream bool, c config.PluginConfig) bool {
	l1Cache := configuredStructuredL1Cache(c)
	if l1Cache == nil {
		return false
	}
	entry, ok := l1Cache.Get(cacheEntryKey)
	if !ok {
		return false
	}

	freshness := StructuredCacheEntryFreshnessAt(entry, time.Now().Unix())
	cacheID := cacheEntryLogID(cacheEntryKey, key)
	if freshness == StructuredCacheFreshnessExpired {
		log.Infof("[%s] l1 structured cache entry expired, cache_id=%s", PLUGIN_NAME, cacheID)
		l1Cache.Delete(cacheEntryKey)
		return false
	}
	if freshness == StructuredCacheFreshnessStale && !c.StaleWhileRevalidateEnabled() {
		log.Infof("[%s] l1 structured cache entry stale, cache_id=%s", PLUGIN_NAME, cacheID)
		l1Cache.Delete(cacheEntryKey)
		return false
	}
	if freshness == StructuredCacheFreshnessStale {
		log.Infof("[%s] stale l1 structured cache hit, cache_id=%s", PLUGIN_NAME, cacheID)
	} else {
		log.Infof("[%s] l1 structured cache hit, cache_id=%s", PLUGIN_NAME, cacheID)
	}
	if !stream {
		processStructuredCacheHit(key, entry, ctx, log)
		return true
	}
	processStructuredStreamingCacheHit(key, entry, ctx, log)
	return true
}

func populateL1StructuredCache(cacheEntryKey string, entry StructuredCacheEntry, c config.PluginConfig, log logs.Log) {
	l1Cache := configuredStructuredL1Cache(c)
	if l1Cache == nil || cacheEntryKey == "" {
		return
	}
	if !validatedStructuredEntryForL1(entry, time.Now().Unix()) {
		log.Debugf("[%s] skip l1 structured cache population for invalid structured metadata, cache_id=%s", PLUGIN_NAME, cacheLogID(cacheEntryKey))
		return
	}
	l1Cache.Put(cacheEntryKey, entry)
	log.Debugf("[%s] populated l1 structured cache, cache_id=%s", PLUGIN_NAME, cacheLogID(cacheEntryKey))
}

func validatedStructuredEntryForL1(entry StructuredCacheEntry, nowUnix int64) bool {
	if entry.Version != StructuredCacheEntryVersion {
		return false
	}
	if strings.TrimSpace(entry.Answer.Content) == "" ||
		strings.TrimSpace(entry.Scope.Type) == "" ||
		strings.TrimSpace(entry.RequestFingerprint.Hash) == "" {
		return false
	}
	if entry.SoftExpiresAt <= 0 || entry.HardExpiresAt <= 0 {
		return false
	}
	if entry.SoftExpiresAt > entry.HardExpiresAt {
		return false
	}
	return StructuredCacheEntryFreshnessAt(entry, nowUnix) != StructuredCacheFreshnessExpired
}

func configuredStructuredL1Cache(c config.PluginConfig) *structuredL1Cache {
	if !c.L1Cache.Enabled {
		return nil
	}
	l1Cache, ok := c.GetL1CacheStore().(*structuredL1Cache)
	if !ok {
		return nil
	}
	return l1Cache
}

func redisCacheValueString(response resp.Value) string {
	if values := response.Array(); len(values) == 1 {
		return values[0].String()
	}
	return response.String()
}

func processStructuredCacheHit(key string, entry StructuredCacheEntry, ctx wrapper.HttpContext, log logs.Log) {
	cacheID := cacheLogID(key)
	if strings.TrimSpace(entry.Answer.Content) == "" {
		log.Warnf("[%s] [processStructuredCacheHit] cached response is empty, cache_id=%s", PLUGIN_NAME, cacheID)
		proxywasm.ResumeHttpRequest()
		return
	}

	log.Debugf("[%s] [processStructuredCacheHit] cached structured response, cache_id=%s", PLUGIN_NAME, cacheID)

	requestID := getProxyWasmPropertyString([]string{"x_request_id"})
	responseBody, err := BuildOpenAINonStreamingCacheHitResponse(entry, requestID)
	if err != nil {
		log.Errorf("[%s] [processStructuredCacheHit] failed to build cache hit response, cache_id=%s, error: %v", PLUGIN_NAME, cacheID, err)
		proxywasm.ResumeHttpRequest()
		return
	}

	ctx.SetContext(CACHE_KEY_CONTEXT_KEY, nil)
	ctx.SetUserAttribute("cache_status", "hit")
	ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)

	proxywasm.SendHttpResponseWithDetail(200, "ai-cache.hit", [][2]string{{"content-type", "application/json; charset=utf-8"}}, responseBody, -1)
}

func processStructuredStreamingCacheHit(key string, entry StructuredCacheEntry, ctx wrapper.HttpContext, log logs.Log) {
	cacheID := cacheLogID(key)
	if strings.TrimSpace(entry.Answer.Content) == "" {
		log.Warnf("[%s] [processStructuredStreamingCacheHit] cached response is empty, cache_id=%s", PLUGIN_NAME, cacheID)
		proxywasm.ResumeHttpRequest()
		return
	}

	log.Debugf("[%s] [processStructuredStreamingCacheHit] cached structured response, cache_id=%s", PLUGIN_NAME, cacheID)

	requestID := getProxyWasmPropertyString([]string{"x_request_id"})
	responseBody, err := BuildOpenAIStreamingCacheHitResponse(entry, requestID)
	if err != nil {
		log.Errorf("[%s] [processStructuredStreamingCacheHit] failed to build cache hit response, cache_id=%s, error: %v", PLUGIN_NAME, cacheID, err)
		proxywasm.ResumeHttpRequest()
		return
	}

	ctx.SetContext(CACHE_KEY_CONTEXT_KEY, nil)
	ctx.SetUserAttribute("cache_status", "hit")
	ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)

	proxywasm.SendHttpResponseWithDetail(200, "ai-cache.hit", [][2]string{{"content-type", "text/event-stream; charset=utf-8"}}, responseBody, -1)
}

// processCacheHit handles a successful cache hit.
func processCacheHit(key string, response string, stream bool, ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log) {
	cacheID := cacheLogID(key)
	if strings.TrimSpace(response) == "" {
		log.Warnf("[%s] [processCacheHit] cached response is empty, cache_id=%s", PLUGIN_NAME, cacheID)
		proxywasm.ResumeHttpRequest()
		return
	}

	log.Debugf("[%s] [processCacheHit] cached response, cache_id=%s, length=%d", PLUGIN_NAME, cacheID, len(response))

	// Escape the response to ensure consistent formatting
	escapedResponse := strings.Trim(strconv.Quote(response), "\"")

	ctx.SetContext(CACHE_KEY_CONTEXT_KEY, nil)

	ctx.SetUserAttribute("cache_status", "hit")
	ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)

	if stream {
		proxywasm.SendHttpResponseWithDetail(200, "ai-cache.hit", [][2]string{{"content-type", "text/event-stream; charset=utf-8"}}, []byte(fmt.Sprintf(c.StreamResponseTemplate, escapedResponse)), -1)
	} else {
		proxywasm.SendHttpResponseWithDetail(200, "ai-cache.hit", [][2]string{{"content-type", "application/json; charset=utf-8"}}, []byte(fmt.Sprintf(c.ResponseTemplate, escapedResponse)), -1)
	}
}

// performSimilaritySearch determines the appropriate similarity search method to use.
func performSimilaritySearch(key string, ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log, queryString string, stream bool) error {
	activeVectorProvider := c.GetVectorProvider()
	if activeVectorProvider == nil {
		return logAndReturnError(log, "[performSimilaritySearch] no vector provider configured for similarity search")
	}

	// Check if the active vector provider implements the StringQuerier interface.
	if _, ok := activeVectorProvider.(vector.StringQuerier); ok {
		log.Debugf("[%s] [performSimilaritySearch] active vector provider implements StringQuerier interface, performing string query", PLUGIN_NAME)
		return performStringQuery(key, queryString, ctx, c, log, stream)
	}

	// Check if the active vector provider implements the EmbeddingQuerier interface.
	if _, ok := activeVectorProvider.(vector.EmbeddingQuerier); ok {
		log.Debugf("[%s] [performSimilaritySearch] active vector provider implements EmbeddingQuerier interface, performing embedding query", PLUGIN_NAME)
		return performEmbeddingQuery(key, ctx, c, log, stream)
	}

	return logAndReturnError(log, "[performSimilaritySearch] no suitable querier or embedding provider available for similarity search")
}

// performStringQuery executes the string-based similarity search.
func performStringQuery(key string, queryString string, ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log, stream bool) error {
	stringQuerier, ok := c.GetVectorProvider().(vector.StringQuerier)
	if !ok {
		return logAndReturnError(log, "[performStringQuery] active vector provider does not implement StringQuerier interface")
	}

	return stringQuerier.QueryString(queryString, ctx, log, func(results []vector.QueryResult, ctx wrapper.HttpContext, log logs.Log, err error) {
		handleQueryResults(key, results, ctx, log, stream, c, err)
	})
}

// performEmbeddingQuery executes the embedding-based similarity search.
func performEmbeddingQuery(key string, ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log, stream bool) error {
	embeddingQuerier, ok := c.GetVectorProvider().(vector.EmbeddingQuerier)
	if !ok {
		return logAndReturnError(log, fmt.Sprintf("[performEmbeddingQuery] active vector provider does not implement EmbeddingQuerier interface"))
	}

	activeEmbeddingProvider := c.GetEmbeddingProvider()
	if activeEmbeddingProvider == nil {
		return logAndReturnError(log, fmt.Sprintf("[performEmbeddingQuery] no embedding provider configured for similarity search"))
	}

	return activeEmbeddingProvider.GetEmbedding(key, ctx, func(textEmbedding []float64, err error) {
		log.Debugf("[%s] [performEmbeddingQuery] GetEmbedding success, length of embedding: %d, error: %v", PLUGIN_NAME, len(textEmbedding), err)
		if err != nil {
			handleInternalError(err, fmt.Sprintf("[%s] [performEmbeddingQuery] error getting embedding, cache_id=%s", PLUGIN_NAME, cacheLogID(key)), log)
			return
		}
		ctx.SetContext(CACHE_KEY_EMBEDDING_KEY, textEmbedding)

		err = embeddingQuerier.QueryEmbedding(textEmbedding, ctx, log, func(results []vector.QueryResult, ctx wrapper.HttpContext, log logs.Log, err error) {
			handleQueryResults(key, results, ctx, log, stream, c, err)
		})
		if err != nil {
			handleInternalError(err, fmt.Sprintf("[%s] [performEmbeddingQuery] error querying vector database, cache_id=%s", PLUGIN_NAME, cacheLogID(key)), log)
		}
	})
}

// handleQueryResults processes the results of similarity search and determines next actions.
func handleQueryResults(key string, results []vector.QueryResult, ctx wrapper.HttpContext, log logs.Log, stream bool, c config.PluginConfig, err error) {
	cacheID := cacheLogID(key)
	if err != nil {
		handleInternalError(err, fmt.Sprintf("[%s] [handleQueryResults] error querying vector database, cache_id=%s", PLUGIN_NAME, cacheID), log)
		return
	}

	if len(results) == 0 {
		log.Warnf("[%s] [handleQueryResults] no similar keys found, cache_id=%s", PLUGIN_NAME, cacheID)
		proxywasm.ResumeHttpRequest()
		return
	}

	mostSimilarData := results[0]
	candidateID := cacheLogID(mostSimilarData.Text)
	log.Debugf("[%s] [handleQueryResults] most similar candidate found, cache_id=%s, candidate_id=%s, score=%f", PLUGIN_NAME, cacheID, candidateID, mostSimilarData.Score)
	simThreshold := c.GetVectorProviderConfig().Threshold
	simThresholdRelation := c.GetVectorProviderConfig().ThresholdRelation
	if compare(simThresholdRelation, mostSimilarData.Score, simThreshold) {
		log.Infof("[%s] semantic candidate accepted, cache_id=%s, candidate_id=%s, score=%f", PLUGIN_NAME, cacheID, candidateID, mostSimilarData.Score)
		if mostSimilarData.Answer != "" {
			// direct return the answer if available
			cacheCompatibilityResponse(ctx, c, key, mostSimilarData.Answer, log)
			processCacheHit(key, mostSimilarData.Answer, stream, ctx, c, log)
		} else {
			if c.GetCacheProvider() != nil {
				CheckCacheForKey(mostSimilarData.Text, ctx, c, log, stream, false)
			} else {
				// Otherwise, do not check the cache, directly return
				log.Infof("[%s] semantic cache candidate has no answer and no cache provider, candidate_id=%s", PLUGIN_NAME, candidateID)
				proxywasm.ResumeHttpRequest()
			}
		}
	} else {
		log.Infof("[%s] semantic score below threshold, cache_id=%s, candidate_id=%s, threshold=%f, score=%f", PLUGIN_NAME, cacheID, candidateID, simThreshold, mostSimilarData.Score)
		proxywasm.ResumeHttpRequest()
	}
}

// logAndReturnError logs an error and returns it.
func logAndReturnError(log logs.Log, message string) error {
	message = fmt.Sprintf("[%s] %s", PLUGIN_NAME, message)
	log.Errorf(message)
	return errors.New(message)
}

// handleInternalError logs an error and resumes the HTTP request.
func handleInternalError(err error, message string, log logs.Log) {
	if err != nil {
		log.Errorf("[%s] [handleInternalError] %s: %v", PLUGIN_NAME, message, err)
	} else {
		log.Errorf("[%s] [handleInternalError] %s", PLUGIN_NAME, message)
	}
	// proxywasm.SendHttpResponse(500, [][2]string{{"content-type", "text/plain"}}, []byte("Internal Server Error"), -1)
	proxywasm.ResumeHttpRequest()
}

// Caches non-authoritative compatibility values that cannot provide structured usage/model facts.
func cacheCompatibilityResponse(ctx wrapper.HttpContext, c config.PluginConfig, key string, value string, log logs.Log) {
	cacheID := cacheLogID(key)
	if strings.TrimSpace(value) == "" {
		log.Warnf("[%s] [cacheCompatibilityResponse] cached value is empty, cache_id=%s", PLUGIN_NAME, cacheID)
		return
	}

	activeCacheProvider := c.GetCacheProvider()
	if activeCacheProvider != nil {
		keyMaterial := ScopedCacheKeyMaterialForLookup(ctx, c, key)
		if !keyMaterial.Valid {
			log.Warnf("[%s] [cacheCompatibilityResponse] skip cache write, cache_id=%s, reason: %s", PLUGIN_NAME, cacheID, keyMaterial.BypassReason)
			return
		}
		queryKey := activeCacheProvider.GetCacheKeyPrefix() + keyMaterial.EntryKey
		_ = activeCacheProvider.Set(queryKey, value, nil)
		log.Debugf("[%s] [cacheCompatibilityResponse] compatibility cache set success, cache_id=%s, length of value: %d", PLUGIN_NAME, cacheLogID(queryKey), len(value))
	}
}

// Caches the response value. It returns true when an async cache write was dispatched.
func cacheResponse(ctx wrapper.HttpContext, c config.PluginConfig, key string, value string, log logs.Log, onStored func(bool)) bool {
	cacheID := cacheLogID(key)
	if strings.TrimSpace(value) == "" {
		log.Warnf("[%s] [cacheResponse] cached value is empty, cache_id=%s", PLUGIN_NAME, cacheID)
		return false
	}

	activeCacheProvider := c.GetCacheProvider()
	if activeCacheProvider != nil {
		keyMaterial := ScopedCacheKeyMaterialForLookup(ctx, c, key)
		if !keyMaterial.Valid {
			log.Warnf("[%s] [cacheResponse] skip cache write, cache_id=%s, reason: %s", PLUGIN_NAME, cacheID, keyMaterial.BypassReason)
			return false
		}
		queryKey := activeCacheProvider.GetCacheKeyPrefix() + keyMaterial.EntryKey
		entry := NewStructuredCacheEntryForAnswer(value, keyMaterial)
		if responseBody, ok := ctx.GetContext(CACHE_RESPONSE_BODY_CONTEXT_KEY).([]byte); ok {
			entry = NewStructuredCacheEntryForOpenAIResponse(responseBody, value, keyMaterial)
			ctx.SetContext(CACHE_RESPONSE_BODY_CONTEXT_KEY, nil)
		} else if streamFacts, ok := ctx.GetContext(CACHE_STREAM_FACTS_CONTEXT_KEY).(StructuredStreamingCacheFacts); ok && streamFacts.HasReplayFacts() {
			entry = NewStructuredCacheEntryForOpenAIStreamingResponse(streamFacts, value, keyMaterial)
			ctx.SetContext(CACHE_STREAM_FACTS_CONTEXT_KEY, nil)
		}
		ApplyStructuredCacheEntryExpiry(&entry, time.Now().Unix(), c.CacheSoftTTLSeconds(), c.CacheHardTTLSeconds())
		payload, err := EncodeRedisCacheEntryPayload(entry)
		if err != nil {
			log.Errorf("[%s] [cacheResponse] failed to encode structured cache entry, cache_id=%s, error: %v", PLUGIN_NAME, cacheLogID(queryKey), err)
			return false
		}
		ctx.SetContext(CACHE_VECTOR_PAYLOAD_METADATA_CONTEXT_KEY, newVectorPayloadMetadata(queryKey, entry))
		ctx.SetContext(CACHE_WRITE_PENDING_CONTEXT_KEY, true)
		if err := activeCacheProvider.Set(queryKey, payload, func(response resp.Value) {
			if response.Error() != nil {
				log.Warnf("[%s] [cacheResponse] cache set failed, cache_id=%s, error: %v", PLUGIN_NAME, cacheLogID(queryKey), response.Error())
				if onStored != nil {
					onStored(false)
				}
				return
			}
			log.Debugf("[%s] [cacheResponse] cache set success, cache_id=%s, length of value: %d", PLUGIN_NAME, cacheLogID(queryKey), len(payload))
			if onStored != nil {
				onStored(true)
			}
		}); err != nil {
			ctx.SetContext(CACHE_WRITE_PENDING_CONTEXT_KEY, nil)
			log.Warnf("[%s] [cacheResponse] cache set dispatch failed, cache_id=%s, error: %v", PLUGIN_NAME, cacheLogID(queryKey), err)
			return false
		}
		return true
	}
	return false
}

// Handles embedding upload if available
func uploadEmbeddingAndAnswer(ctx wrapper.HttpContext, c config.PluginConfig, key string, value string, log logs.Log) {
	embedding := ctx.GetContext(CACHE_KEY_EMBEDDING_KEY)
	if embedding == nil {
		return
	}

	emb, ok := embedding.([]float64)
	if !ok {
		log.Errorf("[%s] [uploadEmbeddingAndAnswer] embedding is not of expected type []float64", PLUGIN_NAME)
		return
	}

	activeVectorProvider := c.GetVectorProvider()
	if activeVectorProvider == nil {
		log.Debugf("[%s] [uploadEmbeddingAndAnswer] no vector provider configured for uploading embedding", PLUGIN_NAME)
		return
	}

	// Attempt to upload answer embedding first
	if metadata, ok := ctx.GetContext(CACHE_VECTOR_PAYLOAD_METADATA_CONTEXT_KEY).(vector.PayloadMetadata); ok && metadata.CacheEntryKey != "" {
		if metadataUploader, ok := activeVectorProvider.(vector.AnswerAndEmbeddingMetadataUploader); ok {
			log.Infof("[%s] uploading answer embedding with cache metadata, cache_id=%s", PLUGIN_NAME, cacheLogID(metadata.CacheEntryKey))
			err := metadataUploader.UploadAnswerAndEmbeddingWithMetadata(key, emb, value, metadata, ctx, log, nil)
			if err != nil {
				log.Warnf("[%s] [uploadEmbeddingAndAnswer] failed to upload answer embedding with cache metadata, cache_id=%s, error: %v", PLUGIN_NAME, cacheLogID(metadata.CacheEntryKey), err)
			} else {
				return
			}
		}
	}

	if ansEmbUploader, ok := activeVectorProvider.(vector.AnswerAndEmbeddingUploader); ok {
		log.Infof("[%s] uploading answer embedding, cache_id=%s", PLUGIN_NAME, cacheLogID(key))
		err := ansEmbUploader.UploadAnswerAndEmbedding(key, emb, value, ctx, log, nil)
		if err != nil {
			log.Warnf("[%s] [uploadEmbeddingAndAnswer] failed to upload answer embedding, cache_id=%s, error: %v", PLUGIN_NAME, cacheLogID(key), err)
		} else {
			return // If successful, return early
		}
	}

	// If answer embedding upload fails, attempt normal embedding upload
	if embUploader, ok := activeVectorProvider.(vector.EmbeddingUploader); ok {
		log.Infof("[%s] uploading embedding, cache_id=%s", PLUGIN_NAME, cacheLogID(key))
		err := embUploader.UploadEmbedding(key, emb, ctx, log, nil)
		if err != nil {
			log.Warnf("[%s] [uploadEmbeddingAndAnswer] failed to upload embedding, cache_id=%s, error: %v", PLUGIN_NAME, cacheLogID(key), err)
		}
	}
}

func newVectorPayloadMetadata(cacheEntryKey string, entry StructuredCacheEntry) vector.PayloadMetadata {
	model := normalizeOr(entry.Model, entry.Provider.Model)
	providerModel := normalizeOr(entry.Provider.Model, model)
	return vector.PayloadMetadata{
		CacheEntryKey: cacheEntryKey,
		Scope: vector.PayloadScope{
			Type:     entry.Scope.Type,
			Tenant:   entry.Scope.Tenant,
			Consumer: entry.Scope.Consumer,
		},
		Route: vector.PayloadRoute{
			ID:      entry.Route.ID,
			Name:    entry.Route.Name,
			APIName: entry.Route.APIName,
		},
		Provider: vector.PayloadProvider{
			Name:  entry.Provider.Name,
			Model: providerModel,
		},
		Model:         model,
		PolicyVersion: entry.PolicyVersion,
		RequestFingerprint: vector.PayloadRequestFingerprint{
			Hash: entry.RequestFingerprint.Hash,
		},
		Semantic: vector.PayloadSemantic{
			TextHash: entry.Semantic.TextHash,
		},
		SoftExpiresAt: entry.SoftExpiresAt,
		HardExpiresAt: entry.HardExpiresAt,
	}
}

func cacheLogID(key string) string {
	return collapseLogKey(key)
}

func cacheEntryLogID(cacheEntryKey string, key string) string {
	if cacheEntryKey != "" {
		return cacheLogID(cacheEntryKey)
	}
	return cacheLogID(key)
}

// 主要用于相似度/距离/点积判断
// 余弦相似度度量的是两个向量在方向上的相似程度。相似度越高，两个向量越接近。
// 距离度量的是两个向量在空间上的远近程度。距离越小，两个向量越接近。
// compare 函数根据操作符进行判断并返回结果
func compare(operator string, value1 float64, value2 float64) bool {
	switch operator {
	case "gt":
		return value1 > value2
	case "gte":
		return value1 >= value2
	case "lt":
		return value1 < value2
	case "lte":
		return value1 <= value2
	default:
		return false
	}
}
