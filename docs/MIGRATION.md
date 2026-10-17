# 从旧应用迁移

入口：**设置 → 数据 → 导入旧数据**。导入结果会显示已导入、跳过（重复）、失败与附件数量。

| 来源 | 上传文件 | 内容 |
|---|---|---|
| Memos | Settings → Export 的 JSON | 正文、时间、可见性、位置 |
| Moments | 容器内 `/app/data/db.sqlite` | `Memo` 表：正文、时间、可见性、位置 |
| Ech0 | `ech0 export capsule --zip` | `echoes/**/*.md` 的 frontmatter 与正文；可含媒体 |
| Wisp | 内容包 zip（`wisp.yaml` version 1） | 正文、标签、位置、扩展卡片、原图/视频封面 |

## 步骤

1. 在旧应用导出数据；Wisp 侧先做一次数据库备份。
2. 设置 → 数据 → 导入旧数据，选择来源并上传文件。
3. 导入完成后检查：时间线、标签、附件、扩展卡片。
4. 重复导入是安全的：已存在的帖子会跳过，缺失附件会补挂。

## 可见性映射

| 来源值 | Wisp |
|---|---|
| Memos `PUBLIC` | public |
| Memos `PROTECTED` / `PRIVATE` | private |
| Moments `showType=1` | public |
| Moments `showType≠1` | private |
| Ech0 / Wisp capsule 的 `unlisted` | private（仅链接已移除） |

## 去重与附件

- 正文非空：按 `content_md` 精确匹配。
- 纯卡片帖：按清洗后的 `blocks` JSON 指纹。
- 无正文图集：按附件 SHA256 集合。
- 已存在帖子：逐文件比较 sha，补挂缺失附件；不会重复上传已有文件。
- 秒传只复用未绑定帖子的附件；同一文件用于多条帖子时会各存一份，删除互不影响。

## 限制

- Wisp capsule：zip ≤256MB；解压后总量 ≤256MB；单附件 ≤32MB；单篇正文 ≤4MB；
  条目 ≤20000；帖子 ≤5000 条；必须有 `wisp.yaml` 且版本为 1。
- Memos / Moments / Ech0：上传文件 ≤64MB，帖子 ≤5000 条；Ech0 之外的旧应用只迁移文本与元数据。
- Memos / Moments 的图片、音频等二进制文件不会自动迁移；Wisp 与 Ech0 capsule 可带媒体。
- 评论、点赞、阅读数不迁移。
- Moments 的 `music163Url` / `bilibiliUrl` 字段不会单独保留；如果原正文没有链接，需要手工补。

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
