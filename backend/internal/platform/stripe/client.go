// Package stripe 是 Stripe API 的 client：只含收款開通需要的呼叫（Accounts v2、Account Link、Account Session）與 webhook 簽章驗證。
// 它不認得賣家模組的型別；對應成中立快照的 adapter 在 seller/internal/payment。
package stripe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// APIVersion 是釘住的 Stripe API 版本（計畫 E1 2026-11-03 修訂）。
const APIVersion = "2026-09-30.preview"

// DefaultBaseURL 是 Stripe API 的位址。
const DefaultBaseURL = "https://api.stripe.com"

// DefaultTimeout 是一次呼叫（含 429 退避）的總上限。
const DefaultTimeout = 30 * time.Second

const (
	maxResponseBytes = 8 << 20
	maxAttempts      = 3
	baseBackoff      = 500 * time.Millisecond
	maxBackoff       = 5 * time.Second
)

// Config 是 Client 的設定。
type Config struct {
	// BaseURL 預設 DefaultBaseURL；測試指向 httptest。
	BaseURL       string
	SecretKey     string
	WebhookSecret string
	// Timeout 是一次呼叫的總上限，零值用 DefaultTimeout。
	Timeout    time.Duration
	HTTPClient *http.Client
	// Now 驗簽章時間用；nil 時用 time.Now。
	Now func() time.Time
	// Wait 是 429 退避的等待；nil 時用計時器，ctx 結束就回傳。
	Wait func(ctx context.Context, d time.Duration) error
}

// Client 呼叫 Stripe。
type Client struct {
	base          *url.URL
	key, whsecret string
	timeout       time.Duration
	http          *http.Client
	now           func() time.Time
	wait          func(ctx context.Context, d time.Duration) error
}

// New 建立 Client；金鑰與 webhook 密鑰必填。
func New(cfg Config) (*Client, error) {
	if cfg.SecretKey == "" {
		return nil, errors.New("stripe：缺少 SecretKey")
	}
	if cfg.WebhookSecret == "" {
		return nil, errors.New("stripe：缺少 WebhookSecret")
	}
	raw := cfg.BaseURL
	if raw == "" {
		raw = DefaultBaseURL
	}
	base, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") {
		return nil, fmt.Errorf("stripe：BaseURL %q 不是有效的網址", raw)
	}
	c := &Client{base: base, key: cfg.SecretKey, whsecret: cfg.WebhookSecret, timeout: cfg.Timeout, http: cfg.HTTPClient, now: cfg.Now, wait: cfg.Wait}
	if c.timeout <= 0 {
		c.timeout = DefaultTimeout
	}
	if c.http == nil {
		c.http = &http.Client{}
	}
	if c.now == nil {
		c.now = time.Now
	}
	if c.wait == nil {
		c.wait = sleep
	}
	return c, nil
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// APIError 是 Stripe 回的非 2xx 回應。
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("stripe：HTTP %d %s：%s", e.Status, e.Code, e.Message)
}

// Permanent 回傳重試也不會成功：4xx 而且不是逾時（408）與限流（429）。
func (e *APIError) Permanent() bool {
	return e.Status >= 400 && e.Status < 500 && e.Status != http.StatusRequestTimeout && e.Status != http.StatusTooManyRequests
}

func apiError(status int, body []byte) *APIError {
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &env)
	return &APIError{Status: status, Code: env.Error.Code, Message: env.Error.Message}
}

type request struct {
	method      string
	path        string
	body        []byte
	contentType string
	idempotency string
}

func (c *Client) do(ctx context.Context, r request) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	for attempt := 1; ; attempt++ {
		var body io.Reader
		if r.body != nil {
			body = bytes.NewReader(r.body)
		}
		req, err := http.NewRequestWithContext(ctx, r.method, c.base.String()+r.path, body)
		if err != nil {
			return nil, fmt.Errorf("stripe：組請求失敗：%w", err)
		}
		req.SetBasicAuth(c.key, "")
		req.Header.Set("Stripe-Version", APIVersion)
		if r.contentType != "" {
			req.Header.Set("Content-Type", r.contentType)
		}
		if r.idempotency != "" {
			req.Header.Set("Idempotency-Key", r.idempotency)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("stripe：%s %s：%w", r.method, r.path, err)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("stripe：讀回應失敗：%w", err)
		}
		if len(data) > maxResponseBytes {
			return nil, fmt.Errorf("stripe：回應超過 %d bytes", maxResponseBytes)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return data, nil
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxAttempts {
			if err := c.wait(ctx, backoff(attempt, resp.Header.Get("Retry-After"))); err != nil {
				return nil, fmt.Errorf("stripe：限流退避被中斷：%w", err)
			}
			continue
		}
		return nil, apiError(resp.StatusCode, data)
	}
}

func backoff(attempt int, retryAfter string) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && s >= 0 {
		return min(time.Duration(s)*time.Second, maxBackoff)
	}
	return min(baseBackoff<<(attempt-1), maxBackoff)
}

func (c *Client) json(ctx context.Context, method, path string, body any, idempotency string, out any) error {
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return fmt.Errorf("stripe：編碼請求失敗：%w", err)
		}
	}
	r := request{method: method, path: path, body: raw, idempotency: idempotency}
	if raw != nil {
		r.contentType = "application/json"
	}
	data, err := c.do(ctx, r)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("stripe：解析回應失敗：%w", err)
	}
	return nil
}

// Dashboard 是連線帳戶的 dashboard 種類，決定誰向賣家收件：express 由 Stripe 收，none 由平台收（spike Q9）。
type Dashboard string

// Dashboard 的值。
const (
	DashboardExpress Dashboard = "express"
	DashboardNone    Dashboard = "none"
)

// CreateAccount 建立 v2 recipient 帳戶（US、individual、轉帳能力），以 idempotencyKey 冪等；回傳帳戶 ID。
func (c *Client) CreateAccount(ctx context.Context, idempotencyKey string, dashboard Dashboard) (string, error) {
	body := map[string]any{
		"identity": map[string]any{"country": "us", "entity_type": "individual"},
		"configuration": map[string]any{"recipient": map[string]any{"capabilities": map[string]any{
			"stripe_balance": map[string]any{"stripe_transfers": map[string]any{"requested": true}},
		}}},
		"defaults":  map[string]any{"responsibilities": map[string]any{"fees_collector": "application", "losses_collector": "application"}},
		"dashboard": string(dashboard),
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.json(ctx, http.MethodPost, "/v2/core/accounts", body, idempotencyKey, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", errors.New("stripe：建立帳戶的回應沒有 id")
	}
	return out.ID, nil
}

// AccountLink 是託管開通頁面的連結。
type AccountLink struct {
	URL       string
	ExpiresAt *time.Time
}

// CreateAccountLink 產生 v2 Account Link（託管的開通頁面）。2026-09-30 起沒有 configurations 欄位。
func (c *Client) CreateAccountLink(ctx context.Context, accountID, returnURL, refreshURL string) (AccountLink, error) {
	body := map[string]any{
		"account": accountID,
		"use_case": map[string]any{"type": "account_onboarding", "account_onboarding": map[string]any{
			"return_url": returnURL, "refresh_url": refreshURL,
		}},
	}
	var out struct {
		URL       string     `json:"url"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.json(ctx, http.MethodPost, "/v2/core/account_links", body, "", &out); err != nil {
		return AccountLink{}, err
	}
	if out.URL == "" {
		return AccountLink{}, errors.New("stripe：Account Link 的回應沒有 url")
	}
	return AccountLink{URL: out.URL, ExpiresAt: out.ExpiresAt}, nil
}

// AccountSession 是內嵌開通元件的 session。
type AccountSession struct {
	ClientSecret string
	ExpiresAt    *time.Time
}

// CreateAccountSession 產生 v1 Account Session（內嵌的 account_onboarding 元件）。
func (c *Client) CreateAccountSession(ctx context.Context, accountID string) (AccountSession, error) {
	form := url.Values{"account": {accountID}, "components[account_onboarding][enabled]": {"true"}}
	data, err := c.do(ctx, request{method: http.MethodPost, path: "/v1/account_sessions", body: []byte(form.Encode()), contentType: "application/x-www-form-urlencoded"})
	if err != nil {
		return AccountSession{}, err
	}
	var out struct {
		ClientSecret string `json:"client_secret"`
		ExpiresAt    *int64 `json:"expires_at"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return AccountSession{}, fmt.Errorf("stripe：解析回應失敗：%w", err)
	}
	if out.ClientSecret == "" {
		return AccountSession{}, errors.New("stripe：Account Session 的回應沒有 client_secret")
	}
	s := AccountSession{ClientSecret: out.ClientSecret}
	if out.ExpiresAt != nil {
		t := time.Unix(*out.ExpiresAt, 0).UTC()
		s.ExpiresAt = &t
	}
	return s, nil
}

// StatusDetail 是能力狀態的一筆細節：code 是原因碼，resolution 是誰要行動（provide_info、contact_stripe、no_resolution）。
type StatusDetail struct {
	Code       string `json:"code"`
	Resolution string `json:"resolution"`
}

// Capability 是能力的狀態與細節；Present 為 false 表示回應裡沒有這個能力。
type Capability struct {
	Present bool
	Status  string
	Details []StatusDetail
}

// Account 是 v2 帳戶中收款開通用得到的欄位；其餘欄位丟棄。
type Account struct {
	ID           string
	Transfers    Capability
	Payouts      Capability
	Requirements json.RawMessage
}

type capabilityWire struct {
	Status        string         `json:"status"`
	StatusDetails []StatusDetail `json:"status_details"`
}

func (w *capabilityWire) capability() Capability {
	if w == nil {
		return Capability{}
	}
	return Capability{Present: true, Status: w.Status, Details: w.StatusDetails}
}

// RetrieveAccount 讀 v2 帳戶，include recipient 設定與 requirements。
func (c *Client) RetrieveAccount(ctx context.Context, accountID string) (Account, error) {
	path := "/v2/core/accounts/" + url.PathEscape(accountID) + "?include=configuration.recipient&include=requirements"
	var out struct {
		ID            string `json:"id"`
		Configuration struct {
			Recipient *struct {
				Capabilities struct {
					StripeBalance struct {
						StripeTransfers *capabilityWire `json:"stripe_transfers"`
						Payouts         *capabilityWire `json:"payouts"`
					} `json:"stripe_balance"`
				} `json:"capabilities"`
			} `json:"recipient"`
		} `json:"configuration"`
		Requirements json.RawMessage `json:"requirements"`
	}
	if err := c.json(ctx, http.MethodGet, path, nil, "", &out); err != nil {
		return Account{}, err
	}
	if out.ID == "" {
		return Account{}, errors.New("stripe：讀取帳戶的回應沒有 id")
	}
	a := Account{ID: out.ID, Requirements: out.Requirements}
	if r := out.Configuration.Recipient; r != nil {
		a.Transfers = r.Capabilities.StripeBalance.StripeTransfers.capability()
		a.Payouts = r.Capabilities.StripeBalance.Payouts.capability()
	}
	return a, nil
}
