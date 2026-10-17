package http

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	wisp "github.com/lmb666666/wisp"
	"github.com/lmb666666/wisp/internal/auth"
	"github.com/lmb666666/wisp/internal/config"
	wispdb "github.com/lmb666666/wisp/internal/db"
	"github.com/lmb666666/wisp/internal/media"
	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/service"
	"github.com/lmb666666/wisp/internal/webhook"
)

// 用真实 embed（而非 MapFS）验证静态资源路由——/assets 曾被 SPA fallback 吞掉。
func TestStaticAssetsFromRealEmbed(t *testing.T) {
	dir := t.TempDir()
	database, err := wispdb.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	if err := wispdb.Migrate(database, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}

	q := repo.New(database)
	postSvc := service.NewPostService(q, "http://test.local", "")
	hooks := webhook.NewDispatcher(q)

	sub, err := fs.Sub(wisp.WebFS, "web/dist")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DataDir: dir, PublicURL: "http://test.local", APIRateLimit: 1000}
	e, err := New(cfg, sub, Deps{DB: database, Querier: q, Auth: auth.NewService(q), Posts: postSvc,
		Uploader: media.NewUploader(mustLocalStore(dir), q),
		Storage:  media.NewSelector(mustLocalStore(dir)),
		Webhooks: hooks})
	if err != nil {
		t.Fatal(err)
	}

	entries, err := fs.ReadDir(sub, "assets")
	if err != nil {
		t.Skipf("embed 无 assets（前端未构建进本次构建）: %v", err)
	}
	cssName := ""
	for _, en := range entries {
		if strings.HasSuffix(en.Name(), ".css") {
			cssName = en.Name()
		}
	}
	if cssName == "" {
		t.Skip("no css asset")
	}

	req := httptest.NewRequest(http.MethodGet, "/assets/"+cssName, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Fatalf("content-type = %q body = %.80s", ct, rec.Body.String())
	}
}

func mustLocalStore(dir string) *media.LocalStorage {
	ls, err := media.NewLocalStorage(dir + "/media")
	if err != nil {
		panic(err)
	}
	return ls
}

// TestRoutesComplete 路由完整性守卫：所有已规划端点必须存在（非 404）。
// /assets 与 /api/v1/ping 曾在历次编辑中静默丢失，此测试防止再犯。
func TestRoutesComplete(t *testing.T) {
	dir := t.TempDir()
	database, err := wispdb.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	if err := wispdb.Migrate(database, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	sub, err := fs.Sub(wisp.WebFS, "web/dist")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DataDir: dir, PublicURL: "http://test.local", APIRateLimit: 1000, EnableSwagger: true}
	e, err := New(cfg, sub, Deps{DB: database, Querier: repo.New(database), Auth: auth.NewService(repo.New(database)),
		Posts:    service.NewPostService(repo.New(database), "http://test.local", ""),
		Uploader: media.NewUploader(mustLocalStore(dir), repo.New(database)),
		Storage:  media.NewSelector(mustLocalStore(dir)), Webhooks: webhook.NewDispatcher(repo.New(database))})
	if err != nil {
		t.Fatal(err)
	}

	type route struct {
		method, path string
	}
	routes := []route{
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/api/v1/ping"},
		{http.MethodGet, "/api/v1/auth/status"},
		{http.MethodPost, "/api/v1/auth/login"},
		{http.MethodGet, "/api/v1/auth/me"},
		{http.MethodGet, "/api/v1/auth/tokens"},
		{http.MethodGet, "/api/v1/auth/passkeys"},
		{http.MethodPost, "/api/v1/auth/passkey/login/begin"},
		{http.MethodGet, "/api/v1/posts"},
		{http.MethodPost, "/api/v1/posts"},
		{http.MethodGet, "/api/v1/posts/x"},
		{http.MethodPost, "/api/v1/posts/x/reactions"},
		{http.MethodGet, "/api/v1/search"},
		{http.MethodGet, "/api/v1/tags"},
		{http.MethodGet, "/api/v1/stats/daily"},
		{http.MethodPost, "/api/v1/attachments"},
		{http.MethodPost, "/api/v1/attachments/x/poster"},
		{http.MethodGet, "/api/v1/admin/backup"},
		{http.MethodGet, "/api/v1/admin/webhooks"},
		{http.MethodPost, "/api/v1/admin/import"},
		{http.MethodGet, "/api/v1/admin/settings/storage"},
		{http.MethodPut, "/api/v1/admin/settings/storage"},
		{http.MethodPost, "/api/v1/admin/settings/storage/test"},
		{http.MethodGet, "/api/v1/admin/settings/notify"},
		{http.MethodPut, "/api/v1/admin/settings/notify"},
		{http.MethodGet, "/feed.xml"},
		{http.MethodGet, "/swagger"},
		{http.MethodGet, "/swagger/openapi.yaml"},
		{http.MethodGet, "/assets/"},
	}
	for _, r := range routes {
		req := httptest.NewRequest(r.method, r.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		// 区分「路由未匹配」（Echo 默认体 {"message":"Not Found"}）与业务 404
		if rec.Code == http.StatusNotFound && strings.Contains(rec.Body.String(), `"message":"Not Found"`) {
			t.Errorf("route missing: %s %s", r.method, r.path)
		}
	}
}
