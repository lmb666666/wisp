package http

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

// formFileError 把 multipart 解析失败映射成可操作的响应。
// Go 的 ReadTimeout 会在慢上传中途截断 body，之前统一报“缺少 file 字段”会误导用户。
func formFileError(c echo.Context, err error) error {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) ||
		(errors.As(err, &netErr) && netErr.Timeout()) {
		return apiError(c, http.StatusRequestTimeout, "timeout", "上传超时，请检查网络后重试")
	}
	return apiError(c, http.StatusBadRequest, "bad_request", "上传中断或缺少 file 字段")
}
