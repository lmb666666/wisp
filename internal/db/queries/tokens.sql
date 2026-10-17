-- name: CreateToken :one
INSERT INTO api_tokens (id, name, token_hash, scopes, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTokenByHash :one
SELECT * FROM api_tokens WHERE token_hash = ? LIMIT 1;

-- name: ListTokens :many
SELECT id, name, scopes, last_used_at, created_at FROM api_tokens ORDER BY created_at DESC;

-- name: TouchToken :exec
UPDATE api_tokens SET last_used_at = ? WHERE id = ?;

-- name: DeleteToken :exec
DELETE FROM api_tokens WHERE id = ?;
