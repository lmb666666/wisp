package service

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wispdb "github.com/lmb666666/wisp/internal/db"
	"github.com/lmb666666/wisp/internal/media"
	"github.com/lmb666666/wisp/internal/repo"
)

func newTestPostService(t *testing.T) (*PostService, repo.Querier, *sql.DB, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := wispdb.Open(filepath.Join(dir, "service.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := wispdb.Migrate(database, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}

	q := repo.New(database)
	now := time.Now().Unix()
	user, err := q.CreateUser(context.Background(), repo.CreateUserParams{
		Username: "owner", PasswordHash: "x", DisplayName: "Owner", IsOwner: 1,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewPostService(database, q, "http://test.local", ""), q, database, user.ID
}

func assertTagSet(t *testing.T, got []string, want ...string) {
	t.Helper()
	set := map[string]bool{}
	for _, tag := range got {
		set[tag] = true
	}
	if len(set) != len(want) {
		t.Fatalf("tags = %v, want %v", got, want)
	}
	for _, tag := range want {
		if !set[tag] {
			t.Fatalf("tags = %v, want %v", got, want)
		}
	}
}

func testAttachment(t *testing.T, q repo.Querier, key string) repo.Attachment {
	t.Helper()
	row, err := q.CreateAttachment(context.Background(), repo.CreateAttachmentParams{
		ID: key + "-id", Kind: "image", Mime: "image/jpeg", Size: 10,
		Storage: "local", StorageKey: key, ThumbPath: key + ".thumb.jpg",
		Sha256: key, CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func TestCreateParsesAndDedupesTags(t *testing.T) {
	svc, _, _, uid := newTestPostService(t)
	ctx := context.Background()

	dto, err := svc.Create(ctx, uid, CreateParams{
		ContentMD: "#日记 今天写了点东西 #代码\n\n#日记",
		Tags:      []string{" 日记 ", "手账", strings.Repeat("长", 65)},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertTagSet(t, dto.Tags, "日记", "代码", "手账")
}

func TestCreateRejectsEmptyContent(t *testing.T) {
	svc, _, _, uid := newTestPostService(t)

	if _, err := svc.Create(context.Background(), uid, CreateParams{ContentMD: "   "}); err == nil {
		t.Fatal("空内容应当失败")
	}
}

func TestCreateIdempotencyKeyIsSingleUse(t *testing.T) {
	svc, _, database, uid := newTestPostService(t)
	ctx := context.Background()

	first, err := svc.Create(ctx, uid, CreateParams{ContentMD: "只发一次", IdempotencyKey: "key-1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Create(ctx, uid, CreateParams{ContentMD: "只发一次", IdempotencyKey: "key-1"})
	if !errors.Is(err, ErrIdempotencyReplay) {
		t.Fatalf("重复幂等键 = %v, want ErrIdempotencyReplay", err)
	}

	var count int
	if err := database.QueryRow(`SELECT count(*) FROM posts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("posts = %d, want 1（重放不得新建帖子）", count)
	}

	if _, err := svc.Get(ctx, first.ID); err != nil {
		t.Fatalf("首次帖子应存在: %v", err)
	}
	if _, err := svc.Create(ctx, uid, CreateParams{ContentMD: "新键", IdempotencyKey: "key-2"}); err != nil {
		t.Fatalf("新幂等键应成功: %v", err)
	}
}

func TestCreateBindsAttachmentAndRejectsReuse(t *testing.T) {
	svc, q, _, uid := newTestPostService(t)
	ctx := context.Background()
	att := testAttachment(t, q, "2026/11/a.jpg")

	dto, err := svc.Create(ctx, uid, CreateParams{ContentMD: "带图", AttachmentIDs: []string{att.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(dto.Attachments) != 1 || dto.Attachments[0].ID != att.ID {
		t.Fatalf("attachments = %+v", dto.Attachments)
	}

	_, err = svc.Create(ctx, uid, CreateParams{ContentMD: "再绑一次", AttachmentIDs: []string{att.ID}})
	if err == nil || !strings.Contains(err.Error(), "已绑定") {
		t.Fatalf("复用已绑定附件 = %v, want 已绑定错误", err)
	}
}

func TestUpdateRebuildsHTMLAndTags(t *testing.T) {
	svc, _, _, uid := newTestPostService(t)
	ctx := context.Background()

	post, err := svc.Create(ctx, uid, CreateParams{ContentMD: "旧内容 #旧"})
	if err != nil {
		t.Fatal(err)
	}
	if post.EditedAt != 0 {
		t.Fatalf("新帖 edited_at = %d, want 0", post.EditedAt)
	}

	newContent := "新内容 #新"
	updated, err := svc.Update(ctx, UpdateParams{ID: post.ID, ContentMD: &newContent})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updated.ContentHTML, "新内容") {
		t.Fatalf("content_html 未重渲染: %q", updated.ContentHTML)
	}
	if updated.EditedAt == 0 {
		t.Fatal("编辑后 edited_at 应写入")
	}
	assertTagSet(t, updated.Tags, "新")
}

func TestUpdateNoopKeepsEditedAt(t *testing.T) {
	svc, _, _, uid := newTestPostService(t)
	ctx := context.Background()

	post, err := svc.Create(ctx, uid, CreateParams{ContentMD: "内容不变 #标签"})
	if err != nil {
		t.Fatal(err)
	}
	same := post.ContentMD
	updated, err := svc.Update(ctx, UpdateParams{ID: post.ID, ContentMD: &same})
	if err != nil {
		t.Fatal(err)
	}
	if updated.EditedAt != 0 {
		t.Fatalf("无变化编辑 touched edited_at = %d", updated.EditedAt)
	}
	if updated.PublishedAt != post.PublishedAt {
		t.Fatalf("published_at 被改动: %d -> %d", post.PublishedAt, updated.PublishedAt)
	}
}

func TestUpdatePreservesExplicitTagsNotInBody(t *testing.T) {
	svc, _, _, uid := newTestPostService(t)
	ctx := context.Background()

	post, err := svc.Create(ctx, uid, CreateParams{ContentMD: "正文", Tags: []string{"收藏"}})
	if err != nil {
		t.Fatal(err)
	}
	newContent := "新正文 #新"
	updated, err := svc.Update(ctx, UpdateParams{ID: post.ID, ContentMD: &newContent})
	if err != nil {
		t.Fatal(err)
	}
	assertTagSet(t, updated.Tags, "新", "收藏")
}

func TestUpdateRejectsNonPositivePublishedAt(t *testing.T) {
	svc, _, _, uid := newTestPostService(t)
	ctx := context.Background()

	post, err := svc.Create(ctx, uid, CreateParams{ContentMD: "正文"})
	if err != nil {
		t.Fatal(err)
	}
	bad := int64(0)
	if _, err := svc.Update(ctx, UpdateParams{ID: post.ID, PublishedAt: &bad}); err == nil {
		t.Fatal("published_at=0 应当失败")
	}
}

func TestSanitizeBlocksFiltersUntrustedFields(t *testing.T) {
	lat, lng := 200.0, 30.0
	blocks := SanitizeBlocks([]PostBlock{
		{Type: "website", URL: "https://example.com/a", Title: "evil", Cover: "evil", Provider: "evil"},
		{Type: "website", URL: "https://example.com/a"}, // 重复
		{Type: "website", URL: "javascript:alert(1)"},
		{Type: "github", URL: "https://gitlab.com/x/y"},
		{Type: "github", URL: "https://github.com/x/y", Name: "clear-me"},
		{Type: "location", Name: "  家里  ", Lat: &lat, Lng: &lng},
		{Type: "location", Name: "   "},
		{Type: "unknown", URL: "https://x.com"},
		{Type: "website", URL: "https://a.com"},
		{Type: "website", URL: "https://b.com"},
		{Type: "website", URL: "https://c.com"},
		{Type: "website", URL: "https://d.com"},
	})
	if len(blocks) != 4 {
		t.Fatalf("blocks = %+v, want 4", blocks)
	}
	if blocks[0].Title != "" || blocks[0].Cover != "" || blocks[0].Provider != "" {
		t.Fatalf("展示字段未清洗: %+v", blocks[0])
	}
	if blocks[1].Type != "github" || blocks[1].URL != "https://github.com/x/y" || blocks[1].Name != "" {
		t.Fatalf("github block = %+v", blocks[1])
	}
	loc := blocks[2]
	if loc.Type != "location" || loc.Name != "家里" {
		t.Fatalf("location block = %+v", loc)
	}
	if loc.Lat != nil {
		t.Fatalf("越界纬度应清空: %+v", loc)
	}
	if loc.Lng == nil || *loc.Lng != 30 {
		t.Fatalf("合法经度应保留: %+v", loc)
	}
	if blocks[3].URL != "https://a.com" {
		t.Fatalf("第 4 个有效卡片 = %+v", blocks[3])
	}
}

func TestListFiltersByAllTags(t *testing.T) {
	svc, _, _, uid := newTestPostService(t)
	ctx := context.Background()

	both, err := svc.Create(ctx, uid, CreateParams{ContentMD: "#a #b 两条都命中"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, uid, CreateParams{ContentMD: "#a 只有一条"}); err != nil {
		t.Fatal(err)
	}

	items, hasMore, err := svc.List(ctx, ListParams{Tags: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasMore || len(items) != 1 || items[0].ID != both.ID {
		t.Fatalf("双标签过滤 = %d 条 (hasMore=%v)", len(items), hasMore)
	}

	items, _, err = svc.List(ctx, ListParams{Tag: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("单标签过滤 = %d 条, want 2", len(items))
	}
}

func TestSearchFindsLongAndShortTerms(t *testing.T) {
	svc, _, _, uid := newTestPostService(t)
	ctx := context.Background()

	post, err := svc.Create(ctx, uid, CreateParams{ContentMD: "今天去公园散步"})
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"公园", "今天去公园"} {
		got, err := svc.Search(ctx, query, 10)
		if err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
		if len(got) != 1 || got[0].ID != post.ID {
			t.Fatalf("search %q = %d 条", query, len(got))
		}
	}
	if got, err := svc.Search(ctx, "  ", 10); err != nil || got != nil {
		t.Fatalf("空查询 = %v, %v", got, err)
	}
}

type recordingStore struct {
	deleted []string
}

func (r *recordingStore) Put(context.Context, string, io.Reader, string) error { return nil }
func (r *recordingStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}
func (r *recordingStore) Delete(_ context.Context, key string) error {
	r.deleted = append(r.deleted, key)
	return nil
}
func (r *recordingStore) PublicURL(key string) string { return "/media/" + key }

var _ media.Storage = (*recordingStore)(nil)

func TestDeleteRemovesPostAndMediaFiles(t *testing.T) {
	svc, q, _, uid := newTestPostService(t)
	ctx := context.Background()
	att := testAttachment(t, q, "2026/11/del.jpg")
	post, err := svc.Create(ctx, uid, CreateParams{ContentMD: "待删除", AttachmentIDs: []string{att.ID}})
	if err != nil {
		t.Fatal(err)
	}

	store := &recordingStore{}
	if err := svc.Delete(ctx, post.ID, store, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, post.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除后 Get = %v, want ErrNotFound", err)
	}
	want := map[string]bool{"2026/11/del.jpg": true, "2026/11/del.jpg.thumb.jpg": true}
	if len(store.deleted) != len(want) {
		t.Fatalf("删除的媒体 key = %v", store.deleted)
	}
	for _, key := range store.deleted {
		if !want[key] {
			t.Fatalf("意外删除 key = %q", key)
		}
	}
}
