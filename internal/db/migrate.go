package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pressly/goose/v3"

	wisp "github.com/lmb666666/wisp"
)

// Migrate 执行 goose 迁移。
// 若库不是全新（current > 0）且目标版本更新，先用 VACUUM INTO
// 做在线一致性快照（「升级不碎」第一道保险），失败则拒绝迁移。
func Migrate(database *sql.DB, backupDir string) error {
	goose.SetBaseFS(wisp.MigrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	current, err := currentVersion(database)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	target, err := latestVersion()
	if err != nil {
		return err
	}

	if current > 0 && current < target {
		if err := snapshot(database, backupDir, current); err != nil {
			return fmt.Errorf("pre-migrate snapshot: %w", err)
		}
	}

	if err := goose.Up(database, "migrations"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// currentVersion 直接查 goose 的版本表，全新库返回 0。
// 不依赖 goose 内部对「表不存在」的处理方式。
func currentVersion(database *sql.DB) (int64, error) {
	var n int
	if err := database.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'goose_db_version'`,
	).Scan(&n); err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	var v sql.NullInt64
	if err := database.QueryRow(`SELECT max(version_id) FROM goose_db_version`).Scan(&v); err != nil {
		return 0, err
	}
	return v.Int64, nil
}

// latestVersion 从内嵌迁移文件名解析最大版本号（约定：NNNNN_desc.sql）。
func latestVersion() (int64, error) {
	entries, err := wisp.MigrationsFS.ReadDir("migrations")
	if err != nil {
		return 0, fmt.Errorf("read migrations: %w", err)
	}
	var max int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			return 0, fmt.Errorf("bad migration filename: %s", e.Name())
		}
		v, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse migration version from %s: %w", e.Name(), err)
		}
		if v > max {
			max = v
		}
	}
	if max == 0 {
		return 0, errors.New("no migration files found")
	}
	return max, nil
}

// snapshot 用 VACUUM INTO 生成在线一致性备份（不锁库、无需外部工具）。
func snapshot(database *sql.DB, dir string, version int64) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("pre-migrate-v%d-%s.db", version, time.Now().Format("20060102-150405"))
	dest := filepath.Join(dir, name)
	if _, err := database.ExecContext(context.Background(), "VACUUM INTO ?", dest); err != nil {
		return fmt.Errorf("vacuum into %s: %w", dest, err)
	}
	return nil
}
