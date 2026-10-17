-- +goose Up
-- P1：附件真实存储键 + PAT 令牌。

ALTER TABLE attachments ADD COLUMN storage_key TEXT NOT NULL DEFAULT '';

CREATE TABLE api_tokens (
    id           TEXT PRIMARY KEY, -- ULID
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE, -- sha256(wpat_...)，明文只在创建响应里出现一次
    scopes       TEXT NOT NULL DEFAULT 'read', -- read / write / read,write
    last_used_at INTEGER,
    created_at   INTEGER NOT NULL
);
