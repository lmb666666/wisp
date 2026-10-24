-- +goose Up
-- 站点所有者唯一：并发 /auth/init 不可能产生第二个 owner（根治 TOCTOU）。
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_single_owner
ON users ((1)) WHERE is_owner = 1;

-- +goose Down
DROP INDEX IF EXISTS idx_users_single_owner;
