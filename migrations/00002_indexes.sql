-- +goose Up
-- 常用查询索引。IF NOT EXISTS 保证重放幂等。

CREATE INDEX IF NOT EXISTS idx_posts_published ON posts (published_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_visibility ON posts (visibility);
CREATE INDEX IF NOT EXISTS idx_attachments_post ON attachments (post_id);
CREATE INDEX IF NOT EXISTS idx_attachments_sha256 ON attachments (sha256);
CREATE INDEX IF NOT EXISTS idx_post_tags_tag ON post_tags (tag_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions (user_id);
