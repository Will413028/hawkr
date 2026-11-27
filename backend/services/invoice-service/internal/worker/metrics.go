package worker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)


const (
	workerNameIssue    = "op"
	workerNameOutbox   = "ob"
	workerNameDeadline = "dl"
	workerNameTrack    = "trk"
	workerNameHealth   = "hc"
	workerNameRecheck  = "rc"
	workerNameGov      = "gov"
	workerNamePDF      = "pdf"
)

var invoiceIssueResults = []string{
	"succeeded",
	"permanent_failed",
	"retry_exhausted",
	"unknown_terminal",
	"cancelled",
	"not_applicable",
	"resolved_externally",
}

var invoiceFulfillmentModes = []string{
	"AMEGO_AUTO",
	"SELLER_SELF_ISSUE",
	"PLATFORM_RECEIPT",
	"LEGACY_MANUAL",
}

var invoiceDocumentKinds = []string{"B2C", "B2B", "RECEIPT"}

// Metrics 持有 invoice-service 自己 export 的業務指標。
//
// pkg/outbox.NewMetrics 的四支（M11）與 pkg/mq 內建的 mq_consume_total（M12）不在這裡，
// 它們由各自的 framework 註冊。
type Metrics struct {
	// M1：開票結果。語意是「一個 operation 離開自動推進迴圈時恰好 +1」（一輩子一次），
	// 不是嘗試級計數。
	IssueTotal *prometheus.CounterVec
	// M5：我方 client-side 限速上限。《硬上限依賴接入治理規範》三件套的「上限」series
	// ——只有用量沒有上限，佔比算不出來。
	//
	// ⚠️ 它的另一半（用量＝M3）與 M2／M4／M14 都在 internal/client/amego/metrics.go
	// ——觀測點在那一側，放這裡的十個月間它們是「有註冊、零觀測」（見該檔檔頭）。
	// 本欄留在這裡是因為它的值來自設定、在啟動時由 di.go 設一次，與往返無關。
	AmegoRateLimitQPS prometheus.Gauge
	// M22：開票端到端延遲。上界必須到 172800（48h），逾期案件全擠進 +Inf 時 p99 沒有意義。
	IssueE2EDuration *prometheus.HistogramVec
	// M25：每支排程器單次 tick 的耗時。
	WorkerTickDuration *prometheus.HistogramVec
	// M26：每支排程器的 tick 間隔（啟動時設一次）。M25 是分子、它是分母，缺一算不出飽和度。
	WorkerTickInterval *prometheus.GaugeVec
}

// NewMetrics 註冊並**預先展開**全部已知 label 組合。
//
// 走 promauto ⇒ 註冊進 prometheus.DefaultRegisterer，也就是 pkg/server 的
// `/metrics`（promhttp.Handler()）會 gather 的那一個。同一個 process 內呼叫兩次會在
// 第二次 promauto 註冊時 panic ⇒ 只能 provide 一次。
func NewMetrics() *Metrics {
	m := &Metrics{
		IssueTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "invoice_issue_total",
			Help: "Invoice operations that left the automatic pipeline, by terminal result",
		}, []string{"result", "fulfillment_mode"}),

		AmegoRateLimitQPS: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "invoice_amego_client_rate_limit_qps",
			Help: "Configured client-side rate limit toward AMEGO (the ceiling half of the quota pair)",
		}),

		IssueE2EDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name: "invoice_issue_e2e_duration_seconds",
			Help: "End-to-end issuance latency from payment instant to provider issue instant",
			Buckets: []float64{
				5, 10, 30, 60, 120, 300, 600, 1800, 3600, 7200, 21600, 86400, 172800,
			},
		}, []string{"document_kind"}),

		WorkerTickDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "invoice_worker_tick_duration_seconds",
			Help:    "Duration of one scheduler tick, per worker",
			Buckets: []float64{.05, .1, .25, .5, 1, 2, 5, 10, 30, 60, 120},
		}, []string{"worker"}),

		WorkerTickInterval: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "invoice_worker_tick_interval_seconds",
			Help: "Configured tick interval, per worker (the denominator for tick saturation)",
		}, []string{"worker"}),
	}
	m.preRegister()
	return m
}

func (m *Metrics) preRegister() {
	for _, result := range invoiceIssueResults {
		for _, mode := range invoiceFulfillmentModes {
			m.IssueTotal.WithLabelValues(result, mode).Add(0)
		}
	}
	for _, kind := range invoiceDocumentKinds {
		m.IssueE2EDuration.WithLabelValues(kind)
	}
	for _, w := range []string{workerNameIssue, workerNameOutbox, workerNameDeadline} {
		m.WorkerTickDuration.WithLabelValues(w)
		m.WorkerTickInterval.WithLabelValues(w).Set(0)
	}
}

// ObserveIssueResult 記一次 M1。呼叫端必須保證「一個 operation 一輩子一次」。
func (m *Metrics) ObserveIssueResult(result, fulfillmentMode string) {
	if !containsString(invoiceIssueResults, result) {
		return
	}
	if !containsString(invoiceFulfillmentModes, fulfillmentMode) {
		return
	}
	m.IssueTotal.WithLabelValues(result, fulfillmentMode).Inc()
}

// ObserveIssueE2E 記一次 M22。
func (m *Metrics) ObserveIssueE2E(documentKind string, seconds float64) {
	if !containsString(invoiceDocumentKinds, documentKind) || seconds < 0 {
		return
	}
	m.IssueE2EDuration.WithLabelValues(documentKind).Observe(seconds)
}

func containsString(vals []string, v string) bool {
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}
