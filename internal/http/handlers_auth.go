package http

import (
	"net/http"
	"regexp"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/auth"
	"github.com/lmb666666/wisp/internal/repo"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{2,32}$`)

// authStatus 首启向导判断：没有任何用户 → 未初始化。
func (s *Server) authStatus(c echo.Context) error {
	n, err := s.q.CountUsers(c.Request().Context())
	if err != nil {
		return s.internalError(c, err)
	}
	resp := map[string]any{"initialized": n > 0}
	// 用户名不向未鉴权访客暴露（防枚举）；登录后的 /auth/me 已提供
	if s.current(c) != nil {
		if owner, err := s.q.GetOwnerUser(c.Request().Context()); err == nil {
			resp["username"] = owner.Username
		}
	}
	return c.JSON(http.StatusOK, resp)
}

type initReq struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// authInit 首启向导：仅当零用户时允许创建 owner，并直接登录。
func (s *Server) authInit(c echo.Context) error {
	var req initReq
	if err := c.Bind(&req); err != nil || req.Username == "" || req.Password == "" {
		return apiError(c, http.StatusBadRequest, "bad_request", "需要 username 与 password")
	}
	if len(req.Password) < 8 {
		return apiError(c, http.StatusBadRequest, "bad_request", "密码至少 8 位")
	}
	if !usernamePattern.MatchString(req.Username) {
		return apiError(c, http.StatusBadRequest, "bad_request", "用户名只能是 2~32 位字母/数字/下划线/连字符")
	}
	n, err := s.q.CountUsers(c.Request().Context())
	if err != nil {
		return s.internalError(c, err)
	}
	if n > 0 {
		return apiError(c, http.StatusConflict, "conflict", auth.ErrAlreadyInitialized.Error())
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return s.internalError(c, err)
	}
	now := time.Now().Unix()
	user, err := s.q.CreateUser(c.Request().Context(), repo.CreateUserParams{
		Username:     req.Username,
		PasswordHash: hash,
		DisplayName:  req.DisplayName,
		IsOwner:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return s.internalError(c, err)
	}
	plain, err := s.auth.Login(c.Request().Context(), user.Username, req.Password, c.Request().UserAgent())
	if err != nil {
		return s.internalError(c, err)
	}
	s.setSessionCookie(c, plain)
	return c.JSON(http.StatusCreated, map[string]string{"username": user.Username})
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) authLogin(c echo.Context) error {
	var req loginReq
	if err := c.Bind(&req); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求格式错误")
	}
	plain, err := s.auth.Login(c.Request().Context(), req.Username, req.Password, c.Request().UserAgent())
	if err != nil {
		return apiError(c, http.StatusUnauthorized, "invalid_credentials", err.Error())
	}
	s.setSessionCookie(c, plain)
	return c.JSON(http.StatusOK, map[string]string{"username": req.Username})
}

func (s *Server) authLogout(c echo.Context) error {
	if ck, err := c.Cookie(auth.SessionCookie); err == nil {
		_ = s.auth.Logout(c.Request().Context(), ck.Value)
	}
	s.clearSessionCookie(c)
	return c.NoContent(http.StatusNoContent)
}

// authMe 当前身份（会话或 PAT）。
func (s *Server) authMe(c echo.Context) error {
	p := s.current(c)
	if p == nil {
		return apiError(c, http.StatusUnauthorized, "unauthorized", "未登录")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"username":   p.user,
		"is_owner":   p.isOwner,
		"via_pat":    p.viaPAT,
		"token_name": p.tokenName,
	})
}

type tokenReq struct {
	Name   string `json:"name"`
	Scopes string `json:"scopes"`
}

// createToken 新建 PAT（明文只出现一次）。
func (s *Server) createToken(c echo.Context) error {
	var req tokenReq
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return apiError(c, http.StatusBadRequest, "bad_request", "需要 name")
	}
	if req.Scopes == "" {
		req.Scopes = "read"
	}
	if !auth.ValidScopes(req.Scopes) {
		return apiError(c, http.StatusBadRequest, "bad_request", "scopes 只能是 read / write / read,write")
	}
	plain, row, err := s.auth.CreateToken(c.Request().Context(), req.Name, req.Scopes)
	if err != nil {
		return s.internalError(c, err)
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"id": row.ID, "name": row.Name, "scopes": row.Scopes, "token": plain,
	})
}

func (s *Server) listTokens(c echo.Context) error {
	rows, err := s.auth.ListTokens(c.Request().Context())
	if err != nil {
		return s.internalError(c, err)
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		item := map[string]any{"id": r.ID, "name": r.Name, "scopes": r.Scopes, "created_at": r.CreatedAt}
		if r.LastUsedAt.Valid {
			item["last_used_at"] = r.LastUsedAt.Int64
		}
		items = append(items, item)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (s *Server) deleteToken(c echo.Context) error {
	if err := s.auth.DeleteToken(c.Request().Context(), c.Param("id")); err != nil {
		return s.internalError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
