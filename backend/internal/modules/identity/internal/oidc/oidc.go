// Package oidc 驗證各 issuer 的 JWT 簽章與 claims。
package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

const (
	GoogleIssuer         = "https://accounts.google.com"
	GoogleIssuerNoScheme = "accounts.google.com"
	GoogleJWKS           = "https://www.googleapis.com/oauth2/v3/certs"
	AppleIssuer          = "https://appleid.apple.com"
	AppleJWKS            = "https://appleid.apple.com/auth/keys"
	fetchTimeout         = 5 * time.Second
	minRefreshInterval   = time.Minute
	claimLeeway          = 5 * time.Second
)

// ErrInvalid 表示 token 無效：簽章、issuer、audience、期限或格式不對。
var ErrInvalid = errors.New("oidc：token 無效")

// Config 是一組允許的 issuer、金鑰來源與 claim policy。
type Config struct {
	Issuers      []string
	JWKSURL      string
	PublicKey    *rsa.PublicKey
	PublicKeyPEM string
	Audiences    []string
	Validators   []jwt.Validator
	HTTPClient   *http.Client
	Now          func() time.Time
	Context      context.Context
}

// Claims 是驗證過的 token 內容。
type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	// Audience 是 token 的 aud 裡被接受的那一個。
	Audience string
	// Name 是使用者的姓名（Google 的 name）；Apple 的 token 沒有。
	Name string
	// Nonce 是 token 的 nonce claim，沒有時是空字串。
	Nonce string
	// IssuedAt 是 token 的 iat，沒有時是零值。
	IssuedAt time.Time
}

// Verifier 驗證指定 issuer 集合的 token。
type Verifier struct {
	cfg         Config
	key         any
	cache       *jwk.Cache
	mu          sync.Mutex
	lastRefresh time.Time
	fetchErr    error
	refreshing  chan struct{}
}

// New 建立 Verifier；遠端公鑰在第一次驗證時載入。
func New(cfg Config) (*Verifier, error) {
	if len(cfg.Issuers) == 0 || slices.Contains(cfg.Issuers, "") || (cfg.JWKSURL == "" && cfg.PublicKey == nil && cfg.PublicKeyPEM == "") || (cfg.JWKSURL != "" && len(cfg.Audiences) == 0) {
		return nil, errors.New("oidc：issuer、公鑰與 audience 都要設定")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now //nolint:forbidigo // assigns the clock dependency without calling it
	}
	v := &Verifier{cfg: cfg}
	if cfg.PublicKey != nil {
		v.key = cfg.PublicKey
		return v, nil
	}
	if cfg.PublicKeyPEM != "" {
		key, err := jwk.ParseKey([]byte(cfg.PublicKeyPEM), jwk.WithPEM(true))
		if err != nil {
			return nil, fmt.Errorf("oidc：PEM：%w", err)
		}
		var public rsa.PublicKey
		if err := jwk.Export(key, &public); err != nil {
			return nil, fmt.Errorf("oidc：RSA 公鑰：%w", err)
		}
		v.key = &public
		return v, nil
	}
	return v, nil
}

func (v *Verifier) keys(ctx context.Context, kid string) (jwk.Set, error) {
	v.mu.Lock()
	cache := v.cache
	v.mu.Unlock()
	var keys jwk.Set
	var err error
	if cache != nil && cache.IsRegistered(ctx, v.cfg.JWKSURL) {
		keys, err = cache.Lookup(ctx, v.cfg.JWKSURL)
		if err == nil {
			if _, ok := keys.LookupKeyID(kid); ok {
				return keys, nil
			}
		}
	}
	v.mu.Lock()
	if v.refreshing != nil {
		wait := v.refreshing
		v.mu.Unlock()
		select {
		case <-wait:
			return v.keys(ctx, kid)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if v.cache != nil && v.cfg.Now().Sub(v.lastRefresh) < minRefreshInterval {
		fetchErr := v.fetchErr
		v.mu.Unlock()
		return keys, fetchErr
	}
	if v.cache == nil {
		lifetime := v.cfg.Context
		if lifetime == nil {
			lifetime = context.Background()
		}
		cache, err := jwk.NewCache(lifetime, httprc.NewClient())
		if err != nil {
			v.mu.Unlock()
			return nil, err
		}
		v.cache = cache
	}
	v.lastRefresh = v.cfg.Now()
	cache = v.cache
	v.refreshing = make(chan struct{})
	wait := v.refreshing
	v.mu.Unlock()
	//nolint:gosec // The shared refresh uses the verifier lifetime, not a caller's context.
	go v.refresh(cache)
	select {
	case <-wait:
		return v.keys(ctx, kid)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (v *Verifier) refresh(cache *jwk.Cache) {
	hc := v.cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: fetchTimeout}
	}
	lifetime := v.cfg.Context
	if lifetime == nil {
		lifetime = context.Background()
	}
	fetchCtx, cancel := context.WithTimeout(lifetime, fetchTimeout)
	defer cancel()
	var err error
	if !cache.IsRegistered(fetchCtx, v.cfg.JWKSURL) {
		err = cache.Register(fetchCtx, v.cfg.JWKSURL, jwk.WithHTTPClient(hc), jwk.WithMinInterval(minRefreshInterval))
		if err == nil {
			_, err = cache.Lookup(fetchCtx, v.cfg.JWKSURL)
		}
	} else {
		_, err = cache.Refresh(fetchCtx, v.cfg.JWKSURL)
	}
	v.mu.Lock()
	v.fetchErr = err
	close(v.refreshing)
	v.refreshing = nil
	v.mu.Unlock()
}

// Verify 驗證 token；ErrInvalid 是憑證錯誤，其他錯誤表示公鑰服務無法使用。
func (v *Verifier) Verify(ctx context.Context, raw string) (Claims, error) {
	header, _, ok := strings.Cut(raw, ".")
	if !ok {
		return Claims{}, ErrInvalid
	}
	encoded, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return Claims{}, apperr.WithCause(ErrInvalid, err)
	}
	var h struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}
	if err := json.Unmarshal(encoded, &h); err != nil || h.Algorithm != "RS256" {
		return Claims{}, ErrInvalid
	}
	options := []jwt.ParseOption{jwt.WithValidate(true), jwt.WithRequiredClaim(jwt.ExpirationKey), jwt.WithRequiredClaim(jwt.SubjectKey), jwt.WithClock(jwt.ClockFunc(v.cfg.Now)), jwt.WithAcceptableSkew(claimLeeway)}
	if v.key == nil {
		if h.KeyID == "" {
			return Claims{}, ErrInvalid
		}
		keys, err := v.keys(ctx, h.KeyID)
		if err != nil {
			return Claims{}, fmt.Errorf("oidc：抓公鑰：%w", err)
		}
		if keys == nil {
			return Claims{}, ErrInvalid
		}
		key, ok := keys.LookupKeyID(h.KeyID)
		if !ok {
			return Claims{}, ErrInvalid
		}
		if alg, has := key.Algorithm(); has && alg != jwa.RS256() {
			return Claims{}, ErrInvalid
		}
		if usage, has := key.KeyUsage(); has && usage != "sig" {
			return Claims{}, ErrInvalid
		}
		var public rsa.PublicKey
		if err := jwk.Export(key, &public); err != nil {
			return Claims{}, ErrInvalid
		}
		selected, err := key.Clone()
		if err != nil {
			return Claims{}, ErrInvalid
		}
		if err := selected.Set(jwk.AlgorithmKey, jwa.RS256()); err != nil {
			return Claims{}, ErrInvalid
		}
		set := jwk.NewSet()
		if err := set.AddKey(selected); err != nil {
			return Claims{}, ErrInvalid
		}
		options = append(options, jwt.WithKeySet(set))
	} else {
		options = append(options, jwt.WithKey(jwa.RS256(), v.key))
	}
	options = append(options, jwt.WithValidator(jwt.ValidatorFunc(func(_ context.Context, tok jwt.Token) error {
		issuer, _ := tok.Issuer()
		if !slices.Contains(v.cfg.Issuers, issuer) {
			return ErrInvalid
		}
		return nil
	})))
	for _, validator := range v.cfg.Validators {
		options = append(options, jwt.WithValidator(validator))
	}
	if len(v.cfg.Audiences) > 0 {
		options = append(options, jwt.WithValidator(jwt.ValidatorFunc(func(_ context.Context, tok jwt.Token) error {
			audiences, _ := tok.Audience()
			for _, audience := range audiences {
				if slices.Contains(v.cfg.Audiences, audience) {
					return nil
				}
			}
			return ErrInvalid
		})))
	}
	tok, err := jwt.Parse([]byte(raw), options...)
	if err != nil {
		return Claims{}, apperr.WithCause(ErrInvalid, err)
	}
	sub, _ := tok.Subject()
	if sub == "" {
		return Claims{}, ErrInvalid
	}
	audience := ""
	if len(v.cfg.Audiences) > 0 {
		audiences, _ := tok.Audience()
		i := slices.IndexFunc(audiences, func(a string) bool { return slices.Contains(v.cfg.Audiences, a) })
		audience = audiences[i]
	}
	var email, name string
	if tok.Has("email") {
		if err := tok.Get("email", &email); err != nil {
			return Claims{}, apperr.WithCause(ErrInvalid, err)
		}
	}
	if tok.Has("name") {
		if err := tok.Get("name", &name); err != nil {
			return Claims{}, apperr.WithCause(ErrInvalid, err)
		}
	}
	var nonce string
	if tok.Has("nonce") {
		if err := tok.Get("nonce", &nonce); err != nil {
			return Claims{}, apperr.WithCause(ErrInvalid, err)
		}
	}
	issuedAt, _ := tok.IssuedAt()
	var verified any
	_ = tok.Get("email_verified", &verified)
	return Claims{Subject: sub, Email: email, EmailVerified: verified == true || verified == "true", Audience: audience, Name: name, Nonce: nonce, IssuedAt: issuedAt}, nil
}
