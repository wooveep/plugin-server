# GLM 工具 API

- Category: 工具-api
- Source root: https://docs.bigmodel.cn/api-reference/工具-api
- Pages: 7
- Fetched on: 2026-04-23
- Fetch failures: 0

## Sources

- https://docs.bigmodel.cn/api-reference/工具-api/ocr-服务
- https://docs.bigmodel.cn/api-reference/工具-api/内容安全
- https://docs.bigmodel.cn/api-reference/工具-api/文件解析
- https://docs.bigmodel.cn/api-reference/工具-api/文件解析同步
- https://docs.bigmodel.cn/api-reference/工具-api/网络搜索
- https://docs.bigmodel.cn/api-reference/工具-api/网页阅读
- https://docs.bigmodel.cn/api-reference/工具-api/解析结果

---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# OCR 服务

> 上传图片文件，使用指定工具类型进行 OCR（光学字符识别），支持手写体、文字等识别模式，见 [OCR 服务](https://docs.bigmodel.cn/cn/guide/tools/zhipu-ocr)



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/files/ocr
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
  /paas/v4/files/ocr:
    post:
      tags:
        - 工具 API
      summary: OCR 服务
      description: >-
        上传图片文件，使用指定工具类型进行 OCR（光学字符识别），支持手写体、文字等识别模式，见 [OCR
        服务](https://docs.bigmodel.cn/cn/guide/tools/zhipu-ocr)
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file:
                  type: string
                  format: binary
                  description: 待识别的图片文件（如 JPG、PNG）
                tool_type:
                  type: string
                  enum:
                    - hand_write
                  description: OCR识别工具类型，可选 hand_write（手写体识别）
                language_type:
                  type: string
                  enum:
                    - CHN_ENG
                    - AUTO
                    - ENG
                    - JAP
                    - KOR
                    - FRE
                    - SPA
                    - POR
                    - GER
                    - ITA
                    - RUS
                    - DAN
                    - DUT
                    - MAL
                    - SWE
                    - IND
                    - POL
                    - ROM
                    - TUR
                    - GRE
                    - HUN
                    - THA
                    - VIE
                    - ARA
                    - HIN
                  description: 语言/识别模型类型，可选 CHN_ENG等
                probability:
                  type: boolean
                  example: true
                  default: false
                  description: 是否返回置信度（概率）信息。true 为返回
              required:
                - file
                - tool_type
        required: true
      responses:
        '200':
          description: 结果获取成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/OCRResultResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    OCRResultResponse:
      type: object
      properties:
        task_id:
          type: string
          description: OCR识别任务ID
          example: ce2641ced3e34e67b47f3b0feeb25aee
        message:
          type: string
          description: 结果状态描述
          example: 成功
        status:
          type: string
          enum:
            - succeeded
            - failed
          description: 任务处理状态
          example: succeeded
        words_result_num:
          type: integer
          description: 识别到的文本块/行数
          example: 4
        words_result:
          type: array
          description: 每个识别文本块/行的详细结果
          items:
            type: object
            properties:
              location:
                type: object
                properties:
                  left:
                    type: integer
                    example: 79
                  top:
                    type: integer
                    example: 122
                  width:
                    type: integer
                    example: 1483
                  height:
                    type: integer
                    example: 182
                required:
                  - left
                  - top
                  - width
                  - height
              words:
                type: string
                description: 识别出的文本内容
                example: 你好,世界!
              probability:
                type: object
                description: 置信度信息
                properties:
                  average:
                    type: number
                    example: 0.7320847511
                  variance:
                    type: number
                    example: 0.08768635988
                  min:
                    type: number
                    example: 0.3193874359
                required:
                  - average
                  - variance
                  - min
            required:
              - location
              - words
              - probability
      required:
        - task_id
        - message
        - status
        - words_result_num
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

# 内容安全

> 可对文本、图片、音频、视频格式类型的内容进行检测，精准识别涉黄、涉暴、违法违规等风险内容，并输出结构化审核结果（包括内容类型、风险类型及具体风险内容片段），快速定位和处理违规信息。



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/moderations
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
  /paas/v4/moderations:
    post:
      tags:
        - 工具 API
      summary: 内容安全
      description: >-
        可对文本、图片、音频、视频格式类型的内容进行检测，精准识别涉黄、涉暴、违法违规等风险内容，并输出结构化审核结果（包括内容类型、风险类型及具体风险内容片段），快速定位和处理违规信息。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ModerationRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ModerationResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    ModerationRequest:
      type: object
      properties:
        model:
          type: string
          description: 安全模型
          enum:
            - moderation
          default: moderation
        input:
          description: |-
            需要审核的内容
             纯文本字符串（最大输入长度：`2000` 字符）
            多媒体对象(`type+url`)
             图片：图片小于 `10M`，最低分辨率 `20* 20`，不超过 `6000 * 6000`
            视频：推荐视频时长 `30` 秒
            音频：推荐音频时长 `60` 秒
          oneOf:
            - type: string
              description: 需要审核的文本内容
              example: 审核内容安全样例字符串。
            - type: object
              description: 单个多模态审核内容
              properties:
                type:
                  type: string
                  description: 内容类型，可以是文本、视频链接、音频链接或图片链接
                  enum:
                    - text
                    - image_url
                    - video_url
                    - audio_url
                text:
                  type: string
                  description: 当 `type` 为 `text` 时的文本内容
                  example: 待审核内容
                image_url:
                  type: object
                  description: 当 `type` 为 `image_url` 时的图片参数
                  properties:
                    url:
                      type: string
                      description: 当 `type` 为 `image_url` 时的图片链接地址
                video_url:
                  type: object
                  description: 当 `type` 为 `video_url` 时的视频参数
                  properties:
                    url:
                      type: string
                      description: 当 `type` 为 `video_url` 时的视频链接地址
                audio_url:
                  type: object
                  description: 当 `type` 为 `audio_url` 时的音频参数
                  properties:
                    url:
                      type: string
                      description: 当 `type` 为 `audio_url` 时的音频链接地址
              required:
                - type
            - type: array
              description: 审核多模态内容数组，支持文本、文件和图片
              items:
                type: object
                properties:
                  type:
                    type: string
                    description: 内容类型，可以是文本、视频链接、音频链接或图片链接
                    enum:
                      - text
                      - image_url
                      - video_url
                      - audio_url
                  text:
                    type: string
                    description: 当 `type` 为 `text` 时的文本内容
                    example: 待审核内容
                  image_url:
                    type: object
                    description: 当 `type` 为 `image_url` 时的图片参数
                    properties:
                      url:
                        type: string
                        description: 当 `type` 为 `image_url` 时的图片链接地址
                  video_url:
                    type: object
                    description: 当 `type` 为 `video_url` 时的视频参数
                    properties:
                      url:
                        type: string
                        description: 当 `type` 为 `video_url` 时的视频链接地址
                  audio_url:
                    type: object
                    description: 当 `type` 为 `audio_url` 时的音频参数
                    properties:
                      url:
                        type: string
                        description: 当 `type` 为 `audio_url` 时的音频链接地址
      required:
        - model
        - input
    ModerationResponse:
      type: object
      properties:
        id:
          description: 任务 `ID`
          type: string
        created:
          type: integer
          description: 请求创建时间，是以秒为单位的 `Unix` 时间戳
        request_id:
          type: string
          description: 请求标识符
        result_list:
          type: array
          items:
            type: object
            properties:
              content_type:
                description: 内容类型
                type: string
              risk_level:
                type: string
                description: 风险判定结果`PASS`：正常内容`REVIEW`：可疑内容`REJECT`：违规内容
              risk_type:
                type: array
                description: 风险类型列表
                items:
                  type: string
        usage:
          type: object
          properties:
            moderation_text:
              type: object
              properties:
                call_count:
                  type: number
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

# 文件解析

> 创建文件解析任务，支持多种文件格式和解析工具。见 [文件解析服务](https://docs.bigmodel.cn/cn/guide/tools/file-parser)



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/files/parser/create
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
  /paas/v4/files/parser/create:
    post:
      tags:
        - 工具 API
      summary: 文件解析
      description: 创建文件解析任务，支持多种文件格式和解析工具。见 [文件解析服务](https://docs.bigmodel.cn/cn/guide/tools/file-parser)
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file:
                  type: string
                  format: binary
                  description: 待解析文件
                tool_type:
                  type: string
                  enum:
                    - lite
                    - expert
                    - prime
                  description: 使用的解析工具类型
                file_type:
                  type: string
                  enum:
                    - PDF
                    - DOCX
                    - DOC
                    - XLS
                    - XLSX
                    - PPT
                    - PPTX
                    - PNG
                    - JPG
                    - JPEG
                    - CSV
                    - TXT
                    - MD
                    - HTML
                    - BMP
                    - GIF
                    - WEBP
                    - HEIC
                    - EPS
                    - ICNS
                    - IM
                    - PCX
                    - PPM
                    - TIFF
                    - XBM
                    - HEIF
                    - JP2
                  description: >-
                    文件类型。Lite支持：pdf,docx,doc,xls,xlsx,ppt,pptx,png,jpg,jpeg,csv,txt,md。Expert支持：pdf。Prime支持：pdf,docx,doc,xls,xlsx,ppt,pptx,png,jpg,jpeg,csv,txt,md,html,bmp,gif,webp,heic,eps,icns,im,pcx,ppm,tiff,xbm,heif,jp2
              required:
                - file
                - tool_type
        required: true
      responses:
        '200':
          description: 任务创建成功
          content:
            application/json:
              schema:
                type: object
                properties:
                  success:
                    type: boolean
                    example: true
                  message:
                    type: string
                    example: 任务创建成功
                  task_id:
                    type: string
                    example: task_123456789
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
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

# 文件解析(同步)

> 创建文件解析任务，支持多种文件格式和解析工具。见 [文件解析服务](https://docs.bigmodel.cn/cn/guide/tools/file-parser)



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/files/parser/sync
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
  /paas/v4/files/parser/sync:
    post:
      tags:
        - 工具 API
      summary: 文件解析(同步)
      description: 创建文件解析任务，支持多种文件格式和解析工具。见 [文件解析服务](https://docs.bigmodel.cn/cn/guide/tools/file-parser)
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file:
                  type: string
                  format: binary
                  description: 待解析文件
                tool_type:
                  type: string
                  enum:
                    - prime-sync
                  description: 使用的解析工具类型
                file_type:
                  type: string
                  enum:
                    - WPS
                    - PDF
                    - DOCX
                    - DOC
                    - XLS
                    - XLSX
                    - PPT
                    - PPTX
                    - PNG
                    - JPG
                    - JPEG
                    - CSV
                    - TXT
                    - MD
                    - HTML
                    - BMP
                    - GIF
                    - WEBP
                    - HEIC
                    - EPS
                    - ICNS
                    - IM
                    - PCX
                    - PPM
                    - TIFF
                    - XBM
                    - HEIF
                    - JP2
                  description: >-
                    文件类型支持：pdf,docx,doc,xls,xlsx,ppt,pptx,png,jpg,jpeg,csv,txt,md,html,bmp,gif,webp,heic,eps,icns,im,pcx,ppm,tiff,xbm,heif,jp2
              required:
                - file
                - tool_type
        required: true
      responses:
        '200':
          description: 结果获取成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FileParseResultResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    FileParseResultResponse:
      type: object
      properties:
        status:
          type: string
          enum:
            - processing
            - succeeded
            - failed
          description: 任务处理状态
          example: succeeded
        message:
          type: string
          description: 结果状态描述
          example: 结果获取成功
        content:
          type: string
          description: 当`format_type=text`时返回的解析文本内容
          example: 这是解析后的文本内容...
          nullable: true
        task_id:
          type: string
          description: 文件解析任务`ID`
          example: task_123456789
        parsing_result_url:
          type: string
          description: 当`format_type=download_link`时返回的结果下载链接
          example: https://example.com/download/result.zip
          nullable: true
      required:
        - status
        - message
        - task_id
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

# 网络搜索

> `Web Search API` 是一个专给大模型用的搜索引擎，在传统搜索引擎网页读取、排序的能力基础上，增强了意图识别能力，返回更适合大模型处理的结果（网页标题、`URL`、摘要、名称、图标等）。支持意图增强检索、结构化输出和多引擎支持。见 [网络搜索服务](https://docs.bigmodel.cn/cn/guide/tools/web-search)



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/web_search
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
  /paas/v4/web_search:
    post:
      tags:
        - 工具 API
      summary: 网络搜索
      description: >-
        `Web Search API`
        是一个专给大模型用的搜索引擎，在传统搜索引擎网页读取、排序的能力基础上，增强了意图识别能力，返回更适合大模型处理的结果（网页标题、`URL`、摘要、名称、图标等）。支持意图增强检索、结构化输出和多引擎支持。见
        [网络搜索服务](https://docs.bigmodel.cn/cn/guide/tools/web-search)
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/WebSearchRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/WebSearchResponse'
        default:
          description: >-
            请求失败。可能的错误码：1701-网络搜索并发已达上限，请稍后重试或减少并发请求；1702-系统未找到可用的搜索引擎服务，请检查配置或联系管理员；1703-搜索引擎未返回有效数据，请调整查询条件。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    WebSearchRequest:
      type: object
      properties:
        search_query:
          type: string
          description: 需要进行搜索的内容，建议搜索 `query` 不超过 `70` 个字符。
          maxLength: 70
        search_engine:
          type: string
          description: |-
            要调用的搜索引擎编码。目前支持：
            `search_std`：智谱基础版搜索引擎
            `search_pro`：智谱高阶版搜索引擎
            `search_pro_sogou`：搜狗
            `search_pro_quark`：夸克搜索
          example: search_std
          enum:
            - search_std
            - search_pro
            - search_pro_sogou
            - search_pro_quark
        search_intent:
          type: boolean
          description: |-
            是否进行搜索意图识别，默认不执行搜索意图识别。
            `true`：执行搜索意图识别，有搜索意图后执行搜索
            `false`：跳过搜索意图识别，直接执行搜索
          default: false
        count:
          type: integer
          description: |-
            返回结果的条数。可填范围：`1-50`，最大单次搜索返回`50`条，默认为`10`。
            支持的搜索引擎：`search_pro_sogou`、`search_std`、`search_pro`
            `search_pro_sogou`: 可选枚举值，10、20、30、40、50
          minimum: 1
          maximum: 50
          default: 10
        search_domain_filter:
          type: string
          description: |-
            用于限定搜索结果的范围，仅返回指定白名单域名的内容。
            白名单域名:（如 `www.example.com`）
            支持的搜索引擎：`search_std、search_pro 、search_pro_sogou`
        search_recency_filter:
          type: string
          description: >-
            搜索指定时间范围内的网页。默认为
            `noLimit`。可填值：`oneDay`（一天内）、`oneWeek`（一周内）、`oneMonth`（一个月内）、`oneYear`（一年内）、`noLimit`（不限，默认）。支持的搜索引擎：`search_std、search_pro、search_pro_Sogou、search_pro_quark`
          default: noLimit
          enum:
            - oneDay
            - oneWeek
            - oneMonth
            - oneYear
            - noLimit
        content_size:
          type: string
          description: >-
            控制返回网页内容的长短。`medium`：返回摘要信息，满足大模型的基础推理需求，满足常规问答任务的信息检索需求。`high`：最大化上下文，信息量较大但内容详细，适合需要信息细节的场景。支持的搜索引擎：`search_std、search_pro、search_pro_Sogou、search_pro_quark`
          enum:
            - medium
            - high
        request_id:
          type: string
          description: 由用户端传递，需要唯一；用于区分每次请求的唯一标识符。如果用户端未提供，平台将默认生成。
        user_id:
          type: string
          description: >-
            终端用户的唯一`ID`，帮助平台对终端用户的非法活动、生成非法不当信息或其他滥用行为进行干预。`ID`长度要求：至少`6`个字符，最多`128`个字符。
          minLength: 6
          maxLength: 128
      required:
        - search_query
        - search_engine
        - search_intent
    WebSearchResponse:
      type: object
      properties:
        id:
          type: string
          description: 任务 ID
        created:
          type: integer
          description: 请求创建时间，是以秒为单位的 `Unix` 时间戳
        request_id:
          type: string
          description: 请求标识符
        search_intent:
          type: array
          description: 搜索意图结果
          items:
            type: object
            properties:
              query:
                type: string
                description: 原始搜索query
              intent:
                type: string
                description: >-
                  识别的意图类型。`SEARCH_ALL` = 搜索全网，`SEARCH_NONE` =
                  无搜索意图，`SEARCH_ALWAYS` = 强制搜索模式：当`search_intent=false`时返回此值
                enum:
                  - SEARCH_ALL
                  - SEARCH_NONE
                  - SEARCH_ALWAYS
              keywords:
                type: string
                description: 改写后的搜索关键词
        search_result:
          type: array
          description: 搜索结果
          items:
            type: object
            properties:
              title:
                type: string
                description: 标题
              content:
                type: string
                description: 内容摘要
              link:
                type: string
                description: 结果链接
              media:
                type: string
                description: 网站名称
              icon:
                type: string
                description: 网站图标
              refer:
                type: string
                description: 角标序号
              publish_date:
                type: string
                description: 网站发布时间
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

# 网页阅读

> 读取并解析指定 `URL` 的网页内容，可选择返回格式、支持控制缓存、图片保留与摘要选项等。



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/reader
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
  /paas/v4/reader:
    post:
      tags:
        - 工具 API
      summary: 网页阅读
      description: 读取并解析指定 `URL` 的网页内容，可选择返回格式、支持控制缓存、图片保留与摘要选项等。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ReaderRequest'
            examples:
              Basic:
                value:
                  url: https://www.example.com
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ReaderResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    ReaderRequest:
      type: object
      properties:
        url:
          type: string
          description: 需要抓取的`url`
        timeout:
          type: integer
          description: 请求超时时间（秒），默认值 `20`
          default: 20
        no_cache:
          type: boolean
          description: 是否禁用缓存（`true`/`false`），默认值 `false`
          default: false
        return_format:
          type: string
          description: 返回格式（如：`markdown`、`text`等），默认值 `markdown`
          default: markdown
        retain_images:
          type: boolean
          description: 是否保留图片（`true`/`false`），默认值 `true`
          default: true
        no_gfm:
          type: boolean
          description: 是否禁用 `GitHub Flavored Markdown`（`true`/`false`），默认值 `false`
          default: false
        keep_img_data_url:
          type: boolean
          description: 是否保留图片数据 `URL`（`true`/`false`），默认值 `false`
          default: false
        with_images_summary:
          type: boolean
          description: 是否包含图片摘要（`true`/`false`），默认值 `false`
          default: false
        with_links_summary:
          type: boolean
          description: 是否包含链接摘要（`true`/`false`），默认值 `false`
          default: false
      required:
        - url
    ReaderResponse:
      type: object
      properties:
        id:
          description: 任务 `ID`
          type: string
        created:
          type: integer
          format: int64
          description: 请求创建时间，是以秒为单位的 `Unix` 时间戳
        request_id:
          type: string
          description: 由用户端传递，需要唯一；用于区分每次请求的唯一标识符。如果用户端未提供，平台将默认生成。
        model:
          type: string
          description: 模型编码
        reader_result:
          type: object
          description: 网页阅读结果
          properties:
            content:
              type: string
              description: 网页解析后的主要内容（正文、图片、链接等标记）
            description:
              type: string
              description: 网页简要描述
            title:
              type: string
              description: 网页标题
            url:
              type: string
              description: 网页原始地址
            external:
              type: object
              description: 网页引用的外部资源对象
              properties:
                stylesheet:
                  type: object
                  description: 外部样式表集合
                  additionalProperties:
                    type: object
                    properties:
                      type:
                        type: string
                        description: 样式表类型，通常为`text/css`
            metadata:
              type: object
              description: 页面元数据信息
              properties:
                keywords:
                  type: string
                  description: 页面关键词
                viewport:
                  type: string
                  description: 页面视口设置
                description:
                  type: string
                  description: 元数据描述
                format-detection:
                  type: string
                  description: 格式检测设置，如`telephone=no`
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

# 解析结果

> 异步获取文件解析任务的结果，支持返回纯文本或下载链接格式。见 [文件解析服务](https://docs.bigmodel.cn/cn/guide/tools/file-parser)



## OpenAPI

````yaml /openapi/openapi.json get /paas/v4/files/parser/result/{taskId}/{format_type}
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
  /paas/v4/files/parser/result/{taskId}/{format_type}:
    get:
      tags:
        - 工具 API
      summary: 解析结果
      description: 异步获取文件解析任务的结果，支持返回纯文本或下载链接格式。见 [文件解析服务](https://docs.bigmodel.cn/cn/guide/tools/file-parser)
      parameters:
        - name: taskId
          in: path
          description: 文件解析任务ID
          required: true
          schema:
            type: string
            example: task_123456789
        - name: format_type
          in: path
          description: 结果返回格式类型
          required: true
          schema:
            type: string
            enum:
              - text
              - download_link
            example: text
      responses:
        '200':
          description: 结果获取成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FileParseResultResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    FileParseResultResponse:
      type: object
      properties:
        status:
          type: string
          enum:
            - processing
            - succeeded
            - failed
          description: 任务处理状态
          example: succeeded
        message:
          type: string
          description: 结果状态描述
          example: 结果获取成功
        content:
          type: string
          description: 当`format_type=text`时返回的解析文本内容
          example: 这是解析后的文本内容...
          nullable: true
        task_id:
          type: string
          description: 文件解析任务`ID`
          example: task_123456789
        parsing_result_url:
          type: string
          description: 当`format_type=download_link`时返回的结果下载链接
          example: https://example.com/download/result.zip
          nullable: true
      required:
        - status
        - message
        - task_id
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
