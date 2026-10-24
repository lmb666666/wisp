package unfurl

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/lmb666666/wisp/internal/repo"
)

// goldmark GFM 对「独占一段的 URL」通常渲染成 <p><a href>…</a></p>，
// 但部分主机名（如 localhost）不会被 linkify，退化为纯文本 <p>URL</p>。
// 两种形态都升级为卡片；正文内嵌链接一律保持原样。
var (
	bareLinked = regexp.MustCompile(`<p><a href="(https?://[^"]+)">[^<]*</a></p>`)
	barePlain  = regexp.MustCompile(`<p>(https?://[^\s<]+)</p>`)
	anyAnchor  = regexp.MustCompile(`<a href="(https?://[^"]+)"`)
)

const maxCards = 5

// PreviewStore 卡片数据源（repo.Querier 的子集，便于测试）。
type PreviewStore interface {
	GetLinkPreview(ctx context.Context, url string) (repo.LinkPreview, error)
}

// BareURLs 提取正文中的全部链接（含行内链接）用于预取，去重后最多 5 个。
// 捕获自 HTML 属性，先 Unescape 还原真实 URL（`&amp;` → `&`），保证与
// Enrich 查缓存用的 key 一致。
func BareURLs(renderedHTML string) []string {
	seen := map[string]bool{}
	var urls []string
	for _, re := range []*regexp.Regexp{bareLinked, barePlain, anyAnchor} {
		for _, m := range re.FindAllStringSubmatch(renderedHTML, -1) {
			u := html.UnescapeString(m[1])
			if !seen[u] && len(urls) < maxCards {
				seen[u] = true
				urls = append(urls, u)
			}
		}
	}
	return urls
}

// Enrich 读取时注入卡片：
//   - 独占一段的链接 → 原地替换为卡片（正文更整洁）
//   - 正文内嵌链接 → 在文末追加卡片区（Telegram/Twitter 式 link preview）
//
// 未命中缓存的链接保持原样；预取完成后下次读取自然升级。
func Enrich(ctx context.Context, store PreviewStore, renderedHTML string) string {
	rendered := map[string]bool{}

	cardFor := func(u string) (string, bool) {
		pv, err := store.GetLinkPreview(ctx, u)
		if err != nil || strings.TrimSpace(pv.Title) == "" {
			return "", false
		}
		host := ""
		if parsed, err := url.Parse(u); err == nil {
			host = parsed.Hostname()
		}
		var b strings.Builder
		b.WriteString(`<a class="link-card" href="`)
		b.WriteString(html.EscapeString(u))
		b.WriteString(`" target="_blank" rel="noopener nofollow"`)
		if pv.EmbedHtml != "" {
			// 前端点击后才加载 iframe（隐私优先），RSS/OG 始终是外链卡
			b.WriteString(` data-embed-src="`)
			b.WriteString(html.EscapeString(pv.EmbedHtml))
			b.WriteString(`" data-embed-type="`)
			if pv.Provider == "music" {
				b.WriteString("audio")
			} else {
				b.WriteString("video")
			}
			b.WriteString(`"`)
		}
		b.WriteString(`>`)
		if pv.Cover != "" {
			b.WriteString(`<img class="link-card-cover" src="`)
			b.WriteString(html.EscapeString(pv.Cover))
			b.WriteString(`" loading="lazy" alt=""/>`)
		}
		b.WriteString(`<span class="link-card-body"><span class="link-card-title">`)
		b.WriteString(html.EscapeString(pv.Title))
		b.WriteString(`</span><span class="link-card-meta">`)
		b.WriteString(html.EscapeString(strings.TrimSpace(pv.Provider + " · " + host)))
		b.WriteString(`</span></span>`)
		if pv.EmbedHtml != "" {
			b.WriteString(`<span class="link-card-play" aria-hidden="true">▶</span>`)
		}
		b.WriteString(`</a>`)
		return b.String(), true
	}

	// 1) 独占段落的链接原地成卡（捕获值是 HTML 转义形态，先还原）
	out := bareLinked.ReplaceAllStringFunc(renderedHTML, func(match string) string {
		u := html.UnescapeString(bareLinked.FindStringSubmatch(match)[1])
		if card, ok := cardFor(u); ok {
			rendered[u] = true
			return card
		}
		return match
	})
	out = barePlain.ReplaceAllStringFunc(out, func(match string) string {
		u := html.UnescapeString(barePlain.FindStringSubmatch(match)[1])
		if card, ok := cardFor(u); ok {
			rendered[u] = true
			return card
		}
		return match
	})

	// 2) 正文内其他链接 → 文末追加卡片区
	var extra []string
	for _, m := range anyAnchor.FindAllStringSubmatch(out, -1) {
		u := html.UnescapeString(m[1])
		if rendered[u] || len(extra) >= maxCards {
			continue
		}
		if card, ok := cardFor(u); ok {
			rendered[u] = true
			extra = append(extra, card)
		}
	}
	if len(extra) > 0 {
		out += `<div class="link-cards">` + strings.Join(extra, "") + `</div>`
	}
	return out
}
