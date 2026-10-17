package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	wisp "github.com/lmb666666/wisp"
)

func openTemp(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	database, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database, dir
}

func TestMigrateFreshDatabase(t *testing.T) {
	database, dir := openTemp(t)
	backupDir := filepath.Join(dir, "backups")

	if err := Migrate(database, backupDir); err != nil {
		t.Fatalf("migrate fresh: %v", err)
	}

	want, err := latestVersion()
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := currentVersion(database); got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}

	// 核心表应全部存在
	for _, table := range []string{
		"users", "sessions", "posts", "attachments", "tags",
		"post_tags", "settings", "link_previews", "api_tokens",
		"idempotency_keys", "reactions", "posts_fts",
	} {
		var n int
		if err := database.QueryRow(
			`SELECT count(*) FROM sqlite_master WHERE type IN ('table','virtual table') AND name = ?`, table,
		).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("table %s missing", table)
		}
	}

	// 全新库迁移不应产生快照
	if entries, _ := os.ReadDir(backupDir); len(entries) != 0 {
		t.Errorf("fresh migrate should not snapshot, got %d files", len(entries))
	}
}

// TestMigrateSnapshotBeforeUpgrade：真实升级路径——先只升到 v2，
// 再完整 Migrate（v2 → latest），应当先快照后重放后续迁移。
func TestMigrateSnapshotBeforeUpgrade(t *testing.T) {
	database, dir := openTemp(t)
	backupDir := filepath.Join(dir, "backups")

	goose.SetBaseFS(wisp.MigrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(database, "migrations", 2); err != nil {
		t.Fatalf("seed to v2: %v", err)
	}

	// 写入真实业务数据（升级演练：数据必须原样穿越迁移）
	now := time.Now().Unix()
	if _, err := database.Exec(
		`INSERT INTO users (username, password_hash, is_owner, created_at, updated_at)
		 VALUES ('drill', 'x', 1, ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(
		`INSERT INTO posts (id, user_id, content_md, content_html, visibility, published_at, source, created_at)
		 VALUES ('01DRILLPOST0000000000', 1, '演练数据', '<p>演练数据</p>', 'public', ?, 'web', ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(
		`INSERT INTO posts (id, user_id, content_md, content_html, visibility, published_at, source, created_at)
		 VALUES ('01DRILLUNLISTED0000000', 1, '旧仅链接', '<p>旧仅链接</p>', 'unlisted', ?, 'web', ?)`, now, now); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(database, backupDir); err != nil {
		t.Fatalf("upgrade from v2: %v", err)
	}

	// 数据穿越迁移后仍在
	var content string
	if err := database.QueryRow(`SELECT content_md FROM posts WHERE id = '01DRILLPOST0000000000'`).Scan(&content); err != nil {
		t.Fatalf("drill data lost after migration: %v", err)
	}
	if content != "演练数据" {
		t.Errorf("content = %q", content)
	}
	var visibility string
	if err := database.QueryRow(
		`SELECT visibility FROM posts WHERE id = '01DRILLUNLISTED0000000'`).Scan(&visibility); err != nil {
		t.Fatal(err)
	}
	if visibility != "private" {
		t.Errorf("unlisted should migrate to private, got %q", visibility)
	}
	if _, err := database.Exec(`SELECT pinned_at FROM posts LIMIT 1`); err == nil {
		t.Error("pinned_at column should be dropped")
	}

	want, err := latestVersion()
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := currentVersion(database); got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want exactly 1 snapshot, got %d", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "pre-migrate-v2-") {
		t.Errorf("snapshot name = %s, want prefix pre-migrate-v2-", entries[0].Name())
	}
}
