# Changelog

## [0.3.0] - 2026-10-24

### 移除

- **点赞功能整体移除**：reactions 表（迁移 00012）、API 端点、前端表情栏与点赞通知（ntfy/Bark/SMTP 通知渠道随唯一事件一并下线，Webhook 不受影响）
- **私密发布移除**：可见性只保留 public（迁移 00013 把历史私密帖转为公开）；`visibility=private` 请求返回 400；内容包导出不再有 include_private 选项

### 安全修复

- `/media/*` 目录请求返回 404：不再向匿名访客枚举附件文件名
- 修复前端乐观更新未转义注入 HTML 的 XSS（可经 `/share` 远程触发）
- 限流按真实客户端 IP 计数（新增 `WISP_TRUST_PROXIES`，默认信任 TCP 对端，杜绝 X-Forwarded-For 伪造绕过）
- 修改密码后吊销该用户其余全部会话（当前设备保持登录）
- 发帖/编辑改为单事务（标签重建、附件绑定原子化，中途失败不再丢标签/产生半成品）
- 图片上传先校验像素上限（64MP）再解码，防 decompression bomb OOM
- Ech0 导入的 Markdown 条目加 4MB 解压上限，防 zip bomb
- 内容包导出对 `remote_url` 的抓取与 unfurl 共用 SSRF 防护（拒绝私网/环回地址）
- unfurl 抓取改为「校验后直连已校验 IP」，消除 DNS rebinding 窗口
- JSON 端点请求体限 4MB、上传限 40MB（防匿名大包内存/磁盘滥用）
- 私密帖不再可被点赞（此前知道 ULID 的访客可对私密帖互动）
- 非内联扩展名的媒体响应强制 `attachment` 下载头与 `application/octet-stream`，杜绝 `.bin` 内容嗅探成 HTML 的同源 XSS 载体
- 迁移 00011：owner 部分唯一索引，并发首启不可能产生第二个所有者
- 登录对不存在用户名执行等价哈希校验，抹平时序差（防用户名枚举）
- Passkey 未完成仪式加 5 分钟 TTL 与容量回收；注册完成回调补 nil 检查
- 内部错误不再把 `err.Error()`（含 SQL 细节）返回给客户端；DB 故障不再误报 401 触发前端登出

### 修复与改进

- 链接卡片 href 双重转义修复（带查询参数的链接卡跳转地址正确）
- 纯标签行移除跳过代码块（围栏与缩进），`#注释` 类代码行不再被误删
- 短词搜索改用 `instr()` 字面子串匹配（搜 `100%` 不再当通配符）
- 发布/编辑无字段变化时不再触碰 `edited_at`；`published_at` 拒绝非正数；显式标签上限 16 个/64 字符
- 链接预取并发上限 4（导入大批量时不再产生数千 goroutine）
- 存储热切换先持久化再切换（DB 写失败不漂移）；媒体删除先删行后删对象
- 幂等键：长度上限 128；重放已删除帖子返回 409 而不是静默新建；插入失败记日志
- 重复点赞不再重复触发通知（仅新点赞通知）
- Webhook 投递失败按 2s/8s/30s 退避重试；进程退出前排空队列（Stop/Wait）
- 优雅退出：Shutdown 上限 15s，第二次信号立即强退；端口绑定成功后才输出 listening 日志
- `./wisp` 拒绝未知命令行参数（此前被静默忽略并直接启动服务）
- Meting 设置清空后回退环境变量；通知地址校验 http(s) scheme；Lsky base_url 校验收紧
- 迁移前快照收紧为 0600；RSS 时间改 RFC1123Z；SPA index 加 no-cache
- 前端：离线队列 401/403 保留等待重登（不再静默丢帖）；入队失败保留内容；设置页加载失败有错误态；`/share` 与旧草稿合并不覆盖；登出清理 SW 媒体缓存；点赞按钮防连发；Passkey 注册后免刷新；时间字段清空不再提交 NaN
- i18n 补齐导入页文案；移除未使用组件与不可达分支

### 维护

- 升级全部 Go 与前端依赖：Echo 4.16.2、minio-go 7.4.1、goose 3.29、modernc SQLite 1.61、x/crypto 0.58、React 19.3.1、TanStack Query/Router、TipTap 3.32、zod 4.6.7、lucide-react 1.61 等
- GitHub Actions 升至当前最新大版本：checkout v7、setup-go/setup-node v7、pnpm/action-setup v6、golangci-lint-action v9、goreleaser-action v7、Docker 系列 v4/v7
- 移除误提交的手动探针脚本 `web/mtest2.mjs`
- 精简公开文档：品牌资料（BRAND）与未使用的 Logo 源素材移出仓库；发布说明与 docs 索引删除；CAPSULE 并入 MIGRATION；架构要点并入 CONTRIBUTING

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
