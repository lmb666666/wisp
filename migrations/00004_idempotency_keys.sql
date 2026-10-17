-- +goose Up
-- 离线发送队列的幂等键：同一 Idempotency-Key 重试不会重复发帖。
CREATE TABLE idempotency_keys (
    key       TEXT PRIMARY KEY,
    post_id   TEXT    NOT NULL,
    created_at INTEGER NOT NULL
);
