package worker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// INV.trk 的四支字軌指標（§8.2.3 M9／M10／M21／M24）
//
// 🔴 四支全部在此**預先註冊並設 0**，理由同 metrics.go 檔頭：Prometheus 告警的
// selector 對**不存在的序列**永遠不會 fire。這批指標先前完全沒有生產者，於是
// business-alerts.yml 的 InvoiceTrackBalanceLow／InvoiceTrackExhausted／
// InvoiceTrackNextPeriodInsufficient 三條**從上線那天起就結構上不可能觸發**（#2959）。
//
// 🔴 **一律不帶 store_id／issuer_id**（§8.2.3 末的 label 基數紅線，M9／M10／M21／M24
// 各自的說明欄也逐字重申）。「是哪一家」是 ADM 工作台的問題不是 metric 的問題。
//
// M9 與 M10 為什麼要成對（§8.2.4 ①(b) 逐字「M9 ∧ M10 才有意義」）：
// 一家新賣家剛配到號段、本期尚未取用，會把 M9 這個跨賣家最小值拉低到看不出真實風險
// ⇒ 告警只用 M10（門檻判定在產生指標時就做完）。只有 M9 的失敗形狀見 §8.6 T-33。
//
// M10／M21／M24 是**三件事**，不要合併（§8.2.3 M24 逐字）：
//   M10 本期快用完　　M21 本期已用盡　　M24 下期配到的不夠
// 財政部依「本期最後已取用本數」決定下期配號 ⇒ 成長中的賣家必然短缺，M24 是
// 告警 ⑱ 的**唯一**資料源。
// ---------------------------------------------------------------------------

var (
	// M9：跨所有賣家的當期字軌剩餘張數最小值。
	trackRemainingMin = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "invoice_track_remaining_min",
		Help: "Minimum remaining invoice-number capacity across all active sellers (58 M9)",
	})

	// M10：當期餘量低於 INVOICE_TRACK_BALANCE_WARN_THRESHOLD 的賣家家數。
	trackLowSellers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "invoice_track_low_sellers",
		Help: "Sellers whose current-period track balance is below the warn threshold (58 M10)",
	})

	// M21：當期字軌已用盡的賣家家數（remaining == 0，或字軌 status ∈ {3 過期, 9 用畢}）。
	trackExhaustedSellers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "invoice_track_exhausted_sellers",
		Help: "Sellers whose current-period track is exhausted or expired (58 M21)",
	})

	// M24：下期配額不足的賣家家數。
	trackNextPeriodInsufficientSellers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "invoice_track_next_period_insufficient_sellers",
		Help: "Sellers whose next-period track allocation is below projected demand (58 M24)",
	})
)

// registerTrackMetrics 把四支設成 0，讓序列在第一次掃描之前就存在。
//
// 🔴 M9 的初始值刻意是 0 而不是「無限大」：0 代表「還沒量到」與「真的沒有餘量」
// 同形，而後者本來就該讓 InvoiceTrackBalanceLow 亮。反過來設一個很大的初始值，
// 會讓「掃描從來沒跑成功過」看起來像「所有賣家都很健康」——那正是本波要修的病。
func registerTrackMetrics() {
	trackRemainingMin.Set(0)
	trackLowSellers.Set(0)
	trackExhaustedSellers.Set(0)
	trackNextPeriodInsufficientSellers.Set(0)
}
