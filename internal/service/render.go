// Package service 实现业务逻辑：渲染、标签、帖子。
package service

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			// 输出 CSS class 而不是内联颜色，交给前端按亮/暗主题着色
			highlighting.WithFormatOptions(
				html.WithClasses(true),
				html.WithLineNumbers(false),
			),
			highlighting.WithStyle("github"),
		),
	),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

// 纯标签行（整行仅由 #tag 组成）在正文中移除——标签已以 chips 形式单独展示。
var tagOnlyLine = regexp.MustCompile(`(?m)^[ \t]*(?:#[^\s#]+[ \t]*)+$`)

// RenderHTML 把 Markdown 渲染为 HTML（不启用 raw HTML，杜绝注入）。
func RenderHTML(source string) (string, error) {
	cleaned := tagOnlyLine.ReplaceAllString(source, "")
	var buf bytes.Buffer
	if err := md.Convert([]byte(cleaned), &buf); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return buf.String(), nil
}

// tagPattern：行首或空白后的 #tag（中文/字母/数字/下划线/连字符，1~64 字符）。
// 排除 `# 一级标题`（# 后有空格）、URL fragment（前导是 / 或字母）等场景。
var tagPattern = regexp.MustCompile(`(?:^|\s)#([^\s#]{1,64})`)

// ExtractTags 提取去重后的标签（最多 8 个）。
func ExtractTags(source string) []string {
	seen := map[string]bool{}
	var tags []string
	for _, m := range tagPattern.FindAllStringSubmatch(source, -1) {
		tag := strings.Trim(m[1], "，。,.:：!！?？()（）[]【】「」")
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
		if len(tags) >= 8 {
			break
		}
	}
	return tags
}

// Excerpt 提取纯文本摘要（RSS / OG 用）。
func Excerpt(source string, maxRunes int) string {
	text := source
	// 去掉图片与链接语法，保留链接文字
	text = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`).ReplaceAllString(text, "$1")
	// 去掉常见标记符
	text = regexp.MustCompile(`[#>*_\x60~\-]{1,}`).ReplaceAllString(text, " ")
	fields := strings.Fields(text)
	text = strings.Join(fields, " ")
	runes := []rune(text)
	if len(runes) > maxRunes {
		return strings.TrimSpace(string(runes[:maxRunes])) + "…"
	}
	return strings.TrimSpace(text)
}
