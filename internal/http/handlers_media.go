package http

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/repo"
)

const maxPosterSize = 5 << 20 // 5MB

// posterKey 视频封面固定写本地；按附件 ID 命名，避免空 storage_key 的远端附件互相覆盖。
func posterKey(attachmentID string) string {
	return "posters/" + attachmentID + ".jpg"
}

// uploadPoster 视频封面（客户端截帧上传）：POST /attachments/:id/poster (multipart file)。
func (s *Server) uploadPoster(c echo.Context) error {
	att, err := s.q.GetAttachment(c.Request().Context(), c.Param("id"))
	if err != nil {
		return apiError(c, http.StatusNotFound, "not_found", "附件不存在")
	}
	if att.Kind != "video" {
		return apiError(c, http.StatusBadRequest, "bad_request", "只能为视频设置封面")
	}
	if s.localStore == nil {
		return apiError(c, http.StatusServiceUnavailable, "unavailable", "本地存储未启用")
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "缺少 file 字段")
	}
	if fh.Size > maxPosterSize {
		return apiError(c, http.StatusRequestEntityTooLarge, "too_large", "封面超过 5MB")
	}
	f, err := fh.Open()
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	defer f.Close()

	// 封面固定写本地存储：小文件，且保证 /media 路由始终可访问（主存储可能是 S3/图床）
	key := posterKey(att.ID)
	if err := s.localStore.Put(c.Request().Context(), key, f, "image/jpeg"); err != nil {
		return s.internalError(c, err)
	}
	if err := s.q.SetAttachmentPoster(c.Request().Context(), repo.SetAttachmentPosterParams{
		PosterPath: key, ID: att.ID,
	}); err != nil {
		_ = s.localStore.Delete(c.Request().Context(), key)
		return s.internalError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"poster_url": "/media/" + key})
}
