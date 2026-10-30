# GLM 智能体 API（旧）

- Category: 智能体-api（旧）
- Source root: https://docs.bigmodel.cn/api-reference/智能体-api（旧）
- Pages: 7
- Fetched on: 2026-04-23
- Fetch failures: 0

## Sources

- https://docs.bigmodel.cn/api-reference/智能体-api（旧）/创建新会话
- https://docs.bigmodel.cn/api-reference/智能体-api（旧）/推理接口
- https://docs.bigmodel.cn/api-reference/智能体-api（旧）/推荐问题接口
- https://docs.bigmodel.cn/api-reference/智能体-api（旧）/文件上传
- https://docs.bigmodel.cn/api-reference/智能体-api（旧）/知识库切片引用位置信息
- https://docs.bigmodel.cn/api-reference/智能体-api（旧）/获取文件解析状态
- https://docs.bigmodel.cn/api-reference/智能体-api（旧）/获取智能体输入参数

---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 创建新会话

> 为指定智能体（应用）创建新会话，返回会话`ID`。



## OpenAPI

````yaml /openapi/openapi.json post /llm-application/open/v2/application/{app_id}/conversation
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
  /llm-application/open/v2/application/{app_id}/conversation:
    post:
      tags:
        - 智能体 API（旧）
      summary: 创建新会话
      description: 为指定智能体（应用）创建新会话，返回会话`ID`。
      parameters:
        - name: app_id
          in: path
          required: true
          description: 智能体（应用）id
          schema:
            type: string
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApplicationConversationResponse'
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
      deprecated: true
      security:
        - bearerAuth: []
components:
  schemas:
    ApplicationConversationResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            conversation_id:
              type: string
              description: 会话id
          required:
            - conversation_id
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
      required:
        - data
        - code
        - message
        - timestamp
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
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

# 推理接口

> 对话型或文本型应用推理接口，支持同步和流式`SSE`调用。



## OpenAPI

````yaml /openapi/openapi.json post /llm-application/open/v3/application/invoke
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
  /llm-application/open/v3/application/invoke:
    post:
      tags:
        - 智能体 API（旧）
      summary: 推理接口
      description: 对话型或文本型应用推理接口，支持同步和流式`SSE`调用。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ApplicationInvokeRequest'
        required: true
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApplicationInvokeResponse'
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
      deprecated: true
      security:
        - bearerAuth: []
components:
  schemas:
    ApplicationInvokeRequest:
      type: object
      properties:
        app_id:
          type: string
          description: 应用id
        conversation_id:
          type: string
          description: 会话id, 未传则默认创建新会话
        third_request_id:
          type: string
          description: 三方请求id(调用插件时传入, 用于链路排查问题)
        stream:
          type: boolean
          description: 默认true，false时为同步调用
        messages:
          type: array
          items:
            $ref: '#/components/schemas/ApplicationInvokeMessage'
          description: 用户输入列表
        role:
          type: string
          description: 对话类应用请求必传：user（用户输入）， assistant（模型返回）
        send_log_event:
          type: boolean
          description: 是否实时推送过程日志，默认false
      required:
        - app_id
        - messages
    ApplicationInvokeResponse:
      type: object
      properties:
        request_id:
          type: string
          description: 请求id
        conversation_id:
          type: string
          description: 会话id
        app_id:
          type: string
          description: 应用id
        choices:
          type: array
          items:
            $ref: '#/components/schemas/ApplicationInvokeChoice'
          description: 增量返回的信息
        usage:
          type: array
          items:
            $ref: '#/components/schemas/ApplicationInvokeUsage'
          description: 本次调用的 tokens 数量统计
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    ApplicationInvokeMessage:
      type: object
      properties:
        role:
          type: string
          description: 角色 user/assistant
        content:
          type: array
          items:
            $ref: '#/components/schemas/ApplicationInvokeContent'
          description: 具体内容
      required:
        - content
    ApplicationInvokeChoice:
      type: object
      properties:
        index:
          type: integer
          description: 结果下标
        finish_reason:
          type: string
          description: 'stop: 正常结束 error: 执行失败'
        error_msg:
          type: object
          description: 异常信息
          properties:
            code:
              type: integer
            msg:
              type: string
        delta:
          $ref: '#/components/schemas/ApplicationInvokeDelta'
        messages:
          $ref: '#/components/schemas/ApplicationInvokeMessages'
    ApplicationInvokeUsage:
      type: object
      properties:
        model:
          type: string
          description: 推理model
        nodeName:
          type: string
          description: 节点名称
        inputTokenCount:
          type: integer
          description: 输入的 tokens 数量
        outputTokenCount:
          type: integer
          description: 输出的 tokens 数量
        totalTokenCount:
          type: integer
          description: 总 tokens 数量
    ApplicationInvokeContent:
      type: object
      properties:
        type:
          type: string
          description: 内容类型 input/upload_file/upload_image/upload_video/selection_list
        value:
          type: string
          description: 用户输入/下拉选项/文件ID/图片视频url
        key:
          type: string
          description: 字段名称(文本类应用请求时必须传该字段)
      required:
        - type
        - value
    ApplicationInvokeDelta:
      type: object
      properties:
        content:
          $ref: '#/components/schemas/MessageData'
        event:
          $ref: '#/components/schemas/ApplicationInvokeEvent'
        tool_calls:
          $ref: '#/components/schemas/ToolCallsData'
    ApplicationInvokeMessages:
      type: object
      properties:
        content:
          $ref: '#/components/schemas/MessageData'
        event:
          type: array
          items:
            $ref: '#/components/schemas/ApplicationInvokeEvent'
    MessageData:
      type: object
      properties:
        msg:
          type: string
          description: 推理内容/文本/图片/视频/工具等
        type:
          type: string
          description: text/image/video/all_tools
        code:
          type: string
          description: 代码（all_tools时）
        file:
          type: string
          description: 文件URL（all_tools时）
        url:
          type: string
          description: 图片/视频url
        coverUrl:
          type: string
          description: 视频封面url
    ApplicationInvokeEvent:
      type: object
      properties:
        node_id:
          type: string
          description: 节点id
        node_name:
          type: string
          description: 节点名称
        type:
          type: string
          description: 事件类型 node_processing/tool_processing/tool_finish/node_finish
        content:
          type: string
          description: 输入输出内容
        time:
          type: integer
          description: 毫秒
        tool_calls:
          $ref: '#/components/schemas/ToolCallsData'
    ToolCallsData:
      type: object
      properties:
        type:
          type: string
          description: function/retrieval/web_search
        tool_calls_data:
          type: object
          description: 工具消息体/日志体
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

# 推荐问题接口

> 获取推荐问题列表。



## OpenAPI

````yaml /openapi/openapi.json get /llm-application/open/history_session_record/{app_id}/{conversation_id}
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
  /llm-application/open/history_session_record/{app_id}/{conversation_id}:
    get:
      tags:
        - 智能体 API（旧）
      summary: 推荐问题接口
      description: 获取推荐问题列表。
      parameters:
        - name: app_id
          in: path
          required: true
          description: 应用id
          schema:
            type: string
        - name: conversation_id
          in: path
          required: true
          description: 会话id
          schema:
            type: string
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/HistorySessionRecordResponse'
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
      deprecated: true
      security:
        - bearerAuth: []
components:
  schemas:
    HistorySessionRecordResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            problems:
              type: array
              items:
                type: string
              description: 推荐问题列表
          required:
            - problems
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
      required:
        - data
        - code
        - message
        - timestamp
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
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

# 文件上传

> 上传文件到智能体（应用），同步返回上传结果。需通过文件解析状态接口获取解析结果。



## OpenAPI

````yaml /openapi/openapi.json post /llm-application/open/v2/application/file_upload
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
  /llm-application/open/v2/application/file_upload:
    post:
      tags:
        - 智能体 API（旧）
      summary: 文件上传
      description: 上传文件到智能体（应用），同步返回上传结果。需通过文件解析状态接口获取解析结果。
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                app_id:
                  type: string
                  description: 智能体（应用）id
                upload_unit_id:
                  type: string
                  description: 上传文件组件id，文本类必须传，对话类临时文件不传
                files:
                  type: string
                  format: binary
                  description: >-
                    支持多文件上传，files 字段可重复出现，每个文件单独作为一个 form-data 字段上传。示例：--form
                    'files=@"/path/to/file1"' --form 'files=@"/path/to/file2"'
                conversation_id:
                  type: string
                  description: 对话类型应用上传临时文件时需先通过创建新会话获得会话ID（对话类型上传临时文件时必传，文本类不传）
                file_type:
                  type: integer
                  description: '文件类型 1: excel 2: 文档 3: 音频 4: 图片 5: 视频'
              required:
                - app_id
                - files
        required: true
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApplicationFileUploadResponse'
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
      deprecated: true
      security:
        - bearerAuth: []
components:
  schemas:
    ApplicationFileUploadResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            success_info:
              type: array
              items:
                $ref: '#/components/schemas/ApplicationFileUploadSuccessInfo'
              description: 上传成功的文件
            fail_info:
              type: array
              items:
                $ref: '#/components/schemas/ApplicationFileUploadFailInfo'
              description: 上传失败的文件
          required:
            - success_info
            - fail_info
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
      required:
        - data
        - code
        - message
        - timestamp
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    ApplicationFileUploadSuccessInfo:
      type: object
      properties:
        file_id:
          type: string
          description: 文件id
        file_name:
          type: string
          description: 文件名
      required:
        - file_id
        - file_name
    ApplicationFileUploadFailInfo:
      type: object
      properties:
        file_name:
          type: string
          description: 文件名
        fail_reason:
          type: string
          description: 失败原因
      required:
        - file_name
        - fail_reason
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

# 知识库切片引用位置信息

> 获取知识库切片引用的位置信息。



## OpenAPI

````yaml /openapi/openapi.json post /llm-application/open/v2/application/slice_info
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
  /llm-application/open/v2/application/slice_info:
    post:
      tags:
        - 智能体 API（旧）
      summary: 知识库切片引用位置信息
      description: 获取知识库切片引用的位置信息。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ApplicationSliceInfoRequest'
        required: true
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApplicationSliceInfoResponse'
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
      deprecated: true
      security:
        - bearerAuth: []
components:
  schemas:
    ApplicationSliceInfoRequest:
      type: object
      properties:
        request_id:
          type: string
          description: 创建对话或文本请求接口返回的id
        node_id:
          type: string
          description: 节点id
      required:
        - request_id
        - node_id
    ApplicationSliceInfoResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            document_slices:
              type: array
              items:
                $ref: '#/components/schemas/DocumentSlices'
              description: 文档切片信息
            has_old_document:
              type: boolean
              description: 是否存在没有切片位置的历史文档
          required:
            - document_slices
            - has_old_document
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
      required:
        - data
        - code
        - message
        - timestamp
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    DocumentSlices:
      type: object
      properties:
        document:
          $ref: '#/components/schemas/DocumentInfo'
        slice_info:
          type: array
          items:
            $ref: '#/components/schemas/SliceInfo'
          description: 切片信息
        hide_positions:
          type: boolean
          description: 是否存在历史文档切片没有位置信息
        images:
          type: array
          items:
            $ref: '#/components/schemas/SliceImage'
          description: 图片列表
      required:
        - document
        - slice_info
        - hide_positions
    DocumentInfo:
      type: object
      properties:
        id:
          type: string
          description: 文档ID
        name:
          type: string
          description: 文档名称
        url:
          type: string
          description: 文档URL
        dtype:
          type: integer
          description: 文档类型
      required:
        - id
        - name
        - url
        - dtype
    SliceInfo:
      type: object
      properties:
        document_id:
          type: string
          description: 文档ID
        position:
          $ref: '#/components/schemas/SlicePosition'
        line:
          type: integer
          description: sheet行号
        sheet_name:
          type: string
          description: sheet名称
        text:
          type: string
          description: 切片内容
      required:
        - document_id
        - text
    SliceImage:
      type: object
      properties:
        text:
          type: string
          description: 图片名称
        cos_url:
          type: string
          description: 图片地址
      required:
        - text
        - cos_url
    SlicePosition:
      type: object
      properties:
        x0:
          type: number
          format: float
          description: 左侧到行左侧的距离
        x1:
          type: number
          format: float
          description: 字符顶部到顶部距离
        top:
          type: number
          format: float
          description: 字符顶部到顶部距离
        bottom:
          type: number
          format: float
          description: 字符底部到顶部距离
        page:
          type: integer
          description: 所在页
        height:
          type: number
          format: float
          description: 所在页高
        width:
          type: number
          format: float
          description: 所在页宽度
      required:
        - x0
        - x1
        - top
        - bottom
        - page
        - height
        - width
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

# 获取文件解析状态

> 获取指定文件的解析状态。



## OpenAPI

````yaml /openapi/openapi.json post /llm-application/open/v2/application/file_stat
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
  /llm-application/open/v2/application/file_stat:
    post:
      tags:
        - 智能体 API（旧）
      summary: 获取文件解析状态
      description: 获取指定文件的解析状态。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ApplicationFileStatRequest'
        required: true
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApplicationFileStatResponse'
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
      deprecated: true
      security:
        - bearerAuth: []
components:
  schemas:
    ApplicationFileStatRequest:
      type: object
      properties:
        app_id:
          type: string
          description: 智能体（应用）id
        file_ids:
          type: array
          items:
            type: string
          description: 文件id列表
      required:
        - app_id
        - file_ids
    ApplicationFileStatResponse:
      type: object
      properties:
        data:
          type: array
          items:
            $ref: '#/components/schemas/ApplicationFileStatItem'
          description: 文件解析状态列表
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
      required:
        - data
        - code
        - message
        - timestamp
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    ApplicationFileStatItem:
      type: object
      properties:
        file_id:
          type: string
          description: 文件id
        code:
          type: integer
          description: 文档解析状态码
        msg:
          type: string
          description: 描述
      required:
        - file_id
        - code
        - msg
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

# 获取智能体输入参数

> 获取指定智能体应用的输入参数列表。



## OpenAPI

````yaml /openapi/openapi.json get /llm-application/open/v2/application/{app_id}/variables
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
  /llm-application/open/v2/application/{app_id}/variables:
    get:
      tags:
        - 智能体 API（旧）
      summary: 获取智能体输入参数
      description: 获取指定智能体应用的输入参数列表。
      parameters:
        - name: app_id
          in: path
          required: true
          description: 智能体（应用）id，获取位置：我的智能体列表页面
          schema:
            type: string
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApplicationVariablesResponse'
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
      deprecated: true
      security:
        - bearerAuth: []
components:
  schemas:
    ApplicationVariablesResponse:
      type: object
      properties:
        data:
          type: array
          items:
            $ref: '#/components/schemas/KeyValuePair'
          description: 变量列表
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
      required:
        - data
        - code
        - message
        - timestamp
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    KeyValuePair:
      type: object
      properties:
        id:
          type: string
          description: 变量id
        name:
          type: string
          description: 变量名称
        type:
          type: string
          description: >-
            变量类型 Input: 文本输入 selection_list: 下拉框 upload_file: 文件上传 upload_image:
            图片上传 upload_video: 视频上传 upload_audio: 音频上传
        tips:
          type: string
          description: 提示词
        allowed_values:
          type: array
          items:
            type: string
          description: 下拉框选项, 当type = selection_list, 会有此值
        input_template:
          $ref: '#/components/schemas/InputTemplate'
      required:
        - id
        - name
        - type
        - tips
        - allowed_values
        - input_template
    InputTemplate:
      type: object
      properties:
        options:
          type: array
          items: {}
          description: 可选项
      required:
        - options
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````
