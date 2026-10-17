// Package backup 提供数据库定时快照与保留策略。
package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lmb666666/wisp/internal/repo"
)

const settingKey = "backup_settings"

// Settings 持久化在 settings KV 中；LastRunAt 由服务端维护。
type Settings struct {
	Enabled       bool  `json:"enabled"`
	IntervalHours int   `json:"interval_hours"`
	Retention     int   `json:"retention"`
	LastRunAt     int64 `json:"last_run_at"`
}

// File 是 backups 目录中的一个定时快照。
type File struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	CreatedAt int64  `json:"created_at"`
}

// Defaults 默认开启、每日一份、保留 7 份。
func Defaults() Settings {
	return Settings{Enabled: true, IntervalHours: 24, Retention: 7}
}

func normalize(s Settings) Settings {
	if s.IntervalHours < 1 || s.IntervalHours > 720 {
		s.IntervalHours = 24
	}
	if s.Retention < 1 || s.Retention > 365 {
		s.Retention = 7
	}
	return s
}

// Service 负责读写备份设置、执行快照与清理旧文件。
type Service struct {
	db  *sql.DB
	q   repo.Querier
	dir string

	mu  sync.Mutex
	wg  sync.WaitGroup
	Now func() time.Time
	Log *slog.Logger
}

func New(db *sql.DB, q repo.Querier, dir string) *Service {
	return &Service{db: db, q: q, dir: dir, Now: time.Now, Log: slog.Default()}
}

func (s *Service) Dir() string { return s.dir }

// Settings 读取设置；缺失或损坏时回退默认值，不让站点启动失败。
func (s *Service) Settings(ctx context.Context) (Settings, error) {
	raw, err := s.q.GetSetting(ctx, settingKey)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && strings.TrimSpace(raw) == "") {
		return Defaults(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	var st Settings
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		s.log().Warn("invalid backup settings, using defaults", "err", err)
		return Defaults(), nil
	}
	return normalize(st), nil
}

// Save 更新设置并保留服务端维护的 LastRunAt。
func (s *Service) Save(ctx context.Context, in Settings) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.Settings(ctx)
	if err != nil {
		return Settings{}, err
	}
	in = normalize(in)
	in.LastRunAt = cur.LastRunAt
	if err := s.writeSettings(ctx, in); err != nil {
		return Settings{}, err
	}
	return in, nil
}

func (s *Service) writeSettings(ctx context.Context, st Settings) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.q.UpsertSetting(ctx, repo.UpsertSettingParams{
		Key: settingKey, Value: string(raw), UpdatedAt: s.Now().Unix(),
	})
}

// Due 报告当前是否到期（禁用时永远不到期；从未跑过视为到期）。
func (s *Service) Due(ctx context.Context) (bool, error) {
	st, err := s.Settings(ctx)
	if err != nil || !st.Enabled {
		return false, err
	}
	if st.LastRunAt <= 0 {
		return true, nil
	}
	next := st.LastRunAt + int64(st.IntervalHours)*3600
	return s.Now().Unix() >= next, nil
}

// Tick 到期则执行一次快照；返回是否实际执行。
func (s *Service) Tick(ctx context.Context) (bool, error) {
	due, err := s.Due(ctx)
	if err != nil || !due {
		return false, err
	}
	if _, err := s.Run(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// Run 立即生成一次快照并执行保留策略。
func (s *Service) Run(ctx context.Context) (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return File{}, fmt.Errorf("create backup dir: %w", err)
	}
	now := s.Now()
	name := fmt.Sprintf("scheduled-%s.db", now.Format("20060102-150405"))
	path := filepath.Join(s.dir, name)
	for i := 1; ; i++ {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			break
		}
		name = fmt.Sprintf("scheduled-%s-%d.db", now.Format("20060102-150405"), i)
		path = filepath.Join(s.dir, name)
	}
	// 先写临时文件，成功后再改名：崩溃不会留下以 .db 结尾的半成品。
	tmp := path + ".tmp"
	_ = os.Remove(tmp)
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", tmp); err != nil {
		_ = os.Remove(tmp)
		return File{}, fmt.Errorf("vacuum into backup: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return File{}, fmt.Errorf("finalize backup: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = os.Remove(path)
		return File{}, fmt.Errorf("chmod backup: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		_ = os.Remove(path)
		return File{}, fmt.Errorf("stat backup: %w", err)
	}
	if info.Size() == 0 {
		_ = os.Remove(path)
		return File{}, fmt.Errorf("backup is empty")
	}

	st, err := s.Settings(ctx)
	if err != nil {
		return File{}, err
	}
	st.LastRunAt = now.Unix()
	if err := s.writeSettings(ctx, st); err != nil {
		return File{}, fmt.Errorf("save backup settings: %w", err)
	}
	if err := s.prune(st.Retention, name); err != nil {
		s.log().Warn("prune scheduled backups failed", "err", err)
	}
	return File{Name: name, Size: info.Size(), CreatedAt: info.ModTime().Unix()}, nil
}

// Files 返回 scheduled-* 快照（新→旧）。
func (s *Service) Files() ([]File, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []File{}, nil
		}
		return nil, err
	}
	files := make([]File, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "scheduled-") || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, File{Name: e.Name(), Size: info.Size(), CreatedAt: info.ModTime().Unix()})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].CreatedAt != files[j].CreatedAt {
			return files[i].CreatedAt > files[j].CreatedAt
		}
		return files[i].Name > files[j].Name
	})
	return files, nil
}

// prune 保留最新 retention 份；keep 指定的当前快照永不删除（防同秒/时钟回拨）。
func (s *Service) prune(retention int, keep string) error {
	files, err := s.Files()
	if err != nil {
		return err
	}
	if retention < 1 {
		retention = 1
	}
	keptOthers := 0
	for _, f := range files {
		if f.Name == keep {
			continue
		}
		if keptOthers < retention-1 {
			keptOthers++
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, f.Name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Start 启动时立即检查一次，之后每 10 分钟检查；错误只记日志，不影响进程。
func (s *Service) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		tick := func() {
			if _, err := s.Tick(ctx); err != nil {
				s.log().Warn("scheduled backup failed", "err", err)
			}
		}
		tick()
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tick()
			}
		}
	}()
}

// Wait 等待后台检查/快照退出（进程关闭前调用，避免 DB 已关闭仍在备份）。
func (s *Service) Wait() { s.wg.Wait() }

func (s *Service) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}
