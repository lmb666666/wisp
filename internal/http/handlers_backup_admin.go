package http

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/backup"
)

// getBackupSettings GET /admin/settings/backup
func (s *Server) getBackupSettings(c echo.Context) error {
	if s.backup == nil {
		return apiError(c, http.StatusServiceUnavailable, "unavailable", "备份服务未启用")
	}
	ctx := c.Request().Context()
	st, err := s.backup.Settings(ctx)
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	files, err := s.backup.Files()
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"settings": st, "backups": files})
}

// putBackupSettings PUT /admin/settings/backup
func (s *Server) putBackupSettings(c echo.Context) error {
	if s.backup == nil {
		return apiError(c, http.StatusServiceUnavailable, "unavailable", "备份服务未启用")
	}
	var in backup.Settings
	if err := c.Bind(&in); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求体格式错误")
	}
	st, err := s.backup.Save(c.Request().Context(), in)
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.JSON(http.StatusOK, st)
}

// runBackup POST /admin/backup/run：立即生成快照（即使自动备份已关闭）。
func (s *Server) runBackup(c echo.Context) error {
	if s.backup == nil {
		return apiError(c, http.StatusServiceUnavailable, "unavailable", "备份服务未启用")
	}
	file, err := s.backup.Run(c.Request().Context())
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "backup_failed", err.Error())
	}
	return c.JSON(http.StatusOK, file)
}
