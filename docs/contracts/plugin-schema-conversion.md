# Higress 元资料转换 v1

实现版本 `higress-conversion.v1`。ART-04 负责转换，ART-01 验证受控扩展，SDK/Console 必须消费精确版本，不能从 latest 补回缺失元资料。普通 JSON Schema 使用 Draft 2020-12；原 OpenAPI 包装不能直接作 JSON Schema。

| 输入 | 包内位置 | 摘要覆盖 / 消费方 |
| --- | --- | --- |
| apiVersion | 只接受 1.0.0，其他显式拒绝 | 转换器版本锁 |
| info 全部身份及展示字段 | config.schema.json 的 x-higress.info | 原字节文件摘要，ART/SDK/Console |
| spec.phase / priority | x-higress.phase / priority；priority 精确 int32 | 同上，SDK 编译 |
| configSchema.openAPIV3Schema | config.schema.json | 全局配置完整验证 |
| routeConfigSchema.openAPIV3Schema | 独立 route.schema.json，扩展记录路径；缺失显式 null | 路由配置完整验证；不假设等同全局 |
| x-title-i18n / x-description-i18n / x-scope / x-sensitive | x-higress.annotations.{global,route} 按原 JSON Pointer 保留；x-sensitive 必须是 boolean | 展示、scope、敏感字段；不作为 JSON Schema 校验关键字 |
| format: password | 同一 annotations 位置保留 format: password，仅接受 string 字段 | 密码输入展示，不注册为 JSON Schema 校验 format；其他 format 沿原严格白名单验证 |
| writeOnly | 保留在标准 Draft 2020-12 schema | 标准注解；不以它替代或丢弃原敏感展示标记 |
| wrapper x-higress-external-resource(s) / spec.externalResources | x-higress.external_resources，保留 global/route 归属和原字段 | 资源闭包；消费者遇未知结构须拒绝，不能静默丢弃 |
| example | examples 单项数组；同时存在两者拒绝 | 校验语义不变，展示数据保留 |
| nullable | 只允许独立 type 转为含 null 的类型数组 | enum/ref/组合等含糊情况拒绝 |
| boolean exclusiveMinimum/Maximum | true 把对应 bound 转为数值排他界，false 只移除布尔标志 | 精确十进制，不经浮点 |
| defaults | 独立 defaults.json，按全局选定 schema 验证；路由配置需另验 route schema | 原字节入 files；不是 example 自动生成值 |
| 本地引用 | 全部声明 config_schema 文件、完整闭包，未用分支也检查 | 禁网；未知 URI、anchor、关键字明确拒绝 |
| 来源 | source_spec_digest、converter_version 和完整包 source | verified 仅凭独立可信 source/Wasm/content 三摘要回执 |

未知 spec/wrapper/运行关键字返回 `unsupported_schema_conversion`；未知 JSON Schema 关键字返回 `ART_CONFIG_SCHEMA`。不忽略不认识的字段。局部转换保持 schema 位置，example/default 中的普通对象不当 schema 遍历。

外部资源声明接受现有 `redis`、`http` 与 `http-callout` 类型。可选的 `protocol` 仅接受 `http`；声明该协议时，SDK 纯编译还须核对冻结 McpBridge registry 的实际协议、命名空间、域名和精确端口。未知声明字段或其他协议继续拒绝；声明本身不创建服务或证明调用成功。

敏感字段不声明 default、example 或 examples。仅当该 global/route scope 显式声明 x-sensitive:true、writeOnly:true 或 format:password 时，祖先对象的这些数据也按原 ART 凭据策略拒绝字面凭据；未标记的旧 scope 保持原转换行为。defaults.json 的受控引用规则不变，bearer_token 与 auth_token 明确属于凭据字段。转换器拒绝带凭据的样例，不删除原样例后宣称原 spec 已通过。展示消费者需从原 scope/JSON Pointer 恢复两项展示标记；严格的 SDK 扩展 schema 镜像也必须同步，仅改生产者不代表旧消费者已能接收新元资料。

祖先样例按 schema 声明的属性、数组位置及本地引用定位敏感值，不依赖凭据字段的名称；独立 scope 不相互继承标记。组合分支保留各自声明的敏感路径。敏感标记位于无法可靠定位实例的条件、content 或 unevaluated 位置时，返回明确的转换不支持错误，不猜测条件结果或读取外部引用。

真实测试输入为 key-auth 与 ai-billing 的 spec 原字节，固定保存在 tests/fixtures/schema-conversion/。包括不同 global/route、AUTHN/priority，以及 Redis 外部资源声明。测试验证路由 allow 空数组可用、缺字段拒绝、精确小数、nullable 和不支持反例。

打包器固定 USTAR 成员排序/权限/时间和 gzip 头；相同输入归档逐字节一致。content_digest 使用冻结 artifact-json.v1 投影及 MF01 的 mf-json.v1；Wasm/schema/default/docs 全取原字节摘要。schema 转换改变配置解释时必须发布新插件版本。

本文件是当前转换片段交接，不表示 SDK 已完成扩展消费、网关 ABI 或运行验证通过。
