package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lmb666666/wisp/internal/repo"
)

// renderVersion 渲染器版本：变更 content_html 生成方式时递增，启动时自动重渲染历史帖。
// 2 = 代码高亮改为 class 输出（支持亮/暗主题）。
const renderVersion = "2"
const renderVersionKey = "render_version"

// ReRenderAll 在渲染器升级后一次性重渲染所有帖子（幂等，按 settings 版本号跳过）。
func ReRenderAll(ctx context.Context, q repo.Querier) error {
	if val, err := q.GetSetting(ctx, renderVersionKey); err == nil && val == renderVersion {
		return nil
	}
	rows, err := q.ListPostContentForRerender(ctx)
	if err != nil {
		return fmt.Errorf("list posts for rerender: %w", err)
	}
	failed := 0
	for _, row := range rows {
		html, err := RenderHTML(row.ContentMd)
		if err != nil {
			failed++
			continue
		}
		if err := q.UpdatePostHTML(ctx, repo.UpdatePostHTMLParams{ContentHtml: html, ID: row.ID}); err != nil {
			failed++
		}
	}
	if err := q.UpsertSetting(ctx, repo.UpsertSettingParams{
		Key: renderVersionKey, Value: renderVersion, UpdatedAt: time.Now().Unix(),
	}); err != nil {
		return fmt.Errorf("save render version: %w", err)
	}
	slog.Info("content_html re-rendered", "posts", len(rows), "failed", failed, "version", renderVersion)
	return nil
}
