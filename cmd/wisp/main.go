package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
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
	// wisp 不接受命令行参数（配置全走环境变量）；未知参数直接报错，
	// 避免 `./wisp --version` 之类误用被静默忽略后以默认配置启动
	if len(os.Args) > 1 {
		fmt.Fprintf(os.Stderr, "wisp 不接受命令行参数（收到 %q）；配置请使用 WISP_* 环境变量\n", os.Args[1])
		os.Exit(2)
	}
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
	postSvc := service.NewPostService(database, q, cfg.PublicURL, cfg.MetingAPI)
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

	// 优雅退出：第一个信号触发 Shutdown（上限 15s，慢连接不强拖）；
	// 第二个信号立即强退——不再吞掉用户的「快退」意图
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	secondSig := make(chan os.Signal, 1)
	signal.Notify(secondSig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(secondSig)
	go func() {
		<-ctx.Done()
		<-secondSig
		slog.Warn("forced exit on second signal")
		os.Exit(1)
	}()
	backupSvc.Start(ctx)
	defer func() {
		stop()
		backupSvc.Wait()
		hooks.Stop() // 排空 webhook 队列再退，避免事件丢失
		hooks.Wait()
	}()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		_ = e.Shutdown(shutdownCtx)
	}()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           e,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		ReadHeaderTimeout: 10 * time.Second,
	}

	addr := srv.Addr
	// 先绑定再打日志：端口被占时不会先出现误导性的 "listening"
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("http server: %w", err)
	}
	slog.Info("wisp listening", "addr", addr, "data_dir", cfg.DataDir, "public_url", cfg.PublicURL)
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
