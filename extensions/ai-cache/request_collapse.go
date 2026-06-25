package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

type requestCollapseStore struct {
	mu       sync.Mutex
	inFlight map[string]int
}

type requestCollapseLease struct {
	CacheEntryKey     string
	LockKey           string
	NotifyKey         string
	Token             string
	LocalLeader       bool
	DistributedLocked bool
}

func newRequestCollapseStore() *requestCollapseStore {
	return &requestCollapseStore{
		inFlight: map[string]int{},
	}
}

func (s *requestCollapseStore) Begin(cacheEntryKey string) (requestCollapseLease, bool) {
	if s == nil || cacheEntryKey == "" {
		return requestCollapseLease{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.inFlight[cacheEntryKey]; exists {
		s.inFlight[cacheEntryKey]++
		return requestCollapseLease{CacheEntryKey: cacheEntryKey}, false
	}
	s.inFlight[cacheEntryKey] = 0
	return requestCollapseLease{
		CacheEntryKey: cacheEntryKey,
		LocalLeader:   true,
	}, true
}

func (s *requestCollapseStore) End(cacheEntryKey string) int {
	if s == nil || cacheEntryKey == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	waiters := s.inFlight[cacheEntryKey]
	delete(s.inFlight, cacheEntryKey)
	return waiters
}

func handleRequestCollapseMiss(cacheEntryKey string, lockKey string, key string, missResponse resp.Value, ctx wrapper.HttpContext, log logs.Log, stream bool, c config.PluginConfig) bool {
	if !c.RequestCollapse.Enabled || cacheEntryKey == "" || c.GetCacheProvider() == nil {
		return false
	}
	store := configuredRequestCollapseStore(c)
	if store == nil {
		return false
	}

	staleEntry, hasStaleFallback := staleFallbackEntryFromResponse(missResponse, c)
	notifyKey := collapseNotifyKey(cacheEntryKey)
	cacheID := collapseLogKey(cacheEntryKey)
	lease, leader := store.Begin(cacheEntryKey)
	if !leader {
		log.Infof("[%s] request collapse follower waits, cache_id=%s", PLUGIN_NAME, cacheID)
		ctx.SetUserAttribute("cache_lock_behavior", "waiter")
		ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
		waitForCollapsedCache(cacheEntryKey, notifyKey, key, ctx, log, stream, c, staleEntry, hasStaleFallback)
		return true
	}

	lease.LockKey = lockKey
	lease.NotifyKey = notifyKey
	lease.Token = newRequestCollapseToken(cacheEntryKey)
	if c.RequestCollapse.DistributedLock && lockKey != "" {
		if err := c.GetCacheProvider().SetNX(lockKey, lease.Token, c.RequestCollapse.LockTTLSeconds, func(response resp.Value) {
			if redisSetNXAcquired(response) {
				lease.DistributedLocked = true
				ctx.SetContext(CACHE_REQUEST_COLLAPSE_LEASE_CONTEXT_KEY, lease)
				ctx.SetUserAttribute("cache_lock_behavior", "leader")
				ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
				log.Infof("[%s] acquired request collapse lock, cache_id=%s", PLUGIN_NAME, cacheID)
				proxywasm.ResumeHttpRequest()
				return
			}
			store.End(cacheEntryKey)
			ctx.SetUserAttribute("cache_lock_behavior", "waiter")
			ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
			if response.Error() != nil {
				log.Warnf("[%s] request collapse lock failed, cache_id=%s, error: %v", PLUGIN_NAME, cacheID, response.Error())
			} else {
				log.Infof("[%s] request collapse lock already held, cache_id=%s", PLUGIN_NAME, cacheID)
			}
			waitForCollapsedCache(cacheEntryKey, notifyKey, key, ctx, log, stream, c, staleEntry, hasStaleFallback)
		}); err != nil {
			store.End(cacheEntryKey)
			log.Warnf("[%s] request collapse lock dispatch failed, cache_id=%s, error: %v", PLUGIN_NAME, cacheID, err)
			return false
		}
		return true
	}

	ctx.SetContext(CACHE_REQUEST_COLLAPSE_LEASE_CONTEXT_KEY, lease)
	ctx.SetUserAttribute("cache_lock_behavior", "leader")
	ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
	log.Infof("[%s] request collapse local leader proceeds, cache_id=%s", PLUGIN_NAME, cacheID)
	proxywasm.ResumeHttpRequest()
	return true
}

func redisSetNXAcquired(response resp.Value) bool {
	if response.Error() != nil {
		return false
	}
	if strings.EqualFold(response.String(), "OK") {
		return true
	}
	if response.Integer() == 1 || response.Bool() {
		return true
	}
	return false
}

func waitForCollapsedCache(cacheEntryKey string, notifyKey string, key string, ctx wrapper.HttpContext, log logs.Log, stream bool, c config.PluginConfig, staleEntry StructuredCacheEntry, hasStaleFallback bool) {
	attempts := collapseWaitAttempts(c)
	activeCacheProvider := c.GetCacheProvider()
	cacheID := collapseLogKey(cacheEntryKey)
	var poll func(onMiss func())
	poll = func(onMiss func()) {
		if activeCacheProvider == nil {
			serveCollapseFallbackOrResume(cacheEntryKey, key, ctx, log, stream, staleEntry, hasStaleFallback)
			return
		}
		if err := activeCacheProvider.Get(cacheEntryKey, func(response resp.Value) {
			if handleCacheHitResponse(cacheEntryKey, key, response, ctx, log, stream, c) {
				ctx.SetUserAttribute("cache_lock_behavior", "wait_hit")
				ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
				return
			}
			onMiss()
		}); err != nil {
			log.Warnf("[%s] request collapse wait lookup dispatch failed, cache_id=%s, error: %v", PLUGIN_NAME, cacheID, err)
			serveCollapseFallbackOrResume(cacheEntryKey, key, ctx, log, stream, staleEntry, hasStaleFallback)
		}
	}
	var wait func(remaining int)
	wait = func(remaining int) {
		if remaining <= 0 || activeCacheProvider == nil {
			serveCollapseFallbackOrResume(cacheEntryKey, key, ctx, log, stream, staleEntry, hasStaleFallback)
			return
		}
		waitMiss := func() {
			wait(remaining - 1)
		}
		if notifyKey == "" {
			poll(waitMiss)
			return
		}
		err := activeCacheProvider.Command([]interface{}{"blpop", notifyKey, collapseWaitTimeoutArgument(c)}, func(response resp.Value) {
			if response.Error() != nil {
				log.Warnf("[%s] request collapse wait failed, cache_id=%s, error: %v", PLUGIN_NAME, cacheID, response.Error())
			}
			poll(waitMiss)
		})
		if err != nil {
			log.Warnf("[%s] request collapse wait dispatch failed, cache_id=%s, error: %v", PLUGIN_NAME, cacheID, err)
			poll(waitMiss)
		}
	}
	wait(attempts)
}

func serveCollapseFallbackOrResume(cacheEntryKey string, key string, ctx wrapper.HttpContext, log logs.Log, stream bool, staleEntry StructuredCacheEntry, hasStaleFallback bool) {
	cacheID := collapseLogKey(cacheEntryKey)
	if hasStaleFallback {
		log.Infof("[%s] request collapse stale fallback, cache_id=%s", PLUGIN_NAME, cacheID)
		ctx.SetUserAttribute("cache_lock_behavior", "stale_fallback")
		ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
		if stream {
			processStructuredStreamingCacheHit(key, staleEntry, ctx, log)
			return
		}
		processStructuredCacheHit(key, staleEntry, ctx, log)
		return
	}
	log.Infof("[%s] request collapse wait exhausted, cache_id=%s", PLUGIN_NAME, cacheID)
	ctx.SetUserAttribute("cache_lock_behavior", "wait_timeout")
	ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
	proxywasm.ResumeHttpRequest()
}

func releaseRequestCollapse(ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log) {
	releaseRequestCollapseWithNotify(ctx, c, log, false)
}

func releaseRequestCollapseWithNotify(ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log, notify bool) {
	raw := ctx.GetContext(CACHE_REQUEST_COLLAPSE_LEASE_CONTEXT_KEY)
	if raw == nil {
		return
	}
	lease, ok := raw.(requestCollapseLease)
	ctx.SetContext(CACHE_REQUEST_COLLAPSE_LEASE_CONTEXT_KEY, nil)
	if !ok {
		return
	}
	waiters := 0
	if lease.LocalLeader {
		if store := configuredRequestCollapseStore(c); store != nil {
			waiters = store.End(lease.CacheEntryKey)
		}
	}
	if notify {
		notifyCollapsedWaiters(lease.NotifyKey, waiters, lease.DistributedLocked, c, log)
	}
	if lease.DistributedLocked && lease.LockKey != "" && c.GetCacheProvider() != nil {
		if err := c.GetCacheProvider().Unlock(lease.LockKey, lease.Token, nil); err != nil {
			log.Warnf("[%s] request collapse unlock dispatch failed, cache_id=%s, error: %v", PLUGIN_NAME, collapseLogKey(lease.CacheEntryKey), err)
		}
	}
}

func staleFallbackEntryFromResponse(response resp.Value, c config.PluginConfig) (StructuredCacheEntry, bool) {
	if !c.RequestCollapse.StaleFallback || response.Error() != nil || response.IsNull() {
		return StructuredCacheEntry{}, false
	}
	decoded, err := DecodeRedisCacheEntryPayload(redisCacheValueString(response))
	if err != nil || !decoded.IsAuthoritative() {
		return StructuredCacheEntry{}, false
	}
	entry := *decoded.StructuredEntry
	if StructuredCacheEntryFreshnessAt(entry, time.Now().Unix()) != StructuredCacheFreshnessStale {
		return StructuredCacheEntry{}, false
	}
	return entry, true
}

func configuredRequestCollapseStore(c config.PluginConfig) *requestCollapseStore {
	if !c.RequestCollapse.Enabled {
		return nil
	}
	store, ok := c.GetRequestCollapseStore().(*requestCollapseStore)
	if !ok {
		return nil
	}
	return store
}

func newRequestCollapseToken(cacheEntryKey string) string {
	return fmt.Sprintf("%d:%s", time.Now().UnixNano(), collapseLogKey(cacheEntryKey))
}

func collapseWaitAttempts(c config.PluginConfig) int {
	if c.RequestCollapse.WaitAttempts <= 0 {
		return config.DEFAULT_COLLAPSE_WAIT_ATTEMPTS
	}
	return c.RequestCollapse.WaitAttempts
}

func collapseWaitTimeoutArgument(c config.PluginConfig) string {
	waitMilliseconds := c.RequestCollapse.WaitMilliseconds
	if waitMilliseconds <= 0 {
		waitMilliseconds = config.DEFAULT_COLLAPSE_WAIT_MILLISECONDS
	}
	seconds := float64(waitMilliseconds) / 1000
	return strconv.FormatFloat(seconds, 'f', -1, 64)
}

func collapseNotifyExpireSeconds(c config.PluginConfig) int {
	waitMilliseconds := c.RequestCollapse.WaitMilliseconds
	if waitMilliseconds <= 0 {
		waitMilliseconds = config.DEFAULT_COLLAPSE_WAIT_MILLISECONDS
	}
	return (waitMilliseconds+999)/1000 + 1
}

func collapseNotifyKey(cacheEntryKey string) string {
	if cacheEntryKey == "" {
		return ""
	}
	if strings.Contains(cacheEntryKey, ":entry:") {
		return strings.Replace(cacheEntryKey, ":entry:", ":notify:", 1)
	}
	return cacheEntryKey + ":notify"
}

func collapseLogKey(cacheEntryKey string) string {
	if cacheEntryKey == "" {
		return "unknown"
	}
	sum := sha256.Sum256([]byte(cacheEntryKey))
	return hex.EncodeToString(sum[:])[:16]
}

func notifyCollapsedWaiters(notifyKey string, waiterCount int, mayHaveRemoteWaiters bool, c config.PluginConfig, log logs.Log) {
	activeCacheProvider := c.GetCacheProvider()
	if notifyKey == "" || activeCacheProvider == nil {
		return
	}
	if waiterCount <= 0 && !mayHaveRemoteWaiters {
		return
	}
	notifyCount := waiterCount
	if notifyCount < 1 {
		notifyCount = 1
	}
	if notifyCount > 16 {
		notifyCount = 16
	}
	args := []interface{}{"rpush", notifyKey}
	for i := 0; i < notifyCount; i++ {
		args = append(args, "ready")
	}
	if err := activeCacheProvider.Command(args, nil); err != nil {
		log.Warnf("[%s] request collapse notify dispatch failed, notify_id=%s, error: %v", PLUGIN_NAME, collapseLogKey(notifyKey), err)
		return
	}
	if err := activeCacheProvider.Command([]interface{}{"expire", notifyKey, collapseNotifyExpireSeconds(c)}, nil); err != nil {
		log.Warnf("[%s] request collapse notify expiry dispatch failed, notify_id=%s, error: %v", PLUGIN_NAME, collapseLogKey(notifyKey), err)
	}
}
