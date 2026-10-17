-- +goose Up
-- Passkey（WebAuthn）凭证。
CREATE TABLE passkeys (
    id          TEXT PRIMARY KEY, -- credential ID（base64url）
    user_id     INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    public_key  BLOB    NOT NULL,
    attestation TEXT    NOT NULL DEFAULT '',
    transports  TEXT    NOT NULL DEFAULT '', -- 逗号分隔
    sign_count  INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL
);

CREATE INDEX idx_passkeys_user ON passkeys (user_id);
