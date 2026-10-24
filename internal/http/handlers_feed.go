package http

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/service"
)

// handleFeed RSS 2.0 订阅（全量或 ?tag=）。
func (s *Server) handleFeed(c echo.Context) error {
	items, _, err := s.posts.List(c.Request().Context(), service.ListParams{
		Tag:   c.QueryParam("tag"),
		Limit: 20,
	})
	if err != nil {
		return s.internalError(c, err)
	}

	base := strings.TrimRight(s.cfg.PublicURL, "/")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintf(&b, `<rss version="2.0"><channel><title>%s</title><link>%s</link><description>%s</description>`,
		xesc("Wisp"), xesc(base), xesc("Wisp · 说说与动态"))

	for _, it := range items {
		link := base + "/p/" + it.ID
		desc := it.ContentHTML
		fmt.Fprintf(&b,
			`<item><title>%s</title><link>%s</link><guid isPermaLink="true">%s</guid>`+
				`<pubDate>%s</pubDate><description><![CDATA[%s]]></description></item>`,
			xesc(service.Excerpt(it.ContentMD, 60)),
			xesc(link), xesc(link),
			time.Unix(it.PublishedAt, 0).UTC().Format(time.RFC1123Z),
			cdata(desc),
		)
	}
	b.WriteString(`</channel></rss>`)

	c.Response().Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	return c.String(http.StatusOK, b.String())
}

func xesc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// cdata 安全包裹 CDATA（处理 "]]>" 序列）。
func cdata(s string) string {
	return strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>")
}
