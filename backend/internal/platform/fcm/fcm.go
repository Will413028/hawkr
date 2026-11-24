// Package fcm 是 Firebase Cloud Messaging（HTTP v1）的推播 adapter：只用 net/http 與 oauth2 取得 access token，不重試——
// 每個 token 的結果分成已送出、token 失效、暫時性失敗、永久失敗與憑證錯誤，由呼叫端（River job）決定重試與停用 token。
package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// MaxBatch 是一批最多的 token 數；Send 收到更多時自己分批。
const MaxBatch = 500

const (
	scope       = "https://www.googleapis.com/auth/firebase.messaging"
	endpoint    = "https://fcm.googleapis.com"
	concurrency = 16
	maxResponse = 1 << 20
)

// Message 是要推給裝置的內容。Data 的值都必須是字串（FCM 的限制）。
type Message struct {
	Title string
	Body  string
	// ImageURL 非空時帶在 apns.fcm_options.image（iOS 的 notification service extension 下載後附在通知上）。
	ImageURL string
	Data     map[string]string
}

// Outcome 是一個 token 的送出結果。
type Outcome int

const (
	// OutcomeSent 已交給 FCM。
	OutcomeSent Outcome = iota
	// OutcomeUnregistered 是 FCM 回 UNREGISTERED 或 SENDER_ID_MISMATCH：token 已失效，呼叫端應停用（刪除）它。只有這兩個碼會讓 token 被停用。
	OutcomeUnregistered
	// OutcomeTransient 是暫時性失敗（配額、服務暫時不可用、網路）：重試可能成功，token 不動。
	OutcomeTransient
	// OutcomePermanent 是永久失敗（例如 INVALID_ARGUMENT）：重試不會成功，但不能據此停用 token。
	OutcomePermanent
	// OutcomeAuth 是 THIRD_PARTY_AUTH_ERROR 或本服務的憑證被拒（401、403）：是設定問題，呼叫端要告警；token 不動，重試在修好設定後可能成功。
	OutcomeAuth
)

// Result 是一個 token 的結果；Detail 是 FCM 的錯誤碼或 HTTP 狀態（不含 token 與訊息內容）。
type Result struct {
	Token   string
	Outcome Outcome
	Detail  string
}

// Sender 把一則訊息推給一批 token，回傳與 tokens 等長、同序的結果。error 只用在整個呼叫無法進行（例如 ctx 取消、而且還沒有任何 token 被收下）；送到一半被取消、已有 token 被收下時回傳部分結果（其餘 token 記暫時性失敗）而不是錯誤。
type Sender interface {
	Send(ctx context.Context, tokens []string, m Message) ([]Result, error)
}

// Config 是 Client 的設定。
type Config struct {
	ProjectID string
	// ServiceAccountJSON 是 Firebase 服務帳戶的金鑰內容（不是路徑）。
	ServiceAccountJSON []byte
	// BaseURL 與 HTTPClient 只給測試換掉；空值是正式的 FCM 與預設逾時的 client。
	BaseURL     string
	HTTPClient  *http.Client
	TokenSource oauth2.TokenSource
}

// Client 是 FCM HTTP v1 的 Sender。
type Client struct {
	project string
	base    string
	http    *http.Client
	tokens  oauth2.TokenSource
}

// New 建立 Client；ProjectID 必填，TokenSource 與 ServiceAccountJSON 至少一個。
func New(ctx context.Context, c Config) (*Client, error) {
	if c.ProjectID == "" {
		return nil, errors.New("fcm：缺少 ProjectID")
	}
	ts := c.TokenSource
	if ts == nil {
		if len(c.ServiceAccountJSON) == 0 {
			return nil, errors.New("fcm：缺少服務帳戶金鑰")
		}
		cfg, err := google.JWTConfigFromJSON(c.ServiceAccountJSON, scope)
		if err != nil {
			return nil, fmt.Errorf("fcm：服務帳戶金鑰不合格式：%w", err)
		}
		ts = oauth2.ReuseTokenSource(nil, cfg.TokenSource(ctx))
	}
	cl := &Client{project: c.ProjectID, base: c.BaseURL, http: c.HTTPClient, tokens: ts}
	if cl.base == "" {
		cl.base = endpoint
	}
	if cl.http == nil {
		cl.http = &http.Client{Timeout: 15 * time.Second}
	}
	return cl, nil
}

// Chunks 把 tokens 依 MaxBatch 切成批。
func Chunks(tokens []string) [][]string {
	return slices.Collect(slices.Chunk(tokens, MaxBatch))
}

// Send 逐批（每批至多 MaxBatch 個 token）推送；HTTP v1 沒有多播端點，批內以有限的並行逐 token 呼叫。
func (c *Client) Send(ctx context.Context, tokens []string, m Message) ([]Result, error) {
	out := make([]Result, 0, len(tokens))
	access, err := c.tokens.Token()
	if err != nil {
		return nil, fmt.Errorf("fcm：取得 access token：%w", err)
	}
	for _, batch := range Chunks(tokens) {
		res := make([]Result, len(batch))
		var wg sync.WaitGroup
		sem := make(chan struct{}, concurrency)
		for i, tok := range batch {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				res[i] = c.sendOne(ctx, access.AccessToken, tok, m)
			}()
		}
		wg.Wait()
		out = append(out, res...)
		if err := ctx.Err(); err != nil {
			return cancelled(out, tokens, err)
		}
	}
	return out, nil
}

// cancelled 處理送到一半 ctx 取消：已有 token 被 FCM 收下時回傳已完成批次的結果、其餘 token 記暫時性失敗，
// 讓呼叫端能把已送出的結果記進帳本，免得重試時對已收到的裝置再推一次；一個都沒送出時回 ctx 的錯誤，重試是安全的。
func cancelled(done []Result, tokens []string, err error) ([]Result, error) {
	if !slices.ContainsFunc(done, func(r Result) bool { return r.Outcome == OutcomeSent }) {
		return nil, err
	}
	for _, tok := range tokens[len(done):] {
		done = append(done, Result{Token: tok, Outcome: OutcomeTransient, Detail: "canceled"})
	}
	return done, nil
}

type fcmError struct {
	Error struct {
		Status  string `json:"status"`
		Details []struct {
			ErrorCode string `json:"errorCode"`
		} `json:"details"`
	} `json:"error"`
}

func (c *Client) sendOne(ctx context.Context, access, token string, m Message) Result {
	body, err := json.Marshal(Payload(token, m))
	if err != nil {
		return Result{Token: token, Outcome: OutcomePermanent, Detail: "marshal"}
	}
	u := c.base + "/v1/projects/" + url.PathEscape(c.project) + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return Result{Token: token, Outcome: OutcomePermanent, Detail: "request"}
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{Token: token, Outcome: OutcomeTransient, Detail: "network"}
	}
	defer resp.Body.Close() //nolint:errcheck // 回應本文已讀完
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Result{Token: token, Outcome: OutcomeSent}
	}
	return classify(token, resp.StatusCode, raw)
}

func classify(token string, status int, raw []byte) Result {
	var e fcmError
	_ = json.Unmarshal(raw, &e)
	code := e.Error.Status
	for _, d := range e.Error.Details {
		if d.ErrorCode != "" {
			code = d.ErrorCode
		}
	}
	detail := code
	if detail == "" {
		detail = fmt.Sprintf("http %d", status)
	}
	var o Outcome
	switch {
	case code == "UNREGISTERED" || code == "SENDER_ID_MISMATCH":
		o = OutcomeUnregistered
	case code == "THIRD_PARTY_AUTH_ERROR" || status == http.StatusUnauthorized || status == http.StatusForbidden:
		o = OutcomeAuth
	case code == "QUOTA_EXCEEDED" || code == "UNAVAILABLE" || code == "INTERNAL" || status == http.StatusTooManyRequests || status >= 500:
		o = OutcomeTransient
	default:
		o = OutcomePermanent
	}
	return Result{Token: token, Outcome: o, Detail: detail}
}

// Payload 是送給 FCM HTTP v1 的請求本文：可見的 notification、data，加上 iOS 的 mutable-content（讓 App 的 notification service extension 能改寫通知）與可選的圖片。
func Payload(token string, m Message) map[string]any {
	apns := map[string]any{"payload": map[string]any{"aps": map[string]any{"mutable-content": 1}}}
	if m.ImageURL != "" {
		apns["fcm_options"] = map[string]string{"image": m.ImageURL}
	}
	msg := map[string]any{"token": token, "notification": map[string]string{"title": m.Title, "body": m.Body}, "apns": apns}
	if len(m.Data) > 0 {
		msg["data"] = m.Data
	}
	return map[string]any{"message": msg}
}
