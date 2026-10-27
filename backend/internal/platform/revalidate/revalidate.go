// Package revalidate 通知 website 讓 Next.js 的快取失效（docs/開發規範/後端服務責任邊界規範.md §9）。
//
// 單體裡以 River job 送出（ADR 2026-09-24-go-modular-monolith D6）：修改資料的 use case
// 在自己的 transaction 內用 jobs.Enqueue 排入 Args，commit 之後 job 才看得到，所以不會有
// 「資料寫了、失效沒排」，也不會在 transaction 還沒 commit 時就讓快取失效。
//
// 送出的語意照 §9.4、§9.5 的單體註記：同一次修改的 tag 一次送出；每次 5 秒 timeout；暫時性的失敗
// （連線錯誤、timeout、5xx、429）以 River 預設的退避重試，其他非 2xx 是設定錯誤，取消不重試。
// revalidateTag 冪等，重試安全。失敗都記 WARN。
package revalidate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
	"uuid"

	"github.com/riverqueue/river"
)

const (
	// timeout 是一次送出的時間上限（§9.5）。
	timeout = 5 * time.Second
	// maxAttempts 是含第一次在內的送出次數。River 預設的退避是 attempt^4 秒：第 2～5 次約在 1 秒、
	// 17 秒、98 秒、6 分鐘後，第 6 次會在 16 分鐘後，已經超過貼文相關快取在 website 最長的 TTL
	// （website 的 CACHE_TTL.DISCOVERY，900 秒），那時快取自己就過期了，重試沒有意義。
	maxAttempts = 5
)

// PostTag 是一篇貼文的 cache tag：website 把 post: 展開到貼文列表（lib/api/revalidate-request.ts）。不送 hashtag:
// tag：website 把它展開成商品 hashtag 的選單快取，貼文的熱門話題在貼文列表的快取裡。
func PostTag(postID uuid.UUID) string {
	return "post:" + postID.String()
}

// StoreTags 是一家店的店家頁 cache tag：website 以網址上的 store_id 或 store_code 為 key（lib/seo/tags.ts 的 sellerTag），兩個都送。
func StoreTags(storeID, storeCode int64) []string {
	return []string{"seller:" + strconv.FormatInt(storeID, 10), "seller:" + strconv.FormatInt(storeCode, 10)}
}

// Args 是一次失效：同一次修改影響的所有 tag。
type Args struct {
	Tags []string `json:"tags"`
}

// Kind 是 River 的 job 種類名稱。
func (Args) Kind() string { return "revalidate" }

// InsertOpts 讓每個排入的 job 都帶重試上限。
func (Args) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: maxAttempts}
}

// Config 是 website 的位置與共用 secret（§9.7 的 WEB_REVALIDATE_BASE_URL、WEB_REVALIDATE_SECRET）。
type Config struct {
	BaseURL string
	Secret  string
}

// Enabled 回報兩個值是否都有設定。沒設定時 worker 不送出（本機開發通常沒有 website）。
func (c Config) Enabled() bool { return c.BaseURL != "" && c.Secret != "" }

// Worker 執行 Args。
type Worker struct {
	river.WorkerDefaults[Args]
	cfg  Config
	http *http.Client
	log  *slog.Logger
}

// NewWorker 建立 worker。沒有設定時記一次 WARN：部署漏設時快取永遠不會失效，這是唯一的訊號（§9.5）。
func NewWorker(cfg Config, log *slog.Logger) *Worker {
	if !cfg.Enabled() {
		log.Warn("revalidate 未啟用：WEB_REVALIDATE_BASE_URL 或 WEB_REVALIDATE_SECRET 沒有設定，website 的快取不會失效")
	}
	return &Worker{cfg: cfg, http: &http.Client{}, log: log}
}

// Timeout 是 River 給一次執行的時間上限。
func (w *Worker) Timeout(*river.Job[Args]) time.Duration { return timeout }

// Work 送出一次失效。暫時性的失敗回傳 error 讓 River 退避重試；其他失敗取消 job，不再重試。
func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	if !w.cfg.Enabled() || len(job.Args.Tags) == 0 {
		return nil
	}
	err := w.send(ctx, job.Args.Tags)
	if err == nil {
		return nil
	}
	w.log.WarnContext(ctx, "revalidate 送出失敗", "tags", job.Args.Tags, "attempt", job.Attempt, "error", err)
	if retryable(err) {
		return err
	}
	return river.JobCancel(err)
}

func (w *Worker) send(ctx context.Context, tags []string) error {
	body, err := json.Marshal(Args{Tags: tags})
	if err != nil {
		return fmt.Errorf("revalidate：編碼：%w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.BaseURL+"/api/revalidate", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("revalidate：建立請求：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-revalidate-secret", w.cfg.Secret)
	resp, err := w.http.Do(req)
	if err != nil {
		return &transportError{err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	preview, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
	return &statusError{code: resp.StatusCode, body: string(preview)}
}

// statusError 是 website 回了非 2xx。
type statusError struct {
	code int
	body string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("revalidate：status %d：%s", e.code, e.body)
}

// transportError 是請求沒有得到任何回應的錯誤。
type transportError struct{ err error }

func (e *transportError) Error() string { return "revalidate：送出：" + e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

// retryable 回報失敗是否是暫時性的：沒有得到回應（連線錯誤、timeout）、website 暫時不可用（5xx）
// 或限流（429）。其他非 2xx（400、401 secret 不符、404 網址錯）是設定錯誤，重試也不會好。
func retryable(err error) bool {
	var te *transportError
	if errors.As(err, &te) {
		return true
	}
	var se *statusError
	return errors.As(err, &se) && (se.code >= 500 || se.code == http.StatusTooManyRequests)
}
