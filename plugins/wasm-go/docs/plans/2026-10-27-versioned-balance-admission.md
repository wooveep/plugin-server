# 真实Key身份与版本化余额只读准入

| 字段 | 内容 |
| --- | --- |
| 状态 | 原生接线已编码，整批Go/Redis/Wasm验证待运行；实际K3S未验 |
| 创建 / 最近更新 | 2026-10-27 / 2026-10-27 |
| 任务 | WG-I01认证主体贡献、WG-I04余额读端、WG-I05配置/模块依赖串行贡献 |
| 负责人 / 日期 | Codex代班；自然人、期限待确认 |
| 源基线 | b72dd66a3fd98bd927f8f1a402d239ccb3a3ac58 |

## 输入与责任

消费 [MF-CACHE01 PR57](https://github.com/wooveep/ocloudmodelfusion/pull/57)固定源码59cbc044与证据ccc39542的原钱包/版本化投影，以及冻结common、balance、identity-eligibility与IF05。原输入保持不变；本批UUID与signed-bigint上限是原生PG适配边界。

[WG-I01登记](https://github.com/wooveep/wasm-go/issues/3#issuecomment-6440078052)独占共享balanceidentity及key-auth接线；[WG-I04登记](https://github.com/wooveep/wasm-go/issues/7#issuecomment-6440076447)负责quota读脚本/原生/传输/路径与响应归属；[WG-I05登记](https://github.com/wooveep/wasm-go/issues/8#issuecomment-6440079529)独占spec、版本、示例和两个实际模块依赖。原docs/README.md、ai-billing/VERSION、用户未跟踪计划及ai-load-balancer保持原样。没有修改ai-proxy/provider/wrapper发送决策或Console维护写者。

CodeGraph先定位，graphify报告作为仓库地图；GitNexus原索引落后6提交，按仓库规则仅analyze --index-only刷新。修改前分别impact已修改符号，报告含跨wrapper CRITICAL及未解析边界，已告知用户；不把缺边或结果截断当无影响。代码完成后更新既有graphify索引，提交前detect-changes检查staged范围。

## 本批业务

1. key-auth按实际成功认证的credential读取完整绑定，携带原PG账户、具体Key ID、字符串credential_generation。先清理客户端同名字段；public/unmatched、auth-only和无原生绑定旧consumer不能留下原生钱包身份。既有consumer名称/allow-list语义保持。
2. quota消费该可信链和全局受控scope/generation，一次只读Lua读取同slot control/account/key mapping；同代/schema/ready、账户与授权版本/凭据代际/当前Key enabled均匹配后才判断正余额。钱和版本只做规范字符串精确校验，不转double。
3. 缺失、旧代、未就绪、旧授权、非法金额和依赖失败分别记录原因。拒绝后不Resume；不计价、不预扣、不结算、不写余额/映射/ready，不因响应或迟到usage再判一次。
4. 选中规则内的路径与方法形成交集，保留既有规则覆盖过滤器的语义；规则不能替换账户或全局namespace/Redis。已受理任务查询不做新销售余额准入，通过路由绑定和方法过滤排除，不能仅按GET/HEAD跳过已分类的新调用；内部事实、结算和任务查询入口不绑定quota。实际发布必须证明新调用入口分类和完整过滤器顺序。
5. 新读写协议一起发布。quota2.0.0明确拒绝旧模板与allow-missing；key-auth1.1.0提供可选原生绑定，缺绑定不能通过新quota。两个嵌套模块的失效绝对replace改为真实仓库相对路径，模块解析和实际Wasm字节分别留证。

## 验证与出口

- [ ] 同一批次验证共享身份、两个模块全部Go测试，明确模拟宿主与实际Redis区别。
- [ ] 显式先构建两个Wasm并传固定WASM_FILE_PATH，防止既有测试框架把编译失败静默skip后算通过；记录实际摘要、命令、工具链和模块解析。
- [ ] 实际Redis覆盖超过2^53/i64边界、缺失/错代/schema/blocked、Key撤销与授权/凭据代际失配；读取前后DUMP不变。
- [ ] Go/Wasm模拟宿主核实际credential绑定、客户端字段清理、拒绝无Resume、原路径交集及所有响应类透明。
- [ ] 固定源码与输入、CodeGraph、两轴静态复核、原始失败/通过日志、相对链接和Issue回填。
- [ ] Console/CUT/SDK生成当前具体Key绑定及同代配置，REL/DEP观察实际完整链、维护拒绝/恢复与K3S加载字节，完成IF05/APP-V9和相关整卡联合出口。

本批源/模拟宿主/原生Redis通过不能替代最后一项。实际配置生成和观察尚未接入，所有相关整卡保持OPEN；不做历史迁移或付费探测。

## 进展

2026-10-27 旧两个模块 `GOWORK=off go list -m -json all`均因迁移前绝对路径不存在而失败；保留原预检。WG-I05按当前workspace相对路径修正后两个模块解析均通过，没有各卡分别替换依赖。

2026-10-27 旧测试要求裸GET、allow-missing并禁止全部EVAL，与冻结IF05冲突。更新真实路径交集、配置继承、响应归属和schema示例/解析边界；新增实际Redis只读案例与真实credential身份的Go/Wasm宿主案例，不用文案字串断言冒充业务结果。当前尚未运行本批统一检查。
