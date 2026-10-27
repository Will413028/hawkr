package payuni

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	rateRecoveryWindow = 5 * time.Minute
	rateDegradeFactor  = 0.5
)

type rateLimiter struct {
	mu         sync.Mutex
	lim        *rate.Limiter
	baseQPS    float64
	degradedAt time.Time
	now        func() time.Time

	onThrottle func()
	onDegrade  func()
}

func newRateLimiter(qps int, now func() time.Time) *rateLimiter {
	if qps <= 0 {
		qps = 1
	}
	if now == nil {
		now = time.Now
	}
	return &rateLimiter{
		lim:     rate.NewLimiter(rate.Limit(qps), qps),
		baseQPS: float64(qps),
		now:     now,
	}
}

func (r *rateLimiter) Wait(ctx context.Context) error {
	r.recover()
	if !r.lim.Allow() {
		if r.onThrottle != nil {
			r.onThrottle()
		}
		return r.lim.Wait(ctx) //nolint:wrapcheck // ctx 錯誤原樣上拋，由呼叫端歸類
	}
	return nil
}

func (r *rateLimiter) Degrade() {
	r.mu.Lock()
	r.degradedAt = r.now()
	r.lim.SetLimit(rate.Limit(r.baseQPS * rateDegradeFactor))
	r.mu.Unlock()
	if r.onDegrade != nil {
		r.onDegrade()
	}
}

func (r *rateLimiter) recover() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.degradedAt.IsZero() {
		return
	}
	elapsed := r.now().Sub(r.degradedAt)
	if elapsed >= rateRecoveryWindow {
		r.degradedAt = time.Time{}
		r.lim.SetLimit(rate.Limit(r.baseQPS))
		return
	}
	ratio := float64(elapsed) / float64(rateRecoveryWindow)
	r.lim.SetLimit(rate.Limit(r.baseQPS * (rateDegradeFactor + rateDegradeFactor*ratio)))
}

func (r *rateLimiter) currentQPS() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return float64(r.lim.Limit())
}

func looksRateLimited(err error) bool {
	if err == nil {
		return false
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, code := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		if strings.Contains(msg, fmt.Sprintf("payuni api status %d", code)) {
			return true
		}
	}
	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "too many requests")
}
