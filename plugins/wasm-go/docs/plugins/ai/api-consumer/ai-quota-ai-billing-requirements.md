---
title: AI 金额配额与账单插件需求说明（已废弃）
keywords: [AI网关, AI配额, AI账单, ai-quota, ai-billing]
description: 已由当前插件契约取代的历史需求说明
---

# 已废弃：AI 金额配额与账单插件需求说明

本文档已废弃（superseded），不得再作为 `ai-quota` 或 `ai-billing`
的当前实现契约。历史版本曾把配额准入、响应处理、事件投递和账务职责混在
同一设计中；这些边界已经拆分。

当前权威文档：

- [`ai-quota` 配置参考](./ai-quota.md)
- [`ai-quota` 资源契约](../../../../resources/plugins/ai-quota/README.md)
- [`ai-billing` 插件契约](../../../../extensions/ai-billing/README.md)
- [`ai-billing` 资源契约](../../../../resources/plugins/ai-billing/README.md)

当前职责边界是：

- `ai-quota` 只把 Redis 热余额作为只读请求准入输入；请求头阶段的准入结果
  是该插件对请求的最终账户执法动作。
- `ai-billing` 独立发布账单事件，不扣减余额，也不更新账户数据库。
- Console 或 billing-service 拥有权威账户、幂等结算、账单流水、余额投影、
  对账、补偿和 Redis 热余额重建。
- `ai-quota`、`ai-billing` 与 `ai-statistics` 不共享私有运行时状态，可以
  独立部署。

系统级价格表、账单事件消费和 billing-service 结算仍是有效的管理面/账务
职责；废弃的是把这些职责描述为 `ai-quota` 响应阶段行为的旧契约。
