package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/capsule"
)

// exportCapsule GET /admin/export/capsule?include_private=1
// 流式导出 Wisp 内容包（Markdown + frontmatter + 原图），可导入任意实例。
func (s *Server) exportCapsule(c echo.Context) error {
	includePrivate := c.QueryParam("include_private") == "1"
	name := fmt.Sprintf("wisp-capsule-%s.zip", time.Now().Format("20060102-150405"))

	h := c.Response().Header()
	h.Set("Content-Type", "application/zip")
	h.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	h.Set("Cache-Control", "no-store")
	c.Response().WriteHeader(http.StatusOK)

	stats, err := capsule.Export(c.Request().Context(), s.q, s.store, c.Response().Writer, capsule.Options{
		IncludePrivate: includePrivate,
		SourceURL:      s.cfg.PublicURL,
	})
	if err != nil {
		// 响应头已发出，只能记录日志并中断
		slog.Error("capsule export failed", "err", err)
		return nil
	}
	slog.Info("capsule exported", "posts", stats.Posts, "attachments", stats.Attachments,
		"media_bytes", stats.MediaBytes, "warnings", len(stats.Warnings), "include_private", includePrivate)
	for _, w := range stats.Warnings {
		slog.Warn("capsule export warning", "detail", w)
	}
	return nil
}
