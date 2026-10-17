# Security Policy

## 支持的版本

Wisp 目前处于 `0.x` 阶段，安全修复只保证落在最新版本与 `main` 分支。旧版本请先升级再报告。

## 私下报告

请**不要**为安全漏洞开公开 Issue。优先使用 GitHub 仓库的：

**Security → Advisories → Report a vulnerability**

报告中请尽量包含：

- 受影响的版本 / commit
- 部署方式（二进制 / Docker / 反向代理）
- 复现步骤或最小 PoC
- 影响范围（读取、写入、拒绝服务、数据泄露等）
- 如果愿意，留下可联系的 GitHub 账号

维护者会尽量在 7 天内确认，并在修复可用后与你协调披露时间。这是个人项目，没有赏金计划。

## 重点关注面

- 认证与会话：首启初始化、argon2id、Session Cookie、Passkey
- PAT 作用域与幂等键
- CSRF / Origin 校验
- Markdown / 扩展卡片渲染的存储型 XSS
- 上传、媒体路径穿越、内容包 zip 解压
- unfurl 抓取的 SSRF 防护
- 导入器的资源上限与去重逻辑

## 部署者责任

Wisp 是单作者自托管软件，安全性也取决于部署方式：

- 生产环境必须使用 HTTPS，并设置正确的 `WISP_PUBLIC_URL`
- 使用强密码或 Passkey；不要复用其他站点密码
- 不要公开 `data/`、备份文件或 `.env`
- 保持 `WISP_ENABLE_SWAGGER=false`（除非确实需要在线调试）
- 定期升级；升级前保留自动备份，并确认 `data/backups/` 权限为私有
- 第三方服务（S3、Lsky、Meting、Nominatim、Webhook 目标）的安全与可用性由部署者负责

## 不在范围内

- 需要宿主/容器管理员权限才能利用的问题
- 错误配置的反向代理、公开的备份目录、弱密码
- 第三方依赖本身 0day（请同时上报上游；Wisp 会跟进升级）
- 单用户模型下“作者可以编辑自己的内容”这类预期行为
