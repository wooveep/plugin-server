# wasm-go 文档

当前需求、设计和验证记录使用普通 Markdown 与 Git/PR 记录维护。插件行为与配置以当前插件 README 和资源 schema 为入口。

## 当前插件契约

| 主题 | 实现说明 | 资源配置 schema |
| --- | --- | --- |
| 账单事件与请求关联 | [ai-billing](../extensions/ai-billing/README.md) | [ai-billing schema](../resources/plugins/ai-billing/spec.yaml) |
| Redis 余额准入 | [ai-quota](../extensions/ai-quota/README.md) | [ai-quota schema](../resources/plugins/ai-quota/spec.yaml) |
| 薄缓存与物化回放 | [ai-cache](../extensions/ai-cache/README.md) | [ai-cache schema](../resources/plugins/ai-cache/spec.yaml) |
| 薄记忆与事件投递 | [ai-memory](../extensions/ai-memory/README.md) | [ai-memory schema](../resources/plugins/ai-memory/spec.yaml) |
| 上游协议与能力适配 | [ai-proxy](../extensions/ai-proxy/README.md) | [ai-proxy schema](../resources/plugins/ai-proxy/spec.yaml) |
| 请求与路由可观测性 | [ai-statistics](../extensions/ai-statistics/README.md) | [ai-statistics schema](../resources/plugins/ai-statistics/spec.yaml) |

## 设计与验证资料

- [Higress 上游基线与独立插件维护边界](2026-10-22-upstream-boundary.md)
- [缓存与记忆共享运行时设计](2026-06-26-ai-cache-memory-shared-runtime-design.md)
- [薄缓存设计](ai-cache-thin-plugin-design.md)
- [Wasm 镜像规范](wasm-image-spec.md)与[插件构建入口](../Makefile)
- [测试框架说明](../pkg/test/README.md)
- [发布验证记录](release-evidence/)

## 历史资料

OpenSpec 已退出当前需求管理流程。[剩余 OpenSpec 档案](../openspec/)与发布记录中保存的 OpenSpec 命令仅用于追溯当时的变更和验证方式，不作为新实施的前置要求。

2026-10-22 清理了旧 OpenSpec 资料、生成的技能副本和本地 Hook。清理前版本为提交 `6a291d9558093a13e92e5ae8e906e5cbf55a70b4`，可通过 Git 查阅已删除文件，例如：

```bash
git show 6a291d9558093a13e92e5ae8e906e5cbf55a70b4:openspec/specs/ai-billing-events/spec.md
```

现行插件 README 与资源 schema 保留在上表所列位置；GitNexus 的使用入口在 [AGENTS.md](../AGENTS.md)。

## Qwen/DashScope 历史事项核验

清理时，原 `support-qwen-dashscope-openai-media-conversion` 归档仍有两项可选线上兼容性检查未勾选。2026-10-22 已在默认 `modelfusion-dev` VM K3S + Helm 环境完成核验，详见[验收记录与 Higress 更新分析](release-evidence/2026-10-22-qwen-dashscope-k3s.md)和[脱敏证据](release-evidence/2026-10-22-qwen-dashscope-k3s.json)。

| 原任务 | 核验事项 | 状态 |
| --- | --- | --- |
| 6.2 | 对更新后的 ai-proxy 运行 Modelfusion 本地媒体验证，确认图片生成和语音合成不再报告 `plugin_conversion_gap`。 | 已核验：两项 HTTP 200，PNG 与实际 PCM 数据检查通过。 |
| 6.3 | 保存剩余上游 provider 错误的脱敏证据，排除 API key、临时 URL 和凭据。 | 已核验：保留非法音色的上游 `InvalidParameter` 400；临时媒体 URL 已整体替换，凭据未入库。 |

原任务和设计可在上述清理前提交的 `openspec/changes/archive/2026-06-29-support-qwen-dashscope-openai-media-conversion/` 下查阅。此次验收基于真实网关调用；语音接口返回带音频 URL 的 JSON。上游返回 HTTP URL、WAV 声明长度与实际数据不一致的限制已在验收记录中注明。

附加收尾检查发现令牌吊销后仍能到达上游的独立缺陷，已记录并关闭本次测试路由。本表仅表示两项媒体任务通过，不表示凭据吊销或整套环境验收通过。
