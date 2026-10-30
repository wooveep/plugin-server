# 插件源码、编译和完整交付整合

| 字段 | 内容 |
| --- | --- |
| 状态 | 已完成（源码、完整交付及目录消费范围） |
| 创建日期 | 2026-10-31 |
| 最近更新 | 2026-10-31 |
| 涉及项目 | plugin-server、旧 Go/SDK 入口、MF Console、DEP 与工作区索引；共享文件串行接续 |
| 负责人 | root 统一串行协调 |
| 优先级 | 用户指定的插件整合优先批次已收口，继续原79项总任务 |
| 目标日期 | 待确认 |
| 需求来源 | 2026-10-31 用户确认统一插件仓库方案；原 79 项目标保留 |

## 目标与范围

`plugins/` 是唯一日常维护源码树，导入 Higress 插件源码、现行定制 Go 插件及其共享框架和 Proxy-Wasm SDK。`build/` 保存中间产物，`dist/plugins/` 保存按名称和真实版本组织的完整交付制品。原 `higress/` 保留上游原貌，原独立插件目录保留历史并退出日常构建链。

记录导入提交、上游树与定制来源；保留 Git 上游跟踪基线，后续以三方合并保留定制。按 VERSION 的语义版本选择同名实现；无 VERSION 的旧实现单列核对，不伪造历史版本。低版本定制必须合入较新基础并重新发版，不能由版本排序丢弃。

根目录提供单插件、全量、完整包和镜像入口；默认固定本地容器工具链，也支持显式宿主工具链。完整默认集合由正式上游发布清单和定制插件共同定义，示例/调试与原生 `.so` 分别标记。保留既有完整包、来源/摘要校验、HTTP 和 PVC 能力；同名同版本内容固定。

## 依赖与交付顺序

| 顺序 | 交付物 | 状态 |
| --- | --- | --- |
| 1 | 原环境/候选检查点；源码与产物目录分离；导入及 Git 跟踪基线 | 完成；e0785a8、c3093be，真实合并父链保留上游和 SDK 历史 |
| 2 | 上游/定制逐插件合并、版本裁决、完整正式交付清单 | 完成；72 项源码分类、55 项正式集合与真实版本裁决，4cc3e509 保留定制历史合并父链 |
| 3 | 统一固定工具链构建、完整包、镜像与缺包拒绝 | 完成；55/55 固定容器编译，53 新包与 2 原登记包，镜像/PVC/545 HTTP 文件对账通过 |
| 4 | SDK 版本/URL、旧目录退出与项目文档引用 | 完成；旧入口退出，Helm101已按登记元数据贯通55类目录、详情、配置与文档 |
| 5 | 分层批量验证；镜像/PVC/HTTP 对账及当前单网关模型匹配 | 完成；Python114、Go分层回归、实际模型匹配4例与临时资源清理通过 |
| 6 | 提交、Issue 验收证据与恢复原任务 | 完成；源码、镜像、DEP、[消费验收](../../../ocloudmodelfusion/docs/checks/2026-10-31-plugin-directory-consumer.md)及阶段Issue证据已交；原业务目标继续 |

## 验收条件

- [x] 导入来源可追溯；实际 Git 合并父链及 `ort -Xsubtree=plugins` 更新路线保留，已用隔离 Git 夹具核对三方更新。
- [x] 语义版本选择、未声明版本与定制升级可检查，model-router 默认选中真实 Go 2.0.3。
- [x] 从根目录完成单插件和全量本地编译，正式集合完整；缺包直接拒绝。
- [x] 完整包原校验与上传下载通过；镜像、PVC、HTTP 清单与摘要一致，原已发布版本没有被不同字节覆盖。
- [x] 登记元数据的版本/URL与55项正式清单一致；当前单网关实际加载并验证模型匹配正负路径。
- [x] 原独立 Go/SDK 目录退出日常构建，归档提交已推送；统一入口和相关文档已更新。

## 已核对的回归与检查点

实际 E1 构建显式传入 `LEGACY_PLUGINS_DIR=delivery-empty-legacy`，验证脚本要求 legacy 为零，最终仅交付 19 个版本化包。当前镜像和 PVC 缺少 model-router。原本地镜像有 90 个 Wasm、44 类插件。旧下载脚本另存在将 2.0.0 来源写入 1.0.0 目录的问题，不以恢复该别名行为修复缺包。

原 Helm 91 的 9 项候选资源、四成员 preload 和 Recovery 检查点作为历史证据保留。用户随后要求网关从 4 副本减为 1，另一聊天执行 Helm 92 时成员为 `cfwnh`；Helm99 已实际重建为 `6gspj`，本批 Helm100 保持该 UID，旧四成员观察不能作为当前验收证据。本批尚未发送模型调用；不将 applied/preload 状态视为完整业务验收。

## 进展记录

| 日期 | 进展与证据 | 下一步 |
| --- | --- | --- |
| 2026-10-31 | 用户确认整合；根因与初始运行检查点已冻结；[此前 Issue 阶段回填](https://github.com/wooveep/modelfusionproject/issues/1#issuecomment-6529636236) | 已保留导入历史，后续更新沿 Git 三方合并 |
| 2026-10-31 | [55项完整交付](../checks/2026-10-31-unified-plugin-source-and-delivery.md)及[Helm101目录消费](../../../ocloudmodelfusion/docs/checks/2026-10-31-plugin-directory-consumer.md)通过 | 本整合范围完成；继续余额恢复、ART05及原79项业务验收 |

## 完成记录

源码、完整交付和55类目录消费均已验收；[完整交付报告](../checks/2026-10-31-unified-plugin-source-and-delivery.md)与[Helm101消费验收](../../../ocloudmodelfusion/docs/checks/2026-10-31-plugin-directory-consumer.md)分别为权威证据。仅关闭本整合范围，原79项总目标继续。

## 当前消费契约与共享文件责任

另一聊天的路由/性能批次以94187a7b、Helm99完成；root先串行交付Helm100，再在其批次结束后接续Console目录和Helm101。未并发修改同一共享文件或升级同一个release。

### 完整清单与默认版本

权威清单是 [`plugins/release/catalog.json`](../../plugins/release/catalog.json)。共 72 个源码实现项，按真实 SemVer 选择 55 个正式插件；未声明版本、示例、废弃 SNI 和原生 `.so` 保留分类。`declaredVersion` 必须与源码 `VERSION` 一致。C++ OAuth 的 2.0.1 是本次新建本地发行版本，原源码未声明版本；不冒称历史源码为 2.0.0。

| 插件 | 本次默认版本 | 消费路径 |
| --- | --- | --- |
| model-router | 2.0.3（Go） | `/plugins/model-router/2.0.3/plugin.wasm` |
| model-mapper | 2.0.4（Go，含原生正文基准及上游独立映射） | `/plugins/model-mapper/2.0.4/plugin.wasm` |
| ai-proxy | 2.0.10 | `/plugins/ai-proxy/2.0.10/plugin.wasm` |
| ai-quota / ai-cache / ai-history / ai-statistics | 2.0.4 | 对应 `/plugins/<名称>/2.0.4/plugin.wasm` |
| ai-memory | 0.1.7 | `/plugins/ai-memory/0.1.7/plugin.wasm` |
| ai-billing | 1.0.1 | `/plugins/ai-billing/1.0.1/plugin.wasm` |
| key-auth | 1.2.1 | `/plugins/key-auth/1.2.1/plugin.wasm` |

55 项完整集合均已实际入库、交付并通过 HTTP 下载摘要核对，详见[完整交付验收](../checks/2026-10-31-unified-plugin-source-and-delivery.md)。正式入库后产出 `dist/plugin-catalog.json`，包含全部选定实现、artifact_id、version、content_digest、archive_digest、download_url 和 `x-higress` 元数据；镜像内为 `/opt/plugin-server/plugin-catalog.json`。运行时 `GET /artifacts` 返回全部已登记 manifest 与元数据，可一次获取后在前端分页，无须逐项下载 Wasm 或 archive。

### DEP PVC 初始化接入

新镜像提供 `/opt/plugin-server/scripts/seed-registered-store.py`。旧 Chart 脚本会重新校验完整包三次，应由 DEP 责任聊天替换为复制已验证的登记记录。按照当前 PVC 根目录挂载约定使用：

```sh
python3 /opt/plugin-server/scripts/seed-registered-store.py \
  --source /usr/share/plugin-server/plugins \
  --source-state /usr/share/plugin-server/.plugins-artifact-state \
  --destination /data/plugins \
  --destination-state /data/.plugins-artifact-state
```

源登记库以 SQLite 只读模式读取；新版本复制后沿用 artifact_id 和摘要，目标已存在相同内容时保留目标既有 ID，不同内容的同版本拒绝覆盖。列表、镜像清单与 PVC 初始化均不再执行完整包校验；正常 HTTP 下载仍核对被请求文件的已登记大小/摘要。只有首次上传入库校验完整包、配置 schema、默认模板和 Wasm。

源码/模板有变化时先 `make build-all`，再 `make prepare-packages` 生成可审阅候选与 `build/packages/review.json`。独立核对来源、编译结果和配置引用后提供 `TRUSTED_POLICY`，`make package-plugins` 完成首次入库。镜像构建独立对照完整源码清单拒绝遗漏正式插件，不接受旧 E1 的 19 项白名单。

### 分层验证进展

- Python 来源选择、存储、登记复制、完整清单与真实数值元数据全套 114 项通过。
- 55 项配置模板已按实际解析器对齐并通过 schema 校验。空模板仅表示尚未配置；需要凭证的模板使用受控环境引用，不等同于部署即启用外部服务。
- ai-context-limit 分词表已记录官方 URL、校验和及 tiktoken 来源提交，作为显式嵌入输入保存。Rust 工具链包含校验和固定的 WASI SDK，ai-data-masking 已完成实际容器编译。
- 完整编译、正式入库、镜像/PVC/HTTP与当前单网关模型匹配已完成；MF目录名称/默认版本缺口已由Helm101关闭。


## 2026-10-31 原生媒体阻塞修复交付

以下保留交接时的历史状态；当前完成状态以前文及Helm101验收为准。

按 MF/DEP 聊天的新反馈优先完成两个原生媒体包，完整清单镜像工作随后继续。源码整合及媒体角色修复已提交 `4cc3e509`，版本裁决后继为 `c29670ad`，均已推送；Go 原始仓库历史通过真实合并父链保留。

ai-proxy 2.0.9、ai-billing 1.0.1 已在原持久化制品库提交并完成 8 个 HTTP 下载核对。ai-billing 采用 1.0.1 是因为线上已有登记的 1.0.0。配置字段边界、媒体唯一事实归属、源码、回归和接入地址以[原生媒体契约与交付](../checks/2026-10-31-native-media-plugin-contract.md)及其机器回执为权威来源。

本批没有更改 Helm；另一聊天已在单网关上继续部署，Helm 93 为此前交接点，当前修订以该责任聊天的实际记录为准。MF 注册、真正的 Higress VM 初始化与目标路由恢复由该聊天继续，不能用本地 VM 回归替代真实网关验收。完整 55 项正式制品、统一镜像、PVC 清单及 model-router 正负运行路径仍待收口。

2026-10-31 后继：真实 Qwen 媒体初始化暴露 2.0.9 的文本提供方家族限制，root 已从媒体协议归属修复并发布 **ai-proxy 2.0.10**。当前应消费 **2.0.10 / ai-billing 1.0.1**；[Qwen 后继机器回执](../checks/2026-10-31-native-media-qwen-plugin-delivery.json)包含实际登记 ID、Go/Wasm 回归和四个 HTTP 下载摘要。MF/DEP 继续真实 Envoy/媒体路径验收；本条不提前关闭运行验收项。

## 2026-10-31 完整交付、实际模型匹配与消费缺口（Helm100历史检查点）

[完整验收报告](../checks/2026-10-31-unified-plugin-source-and-delivery.md)及[机器回执](../checks/unified-plugin-source-20261031/receipt.json)已记录：55/55 编译、114 项 Python、分层 Go/Wasm 回归、固定镜像、Helm100、79 个镜像/PVC 登记版本、545 个 HTTP 文件和真实单网关模型匹配四例。五个临时资源已删除，Envoy 引用最终清零；16 个部署就绪，四域名 200。

55 个正式版本已通过 Console 原生注册，53 新登记、2 复用，133 项原运行资源规格/代次保持。但库 API 仍由内置 45 类驱动，缺 10 类，默认展示版本沿用旧内置目录；MF/SDK/Web 修复归属待用户确认。不能因制品已可下载而把消费项记为完成。原 79 卡实查仍 27 关闭/52 开放；本批没有付费调用或资金变更。

## 2026-10-31 Helm101目录消费完成

另一聊天批次结束后由root串行接续，MF实现fc6364a1，原生与兼容入口共用已登记元数据。21项分层检查和严格Clippy通过；线上55项详情、配置、README共165次读取全部200，完整列表约156ms、分页20/20/15，原生默认引用与正式清单完全一致。[权威验收](../../../ocloudmodelfusion/docs/checks/2026-10-31-plugin-directory-consumer.md)保留准确镜像、二进制和资源/钱包比较。原Recovery仍requested，未置ready；下一步恢复原79项业务及ART05联合出口。
