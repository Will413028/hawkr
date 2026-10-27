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

// originAuthHeader carries a self-issued claim about which subject a request
// represents. Only our own server-side callers send it.
const originAuthHeader = "X-HK-Origin-Auth"

// originAuthDomain binds the MAC to this exact purpose so a signature can never
// be replayed against another HMAC verifier sharing a secret. Bump the suffix on
// any payload-shape change to invalidate old signatures at the verifier.
const originAuthDomain = "hkorigin1:"

// originAuthVersion is the payload schema version carried in "v".
const originAuthVersion = 1

// originAuthSkew is how far the signed timestamp may be from now, either way.
// Same order of magnitude as the quota window: long enough for clock drift,
// short enough that a captured signature is worth little.
const originAuthSkew = 60 * time.Second

// origin-auth verification outcomes; these are the "result" metric label values.
const (
	originAuthResultOK        = "ok"
	originAuthResultAbsent    = "absent"    // no header, or verification disabled for that class
	originAuthResultBadSig    = "bad_sig"   // MAC mismatch, or signed method/path is not this request
	originAuthResultExpired   = "expired"   // outside originAuthSkew
	originAuthResultMalformed = "malformed" // undecodable / wrong version / unknown p / bad visitor ip
)

// originAuthPayload is the signed claim. Short field names keep the header
// small on a header sent on every SSR subrequest.
type originAuthPayload struct {
	V  int    `json:"v"`
	P  string `json:"p"`            // "visitor" | "ssr" | "ci"
	IP string `json:"ip,omitempty"` // visitor IP, only meaningful when p=visitor
	TS int64  `json:"ts"`           // unix seconds
	M  string `json:"m"`            // HTTP method this signature is valid for
	U  string `json:"u"`            // request path this signature is valid for
}

// originAuthKeys holds the two independent verification keys.
//
// They are two switches, not one. ORIGIN_AUTH_SECRET (+ _PREV for rotation) is
// held by the website and admin workers; ORIGIN_AUTH_CI_SECRET by GitHub
// Actions. Holders with different exposure must not share a key: whoever holds
// the CI secret would otherwise be able to sign p=visitor with any victim's IP
// and lock that person out of login (the auth-failure axis deliberately keeps
// SubjectVisitor in scope) — while metering as result="ok", so no alert fires.
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
//   - Not "fail to boot": k8s/scripts/create-secrets.sh is not executed by any
//     workflow (static-gates.yml only gates its SCRIPT_REVISION); the
//     secrets are created by hand, once per environment. Making them a startup
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
		// WithOptions(zap.AddStacktrace(FatalLevel)) suppresses the stack trace
		// zap attaches to Error by default. A missing secret is a configuration
		// state, not a code path worth tracing — the call site is always this
		// function, and the untrimmed trace buries the message it belongs to.
		l.WithOptions(zap.AddStacktrace(zapcore.FatalLevel)).
			Error("origin-auth verification DISABLED: secret not set — traffic in this class falls back to per-ClientIP quotas",
				zap.String("key", name))
	}
}

// rateLimitIdentity is the resolved answer for one request. clientIP is carried
// alongside because the block log needs BOTH: subject answers "whose quota did
// this spend", clientIP answers "which machine sent it", and under SSR / CI /
// admin those are different answers.
type rateLimitIdentity struct {
	subject  string
	kind     SubjectKind
	clientIP string
}

// rateLimitIdentityKey is the gin-context memo key. Resolution runs once per
// request: it is the only place gateway_origin_auth_total is incremented, and
// several middlewares plus two handlers ask for the answer.
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

// forensicClientIP is the real peer, for logging only — never for a quota key.
// It exists so that "who actually sent this" is resolved in this file too, and
// no other file needs to call c.ClientIP().
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
		// fail-open: a rotation window must not take the website down, and the
		// degraded behaviour is exactly the pre-#2264 behaviour. Alert B watches
		// the bad_sig / expired / malformed ratio so a silently broken signing
		// chain does not just quietly collapse everyone onto one key again.
		return fallback
	}
	return rateLimitIdentity{subject: subject, kind: kind, clientIP: clientIP}
}

// visitorSubject namespaces a human under "v:".
//
// The prefix and the parse are both load-bearing. gin does not validate
// CF-Connecting-IP, so without a prefix anyone able to reach the origin
// directly could send "CF-Connecting-IP: s:ssr" and charge itself to the SSR
// axis, which carries 20x the visitor quota. With it, an unsigned request can
// only ever land inside the "v:" namespace.
func visitorSubject(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "v:invalid"
	}
	return "v:" + parsed.String()
}

// verifyOriginAuth checks one signature and reports the subject it proves.
// The returned subject is meaningful only when result == originAuthResultOK.
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

	// Reading p BEFORE verifying is safe: it only selects which key to check
	// against and grants nothing. Forging p=ci to reach the CI key still has to
	// produce a MAC under that key.
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
		// This class's key is unset ⇒ verification disabled ⇒ nothing was
		// evaluated. Reported as "absent" so it stays out of alert B's ratio
		// (whose denominator excludes absent): while a key is off, "signatures
		// do not verify" is expected and is alert A's job, not B's.
		return "", SubjectVisitor, originAuthResultAbsent
	}

	gotMAC, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return "", SubjectVisitor, originAuthResultMalformed
	}
	if !anyMACMatches(candidates, body, gotMAC) {
		return "", SubjectVisitor, originAuthResultBadSig
	}
	// Bound to this exact request so a captured signature cannot be pointed at
	// another endpoint within its validity window.
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
		// p=visitor without a parseable IP proves nothing about who to charge;
		// treating it as the caller's own address would let a worker launder any
		// junk value into a quota key.
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

// anyMACMatches compares in constant time against every accepted key so a
// secret rotation has an overlap window instead of a cutover.
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
