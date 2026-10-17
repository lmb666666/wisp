-- name: CreateUser :one
INSERT INTO users (username, password_hash, display_name, is_owner, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ? LIMIT 1;

-- name: GetUser :one
SELECT * FROM users WHERE id = ? LIMIT 1;

-- name: GetOwnerUser :one
SELECT * FROM users WHERE is_owner = 1 ORDER BY id LIMIT 1;

-- name: UpdateUserDisplayName :one
UPDATE users SET display_name = ?, updated_at = ? WHERE id = ? RETURNING *;

-- name: UpdateUserPasswordHash :exec
UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?;
