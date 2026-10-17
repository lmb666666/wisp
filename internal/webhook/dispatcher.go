// Package webhook 把站点事件推送到订阅者（HMAC-SHA256 签名，异步不阻塞主流程）。
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/util"
)

const (
	httpTimeout   = 5 * time.Second
	queueSize     = 64
	workerCount   = 2
	maxPayloadLog = 200
)

// Dispatcher 事件分发器：Emit 非阻塞，worker 异步投递。
type Dispatcher struct {
	q  repo.Querier
	ch chan job
}

type job struct {
	hook    repo.Webhook
	event   string
	payload any
}

func NewDispatcher(q repo.Querier) *Dispatcher {
	return &Dispatcher{q: q, ch: make(chan job, queueSize)}
}

// Start 启动 worker（调用方保证进程生命周期）。
func (d *Dispatcher) Start() {
	for i := 0; i < workerCount; i++ {
		go d.worker()
	}
}

// Emit 投递事件：队列满则丢弃并记录（宁可丢通知，不可阻塞发帖）。
func (d *Dispatcher) Emit(event string, payload any) {
	hooks, err := d.q.ListWebhooksByEvent(context.Background(), event)
	if err != nil {
		slog.Warn("webhook query failed", "event", event, "err", err)
		return
	}
	for _, hook := range hooks {
		select {
		case d.ch <- job{hook: hook, event: event, payload: payload}:
		default:
			slog.Warn("webhook queue full, dropping", "event", event)
		}
	}
}

func (d *Dispatcher) worker() {
	hc := &http.Client{Timeout: httpTimeout}
	for job := range d.ch {
		d.deliver(hc, job)
	}
}

type envelope struct {
	Event     string `json:"event"`
	Timestamp int64  `json:"timestamp"`
	Data      any    `json:"data"`
}

func (d *Dispatcher) deliver(hc *http.Client, job job) {
	body, err := json.Marshal(envelope{Event: job.event, Timestamp: time.Now().Unix(), Data: job.payload})
	if err != nil {
		slog.Warn("webhook marshal", "err", err)
		return
	}
	sign := hmacSign(job.hook.Secret, body)
	req, err := http.NewRequest(http.MethodPost, job.hook.Url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Wisp-Event", job.event)
	req.Header.Set("X-Wisp-Signature", "sha256="+sign)
	req.Header.Set("X-Wisp-Delivery", util.NewULID())

	resp, err := hc.Do(req)
	if err != nil {
		slog.Warn("webhook delivery failed", "url", maskURL(job.hook.Url), "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.Warn("webhook non-2xx", "url", maskURL(job.hook.Url), "status", resp.StatusCode)
	}
}

// Sign 暴露给测试：计算 HMAC-SHA256（hex）。
func Sign(secret string, body []byte) string { return hmacSign(secret, body) }

func hmacSign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func maskURL(raw string) string {
	if len(raw) > maxPayloadLog {
		return raw[:maxPayloadLog]
	}
	return raw
}
