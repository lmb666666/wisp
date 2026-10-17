package importer

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// newTempSQLite 在临时文件里执行建表语句，返回字节（模拟用户上传的 db 文件）。
func newTempSQLite(t *testing.T, stmts []string) []byte {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "memo.db")
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(fmt.Errorf("exec %q: %w", s, err))
		}
	}
	_ = db.Close()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseMemosNewProto(t *testing.T) {
	data := []byte(`[
		{"name":"memos/1","content":"第一条 #测试","createTime":"2026-08-02T10:00:00Z","visibility":"PUBLIC"},
		{"name":"memos/2","content":"私密内容","createTime":"2026-08-03T10:00:00Z","visibility":"PRIVATE"}
	]`)
	posts, err := Parse("memos", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Fatalf("posts = %d", len(posts))
	}
	if posts[0].Visibility != "public" || posts[1].Visibility != "private" {
		t.Errorf("visibility = %s / %s", posts[0].Visibility, posts[1].Visibility)
	}
	if posts[0].PublishedAt == 0 {
		t.Error("createTime not parsed")
	}
}

func TestParseMemosLegacy(t *testing.T) {
	data := []byte(`[{"content":"旧版导出","createdTs":1754128800,"visibility":"PROTECTED"}]`)
	posts, err := Parse("memos", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].Visibility != "private" || posts[0].PublishedAt != 1754128800 {
		t.Fatalf("posts = %+v", posts)
	}
}

func TestParseMemosWrapped(t *testing.T) {
	data := []byte(`{"memos":[{"content":"包一层数组","createdTs":1754128800}]}`)
	posts, err := Parse("memos", data)
	if err != nil || len(posts) != 1 {
		t.Fatalf("posts = %d err = %v", len(posts), err)
	}
}

func TestParseMomentsDB(t *testing.T) {
	// 构造与 moments 同构的 Memo 表
	db := newTempSQLite(t, []string{
		`CREATE TABLE "Memo" (id INTEGER PRIMARY KEY, content TEXT, createdAt DATETIME, showType INTEGER DEFAULT 1, location TEXT)`,
		`INSERT INTO "Memo" (content, createdAt, showType, location) VALUES ('第一条 moments', '2026-08-02 10:00:00', 1, '上海')`,
		`INSERT INTO "Memo" (content, createdAt, showType) VALUES ('私密 moments', '2026-08-03 12:30:00', 2)`,
	})
	posts, err := Parse("moments", db)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Fatalf("posts = %d", len(posts))
	}
	if posts[0].Visibility != "public" || posts[1].Visibility != "private" {
		t.Errorf("visibility = %s / %s", posts[0].Visibility, posts[1].Visibility)
	}
	d := newDateFromUnix(posts[0].PublishedAt)
	if d.Month() != 8 || d.Day() != 2 {
		t.Errorf("published_at = %v", d)
	}
	if posts[0].Location != "上海" {
		t.Errorf("location = %q", posts[0].Location)
	}
}

func TestParseEch0Zip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range []struct{ name, body string }{
		{"capsule/ech0.yaml", "site: demo\n"},
		{"capsule/echoes/2026/2026-08-02-01.md", "---\nid: a\ncreated_at: 2026-08-02T10:00:00Z\n---\n第一条 Echo"},
		{"capsule/echoes/2026/2026-08-03-02.md", "无 frontmatter 的 Echo"},
		{"capsule/files/images/x.png", "binary"},
	} {
		w, _ := zw.Create(f.name)
		_, _ = w.Write([]byte(f.body))
	}
	_ = zw.Close()

	posts, err := Parse("ech0", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Fatalf("posts = %d", len(posts))
	}
	if posts[0].ContentMD != "第一条 Echo" || posts[0].Visibility != "public" {
		t.Fatalf("posts[0] = %+v", posts[0])
	}
	d := newDateFromUnix(posts[0].PublishedAt)
	if d.Month() != 8 || d.Day() != 2 {
		t.Errorf("published_at = %v", d)
	}
	if posts[1].ContentMD != "无 frontmatter 的 Echo" {
		t.Errorf("posts[1] = %+v", posts[1])
	}
}

func newDateFromUnix(unix int64) (d dateOnly) {
	t := time.Unix(unix, 0)
	return dateOnly{y: t.Year(), m: int(t.Month()), day: t.Day()}
}

type dateOnly struct {
	y, m, day int
}

func (d dateOnly) Month() int { return d.m }
func (d dateOnly) Day() int   { return d.day }
func (d dateOnly) Year() int  { return d.y }

func writeWispZip(t *testing.T, manifest string, entries map[string][]byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "capsule-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if manifest != "" {
		w, err := zw.Create("wisp.yaml")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(manifest))
	}
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestParseWispCapsuleValidation(t *testing.T) {
	post := []byte("---\nid: 01M\npublished_at: 1750000000\nvisibility: public\n---\n\nhello\n")
	postPath := "posts/2026/10/01M.md"

	valid := writeWispZip(t, "version: 1\nexported_at: 1750000000\n", map[string][]byte{postPath: post})
	posts, err := ParseFile("wisp", valid)
	if err != nil || len(posts) != 1 || posts[0].ContentMD != "hello" {
		t.Fatalf("valid capsule = %d %v", len(posts), err)
	}

	future := writeWispZip(t, "version: 2\n", map[string][]byte{postPath: post})
	if _, err := ParseFile("wisp", future); err == nil || !strings.Contains(err.Error(), "版本") {
		t.Fatalf("future version should fail, got %v", err)
	}

	missing := writeWispZip(t, "", map[string][]byte{postPath: post})
	if _, err := ParseFile("wisp", missing); err == nil || !strings.Contains(err.Error(), "wisp.yaml") {
		t.Fatalf("missing manifest should fail, got %v", err)
	}

	big := append([]byte("---\nid: 01M\n---\n\n"), bytes.Repeat([]byte("x"), maxCapsulePost+1)...)
	oversized := writeWispZip(t, "version: 1\n", map[string][]byte{postPath: big})
	if _, err := ParseFile("wisp", oversized); err == nil || !strings.Contains(err.Error(), "上限") {
		t.Fatalf("oversized post should fail, got %v", err)
	}
}
