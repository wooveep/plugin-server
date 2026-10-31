# 媒体 429 恢复：selection 边界修复

2026-11-01，本批沿用 Helm126 的真实原任务调查，不改写原失败、价格与截止时间。六类媒体既有正向与财务验收仍见部署仓库的 [Helm126 权威记录](../../../modelfusion-deploy/docs/checks/2026-10-31-health-media-fresh-deployed.md#media-transport-helm126)。本页的源码通过不代表停机整链已通过。

原任务 `01a192dc-8fd0-7550-abc7-81600359300d` 只创建一次，首个原任务查询得到真实 429。其后没有第二次物理查询，包括 Console 恢复之后。原通知、receiver 记录、Redis 哈希与停机时间线均保留，内容摘要见 [原始重放证据](health-media-runtime-20261101/ai-proxy-2.0.17/original-failure-replay.json)。

实际捕获的 Redis 输入经生产 `Runtime.Lookup` / `DecideNewCall` 返回 `ready / recovery_required`，但同一通知经 `Executor.Selection → NewRecoveryCandidate` 报 `registration profile requires frozen text selection`。媒体 selection 在 claim 提交前被错误送入文本注册校验器；没有 claim 不是 Redis 的冷却时间或 Console 停机造成的。

修复 `d295014c` 为媒体生成与原任务查询实现现有 D0 selection 的严格 codec，复用原摘要与规范化流程，并按原 operation 分派健康恢复校验。生成仍要求原 dispatch 与 live invocation；查询保留原 operation 且禁止携带 create dispatch；完整字段、原摘要、唯一 claim 及 HTTP sender 约束保留。文本注册与 native embedding 原校验路径继续独立。

完整 2.0.17 源快照 `745b97588683622bf080abc2ca37d0a30da30c3a`，固定 Go 1.26.3 容器构建的 Wasm 为 `704a03ff6fa22a178f52a78df63fa3014817e14549c52f704a8395fc86c32ea3`，11,564,239 字节。[构建回执](health-media-runtime-20261101/ai-proxy-2.0.17/build-command.json)与[边界批次](health-media-runtime-20261101/ai-proxy-2.0.17/boundary-commands.json)保留真实命令、退出码和原日志摘要。首次发布命令因 VERSION 尚未同步而在编译前被目录校验拒绝，源版本与 metadata 同步后构建成功，未放宽校验。

旧恢复入口的 overlay 在真实媒体 selection 生产者上复现 9 条失败；修复后的六类生成及三类查询全部通过，另有 8 条混用身份拒绝验证。媒体、健康和事实三个包的统一批次为 77 个顶层、240 个含子项通过；真实 Redis 地址已配置，票据与健康运行测试实际执行。一个可选 Go/Rust 事件导出入口因未指定目录跳过，未计为通过。原任务私有通知只读重放通过；没有向生产 Redis 写入调试状态。

CodeGraph 已先行核对；当前 plugin-server 索引不含新增 Go 包的唯一文件命中，因此按实际路径读取源文件，并与 Console 现有 ExecutionSelection schema、恢复候选与摘要入口交叉核对。没有重建或升级索引。

完整新字节 Go/Wasm 宿主批次为 323 个顶层 / 2,203 个含子项通过，退出码 0；[实际命令](health-media-runtime-20261101/ai-proxy-2.0.17/full-go-wasm-command.json)绑定同一源与 Wasm 字节。完整包已入库，88 份原不可变 manifest 保持，现 89 个版本、55 类正式默认版本；[来源核对](health-media-runtime-20261101/ai-proxy-2.0.17/qualification.json)与[镜像回执](health-media-runtime-20261101/ai-proxy-2.0.17/server-image-build.json)完整保留。制品 HTTP 交付、原任务 refresh、停机与恢复后的精确财务对账仍在推进。
