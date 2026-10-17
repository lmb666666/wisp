-- name: CreateWebhook :one
INSERT INTO webhooks (id, url, secret, events, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListWebhooks :many
SELECT * FROM webhooks ORDER BY created_at DESC;

-- name: ListWebhooksByEvent :many
SELECT * FROM webhooks WHERE instr(events, sqlc.arg('event')) > 0;

-- name: DeleteWebhook :exec
DELETE FROM webhooks WHERE id = ?;
