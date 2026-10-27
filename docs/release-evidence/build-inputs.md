# WG-I05 构建输入记录

2026-10-27 当前进行[版本化余额准入批次](../plans/2026-10-27-versioned-balance-admission.md)。本页只记录已核实的输入，实际Wasm和部署证据完成后追加，不把源码版本当作已加载版本。

| 模块 | 声明 | 本次实际落点 / 已核事实 |
| --- | --- | --- |
| 工具链 | go.mod最低1.24.1、toolchain1.24.4 | 当前 `go version go1.26.3 linux/amd64`，具体构建命令及产物尚待统一执行 |
| ai-quota | VERSION2.0.0 | 原绝对replace预检失败；当前相对`../..`引用本仓，`../../../proxy-wasm-go-sdk`引用实际相邻SDK；GOWORK=off模块解析通过 |
| key-auth | VERSION1.1.0 | 同上，独立嵌套模块解析通过；未触碰ai-billing用户VERSION改动 |
| Console余额写端 | 59cbc044 / ccc39542，PR57 | 已验原钱包版本投影；当前新库blocked，实际恢复ready/REL/WG/DEP出口尚未验 |

统一检查将显式构建两个Wasm并指定每个模块自己的WASM_FILE_PATH，记录源指纹、实际模块图、工具链、二进制SHA256和宿主加载。Wasm测试框架的默认自动编译失败会skip，不能据此宣称Wasm通过。实际网关部署还须由ART/SDK/REL/DEP关联同一字节与配置观察，当前未执行。
