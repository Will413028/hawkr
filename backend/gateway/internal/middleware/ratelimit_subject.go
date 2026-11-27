package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// SubjectKind classifies WHO a request is charged to, which is a different
// question from what its quota is. Before this existed, every per-IP mechanism
// (tier quotas, the auth-failure axis, SSE connection caps) called
// c.ClientIP() itself, so "one address = one human" was assumed separately each
// time — and it is false for our own server-side callers: the website's SSR
// worker, the admin BFF and the CI runner each present ONE address on behalf of
// an unbounded number of people (#2264).
type SubjectKind int

const (
	SubjectVisitor SubjectKind = iota // one human (browser direct, App, or a worker acting for them)
	SubjectSSR                        // website server-side render, ISR regenerate, build-time prefetch
	SubjectCI                         // CI runner / E2E / smoke
)

// String is the metric label form. Kept to three stable values — never derive a
// label from the subject itself, whose cardinality is unbounded.
func (k SubjectKind) String() string {
	switch k {
	case SubjectSSR:
		return "ssr"
	case SubjectCI:
		return "ci"
	default:
		return "visitor"
	}
}

const originAuthHeader = "X-HK-Origin-Auth"

const originAuthDomain = "hkorigin1:"

const originAuthVersion = 1

const originAuthSkew = 60 * time.Second

const (
	originAuthResultOK        = "ok"
	originAuthResultAbsent    = "absent"
	originAuthResultBadSig    = "bad_sig"
	originAuthResultExpired   = "expired"
	originAuthResultMalformed = "malformed"
)

type originAuthPayload struct {
	V  int    `json:"v"`
	P  string `json:"p"`
	IP string `json:"ip,omitempty"`
	TS int64  `json:"ts"`
	M  string `json:"m"`
	U  string `json:"u"`
}

type originAuthKeys struct {
	main []byte
	prev []byte
	ci   []byte
}

var originAuthKeyHolder atomic.Pointer[originAuthKeys]

// SetOriginAuthSecrets installs the verification keys at boot (di.go) and
// publishes whether each one is active.
//
// An EMPTY secret DISABLES verification for the classes it covers; it is
// neither a boot failure nor a verification against the empty key.
//
//   - Not "verify anyway": HMAC("", ·) is computable by anyone, so every claim
//     would verify — any caller could charge itself to any victim's subject.
//   - Not "fail to boot": the secrets
//     are created by hand, once per environment. Making them a startup
//     requirement means a CrashLoop the first time each environment deploys —
//     including once on prod, triggered by whoever runs the routine dev → main.
//
// Disabled means the affected requests fall back to v:<ClientIP>, i.e. exactly
// the behaviour before this design, and the gauge says so out loud.
func SetOriginAuthSecrets(secret, prevSecret, ciSecret string, l *zap.Logger) {
	originAuthKeyHolder.Store(&originAuthKeys{
		main: []byte(secret),
		prev: []byte(prevSecret),
		ci:   []byte(ciSecret),
	})
	announceOriginAuthKey("main", secret != "", l)
	announceOriginAuthKey("ci", ciSecret != "", l)
}

func announceOriginAuthKey(name string, enabled bool, l *zap.Logger) {
	if enabled {
		originAuthEnabled.WithLabelValues(name).Set(1)
		return
	}
	originAuthEnabled.WithLabelValues(name).Set(0)
	if l != nil {
		l.WithOptions(zap.AddStacktrace(zapcore.FatalLevel)).
			Error("origin-auth verification DISABLED: secret not set — traffic in this class falls back to per-ClientIP quotas",
				zap.String("key", name))
	}
}

type rateLimitIdentity struct {
	subject  string
	kind     SubjectKind
	clientIP string
}

const rateLimitIdentityKey = "hk_ratelimit_identity"

// RateLimitSubject answers "which quota does this request spend, and of what
// kind". It is the single question point: every per-IP quota, block and
// connection cap goes through it (enforced by the banClientIPForQuota ruleguard
// rule).
//
// Exported because one of its call sites — the SSE stream handler — lives in
// package handler. It is also a plain function rather than
// something the limiter middleware hands down, because SSE is tierExempt: the
// limiter early-returns for it and can never pass anything to the SSE cap.
func RateLimitSubject(c *gin.Context) (subject string, kind SubjectKind) {
	id := rateLimitIdentityOf(c)
	return id.subject, id.kind
}

func rateLimitIdentityOf(c *gin.Context) rateLimitIdentity {
	if v, ok := c.Get(rateLimitIdentityKey); ok {
		if id, ok := v.(rateLimitIdentity); ok {
			return id
		}
	}
	id := resolveRateLimitIdentity(c)
	c.Set(rateLimitIdentityKey, id)
	return id
}

func forensicClientIP(c *gin.Context) string { return rateLimitIdentityOf(c).clientIP }

func resolveRateLimitIdentity(c *gin.Context) rateLimitIdentity {
	clientIP := c.ClientIP()
	if principal, err := identityFromHeaders(c.Request.Header); err == nil {
		prefix := "a:"
		if principal.AccountType == AccountTypeAdmin {
			prefix = "m:"
		}
		return rateLimitIdentity{subject: prefix + strconv.FormatInt(principal.AccountID, 10), kind: SubjectVisitor, clientIP: clientIP}
	}
	fallback := rateLimitIdentity{
		subject:  visitorSubject(clientIP),
		kind:     SubjectVisitor,
		clientIP: clientIP,
	}

	raw := c.GetHeader(originAuthHeader)
	if raw == "" {
		originAuthTotal.WithLabelValues(originAuthResultAbsent).Inc()
		return fallback
	}

	subject, kind, result := verifyOriginAuth(c, raw)
	originAuthTotal.WithLabelValues(result).Inc()
	if result != originAuthResultOK {
		return fallback
	}
	return rateLimitIdentity{subject: subject, kind: kind, clientIP: clientIP}
}

func visitorSubject(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "v:invalid"
	}
	return "v:" + parsed.String()
}

func verifyOriginAuth(c *gin.Context, raw string) (string, SubjectKind, string) {
	body, sig, found := strings.Cut(raw, ".")
	if !found || body == "" || sig == "" {
		return "", SubjectVisitor, originAuthResultMalformed
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", SubjectVisitor, originAuthResultMalformed
	}
	var p originAuthPayload
	if err := json.Unmarshal(payloadBytes, &p); err != nil || p.V != originAuthVersion {
		return "", SubjectVisitor, originAuthResultMalformed
	}

	keys := originAuthKeyHolder.Load()
	if keys == nil {
		keys = &originAuthKeys{}
	}
	var kind SubjectKind
	var candidates [][]byte
	switch p.P {
	case "visitor":
		kind, candidates = SubjectVisitor, nonEmptyKeys(keys.main, keys.prev)
	case "ssr":
		kind, candidates = SubjectSSR, nonEmptyKeys(keys.main, keys.prev)
	case "ci":
		kind, candidates = SubjectCI, nonEmptyKeys(keys.ci)
	default:
		return "", SubjectVisitor, originAuthResultMalformed
	}
	if len(candidates) == 0 {
		return "", SubjectVisitor, originAuthResultAbsent
	}

	gotMAC, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return "", SubjectVisitor, originAuthResultMalformed
	}
	if !anyMACMatches(candidates, body, gotMAC) {
		return "", SubjectVisitor, originAuthResultBadSig
	}
	if p.M != c.Request.Method || p.U != c.Request.URL.Path {
		return "", SubjectVisitor, originAuthResultBadSig
	}
	if delta := time.Since(time.Unix(p.TS, 0)); delta > originAuthSkew || delta < -originAuthSkew {
		return "", SubjectVisitor, originAuthResultExpired
	}

	switch kind {
	case SubjectSSR:
		return "s:ssr", SubjectSSR, originAuthResultOK
	case SubjectCI:
		return "s:ci", SubjectCI, originAuthResultOK
	default:
		ip := net.ParseIP(p.IP)
		if ip == nil {
			return "", SubjectVisitor, originAuthResultMalformed
		}
		return "v:" + ip.String(), SubjectVisitor, originAuthResultOK
	}
}

func nonEmptyKeys(keys ...[]byte) [][]byte {
	out := make([][]byte, 0, len(keys))
	for _, k := range keys {
		if len(k) > 0 {
			out = append(out, k)
		}
	}
	return out
}

func anyMACMatches(keys [][]byte, body string, got []byte) bool {
	ok := false
	for _, k := range keys {
		mac := hmac.New(sha256.New, k)
		mac.Write([]byte(originAuthDomain))
		mac.Write([]byte(body))
		if hmac.Equal(got, mac.Sum(nil)) {
			ok = true
		}
	}
	return ok
}

// SignOriginAuth mints a signature for tests and for local tooling that needs to
// exercise a non-visitor axis. Production signers live in the website / admin
// workers and in CI, each holding only the key for the classes it may claim.
func SignOriginAuth(secret []byte, kind SubjectKind, visitorIP, method, path string, at time.Time) string {
	payload, err := json.Marshal(originAuthPayload{
		V: originAuthVersion, P: kind.String(), IP: visitorIP,
		TS: at.Unix(), M: method, U: path,
	})
	if err != nil {
		return ""
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(originAuthDomain))
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
