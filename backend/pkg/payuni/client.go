package payuni

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	SandboxBaseURL = "https://sandbox-api.payuni.com.tw"
	ProdBaseURL    = "https://api.payuni.com.tw"
)

type Client struct {
	AgentID    string
	HashKey    string
	HashIV     string
	IsSandbox  bool
	HTTPClient *http.Client

	// testBaseURL overrides BaseURL() when set. Test-only seam (same-package
	// tests construct Client via struct literal); zero value never affects
	// production callers, which always go through NewClient.
	testBaseURL string
}

func NewClient(agentID, hashKey, hashIV string, isSandbox bool) *Client {
	return &Client{
		AgentID:    agentID,
		HashKey:    hashKey,
		HashIV:     hashIV,
		IsSandbox:  isSandbox,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) BaseURL() string {
	if c.testBaseURL != "" {
		return c.testBaseURL
	}
	if c.IsSandbox {
		return SandboxBaseURL
	}
	return ProdBaseURL
}

// BindSellerRequest represents the data for binding (registering) a merchant
// url tags are used by go-querystring for EncryptInfo serialization
type BindSellerRequest struct {
	Name          string `json:"Name" url:"Name"`
	UBN           string `json:"UBN" url:"UBN"`
	Email         string `json:"Email" url:"Email"`
	MerID         string `json:"MerID" url:"MerID"`
	MerEmail      string `json:"MerEmail" url:"MerEmail"`
	MerName       string `json:"MerName" url:"MerName"`
	MerEngName    string `json:"MerEngName" url:"MerEngName"`
	MerURL        string `json:"MerURL" url:"MerURL"`
	MerAddr       string `json:"MerAddr" url:"MerAddr"`
	MerAddrPost   string `json:"MerAddrPost" url:"MerAddrPost"`
	MerDesc       string `json:"MerDesc" url:"MerDesc"`
	ContactName   string `json:"ContactName" url:"ContactName"`
	ContactMobile string `json:"ContactMobile" url:"ContactMobile"`
	MerType       int32  `json:"MerType" url:"MerType"`
	MerMCC        int32  `json:"MerMCC" url:"MerMCC"`
	GoodsRate     string `json:"GoodsRate" url:"GoodsRate"`
	Payment       string `json:"Payment" url:"Payment"`
	DeliverTime   int32  `json:"DeliverTime" url:"DeliverTime"`
	EscrowType    int32  `json:"EscrowType" url:"EscrowType"`
	MerUnitPrice  int32  `json:"MerUnitPrice" url:"MerUnitPrice"`
	SellType      int32  `json:"SellType" url:"SellType"`

	// Optional fields
	Fee    string `json:"Fee,omitempty" url:"Fee,omitempty"`
	MerTel string `json:"MerTel,omitempty" url:"MerTel,omitempty"`

	// Business member fields (omitempty: not sent for individual sellers)
	MBCName    string `json:"MBCName,omitempty" url:"MBCName,omitempty"`
	MBCTel     string `json:"MBCTel,omitempty" url:"MBCTel,omitempty"`
	MBCMail    string `json:"MBCMail,omitempty" url:"MBCMail,omitempty"`
	MCName     string `json:"MCName,omitempty" url:"MCName,omitempty"`
	MCID       string `json:"MCID,omitempty" url:"MCID,omitempty"`
	ContactTel string `json:"ContactTel,omitempty" url:"ContactTel,omitempty"`
}

// UpdateMerchantRequest carries the issue #2806 Phase 2 deny-by-default
// allowlist for PAYUni's update_merchant API. Confirmed against official
// PAYUni docs (2026-08-30): this is a true partial update — omit any field
// you are not changing, do not resend the full profile. MerID is the only
// required field. Deliberately does NOT declare Fee/Payment/PaymentOff/
// CloseApplySwitch/OpenApplySwitch — those are real update_merchant fields
// PAYUni accepts, but Seller must never be able to reach them through this
// type no matter what a caller sends.
type UpdateMerchantRequest struct {
	MerID         string `json:"MerID" url:"MerID"`
	ContactName   string `json:"ContactName,omitempty" url:"ContactName,omitempty"`
	ContactMobile string `json:"ContactMobile,omitempty" url:"ContactMobile,omitempty"`
	ContactTel    string `json:"ContactTel,omitempty" url:"ContactTel,omitempty"`
	MerEmail      string `json:"MerEmail,omitempty" url:"MerEmail,omitempty"`
	MerAddr       string `json:"MerAddr,omitempty" url:"MerAddr,omitempty"`
	MerAddrPost   string `json:"MerAddrPost,omitempty" url:"MerAddrPost,omitempty"`
	MerTel        string `json:"MerTel,omitempty" url:"MerTel,omitempty"`
}

// AgentApiResponse generic response
type AgentApiResponse struct {
	Status      string `json:"Status"`
	Message     string `json:"Message,omitempty"`
	Version     string `json:"Version,omitempty"`
	EncryptInfo string `json:"EncryptInfo,omitempty"`
	HashInfo    string `json:"HashInfo,omitempty"`
}

// SearchSellerStatusRequest
type SearchSellerStatusRequest struct {
	MerID string `json:"MerID" url:"MerID"`
}

// SellerStatusResponse (Decrypted data) — fields aligned to PayUni docs:
// https://docs.payuni.com.tw/web/#/9/138
//
// JSON tags for channel statuses intentionally drop the "Status" suffix
// because PayUni's spec names them CREDIT / ApplePay / GooglePay / ATM.
// Go field names keep the Status suffix for readability at call sites.
//
// `Raw` carries every key/value PayUni returned (the EncryptInfo decrypts to
// a URL-encoded query string), serialized as a JSON object so the caller can
// persist it as jsonb and query individual keys later. Includes keys the
// struct doesn't decode (Red / UnionPay / Inst3-30 / ForeignCard / SamsungPay
// / CVS / ICASH / AFTEE / JKoPay / SP* / HOME) so future SDK additions can
// read historical rows without losing fidelity.
type SellerStatusResponse struct {
	Status          string          `json:"Status"`
	Message         string          `json:"Message"`
	MerID           string          `json:"MerID"`
	MerStatus       string          `json:"MerStatus"`
	Reason          string          `json:"Reason"` // 商店審核未通過原因, max 200 chars
	MemberStatus    string          `json:"MemberStatus"`
	CreditStatus    string          `json:"CREDIT"`
	ApplePayStatus  string          `json:"ApplePay"`
	GooglePayStatus string          `json:"GooglePay"`
	ATMStatus       string          `json:"ATM"`
	Raw             json.RawMessage `json:"-"`
}

// SearchSellerApiResponse (Response for search endpoint)
type SearchSellerApiResponse struct {
	AgentID     string `json:"AgentID"`
	MerID       string `json:"MerID"`
	Version     string `json:"Version"`
	EncryptInfo string `json:"EncryptInfo"`
	HashInfo    string `json:"HashInfo"`
}

// BindMerchant calls /api/agent/bind_merchant
func (c *Client) BindSeller(ctx context.Context, req *BindSellerRequest) (*AgentApiResponse, error) {
	// Encrypt
	encryptInfo, err := Encrypt(req, c.HashKey, c.HashIV)
	if err != nil {
		return nil, fmt.Errorf("encrypt failed: %w", err)
	}

	hashInfo := GenerateHash(encryptInfo, c.HashKey, c.HashIV)

	form := url.Values{}
	form.Set("AgentID", c.AgentID)
	form.Set("Version", "1.0")
	form.Set("EncryptInfo", encryptInfo)
	form.Set("HashInfo", hashInfo)

	respBody, err := c.postForm(ctx, "/api/agent/bind_merchant", form)
	if err != nil {
		return nil, err
	}

	var apiResp AgentApiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response %s failed: %w", string(respBody), err)
	}

	if apiResp.Status != "SUCCESS" {
		encryptPrefix := encryptInfo
		if len(encryptPrefix) > 40 {
			encryptPrefix = encryptPrefix[:40]
		}
		log.Printf("[PAYUNi BindSeller] 失敗 Status=%s Message=%s EncryptInfo(前40)=%s HashInfo=%s",
			apiResp.Status, apiResp.Message, encryptPrefix, hashInfo)
	}

	return &apiResp, nil
}

// UpdateMerchant calls /api/agent/update_merchant. Response shape matches
// BindSeller (Status/Message only, no EncryptInfo payload to decrypt) —
// confirmed against official PAYUni docs, 2026-08-30.
func (c *Client) UpdateMerchant(ctx context.Context, req *UpdateMerchantRequest) (*AgentApiResponse, error) {
	encryptInfo, err := Encrypt(req, c.HashKey, c.HashIV)
	if err != nil {
		return nil, fmt.Errorf("encrypt failed: %w", err)
	}

	hashInfo := GenerateHash(encryptInfo, c.HashKey, c.HashIV)

	form := url.Values{}
	form.Set("AgentID", c.AgentID)
	form.Set("Version", "1.0")
	form.Set("EncryptInfo", encryptInfo)
	form.Set("HashInfo", hashInfo)

	respBody, err := c.postForm(ctx, "/api/agent/update_merchant", form)
	if err != nil {
		return nil, err
	}

	var apiResp AgentApiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response %s failed: %w", string(respBody), err)
	}

	if apiResp.Status != "SUCCESS" {
		encryptPrefix := encryptInfo
		if len(encryptPrefix) > 40 {
			encryptPrefix = encryptPrefix[:40]
		}
		log.Printf("[PAYUNi UpdateMerchant] 失敗 Status=%s Message=%s EncryptInfo(前40)=%s HashInfo=%s",
			apiResp.Status, apiResp.Message, encryptPrefix, hashInfo)
	}

	return &apiResp, nil
}

// GetMerchantStatus calls /api/agent/merchant/{mer_id}/status
func (c *Client) GetSellerStatus(ctx context.Context, merID string) (*SellerStatusResponse, error) {
	req := SearchSellerStatusRequest{MerID: merID}

	encryptInfo, err := Encrypt(req, c.HashKey, c.HashIV)
	if err != nil {
		return nil, fmt.Errorf("encrypt failed: %w", err)
	}

	hashInfo := GenerateHash(encryptInfo, c.HashKey, c.HashIV)

	form := url.Values{}
	form.Set("AgentID", c.AgentID)
	form.Set("Version", "1.0")
	form.Set("EncryptInfo", encryptInfo)
	form.Set("HashInfo", hashInfo)

	respBody, err := c.postForm(ctx, "/api/agent/search_merchant_status", form)
	if err != nil {
		return nil, err
	}

	var apiResp SearchSellerApiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	if apiResp.EncryptInfo == "" {
		return nil, fmt.Errorf("missing EncryptInfo in response: %s", string(respBody))
	}

	// Decrypt
	decryptedJson, err := Decrypt(apiResp.EncryptInfo, c.HashKey, c.HashIV)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed: %w", err)
	}

	// Decrypted data is URL ENCODED query string
	// "Status=SUCCESS&Message=...&MerStatus=1..."
	// We need to parse this into SellerStatusResponse

	vals, err := url.ParseQuery(decryptedJson)
	if err != nil {
		return nil, fmt.Errorf("parse decrypted query string failed: %w", err)
	}

	// Preserve every key PayUni returned (including those the struct doesn't
	// decode — Reason / Red / UnionPay / Inst3-30 / ForeignCard / SamsungPay /
	// CVS / ICASH / AFTEE / JKoPay / SP* / HOME) so callers can persist as
	// jsonb and query later.
	rawMap := make(map[string]string, len(vals))
	for k, v := range vals {
		if len(v) > 0 {
			rawMap[k] = v[0]
		}
	}
	rawBytes, err := json.Marshal(rawMap)
	if err != nil {
		return nil, fmt.Errorf("marshal raw response map: %w", err)
	}

	// PayUni's official spec names the channel fields CREDIT / ApplePay /
	// GooglePay / ATM (no "Status" suffix). The previous SDK used
	// CreditStatus / ApplePayStatus / etc. and silently got empty strings.
	resp := &SellerStatusResponse{
		Status:          vals.Get("Status"),
		Message:         vals.Get("Message"),
		MerID:           vals.Get("MerID"),
		MerStatus:       vals.Get("MerStatus"),
		Reason:          vals.Get("Reason"),
		MemberStatus:    vals.Get("MemberStatus"),
		CreditStatus:    vals.Get("CREDIT"),
		ApplePayStatus:  vals.Get("ApplePay"),
		GooglePayStatus: vals.Get("GooglePay"),
		ATMStatus:       vals.Get("ATM"),
		Raw:             rawBytes,
	}

	return resp, nil
}

func (c *Client) postForm(ctx context.Context, endpoint string, data url.Values) ([]byte, error) {
	fullURL := c.BaseURL() + endpoint

	log.Printf("PAYUNI POST %s", fullURL)

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "payuni-go-client")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Printf("PAYUNI RESP: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("payuni api status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
