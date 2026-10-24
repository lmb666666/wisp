# Contributing to Wisp

感谢你愿意一起把 Wisp 做得更好。这个项目刻意保持小：单作者、单进程、无评论、无多用户。提交大功能前请先开 Issue 对齐范围。

## 开发环境

需要：

- Go 1.26+
- Node.js 22+（CI 使用 24）
- pnpm 11
- SQLite 由 `modernc.org/sqlite` 内置，无需安装
- `sqlc`（修改 SQL 时）

```bash
pnpm --dir web install
make dev            # Go :8080 + Vite :5173
```

首次运行会在 `./data/` 创建数据库并执行迁移。Vite 会把 `/api` 与 `/healthz` 代理到 8080。

## 代码结构

```
Browser / PWA
   │  HTTP (REST + SPA)
   ▼
Echo v4 ── 会话 / PAT / CSRF / 限流 / CORS
   ├─ HTTP handlers ── service（渲染、标签、附件、卡片）
   │                      ├─ unfurl（链接解析 / 缓存）
   │                      ├─ media（Storage 驱动 / 上传派生）
   │                      └─ capsule / importer / backup
   ├─ sqlc Queries ── SQLite（WAL + FTS5 + 触发器）
   └─ Web（go:embed web/dist）
```

| 路径 | 职责 |
|---|---|
| `cmd/wisp` / `cmd/wisp-cli` | 服务入口；PAT 驱动的官方 CLI |
| `internal/http` | 路由、中间件、DTO 映射、OG/RSS/静态资源 |
| `internal/service` | 帖子领域逻辑：渲染、标签、附件、卡片解析 |
| `internal/db` / `internal/repo` | SQLite 打开、WAL、goose 迁移与快照；sqlc 生成代码（不要手改） |
| `internal/media` | 上传管线、缩略图、Storage 接口与 local / S3 / Lsky 驱动 |
| `internal/unfurl` | B 站 / YouTube / Meting / GitHub / OG 解析 |
| `internal/capsule` / `internal/importer` / `internal/backup` | 内容包导出；多来源导入；定时快照 |
| `internal/auth` | 密码、会话、PAT、Passkey |
| `web/src` | React SPA：routes、components、api、locales、styles |
| `migrations/` | 只增不改的 goose SQL 迁移 |

请求链路：Echo 中间件（恢复、安全头、压缩、CORS、限流）→ 写操作由 `requireUser`（会话）或 `requireWrite`（会话 / PAT write）保护 → Handler 绑定参数调用 `PostService` → sqlc 组装 DTO。Markdown 写入时由 goldmark 渲染 HTML，读取时对链接做 unfurl enrich（缓存表 `link_previews`）。前端用 TanStack Query 缓存，发布走乐观更新 + Idempotency-Key，离线队列存 IndexedDB。

安全基线（改动时不要破坏）：密码 argon2id；PAT 只存 SHA-256；Cookie 写请求校验 Origin；上传限制大小与 MIME、文件名/键由服务端生成；unfurl 对用户 URL 做 SSRF 防护；导入限制解压总量与路径穿越。

已知取舍：单进程单写者，不支持多实例共享同一个 SQLite；发帖/导入未整体包在一个 DB 事务内，极端失败可能留下可由媒体库清理的孤儿文件；切换存储驱动后旧驱动对象的删除只能尽力而为。

## 提交前检查

```bash
gofmt -l .                       # 应无输出
go test ./...
golangci-lint run ./...
pnpm --dir web lint
pnpm --dir web typecheck
pnpm --dir web test
pnpm --dir web build
```

涉及前端交互的改动，请跑 Playwright；涉及 API/导入/备份的改动，请跑验收矩阵：

```bash
# 终端 1：全新实例
WISP_PORT=18099 WISP_PUBLIC_URL=http://localhost:18099 \
WISP_DATA_DIR=/tmp/wisp-e2e WISP_API_RATE_LIMIT=1000 ./wisp

# 终端 2
WISP_URL=http://localhost:18099 pnpm --dir web exec playwright test

# 矩阵（默认 :18200，需要空库实例）
python3 scripts/full_matrix.py
```

`scripts/full_matrix.py` 会实际创建、编辑、导入、删除数据，请只对测试实例运行。

## 代码约定

- Go：`gofmt` + `golangci-lint`；SQL 走 sqlc，不手拼用户输入；错误要能定位上下文。
- 前端：TypeScript 严格模式、函数组件 + hooks、Tailwind + 现有 shadcn 组件；不要引入新的原生 `prompt/confirm`。
- 文案：中文与英文 `web/src/locales/{zh,en}.json` 同步更新。
- 迁移：新增迁移必须同时考虑回滚/升级演练；不要修改已发布的迁移文件。
- 提交信息：建议 Conventional Commits（`feat:`、`fix:`、`docs:`、`refactor:`、`test:`、`chore:`）。
- 隐私：测试与截图不要包含真实用户数据、令牌、密码或私有地址。

## 产品边界

以下能力是明确不做（不是“以后再说”）：

评论、MCP Server、Copilot/向量 RAG、TUI、实时日志控制台、多用户/角色权限、OAuth2/OIDC、Connect/Hub 联邦、多套主题、Tweet/X 卡片、原生 App、ActivityPub。

如果你认为边界需要调整，请先开 Issue 说明使用场景，而不是直接提交大改动。

## Pull Request

一个 PR 尽量只做一件事，并包含：

- 改动说明与用户可见影响
- 测试方式 / 新增测试
- 截图（UI 改动）
- 破坏性变更与迁移说明
- 文档同步

维护者可能为了范围或可维护性拒绝一些功能，请理解这是产品选择而非对贡献价值的否定。

## License

提交代码即表示同意以 [MIT](LICENSE) 许可证发布。
