package util

import (
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
)

func TestNewULID(t *testing.T) {
	a := NewULID()
	b := NewULID()
	if len(a) != 26 {
		t.Fatalf("ULID 长度 = %d, want 26", len(a))
	}
	if _, err := ulid.Parse(a); err != nil {
		t.Fatalf("ULID 无法解析: %v", err)
	}
	if a == b {
		t.Fatal("连续生成出现重复 ULID")
	}
}

func TestNullHelpers(t *testing.T) {
	ts := time.Unix(1700000000, 0)
	n := NullInt64(ts)
	if !n.Valid || n.Int64 != 1700000000 {
		t.Fatalf("NullInt64 = %+v", n)
	}

	if got := NullStr(""); got.Valid {
		t.Fatalf("空串应为 NULL: %+v", got)
	}
	if got := NullStr("x"); !got.Valid || got.String != "x" {
		t.Fatalf("非空串 = %+v", got)
	}
}
