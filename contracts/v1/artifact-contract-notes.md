# ART D0 离线机器契约

权威逻辑规格为 IF-07；本文件只说明 D0 测试输入与可复核边界。`schema_version` 固定 `artifact.v1`，内容投影为 `artifact-json.v1`，公共字节算法直接使用固定 MF 工具中的 `mf-json.v1`，没有第二份近似实现。

| IF-07 字段 | schema 路径／规则 |
| --- | --- |
| 客户端描述 | `package-descriptor.schema.json`：严格只含 11 个内容字段；归档根为 `package.json`，不列自身 |
| 服务端回执 | `artifact-manifest.schema.json`：引用客户端定义，加 artifact_id/content_digest/archive_digest/created_at/created_by/download_url；客户端不能指定这些字段 |
| ID / 时间 / 摘要 / 文件大小 | 固定 common 的 Id / Time / Digest / Revision；大小为十进制字符串 |
| files / 选定文件 / 文档 | shape 校验路径与角色；独立域 helper 校验 path/env 名唯一、恰一 Wasm、选定引用及文档完整性；额外 config_schema 仅由包内本地引用闭包使用 |
| source.local_build | repo/40 位 commit/dirty_digest/build_inputs_digest 必填 |
| source.oci_import | original_ref/manifest_digest/layer_digest 必填；不选择首个任意 layer |
| source.legacy_unknown | verification 必须 unknown，不伪造 verified 来源 |
| 内容身份 | 只投影全部 11 个内容字段，files 按 path、environment_refs 按 name、documentation_paths 按路径排序，再调用公共 mf-json.v1；服务端字段不参与 |

`canonical/base.json` 是实现前物化的独立 ASCII golden 字面量，无换行。Unicode、精确数字、重复键及资源限额向量完整保持 common 包原字节，不重新生成期望。机器包只使用 json_schema、d0-assertions.v1、mf-json.v1。原语语料证明其明示检查；完整域投影、归档、状态与读回动作另由本仓测试真实执行，不能把原语接受冒充生产能力。域 content_digest 错误报告 canonical 阶段；包内同一固定摘要的独立 eq 不变量使用 semantic 阶段，其 case_id 分开登记。

测试工具固定源为 MF 提交 `753704b29a6ea40815d45dcbbce16cc0d91bace2`，逐文件摘要见 `tests/contracts/tool-manifest.json`。公共输入为 `mf-common-2026-10-25.3`，manifest 原字节 SHA-256 为 `09d0906d9f09b04f24266dee6bfab05785c5245373cc222612ec5b3b11e036f5`。生产镜像与生产依赖未改变。工具/元 schema 副本仅存在测试目录，ART 机器包不带代码；common 是独立锁项，不在 ART 包重复登记 URN。

离线验包器只读取内存，不 extract，不写 plugins、不调用 upload/pull/ORAS。归档、tar 展开、成员数、成员大小及 payload 总大小限制均来自冻结 fixture policy，等值与超过边界分别执行。超过 archive 预算时明确省略无法完整计算的摘要，绝不把部分字节摘要当完整 archive_digest。I/O 或测试依赖故障报告 execution_error。

Wasm 限定本轮惯用的 `empty-core-custom-sections-v1` 惰性夹具：校验 magic/version、完整 LEB128/section framing/custom name UTF-8，遇非空标准 section 显式 ART_WASM_PROFILE 拒绝，未宣称通用 Wasm 验证或当前网关 ABI 可加载。配置为独立 Draft 2020-12，选定 schema 与全部本地 schema 引用先解析并完整校验 defaults；拒绝网络/未知引用、spec.yaml 包装、隐藏归档成员与脚本。合成秘密规则只识别冻结测试标记，没有任意字符串密钥识别承诺。verified 来源通过固定合成 source receipt 匹配，仅证明测试责任衔接，真实来源证明留 P1。

复现：在已按 `tests/contracts/requirements.lock` 准备的环境，从本仓执行 `python -B -m unittest discover -s tests -p 'test_contract_artifact_*.py' -v`；类型检查覆盖 4 个工具和 3 个测试文件。直接 `tests/contracts/export_bundle.py --offline --output NEW_DIR` 导出完整惰性包，拒绝已存在输出。源清单和 export-map 由 ART01 单写，02/03 先交 entry-proposals 再串行登记；已导出的固定版本不覆盖。

D0 通过仅表示上述离线契约、真实归档字节和纯协议样例一致。生产服务、持久 commit/query、实际 HTTP、并发提交、目录注册、网关加载、ready/supported/production available、构建和部署均为 not_run。

复审修复：验包工具只把明确输入 JSON/shape 错误译为 invalid；依赖/环境及未知工具异常保持 execution_error，含针对不合法 defaults 包的注错守卫。配置引用复用固定 common resolver，接受 revision anchor、percent 编码 anchor/JSON Pointer，未知 anchor 使用 ART_CONFIG_REFERENCE。14 项测试及禁网隔离 178/178 再验通过，固定 ART.3 包字节未变。
