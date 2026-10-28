package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/memorycache"
)

type memoryTrustedCacheDigestPayload struct {
	SchemaVersion int                               `json:"schema_version"`
	Messages      []memoryTrustedCacheDigestMessage `json:"messages"`
}

type memoryTrustedCacheDigestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func removeCallerMemoryCacheDigest() {
	_ = proxywasm.RemoveHttpRequestHeader(memorycache.DeprecatedDigestHeader)
}

func writeMemoryCacheDigest(input memoryMessageAssemblyInput) error {
	digest, err := buildMemoryCacheDigest(input)
	if err != nil {
		return err
	}
	return proxywasm.SetProperty([]string{memorycache.TrustedDigestProperty}, []byte(digest))
}

func buildMemoryCacheDigest(input memoryMessageAssemblyInput) (string, error) {
	effective := input
	if countCurrentUserMessages(effective.Current) > 1 {
		effective.Recent = nil
	}
	messages := make([]memoryTrustedCacheDigestMessage, 0, 1+len(effective.Recent))
	appendMessage := func(role string, content interface{}) {
		contentText := strings.TrimSpace(memoryTextContent(content))
		if contentText == "" {
			return
		}
		role = strings.TrimSpace(role)
		if role == "" {
			role = "system"
		}
		messages = append(messages, memoryTrustedCacheDigestMessage{Role: role, Content: contentText})
	}
	if effective.MemoryMessage != nil {
		appendMessage(effective.MemoryMessage.Role, effective.MemoryMessage.Content)
	}
	for _, message := range effective.Recent {
		appendMessage(message.Role, message.Content)
	}
	// Callers establish the effective context first. An empty message list is
	// a known context too; it must remain distinct from a failed/unknown read.
	body, err := json.Marshal(memoryTrustedCacheDigestPayload{
		SchemaVersion: 1,
		Messages:      messages,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
