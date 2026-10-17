package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	wisp "github.com/lmb666666/wisp"
	"github.com/lmb666666/wisp/internal/auth"
	"github.com/lmb666666/wisp/internal/backup"
	"github.com/lmb666666/wisp/internal/config"
	wispdb "github.com/lmb666666/wisp/internal/db"
	wisphttp "github.com/lmb666666/wisp/internal/http"
	"github.com/lmb666666/wisp/internal/media"
	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/service"
	"github.com/lmb666666/wisp/internal/webhook"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(newLogger(cfg.LogLevel))

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	backupDir := filepath.Join(cfg.DataDir, "backups")

	database, err := wispdb.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	if err := wispdb.Migrate(database, backupDir); err != nil {
		return err
	}
	slog.Info("database ready", "path", cfg.DBPath)

	// ---- 服务装配 ----
	q := repo.New(database)
	authSvc := auth.NewService(q)
	passkeySvc, err := auth.NewWebAuthnService(q, cfg.PublicURL)
	if err != nil {
		slog.Warn("passkey disabled", "err", err)
	}
	postSvc := service.NewPostService(q, cfg.PublicURL, cfg.MetingAPI)
	if err := service.ReRenderAll(context.Background(), q); err != nil {
		slog.Warn("re-render posts failed", "err", err)
	}
	localStore, err := media.NewLocalStorage(filepath.Join(cfg.DataDir, "media"))
	if err != nil {
		return err
	}
	selector := media.NewSelector(localStore)
	uploader := media.NewUploader(selector, q)
	// 数据库中的设置优先（即使是空串=显式关闭）；无记录时用环境变量
	if val, err := q.GetSetting(context.Background(), "meting_api_url"); err == nil {
		postSvc.SetMetingAPI(val)
	}
	hooks := webhook.NewDispatcher(q)
	hooks.Start()
	postSvc.WithNotifier(hooks.Emit)
	backupSvc := backup.New(database, q, backupDir)

	// 后台清理：过期会话 + 7 天前的幂等键，启动时一次 + 每 24h
	go func() {
		clean := func() {
			now := time.Now().Unix()
			_ = q.DeleteExpiredSessions(context.Background(), now)
			_ = q.DeleteOldIdempotencyKeys(context.Background(), now-7*24*3600)
		}
		clean()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			<-ticker.C
			clean()
		}
	}()

	webFS, err := fs.Sub(wisp.WebFS, "web/dist")
	if err != nil {
		return err
	}
	// 启动时应用已保存的存储配置（失败回退本地）
	st := media.DefaultSettings()
	if val, err := q.GetSetting(context.Background(), "storage_settings"); err == nil && val != "" {
		_ = json.Unmarshal([]byte(val), &st)
	}
	primary, err := media.BuildStorage(context.Background(), st)
	switch {
	case err != nil:
		slog.Warn("storage settings invalid, fallback to local", "err", err)
		selector.Configure(localStore, false)
	case primary != nil:
		selector.Configure(primary, st.MirrorLocal)
	default:
		selector.Configure(localStore, st.MirrorLocal)
	}

	e, err := wisphttp.New(cfg, webFS, wisphttp.Deps{
		DB:         database,
		Querier:    q,
		Auth:       authSvc,
		Posts:      postSvc,
		Uploader:   uploader,
		Storage:    selector,
		Selector:   selector,
		LocalStore: localStore,
		Webhooks:   hooks,
		Passkeys:   passkeySvc,
		Backup:     backupSvc,
	})
	if err != nil {
		return err
	}

	// 优雅退出
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	backupSvc.Start(ctx)
	defer func() {
		stop()
		backupSvc.Wait()
	}()
	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		_ = e.Shutdown(context.Background())
	}()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           e,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		ReadHeaderTimeout: 10 * time.Second,
	}

	addr := srv.Addr
	slog.Info("wisp listening", "addr", addr, "data_dir", cfg.DataDir, "public_url", cfg.PublicURL)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

func newLogger(level string) *slog.Logger {
	var lv slog.Level
	if err := lv.UnmarshalText([]byte(level)); err != nil {
		lv = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lv}))
}
