-- +goose Up
-- Webhook 事件推送：post.created / post.updated / post.deleted（逗号分隔订阅）。
CREATE TABLE webhooks (
    id         TEXT PRIMARY KEY, -- ULID
    url        TEXT NOT NULL,
    secret     TEXT NOT NULL, -- HMAC-SHA256 签名密钥，创建时返回一次
    events     TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
