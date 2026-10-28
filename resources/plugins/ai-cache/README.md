---
title: AI 缓存
keywords: [higress,ai cache]
description: ai-cache 薄网关缓存插件配置参考
---

**Note**

> 若使用 tinygo 编译，则需要数据面的 proxy wasm 版本大于等于 0.2.100，且编译时需要带上版本的 tag，例如：`tinygo build -o main.wasm -scheduler=none -target=wasi -gc=custom -tags="custommalloc nottinygc_finalizer proxy_wasm_version_0_2_100" ./`

## 功能说明

`ai-cache` 是一个薄 Higress Wasm-Go 网关缓存插件。它只在在线请求路径上执行
Console 已物化缓存记录的读取、校验、OpenAI-compatible 回放、响应事实捕获和
Redis Stream `CacheEvent` 投递。

插件不在网关内生成 embedding，不执行 vector search，不写 PostgreSQL，不做价格计算、
账单结算、状态修复或删除，也不持有上游模型凭据。缓存物化、semantic lookup、
embedding、vector indexing、pricing、settlement、repair、deletion 和 reconciliation
均由 Console 或后端 worker 负责。

**提示**

携带请求头 `x-higress-skip-ai-cache: on` 时，当前请求不会使用缓存内容，会直接转发给后端服务，
同时该请求返回的响应也不会写入缓存事件。

## 运行属性

插件执行阶段：`default`
插件执行优先级：`250`

## 运行职责

仅同步文本输出请求参与缓存；明确请求非文本输出或 Responses 后台执行时直接旁路，文本与音频等混合响应也不生成文本缓存。图片、音频和文件等输入附件仍可用于文本输出请求。

用量从原 JSON/SSE 计数保留，区分字段缺失与明确的 `0`，不推算缺项、总量或供应商缓存拆分。平台缓存命中不改写供应商缓存计数；原生计费桥遇到不完整用量时仍报告 missing，交由账务挂账处理。

插件负责：

- 按 JSON `content-type`、`route_policy.enabled_path_suffixes` 和 no-store 信号做 request gating。
- 从 `tenant_header`、`consumer_header`、`session_header` 和请求上下文提取身份与请求事实。
- 为 AI 请求生成带 tenant、consumer、route、model、scope、policy 和 request digest 的物化查询 key。
- 从 Redis 读取 Console 预生成的结构化回放记录。
- 在 Redis miss 后可选短超时调用 Console `/internal/cache/lookup`。
- 对物化记录执行 schema、过期时间、scope、tenant、consumer、route、model、policy 和 digest 校验。
- 命中后只回放安全的结构化响应或流式 chunk。
- 捕获符合策略的上游响应事实，并在响应结束后通过 Redis Stream 投递 `CacheEvent`。
- 对 Redis、Console、解析、校验、响应捕获和事件投递失败执行 fail-open。

插件不负责：

- 在线 embedding、rerank 或 vector search。
- PostgreSQL 持久化、物化记录生成、修复或删除。
- provider 价格、折扣、余额、账单结算或 reconciliation。
- backend model cost attribution。
- 暴露缓存管理 API 或执行管理面鉴权。

## 配置说明

`ai-cache` 公开配置只保留薄缓存路径：materialized Redis lookup、可选 Console lookup、
CacheEvent Redis Stream、route policy、身份 header、缓存 scope、policy version 和 fail-open
策略。旧的 `vector`、`embedding`、`cache`、GJSON 提取路径和响应模板配置不再作为资源插件配置项暴露。

```yaml
materialized_lookup:
  redis:
    enabled: true
    service_name: redis-stack-server.dns
    service_port: 6379
    key_prefix: cache:materialized:
    timeout: 80
console_lookup:
  enabled: true
  service_name: modelfusion-console-api.dns
  service_port: 80
  path: /internal/cache/lookup
  timeout: 120
redis_stream:
  enabled: true
  service_name: redis-stack-server.dns
  service_port: 6379
  stream: cache:events
  field: event
  timeout: 120
route_policy:
  enable_redis_lookup: true
  enable_console_lookup: true
  enable_replay: true
  enable_bypass: false
  enabled_path_suffixes:
  - /v1/chat/completions
  memory:
    enabled: true
    cache_mode: policy_digest
    policy_version: memory-policy-v1
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
session_header: x-openclaw-session-key
cache_scope: consumer
cache_policy_version: cache-policy-v1
fail_policy: open
```

## 配置字段

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| materialized_lookup.redis.enabled | bool | false | 启用 Redis 物化回放查询 |
| materialized_lookup.redis.service_name | string | - | 物化记录 Redis 服务名 |
| materialized_lookup.redis.service_port | int | 6379 | Redis 服务端口 |
| materialized_lookup.redis.key_prefix | string | `cache:materialized:` | 物化记录 key 前缀 |
| materialized_lookup.redis.timeout | int | 80 | Redis 查询超时时间，单位毫秒 |
| console_lookup.enabled | bool | false | Redis miss 后启用可选 Console 查询 |
| console_lookup.service_name | string | - | Console 服务名 |
| console_lookup.service_port | int | 80 | Console 服务端口 |
| console_lookup.path | string | `/internal/cache/lookup` | Console 查询路径 |
| console_lookup.timeout | int | 120 | Console 查询超时时间，单位毫秒 |
| redis_stream.enabled | bool | false | 启用 `CacheEvent` 投递 |
| redis_stream.service_name | string | - | 事件 Redis 服务名 |
| redis_stream.service_port | int | 6379 | 事件 Redis 服务端口 |
| redis_stream.stream | string | `cache:events` | Redis Stream 名称 |
| redis_stream.field | string | `event` | Redis Stream 字段名 |
| redis_stream.timeout | int | 120 | 事件投递超时时间，单位毫秒 |
| route_policy.enable_redis_lookup | bool | true | 允许 Redis 物化查询 |
| route_policy.enable_console_lookup | bool | `console_lookup.enabled` | 允许 Console fallback 查询 |
| route_policy.enable_replay | bool | true | 校验命中后允许本地回放 |
| route_policy.enable_bypass | bool | false | 对当前路由绕过薄缓存 |
| route_policy.enabled_path_suffixes | []string | nil | 允许缓存的请求路径后缀；为空时不限制路径 |
| route_policy.memory.enabled | bool | false | 启用 memory-aware 缓存策略 |
| route_policy.memory.cache_mode | string | `policy_digest` | `policy_digest` 将 memory policy 和 digest 纳入缓存 key；`bypass` 跳过缓存 |
| route_policy.memory.policy_version | string | - | memory-aware 缓存 key 使用的 memory policy 版本 |
| tenant_header | string | `x-mse-tenant` | 租户身份请求头 |
| consumer_header | string | `x-mse-consumer` | 消费者身份请求头 |
| session_header | string | `x-openclaw-session-key` | 会话身份请求头 |
| cache_scope | string | `tenant` | `tenant` 或 `consumer`；memory-aware 路由建议使用 `consumer` |
| cache_policy_version | string | - | 启用薄缓存查询、回放或事件投递时必填 |
| fail_policy | string | `open` | 当前生产行为为 fail-open |

`policy_digest` 通过私有 Proxy-Wasm filter-state 属性
`mse_ai_memory_context_digest` 接收 `ai-memory` 生成的摘要，不使用 HTTP
请求头。两个插件都会移除已废弃的 `x-mse-memory-digest` 请求头。

## 物化回放记录

物化记录是 Console 拥有的 Redis value，使用 `schema_version:
ai-cache.materialized.v1`。记录必须包含 tenant、可选 consumer、route、model、cache
scope、cache policy version、request digest、soft/hard 过期时间、`usage`、
`finish_reason`，以及非流式回放的 `response`，或流式回放的
`stream_replayable: true` 和 `stream_chunks`。

插件会拒绝 schema 版本不支持、过期、scope/tenant/consumer/route/model/policy/digest
不匹配以及 payload 格式错误的记录，并继续上游请求。

## CacheEvent

上游响应结束后，插件使用如下 Redis Stream 写入：

```text
XADD cache:events * event <CacheEvent JSON>
```

事件包含身份、路由、模型、请求摘要、policy、状态码、流式标记、gate 标记、
开始/结束时间和插件版本。策略允许时，事件可以包含用户内容、助手内容、usage、
finish reason 以及安全的 provider/runtime 事实。

事件不得包含 Authorization 请求头、API Key、internal bearer token、Redis 凭据、
上游 provider 凭据或敏感原文。Redis Stream 投递失败时用户响应保持不变。

## 插件顺序和隔离

当 `ai-memory` 会影响回答时，推荐将 `ai-memory` 放在 `ai-cache` 之前。推荐的缓存策略包括
consumer 级作用域、使用 `policy_digest` 将 memory policy version 和组装 memory digest
纳入 key/policy 材料，或在 Console 尚未物化 memory-aware 回放记录前使用 `bypass`。

`ai-cache` 与 `ai-memory` 后端状态必须隔离：cache 和 memory 使用独立 PostgreSQL 表、
Redis Stream、Redis key 前缀、vector namespace、payload schema、retention/deletion
policy、management API 和 authorization boundary。

## Billing 事实

缓存回放时，`ai-cache` 会设置可信网关事实，例如 `upstream_invoked=false`，供 `ai-billing`
消费。真实上游 provider 调用会发出 `upstream_invoked=true`。这些事实不会加入用户可见的
OpenAI 响应体，用户请求或响应体中的同名字段也不能控制它们。
