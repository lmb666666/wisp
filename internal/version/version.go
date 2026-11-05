// Package version 暴露构建版本信息（goreleaser 通过 ldflags 注入）。
package version

var (
	Version = "0.3.1"
	Commit  = ""
	Date    = ""
)
