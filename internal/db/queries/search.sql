-- name: SearchPublicPosts :many
SELECT p.* FROM posts p
JOIN posts_fts f ON f.rowid = p.rowid
WHERE f.content_md MATCH ?
  AND p.visibility = 'public'
  AND p.published_at < ?
ORDER BY p.published_at DESC
LIMIT ?;

-- name: SearchAuthorPosts :many
SELECT p.* FROM posts p
JOIN posts_fts f ON f.rowid = p.rowid
WHERE f.content_md MATCH ?
  AND p.published_at < ?
ORDER BY p.published_at DESC
LIMIT ?;

-- name: SearchPublicPostsLike :many
SELECT * FROM posts
WHERE content_md LIKE ('%' || ? || '%')
  AND visibility = 'public'
  AND published_at < ?
ORDER BY published_at DESC
LIMIT ?;

-- name: SearchAuthorPostsLike :many
SELECT * FROM posts
WHERE content_md LIKE ('%' || ? || '%')
  AND published_at < ?
ORDER BY published_at DESC
LIMIT ?;
