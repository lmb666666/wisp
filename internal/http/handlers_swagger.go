// Package http 的 Swagger 资源：OpenAPI YAML 以字符串嵌入（ENABLE_SWAGGER 时暴露）。
package http

// openAPISpec 核心 API 的 OpenAPI 3.0 描述（紧凑版，覆盖全部公开端点）。
const openAPISpec = `openapi: 3.0.3
info:
  title: Wisp API
  version: "0.2"
  description: |
    Wisp 自托管说说 API。认证方式二选一：
    1. 会话 Cookie（Web UI 使用，HttpOnly）；
    2. PAT：Authorization: Bearer wpat_xxx（read=只读，write=读写）。
servers:
  - url: /api/v1
components:
  securitySchemes:
    bearerPAT: { type: http, scheme: bearer, description: "wpat_ 前缀令牌" }
    sessionCookie: { type: apiKey, in: cookie, name: wisp_session, description: "Web 会话（管理端点）" }
  schemas:
    Error:
      type: object
      properties:
        error:
          type: object
          properties:
            code: { type: string }
            message: { type: string }
    Attachment:
      type: object
      properties:
        id: { type: string }
        kind: { type: string, enum: [image, video, audio, file] }
        mime: { type: string }
        url: { type: string }
        thumb_url: { type: string }
        poster_url: { type: string }
        blurhash: { type: string }
        width: { type: integer }
        height: { type: integer }
    Post:
      type: object
      properties:
        id: { type: string }
        content_md: { type: string }
        content_html: { type: string }
        visibility: { type: string, enum: [public, private] }
        published_at: { type: integer, description: unix 秒，可改（补发） }
        edited_at: { type: integer }
        source: { type: string, example: api }
        tags: { type: array, items: { type: string } }
        attachments: { type: array, items: { $ref: '#/components/schemas/Attachment' } }
        reactions:
          type: array
          items:
            type: object
            properties:
              emoji: { type: string }
              count: { type: integer }
              mine: { type: boolean }
        blocks:
          type: array
          description: 扩展卡片（与正文分离）：音乐 / GitHub / 位置 / 网站 / 视频
          items:
            type: object
            properties:
              type: { type: string, enum: [music, github, location, website, video] }
              url: { type: string }
              name: { type: string, description: location 名称 }
              lat: { type: number, description: location 纬度 }
              lng: { type: number, description: location 经度 }
              title: { type: string, description: 服务端解析结果 }
              cover: { type: string }
              audio_url: { type: string, description: Meting 播放地址 }
              embed_url: { type: string, description: 视频点击后加载的播放器地址 }
              provider: { type: string }
security:
  - bearerPAT: []
paths:
  /auth/status:
    get:
      security: []
      summary: 站点是否已初始化
      responses: { "200": { description: OK } }
  /auth/init:
    post:
      security: []
      summary: 首启向导创建 owner（仅零用户时可用）
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [username, password]
              properties:
                username: { type: string }
                password: { type: string, minLength: 8 }
                display_name: { type: string }
      responses: { "201": { description: 创建并登录 }, "409": { description: 已初始化 } }
  /auth/login:
    post:
      security: []
      summary: 登录（设置会话 Cookie）
      responses: { "200": { description: OK }, "401": { description: 凭证错误 } }
  /auth/logout:
    post: { summary: 登出, responses: { "204": { description: OK } } }
  /auth/me:
    get: { summary: 当前身份, responses: { "200": { description: OK } } }
  /auth/tokens:
    get: { summary: PAT 列表（仅会话）, responses: { "200": { description: OK } } }
    post:
      summary: 创建 PAT（明文仅返回一次）
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name: { type: string }
                scopes: { type: string, enum: [read, "read,write"] }
      responses: { "201": { description: OK } }
  /auth/tokens/{id}:
    delete: { summary: 吊销 PAT, parameters: [{ name: id, in: path, required: true, schema: { type: string } }], responses: { "204": { description: OK } } }
  /posts:
    get:
      security: []
      summary: 时间线（游客公开内容；PAT/会话含私密）
      parameters:
        - { name: limit, in: query, schema: { type: integer, default: 20, maximum: 50 } }
        - { name: before, in: query, schema: { type: string, description: 复合游标 published_at.id } }
        - { name: tag, in: query, schema: { type: string, description: 单标签过滤（兼容） } }
        - { name: tags, in: query, schema: { type: string, description: "JSON 数组或逗号分隔；ALL 语义" } }
        - { name: from, in: query, schema: { type: integer, description: unix 秒，含边界 } }
        - { name: to, in: query, schema: { type: integer, description: unix 秒，含边界 } }
        - { name: vis, in: query, schema: { type: string, enum: [public, private], description: 游客强制 public } }
      responses: { "200": { description: OK } }
    post:
      summary: 发帖（需要 write）
      parameters:
        - { name: Idempotency-Key, in: header, schema: { type: string }, description: 离线重试幂等 }
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [content_md]
              properties:
                content_md: { type: string }
                visibility: { type: string, default: public }
                published_at: { type: integer }
                attachment_ids: { type: array, items: { type: string } }
                client: { type: string }
                blocks:
                  type: array
                  items:
                    type: object
                    properties:
                      type: { type: string, enum: [music, github, location, website, video] }
                      url: { type: string, description: 链接 }
                      name: { type: string, description: location 名称 }
                      lat: { type: number }
                      lng: { type: number }
      responses: { "201": { description: OK }, "403": { description: 作用域不足 } }
  /posts/{id}:
    get:
      security: []
      parameters: [{ name: id, in: path, required: true, schema: { type: string } }]
      responses: { "200": { description: OK }, "404": { description: 不存在或私密 } }
    patch:
      summary: 编辑（content_md 变更会重渲染并重建标签）
      parameters: [{ name: id, in: path, required: true, schema: { type: string } }]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                content_md: { type: string }
                visibility: { type: string }
                location: { type: string }
                published_at: { type: integer }
                blocks:
                  type: array
                  items:
                    type: object
                    properties:
                      type: { type: string, enum: [music, github, location, website, video] }
                      url: { type: string }
                      name: { type: string }
                      lat: { type: number }
                      lng: { type: number }
      responses: { "200": { description: OK } }
    delete:
      summary: 删除（连带清理本地附件文件）
      parameters: [{ name: id, in: path, required: true, schema: { type: string } }]
      responses: { "204": { description: OK } }
  /posts/{id}/reactions:
    post:
      security: []
      summary: 点赞（需 X-Client-ID 头，幂等）
      parameters: [{ name: id, in: path, required: true, schema: { type: string } }]
      requestBody:
        content:
          application/json:
            schema: { type: object, required: [emoji], properties: { emoji: { type: string } } }
      responses: { "204": { description: OK } }
    delete:
      security: []
      summary: 取消点赞
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
        - { name: emoji, in: query, required: true, schema: { type: string } }
      responses: { "204": { description: OK } }
  /search:
    get:
      security: []
      summary: 全文搜索（≥3 字符 FTS5 trigram；短词 LIKE）
      parameters:
        - { name: q, in: query, required: true, schema: { type: string } }
        - { name: limit, in: query, schema: { type: integer } }
      responses: { "200": { description: OK } }
  /tags:
    get:
      security: []
      summary: 标签聚合（含 id 与计数）
      responses: { "200": { description: OK } }
  /tags/{id}:
    patch:
      summary: 重命名标签（仅会话；重名返回 409）
      security: [{ sessionCookie: [] }]
      parameters: [{ name: id, in: path, required: true, schema: { type: integer } }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { type: object, required: [name], properties: { name: { type: string } } }
      responses: { "200": { description: OK }, "409": { description: 同名标签已存在 } }
    delete:
      summary: 删除标签（只解除帖子关联）
      security: [{ sessionCookie: [] }]
      parameters: [{ name: id, in: path, required: true, schema: { type: integer } }]
      responses: { "204": { description: OK } }
  /tags/merge:
    post:
      summary: 合并标签到已有或新名称（事务；不修改正文）
      security: [{ sessionCookie: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [from_ids, into]
              properties:
                from_ids: { type: array, items: { type: integer } }
                into: { type: string, description: 目标标签名，不存在则创建 }
      responses: { "200": { description: OK }, "404": { description: 源标签不存在 } }
  /attachments:
    post:
      summary: 上传附件（multipart file；sha256 秒传）
      requestBody:
        content:
          multipart/form-data:
            schema: { type: object, properties: { file: { type: string, format: binary } } }
      responses: { "201": { description: OK }, "413": { description: 超过 32MB } }
  /attachments/{id}/poster:
    post:
      summary: 上传视频封面（客户端截帧，JPEG ≤5MB）
      parameters: [{ name: id, in: path, required: true, schema: { type: string } }]
      responses: { "200": { description: OK } }
  /stats/daily:
    get:
      security: []
      summary: 每日发帖计数（热力图）
      parameters: [{ name: year, in: query, schema: { type: integer } }]
      responses: { "200": { description: OK } }
  /admin/overview:
    get:
      summary: 站点概览（帖子/今日/媒体/标签/DB/备份/版本/运行时长，仅会话）
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
  /admin/backup:
    get:
      summary: 备份数据库（zip，仅会话）
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
  /admin/settings/backup:
    get:
      summary: 自动备份设置与 scheduled-* 快照列表
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
    put:
      summary: 保存自动备份设置（preserve last_run_at）
      security: [{ sessionCookie: [] }]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                enabled: { type: boolean }
                interval_hours: { type: integer, minimum: 1, maximum: 720 }
                retention: { type: integer, minimum: 1, maximum: 365 }
      responses: { "200": { description: OK } }
  /admin/backup/run:
    post:
      summary: 立即生成数据库快照（即使自动备份关闭）
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
  /admin/export/capsule:
    get:
      summary: 导出内容包（zip：Markdown + 原图 + 扩展卡片）
      security: [{ sessionCookie: [] }]
      parameters: [{ name: include_private, in: query, schema: { type: integer, enum: [0, 1] } }]
      responses: { "200": { description: zip 流 } }
  /admin/media:
    get:
      summary: 媒体库列表（仅会话）
      security: [{ sessionCookie: [] }]
      parameters:
        - { name: kind, in: query, schema: { type: string, enum: [image, video, audio, file] } }
        - { name: storage, in: query, schema: { type: string } }
        - { name: orphan, in: query, schema: { type: integer, enum: [0, 1] } }
        - { name: limit, in: query, schema: { type: integer, maximum: 200, default: 50 } }
        - { name: offset, in: query, schema: { type: integer, default: 0 } }
      responses: { "200": { description: OK } }
  /admin/media/stats:
    get:
      summary: 媒体占用统计（按 kind/storage）
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
  /admin/media/{id}:
    delete:
      summary: 删除媒体（已绑定默认 409；force=1 强制）
      security: [{ sessionCookie: [] }]
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
        - { name: force, in: query, schema: { type: integer, enum: [0, 1] } }
      responses: { "204": { description: OK }, "409": { description: 仍被帖子引用 } }
  /admin/settings/storage:
    get:
      summary: 存储配置（密钥脱敏）
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
    put:
      summary: 保存存储配置并热切换
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
  /admin/settings/storage/test:
    post:
      summary: 存储连通性测试
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
  /admin/settings/geo:
    get:
      summary: 地理编码端点配置（Nominatim 兼容；空串禁用搜索/反查）
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
`

// swaggerUI 极简壳：UI 从 CDN 加载，离线环境直接阅读 /swagger/openapi.yaml。
const swaggerUI = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"/><title>Wisp API</title>
<meta name="robots" content="noindex"/>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"/></head>
<body><div id="swagger"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url:"/swagger/openapi.yaml",dom_id:"#swagger"})</script></body></html>`
