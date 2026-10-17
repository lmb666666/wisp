package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/media"
	"github.com/lmb666666/wisp/internal/repo"
)

const settingKeyStorage = "storage_settings"

const secretMask = "***"

// maskSecrets 脱敏密钥字段（GET 响应用）。
func maskSecrets(st media.StoredSettings) media.StoredSettings {
	if st.S3.SecretKey != "" {
		st.S3.SecretKey = secretMask
	}
	if st.Lsky.Token != "" {
		st.Lsky.Token = secretMask
	}
	return st
}

// mergeSecrets：PUT 时密钥字段为空或掩码 → 沿用旧值。
func (s *Server) mergeSecrets(ctx context.Context, incoming media.StoredSettings) media.StoredSettings {
	old := s.loadStorageSettings(ctx)
	if incoming.S3.SecretKey == "" || incoming.S3.SecretKey == secretMask {
		incoming.S3.SecretKey = old.S3.SecretKey
	}
	if incoming.Lsky.Token == "" || incoming.Lsky.Token == secretMask {
		incoming.Lsky.Token = old.Lsky.Token
	}
	return incoming
}

// loadStorageSettings 读取已保存配置（不存在返回 local 默认）。
func (s *Server) loadStorageSettings(ctx context.Context) media.StoredSettings {
	st := media.DefaultSettings()
	val, err := s.q.GetSetting(ctx, settingKeyStorage)
	if err != nil || val == "" {
		return st
	}
	_ = json.Unmarshal([]byte(val), &st)
	return st
}

// getStorageSettings GET /admin/settings/storage（密钥脱敏）。
func (s *Server) getStorageSettings(c echo.Context) error {
	st := s.loadStorageSettings(c.Request().Context())
	return c.JSON(http.StatusOK, maskSecrets(st))
}

// putStorageSettings PUT /admin/settings/storage：保存并热切换驱动。
func (s *Server) putStorageSettings(c echo.Context) error {
	var st media.StoredSettings
	if err := c.Bind(&st); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求格式错误")
	}
	if st.Backend != "local" && st.Backend != "s3" && st.Backend != "lsky" {
		return apiError(c, http.StatusBadRequest, "bad_request", "backend 必须是 local/s3/lsky")
	}
	st = s.mergeSecrets(c.Request().Context(), st)

	primary, err := media.BuildStorage(c.Request().Context(), st)
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	if primary == nil {
		primary = s.localStore
	}
	s.selector.Configure(primary, st.MirrorLocal)

	blob, _ := json.Marshal(st)
	if err := s.q.UpsertSetting(c.Request().Context(), repo.UpsertSettingParams{
		Key: settingKeyStorage, Value: string(blob), UpdatedAt: time.Now().Unix(),
	}); err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.JSON(http.StatusOK, maskSecrets(st))
}

// testStorage POST /admin/settings/storage/test：用提交的配置真实写删一个测试对象。
func (s *Server) testStorage(c echo.Context) error {
	var st media.StoredSettings
	if err := c.Bind(&st); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求格式错误")
	}
	if st.Backend == "local" {
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "message": "local 驱动始终可用"})
	}
	st = s.mergeSecrets(c.Request().Context(), st)
	primary, err := media.BuildStorage(c.Request().Context(), st)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
	}
	key := "wisp-connectivity-test.txt"
	if err := primary.Put(c.Request().Context(), key, strings.NewReader("wisp"), "text/plain"); err != nil {
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
	}
	_ = primary.Delete(c.Request().Context(), key)
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "message": "连通正常（已写入并清理测试对象）"})
}

const settingKeyMeting = "meting_api_url"

// getMetingSettings GET /admin/settings/meting：返回当前生效地址（数据库优先，未设置时回退环境变量）。
func (s *Server) getMetingSettings(c echo.Context) error {
	url := s.cfg.MetingAPI
	if val, err := s.q.GetSetting(c.Request().Context(), settingKeyMeting); err == nil {
		url = val
	}
	return c.JSON(http.StatusOK, map[string]string{"url": url})
}

// putMetingSettings PUT /admin/settings/meting：保存并立即生效。
func (s *Server) putMetingSettings(c echo.Context) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&req); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求格式错误")
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL != "" && !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		return apiError(c, http.StatusBadRequest, "bad_request", "地址需以 http(s):// 开头")
	}
	if err := s.q.UpsertSetting(c.Request().Context(), repo.UpsertSettingParams{
		Key: settingKeyMeting, Value: req.URL, UpdatedAt: time.Now().Unix(),
	}); err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	s.posts.SetMetingAPI(req.URL)
	return c.JSON(http.StatusOK, map[string]string{"url": req.URL})
}
