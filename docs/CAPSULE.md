# Wisp 内容包（Capsule）

> 目标：把「所有内容 + 原图 + 扩展卡片」装进一个 zip，可导入任意 Wisp 实例，也可长期归档。
> 这是数据库备份之外的「可携带」导出；数据库备份仍用于整站恢复。

## 导出

界面：**设置 → 数据 → 导出内容包**（可勾选是否包含私密内容）。

API：

```bash
curl -b "wisp_session=<会话 Cookie>" \
  -o wisp-capsule.zip \
  "https://wisp.example.com/api/v1/admin/export/capsule?include_private=1"
```

- 流式生成 zip，不占用大量内存。
- 文件名 `wisp-capsule-YYYYMMDD-HHMMSS.zip`。
- 外置存储（S3 / Lsky）优先通过配置的存储读取，失败再尝试 `remote_url`；仍失败的附件仍完成导出，
  以 `remote_url` 形式保留在清单里，warning 同时写入服务端日志和 `wisp.yaml.warnings`。
- 单个附件导出上限 32MB；超限不会静默截断，而是记录 warning 并保留 `remote_url`。
- 视频封面固定存本地，会一并打包；缩略图不导出（导入时重新生成）。

## 格式（version 1）

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
include_private: true
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

- 只导出 Markdown 源文，不导出 HTML；导入时按当前渲染器重新渲染（避免版本差异）。
- `blocks` 只包含可重建字段（type/url/name/lat/lng），播放地址等展示信息由服务端重新解析。
- 附件若无法打包，会带 `remote_url` 而不是 `file`。

## 导入

界面：**设置 → 数据 → 导入旧数据**，来源选 **Wisp**，上传 zip。

API：

```bash
curl -b "wisp_session=<Cookie>" \
  -F "source=wisp" \
  -F "file=@wisp-capsule.zip" \
  "https://wisp.example.com/api/v1/admin/import"
```

- 媒体按 SHA256 秒传，但只复用**尚未绑定帖子**的附件；同一文件用于多条帖子时各存一份，
  避免删除其中一帖时相互影响。
- 去重：正文非空按正文精确匹配；纯卡片帖按清洗后的 blocks 指纹匹配；无正文/无卡片的图集按附件
  SHA256 集合匹配。重复导入安全，已存在帖子的缺失附件会补挂。
- 视频封面会写入本地存储并回填（远端附件同样按附件 ID 存本地封面）。
- 格式校验：必须包含 `wisp.yaml` 且 `version: 1`；不兼容版本直接拒绝。
- 单次限制：zip ≤256MB；解压后总量 ≤256MB、单附件 ≤32MB、单篇正文 ≤4MB、条目 ≤20000、
  帖子 ≤5000 条；超出请拆分导出。
- 外链附件（未打包媒体）会以 `remote_url` 建一条远程附件记录，不占本地空间。

## 往返验证

`TestCapsuleRoundTrip` 覆盖：带标签/位置/私密/图片/音乐块/位置卡的帖子 → 导出 → 空实例导入 → 字段与媒体一致 → 再次导入全部去重。

## 安全

- 导出仅限已登录会话；是否含私密内容由导出者选择。
- zip 条目路径拒绝 `..`、绝对路径与反斜杠（防 zip-slip）。
- 导入不执行 Markdown 中的原始 HTML（渲染器默认关闭 raw HTML）。
