-- +goose Up
-- 扩展卡片（音乐 / GitHub / 位置）独立于正文存储：不写入 Markdown 内容。
ALTER TABLE posts ADD COLUMN blocks TEXT NOT NULL DEFAULT '[]';
