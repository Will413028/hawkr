package httpapi

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"connectrpc.com/connect"

	identityconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
	mediaconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1/mediav1alpha1connect"
	sellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	socialconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
)

// 單體的 Connect 以限流的身分（subject，見 subject.go）分級計數，每個 instance 各自計數：這是粗略的防護，全站的
// 限流在後端 CD 設在 CDN（模組 CARRYOVER.md 的傳輸層盤點）。web 登入入口（/api/auth/*）另有每 IP 的級別（tierAuth）。
// 轉給 gateway 的 REST 由 gateway 自己限流。級別與額度沿用 gateway 的 classifyTier（每 60 秒）。

const rateWindow = time.Minute

// rateTier 是一個級別在 rateWindow 內的額度。ssr、ci 是網頁 SSR 與 CI 的身分（它們代表很多人），
// 0 表示同 visitor。
type rateTier struct {
	name             string
	visitor, ssr, ci int
}

var (
	tierPublic  = rateTier{name: "public", visitor: 600, ssr: 12000, ci: 300}
	tierDefault = rateTier{name: "default", visitor: 300}
	// tierProductView 是商品瀏覽打點：匿名、不需要登入、每次都寫入一筆事件，是公開寫入裡最容易被灌的一個，額度比一般的
	// 公開讀取低（頁面每載入一次送一次；共用位址的人也在額度內）。
	tierProductView = rateTier{name: "product-view", visitor: 120, ssr: 2400, ci: 60}
	// tierPostView 是貼文瀏覽打點：匿名、每次都可能寫入一筆去重列，額度同 tierProductView（頁面每載入一篇送一次；共用位址的人也在額度內）。
	tierPostView = rateTier{name: "post-view", visitor: 120, ssr: 2400, ci: 60}
)

// tierBeginUpload 是 MediaAppService.BeginUpload 的額度：每次都發預簽網址並寫一筆暫存列，額度與 tierProductView 的 visitor 相同（每個登入帳號每分鐘 120 次；
// ssr 與 ci 不設，因為只有登入帳號會呼叫）。一次發文或換頭像只用一次，批次上傳的使用者也在額度內。
var tierBeginUpload = rateTier{name: "begin-upload", visitor: 120}

// tierAppLogin 是 App 原生登入的兩個公開 procedure（IssueAppLoginNonce、ExchangeIdToken）的額度，兩者共用同一個計數器，以來源 IP 計數（沒有登入的
// 請求；帶有效 session 重新驗證的以帳號計）。一次登入是兩個請求，每分鐘 30 個（約 15 次登入），與 web 的 tierAuth 同一個保守的預設，擋灌 nonce 與
// 猜 ID token 的程式；共用位址的人也在額度內。SSR 與 CI 的身分沒有加倍：App 的請求不經它們。
var tierAppLogin = rateTier{name: "app-login", visitor: 30}

// procedureTiers 是需要自己額度的 procedure；其餘依 service 的使用者類別。
var procedureTiers = map[string]rateTier{
	sellerconnect.SellerPublicServiceRecordProductViewProcedure:      tierProductView,
	socialconnect.SocialPublicServiceRecordPostViewProcedure:         tierPostView,
	mediaconnect.MediaAppServiceBeginUploadProcedure:                 tierBeginUpload,
	identityconnect.IdentityPublicServiceIssueAppLoginNonceProcedure: tierAppLogin,
	identityconnect.IdentityPublicServiceExchangeIdTokenProcedure:    tierAppLogin,
}

// tierOf 決定 Connect procedure 的級別：procedureTiers 列出的用自己的級別，其餘依 service 的使用者類別。
func tierOf(procedure string) rateTier {
	if t, ok := procedureTiers[procedure]; ok {
		return t
	}
	audience, _ := rpc.AudienceOf(procedure)
	switch audience {
	case rpc.AudiencePublic:
		return tierPublic
	case rpc.AudienceApp, rpc.AudienceAdmin, rpc.AudienceDelegated:
	}
	return tierDefault
}

func (t rateTier) limit(subject string) int {
	switch {
	case subject == "s:ssr" && t.ssr > 0:
		return t.ssr
	case subject == "s:ci" && t.ci > 0:
		return t.ci
	}
	return t.visitor
}

// maxRateKeys 是計數器的上限（掃描器換位址灌進來時，記憶體不會無限增長）。滿了先清掉過期的；一個時間窗內
// 還有這麼多個身分，只可能是攻擊，這時新的身分不計數（放行），既有的計數保留：不能讓攻擊者以大量位址把所有人的
// 計數歸零。需要以帳號（不是位址）計數的操作，額度宣告在方法的授權宣告上，由 accountLimiter 計數（見
// internal/httpapi/seller 的宣告表）；它同樣只在這個程序的記憶體內。
const maxRateKeys = 100_000

type rateCounter struct {
	count int
	reset time.Time
}

// rateLimiter 是固定時間窗的計數。
type rateLimiter struct {
	// failClosed 為 true 時，計數表滿了、新的 key 無法計數就拒絕；false（IP 層）是放行。
	failClosed bool
	now        func() time.Time
	mu         sync.Mutex
	m          map[string]*rateCounter
}

func newRateLimiter(now func() time.Time) *rateLimiter {
	return &rateLimiter{now: now, m: map[string]*rateCounter{}}
}

// allow 記一次 key 的請求（時間窗是 rateWindow），回傳是否在額度內；超過時一併回傳多久之後可以再試。
func (l *rateLimiter) allow(key string, limit int) (bool, time.Duration) {
	return l.allowAll(rateRequest{key: key, limit: limit, window: rateWindow})
}

// rateRequest 是一次計數：key 在 window 內最多 limit 次。同一個 key 每次要用同一個時間窗。
type rateRequest struct {
	key    string
	limit  int
	window time.Duration
}

// allowAll 為一個請求在一或多個 key 計數：全部在額度內才一起記一次；任何一個超過就都不記（沒被服務的請求不燒掉其他時間窗的
// 額度），回傳最久的等待時間（等到那時，所有超過的時間窗都已重設）。
func (l *rateLimiter) allowAll(reqs ...rateRequest) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	counters := make([]*rateCounter, len(reqs))
	allowed, wait := true, time.Duration(0)
	for i, r := range reqs {
		c, ok := l.m[r.key]
		if !ok && len(l.m) >= maxRateKeys {
			for k, old := range l.m {
				if !now.Before(old.reset) {
					delete(l.m, k)
				}
			}
			if len(l.m) >= maxRateKeys {
				if l.failClosed {
					allowed, wait = false, max(wait, r.window)
				}
				continue
			}
		}
		if !ok || !now.Before(c.reset) {
			c = &rateCounter{reset: now.Add(r.window)}
			l.m[r.key] = c
		}
		counters[i] = c
		if c.count+1 > r.limit {
			allowed, wait = false, max(wait, c.reset.Sub(now))
		}
	}
	if !allowed {
		return false, wait
	}
	for _, c := range counters {
		if c != nil {
			c.count++
		}
	}
	return true, 0
}

// withRateLimit 為 Connect 的請求計數：超過 procedure 級別的額度就回
// resource_exhausted（帶 RetryInfo），不記。
func withRateLimit(l *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subject := rpc.SubjectFrom(r.Context())
		tier := tierOf(r.URL.Path)
		if ok, wait := l.allowAll(rateRequest{key: tier.name + "|" + subject, limit: tier.limit(subject), window: rateWindow}); !ok {
			_ = connect.NewErrorWriter().Write(w, r, rpc.NegotiatedRetryLater(w, r, wait))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withFileRateLimit 為檔案回應（/api/files/）計數：級別與後台、App 的 Connect 方法相同（tierDefault，身分由入口判定：管理員是 m:<ID>），
// 超過額度回 429 加 Retry-After（沒有 Connect 的錯誤格式）；回應不可快取。
func withFileRateLimit(l *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tier := tierDefault
		subject := rpc.SubjectFrom(r.Context())
		if ok, wait := l.allow(tier.name+"|"+subject, tier.limit(subject)); !ok {
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("Retry-After", strconv.Itoa(int(wait/time.Second)+1))
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// tierAuth 是 web 登入入口（/api/auth/*：start、callback）的額度，以連線端 IP 計數，不分登入與否、也不信 origin-auth 的身分：登入的請求
// 是瀏覽器導覽，不經 SSR。一次登入是 start 加 callback 兩個請求，每個來源每分鐘 30 個請求（約 15 次登入）：共用位址（公司、校園的 NAT）的
// 人也在額度內，掃描器與灌 login_flows 的程式則被擋。業界沒有固定的數字，這是保守的預設，要調整先看 429 的比例。
var tierAuth = rateTier{name: "auth", visitor: 30}

// withAuthRateLimit 為 web 登入入口計數：每個連線端 IP（IPv6 以 /64 聚合，同 visitorSubject）一個計數器，start 與 callback 共用；超過額度回
// 429 加 Retry-After；回應不可快取。
func withAuthRateLimit(l *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subject := visitorSubject(rpc.ClientIPFrom(r.Context()))
		if ok, wait := l.allow(tierAuth.name+"|"+subject, tierAuth.limit(subject)); !ok {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Retry-After", strconv.Itoa(int(wait/time.Second)+1))
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// accountLimiter 是以帳號為身分的額度計數（httpseller.Limiter）：key 由呼叫方組成（方法加帳號），額度與時間窗由方法的
// 授權宣告給。它與入口的 IP 限流是兩份計數。
type accountLimiter struct{ l *rateLimiter }

// newAccountLimiter 的計數表滿了時，新的帳號被拒絕（fail closed），與 IP 層相反：IP 層的 key 可以被掃描器大量製造，放行
// 才不會誤傷正常的訪客；帳號層的 key 是已登入的帳號，只有身分比對與對外查詢這類低頻、敏感的方法，表滿代表有人以大量帳號
// 灌入，這時不能讓新的帳號繞過額度。
func newAccountLimiter(now func() time.Time) accountLimiter {
	l := newRateLimiter(now)
	l.failClosed = true
	return accountLimiter{l}
}

func (a accountLimiter) Allow(limits ...httpseller.Limit) (bool, time.Duration) {
	reqs := make([]rateRequest, len(limits))
	for i, lim := range limits {
		reqs[i] = rateRequest{key: lim.Key, limit: lim.Quota, window: lim.Window}
	}
	return a.l.allowAll(reqs...)
}
