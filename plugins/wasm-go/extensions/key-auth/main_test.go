// Copyright (c) 2023 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

// 测试配置：基本 key-auth 配置
var basicKeyAuthConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"consumers": []map[string]interface{}{
			{
				"name":       "consumer1",
				"credential": "token1",
			},
			{
				"name":       "consumer2",
				"credential": "token2",
			},
		},
		"keys":        []string{"x-api-key", "apikey"},
		"in_header":   true,
		"in_query":    false,
		"global_auth": true,
	})
	return data
}()

// 测试配置：全局认证关闭
var globalAuthFalseConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"consumers": []map[string]interface{}{
			{
				"name":       "consumer1",
				"credential": "token1",
			},
		},
		"keys":        []string{"x-api-key"},
		"in_header":   true,
		"in_query":    false,
		"global_auth": false,
	})
	return data
}()

// 测试配置：从 query 参数获取 key
var queryKeyConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"consumers": []map[string]interface{}{
			{
				"name":       "consumer1",
				"credential": "token1",
			},
		},
		"keys":        []string{"apikey"},
		"in_header":   false,
		"in_query":    true,
		"global_auth": true,
	})
	return data
}()

// 测试配置：多个 key 来源
var multipleKeysConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"consumers": []map[string]interface{}{
			{
				"name":       "consumer1",
				"credential": "token1",
			},
		},
		"keys":        []string{"x-api-key", "apikey", "authorization"},
		"in_header":   true,
		"in_query":    true,
		"global_auth": true,
	})
	return data
}()

// 测试配置：无效配置 - 缺少 keys
var invalidNoKeysConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"consumers": []map[string]interface{}{
			{
				"name":       "consumer1",
				"credential": "token1",
			},
		},
		"in_header":   true,
		"in_query":    false,
		"global_auth": true,
	})
	return data
}()

// 测试配置：无效配置 - 空的 keys
var invalidEmptyKeysConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"consumers": []map[string]interface{}{
			{
				"name":       "consumer1",
				"credential": "token1",
			},
		},
		"keys":        []string{},
		"in_header":   true,
		"in_query":    false,
		"global_auth": true,
	})
	return data
}()

// 测试配置：无效配置 - 缺少 consumers
var invalidNoConsumersConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"keys":        []string{"x-api-key"},
		"in_header":   true,
		"in_query":    false,
		"global_auth": true,
	})
	return data
}()

// 测试配置：无效配置 - 空的 consumers
var invalidEmptyConsumersConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"consumers":   []map[string]interface{}{},
		"keys":        []string{"x-api-key"},
		"in_header":   true,
		"in_query":    false,
		"global_auth": true,
	})
	return data
}()

// 测试配置：无效配置 - 缺少 in_query 和 in_header
var invalidNoSourceConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"consumers": []map[string]interface{}{
			{
				"name":       "consumer1",
				"credential": "token1",
			},
		},
		"keys":        []string{"x-api-key"},
		"global_auth": true,
	})
	return data
}()

// 测试配置：无效配置 - 重复的 credential
var invalidDuplicateCredentialConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"consumers": []map[string]interface{}{
			{
				"name":       "consumer1",
				"credential": "token1",
			},
			{
				"name":       "consumer2",
				"credential": "token1", // 重复的 credential
			},
		},
		"keys":        []string{"x-api-key"},
		"in_header":   true,
		"in_query":    false,
		"global_auth": true,
	})
	return data
}()

// 测试配置：规则配置 - 带 allow 列表
var ruleConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"consumers": []map[string]interface{}{
			{
				"name":       "consumer1",
				"credential": "token1",
			},
			{
				"name":       "consumer2",
				"credential": "token2",
			},
		},
		"keys":        []string{"x-api-key"},
		"in_header":   true,
		"in_query":    false,
		"global_auth": true,
		"_rules_": []map[string]interface{}{
			{
				"_match_route_": []string{"test-route"},
				"allow":         []string{"consumer1"},
			},
		},
	})
	return data
}()

// 测试配置：规则配置 - 空的 allow 列表
var invalidRuleConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"consumers": []map[string]interface{}{
			{
				"name":       "consumer1",
				"credential": "token1",
			},
		},
		"keys":        []string{"x-api-key"},
		"in_header":   true,
		"in_query":    false,
		"global_auth": true,
		"_rules_": []map[string]interface{}{
			{
				"_match_route_": []string{"test-route"},
				"allow":         []string{},
			},
		},
	})
	return data
}()

func mustJSON(value map[string]interface{}) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

const multiKeyResponseBody = "Request denied by Key Auth check. Multiple distinct API keys found."

func requestHeaders(extra ...[2]string) [][2]string {
	headers := [][2]string{
		{":authority", "example.com"},
		{":path", "/test"},
		{":method", "GET"},
	}
	return append(headers, extra...)
}

func headerValues(headers [][2]string, name string) []string {
	var values []string
	for _, header := range headers {
		if strings.EqualFold(header[0], name) {
			values = append(values, header[1])
		}
	}
	return values
}

func requireHeaderValue(t *testing.T, headers [][2]string, name string, value string) {
	t.Helper()
	require.Equal(t, []string{value}, headerValues(headers, name))
}

func requireNoHeader(t *testing.T, headers [][2]string, name string) {
	t.Helper()
	require.Empty(t, headerValues(headers, name))
}

func requireLocalResponseHeader(t *testing.T, headers [][2]string, name string, value string) {
	t.Helper()
	require.Contains(t, headerValues(headers, name), value)
}

func TestParseGlobalConfig(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		// 测试基本 key-auth 配置解析
		t.Run("basic key-auth config", func(t *testing.T) {
			host, status := test.NewTestHost(basicKeyAuthConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			config, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, config)

			keyAuthConfig := config.(*KeyAuthConfig)
			// 注意：由于字段是私有的，我们只能验证配置能够成功解析
			require.NotNil(t, keyAuthConfig)
			require.Len(t, keyAuthConfig.Keys, 2)
			require.Equal(t, "x-api-key", keyAuthConfig.Keys[0])
			require.Equal(t, "apikey", keyAuthConfig.Keys[1])
			require.True(t, keyAuthConfig.InHeader)
			require.False(t, keyAuthConfig.InQuery)
		})

		// 测试全局认证关闭配置
		t.Run("global auth false config", func(t *testing.T) {
			host, status := test.NewTestHost(globalAuthFalseConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			config, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, config)

			keyAuthConfig := config.(*KeyAuthConfig)
			// 注意：由于字段是私有的，我们只能验证配置能够成功解析
			require.NotNil(t, keyAuthConfig)
			require.Len(t, keyAuthConfig.Keys, 1)
			require.Equal(t, "x-api-key", keyAuthConfig.Keys[0])
		})

		// 测试从 query 参数获取 key 的配置
		t.Run("query key config", func(t *testing.T) {
			host, status := test.NewTestHost(queryKeyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			config, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, config)

			keyAuthConfig := config.(*KeyAuthConfig)
			require.NotNil(t, keyAuthConfig)
			require.False(t, keyAuthConfig.InHeader)
			require.True(t, keyAuthConfig.InQuery)
			require.Len(t, keyAuthConfig.Keys, 1)
			require.Equal(t, "apikey", keyAuthConfig.Keys[0])
		})

		// 测试多个 key 来源的配置
		t.Run("multiple keys config", func(t *testing.T) {
			host, status := test.NewTestHost(multipleKeysConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			config, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, config)

			keyAuthConfig := config.(*KeyAuthConfig)
			require.NotNil(t, keyAuthConfig)
			require.True(t, keyAuthConfig.InHeader)
			require.True(t, keyAuthConfig.InQuery)
			require.Len(t, keyAuthConfig.Keys, 3)
			require.Equal(t, "x-api-key", keyAuthConfig.Keys[0])
			require.Equal(t, "apikey", keyAuthConfig.Keys[1])
			require.Equal(t, "authorization", keyAuthConfig.Keys[2])
		})

		// 测试无效配置 - 缺少 keys
		t.Run("invalid no keys config", func(t *testing.T) {
			host, status := test.NewTestHost(invalidNoKeysConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		// 测试无效配置 - 空的 keys
		t.Run("invalid empty keys config", func(t *testing.T) {
			host, status := test.NewTestHost(invalidEmptyKeysConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		// 测试无效配置 - 缺少 consumers
		t.Run("invalid no consumers config", func(t *testing.T) {
			host, status := test.NewTestHost(invalidNoConsumersConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		// An empty credential registry remains a valid deny-all state.
		t.Run("empty consumers config", func(t *testing.T) {
			host, status := test.NewTestHost(invalidEmptyConsumersConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
		})

		// 测试无效配置 - 缺少 in_query 和 in_header
		t.Run("invalid no source config", func(t *testing.T) {
			host, status := test.NewTestHost(invalidNoSourceConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		// 测试无效配置 - 重复的 credential
		t.Run("invalid duplicate credential config", func(t *testing.T) {
			host, status := test.NewTestHost(invalidDuplicateCredentialConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})
	})
}

func TestParseRuleConfig(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		// 测试有效的规则配置
		t.Run("valid rule config", func(t *testing.T) {
			host, status := test.NewTestHost(ruleConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			config, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, config)

			keyAuthConfig := config.(*KeyAuthConfig)
			// 注意：由于配置解析逻辑的复杂性，我们只验证配置能够成功解析
			require.NotNil(t, keyAuthConfig)
			// allow 字段的解析可能需要更复杂的配置结构
		})

		// Explicitly empty allow lists remain configured and deny all consumers.
		t.Run("empty allow rule config", func(t *testing.T) {
			host, status := test.NewTestHost(invalidRuleConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
		})
	})
}

func TestParseGlobalConfigLocalYAMLEnhancement(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		tests := []struct {
			name   string
			config json.RawMessage
			status types.OnPluginStartStatus
		}{
			{
				name: "multiple credentials with tenant and realm",
				config: mustJSON(map[string]interface{}{
					"consumers": []map[string]interface{}{
						{
							"name":        "consumer1",
							"tenant":      "tenant-a",
							"credentials": []string{"token1", "token1-alt"},
						},
					},
					"keys":        []string{"x-api-key"},
					"in_header":   true,
					"global_auth": true,
					"realm":       "Custom Gateway",
				}),
				status: types.OnPluginStartStatusOK,
			},
			{
				name: "top-level credentials mode",
				config: mustJSON(map[string]interface{}{
					"credentials": []string{"token1", "token2"},
					"keys":        []string{"x-api-key"},
					"in_header":   true,
					"global_auth": true,
				}),
				status: types.OnPluginStartStatusOK,
			},
			{
				name: "consumer keys allow missing global keys",
				config: mustJSON(map[string]interface{}{
					"consumers": []map[string]interface{}{
						{
							"name":       "consumer1",
							"credential": "token1",
							"keys":       []string{"consumer-key"},
							"in_header":  true,
						},
					},
					"global_auth": true,
				}),
				status: types.OnPluginStartStatusOK,
			},
			{
				name: "consumers conflict with top-level credentials",
				config: mustJSON(map[string]interface{}{
					"consumers": []map[string]interface{}{
						{
							"name":       "consumer1",
							"credential": "token1",
						},
					},
					"credentials": []string{"token2"},
					"keys":        []string{"x-api-key"},
					"in_header":   true,
					"global_auth": true,
				}),
				status: types.OnPluginStartStatusFailed,
			},
			{
				name: "consumer credential conflicts with credentials",
				config: mustJSON(map[string]interface{}{
					"consumers": []map[string]interface{}{
						{
							"name":        "consumer1",
							"credential":  "token1",
							"credentials": []string{"token2"},
						},
					},
					"keys":        []string{"x-api-key"},
					"in_header":   true,
					"global_auth": true,
				}),
				status: types.OnPluginStartStatusFailed,
			},
			{
				name: "empty consumer credentials rejected",
				config: mustJSON(map[string]interface{}{
					"consumers": []map[string]interface{}{
						{
							"name":        "consumer1",
							"credentials": []string{},
						},
					},
					"keys":        []string{"x-api-key"},
					"in_header":   true,
					"global_auth": true,
				}),
				status: types.OnPluginStartStatusFailed,
			},
			{
				name: "empty credential string rejected",
				config: mustJSON(map[string]interface{}{
					"consumers": []map[string]interface{}{
						{
							"name":        "consumer1",
							"credentials": []string{"token1", ""},
						},
					},
					"keys":        []string{"x-api-key"},
					"in_header":   true,
					"global_auth": true,
				}),
				status: types.OnPluginStartStatusFailed,
			},
			{
				name: "duplicate credential in same consumer rejected",
				config: mustJSON(map[string]interface{}{
					"consumers": []map[string]interface{}{
						{
							"name":        "consumer1",
							"credentials": []string{"token1", "token1"},
						},
					},
					"keys":        []string{"x-api-key"},
					"in_header":   true,
					"global_auth": true,
				}),
				status: types.OnPluginStartStatusFailed,
			},
			{
				name: "duplicate credential across consumers rejected",
				config: mustJSON(map[string]interface{}{
					"consumers": []map[string]interface{}{
						{
							"name":       "consumer1",
							"credential": "token1",
						},
						{
							"name":        "consumer2",
							"credentials": []string{"token1"},
						},
					},
					"keys":        []string{"x-api-key"},
					"in_header":   true,
					"global_auth": true,
				}),
				status: types.OnPluginStartStatusFailed,
			},
			{
				name: "missing global keys for fallback consumer rejected",
				config: mustJSON(map[string]interface{}{
					"consumers": []map[string]interface{}{
						{
							"name":       "consumer1",
							"credential": "token1",
						},
					},
					"in_header":   true,
					"global_auth": true,
				}),
				status: types.OnPluginStartStatusFailed,
			},
			{
				name: "consumer with no enabled source rejected",
				config: mustJSON(map[string]interface{}{
					"consumers": []map[string]interface{}{
						{
							"name":       "consumer1",
							"credential": "token1",
							"keys":       []string{"x-api-key"},
							"in_header":  false,
							"in_query":   false,
						},
					},
					"global_auth": true,
				}),
				status: types.OnPluginStartStatusFailed,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				host, status := test.NewTestHost(tt.config)
				defer host.Reset()
				require.Equal(t, tt.status, status)
			})
		}
	})
}

func TestOnHTTPRequestHeaders(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试全局认证开启 - 有效的 API key
		t.Run("global auth true - valid api key", func(t *testing.T) {
			host, status := test.NewTestHost(basicKeyAuthConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置有效的 API key 在请求头中
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/test"},
				{":method", "GET"},
				{"x-api-key", "token1"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())

			localResponse := host.GetLocalResponse()
			require.Nil(t, localResponse, "Valid API key should pass through")

			// 验证是否添加了 X-Mse-Consumer 头
			headers := host.GetRequestHeaders()
			consumerHeaderFound := false
			for _, header := range headers {
				if header[0] == "x-mse-consumer" && header[1] == "consumer1" {
					consumerHeaderFound = true
					break
				}
			}
			require.True(t, consumerHeaderFound, "X-Mse-Consumer header should be added")

			host.CompleteHttp()
		})

		// 测试全局认证开启 - 无效的 API key
		t.Run("global auth true - invalid api key", func(t *testing.T) {
			host, status := test.NewTestHost(basicKeyAuthConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/test"},
				{":method", "GET"},
				{"x-api-key", "invalid-token"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "Invalid API key should be rejected")
			require.Equal(t, uint32(403), localResponse.StatusCode) // Forbidden

			host.CompleteHttp()
		})

		// 测试全局认证开启 - 缺少 API key
		t.Run("global auth true - missing api key", func(t *testing.T) {
			host, status := test.NewTestHost(basicKeyAuthConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/test"},
				{":method", "GET"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "Missing API key should be rejected")
			require.Equal(t, uint32(401), localResponse.StatusCode) // Unauthorized

			host.CompleteHttp()
		})

		// 测试全局认证开启 - 多个 API key（应该被拒绝）
		t.Run("global auth true - multiple api keys", func(t *testing.T) {
			host, status := test.NewTestHost(basicKeyAuthConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/test"},
				{":method", "GET"},
				{"x-api-key", "token1"},
				{"apikey", "token2"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "Multiple API keys should be rejected")
			require.Equal(t, uint32(401), localResponse.StatusCode) // Unauthorized

			host.CompleteHttp()
		})

		// 测试全局认证关闭 - 无 allow 列表（直接放行）
		t.Run("global auth false - no allow list", func(t *testing.T) {
			host, status := test.NewTestHost(globalAuthFalseConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/test"},
				{":method", "GET"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())

			localResponse := host.GetLocalResponse()
			require.Nil(t, localResponse, "No auth required should pass through")

			host.CompleteHttp()
		})

		// 测试从 query 参数获取 API key
		t.Run("query api key", func(t *testing.T) {
			host, status := test.NewTestHost(queryKeyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置包含 API key 的查询参数
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/test?apikey=token1"},
				{":method", "GET"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())

			localResponse := host.GetLocalResponse()
			require.Nil(t, localResponse, "Valid API key in query should pass through")

			host.CompleteHttp()
		})

		// 测试从 query 参数获取 API key - 无效的 key
		t.Run("query api key - invalid", func(t *testing.T) {
			host, status := test.NewTestHost(queryKeyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/test?apikey=invalid-token"},
				{":method", "GET"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "Invalid API key in query should be rejected")
			require.Equal(t, uint32(403), localResponse.StatusCode) // Forbidden

			host.CompleteHttp()
		})

		// 测试从 query 参数获取 API key - 缺少 key
		t.Run("query api key - missing", func(t *testing.T) {
			host, status := test.NewTestHost(queryKeyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/test"},
				{":method", "GET"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "Missing API key in query should be rejected")
			require.Equal(t, uint32(401), localResponse.StatusCode) // Unauthorized

			host.CompleteHttp()
		})
	})
}

func TestOnHTTPRequestHeadersLocalYAMLEnhancement(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("first and second credentials authenticate same consumer with tenant", func(t *testing.T) {
			config := mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":        "consumer1",
						"tenant":      "tenant-a",
						"credentials": []string{"token1", "token1-alt"},
					},
				},
				"keys":        []string{"x-api-key"},
				"in_header":   true,
				"global_auth": true,
			})

			for _, credential := range []string{"token1", "token1-alt"} {
				func() {
					host, status := test.NewTestHost(config)
					defer host.Reset()
					require.Equal(t, types.OnPluginStartStatusOK, status)

					action := host.CallOnHttpRequestHeaders(requestHeaders([2]string{"x-api-key", credential}))

					require.Equal(t, types.ActionContinue, action)
					require.Nil(t, host.GetLocalResponse())
					headers := host.GetRequestHeaders()
					requireHeaderValue(t, headers, "x-mse-consumer", "consumer1")
					requireHeaderValue(t, headers, "x-mse-tenant", "tenant-a")
					host.CompleteHttp()
				}()
			}
		})

		t.Run("consumer keys override global keys", func(t *testing.T) {
			config := mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":       "consumer1",
						"credential": "token1",
						"keys":       []string{"consumer-key"},
					},
				},
				"keys":        []string{"x-api-key"},
				"in_header":   true,
				"global_auth": true,
			})

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders(requestHeaders([2]string{"consumer-key", "token1"}))
				require.Nil(t, host.GetLocalResponse())
				requireHeaderValue(t, host.GetRequestHeaders(), "x-mse-consumer", "consumer1")
				host.CompleteHttp()
			}()

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders(requestHeaders([2]string{"x-api-key", "token1"}))
				localResponse := host.GetLocalResponse()
				require.NotNil(t, localResponse)
				require.Equal(t, uint32(401), localResponse.StatusCode)
				host.CompleteHttp()
			}()
		})

		t.Run("consumer without keys uses global keys", func(t *testing.T) {
			host, status := test.NewTestHost(mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":       "consumer1",
						"credential": "token1",
					},
				},
				"keys":        []string{"x-api-key"},
				"in_header":   true,
				"global_auth": true,
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders(requestHeaders([2]string{"x-api-key", "token1"}))

			require.Nil(t, host.GetLocalResponse())
			requireHeaderValue(t, host.GetRequestHeaders(), "x-mse-consumer", "consumer1")
			host.CompleteHttp()
		})

		t.Run("consumer source override authenticates query", func(t *testing.T) {
			host, status := test.NewTestHost(mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":       "consumer1",
						"credential": "token1",
						"in_header":  false,
						"in_query":   true,
					},
				},
				"keys":        []string{"apikey"},
				"in_header":   true,
				"in_query":    false,
				"global_auth": true,
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/test?apikey=token1"},
				{":method", "GET"},
			})

			require.Nil(t, host.GetLocalResponse())
			requireHeaderValue(t, host.GetRequestHeaders(), "x-mse-consumer", "consumer1")
			host.CompleteHttp()
		})

		t.Run("query is checked when header and query are enabled", func(t *testing.T) {
			host, status := test.NewTestHost(mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":       "consumer1",
						"credential": "token1",
					},
				},
				"keys":        []string{"apikey"},
				"in_header":   true,
				"in_query":    true,
				"global_auth": true,
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/test?apikey=token1"},
				{":method", "GET"},
			})

			require.Nil(t, host.GetLocalResponse())
			requireHeaderValue(t, host.GetRequestHeaders(), "x-mse-consumer", "consumer1")
			host.CompleteHttp()
		})

		t.Run("same credential across header and query authenticates", func(t *testing.T) {
			host, status := test.NewTestHost(mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":       "consumer1",
						"credential": "token1",
					},
				},
				"keys":        []string{"apikey"},
				"in_header":   true,
				"in_query":    true,
				"global_auth": true,
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/test?apikey=token1"},
				{":method", "GET"},
				{"apikey", "token1"},
			})

			require.Nil(t, host.GetLocalResponse())
			requireHeaderValue(t, host.GetRequestHeaders(), "x-mse-consumer", "consumer1")
			host.CompleteHttp()
		})

		t.Run("authorization bearer and raw values authenticate", func(t *testing.T) {
			config := mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":       "consumer1",
						"credential": "real-api-key",
					},
				},
				"keys":        []string{"Authorization"},
				"in_header":   true,
				"global_auth": true,
			})

			for _, value := range []string{"Bearer real-api-key", "real-api-key"} {
				func() {
					host, status := test.NewTestHost(config)
					defer host.Reset()
					require.Equal(t, types.OnPluginStartStatusOK, status)

					host.CallOnHttpRequestHeaders(requestHeaders([2]string{"Authorization", value}))

					require.Nil(t, host.GetLocalResponse())
					requireHeaderValue(t, host.GetRequestHeaders(), "x-mse-consumer", "consumer1")
					host.CompleteHttp()
				}()
			}
		})

		t.Run("same credential across authorization and api key headers authenticates", func(t *testing.T) {
			host, status := test.NewTestHost(mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":       "consumer1",
						"credential": "real-api-key",
					},
				},
				"keys":        []string{"Authorization", "x-api-key", "apikey"},
				"in_header":   true,
				"global_auth": true,
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders(requestHeaders(
				[2]string{"Authorization", "Bearer real-api-key"},
				[2]string{"x-api-key", "real-api-key"},
				[2]string{"apikey", "real-api-key"},
			))

			require.Nil(t, host.GetLocalResponse())
			requireHeaderValue(t, host.GetRequestHeaders(), "x-mse-consumer", "consumer1")
			host.CompleteHttp()
		})

		t.Run("bearer prefix is not stripped from non-authorization headers", func(t *testing.T) {
			host, status := test.NewTestHost(mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":       "consumer1",
						"credential": "real-api-key",
					},
				},
				"keys":        []string{"x-api-key"},
				"in_header":   true,
				"global_auth": true,
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders(requestHeaders([2]string{"x-api-key", "Bearer real-api-key"}))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse)
			require.Equal(t, uint32(403), localResponse.StatusCode)
			host.CompleteHttp()
		})

		t.Run("multiple credentials across header query and repeated query are rejected", func(t *testing.T) {
			config := mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":        "consumer1",
						"credentials": []string{"token1", "token2"},
					},
				},
				"keys":        []string{"apikey"},
				"in_header":   true,
				"in_query":    true,
				"global_auth": true,
			})

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/test?apikey=token2"},
					{":method", "GET"},
					{"apikey", "token1"},
				})
				localResponse := host.GetLocalResponse()
				require.NotNil(t, localResponse)
				require.Equal(t, uint32(401), localResponse.StatusCode)
				require.Equal(t, "key-auth.multi_key", localResponse.StatusCodeDetail)
				require.Equal(t, multiKeyResponseBody, string(localResponse.Data))
				host.CompleteHttp()
			}()

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/test?apikey=token1&apikey=token1"},
					{":method", "GET"},
				})
				require.Nil(t, host.GetLocalResponse())
				requireHeaderValue(t, host.GetRequestHeaders(), "x-mse-consumer", "consumer1")
				host.CompleteHttp()
			}()

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/test?apikey=token1&apikey=token2"},
					{":method", "GET"},
				})
				localResponse := host.GetLocalResponse()
				require.NotNil(t, localResponse)
				require.Equal(t, uint32(401), localResponse.StatusCode)
				require.Equal(t, "key-auth.multi_key", localResponse.StatusCodeDetail)
				require.Equal(t, multiKeyResponseBody, string(localResponse.Data))
				host.CompleteHttp()
			}()
		})

		t.Run("same credential duplicates respect consumer source isolation", func(t *testing.T) {
			config := mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":       "consumer1",
						"credential": "token1",
						"keys":       []string{"Authorization"},
						"in_header":  true,
						"in_query":   false,
					},
					{
						"name":       "consumer2",
						"credential": "token2",
						"keys":       []string{"x-api-key", "apikey"},
						"in_header":  true,
						"in_query":   false,
					},
				},
				"keys":        []string{"Authorization", "x-api-key", "apikey"},
				"in_header":   true,
				"global_auth": true,
			})

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders(requestHeaders(
					[2]string{"Authorization", "Bearer token1"},
					[2]string{"x-api-key", "token1"},
				))
				require.Nil(t, host.GetLocalResponse())
				requireHeaderValue(t, host.GetRequestHeaders(), "x-mse-consumer", "consumer1")
				host.CompleteHttp()
			}()

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders(requestHeaders(
					[2]string{"x-api-key", "token1"},
					[2]string{"apikey", "token1"},
				))
				localResponse := host.GetLocalResponse()
				require.NotNil(t, localResponse)
				require.Equal(t, uint32(403), localResponse.StatusCode)
				host.CompleteHttp()
			}()
		})

		t.Run("allow list uses authenticated consumer name", func(t *testing.T) {
			config := mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":        "consumer1",
						"credentials": []string{"token1", "token1-alt"},
					},
					{
						"name":       "consumer2",
						"credential": "token2",
					},
				},
				"keys":        []string{"x-api-key"},
				"in_header":   true,
				"global_auth": true,
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"test-route"},
						"allow":         []string{"consumer1"},
					},
				},
			})

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("test-route"))
				host.CallOnHttpRequestHeaders(requestHeaders([2]string{"x-api-key", "token1-alt"}))
				require.Nil(t, host.GetLocalResponse())
				requireHeaderValue(t, host.GetRequestHeaders(), "x-mse-consumer", "consumer1")
				host.CompleteHttp()
			}()

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("test-route"))
				host.CallOnHttpRequestHeaders(requestHeaders([2]string{"x-api-key", "token2"}))
				localResponse := host.GetLocalResponse()
				require.NotNil(t, localResponse)
				require.Equal(t, uint32(403), localResponse.StatusCode)
				host.CompleteHttp()
			}()
		})

		t.Run("top-level credentials authenticate without identity headers", func(t *testing.T) {
			config := mustJSON(map[string]interface{}{
				"credentials": []string{"token1"},
				"keys":        []string{"x-api-key", "apikey"},
				"in_header":   true,
				"global_auth": true,
			})

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				host.CallOnHttpRequestHeaders(requestHeaders(
					[2]string{"x-api-key", "token1"},
					[2]string{"X-Mse-Consumer", "spoofed"},
					[2]string{"X-Mse-Tenant", "spoofed"},
				))

				require.Nil(t, host.GetLocalResponse())
				headers := host.GetRequestHeaders()
				requireNoHeader(t, headers, "x-mse-consumer")
				requireNoHeader(t, headers, "x-mse-tenant")
				host.CompleteHttp()
			}()

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				host.CallOnHttpRequestHeaders(requestHeaders(
					[2]string{"x-api-key", "token1"},
					[2]string{"apikey", "token1"},
					[2]string{"X-Mse-Consumer", "spoofed"},
					[2]string{"X-Mse-Tenant", "spoofed"},
				))

				require.Nil(t, host.GetLocalResponse())
				headers := host.GetRequestHeaders()
				requireNoHeader(t, headers, "x-mse-consumer")
				requireNoHeader(t, headers, "x-mse-tenant")
				host.CompleteHttp()
			}()
		})

		t.Run("spoofed identity headers are overwritten or removed", func(t *testing.T) {
			config := mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":        "consumer1",
						"tenant":      "tenant-a",
						"credentials": []string{"token1"},
					},
				},
				"keys":        []string{"x-api-key"},
				"in_header":   true,
				"global_auth": true,
			})

			func() {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders(requestHeaders(
					[2]string{"x-api-key", "token1"},
					[2]string{"X-Mse-Consumer", "spoofed"},
					[2]string{"X-Mse-Tenant", "spoofed"},
				))
				headers := host.GetRequestHeaders()
				requireHeaderValue(t, headers, "x-mse-consumer", "consumer1")
				requireHeaderValue(t, headers, "x-mse-tenant", "tenant-a")
				host.CompleteHttp()
			}()

			func() {
				host, status := test.NewTestHost(mustJSON(map[string]interface{}{
					"consumers": []map[string]interface{}{
						{
							"name":       "consumer1",
							"credential": "token1",
						},
					},
					"keys":        []string{"x-api-key"},
					"in_header":   true,
					"global_auth": true,
				}))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders(requestHeaders(
					[2]string{"x-api-key", "token1"},
					[2]string{"X-Mse-Tenant", "spoofed"},
				))
				headers := host.GetRequestHeaders()
				requireHeaderValue(t, headers, "x-mse-consumer", "consumer1")
				requireNoHeader(t, headers, "x-mse-tenant")
				host.CompleteHttp()
			}()
		})

		t.Run("configured realm appears in failure response", func(t *testing.T) {
			host, status := test.NewTestHost(mustJSON(map[string]interface{}{
				"consumers": []map[string]interface{}{
					{
						"name":       "consumer1",
						"credential": "token1",
					},
				},
				"keys":        []string{"x-api-key"},
				"in_header":   true,
				"global_auth": true,
				"realm":       "Custom Gateway",
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders(requestHeaders())

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse)
			require.Equal(t, uint32(401), localResponse.StatusCode)
			requireLocalResponseHeader(t, localResponse.Headers, "WWW-Authenticate", "Key realm=Custom Gateway")
			host.CompleteHttp()
		})
	})
}
