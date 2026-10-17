package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/lmb666666/wisp/internal/repo"
)

const settingKeyNotify = "notify_settings"

// SMTPSettings 邮件通知（net/smtp，STARTTLS 自动协商）。
type SMTPSettings struct {
	Host string `json:"smtp_host"`
	Port int    `json:"smtp_port"`
	User string `json:"smtp_user"`
	Pass string `json:"smtp_pass"`
	From string `json:"smtp_from"`
	To   string `json:"smtp_to"`
}

// NotifySettings 通知渠道（自托管作者的心头好：ntfy / Bark / 邮件）。
type NotifySettings struct {
	NtfyTopicURL string       `json:"ntfy_topic_url"` // 如 https://ntfy.sh/wisp-xxxxx
	BarkURL      string       `json:"bark_url"`       // 如 https://api.day.app/xxxxx
	SMTP         SMTPSettings `json:"smtp"`
	// NotifyOnReactions 预留扩展：目前唯一事件是「有人点赞」
	NotifyOnReactions bool `json:"notify_on_reactions"`
}

// maskSecrets GET 时脱敏 SMTP 密码。
func (st NotifySettings) maskSecrets() NotifySettings {
	if st.SMTP.Pass != "" {
		st.SMTP.Pass = "***"
	}
	return st
}

// mergeSecrets PUT 时空值或掩码沿用旧密码。
func (s *Server) mergeNotifySecrets(ctx context.Context, incoming NotifySettings) NotifySettings {
	old := s.loadNotifySettings(ctx)
	if incoming.SMTP.Pass == "" || incoming.SMTP.Pass == "***" {
		incoming.SMTP.Pass = old.SMTP.Pass
	}
	return incoming
}

// loadNotifySettings 读取通知配置（默认开启 reactions 推送，未配渠道则静默）。
func (s *Server) loadNotifySettings(ctx context.Context) NotifySettings {
	st := NotifySettings{NotifyOnReactions: true}
	if val, err := s.q.GetSetting(ctx, settingKeyNotify); err == nil && val != "" {
		_ = json.Unmarshal([]byte(val), &st)
	}
	return st
}

// getNotifySettings GET /admin/settings/notify（密码脱敏）。
func (s *Server) getNotifySettings(c echo.Context) error {
	return c.JSON(http.StatusOK, s.loadNotifySettings(c.Request().Context()).maskSecrets())
}

// putNotifySettings PUT /admin/settings/notify。
func (s *Server) putNotifySettings(c echo.Context) error {
	var st NotifySettings
	if err := c.Bind(&st); err != nil {
		return apiError(c, http.StatusBadRequest, "bad_request", "请求格式错误")
	}
	st = s.mergeNotifySecrets(c.Request().Context(), st)
	blob, _ := json.Marshal(st)
	if err := s.q.UpsertSetting(c.Request().Context(), repo.UpsertSettingParams{
		Key: settingKeyNotify, Value: string(blob), UpdatedAt: time.Now().Unix(),
	}); err != nil {
		return apiError(c, http.StatusInternalServerError, "internal", err.Error())
	}
	return c.JSON(http.StatusOK, st.maskSecrets())
}

// pushNotification 异步推送一条通知（失败只记日志）。
func (s *Server) pushNotification(title, body string) {
	st := s.loadNotifySettings(context.Background())
	if !st.NotifyOnReactions {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if st.NtfyTopicURL != "" {
			s.postNotify(ctx, st.NtfyTopicURL, title, body)
		}
		if st.BarkURL != "" {
			s.postNotify(ctx, strings.TrimRight(st.BarkURL, "/")+"/"+url.PathEscape(title)+"/"+url.PathEscape(body), "", "")
		}
		if st.SMTP.Host != "" && st.SMTP.To != "" {
			if err := sendMail(st.SMTP, title, body); err != nil {
				slog.Warn("notify mail failed", "err", err)
			}
		}
	}()
}

// sendMail 零依赖邮件通知（net/smtp；服务器支持 STARTTLS 时自动加密）。
func sendMail(st SMTPSettings, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", st.Host, st.Port)
	headers := map[string]string{
		"From":         st.From,
		"To":           st.To,
		"Subject":      mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version": "1.0",
		"Content-Type": `text/plain; charset="UTF-8"`,
	}
	var msg bytes.Buffer
	for k, v := range headers {
		fmt.Fprintf(&msg, "%s: %s\r\n", k, v)
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	var auth smtp.Auth
	if st.User != "" {
		auth = smtp.PlainAuth("", st.User, st.Pass, st.Host)
	}
	return smtp.SendMail(addr, auth, st.From, []string{st.To}, msg.Bytes())
}

func (s *Server) postNotify(ctx context.Context, endpoint, title, body string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return
	}
	if title != "" {
		req.Header.Set("Title", title)
	}
	req.Header.Set("Tags", "bell")
	hc := &http.Client{Timeout: 5 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		slog.Warn("notify push failed", "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.Warn("notify push non-2xx", "status", resp.StatusCode)
	}
}

// notifyReaction 有人点赞时通知作者（sendReactionNotify 由 addReaction 调用）。
func (s *Server) sendReactionNotify(postID, emoji string) {
	st := s.loadNotifySettings(context.Background())
	if !st.NotifyOnReactions || (st.NtfyTopicURL == "" && st.BarkURL == "") {
		return
	}
	dto, err := s.posts.Get(context.Background(), postID, false, "")
	if err != nil {
		return
	}
	excerpt := dto.ContentMD
	if len([]rune(excerpt)) > 40 {
		excerpt = string([]rune(excerpt)[:40]) + "…"
	}
	s.pushNotification(
		fmt.Sprintf("%s 有人点赞", emoji),
		fmt.Sprintf("「%s」", excerpt),
	)
}
