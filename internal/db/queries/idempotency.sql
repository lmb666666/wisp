-- name: GetIdempotentPostID :one
SELECT post_id FROM idempotency_keys WHERE key = ? LIMIT 1;

-- name: InsertIdempotencyKey :exec
INSERT INTO idempotency_keys (key, post_id, created_at) VALUES (?, ?, ?)
ON CONFLICT (key) DO NOTHING;

-- name: DeleteOldIdempotencyKeys :exec
DELETE FROM idempotency_keys WHERE created_at < ?;
