package amego

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// AMEGO 往返的四支指標：M2／M3／M4／M14（§8.2.3）
//
// 🔴 **它們先前全部「有註冊、零觀測」**（2026-09-05 第 6 輪稽核實測）。M3／M4／M14
// 原本宣告在 internal/worker/metrics.go 的 Metrics struct 上、也被 preRegister 預先
// 展開，於是 /metrics 上看得到三行值為 0 的序列 —— 而全服務**沒有任何一處 .Inc()**。
// M2 更徹底：連宣告都不存在。後果逐條是：
//
//	① InvoiceAmegoRateLimited（M4）永不觸發 ⇒ AMEGO 觸頂（code 21）完全看不到
//	② InvoiceAmegoRateNearLimit 的分子是 M3 ⇒ 佔比恆 0 ⇒ 永不觸發
//	   （《硬上限依賴接入治理規範》要求的「用量／上限」那一對，用量那半是死的）
//	③ recording rule invoice:amego_success_ratio:7d（SLO-4）的分子分母都是 M2
//	   ⇒ SLO-4 從來算不出來
//	④ InvoiceAuditWriteFailing（M14）永不觸發 ⇒ §8.2.1「例外必吞」失去它唯一的可見形式
//	⑤ 三條 runbook（§12.5 R1／R3／R4）叫人「先看 M2 是不是完全不增加」「看 M3 的 code
//	   分佈分辨 3040111 與 3040174」——事故當下照做只會拿到空的，而**空的長得像
//	   「卡住」**，於是每一次都會被判成 worker 沒跑。
//
// **為什麼放在 amego package 而不是 worker/metrics.go**（與 internal/mq/metrics.go 同一個
// 理由，見該檔檔頭）：
//   - 觀測點只有 `do()` 與 `repoAuditSink.logFail` 兩處，都在 client 這一側；
//     worker 不 import amego，amego 也不該 import worker（audit_sink.go 原本那句
//     「本層不 import 它以免產生 client → worker 的反向依賴」正是 M14 十個月沒有被
//     遞增的原因——依賴方向是對的，缺的是把指標放到依賴的下游）
//   - package-level var 在 process 內天然只註冊一次，沒有「NewMetrics 呼叫兩次即 panic」
//     那條要靠 DI 紀律維持的不變式
//
// label 基數紅線（§8.2.3 末）：endpoint 是 15 個固定路徑、code 走白名單映射
// （`APIError.MetricCodeLabel`，未知一律 "other"）、outcome 四值。三者都是封閉小基數。
// ---------------------------------------------------------------------------

// amegoOutcomes 是 M2 的 outcome 值域（§8.2.3 M2，四值）。
//
// 🔴 `biz_error` 與 `transport_error` **必須分開**：AMEGO 一律回 HTTP 200，用 status
// 分不出來。而 SLO-4 的分子刻意含 biz_error（對方回業務錯誤代表往返成功，混進失敗
// 會把我方 bug 記在對方帳上）—— 兩者合併就等於把 SLO-4 的語意改掉。
var amegoOutcomes = []string{
	outcomeOK,             // 沒有錯誤
	outcomeBizError,       // 對方明確回了一個業務碼（往返成功）
	outcomeTransportError, // 連線／解碼失敗、斷路器開著、本地限速排隊逾時
	outcomeTimeout,        // 逾時
}

const (
	outcomeOK             = "ok"
	outcomeBizError       = "biz_error"
	outcomeTransportError = "transport_error"
	outcomeTimeout        = "timeout"
)

// amegoPreExpandedCodes 是 M3 在啟動時就展開的 code 值域。
//
// 刻意只有兩個：`"0"`（成功）與 `"other"`（未知碼）。已登記的錯誤碼有數十個，
// 15 endpoint × 數十碼 ＝ 數百條恆為 0 的序列，而**告警需要的只有「有沒有在打」**
// （② 的分子是不帶 code filter 的 `rate(invoice_amego_response_code_total[1m])`）。
// 真實錯誤碼在第一次發生時自然出現，runbook 要看的「code 分佈」那時才有意義。
var amegoPreExpandedCodes = []string{"0", CodeOther}

// M2：AMEGO 往返延遲。bucket 上界 30s 對齊開票類 deadline（§8.2.3 M2）。
var amegoRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "invoice_amego_request_duration_seconds",
	Help:    "AMEGO round-trip latency by endpoint and outcome (58 M2)",
	Buckets: []float64{.1, .25, .5, 1, 2, 5, 10, 20, 30},
}, []string{"endpoint", "outcome"})

// M3：AMEGO 逐碼分佈（白名單映射，未知碼收斂成 "other"）。
var amegoResponseCode = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "invoice_amego_response_code_total",
	Help: "AMEGO response codes, whitelisted (unknown codes collapse to \"other\") (58 M3)",
}, []string{"endpoint", "code"})

// M4：rate limit 命中。source=amego（收到 code 21）／local（我方 limiter 擋下）
// 必須分開：前者代表限速沒守住，後者是設計中的正常行為。
var amegoRateLimited = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "invoice_amego_rate_limited_total",
	Help: "Rate limit hits; source=amego means AMEGO rejected us, source=local means our own limiter did (58 M4)",
}, []string{"source"})

// M14：provider 稽核寫入失敗。沒有這支，「例外必吞」就等於「例外消失」。
var amegoAuditWriteFailed = promauto.NewCounter(prometheus.CounterOpts{
	Name: "invoice_audit_write_failed_total",
	Help: "Provider audit-log writes that failed (the only visible form of the swallowed defer) (58 M14)",
})

// observedEndpoints 是真的會被 `do()` 觀測到的端點集合。
//
// 🔴 `/json/time` 不在裡面：`ServerTime` 刻意走自己的路徑（免簽章、免 Credential、
// 不走斷路器、不寫 provider audit —— 校時是斷路器開路期間的恢復手段）。它因此
// **不經過 `do()` 那個 defer**。替它預先展開一條永遠為 0 的序列，等於在儀表板上
// 宣告「這支端點從來沒有出過問題」，而那正是本檔要消滅的形狀。
// 哪天 ServerTime 改走 do()，`TestServerTimeIsOutsideTheObservationWindow` 會轉紅
// 提醒把它加回來。
func observedEndpoints() []string {
	all := Endpoints()
	out := make([]string, 0, len(all))
	for _, ep := range all {
		if ep == EndpointServerTime {
			continue
		}
		out = append(out, ep)
	}
	return out
}

// init 預先展開已知 label 組合。
//
// counter／histogram 在第一次觀測之前不會出現在 /metrics，而 Prometheus 告警的
// selector 對**不存在的序列**永遠不會 fire。驗收判準是「序列存在」不是「值為 0」
// ——值為 0 才是正確的初始狀態。
func init() {
	for _, ep := range observedEndpoints() {
		for _, oc := range amegoOutcomes {
			amegoRequestDuration.WithLabelValues(ep, oc)
		}
		for _, code := range amegoPreExpandedCodes {
			amegoResponseCode.WithLabelValues(ep, code).Add(0)
		}
	}
	for _, src := range []string{rateLimitSourceAMEGO, rateLimitSourceLocal} {
		amegoRateLimited.WithLabelValues(src).Add(0)
	}
	amegoAuditWriteFailed.Add(0)
}

const (
	// rateLimitSourceAMEGO ＝ 對方回 code 21（我方唯一能觀測到的觸頂訊號，
	// AMEGO 未公開 QPS 上限、回應也沒有 Retry-After）。
	rateLimitSourceAMEGO = "amego"
	// rateLimitSourceLocal ＝ 我方 client-side limiter 擋下。這是設計中的正常行為，
	// 與上一個混在同一條序列裡會讓告警 ① 在尖峰長期紅著而被忽略。
	rateLimitSourceLocal = "local"
)

// amegoRateLimitCode 是 AMEGO 的「請求過於頻繁」碼（§4.5.6）。
const amegoRateLimitCode = "21"

// observeCall 記一次往返（M2 ＋ M3 ＋ M4 的 amego 側）。
//
// 呼叫點只有 `do()` 的 defer 一處：那裡是「無論從哪一條 return 出去都恰好經過一次」
// 的位置，與 provider audit 落庫用的是同一個 defer。分散到各 return 前面的話，
// 新增一條 return 就會少一次觀測，而少的那次沒有任何東西看得出來。
func observeCall(endpoint string, apiErr *APIError, seconds float64) {
	outcome := outcomeOK
	code := "0"
	if apiErr != nil {
		outcome = outcomeFor(apiErr)
		code = apiErr.MetricCodeLabel()
	}
	amegoRequestDuration.WithLabelValues(endpoint, outcome).Observe(seconds)
	amegoResponseCode.WithLabelValues(endpoint, code).Inc()
	if apiErr != nil && apiErr.Code == amegoRateLimitCode {
		amegoRateLimited.WithLabelValues(rateLimitSourceAMEGO).Inc()
	}
}

// outcomeFor 把一個 APIError 折成 M2 的 outcome。
//
// 判準錨在 `Kind`（invoice_provider_audit_logs 的 error_kind 四值），不是 Class
// ——Class 講的是「呼叫端該怎麼辦」，這裡問的是「這次往返發生了什麼」，兩者不同義：
// 一個 PERMANENT 的業務碼與一個 PERMANENT 的編碼失敗對 SLO-4 的貢獻方向相反。
func outcomeFor(e *APIError) string {
	switch e.Kind {
	case ErrorKindTimeout:
		return outcomeTimeout
	case ErrorKindProvider:
		return outcomeBizError
	case ErrorKindConn, ErrorKindDecode:
		return outcomeTransportError
	}
	// Kind 為空的只剩我方自產的閘門類錯誤（ENV_GUARD／ENCODE 之前的分支）與
	// 斷路器開啟：都沒有真的送出去 ⇒ 算傳輸面失敗，不得記成對方的業務錯誤。
	return outcomeTransportError
}

// observeLocalRateLimited 記一次「被我方 limiter 擋到逾時」。
func observeLocalRateLimited() {
	amegoRateLimited.WithLabelValues(rateLimitSourceLocal).Inc()
}

// ObserveAuditWriteFailed 遞增 M14。
//
// 匯出是因為唯一的呼叫端在 internal/client（repoAuditSink.logFail）——它 import
// amego，反向不成立。這也是這支指標從宣告到今天從未被遞增過的原因：它原本被放在
// internal/worker 上，而那個 package 在依賴圖的另一邊。
func ObserveAuditWriteFailed() { amegoAuditWriteFailed.Inc() }
