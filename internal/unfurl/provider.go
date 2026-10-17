// Package unfurl 把帖子里的裸链接解析成富内容卡片（Ech0「特殊内容」的对应实现）。
// 设计：Markdown 是唯一真源；content_html 保持素净，卡片在读取时由 Enrich 注入，
// 抓取在发布时异步进行（失败不阻塞发布、不吞内容）。
package unfurl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const fetchTimeout = 8 * time.Second
const maxBody = 1 << 20 // 1MB

// Preview 抓取结果（落 link_previews 表）。
type Preview struct {
	Provider  string
	Title     string
	Cover     string
	EmbedHTML string // 保留字段：v0.3 暂不出 iframe（隐私与 RSS 一致性），卡片一律外链
}

// Fetcher 用可替换的 HTTP 客户端（测试注入）。
type Fetcher func(ctx context.Context, url string) ([]byte, error)

// Resolve 按 URL 匹配 provider 并抓取；未匹配的走通用 OG。
// metingAPI 为自托管 Meting 端点；留空则音乐卡禁用（解析失败不阻塞，降级普通链接）。
func Resolve(ctx context.Context, rawURL, metingAPI string, fetch, trustedFetch Fetcher) (*Preview, error) {
	if trustedFetch == nil {
		trustedFetch = fetch
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	host := strings.ToLower(u.Hostname())
	try := func(run func() (*Preview, error)) (*Preview, error) {
		pv, err := run()
		if err == nil && pv != nil && pv.Title != "" {
			return pv, nil
		}
		return resolveOG(ctx, rawURL, fetch)
	}
	switch {
	case host == "bilibili.com" || strings.HasSuffix(host, ".bilibili.com") || host == "b23.tv":
		return try(func() (*Preview, error) { return resolveBilibili(ctx, u, fetch) })
	case host == "youtube.com" || host == "youtu.be" || strings.HasSuffix(host, ".youtube.com"):
		return try(func() (*Preview, error) { return resolveYouTube(ctx, rawURL, fetch) })
	case host == "music.163.com":
		if metingAPI != "" {
			return try(func() (*Preview, error) { return resolveMeting(ctx, rawURL, metingAPI, trustedFetch) })
		}
		// 未配置 Meting：只渲染普通链接/OG 卡片，不提供无法播放的假播放器
		return resolveOG(ctx, rawURL, fetch)
	case host == "y.qq.com":
		if metingAPI != "" {
			return try(func() (*Preview, error) { return resolveMeting(ctx, rawURL, metingAPI, trustedFetch) })
		}
		return resolveOG(ctx, rawURL, fetch)
	case host == "github.com":
		return try(func() (*Preview, error) { return resolveGitHub(ctx, u, fetch) })
	default:
		return resolveOG(ctx, rawURL, fetch)
	}
}

// ---------- Bilibili ----------

var bvPattern = regexp.MustCompile(`(BV[0-9A-Za-z]{10})`)
var youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{6,20}$`)

// VideoEmbed 把受支持的视频链接转成点击后加载的 embed URL。
// 仅接受能确定性解析出视频 ID 的源，短链/播放列表交给自动 unfurl。
func VideoEmbed(rawURL string) (embed, provider string, ok bool) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", "", false
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "bilibili.com" || strings.HasSuffix(host, ".bilibili.com"):
		bvid := bvPattern.FindString(u.Path)
		if bvid == "" {
			return "", "", false
		}
		return "https://player.bilibili.com/player.html?bvid=" + bvid + "&autoplay=0", "bilibili", true
	case host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") || host == "youtu.be":
		var id string
		if host == "youtu.be" {
			id = strings.Trim(u.Path, "/")
		} else if v := u.Query().Get("v"); v != "" {
			id = v
		} else {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live") {
				id = parts[1]
			}
		}
		if !youtubeIDPattern.MatchString(id) {
			return "", "", false
		}
		return "https://www.youtube-nocookie.com/embed/" + id, "youtube", true
	}
	return "", "", false
}

func resolveBilibili(ctx context.Context, u *url.URL, fetch Fetcher) (*Preview, error) {
	bvid := bvPattern.FindString(u.Path)
	if bvid == "" && u.Path == "/" { // b23.tv 短链无法本地解析，降级 OG
		return resolveOG(ctx, u.String(), fetch)
	}
	if bvid == "" {
		return nil, fmt.Errorf("bilibili: 未找到 BV 号")
	}
	data, err := fetch(ctx, "https://api.bilibili.com/x/web-interface/view?bvid="+bvid)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Title string `json:"title"`
			Pic   string `json:"pic"`
			Owner struct {
				Name string `json:"name"`
			}
			Stat struct {
				View int64 `json:"view"`
			}
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil || resp.Code != 0 {
		return nil, fmt.Errorf("bilibili api: code=%d", resp.Code)
	}
	return &Preview{
		Provider:  "bilibili",
		Title:     resp.Data.Title + " · " + resp.Data.Owner.Name,
		Cover:     resp.Data.Pic,
		EmbedHTML: "https://player.bilibili.com/player.html?bvid=" + bvid + "&autoplay=0",
	}, nil
}

// ---------- YouTube（oEmbed，免 key）----------

func resolveYouTube(ctx context.Context, rawURL string, fetch Fetcher) (*Preview, error) {
	data, err := fetch(ctx, "https://www.youtube.com/oembed?url="+url.QueryEscape(rawURL)+"&format=json")
	if err != nil {
		return nil, err
	}
	var o struct {
		Title      string `json:"title"`
		Thumbnail  string `json:"thumbnail_url"`
		AuthorName string `json:"author_name"`
	}
	if err := json.Unmarshal(data, &o); err != nil || o.Title == "" {
		return nil, fmt.Errorf("youtube oembed: 无效响应")
	}
	embed := ""
	if m := ytID.FindStringSubmatch(rawURL); m != nil {
		embed = "https://www.youtube-nocookie.com/embed/" + m[1]
	}
	return &Preview{Provider: "youtube", Title: o.Title + " · " + o.AuthorName, Cover: o.Thumbnail, EmbedHTML: embed}, nil
}

var ytID = regexp.MustCompile(`(?:v=|youtu\.be/)([A-Za-z0-9_-]{11})`)

// ---------- GitHub 仓库 ----------

var ghPattern = regexp.MustCompile(`^/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/?$`)

func resolveGitHub(ctx context.Context, u *url.URL, fetch Fetcher) (*Preview, error) {
	m := ghPattern.FindStringSubmatch(u.Path)
	if m == nil {
		return resolveOG(ctx, u.String(), fetch)
	}
	data, err := fetch(ctx, "https://api.github.com/repos/"+m[1]+"/"+m[2])
	if err != nil {
		return nil, err
	}
	var repo struct {
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Stars       int    `json:"stargazers_count"`
		Language    string `json:"language"`
	}
	if err := json.Unmarshal(data, &repo); err != nil || repo.FullName == "" {
		return nil, fmt.Errorf("github api: 无效响应")
	}
	title := repo.FullName
	if repo.Language != "" {
		title += " · " + repo.Language
	}
	return &Preview{
		Provider: "github",
		Title:    title + " · ★ " + strconv.Itoa(repo.Stars),
		Cover:    "",
	}, nil
}

// ---------- Meting 音乐卡（网易云 / QQ 音乐，需自托管端点）----------

func resolveMeting(ctx context.Context, rawURL, metingAPI string, fetch Fetcher) (*Preview, error) {
	server, _, id, err := MusicSource(rawURL)
	if err != nil {
		return nil, fmt.Errorf("meting: %w", err)
	}
	data, err := fetch(ctx, BuildMetingURL(metingAPI, server, "song", id))
	if err != nil {
		return nil, err
	}
	var arr []struct {
		Name   string `json:"name"`
		Artist string `json:"artist"`
		Pic    string `json:"pic"`
		Cover  string `json:"cover"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(data, &arr); err != nil || len(arr) == 0 {
		return nil, fmt.Errorf("meting api: 无效响应")
	}
	cover := arr[0].Pic
	if cover == "" {
		cover = arr[0].Cover
	}
	sep := ""
	if arr[0].Artist != "" {
		sep = " · "
	}
	// Meting 的 song 响应通常直接带可播放 url（可能是 CDN 直链）；
	// 没有时才回落到 type=url 接口。
	audio := strings.TrimSpace(arr[0].URL)
	if !strings.HasPrefix(audio, "http://") && !strings.HasPrefix(audio, "https://") {
		audio = BuildMetingURL(metingAPI, server, "url", id)
	}
	return &Preview{
		Provider:  "music",
		Title:     arr[0].Name + sep + arr[0].Artist,
		Cover:     cover,
		EmbedHTML: audio,
	}, nil
}

// ---------- 通用 OG ----------

var (
	ogTitle   = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)["']`)
	ogImage   = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)["']`)
	altTitle  = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	altImage  = regexp.MustCompile(`(?i)<img[^>]+src=["']([^"']+\.(?:png|jpe?g|webp|gif))["']`)
	titleTag  = regexp.MustCompile(`(?i)&(amp|lt|gt|quot|#39);`)
	scriptTag = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleTag  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	tagRe     = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRe   = regexp.MustCompile(`\s+`)
)

func resolveOG(ctx context.Context, rawURL string, fetch Fetcher) (*Preview, error) {
	data, err := fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	html := string(data)
	p := &Preview{Provider: "og"}
	if m := ogTitle.FindStringSubmatch(html); m != nil {
		p.Title = m[1]
	} else if m := altTitle.FindStringSubmatch(html); m != nil {
		p.Title = strings.TrimSpace(m[1])
	}
	if m := ogImage.FindStringSubmatch(html); m != nil {
		p.Cover = m[1]
	} else if m := altImage.FindStringSubmatch(html); m != nil {
		p.Cover = m[1]
	}
	if p.Title == "" {
		// 兜底：取正文前 60 字符
		text := spaceRe.ReplaceAllString(tagRe.ReplaceAllString(scriptTag.ReplaceAllString(styleTag.ReplaceAllString(html, ""), " "), " "), " ")
		runes := []rune(strings.TrimSpace(text))
		if len(runes) > 60 {
			runes = runes[:60]
		}
		p.Title = string(runes)
	}
	if p.Title == "" {
		return nil, errors.New("og: 无可提取内容")
	}
	// 相对路径封面补全
	if p.Cover != "" && !strings.HasPrefix(p.Cover, "http") {
		if base, err := url.Parse(rawURL); err == nil {
			if ref, err := base.Parse(p.Cover); err == nil {
				p.Cover = ref.String()
			}
		}
	}
	_ = titleTag // 预留：实体解码
	return p, nil
}

// ---------- SSRF 防护 ----------

// NewFetcher 面向用户内容的抓取器：拒绝解析到私网/环回/链路本地的地址
// （防内网探测与云元数据端点 169.254.169.254）。
func NewFetcher() Fetcher {
	dialer := &net.Dialer{Timeout: fetchTimeout}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve %s: %w", host, err)
			}
			for _, ip := range ips {
				if isBlockedIP(ip.IP) {
					return nil, fmt.Errorf("blocked private address for %s", host)
				}
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	hc := &http.Client{Timeout: fetchTimeout, Transport: transport}
	return func(ctx context.Context, rawURL string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; WispBot/0.2; +https://github.com/lmb666666/wisp)")
		req.Header.Set("Accept", "text/html,application/json")
		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp.Body, maxBody))
	}
}

// NewUnrestrictedFetcher 供管理员自配置的可信端点（如自托管 Meting）使用，无内网限制。
func NewUnrestrictedFetcher() Fetcher {
	hc := &http.Client{Timeout: fetchTimeout}
	return func(ctx context.Context, rawURL string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp.Body, maxBody))
	}
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
