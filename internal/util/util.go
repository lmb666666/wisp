// Package util 提供跨模块的小工具：ULID 生成、可空列映射。
package util

import (
	"crypto/rand"
	"database/sql"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewULID 生成按时间排序的 ULID 字符串（posts/attachments/api_tokens 主键）。
// 熵源用 crypto/rand：ulid.DefaultEntropy 基于 math/rand（时间种子），ID 可预测枚举；
// crypto/rand.Reader 并发安全，且无跨调用状态。
func NewULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// NullInt64 把时间转为可空 unix 秒列。
func NullInt64(t time.Time) sql.NullInt64 {
	return sql.NullInt64{Int64: t.Unix(), Valid: true}
}

// NullStr 把字符串转为可空文本列。
func NullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
