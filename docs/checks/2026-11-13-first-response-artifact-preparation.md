# 首响应采集制品候选准备

2026-11-13，当前 dev。采集源码基于 b1df05c7776b9465923f5877c19ab6b833100bd2，新增首响应头耗时沿[采集说明](2026-11-13-first-response-telemetry.md)。当前权威 ai-proxy VERSION/catalog/spec 为2.0.80；旧option.yaml不是发行版本来源。本批推进为2.0.81，保留旧2.0.79/2.0.80归档与活动路由原引用。

共享executionfacts输入依既有source_inputs不可变规则影响46个Go发行实现，只增补丁号并保持原alpha性质；元数据仅改各VERSION、spec的info.version及catalog的对应declaredVersion。8个C++与1个Rust原制品保持。构建使用仓库固定Go1.26.3容器（toolchains.json的不可变digest），不拿临时宿主Wasm替代正式完整包，不增加业务代码、依赖、推理、价格或资金修改。

| 插件 | 原版本 | 候选版本 |
| --- | --- | --- |
| ai-agent | 2.0.7 | 2.0.8 |
| ai-billing | 1.0.16 | 1.0.17 |
| ai-cache | 2.0.29 | 2.0.30 |
| ai-context-limit | 1.0.4 | 1.0.5 |
| ai-endpoint-picker | 2.0.6 | 2.0.7 |
| ai-history | 2.0.17 | 2.0.18 |
| ai-intent | 2.0.7 | 2.0.8 |
| ai-json-resp | 2.0.7 | 2.0.8 |
| ai-load-balancer | 2.0.7 | 2.0.8 |
| ai-memory | 0.1.21 | 0.1.22 |
| ai-prompt-decorator | 2.0.7 | 2.0.8 |
| ai-prompt-template | 2.0.7 | 2.0.8 |
| ai-proxy | 2.0.80 | 2.0.81 |
| ai-quota | 2.0.13 | 2.0.14 |
| ai-rag | 2.0.7 | 2.0.8 |
| ai-search | 2.0.7 | 2.0.8 |
| ai-security-guard | 2.0.7 | 2.0.8 |
| ai-statistics | 2.0.14 | 2.0.15 |
| ai-token-ratelimit | 2.0.11 | 2.0.12 |
| ai-transformer | 2.0.7 | 2.0.8 |
| cache-control | 2.0.7 | 2.0.8 |
| cluster-key-rate-limit | 2.0.8 | 2.0.9 |
| cors | 2.0.8 | 2.0.9 |
| de-graphql | 2.0.7 | 2.0.8 |
| ext-auth | 2.0.7 | 2.0.8 |
| frontend-gray | 2.0.7 | 2.0.8 |
| geo-ip | 2.0.7 | 2.0.8 |
| gw-error-format | 1.0.6 | 1.0.7 |
| hmac-auth-apisix | 2.0.7 | 2.0.8 |
| ip-restriction | 2.0.7 | 2.0.8 |
| json-converter | 2.0.7 | 2.0.8 |
| key-auth | 1.2.7 | 1.2.8 |
| log-request-response | 1.0.4 | 1.0.5 |
| mcp-router | 2.0.7 | 2.0.8 |
| mcp-server | 2.0.7 | 2.0.8 |
| model-mapper | 2.0.9 | 2.0.10 |
| model-router | 2.0.7 | 2.0.8 |
| nginx-rewrite-compatible | 1.0.4 | 1.0.5 |
| oidc | 2.0.7 | 2.0.8 |
| opa | 2.0.7 | 2.0.8 |
| replay-protection | 1.0.4-alpha | 1.0.5-alpha |
| request-validation | 2.0.7 | 2.0.8 |
| response-cache | 1.0.7 | 1.0.8 |
| traffic-tag | 2.0.7 | 2.0.8 |
| transformer | 2.0.7 | 2.0.8 |
| waf | 2.0.7 | 2.0.8 |

固定工具链完整构建55项通过；完整包、来源策略经独立核对，本地登记库追加保留旧55项，并构建了未部署的候选PluginServer镜像。发行前备份本地登记库及原策略，原版本不覆盖；未来镜像/PVC只按原种子流程追加已审完整版本。新增目录不改变已冻结路由引用，正式Qwen/Deep须另获明确原生发布范围后才能切换。Console接收列与认证ingest应先部署；真实首响应采集仍需后续真实请求验证，不凭包构建通过关闭I22。

候选构建后，本地旧登记库副本按既有种子入口从236项追加到282项，旧236项manifest、archive摘要与artifact ID保留。验证发现既存`GET /artifacts`将整个列表交给单包JSON适配器：旧236项逐条编码已合计4,056,879字节，新282项为4,508,919字节，均触发1MiB输入限制并返回400/E_LIMIT。集合响应改为逐条沿用原canonical编码后组成同形JSON数组；不改变mf-json.v1适配器、单包限制、登记或归档契约。现有HTTP单元套件16项通过，其中新增一项覆盖大于4MiB的集合和精确数值；旧URL下载、重启与VM实际追加仍需分别留证，当前不据此宣称插件已部署或I22完成。

2026-11-13实际阶段：plugin2于Helm97正常追加236→282，旧236的manifest/archive/ID、新55全部491个inventory文件、旧59引用URL及1000个唯一HTTP载荷摘要通过，集合200/4,509,216字节，API正常启动登记55。但影响核对漏了启动协调器：全目录选择除了已审model-router，还将两条internal限流引用推进新版本，其中ai-token-ratelimit的Wasm确有变化。按已确认逆覆盖正常回退至98；当前原55选择和59引用、完整价格/provider/portal/自有原null价本文档及131迁移账本恢复，7PVC和6非Helm Secret保持。新增46和登记历史保留于282库，没有替换库或删除不可变版本；原Plugin6的集合400/E_LIMIT也如实恢复。该次异常不作为新RT样本或完整I22通过。

后续最小候选冻结[首响应选择目录](../../plugins/release/first-response-selection.json)：只选ai-proxy2.0.81，其他54项使用98原选中的不可变版本。该文件沿既有`selected_entries`元数据目录格式，供`verify-release-store.py`选择已登记完整包；不作为`build-all`的源码目录，也不修改各实现VERSION或旧包。Docker使用`--build-arg PLUGIN_RELEASE_CATALOG=plugins/release/first-response-selection.json`，默认构建仍使用原源码目录。旧54包来自原不可变image6和本地原登记库，新proxy来自已审完整包；只复制原manifest/archive/payload/ID，禁止重编旧版本。镜像/catalog endpoint/API登记使用同一生成目录，未选中的新46仍可保留在登记库及来源策略中。

新proxy的首响应观测由`managed_telemetry.go`直接冻结独立RuntimeTelemetry，再由`dispatchManagedSnapshot`写入原私有Redis/outbox，不经过旧ai-billing的重新序列化；因此最小选择可保留其他54项代码。既有导出入口已实际导出55份完整包：旧54的目录条目、manifest、archive、载荷及ID与原版本逐字节相同，新proxy与已审plugin2完整包相同，无重编或重新打包；生成目录摘要为`sha256:1082d9ec83101fe1020c214b8bfea5f1be8fc2c5c8d2c7af1ab7d7620a770087`。现有release catalogue单元测试5项通过。下一次部署前需以98完整59资源spec作为比较基线，确认model-router、两internal及其他旧引用/配置均不变；上述预构建阶段尚未构建候选或再次部署，实际自有mock调用另行核对具体范围。

2026-11-13后续：固定84da4b26经双轴及主审独立旧54/新proxy完整包核验后，从干净归档用可信ARG构建推送plugin3，镜像`V2.2.1-dev-20261113-calls-plugin3@sha256:ca2a337c7447ad22c1e6a0b0fa645c6c5c771c0193f90dfc4d0aaaaf505d479f`。实际提取55完整包546文件、inventory491条的内容/大小及manifest/archive/ID均与冻结导出相同；catalog仍1082d9ec…770087，原始selector为d95eeb95…581c，MF canonical的source_catalog_digest另为efa4aec5…12ce，不把pretty原始字节摘要当canonical绑定。策略a33f9f5b…ab5保留262条来源回执，已有新增46全部保留，其中仅proxy081成为本候选的新默认选择。

从已经282项的登记库副本启动同一plugin3，正常seed的原始返回是copied空数组、retained55项；未再追加或重跑种子。重启前后仍282项，完整manifest对象、ID和archive摘要均保持；HTTP目录与实际image字节相同，集合200/4,509,216字节。59条现有引用通过27个唯一URL逐HTTP及原manifest的Wasm摘要验证，另核proxy081；其中3个原CRD本来没有sha256，不给资源补写SHA。早期宿主复制与临时验证脚本把返回数组当计数、假定每CRD都有sha的准备失败均保留，不判为制品或产品失败。实际种子与HTTP阶段在同一副本继续，所有临时容器已清理。这是本机副本验证，VM当前99仍Plugin6/原55选择，未部署plugin3或调用mock，不据此关闭I22。
