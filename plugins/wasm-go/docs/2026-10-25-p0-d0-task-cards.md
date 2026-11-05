> 2026-11-05 迁入说明：本页保留 Go 契约夹具与任务来源，正文状态属于 2026-10-25 历史检查点。当前交付以[2.2.1 发布准备](../../../../docs/plans/2026-11-05-release-2.2.1.md)及当前源码为准；全新安装按现行契约验证。

# WASM P0/D0 可派工任务卡

> 2026-10-25任务跟踪：本页原D0卡继续为白名单权威；每卡Issue、最小片段及更新后的首批并行顺序统一登记在[派工与Issue总表](../../../../docs/plans/2026-10-25-gap-dispatch-and-issue-tracking.md)。D0完成不代表后续产品或工程边界验收完成。

更新：2026-10-25。性质：实施拆卡；4张卡均待实施、验收未运行。实际人员和日期待排期，下面owner均为唯一模块职责。本轮仅更新计划文档与相关索引，没有创建schema、测试工具、夹具、业务实现或部署资产。

承接[P0/D0派工总表](../../../../docs/plans/2026-10-25-p0-d0-dispatch-plan.md)、[接口实施计划](2026-10-25-interface-implementation-plan.md)、[EB-01/02验证计划](2026-10-25-execution-boundary-verification-plan.md)及[共享FX夹具](../../../../modelfusion-deploy/docs/plans/2026-10-25-boundary-fixtures-plan.md)。D0只交付固定契约消费者、正负语料和离线调度接缝；可信生产入口、真实选择/guard、Consume客户端、上游observer及运行发布属于后续P1，不纳入本次卡的退出条件。

## 1. CodeGraph核对与实施边界

2026-10-25先通过CodeGraph追查宿主、matcher、retry及现有契约测试，再定向读取未索引的Go配置；没有运行构建和测试。

| 现有位置 | 已确认事实 | D0采取的边界 |
| --- | --- | --- |
| [go.mod](../go.mod):1–18 | module为`github.com/higress-group/wasm-go`，Go 1.24.1；proxy-wasm SDK固定`v0.0.0-20251103120604-77e9cce339d2`，已有`invopop/jsonschema v0.13.0` | 不能因存在schema生成依赖就声称已有JSON Schema运行验证器；新验证工具使用独立测试模块，禁止为D0给生产根模块新增依赖 |
| 工作区根、higress-plugin根、wasm-go根 | 本轮定向检查均没有`go.work` | 未来命令使用`GOWORK=off`；新工具不创建跨仓go.work，也不读取邻仓`../`包。工具自身的固定依赖bootstrap与离线执行分开记录 |
| [pkg/test/host.go](../pkg/test/host.go):108–152、254–269 | NewTestHost构造proxytest宿主；便利CallOnHttpCall只取第一CalloutID，底层宿主和属性列表可用于指定ID | 优先薄adapter，禁止多个卡共同改host；正反序都必须按明确ID而非便利函数触发 |
| [pkg/matcher/rule_matcher_test.go](../pkg/matcher/rule_matcher_test.go):91–209、259–264、372–387、565–585、595–675 | 已有host/service匹配、配置解析、备份与隔离测试 | 可复用当前解析入口和测试风格；不等于已经覆盖可信selection、header/body阶段或真正Envoy重入 |
| [pkg/wrapper/plugin_wrapper.go](../pkg/wrapper/plugin_wrapper.go):1072–1092、1095–1124 | header先匹配并缓存配置，body阶段复用 | D0语料须区分before_match与after_match变更，不能设计成正文到了才假定matcher重新选择 |
| [provider/retry_test.go](../extensions/ai-proxy/provider/retry_test.go):23–27、50–52、105–125 | 多个getter返回默认，RouteCall空实现，已有测试仅覆盖提前返回 | D0建立独立调度夹具，不将旧mock的默认值/空调用当完整callback通过；不趁机修改生产retry循环 |
| [release_surface_contract_test.go](../release_surface_contract_test.go):14–57、167–230、233–245 | 读取本仓插件YAML并手工核字段/模式 | 保留资源schema职责；新IF包消费者单独实现，不把OpenAPI插件配置schema冒充IF传输契约 |

所有新路径下文均为拟新增白名单；存在的源码只读引用。D0新增工具必须显式报告`evidence_level=offline_fixture`，并用`subject_layer=contract_fixture/host_emulator`区分对象层；IF09封装使用`run_kind=offline_fixture`，真实网关EB案例保持not_run；对未实现P1能力只登记未覆盖，不能通过测试模型代替生产实现。

## 2. 导入与共享文件约定

common唯一权威是`ocloudmodelfusion/contracts/v1/common.schema.json`（D0-MF-01）。IF-01/02由D0-MF-02的execution/dispatch包发布；IF-03/05由D0-MF-03对应包发布；IF-07由D0-MF-05发布责任包提供，artifact来自plugin-server，IF-09来自deploy。WASM只消费固定包，不复制编辑另一份common、不自定公共枚举、不在测试运行时读取相邻源码目录。

本仓固定导入目录为`testdata/contracts/vendor/`，固定清单为`testdata/contracts/contracts.lock.json`。这是测试数据vendor，不使用Go vendor模式。包内相对布局、`$id`和`$ref`保留；按发布者/包名隔离目录，由lock绑定准确版本、发布源引用、包摘要、文件摘要和依赖闭包。common只装一个固定版本，其余包引用映射到该版本；冲突不自动取最高版。下载/导入是显式更新步骤，日常测试禁止网络解析`$ref`、latest、隐式生成或下载schema。修订发生在权威包，再更新lock与完整语料；不能只改本仓副本让测试通过。

manifest/lock完整格式直接服从[根派工协议§3](../../../../docs/plans/2026-10-25-p0-d0-dispatch-plan.md#3-契约包与消费协议)：含bundle_format_version、entries、fixtures的validation/asset_refs/expected及assets；lock顶层packages及其五字段保持统一。源码版本、文件hash由锁定manifest读取，不另造第二套lock字段。加载先校验manifest及文件摘要，再建立离线URN registry；缺失、错hash、重复URN、越出vendor_root的路径均拒绝，不允许网络或邻仓解析。

| 导入批次 | 必需包 | 开放的后续工作 | 缺包时处理 |
| --- | --- | --- | --- |
| A，最小首交付 | common＋MF02 execution/dispatch及其明示依赖闭包 | WG-02、WG-03、身份/回调最小离线驱动 | 只阻塞需要未交付定义的断言；允许编写明确未验证的草案语料，禁止伪造正式包摘要 |
| B，事实/余额 | MF03 IF-03/05对应包 | 对应编码及来源/准入样例检查 | 不阻塞A及WG-02/03；无包则相关case not_run |
| C，发布/证据 | MF IF-07、plugin-server artifact、deploy IF-09，各自独立安装 | 发布引用校验、FX-02正式证据封装 | 不阻塞本地离线结果；没有IF-09时结果只作本地诊断，不能冒充正式证据包 |

D0-WG-01唯一修改vendor、lock、导入说明及独立验证工具依赖文件。D0-WG-02/03只读这些文件，升级需求回交WG-01串行合入。`pkg/test/host.go`默认全部只读；若薄adapter无法使用现有导出能力，WG-03提交具体缺口及最小改动提案，经范围更新后由WG-03唯一集成，其他卡不并行碰它。本卡范围不预先授权改host。

## 3. D0-WG-01：固定版本契约导入与验证消费者

**唯一owner：WG-D0合同测试消费者。** 输入为批次A的正式包、发布清单及正负样例；硬依赖D0-MF-01和D0-MF-02，不等待其余MF/ART/DEP包。输出为可离线重现的vendor/lock、schema验证入口、导入报告及正负契约测试。映射WG-I00、IF01/02字段验证；不替代任何真实EB通过。

**白名单：**拟新增`testdata/contracts/contracts.lock.json`、`testdata/contracts/README.md`、`testdata/contracts/vendor/`（只允许清单列出的正式导入文件）；`tools/contractcheck/go.mod`、`tools/contractcheck/go.sum`、`tools/contractcheck/main.go`、`tools/contractcheck/lock_test.go`、`tools/contractcheck/schema_test.go`、`tools/contractcheck/testdata/`（验证器自身篡改负例）。现有根`go.mod/go.sum`、插件`spec.yaml`、root契约测试及权威schema不修改。

**实现与依赖选择：**工具为独立Go模块。实施时先确认权威包声明的schema draft、远程引用禁用接口和格式校验要求，选择能够完整支持该draft的验证器，固定精确依赖版本与go.sum，并用上游包的正负样例验证；此项未完成前不得称工具bootstrap完成。不要用自写有限字段检查器冒充完整schema引擎。bootstrap允许按显式固定依赖准备本地缓存；日常离线验收用`GOPROXY=off GOWORK=off`，缺依赖缓存报告bootstrap阻塞，不静默联网。保留Go 1.24.1兼容性，工具不得提高生产模块版本。

**未来验收命令**（当前工具不存在，均未执行；工作目录为`tools/contractcheck`）：

```bash
GOWORK=off GOPROXY=off go test -count=1 ./...
GOWORK=off GOPROXY=off go run . verify --lock ../../testdata/contracts/contracts.lock.json --batch A
```

`verify`及后文子命令是本卡拟实现CLI，不宣称已有命令。断言：同一包原字节可通过；改一字节、错误包摘要、缺common、依赖冲突、遗漏必需字段、错误digest/revision类型、尝试网络`$ref`均非零退出且定位package/schema/fixture；负例必须匹配预定错误分类，不能因路径不存在就算正确拒绝。仅调整并已冻结的scope可跳过B/C；显式请求B/C却缺包须失败或报告blocked，不能成功退出零用例。

**退出/阻塞：**批次A全部正式文件可校验、正确拒绝完整负例且离线运行可重现后，WG-01最小卡完成，B/C作为独立增量；验收摘要交WG-02/03。common或MF02版本未发布是输入阻塞，工具开发和验证器自测可先行；draft能力不足由本owner解决，不要求业务先实现。

## 4. D0-WG-02：CallIdentity/BodyBasis与matcher输入语料

**唯一owner：WG-I01执行协调测试。** 输入为批次A、EB01-V01/V02/V03/V06的固定断言及当前matcher语义。输出为机器可读输入/期望清单、阶段顺序和关联反例；只验证语料与已有matcher/测试适配，不实现生产CallIdentity、BodyBasis或目标选择。与WG-03并行；语料合入依WG-01批次A，Go测试接入另依WG-03的最小loader接口先合入，不等待全部调度测试。

**白名单：**拟新增`testdata/execution/identity-body.json`、`testdata/execution/matcher-cases.json`、`testdata/execution/README.md`；`internal/executionfixture/identity_fixture_test.go`、`internal/executionfixture/matcher_fixture_test.go`。测试所需共享加载接口消费WG-03定义，不自行新增第二套loader或修改host；现有matcher/wrapper/ai-proxy全部只读。此处fixture元数据是测试格式，不新增产品schema权威。

**最小正负集合：**header仅有CallIdentity时允许继续收body但不能构造完整发送DTO；三chunk完整原body、明确空body各有正确BodyBasis；body未结束、错call、旧Context generation、旧release、变换后body冒充原body、缺规范化版本均有独立预期拒绝。原正文摘要与outbound摘要分开；两阶段只为实现适配，不降低IF-01传输必填。matcher输入覆盖单目标、同DNS不同绑定、route/service首匹配、global/缺规则、旧backup以及before_match/after_match变更。既有matcher能返回旧配置时保存实际返回与预期差异，不能把测试输入校验当生产已拒绝旧选择。

**未来验收命令**（第一条目录`tools/contractcheck`，第二条目录wasm-go）：

```bash
GOWORK=off GOPROXY=off go run . fixtures --lock ../../testdata/contracts/contracts.lock.json --cases ../../testdata/execution/identity-body.json
GOWORK=off GOPROXY=off go test ./internal/executionfixture ./pkg/matcher -run 'TestD0Identity|TestD0Matcher|TestHostMatch|TestParseOverrideConfig|TestRuleLevelConfigIsolation' -count=1
```

断言：每个fixture有唯一稳定case ID、输入摘要、预期阶段/错误分类和来源IF/EB；负例不能只少一个无关字段导致提前失败；同body不同分chunk得到同原始摘要；改变payload必须改变摘要；读取原body失败不得被语料解释为合法空body。已有matcher结果与阶段输入分别记录；真正生产冻结/拒绝仍留P1/真实EB未运行。

**退出/阻塞：**最小集合可被A包接受/拒绝、测试有确定行为且所有P1未覆盖项显式标记即完成。matcher私有接缝无法在外部测试访问时不复制生产算法；可改为只覆盖导出入口，剩余点记录P1接缝依赖。增加同包测试文件须先更新白名单，不能临时改生产导出符号。

## 5. D0-WG-03：显式CalloutID与私有快照调度夹具

**唯一owner：WG-I02派发客户端测试。** 输入为EB01-V05/EB02-V02/V04、批次A身份/结果样例、固定SDK宿主接口。输出为仅测试使用的薄adapter、不可变快照样例和可重复callback调度；可在正式包到达前写明确标草案的接缝自测，发布合入依WG-01。与WG-02无实现先后依赖。

**白名单：**拟新增`internal/executionfixture/loader.go`、`internal/executionfixture/host_adapter.go`、`internal/executionfixture/snapshot.go`、`internal/executionfixture/scheduler_test.go`、`testdata/execution/callback-schedules.json`。共享loader/adapter/snapshot由本卡独占；仅测试包及D0驱动可引用，生产插件不得import此包。既有`pkg/test/host.go`、`provider/retry_test.go`和生产retry不修改。

**接缝最小职责：**从真实模拟宿主读取CalloutID并登记；指定ID调度callback；深拷贝发送前call/attempt/context_generation/selection/body摘要，返回可追溯snapshot ID；支持明确barrier和释放顺序，无后台sleep碰概率。生命周期与IF DTO关联检查只在夹具中实现，不伪装为生产guard。包装已有TestHost底层能力；若接口无法表达重复/关闭后回调，输出`unsupported_in_emulator`并按对应layer记未覆盖，不编造宿主成功回调。

**未来验收命令**（目录wasm-go）：

```bash
GOWORK=off GOPROXY=off go test ./internal/executionfixture -run 'TestD0Callout|TestD0Snapshot|TestD0Schedule' -count=1
(cd extensions/ai-proxy && GOWORK=off GOPROXY=off go test ./provider -count=1)
```

断言：至少两个pending callout，正序/反序都正确关联；收到B回调时不得消费A；后改可变map/byte slice不改变已捕获A快照；同Context编号不同generation不能混用；重复callback不产生第二份模拟主记录；缺status仍保持缺失，201/202/截断为不同样例。明确区分调度模型拒绝与真实SDK行为，所有动作日志记录来源层。已有provider回归只证明未破坏旧测试，不证明新retry实现正确。全局宿主状态用串行测试/cleanup隔离，不对依赖同一全局VM的case调用`t.Parallel()`。

**退出/阻塞：**正反序与快照拷贝自测可重现、host模拟能力缺口清单完整即完成。无法模拟的真实宿主事件只阻塞对应EB运行，不要求D0修改Higress核心；不能用最终共享streamInfo回填私有结果来通过。

## 6. D0-WG-04：最小离线驱动及结果交接

**唯一owner：WG-D0离线集成。** 输入为WG-01批次A、WG-02语料及WG-03adapter；输出为单命令运行入口、案例计数、逐层结果和FX交接材料。硬依赖WG-02/03完成最小稳定接口；正式IF-09证据封装另依批次C，不阻塞离线驱动本身。

**白名单：**拟新增`scripts/verify-d0-contracts.sh`、`internal/executionfixture/driver_test.go`、`docs/d0-fixture-runbook.md`。结果写命令显式传入的临时输出目录，不提交伪造运行报告；本卡不得改WG-01的工具/lock，或WG-03共享loader/adapter。需要新能力通过各owner先合入，再由本卡串行接入；不另建schema验证器或FX-02验证器。

**未来验收命令**（目录wasm-go，输出目录须由运行者预先选择）：

```bash
GOWORK=off GOPROXY=off bash scripts/verify-d0-contracts.sh --batch A --output /tmp/wasm-d0-evidence
```

脚本接口由本卡实现；不得默认读取相邻仓库或真实环境。输出记录锁文件/语料/工具摘要、当前提交及工作区变化、层级、case数、失败与未覆盖清单、实际命令和退出状态。固定两次同输入的业务断言结果一致，run ID/耗时差异不比较；空测试集、拼错`-run`导致零测试、前序命令失败但后序覆盖退出码、未跑而写passed都必须被驱动负例捕获。故意错lock、丢fixture、错CalloutID应产生可定位失败，缺bootstrap缓存或未交IF包应明确阻塞。

正式证据一旦采用IF-09，调用FX-02共享封装/校验器固定版本，验证引用闭包和实际文件摘要；没有共享工具时保留本地诊断，不复制协议或宣称证据验收完成。D0通过只能说明离线资产可用；真实EB、P1执行能力、supported与发布状态均保持原状态。

**退出/阻塞：**批次A单命令可离线运行全部最小case并正确汇总失败/未覆盖，且纯本仓独立checkout可用；随后交WASM执行owner进入P1排期。IF-09缺失只阻塞正式证据导出；真实网关、钱包、供应商桩未就绪不阻塞D0工具退出。

## 7. 合入顺序与验收责任

顺序为WG-01批次A最小导入→WG-02和WG-03分别合入→WG-04串行集成；WG-01批次B/C可独立增量，不进入A的关键路径。各卡预先只按自己的白名单编写，公共接口需求先由WG-01或WG-03合入后再消费；不采用多分支同时改共享host、lock或loader再靠最后解决冲突。

D0-WG-01 owner裁定导入完整性，WG-02 owner裁定语料与matcher观察，WG-03 owner裁定宿主adapter能力，WG-04 owner裁定离线汇总；正式EB case结果由DEP-VERIFY裁定，WASM执行owner负责EB-01工程合流，CON-REL负责可发布组合。这些层次不可互相覆盖。实际case不变量违反为failed；缺工具/包/采集无法判断为blocked；未执行为not_run。夹具负例正确识别仅说明夹具通过，不将其指向的生产缺陷改为passed。

本轮仅完成上述拆卡与相对链接检查，所有未来命令均未执行。后续实施应按仓库AGENTS对实际符号变更进行影响分析，并重新核对工具链/依赖解析结果；不把本次静态证据当未来运行环境事实。


## 8. 全范围复审后的消费格式补充

WG-01从显式导出包导入，包根固定`bundle-manifest.json`，落到lock指定的`vendor_root/bundle-manifest.json`，其余文件保留包内路径；按完整小包导入全部entries/fixtures/assets，不能裁剪defs或只复制schema仍声称原包已验证。白名单仍是WG-01的vendor子树，导入PR须附manifest展开后的准确文件列表。

WG-01加载器必须实现根协议的base URI＋fragment解析、选择器/预期阶段、辅助assets完整性和版本化validator分派；未实现validator的案例显式blocked，禁止默默只跑shape并宣布整卡完成。WG-02/03提供自己语料的validation描述，WG-04只消费已锁工具。公共规范字节golden来自MF01，不能用Go默认序列化替代mf-json.v1；所有辅助文件改一字节均要被锁检查发现。

新增验收：导出包源仓移除后，单独消费仓、无网络仍能运行指定scope全案例；未知fragment/validator、漏tar或expected、NDJSON选择错误均按指定阶段失败。WG-I08/I09及BODY真实接线仍是后续产品任务，本批只补已有卡的契约语料，不新增D0业务实现或真实宿主结论。
