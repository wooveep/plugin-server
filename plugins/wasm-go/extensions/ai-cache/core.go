package main

import (
	"errors"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

func CheckMaterializedCacheForKey(material ScopedCacheKeyMaterial, ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log, stream bool) error {
	redisClient := c.GetMaterializedRedisClient()
	if redisClient == nil {
		return errors.New("materialized Redis client is not configured")
	}

	log.Debugf("[%s] [CheckMaterializedCacheForKey] querying materialized cache with key: %s", PLUGIN_NAME, material.RedisKey)
	if err := redisClient.Get(material.RedisKey, func(response resp.Value) {
		handleMaterializedCacheResponse(material, response, ctx, log, stream, c)
	}); err != nil {
		log.Errorf("[%s] [CheckMaterializedCacheForKey] failed to retrieve key: %s from materialized cache, error: %v", PLUGIN_NAME, material.RedisKey, err)
		return err
	}
	return nil
}

func handleMaterializedCacheResponse(material ScopedCacheKeyMaterial, response resp.Value, ctx wrapper.HttpContext, log logs.Log, stream bool, c config.PluginConfig) {
	if err := response.Error(); err != nil {
		log.Errorf("[%s] [handleMaterializedCacheResponse] error retrieving materialized key: %s, error: %v", PLUGIN_NAME, material.RedisKey, err)
		proxywasm.ResumeHttpRequest()
		return
	}
	if response.IsNull() {
		log.Infof("[%s] [handleMaterializedCacheResponse] materialized cache miss for key: %s", PLUGIN_NAME, material.RedisKey)
		if shouldUseConsoleLookup(c) {
			if err := lookupMaterializedRecordFromConsole(material, ctx, c, log, stream); err != nil {
				log.Warnf("[%s] [handleMaterializedCacheResponse] Console lookup dispatch failed for key: %s, error: %v", PLUGIN_NAME, material.RedisKey, err)
				proxywasm.ResumeHttpRequest()
			}
			return
		}
		proxywasm.ResumeHttpRequest()
		return
	}

	if !c.RoutePolicy.EnableReplay {
		log.Infof("[%s] [handleMaterializedCacheResponse] replay is disabled for materialized key: %s", PLUGIN_NAME, material.RedisKey)
		proxywasm.ResumeHttpRequest()
		return
	}

	record, err := LoadAndValidateMaterializedRecord(response.String(), material, stream)
	if err != nil {
		log.Warnf("[%s] [handleMaterializedCacheResponse] materialized cache record rejected for key: %s, reason: %v", PLUGIN_NAME, material.RedisKey, err)
		proxywasm.ResumeHttpRequest()
		return
	}
	if err := replayMaterializedRecord(record, stream, ctx, log); err != nil {
		log.Warnf("[%s] [handleMaterializedCacheResponse] materialized cache replay failed for key: %s, error: %v", PLUGIN_NAME, material.RedisKey, err)
		proxywasm.ResumeHttpRequest()
		return
	}
}
