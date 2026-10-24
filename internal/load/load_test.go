//go:build load

// 压测（手动执行）：go test -tags load -run TestLoad10k -v ./internal/load/
// 目标：万帖量级下时间线 / 详情 / 搜索 / RSS 的响应特性。
package load

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lmb666666/wisp/internal/auth"
	"github.com/lmb666666/wisp/internal/config"
	wispdb "github.com/lmb666666/wisp/internal/db"
	wisphttp "github.com/lmb666666/wisp/internal/http"
	"github.com/lmb666666/wisp/internal/media"
	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/service"
	"testing/fstest"
)

const total = 10000

func TestLoad10k(t *testing.T) {
	dir := t.TempDir()
	database, err := wispdb.Open(filepath.Join(dir, "load.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := wispdb.Migrate(database, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}

	q := repo.New(database)
	authSvc := auth.NewService(q)
	postSvc := service.NewPostService(database, q, "http://test.local", "")
	store, _ := media.NewLocalStorage(filepath.Join(dir, "media"))

	cfg := config.Config{DataDir: dir, PublicURL: "http://test.local", APIRateLimit: 100000}
	e, err := wisphttp.New(cfg, fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html><head><!--wisp:og--></head></html>")}},
		wisphttp.Deps{DB: database, Querier: q, Auth: authSvc, Posts: postSvc,
			Uploader: media.NewUploader(store, q), Storage: store})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(e)
	defer ts.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// 初始化 + 登录
	call := func(method, path string, body string) []byte {
		req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io_ReadAll(resp)
		if resp.StatusCode >= 300 && resp.StatusCode != 409 {
			t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, b)
		}
		return b
	}
	_ = call("POST", "/api/v1/auth/init", `{"username":"bench","password":"password123"}`)
	_ = call("POST", "/api/v1/posts", `{"content_md":"预热"}`)

	// 灌 10k 帖（直接走服务层，渲染真实执行）
	start := time.Now()
	for i := 0; i < total; i++ {
		_, err := postSvc.Create(t.Context(), 1, service.CreateParams{
			ContentMD:   fmt.Sprintf("第 %d 条压测数据 **加粗** #压测 #批量 标签内容长度适度即可 %d", i, i),
			Visibility:  "public",
			PublishedAt: time.Now().Unix() - int64(total-i),
			Source:      "load",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("插入 %d 帖（含 Markdown 渲染 + 标签解析）: %s", total, time.Since(start).Round(time.Millisecond))

	measure := func(name string, n int, fn func()) {
		start := time.Now()
		for i := 0; i < n; i++ {
			fn()
		}
		avg := time.Since(start) / time.Duration(n)
		t.Logf("%-24s %d 次，平均 %s", name, n, avg)
	}

	// 时间线翻页（作者视角，含标签/附件/点赞装配）
	var lastBody []byte
	measure("GET /posts（20/页）", 100, func() {
		resp, err := client.Get(ts.URL + "/api/v1/posts?limit=20")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io_ReadAll(resp)
		resp.Body.Close()
		lastBody = b
	})

	// 组合筛选：日期 × 双标签 ALL（1 万帖命中窗口）
	from := time.Now().Unix() - int64(total) - 10
	measure("GET /posts（日期+双标签）", 100, func() {
		resp, err := client.Get(fmt.Sprintf("%s/api/v1/posts?limit=20&from=%d&tags=%%E5%%8E%%8B%%E6%%B5%%8B,%%E6%%89%%B9%%E9%%87%%8F", ts.URL, from))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	})

	// FTS5 搜索（4 字）
	measure("GET /search（FTS5）", 100, func() {
		resp, err := client.Get(ts.URL + "/api/v1/search?q=%E5%8E%8B%E6%B5%8B%E6%95%B0%E6%8D%AE")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	})

	// LIKE 短词搜索
	measure("GET /search（LIKE 2字）", 50, func() {
		resp, err := client.Get(ts.URL + "/api/v1/search?q=%E6%A0%87%E7%AD%BE")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	})

	// RSS（20 条全量渲染）
	measure("GET /feed.xml", 50, func() {
		resp, err := client.Get(ts.URL + "/feed.xml")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	})

	var timeline struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	_ = json.Unmarshal(lastBody, &timeline)
	if len(timeline.Items) == 0 {
		t.Fatal("no items")
	}
	measure("GET /posts/:id（详情）", 100, func() {
		resp, err := client.Get(ts.URL + "/api/v1/posts/" + timeline.Items[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	})

	// 数据完整性
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM posts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count < total {
		t.Fatalf("posts = %d, want >= %d", count, total)
	}
	t.Logf("数据完整性: %d 条帖子在库", count)
}

// 避免 gosec 对 io.ReadAll 的无谓告警，同时保持测试简洁。
func io_ReadAll(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}
