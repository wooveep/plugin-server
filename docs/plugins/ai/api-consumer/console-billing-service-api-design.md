---
title: Console Billing Service API 设计规范
keywords: [AI网关, Billing Service, Console, 账单, 余额, 价格]
description: console billing-service 与 ai-quota、ai-billing 对接的 API 和数据契约设计
---

# Console Billing Service API 设计规范

## 1. 背景

`ai-quota` 和 `ai-billing` 插件只运行在 Higress 网关请求链路内，不适合作为账户、价格、余额和账单流水的事实源。`ai-billing` 上报的是请求详单事件，余额真实源应是 billing-service DB 中的账单流水和账户余额投影。console billing-service 需要承担统一的管理面和账务事实源职责，并向网关侧刷新 Redis 热数据。

本规范定义 console billing-service 与网关插件的对接边界、核心数据模型、Redis key 约定和 API 设计。

## 2. 总体职责

```text
console UI
  └─ 管理 tenant、consumer、quota_scope、价格策略、余额、账单
        │
        ▼
billing-service
  ├─ DB 事实源
  ├─ billing event 入库
  ├─ 账单流水事务
  ├─ 账户余额投影
  ├─ 基础价格表管理
  ├─ 租户折扣和覆盖价格
  ├─ billing event 幂等处理
  ├─ 账单流水写入
  ├─ Redis 热余额刷新
  └─ Redis 租户生效价格刷新
        │
        ▼
Redis
  ├─ billing:balance:{tenant}:{quota_scope}:{consumer}
  ├─ billing:effective_price:{tenant}:{provider}:{model}:input
  └─ billing:effective_price:{tenant}:{provider}:{model}:output
        ▲
        │
Higress Gateway
  ├─ ai-quota 只读热余额执行请求准入
  └─ ai-billing 上报 billing event
```

## 3. 数据归属

| 数据 | 事实源 | Redis 作用 | 说明 |
| --- | --- | --- | --- |
| tenant | billing-service DB | 无或缓存 | 租户是价格策略和余额隔离边界 |
| consumer 账户 | billing-service DB | 无或缓存 | consumer 归属于 tenant |
| quota_scope | billing-service DB | 无或缓存 | AI 路由额度作用域 |
| billing event | billing-service DB | 可选补偿队列 | 由 `ai-billing` 上报，是结算输入 |
| 账单流水 | billing-service DB | 不缓存或仅查询缓存 | 余额真实源，包含充值、消费、调整、补偿 |
| account balance 投影 | billing-service DB | 热余额 | 由流水事务更新，用于快速查询和 Redis 重建 |
| 基础价格 | billing-service DB | 不直接给网关使用 | provider/model 标准价格 |
| 租户价格策略 | billing-service DB | 不直接给网关使用 | 折扣或覆盖价格 |
| 租户生效价格 | billing-service DB 或计算投影 | 热价格 | 供 billing-service 结算与缓存重建 |
| 幂等状态 | billing-service DB | 短 TTL 去重 | 防重复结算 |

余额真实源关系：

```text
account_balance = sum(billing_statements)
```

工程实现可以使用 `account_balances` 表保存当前余额投影，但该投影必须由账单流水、充值流水、调整流水在同一 DB 事务内更新。Redis 余额只是 `ai-quota` 的只读请求准入输入；结算、账户状态变更和 Redis 投影刷新均由 billing-service 负责，Redis 不能作为真实源。

## 4. 金额和价格单位

金额必须使用整数存储和传输，避免浮点误差。

推荐：

```text
amount_scale = 1,000,000
1 元 = 1,000,000 micro_yuan
price_unit_tokens = 1,000,000
```

基础价格和租户生效价格都表示为每 `price_unit_tokens` token 的 micro 金额：

```json
{
  "provider": "dashscope",
  "model": "qwen-turbo",
  "input_price": 800000,
  "output_price": 2000000,
  "amount_scale": 1000000,
  "price_unit_tokens": 1000000,
  "currency": "CNY"
}
```

费用计算：

```text
input_cost = ceil(input_tokens * input_price / price_unit_tokens)
output_cost = ceil(output_tokens * output_price / price_unit_tokens)
total_cost = input_cost + output_cost
```

## 5. Redis key 规范

### 5.1 余额

```text
billing:balance:{tenant}:{quota_scope}:{consumer}
```

示例：

```text
billing:balance:tenant-a:route:qwen:consumer-a = 50000000
billing:balance:tenant-a:route:deepseek:consumer-a = 10000000
```

含义：

- value 为整数金额，单位为 micro currency。
- 同一 tenant 下的同一 consumer 可以在不同 `quota_scope` 下拥有不同余额。
- 不同 tenant 的余额必须相互隔离。

### 5.2 价格

```text
billing:effective_price:{tenant}:{provider}:{model}:input
billing:effective_price:{tenant}:{provider}:{model}:output
billing:price_version:{tenant}
```

示例：

```text
billing:effective_price:tenant-a:dashscope:qwen-turbo:input = 800000
billing:effective_price:tenant-a:dashscope:qwen-turbo:output = 2000000
billing:price_version:tenant-a = 2026-05-01
```

### 5.3 幂等

```text
billing:idempotency:{request_id}
```

建议 TTL：

```text
24h 到 72h
```

### 5.4 后续预冻结扩展

```text
billing:reservation:{tenant}:{quota_scope}:{consumer}:{request_id}
```

v1 不要求实现预冻结。

### 5.5 Redis 重启恢复

Redis 不是真实源。Redis 重启、清空或数据丢失后，billing-service 必须能从 DB 重建热缓存：

```text
1. 从 account_balances 重建 billing:balance:{tenant}:{quota_scope}:{consumer}。
2. 从 price_books 和 tenant_price_rules 计算租户生效价格。
3. 重建 billing:effective_price:{tenant}:{provider}:{model}:{token_type}。
4. 写入 billing:price_version:{tenant}。
```

重建任务应支持：

- billing-service 启动时 warm-up。
- 管理 API 手动刷新。
- 指定 tenant、quota_scope、consumer、provider、model 的局部刷新。

## 6. API 通用约定

### 6.1 Base URL

```text
/v1/billing
```

### 6.2 响应格式

成功响应：

```json
{
  "success": true,
  "data": {}
}
```

失败响应：

```json
{
  "success": false,
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "invalid request"
  }
}
```

### 6.3 错误码

| code | HTTP 状态码 | 说明 |
| --- | --- | --- |
| `INVALID_ARGUMENT` | 400 | 请求参数错误 |
| `UNAUTHORIZED` | 401 | 未认证 |
| `FORBIDDEN` | 403 | 无权限 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `CONFLICT` | 409 | 幂等冲突或状态冲突 |
| `INSUFFICIENT_BALANCE` | 409 | 余额不足 |
| `INTERNAL_ERROR` | 500 | 服务内部错误 |

## 7. 账户与余额 API

### 7.1 查询余额

```text
GET /v1/billing/tenants/{tenant}/accounts/{consumer}/balances/{quota_scope}
```

响应：

```json
{
  "success": true,
  "data": {
    "tenant": "tenant-a",
    "consumer": "consumer-a",
    "quota_scope": "route:qwen",
    "balance": 50000000,
    "currency": "CNY",
    "amount_scale": 1000000,
    "updated_at": 1730000000000
  }
}
```

### 7.2 设置余额

```text
PUT /v1/billing/tenants/{tenant}/accounts/{consumer}/balances/{quota_scope}
```

请求：

```json
{
  "balance": 50000000,
  "currency": "CNY",
  "reason": "initial grant"
}
```

行为：

- 写入 DB。
- 写入余额变更流水。
- 在同一事务内更新 `account_balances` 投影。
- 刷新 Redis `billing:balance:{tenant}:{quota_scope}:{consumer}`。

### 7.3 调整余额

```text
POST /v1/billing/tenants/{tenant}/accounts/{consumer}/balances/{quota_scope}:adjust
```

请求：

```json
{
  "delta": -1000000,
  "reason": "manual adjustment",
  "idempotency_key": "adjust-001"
}
```

响应：

```json
{
  "success": true,
  "data": {
    "tenant": "tenant-a",
    "consumer": "consumer-a",
    "quota_scope": "route:qwen",
    "balance_before": 50000000,
    "balance_after": 49000000
  }
}
```

行为：

- 使用 `idempotency_key` 防重复调整。
- DB 事务写入调整流水并更新余额投影。
- 刷新 Redis 热余额。

## 8. 租户价格策略 API

### 8.1 查询租户生效价格

```text
GET /v1/billing/tenants/{tenant}/effective-prices/{provider}/{model}
```

响应：

```json
{
  "success": true,
  "data": {
    "tenant": "tenant-a",
    "provider": "dashscope",
    "model": "qwen-turbo",
    "input_price": 800000,
    "output_price": 2000000,
    "currency": "CNY",
    "amount_scale": 1000000,
    "price_unit_tokens": 1000000,
    "version": "2026-05-01",
    "source": "discount"
  }
}
```

### 8.2 配置租户折扣

```text
PUT /v1/billing/tenants/{tenant}/price-rules/{provider}/{model}
```

请求：

```json
{
  "discount": 0.8,
  "version": "2026-05-01",
  "active": true
}
```

### 8.3 配置租户覆盖价格

```text
PUT /v1/billing/tenants/{tenant}/override-prices/{provider}/{model}
```

请求：

```json
{
  "input_price": 700000,
  "output_price": 1800000,
  "currency": "CNY",
  "amount_scale": 1000000,
  "price_unit_tokens": 1000000,
  "version": "2026-05-01",
  "active": true
}
```

处理要求：

- 覆盖价格优先级高于折扣价格。
- 租户生效价格必须由 billing-service 计算并刷新 Redis。
- 租户生效价格供 billing-service 结算使用；`ai-quota` 不理解折扣规则，也不读取价格 key。

## 9. quota_scope API

### 9.1 创建 quota_scope

```text
POST /v1/billing/quota-scopes
```

请求：

```json
{
  "quota_scope": "route:qwen",
  "display_name": "Qwen route quota",
  "description": "Quota scope for qwen AI route"
}
```

### 9.2 查询 quota_scope

```text
GET /v1/billing/quota-scopes/{quota_scope}
```

响应：

```json
{
  "success": true,
  "data": {
    "quota_scope": "route:qwen",
    "display_name": "Qwen route quota",
    "description": "Quota scope for qwen AI route",
    "created_at": 1730000000000,
    "updated_at": 1730000000000
  }
}
```

### 9.3 路由绑定

```text
POST /v1/billing/quota-scopes/{quota_scope}/route-bindings
```

请求：

```json
{
  "route": "ai/qwen-route",
  "tenant": "tenant-a",
  "provider": "dashscope",
  "default_model": "qwen-turbo"
}
```

说明：

- 该绑定用于 console 生成或校验 Higress WasmPlugin `matchRules`。
- 网关运行时仍以 WasmPlugin 配置中的 `quota_scope` 为准。

## 10. 基础价格 API

### 10.1 查询价格列表

```text
GET /v1/billing/prices
```

可选查询参数：

```text
provider=dashscope
model=qwen-turbo
active=true
```

响应：

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "provider": "dashscope",
        "model": "qwen-turbo",
        "input_price": 800000,
        "output_price": 2000000,
        "currency": "CNY",
        "amount_scale": 1000000,
        "price_unit_tokens": 1000000,
        "version": "2026-05-01",
        "active": true
      }
    ]
  }
}
```

### 10.2 查询单个模型价格

```text
GET /v1/billing/prices/{provider}/{model}
```

### 10.3 创建或更新价格

```text
PUT /v1/billing/prices/{provider}/{model}
```

请求：

```json
{
  "input_price": 800000,
  "output_price": 2000000,
  "currency": "CNY",
  "amount_scale": 1000000,
  "price_unit_tokens": 1000000,
  "version": "2026-05-01",
  "active": true
}
```

行为：

- 写入 DB 价格表。
- 刷新 Redis：
  - 重新计算受影响租户的生效价格
  - `billing:effective_price:{tenant}:{provider}:{model}:input`
  - `billing:effective_price:{tenant}:{provider}:{model}:output`
  - `billing:price_version:{tenant}`

## 11. Billing Event API

### 11.1 上报账单事件

```text
POST /v1/billing/events
```

调用方：

- `ai-billing` 插件。
- 后续补偿任务。

请求：

```json
{
  "request_id": "req-001",
  "idempotency_key": "consumer-a:route:qwen:req-001",
  "tenant": "tenant-a",
  "consumer": "consumer-a",
  "quota_scope": "route:qwen",
  "provider": "dashscope",
  "model": "qwen-turbo",
  "route": "ai/qwen-route",
  "cluster": "outbound|443||dashscope.example.com",
  "input_tokens": 10,
  "output_tokens": 100,
  "total_tokens": 110,
  "stream": true,
  "status_code": 200,
  "started_at": 1730000000000,
  "ended_at": 1730000001200,
  "usage_missing": false,
  "price_version": "2026-05-01"
}
```

响应：

```json
{
  "success": true,
  "data": {
    "event_id": "evt-001",
    "request_id": "req-001",
    "idempotent_replayed": false,
    "calculated_cost": 220,
    "currency": "CNY",
    "price_version": "2026-05-01"
  }
}
```

处理要求：

- `idempotency_key` 必须唯一。
- 重复上报同一 `idempotency_key` 时不得重复写账单流水。
- billing-service 必须使用 DB 中的基础价格和租户价格策略重新计算最终费用。
- `usage_missing = true` 时，不做正常 token 费用结算，应写入异常事件。
- 成功结算时，billing-service 必须在同一事务内写入账单流水并更新账户余额投影。

### 11.2 查询账单事件

```text
GET /v1/billing/events/{request_id}
```

响应：

```json
{
  "success": true,
  "data": {
    "event_id": "evt-001",
    "request_id": "req-001",
    "tenant": "tenant-a",
    "consumer": "consumer-a",
    "quota_scope": "route:qwen",
    "provider": "dashscope",
    "model": "qwen-turbo",
    "input_tokens": 10,
    "output_tokens": 100,
    "calculated_cost": 220,
    "status": "settled",
    "created_at": 1730000001300
  }
}
```

## 12. 账单流水 API

### 12.1 查询流水

```text
GET /v1/billing/statements
```

查询参数：

```text
tenant=tenant-a
consumer=consumer-a
quota_scope=route:qwen
from=1730000000000
to=1730100000000
type=usage
```

响应：

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "statement_id": "stmt-001",
        "tenant": "tenant-a",
        "consumer": "consumer-a",
        "quota_scope": "route:qwen",
        "type": "usage",
        "amount": -220,
        "balance_after": 49999780,
        "request_id": "req-001",
        "created_at": 1730000001300
      }
    ],
    "next_token": ""
  }
}
```

流水类型建议：

| type | 说明 |
| --- | --- |
| `recharge` | 充值 |
| `adjustment` | 人工调整 |
| `usage` | AI 请求消费 |
| `refund` | 退款或补偿 |
| `correction` | 对账修正 |

## 13. Redis 刷新 API

### 13.1 手动刷新余额缓存

```text
POST /v1/billing/cache/balances:refresh
```

请求：

```json
{
  "tenant": "tenant-a",
  "consumer": "consumer-a",
  "quota_scope": "route:qwen"
}
```

### 13.2 手动刷新价格缓存

```text
POST /v1/billing/cache/effective-prices:refresh
```

请求：

```json
{
  "tenant": "tenant-a",
  "provider": "dashscope",
  "model": "qwen-turbo"
}
```

### 13.3 全量重建缓存

```text
POST /v1/billing/cache:rebuild
```

请求：

```json
{
  "tenant": "tenant-a"
}
```

说明：

- `tenant` 缺省时表示全量重建所有租户缓存。
- 该接口用于 Redis 重启、数据清理或价格策略批量变更后的恢复。

## 14. 对账与补偿

billing-service 应提供异步对账能力：

- 对比 billing event、服务端结算结果和账单流水。
- 在 billing-service 刷新投影后对比 Redis 热余额与 DB 余额。
- 对 usage 缺失事件进行人工或自动补偿。
- 对服务端价格事实缺失导致的未结算事件进行补偿。

建议内部任务：

```text
1. scan unsettled events
2. recalculate cost by DB base price and tenant price rules
3. write statement
4. update account_balances projection
5. refresh Redis balance and effective price when needed
6. mark event settled
```

## 15. 安全要求

- 管理 API 必须只允许 console 后端或授权管理身份调用。
- `POST /v1/billing/events` 必须校验来自网关或可信内部网络。
- 所有余额调整 API 必须写审计日志。
- 所有幂等 key 的请求体摘要应可追溯，避免同 key 不同内容被静默覆盖。

## 16. 验收标准

- billing-service 可以管理 tenant 下 consumer 在不同 `quota_scope` 下的金额余额。
- billing-service 可以管理 provider/model 基础价格。
- billing-service 可以管理 tenant 级折扣和覆盖价格。
- billing-service 可以生成租户生效价格并刷新到 Redis。
- 更新余额后 Redis 热余额同步刷新。
- 更新基础价格或租户价格策略后 Redis 租户生效价格同步刷新。
- `ai-billing` 上报 billing event 后，billing-service 能幂等写入事件和流水。
- billing-service 能在同一事务内写入流水并更新 `account_balances` 投影。
- 重复 billing event 不会重复结算。
- usage 缺失事件会被记录为异常事件。
- billing event、服务端结算结果和账单流水不一致时，billing-service 可以记录差异并后续补偿。
- Redis 重启或数据丢失后，billing-service 可以从 DB 全量或局部重建余额和租户生效价格缓存。
