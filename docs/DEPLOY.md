# 部署指南

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `WISP_PORT` | `8080` | HTTP 监听端口 |
| `WISP_DATA_DIR` | `./data` | 数据目录（数据库、附件、备份） |
| `WISP_DB_PATH` | `<data>/wisp.db` | 数据库文件路径 |
| `WISP_LOG_LEVEL` | `info` | debug / info / warn / error |
| `WISP_ENABLE_SWAGGER` | `false` | 暴露 OpenAPI 文档 |
| `WISP_PUBLIC_URL` | `http://localhost:8080` | 对外访问地址（RSS 链接 / OG 卡片用，**生产必填**） |
| `WISP_API_RATE_LIMIT` | `20` | API 限流（每秒请求数 / IP） |
| `WISP_AUTH_RATE_LIMIT` | `1` | 登录/首启限流（每秒请求数 / IP；Argon2 内存开销高） |
| `WISP_READ_TIMEOUT` | `10m` | 整个请求（含上传 body）的读取上限 |
| `WISP_WRITE_TIMEOUT` | `5m` | 响应写入上限 |
| `WISP_CORS_ORIGINS` | （空） | 允许跨域调用 API 的来源白名单（逗号分隔，仅 /api/v1，PAT 认证） |
| `WISP_TRUST_PROXIES` | `false` | 置 `true` 时从 `X-Forwarded-For` 取客户端 IP（限流用）；仅在可信反向代理后开启 |
| `WISP_METING_API` | （空） | Meting 音乐解析端点（自托管；配置后网易云/QQ 音乐链接自动出音乐卡） |
| `WISP_GEOCODE_URL` | （空） | Nominatim 兼容端点根地址（如 `https://nominatim.openstreetmap.org`）；配置后位置卡支持搜索与坐标反查，留空则只用地理定位/手填 |

## Docker Compose（推荐）

从源码构建：

```bash
mkdir -p data
WISP_UID="$(id -u)" WISP_GID="$(id -g)" \
WISP_PUBLIC_URL=https://wisp.example.com \
docker compose up -d --build
```

使用发布镜像：

```bash
mkdir -p data
WISP_UID="$(id -u)" WISP_GID="$(id -g)" WISP_PUBLIC_URL=https://wisp.example.com docker compose pull
WISP_UID="$(id -u)" WISP_GID="$(id -g)" WISP_PUBLIC_URL=https://wisp.example.com docker compose up -d
```

首次访问会进入首启向导；创建所有者后即可发布。数据、媒体与备份都在 `./data`。

### 容器安全默认

compose 默认以宿主 UID 运行，并启用：

- `read_only: true`：根文件系统只读；只有 `./data` 与 `/tmp` 内存盘可写
- `tmpfs: /tmp`：临时目录走内存盘
- `cap_drop: ALL` + `no-new-privileges:true`
- `stop_grace_period: 30s`：给优雅退出、Webhook 排空与备份收尾留时间

内容包导入的临时文件会落在数据目录，不占用 `/tmp` 内存盘。若自行修改 compose，请保留 `/data` 可写；需要导入 / 上传临时文件时也保留 `/tmp` 可写（或挂载 tmpfs）。

查看日志与状态：

```bash
docker compose logs -f wisp
docker compose ps
```

## 二进制

从 GitHub Releases 下载对应平台压缩包，解压后启动：

```bash
WISP_PUBLIC_URL=https://wisp.example.com \
WISP_DATA_DIR=/var/lib/wisp \
./wisp
```

systemd 示例（按实际路径调整）：

```ini
[Unit]
Description=Wisp
After=network.target

[Service]
User=wisp
Group=wisp
Environment=WISP_PUBLIC_URL=https://wisp.example.com
Environment=WISP_DATA_DIR=/var/lib/wisp
ExecStart=/usr/local/bin/wisp
Restart=on-failure
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

注意：`WISP_PUBLIC_URL` 必须是浏览器实际访问的地址（含协议），否则 RSS、OG 与分享链接会生成错误域名。

### 数据目录权限（绑定挂载必读）

镜像默认以非 root（uid **65532**）运行，compose 默认以宿主用户（`${WISP_UID:-1000}`）运行。
挂载的数据目录必须可被容器内用户写入，否则启动即报
`ping sqlite: unable to open database file (14)`。两种处理任选：

```bash
# 方式 A：数据目录交给镜像默认用户（65532）
sudo chown -R 65532:65532 ./data

# 方式 B（compose 默认）：容器以你的宿主 UID 运行
WISP_UID=$(id -u) WISP_GID=$(id -g) docker compose up -d
```

如果目录已被 root 容器意外创建（`ls -ldn data` 显示 `0:0`），可用一次性 root 容器修正，
无需宿主机 sudo：

```bash
docker run --rm -v "$(pwd)/data:/data" alpine:latest chown -R 1000:1000 /data
```

## 反向代理

> 反代部署请同时设置 `WISP_TRUST_PROXIES=true`，wisp 才会从 `X-Forwarded-For`
> 识别真实客户端 IP（限流按 IP 计数）；仅直连部署时保持默认 false。
>
> Passkey 需要 HTTPS + 域名（WebAuthn 不允许 IP 作为 RPID）；用 IP 访问时
> passkey 会自动禁用，密码登录不受影响。

### Caddy（自动 HTTPS，最省心）

```caddy
wisp.example.com {
    reverse_proxy localhost:8080
}
```

### Nginx

```nginx
server {
    listen 443 ssl http2;
    server_name wisp.example.com;

    # 覆盖 32MiB 附件 + multipart 开销，以及 256MB 内容包导入
    client_max_body_size 300m;
    client_body_timeout 600s;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}
```

## 备份与恢复

### 备份

**方式一：自动备份（默认开启）**——每 24 小时检查一次（启动时也检查），用 `VACUUM INTO` 生成
`data/backups/scheduled-<时间>.db`，默认保留最近 7 份。设置 → 数据 可改间隔/份数或立即备份；
失败只记日志，不影响站点。该快照只含数据库，不含 `data/media/`。

**方式二：Web 手动下载**——设置 → 数据 → 「下载备份」，得到当前一致性数据库 zip。
**注意：只含数据库，不含媒体原文件**。

**方式三：外部定时归档**——如需异地备份，可 crontab 调用 Web 备份接口（或直接同步 `data/backups/`）：

```bash
curl -b "wisp_session=<你的会话 Cookie>" \
     -o wisp-backup-$(date +%F).zip \
     https://wisp.example.com/api/v1/admin/backup
```

### 恢复

1. 停止 wisp（`docker compose down` 或停掉进程）
2. 解压备份 zip，得到 `wisp.db`
3. 用它替换数据目录中的 `wisp.db`，**并删除同目录的 `wisp.db-wal` / `wisp.db-shm`**（旧 WAL 会让新库读到脏页）
4. 重新启动。自动备份会在启动检查时继续按保留策略滚动。

升级导致的回滚同理：每次迁移前 wisp 会自动在 `data/backups/` 留快照，
回滚 = 停服 → 用快照覆盖 `wisp.db`（删 WAL/SHM）→ 退回旧版本二进制 → 启动。

## 升级

- Docker：拉取新镜像并 `docker compose up -d`。
- 二进制：替换 `wisp` 后重启。
- 启动时自动执行迁移；非空库升级前会在 `data/backups/` 生成 `pre-migrate-v*-*.db`。
- 先备份再升级；媒体文件不包含在数据库快照里。

## 健康检查

```bash
curl -fsS https://wisp.example.com/healthz
# {"status":"ok"}        # 进程存活

curl -fsS https://wisp.example.com/readyz
# {"status":"ready"}     # 数据库可读；库故障时返回 503
```
