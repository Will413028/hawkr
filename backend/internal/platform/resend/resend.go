// Package resend 是 Resend 的 email adapter：寄信（POST /emails）與退信、投訴 webhook 的簽章驗證與解析。只用 net/http，不重試——
// 對外只回暫時性（ErrTransient，重試可能成功）或永久性（ErrPermanent，重試不會成功）兩種錯誤，重試交給呼叫端（River job）。
package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrTransient 標示暫時性失敗（網路、逾時、429、5xx、同一個冪等鍵的請求還在處理、金鑰被拒）：重試可能成功。
var ErrTransient = errors.New("resend：暫時性失敗")

// ErrPermanent 標示永久失敗（請求被拒：參數、寄件網域或金鑰有問題）：重試不會成功。
var ErrPermanent = errors.New("resend：永久失敗")

// ErrAuth 標示金鑰被拒（401、403）：是設定問題，不是這封信的問題；與 ErrTransient 一起回傳（修好設定後重試可能成功），呼叫端以它告警。
var ErrAuth = errors.New("resend：金鑰被拒")

// Email 是一封純文字信。IdempotencyKey 非空時以 Idempotency-Key 送出，Resend 會合併 24 小時內同鍵的請求（job 在送出後、寫回結果前當掉重試也不重寄）。
type Email struct {
	To             string
	Subject        string
	Text           string
	IdempotencyKey string
}

// Sender 寄一封信；失敗時回 ErrTransient 或 ErrPermanent（以 errors.Is 判斷）。
type Sender interface {
	Send(ctx context.Context, e Email) error
}

const (
	defaultBaseURL = "https://api.resend.com"
	maxResponse    = 1 << 20
)

// Config 是 Client 的設定。BaseURL 與 HTTPClient 只給測試換掉。
type Config struct {
	APIKey     string
	From       string
	BaseURL    string
	HTTPClient *http.Client
}

// Client 是 Resend 的 Sender。
type Client struct {
	key, from, base string
	http            *http.Client
}

// New 建立 Client；APIKey 與 From（寄件人，例如 `Hawkr <noreply@mail.example.com>`）必填。
func New(c Config) (*Client, error) {
	if c.APIKey == "" || c.From == "" {
		return nil, errors.New("resend：APIKey 與 From 必填")
	}
	cl := &Client{key: c.APIKey, from: c.From, base: c.BaseURL, http: c.HTTPClient}
	if cl.base == "" {
		cl.base = defaultBaseURL
	}
	if cl.http == nil {
		cl.http = &http.Client{Timeout: 15 * time.Second}
	}
	return cl, nil
}

// Send 寄信。回應 2xx 是成功；其餘依狀態碼分類，錯誤訊息只帶狀態碼與 Resend 的錯誤名稱，不含收件人與內容。
func (c *Client) Send(ctx context.Context, e Email) error {
	body, err := json.Marshal(map[string]any{"from": c.from, "to": []string{e.To}, "subject": e.Subject, "text": e.Text})
	if err != nil {
		return fmt.Errorf("%w：組請求", ErrPermanent)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w：組請求", ErrPermanent)
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	if e.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", e.IdempotencyKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w：網路", ErrTransient)
	}
	defer resp.Body.Close() //nolint:errcheck // 回應本文已讀完
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var apiErr struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(raw, &apiErr)
	return classify(resp.StatusCode, apiErr.Name)
}

// Resend 對冪等鍵衝突回 409，以錯誤名稱區分：concurrent_idempotent_requests 是同鍵請求還在處理（暫時性）；
// invalid_idempotent_request 是同鍵卻帶不同內容（永久，重試只會再被拒）。
func classify(status int, name string) error {
	detail := fmt.Sprintf("http %d %s", status, name)
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("%w：%w：%s", ErrTransient, ErrAuth, detail)
	case status == http.StatusConflict && name == "invalid_idempotent_request":
		return fmt.Errorf("%w：%s", ErrPermanent, detail)
	case status == http.StatusTooManyRequests || status == http.StatusRequestTimeout || status == http.StatusConflict || status >= 500:
		return fmt.Errorf("%w：%s", ErrTransient, detail)
	default:
		return fmt.Errorf("%w：%s", ErrPermanent, detail)
	}
}
