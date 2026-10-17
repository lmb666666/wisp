package media

import (
	"image"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// resize 等比缩放到长边 ≤ maxEdge（不放大），用于 blurhash 编码与缩略图。
func resize(src image.Image, maxEdge int) image.Image {
	w := src.Bounds().Dx()
	h := src.Bounds().Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	scale := min(1, float64(maxEdge)/float64(max(w, h)))
	tw := max(1, int(float64(w)*scale))
	th := max(1, int(float64(h)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

// normalizeExt 规范化存储扩展名：白名单内保留，否则按 MIME 推断。
func normalizeExt(ext, mime string) string {
	allowed := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
		".mp4": true, ".webm": true, ".mov": true,
		".mp3": true, ".m4a": true, ".ogg": true, ".wav": true,
		".pdf": true, ".zip": true,
	}
	ext = strings.ToLower(ext)
	if allowed[ext] {
		return ext
	}
	byMime := map[string]string{
		"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif",
		"video/mp4": ".mp4", "video/webm": ".webm", "video/quicktime": ".mov",
		"audio/mpeg": ".mp3", "audio/mp4": ".m4a", "audio/ogg": ".ogg", "audio/wav": ".wav",
		"application/pdf": ".pdf", "application/zip": ".zip",
	}
	if e, ok := byMime[mime]; ok {
		return e
	}
	return ".bin"
}
