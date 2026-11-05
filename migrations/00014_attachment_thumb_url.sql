-- +goose Up
-- 远端存储的缩略图绝对 URL（S3 / Lsky）。本地驱动为空，继续走 /media/<thumb_path>。
ALTER TABLE attachments ADD COLUMN thumb_remote_url TEXT NOT NULL DEFAULT '';
