-- +goose Up
-- 点赞（emoji reactions）：访客用浏览器匿名标识（X-Client-ID），单帖单表情每人一次。
CREATE TABLE reactions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id    TEXT    NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    client_id  TEXT    NOT NULL,
    emoji      TEXT    NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE (post_id, client_id, emoji)
);

CREATE INDEX idx_reactions_post ON reactions (post_id);

-- 全文搜索：FTS5 trigram（中文子串匹配），外部内容表跟随 posts.rowid
CREATE VIRTUAL TABLE posts_fts USING fts5(
    content_md,
    content = 'posts',
    content_rowid = 'rowid',
    tokenize = 'trigram'
);

-- +goose StatementBegin
CREATE TRIGGER posts_fts_ai AFTER INSERT ON posts BEGIN
    INSERT INTO posts_fts (rowid, content_md) VALUES (new.rowid, new.content_md);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER posts_fts_ad AFTER DELETE ON posts BEGIN
    INSERT INTO posts_fts (posts_fts, rowid, content_md)
    VALUES ('delete', old.rowid, old.content_md);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER posts_fts_au AFTER UPDATE OF content_md ON posts BEGIN
    INSERT INTO posts_fts (posts_fts, rowid, content_md)
    VALUES ('delete', old.rowid, old.content_md);
    INSERT INTO posts_fts (rowid, content_md) VALUES (new.rowid, new.content_md);
END;
-- +goose StatementEnd
