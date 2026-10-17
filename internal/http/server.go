package http

import (
	"database/sql"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/lmb666666/wisp/internal/auth"
	"github.com/lmb666666/wisp/internal/backup"
	"github.com/lmb666666/wisp/internal/config"
	"github.com/lmb666666/wisp/internal/media"
	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/service"
	"github.com/lmb666666/wisp/internal/webhook"
)

// ogMarker 前端 index.html 中的占位注释，服务端替换为 OG meta。
const ogMarker = "<!--wisp:og-->"

const fallbackIndex = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"/><title>Wisp</title>%s</head>
<body><p>Wisp API 运行中。前端尚未构建：在 web/ 目录执行 <code>pnpm install &amp;&amp; pnpm build</code>。</p></body></html>`

// Server 聚合所有依赖的 HTTP 层。
type Server struct {
	cfg        config.Config
	db         *sql.DB
	q          repo.Querier
	auth       *auth.Service
	posts      *service.PostService
	uploader   *media.Uploader
	store      media.Storage
	selector   *media.Selector
	localStore *media.LocalStorage
	passkeys   *auth.WebAuthnService
	backup     *backup.Service
	startedAt  time.Time
	webFS      fs.FS
	indexHTML  string
}

// Deps HTTP 层依赖注入。
type Deps struct {
	DB         *sql.DB
	Querier    repo.Querier
	Auth       *auth.Service
	Posts      *service.PostService
	Uploader   *media.Uploader
	Storage    media.Storage
	Selector   *media.Selector
	LocalStore *media.LocalStorage
	Webhooks   *webhook.Dispatcher
	Passkeys   *auth.WebAuthnService
	Backup     *backup.Service
}

// New 构建 Echo 实例：API、认证、附件、RSS、静态资源与 SPA fallback。
func New(cfg config.Config, webFS fs.FS, deps Deps) (*echo.Echo, error) {
	s := &Server{
		cfg:        cfg,
		db:         deps.DB,
		q:          deps.Querier,
		auth:       deps.Auth,
		posts:      deps.Posts,
		uploader:   deps.Uploader,
		store:      deps.Storage,
		selector:   deps.Selector,
		localStore: deps.LocalStore,
		backup:     deps.Backup,
		startedAt:  time.Now(),
	}
	if s.backup == nil && deps.DB != nil && deps.Querier != nil {
		s.backup = backup.New(deps.DB, deps.Querier, filepath.Join(cfg.DataDir, "backups"))
	}
	indexHTML := fmt.Sprintf(fallbackIndex, ogMarker)
	if b, err := fs.ReadFile(webFS, "index.html"); err == nil {
		indexHTML = string(b)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read embedded index.html: %w", err)
	}
	s.webFS = webFS
	s.indexHTML = indexHTML

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{Skipper: func(c echo.Context) bool {
		return strings.HasPrefix(c.Path(), "/api/")
	}}))

	// ---- 公开端点 ----
	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// 本地附件静态路由：始终挂在 LocalStore 上（生产 Storage 是可切换的 Selector）
	if deps.LocalStore != nil {
		e.GET("/media/*", echo.WrapHandler(
			http.StripPrefix("/media/", http.FileServer(http.Dir(deps.LocalStore.Root())))))
	}

	// 前端构建产物（/assets/*）——必须在 SPA fallback 之前注册
	if st, err := fs.Stat(webFS, "assets"); err == nil && st.IsDir() {
		assetsFS, err := fs.Sub(webFS, "assets")
		if err != nil {
			return nil, err
		}
		e.GET("/assets/*", echo.WrapHandler(
			http.StripPrefix("/assets/", http.FileServer(http.FS(assetsFS)))))
	}

	// RSS / Atom
	e.GET("/feed.xml", s.handleFeed)

	// SPA + 详情页 OG 注入
	e.GET("/", s.spaIndex)
	e.GET("/p/:id", s.postPage)
	e.GET("/:file", s.serveRootFile)
	e.GET("/*", s.spaIndex)

	// ---- API ----
	api := e.Group("/api/v1")
	api.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(cfg.APIRateLimit))))
	if len(cfg.CORSOrigins) > 0 {
		api.Use(corsForAPI(cfg.CORSOrigins))
	}

	authGroup := api.Group("/auth")
	authGroup.GET("/status", s.authStatus, s.resolveAuth)
	authGroup.POST("/init", s.authInit)
	authGroup.POST("/login", s.authLogin)
	authGroup.POST("/logout", s.authLogout, s.requireUser)
	authGroup.GET("/me", s.authMe, s.resolveAuth)
	authGroup.GET("/profile", s.getProfile, s.requireUser)
	authGroup.PATCH("/profile", s.updateProfile, s.requireUser)
	authGroup.POST("/passkey/register/begin", s.beginPasskeyRegistration, s.requireUser)
	authGroup.POST("/passkey/register/finish", s.finishPasskeyRegistration, s.requireUser)
	authGroup.POST("/passkey/login/begin", s.beginPasskeyLogin)
	authGroup.POST("/passkey/login/finish", s.finishPasskeyLogin)
	authGroup.GET("/passkeys", s.listPasskeys, s.requireUser)
	authGroup.DELETE("/passkeys/:id", s.deletePasskey, s.requireUser)
	authGroup.GET("/tokens", s.listTokens, s.requireUser)
	authGroup.POST("/tokens", s.createToken, s.requireUser)
	authGroup.DELETE("/tokens/:id", s.deleteToken, s.requireUser)

	api.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"pong": "wisp"})
	})

	api.GET("/posts", s.listPosts, s.resolveAuth)
	api.POST("/posts", s.createPost, s.resolveAuth, s.requireWrite)
	api.GET("/posts/:id", s.getPost, s.resolveAuth)
	api.PATCH("/posts/:id", s.updatePost, s.resolveAuth, s.requireWrite)
	api.DELETE("/posts/:id", s.deletePost, s.resolveAuth, s.requireWrite)
	api.POST("/posts/:id/reactions", s.addReaction)
	api.DELETE("/posts/:id/reactions", s.removeReaction)
	api.GET("/search", s.searchPosts, s.resolveAuth)

	api.GET("/tags", s.listTags)
	api.PATCH("/tags/:id", s.renameTag, s.requireUser)
	api.POST("/tags/merge", s.mergeTags, s.requireUser)
	api.DELETE("/tags/:id", s.deleteTag, s.requireUser)
	api.GET("/stats/daily", s.statsDaily, s.resolveAuth)
	api.GET("/admin/overview", s.overview, s.requireUser)
	api.GET("/admin/backup", s.downloadBackup, s.requireUser)
	api.GET("/admin/settings/backup", s.getBackupSettings, s.requireUser)
	api.PUT("/admin/settings/backup", s.putBackupSettings, s.requireUser)
	api.POST("/admin/backup/run", s.runBackup, s.requireUser)
	api.GET("/admin/export/capsule", s.exportCapsule, s.requireUser)
	api.GET("/admin/media", s.listMedia, s.requireUser)
	api.GET("/admin/media/stats", s.mediaStats, s.requireUser)
	api.DELETE("/admin/media/:id", s.deleteMedia, s.requireUser)
	api.POST("/admin/import", s.adminImport, s.requireUser)
	api.GET("/admin/webhooks", s.listWebhooksAdmin, s.requireUser)
	api.POST("/admin/webhooks", s.createWebhook, s.requireUser)
	api.DELETE("/admin/webhooks/:id", s.deleteWebhook, s.requireUser)
	api.GET("/admin/settings/storage", s.getStorageSettings, s.requireUser)
	api.PUT("/admin/settings/storage", s.putStorageSettings, s.requireUser)
	api.POST("/admin/settings/storage/test", s.testStorage, s.requireUser)
	api.GET("/admin/settings/geo", s.getGeoSettings, s.requireUser)
	api.GET("/admin/settings/meting", s.getMetingSettings, s.requireUser)
	api.PUT("/admin/settings/meting", s.putMetingSettings, s.requireUser)
	api.GET("/admin/settings/notify", s.getNotifySettings, s.requireUser)
	api.PUT("/admin/settings/notify", s.putNotifySettings, s.requireUser)
	api.POST("/attachments", s.uploadAttachment, s.resolveAuth, s.requireWrite)
	api.POST("/attachments/:id/poster", s.uploadPoster, s.resolveAuth, s.requireWrite)

	if cfg.EnableSwagger {
		e.GET("/swagger", func(c echo.Context) error { return c.HTML(http.StatusOK, swaggerUI) })
		e.GET("/swagger/openapi.yaml", func(c echo.Context) error {
			c.Response().Header().Set("Content-Type", "application/yaml")
			return c.String(http.StatusOK, openAPISpec)
		})
	}

	return e, nil
}

func (s *Server) spaIndex(c echo.Context) error {
	extra := `<meta property="og:type" content="website"/>` + s.defaultOGImage()
	page := injectOG(s.indexHTML, "Wisp", "轻轻说出，牢牢拥有。自托管的轻量说说。", extra)
	return c.HTML(http.StatusOK, page)
}

func (s *Server) defaultOGImage() string {
	base := strings.TrimRight(s.cfg.PublicURL, "/")
	return fmt.Sprintf(
		`<meta property="og:image" content="%s/og.png"/><meta name="twitter:card" content="summary_large_image"/>`,
		html.EscapeString(base),
	)
}

// postPage 详情页：服务端注入真实 OG meta（SSR-lite）。
func (s *Server) postPage(c echo.Context) error {
	id := c.Param("id")
	title, desc := "Wisp", "来自 Wisp 的一条动态"
	extra := ""

	dto, err := s.posts.Get(c.Request().Context(), id, false, "")
	found := false
	switch {
	case err == nil:
		found = true
		if t := service.Excerpt(dto.ContentMD, 60); t != "" {
			title = t
		}
		desc = service.Excerpt(dto.ContentMD, 160)
		for _, a := range dto.Attachments {
			if a.Kind == "image" && a.ThumbURL != "" {
				extra += fmt.Sprintf(`<meta property="og:image" content="%s"/>`,
					html.EscapeString(s.cfg.PublicURL+a.ThumbURL))
				break
			}
		}
	case errors.Is(err, service.ErrNotFound):
		extra = `<meta name="robots" content="noindex"/>`
	default:
		slog.Warn("og lookup failed", "id", id, "err", err)
	}
	if found {
		extra += `<meta property="og:type" content="article"/>`
		if strings.Contains(extra, "og:image") {
			extra += `<meta name="twitter:card" content="summary_large_image"/>`
		} else {
			extra += s.defaultOGImage()
		}
	}

	page := injectOG(s.indexHTML, title, desc, extra)
	return c.HTML(http.StatusOK, page)
}

func injectOG(page, title, desc, extra string) string {
	t := html.EscapeString(title)
	d := html.EscapeString(desc)
	meta := fmt.Sprintf(
		`<meta property="og:title" content="%s"/><meta property="og:description" content="%s"/>`+
			`<meta name="description" content="%s"/>%s`,
		t, d, d, extra,
	)
	if strings.Contains(page, ogMarker) {
		return strings.Replace(page, ogMarker, meta, 1)
	}
	return strings.Replace(page, "</head>", meta+"</head>", 1)
}

// corsForAPI 面向第三方 Web 客户端的受限 CORS：仅 /api/v1、仅白名单来源、
// 凭证一律不带（PAT 认证，避免 Cookie 跨站暴露）。
func corsForAPI(allowed []string) echo.MiddlewareFunc {
	allow := map[string]bool{}
	for _, o := range allowed {
		allow[strings.TrimSpace(o)] = true
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			origin := c.Request().Header.Get("Origin")
			if origin != "" && allow[origin] {
				h := c.Response().Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Client-ID")
				h.Set("Access-Control-Max-Age", "86400")
				if c.Request().Method == http.MethodOptions {
					return c.NoContent(http.StatusNoContent)
				}
			}
			return next(c)
		}
	}
}

// serveRootFile 返回前端构建产物的根级文件（sw.js、manifest、图标、workbox 等）。
// 不存在的路径回退到 SPA；单个路径段才进入这里，深层路由由 /* 处理。
func (s *Server) serveRootFile(c echo.Context) error {
	name := c.Param("file")
	if name == "" || name == "index.html" || strings.ContainsAny(name, "/\\") || strings.HasPrefix(name, ".") {
		return s.spaIndex(c)
	}
	st, err := fs.Stat(s.webFS, name)
	if err != nil || st.IsDir() {
		// 形如 xxx.js / favicon.ico 的缺失资源应 404，而不是返回 index.html
		if filepath.Ext(name) != "" {
			return echo.ErrNotFound
		}
		return s.spaIndex(c)
	}
	data, err := fs.ReadFile(s.webFS, name)
	if err != nil {
		return s.spaIndex(c)
	}
	ctype := mime.TypeByExtension(filepath.Ext(name))
	switch {
	case strings.HasSuffix(name, ".webmanifest"):
		ctype = "application/manifest+json"
	case strings.HasSuffix(name, ".js"):
		ctype = "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		ctype = "image/svg+xml"
	case ctype == "":
		ctype = http.DetectContentType(data)
	}
	switch name {
	case "sw.js", "registerSW.js":
		c.Response().Header().Set("Cache-Control", "no-cache")
	default:
		c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	}
	return c.Blob(http.StatusOK, ctype, data)
}
