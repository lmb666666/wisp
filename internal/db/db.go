// Package db 负责 SQLite 的打开与 schema 迁移。
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// maxOpenConns 连接池上限。WAL 是「多读 + 单写」，读请求可以并行；
// 不再把整个进程压到一条连接上，否则一次长事务（导入、标签合并）或
// 备份的 VACUUM INTO 会让全站在事务期间无法响应任何请求。
const maxOpenConns = 8

// Open 打开 SQLite 库。
// 固定打开 WAL、busy_timeout(5s)、foreign_keys、synchronous(NORMAL)；
// 写事务统一以 BEGIN IMMEDIATE 开始（_txlock=immediate）：先排队拿写锁，
// 若用默认的 deferred，多连接下「先读后写」的事务升级锁时会直接返回
// SQLITE_BUSY，而不是等待 busy_timeout。
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)&_txlock=immediate",
		path,
	)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqldb.SetMaxOpenConns(maxOpenConns)
	sqldb.SetMaxIdleConns(maxOpenConns)
	if err := sqldb.Ping(); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return sqldb, nil
}

// HardenFiles 把数据库及其 WAL/SHM 收紧到 0600（不存在则跳过）。
// 数据库含密码/令牌哈希和存储密钥，不能随默认 umask 变成 0644。
func HardenFiles(path string) error {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(p, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("chmod %s: %w", p, err)
		}
	}
	return nil
}
