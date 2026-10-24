package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lmb666666/wisp/internal/media"
	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/unfurl"
	"github.com/lmb666666/wisp/internal/util"
)

var ErrNotFound = errors.New("资源不存在")

// AttachmentDTO 附件的对外表示。
type AttachmentDTO struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Mime      string `json:"mime"`
	URL       string `json:"url"`
	ThumbURL  string `json:"thumb_url,omitempty"`
	PosterURL string `json:"poster_url,omitempty"`
	Blurhash  string `json:"blurhash,omitempty"`
	Width     int64  `json:"width"`
	Height    int64  `json:"height"`
}

// PostBlock 扩展卡片：与正文分离存储，渲染在正文下方。
type PostBlock struct {
	Type     string   `json:"type"` // music | github | location | website | video
	URL      string   `json:"url,omitempty"`
	Name     string   `json:"name,omitempty"`
	Lat      *float64 `json:"lat,omitempty"`
	Lng      *float64 `json:"lng,omitempty"`
	Title    string   `json:"title,omitempty"`
	Cover    string   `json:"cover,omitempty"`
	Provider string   `json:"provider,omitempty"`
	AudioURL string   `json:"audio_url,omitempty"`
	EmbedURL string   `json:"embed_url,omitempty"`
}

// PostDTO 帖子的对外表示。
type PostDTO struct {
	ID          string          `json:"id"`
	ContentMD   string          `json:"content_md"`
	ContentHTML string          `json:"content_html"`
	Visibility  string          `json:"visibility"`
	Location    string          `json:"location,omitempty"`
	PublishedAt int64           `json:"published_at"`
	EditedAt    int64           `json:"edited_at,omitempty"`
	Source      string          `json:"source"`
	Client      string          `json:"client,omitempty"`
	Tags        []string        `json:"tags"`
	Attachments []AttachmentDTO `json:"attachments"`
	Blocks      []PostBlock     `json:"blocks"`
}

// PostService 帖子的业务逻辑（渲染、标签、附件装配）。
type PostService struct {
	q         repo.Querier
	db        *sql.DB // 事务用（Create/Update 的多步写入）
	publicURL string
	fetch     unfurl.Fetcher // 用户内容 URL：SSRF 防护
	trusted   unfurl.Fetcher // 管理员自配置端点（Meting）：无内网限制
	notify    func(event string, payload any)

	metingMu  sync.RWMutex
	metingAPI string
}

// prefetchSem 限制并发预取 goroutine（导入大批量时防止洪峰）。
var prefetchSem = make(chan struct{}, 4)

func NewPostService(db *sql.DB, q repo.Querier, publicURL, metingAPI string) *PostService {
	return &PostService{
		db: db, q: q, publicURL: publicURL, metingAPI: metingAPI,
		fetch:   unfurl.NewFetcher(),
		trusted: unfurl.NewUnrestrictedFetcher(),
	}
}

// SetMetingAPI 运行时更新音乐解析端点（设置页保存后立即生效）。
func (s *PostService) SetMetingAPI(url string) {
	s.metingMu.Lock()
	s.metingAPI = url
	s.metingMu.Unlock()
}

func (s *PostService) getMetingAPI() string {
	s.metingMu.RLock()
	defer s.metingMu.RUnlock()
	return s.metingAPI
}

// WithNotifier 注入事件回调（webhook 分发器），Create/Update/Delete 成功后触发。
func (s *PostService) WithNotifier(fn func(event string, payload any)) {
	s.notify = fn
}

func (s *PostService) emit(event string, payload any) {
	if s.notify != nil {
		s.notify(event, payload)
	}
}

// prefetchLinks 发布/编辑后异步抓取正文链接与扩展卡片的预览数据（不阻塞、失败静默、并发上限 4）。
func (s *PostService) prefetchLinks(contentHTML string, extra []string) {
	urls := append(unfurl.BareURLs(contentHTML), extra...)
	if len(urls) == 0 {
		return
	}
	go func() {
		prefetchSem <- struct{}{}
		defer func() { <-prefetchSem }()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, u := range urls {
			if _, err := s.q.GetLinkPreview(ctx, u); err == nil {
				continue // 已有缓存
			}
			pv, err := unfurl.Resolve(ctx, u, s.getMetingAPI(), s.fetch, s.trusted)
			if err != nil || pv == nil {
				continue
			}
			_ = s.q.UpsertLinkPreview(ctx, repo.UpsertLinkPreviewParams{
				Url: u, Provider: pv.Provider, Title: pv.Title,
				Cover: pv.Cover, EmbedHtml: pv.EmbedHTML, FetchedAt: time.Now().Unix(),
			})
		}
	}()
}

type CreateParams struct {
	ContentMD     string
	Location      string
	PublishedAt   int64 // 0 = 现在（支持补发过去的时刻）
	Source        string
	Client        string
	AttachmentIDs []string
	Blocks        []PostBlock
	Tags          []string // 显式标签（导入 frontmatter 用；正文 #tag 仍会解析）
}

// Create 发帖：渲染 HTML、提取标签、绑定附件。
func (s *PostService) Create(ctx context.Context, authorID int64, p CreateParams) (*PostDTO, error) {
	blocks := sanitizeBlocks(p.Blocks)
	if strings.TrimSpace(p.ContentMD) == "" && len(p.AttachmentIDs) == 0 && len(blocks) == 0 {
		return nil, errors.New("内容不能为空")
	}
	html, err := RenderHTML(p.ContentMD)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	if p.PublishedAt <= 0 {
		p.PublishedAt = now
	}
	if p.Source == "" {
		p.Source = "api"
	}

	attIDs := make([]string, 0, len(p.AttachmentIDs))
	seenAtt := map[string]bool{}
	for _, id := range p.AttachmentIDs {
		if id == "" || seenAtt[id] {
			continue
		}
		att, err := s.q.GetAttachment(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("附件不存在: %s", id)
		}
		if att.PostID.Valid {
			return nil, fmt.Errorf("附件已绑定其他帖子: %s", id)
		}
		seenAtt[id] = true
		attIDs = append(attIDs, id)
	}
	if strings.TrimSpace(p.ContentMD) == "" && len(blocks) == 0 && len(attIDs) == 0 {
		return nil, errors.New("内容不能为空")
	}

	post, err := s.createWithDeps(ctx, authorID, p, html, blocks, attIDs)
	if err != nil {
		return nil, err
	}

	s.prefetchLinks(post.ContentHtml, blockURLs(blocks))
	dto, err := s.assemble(ctx, post)
	if err == nil {
		s.emit("post.created", dto)
	}
	return dto, err
}

// createWithDeps 在单个事务里完成：建帖、标签、附件绑定；任一步失败整体回滚。
func (s *PostService) createWithDeps(ctx context.Context, authorID int64, p CreateParams, html string, blocks []PostBlock, attIDs []string) (repo.Post, error) {
	now := time.Now().Unix()
	blocksJSON, _ := json.Marshal(blocks)
	post := repo.Post{}
	err := s.inTx(ctx, func(txq repo.Querier) error {
		created, err := txq.CreatePost(ctx, repo.CreatePostParams{
			ID:          util.NewULID(),
			UserID:      authorID,
			ContentMd:   p.ContentMD,
			ContentHtml: html,
			Visibility:  "public",
			Location:    p.Location,
			PublishedAt: p.PublishedAt,
			Source:      p.Source,
			Client:      p.Client,
			Blocks:      string(blocksJSON),
			CreatedAt:   now,
		})
		if err != nil {
			return fmt.Errorf("create post: %w", err)
		}
		post = created
		if err := s.attachTagsIn(ctx, txq, created.ID, mergeTags(p.ContentMD, p.Tags)); err != nil {
			return err
		}
		for _, attID := range attIDs {
			if err := txq.AttachToPost(ctx, repo.AttachToPostParams{PostID: sql.NullString{String: post.ID, Valid: true}, ID: attID}); err != nil {
				return fmt.Errorf("attach %s: %w", attID, err)
			}
		}
		return nil
	})
	return post, err
}

// inTx 在事务中执行 fn；错误自动回滚。单写连接 SQLite 下仍保证原子可见。
func (s *PostService) inTx(ctx context.Context, fn func(txq repo.Querier) error) error {
	if s.db == nil {
		return fn(s.q)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	txq := repo.Querier(s.q)
	if qq, ok := s.q.(*repo.Queries); ok {
		txq = qq.WithTx(tx)
	}
	if err := fn(txq); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// mergeTags 合并正文与显式标签：去重、裁空白、单标签 ≤64 字符、总数 ≤16（防导入超量）。
func mergeTags(contentMD string, explicit []string) []string {
	out := make([]string, 0, len(explicit)+8)
	seen := map[string]bool{}
	add := func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] || len([]rune(tag)) > 64 {
			return
		}
		if len(out) >= 16 {
			return
		}
		seen[tag] = true
		out = append(out, tag)
	}
	for _, tag := range ExtractTags(contentMD) {
		add(tag)
	}
	for _, tag := range explicit {
		add(tag)
	}
	return out
}

// attachTagsIn 用给定查询集绑定标签（事务内复用）。
func (s *PostService) attachTagsIn(ctx context.Context, q repo.Querier, postID string, tags []string) error {
	for _, tag := range tags {
		tg, err := q.UpsertTag(ctx, tag)
		if err != nil {
			return fmt.Errorf("upsert tag %s: %w", tag, err)
		}
		if err := q.LinkPostTag(ctx, repo.LinkPostTagParams{PostID: postID, TagID: tg.ID}); err != nil {
			return fmt.Errorf("link tag %s: %w", tag, err)
		}
	}
	return nil
}

type UpdateParams struct {
	ID          string
	ContentMD   *string
	Location    *string
	PublishedAt *int64
	Blocks      *[]PostBlock
}

// Update 编辑帖子；内容变化时重渲染并重建标签。
func (s *PostService) Update(ctx context.Context, p UpdateParams) (*PostDTO, error) {
	post, err := s.q.GetPost(ctx, p.ID)
	if err != nil {
		return nil, ErrNotFound
	}

	params := repo.UpdatePostParams{ID: p.ID, EditedAt: util.NullInt64(time.Now())}
	if p.ContentMD != nil && *p.ContentMD != post.ContentMd {
		html, err := RenderHTML(*p.ContentMD)
		if err != nil {
			return nil, err
		}
		params.ContentMd = sql.NullString{String: *p.ContentMD, Valid: true}
		params.ContentHtml = sql.NullString{String: html, Valid: true}
	}
	if p.Location != nil {
		params.Location = sql.NullString{String: *p.Location, Valid: true}
	}
	if p.PublishedAt != nil {
		if *p.PublishedAt <= 0 {
			return nil, errors.New("非法发布时间")
		}
		params.PublishedAt = sql.NullInt64{Int64: *p.PublishedAt, Valid: true}
	}
	var updatedBlocks []PostBlock
	if p.Blocks != nil {
		updatedBlocks = sanitizeBlocks(*p.Blocks)
		blob, _ := json.Marshal(updatedBlocks)
		params.Blocks = util.NullStr(string(blob))
	}
	// 无任何字段变化时不写入（不触碰 edited_at、不触发 FTS）。
	if !params.ContentMd.Valid && !params.Location.Valid &&
		!params.PublishedAt.Valid && !params.Blocks.Valid {
		return s.assemble(ctx, post)
	}

	// UpdatePost + 标签重建在同一事务：中途失败不丢旧标签。
	var updated repo.Post
	err = s.inTx(ctx, func(txq repo.Querier) error {
		up, err := txq.UpdatePost(ctx, params)
		if err != nil {
			return fmt.Errorf("update post: %w", err)
		}
		updated = up
		if params.ContentMd.Valid {
			// 保留不在旧正文里的标签（胶囊 frontmatter/显式标签），正文标签按新内容重建。
			oldBodyTags := map[string]bool{}
			for _, tag := range ExtractTags(post.ContentMd) {
				oldBodyTags[tag] = true
			}
			rows, err := txq.ListPostTags(ctx, p.ID)
			if err != nil {
				return err
			}
			var preserved []string
			for _, tg := range rows {
				if !oldBodyTags[tg.Name] {
					preserved = append(preserved, tg.Name)
				}
			}
			if err := txq.ClearPostTags(ctx, p.ID); err != nil {
				return err
			}
			return s.attachTagsIn(ctx, txq, p.ID, mergeTags(params.ContentMd.String, preserved))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	post = updated
	s.prefetchLinks(updated.ContentHtml, blockURLs(updatedBlocks))
	dto, err := s.assemble(ctx, updated)
	if err == nil {
		s.emit("post.updated", dto)
	}
	return dto, err
}

// Delete 删除帖子并尽力清理附件文件（本地驱动）。
func (s *PostService) Delete(ctx context.Context, id string, store media.Storage, local *media.LocalStorage) error {
	atts, err := s.q.ListPostAttachments(ctx, util.NullStr(id))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("list attachments: %w", err)
	}
	if err := s.q.DeletePost(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("delete post: %w", err)
	}
	s.emit("post.deleted", map[string]string{"id": id})
	for _, a := range atts {
		for _, key := range []string{a.StorageKey, a.ThumbPath} {
			if key == "" {
				continue
			}
			_ = store.Delete(ctx, key)
			if local != nil {
				_ = local.Delete(ctx, key) // 切换存储后清理旧本地副本
			}
		}
		if a.PosterPath != "" && local != nil {
			_ = local.Delete(ctx, a.PosterPath)
		}
	}
	return nil
}

// Get 按 id 取帖子（全部内容公开，可见性过滤已随私密发布移除）。
func (s *PostService) Get(ctx context.Context, id string) (*PostDTO, error) {
	post, err := s.q.GetPost(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return s.assemble(ctx, post)
}

// Search 全文搜索：≥3 字符走 FTS5 trigram（短语匹配），短词回退 instr 字面匹配。
func (s *PostService) Search(ctx context.Context, query string, limit int) ([]*PostDTO, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	} else if limit > 50 {
		limit = 50
	}
	before := time.Now().Add(time.Minute).Unix()

	var (
		posts []repo.Post
		err   error
	)
	if len([]rune(query)) >= 3 {
		match := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
		posts, err = s.q.SearchPosts(ctx, repo.SearchPostsParams{ContentMd: match, PublishedAt: before, Limit: int64(limit)})
	} else {
		posts, err = s.q.SearchPostsBySubstring(ctx, repo.SearchPostsBySubstringParams{INSTR: query, PublishedAt: before, Limit: int64(limit)})
	}
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	dtos := make([]*PostDTO, 0, len(posts))
	for _, post := range posts {
		dto, err := s.assemble(ctx, post)
		if err != nil {
			return nil, err
		}
		dtos = append(dtos, dto)
	}
	return dtos, nil
}

type ListParams struct {
	Tag      string
	Tags     []string // ALL 语义：帖子需同时包含全部标签
	From, To int64    // unix 秒，含边界；0 表示不限
	Before   int64    // 游标：published_at <
	BeforeID string   // 游标第二维：同秒内按 id 倒序续页（复合游标，同秒不丢帖）
	Limit    int
}

// maxULID 大于任何合法 ULID（ULID 字母表中 'Z' 最大），空游标第二维时使用。
const maxULID = "ZZZZZZZZZZZZZZZZZZZZZZZZZZ"

func (p *ListParams) normalize() {
	if p.Limit <= 0 || p.Limit > 50 {
		p.Limit = 20
	}
	if p.Before <= 0 {
		p.Before = time.Now().Add(time.Minute).Unix()
	}
	if p.BeforeID == "" {
		p.BeforeID = maxULID
	}
}

// List 时间线分页（游标式）。返回 items 与 has_more。
func (s *PostService) List(ctx context.Context, p ListParams) ([]*PostDTO, bool, error) {
	p.normalize()
	tags := make([]string, 0, len(p.Tags)+1)
	seen := map[string]bool{}
	for _, tag := range append(append([]string{}, p.Tags...), p.Tag) {
		tag = strings.TrimSpace(strings.TrimPrefix(tag, "#"))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	var (
		posts []repo.Post
		err   error
	)
	// 组合筛选（日期 / 多标签）走 json_each 参数化查询；默认时间线保持原查询。
	if len(tags) > 0 || p.From > 0 || p.To > 0 {
		tagsJSON, _ := json.Marshal(tags)
		posts, err = s.q.ListFilteredPosts(ctx, repo.ListFilteredPostsParams{
			FromTs:   p.From,
			ToTs:     p.To,
			TagCount: int64(len(tags)),
			TagsJson: string(tagsJSON),
			Before:   p.Before,
			BeforeID: p.BeforeID,
			Limit:    int64(p.Limit),
		})
		if err != nil {
			return nil, false, fmt.Errorf("list filtered posts: %w", err)
		}
		dtos := make([]*PostDTO, 0, len(posts))
		for _, post := range posts {
			dto, err := s.assemble(ctx, post)
			if err != nil {
				return nil, false, err
			}
			dtos = append(dtos, dto)
		}
		return dtos, len(posts) == p.Limit, nil
	}
	posts, err = s.q.ListPosts(ctx, repo.ListPostsParams{Before: p.Before, BeforeID: p.BeforeID, Limit: int64(p.Limit)})
	if err != nil {
		return nil, false, fmt.Errorf("list posts: %w", err)
	}
	dtos := make([]*PostDTO, 0, len(posts))
	for _, post := range posts {
		dto, err := s.assemble(ctx, post)
		if err != nil {
			return nil, false, err
		}
		dtos = append(dtos, dto)
	}
	return dtos, len(posts) == p.Limit, nil
}

// assemble 装配 DTO：标签 + 附件 + 卡片。
func (s *PostService) assemble(ctx context.Context, post repo.Post) (*PostDTO, error) {
	tagRows, err := s.q.ListPostTags(ctx, post.ID)
	if err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(tagRows))
	for _, t := range tagRows {
		tags = append(tags, t.Name)
	}

	attRows, err := s.q.ListPostAttachments(ctx, util.NullStr(post.ID))
	if err != nil {
		return nil, err
	}
	atts := make([]AttachmentDTO, 0, len(attRows))
	for _, a := range attRows {
		atts = append(atts, s.attachmentDTO(a))
	}

	dto := &PostDTO{
		ID:          post.ID,
		ContentMD:   post.ContentMd,
		ContentHTML: unfurl.Enrich(ctx, s.q, post.ContentHtml),
		Blocks:      s.resolveBlocks(ctx, post.Blocks),
		Visibility:  "public",
		Location:    post.Location,
		PublishedAt: post.PublishedAt,
		Source:      post.Source,
		Client:      post.Client,
		Tags:        tags,
		Attachments: atts,
	}
	if post.EditedAt.Valid {
		dto.EditedAt = post.EditedAt.Int64
	}
	return dto, nil
}

func (s *PostService) attachmentURL(a repo.Attachment) string {
	if a.RemoteUrl != "" {
		return a.RemoteUrl
	}
	if a.StorageKey == "" {
		return ""
	}
	if a.Storage == "local" {
		return "/media/" + a.StorageKey // 相对路径，前端同域；RSS/OG 由调用方拼绝对地址
	}
	return a.StorageKey
}

func (s *PostService) attachmentDTO(a repo.Attachment) AttachmentDTO {
	poster := ""
	if a.PosterPath != "" {
		poster = "/media/" + a.PosterPath
	}
	return AttachmentDTO{
		ID:        a.ID,
		Kind:      a.Kind,
		Mime:      a.Mime,
		URL:       s.attachmentURL(a),
		ThumbURL:  s.thumbURL(a),
		PosterURL: poster,
		Blurhash:  a.Blurhash,
		Width:     a.Width,
		Height:    a.Height,
	}
}

func (s *PostService) thumbURL(a repo.Attachment) string {
	if a.ThumbPath == "" {
		return ""
	}
	if a.RemoteUrl != "" {
		if a.Storage == "s3" {
			return a.RemoteUrl + ".thumb.jpg" // S3 缩略图键为原 key + .thumb.jpg
		}
		return a.RemoteUrl // 图床场景缩略图策略由后缀约定
	}
	return "/media/" + a.ThumbPath
}

// 供 HTTP 层将附件行转 DTO（上传接口直接返回 AttachmentDTO）。
func AttachmentToDTO(a repo.Attachment, ps *PostService) AttachmentDTO {
	return ps.attachmentDTO(a)
}

// blockURLs 提取需要预取的扩展卡片链接。
func blockURLs(blocks []PostBlock) []string {
	out := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if b.URL != "" {
			out = append(out, b.URL)
		}
	}
	return out
}

// SanitizeBlocks 供导入层复用同一套清洗规则，保证指纹与落库一致。
func SanitizeBlocks(in []PostBlock) []PostBlock { return sanitizeBlocks(in) }

// sanitizeBlocks 只信任 type/url/name，丢弃客户端提交的展示字段。
func sanitizeBlocks(in []PostBlock) []PostBlock {
	out := make([]PostBlock, 0, len(in))
	seen := map[string]bool{}
	for _, b := range in {
		b.Title, b.Cover, b.Provider, b.AudioURL, b.EmbedURL = "", "", "", "", ""
		switch b.Type {
		case "music", "github", "website", "video":
			b.Name = ""
			if !strings.HasPrefix(b.URL, "http://") && !strings.HasPrefix(b.URL, "https://") {
				continue
			}
			if len(b.URL) > 2048 {
				continue
			}
			u, err := url.Parse(b.URL)
			if err != nil || u.Hostname() == "" {
				continue
			}
			switch b.Type {
			case "music":
				if _, _, _, err := unfurl.MusicSource(b.URL); err != nil {
					continue
				}
			case "github":
				if host := strings.ToLower(u.Hostname()); host != "github.com" && host != "www.github.com" {
					continue
				}
			case "video":
				if _, _, ok := unfurl.VideoEmbed(b.URL); !ok {
					continue
				}
			}
		case "location":
			b.Name = strings.TrimSpace(b.Name)
			if b.Name == "" || len([]rune(b.Name)) > 120 {
				continue
			}
			if b.Lat != nil && (*b.Lat < -90 || *b.Lat > 90) {
				b.Lat = nil
			}
			if b.Lng != nil && (*b.Lng < -180 || *b.Lng > 180) {
				b.Lng = nil
			}
		default:
			continue
		}
		key := b.Type + "|" + b.URL + "|" + b.Name
		if b.Type == "location" && b.Lat != nil && b.Lng != nil {
			key += fmt.Sprintf("|%.6f,%.6f", *b.Lat, *b.Lng)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, b)
		if len(out) >= 4 {
			break
		}
	}
	return out
}

// resolveBlocks 读取时补齐标题/封面/播放地址。
func (s *PostService) resolveBlocks(ctx context.Context, raw string) []PostBlock {
	if raw == "" {
		return []PostBlock{}
	}
	var blocks []PostBlock
	if err := json.Unmarshal([]byte(raw), &blocks); err != nil {
		return []PostBlock{}
	}
	meting := s.getMetingAPI()
	out := make([]PostBlock, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "music", "github", "website", "video":
			if pv, err := s.q.GetLinkPreview(ctx, b.URL); err == nil {
				b.Title, b.Cover, b.Provider = pv.Title, pv.Cover, pv.Provider
				if b.Type == "video" && (strings.HasPrefix(pv.EmbedHtml, "https://") || strings.HasPrefix(pv.EmbedHtml, "http://")) {
					b.EmbedURL = pv.EmbedHtml
				}
			}
			if b.Type == "video" && b.EmbedURL == "" {
				if embed, provider, ok := unfurl.VideoEmbed(b.URL); ok {
					b.EmbedURL = embed
					if b.Provider == "" {
						b.Provider = provider
					}
				}
			}
			if b.Type == "music" && meting != "" {
				if pv, err := s.q.GetLinkPreview(ctx, b.URL); err == nil &&
					(strings.HasPrefix(pv.EmbedHtml, "http://") || strings.HasPrefix(pv.EmbedHtml, "https://")) {
					b.AudioURL = pv.EmbedHtml
				} else if server, _, id, err := unfurl.MusicSource(b.URL); err == nil {
					b.AudioURL = unfurl.BuildMetingURL(meting, server, "url", id)
				}
			}
		}
		out = append(out, b)
	}
	return out
}
