# 插件源码、编译和完整交付整合

| 字段 | 内容 |
| --- | --- |
| 状态 | 实施中 |
| 创建日期 | 2026-10-31 |
| 最近更新 | 2026-10-31 |
| 涉及项目 | 本批 root：plugin-server、旧 Go/SDK 构建入口及工作区索引；另一聊天：MF/SDK/Web 与 DEP |
| 负责人 | root 统一串行协调 |
| 优先级 | 用户确认的当前最高优先级；root 暂停其他产品工作，MF/DEP 由已确认的另一聊天实施 |
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
| 2 | 上游/定制逐插件合并、版本裁决、完整正式交付清单 | 已形成 72 项源码分类、55 项正式集合和版本裁决；定制合并待本批测试/提交 |
| 3 | 统一固定工具链构建、完整包、镜像与缺包拒绝 | 实施中；Go/Rust 已有实际产物，完整批次与包入库待完成 |
| 4 | SDK 版本/URL、旧目录退出与项目文档引用 | 旧入口已标记归档；消费契约见下文，MF/DEP 由另一聊天修改 |
| 5 | 分层批量验证；镜像/PVC/HTTP 对账及当前单网关模型匹配 | 待验证 |
| 6 | 提交、Issue 验收证据与恢复原任务 | 待完成 |

## 验收条件

- [x] 导入来源可追溯；实际 Git 合并父链及 `ort -Xsubtree=plugins` 更新路线保留，已用隔离 Git 夹具核对三方更新。
- [ ] 语义版本选择、未声明版本与定制升级可检查，model-router 默认选中真实 Go 新版本。
- [ ] 从根目录完成单插件和全量本地编译，正式集合完整；缺包直接拒绝。
- [ ] 完整包原校验与上传下载通过；镜像、PVC、HTTP 清单与摘要一致，原已发布版本没有被不同字节覆盖。
- [ ] SDK spec/版本映射与实际 HTTP URL 一致；当前单网关实际加载并验证模型匹配正负路径。
- [ ] 原独立目录退出日常构建；相关文档和相对链接更新。

## 已核对的回归与检查点

实际 E1 构建显式传入 `LEGACY_PLUGINS_DIR=delivery-empty-legacy`，验证脚本要求 legacy 为零，最终仅交付 19 个版本化包。当前镜像和 PVC 缺少 model-router。原本地镜像有 90 个 Wasm、44 类插件。旧下载脚本另存在将 2.0.0 来源写入 1.0.0 目录的问题，不以恢复该别名行为修复缺包。

原 Helm 91 的 9 项候选资源、四成员 preload 和 Recovery 检查点作为历史证据保留。用户随后要求网关从 4 副本减为 1，另一聊天已执行 Helm 92，当前成员为 `cfwnh`；旧四成员观察不能作为当前验收证据。本批尚未发送模型调用；不将 applied/preload 状态视为完整业务验收。

## 进展记录

| 日期 | 进展与证据 | 下一步 |
| --- | --- | --- |
| 2026-10-31 | 用户确认整合；根因与当前运行检查点已冻结；[Issue 阶段回填](https://github.com/wooveep/modelfusionproject/issues/1#issuecomment-6529636236) | 导入源码并保留上游三方合并基线 |

## 完成记录

待实际实施及验收后填写。

## 当前消费契约与共享文件责任

root 仅修改插件整合、原 Go/SDK 入口及本层文档。MF Console/Rust SDK/Web 与 DEP 文件、Helm 操作由另一聊天持有；本批不并发修改或升级同一个 release。

### 完整清单与默认版本

权威清单是 [`plugins/release/catalog.json`](../../plugins/release/catalog.json)。共 72 个源码实现项，按真实 SemVer 选择 55 个正式插件；未声明版本、示例、废弃 SNI 和原生 `.so` 保留分类。`declaredVersion` 必须与源码 `VERSION` 一致。C++ OAuth 的 2.0.1 是本次新建本地发行版本，原源码未声明版本；不冒称历史源码为 2.0.0。

| 插件 | 本次默认版本 | 消费路径 |
| --- | --- | --- |
| model-router | 2.0.3（Go） | `/plugins/model-router/2.0.3/plugin.wasm` |
| model-mapper | 2.0.4（Go，含原生正文基准及上游独立映射） | `/plugins/model-mapper/2.0.4/plugin.wasm` |
| ai-proxy | 2.0.9 | `/plugins/ai-proxy/2.0.9/plugin.wasm` |
| ai-quota / ai-cache / ai-history / ai-statistics | 2.0.4 | 对应 `/plugins/<名称>/2.0.4/plugin.wasm` |
| ai-memory | 0.1.7 | `/plugins/ai-memory/0.1.7/plugin.wasm` |
| ai-billing | 1.0.1 | `/plugins/ai-billing/1.0.1/plugin.wasm` |
| key-auth | 1.2.1 | `/plugins/key-auth/1.2.1/plugin.wasm` |

这些是待发布目标，不能据此宣称 HTTP 已可下载。正式入库后产出 `dist/plugin-catalog.json`，包含全部选定实现、artifact_id、version、content_digest、archive_digest、download_url 和 `x-higress` 元数据；镜像内为 `/opt/plugin-server/plugin-catalog.json`。运行时 `GET /artifacts` 返回全部已登记 manifest 与元数据，可一次获取后在前端分页，无须逐项下载 Wasm 或 archive。

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

- Python 来源选择、存储、登记复制与清单读取首批 19 项已通过；新完整清单门槛与后续全套仍待统一运行。
- 55 项配置模板已按实际解析器对齐并通过 schema 校验。空模板仅表示尚未配置；需要凭证的模板使用受控环境引用，不等同于部署即启用外部服务。
- ai-context-limit 分词表已记录官方 URL、校验和及 tiktoken 来源提交，作为显式嵌入输入保存。Rust 工具链包含校验和固定的 WASI SDK，ai-data-masking 已完成实际容器编译。
- 最终来源提交、全量编译、正式入库、镜像发布和当前网关运行验收尚未完成，后续以实际证据更新。
