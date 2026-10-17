package wisp

import "embed"

// MigrationsFS 内嵌 goose SQL 迁移文件（自带 migrations/ 前缀，
// goose.SetBaseFS 后按 "migrations" 目录使用）。
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
