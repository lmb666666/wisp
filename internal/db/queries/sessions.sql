-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, user_agent, expires_at, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetUserBySession :one
SELECT u.* FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id = ? AND s.expires_at > ?
LIMIT 1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = ?;

-- name: DeleteSessionsByUserKeep :exec
DELETE FROM sessions WHERE user_id = sqlc.arg(user_id) AND id != sqlc.arg(keep_id);

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < ?;
