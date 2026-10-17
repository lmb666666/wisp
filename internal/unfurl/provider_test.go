package unfurl

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/lmb666666/wisp/internal/repo"
)

func mockFetch(responses map[string]string) Fetcher {
	return func(_ context.Context, u string) ([]byte, error) {
		if body, ok := responses[u]; ok {
			return []byte(body), nil
		}
		return nil, errors.New("no mock for " + u)
	}
}

func TestResolveBilibili(t *testing.T) {
	fetch := mockFetch(map[string]string{
		"https://api.bilibili.com/x/web-interface/view?bvid=BV1xx411c7mD": `{"code":0,"data":{"title":"测试视频","pic":"http://i0.hdslb.com/bfs/x.jpg","owner":{"name":"UP主"},"stat":{"view":123}}}`,
	})
	pv, err := Resolve(context.Background(), "https://www.bilibili.com/video/BV1xx411c7mD", "", fetch, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if pv.Provider != "bilibili" || !strings.Contains(pv.Title, "测试视频") || !strings.Contains(pv.Title, "UP主") {
		t.Fatalf("preview = %+v", pv)
	}
	if pv.Cover == "" {
		t.Error("cover empty")
	}
	if pv.EmbedHTML != "https://player.bilibili.com/player.html?bvid=BV1xx411c7mD&autoplay=0" {
		t.Errorf("embed = %q", pv.EmbedHTML)
	}
}

func TestResolveMeting(t *testing.T) {
	fetch := func(_ context.Context, u string) ([]byte, error) {
		parsed, _ := url.Parse(u)
		q := parsed.Query()
		if q.Get("server") == "netease" && q.Get("type") == "song" && q.Get("id") == "1868553" {
			return []byte(`[{"name":"晴天","artist":"周杰伦","pic":"http://img.example/cover.jpg"}]`), nil
		}
		return nil, errors.New("unexpected meting url: " + u)
	}
	pv, err := Resolve(context.Background(), "https://music.163.com/song?id=1868553", "https://meting.example", fetch, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if pv.Provider != "music" || pv.Title != "晴天 · 周杰伦" {
		t.Fatalf("preview = %+v", pv)
	}
	audio, err := url.Parse(pv.EmbedHTML)
	if err != nil {
		t.Fatal(err)
	}
	q := audio.Query()
	if q.Get("server") != "netease" || q.Get("type") != "url" || q.Get("id") != "1868553" {
		t.Errorf("audio url = %s", pv.EmbedHTML)
	}
}

func TestBuildMetingURLFormats(t *testing.T) {
	// 查询参数式
	got := BuildMetingURL("https://api.example.com", "netease", "song", "42")
	u, _ := url.Parse(got)
	if u.Query().Get("server") != "netease" || u.Query().Get("type") != "song" || u.Query().Get("id") != "42" {
		t.Errorf("query format = %s", got)
	}
	// 占位符式（Ech0 默认格式）
	got = BuildMetingURL("https://music.3e0.cn/?server=:server&type=:type&id=:id&r=:r", "tencent", "url", "abc123")
	if !strings.Contains(got, "server=tencent") || !strings.Contains(got, "type=url") ||
		!strings.Contains(got, "id=abc123") || strings.Contains(got, ":r") {
		t.Errorf("placeholder format = %s", got)
	}
}

func TestMusicSource(t *testing.T) {
	cases := []struct{ url, server, typ, id string }{
		{"https://music.163.com/song?id=1868553", "netease", "song", "1868553"},
		{"https://music.163.com/#/song?id=123", "netease", "song", "123"},
		{"https://y.qq.com/n/ryqq/songDetail/003OUlho2HcRHC", "tencent", "song", "003OUlho2HcRHC"},
		{"https://y.qq.com/n/yqq/song/123.html?songid=456", "tencent", "song", "456"},
	}
	for _, c := range cases {
		server, typ, id, err := MusicSource(c.url)
		if err != nil || server != c.server || typ != c.typ || id != c.id {
			t.Errorf("MusicSource(%s) = %s/%s/%s/%v", c.url, server, typ, id, err)
		}
	}
	if _, _, _, err := MusicSource("https://music.163.com/playlist?id=1"); err == nil {
		t.Error("playlist should be rejected for now")
	}
}

func TestResolveGitHub(t *testing.T) {
	fetch := mockFetch(map[string]string{
		"https://api.github.com/repos/oklog/ulid": `{"full_name":"oklog/ulid","description":"ULID for Go","stargazers_count":3000,"language":"Go"}`,
	})
	pv, err := Resolve(context.Background(), "https://github.com/oklog/ulid", "", fetch, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if pv.Provider != "github" || !strings.Contains(pv.Title, "★ 3000") {
		t.Fatalf("preview = %+v", pv)
	}
}

func TestResolveGenericOG(t *testing.T) {
	fetch := mockFetch(map[string]string{
		"https://example.com/post": `<html><head>
			<meta property="og:title" content="一篇文章"/>
			<meta property="og:image" content="/img/cover.png"/>
		</head><body>ignored</body></html>`,
	})
	pv, err := Resolve(context.Background(), "https://example.com/post", "", fetch, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if pv.Provider != "og" || pv.Title != "一篇文章" {
		t.Fatalf("preview = %+v", pv)
	}
	if pv.Cover != "https://example.com/img/cover.png" { // 相对路径补全
		t.Errorf("cover = %s", pv.Cover)
	}
}

type fakeStore map[string]repo.LinkPreview

func (f fakeStore) GetLinkPreview(_ context.Context, u string) (repo.LinkPreview, error) {
	if pv, ok := f[u]; ok {
		return pv, nil
	}
	return repo.LinkPreview{}, errors.New("miss")
}

func TestEnrich(t *testing.T) {
	store := fakeStore{
		"https://github.com/oklog/ulid": {Provider: "github", Title: "oklog/ulid · ★ 3000"},
		"https://inline.example/y":      {Provider: "og", Title: "行内链接卡片"},
	}
	in := `<p>看这个 <a href="https://inline.example/y">https://inline.example/y</a> 挺好</p>` +
		`<p><a href="https://github.com/oklog/ulid">https://github.com/oklog/ulid</a></p>` +
		`<p><a href="https://unknown.example/x">https://unknown.example/x</a></p>`
	out := Enrich(context.Background(), store, in)

	// 独占段落 → 原地卡片；行内链接 → 文末卡片区
	if strings.Count(out, `class="link-card"`) != 2 {
		t.Errorf("want 2 cards (1 inline-block, 1 appended):\n%s", out)
	}
	if !strings.Contains(out, `class="link-cards"`) || !strings.Contains(out, "行内链接卡片") {
		t.Errorf("inline card section missing:\n%s", out)
	}
	if !strings.Contains(out, "★ 3000") {
		t.Errorf("github card missing")
	}
	// 未命中缓存 → 保持原样且不追加
	if !strings.Contains(out, `<p><a href="https://unknown.example/x">`) {
		t.Errorf("cache miss should stay untouched:\n%s", out)
	}
}

func TestBareURLs(t *testing.T) {
	// 独占段落 + 行内链接都应被预取（去重）
	in := `<p><a href="https://a.example/x">https://a.example/x</a></p><p>text <a href="https://b.example">link</a></p><p><a href="https://a.example/x">dup</a></p>`
	got := BareURLs(in)
	if len(got) != 2 || got[0] != "https://a.example/x" || got[1] != "https://b.example" {
		t.Fatalf("bare urls = %v", got)
	}
}

func TestOGFallbackText(t *testing.T) {
	fetch := mockFetch(map[string]string{
		"https://plain.example": `<html><head><title>标题</title><script>var x="<p>fake</p>";</script></head>
			<body><div>正文内容在这里</div></body></html>`,
	})
	pv, err := Resolve(context.Background(), "https://plain.example", "", fetch, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if pv.Title != "标题" {
		t.Fatalf("title = %q", pv.Title)
	}
	_ = url.QueryEscape
}

func TestSSRFGuard(t *testing.T) {
	fetch := NewFetcher()
	private := []string{
		"http://127.0.0.1/x", "http://localhost/x", "http://10.0.0.1/x",
		"http://192.168.1.1/x", "http://169.254.169.254/latest/meta-data/",
	}
	for _, u := range private {
		if _, err := fetch(context.Background(), u); err == nil {
			t.Errorf("private fetch should be blocked: %s", u)
		}
	}
}

func TestVideoEmbed(t *testing.T) {
	cases := []struct {
		url      string
		provider string
		embed    string
	}{
		{"https://www.bilibili.com/video/BV1xx411c7mD?spm=1", "bilibili", "https://player.bilibili.com/player.html?bvid=BV1xx411c7mD&autoplay=0"},
		{"https://youtu.be/dQw4w9WgXcQ", "youtube", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "youtube", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"},
		{"https://www.youtube.com/shorts/dQw4w9WgXcQ", "youtube", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"},
		{"https://m.youtube.com/watch?v=dQw4w9WgXcQ", "youtube", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"},
	}
	for _, tc := range cases {
		embed, provider, ok := VideoEmbed(tc.url)
		if !ok || provider != tc.provider || embed != tc.embed {
			t.Errorf("VideoEmbed(%q) = %q %q %v", tc.url, embed, provider, ok)
		}
	}
	for _, bad := range []string{
		"https://b23.tv/abc", "https://www.bilibili.com/video/av1",
		"https://youtube.com/", "https://example.com/video/BV1xx411c7mD",
		"ftp://youtu.be/dQw4w9WgXcQ", "javascript:alert(1)",
	} {
		if _, _, ok := VideoEmbed(bad); ok {
			t.Errorf("VideoEmbed(%q) should fail", bad)
		}
	}
}
