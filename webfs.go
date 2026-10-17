// Package wisp 提供前端构建产物的嵌入文件系统。
package wisp

import "embed"

//go:embed all:web/dist
var WebFS embed.FS
