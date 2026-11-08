// Package mood 定义每日心情的取值与日期校验。
package mood

import (
	"regexp"
	"time"
)

const (
	Happy = "happy"
	Calm  = "calm"
	Okay  = "okay"
	Tired = "tired"
	Low   = "low"
)

// valid 是受支持的心情值集合；前端按 happy→low 的顺序展示。
var valid = map[string]bool{
	Happy: true,
	Calm:  true,
	Okay:  true,
	Tired: true,
	Low:   true,
}

// Valid 判断是否为受支持的心情值。
func Valid(value string) bool { return valid[value] }

// datePattern 严格匹配 YYYY-MM-DD；time.Parse 对零填充较宽松，先用正则挡一层。
var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ValidDate 校验真实存在的日历日期（2026-02-30 不合法）。
func ValidDate(value string) bool {
	if !datePattern.MatchString(value) {
		return false
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}
