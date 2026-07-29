---
title: AI 金额配额
keywords: [AI网关, AI配额, 金额配额]
description: ai-quota 只读 Redis 余额准入插件配置参考
---

## 功能说明

`ai-quota` 在请求头阶段读取 Console 或 billing-service 派生的 Redis
热余额。余额大于 `0` 时放行；零余额和负余额返回固定的 `403 no_balance`；
余额 key 不存在时由 `missing_balance_policy` 决定拒绝或放行。

这次请求准入是插件对该请求的最终账户执法动作。插件把 Redis 余额热数据
作为只读输入，不修改账户状态。Console 或 billing-service 负责权威账户、
结算、账单、对账以及 Redis 热数据重建。

## 请求范围

- 缺省或显式 `path_scope: legacy_suffixes` 保留历史后缀门控，默认后缀为
  `/v1/chat/completions` 和 `/v1/messages`。
- `path_scope: bound_target` 默认处理 Higress 绑定目标已经选中的全部请求；
  缺省或字面空数组过滤器不额外收窄，也不要求合成路径列表。
- 在 `bound_target` 下，显式非空的 `enable_path_suffixes` 和
  `enable_methods` 都是与绑定目标求交集的高级过滤器；同时配置时必须全部
  匹配，过滤器不会扩大绑定目标。

## 配置说明

| 名称 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `redis` | object | 无 | Redis 连接配置；`service_name` 必填 |
| `quota_scope` | string | `global` | 当前路由或规则的额度作用域 |
| `provider` | string | `default` | 当前准入上下文的 provider 标识 |
| `tenant_header` | string | `x-mse-tenant` | 租户身份请求头 |
| `consumer_header` | string | `x-mse-consumer` | consumer 身份请求头 |
| `balance_key_template` | string | `billing:balance:{tenant}:{quota_scope}:{consumer}` | 余额 key 模板 |
| `path_scope` | string | `legacy_suffixes` | `legacy_suffixes` 或 `bound_target` |
| `enable_path_suffixes` | []string | 历史默认后缀 | 可选路径过滤器 |
| `enable_methods` | []string | 无 | 可选 HTTP 方法过滤器，按精确大小写匹配 |
| `missing_balance_policy` | string | `deny` | 余额缺失策略：`deny` 或 `allow` |

`redis` 支持 `service_name`、`service_port`、`username`、`password`、
`timeout` 和 `database`。默认端口为 `6379`（static 服务为 `80`），默认
超时为 `1000ms`，默认 database 为 `0`。

## 配置示例

```yaml
redis:
  service_name: redis-service.default.svc.cluster.local
  service_port: 6379
  username: <redis-username>
  password: <redis-password>
  timeout: 1000
  database: 0
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
balance_key_template: "billing:balance:{tenant}:{quota_scope}:{consumer}"
path_scope: legacy_suffixes
enable_path_suffixes:
  - /v1/chat/completions
  - /v1/messages
missing_balance_policy: deny
```

规则只覆盖 `quota_scope`、`provider`、`path_scope` 及显式路径/方法过滤器。
Redis、身份请求头、余额 key 模板和余额缺失策略由全局配置继承。例如，
绑定到 provider-native 路由且无需额外过滤时：

```yaml
quota_scope: route:qwen
provider: dashscope
path_scope: bound_target
```

显式非空过滤器只收窄绑定目标。`enable_methods` 按精确大小写匹配，示例使用
规范的大写方法：

```yaml
path_scope: bound_target
enable_path_suffixes:
  - /v1/responses
enable_methods:
  - POST
```

`ai-quota`、`ai-billing` 和 `ai-statistics` 不共享私有运行时状态，可以
独立部署。
