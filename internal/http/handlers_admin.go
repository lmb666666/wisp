package http

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
)

// downloadBackup 生成在线一致性快照（VACUUM INTO）并打包为 zip 下载。
// 恢复流程见 docs/DEPLOY.md：停服 → 替换 db 文件 → 重启。
func (s *Server) downloadBackup(c echo.Context) error {
	if err := os.MkdirAll(s.backupDir(), 0o755); err != nil {
		return s.internalError(c, err)
	}
	name := fmt.Sprintf("web-backup-%s.db", time.Now().Format("20060102-150405"))
	tmp := filepath.Join(s.backupDir(), name)
	defer func() { _ = os.Remove(tmp) }()

	if _, err := s.db.ExecContext(c.Request().Context(), "VACUUM INTO ?", tmp); err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", "创建快照失败: "+err.Error())
	}

	f, err := os.Open(tmp)
	if err != nil {
		return s.internalError(c, err)
	}
	defer func() { _ = f.Close() }()

	zipName := fmt.Sprintf("wisp-backup-%s.zip", time.Now().Format("20060102-150405"))
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+zipName+`"`)
	c.Response().Header().Set("Content-Type", "application/zip")
	c.Response().WriteHeader(http.StatusOK)

	zw := zip.NewWriter(c.Response().Writer)
	defer func() { _ = zw.Close() }()
	entry, err := zw.Create("wisp.db")
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, f)
	return err
}

func (s *Server) backupDir() string {
	return filepath.Join(s.cfg.DataDir, "backups")
}
