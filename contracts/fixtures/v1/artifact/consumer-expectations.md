# ART 消费者交接

固定导入 ART bundle 和独立的 mf-common-2026-10-25.3，两项在同一 lock 中，使用原字节 manifest 摘要；完整保留 entries/fixtures/assets。不能按 imported_schema_ids 裁剪包，不能联网补 ref，不能读取邻仓浮动工作树。ART01 的最小固定版本为 art-contracts-2026-10-25.1；02、03 串行追加发布新版本，旧固定包保持不变。

客户端归档中的 package.json 是 PackageDescriptor；服务端提交回执另存，不能塞回原归档，也不能让客户端填写 artifact_id/created_at/by/download_url。归档摘要与内容摘要分开；同内容不同压缩取第一次 artifact/archive 结果。unknown 先查原 upload_id 或原版本加内容身份，不能换内容或新键试探。

`readback/expected.json` 的 @artifact-manifest 是逻辑读取对象，其他路径是清单声明 payload；不猜尚未定义的实际 HTTP API。fixture-a/fixture-b 以及观察中的 HTTP 200 均为合成资料。expected 必须非空、含外部服务端 manifest 和所有 payload；每个预先固定实例都要取得成功状态及正确摘要。真实 HTTP、多实例采集与网关加载留 P1/DEP，仍 not_run。

Console 登记失败只重试同登记意图，保留 ART 包；artifact committed 不授予 ready 或 available。SDK 固定版本/引用，不能 latest 回退。旧 release、媒体任务、历史与回滚引用未清或删除 outcome unknown 时默认 retain。

`lifecycle-cases.json` 固定 21 条状态轨迹，含 6 个类型正确的非法转换；`readback/cases.json` 固定 8 条完整性正反例。引用逐一锁原字节。测试 oracle 只解释有限纯数据事实，未实现服务器持久状态机。
