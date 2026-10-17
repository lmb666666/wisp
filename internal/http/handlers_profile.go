package http

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/auth"
)

// getProfile GET /auth/profile。
func (s *Server) getProfile(c echo.Context) error {
	p := s.current(c)
	if p == nil {
		return apiError(c, http.StatusUnauthorized, "unauthorized", "未登录")
	}
	prof, err := s.auth.Profile(c.Request().Context(), p.userID)
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.JSON(http.StatusOK, prof)
}

type profileReq struct {
	DisplayName     *string `json:"display_name"`
	CurrentPassword string  `json:"current_password"`
	NewPassword     string  `json:"new_password"`
}

// updateProfile PATCH /auth/profile：改昵称与/或密码。
func (s *Server) updateProfile(c echo.Context) error {
	p := s.current(c)
	if p == nil {
		return apiError(c, http.StatusUnauthorized, "unauthorized", "未登录")
	}
	var req profileReq
	if err := c.Bind(&req); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求格式错误")
	}
	ctx := c.Request().Context()

	if req.NewPassword != "" {
		if err := s.auth.ChangePassword(ctx, p.userID, req.CurrentPassword, req.NewPassword); err != nil {
			if errors.Is(err, auth.ErrCurrentPassword) {
				return apiError(c, http.StatusBadRequest, "bad_password", err.Error())
			}
			return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
		}
	}
	if req.DisplayName != nil {
		if _, err := s.auth.UpdateDisplayName(ctx, p.userID, *req.DisplayName); err != nil {
			return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
		}
	}
	prof, err := s.auth.Profile(ctx, p.userID)
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.JSON(http.StatusOK, prof)
}
