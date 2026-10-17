-- name: GetLinkPreview :one
SELECT * FROM link_previews WHERE url = ? LIMIT 1;

-- name: UpsertLinkPreview :exec
INSERT INTO link_previews (url, provider, title, cover, embed_html, fetched_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (url) DO UPDATE SET
    provider = excluded.provider,
    title = excluded.title,
    cover = excluded.cover,
    embed_html = excluded.embed_html,
    fetched_at = excluded.fetched_at;
