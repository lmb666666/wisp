# 迁移与内容包

## 从旧应用迁移

入口：**设置 → 数据 → 导入旧数据**。导入结果会显示已导入、跳过（重复）、失败与附件数量。

| 来源 | 上传文件 | 内容 |
|---|---|---|
| Memos | Settings → Export 的 JSON | 正文、时间、可见性、位置 |
| Moments | 容器内 `/app/data/db.sqlite` | `Memo` 表：正文、时间、可见性、位置 |
| Ech0 | `ech0 export capsule --zip` | `echoes/**/*.md` 的 frontmatter 与正文；可含媒体 |
| Wisp | 内容包 zip（`wisp.yaml` version 1） | 正文、标签、位置、扩展卡片、原图/视频封面 |

步骤：旧应用导出 → Wisp 侧先做一次数据库备份 → 上传导入 → 检查时间线 / 标签 / 附件 / 扩展卡片。重复导入是安全的。

## 可见性映射

| 来源值 | Wisp |
|---|---|
| Memos `PUBLIC` | public |
| 所有来源（Memos/Moments/Ech0/Wisp） | public（私密发布已移除，历史内容一律公开） |

## 去重与附件

- 正文非空：按 `content_md` 精确匹配；纯卡片帖：按清洗后的 `blocks` JSON 指纹；无正文图集：按附件 SHA256 集合。
- 已存在帖子：逐文件比较 sha，补挂缺失附件，不重复上传。
- 秒传只复用未绑定帖子的附件；同一文件用于多条帖子时会各存一份，删除互不影响。
- Memos / Moments 的图片等二进制文件不会自动迁移；Wisp 与 Ech0 capsule 可带媒体。评论、点赞、阅读数不迁移。
- Moments 的 `music163Url` / `bilibiliUrl` 字段不单独保留；原正文没有链接时需手工补。

## 限制

- Wisp capsule：zip ≤256MB；解压后总量 ≤256MB；单附件 ≤32MB；单篇正文 ≤4MB；条目 ≤20000；帖子 ≤5000 条；必须有 `wisp.yaml` 且版本为 1。
- Memos / Moments / Ech0：上传文件 ≤64MB，帖子 ≤5000 条；Ech0 之外的旧应用只迁移文本与元数据。

## Wisp 内容包（Capsule）

把「所有内容 + 原图 + 扩展卡片」装进一个 zip，可导入任意 Wisp 实例，也可长期归档。这是数据库备份之外的「可携带」导出；整站恢复仍用数据库备份。

### 导出

界面：**设置 → 数据 → 导出内容包**。

```bash
curl -b "wisp_session=<会话 Cookie>" \
  -o wisp-capsule.zip \
  "https://wisp.example.com/api/v1/admin/export/capsule"
```

- 流式生成 zip，不占用大量内存；文件名 `wisp-capsule-YYYYMMDD-HHMMSS.zip`。
- 外置存储（S3 / Lsky）优先从配置存储读取，失败再试 `remote_url`；仍失败的附件以 `remote_url` 形式保留在清单里，warning 写入日志与 `wisp.yaml.warnings`。
- 单附件导出上限 32MB，超限记录 warning 并保留 `remote_url`，不静默截断。
- 视频封面固定存本地，一并打包；缩略图不导出（导入时重新生成）。
- 导出仅限已登录会话；内容包包含全部帖子（可见性概念已移除）。

### 格式（version 1）

```
wisp-capsule/
  wisp.yaml
  posts/2026/10/<ulid>.md
  files/2026/10/<ulid>.png
```

`wisp.yaml`：

```yaml
version: 1
exported_at: 1792229851
source_url: https://wisp.example.com
stats: {posts: 128, attachments: 45, media_bytes: 12345678}
warnings: []
```

每条帖子是一个 Markdown 文件，frontmatter 示例：

```yaml
---
id: 01M4N0RHF5AY1X9T12RNKR72FV
published_at: 1792229851
visibility: public
tags: [设计, 日记]
location: 上海 · 徐汇
blocks:
  - {type: music, url: https://music.163.com/song?id=33894312}
  - {type: location, name: 上海 · 徐汇, lat: 31.18, lng: 121.43}
attachments:
  - {file: files/2026/10/01M....png, kind: image, mime: image/png, width: 800, height: 600, blurhash: L...}
---

正文 Markdown。
```

- 只导出 Markdown 源文，不导出 HTML；导入时按当前渲染器重新渲染。
- `blocks` 只包含可重建字段（type/url/name/lat/lng），播放地址等展示信息由服务端重新解析。
- 无法打包的附件带 `remote_url` 而不是 `file`。

### 导入

界面：**设置 → 数据 → 导入旧数据**，来源选 **Wisp**，上传 zip。API：`POST /api/v1/admin/import`（multipart `source=wisp` + `file`）。

- 格式校验：必须包含 `wisp.yaml` 且 `version: 1`，不兼容版本直接拒绝。
- 视频封面写入本地存储并按附件 ID 回填；外链附件（未打包媒体）以 `remote_url` 建远程附件记录，不占本地空间。
- zip 条目路径拒绝 `..`、绝对路径与反斜杠；导入不执行正文中的原始 HTML。
- 去重与上限见上文「去重与附件」「限制」。

## JSON 兜底

任何来源都可以先整理为如下结构，再以 `source=memos` 上传（数组或 `{"memos":[...]}` 包裹）：

```json
[
  {
    "content": "Markdown 正文 #迁移",
    "createdTs": 1754128800,
    "visibility": "PUBLIC"
  }
]
```
