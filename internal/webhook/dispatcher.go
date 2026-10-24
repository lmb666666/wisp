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
	"sync"
	"time"

	"github.com/lmb666666/wisp/internal/repo"
	"github.com/lmb666666/wisp/internal/util"
)

const (
	httpTimeout   = 5 * time.Second
	queueSize     = 64
	workerCount   = 2
	maxPayloadLog = 200
	retryDelays   = 3 // 失败重试次数（间隔 2s/8s/30s）
)

var retryBackoff = []time.Duration{2 * time.Second, 8 * time.Second, 30 * time.Second}

// Dispatcher 事件分发器：Emit 非阻塞，worker 异步投递（失败带退避重试）。
type Dispatcher struct {
	q  repo.Querier
	ch chan job
	wg sync.WaitGroup
}

type job struct {
	hook    repo.Webhook
	event   string
	payload any
}

func NewDispatcher(q repo.Querier) *Dispatcher {
	return &Dispatcher{q: q, ch: make(chan job, queueSize)}
}

// Start 启动 worker；进程退出前调用 Stop + Wait 避免队列内事件丢失。
func (d *Dispatcher) Start() {
	d.wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go d.worker()
	}
}

// Stop 关闭队列（不再接收新事件）；Wait 等待 worker 排空退出。只允许调用一次。
func (d *Dispatcher) Stop() { close(d.ch) }

func (d *Dispatcher) Wait() { d.wg.Wait() }

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
	defer d.wg.Done()
	hc := &http.Client{Timeout: httpTimeout}
	for job := range d.ch {
		d.deliverWithRetry(hc, job)
	}
}

type envelope struct {
	Event     string `json:"event"`
	Timestamp int64  `json:"timestamp"`
	Data      any    `json:"data"`
}

// deliverWithRetry 投递失败按退避重试；总耗时上限约 45s（2s/8s/30s）。
func (d *Dispatcher) deliverWithRetry(hc *http.Client, jb job) {
	for attempt := 0; ; attempt++ {
		if d.deliver(hc, jb) {
			return
		}
		if attempt >= len(retryBackoff) {
			return
		}
		time.Sleep(retryBackoff[attempt])
	}
}

// deliver 单次投递；返回是否成功（2xx）。
func (d *Dispatcher) deliver(hc *http.Client, jb job) bool {
	body, err := json.Marshal(envelope{Event: jb.event, Timestamp: time.Now().Unix(), Data: jb.payload})
	if err != nil {
		slog.Warn("webhook marshal", "err", err)
		return false
	}
	sign := hmacSign(jb.hook.Secret, body)
	req, err := http.NewRequest(http.MethodPost, jb.hook.Url, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Wisp-Event", jb.event)
	req.Header.Set("X-Wisp-Signature", "sha256="+sign)
	req.Header.Set("X-Wisp-Delivery", util.NewULID())

	resp, err := hc.Do(req)
	if err != nil {
		slog.Warn("webhook delivery failed", "url", maskURL(jb.hook.Url), "err", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.Warn("webhook non-2xx", "url", maskURL(jb.hook.Url), "status", resp.StatusCode)
		return false
	}
	return true
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
