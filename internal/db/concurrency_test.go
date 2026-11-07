package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newConcurrencyTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := Open(filepath.Join(t.TempDir(), "concurrency.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.Exec(`CREATE TABLE counter (v INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	return database
}

// 未提交的写事务不能阻塞读请求：否则一次大事务（导入、标签合并）或
// 备份的 VACUUM INTO 会让整个站点在事务期间不可读。
func TestReadIsNotBlockedByOpenWriteTransaction(t *testing.T) {
	database := newConcurrencyTestDB(t)
	ctx := context.Background()

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO counter (v) VALUES (1)`); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		var n int
		done <- database.QueryRowContext(ctx, `SELECT count(*) FROM counter`).Scan(&n)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("并发读失败: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("读请求被未提交的写事务阻塞：连接池仍是单连接")
	}
}

// 多连接下「先读后写」的事务必须能排队拿到写锁。
// 默认 deferred 事务在升级写锁时直接返回 SQLITE_BUSY（不走 busy_timeout），
// 因此 DSN 必须带 _txlock=immediate。
func TestConcurrentReadThenWriteTransactions(t *testing.T) {
	database := newConcurrencyTestDB(t)
	const workers = 16

	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			tx, err := database.BeginTx(ctx, nil)
			if err != nil {
				errCh <- fmt.Errorf("worker %d begin: %w", i, err)
				return
			}
			defer func() { _ = tx.Rollback() }()

			// 先读后写：模拟标签合并 / 建帖前校验的语句顺序。
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM counter`).Scan(&n); err != nil {
				errCh <- fmt.Errorf("worker %d read: %w", i, err)
				return
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO counter (v) VALUES (?)`, i); err != nil {
				errCh <- fmt.Errorf("worker %d write: %w", i, err)
				return
			}
			if err := tx.Commit(); err != nil {
				errCh <- fmt.Errorf("worker %d commit: %w", i, err)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}

	var n int
	if err := database.QueryRow(`SELECT count(*) FROM counter`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != workers {
		t.Fatalf("写入 %d 行，期望 %d 行", n, workers)
	}
}
