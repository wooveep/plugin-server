# 健康与媒体运行路径：原生制品批次（2026-10-31）

本批固定代理源 `981805f9`、完整包 **ai-proxy 2.0.13**。这是源码、原生宿主、Redis 与制品交付证据；实际 K3S 整链验收尚未运行，当前环境仍为 Helm111。

普通文本健康读取、自然 429 合并和有限恢复消费直接使用 Redis；原事件写入既有 execution Stream，`message_type=health_runtime`。恢复消费与原事件追加同一 Lua 原子提交，只有首次明确消费允许真实发送。重复/未知消费不重发，迟到成功受原 epoch、撤销水位和有效期约束；重放不重新起算冷却时间。

媒体由 Console 保留原有限 handoff、安装原票据后通知网关。网关在 Redis 消费原票据，使用同一 Stream 回报 `media_dispatch` / `media_observation`；确认通知或写入 Stream 本身不等于厂商接受。票据遗失、过期、未知和已发布状态不生成替代 create。内部作业的原资格检查独立放在 `managed_execution.internal_authority`，保留作业、Key、正文和费用归属核验；健康配置不再借用该 HTTP 地址。

[完整验证回执](health-media-runtime-20261031/validation.json)记录实际命令、日志摘要与初次失败。完整代理 Go/Wasm **321 个顶层 / 2,196 个含子项通过**，约393秒；受管理宿主批次60/161通过；健康原生 Redis 八类业务路径通过。宿主、真实 Redis、实际 Envoy/厂商执行的层级分别记录。

[构建回执](health-media-runtime-20261031/proxy-build-receipt.json)、[整包评审](health-media-runtime-20261031/proxy-package-review.json)、[来源核对](health-media-runtime-20261031/proxy-qualification.json)保留固定 Go1.26.3 容器、533个原提交输入、Wasm/完整包及原生接纳摘要。[镜像回执](health-media-runtime-20261031/image-build-receipt.json)对应已推送制品；没有以部署旧版本代替本批整链验收。
