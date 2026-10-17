package http

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/repo"
)

// normalizeTagName 统一去除前导 # 与首尾空白；拒绝空名、空白和超长标签。
func normalizeTagName(raw string) (string, error) {
	name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "#"))
	if name == "" {
		return "", errors.New("标签名不能为空")
	}
	if strings.ContainsAny(name, " \t\r\n") {
		return "", errors.New("标签名不能包含空白字符")
	}
	if utf8.RuneCountInString(name) > 64 {
		return "", errors.New("标签名不能超过 64 个字符")
	}
	return name, nil
}

func (s *Server) tagID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("非法标签 ID")
	}
	return id, nil
}

// renameTag PATCH /admin/tags/:id {name}
func (s *Server) renameTag(c echo.Context) error {
	id, err := s.tagID(c)
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&in); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求体格式错误")
	}
	name, err := normalizeTagName(in.Name)
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	ctx := c.Request().Context()
	tag, err := s.q.GetTag(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return apiError(c, http.StatusNotFound, "not_found", "标签不存在")
	}
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	existing, err := s.q.GetTagByName(ctx, name)
	switch {
	case err == nil && existing.ID != tag.ID:
		return apiError(c, http.StatusConflict, "tag_exists", "同名标签已存在，可将其合并")
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	if err := s.q.RenameTag(ctx, repo.RenameTagParams{Name: name, ID: id}); err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"id": id, "name": name})
}

// mergeTags POST /admin/tags/merge {from_ids, into}
func (s *Server) mergeTags(c echo.Context) error {
	var in struct {
		FromIDs []int64 `json:"from_ids"`
		Into    string  `json:"into"`
	}
	if err := c.Bind(&in); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求体格式错误")
	}
	name, err := normalizeTagName(in.Into)
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	if len(in.FromIDs) == 0 {
		return apiError(c, http.StatusBadRequest, "bad_request", "至少选择一个源标签")
	}
	if len(in.FromIDs) > 100 {
		return apiError(c, http.StatusBadRequest, "bad_request", "一次最多合并 100 个标签")
	}
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(in.FromIDs))
	for _, id := range in.FromIDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return apiError(c, http.StatusBadRequest, "bad_request", "至少选择一个源标签")
	}
	if len(ids) == 1 {
		// 单标签合并到自身名称是空操作
		if tag, err := s.q.GetTag(c.Request().Context(), ids[0]); err == nil && tag.Name == name {
			return apiError(c, http.StatusBadRequest, "bad_request", "目标与源标签相同")
		}
	}

	ctx := c.Request().Context()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	defer func() { _ = tx.Rollback() }()
	q := repo.New(tx)

	target, err := q.GetTagByName(ctx, name)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		target, err = q.CreateTag(ctx, name)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "internal", err.Error())
		}
	case err != nil:
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}

	merged := 0
	for _, id := range ids {
		if id == target.ID {
			continue
		}
		if _, err := q.GetTag(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return apiError(c, http.StatusNotFound, "not_found", "源标签不存在")
		} else if err != nil {
			return apiError(c, http.StatusInternalServerError, "internal", err.Error())
		}
		if err := q.MergeTagPosts(ctx, repo.MergeTagPostsParams{TargetID: target.ID, SourceID: id}); err != nil {
			return apiError(c, http.StatusInternalServerError, "internal", err.Error())
		}
		if err := q.DeleteTagLinks(ctx, id); err != nil {
			return apiError(c, http.StatusInternalServerError, "internal", err.Error())
		}
		if err := q.DeleteTag(ctx, id); err != nil {
			return apiError(c, http.StatusInternalServerError, "internal", err.Error())
		}
		merged++
	}
	if err := tx.Commit(); err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{
		"id": target.ID, "name": target.Name, "merged": merged,
	})
}

// deleteTag DELETE /admin/tags/:id：只解绑，不删帖。
func (s *Server) deleteTag(c echo.Context) error {
	id, err := s.tagID(c)
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	ctx := c.Request().Context()
	if _, err := s.q.GetTag(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return apiError(c, http.StatusNotFound, "not_found", "标签不存在")
	} else if err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	if err := s.q.DeleteTagLinks(ctx, id); err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	if err := s.q.DeleteTag(ctx, id); err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}
