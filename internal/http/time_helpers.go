package http

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// 便于测试替换的时间函数。
var timeSince = time.Since

const (
	maxTZOffset = 14 * 60
	minTZOffset = -14 * 60
)

// parseTZOffset 解析客户端传来的 UTC 偏移分钟数；缺省/非法时回退服务端本地时区。
// 前端使用 -new Date().getTimezoneOffset()（东八区为 480）。
func parseTZOffset(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		n, err := strconv.Atoi(raw)
		if err == nil && n >= minTZOffset && n <= maxTZOffset {
			return n
		}
	}
	_, offset := time.Now().Zone()
	return offset / 60
}

func tzLocation(offsetMinutes int) *time.Location {
	return time.FixedZone("client", offsetMinutes*60)
}

// tzModifier 供 SQLite date(...) 使用，例如 "+480 minutes"。
func tzModifier(offsetMinutes int) string {
	return fmt.Sprintf("%+d minutes", offsetMinutes)
}

func yearBounds(year, offsetMinutes int) (int64, int64) {
	loc := tzLocation(offsetMinutes)
	return time.Date(year, 1, 1, 0, 0, 0, 0, loc).Unix(),
		time.Date(year+1, 1, 1, 0, 0, 0, 0, loc).Unix()
}

func todayBounds(offsetMinutes int) (int64, int64) {
	now := time.Now().In(tzLocation(offsetMinutes))
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix(),
		time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()).Unix()
}
