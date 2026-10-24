// Package importer 把 Moments / Ech0 / Memos 的导出物归一化为可导入的帖子列表。
// 设计约定：
//   - v1 只迁文本（content/时间/标签/可见性）；二进制附件留在原站，文档见 docs/MIGRATION.md
//   - 按内容去重（content_md 精确匹配），重复导入是安全的
package importer

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"

	"github.com/lmb666666/wisp/internal/capsule"
)

// Post 归一化后的导入条目。
type Post struct {
	ContentMD   string
	PublishedAt int64
	Visibility  string // public / private
	Location    string
	Source      string
	Tags        []string
	Blocks      []Block
	Files       []PostFile // 随帖附件
}

// Block 扩展卡片（Wisp capsule 使用）。
type Block struct {
	Type string   `yaml:"type" json:"type"`
	URL  string   `yaml:"url,omitempty" json:"url,omitempty"`
	Name string   `yaml:"name,omitempty" json:"name,omitempty"`
	Lat  *float64 `yaml:"lat,omitempty" json:"lat,omitempty"`
	Lng  *float64 `yaml:"lng,omitempty" json:"lng,omitempty"`
}

// PostFile 导入附件：原始字节或外链，附可选的视频封面。
type PostFile struct {
	Name       string
	Mime       string
	Kind       string
	Width      int64
	Height     int64
	Blurhash   string
	Data       []byte
	RemoteURL  string
	PosterData []byte
}

const maxImportSize = 64 << 20 // 64MB

func normalizeVisibility(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "private", "protected":
		return "private"
	case "unlisted":
		return "private" // 仅链接已移除；导入时按私密处理
	default:
		return "public"
	}
}

// ParseFile 从磁盘读取（wisp capsule 用流式 zip.Reader，避免大包全部进内存）。
func ParseFile(source, file string) ([]Post, error) {
	if source != "wisp" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		return Parse(source, data)
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		return nil, fmt.Errorf("打开 wisp capsule: %w", err)
	}
	return parseWispZip(zr)
}

// Parse 依据 source 分发解析。
func Parse(source string, data []byte) ([]Post, error) {
	if source == "wisp" {
		zr, err := zip.NewReader(bytesReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("打开 wisp capsule: %w", err)
		}
		return parseWispZip(zr)
	}
	if len(data) > maxImportSize {
		return nil, fmt.Errorf("文件超过 64MB 限制")
	}
	switch source {
	case "memos":
		return parseMemos(data)
	case "moments":
		return parseMomentsDB(data)
	case "ech0":
		return parseEch0Zip(data)
	default:
		return nil, fmt.Errorf("未知来源 %q（支持 memos / moments / ech0）", source)
	}
}

// ---------- Memos：官方导出 JSON ----------

func parseMemos(data []byte) ([]Post, error) {
	// 兼容三种形态：纯数组 / {"memos":[...]} / {"data":[...]}
	var raw []json.RawMessage
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("memos json: %w", err)
		}
	} else {
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(data, &wrapper); err != nil {
			return nil, fmt.Errorf("memos json: %w", err)
		}
		for _, key := range []string{"memos", "data", "items"} {
			if v, ok := wrapper[key]; ok {
				if err := json.Unmarshal(v, &raw); err != nil {
					return nil, fmt.Errorf("memos json: %w", err)
				}
				break
			}
		}
		if raw == nil {
			return nil, fmt.Errorf("memos json: 未找到 memos 数组")
		}
	}

	posts := make([]Post, 0, len(raw))
	for i, item := range raw {
		var m struct {
			Content    string `json:"content"`
			CreatedTs  int64  `json:"createdTs"`  // 旧版：unix 秒
			CreateTime string `json:"createTime"` // 新版（proto）：RFC3339
			CreatedAt  string `json:"createdAt"`
			UpdatedTs  int64  `json:"updatedTs"`
			UpdateTime string `json:"updateTime"`
			Visibility string `json:"visibility"`
		}
		if err := json.Unmarshal(item, &m); err != nil {
			return nil, fmt.Errorf("memos json 第 %d 条: %w", i+1, err)
		}
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		ts := m.CreatedTs
		if ts == 0 {
			ts = m.UpdatedTs
		}
		if ts == 0 {
			for _, s := range []string{m.CreateTime, m.CreatedAt, m.UpdateTime} {
				if s == "" {
					continue
				}
				if t, err := time.Parse(time.RFC3339, s); err == nil {
					ts = t.Unix()
					break
				}
			}
		}
		posts = append(posts, Post{
			ContentMD:   m.Content,
			PublishedAt: ts,
			Visibility:  normalizeVisibility(m.Visibility),
			Source:      "import",
		})
	}
	return posts, nil
}

// ---------- Moments：SQLite（表 Memo）----------

func parseMomentsDB(data []byte) ([]Post, error) {
	tmp, err := writeTemp(data, "wisp-import-*.db")
	if err != nil {
		return nil, err
	}
	defer removeTemp(tmp)

	db, err := sql.Open("sqlite", "file:"+tmp+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("打开 moments 数据库: %w", err)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`SELECT content, createdAt, showType, location FROM "Memo" ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询 Memo 表: %w（请确认上传的是 moments 的 db.sqlite）", err)
	}
	defer func() { _ = rows.Close() }()

	posts := make([]Post, 0, 256)
	for rows.Next() {
		var (
			content  string
			created  any
			showType int64
			location sql.NullString
		)
		if err := rows.Scan(&content, &created, &showType, &location); err != nil {
			return nil, fmt.Errorf("扫描 Memo 行: %w", err)
		}
		if strings.TrimSpace(content) == "" {
			continue
		}
		vis := "public"
		if showType != 1 {
			vis = "private"
		}
		posts = append(posts, Post{
			ContentMD:   content,
			PublishedAt: parseFlexibleTime(created),
			Visibility:  vis,
			Location:    location.String,
			Source:      "import",
		})
	}
	return posts, rows.Err()
}

// parseFlexibleTime：moments 的 createdAt 在不同 gorm 驱动下可能是 RFC3339 文本或 unix 时间。
func parseFlexibleTime(v any) int64 {
	switch t := v.(type) {
	case time.Time:
		return t.Unix()
	case int64:
		return t
	case float64:
		return int64(t)
	case string:
		s := strings.TrimSpace(t)
		if n, err := parseUnixLenient(s); err == nil {
			return n
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
			if tt, err := time.Parse(layout, s); err == nil {
				return tt.Unix()
			}
		}
	}
	return time.Now().Unix()
}

func parseUnixLenient(s string) (int64, error) {
	var n int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("not unix")
		}
		n = n*10 + int64(ch-'0')
	}
	if len(s) > 12 { // 毫秒
		n /= 1000
	}
	return n, nil
}

// ---------- Ech0：capsule zip（echoes/**/*.md，frontmatter）----------

var (
	ech0CreatedRe = regexp.MustCompile(`(?m)^created_at:\s*(.+)$`)
	ech0FM        = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n?`)
	// frontmatter drivers: 形如 `  - key: cover.png` 的附件声明
	ech0FileKeyRe = regexp.MustCompile(`(?m)^\s*-\s*key:\s*["']?([^"'\s]+)["']?\s*$`)
)

const (
	maxCapsuleFile  = 32 << 20  // 单个附件上限 32MB
	maxCapsuleTotal = 256 << 20 // 单次导入附件总量上限
	maxCapsulePost  = 4 << 20   // 单篇 Markdown 上限
	maxCapsuleMeta  = 1 << 20   // wisp.yaml 上限
)

func parseEch0Zip(data []byte) ([]Post, error) {
	zr, err := zip.NewReader(bytesReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("打开 capsule zip: %w", err)
	}

	// 媒体索引：basename → zip entry（capsule 里为 files/images/xxx 等）
	mediaIndex := map[string]*zip.File{}
	for _, f := range zr.File {
		if !strings.Contains(f.Name, "files/") || strings.HasSuffix(f.Name, "/") {
			continue
		}
		mediaIndex[path.Base(f.Name)] = f
	}

	posts := make([]Post, 0, 256)
	var totalMedia int64
	for _, f := range zr.File {
		dir, name := path.Split(f.Name)
		if !strings.Contains(dir, "echoes/") || !strings.HasSuffix(name, ".md") {
			continue
		}
		// 4MB 上限（与 wisp capsule 正文一致），防 zip bomb
		raw, err := readZipEntry(f, 4<<20)
		if err != nil {
			return nil, fmt.Errorf("读取 %s: %w", f.Name, err)
		}
		content := string(raw)

		body := content
		ts := time.Now().Unix()
		var keys []string
		if fm := ech0FM.FindStringSubmatch(content); fm != nil {
			front := fm[1]
			body = strings.TrimPrefix(content, fm[0])
			if m := ech0CreatedRe.FindStringSubmatch(front); m != nil {
				if tt, err := time.Parse(time.RFC3339, strings.TrimSpace(m[1])); err == nil {
					ts = tt.Unix()
				}
			}
			for _, m := range ech0FileKeyRe.FindAllStringSubmatch(front, -1) {
				keys = append(keys, m[1])
			}
		}
		if strings.TrimSpace(body) == "" && len(keys) == 0 {
			continue
		}

		var files []PostFile
		for _, key := range keys {
			mf, ok := mediaIndex[key]
			if !ok || mf.UncompressedSize64 > maxCapsuleFile || totalMedia > maxCapsuleTotal {
				continue
			}
			if totalMedia+int64(mf.UncompressedSize64) > maxCapsuleTotal {
				break
			}
			mrc, err := mf.Open()
			if err != nil {
				continue
			}
			blob, err := io.ReadAll(io.LimitReader(mrc, maxCapsuleFile+1))
			_ = mrc.Close()
			if err != nil || int64(len(blob)) > maxCapsuleFile {
				continue
			}
			totalMedia += int64(len(blob))
			files = append(files, PostFile{Name: key, Mime: mimeByExt(key), Data: blob})
		}

		posts = append(posts, Post{
			ContentMD:   strings.TrimSpace(body),
			PublishedAt: ts,
			Visibility:  "public", // capsule 默认不含私密 Echo
			Source:      "import",
			Files:       files,
		})
	}
	if len(posts) == 0 {
		return nil, fmt.Errorf("capsule 中未找到 echoes/ 下的 Echo 文件")
	}
	return posts, nil
}

// mimeByExt 依据扩展名推断 MIME（导入附件的类型标注）。
func mimeByExt(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".ogg":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	default:
		return ""
	}
}

// ---------- Wisp capsule ----------

type wispFrontMatter = capsule.FrontMatter

func parseWispZip(zr *zip.Reader) ([]Post, error) {
	if len(zr.File) > 20000 {
		return nil, fmt.Errorf("capsule 条目过多")
	}
	files := map[string]*zip.File{}
	var total int64
	for _, f := range zr.File {
		if f.UncompressedSize64 > uint64(maxCapsuleTotal) || total > maxCapsuleTotal-int64(f.UncompressedSize64) {
			return nil, fmt.Errorf("capsule 解压后超过 %dMB 上限", maxCapsuleTotal>>20)
		}
		total += int64(f.UncompressedSize64)
		files[f.Name] = f
	}

	manifestFile, ok := files["wisp.yaml"]
	if !ok {
		return nil, fmt.Errorf("capsule 缺少 wisp.yaml")
	}
	manifestRaw, err := readZipEntry(manifestFile, maxCapsuleMeta)
	if err != nil {
		return nil, fmt.Errorf("读取 wisp.yaml: %w", err)
	}
	var manifest capsule.Manifest
	if err := yaml.Unmarshal(manifestRaw, &manifest); err != nil {
		return nil, fmt.Errorf("解析 wisp.yaml: %w", err)
	}
	if manifest.Version != capsule.Version {
		return nil, fmt.Errorf("不支持的 capsule 版本 %d（当前支持 %d）", manifest.Version, capsule.Version)
	}

	names := make([]string, 0, len(files))
	for name := range files {
		if strings.HasPrefix(name, "posts/") && strings.HasSuffix(name, ".md") {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	posts := make([]Post, 0, len(names))
	var mediaTotal, actualTotal int64
	for _, name := range names {
		f := files[name]
		content, err := readZipEntry(f, maxCapsulePost)
		if err != nil {
			return nil, fmt.Errorf("读取 %s: %w", name, err)
		}
		actualTotal += int64(len(content))
		if actualTotal > maxCapsuleTotal {
			return nil, fmt.Errorf("capsule 实际解压后超过 %dMB 上限", maxCapsuleTotal>>20)
		}

		body := string(content)
		var front wispFrontMatter
		if fm := ech0FM.FindStringSubmatch(body); fm != nil {
			body = strings.TrimPrefix(body, fm[0])
			if err := yaml.Unmarshal([]byte(fm[1]), &front); err != nil {
				return nil, fmt.Errorf("解析 %s frontmatter: %w", name, err)
			}
		}
		if strings.TrimSpace(body) == "" && len(front.Blocks) == 0 && len(front.Attachments) == 0 {
			continue
		}

		p := Post{
			ContentMD:   strings.TrimSpace(body),
			PublishedAt: front.PublishedAt,
			Visibility:  normalizeVisibility(front.Visibility),
			Location:    front.Location,
			Source:      "import",
			Tags:        front.Tags,
		}
		if p.PublishedAt == 0 {
			p.PublishedAt = time.Now().Unix()
		}
		for _, b := range front.Blocks {
			p.Blocks = append(p.Blocks, Block{Type: b.Type, URL: b.URL, Name: b.Name, Lat: b.Lat, Lng: b.Lng})
		}

		for _, a := range front.Attachments {
			item := PostFile{
				Name: path.Base(a.File), Mime: a.Mime, Kind: a.Kind,
				Width: a.Width, Height: a.Height, Blurhash: a.Blurhash,
				RemoteURL: a.RemoteURL,
			}
			if a.File != "" {
				af, ok := files[a.File]
				if !ok {
					if a.RemoteURL == "" {
						return nil, fmt.Errorf("%s 引用的附件不存在: %s", name, a.File)
					}
				} else {
					if af.UncompressedSize64 > uint64(maxCapsuleFile) || mediaTotal > maxCapsuleTotal-int64(af.UncompressedSize64) {
						return nil, fmt.Errorf("媒体解压后超过 %dMB 上限", maxCapsuleTotal>>20)
					}
					data, err := readZipEntry(af, maxCapsuleFile)
					if err != nil {
						return nil, fmt.Errorf("读取附件 %s: %w", a.File, err)
					}
					mediaTotal += int64(len(data))
					actualTotal += int64(len(data))
					if actualTotal > maxCapsuleTotal {
						return nil, fmt.Errorf("capsule 实际解压后超过 %dMB 上限", maxCapsuleTotal>>20)
					}
					item.Data = data
				}
			}
			if a.PosterFile != "" {
				if pf, ok := files[a.PosterFile]; ok {
					data, err := readZipEntry(pf, maxCapsuleFile)
					if err != nil {
						return nil, fmt.Errorf("读取封面 %s: %w", a.PosterFile, err)
					}
					item.PosterData = data
					actualTotal += int64(len(data))
					if actualTotal > maxCapsuleTotal {
						return nil, fmt.Errorf("capsule 实际解压后超过 %dMB 上限", maxCapsuleTotal>>20)
					}
				}
			}
			if item.Name == "" {
				item.Name = "media-" + fmt.Sprint(len(p.Files)+1)
			}
			p.Files = append(p.Files, item)
		}
		posts = append(posts, p)
	}
	if len(posts) == 0 {
		return nil, fmt.Errorf("capsule 中没有 posts/ 内容")
	}
	return posts, nil
}

// readZipEntry 限制单条解压大小并传播 CRC/读取错误。
func readZipEntry(f *zip.File, limit int64) ([]byte, error) {
	if f.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("条目超过 %dMB 上限", limit>>20)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(rc, limit+1))
	closeErr := rc.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("条目超过 %dMB 上限", limit>>20)
	}
	return data, nil
}
