package worker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// 業務指標（§8.2.3）
//
// 🔴 **本檔的重點是「預先註冊」不是「用到才建」。** Go 的 counter/histogram 在第一次
// 觀測之前不會出現在 /metrics，而 Prometheus 告警的 selector 對**不存在的序列**永遠
// 不會 fire ⇒ `business-alerts.yml` 裡的 invoice 告警會是永久靜默的擺設。
// 所以每個已知的 label 組合在啟動時就 `WithLabelValues(...)` 一次（counter 用
// `.Add(0)`），讓序列存在且值為 0。
//
// **驗收判準是「序列存在」，不是「值為 0」**——值為 0 才是正確的初始狀態。
//
// label 基數紅線（§8.2.3 末）：禁止把 store_id／issuer_id／order_no／invoice_number／
// buyer_id／carrier_id／AMEGO 原始錯誤碼當 label。「是哪一家、哪一張」是稽核問題與
// ADM 工作台問題，不是 metric 問題。
// ---------------------------------------------------------------------------

// 本波實際會觸發的 worker 名（M25／M26 的 label 值域）。
//
// §8.2.3 M25 定義的封閉值域是八值（op/rc/pdf/ob/dl/gov/trk/hc），但現在只有三支
// 排程器真的存在。**刻意只預先註冊這三個**：替不存在的 worker 造一條恆為 0 的序列，
// 會讓告警 ⑲（tick 耗時 ÷ interval）對一支根本沒在跑的 worker 算出「健康」，
// 而那正是「沒有這支 worker」與「這支 worker 壞了」唯一分得出來的地方。
// 其餘五支隨各自的排程器落地時登記。
//
// 🔴 新增排程器時**這裡與 di.go 的 WorkerTickInterval 兩處都要加**：只加 di.go 那半，
// M25 的 histogram 序列在第一次 tick 之前不存在（告警恆不觸發）；只加這裡那半，M26 的
// 分母是 0（飽和度算出 +Inf 或 NaN）。兩處都不會編譯錯。
const (
	workerNameIssue    = "op"  // INV.op：開票送件
	workerNameOutbox   = "ob"  // INV.ob：outbox 派送
	workerNameDeadline = "dl"  // INV.dl：期限與 inbox 兜底掃描
	workerNameTrack    = "trk" // INV.trk：字軌餘量巡檢
	workerNameHealth   = "hc"  // INV.hc：provider 憑證與 provider 健康檢查
	workerNameRecheck  = "rc"  // INV.rc：結果未明對帳 ＋ 上傳狀態覆查
	workerNameGov      = "gov" // INV.gov：治理到期掃描 ＋ 跨期關帳窗口
	workerNamePDF      = "pdf" // INV.pdf：發票 PDF 抓檔落自有儲存
)

// invoiceIssueResults 是 M1 `invoice_issue_total` 的 result 值域（§8.2.3 M1，7 值）。
var invoiceIssueResults = []string{
	"succeeded",
	"permanent_failed",
	"retry_exhausted",
	"unknown_terminal",
	"cancelled",
	"not_applicable",
	"resolved_externally",
}

// invoiceFulfillmentModes 是 M1 的第二個 label 值域（CodeMeta InvoiceFulfillmentMode，4 值）。
var invoiceFulfillmentModes = []string{
	"AMEGO_AUTO",
	"SELLER_SELF_ISSUE",
	"PLATFORM_RECEIPT",
	"LEGACY_MANUAL",
}

// invoiceDocumentKinds 是 M22 的 label 值域（封閉三值）。
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

// preRegister 展開每個已知 label 組合，讓序列在第一次觀測之前就存在。
//
// counter 用 `.Add(0)`、histogram 用 `.WithLabelValues(...)`（取得 observer 即建立序列）。
// 拿掉這個函式的任何一段，對應的告警就會從「未觸發」變成「永遠不會觸發」，而
// /metrics 看起來只是少了幾行——沒有任何東西會轉紅。
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
		// interval 的實際值由 di.go 在啟動時設定；這裡先讓序列存在。
		m.WorkerTickInterval.WithLabelValues(w).Set(0)
	}
}

// ObserveIssueResult 記一次 M1。呼叫端必須保證「一個 operation 一輩子一次」。
func (m *Metrics) ObserveIssueResult(result, fulfillmentMode string) {
	if !containsString(invoiceIssueResults, result) {
		// 值域外的 result 會造出一條沒有人會查的新序列 ⇒ 收斂到 not_applicable 之外
		// 不做，直接不記並不阻斷業務。
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
		// 負延遲代表時鐘偏移（§8.3.1 code 15），記進去只會污染 p99。
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
