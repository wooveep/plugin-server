# 托管流式原用量修复与2.0.15交付

2026-10-31。实际UserWeb在Helm117已显示完整SSE回复，物理上游仅一次，但原attempt和delivery均为 `missing / stream_usage_not_observed`，钱包版本2未变化。[真实失败原件](../../../modelfusion-deploy/docs/checks/health-media-runtime-20261031/portal-ui-sse117-missing-usage.json)保留，不把页面输出当作结算通过，也不改写该次缺用量事实。

## 原因与修复边界

`onHttpRequestBody` 先由 `onManagedRequestBody` 接管，旧路径中的 `normalizeOpenAiRequestBody` 不会执行。托管 `managedProtocolProfile::PrepareRequest` 在原生同协议时保留了映射后请求，因而未要求OpenAI流式供应商返回usage。

源头修复 `e1766d5e36be76241f603d2f4b9b4617a3edbb5d` 只在实际上游协议为 `openai_chat` 且流式时写入 `stream_options.include_usage=true`，包括原协议转换为Chat的路径；随后原发送准备器冻结实际outbound字节摘要。客户传false也不会禁止托管计量索取原usage。非流式及原生Responses/Messages不注入Chat选项；供应商仍缺用量时继续记录missing，不估算或补零。没有增加计费写者、队列、重发或Console反向调用。

三个版本元数据同步为2.0.15。构建首次在既有目录校验处发现总catalog仍声明14，于 `0f222691f4ebf4835456911e47116423a87d874c` 同步目录；校验保持。编译回执按534个插件输入的最近提交绑定e176，目录/服务器冻结提交单列0f222，完整来源可以核对。

## 实际验证和完整包

[针对原缺口的红例](health-media-runtime-20261031/ai-proxy-2.0.15/red-command.json)真实执行失败，显示managed Chat stream没有usage请求。修复后用固定构建的Wasm执行[整个Go/Wasm批次](health-media-runtime-20261031/ai-proxy-2.0.15/full-test-command.json)：323顶层、2200含子例通过，退出0；同协议false/缺省、转换Chat及既有原始用量分类都在原模块中验证。

[构建](health-media-runtime-20261031/ai-proxy-2.0.15/build-receipt.json)、[包审核](health-media-runtime-20261031/ai-proxy-2.0.15/package-review.json)及[来源/实际入库](health-media-runtime-20261031/ai-proxy-2.0.15/qualification.json)记录固定Go1.26.3、534个原输入及原86个manifest保持：

| 对象 | SHA256 |
| --- | --- |
| Wasm | `23a554990f88fcdac352315508b5bd25ec75cada39830b2f88552d241b1ff75f` |
| 完整包content | `3daf39f61b74fb75d6457dcbd7c9bc274706f5653bd2d4c546a3e0c879006ec9` |
| archive | `3e6e036f2b405d3d8ecfa5bbf38c0ea81e2b8d625fc31b2c682a2639b558f9ab` |
| 原始manifest | `d4dd640c7d7dad3ded9c62322819a3fac4bbc28762b328db5e6cac62855535cd` |

完整本地目录87版本、55正式类通过既有验证器。[服务器镜像回执](health-media-runtime-20261031/ai-proxy-2.0.15/server-image-build.json)固定镜像 `sha256:7f0167ba33a7261fcf137dd039635536eb3a603d0dc860569d13591ebc5a6f29`，inventory `sha256:ec2897ccfc28ccf21ac0a40b0e9d786ce8b3084df272c59d0a1363172992fbf3`。构建时 `deployed=false`，实际Helm、HTTP、网关加载及新调用资金验收将在后续运行原件中追加；本批测试不代替真实计费或全部媒体/停机业务通过。

## 实际部署与新调用

[DEP权威运行回执](../../../modelfusion-deploy/docs/checks/2026-10-31-health-media-fresh-deployed.md#managed-stream-helm120)记录Helm118首次引用时序404、修正为先制品HTTP核对后切引用的119/120，以及873个实际HTTP文件全200/摘要一致。实际全局媒体和Portal原生rev4两处ECDS、本地Wasm均为上表15摘要，API/Gateway UID与PVC保持。

原UserWeb单独发送一条新消息，真实SSE200及原17/5用量、物理仅一次，原attempt/delivery两FACT均reported/provider。销售612500、采购122500（scale10）分别应用一次，钱包版本2到3、余额投递delivered。上游为明确受控synthetic fixture；供应商真实媒体另验。原14缺usage的调用原件保持，未重发或补写账务。
