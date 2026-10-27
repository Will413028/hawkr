package payuni

import "golang.org/x/time/rate"

// DefaultMaxQPS 是 PayUni 對代理商 API 的呼叫上限（外部限制，舊 seller-service 沿用的 2 QPS）。
const DefaultMaxQPS = 2.0

// newLimiter 是同一個 client 所有呼叫（bind、update、查詢，不分哪個 job）共用的 token bucket：burst 為 1，所以任何
// 時候的送出速率都不超過上限（含大量輪詢同時到達時，不會一次衝出去）。maxQPS 不是正數時用預設值。
func newLimiter(maxQPS float64) *rate.Limiter {
	if maxQPS <= 0 {
		maxQPS = DefaultMaxQPS
	}
	return rate.NewLimiter(rate.Limit(maxQPS), 1)
}
