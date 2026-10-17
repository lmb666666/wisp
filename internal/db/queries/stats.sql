-- name: DailyCountsAuthor :many
SELECT date(published_at, 'unixepoch', 'localtime') AS day, count(*) AS count
FROM posts
WHERE published_at >= ? AND published_at < ?
GROUP BY day;

-- name: DailyCountsPublic :many
SELECT date(published_at, 'unixepoch', 'localtime') AS day, count(*) AS count
FROM posts
WHERE published_at >= ? AND published_at < ? AND visibility = 'public'
GROUP BY day;

-- name: OverviewPosts :one
SELECT count(*) AS total,
       count(CASE WHEN visibility = 'public' THEN 1 END) AS public,
       count(CASE WHEN visibility = 'private' THEN 1 END) AS private,
       count(CASE WHEN published_at >= sqlc.arg('today_start') AND published_at < sqlc.arg('today_end') THEN 1 END) AS today
FROM posts;

-- name: CountTags :one
SELECT count(*) FROM tags t
WHERE EXISTS (SELECT 1 FROM post_tags pt WHERE pt.tag_id = t.id);
