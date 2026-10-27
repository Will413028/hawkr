package amego

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// 降檔參數：觸發 code 21 時砍半，5 分鐘後線性回升（避免整批同時被擋、也避免
// 一恢復就再次觸頂）。AMEGO 沒有公開 rate limit、回應也沒有任何 rate-limit header
// ⇒ 依《硬上限依賴接入治理規範》「自訂上限也是上限」，這個數字就是 max series。
const (
	rateRecoveryWindow = 5 * time.Minute
	rateDegradeFactor  = 0.5
)

// rateLimiter 是行程內的限速器。
//
// ⚠️ 它是**每副本**上限不是全站上限：replicas = N ⇒ 真實上限 N × QPS，
// §8.2.4 的告警 expr 在多副本時必須從 max(...) 改成 sum(...)，否則上限被低估 N 倍。
type rateLimiter struct {
	mu         sync.Mutex
	lim        *rate.Limiter
	baseQPS    float64
	degradedAt time.Time
	now        func() time.Time

	// onLocalThrottle 在 Wait 真的排到隊時觸發，對應
	// invoice_amego_rate_limited_total{source="local"}。預設 no-op。
	onLocalThrottle func()
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

// Wait 吃**該次呼叫自己的 ctx**。
//
// 🔴 傳 context.Background 會讓排隊時間跑到租約外面去偷租約 ⇒「單筆最壞耗時 <
// 租約」在高負載時無聲失效（《多副本排程搶佔規範》C1 的形狀）。吃同一個 ctx 之後，
// 排隊時間被那支 API 的 deadline 涵蓋，租約下界才不必為它另計一項。
func (r *rateLimiter) Wait(ctx context.Context) error {
	r.recover()
	if !r.lim.Allow() {
		if r.onLocalThrottle != nil {
			r.onLocalThrottle()
		}
		return r.lim.Wait(ctx) //nolint:wrapcheck // ctx 錯誤原樣上拋，由呼叫端歸類
	}
	return nil
}

// Degrade 在收到 code 21 時把上限砍半。
func (r *rateLimiter) Degrade() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.degradedAt = r.now()
	r.lim.SetLimit(rate.Limit(r.baseQPS * rateDegradeFactor))
}

// recover 讓上限在降檔後的 rateRecoveryWindow 內線性回到 baseQPS。
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
	cur := r.baseQPS * (rateDegradeFactor + rateDegradeFactor*ratio)
	r.lim.SetLimit(rate.Limit(cur))
}

// currentQPS 供測試檢查降檔／回升。
func (r *rateLimiter) currentQPS() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return float64(r.lim.Limit())
}
