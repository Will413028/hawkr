package httpapi

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"connectrpc.com/connect"

	identityconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
	mediaconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1/mediav1alpha1connect"
	notificationconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1/notificationv1alpha1connect"
	sellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	socialconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
)

const rateWindow = time.Minute

type rateTier struct {
	name             string
	visitor, ssr, ci int
}

var (
	tierPublic      = rateTier{name: "public", visitor: 600, ssr: 12000, ci: 300}
	tierDefault     = rateTier{name: "default", visitor: 300}
	tierProductView = rateTier{name: "product-view", visitor: 120, ssr: 2400, ci: 60}
	tierPostView    = rateTier{name: "post-view", visitor: 120, ssr: 2400, ci: 60}
)

var tierBeginUpload = rateTier{name: "begin-upload", visitor: 120}

var tierAppLogin = rateTier{name: "app-login", visitor: 30}

var tierWatch = rateTier{name: "watch", visitor: 120}

var procedureTiers = map[string]rateTier{
	sellerconnect.SellerPublicServiceRecordProductViewProcedure:      tierProductView,
	socialconnect.SocialPublicServiceRecordPostViewProcedure:         tierPostView,
	mediaconnect.MediaAppServiceBeginUploadProcedure:                 tierBeginUpload,
	mediaconnect.MediaAppServiceBeginSecureUploadProcedure:           tierBeginUpload,
	identityconnect.IdentityPublicServiceIssueAppLoginNonceProcedure: tierAppLogin,
	identityconnect.IdentityPublicServiceExchangeIdTokenProcedure:    tierAppLogin,

	notificationconnect.NotificationAppServiceWatchNotificationsProcedure: tierWatch,
}

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

const maxRateKeys = 100_000

type rateCounter struct {
	count int
	reset time.Time
}

type rateLimiter struct {
	failClosed bool
	now        func() time.Time
	mu         sync.Mutex
	m          map[string]*rateCounter
}

func newRateLimiter(now func() time.Time) *rateLimiter {
	return &rateLimiter{now: now, m: map[string]*rateCounter{}}
}

func (l *rateLimiter) allow(key string, limit int) (bool, time.Duration) {
	return l.allowAll(rateRequest{key: key, limit: limit, window: rateWindow})
}

type rateRequest struct {
	key    string
	limit  int
	window time.Duration
}

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

var tierAuth = rateTier{name: "auth", visitor: 30}

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

type accountLimiter struct{ l *rateLimiter }

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
