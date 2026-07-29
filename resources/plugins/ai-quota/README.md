---
title: AI 金额配额
keywords: [AI网关, AI配额, 金额配额]
description: ai-quota 只读 Redis 余额准入资源说明
---

## 功能说明

`ai-quota` 在请求头阶段读取 Console 或 billing-service 派生的 Redis
热余额。余额大于 `0` 时放行；零余额和负余额返回固定的 `403 no_balance`；
余额 key 不存在时由 `missing_balance_policy` 决定拒绝或放行。

这次请求准入是插件对该请求的最终账户执法动作。Redis 余额热数据是插件的
只读输入。权威账户、结算、账单、对账和 Redis 热数据重建属于 Console 或
billing-service。

## 运行属性

插件执行阶段：`默认阶段`

插件执行优先级：`280`

## 请求范围

- 缺省或显式 `path_scope: legacy_suffixes` 保留历史后缀门控，默认后缀为
  `/v1/chat/completions` 和 `/v1/messages`。
- `path_scope: bound_target` 默认处理 Higress 绑定目标已经选中的全部请求；
  缺省或字面空数组过滤器不额外收窄。
- 显式非空的 `enable_path_suffixes` 和 `enable_methods` 是与绑定目标求
  交集的高级过滤器，不能扩大绑定目标。`enable_methods` 按精确大小写
  匹配，示例使用规范的大写方法。

## 配置字段

全局配置支持 `redis`、`quota_scope`、`provider`、`tenant_header`、
`consumer_header`、`balance_key_template`、`path_scope`、
`enable_path_suffixes`、`enable_methods` 和 `missing_balance_policy`。

`redis` 支持 `service_name`、`service_port`、`username`、`password`、
`timeout` 和 `database`，其中 `service_name` 必填。

规则只覆盖 `quota_scope`、`provider`、`path_scope`、
`enable_path_suffixes` 和 `enable_methods`；其余准入基础设施继承自全局。

## 配置示例

```yaml
redis:
  service_name: redis-service.default.svc.cluster.local
  service_port: 6379
  username: <redis-username>
  password: <redis-password>
  timeout: 1000
  database: 0
quota_scope: global
provider: default
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
balance_key_template: "billing:balance:{tenant}:{quota_scope}:{consumer}"
path_scope: legacy_suffixes
enable_path_suffixes:
  - /v1/chat/completions
  - /v1/messages
missing_balance_policy: deny
```

绑定目标已经精确选中 provider-native 路由且不需要额外过滤时，规则使用：

```yaml
quota_scope: route:qwen
provider: dashscope
path_scope: bound_target
```

显式交集示例：

```yaml
path_scope: bound_target
enable_path_suffixes:
  - /v1/responses
enable_methods:
  - POST
```

`ai-quota`、`ai-billing` 和 `ai-statistics` 不共享私有运行时状态，可以
独立部署。
