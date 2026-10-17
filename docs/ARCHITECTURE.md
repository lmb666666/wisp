# 架构

Wisp 是一个单进程、单作者的 Go + React 应用。目标不是横向扩展，而是：**一个二进制、一份 data 目录、可读、可迁移、升级可控**。

## 总览

```
Browser / PWA
   │  HTTP (REST + SPA)
   ▼
Echo v4 ── 会话 / PAT / CSRF / 限流 / CORS
   │
   ├─ HTTP handlers ── service（渲染、标签、附件、卡片）
   │                         │
   │                         ├─ unfurl（链接解析 / 缓存）
   │                         ├─ media（Storage 驱动 / 上传派生）
   │                         └─ capsule / importer / backup
   │
   ├─ sqlc Queries ── SQLite（WAL + FTS5 + 触发器）
   └─ Web（go:embed web/dist）
```

媒体默认写 `data/media/`，也可切换到 S3 兼容存储或 Lsky Pro；`Selector` 负责热切换与可选本地镜像。

## 目录

| 路径 | 职责 |
|---|---|
| `cmd/wisp` | 服务入口：配置、迁移、依赖装配、HTTP、优雅退出 |
| `cmd/wisp-cli` | PAT 驱动的官方 CLI |
| `internal/http` | 路由、中间件、DTO 映射、OG/RSS/静态资源 |
| `internal/service` | 帖子领域逻辑：渲染、标签、附件、卡片解析 |
| `internal/db` | SQLite 打开、WAL、goose 迁移与迁移前快照 |
| `internal/repo` | sqlc 生成代码（不要手改） |
| `internal/media` | 上传管线、缩略图、Storage 接口与驱动 |
| `internal/unfurl` | B 站 / YouTube / Meting / GitHub / OG 解析 |
| `internal/capsule` | 内容包导出 |
| `internal/importer` | Memos / Moments / Ech0 / Wisp 解析 |
| `internal/backup` | 定时快照、保留策略 |
| `internal/auth` | 密码、会话、PAT、Passkey |
| `web/src` | React SPA：routes、components、api、locales、styles |
| `migrations/` | 只增不改的 goose SQL 迁移 |

## 请求链路

1. Echo 中间件完成恢复、安全头、压缩、CORS、限流。
2. 公开读取可直接访问；写操作由 `requireUser`（会话）或 `requireWrite`（会话/PAT write）保护。
3. Handler 绑定参数并调用 `PostService`；错误统一为 `{"error":{"code","message"}}`。
4. Service 用 sqlc 查询组装 DTO：标签、附件 URL、reactions、扩展卡片。
5. Markdown 在写入时由 goldmark 渲染为 HTML；读取时对链接做 unfurl enrich（缓存表 `link_previews`）。
6. 前端用 TanStack Query 缓存；发布采用乐观更新 + Idempotency-Key；离线队列存 IndexedDB。

## 数据

- SQLite 单写连接 + WAL + `busy_timeout`；迁移由 goose 执行，非空库升级前 `VACUUM INTO` 快照。
- ID 使用 ULID，时间线游标为 `published_at.id`，同秒不丢帖。
- 标签：`tags` + `post_tags` 多对多，支持重命名/合并/只解绑。
- 扩展卡片以 JSON 存在 `posts.blocks`，展示字段（标题/封面/播放地址）读取时补齐，不信任客户端。
- FTS5 trigram 支持中文子串；短于 3 字符退回 LIKE。
- 可见性只有 `public` / `private`；游客只能读 public，作者可读全部。

## 认证与安全

- 密码 argon2id（PHC 格式）；会话存库、可吊销、HttpOnly + SameSite=Lax、HTTPS 下 Secure。
- PAT `wpat_` 前缀，只存 SHA-256；`read` / `read,write` 作用域；带 IP 限流。
- Cookie 写请求校验 Origin 与请求 Host / PublicURL；PAT 不受 CSRF 限制。
- 上传限制大小与 MIME；文件名/键由服务端生成；本地媒体只从 data 目录提供。
- unfurl 对用户输入 URL 做 SSRF 防护；Meting / Webhook 等管理员配置端点走单独的受信通道。
- 内容包导入限制 zip 解压总量、条目数、清单版本，并拒绝路径穿越。

## 媒体

`Storage` 接口：`Put` / `Get` / `Delete` / `PublicURL`。驱动：

- `local`：`data/media/`，默认
- `s3`：S3 兼容（AWS、MinIO、R2 等）
- `lsky`：Lsky Pro V2 上传

`Selector` 支持运行时切换；开启镜像时主存储成功后落一份本地副本。上传会生成缩略图与 blurhash；视频封面由浏览器截帧后上传，固定存本地。

## 内容包与备份

- Capsule：`wisp.yaml`（version 1）+ `posts/YYYY/MM/<ulid>.md` + `files/`；导出流式写 zip，导入按正文/blocks/附件 sha 去重并补挂缺失媒体。
- 备份：定时 `VACUUM INTO data/backups/scheduled-*.db`，默认 24h、保留 7 份；写临时文件后原子改名；启动检查、关闭等待后台任务。
- 媒体不在数据库快照内；完整归档使用 Capsule，或单独同步 `data/media/`。

## 前端

- TanStack Router 按路由拆分；页面切换动画只作用于叶子，避免设置页导航重挂载。
- 编辑器 TipTap + markdown；草稿 400ms 防抖写 IndexedDB；发布失败进离线队列。
- 主题用 CSS 变量 + `.dark` 类；无多主题系统。
- PWA 由 vite-plugin-pwa 生成；Service Worker 预缓存壳与静态资源，媒体 CacheFirst。

## 测试

- Go：`go test ./...`，包含 API 全链路、筛选、标签、媒体、内容包往返、备份时间注入、迁移升级。
- 前端：Vitest + Playwright（移动视口）。
- 验收矩阵：`scripts/full_matrix.py`（空库实例）。
- 压测：`go test -tags load -run TestLoad10k ./internal/load/`。

## 已知取舍

- 单进程单写者；不做多实例共享同一个 SQLite。
- 发帖/导入未整体包在一个 DB 事务内；极端失败可能留下可由媒体库清理的孤儿文件。
- 切换存储后，旧驱动的对象删除只能尽力而为；附件表不保存历史驱动配置。
- 评论、多用户、联邦、MCP 等明确不做，见 CONTRIBUTING 的产品边界。
