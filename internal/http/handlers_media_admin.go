package http

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/service"
)

// listMedia GET /admin/media?kind=&orphan=1&limit=&offset=
func (s *Server) listMedia(c echo.Context) error {
	kind := strings.TrimSpace(c.QueryParam("kind"))
	if kind != "" && kind != "image" && kind != "video" && kind != "audio" && kind != "file" {
		return apiError(c, http.StatusBadRequest, "bad_request", "非法类型")
	}
	storage := strings.TrimSpace(c.QueryParam("storage"))
	orphan := int64(0)
	if c.QueryParam("orphan") == "1" {
		orphan = 1
	}
	limit, _ := strconv.ParseInt(c.QueryParam("limit"), 10, 64)
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset, _ := strconv.ParseInt(c.QueryParam("offset"), 10, 64)
	if offset < 0 {
		offset = 0
	}

	rows, err := s.q.ListMedia(c.Request().Context(), repo.ListMediaParams{
		Kind: any(kind), Storage: any(storage), Orphan: any(orphan), Limit: limit, Offset: offset,
	})
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	total, err := s.q.CountMedia(c.Request().Context(), repo.CountMediaParams{
		Kind: any(kind), Storage: any(storage), Orphan: any(orphan),
	})
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}

	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		url := r.RemoteUrl
		if url == "" {
			url = "/media/" + r.StorageKey
		}
		thumb := ""
		if r.ThumbPath != "" {
			thumb = url
			if r.RemoteUrl == "" {
				thumb = "/media/" + r.ThumbPath
			} else if r.Storage == "s3" {
				thumb = r.RemoteUrl + ".thumb.jpg"
			}
		}
		item := map[string]any{
			"id": r.ID, "kind": r.Kind, "mime": r.Mime, "size": r.Size,
			"width": r.Width, "height": r.Height, "url": url, "thumb_url": thumb,
			"storage": r.Storage, "created_at": r.CreatedAt, "post_id": nil,
			"post_excerpt": "",
		}
		if r.PostID.Valid {
			item["post_id"] = r.PostID.String
			if r.PostContent.Valid {
				item["post_excerpt"] = service.Excerpt(r.PostContent.String, 60)
			}
		}
		items = append(items, item)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": total})
}

// mediaStats GET /admin/media/stats
func (s *Server) mediaStats(c echo.Context) error {
	rows, err := s.q.MediaStats(c.Request().Context())
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	byKind := map[string]map[string]any{}
	var totalBytes int64
	var totalCount int64
	for _, r := range rows {
		bytes := asInt64(r.Bytes)
		totalBytes += bytes
		totalCount += r.Count
		if byKind[r.Kind] == nil {
			byKind[r.Kind] = map[string]any{"count": int64(0), "bytes": int64(0), "by_storage": map[string]any{}}
		}
		byKind[r.Kind]["count"] = byKind[r.Kind]["count"].(int64) + r.Count
		byKind[r.Kind]["bytes"] = byKind[r.Kind]["bytes"].(int64) + bytes
		byKind[r.Kind]["by_storage"].(map[string]any)[r.Storage] = map[string]int64{"count": r.Count, "bytes": bytes}
	}
	orphan, _ := s.q.CountOrphanMedia(c.Request().Context())
	return c.JSON(http.StatusOK, map[string]any{
		"count": totalCount, "bytes": totalBytes, "orphan": orphan, "by_kind": byKind,
	})
}

// deleteMedia DELETE /admin/media/:id?force=1
func (s *Server) deleteMedia(c echo.Context) error {
	att, err := s.q.GetAttachment(c.Request().Context(), c.Param("id"))
	if err != nil {
		return apiError(c, http.StatusNotFound, "not_found", "附件不存在")
	}
	if att.PostID.Valid && c.QueryParam("force") != "1" {
		return apiError(c, http.StatusConflict, "in_use", "附件仍被帖子引用；如需强制删除请加 force=1")
	}

	ctx := c.Request().Context()
	if att.StorageKey != "" {
		if err := s.store.Delete(ctx, att.StorageKey); err != nil {
			slog.Warn("delete media object failed", "id", att.ID, "key", att.StorageKey, "err", err)
		}
		if s.localStore != nil {
			_ = s.localStore.Delete(ctx, att.StorageKey) // 切换存储后的旧本地副本
		}
	}
	if att.ThumbPath != "" {
		if err := s.store.Delete(ctx, att.ThumbPath); err != nil {
			slog.Warn("delete media thumb failed", "id", att.ID, "key", att.ThumbPath, "err", err)
		}
		if s.localStore != nil {
			_ = s.localStore.Delete(ctx, att.ThumbPath)
		}
	}
	if att.PosterPath != "" && s.localStore != nil {
		if err := s.localStore.Delete(ctx, att.PosterPath); err != nil {
			slog.Warn("delete media poster failed", "id", att.ID, "key", att.PosterPath, "err", err)
		}
	}
	if err := s.q.DeleteAttachment(ctx, att.ID); err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case []byte:
		x, _ := strconv.ParseInt(string(n), 10, 64)
		return x
	case string:
		x, _ := strconv.ParseInt(n, 10, 64)
		return x
	default:
		return 0
	}
}
