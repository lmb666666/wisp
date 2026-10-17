package http

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/auth"
)

const ctxPrincipal = "principal"

// principal 统一身份：会话用户（hasWrite=true）或 PAT（按作用域）。
type principal struct {
	userID    int64
	user      string // username
	isOwner   bool
	viaPAT    bool
	viaCookie bool
	tokenName string
	hasWrite  bool
}

func (s *Server) current(c echo.Context) *principal {
	p, _ := c.Get(ctxPrincipal).(*principal)
	return p
}

func apiError(c echo.Context, status int, code, msg string) error {
	return c.JSON(status, map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}

// resolveAuth 尽力识别身份（不强制），供 GET 类接口区分作者视角。
// 身份来源优先级：会话 Cookie → Bearer PAT。
func (s *Server) resolveAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if ck, err := c.Cookie(auth.SessionCookie); err == nil && ck.Value != "" {
			user, err := s.auth.UserBySession(c.Request().Context(), ck.Value)
			if err == nil {
				c.Set(ctxPrincipal, &principal{
					userID:    user.ID,
					user:      user.Username,
					isOwner:   user.IsOwner == 1,
					viaCookie: true,
					hasWrite:  true,
				})
				return next(c)
			}
			if !errors.Is(err, auth.ErrUnauthorized) {
				return apiError(c, http.StatusInternalServerError, "internal", err.Error())
			}
		}
		if h := c.Request().Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			p, err := s.principalByToken(c.Request().Context(), strings.TrimPrefix(h, "Bearer "))
			switch {
			case err == nil:
				c.Set(ctxPrincipal, p)
			case errors.Is(err, auth.ErrUnauthorized):
				// 明确提供了凭证但无效：不能静默降级为游客
				return apiError(c, http.StatusUnauthorized, "unauthorized", "令牌无效")
			default:
				return apiError(c, http.StatusInternalServerError, "internal", err.Error())
			}
		}
		return next(c)
	}
}

// requireUser 仅限会话（Web UI 与令牌管理）。
func (s *Server) requireUser(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ck, err := c.Cookie(auth.SessionCookie)
		if err != nil || ck.Value == "" {
			return apiError(c, http.StatusUnauthorized, "unauthorized", auth.ErrUnauthorized.Error())
		}
		user, err := s.auth.UserBySession(c.Request().Context(), ck.Value)
		if err != nil {
			return apiError(c, http.StatusUnauthorized, "unauthorized", err.Error())
		}
		c.Set(ctxPrincipal, &principal{
			userID: user.ID, user: user.Username,
			isOwner: user.IsOwner == 1, viaCookie: true, hasWrite: true,
		})
		if err := s.ensureSameOrigin(c); err != nil {
			return err
		}
		return next(c)
	}
}

// sameOrigin 校验 Cookie 凭证的写请求 Origin（CSRF 防线；PAT 凭证不受此约束）。
// Origin 的 host 需与「请求自身的 Host 头」或「配置的 PublicURL」一致——
// 站点常通过多个地址访问（localhost / 局域网 IP / 域名），请求 Host 才是浏览器
// 实际发往的地址；攻击者页面的 Origin 永远不会等于受害站点的 Host。
func (s *Server) ensureSameOrigin(c echo.Context) error {
	if c.Request().Method == http.MethodGet || c.Request().Method == http.MethodHead ||
		c.Request().Method == http.MethodOptions {
		return nil
	}
	p := s.current(c)
	if p == nil || !p.viaCookie {
		return nil
	}
	origin := c.Request().Header.Get("Origin")
	if origin == "" {
		return nil // 同源 fetch/表单可能不带 Origin；SameSite=Lax 兜底
	}
	o, err := url.Parse(origin)
	if err != nil || (o.Host != c.Request().Host && o.Host != s.publicHost()) {
		return apiError(c, http.StatusForbidden, "forbidden", "跨站请求被拒绝")
	}
	return nil
}

func (s *Server) publicHost() string {
	u, err := url.Parse(s.cfg.PublicURL)
	if err != nil {
		return ""
	}
	return u.Host
}

// requireWrite 会话用户或持 write 作用域的 PAT（先过 resolveAuth）。
func (s *Server) requireWrite(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if err := s.ensureSameOrigin(c); err != nil {
			return err
		}
		p := s.current(c)
		switch {
		case p == nil:
			return apiError(c, http.StatusUnauthorized, "unauthorized", auth.ErrUnauthorized.Error())
		case !p.hasWrite:
			return apiError(c, http.StatusForbidden, "forbidden", auth.ErrTokenScope.Error())
		}
		return next(c)
	}
}

// principalByToken 用明文 PAT 换取身份。
func (s *Server) principalByToken(ctx context.Context, raw string) (*principal, error) {
	tok, err := s.auth.TokenByPlain(ctx, raw)
	if err != nil {
		return nil, err
	}
	user, err := s.auth.Owner(ctx)
	if err != nil {
		return nil, err
	}
	return &principal{
		userID:    user.ID,
		user:      user.Username,
		isOwner:   true,
		viaPAT:    true,
		tokenName: tok.Name,
		hasWrite:  auth.HasScope(tok.Scopes, "write"),
	}, nil
}

// 会话 Cookie：PublicURL 为 https 时自动带 Secure。
func (s *Server) setSessionCookie(c echo.Context, value string) {
	c.SetCookie(&http.Cookie{
		Name:     auth.SessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.HasPrefix(s.cfg.PublicURL, "https"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})
}

func (s *Server) clearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name: auth.SessionCookie, Value: "", Path: "/",
		HttpOnly: true, Secure: strings.HasPrefix(s.cfg.PublicURL, "https"),
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}
