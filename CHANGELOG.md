# Changelog

## [Unreleased]

## [0.2.0] - 2026-10-17

首个公开版本。单作者、自托管、单二进制的轻量「说说 / 朋友圈」。

### 写作与浏览

- TipTap Markdown 编辑器；粘贴/拖拽图片自动压缩上传；视频封面客户端截帧
- 常驻发布框、IndexedDB 草稿、乐观发送；离线自动进队列，联网后按幂等键补发
- 时间线无限滚动、按日分组、朋友圈式图片网格、blurhash 占位、lightbox
- 点赞；标签聚合；FTS5 全文搜索；日期 × 可见性 × 多标签组合筛选（URL 保持、⌘K）
- 浅色 / 深色主题；中英文界面；Passkey 登录

### 扩展卡片

- 音乐、GitHub、网站、视频、位置卡片；统一 BlockShell
- 网站 / GitHub / 视频标题与封面异步抓取并缓存
- 位置卡支持坐标、OSM 地图（点击加载）与可选 Nominatim 搜索/反查
- 视频支持 B 站 BV 与 YouTube，点击封面后才挂载播放器

### 媒体与存储

- 图片 / 视频 / 音频附件；服务端缩略图、blurhash、SHA256 秒传（仅未绑定附件）
- 可插拔 Storage：本地 / S3 兼容 / Lsky Pro；运行时切换与可选本地镜像
- 媒体库：类型 / 存储 / 无主筛选、占用统计、安全删除；视频封面按附件 ID 存储

### 数据与运维

- Wisp Capsule 内容包导出/导入（Markdown + 原图 + 卡片）；清单版本校验与解压上限
- 自动数据库备份：默认 24h、保留 7 份，原子写入、启动检查、关闭等待
- 发帖/编辑/删除前校验附件归属；空正文/位置可清空；正文编辑保留 frontmatter 标签
- 标签重命名、事务合并、只解绑删除；零引用标签不出现在聚合中
- 站点概览：帖子 / 今日 / 媒体 / 标签 / DB / 上次备份 / 版本 / 运行时长
- 从 Memos / Moments / Ech0 / Wisp 导入；图集按附件 sha 去重、缺失媒体补挂

### API 与交付

- REST API（`/api/v1`）、作用域 PAT、Idempotency-Key、IP 限流、OpenAPI / Swagger
- RSS 2.0 全量与按标签；详情页服务端注入 OG meta；默认社交预览图
- 官方 `wisp-cli`
- `go:embed` 单二进制、distroless Docker、goreleaser 多平台、GitHub Actions
- PWA 可安装、离线壳、媒体缓存、系统分享 Target、iOS 主屏图标与 maskable 图标

### 变更与移除

- 可见性只保留公开 / 私密；历史 `unlisted` 迁移为私密（迁移 00009）
- 移除置顶功能，删除 `pinned_at`（迁移 00010）
- 移除与网站卡片重复的行内链接按钮；Markdown 链接仍可用
- 修复发布后草稿回填、`/share` 覆盖草稿、媒体分页、私密缓存登出残留等问题
- 管理端点恢复 CSRF Origin 校验；修复生产环境 `/media/*` 未挂载 LocalStore 的问题

### 安全与验证

- argon2id、可吊销会话、PAT 作用域、CORS 白名单、SSRF 防护、上传限制、zip 解压上限
- Go 测试、golangci-lint、Playwright E2E、76 项验收矩阵与 10k 帖压测
