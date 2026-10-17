package http

import "time"

// 便于测试替换的时间函数（概览/统计统一本地时区）。
var (
	timeSince    = time.Since
	startOfToday = func() int64 {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	}
	endOfToday = func() int64 {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()).Unix()
	}
)
