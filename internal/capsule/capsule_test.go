package capsule

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wispdb "github.com/lmb666666/wisp/internal/db"
	"github.com/lmb666666/wisp/internal/media"
	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/util"
)

func TestSafeEntry(t *testing.T) {
	valid := []string{"wisp.yaml", "posts/2026/11/x.md", "files/2026/11/a.jpg"}
	for _, name := range valid {
		if !safeEntry(name) {
			t.Errorf("%q 应合法", name)
		}
	}
	invalid := []string{"", "/abs", "../escape", "a/../b", "a//b", `a\b`, "./x"}
	for _, name := range invalid {
		if safeEntry(name) {
			t.Errorf("%q 应被拒绝", name)
		}
	}
}

func TestExportWritesManifestPostsAndMedia(t *testing.T) {
	dir := t.TempDir()
	database, err := wispdb.Open(filepath.Join(dir, "capsule.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := wispdb.Migrate(database, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	q := repo.New(database)
	ctx := context.Background()
	now := time.Unix(1762300000, 0)

	user, err := q.CreateUser(ctx, repo.CreateUserParams{
		Username: "owner", PasswordHash: "x", IsOwner: 1, CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	post, err := q.CreatePost(ctx, repo.CreatePostParams{
		ID: util.NewULID(), UserID: user.ID, ContentMd: "内容 #导出", ContentHtml: "<p>内容</p>",
		Visibility: "public", Location: "上海", PublishedAt: now.Unix(), Source: "web", CreatedAt: now.Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	tag, err := q.UpsertTag(ctx, "导出")
	if err != nil {
		t.Fatal(err)
	}
	if err := q.LinkPostTag(ctx, repo.LinkPostTagParams{PostID: post.ID, TagID: tag.ID}); err != nil {
		t.Fatal(err)
	}

	local, err := media.NewLocalStorage(filepath.Join(dir, "media"))
	if err != nil {
		t.Fatal(err)
	}
	const key = "2026/11/photo.jpg"
	if err := local.Put(ctx, key, strings.NewReader("jpeg-data"), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	att, err := q.CreateAttachment(ctx, repo.CreateAttachmentParams{
		ID: util.NewULID(), Kind: "image", Mime: "image/jpeg", Size: 9,
		Storage: "local", StorageKey: key, Sha256: "sha", CreatedAt: now.Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.AttachToPost(ctx, repo.AttachToPostParams{
		PostID: sql.NullString{String: post.ID, Valid: true}, ID: att.ID,
	}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	stats, err := Export(ctx, q, local, &buf, Options{SourceURL: "https://wisp.example"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Posts != 1 || stats.Attachments != 1 || stats.MediaBytes != 9 {
		t.Fatalf("stats = %+v", stats)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		entries[f.Name] = b
	}

	manifest := string(entries["wisp.yaml"])
	if !strings.Contains(manifest, "version: 1") || !strings.Contains(manifest, "source_url: https://wisp.example") {
		t.Fatalf("wisp.yaml = %s", manifest)
	}
	published := time.Unix(post.PublishedAt, 0).UTC()
	postName := fmt.Sprintf("posts/%04d/%02d/%s.md", published.Year(), int(published.Month()), post.ID)
	body, ok := entries[postName]
	if !ok {
		t.Fatalf("缺少帖子文件 %s；实际条目 %v", postName, keys(entries))
	}
	if !strings.Contains(string(body), "tags:") || !strings.Contains(string(body), "内容 #导出") {
		t.Fatalf("帖子内容 = %s", body)
	}
	if got := string(entries["files/"+key]); got != "jpeg-data" {
		t.Fatalf("媒体内容 = %q", got)
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
