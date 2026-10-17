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
