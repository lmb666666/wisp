// Package media 提供附件存储抽象（local / S3 / 图床可插拔）与上传处理。
package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Storage 存储驱动接口：v0.1 实现 local；v0.2 增加 S3 与 Lsky Pro 图床。
// 语义约定：Put 后 PublicURL 必须可被公网（或本机）访问。
type Storage interface {
	Put(ctx context.Context, key string, r io.Reader, contentType string) error
	// Get 读取原始文件（胶囊导出 / 归档用）。
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	// PublicURL 返回可直接访问的 URL（本地驱动为相对路径 /media/<key>）。
	PublicURL(key string) string
}

// LocalStorage 本地磁盘驱动，root 一般为 <data>/media。
type LocalStorage struct {
	root string
}

func NewLocalStorage(root string) (*LocalStorage, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create media root: %w", err)
	}
	return &LocalStorage{root: root}, nil
}

// Root 返回磁盘根目录（HTTP 静态路由挂载用）。
func (l *LocalStorage) Root() string { return l.root }

func (l *LocalStorage) Put(_ context.Context, key string, r io.Reader, _ string) error {
	dst := filepath.Join(l.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (l *LocalStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	f, err := os.Open(filepath.Join(l.root, filepath.FromSlash(key)))
	if err != nil {
		return nil, fmt.Errorf("open media: %w", err)
	}
	return f, nil
}

func (l *LocalStorage) Delete(_ context.Context, key string) error {
	err := os.Remove(filepath.Join(l.root, filepath.FromSlash(key)))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (l *LocalStorage) PublicURL(key string) string { return "/media/" + key }
