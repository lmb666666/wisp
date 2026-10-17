package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	wispdb "github.com/lmb666666/wisp/internal/db"
	"github.com/lmb666666/wisp/internal/repo"
)

func newTestService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	dir := t.TempDir()
	db, err := wispdb.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := wispdb.Migrate(db, filepath.Join(dir, "migrations")); err != nil {
		t.Fatal(err)
	}
	svc := New(db, repo.New(db), filepath.Join(dir, "backups"))
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)
	svc.Now = func() time.Time { return now }
	t.Cleanup(func() {})
	return svc, context.Background()
}

func TestScheduledBackup(t *testing.T) {
	svc, ctx := newTestService(t)
	clock := time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)
	svc.Now = func() time.Time { return clock }

	st, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || st.IntervalHours != 24 || st.Retention != 7 {
		t.Fatalf("defaults = %+v", st)
	}
	// 保存：保留 last_run_at，非法值归一化
	st, err = svc.Save(ctx, Settings{Enabled: true, IntervalHours: 0, Retention: 1000, LastRunAt: 999})
	if err != nil {
		t.Fatal(err)
	}
	if st.IntervalHours != 24 || st.Retention != 7 || st.LastRunAt != 0 {
		t.Fatalf("saved = %+v", st)
	}

	// 从未跑过 → 到期；运行后 1 小时不到期，25 小时到期
	if due, _ := svc.Due(ctx); !due {
		t.Fatal("first run should be due")
	}
	f1, err := svc.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if f1.Size == 0 || f1.Name != "scheduled-20261017-120000.db" {
		t.Fatalf("file = %+v", f1)
	}
	clock = clock.Add(time.Hour)
	if ran, err := svc.Tick(ctx); err != nil || ran {
		t.Fatalf("tick after 1h = %v %v", ran, err)
	}
	clock = clock.Add(24 * time.Hour)
	if ran, err := svc.Tick(ctx); err != nil || !ran {
		t.Fatalf("tick after 25h = %v %v", ran, err)
	}
	// 第三次 + 保留 2
	clock = clock.Add(25 * time.Hour)
	if _, err := svc.Save(ctx, Settings{Enabled: true, IntervalHours: 24, Retention: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Run(ctx); err != nil {
		t.Fatal(err)
	}
	files, err := svc.Files()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("retention kept %d files: %+v", len(files), files)
	}

	// 禁用后 Tick 不执行
	if _, err := svc.Save(ctx, Settings{Enabled: false, IntervalHours: 24, Retention: 2}); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(100 * time.Hour)
	if ran, err := svc.Tick(ctx); err != nil || ran {
		t.Fatalf("disabled tick = %v %v", ran, err)
	}
}

func TestSameSecondRetentionKeepsNewest(t *testing.T) {
	svc, ctx := newTestService(t)
	clock := time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)
	svc.Now = func() time.Time { return clock }
	if _, err := svc.Save(ctx, Settings{Enabled: true, IntervalHours: 24, Retention: 1}); err != nil {
		t.Fatal(err)
	}
	first, err := svc.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first.Name == second.Name {
		t.Fatalf("expected unique names, got %s", first.Name)
	}
	files, err := svc.Files()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != second.Name {
		t.Fatalf("retention kept %+v, want only %s", files, second.Name)
	}
	if _, err := os.Stat(filepath.Join(svc.Dir(), second.Name)); err != nil {
		t.Fatalf("new snapshot missing: %v", err)
	}
}

func TestStartRunsImmediatelyAndWaits(t *testing.T) {
	svc, _ := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	svc.Start(ctx)
	t.Cleanup(func() {
		cancel()
		svc.Wait()
	})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		files, err := svc.Files()
		if err != nil {
			t.Fatal(err)
		}
		if len(files) > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("startup tick did not create a backup")
}
