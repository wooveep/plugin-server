package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"cluster-key-rate-limit/config"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/resp"
)

type rpmCommittedCtx struct {
	wrapper.HttpContext
}

func (rpmCommittedCtx) GetExecutionPhase() iface.HTTPExecutionPhase { return iface.EncodeHeader }
func (rpmCommittedCtx) GetStringContext(string, string) string      { return "req-rpm-committed" }
func (rpmCommittedCtx) GetContext(string) interface{}               { return nil }

func rpmEvalResp(threshold, current, ttl int) []byte {
	raw, _ := resp.ArrayValue([]resp.Value{
		resp.ArrayValue([]resp.Value{
			resp.IntegerValue(threshold),
			resp.IntegerValue(current),
			resp.IntegerValue(ttl),
		}),
	}).MarshalRESP()
	return raw
}

func TestClusterKeyRateLimitLocalReplyRegressionU04U07(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("zero global threshold and invalid rate or policy are rejected at config boundary", func(t *testing.T) {
			zeroCfg, err := json.Marshal(map[string]any{
				"rule_name": "rpm-zero",
				"global_threshold": map[string]any{
					"query_per_minute": 0,
				},
				"redis": map[string]any{
					"service_name": "redis.static",
					"service_port": 6379,
				},
			})
			require.NoError(t, err)
			host, status := test.NewTestHost(zeroCfg)
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)

			for idx, badRate := range []any{-1, 1.5, "0"} {
				badRateCfg, err := json.Marshal(map[string]any{
					"rule_name": "rpm-bad-rate",
					"rule_items": []map[string]any{
						{
							"limit_by_per_header": "x-modelfusion-principal-id",
							"limit_keys": []map[string]any{
								{"key": "*", "query_per_minute": badRate},
							},
						},
					},
					"redis": map[string]any{"service_name": "redis.static", "service_port": 6379},
				})
				require.NoError(t, err)
				badHost, badStatus := test.NewTestHost(badRateCfg)
				badHost.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, badStatus, "bad rate %d", idx)
			}

			for idx, badPolicy := range []any{
				map[string]any{"input_protocol": "anthropic_messages", "response_profile": "openai"},
				map[string]any{"response_profile": "deepseek"},
				map[string]any{"input_protocol": "openai_chat", "response_profile": "unknown"},
			} {
				badPolicyCfg, err := json.Marshal(map[string]any{
					"rule_name":        "rpm-bad-policy",
					"global_threshold": map[string]any{"query_per_minute": 10},
					"redis":            map[string]any{"service_name": "redis.static", "service_port": 6379},
					"local_reply_policy": badPolicy,
				})
				require.NoError(t, err)
				badHost, badStatus := test.NewTestHost(badPolicyCfg)
				badHost.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, badStatus, "bad policy %d", idx)
			}
		})

		t.Run("ST01 committed response logs critical error without legacy fallback", func(t *testing.T) {
			okCfg, err := json.Marshal(map[string]any{
				"rule_name":        "rpm-st01",
				"global_threshold": map[string]any{"query_per_minute": 10},
				"redis":            map[string]any{"service_name": "redis.static", "service_port": 6379},
				"rejected_code":    429,
				"rejected_msg":     "legacy-msg",
				"local_reply_policy": map[string]any{
					"input_protocol": "openai_chat",
				},
			})
			require.NoError(t, err)
			host, status := test.NewTestHost(okCfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			matched, err := host.GetMatchConfig()
			require.NoError(t, err)
			host.InitHttp()

			rejected(rpmCommittedCtx{}, *matched.(*config.ClusterKeyRateLimitConfig), LimitContext{count: 10, remaining: -1, reset: 20})
			require.Nil(t, host.GetLocalResponse(), "must not send legacy fallback when SendLocalReply fails on committed response")
			joined := strings.Join(host.GetCriticalLogs(), "\n")
			require.Contains(t, joined, "cluster-key-rate-limit failed to send local reply (status=429, detail=cluster-key-rate-limit.rejected)")
			require.Contains(t, joined, clientresponse.ErrLocalReplyAfterResponseCommitted.Error())
		})
	})

	test.RunTest(t, func(t *testing.T) {
		cfg, err := json.Marshal(map[string]any{
			"_rules_": []map[string]any{
				{
					"_match_route_": []string{"ai-chat-api"},
					"rule_name":     "ai-chat-rpm-limit",
					"rule_items": []map[string]any{
						{
							"limit_by_per_header": "x-modelfusion-principal-id",
							"limit_keys": []map[string]any{
								{
									"key":              "regexp:^unlimited-user$",
									"query_per_minute": 0,
								},
								{
									"key":              "*",
									"query_per_minute": 10,
								},
							},
						},
					},
					"redis": map[string]any{
						"service_name": "redis.static",
						"service_port": 6379,
					},
					"show_limit_quota_header": true,
					"rejected_code":           429,
					"rejected_msg":            "Too many gateway requests",
					"local_reply_policy": map[string]any{
						"input_protocol":   "openai_chat",
						"response_profile": "qwen",
					},
				},
				{
					"_match_route_": []string{"ai-messages-api"},
					"rule_name":     "ai-messages-rpm-limit",
					"global_threshold": map[string]any{
						"query_per_minute": 5,
					},
					"redis": map[string]any{
						"service_name": "redis.static",
						"service_port": 6379,
					},
					"show_limit_quota_header": true,
					"rejected_code":           418,
					"rejected_msg":            "Too many gateway requests",
					"local_reply_policy": map[string]any{
						"input_protocol":   "anthropic_messages",
						"response_profile": "deepseek",
					},
				},
				{
					"_match_route_": []string{"non-ai-api"},
					"rule_name":     "route-rpm-limit",
					"rule_items": []map[string]any{
						{
							"limit_by_consumer": "",
							"limit_keys": []map[string]any{
								{
									"key":              "limited-consumer",
									"query_per_minute": 10,
								},
							},
						},
					},
					"redis": map[string]any{
						"service_name": "redis.static",
						"service_port": 6379,
					},
					"show_limit_quota_header": true,
					"rejected_code":           429,
					"rejected_msg":            "Too many gateway requests",
				},
			},
		})
		require.NoError(t, err)

		t.Run("AI chat route encodes OpenAI rate_limit_exceeded JSON with rate-limit headers", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("ai-chat-api")

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", http.MethodPost},
				{"x-modelfusion-principal-id", "limited-user"},
				{"x-request-id", "req-rpm-chat-429"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)
			host.CallOnRedisCall(0, rpmEvalResp(10, 11, 25))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(429), resp.StatusCode)
			require.Equal(t, "cluster-key-rate-limit.rejected", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"X-RateLimit-Limit", "10"})
			require.Contains(t, resp.Headers, [2]string{"X-RateLimit-Remaining", "0"})
			require.Contains(t, resp.Headers, [2]string{"X-RateLimit-Reset", "25"})
			require.Contains(t, resp.Headers, [2]string{"content-type", "application/json"})
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-rpm-chat-429"})
			require.JSONEq(t, `{"error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"The request limit has been exceeded. Try again later."}}`, string(resp.Data))
		})

		t.Run("AI messages route encodes Anthropic rate_limit_exceeded JSON preserving custom RejectedCode", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("ai-messages-api")

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/v1/messages"},
				{":method", http.MethodPost},
				{"x-request-id", "req-rpm-msg-418"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)
			host.CallOnRedisCall(0, rpmEvalResp(5, 6, 42))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(418), resp.StatusCode)
			require.Equal(t, "cluster-key-rate-limit.rejected", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"X-RateLimit-Limit", "5"})
			require.Contains(t, resp.Headers, [2]string{"X-RateLimit-Remaining", "0"})
			require.Contains(t, resp.Headers, [2]string{"X-RateLimit-Reset", "42"})
			require.Contains(t, resp.Headers, [2]string{"content-type", "application/json"})
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-rpm-msg-418"})
			require.JSONEq(t, `{"type":"error","error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"The request limit has been exceeded. Try again later."}}`, string(resp.Data))
		})

		t.Run("unmatched consumer passes through without Redis call", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("non-ai-api")

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "gateway.example.com"},
				{":path", "/api/v1/orders"},
				{":method", http.MethodGet},
				{"x-mse-consumer", "unlimited-consumer"},
			})
			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Nil(t, host.GetLocalResponse())
		})

		t.Run("non-AI route without local_reply_policy preserves plaintext rejection and X-RateLimit-Reset", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("non-ai-api")

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "gateway.example.com"},
				{":path", "/api/v1/orders"},
				{":method", http.MethodPost},
				{"x-mse-consumer", "limited-consumer"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)
			calls := host.GetRedisCalloutAttributes()
			require.Len(t, calls, 1)
			require.Contains(t, string(calls[0].Query), "higress-cluster-key-rate-limit:{route-rpm-limit}:limit_by_consumer:60:x-mse-consumer:limited-consumer")

			host.CallOnRedisCall(0, rpmEvalResp(10, 11, 25))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(429), resp.StatusCode)
			require.Equal(t, "cluster-key-rate-limit.rejected", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"X-RateLimit-Reset", "25"})
			require.Equal(t, "Too many gateway requests", string(resp.Data))
		})
	})
}
