-- +goose Up
-- 移除私密发布：单作者站点全部内容公开。历史私密帖转为公开（同 00009 的 unlisted 先例）。
-- visibility 列保留（CHECK 允许 public/unlisted/private），应用层不再写入非 public 值。
UPDATE posts SET visibility = 'public' WHERE visibility != 'public';

-- +goose Down
-- 不可逆：私密状态无法还原。
SELECT 1;
