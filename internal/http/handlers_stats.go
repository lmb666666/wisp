package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/repo"
)

type dailyRow struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

// statsDaily 每日发帖计数（热力图用）：?year=2026。登录后含私密。
func (s *Server) statsDaily(c echo.Context) error {
	year := time.Now().Year()
	if y, err := strconv.Atoi(c.QueryParam("year")); err == nil && y >= 2000 && y <= 2100 {
		year = y
	}
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local).Unix()
	end := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.Local).Unix()

	var items []dailyRow
	if s.current(c) != nil {
		rows, err := s.q.DailyCountsAuthor(c.Request().Context(), repo.DailyCountsAuthorParams{
			PublishedAt: start, PublishedAt_2: end,
		})
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "internal", err.Error())
		}
		for _, r := range rows {
			items = append(items, dailyRow{Day: asString(r.Day), Count: r.Count})
		}
	} else {
		rows, err := s.q.DailyCountsPublic(c.Request().Context(), repo.DailyCountsPublicParams{
			PublishedAt: start, PublishedAt_2: end,
		})
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "internal", err.Error())
		}
		for _, r := range rows {
			items = append(items, dailyRow{Day: asString(r.Day), Count: r.Count})
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"year": year, "items": items})
}

// asString：SQLite date() 经驱动返回 string，但 sqlc 推断为 any。
func asString(v any) string {
	s, _ := v.(string)
	return s
}
