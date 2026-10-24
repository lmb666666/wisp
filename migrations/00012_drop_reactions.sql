-- +goose Up
-- 移除点赞功能：表情互动表整体退役（v0.3 起站点不再有点赞交互）。
DROP TABLE IF EXISTS reactions;

-- +goose Down
-- 不可逆：点赞数据不恢复（功能已移除）。
SELECT 1;
