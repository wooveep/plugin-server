# WASM D0 固定契约消费者

2026-10-25：批次 A 已固定，完整消费 common 与 execution 两个独立小包。运行只读本仓 vendor；导入是显式维护步骤。WG01 独占本目录和 `tools/contractcheck/`。

| 输入 | manifest SHA-256 | schema / fixtures / assets |
| --- | --- | --- |
| `mf-common-2026-10-25.3` | `09d0906d9f09b04f24266dee6bfab05785c5245373cc222612ec5b3b11e036f5` | 4 / 93 / 36 |
| `mf-execution-2026-10-25.1` | `7b2d3d58939302bb9baf736713d2e92338e341a373b2829b43b09805f6c1ce49` | 1 / 466 / 27 |

包内容均来自 MF01 owner 显式导出，按各自 manifest 保留 `bundle-manifest.json`、全部 entries、fixtures、assets 的原始字节和包内路径，共 629 文件。`contracts.lock.json` 使用权威五字段，不增加第二种 lock 格式。common 只在 `.3` 包注册一次，execution 的引用在合并的离线 URN registry 中解析。更改必须先发生在权威发布包，再由 WG01 用显式源路径及预期 manifest 摘要重新导入完整小包；日常验收不更新、不下载、不访问相邻源码。

独立 Go 模块只有标准库依赖，保持 `go 1.24.1`，因此没有 `go.sum`。完整 Draft 2020-12 引擎为固定 `jsonschema==4.25.1`，经 Go 薄启动器调用，非有限字段检查器。16 个工具文件由 MF 固定提交 `753704b29a6ea40815d45dcbbce16cc0d91bace2` 逐文件导入，摘要列在 `tools/contractcheck/testdata/tool-lock.json`；不属于机器契约包。

工具布局为 `tools/contractcheck/python/` 下的：`scripts/contracts/{validate_bundle.py,requirements.lock,assertions.schema.json}`；`contracts/v1/{common,fixture-validation,bundle,lock}.schema.json`；`contracts/v1/vendor/json-schema.lock.json`；`contracts/v1/vendor/json-schema/draft2020-12/schema.json` 与 `meta/{core,applicator,unevaluated,validation,meta-data,format-annotation,content}.json`。core 保留原字节，Go 薄桥仅在引擎输入边界调用原工具的 `schema_numbers`，把整值 Decimal 精确转为 native int，避免跨 `$schema` resource 丢失自定义 integer 类型检查；没有 float 中转。

验证器校验包 / 文件摘要、vendor 边界、依赖闭包、URN 与 fragment、schema draft、document / JSON Pointer / NDJSON 选择器、预期阶段和错误分类。固定版本分派 `d0-assertions.v1`、`mf-json.v1`、`d0-resolver.v1`；未知 validator 拒绝，非只跑 shape。公包 559 个正负案例全部执行。Go 消费者另覆盖单字节篡改、错 manifest 摘要、缺 common、重复 URN、HTTP `$ref`、未知 validator、路径越界和工具篡改；schema 测试覆盖缺字段、错 digest/revision、未知 fragment 及大整数。

先显式准备满足 hash lock 的 Python 环境；已准备环境可设置 `D0_PYTHON`。工具会核对精确依赖版本与 8 个本地 metaschema，缺缓存或版本错误报告 blocked，不自动安装。Go 工具使用 `GOWORK=off GOPROXY=off`；不创建 `go.work`，不改根模块。

```bash
cd tools/contractcheck
D0_PYTHON=/tmp/modelfusion-d0-venv/bin/python GOWORK=off GOPROXY=off go test -count=1 ./...
D0_PYTHON=/tmp/modelfusion-d0-venv/bin/python GOWORK=off GOPROXY=off go run . verify --lock ../../testdata/contracts/contracts.lock.json --batch A
```

准确展开的 629 文件清单及实际摘要由单命令驱动输出 `imported-files.json`，其摘要再绑定到 `summary.json`；两份固定 manifest 同时保存每一文件的权威摘要。B/C 尚未安装；显式请求它们报告 `E_SCOPE_BLOCKED`。参见 [运行与隔离验收](../../docs/d0-fixture-runbook.md)。D0 不改变真实 EB、P1、supported 或发布状态。
