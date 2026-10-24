package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/media"
	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/service"
)

type createPostReq struct {
	ContentMD     string              `json:"content_md"`
	Visibility    string              `json:"visibility"` // 仅接受缺省/public；private 返回 400
	Location      string              `json:"location"`
	PublishedAt   int64               `json:"published_at"` // 0 = 现在，支持补发
	Client        string              `json:"client"`
	AttachmentIDs []string            `json:"attachment_ids"`
	Blocks        []service.PostBlock `json:"blocks"`
}

// listPosts 时间线：游标分页，全部内容公开。
func (s *Server) listPosts(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	before, beforeID := parseCursor(c.QueryParam("before"))

	from, ok := parseFilterTime(c, "from")
	if !ok {
		return nil
	}
	to, ok := parseFilterTime(c, "to")
	if !ok {
		return nil
	}
	if from > 0 && to > 0 && from > to {
		return apiError(c, http.StatusBadRequest, "bad_request", "from 不能晚于 to")
	}
	tags, err := parseTagFilter(c.QueryParam("tags"))
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}

	items, hasMore, err := s.posts.List(c.Request().Context(), service.ListParams{
		Tag:      c.QueryParam("tag"),
		Tags:     tags,
		From:     from,
		To:       to,
		Before:   before,
		BeforeID: beforeID,
		Limit:    limit,
	})
	if err != nil {
		return s.internalError(c, err)
	}
	resp := map[string]any{"items": items, "has_more": hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		resp["next_cursor"] = fmt.Sprintf("%d.%s", last.PublishedAt, last.ID)
	}
	return c.JSON(http.StatusOK, resp)
}

// createPost 发帖；source 由身份类型决定（会话=web，PAT=api）。
// 带 Idempotency-Key 头时：重放返回首次结果（离线队列重试不重复发帖）。
func (s *Server) createPost(c echo.Context) error {
	var req createPostReq
	if err := c.Bind(&req); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求格式错误")
	}
	p := s.current(c)
	source := "web"
	if p != nil && p.viaPAT {
		source = "api"
	}

	// 幂等：同一 key 已处理过则直接返回原帖
	if key := c.Request().Header.Get("Idempotency-Key"); key != "" {
		if len(key) > 128 {
			return apiError(c, http.StatusBadRequest, "bad_request", "Idempotency-Key 过长")
		}
		if postID, err := s.q.GetIdempotentPostID(c.Request().Context(), key); err == nil {
			if dto, err := s.posts.Get(c.Request().Context(), postID); err == nil {
				return c.JSON(http.StatusOK, dto)
			}
			// key 已消费但原帖已删除：不能静默新建第二帖
			return apiError(c, http.StatusConflict, "conflict", "该幂等键对应的帖子已删除")
		}
	}

	authorID := p.userID
	if authorID == 0 {
		if owner, oerr := s.auth.Owner(c.Request().Context()); oerr == nil {
			authorID = owner.ID
		}
	}
	if req.Visibility != "" && req.Visibility != "public" {
		return apiError(c, http.StatusBadRequest, "bad_request", "私密发布已移除，visibility 只能为 public")
	}
	dto, err := s.posts.Create(c.Request().Context(), authorID, service.CreateParams{
		ContentMD:     req.ContentMD,
		Location:      req.Location,
		PublishedAt:   req.PublishedAt,
		Source:        source,
		Client:        req.Client,
		AttachmentIDs: req.AttachmentIDs,
		Blocks:        req.Blocks,
	})
	if err != nil {
		return apiError(c, http.StatusBadRequest, "create_failed", err.Error())
	}
	if key := c.Request().Header.Get("Idempotency-Key"); key != "" {
		if err := s.q.InsertIdempotencyKey(c.Request().Context(), repo.InsertIdempotencyKeyParams{
			Key: key, PostID: dto.ID, CreatedAt: time.Now().Unix(),
		}); err != nil {
			slog.Warn("insert idempotency key failed", "key", key, "err", err)
		}
	}
	return c.JSON(http.StatusCreated, dto)
}

func (s *Server) getPost(c echo.Context) error {
	dto, err := s.posts.Get(c.Request().Context(), c.Param("id"))
	if errors.Is(err, service.ErrNotFound) {
		return apiError(c, http.StatusNotFound, "not_found", "帖子不存在")
	}
	if err != nil {
		return s.internalError(c, err)
	}
	return c.JSON(http.StatusOK, dto)
}

type updatePostReq struct {
	ContentMD   *string              `json:"content_md"`
	Location    *string              `json:"location"`
	PublishedAt *int64               `json:"published_at"`
	Blocks      *[]service.PostBlock `json:"blocks"`
}

func (s *Server) updatePost(c echo.Context) error {
	var req updatePostReq
	if err := c.Bind(&req); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求格式错误")
	}
	dto, err := s.posts.Update(c.Request().Context(), service.UpdateParams{
		ID:          c.Param("id"),
		ContentMD:   req.ContentMD,
		Location:    req.Location,
		PublishedAt: req.PublishedAt,
		Blocks:      req.Blocks,
	})
	if errors.Is(err, service.ErrNotFound) {
		return apiError(c, http.StatusNotFound, "not_found", "帖子不存在")
	}
	if err != nil {
		return apiError(c, http.StatusBadRequest, "update_failed", err.Error())
	}
	return c.JSON(http.StatusOK, dto)
}

func (s *Server) deletePost(c echo.Context) error {
	err := s.posts.Delete(c.Request().Context(), c.Param("id"), s.store, s.localStore)
	if errors.Is(err, service.ErrNotFound) {
		return apiError(c, http.StatusNotFound, "not_found", "帖子不存在")
	}
	if err != nil {
		return s.internalError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// listTags 标签聚合（含计数）。
func (s *Server) listTags(c echo.Context) error {
	rows, err := s.q.ListTags(c.Request().Context())
	if err != nil {
		return s.internalError(c, err)
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, map[string]any{"id": r.ID, "name": r.Name, "post_count": r.PostCount})
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

// uploadAttachment multipart 上传（字段名 file）。
func (s *Server) uploadAttachment(c echo.Context) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "缺少 file 字段")
	}
	att, err := s.uploader.Upload(c.Request().Context(), fh)
	if errors.Is(err, media.ErrTooLarge) {
		return apiError(c, http.StatusRequestEntityTooLarge, "too_large", err.Error())
	}
	if err != nil {
		return s.internalError(c, err)
	}
	return c.JSON(http.StatusCreated, service.AttachmentToDTO(*att, s.posts))
}

// parseCursor 解析复合游标 "published_at.ulid"；兼容纯时间戳旧格式。
func parseCursor(raw string) (int64, string) {
	if raw == "" {
		return 0, ""
	}
	ts, id, found := strings.Cut(raw, ".")
	n, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || n <= 0 {
		return 0, ""
	}
	if !found {
		return n, ""
	}
	return n, id
}

func strconvAtoiDefault(s string, def int) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return def
		}
		n = n*10 + int(ch-'0')
	}
	if s == "" {
		return def
	}
	return n
}

// parseFilterTime 解析筛选时间参数；缺省为 0，非法时写入 400 并返回 false。
func parseFilterTime(c echo.Context, name string) (int64, bool) {
	raw := strings.TrimSpace(c.QueryParam(name))
	if raw == "" {
		return 0, true
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		_ = apiError(c, http.StatusBadRequest, "bad_request", name+" 必须是 unix 秒")
		return 0, false
	}
	return v, true
}

// parseTagFilter 支持 JSON 数组（前端精确传递含逗号的标签）与逗号分隔（兼容 API 调用）。
func parseTagFilter(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if len(raw) > 4096 {
		return nil, errors.New("tags 参数过长")
	}
	var values []string
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return nil, errors.New("tags 不是合法的 JSON 数组")
		}
	} else {
		values = strings.Split(raw, ",")
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, tag := range values {
		tag = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tag), "#"))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	if len(out) > 20 {
		return nil, errors.New("一次最多筛选 20 个标签")
	}
	return out, nil
}

// searchPosts 全文搜索：GET /search?q=...（≥3 字符走 FTS5 trigram，短词 LIKE）。
func (s *Server) searchPosts(c echo.Context) error {
	limit := strconvAtoiDefault(c.QueryParam("limit"), 20)
	items, err := s.posts.Search(c.Request().Context(), c.QueryParam("q"), limit)
	if err != nil {
		return s.internalError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}
