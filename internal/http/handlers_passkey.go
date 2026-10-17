package http

import (
	"bytes"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// bufferedBody 把请求体读进内存并重置为可重读（go-webauthn 需要 *http.Request 自行解析 body）。
func bufferedBody(c echo.Context) (*http.Request, error) {
	r := c.Request()
	if r.Body == nil {
		return r, nil
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(b))
	return r, nil
}

// beginPasskeyRegistration POST /auth/passkey/register/begin（仅会话）。
// 返回 PublicKeyCredentialCreationOptions 与仪式 ID（finish 时回传）。
func (s *Server) beginPasskeyRegistration(c echo.Context) error {
	p := s.current(c)
	if p == nil {
		return apiError(c, http.StatusUnauthorized, "unauthorized", "未登录")
	}
	user, err := s.auth.Owner(c.Request().Context())
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	if s.passkeys == nil {
		return apiError(c, http.StatusNotImplemented, "disabled", "Passkey 未启用（检查 WISP_PUBLIC_URL）")
	}
	options, sessionID, err := s.passkeys.BeginRegistration(c.Request().Context(), user)
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"options": options, "session": sessionID})
}

// finishPasskeyRegistration POST /auth/passkey/register/finish?session=<id>。
// body 原样透传给 go-webauthn 解析（浏览器 AuthenticatorAttestationResponse JSON）。
func (s *Server) finishPasskeyRegistration(c echo.Context) error {
	p := s.current(c)
	if p == nil {
		return apiError(c, http.StatusUnauthorized, "unauthorized", "未登录")
	}
	user, err := s.auth.Owner(c.Request().Context())
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	r, err := bufferedBody(c)
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	if err := s.passkeys.FinishRegistration(c.Request().Context(), user, c.QueryParam("session"), r); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

// beginPasskeyLogin POST /auth/passkey/login/begin（无需身份）。
func (s *Server) beginPasskeyLogin(c echo.Context) error {
	if s.passkeys == nil {
		return apiError(c, http.StatusNotImplemented, "disabled", "Passkey 未启用（检查 WISP_PUBLIC_URL）")
	}
	options, sessionID, err := s.passkeys.BeginLogin()
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"options": options, "session": sessionID})
}

// finishPasskeyLogin POST /auth/passkey/login/finish?session=<id>：成功即种会话 Cookie。
func (s *Server) finishPasskeyLogin(c echo.Context) error {
	r, err := bufferedBody(c)
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	user, plain, err := s.passkeys.FinishLogin(c.QueryParam("session"), r)
	if err != nil {
		return apiError(c, http.StatusUnauthorized, "unauthorized", err.Error())
	}
	s.setSessionCookie(c, plain)
	return c.JSON(http.StatusOK, map[string]string{"username": user.Username})
}

// listPasskeys GET /auth/passkeys（当前会话用户的凭证列表）。
func (s *Server) listPasskeys(c echo.Context) error {
	p := s.current(c)
	if p == nil {
		return apiError(c, http.StatusUnauthorized, "unauthorized", "未登录")
	}
	rows, err := s.q.ListPasskeysByUser(c.Request().Context(), p.userID)
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, map[string]any{
			"id":         r.ID,
			"transports": r.Transports,
			"created_at": r.CreatedAt,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

// deletePasskey DELETE /auth/passkeys/:id。
func (s *Server) deletePasskey(c echo.Context) error {
	if err := s.q.DeletePasskey(c.Request().Context(), c.Param("id")); err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}
