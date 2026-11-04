# HTTP 插件制品可靠发布实施计划

> 2026-10-25 细化：制品字段与状态唯一来源为 [IF-07](../../../ocloudmodelfusion/docs/contracts/2026-10-25-release-artifact-contract.md)，具体并行任务与 PS 映射见 [ART-01–06](2026-10-25-interface-implementation-plan.md)。本页保留交付范围与历史背景；接口计划只完成文档，未实现或运行验收。

| 字段 | 内容 |
| --- | --- |
| 状态 | 待排期；未实施 |
| 创建 / 更新日期 | 2026-10-22 / 2026-10-25 |
| 负责人 / 目标日期 | 待确认 / 待确认 |
| 优先级 | PS-01/02 为 P0，PS-03 为 P1 |
| 总体计划 | [跨项目计划与联合验收](../../../docs/plans/2026-10-22-http-plugin-upstream-adoption.md) |

## 目标、约束与现有依据

保持 HTTP GET/PUT 的路径和上传 Token 鉴权；补齐版本映射、完整性、并发发布及来源可追溯。沿用已有批量同步的 staging/失败恢复、旧版本保留和本地自有插件输入。OCI 作为构建时可选输入，运行时继续提供 HTTP 文件。

本文件是 plugin-server 详细任务的权威来源，使用普通 Markdown/Git/PR，不使用 Superpowers 或 OpenSpec。依据：[研究第 9/10 节](../../../ocloudmodelfusion/docs/higress-console-upstream-2026-10-22-research.md)、[历史上传计划](../superpowers/plans/2026-05-13-plugin-upload-api-cli.md)。历史任务状态不作为本期验收结果。

## 代码与项目接口

| 范围 | 现有入口 | 协作方 |
| --- | --- | --- |
| HTTP 上传、静态下载、metadata | [plugin_server.py](../../plugin_server.py)、[upload_plugin.py](../../upload_plugin.py)、[generate_metadata.py](../../generate_metadata.py) | Console 与部署项目使用既有路径 |
| OCI 导入、版本映射、打包 | [pull_plugins.py](../../pull_plugins.py)、[plugins.properties](../../plugins.properties)、[Dockerfile](../../Dockerfile)、[Makefile](../../Makefile) | 本地插件构建产物、OCI 来源 |
| 批量同步、已有回归 | [同步脚本](../../scripts/sync-plugin-artifacts.sh)、[tests/](../../tests/) | 插件构建项目 |
| 镜像构建与发布 | [CI](../../.github/workflows/build-plugin-server-image-and-push.yml) | 部署项目按发布证据选择镜像 |

技术决策和验收结论记录在本文件相应任务下；协议/schema 的详细格式如需单独文档，仅保留一份权威原文并在此链接。

## PS-01：版本、别名与构建输入（P0）

依赖：总体 BASE-00；先于强制版本不可覆盖与 catalog 切换交付。

- [ ] 盘点当前 HTTP 路径、实际文件 SHA-256、SDK 请求版本及构建来源，分开记录新规范版本与仍需兼容的旧 alias。
- [ ] 消除 pull 首轮固定写入 1.0.0、第二轮字符串替换 URL 的隐式版本策略；为旧别名提供显式映射和迁移说明，不直接删除已引用路径。
- [ ] 明确逻辑名、原始制品名、发布路径版本与真实制品版本的关系；支持 json-converter/jsonrpc-converter 这类 alias，未核对的条目不自动上架。
- [ ] 统一 Make、直接 Docker 构建和 CI 的输入校验；修正默认产物目录对搬迁前绝对路径的依赖。由相同的已声明输入生成相同插件集合，不以本地文件“存在”代替校验。
- [ ] 生成首份机器可读 inventory：schemaVersion、逻辑名、下载相对路径、发布版本、制品版本/alias、大小、Wasm SHA-256、来源状态。历史来源不明的记录标为 unknown，不填写猜测值。

验收标准：

- [ ] 用当前全部 properties 与本地插件集合验证：没有隐式把同一下载字节当作不同真实版本；兼容别名有明确记录且 SHA-256 可核对。
- [ ] 缺失文件、非法/重复目标路径、错误 alias 或摘要不一致使预检失败；预检失败不污染原有目录。
- [ ] 保留 ai-billing、ai-memory、本地 fork 和仍被引用的旧版本；Make 与直接 Docker 的声明输入一致时，inventory 的路径/摘要集合一致。

## PS-02：内容校验与同版本可靠发布（P0）

依赖：PS-01。三个写入入口（PUT、sync、pull）遵守相同版本语义；部署模式的共享写入约束由 DEP-01 配套保证。

- [ ] 统一校验 Wasm magic/版本头、基础格式、合理大小上限、实际读取长度与声明长度；HTTP 无效长度/格式返回明确 4xx，超上限返回 413，失败不创建可下载版本。
- [ ] 为每个插件计算并核对 SHA-256，统一生成 metadata；迁移期保留已使用的 metadata.txt/MD5 兼容字段，不以 MD5 替代新摘要。
- [ ] 定义同 name/version 相同字节为幂等成功，不同字节默认拒绝；HTTP 冲突返回 409，CLI/批量导入返回非零退出码及可定位错误。幂等重试不改写已发布文件和来源记录。
- [ ] 在私有临时位置完成接收、校验和 metadata 生成后再发布完整版本；使用明确的提交点与失败恢复，避免 GET 看见半文件、metadata 与插件属于不同版本、失败响应却已部分替换。
- [ ] 在“检查是否已发布”至“提交版本”间协调同制品并发；分别覆盖多线程 PUT、两个 sync/pull 进程以及计划部署支持的共享存储写入者，不仅依赖单进程线程锁。
- [ ] 默认保留旧版本，dry-run 不改动目标；prune 显式执行且依据引用/保留清单判断，常规镜像更新不自动清理旧 URL。
- [ ] 明确 HTTP 缓存与版本不可变的关系，提供可核对的摘要信息；不依赖覆盖同一路径后网关恰好重拉。

验收标准：

- [ ] 已有上传/下载/CLI 行为继续可用；空文件、无效 Wasm、截断、超限、错误摘要、非法路径均被拒绝且原制品不变。
- [ ] 同内容重传成功且字节/来源不变；不同内容重传返回冲突，原文件及 metadata 完整保留。
- [ ] 对同版本并发提交不同内容，至多一个候选成功；成功 GET 只得到完整已提交制品，未发布路径可返回未找到；不同版本互不覆盖。
- [ ] 接收中断、磁盘/rename/metadata 写入失败、发布进程退出后，旧版本保持可读或新版本尚不可见；临时目录不会被当作发布目录下载。
- [ ] sync 的原有保留、dry-run、prune 和失败恢复用例继续通过；CLI 对冲突和校验错误给出失败退出码。

## PS-03：来源锁、OCI 导入与镜像发布证据（P1）

依赖：PS-01/02。借鉴上游 #755/#758/#759/#760 及快照同步机制，使用本项目实际仓库和制品来源。

本任务交付来源/schema 契约及生成、校验工具，可先对基线 catalog 和 fixture 验证；MF-03 使用这些工具合成新的 catalog。最终联合版本的镜像 inventory 与 MF-03 catalog 在 DEP-02 一并生成、核对，避免将工具交付与最终版本绑定互相等待。

- [ ] 将三个 metadata 入口统一到版本化 schema；本地构建记录源码提交、构建输入/工作副本状态及构建信息，OCI 导入记录原 ref、resolved manifest digest、选定 layer 及 Wasm SHA-256，三种摘要明确区分。
- [ ] 正式发布要求选定制品来源已核对；历史 unknown 可保留服务兼容，但单列清单，不能宣传为已完成来源验证的新制品。
- [ ] 固定 OCI 预期 digest，正确处理 registry 端口、tag@digest、多 manifest 和非 Wasm attestation；不再默认取 index/layer 第一项。支持的包类型与提取规则显式定义，歧义输入拒绝。
- [ ] 校验 ORAS 下载工具本身的 checksum，记录受控工具/基础镜像来源；同一 digest 被解析为不符字节时构建失败。
- [ ] 生成构建时插件 inventory 并放入最终镜像；与 Console 的 catalog/spec hash 及本地覆盖来源建立可核对关系。未声明的上游目录项不自动替换本地插件。
- [ ] CI 对两个镜像架构检查插件集合/摘要一致并绑定构建来源标签；输出发布记录，把最终服务器镜像 digest 与 inventory 摘要绑定。镜像自身 digest 在构建后外部记录，不要求嵌入自身以形成循环依赖。
- [ ] 发布预检失败不推送正式版本；以新版本/digest 交付更新，保留可回滚镜像与文件清单，不在本期引入稳定 tag 覆盖恢复流程。

验收标准：

- [ ] 使用本地 OCI fixture 验证多 manifest、错误 digest、错误层类型、端口及 tag@digest；选择错误或来源不匹配均失败，正常输入得到声明的 Wasm SHA-256。
- [ ] 给定同一已锁定输入，两个干净构建得到相同插件路径/内容清单；不要求整张镜像逐字节可复现。
- [ ] 从一次发布记录可追到服务器镜像 digest、插件文件摘要、来源及 catalog 对照；自有插件未被官方快照覆盖。
- [ ] 镜像容器内逐文件计算的摘要与 inventory 一致；外部 HTTP 下载校验由 DEP-02 完成。

## 验证、迁移与交付记录

每项工作记录实际分支/提交和证据；当前 Python 7/7、同步脚本通过仅为研究时基线。沿用并扩充以下测试入口，在项目根执行：

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -B -m unittest discover -s tests -v
bash tests/test_sync_plugin_artifacts.sh
```

发布前先导出已有文件/alias/摘要并保留旧镜像。只将核对后的文件登记为受控版本；已有版本内容冲突通过新版本或明确别名处理，不在原路径修补。多副本/存储切换及 Pod 替换按[部署计划](../../../modelfusion-deploy/docs/2026-10-22-http-plugin-delivery-deployment-plan.md)验收。

| 日期 | 进展 / 证据 | 下一步 |
| --- | --- | --- |
| 2026-10-22 | 计划建立，PS-01/02/03 均未实施 | 待排期；从 BASE-00、PS-01 开始 |

完成时回填上述任务及验收勾选、实际 inventory/schema 位置、测试与发布证据，并同步总体计划。完成前不将已有脚本能力或文档检查计作本期交付。

## PS-04：完整插件包与打包入口（2026-10-24 新需求，设计讨论中）

配合 [Console 动态插件需求](../../../ocloudmodelfusion/docs/2026-10-24-console-requirements-discussion.md)，提供 `XXX-version.tar.gz` 打包与导入，复用 PS-01/02/03 的版本、摘要、原子发布和来源约束。平台管理员上传发布，租户配置使用。

- [ ] 打包脚本显式输入已编译制品、说明文档、配置 schema 和预配置；当前 key-auth/1.0.1 目录不足以独立生成完整包，需组合 SDK 元资料。
- [ ] 定义版本化 manifest、文件清单与 SHA-256；包名与声明名称/版本一致，预配置按配置 schema 校验。
- [ ] 限制解包大小、文件数量与路径，拒绝路径穿越及链接；校验完成后发布完整版本，不执行包内脚本。
- [ ] Console 注册失败可重试，制品未就绪不得暴露为可启用目录项；既有 HTTP Wasm 下载路径继续工作。
- [ ] 联合验收 key-auth 样本、新增插件、无效包、中断重试、持久化和真实网关加载；版本不可变及显式升级已确认，详细契约与 Console 对齐。

状态：需求纳入，未实施；依赖 PS-01/02/03 与 Console MF-03 扩展。

PS-04 决策补充（2026-10-24）：包携带语义版本；上传登记完整不可变版本，不触碰运行实例配置。Console 正常升级只允许严格更高版本，独立回滚可选择保留版本。包内环境默认信息仅声明受控引用，运行值由 Console 服务端解析；不得打包真实凭据或执行环境探测脚本。
