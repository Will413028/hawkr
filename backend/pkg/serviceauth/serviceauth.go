// Package serviceauth provides short-lived, purpose-bound credentials for
// authenticated service-to-service calls. It is deliberately separate from
// end-user JWTs and from the existing internal HMAC purpose.
package serviceauth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	DefaultTTL      = 5 * time.Minute
	MaximumTTL      = 15 * time.Minute
	MinimumKeyBytes = 32

	// ChatStreamMethod is the only method authenticated by the first consumer
	// of this package. Keeping the exact method name here prevents Gateway and
	// IM from independently spelling the auth boundary.
	ChatStreamMethod = "/hawkr.im.IMService/ChatStream"
)

var (
	ErrMissingCurrentKey = errors.New("serviceauth: current key is required")
	ErrWeakKey           = errors.New("serviceauth: key is shorter than 32 bytes")
)

// Config controls one purpose-bound service credential. Issuer, Audience and
// Purpose are required rather than defaulted so a new caller cannot silently
// authenticate against the wrong service or purpose.
type Config struct {
	CurrentKey  string
	PreviousKey string
	Issuer      string
	Audience    string
	Purpose     string
	TTL         time.Duration
	Now         func() time.Time
}

func (c Config) normalized() (Config, error) {
	c.CurrentKey = strings.TrimSpace(c.CurrentKey)
	c.PreviousKey = strings.TrimSpace(c.PreviousKey)
	if c.CurrentKey == "" {
		return Config{}, ErrMissingCurrentKey
	}
	if len(c.CurrentKey) < MinimumKeyBytes {
		return Config{}, fmt.Errorf("%w: current key has %d bytes", ErrWeakKey, len(c.CurrentKey))
	}
	if c.PreviousKey != "" && len(c.PreviousKey) < MinimumKeyBytes {
		return Config{}, fmt.Errorf("%w: previous key has %d bytes", ErrWeakKey, len(c.PreviousKey))
	}
	if strings.TrimSpace(c.Issuer) == "" {
		return Config{}, errors.New("serviceauth: issuer is required")
	}
	if strings.TrimSpace(c.Audience) == "" {
		return Config{}, errors.New("serviceauth: audience is required")
	}
	if strings.TrimSpace(c.Purpose) == "" {
		return Config{}, errors.New("serviceauth: purpose is required")
	}
	if c.TTL == 0 {
		c.TTL = DefaultTTL
	}
	if c.TTL <= 0 || c.TTL > MaximumTTL {
		return Config{}, fmt.Errorf("serviceauth: ttl must be >0 and <= %s", MaximumTTL)
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c, nil
}

// Claims are the claims accepted for a service credential.
type Claims struct {
	Purpose string `json:"purpose"`
	jwt.RegisteredClaims
}

// Signer issues credentials with the current key.
type Signer struct {
	key      []byte
	issuer   string
	audience string
	purpose  string
	ttl      time.Duration
	now      func() time.Time
}

// NewSigner validates and constructs a service credential signer.
func NewSigner(cfg Config) (*Signer, error) {
	cfg, err := cfg.normalized()
	if err != nil {
		return nil, err
	}
	return &Signer{
		key:      []byte(cfg.CurrentKey),
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		purpose:  cfg.Purpose,
		ttl:      cfg.TTL,
		now:      cfg.Now,
	}, nil
}

// Sign creates a five-minute-or-less HS256 service JWT with a unique jti.
func (s *Signer) Sign() (string, error) {
	if s == nil || len(s.key) == 0 {
		return "", ErrMissingCurrentKey
	}
	now := s.now().UTC()
	claims := Claims{
		Purpose: s.purpose,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Audience:  jwt.ClaimStrings{s.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.key)
}

// Verifier validates current credentials and, during rotation, credentials
// signed with the previous key. The previous key is accepted only for the
// same issuer/audience/purpose and normal JWT time validation.
type Verifier struct {
	keys     [][]byte
	issuer   string
	audience string
	purpose  string
	now      func() time.Time
}

// NewVerifier validates and constructs a service credential verifier.
func NewVerifier(cfg Config) (*Verifier, error) {
	cfg, err := cfg.normalized()
	if err != nil {
		return nil, err
	}
	keys := [][]byte{[]byte(cfg.CurrentKey)}
	if cfg.PreviousKey != "" && cfg.PreviousKey != cfg.CurrentKey {
		keys = append(keys, []byte(cfg.PreviousKey))
	}
	return &Verifier{
		keys:     keys,
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		purpose:  cfg.Purpose,
		now:      cfg.Now,
	}, nil
}

// Verify validates a raw service JWT and returns its claims. Callers should
// expose only a generic authentication failure to remote peers; the detailed
// error is for local tests/logging and is not a wire contract.
func (v *Verifier) Verify(raw string) (*Claims, error) {
	if v == nil || len(v.keys) == 0 {
		return nil, ErrMissingCurrentKey
	}
	var lastErr error
	for _, key := range v.keys {
		claims := &Claims{}
		parser := jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(v.issuer),
			jwt.WithAudience(v.audience),
			jwt.WithIssuedAt(),
			jwt.WithExpirationRequired(),
			jwt.WithTimeFunc(v.now),
		)
		token, err := parser.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("serviceauth: unexpected signing method")
			}
			return key, nil
		})
		if err != nil {
			lastErr = err
			continue
		}
		if !token.Valid {
			lastErr = errors.New("serviceauth: token is not valid")
			continue
		}
		if claims.Purpose != v.purpose {
			lastErr = errors.New("serviceauth: wrong purpose")
			continue
		}
		if strings.TrimSpace(claims.ID) == "" {
			lastErr = errors.New("serviceauth: jti is required")
			continue
		}
		return claims, nil
	}
	if lastErr == nil {
		lastErr = errors.New("serviceauth: token rejected")
	}
	return nil, lastErr
}
