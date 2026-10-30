# 原生媒体插件契约核对与交付

日期：2026-10-31。状态：源码核对完成，修复与新包验收进行中；未据此宣称真实网关验收完成。

## 契约判断

1. `usage_estimation` 属于 Console 保存的原始媒体执行策略。`FrozenMediaInput`、`FrozenMediaIntent` 已持有它，并在构造时按媒体 profile 校验；`ai-proxy.managedSettings` 没有使用它生成真实请求或真实 usage。MF 源生成器应保留 Console frozen binding 中的策略，生成 `managed_execution` 时不下发这个字段。插件仍拒绝未知字段，不通过增加未消费字段或放宽解码来消除初始化错误。
2. 当前原生媒体执行明确采用 `http_call + ai_proxy` 的上游事实 owner。`ai-proxy` 的媒体 profile 校验要求实际 `image_generate` 或 `video_generate`，异步 profile 另有 `task_query`；不应加入虚构的 `text_generate`。
3. `ai-billing` 目前把 embedding 以外的能力都当作 `text_generate`，因此拒绝真实媒体绑定。修复应让它识别真实媒体能力并验证该冻结配置，但媒体路径不新增第二条旧式 BillingEvent、文本 delivery bridge、估算 usage 或资金扣费路径。实际媒体上游事实继续由 ai-proxy 记录，Console 负责既有媒体事实接受和结算；文本与内部 embedding 的原有 delivery 语义保持。
4. 验收需要实际执行插件的 `OnPluginStart`，并核对共享 listener 上的管理请求保持通行。仅有 WasmPlugin 名称、过滤器条目或 defaultConfigDisable 标记不足以证明初始化成功。

## 源码依据

- Console：`console/src/media_operations/types.rs` 的 `FrozenMediaInput.usage_estimation`、`FrozenMediaIntent.usage_estimation` 和构造时的 `validate_profile`。
- ai-proxy：`managed_execution.go` 的 `managedSettings` / `configureManagedExecution`；`media_execution.go` 的 `validateManagedMediaRelease`；`pkg/ai/mediaexecution/facts.go` 的 `http_call` / `ai_proxy` 事实归属。
- ai-billing：`native_execution.go` 的 `nativeBillingOperation`、`validateNativeBillingRelease`、`validateNativeBillingDelivery`；后者现有 delivery bridge 专用于文本和内部 embedding，不能把媒体事件装入文本通道。

## 责任与交付安排

root 修改 `plugin-server/plugins/wasm-go` 的插件、测试和包，沿用完整包来源校验与不可变版本。另一聊天负责 MF 源生成器、SDK、Web、真实 VM 加载确认和目标路由恢复；root 不修改这些文件、不升级 Helm。

本次拟交付 ai-proxy 2.0.9、ai-billing 1.0.1，均高于现用的 2.0.8 / 0.1.13。新版本只在完成源码固定、Go/编译后 Wasm 回归及完整包入库后登记。包摘要、artifact_id、真实 HTTP 地址和上传/注册结果待实际完成后填入。

线上只读核对另发现 ai-billing 已登记 1.0.0；本次采用 1.0.1，保证显式保存并生效时按真实最高 SemVer 选择到新包。原 0.1.14 只是未发布的构建阶段版本，其回归记录保留，不作为线上版本声明。
