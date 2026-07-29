---
title: AI 账单事件
keywords: [AI网关, AI账单, ai-billing]
description: ai-billing 请求级账单事件插件配置参考
---

## 功能说明

`ai-billing` 在 AI 响应完成后独立解析 token usage 和 model，默认构造客户用量 billing event，并通过 Redis Stream 写入 Console/billing-service 的结算队列。内部 AI Route 可以配置为 `event_kind=internal_cost`，只用于平台成本归因，不进入普通客户账单。插件使用 Redis `XADD <stream> * event <BillingEvent JSON>`，默认写入 `billing:events`。插件默认 fail-open：Redis 投递超时、网络失败或返回错误时只记录日志，不阻塞用户响应。

`ai-billing` 不计算最终权威费用，不应用租户折扣或覆盖价格，不扣减 Redis 余额，也不直接更新账户或数据库。幂等、结算、账单流水、余额投影和补偿由 Console/billing-service 负责。

## 事件字段

事件包含 `event_id`、`event_kind`、`idempotency_key`、`request_id`、`tenant`、`consumer`、`quota_scope`、`route`、`provider`、`model`、`cluster`、`request_path`、`status_code`、`start_time_ms`、`end_time_ms`、`is_stream`、`usage`、`usage_missing` 和可选 `price_version`。

当 provider usage 暴露 cache 详情时，`usage` 还会包含 `input_cache_hit_tokens`、`input_cache_miss_tokens` 和 `output_tokens`。为了兼容旧事件，`usage.input`、`usage.output` 和 `usage.total` 会继续保留。

`route`、`provider`、`model` 使用 Console 原生对象事实格式：`{ "id"?: "...", "name"?: "..." }`。网关通常不知道 Console UUID，因此默认填充运行时 `name`。

对启用账单的 AI 请求，插件会用网关最终采用的 `x-request-id` 覆盖响应中的 `x-ocmf-gateway-request-id`。该值与事件的 `request_id` 完全一致，可供 Console 和 UI 关联账单；上游伪造的同名响应头不会被保留。未启用账单的路径不会暴露该关联头。响应头仅证明本次网关关联 ID，不能单独证明 Redis 事件已经持久化。

## 配置示例

```yaml
defaultConfig:
  redis_stream:
    service_name: redis-stack-server.dns
    service_port: 6379
    database: 0
    timeout: 500
    stream: billing:events
  quota_scope: global
  event_kind: usage
  provider: default
  tenant_header: x-mse-tenant
  consumer_header: x-mse-consumer
  path_scope: legacy_suffixes
  enable_path_suffixes:
    - /v1/chat/completions
    - /v1/messages
  fail_policy: open
matchRules:
  - ingress:
      - qwen
    config:
      quota_scope: route:qwen
      provider: dashscope
      path_scope: bound_target
  - ingress:
      - ai-memory-digest
    config:
      event_kind: internal_cost
      quota_scope: internal:ai-memory.digest
      provider: dashscope
```

## 配置说明

| 名称 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `redis_stream` | object | 无 | Redis Stream 投递目标 |
| `redis_stream.service_name` | string | 无 | Redis 服务名或 Console 管理的服务注册名 |
| `redis_stream.service_port` | integer | `6379` | Redis 服务端口 |
| `redis_stream.username` | string | 空 | Redis 用户名 |
| `redis_stream.password` | string | 空 | Redis 密码，需按敏感字段处理 |
| `redis_stream.database` | integer | `0` | Redis database |
| `redis_stream.timeout` | integer | `500` | Redis callout 超时时间，单位毫秒 |
| `redis_stream.stream` | string | `billing:events` | Redis Stream 名称 |
| `event_kind` | string | `usage` | 事件类型，可选 `usage` 或 `internal_cost` |
| `quota_scope` | string | `global` | 当前路由或规则的额度作用域 |
| `provider` | string | `default` | AI provider 标识 |
| `tenant_header` | string | `x-mse-tenant` | 租户身份请求头 |
| `consumer_header` | string | `x-mse-consumer` | consumer 身份请求头 |
| `path_scope` | string | `legacy_suffixes` | `legacy_suffixes` 或 `bound_target` |
| `enable_path_suffixes` | []string | 历史有效默认值为 `/v1/chat/completions`、`/v1/messages` | 可选路径过滤器 |
| `enable_methods` | []string | 无 | 可选 HTTP 方法过滤器，按精确大小写匹配 |
| `fail_policy` | string | `open` | 投递失败策略，当前支持 `open` |

全局 `defaultConfig` 必须配置 `redis_stream.service_name`，并推荐放置共享的租户、consumer、路径作用域和 fail-open 设置。`matchRules[].config` 可以只配置当前路由差异，例如 `event_kind`、`provider`、`quota_scope`、`path_scope`、`enable_path_suffixes`、`enable_methods` 或 `fail_policy`；未配置字段会继承全局值。规则级 `redis_stream` 不支持配置，Redis Stream 目标属于插件实例共享配置。

缺省或显式 `legacy_suffixes` 保留上述两个历史默认后缀。在 `bound_target` 下，Higress 绑定目标定义请求集合；缺省或字面空数组过滤器不额外收窄，显式非空过滤器只与绑定目标求交集且不能扩大绑定。ai-billing 对 `enable_methods` 使用精确大小写匹配，示例采用规范的大写方法：

```yaml
path_scope: bound_target
enable_path_suffixes:
  - /v1/responses
enable_methods:
  - POST
```

平台自有的内部 AI Route，例如 `ai-cache.embedding`、`ai-memory.digest`，应配置 `event_kind=internal_cost` 和内部 `quota_scope`。这类事件仍携带 usage、provider、model 和 route 事实，供 Console/billing-service 计算平台成本，但下游不能把它们生成普通客户 usage 账单。

### 受信任的内部成本归因请求头

对于匹配 `internal_cost` 规则的请求，受信任的内部调用方可以同时提供 `x-ai-billing-cost-source` 和 `x-ai-billing-worker-kind`。插件会对该请求使用这对值；如果两者均未提供，则回退到路由配置。仅提供其中一个请求头或请求头值无效时，插件会抑制该请求的 billing event 投递，但不会阻塞模型流量。这些请求头不能启用 `internal_cost` 模式，也不能设置 `bill_customer`。

Console 管理的 `ai-billing` 配置会把 Redis 服务投影为 McpBridge registry 名称，例如 `redis-stack-server.dns`。旧的 `billing_service`、`path` 和 `auth_token` 配置已移除；从旧版本升级时需要删除并重新创建已有 HTTP 形态的 `WasmPlugin`，或通过 Console 重新投影托管绑定。

`ai-billing` 与 `ai-quota`、`ai-statistics` 相互独立，可以在另外两个插件关闭时单独上报 billing event；实际余额扣减和 Redis 投影刷新由 Console/billing-service 完成。
