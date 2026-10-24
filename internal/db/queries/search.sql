-- name: SearchPosts :many
SELECT p.* FROM posts p
JOIN posts_fts f ON f.rowid = p.rowid
WHERE f.content_md MATCH ?
  AND p.published_at < ?
ORDER BY p.published_at DESC
LIMIT ?;

-- name: SearchPostsBySubstring :many
SELECT * FROM posts
WHERE instr(content_md, ?) > 0
  AND published_at < ?
ORDER BY published_at DESC
LIMIT ?;
