package unfurl

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	qqSongDetailRe = regexp.MustCompile(`/songDetail/([A-Za-z0-9]+)`)
	qqPlaylistRe   = regexp.MustCompile(`/playlist/([0-9]+)`)
)

// MusicSource 解析音乐链接为 Meting 所需的 server / type / id。
// server 取值遵循 Meting 约定：netease / tencent。
func MusicSource(rawURL string) (server, typ, id string, err error) {
	u, perr := url.Parse(rawURL)
	if perr != nil {
		return "", "", "", fmt.Errorf("无法解析音乐链接")
	}
	host := strings.ToLower(u.Hostname())
	frag := u.Fragment

	switch {
	case host == "music.163.com" || strings.HasSuffix(host, ".music.163.com"):
		server = "netease"
		id = u.Query().Get("id")
		if id == "" {
			if i := strings.Index(frag, "id="); i >= 0 {
				id = frag[i+3:]
				if j := strings.IndexAny(id, "&?#"); j >= 0 {
					id = id[:j]
				}
			}
		}
		switch {
		case strings.Contains(u.Path, "playlist") || strings.Contains(frag, "playlist"):
			typ = "playlist"
		case strings.Contains(u.Path, "album") || strings.Contains(frag, "album"):
			typ = "album"
		default:
			typ = "song"
		}
	case host == "y.qq.com" || strings.HasSuffix(host, ".qq.com"):
		server = "tencent"
		typ = "song"
		if m := qqSongDetailRe.FindStringSubmatch(u.Path); m != nil {
			id = m[1]
		}
		if id == "" {
			id = u.Query().Get("songid")
		}
		if id == "" {
			id = u.Query().Get("songmid")
		}
		if m := qqPlaylistRe.FindStringSubmatch(u.Path); m != nil {
			typ, id = "playlist", m[1]
		}
	default:
		return "", "", "", fmt.Errorf("仅支持网易云 / QQ 音乐链接")
	}

	if id == "" {
		return "", "", "", fmt.Errorf("链接里没有歌曲 id")
	}
	if typ != "song" {
		return "", "", "", fmt.Errorf("目前只支持歌曲链接（歌单/专辑稍后支持）")
	}
	return server, typ, id, nil
}

// BuildMetingURL 按 Meting 约定拼接接口地址，兼容两种常见格式：
//  1. 查询参数：https://api.example.com/?server=netease&type=song&id=123
//  2. 占位符：  https://music.3e0.cn/?server=:server&type=:type&id=:id&r=:r
func BuildMetingURL(base, server, typ, id string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	r := strconv.FormatInt(time.Now().UnixNano()%1_000_000, 10)

	if strings.Contains(base, ":server") || strings.Contains(base, ":type") || strings.Contains(base, ":id") {
		repl := strings.NewReplacer(
			":server", url.QueryEscape(server),
			":type", url.QueryEscape(typ),
			":id", url.QueryEscape(id),
			":auth", "",
			":r", r,
		)
		return repl.Replace(base)
	}

	sep := "?"
	switch {
	case strings.HasSuffix(base, "?"), strings.HasSuffix(base, "&"):
		sep = ""
	case strings.Contains(base, "?"):
		sep = "&"
	}
	q := url.Values{}
	q.Set("server", server)
	q.Set("type", typ)
	q.Set("id", id)
	q.Set("r", r)
	return base + sep + q.Encode()
}
