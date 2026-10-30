# Kimi Models API

- Source group: official `platform.kimi.com/docs/api` pages
- Fetched on: 2026-04-23

## Sources

- https://platform.kimi.com/docs/api/list-models
- https://platform.kimi.com/docs/api/models-overview

---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# 列出模型

> 列出当前可用的所有模型。

列出当前可用的所有模型。


## OpenAPI

````yaml GET /v1/models
openapi: 3.1.0
info:
  title: Moonshot AI API
  version: 1.0.0
  description: Moonshot AI / Kimi 大语言模型服务 API
servers:
  - url: https://api.moonshot.cn
    description: 生产环境
security: []
paths:
  /v1/models:
    get:
      tags:
        - Models
      summary: 列出模型
      description: 列出当前可用的所有模型。
      responses:
        '200':
          description: 模型列表
          content:
            application/json:
              schema:
                type: object
                properties:
                  object:
                    type: string
                    example: list
                  data:
                    type: array
                    items:
                      type: object
                      properties:
                        id:
                          type: string
                          description: 模型 ID
                          example: kimi-k2.5
                        object:
                          type: string
                          example: model
                        created:
                          type: integer
                          description: 创建时间戳
                        owned_by:
                          type: string
                          example: moonshot
                        context_length:
                          type: integer
                          description: 最大上下文长度（tokens）
                        supports_image_in:
                          type: boolean
                          description: 是否支持图片输入
                        supports_video_in:
                          type: boolean
                          description: 是否支持视频输入
                        supports_reasoning:
                          type: boolean
                          description: 是否支持深度思考
        '401':
          description: 未授权 - API 密钥无效或缺失
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
      security:
        - bearerAuth: []
components:
  schemas:
    ErrorResponse:
      type: object
      properties:
        error:
          type: object
          properties:
            message:
              type: string
              description: 描述错误原因的错误消息
            type:
              type: string
              description: 错误类型
            code:
              type: string
              description: 错误码
          required:
            - message
      required:
        - error
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        Authorization 请求头需要一个 Bearer 令牌。使用 MOONSHOT_API_KEY 作为令牌。这是一个服务端密钥，请在
        [API 密钥页面](https://platform.kimi.com/console/api-keys) 生成。

````
---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# 模型参数参考

export const DocTable = ({columns = [], rows = []}) => {
  return <div className="doc-table-wrap">
      <table className="doc-table">
        {columns.length > 0 ? <colgroup>
            {columns.map((column, index) => <col key={index} style={column.width ? {
    width: column.width
  } : undefined} />)}
          </colgroup> : null}
        <thead>
          <tr>
            {columns.map((column, index) => <th key={index}>{column.title}</th>)}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, rowIndex) => <tr key={rowIndex}>
              {row.map((cell, cellIndex) => <td key={cellIndex}>{cell}</td>)}
            </tr>)}
        </tbody>
      </table>
    </div>;
};

不同模型系列对 Chat Completions API 参数有不同的默认值和约束。完整的模型列表请参阅[模型列表](https://platform.kimi.com/docs/models)。

## 参数对比

<DocTable
  columns={[
{ title: "参数", width: "18%" },
{ title: "kimi-k2.6", width: "18%" },
{ title: "kimi-k2 系列", width: "20%" },
{ title: "kimi-k2-thinking 系列", width: "24%" },
{ title: "moonshot-v1 系列", width: "20%" },
]}
  rows={[
[<code>temperature</code>, <strong>不可修改</strong>, "0.6", "1.0", "0.0"],
[<code>top_p</code>, <>0.95 <strong>不可改</strong></>, "1.0", "1.0", "1.0"],
[<code>n</code>, <>1 <strong>不可改</strong></>, "1（最大 5）", "1（最大 5）", "1（最大 5）"],
[<code>presence_penalty</code>, <>0 <strong>不可改</strong></>, "0（可修改）", "0（可修改）", "0（可修改）"],
[<code>frequency_penalty</code>, <>0 <strong>不可改</strong></>, "0（可修改）", "0（可修改）", "0（可修改）"],
[<code>thinking</code>, "支持", "—", "—", "—"],
]}
/>

<Note>
  当 `temperature` 接近 0 时，`n` 只能为 1，否则将返回 `invalid_request_error`。
</Note>

## Kimi K2.6 — thinking 参数

Kimi K2.6 支持通过 `thinking` 参数控制是否启用深度思考。接受 `{"type": "enabled"}` 或 `{"type": "disabled"}`。

由于 OpenAI SDK 没有原生的 `thinking` 参数，需要使用 `extra_body` 传递：

<CodeGroup>
  ```python Python theme={null}
  completion = client.chat.completions.create(
      model="kimi-k2.6",
      messages=[
          {"role": "user", "content": "你好"}
      ],
      extra_body={
          "thinking": {"type": "disabled"}
      },
      max_tokens=1024*32,
  )
  ```

  ```bash cURL theme={null}
  curl https://api.moonshot.cn/v1/chat/completions \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $MOONSHOT_API_KEY" \
    -d '{
      "model": "kimi-k2.6",
      "messages": [
        {"role": "user", "content": "你好"}
      ],
      "thinking": {"type": "disabled"}
    }'
  ```
</CodeGroup>
