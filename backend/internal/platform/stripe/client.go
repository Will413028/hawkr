// Package stripe 是 Stripe API 的 client（官方 SDK stripe-go 之上）：只含收款開通需要的呼叫（Accounts v2、Account Link、Account Session）與 webhook 簽章驗證。
// 它不認得賣家模組的型別；對應成中立快照的 adapter 在 payment 模組的 internal/account。
// 所有呼叫用 SDK 釘住的 API 版本（stripe.APIVersion），不覆寫 Stripe-Version。
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

	sdk "github.com/stripe/stripe-go/v87"
)

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
	// Wait 是 429 退避的等待；nil 時用計時器，ctx 結束就回傳。
	Wait func(ctx context.Context, d time.Duration) error
}

// Client 呼叫 Stripe。
type Client struct {
	sdk      *sdk.Client
	whsecret string
	timeout  time.Duration
	wait     func(ctx context.Context, d time.Duration) error
}

// New 建立 Client；金鑰與 webhook 密鑰必填。SDK 的自動網路重試、遙測與記錄都關掉：重試由這裡只對 429 做，其餘失敗交給呼叫方（River job）重試。
func New(cfg Config) (*Client, error) {
	if cfg.SecretKey == "" {
		return nil, errors.New("stripe：缺少 SecretKey")
	}
	if strings.TrimSpace(cfg.WebhookSecret) == "" {
		return nil, errors.New("stripe：缺少 WebhookSecret")
	}
	if strings.TrimSpace(cfg.WebhookSecret) != cfg.WebhookSecret {
		return nil, errors.New("stripe：WebhookSecret 前後有空白或換行，請去掉後再設定")
	}
	raw := cfg.BaseURL
	if raw == "" {
		raw = DefaultBaseURL
	}
	base, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") {
		return nil, fmt.Errorf("stripe：BaseURL %q 不是有效的網址", raw)
	}
	hc := &http.Client{}
	if cfg.HTTPClient != nil {
		cp := *cfg.HTTPClient
		hc = &cp
	}
	next := hc.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	hc.Transport = recorder{next: next}
	backends := sdk.NewBackendsWithConfig(&sdk.BackendConfig{
		URL:               sdk.String(base.String()),
		HTTPClient:        hc,
		MaxNetworkRetries: sdk.Int64(0),
		EnableTelemetry:   sdk.Bool(false),
		LeveledLogger:     &sdk.LeveledLogger{Level: sdk.LevelNull},
	})
	c := &Client{
		sdk:      sdk.NewClient(cfg.SecretKey, sdk.WithBackends(backends)),
		whsecret: cfg.WebhookSecret,
		timeout:  cfg.Timeout,
		wait:     cfg.Wait,
	}
	if c.timeout <= 0 {
		c.timeout = DefaultTimeout
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

// Permanent 回傳重試也不會成功：4xx 而且不是逾時（408）與限流（429）。409 只有 code 為 idempotency_error（冪等鍵搭配了不同的參數）才是永久的；
// 其他 409 是同一把鍵的另一個請求還在執行，等一下用同一把鍵重送即可（https://docs.stripe.com/error-low-level 的 Content errors）。
func (e *APIError) Permanent() bool {
	if e.Status == http.StatusConflict {
		return e.Code == "idempotency_error"
	}
	return e.Status >= 400 && e.Status < 500 && e.Status != http.StatusRequestTimeout && e.Status != http.StatusTooManyRequests
}

// exchange 是一次非 2xx 回應的狀態、標頭與本文；SDK 的錯誤型別依 API 版本與錯誤種類而異（有的不帶狀態碼），
// 所以錯誤分類只看 HTTP 層記下的這份。
type exchange struct {
	status int
	header http.Header
	body   []byte
}

type exchangeKey struct{}

func (e *exchange) apiError() *APIError {
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(e.body, &env)
	return &APIError{Status: e.status, Code: env.Error.Code, Message: env.Error.Message}
}

// recorder 讀完回應本文（上限 maxResponseBytes）再還給 SDK，非 2xx 時把狀態、標頭與本文記進呼叫的 ctx。
type recorder struct{ next http.RoundTripper }

func (r recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("stripe：讀回應失敗：%w", err)
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("stripe：回應超過 %d bytes", maxResponseBytes)
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	resp.ContentLength = int64(len(data))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if ex, ok := req.Context().Value(exchangeKey{}).(*exchange); ok {
			*ex = exchange{status: resp.StatusCode, header: resp.Header.Clone(), body: data}
		}
	}
	return resp, nil
}

// call 在總上限 timeout 內執行一次 SDK 呼叫：429 依 Retry-After（沒有就指數退避）等待後重送，最多 maxAttempts 次；
// 其他非 2xx 轉成 *APIError；沒有 HTTP 回應的失敗（連線、逾時、ctx 取消）原樣回傳。
func (c *Client) call(ctx context.Context, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	for attempt := 1; ; attempt++ {
		ex := &exchange{}
		err := fn(context.WithValue(ctx, exchangeKey{}, ex))
		if err == nil {
			return nil
		}
		if ex.status == 0 {
			return err
		}
		if ex.status == http.StatusTooManyRequests && attempt < maxAttempts {
			if werr := c.wait(ctx, backoff(attempt, ex.header.Get("Retry-After"))); werr != nil {
				return fmt.Errorf("stripe：限流退避被中斷：%w", werr)
			}
			continue
		}
		return ex.apiError()
	}
}

func backoff(attempt int, retryAfter string) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && s >= 0 {
		return min(time.Duration(s)*time.Second, maxBackoff)
	}
	return min(baseBackoff<<(attempt-1), maxBackoff)
}

// Dashboard 是連線帳戶的 dashboard 種類，決定誰向賣家收件：express 由 Stripe 收，none 由平台收（spike Q9）。
type Dashboard string

// Dashboard 的值。
const (
	DashboardExpress Dashboard = "express"
	DashboardNone    Dashboard = "none"
)

// CreateAccount 建立 v2 recipient 帳戶（US、individual、轉帳能力），以 idempotencyKey 冪等；回傳帳戶 ID。
// contactEmail 必填：有 recipient 設定的帳戶沒有它時 Stripe 回 400。
func (c *Client) CreateAccount(ctx context.Context, idempotencyKey string, dashboard Dashboard, contactEmail string) (string, error) {
	params := &sdk.V2CoreAccountCreateParams{
		Identity: &sdk.V2CoreAccountCreateIdentityParams{Country: sdk.String("us"), EntityType: sdk.String("individual")},
		Configuration: &sdk.V2CoreAccountCreateConfigurationParams{Recipient: &sdk.V2CoreAccountCreateConfigurationRecipientParams{
			Capabilities: &sdk.V2CoreAccountCreateConfigurationRecipientCapabilitiesParams{
				StripeBalance: &sdk.V2CoreAccountCreateConfigurationRecipientCapabilitiesStripeBalanceParams{
					StripeTransfers: &sdk.V2CoreAccountCreateConfigurationRecipientCapabilitiesStripeBalanceStripeTransfersParams{Requested: sdk.Bool(true)},
				},
			},
		}},
		Defaults: &sdk.V2CoreAccountCreateDefaultsParams{Responsibilities: &sdk.V2CoreAccountCreateDefaultsResponsibilitiesParams{
			FeesCollector: sdk.String("application"), LossesCollector: sdk.String("application"),
		}},
		Dashboard:    sdk.String(string(dashboard)),
		ContactEmail: sdk.String(contactEmail),
	}
	if idempotencyKey != "" {
		params.IdempotencyKey = sdk.String(idempotencyKey)
	}
	var out *sdk.V2CoreAccount
	if err := c.call(ctx, func(ctx context.Context) (err error) {
		out, err = c.sdk.V2CoreAccounts.Create(ctx, params)
		return err
	}); err != nil {
		return "", err
	}
	if out == nil || out.ID == "" {
		return "", errors.New("stripe：建立帳戶的回應沒有 id")
	}
	return out.ID, nil
}

// AccountLink 是託管開通頁面的連結。
type AccountLink struct {
	URL       string
	ExpiresAt *time.Time
}

// CreateAccountLink 產生 v2 Account Link（託管的開通頁面）。
func (c *Client) CreateAccountLink(ctx context.Context, accountID, returnURL, refreshURL string) (AccountLink, error) {
	params := &sdk.V2CoreAccountLinkCreateParams{
		Account: sdk.String(accountID),
		UseCase: &sdk.V2CoreAccountLinkCreateUseCaseParams{
			Type: sdk.String("account_onboarding"),
			AccountOnboarding: &sdk.V2CoreAccountLinkCreateUseCaseAccountOnboardingParams{
				ReturnURL:  sdk.String(returnURL),
				RefreshURL: sdk.String(refreshURL),
			},
		},
	}
	var out *sdk.V2CoreAccountLink
	if err := c.call(ctx, func(ctx context.Context) (err error) {
		out, err = c.sdk.V2CoreAccountLinks.Create(ctx, params)
		return err
	}); err != nil {
		return AccountLink{}, err
	}
	if out == nil || out.URL == "" {
		return AccountLink{}, errors.New("stripe：Account Link 的回應沒有 url")
	}
	link := AccountLink{URL: out.URL}
	if !out.ExpiresAt.IsZero() {
		t := out.ExpiresAt.UTC()
		link.ExpiresAt = &t
	}
	return link, nil
}

// AccountSession 是內嵌開通元件的 session。
type AccountSession struct {
	ClientSecret string
	ExpiresAt    *time.Time
}

// CreateAccountSession 產生 v1 Account Session（內嵌的 account_onboarding 元件）。
// 平台收集需求並承擔損失（losses_collector=application），所以停用 Stripe 使用者驗證
// （disable_stripe_user_authentication），內嵌表單直接顯示，不經 Stripe 登入視窗。
func (c *Client) CreateAccountSession(ctx context.Context, accountID string) (AccountSession, error) {
	params := &sdk.AccountSessionCreateParams{
		Account: sdk.String(accountID),
		Components: &sdk.AccountSessionCreateComponentsParams{AccountOnboarding: &sdk.AccountSessionCreateComponentsAccountOnboardingParams{
			Enabled:  sdk.Bool(true),
			Features: &sdk.AccountSessionCreateComponentsAccountOnboardingFeaturesParams{DisableStripeUserAuthentication: sdk.Bool(true)},
		}},
	}
	var out *sdk.AccountSession
	if err := c.call(ctx, func(ctx context.Context) (err error) {
		out, err = c.sdk.V1AccountSessions.Create(ctx, params)
		return err
	}); err != nil {
		return AccountSession{}, err
	}
	if out == nil || out.ClientSecret == "" {
		return AccountSession{}, errors.New("stripe：Account Session 的回應沒有 client_secret")
	}
	s := AccountSession{ClientSecret: out.ClientSecret}
	if out.ExpiresAt != 0 {
		t := time.Unix(out.ExpiresAt, 0).UTC()
		s.ExpiresAt = &t
	}
	return s, nil
}

// StatusDetail 是能力狀態的一筆細節：code 是原因碼，resolution 是誰要行動（provide_info、contact_stripe、no_resolution）。
type StatusDetail struct {
	Code       string `json:"code"`
	Resolution string `json:"resolution"`
}

// Capability 是能力的狀態與細節。
type Capability struct {
	Status  string
	Details []StatusDetail
}

// Account 是 v2 帳戶中收款開通用得到的欄位；其餘欄位丟棄。Requirements 是回應本文裡 requirements 的原始 JSON。
type Account struct {
	ID           string
	Transfers    Capability
	Payouts      Capability
	Requirements json.RawMessage
}

// RetrieveAccount 讀 v2 帳戶，include recipient 設定與 requirements。
func (c *Client) RetrieveAccount(ctx context.Context, accountID string) (Account, error) {
	params := &sdk.V2CoreAccountRetrieveParams{Include: sdk.StringSlice([]string{"configuration.recipient", "requirements"})}
	var out *sdk.V2CoreAccount
	if err := c.call(ctx, func(ctx context.Context) (err error) {
		out, err = c.sdk.V2CoreAccounts.Retrieve(ctx, accountID, params)
		return err
	}); err != nil {
		return Account{}, err
	}
	if out == nil || out.ID == "" {
		return Account{}, errors.New("stripe：讀取帳戶的回應沒有 id")
	}
	var bal *sdk.V2CoreAccountConfigurationRecipientCapabilitiesStripeBalance
	if out.Configuration != nil && out.Configuration.Recipient != nil && out.Configuration.Recipient.Capabilities != nil {
		bal = out.Configuration.Recipient.Capabilities.StripeBalance
	}
	if bal == nil || bal.StripeTransfers == nil {
		// 沒有轉帳能力的回應是格式錯誤，不是一個狀態；回一般錯誤讓呼叫方重試，不能當成「審核中」存下來。
		return Account{}, fmt.Errorf("stripe：帳戶 %s 的回應沒有 stripe_transfers 能力", out.ID)
	}
	acct := Account{ID: out.ID, Requirements: rawRequirements(out.LastResponse)}
	acct.Transfers = Capability{Status: string(bal.StripeTransfers.Status)}
	for _, d := range bal.StripeTransfers.StatusDetails {
		if d != nil {
			acct.Transfers.Details = append(acct.Transfers.Details, StatusDetail{Code: string(d.Code), Resolution: string(d.Resolution)})
		}
	}
	if p := bal.Payouts; p != nil {
		acct.Payouts = Capability{Status: string(p.Status)}
		for _, d := range p.StatusDetails {
			if d != nil {
				acct.Payouts.Details = append(acct.Payouts.Details, StatusDetail{Code: string(d.Code), Resolution: string(d.Resolution)})
			}
		}
	}
	return acct, nil
}

// rawRequirements 取回應本文裡 requirements 的原始 JSON（SDK 型別只保留它認得的欄位）。
func rawRequirements(res *sdk.APIResponse) json.RawMessage {
	if res == nil {
		return nil
	}
	var env struct {
		Requirements json.RawMessage `json:"requirements"`
	}
	if json.Unmarshal(res.RawJSON, &env) != nil {
		return nil
	}
	return env.Requirements
}

// CloseAccount 關閉 v2 帳戶的 recipient 設定。已關閉的帳戶再關會回 403：此時讀帳戶，closed 為真就視為成功；讀取成功但帳戶沒關時回原本的 403；
// 讀取本身失敗時回讀取的錯誤（不知道帳戶是否已關閉，暫時性的讀取失敗讓呼叫方重試，不被永久的 403 蓋掉）。
func (c *Client) CloseAccount(ctx context.Context, accountID string) error {
	params := &sdk.V2CoreAccountCloseParams{AppliedConfigurations: sdk.StringSlice([]string{"recipient"})}
	err := c.call(ctx, func(ctx context.Context) error {
		_, err := c.sdk.V2CoreAccounts.Close(ctx, accountID, params)
		return err
	})
	var api *APIError
	if err == nil || !errors.As(err, &api) || api.Status != http.StatusForbidden {
		return err
	}
	var acct *sdk.V2CoreAccount
	if rerr := c.call(ctx, func(ctx context.Context) (err error) {
		acct, err = c.sdk.V2CoreAccounts.Retrieve(ctx, accountID, &sdk.V2CoreAccountRetrieveParams{})
		return err
	}); rerr != nil {
		return rerr
	}
	if acct != nil && acct.Closed {
		return nil
	}
	return err
}

const (
	bankAccountsPageLimit = 100
	// maxBankAccountPages 是讀外部銀行帳號時最多翻的頁數；超過視為回應異常，不無限翻頁。
	maxBankAccountPages = 20
)

// ListBankAccountFingerprints 讀連線帳戶目前掛著的外部銀行帳號的 fingerprint（v1 external_accounts，只列 bank_account），翻頁讀完；
// 沒有 fingerprint 的項目略過。fingerprint 是 Stripe 對同一組銀行帳號在平台內一致的識別碼（https://docs.stripe.com/api/external_account_bank_accounts/list）。
func (c *Client) ListBankAccountFingerprints(ctx context.Context, accountID string) ([]string, error) {
	var out []string
	startingAfter := ""
	for range maxBankAccountPages {
		params := &sdk.BankAccountListParams{Account: sdk.String(accountID)}
		params.Limit = sdk.Int64(bankAccountsPageLimit)
		params.Single = true
		if startingAfter != "" {
			params.StartingAfter = sdk.String(startingAfter)
		}
		var data []*sdk.BankAccount
		var hasMore bool
		if err := c.call(ctx, func(ctx context.Context) error {
			list := c.sdk.V1BankAccounts.List(ctx, params)
			if err := list.Err(); err != nil {
				return err
			}
			data, hasMore = list.Data(), list.Meta().HasMore
			return nil
		}); err != nil {
			return nil, err
		}
		for _, ba := range data {
			if ba != nil && ba.Fingerprint != "" {
				out = append(out, ba.Fingerprint)
			}
		}
		if !hasMore {
			return out, nil
		}
		if len(data) == 0 || data[len(data)-1] == nil || data[len(data)-1].ID == "" {
			return nil, errors.New("stripe：外部銀行帳號的回應 has_more 為真但沒有可接續的項目")
		}
		startingAfter = data[len(data)-1].ID
	}
	return nil, fmt.Errorf("stripe：外部銀行帳號超過 %d 頁", maxBankAccountPages)
}
