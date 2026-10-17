// Package capsule 实现 Wisp 的可移植内容包：Markdown + frontmatter + 媒体原文件。
// 导出为 zip，可导入任意 Wisp 实例，也可作为长期归档。
package capsule

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/lmb666666/wisp/internal/media"
	"github.com/lmb666666/wisp/internal/repo"
)

const (
	Version       = 1
	maxRemoteFile = 32 << 20 // 与导入/上传单文件上限一致
)

// Options 导出选项。
type Options struct {
	IncludePrivate bool
	SourceURL      string
}

// Stats 导出统计。
type Stats struct {
	Posts       int      `yaml:"posts"`
	Attachments int      `yaml:"attachments"`
	MediaBytes  int64    `yaml:"media_bytes"`
	Warnings    []string `yaml:"-"`
}

// Manifest wisp.yaml。
type Manifest struct {
	Version        int      `yaml:"version"`
	ExportedAt     int64    `yaml:"exported_at"`
	SourceURL      string   `yaml:"source_url,omitempty"`
	IncludePrivate bool     `yaml:"include_private"`
	Stats          Stats    `yaml:"stats"`
	Warnings       []string `yaml:"warnings,omitempty"`
}

// Block 扩展卡片（只导出可重建的字段）。
type Block struct {
	Type string   `yaml:"type" json:"type"`
	URL  string   `yaml:"url,omitempty" json:"url,omitempty"`
	Name string   `yaml:"name,omitempty" json:"name,omitempty"`
	Lat  *float64 `yaml:"lat,omitempty" json:"lat,omitempty"`
	Lng  *float64 `yaml:"lng,omitempty" json:"lng,omitempty"`
}

// Attachment 附件清单。
type Attachment struct {
	File       string `yaml:"file,omitempty"`
	Kind       string `yaml:"kind"`
	Mime       string `yaml:"mime,omitempty"`
	Width      int64  `yaml:"width,omitempty"`
	Height     int64  `yaml:"height,omitempty"`
	Blurhash   string `yaml:"blurhash,omitempty"`
	PosterFile string `yaml:"poster_file,omitempty"`
	RemoteURL  string `yaml:"remote_url,omitempty"`
}

// FrontMatter 每篇 Markdown 的元数据。
type FrontMatter struct {
	ID          string       `yaml:"id"`
	PublishedAt int64        `yaml:"published_at"`
	Visibility  string       `yaml:"visibility"`
	Tags        []string     `yaml:"tags,omitempty"`
	Location    string       `yaml:"location,omitempty"`
	Blocks      []Block      `yaml:"blocks,omitempty"`
	Attachments []Attachment `yaml:"attachments,omitempty"`
}

func safeEntry(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return false
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// Export 流式写出内容包 zip。
func Export(ctx context.Context, q repo.Querier, store media.Storage, w io.Writer, opts Options) (Stats, error) {
	posts, err := q.ListPostsForExport(ctx, boolToInt(opts.IncludePrivate))
	if err != nil {
		return Stats{}, fmt.Errorf("list posts: %w", err)
	}

	zw := zip.NewWriter(w)
	stats := Stats{Posts: len(posts)}

	client := &http.Client{Timeout: 60 * time.Second}

	for _, post := range posts {
		tags, err := q.ListPostTags(ctx, post.ID)
		if err != nil {
			return stats, fmt.Errorf("tags for %s: %w", post.ID, err)
		}
		tagNames := make([]string, 0, len(tags))
		for _, tg := range tags {
			tagNames = append(tagNames, tg.Name)
		}

		front := FrontMatter{
			ID:          post.ID,
			PublishedAt: post.PublishedAt,
			Visibility:  post.Visibility,
			Tags:        tagNames,
			Location:    post.Location,
		}
		if post.Blocks != "" {
			_ = json.Unmarshal([]byte(post.Blocks), &front.Blocks)
		}

		atts, err := q.ListPostAttachments(ctx, repo.StrNull(post.ID))
		if err != nil {
			return stats, fmt.Errorf("attachments for %s: %w", post.ID, err)
		}
		for _, a := range atts {
			item, n, err := exportAttachment(ctx, store, client, zw, a)
			if err != nil {
				stats.Warnings = append(stats.Warnings, fmt.Sprintf("%s: %v", post.ID, err))
				item = Attachment{Kind: a.Kind, Mime: a.Mime, RemoteURL: a.RemoteUrl}
			}
			stats.Attachments++
			stats.MediaBytes += n
			front.Attachments = append(front.Attachments, item)
		}

		if err := writePost(zw, post, front); err != nil {
			return stats, err
		}
	}

	manifest, err := yaml.Marshal(Manifest{
		Version: Version, ExportedAt: time.Now().Unix(),
		SourceURL: opts.SourceURL, IncludePrivate: opts.IncludePrivate,
		Stats: stats, Warnings: stats.Warnings,
	})
	if err != nil {
		return stats, err
	}
	mw, err := zw.Create("wisp.yaml")
	if err != nil {
		return stats, err
	}
	if _, err := mw.Write(manifest); err != nil {
		return stats, err
	}
	return stats, zw.Close()
}

func exportAttachment(ctx context.Context, store media.Storage, client *http.Client, zw *zip.Writer, a repo.Attachment) (Attachment, int64, error) {
	item := Attachment{Kind: a.Kind, Mime: a.Mime, Width: a.Width, Height: a.Height, Blurhash: a.Blurhash}

	var (
		src io.ReadCloser
		err error
	)
	// 优先从配置的存储读取（S3 私有桶也能导出）；远端外链作为兜底。
	if a.StorageKey != "" && safeEntry(a.StorageKey) {
		src, err = store.Get(ctx, a.StorageKey)
	}
	if src == nil && a.RemoteUrl != "" {
		var resp *http.Response
		resp, err = client.Get(a.RemoteUrl)
		if err != nil {
			return item, 0, fmt.Errorf("下载外链失败: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return item, 0, fmt.Errorf("外链返回 %d", resp.StatusCode)
		}
		if resp.ContentLength > maxRemoteFile {
			_ = resp.Body.Close()
			return item, 0, fmt.Errorf("外链文件超过 %dMB", maxRemoteFile>>20)
		}
		src = resp.Body
	}
	if src == nil {
		if err != nil {
			return item, 0, fmt.Errorf("读取存储失败: %w", err)
		}
		return item, 0, fmt.Errorf("附件没有可导出的内容")
	}
	defer src.Close()

	name := "files/" + a.StorageKey
	if a.StorageKey == "" || a.RemoteUrl != "" {
		ext := path.Ext(a.RemoteUrl)
		if ext == "" || len(ext) > 8 {
			ext = ".bin"
		}
		name = "files/" + a.ID + ext
	}
	if !safeEntry(name) {
		return item, 0, fmt.Errorf("非法的附件路径")
	}

	entry, err := zw.Create(name)
	if err != nil {
		return item, 0, err
	}
	n, err := io.Copy(entry, io.LimitReader(src, maxRemoteFile+1))
	if err != nil {
		return item, n, fmt.Errorf("写入 zip: %w", err)
	}
	if n > maxRemoteFile {
		return item, n, fmt.Errorf("文件超过 %dMB，已截断", maxRemoteFile>>20)
	}
	item.File = name

	if a.PosterPath != "" {
		if pr, perr := store.Get(ctx, a.PosterPath); perr == nil {
			posterName := "files/" + a.PosterPath
			if safeEntry(posterName) {
				if pe, cerr := zw.Create(posterName); cerr == nil {
					if n, copyErr := io.Copy(pe, io.LimitReader(pr, maxRemoteFile+1)); copyErr == nil && n <= maxRemoteFile {
						item.PosterFile = posterName
					}
				}
			}
			_ = pr.Close()
		}
	}
	return item, n, nil
}

func writePost(zw *zip.Writer, post repo.Post, front FrontMatter) error {
	fm, err := yaml.Marshal(front)
	if err != nil {
		return err
	}
	t := time.Unix(post.PublishedAt, 0).UTC()
	name := fmt.Sprintf("posts/%04d/%02d/%s.md", t.Year(), int(t.Month()), post.ID)
	if !safeEntry(name) {
		return fmt.Errorf("非法的帖子路径: %s", post.ID)
	}
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte("---\n")); err != nil {
		return err
	}
	if _, err := w.Write(fm); err != nil {
		return err
	}
	if _, err := w.Write([]byte("---\n\n")); err != nil {
		return err
	}
	_, err = io.WriteString(w, post.ContentMd)
	return err
}

func boolToInt(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
