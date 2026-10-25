# WASM D0 执行夹具

2026-10-25：39 个输入向量已冻结并接入纯离线测试，权威传输类型来自 [批次 A lock](../contracts/contracts.lock.json)。案例文件保存预期和输入，实际结果由每次运行输出目录记录；模板中的 `actual.status=not_run` 不承载运行回执。正式公共包 559 例与本地 39 例分别计数。

| 输入 | 案例 | 执行接缝 |
| --- | --- | --- |
| `identity-body.json` | 13 | header-only 仅收正文；完整 / 三 chunk / 明确空正文；错 call、generation、release、变换正文、缺规范版本、读失败及外部头 |
| `matcher-cases.json` | 11 | 真实导出的 ParseRuleConfig / GetMatchConfig：同 DNS 双绑定、route/service 首匹配、global / 缺规则、旧 backup、before / after match |
| `callback-schedules.json` | 15 | 两个真实 SDK pending CalloutID 的正逆序、私有深拷贝、父 streamInfo 后改、旧 generation、关闭 / 重复、缺 status、201 / 202、截断 / 读失败、错 ID / 快照 |

`input_sha256` 绑定保存的 UTF-8 `input_json_utf8` 原始字节。非空 JSON 正文同时冻结 raw 与 `mf-json.v1` canonical golden，测试调用固定公共引擎逐字节复核；不同分块同摘要，改 payload 不同摘要。明确空正文只采用已声明的 `raw-bytes.v1`，不把 JSON 空字节解释为 null / {}，也不把读取失败变为空正文。MF CallIdentity、BodyBasis 与不完整发送 DTO 另经真实 Draft 2020-12 引擎验证。

`WG_FIXTURE_*` 是本地模型错误分类，不是产品 enum。matcher 的实际返回与夹具关联拒绝分别记录：旧 backup 仍能返回 config-old；after-match 场景缓存 config-A；这些事实不代表生产已有 release / selection guard。header/body 缓存模型只验证所用导出入口，真实 wrapper / Envoy 重入留待 P1 / EB。

调度 adapter 从固定 SDK HostEmulator 的真实 attributes 按唯一 upstream 捕获 ID，记录合成 101/202 到实际 ID 的映射（0 合法），显式调用指定 ID。无 sleep，不使用便利函数的首个 ID。快照深拷贝 map、byte slice 与嵌套 slice，记录不从最新父 streamInfo 重建。实际 host callback 与 `contract_fixture` 生命周期拒绝分别注明层级；重复、关闭后 SDK 能力另有直接探测。SDK 无正文读取错误注入能力，该例只作为模型验证，记录 `contract_fixture`。缺宿主能力明确 blocked。重试 / 重建禁令属于夹具设计约束，生产 retry/guard 未接线。

WG01 独占 vendor / lock / 工具；WG03 独占 loader / adapter / snapshot；WG02 只通过其接口消费数据，WG04 串行集成。旧 host、matcher、wrapper、provider retry、生产模块及部署资产均未修改。

```bash
D0_PYTHON=/tmp/modelfusion-d0-venv/bin/python GOWORK=off GOPROXY=off bash scripts/verify-d0-contracts.sh --batch A --output /tmp/wasm-d0-evidence
```

回执与限制见 [runbook](../../docs/d0-fixture-runbook.md)。输入向量不能代替可信生产 writer、消费客户端、真实上游 observer、真实 EB、IF09 正式封装或 supported 判定。
