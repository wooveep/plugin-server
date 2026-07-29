---
title: AI 账单事件
keywords: [AI网关, AI账单, ai-billing]
description: ai-billing 请求级账单事件插件配置参考
---

> 插件版本：`0.1.7`

## 功能说明

`ai-billing` 在已启用的 AI 请求完成或流终止时生成一个请求级事件，并通过有效全局或路由配置选中的 Redis Stream 执行 `XADD <stream> * event <BillingEvent JSON>`。投递默认为 fail-open：Redis 初始化、网络、超时或命令错误不会阻塞模型响应。插件不计算最终权威费用、不应用折扣、不扣减余额，也不更新账户数据库；幂等结算、账单流水、余额投影和补偿归 Console/billing-service 所有。

## 账单事件契约

完整事件面包括 `event_id`、`event_kind`、`cost_source`、`worker_kind`、`source_job`、`bill_customer`、`idempotency_key`、`request_id`、`tenant`、`consumer`、`route`、`provider`、`quota_scope`、`model`、`request_path`、`status_code`、`usage`、`usage_missing`、`usage_source`、`start_time_ms`、`end_time_ms`、`is_stream`、`cluster`、可选的 `price_version`，以及可信网关生命周期事实 `upstream_invoked`。请求、响应或用户 payload 不能伪造该事实。`event_id` 在请求开始时生成，默认同时作为 `idempotency_key`，重试沿用同一值。

规范事件类型是 `usage` 和 `internal_cost`。旧值 `customer_usage` 仅作为兼容别名接受，并归一化为 `usage`。`cost_source`、`worker_kind`、`source_job` 和 `bill_customer` 是 `internal_cost` 的内部归因字段；普通 `usage` 事件会省略这些字段。`price_version` 只有在存在可信价格事实时才是可选字段。

可信 provider usage 保持权威，设置 `usage_source=provider`、`usage_missing=false`，并在 `usage.details.provider_usage` 保留完整 provider 对象。没有 provider usage 但识别出的结构允许安全估算时，设置 `usage_source=estimated` 和 `usage_missing=false`，且不会虚构 provider-only 详情。无法可信解析时使用零 token 计数、`usage_missing=true` 和 `usage_source=missing`；这种结果不会虚构 model、provider usage、price、cost 或 settlement。

`usage` 以 `unit=token` 以及 `input`、`output`、`total` 表示规范计数，并可包含 cache 详情。事件不提供顶层 `input_tokens`、`output_tokens`、`total_tokens` 或 `gateway_calculated_cost`。事件和日志不包含原始 API key 或真实 Redis 凭据。`route`、`provider` 和 `model` 使用 `{ "id"?: "...", "name"?: "..." }` 事实对象；绑定目标和路径本身不能证明 model、usage、price 或 cost。

## 可信内部成本归因

仅已配置的 `internal_cost` 路由才有资格使用调用级归因头。受信任调用方必须同时提交 `x-ai-billing-cost-source` 和 `x-ai-billing-worker-kind`：完整的一对会覆盖已配置的回退值；两者都缺失时使用已配置的回退值。部分、空、超长、非法字符或重复的头值只抑制 billing event 投递，模型流量继续且不受阻塞。

接受和拒绝的提交值都不会写入日志。客户路由忽略这些头进行事件分类；它们不能启用 `internal_cost`、不能设置 `bill_customer`，也不能改变配置派生的客户结算分类。

## 响应关联契约

对启用账单的请求，`x-ocmf-gateway-request-id` 使用最终规范请求 ID 替换而不是追加任何上游同名值，并与事件中的 `request_id` 完全一致。该规则覆盖非流式、流式、网关本地和缓存响应；流式结束不会追加第二个 `x-ocmf-gateway-request-id` 值。

缺少规范请求上下文时，插件删除上游值且不会虚构关联 ID。该响应头只证明请求关联，不能证明 Redis 已持久化；持久化状态必须由 Console/billing-service 的消费结果确认。

## 配置与迁移

全局投递目标必须提供 `redis_stream.service_name`。`service_port`、`database`、`timeout` 和 `stream` 的默认值分别是 `6379`、`0`、`500` 和 `billing:events`。示例凭据只能使用 `<redis-username>` 和 `<redis-password>` 占位符。

```yaml
defaultConfig:
  redis_stream:
    service_name: redis-stack-server.dns
    service_port: 6379
    username: <redis-username>
    password: <redis-password>
    database: 0
    timeout: 500
    stream: billing:events
  event_kind: usage
  quota_scope: global
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
      cost_source: ai-memory
      worker_kind: digest
      bill_customer: false
      path_scope: bound_target
```

仅按路由投递时，保持基础配置为空，设置 `defaultConfigDisable: true`，并在每条有效匹配规则中提供完整 Redis 目标：

```yaml
defaultConfigDisable: true
matchRules:
  - ingress:
      - qwen
    config:
      redis_stream:
        service_name: redis-stack-server.dns
        service_port: 6379
        username: <redis-username>
        password: <redis-password>
        database: 0
        timeout: 500
        stream: billing:events
      event_kind: usage
      quota_scope: route:qwen
      provider: dashscope
      path_scope: bound_target
```

空基础配置会以休眠 no-op 状态启动，因此未匹配流量不会读取 body、初始化账单状态或发布事件。任何非空基础配置仍必须提供有效的全局 `redis_stream.service_name`。

全局和路由可配置 `event_kind`、`quota_scope`、`provider`、`cost_source`、`worker_kind`、`bill_customer`、`tenant_header`、`consumer_header`、`path_scope`、`enable_path_suffixes`、`enable_methods` 和 `fail_policy`。存在有效全局 Redis 目标时，规则省略的字段继承全局值，且规则不得替换该目标；基础配置休眠时，每条有效规则都必须完整提供 `redis_stream.service_name`。

缺省或显式 `legacy_suffixes` 保留且只保留两个历史默认后缀：`/v1/chat/completions` 和 `/v1/messages`。`bound_target` 由 Higress 绑定定义请求集合；缺省或字面空过滤器不增加收窄。需要高级过滤时，非空 suffix 和 method 只与绑定目标做精确交集，不能扩大绑定；下面示例将 `/v1/responses` 与 `POST` 同时作为高级交集：

```yaml
path_scope: bound_target
enable_path_suffixes:
  - /v1/responses
enable_methods:
  - POST
```

当前配置面已移除 HTTP 形态的 transport。全局和规则级 `billing_service` 都会被拒绝。规则级 `redis_stream` 仅可用于休眠的空基础配置，不能替换已有全局目标。升级旧对象时应删除旧 `billing_service`、`path`、`auth_token` 输入，并重新创建 `WasmPlugin` 或通过 Console 重新投影托管绑定。

## 独立部署边界

`ai-billing`、`ai-quota` 和 `ai-statistics` 可以独立部署。`ai-billing` 只发布账单事件，不依赖或共享彼此的私有运行时状态；余额准入和可观测性分别由另外两个插件独立处理，结算与持久化归 Console/billing-service 所有。
