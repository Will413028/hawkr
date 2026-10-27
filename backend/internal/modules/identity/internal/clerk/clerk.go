// Package clerk 是一個 Clerk application 的 client（ADR 2026-09-27-identity-clerk）：驗證它簽的 session token
// （固定 PEM 公鑰、不連網），以及 Backend API 的停用與刪除。
//
// 本機開發與測試不連 Clerk：Dev 以本機的 RSA 金鑰簽 token（hawkr dev-token），使用者是 dev_ 開頭的 ID。
package clerk

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/oidc"
)

// ErrInvalid 表示 session token 無效：簽章、issuer、期限、azp 或格式不對。
var ErrInvalid = errors.New("clerk：session token 無效")

// ErrRejected 表示 Backend API 拒絕了請求，而且重試也不會成功（金鑰不對、使用者的 ID 格式不對）。
var ErrRejected = errors.New("clerk：Backend API 拒絕，重試不會成功")

// ErrNotFound 表示使用者不存在。
var ErrNotFound = errors.New("clerk：使用者不存在")

// DefaultAPIURL 是 Backend API 的網址。
const DefaultAPIURL = "https://api.clerk.com"

// apiTimeout 是一次 Backend API 呼叫的上限。
const apiTimeout = 10 * time.Second

// Config 是一個 Clerk application 的設定。
type Config struct {
	// PublishableKey 是 pk_test_ 或 pk_live_ 開頭的 key；frontend API 的網址（token 的 issuer）由它解出。
	PublishableKey string
	// SecretKey 是 Backend API 的 key。
	SecretKey string
	JWTKey    string
	// AuthorizedParties 是 web 的 origin：token 帶 azp 時必須在裡面（web 取得的 token 才有 azp，App 的沒有）。
	AuthorizedParties []string
	// APIURL 是 Backend API 的網址；空字串時用 DefaultAPIURL（測試換成本機的 server）。
	APIURL string
	// HTTPClient 呼叫 Backend API 用；nil 時用有逾時的預設 client。
	HTTPClient *http.Client
	// Now 是目前時間；nil 時用 time.Now。
	Now func() time.Time
}

// Client 是一個 Clerk application。
type Client struct {
	issuer   string
	verifier *oidc.Verifier
	apiURL   string
	secret   string
	hc       *http.Client
}

// FrontendAPI 由 publishable key 解出 frontend API 的網址（session token 的 issuer）：key 是
// pk_<環境>_ 加上 base64("<網域>$")。
func FrontendAPI(publishableKey string) (string, error) {
	i := strings.LastIndex(publishableKey, "_")
	if !strings.HasPrefix(publishableKey, "pk_") || i < 0 {
		return "", errors.New("clerk：publishable key 的格式不對")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(publishableKey[i+1:], "="))
	if err != nil {
		return "", fmt.Errorf("clerk：解 publishable key：%w", err)
	}
	host, ok := strings.CutSuffix(string(raw), "$")
	if !ok || host == "" || strings.ContainsAny(host, "/?#@ ") {
		return "", errors.New("clerk：publishable key 的格式不對")
	}
	return "https://" + host, nil
}

// New 建立 Client。
func New(cfg Config) (*Client, error) {
	issuer, err := FrontendAPI(cfg.PublishableKey)
	if err != nil {
		return nil, err
	}
	if cfg.SecretKey == "" {
		return nil, errors.New("clerk：沒有 secret key")
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: apiTimeout}
	}
	if len(cfg.AuthorizedParties) == 0 {
		return nil, errors.New("clerk：authorized parties 必須設定")
	}
	v, err := oidc.New(oidc.Config{Issuers: []string{issuer}, PublicKeyPEM: strings.ReplaceAll(cfg.JWTKey, `\n`, "\n"), Validators: sessionValidators(cfg.AuthorizedParties), Now: cfg.Now})
	if err != nil {
		return nil, err
	}
	apiURL := cfg.APIURL
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}
	return &Client{issuer: issuer, verifier: v, apiURL: strings.TrimRight(apiURL, "/"), secret: cfg.SecretKey, hc: hc}, nil
}

// Issuer 是這個 application 簽的 token 的 issuer。
func (c *Client) Issuer() string { return c.issuer }

// OwnsUser 回傳使用者的 ID 是不是 Clerk 發的（user_ 開頭）。
func (c *Client) OwnsUser(id string) bool { return strings.HasPrefix(id, "user_") }

// Verify 驗證 session token，回傳已驗證的 session claims。
func (c *Client) Verify(ctx context.Context, token string) (oidc.Claims, error) {
	return verify(ctx, c.verifier, token)
}

func verify(ctx context.Context, v *oidc.Verifier, token string) (oidc.Claims, error) {
	claims, err := v.Verify(ctx, token)
	if errors.Is(err, oidc.ErrInvalid) {
		return oidc.Claims{}, errors.Join(err, ErrInvalid)
	}
	return claims, err
}

// BanUser 停用使用者：Clerk 撤銷他所有的 session，之後不能再登入。使用者不存在時成功。
func (c *Client) BanUser(ctx context.Context, id string) error {
	return ignoreNotFound(c.call(ctx, http.MethodPost, "/v1/users/"+url.PathEscape(id)+"/ban"))
}

// DeleteUser 刪除使用者。已經不存在時成功。
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return ignoreNotFound(c.call(ctx, http.MethodDelete, "/v1/users/"+url.PathEscape(id)))
}

func ignoreNotFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// call 呼叫 Backend API。404 回 ErrNotFound；其他 4xx（429 除外）回包著 ErrRejected 的錯誤；5xx、429 與連線錯誤
// 是可以重試的錯誤。
func (c *Client) call(ctx context.Context, method, path string) error {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	req.Header.Set("Accept", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("clerk：%s %s：%w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("clerk：%s %s：讀取回應：%w", method, path, err)
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w：%s", ErrNotFound, path)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return fmt.Errorf("clerk：%s %s：%d", method, path, resp.StatusCode)
	case resp.StatusCode >= 400:
		return fmt.Errorf("%w：%s %s：%d %s", ErrRejected, method, path, resp.StatusCode, clerkErrorCode(body))
	}
	return nil
}

// clerkErrorCode 取出 Clerk 錯誤回應的 code（例如 form_param_unknown），不帶整段 body 進 log。
func clerkErrorCode(body []byte) string {
	var e struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &e) != nil || len(e.Errors) == 0 {
		return ""
	}
	return e.Errors[0].Code
}

// 本機顧客的 token（hawkr dev-token）：使用者是 dev_ 開頭的 ID。
const (
	DevIssuer     = "hawkr-dev"
	DevUserPrefix = "dev_"
)

// Dev 驗證本機的 RSA 金鑰簽的 token，給本機開發與測試用：不連 Clerk，Backend API 的呼叫都在本機完成。
type Dev struct {
	issuer   string
	verifier *oidc.Verifier
}

// NewDev 建立 Dev；now 是目前時間，nil 時用 time.Now。
func NewDev(key *rsa.PublicKey, now func() time.Time) (*Dev, error) {
	v, err := oidc.New(oidc.Config{Issuers: []string{DevIssuer}, PublicKey: key, Validators: sessionValidators(nil), Now: now})
	if err != nil {
		return nil, err
	}
	return &Dev{issuer: DevIssuer, verifier: v}, nil
}

// Issuer 是這個 dev application 的 issuer。
func (d *Dev) Issuer() string { return d.issuer }

// OwnsUser 回傳使用者的 ID 是不是 dev 的（dev_ 開頭）。
func (d *Dev) OwnsUser(id string) bool { return strings.HasPrefix(id, DevUserPrefix) }

// Verify 驗證 dev token 與 dev_ subject。
func (d *Dev) Verify(ctx context.Context, token string) (oidc.Claims, error) {
	claims, err := verify(ctx, d.verifier, token)
	if err != nil {
		return oidc.Claims{}, err
	}
	if !d.OwnsUser(claims.Subject) {
		return oidc.Claims{}, fmt.Errorf("%w：dev token 的 sub 要以 %s 開頭", ErrInvalid, DevUserPrefix)
	}
	return claims, nil
}

// BanUser 不做任何事：dev 使用者沒有 Clerk 的 session。
func (d *Dev) BanUser(context.Context, string) error { return nil }

// DeleteUser 不做任何事：dev 使用者不在 Clerk。
func (d *Dev) DeleteUser(context.Context, string) error { return nil }

// SignDev 以本機的金鑰簽一個 dev token。
func SignDev(key *rsa.PrivateKey, issuer, userID string, ttl time.Duration, now time.Time, audience ...string) (string, error) {
	if !strings.HasPrefix(userID, DevUserPrefix) {
		return "", fmt.Errorf("clerk：dev 使用者的 ID 要以 %s 開頭", DevUserPrefix)
	}
	tok := jwt.New()
	tok.Options().Enable(jwt.FlattenAudience)
	for name, value := range map[string]any{"iss": issuer, "sub": userID, "iat": now.Unix(), "nbf": now.Add(-time.Minute).Unix(), "exp": now.Add(ttl).Unix(), "email": userID + "@dev.localhost", "name": userID} {
		if err := tok.Set(name, value); err != nil {
			return "", err
		}
	}
	if len(audience) > 0 {
		if err := tok.Set("aud", audience); err != nil {
			return "", err
		}
	}
	if issuer == DevIssuer {
		if err := tok.Set("sts", "active"); err != nil {
			return "", err
		}
	}
	headers := jws.NewHeaders()
	if err := headers.Set(jws.TypeKey, "JWT"); err != nil {
		return "", err
	}
	raw, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), key, jws.WithProtectedHeaders(headers)))
	return string(raw), err
}

func sessionValidators(parties []string) []jwt.Validator {
	return []jwt.Validator{
		jwt.ValidatorFunc(func(_ context.Context, tok jwt.Token) error {
			var azp any
			if !tok.Has("azp") {
				return nil
			}
			if err := tok.Get("azp", &azp); err != nil {
				return err
			}
			party, ok := azp.(string)
			if !ok || !slices.Contains(parties, party) {
				return errors.New("clerk：azp 不在白名單")
			}
			return nil
		}),
		jwt.ValidatorFunc(func(_ context.Context, tok jwt.Token) error {
			var sts any
			if !tok.Has("sts") {
				return nil
			}
			if err := tok.Get("sts", &sts); err != nil {
				return err
			}
			if sts != "active" {
				return errors.New("clerk：session 非 active")
			}
			return nil
		}),
	}
}
