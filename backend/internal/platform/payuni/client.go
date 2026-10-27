// Package payuni 是 PayUni 代理商（子商店）API 的 HTTP client：bind_merchant、update_merchant、
// search_merchant_status。它只負責傳輸與加解密，不含任何業務判斷（哪些欄位送什麼值是呼叫方的事）。
//
// 規則（backend AGENTS.md 規則 5 與「設定與 secret」）：
//   - 沒有預設的網址，也沒有預設的金鑰：BaseURL、AgentID、HashKey、HashIV 都由部署設定給，缺一個 New 就失敗。
//   - 每一次呼叫都有 deadline（Config.Timeout，再受 ctx 限制），而且合計不超過 Config.MaxQPS（預設 2，PayUni 的限制）：
//     呼叫先排隊等時段，等待受 ctx 的 deadline 限制。
//   - 傳輸錯誤（連線、逾時、非 200、回應解不開）一律當錯誤回傳，不轉成業務結果；PayUni 回了業務失敗
//     （Status 不是 SUCCESS）回 *APIError，呼叫方以它分辨「可以重試」與「要人處理」。
//   - 請求與回應的內容含個資，這裡不記 log。
package payuni

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/phone"

	"golang.org/x/time/rate"
)

// Config 是 client 的設定。
type Config struct {
	// BaseURL 是 PayUni API 的根網址（含 scheme）；沒有預設值。
	BaseURL string
	// AgentID、HashKey、HashIV 是代理商的識別與金鑰；HashKey 32 bytes、HashIV 16 bytes。
	AgentID string
	HashKey string
	HashIV  string
	// Timeout 是單次 HTTP 呼叫的上限，0 為 15 秒。
	Timeout time.Duration
	// MaxQPS 是這個 client 所有呼叫合計的速率上限（PayUni 的外部限制），0 為 DefaultMaxQPS。一個程序只該有一個 client。
	MaxQPS float64
	// HTTPClient 給測試注入；nil 時用 Timeout 建一個。
	HTTPClient *http.Client
}

// DefaultTimeout 是 Config.Timeout 的預設值。
const DefaultTimeout = 15 * time.Second

// maxResponse 是回應本文的讀取上限。
const maxResponse = 1 << 20

// Client 呼叫 PayUni 代理商 API。
type Client struct {
	base    *url.URL
	agentID string
	key, iv []byte
	http    *http.Client
	limit   *rate.Limiter
}

// New 建立 client；設定不完整或格式不合時回錯誤。
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" || cfg.AgentID == "" || cfg.HashKey == "" || cfg.HashIV == "" {
		return nil, errors.New("payuni：BaseURL、AgentID、HashKey、HashIV 都必須設定")
	}
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" {
		return nil, errors.New("payuni：BaseURL 必須是含 scheme 與主機的網址")
	}
	if len(cfg.HashKey) != keyLen || len(cfg.HashIV) != ivLen {
		return nil, fmt.Errorf("payuni：HashKey 必須是 %d bytes、HashIV 必須是 %d bytes", keyLen, ivLen)
	}
	hc := cfg.HTTPClient
	if hc == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = DefaultTimeout
		}
		hc = &http.Client{Timeout: timeout}
	}
	return &Client{base: base, agentID: cfg.AgentID, key: []byte(cfg.HashKey), iv: []byte(cfg.HashIV), http: hc, limit: newLimiter(cfg.MaxQPS)}, nil
}

// APIError 是 PayUni 回的業務失敗：Status 不是 SUCCESS，Code 是它的 Status（例如 AGENT04017），Message 是它的說明。
type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string { return "payuni：" + e.Code + " " + e.Message }

// AsAPIError 取出 PayUni 的業務失敗；傳輸錯誤回 false。
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

// statusSuccess 是 PayUni 成功時的 Status。
const statusSuccess = "SUCCESS"

// BindMerchantRequest 是 bind_merchant 的欄位（PayUni 的名稱）。Optional 的欄位為空時不送；其餘一律送出，呼叫方負責
// 在送出前確認必填欄位有真實的值（這裡不補預設值）。
type BindMerchantRequest struct {
	Name, UBN, Email, MerID, MerEmail, MerName, MerEngName, MerURL, MerAddr, MerAddrPost, MerDesc, ContactName, ContactMobile string
	GoodsRate, Payment                                                                                                        string
	MerType, MerMCC, DeliverTime, EscrowType, MerUnitPrice, SellType                                                          int
	// Optional。
	MerTel, MBCName, MBCTel, MBCMail, MCName, MCID, ContactTel string
}

func (r BindMerchantRequest) values() url.Values {
	v := url.Values{}
	for k, s := range map[string]string{
		"Name": r.Name, "UBN": r.UBN, "Email": r.Email, "MerID": r.MerID, "MerEmail": r.MerEmail, "MerName": r.MerName,
		"MerEngName": r.MerEngName, "MerURL": r.MerURL, "MerAddr": r.MerAddr, "MerAddrPost": r.MerAddrPost, "MerDesc": r.MerDesc,
		"ContactName": r.ContactName, "ContactMobile": r.ContactMobile, "GoodsRate": r.GoodsRate, "Payment": r.Payment,
	} {
		v.Set(k, s)
	}
	for k, n := range map[string]int{
		"MerType": r.MerType, "MerMCC": r.MerMCC, "DeliverTime": r.DeliverTime, "EscrowType": r.EscrowType,
		"MerUnitPrice": r.MerUnitPrice, "SellType": r.SellType,
	} {
		v.Set(k, fmt.Sprint(n))
	}
	setIf(v, map[string]string{
		"MerTel": wirePhone(r.MerTel), "MBCName": r.MBCName, "MBCTel": wirePhone(r.MBCTel), "MBCMail": r.MBCMail, "MCName": r.MCName, "MCID": r.MCID,
		"ContactTel": wirePhone(r.ContactTel),
	})
	return v
}

// UpdateMerchantRequest 是 update_merchant 的欄位：真正的部分更新，沒有給的欄位 PayUni 不動。這個型別刻意只宣告
// 聯絡與地址欄位，費率、付款方式與開關（Fee、Payment、PaymentOff、CloseApplySwitch、OpenApplySwitch）不在裡面，
// 賣家無論如何碰不到。
type UpdateMerchantRequest struct {
	MerID                                                                          string
	ContactName, ContactMobile, ContactTel, MerEmail, MerAddr, MerAddrPost, MerTel string
}

func (r UpdateMerchantRequest) values() url.Values {
	v := url.Values{"MerID": {r.MerID}}
	setIf(v, map[string]string{
		"ContactName": r.ContactName, "ContactMobile": r.ContactMobile, "ContactTel": wirePhone(r.ContactTel), "MerEmail": r.MerEmail,
		"MerAddr": r.MerAddr, "MerAddrPost": r.MerAddrPost, "MerTel": wirePhone(r.MerTel),
	})
	return v
}

// wirePhone 把正規化的電話（只有數字，可帶「#分機」）轉成 PayUni 要的「區碼-電話號碼」（代理商文件 §7.3.3，例 02-9888888#123）。
// 認不得的格式原樣送出，由 PayUni 以格式錯誤的碼拒絕（賣家可修正的店家欄位），不在這裡猜。
func wirePhone(value string) string {
	if formatted, ok := phone.FormatWithAreaCode(value); ok {
		return formatted
	}
	return value
}

func setIf(v url.Values, fields map[string]string) {
	for k, s := range fields {
		if s != "" {
			v.Set(k, s)
		}
	}
}

// MerchantStatus 是 search_merchant_status 的結果：PayUni 的原始狀態碼（外部事實，不在這裡解讀）。
type MerchantStatus struct {
	MerID        string
	MerStatus    string
	MemberStatus string
	Reason       string
	Credit       string
	ApplePay     string
	GooglePay    string
	ATM          string
}

// BindMerchant 呼叫 /api/agent/bind_merchant。MerID 已存在回 *APIError（Code AGENT04017）。
func (c *Client) BindMerchant(ctx context.Context, req BindMerchantRequest) error {
	_, err := c.call(ctx, "/api/agent/bind_merchant", req.values(), false)
	return err
}

// UpdateMerchant 呼叫 /api/agent/update_merchant。
func (c *Client) UpdateMerchant(ctx context.Context, req UpdateMerchantRequest) error {
	_, err := c.call(ctx, "/api/agent/update_merchant", req.values(), false)
	return err
}

// MerchantStatus 呼叫 /api/agent/search_merchant_status。
func (c *Client) MerchantStatus(ctx context.Context, merID string) (MerchantStatus, error) {
	vals, err := c.call(ctx, "/api/agent/search_merchant_status", url.Values{"MerID": {merID}}, true)
	if err != nil {
		return MerchantStatus{}, err
	}
	// 狀態欄位的名稱照 PayUni 的規格：CREDIT、ApplePay、GooglePay、ATM（沒有 Status 後綴）。
	return MerchantStatus{
		MerID: vals.Get("MerID"), MerStatus: vals.Get("MerStatus"), MemberStatus: vals.Get("MemberStatus"), Reason: vals.Get("Reason"),
		Credit: vals.Get("CREDIT"), ApplePay: vals.Get("ApplePay"), GooglePay: vals.Get("GooglePay"), ATM: vals.Get("ATM"),
	}, nil
}

// call 加密並送出一個請求。bind、update 的回應只有 Status 與 Message 的 JSON；search 的回應有 EncryptInfo，解密後
// 是 URL-encoded 的查詢字串（decrypted 為 true）。
func (c *Client) call(ctx context.Context, path string, payload url.Values, decrypted bool) (url.Values, error) {
	encryptInfo, err := encrypt(payload.Encode(), c.key, c.iv)
	if err != nil {
		return nil, err
	}
	form := url.Values{
		"AgentID": {c.agentID}, "Version": {"1.0"}, "EncryptInfo": {encryptInfo}, "HashInfo": {hashInfo(encryptInfo, c.key, c.iv)},
	}
	endpoint := c.base.JoinPath(path).String()
	// Wait 在預約的時段晚於 ctx 的 deadline 時立刻回錯誤、不佔用時段（呼叫方當成暫時性的失敗重試）。
	if err := c.limit.Wait(ctx); err != nil {
		return nil, fmt.Errorf("payuni：呼叫上限排不進 deadline 之前：%w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("payuni：建立請求：%w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("payuni：呼叫 %s：%w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return nil, fmt.Errorf("payuni：讀取 %s 的回應：%w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		// 回應本文可能含請求的內容，只留狀態碼。
		return nil, fmt.Errorf("payuni：%s 回 HTTP %d", path, resp.StatusCode)
	}
	return parse(path, body, decrypted, c.key, c.iv)
}

type envelope struct {
	Status      string `json:"Status"`
	Message     string `json:"Message"`
	EncryptInfo string `json:"EncryptInfo"`
}

// parse 解讀回應：業務失敗是 *APIError，格式不合是一般錯誤（呼叫方當成暫時性的）。
func parse(path string, body []byte, decrypted bool, key, iv []byte) (url.Values, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("payuni：%s 的回應不是 JSON", path)
	}
	if !decrypted {
		switch env.Status {
		case statusSuccess:
			return nil, nil
		case "":
			return nil, fmt.Errorf("payuni：%s 的回應沒有 Status", path)
		}
		return nil, &APIError{Code: env.Status, Message: env.Message}
	}
	if env.EncryptInfo == "" {
		if env.Status != "" && env.Status != statusSuccess {
			return nil, &APIError{Code: env.Status, Message: env.Message}
		}
		return nil, fmt.Errorf("payuni：%s 的回應沒有 EncryptInfo", path)
	}
	plain, err := decrypt(env.EncryptInfo, key, iv)
	if err != nil {
		return nil, err
	}
	vals, err := url.ParseQuery(plain)
	if err != nil {
		return nil, fmt.Errorf("payuni：%s 的回應內容不是查詢字串", path)
	}
	if status := vals.Get("Status"); status != "" && status != statusSuccess {
		return nil, &APIError{Code: status, Message: vals.Get("Message")}
	}
	return vals, nil
}
