# WASM P0/D0 夹具草稿

2026-10-25：本目录只完成 D0-WG-02/03 的合成数据准备与数据完整性自检。39 个案例均为 `draft`；`domain_validation`、宿主模拟执行及真实网关验证全部 `not_run`。`expected` 的 accept/reject 是未来夹具预期，不能据此关闭 [D0-WG-02](https://github.com/wooveep/wasm-go/issues/9)、[D0-WG-03](https://github.com/wooveep/wasm-go/issues/4) 或任何 EB / P1 / supported 状态。

本目录没有实现 loader、adapter、schema 验证器、公共 runner、生产身份/选择/guard、Go 薄适配或部署。本轮没有改动已有文件。范围依 [本仓 D0 卡](../../docs/2026-10-25-p0-d0-task-cards.md) 与 [根包协议](../../../../docs/plans/2026-10-25-p0-d0-dispatch-plan.md)。

## 交接信息

| 项目 | 记录 |
| --- | --- |
| WG02 唯一模块责任 | WG-I01 执行协调测试 |
| WG03 唯一模块责任 | WG-I02 派发客户端测试 |
| 本轮代班 | Codex agent |
| 自然人 / backup / 排期 | 待确认 / 待确认 / 待确认 |
| 实际分支 | `codex/cache-auth-history-20261023` |
| 读取时 HEAD | `99a51c9e1e8a0f189f0d277628a5c1e3d41c8dd1` |
| 状态 | 数据准备就绪；D0 卡仍待正式输入与接线验收 |
| 输入引用 | MF01 common、MF02 identity/body/outcome 与 WG01 batch-A/loader 的精确版本、manifest 摘要和回执均待冻结；JSON 中保持 null，不伪造锁 |
| WG02 后继 | MF02 冻结定义 → WG01 固定导入 → WG03 loader 最小接口 → WG02 映射 DTO 并执行 shape/semantic 与 matcher 观察 |
| WG03 后继 | WG01 固定导入 → WG03 私有快照和指定 ID 薄适配 → 探测宿主模拟能力并运行调度 → WG04 串行集成 |

已存在的 `docs/README.md`、任务卡、`extensions/ai-billing/VERSION`、`extensions/ai-load-balancer/` 等工作区改动保持原状；本轮不接管这些文件。WG01 独占 vendor / lock / contractcheck 依赖；WG03 独占 loader / adapter / snapshot；WG04 独占驱动。`pkg/test/host.go`、matcher / wrapper / retry 业务源码只读。

## 数据与边界

| 文件 | 稳定 case ID 前缀 | 案例数 | accept / reject 预期 | 主要输入 |
| --- | --- | --- | --- | --- |
| [identity-body.json](identity-body.json) | `D0-WG02-ID-` | 13 | 5 / 8 | header 未齐正文、三 chunk / 单 chunk / 明确空正文、未结束 / 错 call / 旧 generation / 旧 release / 变换正文冒充原文 / 缺规范版本 / 原文读取失败 / 改 payload / 外部头来源 |
| [matcher-cases.json](matcher-cases.json) | `D0-WG02-MATCH-` | 11 | 7 / 4 | 单目标、同 DNS 双绑定、route/service 首匹配、global / 缺规则、旧 backup、before_match / after_match 修改 |
| [callback-schedules.json](callback-schedules.json) | `D0-WG03-CB-` | 15 | 8 / 7 | 两 pending 正逆序、同 call 双 attempt 的父 streamInfo 后改 / A 迟到、未知 ID、map / byte slice 后改、旧 generation、关闭后 / 重复回调、缺 status、201 / 202、截断 / 读错、跨 ID 错快照 |

`input` 是本目录的场景记法，不是尚未冻结的 IF DTO。所有 `WG_DRAFT_*` 错误、selection/call/revision 标签和阶段值均为本地草稿记法，不建立公共 enum 或 schema 权威。`validation.validator_id` 仅为拟议的版本化接口名，明确 `proposed_not_implemented`；须由后继 owner 审核、注册并锁定后才能执行，未知 validator 应报告 blocked。不得把本目录直接包装为已通过的正式 bundle。

`input_json_utf8` 保留 `input` 的确切 JSON 文本，`input_sha256` 只计算该字符串的 UTF-8 字节；自检同时要求解析结果等于 `input`。正文 `raw_sha256` / `body_raw_sha256` / `outbound_raw_sha256` 只针对已保存的原文字节。它们不是 mf-json.v1 规范摘要、业务 BodyBasis 规范摘要、包摘要或运行镜像证据。正文规范化版本待 MF01/MF02 冻结；本轮未复制 common，未创建 lock / vendor / manifest。

header-only 正例仅允许继续收正文，完整发送 DTO、Consume 和派发仍禁止。正文案例证明的是输入向量内部一致性；传输必填项仍须后续正式 schema 验证。同一正文不同分块保存同一原文字节摘要，改 payload 得到不同摘要，变换后 outbound 单列。读取失败不能解释为合法空 body。

matcher 的 `current_matcher_static_prediction` 是基于当前源码的预测，`runtime_observed=false`，`actual.status=not_run`。after_match 改为 B 的案例保存“已缓存 config-A”与“应拒绝错配”的差异；旧 backup 同样保留可能返回旧配置的事实，不能把预期 guard 拒绝写成现有生产能力。

回调中的 101 / 202 / 999 是明确的合成 CalloutID。真正运行时，WG03 必须先从模拟宿主的 callout attributes 捕获实际 ID，断言至少两个 pending，再登记合成 ID → 实际 ID 的绑定并指定 ID 释放；不能默认第一个 ID。`captured_host_callout_id=null` 表示本轮没有捕获宿主 ID。事件以 `both-pending` barrier 起步，后续顺序显式列出，没有 sleep。旧 generation、关闭后和重复回调目前仅有夹具预期；模拟宿主不支持时必须记录 `unsupported_in_emulator`，不能虚构 callback 成功。缺 status 保持 null；201、202、截断与读取失败分别保存，不造 502、不凭未知重建任务。

## CodeGraph 静态核对

2026-10-25 对当前仓库的 `.codegraph/` 直接调用 `codegraph_explore`，读取当前源码与调用边；无源码改动，未运行构建或 Go 测试。旧 `graphify-out/GRAPH_REPORT.md` 记录基线 `35114e9a`，与上述 HEAD 不同，仅作历史导航；下表依据此次 CodeGraph 返回的当前源码。静态路径不证明宿主 / Envoy / ABI 实际运行。

| 源码接缝 | 当前静态事实 | 对本数据的约束 |
| --- | --- | --- |
| [host.go](../../pkg/test/host.go):29、108、172、254、268 | TestHost 嵌入 HostEmulator；便利 `CallOnHttpCall` 取 `attrs[0].CalloutID`；可取得 attributes，已有底层 `CallOnHttpCallResponse(calloutID, ...)` 调用 | 两 pending 必须由 WG03 按实际捕获 ID 指定释放，不能用便利函数证明逆序 |
| [plugin_wrapper.go](../../pkg/wrapper/plugin_wrapper.go):1072、1080、1095、1115 | header 获取并缓存匹配配置；body 使用 `ctx.config`；非 streaming body 在 endOfStream 前暂停 | 区分 before_match 与 after_match；正文收齐和 header 身份分阶段，不能假设 body 自动重选 |
| [rule_matcher.go](../../pkg/matcher/rule_matcher.go):127、140、176、258、404 | 按规则序首匹配；global 回退或 nil；隔离模式解析失败可载入 backup；service 从 cluster 的四段格式读取 port 与 fqdn | 同 DNS 双绑定使用 route / service 区别；记录旧配置预测与未来 guard 差异 |
| [retry_test.go](../../extensions/ai-proxy/provider/retry_test.go):23、50、105 | mock 多个 getter 返回默认，RouteCall 空实现；已有测试主要验证提前返回 | 不能以旧 mock 通过替代私有快照或逆序回调证据，不修改生产 retry |

查询范围：`pkg/test/host.go NewTestHost CallOnHttpCall GetCalloutAttributes CallOnHttpRequestHeaders CallOnHttpRequestBody wrapper OnHttpRequestHeaders OnHttpRequestBody matchConfig pkg/matcher/rule_matcher.go MatchConfig`；随后精确查询 `testHost.CallOnHttpCall testHost.GetHttpCalloutAttributes testHost.CompleteHttp testHost.CallOnHttpStreamingRequestBody serviceMatch` 与 `extensions/ai-proxy/provider/retry_test.go mockHttpContext RouteCall TestPerformRetry`。

## 已执行的数据完整性自检

cwd：本仓根目录。下面是一段一次性、只读的数据检查，完整检查 JSON、非空正负集合、39 个唯一稳定 ID、实际输入字节摘要、引用、正文分块向量和两 pending/barrier；它不实现 matcher / 调度算法，也不运行领域 schema 或预期拒绝逻辑。2026-10-25 输出如下，退出码 0：

```text
identity-body.json: 13 unique cases, 5 accept / 8 reject; JSON and input-byte integrity passed
matcher-cases.json: 11 unique cases, 7 accept / 4 reject; JSON and input-byte integrity passed
callback-schedules.json: 15 unique cases, 8 accept / 7 reject; JSON and input-byte integrity passed
PASS: 39 draft cases; local references, raw-byte hashes, body chunk vectors, explicit callout IDs and barriers are internally consistent.
Scope: data integrity only. Domain schema, semantic validator, host emulator and real gateway remain not_run.
```

复现命令（不需要 Go 依赖、相邻仓库、网络或服务）：

```bash
python3 - <<'PY'
from pathlib import Path
import json, hashlib
root=Path('testdata/execution')
expected={'identity-body.json':(13,5,8),'matcher-cases.json':(11,7,4),'callback-schedules.json':(15,8,7)}
all_ids=set(); docs={}
for file,(count,positive,negative) in expected.items():
    doc=json.loads((root/file).read_text(encoding='utf-8')); docs[file]=doc
    assert doc['status']=='draft' and doc['domain_validation']=='not_run'
    assert doc['host_emulator_validation']=='not_run' and doc['real_gateway_validation']=='not_run'
    assert all(ref['manifest_sha256'] is None for ref in doc['input_refs'])
    assert doc['validation']['registration_status']=='proposed_not_implemented'
    assert len(doc['cases'])==count
    accept=reject=0
    for item in doc['cases']:
        assert item['case_id'] not in all_ids; all_ids.add(item['case_id'])
        assert json.loads(item['input_json_utf8'])==item['input']
        assert hashlib.sha256(item['input_json_utf8'].encode('utf-8')).hexdigest()==item['input_sha256']
        assert item['domain_validation']=='not_run' and item['actual']['status']=='not_run'
        assert item['source_refs'] and item['expected']['stage']=='fixture_semantic'
        if item['expected']['verdict']=='accept':
            accept+=1; assert item['expected']['error_code'] is None
        else:
            reject+=1; assert item['expected']['verdict']=='reject' and item['expected']['error_code'].startswith('WG_DRAFT_')
    assert (accept,reject)==(positive,negative),(file,accept,reject)
    print(f'{file}: {count} unique cases, {accept} accept / {reject} reject; JSON and input-byte integrity passed')
id_doc=docs['identity-body.json']; samples=id_doc['samples']; byid={c['case_id']:c for c in id_doc['cases']}
for sample in samples.values():assert hashlib.sha256(sample['raw_utf8'].encode('utf-8')).hexdigest()==sample['raw_sha256']
for item in id_doc['cases']:
    for key in ('body_sample','basis_sample'):
        ref=item['input'].get(key)
        assert ref is None or ref in samples
    for key in ('same_original_digest_as','different_original_digest_from'):
        if key in item['expected']:assert item['expected'][key] in byid
for cid in ('D0-WG02-ID-002','D0-WG02-ID-003','D0-WG02-ID-004','D0-WG02-ID-012'):
    item=byid[cid]; raw=''.join(item['input']['chunks_utf8'])
    assert raw==samples[item['input']['body_sample']]['raw_utf8']
    assert hashlib.sha256(raw.encode('utf-8')).hexdigest()==item['expected']['raw_body_sha256']
assert byid['D0-WG02-ID-002']['expected']['raw_body_sha256']==byid['D0-WG02-ID-003']['expected']['raw_body_sha256']
assert byid['D0-WG02-ID-002']['expected']['raw_body_sha256']!=byid['D0-WG02-ID-012']['expected']['raw_body_sha256']
match=docs['matcher-cases.json']
for item in match['cases']:
    assert item['input']['request_binding'] in match['bindings']
    assert not item['current_matcher_static_prediction']['runtime_observed']
    for mut in item['input']['mutations']:assert mut['request_binding'] in match['bindings']
sched=docs['callback-schedules.json']
for snap in sched['snapshots'].values():
    for data,digest in (('body_utf8','body_raw_sha256'),('outbound_utf8','outbound_raw_sha256')):
        assert hashlib.sha256(snap[data].encode('utf-8')).hexdigest()==snap[digest]
for item in sched['cases']:
    inp=item['input']; ids={p['synthetic_callout_id'] for p in inp['pending']}
    assert len(ids)==2 and all(p['captured_host_callout_id'] is None for p in inp['pending'])
    assert all(p['snapshot_ref'] in sched['snapshots'] for p in inp['pending'])
    bars={b['barrier_id']:b for b in inp['barriers']}
    assert inp['events'][0]=={'action':'barrier','barrier_id':'both-pending'}
    for bar in bars.values():assert set(bar['requires_pending_ids'])==ids
    for event in inp['events']:
        if event['action']=='callback':
            assert event['response_ref'] in sched['responses']
            if event['synthetic_callout_id'] not in ids:assert item['expected']['error_code']=='WG_DRAFT_CALLOUT_UNKNOWN'
        if 'snapshot_ref' in event:assert event['snapshot_ref'] in sched['snapshots']
    assert not item['host_emulator_expectation']['observed']
print(f'PASS: {len(all_ids)} draft cases; local references, raw-byte hashes, body chunk vectors, explicit callout IDs and barriers are internally consistent.')
print('Scope: data integrity only. Domain schema, semantic validator, host emulator and real gateway remain not_run.')
PY
```

Go 类型检查、指定用例及完整测试套件没有运行：本轮只交数据，loader / adapter 尚未实现，正式 batch A 尚未导入。后继命令以原 D0 卡为准，不能把本次数据自检替换为该卡的退出验收。
