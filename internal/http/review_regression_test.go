package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lmb666666/wisp/internal/config"
)

func initOwner(t *testing.T, base string, owner *http.Client) {
	t.Helper()
	if code, body := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: %d %s", code, body)
	}
}

func postWithIdempotencyKey(client *http.Client, url, key, content string) (int, error) {
	body, _ := json.Marshal(map[string]string{"content_md": content})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// 并发同 key 只能产生一条帖子；此前 check-then-create 会让两个请求都成功。
func TestConcurrentIdempotencyKey(t *testing.T) {
	base, owner, _ := newTestServer(t)
	initOwner(t, base, owner)

	const rounds = 20
	for i := 0; i < rounds; i++ {
		content := fmt.Sprintf("idem-%d", i)
		key := fmt.Sprintf("key-%d", i)
		start := make(chan struct{})
		statuses := make([]int, 2)
		var wg sync.WaitGroup
		for slot := 0; slot < 2; slot++ {
			wg.Add(1)
			go func(slot int) {
				defer wg.Done()
				<-start
				status, err := postWithIdempotencyKey(owner, base+"/api/v1/posts", key, content)
				if err != nil {
					t.Errorf("post %s: %v", content, err)
					return
				}
				statuses[slot] = status
			}(slot)
		}
		close(start)
		wg.Wait()
		for _, status := range statuses {
			if status != http.StatusCreated && status != http.StatusOK {
				t.Fatalf("%s status = %d", content, status)
			}
		}
	}

	code, body := ownerJSON(owner, "GET", base+"/api/v1/posts?limit=50", nil)
	if code != http.StatusOK {
		t.Fatalf("list: %d %s", code, body)
	}
	var list struct {
		Items []struct {
			ContentMD string `json:"content_md"`
		} `json:"items"`
	}
	_ = json.Unmarshal(body, &list)
	counts := map[string]int{}
	for _, item := range list.Items {
		counts[item.ContentMD]++
	}
	for i := 0; i < rounds; i++ {
		content := fmt.Sprintf("idem-%d", i)
		if counts[content] != 1 {
			t.Fatalf("%s appeared %d times, want 1", content, counts[content])
		}
	}
}

// 并发上传相同内容只能落一条无主附件。
func TestConcurrentUploadDedupe(t *testing.T) {
	base, owner, _ := newTestServer(t)
	initOwner(t, base, owner)

	const rounds = 20
	for i := 0; i < rounds; i++ {
		data := pngBytesN(t, 20+i)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for slot := 0; slot < 2; slot++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				code, body := uploadFile(t, owner, base+"/api/v1/attachments", "file",
					fmt.Sprintf("u-%d-%d.png", i, slot), data)
				if code != http.StatusCreated {
					t.Errorf("upload: %d %v", code, body)
				}
			}()
		}
		close(start)
		wg.Wait()
	}

	code, body := ownerJSON(owner, "GET", base+"/api/v1/admin/media?limit=1", nil)
	if code != http.StatusOK {
		t.Fatalf("media: %d %s", code, body)
	}
	var media struct {
		Total int64 `json:"total"`
	}
	_ = json.Unmarshal(body, &media)
	if media.Total != rounds {
		t.Fatalf("media total = %d, want %d (duplicate attachments)", media.Total, rounds)
	}
}

// 统计按客户端时区分日，而不是容器/服务端 UTC。
func TestStatsDailyUsesClientTimezone(t *testing.T) {
	base, owner, _ := newTestServer(t)
	initOwner(t, base, owner)

	ts := time.Date(2026, 11, 5, 0, 30, 0, 0, time.FixedZone("CST", 8*3600)).Unix()
	if code, body := ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{
		"content_md": "tz probe", "published_at": ts,
	}); code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}

	query := func(offset int) string {
		code, body := ownerJSON(owner, "GET",
			base+"/api/v1/stats/daily?year=2026&tz_offset="+strconv.Itoa(offset), nil)
		if code != http.StatusOK {
			t.Fatalf("stats: %d %s", code, body)
		}
		var resp struct {
			Items []struct {
				Day   string `json:"day"`
				Count int64  `json:"count"`
			} `json:"items"`
		}
		_ = json.Unmarshal(body, &resp)
		for _, item := range resp.Items {
			if item.Count > 0 {
				return item.Day
			}
		}
		return ""
	}
	if got := query(480); got != "2026-11-05" {
		t.Errorf("+08 day = %q, want 2026-11-05", got)
	}
	if got := query(0); got != "2026-11-04" {
		t.Errorf("UTC day = %q, want 2026-11-04", got)
	}
}

// 登录/首启/Passkey 登录必须返回完整身份，前端才能立即显示 owner 入口。
func TestAuthResponsesIncludeIdentity(t *testing.T) {
	base, owner, _ := newTestServer(t)
	code, body := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"})
	if code != http.StatusCreated {
		t.Fatalf("init: %d %s", code, body)
	}
	var initResp map[string]any
	_ = json.Unmarshal(body, &initResp)
	if initResp["is_owner"] != true || initResp["username"] != "liang" {
		t.Fatalf("init response = %v", initResp)
	}

	code, body = ownerJSON(owner, "POST", base+"/api/v1/auth/login",
		map[string]string{"username": "liang", "password": "password123"})
	if code != http.StatusOK {
		t.Fatalf("login: %d %s", code, body)
	}
	var loginResp map[string]any
	_ = json.Unmarshal(body, &loginResp)
	if loginResp["is_owner"] != true || loginResp["via_pat"] != false {
		t.Fatalf("login response = %v", loginResp)
	}
}

// passkey 未启用时 finish 应返回 501，而不是 nil pointer panic。
func TestPasskeyFinishDisabled(t *testing.T) {
	base, _, guest := newTestServer(t)
	code, _ := ownerJSON(guest, "POST", base+"/api/v1/auth/passkey/login/finish?session=x",
		map[string]any{})
	if code != http.StatusNotImplemented {
		t.Fatalf("finish status = %d, want 501", code)
	}
}

func TestAbsoluteMediaURL(t *testing.T) {
	s := &Server{cfg: config.Config{PublicURL: "http://test.local/"}}
	if got := s.absoluteMediaURL("https://cdn.example/x.jpg"); got != "https://cdn.example/x.jpg" {
		t.Fatalf("absolute URL changed: %q", got)
	}
	if got := s.absoluteMediaURL("/media/x.jpg"); got != "http://test.local/media/x.jpg" {
		t.Fatalf("relative URL = %q", got)
	}
}

func TestSecurityHeadersAndReadiness(t *testing.T) {
	base, _, guest := newTestServer(t)
	resp, err := guest.Get(base + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("readyz = %d", resp.StatusCode)
	}
	if resp.Header.Get("Referrer-Policy") == "" ||
		resp.Header.Get("Content-Security-Policy") == "" {
		t.Fatalf("security headers missing: %v", resp.Header)
	}
}
