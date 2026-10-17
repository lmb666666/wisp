// Package db 负责 SQLite 的打开与 schema 迁移。
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open 打开 SQLite 库。
// 固定打开 WAL、busy_timeout(5s)、foreign_keys、synchronous(NORMAL)；
// 连接池限制为单连接：个人量级写入极低，串行化是避免 SQLITE_BUSY 最稳妥的方式。
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)",
		path,
	)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqldb.SetMaxOpenConns(1)
	if err := sqldb.Ping(); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return sqldb, nil
}
