package repo

import "database/sql"

// StrNull 把字符串转为可空文本参数（sqlc 生成的 ? 参数）。
func StrNull(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}
