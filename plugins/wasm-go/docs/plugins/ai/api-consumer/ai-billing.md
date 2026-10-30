---
title: AI 账单事件
keywords: [AI网关, AI账单, ai-billing]
description: ai-billing 请求级账单事件插件配置参考
---

## 功能说明

`ai-billing` 在 AI 响应完成后生成请求级 billing event，并通过 HTTP callout 上报给 billing-service。插件独立调用 `pkg/tokenusage` 解析 usage 和 model，不依赖 `ai-quota` 或 `ai-statistics` 的私有上下文。

投递默认 fail-open。billing-service 超时、网络失败或返回 5xx 时，插件只记录日志，不阻塞用户响应。

## 配置示例

```yaml
billing_service:
  service_name: modelfusion-console.higress-system.svc.cluster.local
  service_port: 8080
  path: /internal/billing/events
  timeout: 750
  auth_token: <shared-secret>
quota_scope: route:qwen
provider: dashscope
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
enable_path_suffixes:
  - /v1/chat/completions
  - /v1/messages
fail_policy: open
```

## Event 字段

| 字段 | 说明 |
| --- | --- |
| `request_id` | 请求 ID |
| `idempotency_key` | 幂等键，默认使用请求 ID |
| `tenant` | 租户 |
| `consumer` | consumer |
| `quota_scope` | 额度作用域 |
| `provider` | AI provider |
| `model` | 响应 model |
| `route` | Higress route |
| `cluster` | 上游 cluster |
| `request_path` | 请求路径 |
| `status_code` | 响应状态码 |
| `start_time_ms` / `end_time_ms` | 请求时间 |
| `is_stream` | 是否流式响应 |
| `input_tokens` / `output_tokens` / `total_tokens` | token usage |
| `usage_missing` | usage 是否缺失 |
| `price_version` | 可选价格版本 |
| `gateway_calculated_cost` | 可选网关侧估算费用 |

`ai-billing` 不扣减 Redis 余额，也不直接更新账户或数据库。幂等、结算、账单流水、余额投影和补偿由 billing-service 负责。
