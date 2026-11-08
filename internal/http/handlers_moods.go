package http

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/mood"
	"github.com/lmb666666/wisp/internal/repo"
)

// maxMoodRangeDays 单次查询的最大跨度（约 10 年）：
// 覆盖时间线多年滚动，同时给匿名读一个上限。
const maxMoodRangeDays = 3659

type moodItem struct {
	Date string `json:"date"`
	Mood string `json:"mood"`
}

// listMoods GET /api/v1/moods?from=YYYY-MM-DD&to=YYYY-MM-DD（公开）。
func (s *Server) listMoods(c echo.Context) error {
	from, to := c.QueryParam("from"), c.QueryParam("to")
	if !mood.ValidDate(from) || !mood.ValidDate(to) {
		return apiError(c, http.StatusBadRequest, "bad_request", "from/to 必须是 YYYY-MM-DD")
	}
	if from > to {
		return apiError(c, http.StatusBadRequest, "bad_request", "from 不能晚于 to")
	}
	if moodRangeExceeded(from, to) {
		return apiError(c, http.StatusBadRequest, "bad_request", "时间跨度不能超过 10 年")
	}

	rows, err := s.q.ListMoods(c.Request().Context(), repo.ListMoodsParams{
		UserID:   s.ownerID(c.Request().Context()),
		FromDate: from,
		ToDate:   to,
	})
	if err != nil {
		return s.internalError(c, err)
	}
	items := make([]moodItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, moodItem{Date: row.MoodDate, Mood: row.Mood})
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

// latestMood GET /api/v1/moods/latest（公开）：访客「最近心情」用。
func (s *Server) latestMood(c echo.Context) error {
	row, err := s.q.LatestMood(c.Request().Context(), s.ownerID(c.Request().Context()))
	if errors.Is(err, sql.ErrNoRows) {
		return apiError(c, http.StatusNotFound, "not_found", "还没有心情记录")
	}
	if err != nil {
		return s.internalError(c, err)
	}
	return c.JSON(http.StatusOK, moodItem{Date: row.MoodDate, Mood: row.Mood})
}

// setMood PUT /api/v1/moods/:date（会话或 write PAT）。
func (s *Server) setMood(c echo.Context) error {
	date := c.Param("date")
	if err := validateMoodDate(date); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	var req struct {
		Mood string `json:"mood"`
	}
	if err := c.Bind(&req); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求格式错误")
	}
	if !mood.Valid(req.Mood) {
		return apiError(c, http.StatusBadRequest, "bad_request", "mood 只能是 happy/calm/okay/tired/low")
	}
	p := s.current(c)
	if p == nil {
		return apiError(c, http.StatusUnauthorized, "unauthorized", "未登录")
	}

	now := time.Now().Unix()
	row, err := s.q.UpsertMood(c.Request().Context(), repo.UpsertMoodParams{
		UserID: p.userID, MoodDate: date, Mood: req.Mood, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return s.internalError(c, err)
	}
	s.emitMood("mood.updated", moodItem{Date: row.MoodDate, Mood: row.Mood})
	return c.JSON(http.StatusOK, moodItem{Date: row.MoodDate, Mood: row.Mood})
}

// deleteMood DELETE /api/v1/moods/:date（会话或 write PAT）；不存在也算成功。
func (s *Server) deleteMood(c echo.Context) error {
	date := c.Param("date")
	if !mood.ValidDate(date) {
		return apiError(c, http.StatusBadRequest, "bad_request", "日期必须是 YYYY-MM-DD")
	}
	p := s.current(c)
	if p == nil {
		return apiError(c, http.StatusUnauthorized, "unauthorized", "未登录")
	}

	n, err := s.q.DeleteMood(c.Request().Context(), repo.DeleteMoodParams{UserID: p.userID, MoodDate: date})
	if err != nil {
		return s.internalError(c, err)
	}
	if n > 0 {
		s.emitMood("mood.deleted", map[string]string{"date": date})
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) emitMood(event string, payload any) {
	if s.hooks != nil {
		s.hooks.Emit(event, payload)
	}
}

// validateMoodDate 允许补记历史日期，最多到服务端 UTC 明天（容忍时区领先）。
func validateMoodDate(date string) error {
	if !mood.ValidDate(date) || date < "2000-01-01" {
		return errors.New("日期必须是 2000-01-01 之后的 YYYY-MM-DD")
	}
	limit := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")
	if date > limit {
		return errors.New("不能记录未来日期")
	}
	return nil
}

func moodRangeExceeded(from, to string) bool {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return true
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return true
	}
	return int(end.Sub(start).Hours()/24) > maxMoodRangeDays
}
