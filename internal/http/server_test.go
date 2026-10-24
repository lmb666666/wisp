package http

import (
	"archive/zip"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lmb666666/wisp/internal/auth"
	"github.com/lmb666666/wisp/internal/config"
	wispdb "github.com/lmb666666/wisp/internal/db"
	"github.com/lmb666666/wisp/internal/media"
	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/service"
	"github.com/lmb666666/wisp/internal/webhook"
)

// newTestServer 起一个完整后端（内存外的临时目录 + 真实 SQLite）。
func newTestServer(t *testing.T) (baseURL string, owner *http.Client, guest *http.Client) {
	t.Helper()
	dir := t.TempDir()

	database, err := wispdb.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := wispdb.Migrate(database, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}

	q := repo.New(database)
	authSvc := auth.NewService(q)
	postSvc := service.NewPostService(database, q, "http://test.local", "")
	hooks := webhook.NewDispatcher(q)
	hooks.Start()
	postSvc.WithNotifier(hooks.Emit)
	store, err := media.NewLocalStorage(filepath.Join(dir, "media"))
	if err != nil {
		t.Fatal(err)
	}

	webFS := fstest.MapFS{"index.html": &fstest.MapFile{
		Data: []byte("<!doctype html><html><head><!--wisp:og--></head><body>W</body></html>"),
	}}

	cfg := config.Config{
		DataDir:      dir,
		DBPath:       filepath.Join(dir, "test.db"),
		PublicURL:    "http://test.local",
		APIRateLimit: 1000,
	}
	e, err := New(cfg, webFS, Deps{
		DB: database, Querier: q, Auth: authSvc, Posts: postSvc,
		Uploader: media.NewUploader(store, q), Storage: media.NewSelector(store),
		LocalStore: store, Webhooks: hooks,
		Passkeys: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(e)
	t.Cleanup(ts.Close)

	return ts.URL, newClient(), newClient()
}

func ownerJSON(c *http.Client, method, url string, body any) (int, []byte) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pngBytesN(t *testing.T, n int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func uploadFile(t *testing.T, client *http.Client, url, field, filename string, content []byte) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile(field, filename)
	_, _ = fw.Write(content)
	_ = w.Close()
	req, _ := http.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	return resp.StatusCode, m
}

// TestAPIFullFlow 按真实使用顺序走通 P1 全链路。
func TestAPIFullFlow(t *testing.T) {
	base, owner, guest := newTestServer(t)

	// --- 初始化与登录 ---
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: got %d", code)
	}
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "someone", "password": "password123"}); code != http.StatusConflict {
		t.Fatalf("re-init should conflict, got %d", code)
	}
	code, body := ownerJSON(guest, "GET", base+"/api/v1/auth/status", nil)
	if code != 200 || !strings.Contains(string(body), `"initialized":true`) {
		t.Fatalf("status: %d %s", code, body)
	}

	// --- 发帖与权限 ---
	if code, _ := ownerJSON(guest, "POST", base+"/api/v1/posts",
		map[string]string{"content_md": "hi"}); code != http.StatusUnauthorized {
		t.Fatalf("guest create should 401, got %d", code)
	}

	code, body = ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{
		"content_md": "#hello 这是**加粗**内容\n\n第二段 #测试",
		"visibility": "public",
	})
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	var post struct {
		ID          string   `json:"id"`
		ContentHTML string   `json:"content_html"`
		Tags        []string `json:"tags"`
		Source      string   `json:"source"`
	}
	_ = json.Unmarshal(body, &post)
	if !strings.Contains(post.ContentHTML, "<strong>加粗</strong>") {
		t.Errorf("html missing strong: %s", post.ContentHTML)
	}
	if len(post.Tags) != 2 || post.Tags[0] != "hello" {
		t.Errorf("tags = %v", post.Tags)
	}
	if post.Source != "web" {
		t.Errorf("source = %s", post.Source)
	}

	// 可见性已移除：游客与作者看到同一公开时间线
	if code, _ = ownerJSON(guest, "GET", base+"/api/v1/posts", nil); code != 200 {
		t.Fatalf("guest list: %d", code)
	}

	// --- 编辑（重渲染 + 重建标签）---
	code, body = ownerJSON(owner, "PATCH", base+"/api/v1/posts/"+post.ID,
		map[string]any{"content_md": "编辑后的内容 #新标签"})
	if code != 200 || !strings.Contains(string(body), "新标签") {
		t.Fatalf("patch: %d %s", code, body)
	}

	// --- PAT ---
	code, body = ownerJSON(owner, "POST", base+"/api/v1/auth/tokens",
		map[string]string{"name": "reader", "scopes": "read"})
	if code != http.StatusCreated {
		t.Fatalf("create read token: %d", code)
	}
	var tok struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(body, &tok)
	if !strings.HasPrefix(tok.Token, "wpat_") {
		t.Fatalf("token = %q", tok.Token)
	}

	bearer := func(token string) *http.Client {
		c := newClient()
		c.Transport = bearerTransport{token: token, base: http.DefaultTransport}
		return c
	}
	readC := bearer(tok.Token)

	// read 作用域可读，不可写
	code, body = ownerJSON(readC, "GET", base+"/api/v1/posts?limit=50", nil)
	if code != 200 || !strings.Contains(string(body), "编辑后的内容") {
		t.Errorf("pat read: %d body=%.200s", code, body)
	}
	if code, _ = ownerJSON(readC, "POST", base+"/api/v1/posts",
		map[string]string{"content_md": "x"}); code != http.StatusForbidden {
		t.Errorf("pat read create should 403, got %d", code)
	}

	_, body = ownerJSON(owner, "POST", base+"/api/v1/auth/tokens",
		map[string]string{"name": "writer", "scopes": "write"})
	var wtok struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(body, &wtok)
	code, body = ownerJSON(bearer(wtok.Token), "POST", base+"/api/v1/posts",
		map[string]string{"content_md": "PAT 发的帖"})
	if code != http.StatusCreated {
		t.Fatalf("pat write create: %d %s", code, body)
	}
	if !strings.Contains(string(body), `"source":"api"`) {
		t.Error("pat post source should be api")
	}

	// 无效令牌
	if code, _ = ownerJSON(bearer("wpat_bad"), "GET", base+"/api/v1/posts", nil); code != http.StatusUnauthorized {
		t.Errorf("invalid token should 401, got %d", code)
	}

	// --- 附件：上传 / 秒传 / 绑定 ---
	img := pngBytes(t)
	code, att := uploadFile(t, owner, base+"/api/v1/attachments", "file", "a.png", img)
	if code != http.StatusCreated {
		t.Fatalf("upload: %d %v", code, att)
	}
	attID, _ := att["id"].(string)
	attURL, _ := att["url"].(string)
	thumb, _ := att["thumb_url"].(string)
	if attID == "" || attURL == "" || thumb == "" {
		t.Fatalf("attachment fields = %v", att)
	}

	// 秒传：同内容再次上传应得到同一附件
	code, att2 := uploadFile(t, owner, base+"/api/v1/attachments", "file", "b.png", img)
	if code != http.StatusCreated || att2["id"] != attID {
		t.Errorf("dedupe failed: %v vs %v", att2["id"], attID)
	}

	// 绑定到新帖
	code, body = ownerJSON(owner, "POST", base+"/api/v1/posts",
		map[string]any{"content_md": "带图帖子", "attachment_ids": []string{attID}})
	if code != http.StatusCreated {
		t.Fatalf("post with attachment: %d %s", code, body)
	}
	var withAtt struct {
		ID          string `json:"id"`
		Attachments []struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		} `json:"attachments"`
	}
	_ = json.Unmarshal(body, &withAtt)
	if len(withAtt.Attachments) != 1 || withAtt.Attachments[0].ID != attID {
		t.Fatalf("attachments = %+v", withAtt.Attachments)
	}

	// 媒体可访问
	resp, err := guest.Get(base + attURL)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("media fetch: %v %v", err, resp)
	}
	resp.Body.Close()

	// OG 注入
	resp, _ = guest.Get(base + "/p/" + withAtt.ID)
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), `property="og:title"`) {
		t.Error("og:title missing")
	}
	if !strings.Contains(string(page), `property="og:image"`) || !strings.Contains(string(page), "/media/") {
		t.Error("post og:image missing")
	}
	resp, _ = guest.Get(base + "/")
	home, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(home), "/og.png") {
		t.Error("home og:image missing")
	}

	// RSS
	resp, _ = guest.Get(base + "/feed.xml")
	feed, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(feed), "<rss") || !strings.Contains(string(feed), "带图帖子") {
		t.Error("rss content missing")
	}

	// --- 全文搜索：FTS5（≥3 字符）与 LIKE（短词）---
	code, _ = ownerJSON(owner, "POST", base+"/api/v1/posts",
		map[string]string{"content_md": "FTS5 中文搜索测试 的第一条"})
	if code != http.StatusCreated {
		t.Fatalf("search seed post: %d", code)
	}

	searchCount := func(client *http.Client, q string) int {
		code, body = ownerJSON(client, "GET", base+"/api/v1/search?q="+urlQueryEscape(q), nil)
		if code != 200 {
			return -1
		}
		var sr struct {
			Items []json.RawMessage `json:"items"`
		}
		_ = json.Unmarshal(body, &sr)
		return len(sr.Items)
	}
	if n := searchCount(owner, "中文搜索"); n != 1 { // 4 字 → FTS5 trigram
		t.Errorf("fts5 search hits = %d, want 1", n)
	}
	if n := searchCount(owner, "第一条"); n < 1 { // 3 字 → FTS5
		t.Errorf("fts5 3-char search hits = %d, want >=1", n)
	}
	if n := searchCount(owner, "内容"); n < 1 { // 2 字 → LIKE 兜底
		t.Errorf("like search hits = %d, want >=1", n)
	}

	// --- 复合游标：同秒多帖不丢失、不重复 ---
	sameTs := time.Now().Unix()
	_, _ = ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{"content_md": "同秒A", "published_at": sameTs})
	_, _ = ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{"content_md": "同秒B", "published_at": sameTs})

	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 10; page++ {
		u := base + "/api/v1/posts?limit=2"
		if cursor != "" {
			u += "&before=" + url.QueryEscape(cursor)
		}
		code, body = ownerJSON(owner, "GET", u, nil)
		if code != 200 {
			t.Fatalf("cursor page: %d", code)
		}
		var pg struct {
			Items []struct {
				ID          string `json:"id"`
				ContentMD   string `json:"content_md"`
				PublishedAt int64  `json:"published_at"`
			} `json:"items"`
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		}
		_ = json.Unmarshal(body, &pg)
		for _, it := range pg.Items {
			if seen[it.ID] {
				t.Errorf("duplicate post across pages: %s", it.ID)
			}
			seen[it.ID] = true
		}
		if !pg.HasMore {
			break
		}
		cursor = pg.NextCursor
		if cursor == "" {
			t.Fatal("has_more without cursor")
		}
	}
	if len(seen) < 2 {
		t.Error("expected at least the two same-second posts across pages")
	}

	// --- Webhook：注册 → 发帖 → 送达 + 签名 ---
	deliveries := make(chan *http.Request, 4)
	bodies := make(chan []byte, 4)
	hookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies <- b
		deliveries <- r.Clone(r.Context())
		w.WriteHeader(200)
	}))
	defer hookSrv.Close()

	code, body = ownerJSON(owner, "POST", base+"/api/v1/admin/webhooks",
		map[string]string{"url": hookSrv.URL, "events": "post.created,post.deleted"})
	if code != http.StatusCreated {
		t.Fatalf("webhook create: %d %s", code, body)
	}
	var wh struct {
		Secret string `json:"secret"`
	}
	_ = json.Unmarshal(body, &wh)

	code, _ = ownerJSON(owner, "POST", base+"/api/v1/posts",
		map[string]string{"content_md": "webhook 触发帖"})
	if code != http.StatusCreated {
		t.Fatalf("post for webhook: %d", code)
	}

	select {
	case r := <-deliveries:
		if r.Header.Get("X-Wisp-Event") != "post.created" {
			t.Errorf("event header = %s", r.Header.Get("X-Wisp-Event"))
		}
		sig := strings.TrimPrefix(r.Header.Get("X-Wisp-Signature"), "sha256=")
		b := <-bodies
		mac := hmac.New(sha256.New, []byte(wh.Secret))
		mac.Write(b)
		got, _ := hex.DecodeString(sig)
		if !hmac.Equal(mac.Sum(nil), got) {
			t.Error("signature mismatch")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("webhook not delivered in 3s")
	}

	// --- 导入：memos JSON + 幂等去重 ---
	memosJSON := []byte(`[{"content":"从 Memos 迁移 #导入测试","createdTs":1754128800,"visibility":"PUBLIC"}]`)
	importOnce := func() (int, map[string]any) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("source", "memos")
		fw, _ := w.CreateFormFile("file", "memos.json")
		_, _ = fw.Write(memosJSON)
		_ = w.Close()
		req, _ := http.NewRequest("POST", base+"/api/v1/admin/import", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		resp, err := owner.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		return resp.StatusCode, m
	}
	code, m := importOnce()
	if code != 200 || m["imported"].(float64) != 1 {
		t.Fatalf("import: %d %v", code, m)
	}
	code, m = importOnce()
	if code != 200 || m["skipped"].(float64) != 1 || m["imported"].(float64) != 0 {
		t.Fatalf("re-import should dedupe: %d %v", code, m)
	}

	// --- 扩展卡片（blocks）：音乐走 Meting、位置独立存储、不写入正文 ---
	meting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("type") {
		case "song":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"name":"Test Song","artist":"Tester","pic":"https://img.example/cover.jpg"}]`))
		case "url":
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("fake-mp3"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer meting.Close()
	if code, _ := ownerJSON(owner, "PUT", base+"/api/v1/admin/settings/meting",
		map[string]string{"url": meting.URL}); code != 200 {
		t.Fatalf("set meting: %d", code)
	}

	code, body = ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{
		"content_md": "",
		"blocks": []map[string]string{
			{"type": "music", "url": "https://music.163.com/song?id=1868553"},
			{"type": "location", "name": "上海 · 徐汇"},
		},
	})
	if code != http.StatusCreated {
		t.Fatalf("block-only post: %d %s", code, body)
	}
	var bp struct {
		ID          string `json:"id"`
		ContentMD   string `json:"content_md"`
		ContentHTML string `json:"content_html"`
		Blocks      []struct {
			Type     string `json:"type"`
			Name     string `json:"name"`
			Title    string `json:"title"`
			Cover    string `json:"cover"`
			AudioURL string `json:"audio_url"`
		} `json:"blocks"`
	}
	_ = json.Unmarshal(body, &bp)
	if bp.ContentMD != "" || strings.Contains(bp.ContentHTML, "music.163.com") {
		t.Errorf("block data leaked into content: md=%q html=%q", bp.ContentMD, bp.ContentHTML)
	}
	if len(bp.Blocks) != 2 {
		t.Fatalf("blocks = %+v", bp.Blocks)
	}

	// 异步预取完成后标题/封面/播放地址就位
	deadline := time.Now().Add(5 * time.Second)
	var music struct {
		Title    string
		Cover    string
		AudioURL string
	}
	for time.Now().Before(deadline) {
		_, raw := ownerJSON(owner, "GET", base+"/api/v1/posts/"+bp.ID, nil)
		var got struct {
			Blocks []struct {
				Type     string `json:"type"`
				Title    string `json:"title"`
				Cover    string `json:"cover"`
				AudioURL string `json:"audio_url"`
			} `json:"blocks"`
		}
		_ = json.Unmarshal(raw, &got)
		for _, b := range got.Blocks {
			if b.Type == "music" {
				music.Title, music.Cover, music.AudioURL = b.Title, b.Cover, b.AudioURL
			}
		}
		if music.AudioURL != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(music.Title, "Test Song") {
		t.Errorf("music title = %q", music.Title)
	}
	if music.Cover != "https://img.example/cover.jpg" {
		t.Errorf("music cover = %q", music.Cover)
	}
	audio, perr := url.Parse(music.AudioURL)
	if perr != nil {
		t.Fatalf("audio url: %v", perr)
	}
	aq := audio.Query()
	if aq.Get("server") != "netease" || aq.Get("type") != "url" || aq.Get("id") != "1868553" {
		t.Errorf("audio url = %q", music.AudioURL)
	}
	// 播放地址真实可访问
	resp, err = http.Get(music.AudioURL)
	if err != nil || resp.StatusCode != 200 {
		t.Errorf("audio fetch: %v %v", err, resp)
	} else {
		_ = resp.Body.Close()
	}

	// 非法 blocks 清洗：非音乐链接、非 GitHub 域名、空位置、重复项都被丢弃
	code, body = ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{
		"blocks": []map[string]string{
			{"type": "music", "url": "https://evil.example/song"},
			{"type": "github", "url": "https://evil.example/a/b"},
			{"type": "location", "name": ""},
			{"type": "location", "name": "杭州"},
			{"type": "music", "url": "https://music.163.com/song?id=1868553"},
			{"type": "location", "name": "杭州"},
		},
	})
	if code != http.StatusCreated {
		t.Fatalf("sanitize post: %d %s", code, body)
	}
	var sani struct {
		Blocks []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"blocks"`
	}
	_ = json.Unmarshal(body, &sani)
	if len(sani.Blocks) != 2 || sani.Blocks[0].Type != "location" || sani.Blocks[0].Name != "杭州" || sani.Blocks[1].Type != "music" {
		t.Fatalf("sanitized blocks = %+v", sani.Blocks)
	}

	// 编辑时替换 blocks
	code, _ = ownerJSON(owner, "PATCH", base+"/api/v1/posts/"+bp.ID, map[string]any{
		"blocks": []map[string]string{
			{"type": "github", "url": "https://github.com/oklog/ulid"},
		},
	})
	if code != 200 {
		t.Fatalf("patch blocks: %d", code)
	}
	_, raw := ownerJSON(owner, "GET", base+"/api/v1/posts/"+bp.ID, nil)
	var after struct {
		Blocks []struct {
			Type string `json:"type"`
		} `json:"blocks"`
	}
	_ = json.Unmarshal(raw, &after)
	if len(after.Blocks) != 1 || after.Blocks[0].Type != "github" {
		t.Fatalf("blocks after patch = %+v", after.Blocks)
	}

	// --- 删除：帖子和附件文件一起清理 ---
	if code, _ = ownerJSON(owner, "DELETE", base+"/api/v1/posts/"+withAtt.ID, nil); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
	if code, _ = ownerJSON(guest, "GET", base+"/api/v1/posts/"+withAtt.ID, nil); code != http.StatusNotFound {
		t.Error("deleted post should 404")
	}
	resp, _ = guest.Get(base + attURL)
	if resp.StatusCode == 200 {
		t.Error("media file should be deleted with post")
	}
	resp.Body.Close()
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(url.QueryEscape(s), "+", "%20"), "%2B", "%2B")
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r2)
}

// TestCapsuleRoundTrip 导出 → 空实例导入 → 字段/媒体/卡片/标签一致 → 再导入去重。
func TestCapsuleRoundTrip(t *testing.T) {
	baseA, ownerA, _ := newTestServer(t)
	baseB, ownerB, guestB := newTestServer(t)
	if code, _ := ownerJSON(ownerA, "POST", baseA+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != 201 {
		t.Fatalf("init A: %d", code)
	}
	if code, _ := ownerJSON(ownerB, "POST", baseB+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != 201 {
		t.Fatalf("init B: %d", code)
	}

	code, att := uploadFile(t, ownerA, baseA+"/api/v1/attachments", "file", "a.png", pngBytes(t))
	if code != 201 {
		t.Fatalf("upload: %d", code)
	}
	attID, _ := att["id"].(string)
	code, att2 := uploadFile(t, ownerA, baseA+"/api/v1/attachments", "file", "b.png", pngBytesN(t, 3))
	if code != 201 {
		t.Fatalf("upload 2: %d", code)
	}
	att2ID, _ := att2["id"].(string)

	posts := []map[string]any{
		{"content_md": "带标签和位置 #设计 #日记", "location": "上海 · 徐汇"},
		{"content_md": "图片帖", "attachment_ids": []string{attID}},
		{"content_md": "纯文本帖 #roundtrip"},
		{"content_md": "", "blocks": []map[string]any{
			{"type": "music", "url": "https://music.163.com/song?id=33894312"},
			{"type": "location", "name": "杭州 · 西湖"},
		}},
		{"content_md": "", "attachment_ids": []string{att2ID}},
	}
	for _, body := range posts {
		if code, b := ownerJSON(ownerA, "POST", baseA+"/api/v1/posts", body); code != 201 {
			t.Fatalf("create: %d %s", code, b)
		}
	}

	// 导出
	req, _ := http.NewRequest("GET", baseA+"/api/v1/admin/export/capsule", nil)
	resp, err := ownerA.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || len(raw) < 200 {
		t.Fatalf("export: %d %d bytes", resp.StatusCode, len(raw))
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["wisp.yaml"] {
		t.Error("wisp.yaml missing")
	}
	var postFiles, mediaFiles int
	for name := range names {
		if strings.HasPrefix(name, "posts/") {
			postFiles++
		}
		if strings.HasPrefix(name, "files/") {
			mediaFiles++
		}
	}
	if postFiles != 5 || mediaFiles != 2 {
		t.Fatalf("zip entries: posts=%d media=%d", postFiles, mediaFiles)
	}

	// 导入到空实例 B
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("source", "wisp")
	fw, _ := w.CreateFormFile("file", "capsule.zip")
	_, _ = fw.Write(raw)
	_ = w.Close()
	impReq, _ := http.NewRequest("POST", baseB+"/api/v1/admin/import", &buf)
	impReq.Header.Set("Content-Type", w.FormDataContentType())
	impResp, err := ownerB.Do(impReq)
	if err != nil {
		t.Fatal(err)
	}
	impBody, _ := io.ReadAll(impResp.Body)
	_ = impResp.Body.Close()
	var imp map[string]any
	_ = json.Unmarshal(impBody, &imp)
	if impResp.StatusCode != 200 || imp["imported"].(float64) != 5 {
		t.Fatalf("import: %d %s", impResp.StatusCode, impBody)
	}

	// B 侧校验
	code, body := ownerJSON(ownerB, "GET", baseB+"/api/v1/posts?limit=50", nil)
	if code != 200 {
		t.Fatalf("list B: %d", code)
	}
	var list struct {
		Items []struct {
			ContentMD   string   `json:"content_md"`
			Visibility  string   `json:"visibility"`
			Location    string   `json:"location"`
			Tags        []string `json:"tags"`
			Attachments []struct {
				URL string `json:"url"`
			} `json:"attachments"`
			Blocks []struct {
				Type string `json:"type"`
				Name string `json:"name"`
			} `json:"blocks"`
		} `json:"items"`
	}
	_ = json.Unmarshal(body, &list)
	if len(list.Items) != 5 {
		t.Fatalf("B posts = %d", len(list.Items))
	}
	var sawLocationTag, sawImage, sawBlocks, sawImageOnly bool
	for _, it := range list.Items {
		switch it.ContentMD {
		case "带标签和位置 #设计 #日记":
			sawLocationTag = it.Location == "上海 · 徐汇" && len(it.Tags) == 2
		case "图片帖":
			if len(it.Attachments) == 1 {
				r, err := guestB.Get(baseB + it.Attachments[0].URL)
				if err == nil && r.StatusCode == 200 {
					sawImage = true
					_ = r.Body.Close()
				}
			}
		case "":
			if len(it.Blocks) == 2 {
				sawBlocks = it.Blocks[0].Type == "music" && it.Blocks[1].Name == "杭州 · 西湖"
			}
			if len(it.Attachments) == 1 {
				sawImageOnly = true
			}
		}
	}
	if !sawLocationTag || !sawImage || !sawBlocks || !sawImageOnly {
		t.Fatalf("roundtrip mismatch: tag=%v image=%v blocks=%v imageOnly=%v",
			sawLocationTag, sawImage, sawBlocks, sawImageOnly)
	}

	// 再次导入应全部去重
	buf.Reset()
	w = multipart.NewWriter(&buf)
	_ = w.WriteField("source", "wisp")
	fw, _ = w.CreateFormFile("file", "capsule.zip")
	_, _ = fw.Write(raw)
	_ = w.Close()
	impReq, _ = http.NewRequest("POST", baseB+"/api/v1/admin/import", &buf)
	impReq.Header.Set("Content-Type", w.FormDataContentType())
	impResp, _ = ownerB.Do(impReq)
	impBody, _ = io.ReadAll(impResp.Body)
	_ = impResp.Body.Close()
	_ = json.Unmarshal(impBody, &imp)
	if imp["imported"].(float64) != 0 || imp["skipped"].(float64) != 5 {
		t.Fatalf("re-import should dedupe: %s", impBody)
	}
}

// TestMediaLibrary 覆盖媒体列表/统计/无主筛选与删除保护。
func TestMediaLibrary(t *testing.T) {
	base, owner, guest := newTestServer(t)
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: %d", code)
	}

	ids := make([]string, 0, 3)
	urls := make([]string, 0, 3)
	for i, n := range []int{2, 3, 4} {
		code, att := uploadFile(t, owner, base+"/api/v1/attachments", "file", "m.png", pngBytesN(t, n))
		if code != http.StatusCreated {
			t.Fatalf("upload %d: %d %v", i, code, att)
		}
		ids = append(ids, att["id"].(string))
		urls = append(urls, att["url"].(string))
	}

	code, body := ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{
		"content_md": "两张图", "attachment_ids": []string{ids[0], ids[1]},
	})
	if code != http.StatusCreated {
		t.Fatalf("post: %d %s", code, body)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &created)

	var list struct {
		Items []struct {
			ID      string  `json:"id"`
			Kind    string  `json:"kind"`
			PostID  *string `json:"post_id"`
			Excerpt string  `json:"post_excerpt"`
		} `json:"items"`
		Total int64 `json:"total"`
	}
	code, body = ownerJSON(owner, "GET", base+"/api/v1/admin/media?limit=50", nil)
	if code != http.StatusOK {
		t.Fatalf("list: %d %s", code, body)
	}
	_ = json.Unmarshal(body, &list)
	if list.Total != 3 || len(list.Items) != 3 {
		t.Fatalf("total=%d items=%d, want 3/3", list.Total, len(list.Items))
	}
	attached, orphan := 0, 0
	for _, it := range list.Items {
		if it.Kind != "image" {
			t.Errorf("kind=%q", it.Kind)
		}
		if it.PostID != nil {
			attached++
			if it.Excerpt == "" {
				t.Error("attached item missing excerpt")
			}
		} else {
			orphan++
		}
	}
	if attached != 2 || orphan != 1 {
		t.Fatalf("attached=%d orphan=%d, want 2/1", attached, orphan)
	}

	// 筛选
	code, body = ownerJSON(owner, "GET", base+"/api/v1/admin/media?orphan=1", nil)
	_ = json.Unmarshal(body, &list)
	if code != http.StatusOK || list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != ids[2] {
		t.Fatalf("orphan filter: %d %s", code, body)
	}
	if code, _ = ownerJSON(owner, "GET", base+"/api/v1/admin/media?kind=video", nil); code != http.StatusOK {
		t.Errorf("kind filter: %d", code)
	}
	if code, _ = ownerJSON(owner, "GET", base+"/api/v1/admin/media?kind=bogus", nil); code != http.StatusBadRequest {
		t.Errorf("bad kind should 400, got %d", code)
	}
	// limit 超上限应钳制而不是重置为默认值；offset 翻页
	code, body = ownerJSON(owner, "GET", base+"/api/v1/admin/media?limit=999", nil)
	_ = json.Unmarshal(body, &list)
	if code != http.StatusOK || list.Total != 3 || len(list.Items) != 3 {
		t.Errorf("limit clamp: %d total=%d items=%d", code, list.Total, len(list.Items))
	}
	code, body = ownerJSON(owner, "GET", base+"/api/v1/admin/media?limit=1&offset=2", nil)
	_ = json.Unmarshal(body, &list)
	if code != http.StatusOK || len(list.Items) != 1 || list.Total != 3 {
		t.Errorf("offset page: %d total=%d items=%d", code, list.Total, len(list.Items))
	}

	// 统计
	var stats struct {
		Count  int64 `json:"count"`
		Bytes  int64 `json:"bytes"`
		Orphan int64 `json:"orphan"`
		ByKind map[string]struct {
			Count int64 `json:"count"`
			Bytes int64 `json:"bytes"`
		} `json:"by_kind"`
	}
	code, body = ownerJSON(owner, "GET", base+"/api/v1/admin/media/stats", nil)
	if code != http.StatusOK {
		t.Fatalf("stats: %d %s", code, body)
	}
	_ = json.Unmarshal(body, &stats)
	if stats.Count != 3 || stats.Bytes <= 0 || stats.Orphan != 1 || stats.ByKind["image"].Count != 3 {
		t.Fatalf("stats = %+v", stats)
	}
	if code, _ = ownerJSON(guest, "GET", base+"/api/v1/admin/media/stats", nil); code != http.StatusUnauthorized {
		t.Errorf("guest stats should 401, got %d", code)
	}

	// 已绑定附件默认拒绝删除
	if code, _ = ownerJSON(owner, "DELETE", base+"/api/v1/admin/media/"+ids[0], nil); code != http.StatusConflict {
		t.Fatalf("delete attached should 409, got %d", code)
	}
	// 无主附件可删，存储文件同步消失
	if code, _ = ownerJSON(owner, "DELETE", base+"/api/v1/admin/media/"+ids[2], nil); code != http.StatusNoContent {
		t.Fatalf("delete orphan: %d", code)
	}
	resp, err := guest.Get(base + urls[2])
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("deleted media should 404, got %d", resp.StatusCode)
	}
	// force 删除已绑定附件：帖子附件列表收缩
	if code, _ = ownerJSON(owner, "DELETE", base+"/api/v1/admin/media/"+ids[0]+"?force=1", nil); code != http.StatusNoContent {
		t.Fatalf("force delete: %d", code)
	}
	code, body = ownerJSON(guest, "GET", base+"/api/v1/posts/"+created.ID, nil)
	if code != http.StatusOK {
		t.Fatalf("get post: %d %s", code, body)
	}
	var post struct {
		Attachments []struct {
			ID string `json:"id"`
		} `json:"attachments"`
	}
	_ = json.Unmarshal(body, &post)
	if len(post.Attachments) != 1 || post.Attachments[0].ID != ids[1] {
		t.Fatalf("post attachments after force delete = %+v", post.Attachments)
	}
	code, body = ownerJSON(owner, "GET", base+"/api/v1/admin/media/stats", nil)
	_ = json.Unmarshal(body, &stats)
	if code != http.StatusOK || stats.Count != 1 || stats.Orphan != 0 {
		t.Fatalf("stats after delete: %d %+v", code, stats)
	}
	if code, _ = ownerJSON(owner, "DELETE", base+"/api/v1/admin/media/nope", nil); code != http.StatusNotFound {
		t.Errorf("missing media should 404, got %d", code)
	}
}

// TestTagManagement 覆盖标签重命名、冲突、合并与只解绑删除。
func TestTagManagement(t *testing.T) {
	base, owner, guest := newTestServer(t)
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: %d", code)
	}

	create := func(md string) map[string]any {
		t.Helper()
		code, body := ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]string{"content_md": md})
		if code != http.StatusCreated {
			t.Fatalf("create %q: %d %s", md, code, body)
		}
		var p map[string]any
		_ = json.Unmarshal(body, &p)
		return p
	}
	p1 := create("第一 #日记 #生活")
	p2 := create("第二 #日记本 #生活")
	p3 := create("第三 #工作")

	tags := func() map[string]float64 {
		t.Helper()
		code, body := ownerJSON(owner, "GET", base+"/api/v1/tags", nil)
		if code != http.StatusOK {
			t.Fatalf("tags: %d %s", code, body)
		}
		var resp struct {
			Items []struct {
				ID   float64 `json:"id"`
				Name string  `json:"name"`
			} `json:"items"`
		}
		_ = json.Unmarshal(body, &resp)
		out := map[string]float64{}
		for _, it := range resp.Items {
			out[it.Name] = it.ID
		}
		return out
	}
	postTags := func(id string) []string {
		t.Helper()
		code, body := ownerJSON(owner, "GET", base+"/api/v1/posts/"+id, nil)
		if code != http.StatusOK {
			t.Fatalf("post %s: %d", id, code)
		}
		var p struct {
			ContentMD string   `json:"content_md"`
			Tags      []string `json:"tags"`
		}
		_ = json.Unmarshal(body, &p)
		if p.ContentMD == "" {
			t.Fatalf("post %s missing content", id)
		}
		return p.Tags
	}
	hasAll := func(got []string, want ...string) bool {
		set := map[string]int{}
		for _, g := range got {
			set[g]++
		}
		for _, w := range want {
			if set[w] != 1 {
				return false
			}
		}
		return len(got) == len(want)
	}
	id := func(name string) string {
		t.Helper()
		v, ok := tags()[name]
		if !ok {
			t.Fatalf("tag %q missing", name)
		}
		return strconv.Itoa(int(v))
	}

	diary := id("日记")
	// 重命名（容忍前导 #）
	code, body := ownerJSON(owner, "PATCH", base+"/api/v1/tags/"+diary, map[string]string{"name": "#随笔"})
	if code != http.StatusOK {
		t.Fatalf("rename: %d %s", code, body)
	}
	if _, ok := tags()["日记"]; ok {
		t.Fatal("old tag name still present")
	}
	if !hasAll(postTags(p1["id"].(string)), "随笔", "生活") {
		t.Fatalf("p1 tags after rename = %v", postTags(p1["id"].(string)))
	}
	// 正文不被改写
	_, body = ownerJSON(owner, "GET", base+"/api/v1/posts/"+p1["id"].(string), nil)
	if !strings.Contains(string(body), "#日记") {
		t.Fatal("post body should keep original hashtag")
	}

	essay := id("随笔")
	// 与已有标签重名 → 409
	if code, _ = ownerJSON(owner, "PATCH", base+"/api/v1/tags/"+essay, map[string]string{"name": "生活"}); code != http.StatusConflict {
		t.Fatalf("rename conflict should 409, got %d", code)
	}
	// 非法名称
	for _, bad := range []string{"", "#", "有 空格", strings.Repeat("长", 65)} {
		if code, _ = ownerJSON(owner, "PATCH", base+"/api/v1/tags/"+essay, map[string]string{"name": bad}); code != http.StatusBadRequest {
			t.Errorf("rename %q should 400, got %d", bad, code)
		}
	}
	// 未登录不可操作
	if code, _ = ownerJSON(guest, "DELETE", base+"/api/v1/tags/"+essay, nil); code != http.StatusUnauthorized {
		t.Errorf("guest delete should 401, got %d", code)
	}

	// 合并到新名称：随笔 + 日记本 → 日记
	diarybook := id("日记本")
	code, body = ownerJSON(owner, "POST", base+"/api/v1/tags/merge", map[string]any{
		"from_ids": []int{atoi(essay), atoi(diarybook)}, "into": "日记",
	})
	if code != http.StatusOK || !strings.Contains(string(body), `"merged":2`) {
		t.Fatalf("merge new: %d %s", code, body)
	}
	if !hasAll(postTags(p1["id"].(string)), "日记", "生活") {
		t.Fatalf("p1 after merge = %v", postTags(p1["id"].(string)))
	}
	if !hasAll(postTags(p2["id"].(string)), "日记", "生活") {
		t.Fatalf("p2 after merge = %v", postTags(p2["id"].(string)))
	}
	for _, gone := range []string{"随笔", "日记本"} {
		if _, ok := tags()[gone]; ok {
			t.Fatalf("merged source %q still present", gone)
		}
	}

	// 合并到已有标签：杂记 → 日记
	p4 := create("第四 #日记 #杂记")
	if !hasAll(postTags(p4["id"].(string)), "日记", "杂记") {
		t.Fatalf("p4 before merge = %v", postTags(p4["id"].(string)))
	}
	code, body = ownerJSON(owner, "POST", base+"/api/v1/tags/merge", map[string]any{
		"from_ids": []int{atoi(id("杂记"))}, "into": "日记",
	})
	if code != http.StatusOK || !strings.Contains(string(body), `"merged":1`) {
		t.Fatalf("merge existing: %d %s", code, body)
	}
	if !hasAll(postTags(p4["id"].(string)), "日记") {
		t.Fatalf("p4 after merge = %v", postTags(p4["id"].(string)))
	}
	// 目标与源相同 / 空源 → 400
	code, body = ownerJSON(owner, "POST", base+"/api/v1/tags/merge", map[string]any{
		"from_ids": []int{atoi(id("日记"))}, "into": "日记",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("self merge should 400: %d %s", code, body)
	}
	if code, _ = ownerJSON(owner, "POST", base+"/api/v1/tags/merge", map[string]any{"from_ids": []int{}, "into": "日记"}); code != http.StatusBadRequest {
		t.Errorf("empty merge should 400, got %d", code)
	}

	// 删除只解绑，帖子保留
	work := id("工作")
	if code, _ = ownerJSON(owner, "DELETE", base+"/api/v1/tags/"+work, nil); code != http.StatusNoContent {
		t.Fatalf("delete tag: %d", code)
	}
	if _, ok := tags()["工作"]; ok {
		t.Fatal("deleted tag still present")
	}
	if got := postTags(p3["id"].(string)); len(got) != 0 {
		t.Fatalf("p3 tags after delete = %v", got)
	}
	if code, _ = ownerJSON(owner, "DELETE", base+"/api/v1/tags/"+work, nil); code != http.StatusNotFound {
		t.Errorf("delete missing tag should 404, got %d", code)
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// TestPostFilters 覆盖日期 × 多标签（ALL）× 游标组合筛选。
func TestPostFilters(t *testing.T) {
	base, owner, _ := newTestServer(t)
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: %d", code)
	}
	now := time.Now().Unix()
	mk := func(md string, published int64) string {
		t.Helper()
		code, body := ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{
			"content_md": md, "published_at": published,
		})
		if code != http.StatusCreated {
			t.Fatalf("create %q: %d %s", md, code, body)
		}
		var p struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(body, &p)
		return p.ID
	}
	pA := mk("A #alpha #beta", now-100)
	pD := mk("D #alpha #beta", now-300)
	pE := mk("E #alpha #beta #gamma", now-20)

	type listResp struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		HasMore    bool   `json:"has_more"`
		NextCursor string `json:"next_cursor"`
	}
	ids := func(r listResp) []string {
		out := make([]string, 0, len(r.Items))
		for _, it := range r.Items {
			out = append(out, it.ID)
		}
		return out
	}
	eq := func(got []string, want ...string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	list := func(url string) listResp {
		t.Helper()
		code, body := ownerJSON(owner, "GET", base+url, nil)
		if code != http.StatusOK {
			t.Fatalf("%s: %d %s", url, code, body)
		}
		var r listResp
		_ = json.Unmarshal(body, &r)
		return r
	}

	// ALL 语义：同时含 alpha 与 beta
	if got := ids(list("/api/v1/posts?tags=alpha,beta&limit=50")); !eq(got, pE, pA, pD) {
		t.Fatalf("tags ALL = %v, want [%s %s %s]", got, pE, pA, pD)
	}
	// 日期下界（含边界）
	if got := ids(list(fmt.Sprintf("/api/v1/posts?tags=alpha,beta&from=%d&limit=50", now-100))); !eq(got, pE, pA) {
		t.Fatalf("from filter = %v", got)
	}
	if got := ids(list(fmt.Sprintf("/api/v1/posts?tags=alpha,beta&to=%d&limit=50", now-101))); !eq(got, pD) {
		t.Fatalf("to filter = %v", got)
	}
	// 标签名容错：# 前缀、空格、重复
	if got := ids(list("/api/v1/posts?tags=%23alpha,%20beta,alpha&limit=50")); !eq(got, pE, pA, pD) {
		t.Fatalf("tag normalization = %v", got)
	}

	// 组合 + 游标翻页（limit=2）：不重不漏
	first := list("/api/v1/posts?tags=alpha,beta&limit=2")
	if !first.HasMore || first.NextCursor == "" || len(first.Items) != 2 {
		t.Fatalf("first page = %+v", first)
	}
	second := list("/api/v1/posts?tags=alpha,beta&limit=2&before=" + url.QueryEscape(first.NextCursor))
	all := append(ids(first), ids(second)...)
	if !eq(all, pE, pA, pD) {
		t.Fatalf("paged ids = %v", all)
	}

	// 非法参数
	for _, u := range []string{
		"/api/v1/posts?from=abc", "/api/v1/posts?to=-1",
		"/api/v1/posts?from=200&to=100",
	} {
		code, body := ownerJSON(owner, "GET", base+u, nil)
		if code != http.StatusBadRequest {
			t.Errorf("%s should 400, got %d", u, code)
		}
		var one map[string]any
		if err := json.Unmarshal(body, &one); err != nil {
			t.Errorf("%s response is not a single JSON document: %v (%q)", u, err, body)
		}
	}
}

// TestBackupAdmin 覆盖自动备份设置、立即运行与快照列表。
func TestBackupAdmin(t *testing.T) {
	base, owner, guest := newTestServer(t)
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: %d", code)
	}
	type resp struct {
		Settings struct {
			Enabled       bool  `json:"enabled"`
			IntervalHours int   `json:"interval_hours"`
			Retention     int   `json:"retention"`
			LastRunAt     int64 `json:"last_run_at"`
		} `json:"settings"`
		Backups []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"backups"`
	}
	var got resp
	code, body := ownerJSON(owner, "GET", base+"/api/v1/admin/settings/backup", nil)
	if code != http.StatusOK {
		t.Fatalf("get settings: %d %s", code, body)
	}
	_ = json.Unmarshal(body, &got)
	if !got.Settings.Enabled || got.Settings.IntervalHours != 24 || got.Settings.Retention != 7 || len(got.Backups) != 0 {
		t.Fatalf("defaults = %+v", got)
	}
	if code, _ = ownerJSON(guest, "GET", base+"/api/v1/admin/settings/backup", nil); code != http.StatusUnauthorized {
		t.Errorf("guest should 401, got %d", code)
	}

	code, body = ownerJSON(owner, "PUT", base+"/api/v1/admin/settings/backup",
		map[string]any{"enabled": true, "interval_hours": 2, "retention": 3, "last_run_at": 123})
	if code != http.StatusOK {
		t.Fatalf("put settings: %d %s", code, body)
	}
	_ = json.Unmarshal(body, &got.Settings)
	if got.Settings.IntervalHours != 2 || got.Settings.Retention != 3 || got.Settings.LastRunAt != 0 {
		t.Fatalf("saved settings = %+v", got.Settings)
	}

	var file struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	code, body = ownerJSON(owner, "POST", base+"/api/v1/admin/backup/run", nil)
	if code != http.StatusOK {
		t.Fatalf("run backup: %d %s", code, body)
	}
	_ = json.Unmarshal(body, &file)
	if !strings.HasPrefix(file.Name, "scheduled-") || !strings.HasSuffix(file.Name, ".db") || file.Size == 0 {
		t.Fatalf("backup file = %+v", file)
	}
	code, body = ownerJSON(owner, "GET", base+"/api/v1/admin/settings/backup", nil)
	if code != http.StatusOK {
		t.Fatalf("get after run: %d", code)
	}
	_ = json.Unmarshal(body, &got)
	if got.Settings.LastRunAt <= 0 || len(got.Backups) != 1 || got.Backups[0].Name != file.Name {
		t.Fatalf("after run = %+v", got)
	}
}

// TestOverview 校验概览数字与帖子/媒体统计一致。
func TestOverview(t *testing.T) {
	base, owner, guest := newTestServer(t)
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: %d", code)
	}
	code, att := uploadFile(t, owner, base+"/api/v1/attachments", "file", "o.png", pngBytes(t))
	if code != http.StatusCreated {
		t.Fatalf("upload: %d %v", code, att)
	}
	attID := att["id"].(string)
	for _, p := range []map[string]any{
		{"content_md": "概览 A #概览", "attachment_ids": []string{attID}},
		{"content_md": "概览 B"},
		{"content_md": "概览 C"},
	} {
		if code, body := ownerJSON(owner, "POST", base+"/api/v1/posts", p); code != http.StatusCreated {
			t.Fatalf("create: %d %s", code, body)
		}
	}

	var got struct {
		Posts struct {
			Total int64 `json:"total"`
			Today int64 `json:"today"`
		} `json:"posts"`
		Attachments struct {
			Count     int64            `json:"count"`
			Bytes     int64            `json:"bytes"`
			ByStorage map[string]int64 `json:"by_storage"`
		} `json:"attachments"`
		Tags          int64  `json:"tags"`
		DBBytes       int64  `json:"db_bytes"`
		LastBackupAt  int64  `json:"last_backup_at"`
		Version       string `json:"version"`
		UptimeSeconds int64  `json:"uptime_seconds"`
	}
	code, body := ownerJSON(owner, "GET", base+"/api/v1/admin/overview", nil)
	if code != http.StatusOK {
		t.Fatalf("overview: %d %s", code, body)
	}
	_ = json.Unmarshal(body, &got)
	if got.Posts.Total != 3 || got.Posts.Today != 3 {
		t.Fatalf("posts = %+v", got.Posts)
	}
	if got.Attachments.Count != 1 || got.Attachments.Bytes <= 0 || got.Attachments.ByStorage["local"] != 1 {
		t.Fatalf("attachments = %+v", got.Attachments)
	}
	if got.Tags != 1 || got.DBBytes <= 0 || got.Version == "" || got.UptimeSeconds < 0 {
		t.Fatalf("overview = %+v", got)
	}
	if code, _ = ownerJSON(guest, "GET", base+"/api/v1/admin/overview", nil); code != http.StatusUnauthorized {
		t.Errorf("guest overview should 401, got %d", code)
	}
}

// TestWebsiteVideoBlocks 校验显式网站/视频块的白名单与 embed 生成。
func TestWebsiteVideoBlocks(t *testing.T) {
	base, owner, _ := newTestServer(t)
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: %d", code)
	}
	code, body := ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{
		"content_md": "显式卡片",
		"blocks": []map[string]any{
			{"type": "website", "url": "https://example.com/article"},
			{"type": "website", "url": "javascript:alert(1)"},
			{"type": "website", "url": "https://"},
			{"type": "video", "url": "https://www.bilibili.com/video/BV1xx411c7mD"},
			{"type": "video", "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
			{"type": "video", "url": "https://github.com/oklog/ulid"},
		},
	})
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	var post struct {
		ID     string `json:"id"`
		Blocks []struct {
			Type     string `json:"type"`
			URL      string `json:"url"`
			Provider string `json:"provider"`
			EmbedURL string `json:"embed_url"`
		} `json:"blocks"`
	}
	_ = json.Unmarshal(body, &post)
	if len(post.Blocks) != 3 {
		t.Fatalf("blocks = %+v, want 3 valid", post.Blocks)
	}
	seen := map[string]bool{}
	for _, b := range post.Blocks {
		switch b.URL {
		case "https://example.com/article":
			if b.Type != "website" {
				t.Errorf("website type = %q", b.Type)
			}
		case "https://www.bilibili.com/video/BV1xx411c7mD":
			if b.Type != "video" || !strings.Contains(b.EmbedURL, "player.bilibili.com") {
				t.Errorf("bilibili = %+v", b)
			}
		case "https://www.youtube.com/watch?v=dQw4w9WgXcQ":
			if b.Type != "video" || !strings.Contains(b.EmbedURL, "youtube-nocookie.com/embed/dQw4w9WgXcQ") {
				t.Errorf("youtube = %+v", b)
			}
		default:
			t.Errorf("unexpected block kept: %+v", b)
		}
		seen[b.URL] = true
	}
	// 落库后读取仍保留 embed_url
	code, body = ownerJSON(owner, "GET", base+"/api/v1/posts/"+post.ID, nil)
	if code != http.StatusOK || !strings.Contains(string(body), `"embed_url":"https://player.bilibili.com`) {
		t.Fatalf("get post: %d %s", code, body)
	}
	if seen["javascript:alert(1)"] {
		t.Fatal("javascript URL should be dropped")
	}
}

// TestRequireUserCSRF 覆盖会话型管理端点的 Origin 校验（此前 requireUser 先检查导致失效）。
func TestRequireUserCSRF(t *testing.T) {
	base, owner, _ := newTestServer(t)
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: %d", code)
	}
	// 先拿到会话 Cookie（ownerJSON 使用 cookiejar，这里复用其 Cookie）
	code, _ := ownerJSON(owner, "GET", base+"/api/v1/tags", nil)
	if code != http.StatusOK {
		t.Fatalf("tags: %d", code)
	}
	// 找出 cookiejar 中的 wisp_session
	u, _ := url.Parse(base)
	var cookie *http.Cookie
	for _, ck := range owner.Jar.Cookies(u) {
		if ck.Name == "wisp_session" {
			cookie = ck
			break
		}
	}
	if cookie == nil {
		t.Fatal("session cookie missing")
	}
	mkReq := func(origin string) *http.Request {
		req, _ := http.NewRequest("POST", base+"/api/v1/tags/merge",
			strings.NewReader(`{"from_ids":[1],"into":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		req.AddCookie(cookie)
		return req
	}
	for _, origin := range []string{"http://evil.example", "null"} {
		resp, err := http.DefaultClient.Do(mkReq(origin))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("origin %q: got %d, want 403", origin, resp.StatusCode)
		}
	}
}

// TestAttachmentReuseAcrossPosts 已绑定文件再次上传应得到独立附件，不能静默丢失。
func TestAttachmentReuseAcrossPosts(t *testing.T) {
	base, owner, guest := newTestServer(t)
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: %d", code)
	}
	img := pngBytesN(t, 6)
	code, a1 := uploadFile(t, owner, base+"/api/v1/attachments", "file", "a.png", img)
	if code != http.StatusCreated {
		t.Fatalf("upload1: %d", code)
	}
	id1 := a1["id"].(string)
	code, p1 := ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{
		"content_md": "第一帖", "attachment_ids": []string{id1},
	})
	if code != http.StatusCreated {
		t.Fatalf("post1: %d %s", code, p1)
	}
	code, a2 := uploadFile(t, owner, base+"/api/v1/attachments", "file", "b.png", img)
	if code != http.StatusCreated {
		t.Fatalf("upload2: %d", code)
	}
	id2 := a2["id"].(string)
	if id2 == id1 {
		t.Fatal("bound attachment must not be reused")
	}
	code, p2 := ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{
		"content_md": "第二帖", "attachment_ids": []string{id2},
	})
	if code != http.StatusCreated {
		t.Fatalf("post2: %d %s", code, p2)
	}
	for _, body := range [][]byte{p1, p2} {
		var dto struct {
			ID          string `json:"id"`
			Attachments []struct {
				ID string `json:"id"`
			} `json:"attachments"`
		}
		_ = json.Unmarshal(body, &dto)
		if len(dto.Attachments) != 1 {
			t.Fatalf("post %s attachments = %+v", dto.ID, dto.Attachments)
		}
		code, got := ownerJSON(guest, "GET", base+"/api/v1/posts/"+dto.ID, nil)
		if code != http.StatusOK || !strings.Contains(string(got), `"attachments":[`) {
			t.Errorf("get post %s: %d", dto.ID, code)
		}
	}
}

// TestCreateValidation 非法可见性/只含非法卡片的空帖必须拒绝。
func TestCreateValidation(t *testing.T) {
	base, owner, _ := newTestServer(t)
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: %d", code)
	}
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/posts",
		map[string]any{"content_md": "x", "visibility": "secret"}); code != http.StatusBadRequest {
		t.Errorf("invalid visibility should 400, got %d", code)
	}
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/posts",
		map[string]any{"content_md": "", "blocks": []map[string]any{{"type": "website", "url": "javascript:alert(1)"}}}); code != http.StatusBadRequest {
		t.Errorf("only invalid blocks should 400, got %d", code)
	}
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/posts",
		map[string]any{"content_md": "坏附件", "attachment_ids": []string{"missing"}}); code != http.StatusBadRequest {
		t.Errorf("missing attachment should 400, got %d", code)
	}
	// 失败的创建不能留下半成品帖子
	_, body := ownerJSON(owner, "GET", base+"/api/v1/posts?limit=50", nil)
	var list struct {
		Items []any `json:"items"`
	}
	_ = json.Unmarshal(body, &list)
	if len(list.Items) != 0 {
		t.Fatalf("failed create left %d posts", len(list.Items))
	}
	// 已绑定附件不能再次使用
	code, att := uploadFile(t, owner, base+"/api/v1/attachments", "file", "x.png", pngBytesN(t, 4))
	if code != http.StatusCreated {
		t.Fatalf("upload: %d", code)
	}
	attID := att["id"].(string)
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/posts",
		map[string]any{"content_md": "第一贴", "attachment_ids": []string{attID}}); code != http.StatusCreated {
		t.Fatalf("first post: %d", code)
	}
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/posts",
		map[string]any{"content_md": "复用附件", "attachment_ids": []string{attID}}); code != http.StatusBadRequest {
		t.Errorf("bound attachment should 400, got %d", code)
	}
}

// TestUpdateClearAndTags 空正文/位置可清空；frontmatter 标签在正文编辑后保留。
func TestUpdateClearAndTags(t *testing.T) {
	base, owner, _ := newTestServer(t)
	if code, _ := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"}); code != http.StatusCreated {
		t.Fatalf("init: %d", code)
	}
	code, body := ownerJSON(owner, "POST", base+"/api/v1/posts", map[string]any{
		"content_md": "正文 #正文标签", "location": "上海 · 徐汇",
		"blocks": []map[string]any{{"type": "location", "name": "杭州", "lat": 30.2, "lng": 120.1}},
	})
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	var p struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &p)
	// 通过胶囊导入的显式标签无法从 API 直接构造，这里验证正文/位置清空与正文标签重建。
	code, body = ownerJSON(owner, "PATCH", base+"/api/v1/posts/"+p.ID, map[string]any{
		"content_md": "", "location": "",
	})
	if code != http.StatusOK {
		t.Fatalf("clear: %d %s", code, body)
	}
	var got struct {
		ContentMD string   `json:"content_md"`
		Location  string   `json:"location"`
		Tags      []string `json:"tags"`
		Blocks    []struct {
			Type string `json:"type"`
		} `json:"blocks"`
	}
	_ = json.Unmarshal(body, &got)
	if got.ContentMD != "" || got.Location != "" || len(got.Tags) != 0 || len(got.Blocks) != 1 {
		t.Fatalf("after clear = %+v", got)
	}
}
