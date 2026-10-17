package http

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/version"
)

// overview GET /admin/overview：站点内容、媒体、备份与运行信息（owner）。
func (s *Server) overview(c echo.Context) error {
	ctx := c.Request().Context()
	posts, err := s.q.OverviewPosts(ctx, repo.OverviewPostsParams{
		TodayStart: startOfToday(), TodayEnd: endOfToday(),
	})
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	rows, err := s.q.MediaStats(ctx)
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	var mediaCount, mediaBytes int64
	byStorage := map[string]int64{}
	for _, r := range rows {
		b := asInt64(r.Bytes)
		mediaCount += r.Count
		mediaBytes += b
		byStorage[r.Storage] += r.Count
	}
	tags, err := s.q.CountTags(ctx)
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	dbBytes := int64(0)
	dbPath := s.cfg.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(s.cfg.DataDir, "wisp.db")
	}
	if info, err := os.Stat(dbPath); err == nil {
		dbBytes = info.Size()
	}
	var lastBackup int64
	if s.backup != nil {
		if st, err := s.backup.Settings(ctx); err == nil {
			lastBackup = st.LastRunAt
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"posts": map[string]any{
			"total": posts.Total, "today": posts.Today, "public": posts.Public,
			"private": posts.Private,
		},
		"attachments": map[string]any{
			"count": mediaCount, "bytes": mediaBytes, "by_storage": byStorage,
		},
		"tags":           tags,
		"db_bytes":       dbBytes,
		"last_backup_at": lastBackup,
		"version":        version.Version,
		"uptime_seconds": int64(timeSince(s.startedAt).Seconds()),
	})
}

// getGeoSettings GET /admin/settings/geo：返回可选的 Nominatim 兼容端点。
func (s *Server) getGeoSettings(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{"geocode_url": s.cfg.GeocodeURL})
}
