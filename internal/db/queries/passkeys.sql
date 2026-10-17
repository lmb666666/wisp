-- name: CreatePasskey :exec
INSERT INTO passkeys (id, user_id, public_key, attestation, transports, sign_count, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetPasskey :one
SELECT * FROM passkeys WHERE id = ? LIMIT 1;

-- name: ListPasskeysByUser :many
SELECT * FROM passkeys WHERE user_id = ? ORDER BY created_at DESC;

-- name: ListPasskeys :many
SELECT * FROM passkeys;

-- name: UpdatePasskeySignCount :exec
UPDATE passkeys SET sign_count = ? WHERE id = ?;

-- name: DeletePasskey :exec
DELETE FROM passkeys WHERE id = ?;

-- name: CountPasskeysByUser :one
SELECT count(*) FROM passkeys WHERE user_id = ?;
