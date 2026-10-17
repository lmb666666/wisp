-- +goose Up
-- 置顶功能已移除。
ALTER TABLE posts DROP COLUMN pinned_at;

-- +goose Down
ALTER TABLE posts ADD COLUMN pinned_at INTEGER;
