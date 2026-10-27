// Package revalidate posts cache-bust requests to the hawkr-website
// /api/revalidate endpoint. See 後端服務責任邊界規範 §9.
//
// Semantics:
//   - fire-and-forget: caller hands off the bust and returns immediately;
//     the actual HTTP request runs in a goroutine
//   - best-effort: failures (timeout / non-2xx / network) are logged only,
//     never returned to the caller; cacheLife profile is the safety net
//   - timeout: 5 seconds per request
//
// Tag schema (SSOT in hawkr-website/lib/seo/tags.ts):
//   - "product:<id>"  — PDP detail / product reviews
//   - "seller:<id>"   — seller profile / storefront / review stats
package revalidate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Config carries the runtime configuration loaded from env vars.
type Config struct {
	// BaseURL — e.g. "https://web-dev.example.com" (no trailing slash)
	BaseURL string
	// Secret — must match hawkr-website REVALIDATE_SECRET
	Secret string
	// Timeout — per-request timeout; defaults to 5 seconds when zero
	Timeout time.Duration
}

// Enabled returns true when both BaseURL and Secret are non-empty.
// Disabled state is safe and silent — bust calls become no-ops, which is
// the right behaviour for local dev where the web service isn't wired.
func (c Config) Enabled() bool {
	return c.BaseURL != "" && c.Secret != ""
}

// Client is safe for concurrent use.
type Client struct {
	cfg    Config
	logger *zap.Logger
	http   *http.Client
}

// New builds a Client. Pass a nil logger to get a no-op zap.NewNop().
func New(cfg Config, logger *zap.Logger) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Client{
		cfg:    cfg,
		logger: logger,
		http:   &http.Client{Timeout: cfg.Timeout},
	}
}

// Enabled reports whether busts actually leave the process: a nil client (not
// wired) or a Config missing BaseURL/Secret turns every BustAsync into a no-op.
// Callers use it to skip work they would only do to build the tag list.
func (c *Client) Enabled() bool {
	return c != nil && c.cfg.Enabled()
}

// BustAsync fires off a cache-bust request in a goroutine and returns
// immediately. Caller MUST invoke this after the main transaction commits
// (the typical pattern is `defer client.BustAsync(...)` placed AFTER the
// commit succeeds, or an explicit call at the end of the use case).
//
// Empty tags or a disabled client are no-ops.
func (c *Client) BustAsync(ctx context.Context, tags []string) {
	if c == nil || len(tags) == 0 || !c.cfg.Enabled() {
		return
	}
	// Detach from caller context — caller may cancel ctx right after
	// returning; we still want the bust to complete on its own timeout.
	go func() {
		bg, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
		defer cancel()
		if err := c.do(bg, tags); err != nil {
			c.logger.Warn("revalidate.send.error",
				zap.Strings("tags", tags),
				zap.Error(err))
		}
	}()
}

// maxAttempts — 初次 + 最多 2 次重試 = 最多 3 次送出（後端服務責任邊界規範 §9.5）。
// 只對連線層錯誤（stale pooled connection）重試；timeout / non-2xx 不重試。
const maxAttempts = 3

func (c *Client) do(ctx context.Context, tags []string) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		retryable, err := c.doOnce(ctx, tags, attempt)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retryable || attempt == maxAttempts {
			break
		}
		// stale pooled connection：連線已被判定失效，下次 Do 會開新連線（#1271）
		c.logger.Warn("revalidate.send.retry",
			zap.Strings("tags", tags),
			zap.Int("attempt", attempt),
			zap.Error(err))
	}
	return lastErr
}

// doOnce 送出一次。回傳 (retryable, err)：retryable 僅在連線層錯誤（stale pooled
// connection）為 true；timeout / context deadline / non-2xx 皆 false（不重試）。
func (c *Client) doOnce(ctx context.Context, tags []string, attempt int) (bool, error) {
	body, err := json.Marshal(map[string]any{"tags": tags})
	if err != nil {
		return false, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/api/revalidate", bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-revalidate-secret", c.cfg.Secret)
	resp, err := c.http.Do(req)
	if err != nil {
		return isRetryable(err), fmt.Errorf("do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		c.logger.Info("revalidate.send.success",
			zap.Strings("tags", tags),
			zap.Int("status", resp.StatusCode),
			zap.Int("attempt", attempt))
		return false, nil
	}
	preview, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
	// non-2xx（如 401 secret 不符）屬設定問題，不重試
	return false, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(preview))
}

// isRetryable 判定傳輸層錯誤是否可重試：timeout / context deadline（窗口已耗盡）
// 不重試；其餘（EOF / connection reset / stale pooled connection）可重試。
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}
	return true
}
