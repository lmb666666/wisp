package media

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/util"

	wispdb "github.com/lmb666666/wisp/internal/db"
)

func newTestUploader(t *testing.T) (*Uploader, repo.Querier, *LocalStorage) {
	t.Helper()
	dir := t.TempDir()
	database, err := wispdb.Open(filepath.Join(dir, "media.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := wispdb.Migrate(database, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	q := repo.New(database)
	local, err := NewLocalStorage(filepath.Join(dir, "media"))
	if err != nil {
		t.Fatal(err)
	}
	return NewUploader(NewSelector(local), q), q, local
}

func TestUploadBytesDedupesOnlyUnboundAttachments(t *testing.T) {
	uploader, q, _ := newTestUploader(t)
	ctx := context.Background()
	data := []byte("hello wisp")

	first, err := uploader.UploadBytes(ctx, "note.txt", data, "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	second, err := uploader.UploadBytes(ctx, "note.txt", data, "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("同内容未绑定附件应秒传: %s != %s", first.ID, second.ID)
	}

	// 绑定到帖子后再上传同内容：必须生成新附件，不能复用已绑定行。
	now := time.Now().Unix()
	user, err := q.CreateUser(ctx, repo.CreateUserParams{
		Username: "owner", PasswordHash: "x", IsOwner: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	post, err := q.CreatePost(ctx, repo.CreatePostParams{
		ID: util.NewULID(), UserID: user.ID, ContentMd: "p", ContentHtml: "<p>p</p>",
		Visibility: "public", PublishedAt: now, Source: "test", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.AttachToPost(ctx, repo.AttachToPostParams{
		PostID: sql.NullString{String: post.ID, Valid: true}, ID: first.ID,
	}); err != nil {
		t.Fatal(err)
	}

	third, err := uploader.UploadBytes(ctx, "note.txt", data, "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if third.ID == first.ID {
		t.Fatal("已绑定附件的相同内容不应秒传")
	}
}

func TestUploadBytesDowngradesDisguisedHTML(t *testing.T) {
	uploader, _, _ := newTestUploader(t)

	att, err := uploader.UploadBytes(context.Background(), "photo.png",
		[]byte("<html><script>alert(1)</script></html>"), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if att.Mime != "application/octet-stream" {
		t.Fatalf("mime = %q, want application/octet-stream", att.Mime)
	}
	if att.Kind != "file" {
		t.Fatalf("kind = %q, want file", att.Kind)
	}
}

func TestUploadBytesStoresOriginalAndThumbnail(t *testing.T) {
	uploader, _, local := newTestUploader(t)
	ctx := context.Background()

	att, err := uploader.UploadBytes(ctx, "note.txt", []byte("plain text"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	rc, err := local.Get(ctx, att.StorageKey)
	if err != nil {
		t.Fatalf("原文件未落盘: %v", err)
	}
	_ = rc.Close()
	if att.Storage != "local" {
		t.Fatalf("storage = %q, want local", att.Storage)
	}
	if att.RemoteUrl != "" {
		t.Fatalf("本地存储不应写 remote_url: %q", att.RemoteUrl)
	}
}
