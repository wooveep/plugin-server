# 具体 Key 路由授权与生产调用期限候选

2026-10-31。行为提交 `716455b2`、`70b0ee74`，quota最终文档/来源提交 `ce05e989`，其余包按最后触及自身输入的70提交保留来源。五个冻结候选已完成原边界接纳、Helm111部署及Console登记，见[实际交付](#helm111实际交付)。不能从本批通过推导整个业务链已去耦或 E1 已通过。

quota 在现有一次 Redis 只读脚本中检查具体 Key 的 route grant，并使用稳定账户地址和当前 control/account/mapping 同代校验。旧 Lua 在真实 Redis 上对未授权 Key/其他路由/缺失或损坏授权列表共5例错误放行；候选已转绿。配套 Console 源码为 `c84f7219`，其同事务 revision/outbox 和独立钱包恢复见 MF 的原生候选报告。

普通生产在 Redis 准入后只保留原 call ID 和业务时间，不再签发临时 Console 五秒窗口。明确带 bounded permit 的验证继续检查原到期时间，internal 发送仍须先建立其真实有限许可。Proxy 正文、准备、发送及重试不把未携带临时期限的 production 判作过期；仍受已有上传/上游/重试预算约束。没有续约或延长期限路径。

原合法 headers/body host 回调反例等待6秒后，从403/0次发送转为1次上游 callout且无本地拒绝；另一个聊天在固定70提交上用同一断言独立转绿。共用执行身份包也通过。此处是实际 Wasm SDK host callback，不是在线 Envoy/外部上游证据。

本批5个实际消费者统一固定工具链构建、准备完整包，并用各自实际编译文件显式设置 WASM_FILE_PATH，整包 Go 与支持的 Wasm 宿主用例通过：quota 2.0.6约24秒、proxy 2.0.12约420秒、billing 1.0.2约69秒、cache 2.0.5约70秒、memory 0.1.8约39秒。quota 实际 Redis 环境已配置，未以跳过代替通过。各测试使用各自规定的 Go/宿主边界，不声称每个 Go 用例都在 Wasm 模式重复。源码、工具链、Wasm/完整包/日志摘要见[候选清单](production-admission-20261031/candidates.json)。

准备过程曾因 Go 测试自动生成的 wasm-unit-test.wasm 落入源码目录而被既有来源检查拒绝；已将本次生成物移到 build/test-cache，并显式指定正式编译产物运行宿主测试。未放宽完整包的来源/不可变校验。新包仍只在准备边界；后继接纳只做原上传边界的完整验证，不给列表、选版本或发布追加重复验包。

健康429、待确认outcome、本地veto和半开Claim/Consume、媒体专用派发等 Console 耦合仍开放；本批仅证明上述授权/期限及相关消费者。继续按原scope/epoch/撤销及实际发送/成本规则收敛，不能用健康普通目标通过掩盖特殊路径。

## 容器构建的宿主依赖补正

后继 `a58f15ce` 将默认固定 Go 容器使用的两项缓存归入现有 build/cache，不再先执行宿主 go env；--host 仍使用明确选择的宿主工具链。在只提供 Git/Docker 的 PATH 下，实际固定容器重编 quota2.0.6 成功，输出 Wasm 与上列候选逐字节一致。见[执行摘要](production-admission-20261031/container-no-host-go.json)。测试前后的原候选 Wasm/receipt 已成对保留并恢复；该结果不是新包接纳。本次部署使用原冻结包及其原来源，没有将候选来源改写为a58或后继提交；未来按新构建脚本重新发包时须生成对应的新receipt与完整包。

## Helm111实际交付

镜像服务器源码固定为 `675d5e0a9c5de655d8e360e57511759d7e7b4059`；五个包继续分别绑定原ce05/70来源。按候选清单逐字检查Wasm、完整包、review与来源输入，并从原提交核对1052个不同输入blob。可信policy由原55条来源绑定扩为60条，11项环境引用保持；通过既有 `package-plugins.py ingest` 完整接纳边界，原79条登记保持，新增5条。没有将未接线的健康runtime草稿编译或复制进本次镜像。[来源核对](production-admission-20261031/deployed/qualification-proof.json)、[接纳摘要](production-admission-20261031/deployed/ingest-summary.json)。

实际镜像为 `registry.lc.wooveep.com:9980/modelfusion/plugin-server:native-admission-20261031-675d5e0a-61f5dc4549c1@sha256:596ff60f56a40f2e18d24813888efdbda36c0d4b6690192b6ce89bfa19b2d138`，见[固定构建回执](production-admission-20261031/deployed/image-build-receipt.json)。沿原runtime-chain Chart及 `--reuse-values`，[服务端预演](production-admission-20261031/deployed/dry-run-summary.json)80对象只改变plugin-server Deployment；Gateway保持1副本、UID保持，16个Deployment可用。

实际init新增5、保留79；PVC/PV绑定保持。部署前794个旧payload文件的大小与SHA、79份原manifest及两条retention记录全部保持，部署后843文件、84个有效登记版本。被删除的两版uploaded-directory-fixture没有复活，manifest/Wasm共4次HTTP404。新5包的29个原文件经实际集群服务HTTP回读全部200，大小与摘要匹配。见[部署/存储核对](production-admission-20261031/deployed/runtime-verification.json)、[实际HTTP字节](production-admission-20261031/deployed/artifact-http-verification.json)。

随后通过Console原生登记接口分别使用保留的原幂等key登记ai-billing1.0.2、ai-cache2.0.5、ai-memory0.1.8、ai-proxy2.0.12、ai-quota2.0.6，catalog_revision为78–82。55类正式目录的新5默认版本及精确artifact引用匹配，详情/路由schema/英文README共15次200，已删夹具仍不可选。[Console登记与读取回执](production-admission-20261031/deployed/console-registration-verification.json)。旧冻结发布继续保留其原版本；当前媒体worker仍显式绑定2.0.10，本批不声称已完成全部媒体切换。

同批MF与两端Web的Helm110、原Recovery ready/rev13和实际Redis/PG核对见[MF权威记录](../../../ocloudmodelfusion/docs/checks/2026-10-31-native-catalog-deployed.md)。健康429/半开恢复、待确认outcome与媒体派发路径继续实施，原79卡仍需逐项业务验收。
