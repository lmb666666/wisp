package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type memStorage struct {
	mu     sync.Mutex
	files  map[string][]byte
	prefix string
}

func newMemStorage(prefix string) *memStorage {
	return &memStorage{files: map[string][]byte{}, prefix: prefix}
}

func (m *memStorage) Put(_ context.Context, key string, r io.Reader, _ string) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.files[key] = b
	m.mu.Unlock()
	return nil
}

func (m *memStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *memStorage) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.files, key)
	m.mu.Unlock()
	return nil
}

func (m *memStorage) PublicURL(key string) string { return m.prefix + key }

func readLocal(t *testing.T, local *LocalStorage, key string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(local.Root(), filepath.FromSlash(key)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSelectorMirrorsRemoteWritesLocally(t *testing.T) {
	local, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	remote := newMemStorage("https://cdn.example/")
	sel := NewSelector(local)
	sel.Configure(remote, true)

	if err := sel.Put(context.Background(), "2026/11/a.jpg", bytes.NewReader([]byte("jpeg-bytes")), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	if got := string(remote.files["2026/11/a.jpg"]); got != "jpeg-bytes" {
		t.Fatalf("远端内容 = %q", got)
	}
	if got := string(readLocal(t, local, "2026/11/a.jpg")); got != "jpeg-bytes" {
		t.Fatalf("本地镜像内容 = %q", got)
	}
	if got := sel.PublicURL("2026/11/a.jpg"); got != "https://cdn.example/2026/11/a.jpg" {
		t.Fatalf("PublicURL = %q", got)
	}
}

func TestSelectorWithoutMirrorLeavesNoLocalCopy(t *testing.T) {
	local, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sel := NewSelector(local)
	sel.Configure(newMemStorage("https://cdn.example/"), false)

	if err := sel.Put(context.Background(), "2026/11/b.jpg", bytes.NewReader([]byte("x")), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	if _, err := local.Get(context.Background(), "2026/11/b.jpg"); err == nil {
		t.Fatal("未开启镜像时不应写本地副本")
	}
}

func TestSelectorGetFallsBackToLocal(t *testing.T) {
	local, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := local.Put(context.Background(), "thumbs/poster.jpg", bytes.NewReader([]byte("poster")), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	sel := NewSelector(local)
	sel.Configure(newMemStorage("https://cdn.example/"), false)

	rc, err := sel.Get(context.Background(), "thumbs/poster.jpg")
	if err != nil {
		t.Fatalf("远端缺失时应回退本地: %v", err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if string(b) != "poster" {
		t.Fatalf("回退读取内容 = %q", b)
	}
}

func TestSelectorDeleteRemovesMirror(t *testing.T) {
	local, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	remote := newMemStorage("https://cdn.example/")
	sel := NewSelector(local)
	sel.Configure(remote, true)

	if err := sel.Put(context.Background(), "2026/11/c.jpg", bytes.NewReader([]byte("x")), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	if err := sel.Delete(context.Background(), "2026/11/c.jpg"); err != nil {
		t.Fatal(err)
	}
	if _, ok := remote.files["2026/11/c.jpg"]; ok {
		t.Fatal("远端文件未删除")
	}
	if _, err := local.Get(context.Background(), "2026/11/c.jpg"); err == nil {
		t.Fatal("本地镜像未删除")
	}
}

func TestSelectorConfigureNilFallsBackToLocal(t *testing.T) {
	local, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sel := NewSelector(local)
	sel.Configure(nil, false)

	if sel.Primary() != local {
		t.Fatal("primary 应回退到 local")
	}
	if err := sel.Put(context.Background(), "k", bytes.NewReader([]byte("v")), "text/plain"); err != nil {
		t.Fatal(err)
	}
	if got := string(readLocal(t, local, "k")); got != "v" {
		t.Fatalf("本地内容 = %q", got)
	}
}
