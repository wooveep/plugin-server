# ai-proxy 对下游的 OpenAI 协议支持核对

2026-11-04（Asia/Shanghai）。性质：**源码、离线及 AI-proxy 2.0.31 在线验证**。根据用户最新指示，已独立验证开发003已部署的插件，不等待其尚在实施的C批工具和页面。官方协议基准见[工作区 OpenAI API 协议参考](../../../docs/openai-api-protocol.md)。

本次回答的是下游客户端实际使用什么协议、同协议转发保留什么、协议转换覆盖什么。供应商或模型的固有限制不记为项目实现缺陷；尚未覆盖的适配范围、已复现的语义改变和真实运行结果分别记录。价格仅作为后续真实调用的核算参考。

**修复接续已完成：** 第1～7节保留修复前的2.0.31检查点；字段保真、缓存选项/断点与非流式消息ID已修复，并通过[最终候选的隔离在线回归](2026-11-04-responses-chat-fidelity.md)。该结果不表示正式业务路由已切换，详见第8节。

## 1. 核对版本与证据口径

| 对象 | 本轮读取的快照 | 能说明的范围 |
| --- | --- | --- |
| 主目录 `plugin-server/` | 分支 `codex/model-tools-protocols-20261103`，提交 `4695ad612d9d9aa95568be84939e2cd745ebb013`，ai-proxy **2.0.26** | 主目录当时源码，不当作最新部署版本 |
| 原生协议工作树 | `.local/worktrees/model-tools-native-plugin-20261103`，提交 `d7153c12`，ai-proxy **2.0.31** | 本轮离线测试的源码；开发003仍在继续，不能据此宣布整个 B/C 批完成 |
| 首次离线阶段读取的部署验收 | [Helm63 / ai-proxy 2.0.30 / `812a0849`](../../../modelfusion-deploy/docs/checks/2026-11-03-embedding-edge-gateway-acceptance.md) | 历史向量及普通文本业务证据 |
| 本次在线实际版本 | Helm68，ai-proxy **2.0.31**，来源 `d7153c12` | 两条独立转换路由及原生 DeepSeek 路由的实际 ECDS/缓存 Wasm 已核对；详见第7节 |

离线结果见[初次核对及追加探测回执](2026-11-04-openai-api-capabilities.json)，在线结果见[独立在线机器回执](2026-11-04-openai-downstream-live.json)。本轮未修改业务源码、镜像、原有路由或价册；在线阶段创建并清理两条独立验证路由，另通过产品钱包命令补充0.02元本地测试额度，具体记录见第7节。未据此宣称开发003的C批整体完成。

## 2. 同协议转发与协议转换怎样区分

入口在 [main.go](../../plugins/wasm-go/extensions/ai-proxy/main.go) 的 `onHttpRequestHeader` 中先选择托管分支，再选择兼容分支。因而两种执行方式的能力不能直接合并。

| 分支 | 下游观察到的行为 | 边界 |
| --- | --- | --- |
| 兼容代理的同协议路径 | OpenAI provider 改写上游主机、凭据及操作路径；正文主要保留原 JSON并映射 model；原生响应通常直接交付 | 某些配置会额外改写正文，如 `responseJsonSchema`、上下文、custom settings；Chat 的 developer 角色在兼容处理链中可能被改成 system，不能笼统称字节透传 |
| `protocol_mode: passthrough` 或未分类操作的透明路径 | 保留方法、路径/查询、正文及非凭据头，应用配置中的 basePath、主机与上游认证策略 | 是传输能力；不增加该操作的标准化转换、模型适配或平台资源管理 |
| 托管同协议路径 | 按发布时冻结的 profile、模型、上游路径和凭据发送；原生响应主体保留，解析原始响应产生执行与用量事实 | Chat SSE 会补 `stream_options.include_usage=true`；客户向量缺用量时可写入同一估算 usage。不是严格的原始字节代理 |
| 托管跨协议路径 | 入站正文先转换为逻辑上游协议，再经过厂商适配器；响应转换回入站协议 | 仅支持已登记协议对；一条响应是 SSE 不等于实时逐块交付 |

依据：[透明传输策略](../../plugins/wasm-go/extensions/ai-proxy/provider/transparent.go)、[OpenAI provider](../../plugins/wasm-go/extensions/ai-proxy/provider/openai.go)、[公共兼容处理](../../plugins/wasm-go/extensions/ai-proxy/provider/provider.go)、[托管执行](../../plugins/wasm-go/extensions/ai-proxy/managed_execution.go)。

`openaiCustomUrl` 若填单个完整操作地址，会启用固定操作路径覆盖。要承接多种 OpenAI 操作，应核对配置到底表示 Base URL 还是固定端点，不能只根据 provider 名称判断最终出口。

## 3. 下游文本协议矩阵

下表的“支持”限定于已选模型、厂商及发布配置支持该操作；同协议仍可能被厂商拒绝不支持的模型参数。

| 下游协议 → 逻辑上游协议 | 当前处理 | 覆盖与限制 |
| --- | --- | --- |
| Chat Completions → Chat Completions | 同协议支持 | messages、原生多模态内容、tools、response_format 等可随原正文转发；具体厂商转换器可能只保留自己的字段子集 |
| Responses → Responses | 同协议支持 | POST 创建可以保留 input items、工具、结构化输出等原生请求和原生响应；模型/工具可用性仍由对应上游决定 |
| Anthropic Messages → Messages | 同协议支持 | 已有独立 profile；用于区分真正原生出口和经 Chat 转换的结果 |
| Responses → Chat Completions | **部分转换支持** | 文本输入、instructions、部分采样参数、普通函数声明、文本/函数调用输出及相应 SSE；详见下一节 |
| Anthropic Messages → Chat Completions | 转换支持，覆盖依转换器 | 经已有 `ClaudeToOpenAIConverter` 转换并还原 Messages 响应；不能把 Anthropic 专有工具/思考等所有特性都视为等价 |
| Chat → Responses、Chat → Messages、Responses ↔ Messages | 托管 profile 未登记 | 目前配置校验拒绝这些逻辑协议对；兼容分支中某个厂商转换器的存在，不能证明托管路径也支持 |
| DashScope 原生 → 同一原生协议 | 2.0.30 已有；2.0.31 保留 | 原生文本或图文输入、文本输出；不等于 OpenAI 的 Responses→Chat 转换。A 批已有原生看图验收 |
| Chat → Qwen/Gemini 等厂商物理原生协议 | 部分厂商适配支持 | 这是逻辑 Chat 请求后的第二层厂商转换；每家支持的内容类型和参数分别受其转换器限制 |

托管登记依据：[newManagedProtocolProfile / PrepareRequest](../../plugins/wasm-go/extensions/ai-proxy/managed_protocol.go)、[厂商适配](../../plugins/wasm-go/extensions/ai-proxy/provider/managed_adapter.go)。2.0.31 增加独立的 Qwen Messages 与 DeepSeek Responses 路径；其开发、部署及真实协议结果以开发003最终回执为准。

### Responses→Chat 的具体覆盖

| 下游请求或输出 | 实际处理 | 分类 |
| --- | --- | --- |
| input 字符串、消息数组中的文本 | 转 messages；input_text/output_text 内容合并为消息文本 | 已实现子集 |
| instructions | 转 developer 消息；兼容链可能继续转 system | 已实现，角色语义有适配差异 |
| max_output_tokens | 转 max_completion_tokens | 已实现 |
| function 工具、tool_choice | 转 Chat 的 function 嵌套结构；支持 auto/none/required/指定函数 | 已实现声明与输出，不代表完整函数往返 |
| Chat 文本与 tool_calls 输出 | 转 Responses message/function_call items；保留函数参数分片与输出次序 | 已有测试及旧官方 SDK 消费证据 |
| previous_response_id / conversation | 显式拒绝 | Chat 没有对应服务端 Responses 状态；在当前无状态回退范围内是协议边界，不列缺陷 |
| web_search、file_search、code_interpreter 等 Responses 内置工具 | 显式拒绝 | 通用 Chat 不提供等价的 Responses 托管工具执行协议；不列缺陷 |
| input_image / input_file 等非文本块 | 当前转换器拒绝 | 当前适配范围未覆盖；不能把图片说成 Chat 协议本身不能表达。应保留原生 Responses 路径使用视觉 |
| function_call_output 输入条目 | 本轮探测被拒绝 | 当前未完成工具结果回传映射；Chat 可用 role=tool 表达，属于适配器覆盖边界，不是两协议天然不可能转换 |
| text.format、reasoning、background、store、parallel_tool_calls | 本轮探测转换结果中消失 | 已复现的字段保真差异；其中状态/后台语义还涉及协议能力差异，不能擅自假装已等价执行 |
| function.strict | 本轮探测转换后消失 | strict 同时存在于两种协议；属于需要明确拒绝或补映射的实现差异 |
| 显式 temperature=0 | 本轮探测转换后消失 | 转换目标零值被 omitempty 省略；属于已复现的实现差异 |
| 非流式 output message 的 id | 本次网关输出缺少该字段；用真实、含created的合法Chat响应直接转换也复现 | 返回对象完整度的实现差异；同次流式 message 则有id。不是协议无法表达 |

依据：[responses.go](../../plugins/wasm-go/extensions/ai-proxy/provider/responses.go)、[转换器测试](../../plugins/wasm-go/extensions/ai-proxy/provider/responses_converter_test.go)、[函数流测试](../../plugins/wasm-go/extensions/ai-proxy/provider/responses_functions_test.go)、[本轮探测输入与输出](2026-11-04-openai-api-capabilities.json)。此处记录差异，不自动创建缺陷工单，也未修改既有开发范围。

### 流式语义

兼容分支有 `onStreamingResponseBody` 和增量转换器；Responses 回退已覆盖文本、函数参数及 completed/incomplete 生命周期。已有[官方 SDK 消费回执](2026-11-02-responses-sdk-consumer-source.json)只证明所列三份离线 SSE 能被 SDK 消费。

**托管分支使用单次 HTTP callout 完整收齐响应，再检查终态和转换，最后交付整个 SSE。** 下游可收到正确 SSE 格式，但不会随上游生成即时收到首块。它是当前传输实现的边界，不能将“格式可解析”记作“实时流式等价”。依据：`dispatchManagedPreparedRequest`、`managedProtocolEvents`、`ConvertResponse`、`sendManagedFinal`。

## 4. 语音、视觉、向量及其他 Platform APIs

| 下游接口/能力 | 同协议转发情况 | 转换/托管情况 |
| --- | --- | --- |
| Chat/Responses 图像输入、文本输出 | 原生 JSON 可转发，具体模型需支持视觉 | Responses→Chat 当前拒绝图片块；原生 DashScope 看图已有[A 批证据](../../../modelfusion-deploy/docs/checks/2026-11-03-native-dashscope-text.md)，不能据此代替 OpenAI 视觉所有组合 |
| Chat 音频输入/输出 | 请求结构包含 input_audio、modalities、audio 等；原生 OpenAI 路径可保留 | 没有据此建立全厂商音频转换或音频专项计量验收。Responses 当前官方音频说明差异见协议参考，模型不支持时不记项目缺陷 |
| POST `/v1/audio/speech` | OpenAI provider 有路径声明，JSON 可处理和映射 model，结果可为原生媒体数据 | 部分厂商有特定转换；例如 Qwen 分支生成音频 URL 的 JSON，不能称为与 OpenAI 二进制语音响应完全等价。无独立托管 Speech profile |
| POST `/v1/audio/transcriptions`、`/translations` | 已有路径声明；标准 multipart 走正文保留，不做普通 JSON 转换 | 不能据此承诺 multipart 中 model 别名映射、全厂商 ASR 转换或独立托管音频操作 |
| POST `/v1/embeddings` | 支持文本、文本批量、Token ID及其批量；向量可为浮点数组或 base64，依供应商 | 有专用托管 profile，检查输入/向量索引及只计输入的 usage。single_send，不支持 stream=true/background=true；这符合该接口同步用途，不列协议缺陷 |
| POST `/v1/images/generations` | 已登记；原生 OpenAI JSON 可转发 | 有部分厂商图像转换；平台媒体执行仅接受静态注册的 Qwen/Wan profiles，不等于所有 OpenAI 图像模型 |
| POST `/v1/images/edits`、`/variations` | 已登记；有图片 multipart model 映射；原生 JSON编辑请求也应按官方新版形状区分 | 不存在通用“任何厂商都能改图/变体”承诺；Responses 的 image_generation 工具是另一条协议 |
| Realtime `/v1/realtime` | 已有路径类别，实际双向连接还依赖网关 Upgrade/上游能力 | 未见 Realtime 会话/事件转换及专用托管 profile；不能用 HTTP 路径命中证明 WebSocket/WebRTC 全链通过 |
| Files、Batches、Models、Fine-tuning、Videos | OpenAI provider 登记了若干集合及资源子路径，可进行主机/凭据/路径转发 | 是上游资源代理，不是本地实现这些资源；本轮未做真实文件、训练、视频或批量任务验证 |
| Responses GET/DELETE、cancel/input_items/compact/input_tokens，Chat 已存储结果接口 | 创建之外的许多操作不在独立操作表中，兼容分支可能落透明路径 | 托管文本入口只处理 POST JSON及冻结的上游创建路径，没有这些操作的完整资源生命周期 profile |
| Vector Stores、Uploads、Moderations、Evals、组织管理 API | 未独立登记相应完整协议集；绑定上游时可有透明转发基础 | 无对应统一转换/托管能力声明；Embedding 可用不等于 Vector Stores API 已实现 |

路径依据：[main.go 的操作表](../../plugins/wasm-go/extensions/ai-proxy/main.go)、[OpenAI 默认路径能力](../../plugins/wasm-go/extensions/ai-proxy/provider/openai.go)、[路径常量](../../plugins/wasm-go/extensions/ai-proxy/provider/provider.go)、[multipart 辅助](../../plugins/wasm-go/extensions/ai-proxy/provider/multipart_helper.go)、[向量 profile](../../plugins/wasm-go/extensions/ai-proxy/managed_embedding.go)、[媒体 profiles](../../plugins/wasm-go/pkg/ai/mediaexecution/profiles.go)。

## 5. 判定时排除哪些协议/模型限制

1. 模型不支持特定模态、采样参数、工具或输出格式，且代理正确保留上游拒绝结果：记录模型/供应商限制。
2. Responses 的服务端状态、后台执行与托管工具回退到普通无状态 Chat 时没有等价物：记录协议限制；明确拒绝是有效结果。
3. Embeddings 没有聊天输出、图片变体只适用于部分图像模型、Realtime 需要另一种连接协议：按各接口自己的契约验证。
4. 两边都能表达却未映射的能力：记录当前转换覆盖范围；只有与已承诺支持范围冲突或发生无提示语义改变时，才进一步判项目实现差异。
5. 实际上游路径错误、原生请求被改成另一种协议、文本或工具参数损坏、错误状态丢失等，有独立复现时才按代理实现问题判断。HTTP200、路径声明和文档声称支持均不能单独作成功证据。

兼容分支本地转换错误当前可能由 `util.ErrorHandler` 返回 HTTP500/plain text，托管拒绝使用 `managed_execution_error` JSON；与官方统一 error envelope 并非完全一致。该差异单列，不将厂商自己的合法错误格式误判为代理 bug。

## 6. 本轮离线验证

在上述 2.0.31 源码快照执行，Go 1.26.3；没有供应商或网关网络调用：

| 检查 | 结果 |
| --- | --- |
| `go test -json ./provider ./config ./util -count=1` | 182 顶层 / 720 含子用例通过项，0失败、0跳过 |
| 托管/内部执行与路径、内容类型定向 Go 测试，精确命令见机器回执 | 82 顶层 / 314 含子用例通过项，0失败、0跳过 |
| 调用导出的 `ConvertResponsesRequestToChatCompletion`，5组自写输入 | 确认上表字段丢失与函数结果输入拒绝；保留完整输出 |

合计 **264 顶层 / 1,034 含子用例通过项**。本轮没有重新构建/执行编译后 Wasm，也没有把离线测试记作实际网关、供应商或 SDK 全协议认证。

### 等待开发003期间补充的转换往返核对

2026-11-04 用户要求继续后，增加14组直接转换器探测，11组产生转换结果，3组明确拒绝；14项实际观察均与保留输出一致。完整输入、输出和断言写入[机器回执](2026-11-04-openai-api-capabilities.json)的 `followup_offline_conversion`，未发出网络或供应商请求。这不是额外14项“全部能力支持通过”：预期拒绝、静默遗漏与成功映射分别记录。

| 方向与样本 | 观察结果 |
| --- | --- |
| Messages→Chat，文本与 Base64/URL 图像 | 两种图像均转为 image_url 内容块；Base64数据与URL逐字保留 |
| Messages→Chat，tool_use + tool_result | 工具名、参数和 tool_call_id 保留；字符串结果可回传，文本块数组结果用换行合并 |
| Messages→Chat，any + disable_parallel_tool_use | 转为 required 与 parallel_tool_calls=false |
| Responses→Chat，max_output_tokens + 非零 temperature | 分别映射为 max_completion_tokens，并保留采样值 |
| Responses→Chat，图像、previous_response_id、web_search | 三项明确拒绝；图像是当前转换范围未覆盖，后两项按无状态 Chat 回退的协议边界处理 |
| Responses→Chat，新缓存配置 | prompt_cache_options 和 prompt_cache_key 从转换结果中消失；记录为请求字段保真差异，不据此宣称缓存未执行或费用错误 |
| Chat→Responses，函数调用输出 | call_id、工具名、JSON参数及输入/输出用量保留 |
| Chat→Responses，length、refusal、error | 分别映射 incomplete/max_output_tokens、refusal内容块和failed/error对象 |

`responses.go`、`claude_to_openai.go`、`model.go`经哈希核对仍与 `d7153c12`相同。工作树其他文件正在进行C批修改，本次没有改动它们。裸转换器可能携带供后续处理的内部字段；以上不能替代完整出站处理链、真实HTTP状态和厂商接纳验证，最终下游可用性仍待在线样本。

## 7. 在线验证结果：下游同协议与转换

2026-11-04，实际 **Helm68 / ai-proxy 2.0.31**。使用指定 `.env.local` 的管理员登录及现有测试身份；复用原资产、绑定和凭据，新建两条隔离转换路由，避免把仍固定在2.0.25的旧路由结果当作2.0.31结果。

实际加载的Wasm SHA256为 `5c7b5d626f11c16b167aab61c4b6e4557502b289b0e6f53e9f74304adc7d4f69`，与 `d7153c12` 交付件一致。两条转换路由都经真实入口、授权、mapper、quota、ai-proxy及billing链路；受控收件器保留实际HTTP原件。原生样本使用真实DeepSeek，视觉另复用已有千问Responses成功回执。

### 7.1 本次新网关样本

| 下游 → 上游 | 样本与结果 | 能说明什么 |
| --- | --- | --- |
| Chat → Chat，DeepSeek | 函数调用、回传工具结果、普通SSE，共3次HTTP200 | 工具名、参数、tool_call_id可完成真实两轮往返；最终回答包含测试函数返回的状态；SSE有usage及DONE |
| Responses → Responses，DeepSeek | 函数调用及function_call_output回传，共2次HTTP200 | call_id、参数和结果关联正确；两轮均completed，最终回答包含返回状态；按供方无状态API传完整历史 |
| Responses → Chat，受控真实网关收件 | JSON、SSE及字段保真探测，共3次HTTP200 | input/max_output_tokens正确转换，输出可解析；字段遗漏详见下一节，不记作全字段兼容 |
| Messages → Chat，受控真实网关收件 | 含tool_use/tool_result的历史JSON、普通SSE，共2次HTTP200 | 实际出口保留关联ID与结果文本；Messages响应及message_stop可解析。受控文本回复不作为真实模型选择工具的证据 |
| Responses → Chat，不支持项 | previous_response_id、web_search、input_image、function_call_output，共4次HTTP400 | 明确的发送前拒绝；前两项按协议边界解释，后两项按当前转换器覆盖范围解释 |

共 **14个功能样本：10个HTTP200、4个明确HTTP400**。10个成功调用分别对应一次尝试及交付，20条事实accepted、20笔账务应用。5个真实DeepSeek请求的实际出站摘要与下游原请求逐字节摘要一致，包括工具声明及结果；另5个受控请求的独立收件摘要与网关尝试事实一致。没有用HTTP200或回答自称模型名替代协议/出站证据。

流式转换的Responses事件依次包含created、item/content创建、文本增量/完成、item完成和completed；sequence_number为0至7。Messages流包含message_start、content_block三阶段、message_delta及message_stop。**托管传输仍是聚合后交付SSE，本次没有宣称实时逐token转发通过。**

### 7.2 已复现的实现差异

| 差异 | 独立证据与影响 |
| --- | --- |
| Responses结构化输出未映射 | 下游发送text.format.json_schema，实际Chat收件没有response_format，返回普通文本；不能将此解释为Chat不支持结构化输出 |
| 函数strict、并行约束及零采样值丢失 | 下游tools[].strict=true、parallel_tool_calls=false、temperature=0，实际Chat收件均没有对应字段。普通函数声明及tool_choice=none仍在，证明不是整份工具请求未到达 |
| 非流式Responses message缺id | 网关JSON输出的message item只有type/status/role/content；SSE输出则有id。另用本次真实、含created的完整Chat响应直接转换，仍缺message id，排除受控上游省略created造成的混淆。官方[Responses输出类型](https://developers.openai.com/api/reference/resources/responses/methods/create)包含message id |
| Responses函数结果输入未覆盖 | function_call_output返回400；Chat原生已实测可通过role=tool/tool_call_id完成同类往返，因此是当前适配范围未覆盖，不是两种协议天然无法转换 |

以上是支持矩阵与实现差异，不把“参数被静默丢弃”计作成功支持。未修改转换代码或为本次结果自动扩大开发003的修复范围。

Messages→Chat的实际出口还保留claude_content_blocks及tool_calls[].index等扩展字段。为判断真实影响，使用`.env.local`中的OpenAI凭据直连gpt-6-luna，分别发送保留扩展字段与清理后的请求；**两者均HTTP200**。因此本次不把这些字段列为已证实的上游格式拒绝故障，也不将两次生成文本的差别认定为确定的代理故障。该直连诊断不计入网关样本。

### 7.3 协议、模型与运行条件单列

- Responses的服务端续接状态与托管web_search在当前无状态Chat回退中没有等价执行环境；明确400是有效边界结果，不算项目实现缺陷。
- 图像块当前未被Responses→Chat转换器映射；这不表示Chat协议不支持图像。千问同协议Responses视觉已复用真实成功响应：completed，回答为“图片里有一个女人和一只狗”。不能据此推及所有模型/Chat视觉组合。
- 复用开发003的DeepSeek三协议、恢复后的千问Messages与上述视觉回执，共5份，减少重复供应商调用。此前千问URL错误及权限错误保留为供应商/输入条件，不能仅凭它们判代理缺陷。
- 初次两笔Chat成功后，测试钱包余额不足，后续11次请求在准入处被HTTP403拒绝、没有供应商发送。这11份原件单独保留，不纳入协议可用性判断；经原生钱包命令划拨0.02元测试额度并确认同步后，以新用例名补验。未改写原账务或绕过余额门槛。
- 独立Audio、图像生成/编辑、Realtime/WebSocket/WebRTC、Vector Stores及完整平台资源生命周期本批未在线覆盖；维持第4节的源码/配置边界，不据缺少本批样本认定项目缺陷。

### 7.4 清理与证据

两条临时验证路由已通过产品API删除并回读404。业务删除后仍留有本轮专属运行对象，已按两份唯一release UUID前缀清理2个Ingress及10个专属WasmPlugin，核对剩余0；未删除原有共享资源、资产、绑定、价册或历史事实。Helm维持68，没有重部署镜像。原等待开发003整体完成的检查已不存在，本次分析以已部署插件独立完成。

公开[在线回执](2026-11-04-openai-downstream-live.json)包含每案协议、call/attempt/release ID、原请求/实际收件摘要、状态、用量、流事件、差异及清理结果。密钥、JWT、原HTTP头、完整账本及登录响应仅保留在本地忽略目录 `modelfusion-deploy/.local/openai-downstream-20261104/`，未进入公开回执。

## 8. 已确认实现差异的修复与回归

[修复计划](../plans/2026-11-04-responses-chat-fidelity.md)的RC-01～05已完成：text.format、strict、parallel_tool_calls、显式零值、reasoning.effort、store及共享缓存键/选项保真；消息/文本块缓存断点不再被合并丢失；非流式message补齐上游响应派生ID。后台执行与推理摘要改为明确拒绝。

最终候选为`a61ef533` / `2.0.31-responses.2`，232顶层、897含子用例检查通过。最终候选在线17个样本：8成功、8本地明确拒绝、1供应商参数拒绝；另复用先前修复版本3个真实成功回执。独立收件器与attempt摘要匹配，两个路由实际加载候选Wasm SHA一致；临时资源已清理，正式业务路由未切换。详见[权威验收](2026-11-04-responses-chat-fidelity.md)、[脱敏回执](2026-11-04-responses-chat-fidelity.json)及[仅含转换器与回归的补丁](2026-11-04-responses-chat-fidelity.patch)。

Chat本身也支持prompt_cache_options；本轮实际OpenAI服务拒绝内容块缓存断点的TTL字段，继承请求级TTL后通过。该供应商实际接口/文档差异单列，不算代理缺陷。图像/完整函数历史仍属于适配覆盖缺口；状态续接、托管工具与托管SSE聚合交付边界继续保留。
