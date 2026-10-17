# Wisp v0.2.0

首个公开版本。Wisp 是一个单作者、自托管的轻量「说说 / 朋友圈」：一个 Go 二进制、一份 data 目录，移动端优先，数据完全自持。

## 亮点

- **写作**：TipTap Markdown、图片压缩上传、草稿、离线队列、PWA 系统分享直达发布
- **时间线**：按日分组、九宫格、blurhash、lightbox、点赞、全文搜索、组合筛选
- **扩展卡片**：音乐、GitHub、网站、视频、位置；视频/地图点击后才加载
- **媒体**：本地 / S3 / Lsky 存储、缩略图、秒传、媒体库与无主文件清理
- **数据主权**：Wisp Capsule 导出/导入、自动数据库快照、迁移前快照、标签管理
- **开放**：REST API、作用域 PAT、OpenAPI、RSS、Webhook、官方 CLI
- **部署**：单二进制、distroless Docker、多平台 release、iOS/Android PWA

## 升级 / 迁移注意

- 这是**首个公开版本**，没有更早的公开版本需要迁移。
- 数据库迁移会从 v10 开始；从开发快照升级时，启动前会自动生成 `data/backups/pre-migrate-*.db`。
- 历史 `unlisted`（仅链接）数据会迁移为私密；`pinned_at` 列会被删除。
- 可见性只有公开 / 私密；发布框不再提供仅链接，编辑页也不提供置顶。
- 媒体不在数据库快照内，完整归档请使用 Wisp Capsule 或单独同步 `data/media/`。

## 安装

```bash
git clone https://github.com/lmb666666/wisp.git
cd wisp
WISP_PUBLIC_URL=https://wisp.example.com docker compose up -d --build
```

或从 Releases 下载对应平台的 `wisp` / `wisp-cli`。首次访问按向导创建站点所有者。

## 已知边界

- 单作者；没有评论、多用户、角色权限、联邦、MCP、原生 App
- 不支持多实例共享同一个 SQLite
- 切换存储驱动后，旧驱动对象的清理只能尽力而为

## 验证

- Go 测试全绿；golangci-lint 0 issue
- Playwright E2E 16/16
- 全功能验收矩阵 76/76
- 10k 帖组合筛选 < 5ms（本地）

完整变更见 [CHANGELOG](../CHANGELOG.md)。
