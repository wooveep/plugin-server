# 原生图像可选参数对象与真实供应商修正

源码 `9ea11acb0f0a2edf7ae8fd367ae411a656b430ca`，完整 ai-proxy **2.0.18**。本记录维护协议源码、回归及完整包身份；[DEP 真实任务、发布与账务](../../../modelfusion-deploy/docs/checks/2026-10-31-health-media-fresh-deployed.md#real-media-fixed-helm130)是运行验收权威来源。

真实 Qwen3 异步请求省略可选参数组后，供应商先返回 task，再以 FAILED/InvalidParameter 终止，原错误为 `Input should be a valid dictionary or instance of ImageInterleaveParameters: parameters`。原 task、失败、缺计量与零额未完成应用全部保留；查询原 task 不会使这次无效创建变为成功。

根因位于 `plugins/wasm-go/pkg/ai/mediaexecution/profiles.go` 的协议正文构造：平台允许省略组，原出站也省略，异步服务实际要求参数组是对象。现在仅当原生图像组缺失时，出站补 `{}`；显式参数及其数字原字节保持，兼容 Images 与视频分支保持。原平台正文与 mf-json.v1 body basis 独立保留，没有改请求模型/凭据/查询权或增加 create 重试。

[构造链 RED](qwen-parameters-runtime-20261101/regression-red.json)经过实际 FromPrivateArchive → PrepareCreate，三个原生图像 profile 均在“缺失对象”断言失败；修正后三个 profile 通过，既有显式参数精度断言继续有效。[实际 Redis 分层批次](qwen-parameters-runtime-20261101/library-green-command.json)78 个顶层/244 含子项通过；仅未提供可选跨语言导出目录的一项跳过。[完整新 Wasm 批次](qwen-parameters-runtime-20261101/full-go-wasm-command.json)323 顶层/2203 含子项通过、exit0。guest 日志可能拼接 PASS 行，按唯一 Go PASS 名称统计；原行前缀计数保留，测试名称集合与2.0.17完全一致。

[完整包来源审查](qwen-parameters-runtime-20261101/source-review.json)、[不可变入库](qwen-parameters-runtime-20261101/qualification.json)及[镜像回执](qwen-parameters-runtime-20261101/image-build-receipt.json)相互一致：

| 项目 | 固定值 |
| --- | --- |
| Wasm SHA256 | `992421f094caf36b2c54737f698656000d59ac546eaae3b33c7f77732246e21d` |
| archive SHA256 | `4ab25ca6da9b1d1a9a1db334db6cf969824e8fdd5cb7fe10f532f2a1c7b8d103` |
| content digest | `sha256:60abfedc93f5038cde3f217d127f43298c98d6e32c3fb4ff00c45018f3259676` |
| artifact ID | `c660ea6b-2c0d-499c-82d6-7c495d5efebc` |
| 镜像 digest | `sha256:414acc980296effeb38024243ea39a9980f36d1730b2c2400d6a266b4db92685` |

65 份既有 provenance 回执保留，新增后66份；89份既有 manifest 逐字保持，登记版本90个、正式默认55类。Helm129仅PS Deployment，实际903 HTTP文件全部大小/SHA匹配；目录原生登记revision59后，Helm130仅全局媒体Wasm引用，Gateway实际载入上述字节。四条真实路由revision3 applied并逐路核对ECDS，原API、PVC与Gateway UID保持。

新的独立 Qwen3 修正用例 `01a19347-a8cb-7151-9d91-a0f427aede73` 原请求仍省略组，原冻结正文与首次失败完全相同；实际出站摘要唯一差别为补空对象，最终真实成功一张。新call为 `01a19347-a8cb-7151-9d91-a0e3c1aca959`，task为 `5481f57d-d57d-42ea-94bb-a2d289e683f2`。销售0.10、标准采购0.18CNY分别入账，查询不计作生成，钱包22→23且实际Redis一致。初次失败task没有再次create。旧Qwen2及WAN成功分别复用其原2.0.17运行证据，没有将它们改写成2.0.18新调用。

CodeGraph先查询了当前Go路径与符号，但现有索引没有这些唯一匹配；本次以当前实际源码及固定提交核对，没有重建索引或采信旧切片。后续另修Console将非终态交付unknown覆盖已受理生成进度的问题；本修正及四类真实成功产出不代表完整媒体/HEALTH/W12/EB卡关闭。
