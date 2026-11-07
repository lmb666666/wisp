package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/util"

	wispdb "github.com/lmb666666/wisp/internal/db"
)

func newTestDispatcher(t *testing.T, url, secret string) (*Dispatcher, repo.Querier) {
	t.Helper()
	dir := t.TempDir()
	database, err := wispdb.Open(filepath.Join(dir, "webhook.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := wispdb.Migrate(database, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	q := repo.New(database)
	if _, err := q.CreateWebhook(context.Background(), repo.CreateWebhookParams{
		ID: util.NewULID(), Url: url, Secret: secret,
		Events: "post.created,post.updated", CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	return NewDispatcher(q), q
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}

func TestDispatcherRetriesWithSignature(t *testing.T) {
	const secret = "s3cret"
	var hits int32
	var badSignature int32
	var badEvent int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if r.Header.Get("X-Wisp-Event") != "post.created" {
			atomic.StoreInt32(&badEvent, 1)
		}
		body, _ := io.ReadAll(r.Body)
		if got, want := r.Header.Get("X-Wisp-Signature"), "sha256="+Sign(secret, body); got != want {
			atomic.StoreInt32(&badSignature, 1)
		}
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	oldBackoff := retryBackoff
	retryBackoff = []time.Duration{5 * time.Millisecond, 5 * time.Millisecond}
	defer func() { retryBackoff = oldBackoff }()

	d, _ := newTestDispatcher(t, srv.URL, secret)
	d.Start()
	d.Emit("post.created", map[string]string{"id": "p1"})
	waitFor(t, 2*time.Second, func() bool { return atomic.LoadInt32(&hits) >= 3 })
	d.Stop()
	d.Wait()

	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("投递次数 = %d, want 3（两次失败后成功，不应继续重试）", got)
	}
	if atomic.LoadInt32(&badSignature) != 0 {
		t.Fatal("HMAC 签名不匹配")
	}
	if atomic.LoadInt32(&badEvent) != 0 {
		t.Fatal("X-Wisp-Event 头不正确")
	}
}

func TestStopInterruptsRetryBackoff(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	oldBackoff := retryBackoff
	retryBackoff = []time.Duration{time.Hour} // 若 Stop 不能打断退避，测试会超时
	defer func() { retryBackoff = oldBackoff }()

	d, _ := newTestDispatcher(t, srv.URL, "s")
	d.Start()
	d.Emit("post.created", nil)
	waitFor(t, 2*time.Second, func() bool { return atomic.LoadInt32(&hits) >= 1 })

	start := time.Now()
	d.Stop()
	d.Wait()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Stop 未打断退避等待，耗时 %s", elapsed)
	}
}

func TestSignIsStableHMAC(t *testing.T) {
	// HMAC-SHA256("secret", "payload") 的固定向量，防止签名算法被无意改动。
	const want = "b82fcb791acec57859b989b430a826488ce2e479fdf92326bd0a2e8375a42ba4"
	if got := Sign("secret", []byte("payload")); got != want {
		t.Fatalf("Sign = %q, want %q", got, want)
	}
}

func TestMaskURLDropsCredentialsAndQuery(t *testing.T) {
	got := maskURL("https://user:pass@example.com/hook?token=secret#frag")
	if got != "https://example.com/hook" {
		t.Fatalf("maskURL = %q", got)
	}
	if len(maskURL("https://example.com/"+string(make([]byte, 500)))) > maxPayloadLog {
		t.Fatal("超长 URL 未截断")
	}
}
