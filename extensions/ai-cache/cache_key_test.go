package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildScopedCacheKeyMaterialUsesHashTaggedScopeAndNoRawPrompt(t *testing.T) {
	body := gjson.Parse(`{
		"model":"qwen-turbo",
		"messages":[
			{"role":"system","content":"answer briefly"},
			{"role":"user","content":"What is tenant-a revenue?"}
		],
		"temperature":0.2,
		"top_p":0.9,
		"tools":[{"type":"function","function":{"name":"lookup_revenue"}}],
		"response_format":{"type":"json_object"}
	}`)

	material := BuildScopedCacheKeyMaterialFromInput(ScopedCacheKeyInput{
		ScopeType:     CacheScopeTenant,
		Tenant:        "tenant-a",
		Consumer:      "consumer-a",
		RouteName:     "chat-route",
		Provider:      "dashscope",
		PolicyVersion: "policy-20260625",
		LegacyKey:     "What is tenant-a revenue?",
		Body:          body,
	})

	require.True(t, material.Valid)
	require.Equal(t, CacheScopeTenant, material.Scope.Type)
	require.Equal(t, "tenant-a", material.Scope.Tenant)
	require.Empty(t, material.Scope.Consumer)
	require.Equal(t, "chat-route", material.Route.Name)
	require.Equal(t, "dashscope", material.Provider.Name)
	require.Equal(t, "qwen-turbo", material.Provider.Model)
	require.Equal(t, "policy-20260625", material.PolicyVersion)
	require.NotEmpty(t, material.RequestFingerprint.Hash)
	require.NotEmpty(t, material.RequestFingerprint.MessagesHash)
	require.NotEmpty(t, material.RequestFingerprint.SystemHash)
	require.NotEmpty(t, material.RequestFingerprint.ToolsHash)
	require.NotEmpty(t, material.RequestFingerprint.SamplingHash)
	require.NotEmpty(t, material.RequestFingerprint.OutputFormatHash)
	require.Contains(t, material.EntryKey, "{aiscope:")
	require.Contains(t, material.LockKey, "{aiscope:")
	require.Contains(t, material.TagKey, "{aiscope:")
	require.Contains(t, material.MetadataKey, "{aiscope:")
	require.Equal(t, material.ScopeHashTag, hashTagFromKey(material.EntryKey))
	require.Equal(t, material.ScopeHashTag, hashTagFromKey(material.LockKey))
	require.Equal(t, material.ScopeHashTag, hashTagFromKey(material.TagKey))
	require.Equal(t, material.ScopeHashTag, hashTagFromKey(material.MetadataKey))
	require.NotContains(t, material.EntryKey, "What is tenant-a revenue?")
	require.NotContains(t, material.EntryKey, "tenant-a")
	require.NotContains(t, material.EntryKey, "consumer-a")
}

func TestBuildScopedCacheKeyMaterialRequiresScopedIdentity(t *testing.T) {
	tenantMaterial := BuildScopedCacheKeyMaterialFromInput(ScopedCacheKeyInput{
		ScopeType:     CacheScopeTenant,
		Provider:      "dashscope",
		PolicyVersion: "policy-1",
		LegacyKey:     "hello",
		Body:          gjson.Parse(`{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}]}`),
	})
	require.False(t, tenantMaterial.Valid)
	require.Empty(t, tenantMaterial.EntryKey)
	require.Contains(t, tenantMaterial.BypassReason, "tenant")

	consumerMaterial := BuildScopedCacheKeyMaterialFromInput(ScopedCacheKeyInput{
		ScopeType:     CacheScopeConsumer,
		Tenant:        "tenant-a",
		Provider:      "dashscope",
		PolicyVersion: "policy-1",
		LegacyKey:     "hello",
		Body:          gjson.Parse(`{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}]}`),
	})
	require.False(t, consumerMaterial.Valid)
	require.Empty(t, consumerMaterial.EntryKey)
	require.Contains(t, consumerMaterial.BypassReason, "consumer")
}

func TestBuildScopedCacheKeyMaterialSeparatesScopeRouteModelPolicyAndFingerprint(t *testing.T) {
	base := ScopedCacheKeyInput{
		ScopeType:     CacheScopeConsumer,
		Tenant:        "tenant-a",
		Consumer:      "consumer-a",
		RouteName:     "chat-route",
		Provider:      "dashscope",
		PolicyVersion: "policy-1",
		LegacyKey:     "hello",
		Body:          gjson.Parse(`{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}],"temperature":0.1}`),
	}
	baseKey := BuildScopedCacheKeyMaterialFromInput(base).EntryKey
	require.NotEmpty(t, baseKey)

	cases := []ScopedCacheKeyInput{
		func() ScopedCacheKeyInput { v := base; v.Tenant = "tenant-b"; return v }(),
		func() ScopedCacheKeyInput { v := base; v.Consumer = "consumer-b"; return v }(),
		func() ScopedCacheKeyInput { v := base; v.RouteName = "other-route"; return v }(),
		func() ScopedCacheKeyInput { v := base; v.Provider = "openai"; return v }(),
		func() ScopedCacheKeyInput {
			v := base
			v.Body = gjson.Parse(`{"model":"qwen-plus","messages":[{"role":"user","content":"hello"}],"temperature":0.1}`)
			return v
		}(),
		func() ScopedCacheKeyInput { v := base; v.PolicyVersion = "policy-2"; return v }(),
		func() ScopedCacheKeyInput {
			v := base
			v.Body = gjson.Parse(`{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}],"temperature":0.8}`)
			return v
		}(),
		func() ScopedCacheKeyInput {
			v := base
			v.Body = gjson.Parse(`{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}],"temperature":0.1,"response_format":{"type":"json_object"}}`)
			return v
		}(),
	}

	for _, item := range cases {
		require.NotEqual(t, baseKey, BuildScopedCacheKeyMaterialFromInput(item).EntryKey)
	}
}

func TestBuildScopedCacheKeyMaterialGlobalScopeIgnoresTenantAndConsumer(t *testing.T) {
	base := ScopedCacheKeyInput{
		ScopeType:     CacheScopeGlobal,
		Tenant:        "tenant-a",
		Consumer:      "consumer-a",
		RouteName:     "chat-route",
		Provider:      "dashscope",
		PolicyVersion: "policy-1",
		LegacyKey:     "hello",
		Body:          gjson.Parse(`{"model":"qwen-turbo","messages":[{"role":"user","content":"hello"}]}`),
	}
	other := base
	other.Tenant = "tenant-b"
	other.Consumer = "consumer-b"

	baseMaterial := BuildScopedCacheKeyMaterialFromInput(base)
	otherMaterial := BuildScopedCacheKeyMaterialFromInput(other)

	require.True(t, baseMaterial.Valid)
	require.True(t, otherMaterial.Valid)
	require.Equal(t, baseMaterial.EntryKey, otherMaterial.EntryKey)
	require.Equal(t, CacheScopeGlobal, baseMaterial.Scope.Type)
	require.Empty(t, baseMaterial.Scope.Tenant)
	require.Empty(t, baseMaterial.Scope.Consumer)
}

func hashTagFromKey(key string) string {
	start := -1
	end := -1
	for i, r := range key {
		if r == '{' && start == -1 {
			start = i
		}
		if r == '}' && start != -1 {
			end = i
			break
		}
	}
	if start == -1 || end == -1 {
		return ""
	}
	return key[start+1 : end]
}
