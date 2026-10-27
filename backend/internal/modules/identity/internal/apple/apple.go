// Package apple 呼叫 Sign in with Apple 的 REST API：以 authorization code 換 refresh token，以及撤銷
// token（App Store 審核準則 5.1.1(v)：刪除帳號時撤銷）。兩者都以 ES256 簽的 client secret 認證
// （Apple〈Creating a client secret〉）。
package apple

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// BaseURL 是 Apple 的帳號 API。
const BaseURL = "https://appleid.apple.com"

// requestTimeout 是每次呼叫的上限。
const requestTimeout = 10 * time.Second

// clientSecretTTL 是每次簽的 client secret 的效期；Apple 允許到六個月，每次呼叫重簽，短一點就好。
const clientSecretTTL = 5 * time.Minute

// ErrRejected 表示 Apple 以 4xx 拒絕（設定或參數錯誤），重試也不會成功。
var ErrRejected = errors.New("apple：請求被拒絕")

// Config 是 Apple 開發者帳號的設定。
type Config struct {
	TeamID string
	KeyID  string
	// PrivateKeyPEM 是 Sign in with Apple 金鑰（.p8）的內容。
	PrivateKeyPEM string
	// RedirectURIs 是各 client 在授權時用的 redirect URI：web（Services ID）換 token 時必須帶同一個；
	// App 的 client 不需要。
	RedirectURIs map[string]string
	BaseURL      string
	HTTPClient   *http.Client
	Now          func() time.Time
}

// Client 呼叫 Apple 的 token 與撤銷 API。
type Client struct {
	cfg Config
	key *ecdsa.PrivateKey
}

// New 建立 Client；金鑰格式不對時回錯。
func New(cfg Config) (*Client, error) {
	if cfg.TeamID == "" || cfg.KeyID == "" {
		return nil, errors.New("apple：team ID 與 key ID 都要設定")
	}
	key, err := parseKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = BaseURL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: requestTimeout}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now //nolint:forbidigo // assigns the default clock without calling it
	}
	return &Client{cfg: cfg, key: key}, nil
}

func parseKey(p string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(p))
	if block == nil {
		return nil, errors.New("apple：私鑰不是 PEM")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apple：解析私鑰：%w", err)
	}
	ek, ok := k.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("apple：私鑰不是 EC（ES256）")
	}
	return ek, nil
}

// clientSecret 簽出給 clientID 用的 client secret。
func (c *Client) clientSecret(clientID string) (string, error) {
	now := c.cfg.Now()
	tok := jwt.New()
	tok.Options().Enable(jwt.FlattenAudience)
	for name, value := range map[string]any{"iss": c.cfg.TeamID, "sub": clientID, "aud": []string{BaseURL}, "iat": now.Unix(), "exp": now.Add(clientSecretTTL).Unix()} {
		if err := tok.Set(name, value); err != nil {
			return "", err
		}
	}
	headers := jws.NewHeaders()
	if err := headers.Set(jws.KeyIDKey, c.cfg.KeyID); err != nil {
		return "", err
	}
	raw, err := jwt.Sign(tok, jwt.WithKey(jwa.ES256(), c.key, jws.WithProtectedHeaders(headers)))
	return string(raw), err
}

// Exchange 以 authorization code 換 refresh token，同時回傳 Apple 一併簽發的 ID token（用來確認這個 code 屬於誰）。
func (c *Client) Exchange(ctx context.Context, clientID, code string) (refreshToken, idToken string, err error) {
	form := url.Values{"code": {code}, "grant_type": {"authorization_code"}}
	if uri := c.cfg.RedirectURIs[clientID]; uri != "" {
		form.Set("redirect_uri", uri)
	}
	var resp struct {
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
	}
	if err := c.post(ctx, "/auth/token", clientID, form, &resp); err != nil {
		return "", "", fmt.Errorf("apple：換 refresh token：%w", err)
	}
	if resp.RefreshToken == "" {
		return "", "", errors.New("apple：換 refresh token：回應沒有 refresh_token")
	}
	return resp.RefreshToken, resp.IDToken, nil
}

// Revoke 撤銷 refresh token。token 已經失效時 Apple 回 invalid_grant，視為成功。
func (c *Client) Revoke(ctx context.Context, clientID, refreshToken string) error {
	form := url.Values{"token": {refreshToken}, "token_type_hint": {"refresh_token"}}
	err := c.post(ctx, "/auth/revoke", clientID, form, nil)
	var ae *apiError
	if errors.As(err, &ae) && ae.Code == "invalid_grant" {
		return nil
	}
	if err != nil {
		return fmt.Errorf("apple：撤銷 token：%w", err)
	}
	return nil
}

// apiError 是 Apple 的錯誤回應（RFC 6749 §5.2）。
type apiError struct {
	Status int
	Code   string `json:"error"`
}

func (e *apiError) Error() string { return fmt.Sprintf("HTTP %d：%s", e.Status, e.Code) }

func (e *apiError) Unwrap() error {
	if e.Status >= 400 && e.Status < 500 {
		return ErrRejected
	}
	return nil
}

func (c *Client) post(ctx context.Context, path, clientID string, form url.Values, out any) error {
	secret, err := c.clientSecret(clientID)
	if err != nil {
		return err
	}
	form.Set("client_id", clientID)
	form.Set("client_secret", secret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		ae := &apiError{Status: res.StatusCode}
		_ = json.Unmarshal(body, ae)
		return ae
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("解析回應：%w", err)
	}
	return nil
}
