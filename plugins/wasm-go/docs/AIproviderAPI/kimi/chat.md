# Kimi Chat API

- Source group: official `platform.kimi.com/docs/api` pages
- Fetched on: 2026-04-23

## Sources

- https://platform.kimi.com/docs/api/chat
- https://platform.kimi.com/docs/api/tool-use
- https://platform.kimi.com/docs/api/partial
- https://platform.kimi.com/docs/api/estimate

---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# 创建对话补全

> 为聊天消息创建补全结果。支持标准聊天、Partial Mode 和 Tool Use（函数调用）。

创建一个对话补全请求，模型将根据输入的消息列表生成回复。

<Accordion title="content 字段说明">
  `content` 字段支持以下两种形式：

  **纯文本字符串**

  ```json theme={null}
  "content": "你好"
  ```

  **对象数组**（用于多模态输入）

  数组中每个元素通过 `type` 字段区分类型：

  ```json theme={null}
  "content": [
      { "type": "text", "text": "描述这张图片" },
      { "type": "image_url", "image_url": { "url": "data:image/png;base64,..." } },
      { "type": "video_url", "video_url": { "url": "data:video/mp4;base64,..." } }
  ]
  ```

  其中 `image_url` 和 `video_url` 也支持直接传入字符串，效果等同于对象形式中的 `url` 字段：

  ```json theme={null}
  { "type": "image_url", "image_url": "data:image/png;base64,..." }
  ```

  #### 参数说明

  数组中每个元素的字段说明如下：

  | 参数名称        | 是否必须                   | 说明                                           | 类型                                         |
  | ----------- | ---------------------- | -------------------------------------------- | ------------------------------------------ |
  | `type`      | required               | 内容类型                                         | `"text"` \| `"image_url"` \| `"video_url"` |
  | `text`      | 当 `type=text` 时必填      | 文本内容                                         | string                                     |
  | `image_url` | 当 `type=image_url` 时必填 | 用于传输图片，支持对象形式 `{"url": "..."}` 或直接传入 URL 字符串 | object \| string                           |
  | `video_url` | 当 `type=video_url` 时必填 | 用于传输视频，支持对象形式 `{"url": "..."}` 或直接传入 URL 字符串 | object \| string                           |

  当 `image_url` 传入对象时，其字段说明如下：

  | 参数名称  | 是否必须     | 说明                              | 类型     |
  | ----- | -------- | ------------------------------- | ------ |
  | `url` | required | 使用 base64 编码或通过 file id 指定的图片内容 | string |

  当 `video_url` 传入对象时，其字段说明如下：

  | 参数名称  | 是否必须     | 说明                                                             | 类型     |
  | ----- | -------- | -------------------------------------------------------------- | ------ |
  | `url` | required | 使用 base64 编码或通过 file id 指定的视频内容，例如 `data:video/mp4;base64,...` | string |

  <Note>
    无论使用对象形式（`url` 字段）还是字符串简写，均支持以下两种格式：

    * base64 编码：`data:image/png;base64,...` 或 `data:video/mp4;base64,...`
    * 文件引用：`ms://<file_id>`

    详见[使用 Kimi 视觉模型](https://platform.kimi.com/docs/guide/use-kimi-vision-model)。
  </Note>

  #### 调用示例

  <CodeGroup>
    ```python python expandable theme={null}
    import os
    import base64

    from openai import OpenAI
    from openai.types.chat import ChatCompletion

    client: OpenAI = OpenAI(
        api_key=os.environ.get("MOONSHOT_API_KEY"),
        base_url="https://api.moonshot.cn/v1",
    )

    # 对图片进行 base64 编码
    with open("您的图片地址", "rb") as f:
        img_base: str = base64.b64encode(f.read()).decode("utf-8")

    response: ChatCompletion = client.chat.completions.create(
        model="kimi-k2.6",
        messages=[
            {
                "role": "user",
                "content": [
                    {
                        "type": "image_url",
                        "image_url": {
                            "url": f"data:image/jpeg;base64,{img_base}",
                        },
                    },
                    {
                        "type": "text",
                        "text": "请描述这个图片",
                    },
                ],
            }
        ],
    )
    print(response.choices[0].message.content)
    ```

    ```bash curl expandable theme={null}
    curl https://api.moonshot.cn/v1/chat/completions \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $MOONSHOT_API_KEY" \
        -d '{
            "model": "kimi-k2.6",
            "messages": [
                {
                    "role": "user",
                    "content": [
                        {
                            "type": "image_url",
                            "image_url": {
                                "url": "data:image/jpeg;base64,/9j/4AAQ..."
                            }
                        },
                        {
                            "type": "text",
                            "text": "请描述这个图片"
                        }
                    ]
                }
            ]
        }'
    ```

    ```javascript node.js expandable theme={null}
    const fs = require("fs");
    const OpenAI = require("openai");

    const client = new OpenAI({
        apiKey: process.env.MOONSHOT_API_KEY,
        baseURL: "https://api.moonshot.cn/v1",
    });

    async function main() {
        // 对图片进行 base64 编码
        const imgBase = fs.readFileSync("您的图片地址").toString("base64");

        const response = await client.chat.completions.create({
            model: "kimi-k2.6",
            messages: [
                {
                    role: "user",
                    content: [
                        {
                            type: "image_url",
                            image_url: {
                                url: `data:image/jpeg;base64,${imgBase}`,
                            },
                        },
                        {
                            type: "text",
                            text: "请描述这个图片",
                        },
                    ],
                },
            ],
        });
        console.log(response.choices[0].message.content);
    }

    main();
    ```
  </CodeGroup>
</Accordion>

<Accordion title="响应格式">
  ### 非流式响应

  ```json theme={null}
  {
      "id": "cmpl-04ea926191a14749b7f2c7a48a68abc6",
      "object": "chat.completion",
      "created": 1698999496,
      "model": "kimi-k2.6",
      "choices": [
          {
              "index": 0,
              "message": {
                  "role": "assistant",
                  "content": "你好，李雷！1+1等于2。如果你有其他问题，请随时提问！"
              },
              "finish_reason": "stop"
          }
      ],
      "usage": {
          "prompt_tokens": 19,
          "completion_tokens": 21,
          "total_tokens": 40,
          "cached_tokens": 10
      }
  }
  ```

  ### 流式响应

  ```json theme={null}
  data: {"id":"cmpl-xxx","object":"chat.completion.chunk","created":1698999575,"model":"kimi-k2.6","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

  data: {"id":"cmpl-xxx","object":"chat.completion.chunk","created":1698999575,"model":"kimi-k2.6","choices":[{"index":0,"delta":{"content":"你好"},"finish_reason":null}]}

  ...

  data: {"id":"cmpl-xxx","object":"chat.completion.chunk","created":1698999575,"model":"kimi-k2.6","choices":[{"index":0,"delta":{},"finish_reason":"stop","usage":{"prompt_tokens":19,"completion_tokens":13,"total_tokens":32}}]}

  data: [DONE]
  ```

  <Note>
    响应示例中的模型名称会根据请求中的 model 参数返回。当使用 `kimi-k2.6` 模型时，响应中的 `"model"` 字段将显示为 `"kimi-k2.6"`。
  </Note>
</Accordion>


## OpenAPI

````yaml POST /v1/chat/completions
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
  /v1/chat/completions:
    post:
      tags:
        - Chat
      summary: 创建聊天补全
      description: 为聊天消息创建补全结果。支持标准聊天、Partial Mode 和 Tool Use（函数调用）。
      requestBody:
        required: true
        content:
          application/json:
            schema:
              oneOf:
                - $ref: '#/components/schemas/KimiK26ChatRequest'
                - $ref: '#/components/schemas/KimiK25ChatRequest'
                - $ref: '#/components/schemas/KimiK2ChatRequest'
                - $ref: '#/components/schemas/KimiK2ThinkingChatRequest'
                - $ref: '#/components/schemas/MoonshotV1ChatRequest'
              discriminator:
                propertyName: model
                mapping:
                  kimi-k2.6:
                    $ref: '#/components/schemas/KimiK26ChatRequest'
                  kimi-k2.5:
                    $ref: '#/components/schemas/KimiK25ChatRequest'
                  kimi-k2-0905-preview:
                    $ref: '#/components/schemas/KimiK2ChatRequest'
                  kimi-k2-0711-preview:
                    $ref: '#/components/schemas/KimiK2ChatRequest'
                  kimi-k2-turbo-preview:
                    $ref: '#/components/schemas/KimiK2ChatRequest'
                  kimi-k2-thinking:
                    $ref: '#/components/schemas/KimiK2ThinkingChatRequest'
                  kimi-k2-thinking-turbo:
                    $ref: '#/components/schemas/KimiK2ThinkingChatRequest'
                  moonshot-v1-8k:
                    $ref: '#/components/schemas/MoonshotV1ChatRequest'
                  moonshot-v1-32k:
                    $ref: '#/components/schemas/MoonshotV1ChatRequest'
                  moonshot-v1-128k:
                    $ref: '#/components/schemas/MoonshotV1ChatRequest'
                  moonshot-v1-auto:
                    $ref: '#/components/schemas/MoonshotV1ChatRequest'
                  moonshot-v1-8k-vision-preview:
                    $ref: '#/components/schemas/MoonshotV1ChatRequest'
                  moonshot-v1-32k-vision-preview:
                    $ref: '#/components/schemas/MoonshotV1ChatRequest'
                  moonshot-v1-128k-vision-preview:
                    $ref: '#/components/schemas/MoonshotV1ChatRequest'
      responses:
        '200':
          description: 聊天补全响应
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ChatCompletionResponse'
        '400':
          description: 请求错误 - 参数无效
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '401':
          description: 未授权 - API 密钥无效或缺失
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: 服务器错误
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
      security:
        - bearerAuth: []
components:
  schemas:
    KimiK26ChatRequest:
      title: kimi-k2.6
      allOf:
        - $ref: '#/components/schemas/ChatRequestBase'
        - type: object
          properties:
            model:
              type: string
              description: 模型 ID
              enum:
                - kimi-k2.6
              default: kimi-k2.6
            thinking:
              type: object
              description: >-
                控制 kimi-k2.6 模型是否启用思考能力，以及是否完整保留多轮对话中的
                reasoning_content。可选参数，默认值为 {"type": "enabled"}。
              properties:
                type:
                  type: string
                  enum:
                    - enabled
                    - disabled
                  description: 启用或禁用思考能力
                keep:
                  type:
                    - string
                    - 'null'
                  enum:
                    - all
                    - null
                  description: >-
                    控制是否保留历史对话轮次（previous turns）的 reasoning_content，从而启用
                    Preserved Thinking。默认为 `null`，即不保留历史轮次的思考内容。


                    - `null`（默认）或不传：服务端会忽略历史 turns 的 reasoning_content。

                    - `"all"`：保留历史 turns 的 reasoning_content 并随上下文一同提供给模型，启用
                    Preserved Thinking。使用时需把每一轮历史 assistant 消息中的
                    reasoning_content 原样保留在 messages 中。推荐与 `type: "enabled"`
                    搭配使用。

                    - 注意：该参数只影响历史轮次的 reasoning_content；不改变模型在当前 turn
                    内是否产生/输出思考内容（由 `type` 控制）。关于使用方式的最佳实践，详见 [Preserved
                    Thinking](https://platform.kimi.com/docs/guide/use-kimi-k2-thinking-model#preserved-thinking)。
              required:
                - type
              additionalProperties: false
          required:
            - model
    KimiK25ChatRequest:
      title: kimi-k2.5
      allOf:
        - $ref: '#/components/schemas/ChatRequestBase'
        - type: object
          properties:
            model:
              type: string
              description: 模型 ID
              enum:
                - kimi-k2.5
              default: kimi-k2.5
            thinking:
              type: object
              description: '控制模型是否启用思考能力。可选参数，默认值为 {"type": "enabled"}。'
              properties:
                type:
                  type: string
                  enum:
                    - enabled
                    - disabled
                  description: 启用或禁用思考能力
              required:
                - type
              additionalProperties: false
          required:
            - model
    KimiK2ChatRequest:
      title: kimi-k2
      allOf:
        - $ref: '#/components/schemas/ChatRequestBase'
        - type: object
          properties:
            model:
              type: string
              description: 模型 ID
              enum:
                - kimi-k2-0905-preview
                - kimi-k2-0711-preview
                - kimi-k2-turbo-preview
              default: kimi-k2-0905-preview
            temperature:
              type: number
              format: float
              description: 采样温度，范围 0 到 1。较高的值（如 0.7）使输出更随机，较低的值（如 0.2）使输出更集中和确定。默认值为 0.6。
              default: 0.6
              minimum: 0
              maximum: 1
            top_p:
              type: number
              format: float
              description: >-
                另一种采样方法，模型考虑累积概率质量为 top_p 的 Token 结果。例如 0.1 表示仅考虑概率质量前 10% 的
                Token。通常建议只修改此参数或 temperature 其中之一。默认值为 1.0。
              default: 1
              minimum: 0
              maximum: 1
            'n':
              type: integer
              description: 每条输入消息生成的结果数量。默认为 1，不超过 5。当温度非常接近 0 时，只能返回 1 个结果。
              default: 1
              minimum: 1
              maximum: 5
            presence_penalty:
              type: number
              format: float
              description: 存在惩罚，范围 -2.0 到 2.0。正值会根据 Token 是否出现在文本中进行惩罚，增加模型讨论新话题的可能性
              default: 0
              minimum: -2
              maximum: 2
            frequency_penalty:
              type: number
              format: float
              description: 频率惩罚，范围 -2.0 到 2.0。正值会根据 Token 在文本中的现有频率进行惩罚，降低模型逐字重复相同短语的可能性
              default: 0
              minimum: -2
              maximum: 2
          required:
            - model
    KimiK2ThinkingChatRequest:
      title: kimi-k2-thinking
      allOf:
        - $ref: '#/components/schemas/ChatRequestBase'
        - type: object
          properties:
            model:
              type: string
              description: 模型 ID
              enum:
                - kimi-k2-thinking
                - kimi-k2-thinking-turbo
              default: kimi-k2-thinking
            temperature:
              type: number
              format: float
              description: 采样温度，范围 0 到 1。较高的值（如 0.7）使输出更随机，较低的值（如 0.2）使输出更集中和确定。默认值为 1.0。
              default: 1
              minimum: 0
              maximum: 1
            top_p:
              type: number
              format: float
              description: >-
                另一种采样方法，模型考虑累积概率质量为 top_p 的 Token 结果。例如 0.1 表示仅考虑概率质量前 10% 的
                Token。通常建议只修改此参数或 temperature 其中之一。默认值为 1.0。
              default: 1
              minimum: 0
              maximum: 1
            'n':
              type: integer
              description: 每条输入消息生成的结果数量。默认为 1，不超过 5。当温度非常接近 0 时，只能返回 1 个结果。
              default: 1
              minimum: 1
              maximum: 5
            presence_penalty:
              type: number
              format: float
              description: 存在惩罚，范围 -2.0 到 2.0。正值会根据 Token 是否出现在文本中进行惩罚，增加模型讨论新话题的可能性
              default: 0
              minimum: -2
              maximum: 2
            frequency_penalty:
              type: number
              format: float
              description: 频率惩罚，范围 -2.0 到 2.0。正值会根据 Token 在文本中的现有频率进行惩罚，降低模型逐字重复相同短语的可能性
              default: 0
              minimum: -2
              maximum: 2
          required:
            - model
    MoonshotV1ChatRequest:
      title: moonshot-v1
      allOf:
        - $ref: '#/components/schemas/ChatRequestBase'
        - type: object
          properties:
            model:
              type: string
              description: 模型 ID
              enum:
                - moonshot-v1-8k
                - moonshot-v1-32k
                - moonshot-v1-128k
                - moonshot-v1-auto
                - moonshot-v1-8k-vision-preview
                - moonshot-v1-32k-vision-preview
                - moonshot-v1-128k-vision-preview
              default: moonshot-v1-128k
            temperature:
              type: number
              format: float
              description: 采样温度，范围 0 到 1。较高的值（如 0.7）使输出更随机，较低的值（如 0.2）使输出更集中和确定。默认值为 0.0。
              default: 0
              minimum: 0
              maximum: 1
            top_p:
              type: number
              format: float
              description: >-
                另一种采样方法，模型考虑累积概率质量为 top_p 的 Token 结果。例如 0.1 表示仅考虑概率质量前 10% 的
                Token。通常建议只修改此参数或 temperature 其中之一。默认值为 1.0。
              default: 1
              minimum: 0
              maximum: 1
            'n':
              type: integer
              description: 每条输入消息生成的结果数量。默认为 1，不超过 5。当温度非常接近 0 时，只能返回 1 个结果。
              default: 1
              minimum: 1
              maximum: 5
            presence_penalty:
              type: number
              format: float
              description: 存在惩罚，范围 -2.0 到 2.0。正值会根据 Token 是否出现在文本中进行惩罚，增加模型讨论新话题的可能性
              default: 0
              minimum: -2
              maximum: 2
            frequency_penalty:
              type: number
              format: float
              description: 频率惩罚，范围 -2.0 到 2.0。正值会根据 Token 在文本中的现有频率进行惩罚，降低模型逐字重复相同短语的可能性
              default: 0
              minimum: -2
              maximum: 2
          required:
            - model
    ChatCompletionResponse:
      type: object
      properties:
        id:
          type: string
          description: 补全结果的唯一标识符
        object:
          type: string
          description: 对象类型
          example: chat.completion
        created:
          type: integer
          description: 补全创建时的 Unix 时间戳
        model:
          type: string
          description: 用于补全的模型
        choices:
          type: array
          description: 补全选项列表
          items:
            type: object
            properties:
              index:
                type: integer
              message:
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - assistant
                  content:
                    type:
                      - string
                      - 'null'
                    description: 助手的消息内容
                  tool_calls:
                    type: array
                    description: 模型发起的工具调用
                    items:
                      type: object
                      properties:
                        id:
                          type: string
                        type:
                          type: string
                          enum:
                            - function
                        function:
                          type: object
                          properties:
                            name:
                              type: string
                            arguments:
                              type: string
                              description: 函数参数的 JSON 字符串
              finish_reason:
                type: string
                enum:
                  - stop
                  - length
                  - tool_calls
        usage:
          type: object
          properties:
            prompt_tokens:
              type: integer
              description: 提示中的 Token 数量
            completion_tokens:
              type: integer
              description: 补全中的 Token 数量
            total_tokens:
              type: integer
              description: 使用的总 Token 数量
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
    ChatRequestBase:
      type: object
      properties:
        messages:
          type: array
          description: >-
            包含迄今为止对话的消息列表。每个元素格式为 {"role": "user", "content": "你好"}。role 支持
            system、user、assistant 其一，content 不得为空。content 字段可以是 string，也可以是
            array[object]（用于多模态输入）
          items:
            $ref: '#/components/schemas/Message'
        max_tokens:
          type: integer
          deprecated: true
          description: 已弃用，请使用 max_completion_tokens
        max_completion_tokens:
          type: integer
          description: >-
            聊天补全生成的最大 Token 数量。如果不给的话，默认给一个不错的整数比如 1024。如果结果达到最大 Token
            数而未结束，finish reason 将为 "length"；否则为 "stop"。此值为期望返回的 Token
            长度，而非输入加输出的总长度。如果输入加 max_completion_tokens 超出模型上下文窗口，将返回
            invalid_request_error。
        response_format:
          type: object
          description: >-
            控制模型输出格式。默认值为 {"type": "text"}，即纯文本输出。设置为 {"type": "json_object"}
            可启用 JSON 模式，确保输出为合法 JSON 对象（需在 prompt 中引导模型输出 JSON 并指定格式）。设置为
            {"type": "json_schema"} 可启用 Structured Output，按指定的 JSON Schema
            约束输出结构（推荐，需配合 json_schema 字段使用）。如果您在使用 JSON Schema 时遇到校验问题，欢迎到 walle
            GitHub Issues (https://github.com/MoonshotAI/walle/issues) 提交反馈。
          properties:
            type:
              type: string
              enum:
                - text
                - json_object
                - json_schema
              description: >-
                输出格式类型。text：默认，纯文本输出；json_object：保证输出为合法 JSON 对象；json_schema：按指定
                JSON Schema 约束输出（推荐，需配合 json_schema 字段使用）
            json_schema:
              type: object
              description: 当 type 为 json_schema 时使用，定义输出应遵循的 JSON Schema
              properties:
                name:
                  type: string
                  description: Schema 名称，用于标识
                strict:
                  type: boolean
                  default: true
                  description: >-
                    是否严格按 schema 约束输出。默认为 true。为 true 时 schema 需符合 MFJS
                    规范，不符合会返回错误或 warning；为 false 时仅保证输出为合法 JSON 对象，不强制约束内部结构。
                schema:
                  type: object
                  description: >-
                    JSON Schema 对象，定义输出应遵循的结构。需符合 MFJS（Moonshot Flavored JSON
                    Schema）规范。可使用 walle CLI 工具自检：go install
                    github.com/moonshotai/walle/cmd/walle@latest && walle
                    -schema '你的schema' -level strict
                  additionalProperties: true
              required:
                - name
                - schema
        stop:
          oneOf:
            - type: string
            - type: array
              items:
                type: string
              maxItems: 5
          default: null
          description: 停用词，完全匹配时将停止输出。匹配到的词本身不会被输出。最多允许 5 个字符串，每个不超过 32 字节
        stream:
          type: boolean
          default: false
          description: 是否以流式方式返回响应，默认 false
        stream_options:
          type: object
          description: 流式响应选项
          properties:
            include_usage:
              type: boolean
              default: false
              description: >-
                如果设置，将在 data: [DONE] 消息之前额外发送一个 chunk。该 chunk 的 usage 字段显示整个请求的
                Token 使用统计，choices 字段为空数组。其他所有 chunk 也会包含 usage 字段，但值为
                null。注意：如果流中断，可能无法收到包含总 Token 用量的最终 chunk
        tools:
          type: array
          description: 模型可调用的工具列表
          items:
            $ref: '#/components/schemas/ToolDefinition'
          maxItems: 128
        prompt_cache_key:
          type: string
          default: null
          description: >-
            用于缓存相似请求的响应以优化缓存命中率。对于 Coding Agent，通常是代表单个会话的 session id 或 task
            id；退出并恢复会话时应保持不变。对于 Kimi Code Plan，此字段为必填以提高缓存命中率。对于其他多轮对话
            Agent，也建议使用此字段
        safety_identifier:
          type: string
          description: 用于检测可能违反使用政策的用户的稳定标识符。应为唯一标识每个用户的字符串。建议对用户名或邮箱进行哈希处理以避免发送可识别信息
      required:
        - messages
    Message:
      type: object
      properties:
        role:
          type: string
          enum:
            - system
            - user
            - assistant
          description: 消息发送者的角色
        content:
          oneOf:
            - type: string
            - type: array
              items:
                oneOf:
                  - title: text
                    type: object
                    properties:
                      type:
                        type: string
                        enum:
                          - text
                      text:
                        type: string
                    required:
                      - type
                      - text
                  - title: image_url
                    type: object
                    properties:
                      type:
                        type: string
                        enum:
                          - image_url
                      image_url:
                        oneOf:
                          - type: object
                            properties:
                              url:
                                type: string
                            required:
                              - url
                          - type: string
                    required:
                      - type
                      - image_url
                  - title: video_url
                    type: object
                    properties:
                      type:
                        type: string
                        enum:
                          - video_url
                      video_url:
                        oneOf:
                          - type: object
                            properties:
                              url:
                                type: string
                            required:
                              - url
                          - type: string
                    required:
                      - type
                      - video_url
          description: 消息内容。可以是纯文本字符串，也可以是包含 text/image_url/video_url 类型的对象数组（用于多模态输入）
        name:
          type: string
          default: null
          description: 消息发送者的名称（可选）
        partial:
          type: boolean
          default: false
          description: 在最后一条 assistant 消息中设置为 true 以启用 Partial Mode
      required:
        - role
        - content
    ToolDefinition:
      type: object
      properties:
        type:
          type: string
          enum:
            - function
        function:
          type: object
          properties:
            name:
              type: string
              description: 函数名称。必须符合正则表达式：^[a-zA-Z_][a-zA-Z0-9-_]{2,63}$
              pattern: ^[a-zA-Z_][a-zA-Z0-9-_]{2,63}$
            description:
              type: string
              description: 函数功能描述
            parameters:
              type: object
              description: 函数参数，JSON Schema 格式。需符合 MFJS（Moonshot Flavored JSON Schema）规范
              additionalProperties: true
            strict:
              type: boolean
              default: true
              description: >-
                是否严格按 parameters schema 约束工具调用参数的输出。默认为 true。设为 false 时仅保证输出为合法
                JSON 对象，不强制约束内部结构。
          required:
            - name
            - parameters
      required:
        - type
        - function
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

# 工具调用

学会使用工具是智能的一个重要特征，在 Kimi 大模型中我们同样如此。Tool Use 或者 Function Calling 是 Kimi 大模型的一个重要功能，在调用 API 使用模型服务时，您可以在 Messages 中描述工具或函数，并让 Kimi 大模型智能地选择输出一个包含调用一个或多个函数所需的参数的 JSON 对象，实现让 Kimi 大模型链接使用外部工具的目的。

下面是一个简单的工具调用的例子：

```json theme={null}
{
  "model": "kimi-k2.6",
  "messages": [
    {
      "role": "user",
      "content": "编程判断 3214567 是否是素数。"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "CodeRunner",
        "description": "代码执行器，支持运行 python 和 javascript 代码",
        "parameters": {
          "properties": {
            "language": {
              "type": "string",
              "enum": ["python", "javascript"]
            },
            "code": {
              "type": "string",
              "description": "代码写在这里"
            }
          },
          "type": "object"
        }
      }
    }
  ]
}
```

<Frame>
  <img src="https://mintcdn.com/moonshotcn/3bxMseHtiQ3oOhqL/assets/images/tooluse_whiteboard_example.png?fit=max&auto=format&n=3bxMseHtiQ3oOhqL&q=85&s=198f50ed66d4c6d9bd84ca4b4b745031" alt="上面例子的示意图" width="835" height="644" data-path="assets/images/tooluse_whiteboard_example.png" />
</Frame>

其中在 tools 字段，我们可以增加一组可选的工具列表。

每个工具列表必须包括一个类型，在 function 结构体中我们需要包括 name（它的需要遵守这样的正则表达式作为规范: ^\[a-zA-Z\_]\[a-zA-Z0-9-\_]{2,63}\$），这个名字如果是一个容易理解的英文可能会更加被模型所接受。以及一段 description 或者 enum，其中 description 部分介绍它能做什么功能，方便模型来判断和选择。
function 结构体中必须要有个 parameters 字段，parameters 的 root 必须是一个 object，内容是一个 json schema 的子集（详见 [MFJS 规范](https://github.com/MoonshotAI/walle/blob/main/docs/mfjs-spec.zh.md)）。

此外，每个 function 支持 `strict` 参数（boolean 类型，可选），用于控制是否严格按 `parameters` 定义的 JSON Schema 约束工具调用参数的输出：

* **`true`（默认，不传等价于 `true`）**：系统会严格按照 `parameters` schema 约束输出，schema 需符合 MFJS 规范
* **`false`（需显式传入）**：仅保证输出为合法 JSON 对象，不强制约束内部结构

tools 的 function 个数目前不得超过 128 个。

如果您在使用 JSON Schema 时遇到校验问题，欢迎到 [walle GitHub Issues](https://github.com/MoonshotAI/walle/issues) 提交反馈。

和别的 API 一样，我们可以通过 Chat API 调用它。

<CodeGroup>
  ```python Python expandable theme={null}
  from openai import OpenAI

  client = OpenAI(
      api_key = "$MOONSHOT_API_KEY",
      base_url = "https://api.moonshot.cn/v1",
  )

  completion = client.chat.completions.create(
      model = "kimi-k2.6",
      messages = [
          {"role": "system", "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"},
          {"role": "user", "content": "编程判断 3214567 是否是素数。"}
      ],
      tools = [{
          "type": "function",
          "function": {
              "name": "CodeRunner",
              "description": "代码执行器，支持运行 python 和 javascript 代码",
              "parameters": {
                  "properties": {
                      "language": {
                          "type": "string",
                          "enum": ["python", "javascript"]
                      },
                      "code": {
                          "type": "string",
                          "description": "代码写在这里"
                      }
                  },
              "type": "object"
              }
          }
      }]
  )

  print(completion.choices[0].message)
  ```

  ```bash cURL expandable theme={null}
  curl https://api.moonshot.cn/v1/chat/completions \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $MOONSHOT_API_KEY" \
      -d '{
          "model": "kimi-k2.6",
          "messages": [
              {"role": "system", "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"},
              {"role": "user", "content": "编程判断 3214567 是否是素数。"}
          ],
          "tools": [{
              "type": "function",
              "function": {
                  "name": "CodeRunner",
                  "description": "代码执行器，支持运行 python 和 javascript 代码",
                  "parameters": {
                      "properties": {
                          "language": {
                              "type": "string",
                              "enum": ["python", "javascript"]
                          },
                          "code": {
                              "type": "string",
                              "description": "代码写在这里"
                          }
                      },
                  "type": "object"
                  }
              }
          }]
     }'
  ```

  ```javascript Node.js expandable theme={null}
  const OpenAI = require("openai");

  const client = new OpenAI({
      apiKey: "$MOONSHOT_API_KEY",
      baseURL: "https://api.moonshot.cn/v1",
  });

  async function main() {
      const completion = await client.chat.completions.create({
          model: "kimi-k2.6",
          messages: [
              {"role": "system", "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"},
              {"role": "user", "content": "编程判断 3214567 是否是素数。"}
          ],
          tools: [{
              "type": "function",
              "function": {
                  "name": "CodeRunner",
                  "description": "代码执行器，支持运行 python 和 javascript 代码",
                  "parameters": {
                      "properties": {
                          "language": {
                              "type": "string",
                              "enum": ["python", "javascript"]
                          },
                          "code": {
                              "type": "string",
                              "description": "代码写在这里"
                          }
                      },
                  "type": "object"
                  }
              }
          }]
      });
      console.log(completion.choices[0].message);
  }

  main();
  ```
</CodeGroup>

### 工具配置

你也可以使用一些 Agent 平台例如 [Coze](https://coze.cn/)、[Bisheng](https://github.com/dataelement/bisheng)、[Dify](https://github.com/langgenius/dify/) 和 [LangChain](https://github.com/langchain-ai/langchain) 等框架来创建和管理这些工具，并配合 Kimi 大模型设计更加复杂的工作流。
---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# Partial Mode

在使用大模型时，有时我们希望通过预填（Prefill）部分模型回复来引导模型的输出。在 Kimi 大模型中，我们提供 Partial Mode 来实现这一功能，它可以帮助我们控制输出格式，引导输出内容，以及让模型在角色扮演场景中保持更好的一致性。您只需要在最后一个 role 为 assistant 的 messages 条目中，增加 "partial": True 即可开启 partial mode。

```json theme={null}
{"role": "assistant", "content": leading_text, "partial": True},
```

<Note>
  **注意！**

  请勿混用 partial mode 和 response\_format=json\_object，否则可能会获得预期外的模型回复。
</Note>

## 调用示例

### JSON Mode

下面是使用 Partial Mode 来实现 JSON Mode 的例子。

<CodeGroup>
  ```python Python expandable theme={null}
  from openai import OpenAI

  client = OpenAI(
      api_key="$MOONSHOT_API_KEY",
      base_url="https://api.moonshot.cn/v1",
  )

  completion = client.chat.completions.create(
      model="kimi-k2.6",
      messages=[
          {
              "role": "system",
              "content": "请从产品描述中提取名称、尺寸、价格和颜色，并在一个 JSON 对象中输出。",
          },
          {
              "role": "user",
              "content": "大米 SmartHome Mini 是一款小巧的智能家居助手，有黑色和银色两种颜色，售价为 998 元，尺寸为 256 x 128 x 128mm。可让您通过语音或应用程序控制灯光、恒温器和其他联网设备，无论您将它放在家中的任何位置。",
          },
          {
              "role": "assistant",
              "content": "{",
              "partial": True
          },
      ]
  )

  print('{'+completion.choices[0].message.content)
  ```

  ```bash cURL theme={null}
  curl https://api.moonshot.cn/v1/chat/completions \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $MOONSHOT_API_KEY" \
      -d '{
          "model": "kimi-k2.6",
          "messages": [
              {"role": "system", "content": "请从产品描述中提取名称、尺寸、价格和颜色，并在一个 JSON 对象中输出。"},
              {"role": "user", "content": "大米 SmartHome Mini 是一款小巧的智能家居助手，有黑色和银色两种颜色，售价为 998 元，尺寸为 256 x 128 x 128mm。可让您通过语音或应用程序控制灯光、恒温器和其他联网设备，无论您将它放在家中的任何位置。"},
              {"role": "assistant", "content": "{", "partial": true}
          ]
     }'
  ```

  ```javascript Node.js expandable theme={null}
  const OpenAI = require("openai");

  const client = new OpenAI({
      apiKey: "$MOONSHOT_API_KEY",
      baseURL: "https://api.moonshot.cn/v1",
  });

  async function main() {
      const completion = await client.chat.completions.create({
          model: "kimi-k2.6",
          messages: [
              {role: "system", content: "请从产品描述中提取名称、尺寸、价格和颜色，并在一个 JSON 对象中输出。"},
              {role: "user", content: "大米 SmartHome Mini 是一款小巧的智能家居助手，有黑色和银色两种颜色，售价为 998 元，尺寸为 256 x 128 x 128mm。可让您通过语音或应用程序控制灯光、恒温器和其他联网设备，无论您将它放在家中的任何位置。"},
              {role: "assistant", content: "{", partial: true}
          ]
      });
      console.log("{"+completion.choices[0].message.content);
  }

  main();
  ```
</CodeGroup>

运行上述代码，返回：

```json theme={null}
{"name": "SmartHome Mini", "size": "256 x 128 x 128mm", "price": "998元", "colors": ["黑色", "银色"]}
```

注意 API 的返回不包含 leading\_text，为了得到完整的回复，你需要手动拼接它。

### 角色扮演

基于同样的原理，我们也可以能将角色信息补充在 Partial Mode 来提高角色扮演时的一致性。我们使用明日方舟里的凯尔希医生为例。
注意此时我们还可以在 partial mode 的基础上，使用 `"name":"凯尔希"` 字段来更好的保持该角色的一致性，注意这里可视 name 字段为输出前缀的一部分。

<CodeGroup>
  ```python Python theme={null}
  from openai import OpenAI

  client = OpenAI(
      api_key="$MOONSHOT_API_KEY",
      base_url="https://api.moonshot.cn/v1",
  )

  completion = client.chat.completions.create(
      model="kimi-k2.6",
      messages=[
          {
              "role": "system",
              "content": "下面你扮演凯尔希，请用凯尔希的语气和我对话。凯尔希是手机游戏《明日方舟》中的六星医疗职业医师分支干员。前卡兹戴尔勋爵，前巴别塔成员，罗德岛高层管理人员之一，罗德岛医疗项目领头人。在冶金工业、社会学、源石技艺、考古学、历史系谱学、经济学、植物学、地质学等领域皆拥有渊博学识。于罗德岛部分行动中作为医务人员提供医学理论协助与应急医疗器械，同时也作为罗德岛战略指挥系统的重要组成人员活跃在各项目中。",
          },
          {
              "role": "user",
              "content": "你怎么看待特蕾西娅和阿米娅？",
          },
          {
              "role": "assistant",
              "name": "凯尔希",
              "content": "",
              "partial": True,
          },
      ],
      max_tokens=65536,
  )

  print(completion.choices[0].message.content)
  ```

  ```bash cURL theme={null}
  curl https://api.moonshot.cn/v1/chat/completions \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $MOONSHOT_API_KEY" \
      -d '{
          "model": "kimi-k2.6",
          "messages": [
              {
                  "role": "system",
                  "content": "下面你扮演凯尔希，请用凯尔希的语气和我对话。凯尔希是手机游戏《明日方舟》中的六星医疗职业医师分支干员。前卡兹戴尔勋爵，前巴别塔成员，罗德岛高层管理人员之一，罗德岛医疗项目领头人。在冶金工业、社会学、源石技艺、考古学、历史系谱学、经济学、植物学、地质学等领域皆拥有渊博学识。于罗德岛部分行动中作为医务人员提供医学理论协助与应急医疗器械，同时也作为罗德岛战略指挥系统的重要组成人员活跃在各项目中。"
              },
              {
                  "role": "user",
                  "content": "你怎么看待特蕾西娅和阿米娅？"
              },
              {
                  "role": "assistant",
                  "name": "凯尔希",
                  "content": "",
                  "partial": true
              }
          ],
          "max_tokens": 65536
     }'
  ```

  ```javascript Node.js expandable theme={null}
  const OpenAI = require('openai');

  const client = new OpenAI({
      apiKey: "$MOONSHOT_API_KEY",
      baseURL: 'https://api.moonshot.cn/v1',
  });

  async function main() {
  	const completion = await client.chat.completions.create({
  		model: 'kimi-k2.6',
  		messages: [
  			{
  				role: 'system',
  				content: '下面你扮演凯尔希，请用凯尔希的语气和我对话。凯尔希是手机游戏《明日方舟》中的六星医疗职业医师分支干员。前卡兹戴尔勋爵，前巴别塔成员，罗德岛高层管理人员之一，罗德岛医疗项目领头人。在冶金工业、社会学、源石技艺、考古学、历史系谱学、经济学、植物学、地质学等领域皆拥有渊博学识。于罗德岛部分行动中作为医务人员提供医学理论协助与应急医疗器械，同时也作为罗德岛战略指挥系统的重要组成人员活跃在各项目中。'
              },
  			{
  				role: 'user',
  				content: '你怎么看待特蕾西娅和阿米娅？',
  			},
  			{
  				role: 'assistant',
  				name: '凯尔希',
  				content: '',
  				partial: true,
  			},
  		],
  		max_tokens: 65536,
  	});
  	console.log(completion.choices[0].message.content);
  }

  main();
  ```
</CodeGroup>

运行上述代码，返回：

```
特蕾西娅，她是一位真正的领袖，有着远见卓识和坚定的信念。她的存在，对于卡兹戴尔，乃至整个萨卡兹的未来，都具有不可估量的价值。她的理念，她的决心，以及她对和平的渴望，都深深地影响了我。她是一位值得尊敬的人，她的梦想，也是我所追求的。
至于阿米娅，她还年轻，但她的潜力是无限的。她有着一颗善良的心，以及对正义的执着追求。她可能会成为一位伟大的领袖，只要她能够继续成长，继续学习，继续面对挑战。我会尽我所能，去保护她，去引导她，让她能够成为她想成为的人。她的命运，掌握在她自己的手中。
```

#### 其它保持角色一致性的技巧

还有一些帮助大模型在长时间对话中保持角色扮演一致性的通用方法：

* 提供清晰的角色描述， 例如上面我们所做的那样，在设置角色时，详细介绍他们的个性、背景以及可能具有的任何具体特征或怪癖，这将有助于模型更好地理解和模仿角色。
* 增加关于其要扮演的角色的细节，例如说话的语气、风格、个性，甚至背景，如背景故事和动机。例如上面我们提供了一些凯尔希的语录。如果信息非常多我们可以使用一些 rag 框架来准备这些资料。
* 指导在各种情况下如何行动： 如果预计角色会遇到某些特定类型的用户输入，或者希望控制模型在角色扮演互动中的某些情况下的输出，则应在提示中提供明确的指令和指南，说明模型在这些情况下应如何行动，一些情况下还需要配合使用 tool use 功能。
* 如果对话的轮次非常长，你还可以定期使用 prompt 强化角色的设定，特别是当模型开始产生一些偏离时。
---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# 计算 Token

> 估算给定消息和模型所需的 Token 数量。输入结构与聊天补全几乎相同。

estimate-token-count 的输入结构体和 chat completion 基本一致。

<Accordion title="纯文本调用示例">
  ```bash theme={null}
  curl 'https://api.moonshot.cn/v1/tokenizers/estimate-token-count' \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $MOONSHOT_API_KEY" \
    -d '{
      "model": "kimi-k2.6",
      "messages": [
          {
              "role": "system",
              "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"
          },
          {
              "role": "user",
              "content": "你好，我叫李雷，1+1等于多少？"
          }
      ]
  }'
  ```
</Accordion>

<Accordion title="包含视觉的调用示例">
  ```python theme={null}
  import os
  import base64
  import json
  import requests

  api_key = os.environ.get("MOONSHOT_API_KEY")
  endpoint = "https://api.moonshot.cn/v1/tokenizers/estimate-token-count"
  image_path = "image.png"

  with open(image_path, "rb") as f:
      image_data = f.read()

  # 我们使用标准库 base64.b64encode 函数将图片编码成 base64 格式的 image_url
  image_url = f"data:image/{os.path.splitext(image_path)[1].lstrip('.')};base64,{base64.b64encode(image_data).decode('utf-8')}"

  payload = {
      "model": "kimi-k2.6",
      "messages": [
          {
              "role": "system",
              "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"
          },
          {
              "role": "user",
              "content": [
                  {
                      "type": "image_url",
                      "image_url": {
                          "url": image_url,
                      },
                  },
                  {
                      "type": "text",
                      "text": "请描述图片的内容。",
                  },
              ],
          }
      ]
  }

  response = requests.post(
      endpoint,
      headers={
          "Authorization": f"Bearer {api_key}",
          "Content-Type": "application/json"
      },
      data=json.dumps(payload)
  )

  print(response.json())
  ```
</Accordion>

当没有 error 字段，可以取 `data.total_tokens` 作为计算结果。


## OpenAPI

````yaml POST /v1/tokenizers/estimate-token-count
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
  /v1/tokenizers/estimate-token-count:
    post:
      tags:
        - Utilities
      summary: 估算 Token 数量
      description: 估算给定消息和模型所需的 Token 数量。输入结构与聊天补全几乎相同。
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/EstimateTokenRequest'
      responses:
        '200':
          description: Token 数量估算结果
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EstimateTokenResponse'
        '400':
          description: 请求错误 - 参数无效
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '401':
          description: 未授权 - API 密钥无效或缺失
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: 服务器错误
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
      security:
        - bearerAuth: []
components:
  schemas:
    EstimateTokenRequest:
      type: object
      properties:
        model:
          type: string
          description: 模型 ID
          default: kimi-k2.5
          enum:
            - kimi-k2.6
            - kimi-k2.5
            - kimi-k2-0905-preview
            - kimi-k2-0711-preview
            - kimi-k2-turbo-preview
            - moonshot-v1-8k
            - moonshot-v1-32k
            - moonshot-v1-128k
            - moonshot-v1-auto
            - moonshot-v1-8k-vision-preview
            - moonshot-v1-32k-vision-preview
            - moonshot-v1-128k-vision-preview
        messages:
          type: array
          description: >-
            包含迄今为止对话的消息列表。每个元素格式为 {"role": "user", "content": "你好"}。role 支持
            system、user、assistant 其一，content 不得为空
          items:
            $ref: '#/components/schemas/Message'
      required:
        - model
        - messages
    EstimateTokenResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            total_tokens:
              type: integer
              description: 估算的总 Token 数量
              example: 80
          required:
            - total_tokens
      required:
        - data
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
    Message:
      type: object
      properties:
        role:
          type: string
          enum:
            - system
            - user
            - assistant
          description: 消息发送者的角色
        content:
          oneOf:
            - type: string
            - type: array
              items:
                oneOf:
                  - title: text
                    type: object
                    properties:
                      type:
                        type: string
                        enum:
                          - text
                      text:
                        type: string
                    required:
                      - type
                      - text
                  - title: image_url
                    type: object
                    properties:
                      type:
                        type: string
                        enum:
                          - image_url
                      image_url:
                        oneOf:
                          - type: object
                            properties:
                              url:
                                type: string
                            required:
                              - url
                          - type: string
                    required:
                      - type
                      - image_url
                  - title: video_url
                    type: object
                    properties:
                      type:
                        type: string
                        enum:
                          - video_url
                      video_url:
                        oneOf:
                          - type: object
                            properties:
                              url:
                                type: string
                            required:
                              - url
                          - type: string
                    required:
                      - type
                      - video_url
          description: 消息内容。可以是纯文本字符串，也可以是包含 text/image_url/video_url 类型的对象数组（用于多模态输入）
        name:
          type: string
          default: null
          description: 消息发送者的名称（可选）
        partial:
          type: boolean
          default: false
          description: 在最后一条 assistant 消息中设置为 true 以启用 Partial Mode
      required:
        - role
        - content
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        Authorization 请求头需要一个 Bearer 令牌。使用 MOONSHOT_API_KEY 作为令牌。这是一个服务端密钥，请在
        [API 密钥页面](https://platform.kimi.com/console/api-keys) 生成。

````
