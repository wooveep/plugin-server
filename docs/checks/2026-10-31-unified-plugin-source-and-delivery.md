# 统一插件源码、编译与完整交付验收

日期：2026-10-31（Asia/Shanghai）。插件源码、固定工具链编译、完整包入库、镜像/PVC/HTTP 和真实 model-router 匹配已完成；**后继Helm101已补齐55类目录、真实默认版本、配置与文档，本整合范围完成**。实施范围见[计划](../plans/2026-10-31-unified-plugin-source-and-delivery.md)，[总机器回执](unified-plugin-source-20261031/receipt.json)记录精确提交、版本、制品身份、摘要、工具链和分层回归。

## 源码与发行集合

`plugin-server/plugins/` 已成为唯一日常维护源码树。完整 Higress 插件树保留原始导入和 Git 跟踪基线；定制 Go 框架/插件及 Proxy-Wasm SDK 的历史通过真实合并父链保留。后续沿上游快照父链，使用 `ort -Xsubtree=plugins` 三方合并；隔离 Git 夹具已核对定制和无关变更均能保留。`higress/` 未被本批修改。

权威[来源锁定](../../plugins/upstream-lock.json)、[定制整合与版本裁决](../../plugins/release/custom-integration.json)、[源码清单](../../plugins/release/catalog.json)保留 72 项实现分类，选出 **55 个正式 Wasm 插件：46 Go、8 C++、1 Rust**。按实际 `VERSION` 的 SemVer 选择实现；无版本示例、原生 `.so` 与废弃实现不冒充正式版本。model-router 默认是 Go **2.0.3**；C++ 八项采用新发行 **2.0.1**，高于已核实的公开 2.0.0 命名空间，没有把旧字节伪装为新版本。

旧独立 Go/SDK 的 Make 入口已退出日常构建，只有显式 `ALLOW_ARCHIVED_SOURCE=1` 才能历史重现。归档提交为 Go `1202acf`、SDK `dc7491f`，各自已推送；原有未提交资料保持。根目录 [README](../../README.md)提供单插件、全量、完整包及镜像入口。

## 分层验证

- [49 个实际 Go 模块解析记录](unified-plugin-source-20261031/go-module-preflight.json)已在同一固定容器中按 `GOWORK=off go list -mod=readonly -m -json all` 冻结；记录 1,026 个去重依赖描述，旧绝对 replace 为零。覆盖 46 个正式 Go 插件、共享框架/SDK 和原生模块。
- 55/55 正式插件在固定本地容器工具链下实际编译成功。两个 AssemblyScript 示例和原生 Go `.so` 另行显式编译成功，保持非正式分类；没有将 `.so` 作为 Wasm HTTP 制品。
- Python 全套 **114/114**、编译检查及同步/保留入口通过。既有 D0 `.3` 语料与 golden 未被改写。
- Go 框架、memory 失败项已从版本/契约断言根因修正并分别完整补验。quota、cache、history、statistics、mapper、key-auth、model-router、两种限流插件的原完整 Go/Wasm 回归与本批 Wasm 字节一致；ai-billing 1.0.1 完整回归及 ai-proxy 2.0.10 媒体/Qwen/托管执行分层回归另有精确回执。
- CodeGraph 用于核对存储、协议编码、SDK/消费调用链。新导入路径不在既有索引时按当前源码核对，不以缺边代表无影响；旧 Go 归档提交前的 staged 影响检查为 low、无受影响执行流程。

本批发现并从职责边界修正：Bazel 只读临时产物不能被原地覆盖，构建缓存失效时替换 `build/` 产物；完整默认清单必须全部存在，但旧登记版本可以保留；精确 JSON 解析产生的 Decimal 应由公共序列化边界无损输出，不能临时转成浮点数或字符串。新的回归覆盖这些实际失效路径。

## 不可变制品、镜像与 PVC

53 个新完整包各在首次接纳时执行完整校验；ai-proxy 2.0.10 与 ai-billing 1.0.1 的全部载荷、角色、路径和环境引用与已登记包完全一致，复用原 ID、归档和来源记录。构建/证据代码变化不改写既有包的来源；当前构建提交另记在 review 中。载荷或环境引用变化的同版本仍拒绝。

镜像已推送：

```text
registry.lc.wooveep.com:9980/modelfusion/plugin-server:unified-source-20261031-d4e422d1d302@sha256:791ce5e0431b29545dc040e40416183be9893b059bfee1e69bf8ae97e97cb7ad
```

镜像独立对照源码清单，缺任一正式版本即拒绝构建。默认集合为 55 项，保留旧版本后共 **79 个已登记版本**，无未登记 Wasm。PVC 初始化改用镜像内 `seed-registered-store.py`，复制已验证登记记录，不重新校验完整包；实际新增 53、保留 26，原 ID 与摘要保持。镜像/PVC inventory 完全相同，摘要为 `sha256:d41a60f7c06ed69b8d13dd4f602535d5a45467341002b33887c45c290cc96843`。

[实际 HTTP 回执](unified-plugin-source-20261031/cluster-http-verification.json)覆盖 55 项正式包的 **545 个文件**：manifest、Wasm、schema、默认模板、文档、descriptor、metadata 和归档，全部 200，大小与 SHA256 均与登记记录一致。`GET /artifacts` 实际返回 200、79 项登记元数据，原 404 缺口已消除。

## Helm100 与真实网关

MF/DEP 责任聊天先完成 Helm99 交接及 Qwen 媒体路由真实初始化/重建验收；本批从其已安装 Chart 出发，仅替换两份 plugin-server 模板。server dry-run 确认只改变 plugin-server Deployment 和移除由镜像内入口替代的旧 seed ConfigMap，其余清单保持。随后 **Helm100** 成功部署；API/Admin/Compiler、媒体通知 worker 2.0.10、Secret/PVC 和 gateway 单副本均沿用 99 的状态。Chart SHA256 为 `cef1b629cde0f97b202f478df501d1f681a3303fa2d88acc5d206c4599706384`，DEP 实施提交 `7450923`。

唯一 gateway UID 仍为 `6d0aed93-733b-43d0-8383-79061a0f5979`。完成临时验证并清理后，16 个 Deployment 就绪，四域名入口均为 200；[健康回执](unified-plugin-source-20261031/final-health.json)记录实际观察时间。

在独立域名和临时后端上，真实 Envoy model-router **2.0.3** 完成：

| 用例 | 实际结果 |
| --- | --- |
| 请求头控制组 | 正确模型头匹配后端，200 |
| 正文 model 匹配 | 插件提取模型头并选择匹配后端，200 |
| 正文 model 不匹配 | 选择独立未匹配后端，404 |
| 重复 model 成员 | 插件在转发前拒绝，400 |

[网关实际回执](unified-plugin-source-20261031/model-router-live-acceptance.json)包含 ECDS 和后端收到的正文/模型头。Higress 在 Envoy 配置中呈现的是下载后的本地缓存文件；已对**该实际文件**计算 SHA256，等于登记的 `b70e04819447a66f9330420b388b17fbcf3c8afb3707d1b6786a58af2d085016`。最初仅期待原 URL 出现在 config_dump 的断言不符合实际实现，已按真实缓存文件核对。

五个临时资源已清理。首个 15 秒窗口仍有退役 ECDS 项，随后实际读回确认 Kubernetes 对象、ECDS、active/warming/draining listener 和路由引用均为零；[清理回执](unified-plugin-source-20261031/model-router-cleanup.json)保留该收敛边界，没有重启 gateway 或重跑业务用例来改变证据。

## Console 接入与后继消费完成

[Console 原生注册回执](unified-plugin-source-20261031/console-catalog-registration.json)证实 55 个正式默认版本均已接纳：53 新登记、2 复用，Console 共 75 个登记版本。接纳后，原 133 个 Ingress/WasmPlugin 的 UID、规格与代次逐项保持；上传/登记不会自动改动既有生效绑定。

Helm100时的[实际插件库对账](unified-plugin-source-20261031/console-library-formal-reconciliation.json)发现：列表总数 45，以下 10 类虽有真实登记制品，却未出现在库中；多项行默认版本还来自旧内置目录。缺少的是 `ai-context-limit`、`ai-endpoint-picker`、`gw-error-format`、`hmac-auth-apisix`、`json-converter`、`log-request-response`、`mcp-router`、`nginx-rewrite-compatible`、`replay-protection`、`response-cache`。

当时CodeGraph与当前文件核对确认列表入口 `console/src/compat.rs::list_wasm_plugins` 先从 SDK 内置目录取 45 类，过滤和分页后才查询这些名称的登记引用；因此重新登记、刷新浏览器或增加版本别名均不能补齐名称。单项详情和 README 同样先查内置目录。消费侧应以已接纳制品元数据覆盖可用名称/版本，按真实 SemVer 提供新配置的默认选项，保留已生效实例冻结的精确身份；列表、详情、配置和文档都需贯通，不能靠创建伪造的静态版本补洞。该段保留发现时状态；另一聊天批次结束后已由root串行接续修复。原生runtime和兼容compat两个入口均已贯通，详见[Helm101消费验收](../../../ocloudmodelfusion/docs/checks/2026-10-31-plugin-directory-consumer.md)。

原目标 79 卡最新实查仍为 **27 关闭 / 52 开放**。本批没有执行付费供应商调用或资金变更，不据此关闭完整事实、计费、媒体、W12/EB 或全量业务卡；后继消费缺口已关闭，原目标中的余额恢复与业务验收继续。

本批已[回填原总 Issue](https://github.com/wooveep/modelfusionproject/issues/1#issuecomment-6534322266)，将后继消费结果并入同一阶段评论，原52张开放卡的验收边界保持。

Helm101实际55项详情、路由配置、英语README共165次读取全部200；完整列表155.712ms，分页20/20/15，全部默认引用与正式交付清单一致。133项Ingress/WasmPlugin最终UID/spec与原文本plan_digest保持；key-auth.internal的generation变化单列于消费回执。该批未发送模型调用，原客户钱包和事实/账务数量保持。
