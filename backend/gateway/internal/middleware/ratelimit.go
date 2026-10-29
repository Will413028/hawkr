package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/ratelimit"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// tier is a rate-limit sensitivity class. Endpoints map to a tier by their
// matched route (c.FullPath), classified in one place (classifyTier) so the
// exempt set — SSE/WS streams, the view beacon, health and PayUni callbacks —
// lives in exactly one auditable location. Missing one exempt route would let a
// reconnect storm on a long-lived stream trip the limiter and knock every
// client off; scattering the rule across dozens of route files makes that
// omission far easier. #1416.
type tier int

const (
	tierExempt   tier = iota // not rate limited (long-lived streams / callbacks)
	tierModerate             // anonymous public reads
	tierDefault              // authenticated app + admin traffic
)

var tierNames = map[tier]string{
	tierModerate: "moderate",
	tierDefault:  "default",
}

// tierLabel names every tier including the exempt one, for metrics. tierNames
// deliberately has no exempt entry — it builds quota keys, and exempt routes
// never reach one.
func tierLabel(t tier) string {
	if t == tierExempt {
		return "exempt"
	}
	return tierNames[t]
}

// RateLimitPolicy carries the per-tier quotas (requests per Window) and the
// fallback sizing. Values come from config (env-tunable); see config.Config.
type RateLimitPolicy struct {
	ModeratePerWindow int
	// ModerateSSRPerWindow / ModerateCIPerWindow are the moderate tier's quotas
	// for our own server-side callers. Only this tier splits by kind: it is the
	// anonymous-public-read tier, and 7 days of measurement found no CI traffic
	// in any other tier at all. Inventing values for axes with zero producers
	// would put unmeasured numbers in production dressed as measured ones.
	ModerateSSRPerWindow int
	ModerateCIPerWindow  int
	DefaultPerWindow     int
	// AuthFailurePerWindow caps how many 401 responses one IP may collect per
	// Window before its further AUTHENTICATED requests are refused (see
	// authFailureAxisApplies for the scope). This is a separate axis from the
	// per-tier quotas: a client stuck in a failure loop stays well inside its
	// tier's quota and is never throttled by it. 0 disables.
	AuthFailurePerWindow int
	Window               time.Duration
	FallbackCapacity     int
}

// limitFor resolves the quota for one (tier, kind) pair. Only tierModerate
// distinguishes kinds; every other tier falls to its single value.
//
// A non-positive SSR/CI quota falls back to the visitor quota rather than
// through to the limiter: pkg/ratelimit treats limit <= 0 as UNLIMITED, so a
// field left unset in some RateLimitPolicy literal would not fail to compile,
// would not fail a test, and would silently switch that axis off. Falling back
// keeps the pre-#2264 behaviour; NewRateLimiter logs which field was missing.
func (p RateLimitPolicy) limitFor(t tier, k SubjectKind) int {
	switch t {
	case tierModerate:
		switch k {
		case SubjectSSR:
			if p.ModerateSSRPerWindow > 0 {
				return p.ModerateSSRPerWindow
			}
		case SubjectCI:
			if p.ModerateCIPerWindow > 0 {
				return p.ModerateCIPerWindow
			}
		}
		return p.ModeratePerWindow
	default:
		return p.DefaultPerWindow
	}
}

const publicNamespace = "/api/v1/public"

// exemptExact are routes never rate limited, matched verbatim against gin's
// FullPath (route params kept as ":id"). Long-lived SSE/WS connections and
// low-JWT PayUni callbacks would be wrongly throttled — reconnect storms or
// same-source-IP callback bursts — under a per-IP request limit.
var exemptExact = map[string]struct{}{
	"/api/v1/health":                           {}, // liveness probe
	"/api/v1/app/chat/ws":                      {}, // IM WebSocket upgrade
	"/api/v1/app/notifications/stream":         {}, // notification SSE
	"/api/v1/public/posts/:id/comments/stream": {}, // post comments SSE
	"/webhook/payuni/notify":                   {}, // PayUni server-to-server callback
	"/payment/return":                          {}, // PayUni return (GET + POST)
	"/payment/redirect":                        {}, // WebView payment-page opener
	"/store_map/return":                        {}, // PayUni CVS map return
	"/store_map/redirect":                      {}, // WebView CVS-map opener
	"/.well-known/apple-developer-merchantid-domain-association.txt": {},
}

// exemptPrefixes are route trees never rate limited.
//
// ⚠️ The three AI groups — /app/sellers/me/assistant/, /app/buyers/me/assistant/
// and /app/buyers/me/procurement/ — used to be here and are deliberately GONE
// (#2505). They are now ordinary tierDefault routes, classified one route at a
// time by classifyTier's fallthrough and pinned by
// TestAIRoutesAreNotRateLimitExempt.
//
// Why they were here: "a prefix keeps every stream covered without enumerating
// each one (and missing a new one)". The cost of that convenience was never
// paid by the streams — it was paid by everything else under the same prefix.
// Of the 31 routes under these three trees only 13 are actual SSE streams; the
// other 18 are short-lived unary/JSON/CRUD (the synchronous /turns, product
// autofill, session lookup, snapshot, candidate-search create + poll, selection,
// answer, rewind, crop disambiguation, three quote-batch endpoints, four
// conversation endpoints), and a prefix rule handed every one of them an
// unlimited per-subject budget.
//
// Why the exemption rationale does not transfer: exemptExact exists for
// connections that RECONNECT — a notification SSE or the IM WebSocket drops and
// the client immediately dials again, so a per-request counter sees a storm
// where there is one logical session. No AI consumer does that. All three
// clients issue exactly one request per user turn and never redial it: the
// website is a one-shot fetch with no EventSource and no Last-Event-ID
// (lib/ai-chat/stream-client.ts), and both Flutter apps are a single
// dio.post(responseType: ResponseType.stream) whose non-200 branch surfaces an
// error and closes the controller, with no retry interceptor registered
// anywhere (ai_assistant_repository.dart in each app). One turn is one request,
// so a request counter models these correctly.
//
// The precedent is already written down: the app-version check endpoint is
// not exempt either (/public/app-version-gate) on the grounds that 「豁免等於開一個匿名可無限打 DB
// 的端點——用一個更大的洞換一個較小的洞」. Same trade, larger stakes here, since
// the endpoint behind it bills a paid model rather than a DB read.
//
// ⚠️ What this does NOT buy — do not read tierDefault as cost control. It is a
// per-subject REQUEST counter, and the subject is an address (RateLimitSubject),
// not an account. It bounds turn RATE; it does not bound concurrent streams,
// model tokens, tool calls or spend, and a subject may hold many 120s streams
// while spending very few requests. Those axes are #2505's remaining acceptance
// criteria and are owned elsewhere: the per-account meter is 57 號 §7 (100
// answers/account/month, keyed on quota_account_id, deducted in the same
// transaction as the durable turn result), and the per-turn ceiling that DOES
// exist today is proxy.streamTurnDeadline (120s, propagated to ai-service over
// the gRPC deadline).
//
// No new quota number is introduced here on purpose. 01 號 §3.2.6 forbids
// shipping an invented ceiling (「不得先猜一個數字上線」); these routes join the
// existing, measured tierDefault bucket instead of a hand-picked AI tier.
var exemptPrefixes = []string{
	"/.well-known/",
}

// classifyTier maps a matched route (gin FullPath) to its rate-limit tier.
// Order matters: the exempt rules win over the public namespace, and anything
// unmatched — including 404s where FullPath is "" — falls to the default bucket
// so it is still limited.
func classifyTier(fullPath string) tier {
	if _, ok := exemptExact[fullPath]; ok {
		return tierExempt
	}
	for _, p := range exemptPrefixes {
		if strings.HasPrefix(fullPath, p) {
			return tierExempt
		}
	}
	if strings.HasPrefix(fullPath, publicNamespace+"/") {
		return tierModerate
	}
	return tierDefault
}

// authFailureKey is the per-subject counter of 401 responses. One counter per
// source rather than per route: the abuse being throttled is "this source
// cannot authenticate and keeps trying", which is a property of the source, not
// of any one endpoint. Its blast radius is bounded by authFailureAxisApplies,
// not by the key.
func authFailureKey(subject string) string { return "ratelimit:authfail:" + subject }

// authFailureAxisApplies reports whether the auth-failure axis both observes
// (counts 401s) and acts on (refuses) this tier.
//
// tierModerate — the anonymous public reads under /api/v1/public — is out of
// scope in BOTH directions, and the two halves must stay in step so the axis
// can only ever spend a budget it also feeds.
//
// Not acted on: the axis exists to stop a source that keeps failing to
// authenticate. Refusing that source's anonymous reads prevents no further
// authentication failure — it only escalates "cannot log in" into "cannot use
// the site at all", for everyone sharing the address. Per-IP is the only
// granularity available, so one broken client behind a NAT, a shared office
// egress or a CI runner takes the whole address down with it. On 2026-07-29
// exactly that happened: the website E2E suite (single runner IP, and by design
// full of deliberate-401 assertions) tripped the threshold and was then refused
// /public/marketplace, /public/products and /public/posts, failing every ISR
// revalidate and blocking all web deploys. Abuse of the public reads themselves
// is already covered by the moderate tier quota, which is what that quota is
// for.
//
// Not counted: a route that requires no credentials cannot produce a failed
// authentication attempt. A 401 surfacing there comes from downstream, so
// counting it would let one misbehaving upstream silently spend an entire
// address's authenticated budget.
//
// The OptionalJWT routes' app counterparts stay in scope: those requests do
// attempt authentication. The OptionalJWT public routes themselves
// (/public/carts, /public/products…) are deliberately out:
// they never reject a caller for lacking an identity — the request proceeds as
// anonymous — so there is no failed authentication to throttle and no
// brute-force payoff to deny.
//
// 認證在單體之後，gateway 的 401 來自「單體沒有帶身分 header」：呼叫者沒有登入或 session 已失效。
//
// SubjectKind adds a second exclusion, on top of the tier one above.
// SubjectSSR and SubjectCI are out: SSR is anonymous reading and produces no
// failed authentication attempt, and CI asserts on deliberate 401s by design —
// counting either only lets one shared axis take the blame for everyone.
// SubjectVisitor stays IN, and must: it covers every human (browser direct, App
// and worker-relayed alike), so excluding it would retire the whole defence
// that the 2026-07-29 incident is the reason for.
func authFailureAxisApplies(t tier, k SubjectKind) bool {
	switch k {
	case SubjectSSR, SubjectCI:
		return false
	}
	switch t {
	case tierExempt, tierModerate:
		return false
	default:
		return true
	}
}

// blockLogWindow is the block-log sampling window: at most one line per
// (reason, IP) within it. Deliberately its own constant rather than the tunable
// quota window — log volume is a property of the log pipeline, not of a quota
// operators may retune. It matches the default RATE_LIMIT_WINDOW_SECONDS=60.
const blockLogWindow = time.Minute

// blockLogMaxLinesPerWindow hard-caps the lines one window may emit no matter
// how many distinct sources are blocked. A thousand samples already
// characterises any incident, and past that a single "dropped" line carries the
// true scale.
const blockLogMaxLinesPerWindow = 1000

// blockLogXFFMaxLen bounds the logged X-Forwarded-For. The header is entirely
// caller-controlled and the origin-direct path sits outside Cloudflare's header
// limits, so unbounded it lets one request write up to net/http's default 1MB
// MaxHeaderBytes into a single log line. 256 holds a realistic proxy chain.
const blockLogXFFMaxLen = 256

// newBlockLogSampler builds the block log's own sampler (see logSampler), held
// by the limiter alone so its budget is never spent by another call site.
//
// Why blocked requests need their own log line at all: Logger() is registered
// AFTER the cheap gates, so a 429 abort never reaches it and leaves no
// access-log entry. Prometheus counts the 429 but carries no client IP
// (correctly — that would be unbounded cardinality). Without this, "who is
// hammering us" is unanswerable for exactly the traffic that got blocked. On
// 2026-07-29 the culprit was identifiable only because its loop sat 4% under the
// tier ceiling and was therefore never blocked; once the auth-failure tier
// catches that shape, the same investigation would have come up empty.
func newBlockLogSampler() *logSampler {
	return newLogSampler(blockLogWindow, blockLogMaxLinesPerWindow)
}

// TieredRateLimiter is the global limiter middleware. It classifies each request
// by route into a tier and enforces that tier's per-IP quota via the shared
// (cross-pod) limiter. Exempt routes pass straight through.
//
// It additionally throttles sources that keep failing authentication. The
// per-tier quotas alone do not cover this: on 2026-07-29 a broken client looped
// `DELETE /app/notifications/device-token` at 4.8 req/s — every response a 401 —
// for hours, because tierDefault allows 300 req/60s (5.0 req/s) and the loop sat
// 4% under that ceiling. It never tripped a single limit. A request that cannot
// authenticate is worth strictly less than one that can, so it gets its own,
// much tighter budget — spent only on authenticated traffic, never on the
// anonymous public reads (authFailureAxisApplies).
func TieredRateLimiter(limiter *ratelimit.Limiter, policy RateLimitPolicy) gin.HandlerFunc {
	return tieredRateLimiter(limiter, policy, newBlockLogSampler())
}

// tieredRateLimiter takes the block-log sampler as a parameter so tests can
// drive its clock instead of waiting out a real window.
func tieredRateLimiter(limiter *ratelimit.Limiter, policy RateLimitPolicy, blockLog *logSampler) gin.HandlerFunc {
	return func(c *gin.Context) {
		t := classifyTier(c.FullPath())
		if t == tierExempt {
			c.Next()
			return
		}
		subject, kind := RateLimitSubject(c)
		authAxis := policy.AuthFailurePerWindow > 0 && authFailureAxisApplies(t, kind)

		// Gate on the auth-failure counter before doing any work. Count (not
		// Allow) because this counter is incremented only by 401 responses
		// below — gating with Allow would make every request its own increment.
		if authAxis &&
			limiter.Count(c.Request.Context(), authFailureKey(subject), policy.Window) >= int64(policy.AuthFailurePerWindow) {
			logBlocked(c, blockLog, "auth_failures", subject)
			response.SetRateLimitHeaders(c, policy.AuthFailurePerWindow, 0, policy.Window)
			response.Err(c, http.StatusTooManyRequests, throttleBizCode(c), "too many authentication failures")
			c.Abort()
			return
		}

		limit := policy.limitFor(t, kind)
		key := "ratelimit:" + tierNames[t] + ":" + subject
		if !limiter.Allow(c.Request.Context(), key, limit, policy.Window) {
			logBlocked(c, blockLog, "tier_"+tierNames[t], subject)
			response.SetRateLimitHeaders(c, limit, 0, policy.Window)
			response.Err(c, http.StatusTooManyRequests, throttleBizCode(c), "rate limit exceeded")
			c.Abort()
			return
		}
		c.Next()

		// Count the failure after the handler chain has resolved the status.
		// Allow's return value is irrelevant here — it is being used purely as
		// the atomic INCR+expire that backs the counter Count reads.
		if authAxis && c.Writer.Status() == http.StatusUnauthorized {
			_ = limiter.Allow(c.Request.Context(), authFailureKey(subject),
				policy.AuthFailurePerWindow, policy.Window)
		}
	}
}

// throttleBizCode picks the envelope code a 429 carries. Everywhere it stays the
// legacy 429-as-business-code; on a delegated route it becomes 9273
// DELEGATION_RATE_LIMITED.
//
// 只在真的要回 429 的那兩個分支呼叫，不在每次請求開頭先算好：它查的是一張有鎖的
// 登錄表，而限流中介層跑在**每一個**請求上，被節流的請求則是極少數。
//
// 56 號 §6.1.0 行 2317 定版：代行沿用既有的 tierDefault（300 次 / 60 秒），不新增層級、
// 不新增 per-endpoint 額度，而「9273 即該既有 tier 觸發時回傳的業務碼」。實作原本一路
// 送 429，於是 §6.5.1 登錄的 9273 在整個 repo 沒有任何 emitter，前端對它寫的處置分支
// （倒數後安全重試、不得重複 mutation）永遠不會被觸發。這裡只換業務碼——HTTP 仍是
// 429（bizhttp.go 對 9273 登錄的正是 429），限流行為、額度、key 一律不動。
//
// 兩條 429 分支共用同一個碼是刻意的。auth-failure 軸與 tier 額度是兩個不同的軸，但它
// 們對呼叫端的意思完全相同：你被節流了，倒數後再來，不要重送 mutation。給其中一條裸
// 429、另一條 9273，等於讓同一組端點的同一個 HTTP status 帶兩種業務碼，前端得為一個
// 沒登錄的碼寫特例。要分辨兩者用的是 message 與 block log 的 reason 欄，那兩處都還在。
// （另立一個新業務碼會是 CodeMeta 契約變更，且 §6.1.0 行 2337 明訂新增限流層級不在本期
// 範圍。）
func throttleBizCode(c *gin.Context) int32 {
	if IsDelegatedRoute(c) {
		return bizcode.DelegationRateLimited
	}
	return 429
}

// logBlocked records one line per (reason, subject) per window naming the source
// that got throttled. Field names match Logger()'s access-log schema (ip,
// method, path) so both are queryable the same way. path is the matched route,
// not the raw URL — enough to see what was being hammered without logging IDs
// or query strings.
//
// ip and subject are BOTH emitted and they are not the same question. ip is
// forensic ("which machine sent this"); subject is attribution ("whose quota did
// it spend"), and under SSR / CI / admin-relayed traffic the two differ. subject
// must not be folded into the ip field: "s:ssr" is not an address, and a field
// name that lies turns every existing log query silently wrong — the same reason
// the OTP proto field was renamed off client_ip. Metrics cannot answer this
// either; they deliberately carry no subject label (unbounded cardinality), so
// this log line is the only place "which subject got blocked" exists.
func logBlocked(c *gin.Context, sampler *logSampler, reason, subject string) {
	if !sampler.admitWithNotice(c.Request.Context(), pkglogger.Warn,
		reason+":"+subject, "[RateLimit] block log truncated") {
		return
	}
	pkglogger.Warn(c.Request.Context(), "[RateLimit] request blocked",
		zap.String("reason", reason),
		zap.String("ip", forensicClientIP(c)),
		zap.String("subject", subject),
		zap.String("xff", truncateForLog(c.GetHeader("X-Forwarded-For"), blockLogXFFMaxLen)),
		zap.String("method", c.Request.Method),
		zap.String("path", routeKey(c)),
	)
}

// truncateForLog caps a caller-controlled value and marks that it was cut, so a
// truncated chain is never read as the whole chain.
func truncateForLog(v string, max int) string {
	if len(v) <= max {
		return v
	}
	return v[:max] + "...(truncated)"
}

// NewRateLimiter builds the global tiered limiter, reusing the Redis client
// already injected via SetRedisClient so counting is shared across pods; when
// Redis is unavailable it falls back to a bounded in-memory limiter. Called once
// at engine construction.
func NewRateLimiter(policy RateLimitPolicy, logger *zap.Logger) gin.HandlerFunc {
	policy.publishQuotas(logger)
	limiter := ratelimit.New(GuardRedisClient(), policy.FallbackCapacity, logger)
	return TieredRateLimiter(limiter, policy)
}

// publishQuotas exports the configured ceilings and shouts about any that would
// leave an axis unguarded.
//
// Only the three moderate-tier kind quotas are checked, and deliberately not
// "any non-positive limit": AuthFailurePerWindow = 0 is an existing, documented
// contract meaning "the auth-failure axis is off", so a blanket rule would turn
// that switch into "refuse everything".
func (p RateLimitPolicy) publishQuotas(logger *zap.Logger) {
	for _, q := range []struct {
		kind  SubjectKind
		field string
		value int
	}{
		{SubjectVisitor, "ModeratePerWindow", p.ModeratePerWindow},
		{SubjectSSR, "ModerateSSRPerWindow", p.ModerateSSRPerWindow},
		{SubjectCI, "ModerateCIPerWindow", p.ModerateCIPerWindow},
	} {
		if q.value <= 0 && logger != nil {
			logger.Error("rate-limit quota unset — this axis falls back to the visitor quota; check the RateLimitPolicy literal and its env",
				zap.String("field", q.field), zap.String("subject_kind", q.kind.String()))
		}
		rateLimitQuota.WithLabelValues(tierNames[tierModerate], q.kind.String()).
			Set(float64(p.limitFor(tierModerate, q.kind)))
	}
	rateLimitQuota.WithLabelValues(tierNames[tierDefault], SubjectVisitor.String()).
		Set(float64(p.limitFor(tierDefault, SubjectVisitor)))
}
