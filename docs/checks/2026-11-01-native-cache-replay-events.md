# 发布实例的缓存命中事件边界

Helm139 的完整对话请求实际命中缓存，客户供应商发送0次、当前调用销售扣费一次；随后，命中响应却又产生了 MemoryEvent、标为 `upstream_invoked:true` 的 CacheEvent，以及新的提取与缓存向量化供应商发送。实际账本、原错误标签和成本由 [DEP 运行记录](../../../modelfusion-deploy/docs/checks/2026-11-01-internal-runtime.md)保留。

根因有两处。Memory 原命中识别只接受旧的插件实例名，未接受当前发布器实际生成的 `namespace.rel-<opaque-release>-ai-cache`；Cache 本身则没有在响应事件入口排除自己已经决定发送的本地回放。修复扩展原 `memorycache` 命中名称契约，并在原回放函数保存当前 HTTP 上下文的本地状态，由原响应事件入口一次性使用。已有销售交付桥继续负责当前客户调用。

共享 framework 的命中函数由本聊天串行修改，实际调用方为 Memory；Cache 的私有回放状态改动位于原插件内。正式版本为 ai-memory0.1.10、ai-cache2.0.8。共享源码输入的变化会反映在 Go 插件的保守输入闭包中；本批制作这两个实际消费者的新制品，旧制品及其原来源保持。

[三份 RED](cache-replay-20261101/red-commands.json)分别复现名称契约、已交付 Memory0.1.9 的普通/流式响应学习、已交付 Cache2.0.7 自身本地回放事件。后两者均在 Go/Wasm 上失败，错误与真实运行相符。修复后的完整批次和实际不递归验收继续，未将本地回放响应作为一次新模型发送。

完整批次通过：[Memory72顶层/607含子项](cache-replay-20261101/ai-memory-green-command.json)、[Cache66顶层/562含子项](cache-replay-20261101/ai-cache-green-command.json)，均无跳过；[共享命中契约](cache-replay-20261101/contract-green-command.json)及[catalog五项检查](cache-replay-20261101/catalog-green-command.json)通过。[两份完整包资格](cache-replay-20261101/qualification.json)绑定当前已提交来源、原始输入和实际测试字节；95份制品包含这两个新版本，原93份manifest与69条来源批准保持。实际新包加载与不递归取证继续。
