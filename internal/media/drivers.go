package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Storage 接口的 Put 带 contentType（S3 需要正确 MIME 且不能从 reader 探测——会消耗流）。

// S3Config S3 兼容对象存储配置（AWS S3 / MinIO / R2 / OSS 等）。
type S3Config struct {
	Endpoint  string `json:"endpoint"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	UseSSL    bool   `json:"use_ssl"`
	// BaseURL 对象的公网访问前缀（bucket 需公读或挂 CDN）；为空时附件退化为 key。
	BaseURL string `json:"base_url"`
}

// S3Storage S3 兼容驱动（minio-go）。
type S3Storage struct {
	client  *minio.Client
	bucket  string
	baseURL string
}

// NewS3Storage 创建并验证 bucket 可达。
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("s3 必须配置公网访问前缀 base_url（bucket 公读或 CDN）")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}
	ok, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("s3 bucket check: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("s3 bucket %q 不存在", cfg.Bucket)
	}
	return &S3Storage{client: client, bucket: cfg.Bucket, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

func (s *S3Storage) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, -1,
		minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("s3 get: %w", err)
	}
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, fmt.Errorf("s3 stat: %w", err)
	}
	return obj, nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *S3Storage) PublicURL(key string) string {
	if s.baseURL != "" {
		return s.baseURL + "/" + key
	}
	return key
}

// ---------- Lsky Pro（兰空图床）V2 API ----------

// LskyConfig 兰空 V2 配置：base 形如 https://img.example.com，token 为个人令牌。
type LskyConfig struct {
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
}

// LskyStorage 兰空图床驱动：上传拿外链。
// V2 无公开的按 key 删除接口，Delete 为 best-effort no-op（清理靠图床自身策略）。
type LskyStorage struct {
	baseURL string
	token   string
	hc      *http.Client
	mu      sync.RWMutex
	urls    map[string]string // key → 远端 URL（进程内缓存；持久化以 attachments.remote_url 为准）
}

func NewLskyStorage(cfg LskyConfig) (*LskyStorage, error) {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if !strings.HasPrefix(cfg.BaseURL, "http") || cfg.Token == "" {
		return nil, fmt.Errorf("lsky 配置不完整：需要 base_url 与 token")
	}
	return &LskyStorage{
		baseURL: cfg.BaseURL,
		token:   cfg.Token,
		hc:      &http.Client{Timeout: 60 * time.Second},
		urls:    map[string]string{},
	}, nil
}

func (l *LskyStorage) Put(ctx context.Context, key string, r io.Reader, _ string) error {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", key)
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, r); err != nil {
		return fmt.Errorf("lsky read upload: %w", err)
	}
	if err := mw.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.baseURL+"/api/v1/upload", body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+l.token)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := l.hc.Do(req)
	if err != nil {
		return fmt.Errorf("lsky upload: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	url := extractLskyURL(data)
	if resp.StatusCode != http.StatusOK || url == "" {
		return fmt.Errorf("lsky 上传失败 (HTTP %d): %s", resp.StatusCode, truncateForLog(data))
	}
	l.mu.Lock()
	l.urls[key] = url
	l.mu.Unlock()
	return nil
}

func (l *LskyStorage) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	// 兰空返回的是外链，胶囊导出会直接按 attachments.remote_url 下载
	return nil, fmt.Errorf("lsky: 请通过 remote_url 导出")
}

func (l *LskyStorage) Delete(_ context.Context, _ string) error {
	return nil // best-effort no-op
}

func (l *LskyStorage) PublicURL(key string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.urls[key]
}

// extractLskyURL 从兰空 JSON 响应提取 data.url（status=false 返回空）。
func extractLskyURL(data []byte) string {
	var resp struct {
		Status bool `json:"status"`
		Data   struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil || !resp.Status {
		return ""
	}
	return resp.Data.URL
}

func truncateForLog(b []byte) string {
	if len(b) > 200 {
		return string(b[:200])
	}
	return string(b)
}
