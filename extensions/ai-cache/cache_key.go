package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	CacheScopeTenant   = config.CACHE_SCOPE_TENANT
	CacheScopeConsumer = config.CACHE_SCOPE_CONSUMER
	CacheScopeGlobal   = config.CACHE_SCOPE_GLOBAL

	unknownRoute = "unknown-route"
	unknownModel = "unknown-model"
)

type ScopedCacheKeyInput struct {
	ScopeType     string
	Tenant        string
	Consumer      string
	RouteID       string
	RouteName     string
	APIName       string
	Provider      string
	Model         string
	PolicyVersion string
	LegacyKey     string
	Body          gjson.Result
}

type ScopedCacheKeyMaterial struct {
	Valid              bool
	BypassReason       string
	LegacyKey          string
	EntryKey           string
	LockKey            string
	TagKey             string
	MetadataKey        string
	ScopeHashTag       string
	Scope              StructuredCacheScope
	Route              StructuredRouteFacts
	Provider           StructuredProviderFacts
	PolicyVersion      string
	RequestFingerprint RequestFingerprint
}

func BuildScopedCacheKeyMaterial(c config.PluginConfig, body gjson.Result, legacyKey string) ScopedCacheKeyMaterial {
	tenant, _ := proxywasm.GetHttpRequestHeader(c.TenantHeader)
	consumer, _ := proxywasm.GetHttpRequestHeader(c.ConsumerHeader)
	routeName := getProxyWasmPropertyString([]string{"route_name"})
	return BuildScopedCacheKeyMaterialFromInput(ScopedCacheKeyInput{
		ScopeType:     c.CacheScope,
		Tenant:        tenant,
		Consumer:      consumer,
		RouteName:     routeName,
		Provider:      c.Provider,
		PolicyVersion: c.CachePolicyVersion,
		LegacyKey:     legacyKey,
		Body:          body,
	})
}

func ScopedCacheKeyMaterialForLookup(ctx wrapper.HttpContext, c config.PluginConfig, key string) ScopedCacheKeyMaterial {
	if raw := ctx.GetContext(CACHE_KEY_MATERIAL_CONTEXT_KEY); raw != nil {
		if material, ok := raw.(ScopedCacheKeyMaterial); ok {
			if material.LegacyKey == key {
				return material
			}
			return material.ForLegacyKey(key)
		}
	}
	return BuildScopedCacheKeyMaterialFromInput(ScopedCacheKeyInput{
		ScopeType:     c.CacheScope,
		Provider:      c.Provider,
		PolicyVersion: c.CachePolicyVersion,
		LegacyKey:     key,
	})
}

func BuildScopedCacheKeyMaterialFromInput(input ScopedCacheKeyInput) ScopedCacheKeyMaterial {
	scope, valid, bypassReason := normalizeCacheScope(input.ScopeType, input.Tenant, input.Consumer)
	if !valid {
		return ScopedCacheKeyMaterial{
			Valid:        false,
			BypassReason: bypassReason,
			LegacyKey:    input.LegacyKey,
			Scope:        scope,
		}
	}
	route := StructuredRouteFacts{
		ID:      strings.TrimSpace(input.RouteID),
		Name:    normalizeOr(input.RouteName, unknownRoute),
		APIName: strings.TrimSpace(input.APIName),
	}
	model := normalizeOr(input.Model, strings.TrimSpace(input.Body.Get("model").String()), unknownModel)
	providerName := strings.TrimSpace(input.Provider)
	if providerName == "" {
		return ScopedCacheKeyMaterial{
			Valid:        false,
			BypassReason: "missing provider identity",
			LegacyKey:    input.LegacyKey,
			Scope:        scope,
			Route:        route,
			Provider:     StructuredProviderFacts{Model: model},
		}
	}
	policyVersion := normalizeOr(input.PolicyVersion, config.DEFAULT_CACHE_POLICY_VERSION)
	fingerprint := buildRequestFingerprint(input.Body, input.LegacyKey)

	scopeHashTag := "aiscope:" + stableHash(scope.Type, scope.Tenant, scope.Consumer)
	identityHash := stableHash(
		route.ID,
		route.Name,
		route.APIName,
		providerName,
		model,
		policyVersion,
		fingerprint.Hash,
	)

	return ScopedCacheKeyMaterial{
		Valid:              true,
		LegacyKey:          input.LegacyKey,
		EntryKey:           scopedRedisKey(scopeHashTag, "entry", identityHash),
		LockKey:            scopedRedisKey(scopeHashTag, "lock", identityHash),
		TagKey:             scopedRedisKey(scopeHashTag, "tag", identityHash),
		MetadataKey:        scopedRedisKey(scopeHashTag, "metadata", identityHash),
		ScopeHashTag:       scopeHashTag,
		Scope:              scope,
		Route:              route,
		Provider:           StructuredProviderFacts{Name: providerName, Model: model},
		PolicyVersion:      policyVersion,
		RequestFingerprint: fingerprint,
	}
}

func (m ScopedCacheKeyMaterial) ForLegacyKey(key string) ScopedCacheKeyMaterial {
	return BuildScopedCacheKeyMaterialFromInput(ScopedCacheKeyInput{
		ScopeType:     m.Scope.Type,
		Tenant:        m.Scope.Tenant,
		Consumer:      m.Scope.Consumer,
		RouteID:       m.Route.ID,
		RouteName:     m.Route.Name,
		APIName:       m.Route.APIName,
		Provider:      m.Provider.Name,
		Model:         m.Provider.Model,
		PolicyVersion: m.PolicyVersion,
		LegacyKey:     key,
	})
}

func normalizeCacheScope(scopeType string, tenant string, consumer string) (StructuredCacheScope, bool, string) {
	scopeType = normalizeOr(scopeType, CacheScopeTenant)
	tenant = strings.TrimSpace(tenant)
	consumer = strings.TrimSpace(consumer)
	switch scopeType {
	case CacheScopeGlobal:
		return StructuredCacheScope{Type: CacheScopeGlobal}, true, ""
	case CacheScopeConsumer:
		if tenant == "" {
			return StructuredCacheScope{Type: CacheScopeConsumer}, false, "missing tenant identity for consumer cache scope"
		}
		if consumer == "" {
			return StructuredCacheScope{Type: CacheScopeConsumer, Tenant: tenant}, false, "missing consumer identity for consumer cache scope"
		}
		return StructuredCacheScope{
			Type:     CacheScopeConsumer,
			Tenant:   tenant,
			Consumer: consumer,
		}, true, ""
	default:
		if tenant == "" {
			return StructuredCacheScope{Type: CacheScopeTenant}, false, "missing tenant identity for tenant cache scope"
		}
		return StructuredCacheScope{
			Type:   CacheScopeTenant,
			Tenant: tenant,
		}, true, ""
	}
}

func buildRequestFingerprint(body gjson.Result, legacyKey string) RequestFingerprint {
	messagesHash := componentHash(body, "messages")
	systemHash := systemMessagesHash(body)
	toolsHash := componentHash(body, "tools", "tool_choice", "functions", "function_call")
	samplingHash := componentHash(
		body,
		"temperature",
		"top_p",
		"top_k",
		"n",
		"seed",
		"stop",
		"presence_penalty",
		"frequency_penalty",
		"max_tokens",
		"max_completion_tokens",
	)
	outputFormatHash := componentHash(body, "response_format", "modalities", "audio")
	return RequestFingerprint{
		Hash: stableHash(
			legacyKey,
			body.Get("model").String(),
			messagesHash,
			systemHash,
			toolsHash,
			samplingHash,
			outputFormatHash,
		),
		MessagesHash:     messagesHash,
		SystemHash:       systemHash,
		ToolsHash:        toolsHash,
		SamplingHash:     samplingHash,
		OutputFormatHash: outputFormatHash,
	}
}

func componentHash(body gjson.Result, paths ...string) string {
	components := make(map[string]interface{})
	for _, path := range paths {
		result := body.Get(path)
		if !result.Exists() {
			continue
		}
		value, ok := canonicalValue(result.Raw)
		if !ok {
			continue
		}
		components[path] = value
	}
	if len(components) == 0 {
		return ""
	}
	return stableHash(canonicalJSON(components))
}

func systemMessagesHash(body gjson.Result) string {
	messages := body.Get("messages").Array()
	if len(messages) == 0 {
		return ""
	}
	var systems []interface{}
	for _, message := range messages {
		role := message.Get("role").String()
		if role != "system" && role != "developer" {
			continue
		}
		value, ok := canonicalValue(message.Raw)
		if ok {
			systems = append(systems, value)
		}
	}
	if len(systems) == 0 {
		return ""
	}
	return stableHash(canonicalJSON(systems))
}

func canonicalValue(raw string) (interface{}, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, false
	}
	return value, true
}

func canonicalJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func scopedRedisKey(scopeHashTag string, kind string, identityHash string) string {
	return fmt.Sprintf("{%s}:%s:%s", scopeHashTag, kind, identityHash)
}

func stableHash(parts ...string) string {
	hasher := sha256.New()
	for _, part := range parts {
		_, _ = hasher.Write([]byte(part))
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))[:32]
}

func normalizeOr(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func getProxyWasmPropertyString(path []string) string {
	value, err := proxywasm.GetProperty(path)
	if err != nil {
		return ""
	}
	return string(value)
}
