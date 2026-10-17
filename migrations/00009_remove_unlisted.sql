-- +goose Up
-- 「仅链接」已从产品移除；历史数据转为私密，避免半支持状态。
UPDATE posts SET visibility = 'private' WHERE visibility = 'unlisted';

-- +goose Down
-- 不可逆：无法区分原本的私密帖与由仅链接转换的帖子。
SELECT 1;
