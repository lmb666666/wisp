package http

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/service"
)

// clientIDPattern 匿名访客标识：UUID 或等长随机串。
var clientIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// viewerClientID 浏览者标识：登录用户取 "u:<username>"，游客取 X-Client-ID 头。
func (s *Server) viewerClientID(c echo.Context) string {
	if p := s.current(c); p != nil {
		return "u:" + p.user
	}
	uid := c.Request().Header.Get("X-Client-ID")
	if clientIDPattern.MatchString(uid) {
		return uid
	}
	return ""
}

// addReaction 点赞：POST /posts/:id/reactions {"emoji":"👍"}。
func (s *Server) addReaction(c echo.Context) error {
	var req struct {
		Emoji string `json:"emoji"`
	}
	if err := c.Bind(&req); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求格式错误")
	}
	clientID := s.viewerClientID(c)
	if clientID == "" {
		return apiError(c, http.StatusBadRequest, "bad_request", "缺少 X-Client-ID 标识")
	}
	err := s.posts.React(c.Request().Context(), c.Param("id"), clientID, req.Emoji)
	if errors.Is(err, service.ErrNotFound) {
		return apiError(c, http.StatusNotFound, "not_found", "帖子不存在")
	}
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	go s.sendReactionNotify(c.Param("id"), req.Emoji)
	return c.NoContent(http.StatusNoContent)
}

// removeReaction 取消点赞：DELETE /posts/:id/reactions?emoji=👍。
func (s *Server) removeReaction(c echo.Context) error {
	clientID := s.viewerClientID(c)
	if clientID == "" {
		return apiError(c, http.StatusBadRequest, "bad_request", "缺少 X-Client-ID 标识")
	}
	if err := s.posts.Unreact(c.Request().Context(), c.Param("id"), clientID, c.QueryParam("emoji")); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// searchPosts 全文搜索：GET /search?q=...（≥3 字符走 FTS5 trigram，短词 LIKE）。
func (s *Server) searchPosts(c echo.Context) error {
	limit := strconvAtoiDefault(c.QueryParam("limit"), 20)
	p := s.current(c)
	items, err := s.posts.Search(
		c.Request().Context(),
		c.QueryParam("q"),
		p != nil,
		limit,
		s.viewerClientID(c),
	)
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}
