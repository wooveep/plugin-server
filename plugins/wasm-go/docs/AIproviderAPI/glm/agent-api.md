# GLM Agent API

- Category: agent-api
- Source root: https://docs.bigmodel.cn/api-reference/agent-api
- Pages: 3
- Fetched on: 2026-04-23
- Fetch failures: 0

## Sources

- https://docs.bigmodel.cn/api-reference/agent-api/对话历史
- https://docs.bigmodel.cn/api-reference/agent-api/异步结果
- https://docs.bigmodel.cn/api-reference/agent-api/智能体对话

---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 对话历史

> 查询智能体对话历史，现仅支持 `slides_glm_agent` 智能体



## OpenAPI

````yaml /openapi/openapi.json post /v1/agents/conversation
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
  /v1/agents/conversation:
    post:
      tags:
        - Agent API
      summary: 对话历史
      description: 查询智能体对话历史，现仅支持 `slides_glm_agent` 智能体
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/GlmSlideAgentConversationRequest'
        required: true
      responses:
        '200':
          description: 处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/GlmSlideAgentConversationResponse'
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    GlmSlideAgentConversationRequest:
      type: object
      properties:
        agent_id:
          type: string
          description: 智能体 `ID`
        conversation_id:
          type: string
          description: 对话 `ID`
        custom_variables:
          type: object
          description: 自定义参数
          properties:
            include_pdf:
              type: boolean
              description: 是否导出 `pdf` 文件
            pages:
              type: array
              description: '`Slide` 页信息'
              items:
                type: object
                properties:
                  position:
                    type: number
                    description: '`Slide` 页码'
                  width:
                    type: number
                    description: '`Slide` 宽度, 单位: `cm`'
                  height:
                    type: number
                    description: '`Slide` 高度, 单位: `cm`'
    GlmSlideAgentConversationResponse:
      type: object
      properties:
        conversation_id:
          type: string
          description: 对话 `ID`
        agent_id:
          type: string
          description: 智能体 `ID`
        choices:
          type: array
          description: Agent output.
          items:
            type: object
            properties:
              message:
                type: array
                items:
                  type: object
                  properties:
                    role:
                      type: string
                      description: 智能体的角色 `role = assistant`
                    content:
                      type: array
                      description: 智能体响应内容
                      items:
                        type: object
                        properties:
                          type:
                            type: string
                            description: 响应内容类型：文件下载链接-`file_url`、图片下载链接-`image_url`
                          tag_cn:
                            type: string
                            description: CN Tag.
                          tag_en:
                            type: string
                            description: EN Tag.
                          file_url:
                            type: string
                            description: 如果 `type = file_url`，则这个字段给出文件的具体下载链接
                          image_url:
                            type: string
                            description: 如果 `type = image_url`，则这个字段给出图片的具体下载链接
        error:
          type: object
          properties:
            code:
              type: string
              description: Error code.
            message:
              type: string
              description: Error message.
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

# 异步结果

> 查询智能体异步任务的处理结果和状态。



## OpenAPI

````yaml /openapi/openapi.json post /v1/agents/async-result
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
  /v1/agents/async-result:
    post:
      tags:
        - Agent API
      summary: 异步结果
      description: 查询智能体异步任务的处理结果和状态。
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required:
                - async_id
                - agent_id
              properties:
                async_id:
                  type: string
                  description: 任务ID
                  example: VHJhbnNsYXRvckVudGl0eVRhc2s6OTI5OTY5
                agent_id:
                  type: string
                  description: 智能体ID
                  example: intelligent_education_correction_polling
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AgentAsyncResultResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    AgentAsyncResultResponse:
      type: object
      properties:
        agent_id:
          type: string
          description: 智能体`ID`
        async_id:
          type: string
          description: 异步任务的`ID`
        status:
          type: string
          description: 任务状态，`success/failed/pending`
          enum:
            - success
            - failed
            - pending
        choices:
          type: array
          description: '`agent`输出'
          items:
            type: object
            properties:
              messages:
                type: array
                items:
                  type: object
                  properties:
                    role:
                      type: string
                      description: 用户的输入 `role = assistant`
                    content:
                      type: array
                      items:
                        type: object
                        properties:
                          type:
                            type: string
                            description: 目前支持 `type=file_url`
                          file_url:
                            type: string
                            description: url链接
                          tag_cn:
                            type: string
                            description: 中文描述
                          tag_en:
                            type: string
                            description: 英文描述
        usage:
          type: object
          properties:
            total_tokens:
              type: integer
              description: 消耗总`tokens`数
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

# 智能体对话

> 与智能体进行对话交互。支持同步和流式调用，提供智能体的专业能力。见 [智能体文档](https://docs.bigmodel.cn/cn/guide/agents/translation)



## OpenAPI

````yaml /openapi/openapi.json post /v1/agents
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
  /v1/agents:
    post:
      tags:
        - Agent API
      summary: 智能体对话
      description: 与智能体进行对话交互。支持同步和流式调用，提供智能体的专业能力。见 [智能体文档](https://docs.bigmodel.cn/cn/guide/agents/translation)
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/AgentRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AgentResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    AgentRequest:
      type: object
      required:
        - agent_id
        - messages
      properties:
        agent_id:
          oneOf:
            - type: string
              enum:
                - general_translation
                - slides_glm_agent
                - cartoon_generator_agent
                - ai_drawing_agent
                - receipt_recognition_agent
                - clothes_recognition_agent
                - contract_parser_agent
                - service_check_agent
                - subtitle_translation_agent
                - intelligent_education_correction_agent
                - job_matching_agent
                - social_translation_agent
                - social_literature_translation_agent
                - intelligent_education_solve_agent
                - vidu_template_agent
                - bidwin_parser_agent
              description: >-
                此处提供 通用翻译智能体 `general_translation`
                的拓展参数模板作为示例，其他内嵌智能体需要参考文档在下方手动设置拓展参数。
          description: 智能体 ID
        stream:
          type: boolean
          description: 是否选择流式输出。当一个 `agent` 提供流式与非流式两种输出方式时，可根据需求选择一种。
          default: false
        messages:
          type: array
          description: 会话消息体列表。
          items:
            type: object
            required:
              - role
              - content
            properties:
              role:
                type: string
                description: 消息作者的角色。用户输入时 `role = user`。
                enum:
                  - system
                  - user
                  - assistant
                example: user
              content:
                oneOf:
                  - type: string
                    description: 用户输入的文本内容。
                    example: 请帮我翻译这段文字：Hello World
                  - type: object
                    description: 单个多模态内容，用于只有一个元素的情况
                    properties:
                      type:
                        type: string
                        description: 内容类型，可以是文本、文件`ID`、文件链接或图片链接
                        enum:
                          - text
                          - file_id
                          - file_url
                          - image_url
                      text:
                        type: string
                        description: 当 `type` 为 `text` 时的文本内容
                        example: 请帮我翻译这段文字：Hello World
                      file_id:
                        type: string
                        description: 当 `type` 为 `file_id` 时的文件唯一标识
                        example: agent-1750681215-9b92722d788f4b32bab28cc333293584
                      file_url:
                        type: string
                        description: 当 `type` 为 `file_url` 时的文件链接
                        example: https://example.com/my_oss/document.pdf
                      image_url:
                        type: string
                        description: 当 `type` 为 `image_url` 时的图片链接
                        example: https://example.com/my_oss/image.png
                    required:
                      - type
                  - type: array
                    description: 多模态内容数组，支持文本、文件和图片
                    items:
                      type: object
                      properties:
                        type:
                          type: string
                          description: 内容类型，可以是文本、文件`ID`、文件链接或图片链接
                          enum:
                            - text
                            - file_id
                            - file_url
                            - image_url
                        text:
                          type: string
                          description: 当 `type` 为 `text` 时的文本内容
                          example: 请帮我翻译这段文字：Hello World
                        file_id:
                          type: string
                          description: 当 `type` 为 `file_id` 时的文件唯一标识
                          example: agent-1750681215-9b92722d788f4b32bab28cc333293584
                        file_url:
                          type: string
                          description: 当 `type` 为 `file_url` 时的文件链接
                          example: https://example.com/my_oss/document.pdf
                        image_url:
                          type: string
                          description: 当 `type` 为 `image_url` 时的图片链接
                          example: https://example.com/my_oss/image.png
                      required:
                        - type
          minItems: 1
        custom_variables:
          description: 智能体扩展参数。根据不同智能体的需求提供相应的参数配置。
          oneOf:
            - $ref: '#/components/schemas/TranslationAgentCustomVariables'
              title: 通用翻译智能体
            - $ref: '#/components/schemas/CustomAgentCustomVariables'
              title: 自定义智能体扩展
    AgentResponse:
      type: object
      properties:
        id:
          type: string
          description: 请求唯一标识
        agent_id:
          type: string
          description: 智能体唯一标识
        conversation_id:
          type: string
          description: 对话唯一标识
        async_id:
          type: string
          description: 异步任务唯一标识（异步调用时出现）
        choices:
          type: array
          description: 智能体响应列表。
          items:
            type: object
            properties:
              index:
                type: integer
                description: 结果索引。
              messages:
                type: array
                description: 智能体生成的响应消息列表。
                items:
                  type: object
                  properties:
                    role:
                      type: string
                      description: 响应角色，通常为 `assistant`。
                      example: assistant
                    content:
                      oneOf:
                        - type: string
                          description: 智能体生成的响应内容（纯文本格式）。
                        - type: object
                          description: 单个多模态响应内容，用于响应只有一个元素的情况
                          properties:
                            type:
                              type: string
                              description: 内容类型，可以是文本、文件、图片、音频或视频
                              enum:
                                - text
                                - file_url
                                - image_url
                                - audio_url
                                - video_url
                            text:
                              type: string
                              description: 当 `type` 为 `text` 时的文本内容
                            file_url:
                              type: string
                              description: 当 `type` 为 `file_url` 时的文件链接
                            image_url:
                              type: string
                              description: 当 `type` 为 `image_url` 时的图片链接
                            audio_url:
                              type: string
                              description: 当 `type` 为 `audio_url` 时的音频链接
                            video_url:
                              type: string
                              description: 当 `type` 为 `video_url` 时的视频链接
                          required:
                            - type
                        - type: array
                          description: 多模态响应内容数组，支持文本、文件、图片、音频和视频
                          items:
                            type: object
                            properties:
                              type:
                                type: string
                                description: 内容类型，可以是文本、文件、图片、音频或视频
                                enum:
                                  - text
                                  - file_url
                                  - image_url
                                  - audio_url
                                  - video_url
                              text:
                                type: string
                                description: 当 `type` 为 `text` 时的文本内容
                              file_url:
                                type: string
                                description: 当 `type` 为 `file_url` 时的文件链接
                              image_url:
                                type: string
                                description: 当 `type` 为 `image_url` 时的图片链接
                              audio_url:
                                type: string
                                description: 当 `type` 为 `audio_url` 时的音频链接
                              video_url:
                                type: string
                                description: 当 `type` 为 `video_url` 时的视频链接
                            required:
                              - type
              finish_reason:
                type: string
                description: >-
                  响应结束原因。可能的取值为 正常结束-`stop`、长度达到上限-`length`、内容敏感-`sensitive` 或
                  网络错误-`network_error`。
        usage:
          $ref: '#/components/schemas/UsageStatistics'
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
    TranslationAgentCustomVariables:
      type: object
      description: 当前为 [通用翻译智能体](https://docs.bigmodel.cn/cn/guide/agents/translation) `general_translation` 扩展参数预设
      properties:
        source_lang:
          type: string
          description: 待翻译文本的源语言代码，默认值为 `auto`
          default: auto
          enum:
            - auto
            - zh-CN
            - zh-TW
            - wyw
            - yue
            - en
            - ja
            - ko
            - fr
            - de
            - es
            - ru
            - pt
            - it
            - ar
            - hi
            - bg
            - cs
            - da
            - el
            - et
            - fi
            - hu
            - id
            - lt
            - lv
            - nl
            - 'no'
            - pl
            - ro
            - sk
            - sl
            - sv
            - th
            - tr
            - uk
            - vi
            - my
            - ms
            - Pinyin
            - IPA
        target_lang:
          type: string
          description: 待翻译文本的目标语言代码，默认为 `zh-CN`
          default: zh-CN
          enum:
            - zh-CN
            - zh-TW
            - wyw
            - yue
            - en
            - en-GB
            - en-US
            - ja
            - ko
            - fr
            - de
            - es
            - ru
            - pt
            - it
            - ar
            - hi
            - bg
            - cs
            - da
            - el
            - et
            - fi
            - hu
            - id
            - lt
            - lv
            - nl
            - 'no'
            - pl
            - ro
            - sk
            - sl
            - sv
            - th
            - tr
            - uk
            - vi
            - my
            - ms
            - Pinyin
            - IPA
        glossary:
          type: string
          description: 术语表`id`
        strategy:
          type: string
          description: 翻译策略
          default: general
          enum:
            - general
            - paraphrase
            - two_step
            - three_step
            - reflection
            - cot
        strategy_config:
          type: object
          description: 翻译策略对应的参数
          properties:
            general:
              type: object
              description: 当翻译策略指定为`general`时生效
              properties:
                suggestion:
                  type: string
                  description: 翻译建议或风格要求，如术语对照、文体规范等
            cot:
              type: object
              description: 当翻译策略指定为`cot`时生效
              properties:
                reason_lang:
                  type: string
                  description: 翻译理由的语言，取值 ["from"｜"to"]，默认 "to"
                  default: to
                  enum:
                    - from
                    - to
      example:
        source_lang: en
        target_lang: zh-CN
        strategy: general
        strategy_config:
          general:
            suggestion: 专业技术文档翻译风格
    CustomAgentCustomVariables:
      type: object
      description: 自定义其它智能体的扩展参数，具体参数字段可参考对应的[智能体文档](https://docs.bigmodel.cn/cn/guide/agents/translation)
      additionalProperties: true
      example:
        custom_param1: value1
        custom_param2: 42
        options:
          detailed: true
          format: json
    UsageStatistics:
      type: object
      description: 调用结束时返回的 `Token` 使用统计。
      properties:
        prompt_tokens:
          type: integer
          description: 用户输入的 `Token` 数量
        completion_tokens:
          type: integer
          description: 输出的 `Token` 数量
        total_tokens:
          type: integer
          description: '`Token` 总数'
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````
