# ART P0/D0 独立准备材料

更新：2026-10-25。性质：**draft**；`domain_validation: not_run`，独立消费者回执 `not_run`。这里只新增 D0-ART-01/02/03 白名单内的 fixture 数据和本说明，没有建立公共 schema、bundle-manifest、vendor、lock、helper 或 runner。三卡以及 `ART01.manifest` 均未完成，不将数据完整性自检记为领域验收。

代班执行：Codex agent `/root/artifact_fixture_prep`；自然人 assignee、backup、交付日期均待确认。实际分支 `chore/retire-legacy-workflow-20261022`，起点 HEAD `08ea31ddc2f9b077573961a5cd0d31fbcaf593e0`。共享文件仍由原卡唯一责任人串行集成；本批不接管生产或测试公共入口。

权威入口：[D0 原任务卡](../../../../docs/plans/2026-10-25-p0-d0-task-cards.md)、[实施卡](../../../../docs/plans/2026-10-25-gap-dispatch-task-cards.md)、[D0 包交换协议](../../../../../docs/plans/2026-10-25-p0-d0-dispatch-plan.md)、[IF07](../../../../../ocloudmodelfusion/docs/contracts/2026-10-25-release-artifact-contract.md)、[mf-json.v1](../../../../../ocloudmodelfusion/docs/contracts/2026-10-25-canonical-json-profile.md)。这些链接只用于人工追溯；消费者运行不得读取相邻工作树。

公共输入未冻结导入：D0-MF-01 的 bundle_version / manifest_sha256，以及 ART01 的两个 schema、投影规则和独立来源验证证据均待后继固定。样例的 `schema_version: artifact.v1-draft` 是明确草案值，尚无 schema 声明接受它。OCI 原 ref 的精确机器字段名及阶段细分也留待 ART01 冻结；相关反例只记录文档要求，不自造该 schema。所有 `expected` 是待实现 oracle 的预期，不能理解为实际通过或实际生产错误回执。

`manifests/minimal-descriptor.json` 是客户端 PackageDescriptor，归档根文件名为 `package.json`，只列四个 payload，不列自身。`manifests/minimal-server-manifest.json` 是完全合成的服务端 ArtifactManifest 样例，存于归档之外；artifact_id、created_at/by、download_url、archive_digest 等字段不能由上传客户端取得写权。示例域名 `.invalid` 不发生网络访问，样例时间与身份不代表服务端曾签发回执。

`source.kind=legacy_unknown, verification=unknown` 刻意保留历史兼容语义，不是经验证的新正式发布来源；本批没有虚构可核验源码提交或 OCI 取件记录。客户端写 `verified` 也必须由 ART 独立验证证据。Revision 的通用类型由 MF01 管理，本样例没有向 ArtifactManifest 擅增 revision。

准备内容如下：

| 材料 | 内容与边界 |
| --- | --- |
| `manifest-cases.json` | 20 个描述/回执/摘要案例，包含缺字段、重复路径/键、数字与字符串类型、来源分支及回执字段越权；可变回执字段不影响内容身份 |
| `canonical/*.json`、`digests.json` | 人工按明确 ASCII 字段顺序拼接的精确投影字节，无结尾换行；文件原字节 SHA-256 可独立复核，不调用未来被测规范化器生成预期 |
| `package-recipes/minimal.json`、`packages/*.tar.gz` | 三个小型固定字节归档：A/B 的原 tar 和内容相同，仅 gzip mtime 不同；第三包只改变 docs。USTAR 元信息、gzip 版本和字节摘要有记录 |
| `package-recipes/negative-recipes.json`、`package-cases.json` | 48 个案例，其中 3 个归档已经物化，其余是未运行的构造/策略配方；恶意包不展开、不写临时根外，I/O error 单列 execution_error |
| `lifecycle/*.json`、`lifecycle-cases.json` | 16 条纯数据轨迹，逐步输入/输出引用闭合；提交、同键/同版本冲突、ACK 丢失、查询超时、目录登记重试、非法倒退、保留/删除 unknown 分开 |
| `readback/*.json` | 4 个 payload 的非空预期集合和 9 个覆盖案例；包含空预期、漏 schema、200 错摘要、漏副本、私有 stage 可读等反例，全部观察是合成设计 |
| `*-entry-proposals.json`、`consumer-handoff.json` | 给 ART01 的登记提案与消费者动作清单；不是另一份发布清单或 lock，不是已经导出的可消费包 |
| `package-reports/preparation-self-check.json` | 实际作者数据完整性检查结果；检查范围单独列出，领域/运行验收仍为 not_run |

最小 Wasm 为 `0061736d01000000`，只有合法空模块结构的设计意图；没有导出 Higress 工厂、没有 ABI 或真实宿主加载验证。配置文件为独立 Draft 2020-12 schema，默认值有显式 `{}` 文件；这不代表旧 spec/OpenAPI 包装转换已经实现。

归档上限是每案例的合成策略输入，绝不是产品默认配额。等于边界使用 A 包已记录的 774 archive bytes、1321 expanded bytes、5 members、1189 max-member bytes；超出一单位的反例对同一个小包把相应测试限额降一。这样覆盖四种精确边界而不创建大文件。生产策略的单位、计数口径和默认值仍由后续负责方确定。

领域反例只定位拟验证阶段；例如修改 defaults、Wasm 结构或 schema 时，配方要求同步更新声明的 payload hash/size，让未来测试先证明前序阶段通过，再观察目标阶段拒绝。离线配方尚未执行，不能据当前 JSON 结构断言恶意包已被安全拒绝。synthetic secret 标记只验证明确测试策略和报告脱敏，不承诺识别任意真实凭据。

提交后 ACK 丢失与提交前断线对客户端都可能是 `unknown`。先用原 upload_id / 内容身份查询；查询超时、无状态、非权威 not-found 均不自动成为 failed 或“尚未提交”。确认可重放时也只用同键同内容。相同内容的不同 gzip 字节返回首次 artifact 及首次 archive_digest；同 Wasm 改 docs/default/schema 仍改变内容身份，不能原位覆盖。committed 不能回退到 receiving。

readback 的全部 files 必须按冻结实例集合核对返回字节；包内 `package.json` 不属于 files payload，服务端 manifest 也不在归档。当前非 Wasm 文件读路径尚待产品实现，因此仅写逻辑动作，不猜生产 HTTP 端点。任何单测/合成观察、目录临时文件或 HTTP 200 均不能代替真实完整文件、全副本和网关运行观察。

CodeGraph 于本批重新读取了当前源代码，未 import 业务模块：

- `plugin_server.py:35–45` 写旧 MD5 metadata，`:97–107` 直接写 Wasm 后写 metadata 并返回；不是原子完整包提交。
- `upload_plugin.py:14–32` 仅构造 PUT 并等待响应；没有持久 upload 查询协议。
- `generate_metadata.py:17–42` import 就可能遍历并写目录，因此没有执行它。
- `pull_plugins.py:43–50` 选择首个 Wasm 并 extract，`:152–155` 已有本地文件即复用，`:163–187` ORAS 与首 manifest/layer 处理；本批均未调用。

实际静态核对命令在 `plugin-server/` 执行：

```sh
codegraph explore "plugin-server metadata import package commit upload normalize path Archive Format PackageDescriptor ArtifactManifest"
codegraph explore "generate_metadata.py pull_plugins.py"
python3 --version
```

作者自检采用 Python 3.13.5 标准库的内联只读检查：严格解析容器 JSON、校对全部本地原字节 SHA 引用和 JSON Pointer、案例 ID 唯一/索引覆盖、四个预期 payload 的完整性、归档成员字节/固定头/统计及 A/B 压缩关系、字面量 canonical 文件摘要、Markdown 相对链接。只读取归档成员，不 extract。没有安装/导入 schema validator，没有运行领域 oracle、生产 unittest/Makefile、HTTP、ORAS、构建或部署；未新增持久校验脚本。

后继顺序：MF01 先固定公共包；ART01 冻结两个领域类型、来源机器字段和投影并发布闭包小包；ART02 才将配方物化并实现离线验包器；ART03 实现状态/读回 oracle，随后由 Console/SDK/DEP 独立固定导入回执。仅 ART01 修改共享 bundle-manifest/vendor/lock/helper；其他卡只交各自 entry 提案。旧 release、在途媒体、历史事实、回滚引用或删除响应未知时默认 retain，不能据当前未启用目录推定可 prune。
