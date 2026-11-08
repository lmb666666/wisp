package mood

import "testing"

func TestValid(t *testing.T) {
	for _, value := range []string{Happy, Calm, Okay, Tired, Low} {
		if !Valid(value) {
			t.Errorf("%q 应为合法心情", value)
		}
	}
	for _, value := range []string{"", "great", "HAPPY", "happy ", "unknown"} {
		if Valid(value) {
			t.Errorf("%q 应为非法心情", value)
		}
	}
}

func TestValidDate(t *testing.T) {
	valid := []string{"2000-01-01", "2024-02-29", "2026-11-07", "9999-12-31"}
	for _, value := range valid {
		if !ValidDate(value) {
			t.Errorf("%q 应为合法日期", value)
		}
	}
	invalid := []string{
		"", "2026-1-1", "2026-11-7", "2026-13-01", "2026-02-30",
		"2025-02-29", "2026/11/07", "2026-11-07T00:00:00Z", "abcd-ef-gh",
	}
	for _, value := range invalid {
		if ValidDate(value) {
			t.Errorf("%q 应为非法日期", value)
		}
	}
}
