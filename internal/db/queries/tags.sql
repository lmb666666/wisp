-- name: UpsertTag :one
INSERT INTO tags (name) VALUES (?)
ON CONFLICT (name) DO UPDATE SET name = excluded.name
RETURNING *;

-- name: GetTagByName :one
SELECT * FROM tags WHERE name = ? LIMIT 1;

-- name: ListTags :many
SELECT t.id, t.name, count(pt.post_id) AS post_count
FROM tags t
LEFT JOIN post_tags pt ON pt.tag_id = t.id
GROUP BY t.id
HAVING count(pt.post_id) > 0
ORDER BY post_count DESC, t.name ASC;

-- name: ListPostTags :many
SELECT t.* FROM tags t
JOIN post_tags pt ON pt.tag_id = t.id
WHERE pt.post_id = ?
ORDER BY t.name ASC;

-- name: ClearPostTags :exec
DELETE FROM post_tags WHERE post_id = ?;

-- name: LinkPostTag :exec
INSERT INTO post_tags (post_id, tag_id) VALUES (?, ?) ON CONFLICT DO NOTHING;


-- name: GetTag :one
SELECT * FROM tags WHERE id = ? LIMIT 1;

-- name: CreateTag :one
INSERT INTO tags (name) VALUES (?) RETURNING *;

-- name: RenameTag :exec
UPDATE tags SET name = ? WHERE id = ?;

-- name: MergeTagPosts :exec
INSERT INTO post_tags (post_id, tag_id)
SELECT pt.post_id, sqlc.arg(target_id)
FROM post_tags pt WHERE pt.tag_id = sqlc.arg(source_id)
ON CONFLICT DO NOTHING;

-- name: DeleteTagLinks :exec
DELETE FROM post_tags WHERE tag_id = ?;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = ?;
