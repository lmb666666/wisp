# API

Base URL：`/api/v1`。所有响应为 JSON（内容包/备份/媒体除外）。开启 `WISP_ENABLE_SWAGGER=true` 后可在 `/swagger` 查看，机器可读描述在 `/swagger/openapi.yaml`。

## 认证

两种方式：

| 方式 | 用途 | 说明 |
|---|---|---|
| Session Cookie | Web UI | `wisp_session`，HttpOnly；写请求校验 Origin |
| PAT | CLI / 第三方 | `Authorization: Bearer wpat_...`；创建时只显示一次 |

PAT 作用域：

- `read`：读取（作者视角可读私密）
- `read,write`：读取 + 发帖/编辑/删除等

PAT 在「设置 → API 令牌」创建。管理端点（导入、备份、媒体库、存储设置）只接受会话，不接受 PAT。

## 常用端点

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/posts` | 时间线；`limit`、`before`、`tag`/`tags`、`from`/`to`、`vis` |
| `POST` | `/posts` | 发帖（write）；支持 `Idempotency-Key` |
| `GET` | `/posts/{id}` | 详情；private 对游客 404 |
| `PATCH` | `/posts/{id}` | 编辑（write） |
| `DELETE` | `/posts/{id}` | 删除（write） |
| `POST` | `/posts/{id}/reactions` | 点赞；需要 `X-Client-ID`；幂等 |
| `DELETE` | `/posts/{id}/reactions?emoji=...` | 取消点赞 |
| `GET` | `/search?q=` | 全文搜索（≥3 字符 FTS5，短词 LIKE） |
| `GET` | `/tags` | 标签聚合（仅统计有帖子的标签） |
| `PATCH` | `/tags/{id}` | 重命名（会话） |
| `POST` | `/tags/merge` | 合并（会话） |
| `DELETE` | `/tags/{id}` | 只解绑（会话） |
| `POST` | `/attachments` | multipart `file` 上传（write） |
| `GET` | `/stats/daily?year=` | 年度热力图 |
| `GET` | `/feed.xml` | RSS 2.0；`?tag=` 可筛选 |
| `GET` | `/admin/overview` | 站点概览（会话） |
| `GET/PUT` | `/admin/settings/backup` | 自动备份设置（会话） |
| `POST` | `/admin/backup/run` | 立即备份（会话） |
| `GET` | `/admin/export/capsule` | 导出内容包（会话） |
| `POST` | `/admin/import` | 导入（会话，multipart `source` + `file`） |
| `GET` | `/admin/media` | 媒体库（会话） |

## 分页

时间线使用复合游标：响应里的 `next_cursor` 原样作为下一次的 `before`。不要自行拼游标。

```bash
curl "https://wisp.example.com/api/v1/posts?limit=20&before=1792230000.01M..."
```

## 筛选

- `tag=设计`：单标签（兼容）
- `tags=["设计","日记"]`：JSON 数组；也兼容逗号分隔。**ALL 语义**
- `from` / `to`：unix 秒，含边界
- `vis=public|private`：作者可筛；游客强制 public

## 发帖示例

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

`blocks` 支持 `music` / `github` / `website` / `video` / `location`，最多 4 个；服务端会清洗非法 URL 与展示字段。

## 上传

```bash
curl -X POST https://wisp.example.com/api/v1/attachments \
  -H "Authorization: Bearer wpat_xxx" \
  -F "file=@photo.jpg"
```

单文件 ≤32MB；同内容且未绑定帖子时会秒传。返回 `url`、`thumb_url`、`blurhash` 等字段。

## 错误

```json
{"error":{"code":"bad_request","message":"..."}}
```

常见状态：`400` 参数/校验、`401` 未认证、`403` 作用域/CSRF、`404` 不存在或私密、`409` 冲突、`413` 过大、`429` 限流。

## CORS

只有 `WISP_CORS_ORIGINS` 白名单中的来源可跨域调用 `/api/v1`，且不带 Cookie；跨域请使用 PAT。
