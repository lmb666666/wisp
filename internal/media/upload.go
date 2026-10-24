package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/buckket/go-blurhash"
	_ "golang.org/x/image/webp"

	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/util"
)

const (
	maxUploadSize = 32 << 20 // 32MB
	thumbLongEdge = 640
)

var ErrTooLarge = errors.New("文件超过大小限制")

// Uploader 处理上传：sha256 秒传、类型探测、尺寸/blurhash/缩略图。
// 图片派生物在写入存储前从上传流直接解码——与具体 Storage 驱动无关。
type Uploader struct {
	store Storage
	q     repo.Querier
}

func NewUploader(store Storage, q repo.Querier) *Uploader {
	return &Uploader{store: store, q: q}
}

func kindOf(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	default:
		return "file"
	}
}

type derivedImage struct {
	width, height int
	blurhash      string
	thumb         []byte // JPEG 640px，nil 表示无
}

// Upload 保存一个 multipart 文件并落库；同 sha256 且尚未绑定帖子时复用（秒传）。
func (u *Uploader) Upload(ctx context.Context, fh *multipart.FileHeader) (*repo.Attachment, error) {
	if fh.Size > maxUploadSize {
		return nil, ErrTooLarge
	}
	f, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("open upload: %w", err)
	}
	defer f.Close()
	return u.uploadReader(ctx, fh.Filename, fh.Size, fh.Header.Get("Content-Type"), f)
}

// UploadBytes 保存内存中的文件（导入器迁移媒体时使用）。
func (u *Uploader) UploadBytes(ctx context.Context, filename string, data []byte, mimeHint string) (*repo.Attachment, error) {
	if int64(len(data)) > maxUploadSize {
		return nil, ErrTooLarge
	}
	return u.uploadReader(ctx, filename, int64(len(data)), mimeHint, bytes.NewReader(data))
}

// uploadReader 共用上传管线：sha256 秒传（仅无主附件）→ 类型探测 → 图片派生物 → 存储 → 落库。
func (u *Uploader) uploadReader(ctx context.Context, filename string, size int64, mimeHint string, f io.ReadSeeker) (*repo.Attachment, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return nil, fmt.Errorf("hash upload: %w", err)
	}
	sum := hex.EncodeToString(hasher.Sum(nil))

	if existing, err := u.q.GetAttachmentBySHA256(ctx, sum); err == nil {
		return &existing, nil // 秒传：仅复用未绑定帖子的附件
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind upload: %w", err)
	}

	// 类型：优先客户端声明，探测兜底
	mime := mimeHint
	if mime == "" || mime == "application/octet-stream" {
		head := make([]byte, 512)
		n, _ := io.ReadFull(f, head)
		mime = http.DetectContentType(head[:n])
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("rewind upload: %w", err)
		}
	}
	kind := kindOf(mime)
	ext := normalizeExt(path.Ext(filename), mime)

	// 图片派生物：从上传流直接解码（Put 之前，存储无关）
	var derived derivedImage
	var err error
	if kind == "image" {
		derived, err = deriveFrom(f)
		if err != nil {
			derived = derivedImage{} // 派生失败不阻塞上传
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("rewind upload: %w", err)
		}
	}

	id := util.NewULID()
	now := time.Now()
	key := fmt.Sprintf("%04d/%02d/%s%s", now.Year(), int(now.Month()), id, ext)

	if err := u.store.Put(ctx, key, f, mime); err != nil {
		return nil, fmt.Errorf("store upload: %w", err)
	}
	thumbPath := ""
	if derived.thumb != nil {
		thumbKey := key + ".thumb.jpg"
		if err := u.store.Put(ctx, thumbKey, bytes.NewReader(derived.thumb), "image/jpeg"); err == nil {
			thumbPath = thumbKey
		}
	}

	row, err := u.q.CreateAttachment(ctx, repo.CreateAttachmentParams{
		ID:         id,
		Kind:       kind,
		Mime:       mime,
		Size:       size,
		Width:      int64(derived.width),
		Height:     int64(derived.height),
		Blurhash:   derived.blurhash,
		ThumbPath:  thumbPath,
		Storage:    u.storeKind(),
		StorageKey: key,
		RemoteUrl:  u.remoteURL(key),
		Sha256:     sum,
		CreatedAt:  now.Unix(),
	})
	if err != nil {
		_ = u.store.Delete(ctx, key)
		return nil, fmt.Errorf("insert attachment: %w", err)
	}
	return &row, nil
}

// maxImagePixels 图片解码上限：64MP（如 8000×8000）。超出直接拒绝，
// 防 decompression bomb（头部声明超大尺寸 → image.Decode 数 GB 内存分配）。
const maxImagePixels = 64 << 20

// deriveFrom 从上传流解码图片：原始尺寸 + blurhash + 640px JPEG 缩略图。
func deriveFrom(r io.ReadSeeker) (derivedImage, error) {
	// 先读头部尺寸，超限拒码（不进入全量解码）；DecodeConfig 会消耗字节，需回卷
	start, err := r.Seek(0, io.SeekStart)
	if err != nil {
		return derivedImage{}, fmt.Errorf("seek upload: %w", err)
	}
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return derivedImage{}, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return derivedImage{}, fmt.Errorf("图片尺寸超限（%dx%d，上限 64MP）", cfg.Width, cfg.Height)
	}
	if _, err := r.Seek(start, io.SeekStart); err != nil {
		return derivedImage{}, fmt.Errorf("rewind upload: %w", err)
	}
	img, _, err := image.Decode(r)
	if err != nil {
		return derivedImage{}, fmt.Errorf("decode image: %w", err)
	}
	bounds := img.Bounds()
	d := derivedImage{width: bounds.Dx(), height: bounds.Dy()}

	// blurhash：降到宽 ≤32px 编码，足够生成占位色块
	if enc, err := blurhash.Encode(4, 3, resize(img, 32)); err == nil {
		d.blurhash = enc
	}

	// 缩略图：长边 ≤640 的 JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resize(img, thumbLongEdge), &jpeg.Options{Quality: 82}); err != nil {
		return d, fmt.Errorf("encode thumb: %w", err)
	}
	d.thumb = buf.Bytes()
	return d, nil
}

// storeKind / remoteURL：主驱动元信息（供附件行记录来源）。
func (u *Uploader) storeKind() string {
	switch s := u.store.(type) {
	case *Selector:
		if _, ok := s.Primary().(*LocalStorage); ok {
			return "local"
		}
		return "remote" // 具体类型由 remote_url 体现
	case *LocalStorage:
		return "local"
	default:
		return "remote"
	}
}

func (u *Uploader) remoteURL(key string) string {
	url := u.store.PublicURL(key)
	// 只认绝对地址；S3 未配置公网前缀时 PublicURL 返回裸 key，不能当 URL 用
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url
	}
	return ""
}
