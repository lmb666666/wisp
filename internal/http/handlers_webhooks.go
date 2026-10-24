package http

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/util"
)

func randomHex() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func timeNowUnix() int64 { return time.Now().Unix() }

var validWebhookEvents = map[string]bool{
	"post.created": true,
	"post.updated": true,
	"post.deleted": true,
}

type createWebhookReq struct {
	Url    string `json:"url"`
	Events string `json:"events"` // 逗号分隔：post.created,post.updated,post.deleted
}

// listWebhooks GET /admin/webhooks（secret 脱敏）。
func (s *Server) listWebhooksAdmin(c echo.Context) error {
	rows, err := s.q.ListWebhooks(c.Request().Context())
	if err != nil {
		return s.internalError(c, err)
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, map[string]any{
			"id": r.ID, "url": r.Url, "events": r.Events, "created_at": r.CreatedAt,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

// createWebhook POST /admin/webhooks：secret 只返回一次。
func (s *Server) createWebhook(c echo.Context) error {
	var req createWebhookReq
	if err := c.Bind(&req); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求格式错误")
	}
	req.Url = strings.TrimSpace(req.Url)
	if !strings.HasPrefix(req.Url, "http://") && !strings.HasPrefix(req.Url, "https://") {
		return apiError(c, http.StatusBadRequest, "bad_request", "url 必须是 http(s) 地址")
	}
	events := strings.Split(req.Events, ",")
	cleaned := make([]string, 0, len(events))
	for _, e := range events {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if !validWebhookEvents[e] {
			return apiError(c, http.StatusBadRequest, "bad_request", "非法事件: "+e)
		}
		cleaned = append(cleaned, e)
	}
	if len(cleaned) == 0 {
		return apiError(c, http.StatusBadRequest, "bad_request", "至少订阅一个事件")
	}
	secret := "wsec_" + randomHex()
	row, err := s.q.CreateWebhook(c.Request().Context(), repo.CreateWebhookParams{
		ID:        util.NewULID(),
		Url:       req.Url,
		Secret:    secret,
		Events:    strings.Join(cleaned, ","),
		CreatedAt: timeNowUnix(),
	})
	if err != nil {
		return s.internalError(c, err)
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"id": row.ID, "url": row.Url, "events": row.Events, "secret": secret,
	})
}

// deleteWebhook DELETE /admin/webhooks/:id。
func (s *Server) deleteWebhook(c echo.Context) error {
	if err := s.q.DeleteWebhook(c.Request().Context(), c.Param("id")); err != nil {
		return s.internalError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
