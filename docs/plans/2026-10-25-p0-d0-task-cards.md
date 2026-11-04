# plugin-server：P0 / D0 可派工任务卡

> 2026-10-25任务跟踪：本页原D0卡继续为白名单权威；每卡Issue、最小片段及更新后的首批并行顺序统一登记在[派工与Issue总表](../../../docs/plans/2026-10-25-gap-dispatch-and-issue-tracking.md)。D0完成不代表后续产品或工程边界验收完成。

更新：2026-10-25。状态：仅拆卡文档；下列schema、夹具、校验器与测试均未创建或执行。本轮仅更新计划文档与相关索引，不改业务代码或部署。具体人员/日历待排期，责任按模块唯一指定。

本计划承接[跨项目D0派工总表](../../../docs/plans/2026-10-25-p0-d0-dispatch-plan.md)，把[ART接口实施计划](2026-10-25-interface-implementation-plan.md)中可先行的契约交付拆为三卡。业务字段/状态权威仍为[IF-07](../../../ocloudmodelfusion/docs/contracts/2026-10-25-release-artifact-contract.md)，共同类型见[九类接口](../../../docs/plans/2026-10-25-nine-interface-contracts.md)，运行证据见[IF-09](../../../modelfusion-deploy/docs/plans/2026-10-25-support-evidence-contract.md)与[EB-05](../../../ocloudmodelfusion/docs/plans/2026-10-25-release-boundary-verification-plan.md)。D0通过只表示可供消费者实现的机器契约/样例一致，不表示上传服务已可靠、网关已加载或产品可发布。

## 1. 卡片边界与依赖顺序

| 卡ID / 唯一责任 | 可独立交付的结果 | 硬依赖 / 合入顺序 |
| --- | --- | --- |
| D0-ART-01 / ART契约维护 | 唯一ArtifactManifest机器schema、内容摘要golden语料、公共类型固定导入 | D0-MF-01公共common schema/规范化规则交付后冻结；可先拟字段映射。先合入ART01.manifest最小出口，独立消费者互验稍后回执，不阻塞MF05 |
| D0-ART-02 / ART离线包验证 | 有效/负包配方、确定字节夹具、离线校验器接口与测试规格 | 依ART01.manifest最小出口；负例清单可与01并行，随后合入 |
| D0-ART-03 / ART交接维护 | 提交/读回/unknown状态语料、消费者读取与重放动作样例、交接lock | 依01身份及02验证报告格式；状态场景可并行起草，最后合入 |

只在后续明确获得编码授权后，才按各卡文件白名单实现。D0不包含：修改生产PUT/GET、实际完整包原子存储、并发锁/提交点、实际持久upload查询、上传token策略变更、生产OCI拉取、镜像构建推送、HTTP运行读回、Kubernetes发布或网关测试。这些仍属ART-02/03/04及MF/DEP后续实施，不能偷偷塞进“先补测试”卡。

合入顺序为D0-MF-01公共common固定交付→ART01.manifest最小出口→02→03；D0-MF-05在最小出口后即可合入消费，不等待ART01整卡的独立消费者互验回执。Console/SDK/DEP可以先用明确标为draft的样例并行设计，不以draft当稳定依赖。三卡消费者最终只接受固定内容摘要的已验收交付，不跟随工作目录最新文件或Git分支名。

## 2. 唯一schema、vendor与共享文件owner

唯一作者源：`plugin-server/contracts/v1/artifact-manifest.schema.json`。Console/SDK/DEP可导入锁定副本用于测试，不在邻仓另写同名权威schema；逻辑规格仍链接IF-07，机器约束与文档冲突时卡片失败并回到契约维护修订，不能由某消费者宽松解析解决。

公共类型源由**D0-MF-01**唯一交付：`ocloudmodelfusion/contracts/v1/common.schema.json`。本仓未来固定导入至`contracts/vendor/ocloudmodelfusion/v1/common.schema.json`；采用总表统一契约包manifest和lock，不另造依赖格式。导入是开发期显式动作，**测试/生产运行不访问邻仓相对路径、网络或浮动URL**。

ART自身包manifest拟落`contracts/v1/bundle-manifest.json`，仅D0-ART-01维护。统一字段为`bundle_version / producer_project / source_revision / schema_dialect / entries[{schema_id:URN,path,sha256}] / fixtures[{case_id,path,sha256,expected:{verdict,error_code}}]`；路径为包内路径，D0交换manifest/lock的文件sha256与manifest_sha256统一为64位小写hex、无前缀，固定到精确文件字节；IF-07产品content_digest等仍遵循其`sha256:`前缀约定，两种载体不能混填。生产者源提交/交付revision与内容摘要同时保留，工作树未提交时标明确交付revision，不声称Git提交已包含文件。

common导入lock拟落`contracts/vendor/lock.json`，顶层固定为`packages`数组，每个包项严格采用`bundle_version / producer_project / manifest_sha256 / vendor_root / imported_schema_ids`。平台上游源manifest为`ocloudmodelfusion/contracts/v1/contract-manifest.json`；导入副本可重命名保存为`contracts/vendor/ocloudmodelfusion/bundle-manifest.json`，但字节必须完全不变，manifest_sha256仍是源文件摘要，不能因重命名重写JSON或路径。先验证锁定manifest摘要，再按entries逐文件验证路径、SHA和URN闭包；lock不另复制一份可漂移字段schema。ART的schema_id使用项目统一URN规则，common URN原样沿用D0-MF-01，重复URN异字节或重复路径立即拒绝。

02/03各自在独占case目录中提交fixture及entry提案；01串行合入本仓bundle-manifest，禁止三卡直接共同编辑此共享清单。消费者导入ART包也依同一manifest/lock格式，不能只复制主schema而漏common闭包。

vendor文件保留源字节及$id，通过本地resolver映射官方$id到锁定文件；离线禁止自动联网解析$ref。校验器须先查锁、文件摘要、版本和闭包引用完整性；缺文件、wrong hash、不兼容common或未知规范化版本立即失败。不得复制common类型再手改为“本仓兼容版”。若common存在传递依赖，01同样固定导入整个实际引用闭包，完整列入lock；出现白名单外文件先更新拆卡再实施，不自行猜路径。

| 共享路径（均拟新增，除另注明） | 唯一维护卡/责任 | 其他卡如何使用 |
| --- | --- | --- |
| `contracts/v1/artifact-manifest.schema.json`、`contracts/v1/artifact-contract-notes.md`、`contracts/v1/bundle-manifest.json` | D0-ART-01 / ART契约维护 | 02/03只读；字段/枚举变化由01变更并重跑消费者语料 |
| `contracts/vendor/lock.json`、上游bundle-manifest、vendor common | D0-ART-01 / ART契约维护 | 02/03不改锁或$id；依赖升级必须新版本/摘要交付 |
| `tests/contracts/requirements.txt`、`tests/contracts/requirements.lock` | D0-ART-01 / ART测试基础维护 | 复用公共schema方已选dialect/校验器；首次实现固定版本/校验信息，不临时pip联网获取最新版 |
| `tests/contracts/artifact_contract_support.py`、可选元schema测试vendor及manifest | D0-ART-01 / ART测试基础维护 | 本地schema resolver、规范化和测试加载帮助；无业务服务import、网络或生产目录写入副作用 |
| `tests/contracts/validate_artifact_fixture.py` | D0-ART-02 / ART离线包验证 | 03只消费验证结果；不让每卡另写一套摘要/验包算法 |
| `contracts/fixtures/v1/artifact/manifest-cases.json` | D0-ART-01 | 只维护manifest/摘要样例；02/03在各自独占文件追加案例，禁止多人改此总表 |
| 现有`Makefile`及生产py/shell文件 | 不在任何D0写白名单 | 保持现状；D0用直接测试入口，无须为合入新增业务依赖或修改生产构建 |

官方Draft 2020-12元schema URI仅作为本地注册表键，既不是业务schema URN，也不允许联网获取。01可使用已锁验证器内置元schema闭包；若需显式vendor，限定纯测试目录`tests/contracts/vendor/json-schema-2020-12/`及`tests/contracts/metaschema-manifest.json`，由01记录URI→本地文件/精确摘要/来源版本，缺失任一元schema引用即拒绝。元schema不得写入产品包或混为另一份业务common。

未来测试依赖文件只服务离线契约校验；D0不把JSON Schema校验器塞入生产镜像。该项目现有Python测试主要使用标准库，新增校验库如需引入由01登记固定依赖，不假设已安装。

## 3. D0-ART-01：ArtifactManifest schema与规范摘要语料

**输入。** IF-07第2节、D0-MF-01公共Id/Time/Revision/Digest及规范化规则、冻结common导入物、现有metadata字段映射。ArtifactManifest来源分支local_build/oci_import/legacy_unknown必须沿文档表达，历史未知不能伪造verified。

**输出。** Draft机器schema、文档字段→schema路径对照、有效/无效manifest、精确规范化UTF-8字节与期望content_digest；形成消费者可独立执行的golden corpus。schema只表达字段类型/必填/互斥/enum/path模式，跨文件摘要和default按配置schema验证交02，不能把所有验证都假装JSON Schema能完成。

**后续获授权后的文件白名单。**

- 新增：`contracts/v1/artifact-manifest.schema.json`、`contracts/v1/artifact-contract-notes.md`、`contracts/v1/bundle-manifest.json`、`contracts/vendor/ocloudmodelfusion/v1/common.schema.json`、`contracts/vendor/ocloudmodelfusion/bundle-manifest.json`、`contracts/vendor/lock.json`。
- 新增：`contracts/fixtures/v1/artifact/manifest-cases.json`、`contracts/fixtures/v1/artifact/manifests/*.json`、`contracts/fixtures/v1/artifact/canonical/*.json`、`contracts/fixtures/v1/artifact/digests.json`。
- 新增：`tests/contracts/requirements.txt`、`tests/contracts/requirements.lock`、`tests/contracts/artifact_contract_support.py`、`tests/test_contract_artifact_manifest.py`；仅在锁定验证器未内置所需闭包时，新增`tests/contracts/vendor/json-schema-2020-12/**/*.json`、`tests/contracts/metaschema-manifest.json`，不得扩展到生产目录。
- 只读：现有`plugin_server.py`、`generate_metadata.py`、`pull_plugins.py`、`upload_plugin.py`、`tests/test_plugin_server.py`及IF-07。任何生产路径修改均移出本卡。

`canonical/*.json`保存期望精确字节；不可由被测canonicalizer运行时重新生成期望，否则同一个错误会让自己通过。数值编码/Unicode等按公共规范化规则；若公共规则尚未定义实际用到的边界，01保持blocked并交公共维护卡，不由本仓私自选择新规范。

**必须样例。**

| 输入变化 | 期望断言 |
| --- | --- |
| JSON object键顺序、files声明顺序变化 | 按IF-07规定排序后的规范内容与digest相同；files仍按path唯一 |
| archive_digest、artifact_id、created_at/by、download_url不同 | content_digest不变；这些字段不在规范内容投影，仍接受各自schema校验 |
| Wasm字节、schema/default/docs文件摘要或声明来源变化 | content_digest改变；同name/version不能被03语料视为幂等 |
| 同规范内容两种压缩时间/元信息 | archive_digest不同、content_digest相同；返回首次提交归档信息语义交03 |
| 缺required、错误schema版本、短digest/大写hex、revision类型错、重复file path | schema或明确语义阶段拒绝；不自动补缺省、降级或忽略重复 |
| legacy_unknown来源却填写verified、OCI缺选定layer、local_build缺固定构建输入 | 按分支条件拒绝；有效未知历史样例保持unknown，不当新发布verified |

**最小合入出口：ART01.manifest。** 本仓唯一schema、固定common/vendor锁、规范摘要与正负golden corpus已经完成离线自验并形成不可变bundle-manifest：案例有稳定case_id、预期阶段/错误码及JSON pointer；机器shape约束及独立语义断言覆盖IF-07 R/C要求；全部$ref及元schema闭包本地解析；vendor篡改、缺失、网络引用、URN冲突负例必须拒绝。记录实际输入包版本、命令、工具与逐例结果。这是已验证的稳定片段而非草稿，可先合入交D0-MF-05、D0-ART-02及其他消费者，**不以独立消费者测试完成为前置**。

**整卡最终条件与后续回执。** 最小出口交出后，由D0-MF-05或另一独立消费者固定导入包，运行同一语料，回执一致的规范字节/digest和错误判定；该回执补齐01整卡最终验收，不阻塞已验收最小出口及其后继开发。回执发现不一致时按责任修复并出新固定包版本，旧片段保留证据，通知受影响消费者重新锁定；不能静默覆盖已交付manifest。代码/schema尚未形成时最小出口与整卡均not_run，不以本页表格代替通过。

**依赖与交接。** 先D0-MF-01→ART01.manifest→D0-MF-05/ART02并行；独立消费者回执后补01整卡验收。向D0-ART-02提供schema、规范化器与digest语料固定摘要，向Console D0-MF-01/MF-IF00/MF-REL01、SDK与DEP交付相同包。schema角色不包含生产ReleasePlan写权，不能由ART定义ready/available。

## 4. D0-ART-02：有效/负包与离线验包规格

**输入。** 01已冻结schema/公共类型/规范化支持、IF-07文件角色及环境引用限制、[EB05-V01/02](../../../ocloudmodelfusion/docs/plans/2026-10-25-release-boundary-verification-plan.md)验收边界。有效fixture只用可公开的最小测试Wasm、示例schema/default/docs和合成环境引用，无真实密钥、外部依赖或包内可执行脚本。

**输出。** 有效包清单与构造配方、精确字节/文件digest、恶意/无效归档语料、只读离线验包器与报告契约规格。其职责是报告是否满足包协议，不发布、修复、上传或从registry下载文件。异常归档只在独立临时目录验证，路径预检后才允许受控展开；不写现有`plugins/`，不调用生产pull导入流程。

**后续获授权后的文件白名单。**

- 新增：`contracts/fixtures/v1/artifact/package-cases.json`、`contracts/fixtures/v1/artifact/package-recipes/*.json`、`contracts/fixtures/v1/artifact/packages/*`、`contracts/fixtures/v1/artifact/package-reports/*.json`、`contracts/fixtures/v1/artifact/package-entry-proposals.json`。
- 新增：`tests/contracts/build_artifact_fixtures.py`、`tests/contracts/validate_artifact_fixture.py`、`tests/test_contract_artifact_packages.py`。
- 只读：01输出，现有`pull_plugins.py`/`generate_metadata.py`及三份既有Python测试；不改01共有helper/lock/schema。需新增共享方法先提交01维护责任合入，再本卡消费。

归档大小、解包后大小、成员数与单成员上限由fixture策略输入固定记录，正好等于/超过边界各有样例；本卡不把测试上限冒充产品默认配额。归档配方应可在离线临时目录重建且字节可核对，不下载任意Wasm。最小Wasm的结构验证与真实网关ABI/工厂加载明确分开：有效离线包不宣称可在当前网关执行。

**验包器报告规格。** 输出`case_id`、输入manifest/archive摘要、`validation_stage`、`result:valid|invalid`、`errors[{code,pointer,path?}]`、实际成员/字节统计及normalized_content_digest（仅验证完成后）。真实I/O或依赖错误不伪装为包invalid，应独立报告`execution_error`并令本次测试blocked/failed；错误路径脱敏、无任何secret值。字段是D0测试工具报告，不冒充生产上传API。

**负包矩阵与通过条件。**

| 负包／反例 | 必须结果 |
| --- | --- |
| 截断tar/gzip、声明长度不符、Wasm magic/版本头错误、错误SHA | 在明确阶段拒绝，错误成员可定位；不能只检查文件名或前四字节 |
| 绝对路径、`../`、重复规范化路径、符号/硬链接、额外未声明文件、重复归档成员 | 展开前或受控逐项读取拒绝，临时根之外零写入；不能选择首个.wasm当有效包 |
| 两个Wasm、缺docs/schema/default文件、manifest角色或引用不一致 | 校验失败；不能因为Wasm本身有效忽略完整包 |
| defaults不符合声明schema、未知/不受控environment_ref、真实秘密样例 | 按明确规则拒绝；秘密使用合成标记且报告不回显，不引入不可靠的“任意字符串密钥识别”承诺 |
| 超过固定解包大小/成员数/单文件上限、读取异常 | 边界拒绝、无残留发布目录；不实际生成巨量数据或改变生产限制 |
| 同内容不同压缩字节、同Wasm不同docs/default | 前者合法且内容digest相同；后者内容digest不同；与01/03一致 |

defaults的schema校验使用01固定依赖；包内schema不得联网获取外部$ref，允许引用范围由包内声明闭包/已锁公共schema决定。无法解析引用是明确invalid，不依赖联网碰运气。

**依赖与合入。** 02依01摘要/类型冻结；包配方负例设计可并行。先合有效最小包及报告格式，再负包矩阵。向03交付稳定报告样例/内容identity，向DEP FX-06/FX-02交付独立expected文件集合；P1再将离线验证逻辑按边界接入生产提交，不在D0修改PUT、sync或pull。

## 5. D0-ART-03：提交、读回、unknown语料与消费者交接

**输入。** 01 manifest/digest及02有效/无效包报告，IF-07上传状态、同键/同版本幂等与错误动作，IF-09/EB-05的真实读回证据条件。

**输出。** 纯数据状态轨迹、请求/结果/后续动作样例、manifest→预期HTTP文件集合、消费者交接清单。状态轨迹是协议样例，不能写成“服务已实现持久查询”。接收方据样例写测试适配器，不依赖真实plugin-server、K8s或邻仓工作树。

**后续获授权后的文件白名单。**

- 新增：`contracts/fixtures/v1/artifact/lifecycle-cases.json`、`contracts/fixtures/v1/artifact/lifecycle/*.json`、`contracts/fixtures/v1/artifact/readback/*.json`、`contracts/fixtures/v1/artifact/lifecycle-entry-proposals.json`。
- 新增：`contracts/fixtures/v1/artifact/consumer-handoff.json`、`contracts/fixtures/v1/artifact/consumer-expectations.md`、`tests/test_contract_artifact_lifecycle.py`。
- 只读：01 schema/vendor/摘要helper、02报告/验包器及现有`upload_plugin.py`/`tests/test_upload_plugin.py`；D0-ART-03不新增第二份生产状态schema或公共manifest定义。

轨迹行使用稳定`case_id`、`step`、`actor_kind`、`command`、`input_ref`、`observed_result_ref`、`expected_next_action`、`expected_side_effects`；所有引用在本交付目录内闭合并含文件摘要。具体生产DTO由IF-07及其所属维护方定义，未定义端点标逻辑动作，不猜HTTP URL。

| 必需轨迹 | 期望后续动作／禁止误解 |
| --- | --- |
| receiving→validating→committed；校验rejected、提交前I/O failed | committed不可原地覆盖；invalid修正后新提交意图，不把失败语料写为生产已恢复 |
| 同upload_id同内容重放、同键异内容、同name/version异规范内容 | 同内容取原结果；异键内容冲突可定位；不因Wasm相同忽略schema/default/docs变化 |
| 实际提交后ACK丢失、提交前断线、查询本身超时 | 客户端outcome_unknown；查询原upload/内容身份再决定同键重放；缺状态不是“未提交”，不换内容试探 |
| 两个压缩包内容相同但archive_digest不同 | 同内容幂等取首次artifact/归档来源，不能覆写首次记录 |
| artifact committed但Console登记失败或包HTTP不可读 | 只重试可重放的登记/核对；不得推断网关ready或开放production；ART不替Console改运行实例 |
| empty expected、漏manifest成员、响应200但错摘要、只读首副本、临时目录可读 | 证据不合格；离线预期集合必须非空且覆盖全部声明文件；真正HTTP与多实例验证留P1/DEP |
| 旧release/媒体/回滚引用尚在、delete响应未知 | 默认retain；不能用目录当前未启用推定可prune；没有完整引用或删除结果时不宣称已删 |
| legacy_unknown历史包与新verified包 | 历史保持兼容读取标识，新正式发布要求经核验来源；不得把测试样例当来源真实性证据 |

**通过条件。** 轨迹每步所有输入/输出引用能离线解析且摘要一致；类型正确的非法转换也被测试oracle拒绝；unknown不落成failed、committed不倒退；同一数据由ART协议测试与Console/DEP消费者测试得到一致动作；不得调用生产upload/pull或真实HTTP做D0通过条件。consumer-handoff只描述消费者动作/适用范围及统一bundle-manifest引用，不新增包manifest或lock格式；01统一manifest记录准确源版本、schema/fixture digest及预期verdict/error_code，消费者固定导入transitive闭包并锁定，不通过运行时邻仓读取。

**依赖与合入。** 先01→02固定报告→03；交Console MF-IF00/MF-REL01/MF-REL02，SDK只消费包版本/引用，DEP FX-02/FX-06消费expected与错误语料。D0三卡合入后，后续ART-02才实现真实提交/持久query，ART-03才适配CLI/sync/pull；IF07-V01/02、EB05-V01/02真实运行仍保持not_run。

## 6. 现有测试入口、复用点与不能证明的事

2026-10-25 CodeGraph已核对以下文件。本轮没有执行其中任何测试或import业务模块。

| 现有入口/代码 | 可复用 | 局限与D0约束 |
| --- | --- | --- |
| `tests/test_plugin_server.py:12–39`临时目录与ThreadingHTTPServer；`:41–59`PUT测试；`:82–95`GET测试 | 后续P1本地HTTP隔离测试基架；令牌/路径/下载MIME已有断言 | PUT样例是`b"\\x00asm-built-plugin"`且只验201/字节/MD5，不是完整合法Wasm/包校验；GET样例是任意字节。D0不调用这个server证明原子发布 |
| `tests/test_upload_plugin.py:10–36`mock urlopen；`:38–46`缺文件 | 未来CLI参数/失败动作适配风格 | 仅证明请求构造与文件存在检查，不证明网络发送、ACK丢失或持久查询；03用纯状态语料明确新行为 |
| `tests/test_pull_plugins.py:10–31`已有本地文件禁subprocess下载 | 离线测试不访问registry的约束写法 | 目前样例只证明“文件已存在即复用”；不验证来源、SHA、manifest/layer选择和安全解包 |
| `generate_metadata.py:17–42`顶层遍历plugins并写metadata | 仅用于旧字段→新schema映射参考 | import即可能写目录；D0验证器禁止import此文件。旧MD5/mtime不得充当content_digest或可信来源 |
| `pull_plugins.py:43–50,152–155,165–187`首.wasm提取、已有文件跳下载、首manifest/遍历layer | 仅核对导入兼容边界及P1待改入口 | 不能直接复用为安全离线验包器；D0不调用ORAS、真实tar.extract或修改生产导入 |
| `plugin_server.py:35–45,97–107`生产metadata/PUT | 确认现存写路径与未来兼容对象 | 当前直接写覆盖与分步metadata不满足IF07，D0语料通过不能标ART原子存储完成 |
| `Makefile`的test目标 | 既有整体回归入口：unittest、py_compile、同步shell测试 | 会启动旧测试本地HTTP并生成编译缓存，sync测试另有文件操作；不是新D0全部合格证明，本轮不运行；生产构建/同步依赖与D0隔离 |

未来D0拟使用独立入口 `python3 -B -m unittest discover -s tests -p 'test_contract_artifact_*.py' -v`（文件尚未创建，当前不可声称可运行）；从本仓根执行，schema resolver仅访问锁定本地目录。原回归入口保留为 `python3 -B -m unittest discover -s tests -v` 与 `bash tests/test_sync_plugin_artifacts.sh`，在后续相应授权及实现阶段按影响选择，不因文档变动运行产品测试。D0不得把`docker-build`、`make upload`、sync生产目录或ORAS调用加入测试依赖。

## 7. 卡片完成判定与下一阶段交接

三卡均须附实际变更白名单、schema/fixture/vendor锁摘要、对应测试入口及真实结果。01/02/03当前全部not_run；缺上游common阻塞01最小出口；独立消费者尚未固定导入只阻塞01最终互验回执，不能反向阻塞已验收ART01.manifest、MF05或ART02。两层状态分别登记，不能把“文档已审查”改为schema/tests完成。

D0联合出口：一份权威ArtifactManifest schema、固定公共类型闭包、可独立核对的摘要/包/状态正负语料、可消费的expected清单和明确error/next_action。P1入口：用这些语料约束真实提交点、并发原子性、持久unknown恢复和HTTP读回，再接MF/SDK/DEP发布加载。包提交状态、运行配置生效、supported、production available始终分开；D0不验后面三项，也不改变这些责任方。


## 8. 全范围复审后的包格式与白名单补充

IF07现已分开客户端PackageDescriptor和服务端ArtifactManifest，并以[mf-json.v1](../../../ocloudmodelfusion/docs/contracts/2026-10-25-canonical-json-profile.md)固定公共字节算法。D0-ART-01额外独占拟新增`contracts/v1/package-descriptor.schema.json`、`contracts/v1/export-map.json`、`tests/contracts/export_bundle.py`，以及既有`contracts/fixtures/v1/artifact/`范围内descriptor正负语料；不改生产上传代码。01的最小manifest出口必须同时包含两种schema及其引用闭包，不能把服务端字段当作上传必填。

导出工具输入源manifest/export-map和显式临时输出目录，将schema放`v1/`、语料放`fixtures/artifact/`、源manifest原字节放包根`bundle-manifest.json`；仅复制manifest登记文件，不执行包内脚本。测试vendor导入小的完整发布包，保持根布局；manifest含根D0协议要求的bundle_format_version、validation/asset_refs及assets，辅助期望/断言/归档等均锁摘要。引用base URI与fragment均验证，未知validator拒绝。

02的有效归档根使用`package.json`，只列payload且排除自身，服务端manifest不在归档内；原档中声称committed/upload记录、重复/越界文件或未登记payload均负例拒绝。配置schema须独立Draft 2020-12，旧spec包装转换由后续ART-04负责，D0不直接import会写目录的旧生成脚本。

01公共规范资产从MF01固定包导入，vendor白名单按manifest展开增加相应fixtures/verification文件，记录逐文件清单，不能自行重新生成期望字节。02验包器与03上传语料都区分descriptor/archive输入和服务端输出；对同内容不同压缩字节的案例保持既定幂等规则。最终在仅消费仓的环境完成跨语言规范字节/完整包校验；实际HTTP、并发提交与目录注册仍not_run，归后续ART与MF-REL01。
