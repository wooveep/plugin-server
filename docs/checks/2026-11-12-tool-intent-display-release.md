# 供应商工具意图统计展示发布

日期：2026-11-12。源码在 dev `09b13b58`，开发 K3S 正常 Helm77 升级。问题来自 I06 真实搜索：兼容响应投影丢弃供应商 `usage.x_tools`，导致用户端无法保留原始工具意图统计。

只透传六种登记工具的有效整数 count 和已登记策略；JSON 与 SSE 末帧共用原投影。DeepSeek/Qwen 兼容输出保留这项扩展，严格 OpenAI profile 不增加字段。未知字段和未知策略剔除。该对象是显示数据，没有扩大金融 meter，也不把供应商搜索意图次数等同完成或可计费次数。语义依据[阿里云搜索 API](https://help.aliyun.com/zh/model-studio/web-search)。

共享 Go 输入变更依现有不可变制品规则推进 46 个正式 Go patch 版本，包含 ai-proxy 2.0.80、ai-billing 1.0.16、ai-quota 2.0.13、key-auth 1.2.7；另外8个C++和1个Rust原完整包复用。55/55 固定工具链构建及完整包验证通过，clientresponse 既有/新增简单单元通过。两个独立审查轴先核源码 Git 输入、构建回执、实际 Wasm 和完整归档，再合并独立旧策略，216 个来源回执；没有从待审描述符自证来源。

运行后验：VM 制品登记库236项，原190项ID/content/archive/manifest和载荷字节均保留；全库1400个载荷核对通过，本批55项326载荷与已审归档一致。同一镜像的 plugin-catalog.json 通过 Helm --set-file 进入 API 原生55项启动登记，exit0。发布前后的资源、摘要和逐项完整包哈希见[脱敏机器回执](2026-11-12-tool-intent-display-release.json)。

本记录不代表所有 I06 合同验收。原一次付费搜索没有为补证重放，工具金融观察仍保留 supplier_reports_intent_only / missing；新的原始 SSE 和用户界面需要主审独立验证。跨项目执行结果见[Console 跟进记录](../../../ocloudmodelfusion/docs/checks/2026-11-12-feature-issues-followup.md)。
