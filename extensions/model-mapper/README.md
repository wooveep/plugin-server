# model-mapper：模型映射与原始正文依据

交付版本 `1.0.0`，字段以 [资源 schema](../../resources/plugins/model-mapper/spec.yaml) 为准。插件把 POST `application/json` 请求根节点的 `model` 精确映射到上游模型；托管链在映射前固定原始正文与公开模型，供 ai-proxy 建立冻结的执行选择。

## 实际配置

```json
{
  "capture_body_basis": true,
  "modelMapping": {
    "public.v1": "provider.v2"
  }
}
```

全局与路由 schema 都要求非空 `modelMapping`。键与目标都是非空、可安全放入 header 的字符串，精确区分大小写；没有通配符、前缀匹配或兜底模型。未配置对应模型时返回 422。上例只是字段示意，托管发布必须使用原销售模型与目标 descriptor 中冻结的上游模型。

`capture_body_basis` 是布尔值，默认 `false`；托管执行链必须显式设为 `true`。托管发布的每条有效规则须提供本目标完整的映射与 capture 设置，不从最终响应的 model 或请求 URL 推断映射关系。

## 映射前固定原始正文

请求必须有完整 UTF-8 JSON 对象正文，最大 1 MiB，根字段不能重复，`model` 必须是合法字符串。支持 `application/json; charset=utf-8`，拒绝压缩正文。缺失或非法正文/model 返回 400，不支持的方法返回 405，内容类型/编码不支持返回 415，正文超限返回 413。

在请求 header 阶段，插件先移除客户端提交的所有同名字段：

- `x-modelfusion-original-body-digest`
- `x-modelfusion-original-public-model`

插件暂停后续 header 处理，等待完整正文。原正文 raw bytes 保持不变；`original_body_digest` 在映射前对原 JSON 按 `mf-json.v1` 规范编码后计算 SHA-256，递归排序对象键并保持数字精度、数组顺序及省略/null 差异。capture 开启时固定该摘要与原公开模型，再用 `sjson` 只改写根 `model`，保留其他字段和数字字面量；原字节保留与规范摘要计算是两个独立步骤。实际出站及响应正文摘要仍使用 `raw-bytes.v1`，由发送责任点记录。

改写后移除旧 Content-Length，安装且仅安装一份原始摘要与原公开模型 header，随后允许下游处理。capture 关闭时仍清除客户端字段，但不安装这两个 header。

## 托管发布顺序与边界

插件为默认阶段、优先级 `200`，在同阶段优先级 `100` 的 ai-proxy 之前执行。capture 前的插件不能改写正文。托管范围由路由绑定给出，无需配置 URL 白名单；绑定范围也不改变本插件只接受 POST JSON 的请求约束。

ai-proxy 必须同时核对受控发布顺序、原公开模型、实际映射后的模型和原正文依据。`bodybasis.Read` 只校验字段格式及唯一性，本身不证明 header 来源。mapper 不创建 call/attempt，不发布账务事实，也不按映射后正文重算原始摘要。

`main_test.go` 使用 `pkg/test.RunTest` 核对 Go 与 Wasm 宿主生命周期；真实网关配置、加载顺序和上游收件仍由对应发布的运行观察单独证明。
