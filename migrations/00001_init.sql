-- +goose Up
-- Wisp 核心表。时间统一存 unix 秒；posts/attachments 主键使用 ULID 文本。
-- 回滚策略：恢复迁移前快照（见 internal/db.Migrate），因此不提供 Down 迁移。

CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    display_name  TEXT    NOT NULL DEFAULT '',
    is_owner      INTEGER NOT NULL DEFAULT 0 CHECK (is_owner IN (0, 1)),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);

CREATE TABLE sessions (
    id         TEXT    PRIMARY KEY, -- 登录 token 的 sha256 摘要，明文只存在于 Cookie
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    user_agent TEXT    NOT NULL DEFAULT '',
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE posts (
    id           TEXT    PRIMARY KEY, -- ULID
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    content_md   TEXT    NOT NULL DEFAULT '',
    content_html TEXT    NOT NULL DEFAULT '',
    visibility   TEXT    NOT NULL DEFAULT 'public'
        CHECK (visibility IN ('public', 'unlisted', 'private')),
    pinned_at    INTEGER,
    location     TEXT    NOT NULL DEFAULT '',
    published_at INTEGER NOT NULL, -- 允许补发过去的时刻
    edited_at    INTEGER,
    source       TEXT    NOT NULL DEFAULT 'web', -- web/pwa/share/api/cli/...
    client       TEXT    NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL
);

CREATE TABLE attachments (
    id          TEXT    PRIMARY KEY, -- ULID
    post_id     TEXT    REFERENCES posts (id) ON DELETE CASCADE,
    kind        TEXT    NOT NULL CHECK (kind IN ('image', 'video', 'audio', 'file')),
    mime        TEXT    NOT NULL,
    size        INTEGER NOT NULL DEFAULT 0,
    width       INTEGER NOT NULL DEFAULT 0,
    height      INTEGER NOT NULL DEFAULT 0,
    blurhash    TEXT    NOT NULL DEFAULT '',
    thumb_path  TEXT    NOT NULL DEFAULT '',
    poster_path TEXT    NOT NULL DEFAULT '', -- 视频封面（客户端截帧）
    storage     TEXT    NOT NULL DEFAULT 'local', -- local/s3/lsky/...
    remote_url  TEXT    NOT NULL DEFAULT '',
    sha256      TEXT    NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE TABLE tags (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE post_tags (
    post_id TEXT    NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    tag_id  INTEGER NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_id)
);

CREATE TABLE settings (
    key        TEXT    PRIMARY KEY,
    value      TEXT    NOT NULL,
    updated_at INTEGER NOT NULL
);

-- 链接卡片缓存（v0.3 内容块 unfurl 用，schema 先行）
CREATE TABLE link_previews (
    url        TEXT    PRIMARY KEY,
    provider   TEXT    NOT NULL DEFAULT '',
    title      TEXT    NOT NULL DEFAULT '',
    cover      TEXT    NOT NULL DEFAULT '',
    embed_html TEXT    NOT NULL DEFAULT '',
    fetched_at INTEGER NOT NULL DEFAULT 0
);
