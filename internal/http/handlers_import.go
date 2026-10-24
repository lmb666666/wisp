package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/importer"
	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/service"
	"github.com/lmb666666/wisp/internal/util"
)

const maxImportFile = 256 << 20
const maxImportPosts = 5000

// adminImport 数据导入：POST /admin/import（multipart：source + file）。
// 去重：正文非空按 content_md；纯卡片帖按 blocks 指纹。重复导入安全。
func (s *Server) adminImport(c echo.Context) error {
	source := c.FormValue("source")
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "缺少 file 字段")
	}
	if fh.Size > maxImportFile {
		return apiError(c, http.StatusRequestEntityTooLarge, "too_large", "文件超过 256MB")
	}
	f, err := fh.Open()
	if err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
	}
	defer f.Close()

	var posts []importer.Post
	if source == "wisp" {
		tmp, err := os.CreateTemp("", "wisp-capsule-*.zip")
		if err != nil {
			return s.internalError(c, err)
		}
		defer func() { _ = os.Remove(tmp.Name()) }()
		if _, err := io.Copy(tmp, f); err != nil {
			_ = tmp.Close()
			return apiError(c, http.StatusBadRequest, "bad_request", "读取文件失败")
		}
		_ = tmp.Close()
		posts, err = importer.ParseFile(source, tmp.Name())
		if err != nil {
			return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
		}
	} else {
		data, err := io.ReadAll(io.LimitReader(f, maxImportFile+1))
		if err != nil {
			return apiError(c, http.StatusBadRequest, "bad_request", "读取文件失败")
		}
		posts, err = importer.Parse(source, data)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "bad_request", err.Error())
		}
	}
	if len(posts) > maxImportPosts {
		return apiError(c, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("单次最多导入 %d 条（当前 %d 条），请分批", maxImportPosts, len(posts)))
	}

	ctx := c.Request().Context()
	existing := map[string]string{}
	if rows, err := s.q.ListPostFingerprints(ctx); err == nil {
		for _, row := range rows {
			// 与导入侧使用同一套 block 清洗，避免历史展示字段/超量卡片造成重复导入。
			var stored []service.PostBlock
			if row.Blocks != "" {
				_ = json.Unmarshal([]byte(row.Blocks), &stored)
			}
			raw, _ := json.Marshal(service.SanitizeBlocks(stored))
			if key := fingerprint(row.ContentMd, string(raw)); key != "" {
				existing[key] = row.ID
			}
		}
	}
	// 无正文/无卡片帖（常见图集）用附件 sha256 指纹去重。
	if rows, err := s.q.ListAttachmentOnlyFingerprints(ctx); err == nil {
		grouped := map[string][]string{}
		for _, row := range rows {
			if !row.PostID.Valid {
				continue
			}
			grouped[row.PostID.String] = append(grouped[row.PostID.String], row.Sha256)
		}
		for postID, hashes := range grouped {
			existing[filesKey(hashes)] = postID
		}
	}

	var imported, skipped, failed, filesImported, filesFailed int
	for _, p := range posts {
		blocks := service.SanitizeBlocks(convertBlocks(p.Blocks))
		blocksJSON, _ := json.Marshal(blocks)
		key := fingerprint(p.ContentMD, string(blocksJSON))
		if p.ContentMD == "" && len(blocks) == 0 && len(p.Files) > 0 {
			key = filesKey(fileHashes(p.Files))
		}
		if key == "" {
			continue
		}

		// 已存在同内容帖：补迁缺失的附件（重跑导入可补历史/部分媒体）
		if postID, ok := existing[key]; ok {
			skipped++
			if len(p.Files) > 0 {
				filesImported += s.backfillFiles(ctx, postID, p.Files, &filesFailed)
			}
			continue
		}

		attIDs := make([]string, 0, len(p.Files))
		for _, file := range p.Files {
			att, err := s.uploadOne(ctx, file)
			if err != nil {
				filesFailed++
				continue
			}
			attIDs = append(attIDs, att.ID)
			filesImported++
		}

		dto, err := s.posts.Create(ctx, s.ownerID(ctx), service.CreateParams{
			ContentMD:     p.ContentMD,
			Location:      p.Location,
			PublishedAt:   p.PublishedAt,
			Source:        "import",
			Client:        source,
			AttachmentIDs: attIDs,
			Blocks:        blocks,
			Tags:          p.Tags,
		})
		if err != nil {
			failed++
			continue
		}
		existing[key] = dto.ID
		imported++
	}
	return c.JSON(http.StatusOK, map[string]any{
		"source":         source,
		"total":          len(posts),
		"imported":       imported,
		"skipped":        skipped,
		"failed":         failed,
		"files_imported": filesImported,
		"files_failed":   filesFailed,
	})
}

// uploadOne 附件落库；外链附件（图床）只存 remote_url；视频封面写本地并回填。
func (s *Server) uploadOne(ctx context.Context, f importer.PostFile) (*repo.Attachment, error) {
	var (
		att *repo.Attachment
		err error
	)
	if f.Data == nil {
		if f.RemoteURL == "" {
			return nil, fmt.Errorf("附件缺少内容")
		}
		kind := f.Kind
		if kind == "" {
			kind = "file"
		}
		mime := f.Mime
		if mime == "" {
			mime = "application/octet-stream"
		}
		sum := sha256.Sum256([]byte(f.RemoteURL))
		row, err := s.q.CreateAttachment(ctx, repo.CreateAttachmentParams{
			ID:        util.NewULID(),
			Kind:      kind,
			Mime:      mime,
			Size:      0,
			Width:     f.Width,
			Height:    f.Height,
			Blurhash:  f.Blurhash,
			Storage:   "remote",
			RemoteUrl: f.RemoteURL,
			Sha256:    "remote:" + hex.EncodeToString(sum[:]),
			CreatedAt: time.Now().Unix(),
		})
		if err != nil {
			return nil, err
		}
		att = &row
	} else {
		att, err = s.uploader.UploadBytes(ctx, f.Name, f.Data, f.Mime)
		if err != nil {
			return nil, err
		}
	}
	if len(f.PosterData) > 0 && s.localStore != nil {
		key := posterKey(att.ID) // 远端附件也按 ID 存本地封面
		if err := s.localStore.Put(ctx, key, bytes.NewReader(f.PosterData), "image/jpeg"); err == nil {
			_ = s.q.SetAttachmentPoster(ctx, repo.SetAttachmentPosterParams{PosterPath: key, ID: att.ID})
			att.PosterPath = key
		}
	}
	return att, nil
}

// backfillFiles 给已存在帖子补挂缺失附件；返回新增文件数。
func (s *Server) backfillFiles(ctx context.Context, postID string, files []importer.PostFile, failed *int) int {
	rows, err := s.q.ListPostAttachments(ctx, util.NullStr(postID))
	if err != nil {
		*failed += len(files)
		return 0
	}
	have := map[string]bool{}
	for _, a := range rows {
		have[a.Sha256] = true
	}
	added := 0
	for _, f := range files {
		if have[fileSHA(f)] {
			continue
		}
		att, err := s.uploadOne(ctx, f)
		if err != nil {
			*failed++
			continue
		}
		if err := s.q.AttachToPost(ctx, repo.AttachToPostParams{
			PostID: util.NullStr(postID), ID: att.ID,
		}); err != nil {
			*failed++
			continue
		}
		have[att.Sha256] = true
		added++
	}
	return added
}

func fileSHA(f importer.PostFile) string {
	if f.Data != nil {
		sum := sha256.Sum256(f.Data)
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256([]byte(f.RemoteURL))
	return "remote:" + hex.EncodeToString(sum[:])
}

func fileHashes(files []importer.PostFile) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, fileSHA(f))
	}
	sort.Strings(out)
	return out
}

func filesKey(hashes []string) string {
	if len(hashes) == 0 {
		return ""
	}
	sorted := append([]string(nil), hashes...)
	sort.Strings(sorted)
	return "files:" + strings.Join(sorted, ",")
}

func convertBlocks(in []importer.Block) []service.PostBlock {
	out := make([]service.PostBlock, 0, len(in))
	for _, b := range in {
		out = append(out, service.PostBlock{
			Type: b.Type, URL: b.URL, Name: b.Name, Lat: b.Lat, Lng: b.Lng,
		})
	}
	return out
}

// fingerprint 纯文本按正文，纯卡片按 blocks JSON。
func fingerprint(contentMD, blocksJSON string) string {
	if contentMD != "" {
		return contentMD
	}
	if blocksJSON == "" || blocksJSON == "[]" || blocksJSON == "null" {
		return ""
	}
	return "blocks:" + blocksJSON
}

// ownerID 当前站点所有者（导入一律归属 owner）。
func (s *Server) ownerID(ctx context.Context) int64 {
	if owner, err := s.auth.Owner(ctx); err == nil {
		return owner.ID
	}
	return 1
}
