---
title: AI 账单事件
keywords: [AI网关, AI账单, ai-billing]
description: ai-billing 托管交付事实与兼容账单事件配置参考
---

## 完整冻结凭据目录

`0.1.11` 配置契约保留 `_modelfusion_execution` v1 的单一 `credential_ref`/`credential_revision`，并支持 `modelfusion-execution.v2` 的完整非空 `credentials` 数组。每项仅含这两个字段，ref 唯一、revision 为规范正十进制字符串；拒绝秘密及 scalar/array 混用。完整目录不代表本次已选凭据，实际 D0/FACT 仍保留唯一已选 tuple，billing/cache 不取目录首项作为执行凭据。

> 交付版本：`0.1.11`，以 [资源 schema](spec.yaml) 为准。

## 功能说明

`ai-billing` 在托管原生链中接收 ai-proxy 已冻结的最终交付事实，按原字节投递到受控 Redis Stream；兼容配置保留请求级 `BillingEvent`。插件不计算最终权威费用、不应用折扣、不扣减余额，也不更新账户数据库；事实接收、幂等结算、账单流水、余额投影和补偿归 Console/billing-service 所有。

## 托管原生交付

每个托管目标规则必须成对携带 `native_execution: {producer_id: ...}` 与 SDK 保留对象 `_modelfusion_execution`。后者按上述 v1/v2 分支保留完整冻结 descriptor、原 route/sales_asset/public_model 及单一或完整池凭据引用。producer、release、目标及钱包归属来自受控发布和认证链，不能由请求 body、最终 model 或 URL 推断。

托管配置固定 `path_scope: bound_target`，`enable_methods`、`enable_path_suffixes` 省略或为空数组；不接受手工 URL 或方法筛选。规则范围随绑定路由的全部方法；实际下游请求仍受原生文本执行能力约束。不能靠过滤掉某方法绕过本路由的账务链。

请求 header 阶段，billing 保存 key-auth 安装的原 tenant/principal/account/api_key/credential_generation 身份，关闭兼容事件生成，并跳过请求及响应正文读取。billing 早于 proxy 处理请求，不能把客户端 call/attempt ID 当成已发生的执行。

响应阶段消费内部 `x-modelfusion-delivery-observation` 桥：

1. proxy 清除上游所有同名值，只安装本次冻结的 `call_delivery` Snapshot。
2. billing 移除所有桥 header，要求唯一且合法的规范编码，核对 producer、release、route、sales_asset、public_model 及原 tenant/principal/account。
3. 向配置的私有 Stream 投递 `XADD <stream> * message_type observation event <Snapshot.Bytes>`，保留同一 event/source/revision 及原始规范字节。未知投递结果不能改键、改正文或另造事件。

该分支不从响应估算用量，不生成兼容 `BillingEvent`，不补造 attempt、采购归属或价格。执行登记与逐 attempt 事实由实际发送方 ai-proxy 负责，最终交付由 billing 唯一投递。接收端从已配置的 Stream producer grant 校验权限，body 中的 producer_id 不授予角色。

桥缺失/重复/不合法、归属不符、Redis 投递失败或确认不明时，沿 Q41 记录 `gap`/`unknown` 与固定原因，保持实际响应继续；不虚构零费用、成功持久化或补偿完成。配置身份或连接初始化错误仍会拒绝配置，不能用 fail-open 放宽发布校验。日志不包含桥原文、原始 API Key 或真实 Redis 凭据。

托管规则使用本目标完整配置，连接规则与普通配置相同：有效全局 Redis 存在时规则继承且不能更换；仅路由根下每条规则自带完整 `redis_stream`。`native_execution` 与其配对 envelope 只属于各目标规则，不能在全局启用或继承另一目标的原生身份。非空且无效的基础配置即使附带合法规则，也不能退回独立规则模式绕过校验。

## 兼容账单事件契约

以下事件、内部成本归因、响应关联及配置示例适用于未启用托管原生分支的兼容配置。该分支通过有效配置选定 Redis Stream，执行 `XADD <stream> * event <BillingEvent JSON>`；投递错误保持 fail-open，不阻塞模型响应。

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
