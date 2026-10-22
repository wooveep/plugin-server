# Qwen/DashScope 默认 K3S 媒体验收与 Higress 更新分析

2026-10-22 在默认 `modelfusion-dev` VM K3S + Helm 环境完成原任务 6.2、6.3 的线上核验。图片生成、语音合成均经公开网关和真实 DashScope 返回 HTTP 200，未出现 `plugin_conversion_gap`。本次只提交验收文档；受测代码为 wasm-go `71bd516d25e92a9f24a46244be9d00a1fd67961b`。

结构化的环境、制品哈希、请求、脱敏响应、媒体检查和上游错误保存在[证据 JSON](2026-10-22-qwen-dashscope-k3s.json)。

## Higress 主仓中的相关更新

已获取 `higress` 的远端更新，当前 `main` 与 `origin/main` 均为 [703d3525](https://github.com/wooveep/higress/commit/703d35256ffbd5d06d9469610ce334be14bf9101)。该提交于 2026-10-22 合并 `source`，保留 AIGateway 自定义改动。相对其第一父提交，`plugins/wasm-go/extensions` 与 `plugins/wasm-go/pkg` 共变更 515 个文件，涉及 56 个插件目录。

与当前独立插件仓相关的重点提交如下，均已包含在 Higress 当前 main 历史中：

| 日期 | 提交 | 更新与独立 wasm-go 的差异 |
| --- | --- | --- |
| 2026-10-17 | [50f6ef68](https://github.com/wooveep/higress/commit/50f6ef6822eebf2bfa0160124bc5084608e8e5ee) | ai-proxy、ai-load-balancer、model-mapper、model-router 统一拒绝重复顶层 `model`。独立仓未引入此次新增的 `jsonbody` helper，适合作为后续安全回归重点。 |
| 2026-09-29 | [96441915](https://github.com/wooveep/higress/commit/964419155c4fee45b2e7d42ee1b7e1aeec57ce06) | ai-proxy 的 basePath 与 ext-auth 路径使用保留查询参数的拼接。独立仓 `ProviderConfig.DefaultTransformHeaders` 仍使用 `path.Join`，尚未带入此次 `pathutil` 改动。 |
| 2026-09-08 | [dc099932](https://github.com/wooveep/higress/commit/dc0999326b1a8df4269c13d0cf6ee2b725259f6e) | 在各 provider 的流式转换前补齐 SSE 事件分帧，新增 `sse_framing.go`。独立仓没有该文件，已有 `ExtractStreamingEvents` 等流式逻辑；两套实现的等价性需单独回归，不能据本次非流式媒体验收判定。 |
| 2026-08-09 | [d215a7bc](https://github.com/wooveep/higress/commit/d215a7bcceecf574ebe07aa991015815b97e0699) | 修复 Qwen、Gemini、Vertex embeddings 响应中的不安全类型断言。独立仓 Qwen `buildEmbeddingsResponse` 仍存在 `GetContext(...).(string)`；图片、语音响应使用的实现与此不同。 |

本次未移植这些提交。独立仓已有 Qwen/DashScope 原生图片和语音适配，以及自定义路由、缓存、记忆和账单契约；后续应按提交评审和回归，不能用 Higress 整个插件目录直接覆盖。此次检查回答了“是否存在相关更新”，不构成对上表更新的完整验收。

## 环境与制品

- 环境：`modelfusion-dev` VM，K3S `v1.35.5+k3s1`，Helm `v4.2.2`，命名空间 `higress-system`，Higress chart/app `2.2.1`。
- 管理台、UserWeb、公开网关分别使用 `admin.ai.lc.wooveep.com`、`user.ai.lc.wooveep.com`、`api.ai.lc.wooveep.com`；业务调用未使用 API 端口转发。
- 通过管理台创建专用媒体提供商、模型资产与绑定、测试租户/用户、两条开启 Key Auth 的路由及租户授权；通过 UserWeb 完成临时密码登录、改密和创建 API Key。没有使用 SQL 或仅后端 API 造业务数据。
- 提供商类型为 `qwen`，`qwenEnableCompatible=false`，图片和语音 capabilities 均明确指向 `/api/v1/services/aigc/multimodal-generation/generation`。
- Go `1.26.3`，目标 `wasip1/wasm`，构建参数 `-buildmode=c-shared -trimpath`。共享 SDK 为干净提交 `662ed045bf0b58eb0ab67ba9954ae2fa65343072`；私有 Go workspace 仅修正本地依赖路径，未修改受测源码。

| 插件 | 版本 | 本地构建、HTTP 回读和 Envoy 已加载文件的 SHA-256 |
| --- | --- | --- |
| ai-proxy | 1.0.2 | `c7c448592236c1f6b14449145d68a12987ffcce22c3cdd9e072ba861d506f0fd` |
| ai-statistics | 1.0.2 | `dfc39edb1d10c4b1f5274431a2be8d30c2f021adc799590d44296428d21782a2` |

制品经现有 HTTP 插件服务器上传。首次启用路由时，自动启用的 ai-statistics 1.0.2 尚无制品，返回 404 并导致网关不能就绪。补齐同一 main 构建的制品并重启 gateway 后恢复；验收时所有 Pod 容器 Ready，gateway 可用副本为 1。此项为部署准备问题，没有计作媒体成功或 provider 错误。

## 实际调用结果

成功请求发生在北京时间 2026-10-22 14:27。所有媒体 URL 在公开证据中整体替换为 `[REDACTED_TEMPORARY_MEDIA_URL]`。

| 检查 | 输入与返回 | 结论 |
| --- | --- | --- |
| 图片生成 | `POST /v1/images/generations`；模型 `qwen-image-2.0-pro`；`n=1`、`seed=42`、`size=1280x720`；HTTP 200，JSON `created` 与 `data[0].url` 有效。下载 HTTP 200，PNG 1280×720、1,182,596 字节，逐块 CRC 检查通过。 | 原生图片转换通过。 |
| 语音合成 | `POST /v1/audio/speech`；模型 `qwen3-tts-flash`；音色 `Cherry`、语言 `English`；HTTP 200，JSON `created`、`data.url/id/expires_at` 有效。下载 HTTP 200，WAV PCM16，单声道 24 kHz，实际 90,240 帧、3.760 秒，PCM 非零且按帧对齐。 | 原生语音转换及实际 PCM 数据检查通过，容器声明长度限制见下文。 |
| 无凭据调用 | 同一语音路径不带 API Key，HTTP 401，`Request denied by Key Auth check`。 | 路由认证生效。 |
| 非法音色 | `voice=invalid-voice-k3s-verification`，HTTP 400，DashScope `code=InvalidParameter`，提示音色不存在或未获许可。 | 预期上游参数错误，原始错误语义保留，不是 `plugin_conversion_gap`。 |

图片 SHA-256：`3a2be137406d1684fc84c9e9a7afbb77e81e364c526309271888a87c8aa86cda`。

原始音频 SHA-256：`b083914dde1cc739ae411d8bd55892978eedc5c2071543611625cf447a79b8d0`。哈希针对上游原始文件，未修改文件头。

## 上游行为与验收边界

语音 provider 返回的签名 URL 使用 HTTP。本次将同一主机、路径和签名参数的协议改为 HTTPS 后获取文件，成功返回 200；插件本身仍保留 provider 给出的 URL，未宣称插件已经强制 HTTPS。

上游 WAV 的 RIFF/data 声明长度分别为 2,147,483,591（含 RIFF 头）和 2,147,483,547 字节，实际文件仅 180,524 字节，其中 PCM 为 180,480 字节。验收按实际 PCM 帧、采样率和非零波形计算时长，未把头部声明的 44,739 秒作为有效时长，也未将该文件声称为长度字段完全规范的 WAV。此现象发生于 provider 提供的媒体文件；ai-proxy 转换的是带 URL 的 JSON，没有改写音频字节。

两项正常请求没有遗留的 provider HTTP 错误；证据中另保存一次受控非法音色错误，以核对错误保留和脱敏。语音返回值符合当前插件文档的 JSON URL 契约；此次不覆盖 OpenAI 二进制音频响应、异步图片任务、其他区域/模型或主观音质评估。响应中的 token 数量为零不用于推断费用或验证计费。

## 附加发现与收尾

**K3S-20261022-credential-revocation（P1，未修复）**：类别为认证/凭据生命周期，影响 UserWeb 令牌管理至 Console、Key Auth 的运行时链路。通过 UserWeb 吊销本次新建的测试令牌后，管理接口返回 200，列表显示令牌停用。随后携带该令牌向已授权语音路由发送非法音色请求，立即及延后复查均返回 DashScope `InvalidParameter` 400；预期应在网关返回 401/403。延后复查耗时 0.576 秒，证明请求仍到达上游。首次复查耗时 120.211 秒，不作为正常媒体响应延迟。

影响：不能依赖本次观察到的令牌停用状态确认运行时已拒绝旧凭据。根因尚未定位；不能仅凭结果归因于 ai-proxy。复现请求、状态码、请求 ID 和收尾记录在证据 JSON 的 `additional_findings`、`cleanup` 中。该问题阻断凭据吊销及整套环境安全验收，不改变先前两项媒体转换任务的实测结果。建议在平台仓按“创建令牌→调用成功→吊销→轮询投影完成→旧令牌必须被拒绝”补充回归并定位 Console/Consumer/Key Auth 链路。

**K3S-20261022-route-disable-result（P2，未修复）**：为隔离上述问题，经管理台编辑并关闭两条测试路由。停用操作返回 `validation_error: AI Route not found`（400），但对应 Ingress 均被移除，测试路径最终返回 404。预期停用成功后返回成功状态；当前返回值与实际资源变更不一致，需在平台仓核对停用后的读取/返回逻辑。此项已通过集群与公开路径复核收尾效果，没有把 400 误记为 UI 操作通过。

测试令牌保持停用；两条测试 Ingress 已移除。提供商、模型资产与绑定、专用租户/用户保留供后续复核。本次没有修改平台代码，也未将上述两个附加问题标记为已修复。

## 验证与复核

现有 Qwen 媒体单元测试 6 项全部通过，涵盖两种媒体的请求头/请求体转换、成功响应转换、provider 错误保留：

```bash
go test ./provider -run 'TestQwenProvider.*(ImageGeneration|AudioSpeech)' -count=1 -v
```

命令在 `extensions/ai-proxy` 执行，并使用上述仅修正依赖路径的私有 workspace。线上检查通过后再更新[文档索引](../README.md)中的两项状态。提交前另检查 JSON、相对链接、敏感值和暂存改动范围；GitNexus 使用 `--index-only` 刷新并运行 `detect-changes --scope staged`。

复核前先处理上述令牌生命周期缺陷，或明确隔离测试路由；测试期间在 UserWeb 创建新令牌，在忽略目录保存原始响应，避免将 API Key 或临时签名 URL 放入文档、命令日志或 Git。本文与 JSON 均只保留必要的状态码、请求 ID、配置键、媒体元数据及哈希，未收录登录态、凭据、原始媒体 URL 或含凭据的原始业务配置。
