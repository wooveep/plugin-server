# 原生媒体插件契约核对与交付

日期：2026-10-31。状态：媒体契约修复、Go/编译后 Wasm 回归、原持久化制品库入库及 HTTP 下载核对已完成；MF 注册、真实网关初始化和目标路由验收仍由 MF/DEP 责任聊天执行。

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

本批已交付 ai-proxy 2.0.9、ai-billing 1.0.1。源码、回归、制品 ID、包摘要和 HTTP 核对详见下方实际交付记录。MF 侧注册和显式保存并生效由 MF/DEP 责任聊天继续完成。

线上只读核对另发现 ai-billing 已登记 1.0.0；本次采用 1.0.1，保证显式保存并生效时按真实最高 SemVer 选择到新包。原 0.1.14 只是未发布的构建阶段版本，其回归记录保留，不作为线上版本声明。

## 已完成的交付与接入

[机器可读验收回执](2026-10-31-native-media-plugin-delivery.json)包含源码提交、Go/Wasm 回归、完整入库回执和实际 HTTP 校验。源码已推送至 `codex/artifact-p1-20261026`；媒体修复为 `4cc3e509`，最高 SemVer 发版修正为 `c29670ad`。

| 插件 | 版本 | artifact_id | manifest（集群内） |
| --- | --- | --- | --- |
| ai-proxy | 2.0.9 | `41d50f84-e66b-4d2f-932f-12f8279bf9c2` | [manifest.json](http://higress-plugin-server.higress-system.svc:80/plugins/ai-proxy/2.0.9/manifest.json) |
| ai-billing | 1.0.1 | `78c9e3e1-03d4-4f2d-9e23-4fb10d0fe148` | [manifest.json](http://higress-plugin-server.higress-system.svc:80/plugins/ai-billing/1.0.1/manifest.json) |

两个新包通过现有 `ArtifactStore.commit` 入库，使用单独提供的来源/Wasm/完整内容审批三元组；没有改变服务原策略文件、镜像、Helm、现有生效版本或既有制品。此次采用原服务中的 operator CLI 调用既有入库方法，非 HTTP PUT。当前 HTTP 服务可读取两个包的 manifest、Wasm、配置 schema 和 archive，8 项请求均为 200，大小与摘要相符。

当前线上旧镜像的 `GET /artifacts` 返回 404；本次没有把这个缺口记成通过。两个具体 manifest 地址已可用于现有注册入口；MF 责任聊天继续登记/缓存它们的元数据，再由显式保存并生效选择最高版本。批量元数据接口已在 plugin-server 源码实现，随后续完整集合镜像交付接入。

ai-billing 媒体配置会校验实际图片/视频能力并保持 dormant，不创建文本 delivery 或旧 BillingEvent；原生媒体事实仍由 ai-proxy 的工作器产生。ai-proxy 保持闭合的运行配置，错误下发 `usage_estimation` 会被真实 VM 初始化负例拒绝。基础配置、同步图片、异步图片和视频的 scoped 初始化已运行 Go 与实际编译后 Wasm 回归；这些结果不替代真实 Higress 加载或供应商媒体调用验收。

## 真实 Qwen 提供方的后继修复（进行中）

MF/DEP 聊天在真实发布 `01a18e88-fa15-7610-b3be-fd5360e8111e` 中记录 2026-10-30 20:41:37.288Z 的 Envoy 初始化失败：`managed media route requires its explicit controlled provider`。其只读核对确认冻结 `providers[].type=qwen`，`activeProviderId` 与 provider id 一致；MF 没有丢失提供方。该坏候选已由对方标记 Failed 并清理，管理入口恢复 200。

本地已用完整 v2 冻结配置的 qwen 同步图片案例复现相同失败。源码根因是媒体分支在 `Config.Complete` 已经验证真实业务提供方后，错误地再次要求文本适配器家族必须为 generic/openai。媒体 codec 和发送者实际由冻结 profile 与 notice worker 决定，该限制混淆了业务提供方和原生媒体协议层。

后继 ai-proxy 2.0.10 移除这项错误的家族限制，保留精确 profile、active provider id、冻结凭证池和 token 对应检查；未知提供方仍由既有创建边界拒绝。没有把 qwen 改成 generic，没有加入虚构文本能力或放宽未知运行字段。正在执行真实 `type=qwen` 的同步图片、异步图片、video/task_query 初始化以及 ID/凭证负例；2.0.9 的原交付记录作为历史保留，新包入库证据完成后另行追加。
