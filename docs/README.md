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

## 待核验的历史验证事项

清理时，原 `support-qwen-dashscope-openai-media-conversion` 归档的可选线上兼容性检查仍有两项未勾选，未找到对应独立验收证据。本次将它们转记如下，状态均为“待核验”；负责人和期限待确认。

| 原任务 | 待核验事项 | 状态 |
| --- | --- | --- |
| 6.2 | 对更新后的 ai-proxy 运行 Modelfusion 本地媒体验证，确认图片生成和语音合成不再报告 `plugin_conversion_gap`。 | 待核验 |
| 6.3 | 保存剩余上游 provider 错误的脱敏证据，排除 API key、临时 URL 和凭据。 | 待核验 |

原任务和设计可在上述清理前提交的 `openspec/changes/archive/2026-06-29-support-qwen-dashscope-openai-media-conversion/` 下查阅。元数据清理与单元测试结果不作为这两项线上检查的验收证据。
