# 媒体票据的规范余额键与原等待

Helm124的原生Qwen2已生成并发布票据，但未消费、未发送供应商请求。媒体网关自己拼接了无花括号的余额前缀，并把Key映射放到scope下，和IF05原钱包/account/principal层级不一致。原测试直接模拟allow，未核传入的真实键。[规范键红例](health-media-runtime-20261031/ai-proxy-2.0.16/canonical-keys-red-command.json)已在原源码上失败。

网关改为规范的scope花括号、原account及其具体Key映射。实际Lua在projection_unready时确认票据仍prepared、没有写消费事件；这一明确未消费的分支保留原executor和票据，通过已有Root Tick重读资格。其余拒绝、未知或已消费结果保留原行为。原Key、原attempt、invocation和截止时间都不替换。

RESP边界统一返回已有授权错误或明确projection_unready，不在调度层反复解析原始回复。createTaken仅对真正的消费响应及未知状态封闭，明确未消费的投影等待可续接；首次allow后仍只许一次发送。原宿主用例新增用户/Key键断言及投影恢复/原截止到期两案，原生Redis用规范IF05键验证Lua；[分层命令](health-media-runtime-20261031/ai-proxy-2.0.16/media-boundary-commands.json)全部通过。

本批发布版本2.0.16，保留15完整包。尚待统一实际Wasm批次、完整包入库/部署、六媒体与资金链；没有以宿主夹具代替实际供应商发送。
