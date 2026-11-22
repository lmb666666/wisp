// Package http 的 Swagger 资源：OpenAPI YAML 以字符串嵌入（ENABLE_SWAGGER 时暴露）。
package http

// openAPISpec 核心 API 的 OpenAPI 3.0 描述（紧凑版，覆盖全部公开端点）。
const openAPISpec = `openapi: 3.0.3
info:
  title: Wisp API
  version: "0.5.0"
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
        visibility: { type: string, enum: [public] }
        published_at: { type: integer, description: unix 秒，可改（补发） }
        edited_at: { type: integer }
        source: { type: string, example: api }
        tags: { type: array, items: { type: string } }
        attachments: { type: array, items: { $ref: '#/components/schemas/Attachment' } }
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
  /ping:
    get:
      security: []
      summary: 存活探针
      responses: { "200": { description: OK } }
  /auth/profile:
    get:
      summary: 个人资料（用户名/昵称/Passkey 数）
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
    patch:
      summary: 修改昵称或密码（改密吊销其他会话）
      security: [{ sessionCookie: [] }]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                display_name: { type: string }
                current_password: { type: string, description: 改密时必填 }
                new_password: { type: string, description: 与 current_password 同时提供 }
      responses: { "200": { description: OK }, "401": { description: 当前密码错误 } }
  /auth/passkey/register/begin:
    post:
      summary: 开始 Passkey 注册（返回 WebAuthn 创建选项与仪式 ID）
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK }, "503": { description: Passkey 未启用 } }
  /auth/passkey/register/finish:
    post:
      summary: 完成 Passkey 注册（query.session 为仪式 ID）
      security: [{ sessionCookie: [] }]
      parameters: [{ name: session, in: query, required: true, schema: { type: string } }]
      responses: { "200": { description: OK } }
  /auth/passkey/login/begin:
    post:
      security: []
      summary: 开始 Passkey 登录（发现式，返回请求选项与仪式 ID）
      responses: { "200": { description: OK }, "503": { description: Passkey 未启用 } }
  /auth/passkey/login/finish:
    post:
      security: []
      summary: 完成 Passkey 登录并建立会话（query.session 为仪式 ID）
      parameters: [{ name: session, in: query, required: true, schema: { type: string } }]
      responses: { "200": { description: OK }, "401": { description: 仪式失败或过期 } }
  /auth/passkeys:
    get:
      summary: 已注册 Passkey 列表
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
  /auth/passkeys/{id}:
    delete:
      summary: 删除 Passkey
      security: [{ sessionCookie: [] }]
      parameters: [{ name: id, in: path, required: true, schema: { type: string } }]
      responses: { "204": { description: OK } }
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
      summary: 时间线（全部内容公开）
      parameters:
        - { name: limit, in: query, schema: { type: integer, default: 20, maximum: 50 } }
        - { name: before, in: query, schema: { type: string, description: 复合游标 published_at.id } }
        - { name: tag, in: query, schema: { type: string, description: 单标签过滤（兼容） } }
        - { name: tags, in: query, schema: { type: string, description: "JSON 数组或逗号分隔；ALL 语义" } }
        - { name: from, in: query, schema: { type: integer, description: unix 秒，含边界 } }
        - { name: to, in: query, schema: { type: integer, description: unix 秒，含边界 } }
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
                visibility: { type: string, default: public, description: 仅 public（私密发布已移除） }
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
      responses: { "200": { description: OK }, "404": { description: 不存在 } }
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
  /search:
    get:
      security: []
      summary: 全文搜索（≥3 字符 FTS5 trigram；短词 LIKE）
      parameters:
        - { name: q, in: query, required: true, schema: { type: string } }
        - { name: limit, in: query, schema: { type: integer } }
      responses: { "200": { description: OK } }
  /moods:
    get:
      security: []
      summary: 每日心情区间（公开）
      parameters:
        - { name: from, in: query, required: true, schema: { type: string, format: date } }
        - { name: to, in: query, required: true, schema: { type: string, format: date }, description: 跨度不超过 10 年 }
      responses: { "200": { description: OK }, "400": { description: 参数错误 } }
  /moods/latest:
    get:
      security: []
      summary: 最近一条心情（公开）
      responses: { "200": { description: OK }, "404": { description: 没有记录 } }
  /moods/{date}:
    put:
      summary: 记录或覆盖某天心情（需要 write；支持补记历史日期）
      parameters:
        - { name: date, in: path, required: true, schema: { type: string, format: date } }
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [mood]
              properties:
                mood: { type: string, enum: [happy, calm, okay, tired, low] }
      responses: { "200": { description: OK }, "400": { description: 参数错误 }, "403": { description: 作用域不足 } }
    delete:
      summary: 清除某天心情（需要 write）
      parameters:
        - { name: date, in: path, required: true, schema: { type: string, format: date } }
      responses: { "204": { description: OK }, "400": { description: 参数错误 }, "403": { description: 作用域不足 } }
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
      parameters:
        - { name: year, in: query, schema: { type: integer } }
        - { name: tz_offset, in: query, schema: { type: integer }, description: "UTC 偏移分钟数，如东八区 480" }
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
      responses: { "200": { description: zip 流 } }
  /admin/media:
    get:
      summary: 媒体库列表（复合游标分页，仅会话）
      security: [{ sessionCookie: [] }]
      parameters:
        - { name: kind, in: query, schema: { type: string, enum: [image, video, audio, file] } }
        - { name: storage, in: query, schema: { type: string } }
        - { name: orphan, in: query, schema: { type: integer, enum: [0, 1] } }
        - { name: limit, in: query, schema: { type: integer, maximum: 200, default: 50 } }
        - { name: before_ts, in: query, schema: { type: integer, description: 游标 created_at（unix 秒，0 = 首页） } }
        - { name: before_id, in: query, schema: { type: string, description: 游标附件 id } }
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
  /admin/settings/meting:
    get:
      summary: Meting 音乐解析端点配置（空串禁用音乐卡）
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
    put:
      summary: 保存 Meting 端点（http(s) URL）
      security: [{ sessionCookie: [] }]
      requestBody:
        content:
          application/json:
            schema: { type: object, properties: { url: { type: string } } }
      responses: { "200": { description: OK } }
  /admin/webhooks:
    get:
      summary: Webhook 列表（secret 脱敏）
      security: [{ sessionCookie: [] }]
      responses: { "200": { description: OK } }
    post:
      summary: 注册 Webhook（secret 仅创建时返回一次；HMAC-SHA256 签名投递）
      security: [{ sessionCookie: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [url, events]
              properties:
                url: { type: string, maxLength: 2048 }
                events:
                  type: string
                  description: 逗号分隔（post.created/updated/deleted, mood.updated/deleted）
      responses: { "201": { description: OK } }
  /admin/webhooks/{id}:
    delete:
      summary: 删除 Webhook
      security: [{ sessionCookie: [] }]
      parameters: [{ name: id, in: path, required: true, schema: { type: string } }]
      responses: { "204": { description: OK } }
  /admin/import:
    post:
      summary: 导入旧数据（multipart file + source；≤5000 帖 / ≤256MB，按内容指纹去重）
      security: [{ sessionCookie: [] }]
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              required: [source, file]
              properties:
                source: { type: string, enum: [wisp, memos, moments, ech0] }
                file: { type: string, format: binary }
      responses: { "200": { description: 导入统计 }, "400": { description: 解析失败 } }
`

// swaggerUI 极简壳：UI 从 CDN 加载（版本与 SRI 固定，防 CDN 篡改），
// 离线环境直接阅读 /swagger/openapi.yaml。
const swaggerUI = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"/><title>Wisp API</title>
<meta name="robots" content="noindex"/>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.35.1/swagger-ui.css" integrity="sha384-WlYgdmXYOOBWL5apwjDEZwN+U3HmXSSu5R6V5aGxEcf7z+KuRNwBhoxKVN1i3D5I" crossorigin="anonymous"/></head>
<body><div id="swagger"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.35.1/swagger-ui-bundle.js" integrity="sha384-J5eKRXKO1KzUTPkO0GRmGH2WEYUg6SSZlufilxvTwI6nmhLumjNWFMby3QQnyobZ" crossorigin="anonymous"></script>
<script>SwaggerUIBundle({url:"/swagger/openapi.yaml",dom_id:"#swagger"})</script></body></html>`
