-- name: ListMoods :many
SELECT mood_date, mood FROM daily_moods
WHERE user_id = sqlc.arg('user_id')
  AND mood_date >= sqlc.arg('from_date')
  AND mood_date <= sqlc.arg('to_date')
ORDER BY mood_date ASC;

-- name: ListAllMoods :many
SELECT mood_date, mood FROM daily_moods
ORDER BY mood_date ASC;

-- name: LatestMood :one
SELECT mood_date, mood FROM daily_moods
WHERE user_id = sqlc.arg('user_id')
ORDER BY mood_date DESC
LIMIT 1;

-- name: UpsertMood :one
INSERT INTO daily_moods (user_id, mood_date, mood, created_at, updated_at)
VALUES (sqlc.arg('user_id'), sqlc.arg('mood_date'), sqlc.arg('mood'), sqlc.arg('created_at'), sqlc.arg('updated_at'))
ON CONFLICT (user_id, mood_date) DO UPDATE SET
    mood = excluded.mood,
    updated_at = excluded.updated_at
RETURNING *;

-- name: DeleteMood :execrows
DELETE FROM daily_moods
WHERE user_id = sqlc.arg('user_id') AND mood_date = sqlc.arg('mood_date');
