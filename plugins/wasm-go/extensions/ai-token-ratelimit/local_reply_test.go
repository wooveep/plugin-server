package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/resp"
)

func bigintMultiRuleResp(threshold, current string, ttl int) []byte {
	raw, _ := resp.ArrayValue([]resp.Value{
		resp.ArrayValue([]resp.Value{
			resp.StringValue(threshold),
			resp.StringValue(current),
			resp.IntegerValue(ttl),
		}),
	}).MarshalRESP()
	return raw
}

func TestTokenRateLimitLocalReplyConfigValidationU02(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for idx, bad := range []any{
			map[string]any{"input_protocol": "anthropic_messages", "response_profile": "openai"},
			map[string]any{"response_profile": "deepseek"},
			map[string]any{"input_protocol": "openai_chat", "response_profile": "unknown"},
		} {
			cfg, err := json.Marshal(map[string]any{
				"rule_name": "ai-token-bad-policy",
				"global_threshold": map[string]any{
					"token_per_minute": 100,
				},
				"redis": map[string]any{
					"service_name": "redis.static",
					"service_port": 6379,
				},
				"local_reply_policy": bad,
			})
			require.NoError(t, err)
			host, status := test.NewTestHost(cfg)
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status, "invalid policy %d", idx)
		}

		zeroThresholdCfg, err := json.Marshal(map[string]any{
			"rule_name": "ai-token-zero",
			"global_threshold": map[string]any{
				"token_per_minute": 0,
			},
			"redis": map[string]any{
				"service_name": "redis.static",
				"service_port": 6379,
			},
		})
		require.NoError(t, err)
		zeroHost, zeroStatus := test.NewTestHost(zeroThresholdCfg)
		zeroHost.Reset()
		require.Equal(t, types.OnPluginStartStatusFailed, zeroStatus)
	})
}

func TestTokenRateLimitLocalReplyPositiveBigintAndCustomStatusU03U07(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		const bigintThreshold int64 = 5_000_000_000
		cfg, err := json.Marshal(map[string]any{
			"rule_name": "ai-token-bigint-route",
			"global_threshold": map[string]any{
				"token_per_minute": bigintThreshold,
			},
			"redis": map[string]any{
				"service_name": "redis.static",
				"service_port": 6379,
			},
			"rejected_code": 418,
			"rejected_msg":  "custom-legacy-msg",
			"local_reply_policy": map[string]any{
				"input_protocol":   "anthropic_messages",
				"response_profile": "qwen",
			},
		})
		require.NoError(t, err)

		t.Run("exceeding 64-bit threshold encodes anthropic rate_limit_exceeded with reset header", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/v1/messages"},
				{":method", http.MethodPost},
				{"x-request-id", "req-tpm-bigint-418"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)
			calls := host.GetRedisCalloutAttributes()
			require.Len(t, calls, 1)
			require.Contains(t, string(calls[0].Query), "higress-token-ratelimit:{ai-token-bigint-route}:global_threshold:60")
			require.Contains(t, string(calls[0].Query), strconv.FormatInt(bigintThreshold, 10))

			host.CallOnRedisCall(0, bigintMultiRuleResp("5000000000", "5000000001", 42))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(418), resp.StatusCode)
			require.Equal(t, "ai-token-ratelimit.rejected", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"X-TokenRateLimit-Reset", "42"})
			require.Contains(t, resp.Headers, [2]string{"content-type", "application/json"})
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-tpm-bigint-418"})
			require.JSONEq(t, `{"type":"error","error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"The request limit has been exceeded. Try again later."}}`, string(resp.Data))
		})

		t.Run("within threshold preserves response-phase token accumulation", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/v1/messages"},
				{":method", http.MethodPost},
			}))
			host.CallOnRedisCall(0, bigintMultiRuleResp("5000000000", "100", 60))
			require.Nil(t, host.GetLocalResponse())

			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}))
			body := []byte(`{"model":"qwen-plus","usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`)
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody(body))
			require.Nil(t, host.GetLocalResponse())
			calls := host.GetRedisCalloutAttributes()
			require.Len(t, calls, 1)
			require.Contains(t, string(calls[0].Query), "higress-token-ratelimit:{ai-token-bigint-route}:global_threshold:60")
			require.Contains(t, string(calls[0].Query), "5000000000")
			require.Contains(t, string(calls[0].Query), "20")
		})
	})
}

func TestTokenRateLimitLocalReplyLegacyFallbackAndRuleIsolationU04(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		cfg, err := json.Marshal(map[string]any{
			"_rules_": []map[string]any{
				{
					"_match_route_": []string{"ai-chat-route"},
					"rule_name":     "chat-tpm",
					"global_threshold": map[string]any{
						"token_per_minute": 100,
					},
					"redis": map[string]any{
						"service_name": "redis.static",
						"service_port": 6379,
					},
					"rejected_code": 429,
					"rejected_msg":  "legacy-chat-msg",
					"local_reply_policy": map[string]any{
						"input_protocol":   "openai_chat",
						"response_profile": "openai",
					},
				},
				{
					"_match_route_": []string{"plain-route"},
					"rule_name":     "plain-tpm",
					"global_threshold": map[string]any{
						"token_per_minute": 100,
					},
					"redis": map[string]any{
						"service_name": "redis.static",
						"service_port": 6379,
					},
					"rejected_code": 429,
					"rejected_msg":  "legacy-plain-msg",
				},
			},
		})
		require.NoError(t, err)

		t.Run("AI route rule encodes JSON envelope", func(t *testing.T) {
			hostAI, status := test.NewTestHost(cfg)
			defer hostAI.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			hostAI.SetRouteName("ai-chat-route")
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, hostAI.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", http.MethodPost},
			}))
			hostAI.CallOnRedisCall(0, bigintMultiRuleResp("100", "101", 15))
			respAI := hostAI.GetLocalResponse()
			require.NotNil(t, respAI)
			require.Equal(t, uint32(429), respAI.StatusCode)
			require.JSONEq(t, `{"error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"The request limit has been exceeded. Try again later."}}`, string(respAI.Data))
		})

		t.Run("plain route rule preserves legacy message", func(t *testing.T) {
			hostPlain, status := test.NewTestHost(cfg)
			defer hostPlain.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			hostPlain.SetRouteName("plain-route")
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, hostPlain.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/api/plain"},
				{":method", http.MethodPost},
			}))
			hostPlain.CallOnRedisCall(0, bigintMultiRuleResp("100", "101", 15))
			respPlain := hostPlain.GetLocalResponse()
			require.NotNil(t, respPlain)
			require.Equal(t, uint32(429), respPlain.StatusCode)
			require.Equal(t, "legacy-plain-msg", string(respPlain.Data))
		})
	})
}
