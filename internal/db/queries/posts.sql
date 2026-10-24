-- name: CreatePost :one
INSERT INTO posts (
    id, user_id, content_md, content_html, visibility,
    location, published_at, source, client, blocks, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetPost :one
SELECT * FROM posts WHERE id = ? LIMIT 1;

-- name: DeletePost :exec
DELETE FROM posts WHERE id = ?;

-- name: UpdatePost :one
UPDATE posts SET
    content_md   = COALESCE(sqlc.narg('content_md'), content_md),
    content_html = COALESCE(sqlc.narg('content_html'), content_html),
    visibility   = COALESCE(sqlc.narg('visibility'), visibility),
    location     = COALESCE(sqlc.narg('location'), location),
    blocks       = COALESCE(sqlc.narg('blocks'), blocks),
    published_at = COALESCE(sqlc.narg('published_at'), published_at),
    edited_at    = COALESCE(sqlc.narg('edited_at'), edited_at)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ListPosts :many
SELECT * FROM posts
WHERE (published_at < sqlc.arg('before')
       OR (published_at = sqlc.arg('before') AND id < sqlc.arg('before_id')))
ORDER BY published_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: CountPosts :one
SELECT count(*) FROM posts;

-- name: ListPostContentMDs :many
SELECT content_md FROM posts;

-- name: GetPostIDByContent :one
SELECT id FROM posts WHERE content_md = ? ORDER BY created_at DESC LIMIT 1;

-- name: ListPostContentForRerender :many
SELECT id, content_md FROM posts;

-- name: UpdatePostHTML :exec
UPDATE posts SET content_html = ? WHERE id = ?;

-- name: ListPostsForExport :many
SELECT * FROM posts ORDER BY published_at ASC, id ASC;

-- name: ListPostFingerprints :many
SELECT id, content_md, blocks FROM posts;

-- name: ListAttachmentOnlyFingerprints :many
SELECT a.post_id, a.sha256
FROM attachments a
JOIN posts p ON p.id = a.post_id
WHERE p.content_md = '' AND (p.blocks = '' OR p.blocks = '[]' OR p.blocks = 'null')
ORDER BY a.post_id, a.created_at ASC, a.id ASC;

-- name: ListFilteredPosts :many
SELECT p.* FROM posts p
WHERE (sqlc.arg('from_ts') = 0 OR p.published_at >= sqlc.arg('from_ts'))
  AND (sqlc.arg('to_ts') = 0 OR p.published_at <= sqlc.arg('to_ts'))
  AND (sqlc.arg('tag_count') = 0 OR (
      SELECT count(DISTINCT t.name) FROM post_tags pt
      JOIN tags t ON t.id = pt.tag_id
      WHERE pt.post_id = p.id
        AND t.name IN (SELECT value FROM json_each(sqlc.arg('tags_json')))
  ) = sqlc.arg('tag_count'))
  AND (p.published_at < sqlc.arg('before')
       OR (p.published_at = sqlc.arg('before') AND p.id < sqlc.arg('before_id')))
ORDER BY p.published_at DESC, p.id DESC
LIMIT sqlc.arg('limit');
