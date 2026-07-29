---
title: AI 金额配额
keywords: [AI网关, AI配额, 金额配额]
description: ai-quota 只读 Redis 余额准入插件配置参考
---

## 功能说明

`ai-quota` 在请求头阶段读取 Console 或 billing-service 派生的 Redis
热余额。余额大于 `0` 时放行；零余额和负余额返回固定的
`403 no_balance`；只有余额 key 不存在时才由
`missing_balance_policy` 决定拒绝或放行。

请求准入是插件对该请求的最终账户执法动作。插件把 Redis 余额热数据作为
只读输入，不修改账户状态，也不在响应阶段再次判断配额或写入 Redis。
Console 或 billing-service 负责权威账户、结算、账单、对账以及 Redis
热余额重建。

## 请求范围

- 缺省或显式 `path_scope: legacy_suffixes` 保留历史后缀门控，默认后缀为
  `/v1/chat/completions` 和 `/v1/messages`。
- `path_scope: bound_target` 处理 Higress 绑定目标已经选中的全部请求；
  缺省或字面空数组过滤器不额外收窄，也不需要合成路径列表。
- 在 `bound_target` 下，显式非空的 `enable_path_suffixes` 和
  `enable_methods` 只与绑定目标求交集，不能扩大绑定目标。

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

路由规则只覆盖 `quota_scope`、`provider`、`path_scope` 及显式路径/方法
过滤器。Redis、身份请求头、余额 key 模板和余额缺失策略从全局配置继承。
绑定到 provider-native 路由且无需额外过滤时：

```yaml
quota_scope: route:qwen
provider: dashscope
path_scope: bound_target
```

`ai-quota`、`ai-billing` 和 `ai-statistics` 不共享私有运行时状态，可以
独立部署。
