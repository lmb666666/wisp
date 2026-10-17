-- name: CreateAttachment :one
INSERT INTO attachments (
    id, kind, mime, size, width, height, blurhash,
    thumb_path, poster_path, storage, storage_key, remote_url, sha256, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAttachment :one
SELECT * FROM attachments WHERE id = ? LIMIT 1;

-- name: GetAttachmentBySHA256 :one
SELECT * FROM attachments WHERE sha256 = ? AND post_id IS NULL ORDER BY created_at DESC LIMIT 1;

-- name: AttachToPost :exec
UPDATE attachments SET post_id = ? WHERE id = ? AND post_id IS NULL;

-- name: ListPostAttachments :many
SELECT * FROM attachments WHERE post_id = ? ORDER BY created_at ASC, id ASC;

-- name: DeleteAttachment :exec
DELETE FROM attachments WHERE id = ?;

-- name: SetAttachmentPoster :exec
UPDATE attachments SET poster_path = ? WHERE id = ?;

-- name: ListMedia :many
SELECT a.id, a.kind, a.mime, a.size, a.width, a.height, a.thumb_path, a.storage_key,
       a.storage, a.remote_url, a.post_id, a.created_at, p.content_md AS post_content
FROM attachments a
LEFT JOIN posts p ON p.id = a.post_id
WHERE (sqlc.arg('kind') = '' OR a.kind = sqlc.arg('kind'))
  AND (sqlc.arg('storage') = '' OR a.storage = sqlc.arg('storage'))
  AND (sqlc.arg('orphan') = 0 OR a.post_id IS NULL)
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMedia :one
SELECT count(*) FROM attachments a
WHERE (sqlc.arg('kind') = '' OR a.kind = sqlc.arg('kind'))
  AND (sqlc.arg('storage') = '' OR a.storage = sqlc.arg('storage'))
  AND (sqlc.arg('orphan') = 0 OR a.post_id IS NULL);

-- name: MediaStats :many
SELECT kind, storage, count(*) AS count, coalesce(sum(size), 0) AS bytes
FROM attachments
GROUP BY kind, storage;

-- name: CountOrphanMedia :one
SELECT count(*) FROM attachments WHERE post_id IS NULL;
