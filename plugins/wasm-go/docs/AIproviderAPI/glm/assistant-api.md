# GLM 助理 API

- Category: 助理-api
- Source root: https://docs.bigmodel.cn/api-reference/助理-api
- Pages: 3
- Fetched on: 2026-04-23
- Fetch failures: 0

## Sources

- https://docs.bigmodel.cn/api-reference/助理-api/助手会话列表
- https://docs.bigmodel.cn/api-reference/助理-api/助手列表
- https://docs.bigmodel.cn/api-reference/助理-api/助手对话

---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 助手会话列表

> 查询指定智能体助手的会话列表，支持分页查询。



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/assistant/conversation/list
openapi: 3.0.1
info:
  title: ZHIPU AI API
  description: ZHIPU AI 接口提供强大的 AI 能力，包括聊天对话、工具调用和视频生成。
  license:
    name: ZHIPU AI 开发者协议和政策
    url: https://chat.z.ai/legal-agreement/terms-of-service
  version: 1.0.0
  contact:
    name: Z.AI 开发者
    url: https://chat.z.ai/legal-agreement/privacy-policy
    email: user_feedback@z.ai
servers:
  - url: https://open.bigmodel.cn/api/
    description: 开放平台服务
security:
  - bearerAuth: []
tags:
  - name: 模型 API
    description: Chat API
  - name: 工具 API
    description: Web Search API
  - name: Agent API
    description: Agent API
  - name: 文件 API
    description: File API
  - name: 知识库 API
    description: Knowledge API
  - name: 实时 API
    description: Realtime API
  - name: 批处理 API
    description: Batch API
  - name: 助理 API
    description: Assistant API
  - name: 智能体 API（旧）
    description: QingLiu Agent API
paths:
  /paas/v4/assistant/conversation/list:
    post:
      tags:
        - 助理 API
      summary: 助手会话列表
      description: 查询指定智能体助手的会话列表，支持分页查询。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ConversationListRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                type: object
                properties:
                  success:
                    type: boolean
                    example: true
                  code:
                    type: integer
                    example: 200
                  msg:
                    type: string
                    example: 操作成功
                  data:
                    $ref: '#/components/schemas/ConversationListResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
      deprecated: true
components:
  schemas:
    ConversationListRequest:
      type: object
      required:
        - assistant_id
      properties:
        assistant_id:
          type: string
          description: 智能体ID
          enum:
            - 65940acff94777010aa6b796
            - 65a265419d72d299a9230616
            - 664dd7bd5bb3a13ba0f81668
            - 664e0cade018d633146de0d2
            - 6654898292788e88ce9e7f4c
            - 66437ef3d920bdc5c60f338e
            - 659e54b1b8006379b4b2abd6
            - 65d2f07bb2c10188f885bd89
            - 663058948bb259b7e8a22730
            - 65a393b3619c6f13586246cd
            - 65b356af6924a59d52832e54
            - 668fdd45405f2e3c9f71f832
          x-enum-varnames:
            - ChatGLM（官方）
            - 数据分析（官方）
            - 复杂流程图（官方）
            - 思维导图 MindMap（官方）
            - 提示词工程师（官方）
            - AI画图（官方）
            - AI搜索（官方）
            - PPT助手（官方）
            - arXiv论文速读（官方）
            - 程序员助手Sam（官方）
            - 网文写手（官方）
            - 英语语法助手（官方）
          x-enumNames:
            - ChatGLM（官方）
            - 数据分析（官方）
            - 复杂流程图（官方）
            - 思维导图 MindMap（官方）
            - 提示词工程师（官方）
            - AI画图（官方）
            - AI搜索（官方）
            - PPT助手（官方）
            - arXiv论文速读（官方）
            - 程序员助手Sam（官方）
            - 网文写手（官方）
            - 英语语法助手（官方）
          x-enum-descriptions:
            - ChatGLM（官方） - 嗨~ 我是清言，超开心遇见你！😺 你最近有什么好玩的事情想和我分享吗？
            - 数据分析（官方） - 通过分析用户上传文件或数据说明，帮助用户分析数据并提供图表化。也可通过简单的编码完成文件处理的工作。
            - 复杂流程图（官方） - 人人都能掌握的流程图工具，用五秒钟做一张流程图卷到同事。
            - 思维导图 MindMap（官方） - 告别整理烦恼，任何复杂概念秒变脑图。
            - 提示词工程师（官方） - 人人都是提示词工程师，超强结构化提示词专家，一键改写提示词。
            - AI画图（官方） - 让想象力自由飞翔，你的专属绘画伙伴，画到停不下来。
            - AI搜索（官方） - 连接全网内容，精准搜索，快速分析并总结的智能助手。
            - PPT助手（官方） - 超实用的PPT生成器，支持手动编辑大纲、自动填充章节内容。
            - arXiv论文速读（官方） - 深度解析arXiv论文，让你快速掌握研究动态，节省宝贵时间。
            - 程序员助手Sam（官方） - "编程开发知识搜索引擎"，能帮助程序员解决日常问题。
            - 网文写手（官方） - 大神写作秘诀：一套模板不断重复。
            - 英语语法助手（官方） - 输入单词查询；输入句子检查语法，和语法解释。
          default: 65940acff94777010aa6b796
        page:
          type: integer
          description: 页码，从1开始
          minimum: 1
          default: 1
          example: 1
        page_size:
          type: integer
          description: 每页大小
          minimum: 1
          maximum: 100
          default: 5
          example: 10
    ConversationListResponse:
      type: object
      properties:
        assistant_id:
          type: string
          description: 智能体`ID`
          example: 65940acff94777010aa6b796
        conversation_list:
          type: array
          description: 会话列表
          items:
            type: object
            properties:
              id:
                type: string
                description: 会话`ID`
                example: conv_123456
              assistant_id:
                type: string
                description: 智能体`ID`
                example: 65940acff94777010aa6b796
              create_time:
                type: string
                description: 创建时间
                example: '2024-01-01T10:00:00Z'
              update_time:
                type: string
                description: 更新时间
                example: '2024-01-01T10:30:00Z'
              usage:
                type: object
                description: 使用统计
                properties:
                  prompt_tokens:
                    type: integer
                    description: 输入`token`数
                  completion_tokens:
                    type: integer
                    description: 输出`token`数
                  total_tokens:
                    type: integer
                    description: 总`token`数
            required:
              - id
              - assistant_id
        has_more:
          type: boolean
          description: 是否还有更多数据
          example: true
      required:
        - assistant_id
        - conversation_list
        - has_more
    Error:
      type: object
      properties:
        error:
          required:
            - code
            - message
          type: object
          properties:
            code:
              type: string
            message:
              type: string
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````
---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 助手列表

> 查询指定的智能体助手列表信息，包括智能体助手的详细配置、工具和元数据。



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/assistant/list
openapi: 3.0.1
info:
  title: ZHIPU AI API
  description: ZHIPU AI 接口提供强大的 AI 能力，包括聊天对话、工具调用和视频生成。
  license:
    name: ZHIPU AI 开发者协议和政策
    url: https://chat.z.ai/legal-agreement/terms-of-service
  version: 1.0.0
  contact:
    name: Z.AI 开发者
    url: https://chat.z.ai/legal-agreement/privacy-policy
    email: user_feedback@z.ai
servers:
  - url: https://open.bigmodel.cn/api/
    description: 开放平台服务
security:
  - bearerAuth: []
tags:
  - name: 模型 API
    description: Chat API
  - name: 工具 API
    description: Web Search API
  - name: Agent API
    description: Agent API
  - name: 文件 API
    description: File API
  - name: 知识库 API
    description: Knowledge API
  - name: 实时 API
    description: Realtime API
  - name: 批处理 API
    description: Batch API
  - name: 助理 API
    description: Assistant API
  - name: 智能体 API（旧）
    description: QingLiu Agent API
paths:
  /paas/v4/assistant/list:
    post:
      tags:
        - 助理 API
      summary: 助手列表
      description: 查询指定的智能体助手列表信息，包括智能体助手的详细配置、工具和元数据。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/AssistantListRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                type: object
                properties:
                  success:
                    type: boolean
                    example: true
                  code:
                    type: integer
                    example: 200
                  msg:
                    type: string
                    example: 操作成功
                  data:
                    type: array
                    items:
                      $ref: '#/components/schemas/AssistantInfo'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
      deprecated: true
components:
  schemas:
    AssistantListRequest:
      type: object
      required:
        - assistant_id_list
      properties:
        assistant_id_list:
          type: array
          items:
            type: string
          description: 智能体`ID`列表，如果为空则查询所有可用智能体
          example:
            - 65940acff94777010aa6b796
            - assistant_2
          default: []
    AssistantInfo:
      type: object
      properties:
        assistant_id:
          type: string
          description: 智能体`ID`
          example: 65940acff94777010aa6b796
        name:
          type: string
          description: 智能体名称
          example: 通用智能助手
        avatar:
          type: string
          description: 智能体头像URL
          example: https://example.com/avatar.png
        description:
          type: string
          description: 智能体描述
          example: 一个通用的`AI`助手，可以回答各种问题
        tools:
          type: array
          items:
            type: string
          description: 智能体支持的工具列表
          example:
            - web_search
            - code_interpreter
        tags:
          type: array
          items:
            type: object
            properties:
              key:
                type: string
                description: 标签键
                example: category
              label:
                type: string
                description: 标签值
                example: 通用助手
          description: 智能体标签列表
        status:
          type: string
          description: 智能体状态
          example: active
        starter_prompts:
          type: array
          items:
            type: object
          description: 智能体的起始提示语
        created_at:
          type: string
          description: 创建时间
          example: '2024-01-01T00:00:00Z'
        updated_at:
          type: string
          description: 更新时间
          example: '2024-01-01T00:00:00Z'
      required:
        - assistant_id
        - name
    Error:
      type: object
      properties:
        error:
          required:
            - code
            - message
          type: object
          properties:
            code:
              type: string
            message:
              type: string
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````
---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 助手对话

> 与`AI`助手进行对话，支持流式和同步模式。



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/assistant
openapi: 3.0.1
info:
  title: ZHIPU AI API
  description: ZHIPU AI 接口提供强大的 AI 能力，包括聊天对话、工具调用和视频生成。
  license:
    name: ZHIPU AI 开发者协议和政策
    url: https://chat.z.ai/legal-agreement/terms-of-service
  version: 1.0.0
  contact:
    name: Z.AI 开发者
    url: https://chat.z.ai/legal-agreement/privacy-policy
    email: user_feedback@z.ai
servers:
  - url: https://open.bigmodel.cn/api/
    description: 开放平台服务
security:
  - bearerAuth: []
tags:
  - name: 模型 API
    description: Chat API
  - name: 工具 API
    description: Web Search API
  - name: Agent API
    description: Agent API
  - name: 文件 API
    description: File API
  - name: 知识库 API
    description: Knowledge API
  - name: 实时 API
    description: Realtime API
  - name: 批处理 API
    description: Batch API
  - name: 助理 API
    description: Assistant API
  - name: 智能体 API（旧）
    description: QingLiu Agent API
paths:
  /paas/v4/assistant:
    post:
      tags:
        - 助理 API
      summary: 助手对话
      description: 与`AI`助手进行对话，支持流式和同步模式。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/AssistantRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AssistantResponse'
            text/event-stream:
              schema:
                type: string
                description: 流式响应数据
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
      deprecated: true
components:
  schemas:
    AssistantRequest:
      type: object
      properties:
        assistant_id:
          type: string
          title: 智能体选择
          description: |-
            智能体ID，必需参数。请从下拉框选择智能体：

            • ChatGLM（官方）- 65940acff94777010aa6b796：嗨~ 我是清言，超开心遇见你！😺
            • 数据分析（官方）- 65a265419d72d299a9230616：分析数据并提供图表化
            • 复杂流程图（官方）- 664dd7bd5bb3a13ba0f81668：五秒钟做一张流程图
            • 思维导图 MindMap（官方）- 664e0cade018d633146de0d2：任何复杂概念秒变脑图
            • 提示词工程师（官方）- 6654898292788e88ce9e7f4c：超强结构化提示词专家
            • AI画图（官方）- 66437ef3d920bdc5c60f338e：专属绘画伙伴
            • AI搜索（官方）- 659e54b1b8006379b4b2abd6：连接全网内容，精准搜索
            • PPT助手（官方）- 65d2f07bb2c10188f885bd89：超实用的PPT生成器
            • arXiv论文速读（官方）- 663058948bb259b7e8a22730：深度解析论文
            • 程序员助手Sam（官方）- 65a393b3619c6f13586246cd：编程开发知识搜索引擎
            • 网文写手（官方）- 65b356af6924a59d52832e54：大神写作秘诀
            • 英语语法助手（官方）- 668fdd45405f2e3c9f71f832：检查语法和语法解释
          enum:
            - 65940acff94777010aa6b796
            - 65a265419d72d299a9230616
            - 664dd7bd5bb3a13ba0f81668
            - 664e0cade018d633146de0d2
            - 6654898292788e88ce9e7f4c
            - 66437ef3d920bdc5c60f338e
            - 659e54b1b8006379b4b2abd6
            - 65d2f07bb2c10188f885bd89
            - 663058948bb259b7e8a22730
            - 65a393b3619c6f13586246cd
            - 65b356af6924a59d52832e54
            - 668fdd45405f2e3c9f71f832
          x-enum-varnames:
            - ChatGLM（官方）
            - 数据分析（官方）
            - 复杂流程图（官方）
            - 思维导图 MindMap（官方）
            - 提示词工程师（官方）
            - AI画图（官方）
            - AI搜索（官方）
            - PPT助手（官方）
            - arXiv论文速读（官方）
            - 程序员助手Sam（官方）
            - 网文写手（官方）
            - 英语语法助手（官方）
          x-enumNames:
            - ChatGLM（官方）
            - 数据分析（官方）
            - 复杂流程图（官方）
            - 思维导图 MindMap（官方）
            - 提示词工程师（官方）
            - AI画图（官方）
            - AI搜索（官方）
            - PPT助手（官方）
            - arXiv论文速读（官方）
            - 程序员助手Sam（官方）
            - 网文写手（官方）
            - 英语语法助手（官方）
          x-enum-descriptions:
            - ChatGLM（官方） - 嗨~ 我是清言，超开心遇见你！😺 你最近有什么好玩的事情想和我分享吗？
            - 数据分析（官方） - 通过分析用户上传文件或数据说明，帮助用户分析数据并提供图表化。也可通过简单的编码完成文件处理的工作。
            - 复杂流程图（官方） - 人人都能掌握的流程图工具，用五秒钟做一张流程图卷到同事。
            - 思维导图 MindMap（官方） - 告别整理烦恼，任何复杂概念秒变脑图。
            - 提示词工程师（官方） - 人人都是提示词工程师，超强结构化提示词专家，一键改写提示词。
            - AI画图（官方） - 让想象力自由飞翔，你的专属绘画伙伴，画到停不下来。
            - AI搜索（官方） - 连接全网内容，精准搜索，快速分析并总结的智能助手。
            - PPT助手（官方） - 超实用的PPT生成器，支持手动编辑大纲、自动填充章节内容。
            - arXiv论文速读（官方） - 深度解析arXiv论文，让你快速掌握研究动态，节省宝贵时间。
            - 程序员助手Sam（官方） - "编程开发知识搜索引擎"，能帮助程序员解决日常问题。
            - 网文写手（官方） - 大神写作秘诀：一套模板不断重复。
            - 英语语法助手（官方） - 输入单词查询；输入句子检查语法，和语法解释。
          default: 65940acff94777010aa6b796
        conversation_id:
          type: string
          description: 会话`ID`，可选参数，用于继续之前的对话
        model:
          type: string
          enum:
            - glm-4-assistant
            - glm-4-alltools
          description: 使用的模型名称
          default: glm-4-assistant
        messages:
          type: array
          description: 对话消息列表
          items:
            type: object
            required:
              - role
              - content
            properties:
              role:
                type: string
                description: 消息作者的角色
                enum:
                  - user
                example: user
              content:
                oneOf:
                  - type: string
                    description: 消息文本内容
                    example: 你好
                  - type: array
                    description: 多模态消息内容，支持文本、图片等
                    items:
                      type: object
                      properties:
                        type:
                          type: string
                          enum:
                            - text
                            - image_url
                          description: 内容类型
                        text:
                          type: string
                          description: 文本内容
                        image_url:
                          type: object
                          properties:
                            url:
                              type: string
                              description: 图片`URL`
          minItems: 1
        stream:
          type: boolean
          description: 是否启用流式响应
          default: true
        request_id:
          type: string
          description: 请求任务`ID`，长度必须位于`6`到`64`位之间
          minLength: 6
          maxLength: 64
        user_id:
          type: string
          description: 终端用户的唯一`ID`，长度必须位于`6`到`128`位之间
          minLength: 6
          maxLength: 128
        do_sample:
          type: boolean
          description: 是否稳定输出
        attachments:
          type: array
          items:
            type: object
          description: 附件列表
        metadata:
          type: object
          description: 元数据信息
          additionalProperties: true
        extra_parameters:
          type: object
          description: 额外参数，用于特定智能体的扩展配置
          properties:
            translate:
              type: object
              description: 翻译参数
              properties:
                from:
                  type: string
                  description: 源语言
                to:
                  type: string
                  description: 目标语言
      required:
        - assistant_id
        - model
        - messages
    AssistantResponse:
      type: object
      properties:
        id:
          type: string
          description: 响应`ID`
        request_id:
          type: string
          description: 请求`ID`
        created:
          type: integer
          description: 创建时间戳
        model:
          type: string
          description: 使用的模型
        choices:
          type: array
          items:
            type: object
            properties:
              index:
                type: integer
              message:
                $ref: '#/components/schemas/ChatCompletionResponseMessage'
              finish_reason:
                type: string
        usage:
          type: object
          properties:
            prompt_tokens:
              type: integer
            completion_tokens:
              type: integer
            total_tokens:
              type: integer
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    ChatCompletionResponseMessage:
      type: object
      properties:
        role:
          type: string
          description: 当前对话角色，默认为 `assistant`
          example: assistant
        content:
          oneOf:
            - type: string
              description: >-
                当前对话文本内容。如果调用函数则为 `null`，否则返回推理结果。

                对于`GLM-4.5V`系列模型，返回内容可能包含思考过程标签 `<think> </think>`，文本边界标签
                `<|begin_of_box|> <|end_of_box|>`。
            - type: array
              description: 多模态回复内容，适用于`GLM-4V`系列模型
              items:
                type: object
                properties:
                  type:
                    type: string
                    enum:
                      - text
                    description: 回复内容类型，目前为文本
                  text:
                    type: string
                    description: 文本内容
            - type: string
              nullable: true
              description: 当使用`tool_calls`时，`content`可能为`null`
        reasoning_content:
          type: string
          description: 思维链内容，仅在使用 `glm-4.5` 系列, `glm-4.1v-thinking` 系列模型时返回。
        audio:
          type: object
          description: 当使用 `glm-4-voice` 模型时返回的音频内容
          properties:
            id:
              type: string
              description: 当前对话的音频内容`id`，可用于多轮对话输入
            data:
              type: string
              description: 当前对话的音频内容`base64`编码
            expires_at:
              type: string
              description: 当前对话的音频内容过期时间
        tool_calls:
          type: array
          description: 生成的应该被调用的函数名称和参数。
          items:
            $ref: '#/components/schemas/ChatCompletionResponseMessageToolCall'
    ChatCompletionResponseMessageToolCall:
      type: object
      properties:
        function:
          type: object
          description: 包含生成的函数名称和 `JSON` 格式参数。
          properties:
            name:
              type: string
              description: 生成的函数名称。
            arguments:
              type: string
              description: 生成的函数调用参数的 `JSON` 格式字符串。调用函数前请验证参数。
          required:
            - name
            - arguments
        mcp:
          type: object
          description: '`MCP` 工具调用参数'
          properties:
            id:
              description: '`mcp` 工具调用唯一标识'
              type: string
            type:
              description: 工具调用类型, 例如 `mcp_list_tools, mcp_call`
              type: string
              enum:
                - mcp_list_tools
                - mcp_call
            server_label:
              description: '`MCP`服务器标签'
              type: string
            error:
              description: 错误信息
              type: string
            tools:
              description: '`type = mcp_list_tools` 时的工具列表'
              type: array
              items:
                type: object
                properties:
                  name:
                    description: 工具名称
                    type: string
                  description:
                    description: 工具描述
                    type: string
                  annotations:
                    description: 工具注解
                    type: object
                  input_schema:
                    description: 工具输入参数规范
                    type: object
                    properties:
                      type:
                        description: 固定值 'object'
                        type: string
                        default: object
                        enum:
                          - object
                      properties:
                        description: 参数属性定义
                        type: object
                      required:
                        description: 必填属性列表
                        type: array
                        items:
                          type: string
                      additionalProperties:
                        description: 是否允许额外参数
                        type: boolean
            arguments:
              description: 工具调用参数，参数为 `json` 字符串
              type: string
            name:
              description: 工具名称
              type: string
            output:
              description: 工具返回的结果输出
              type: object
        id:
          type: string
          description: 命中函数的唯一标识符。
        type:
          type: string
          description: 调用的工具类型，目前仅支持 'function', 'mcp'。
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````
