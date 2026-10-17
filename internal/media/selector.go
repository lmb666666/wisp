package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

// Selector 可切换的存储前端：主驱动可动态更换（local/s3/lsky），
// 开启镜像时主驱动写入成功后再落一份本地副本（数据主权兜底）。
type Selector struct {
	mu      sync.RWMutex
	primary Storage
	mirror  bool
	local   *LocalStorage
}

func NewSelector(local *LocalStorage) *Selector {
	return &Selector{primary: local, local: local}
}

// Configure 切换主驱动与镜像开关。
func (s *Selector) Configure(primary Storage, mirror bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if primary == nil {
		primary = s.local
	}
	s.primary = primary
	s.mirror = mirror
}

// Primary 当前主驱动（设置页显示用）。
func (s *Selector) Primary() Storage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.primary
}

func (s *Selector) isLocal(p Storage) bool {
	lp, ok := p.(*LocalStorage)
	return ok && lp == s.local
}

// Put 主驱动写入；镜像开启且主驱动非本地时，经临时文件落一份本地副本。
func (s *Selector) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	s.mu.RLock()
	primary, mirror := s.primary, s.mirror
	s.mu.RUnlock()

	if !mirror || s.isLocal(primary) {
		return primary.Put(ctx, key, r, contentType)
	}

	// 主存储需两遍读取（主 + 本地镜像）：落临时文件
	tmp, err := os.CreateTemp("", "wisp-mirror-*")
	if err != nil {
		return fmt.Errorf("mirror temp: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	defer func() { _ = tmp.Close() }()
	if _, err := io.Copy(tmp, r); err != nil {
		return fmt.Errorf("mirror spool: %w", err)
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := primary.Put(ctx, key, tmp, contentType); err != nil {
		return err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return s.local.Put(ctx, key, tmp, contentType)
}

func (s *Selector) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	s.mu.RLock()
	primary := s.primary
	s.mu.RUnlock()
	r, err := primary.Get(ctx, key)
	if err == nil {
		return r, nil
	}
	if !s.isLocal(primary) {
		// 本地可能存有视频封面或镜像副本，作为兜底读取
		return s.local.Get(ctx, key)
	}
	return nil, err
}

func (s *Selector) Delete(ctx context.Context, key string) error {
	s.mu.RLock()
	primary, mirror := s.primary, s.mirror
	s.mu.RUnlock()

	if err := primary.Delete(ctx, key); err != nil {
		return err
	}
	if mirror && !s.isLocal(primary) {
		_ = s.local.Delete(ctx, key) // 镜像副本 best-effort
	}
	return nil
}

func (s *Selector) PublicURL(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.primary.PublicURL(key)
}
