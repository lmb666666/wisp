-- +goose Up
-- 每日心情：一天一条，日期为作者本地日期（YYYY-MM-DD），写入时 upsert 覆盖。
CREATE TABLE daily_moods (
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    mood_date  TEXT    NOT NULL,
    mood       TEXT    NOT NULL CHECK (mood IN ('happy', 'calm', 'okay', 'tired', 'low')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (user_id, mood_date)
);

CREATE INDEX idx_daily_moods_date ON daily_moods (mood_date);
