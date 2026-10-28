package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

const (
	memoryRecentSchemaVersion = 1
	maxRecentRecordBytes      = 256 * 1024
	maxRecentMessageBytes     = 64 * 1024
)

var memoryLoadRecentMemory = loadRecentMemory

type memoryRecentRecord struct {
	SchemaVersion int                   `json:"schema_version"`
	Messages      []memoryRecentMessage `json:"messages"`
}

type memoryRecentMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func loadRecentMemory(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) error {
	stop := c.Route.RecentWindowTurns - 1
	if stop < 0 {
		stop = 0
	}
	consumer := ctx.GetStringContext(memoryConsumerContextKey, "")
	if c.NativeIdentity != nil {
		// Console materializes native user-owned memory with consumer_id=None.
		// Only its Redis key component is hashed; facts retain the principal UUID.
		consumer = fmt.Sprintf("consumer-name-sha256:%x", sha256.Sum256([]byte(consumer)))
	}
	key := buildRecentMemoryKey(
		c.RecentCache.KeyPrefix,
		ctx.GetStringContext(memoryTenantContextKey, ""),
		consumer,
	)
	client := c.GetRecentRedisClient()
	if client == nil {
		return errors.New("recent memory Redis client is not configured")
	}
	return client.ZRevRange(key, 0, stop, func(response resp.Value) {
		handleRecentMemoryResponse(key, response, ctx, c, log)
	})
}

func handleRecentMemoryResponse(key string, response resp.Value, ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) {
	if err := response.Error(); err != nil {
		log.Warnf("[ai-memory] recent memory lookup rejected for key %s: %v", key, err)
		continueAfterRecentMemory(ctx, c, log)
		return
	}
	if response.IsNull() {
		continueAfterRecentMemory(ctx, c, log)
		return
	}
	if response.Type() != resp.Array {
		log.Warnf("[ai-memory] recent memory lookup returned a non-array for key %s", key)
		continueAfterRecentMemory(ctx, c, log)
		return
	}
	messages, err := loadAndValidateRecentMemory(response.Array())
	if err != nil {
		log.Warnf("[ai-memory] recent memory record rejected for key %s: %v", key, err)
		continueAfterRecentMemory(ctx, c, log)
		return
	}
	ctx.SetContext(memoryRecentMessagesContextKey, messages)
	continueAfterRecentMemory(ctx, c, log)
}

func continueAfterRecentMemory(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) {
	if memoryConsoleAssembleConfigured(c) {
		if err := dispatchMemoryAssemble(ctx, c, log); err != nil {
			log.Warnf("[ai-memory] Console assemble dispatch failed open: %v", err)
			replaceMemoryRequestBodyWithRecentFallback(ctx, log)
			proxywasm.ResumeHttpRequest()
		}
		return
	}
	replaceMemoryRequestBodyWithRecentFallback(ctx, log)
	proxywasm.ResumeHttpRequest()
}

func buildRecentMemoryKey(prefix, tenant, consumer string) string {
	prefix = strings.TrimRight(prefix, ":")
	if prefix == "" {
		prefix = "memory:recent"
	}
	return strings.Join([]string{
		prefix,
		defaultKeyPart(tenant),
		defaultKeyPart(consumer),
	}, ":")
}

func defaultKeyPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func loadAndValidateRecentMemory(members []resp.Value) ([]sessionctx.OpenAIMessage, error) {
	messages := make([]sessionctx.OpenAIMessage, 0, len(members)*2)
	for i := len(members) - 1; i >= 0; i-- {
		memberMessages, err := loadAndValidateRecentMember(members[i].String())
		if err != nil {
			return nil, err
		}
		messages = append(messages, memberMessages...)
	}
	return messages, nil
}

func loadAndValidateRecentMember(raw string) ([]sessionctx.OpenAIMessage, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("empty recent memory record")
	}
	if len(raw) > maxRecentRecordBytes {
		return nil, errors.New("recent memory record too large")
	}
	var record memoryRecentRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return nil, errors.New("recent memory record is not valid JSON")
	}
	if err := validateRecentMemory(record); err != nil {
		return nil, err
	}
	return record.toOpenAIMessages(), nil
}

func validateRecentMemory(record memoryRecentRecord) error {
	if record.SchemaVersion != memoryRecentSchemaVersion {
		return errors.New("unsupported recent memory schema version")
	}
	for _, message := range record.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			return fmt.Errorf("unsupported recent memory role %q", message.Role)
		}
		if len(strings.TrimSpace(message.Content)) > maxRecentMessageBytes {
			return errors.New("recent memory message too large")
		}
	}
	return nil
}

func (r memoryRecentRecord) toOpenAIMessages() []sessionctx.OpenAIMessage {
	messages := make([]sessionctx.OpenAIMessage, 0, len(r.Messages))
	for _, message := range r.Messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		messages = append(messages, sessionctx.OpenAIMessage{
			Role:    message.Role,
			Content: content,
		})
	}
	return messages
}
