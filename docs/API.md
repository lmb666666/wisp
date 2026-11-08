# API

Base URL：`/api/v1`。除内容包 / 备份 / 媒体外，响应均为 JSON。开启 `WISP_ENABLE_SWAGGER=true` 后可在 `/swagger` 查看，机器可读描述在 `/swagger/openapi.yaml`。

## 认证

| 方式 | 用途 | 说明 |
|---|---|---|
| Session Cookie | Web UI | `wisp_session`，HttpOnly；写请求校验 Origin |
| PAT | CLI / 第三方 | `Authorization: Bearer wpat_...`；创建时只显示一次 |

PAT 作用域：

- `read`：读取
- `read,write`：读取 + 发帖 / 编辑 / 删除 / 写心情

PAT 在「设置 → API 令牌」创建。导入、备份、媒体库、存储 / 集成设置等管理端点只接受会话，不接受 PAT。

## 端点

### 认证与账号

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/auth/status` | 是否已初始化；匿名不返回用户名 |
| `POST` | `/auth/init` | 首启创建所有者（仅一次） |
| `POST` | `/auth/login` | 用户名密码登录 |
| `POST` | `/auth/logout` | 吊销当前会话 |
| `GET` | `/auth/me` | 当前身份；未登录返回匿名 |
| `GET/PATCH` | `/auth/profile` | 昵称 / 改密（会话） |
| `POST` | `/auth/passkey/register/begin` / `finish` | 注册 Passkey（会话） |
| `POST` | `/auth/passkey/login/begin` / `finish` | Passkey 登录 |
| `GET` | `/auth/passkeys` | Passkey 列表（会话） |
| `DELETE` | `/auth/passkeys/{id}` | 删除 Passkey（会话） |
| `GET/POST` | `/auth/tokens` | PAT 列表 / 创建（会话） |
| `DELETE` | `/auth/tokens/{id}` | 删除 PAT（会话） |

### 内容

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| `GET` | `/posts` | 公开 | 时间线；`limit`、`before`、`tag` / `tags`、`from` / `to` |
| `POST` | `/posts` | write | 发帖；支持 `Idempotency-Key` |
| `GET` | `/posts/{id}` | 公开 | 详情 |
| `PATCH` | `/posts/{id}` | write | 编辑 |
| `DELETE` | `/posts/{id}` | write | 删除 |
| `GET` | `/search?q=` | 公开 | 全文搜索（≥3 字符 FTS5，短词 LIKE） |
| `GET` | `/tags` | 公开 | 标签聚合（仅统计有帖子的标签） |
| `PATCH` | `/tags/{id}` | 会话 | 重命名 |
| `POST` | `/tags/merge` | 会话 | 合并 |
| `DELETE` | `/tags/{id}` | 会话 | 只解绑，不删帖 |
| `POST` | `/attachments` | write | multipart `file` 上传 |
| `POST` | `/attachments/{id}/poster` | write | 视频封面（multipart `file`） |
| `GET` | `/stats/daily?year=&tz_offset=` | 公开 | 年度热力图；`tz_offset` 为 UTC 偏移分钟（东八区 480） |
| `GET` | `/moods?from=&to=` | 公开 | 每日心情区间（跨度 ≤ 10 年） |
| `GET` | `/moods/latest` | 公开 | 最近一条心情；没有记录时 `404` |
| `PUT` | `/moods/{date}` | write | 记录 / 覆盖某天心情；可补记历史 |
| `DELETE` | `/moods/{date}` | write | 清除某天心情；幂等 |

### 管理（仅会话）

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/admin/overview` | 站点概览 |
| `GET` | `/admin/backup` | 下载当前数据库 zip |
| `GET/PUT` | `/admin/settings/backup` | 自动备份设置 |
| `POST` | `/admin/backup/run` | 立即备份 |
| `GET` | `/admin/export/capsule` | 导出 Wisp 内容包 |
| `POST` | `/admin/import` | 导入（multipart `source` + `file`） |
| `GET` | `/admin/media` | 媒体库 |
| `GET` | `/admin/media/stats` | 媒体占用统计 |
| `DELETE` | `/admin/media/{id}` | 删除附件 |
| `GET/PUT` | `/admin/settings/storage` | 存储驱动设置 |
| `POST` | `/admin/settings/storage/test` | 存储连通性测试 |
| `GET` | `/admin/settings/geo` | 位置搜索端点信息 |
| `GET/PUT` | `/admin/settings/meting` | Meting 音乐端点 |
| `GET/POST` | `/admin/webhooks` | Webhook 列表 / 创建 |
| `DELETE` | `/admin/webhooks/{id}` | 删除 Webhook |

## 分页

时间线使用复合游标：响应里的 `next_cursor` 原样作为下一次的 `before`。不要自行拼游标。

```bash
curl "https://wisp.example.com/api/v1/posts?limit=20&before=1792230000.01M..."
```

## 筛选

- `tag=设计`：单标签（兼容）
- `tags=["设计","日记"]`：JSON 数组；也兼容逗号分隔。**ALL 语义**
- `from` / `to`：unix 秒，含边界

## 发帖

```bash
curl -X POST https://wisp.example.com/api/v1/posts \
  -H "Authorization: Bearer wpat_xxx" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{
    "content_md": "今天也在好好生活。 #日记",
    "visibility": "public",
    "attachment_ids": [],
    "blocks": [
      {"type":"website","url":"https://example.com/article"}
    ]
  }'
```

- `visibility` 只接受 `public`（私密发布已移除）。
- `blocks` 支持 `music` / `github` / `website` / `video` / `location`，最多 4 个；服务端清洗非法 URL 与展示字段。
- `Idempotency-Key` 用于离线重试；同一 key 重放返回首次结果。

## 每日心情

一天一条，`date` 为作者本地日期 `YYYY-MM-DD`。固定 5 档：`happy`（开心）/ `calm`（平静）/ `okay`（一般）/ `tired`（疲惫）/ `low`（低落）。

读取公开；写入需要会话或 `read,write` PAT。历史日期可补记，最晚到服务端 UTC 明天；同一天重复 `PUT` 覆盖旧值。

```bash
curl -X PUT https://wisp.example.com/api/v1/moods/2026-11-08 \
  -H "Authorization: Bearer wpat_xxx" \
  -H "Content-Type: application/json" \
  -d '{"mood":"calm"}'

curl "https://wisp.example.com/api/v1/moods?from=2026-01-01&to=2026-12-31"
```

响应：

```json
{"items":[{"date":"2026-11-08","mood":"calm"}]}
```

## 上传

```bash
curl -X POST https://wisp.example.com/api/v1/attachments \
  -H "Authorization: Bearer wpat_xxx" \
  -F "file=@photo.jpg"
```

单文件 ≤32MB；同内容且未绑定帖子时会秒传。返回 `url`、`thumb_url`、`blurhash`、`width`、`height` 等字段。

## Webhook

可订阅事件：`post.created`、`post.updated`、`post.deleted`、`mood.updated`、`mood.deleted`。

- 请求头：`X-Wisp-Event`、`X-Wisp-Delivery`、`X-Wisp-Signature: sha256=<hex>`
- 签名：`HMAC-SHA256(secret, raw body)`
- 失败按 2s / 8s / 30s 退避重试；非 2xx 视为失败
- 进程退出前会尽量排空队列

```json
{"event":"mood.updated","timestamp":1792230000,"data":{"date":"2026-11-08","mood":"calm"}}
```

## 限流与错误

默认 `/api/v1` 每秒 20 次 / IP；登录与首启每秒 1 次 / IP（Burst 3）。超限返回 `429`。

```json
{"error":{"code":"bad_request","message":"..."}}
```

常见状态：`400` 参数 / 校验、`401` 未认证、`403` 作用域 / CSRF、`404` 不存在、`409` 冲突、`413` 过大、`429` 限流。

## CORS

只有 `WISP_CORS_ORIGINS` 白名单中的来源可跨域调用 `/api/v1`，且不带 Cookie；跨域请使用 PAT。

## RSS 与健康检查

以下端点不在 `/api/v1` 下：

- `GET /feed.xml`：RSS 2.0；`?tag=` 可筛选
- `GET /healthz`：进程存活
- `GET /readyz`：数据库可读；故障时 `503`
