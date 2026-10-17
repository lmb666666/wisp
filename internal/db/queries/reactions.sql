-- name: AddReaction :exec
INSERT INTO reactions (post_id, client_id, emoji, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (post_id, client_id, emoji) DO NOTHING;

-- name: RemoveReaction :exec
DELETE FROM reactions
WHERE post_id = ? AND client_id = ? AND emoji = ?;

-- name: ListPostReactions :many
SELECT emoji,
       count(*) AS count,
       SUM(CASE WHEN client_id = ? THEN 1 ELSE 0 END) AS mine
FROM reactions
WHERE post_id = ?
GROUP BY emoji
ORDER BY count DESC, emoji ASC;
