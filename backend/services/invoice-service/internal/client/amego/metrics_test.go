package amego

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// 本檔守 M2／M3／M4／M14 的兩件事，兩件都是「不會有任何別的東西轉紅」的形狀：
//
//	① 序列在第一次觀測之前就存在（不存在 ⇒ 告警的 selector 永遠不會 fire）
//	② `do()` 真的會觀測它們（有註冊、零觀測 ⇒ 序列存在但恆為 0，
//	   在 `> 0` 與比率型告警上與「一切正常」完全一樣）
//
// ② 是 2026-09-05 第 6 輪稽核抓到的那一格：三支指標宣告完整、預先展開完整、
// /metrics 上看得到，而全服務沒有任何一處 `.Inc()`。**只驗 ① 的測試會全綠。**

// metricsOwnedHere 是本 package 註冊的指標，逐名對拍 §8.2.3。
var metricsOwnedHere = []string{
	"invoice_amego_request_duration_seconds", // M2
	"invoice_amego_response_code_total",      // M3
	"invoice_amego_rate_limited_total",       // M4
	"invoice_audit_write_failed_total",       // M14
}

func TestAmegoMetricsArePreRegistered(t *testing.T) {
	if len(metricsOwnedHere) == 0 {
		t.Fatal("清單是空的 —— 這支測試會恆綠")
	}
	body := scrapeAmegoMetrics(t)
	for _, name := range metricsOwnedHere {
		if !strings.Contains(body, "# HELP "+name+" ") {
			t.Errorf("/metrics 缺少 %s —— 對應的告警規則永遠不會 fire", name)
		}
	}
}

// TestAmegoMetricsPreExpandEveryEndpoint 守「每個端點都預先展開」。
//
// 🔴 錨在 `Endpoints()` 不是手寫一份清單：新增一支 API 時手寫清單不會提醒你回來改，
// 而少展開的那個端點在第一次被打之前不存在於 /metrics。
func TestAmegoMetricsPreExpandEveryEndpoint(t *testing.T) {
	body := scrapeAmegoMetrics(t)
	eps := observedEndpoints()
	if len(eps) == 0 {
		t.Fatal("Endpoints() 是空的 —— 前提壞了")
	}
	for _, ep := range eps {
		want := `invoice_amego_response_code_total{code="0",endpoint="` + ep + `"} `
		if !strings.Contains(body, want) {
			t.Errorf("/metrics 缺少預先展開的序列：%s", want)
		}
	}
	for _, src := range []string{rateLimitSourceAMEGO, rateLimitSourceLocal} {
		if !strings.Contains(body, `invoice_amego_rate_limited_total{source="`+src+`"} `) {
			t.Errorf("/metrics 缺少 source=%s 的序列", src)
		}
	}
}

// TestOutcomeForSeparatesBizFromTransport 是 M2 的鑑別力本體。
//
// SLO-4 的分子刻意含 `biz_error`（對方回業務錯誤代表往返成功）；把它折進
// transport_error 會讓我方 bug 被記在對方帳上，而那個錯誤**不會讓任何測試轉紅**
// ——比率仍然算得出來，只是答案是錯的。所以逐 Kind 釘死。
func TestOutcomeForSeparatesBizFromTransport(t *testing.T) {
	cases := []struct {
		kind string
		want string
	}{
		{ErrorKindProvider, outcomeBizError},
		{ErrorKindTimeout, outcomeTimeout},
		{ErrorKindConn, outcomeTransportError},
		{ErrorKindDecode, outcomeTransportError},
		{"", outcomeTransportError}, // ENV_GUARD／斷路器：一個字都沒送出去
	}
	for _, c := range cases {
		if got := outcomeFor(&APIError{Kind: c.kind}); got != c.want {
			t.Errorf("outcomeFor(Kind=%q) = %q, want %q", c.kind, got, c.want)
		}
	}
	if len(amegoOutcomes) != 4 {
		t.Errorf("outcome 值域有 %d 值，§8.2.3 M2 是 4 值", len(amegoOutcomes))
	}
}

// TestDoObservesEveryCall 是 ② 的本體：**真的打一次**，看 M2／M3 動了。
//
// 不加 t.Parallel()：三支指標是 process 全域的，並行跑會讓「前後差」失去意義。
//
// 變異驗證（2026-09-05 實跑）：把 `do()` defer 裡的 `observeCall(...)` 拿掉，
// 本測試轉紅；還原後轉綠。**只驗「序列存在」的測試對那個變異完全無感**——
// 那正是 M3／M4／M14 有註冊零觀測而沒有任何東西轉紅的原因。
func TestDoObservesEveryCall(t *testing.T) {
	codeSeries := `invoice_amego_response_code_total{code="0",endpoint="` + EndpointIssue + `"}`
	durSeries := `invoice_amego_request_duration_seconds_count{endpoint="` + EndpointIssue + `",outcome="ok"}`
	codeBefore := counterValue(t, codeSeries)
	durBefore := counterValue(t, durSeries)

	h := newHarness(t, Options{AllowRealInvoice: true}, okJSON(`{"code":0,"msg":"","invoice_number":"AA00000001","random_number":"1234","invoice_time":1787578776}`))
	// 回傳值刻意不斷言：本測試問的是「這一次往返有沒有被觀測到」，
	// 而觀測掛在 do() 的 defer 上 —— 它在解析回應之前就已經發生。
	_, _ = h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest())

	if after := counterValue(t, codeSeries); after != codeBefore+1 {
		t.Errorf("M3 %s = %v, want %v —— 一次真的往返沒有留下計數", codeSeries, after, codeBefore+1)
	}
	if after := counterValue(t, durSeries); after != durBefore+1 {
		t.Errorf("M2 %s = %v, want %v —— SLO-4 的分子與分母都來自它", durSeries, after, durBefore+1)
	}
}

// TestServerTimeIsOutsideTheObservationWindow 把觀測窗的邊界寫成會失敗的東西。
//
// `/json/time` 走自己的路徑（免簽章、不走斷路器、不寫 audit）⇒ 不經過 do()
// ⇒ 它**不該**有預先展開的序列：一條永遠為 0 的序列在 `> 0` 型告警與儀表板上
// 等於「這支端點從來沒出過問題」。哪天有人把 ServerTime 改走 do()，這裡轉紅，
// 提醒把它加進 observedEndpoints()。
func TestServerTimeIsOutsideTheObservationWindow(t *testing.T) {
	for _, ep := range observedEndpoints() {
		if ep == EndpointServerTime {
			t.Fatal("EndpointServerTime 進了 observedEndpoints()，但它不經過 do()")
		}
	}
	body := scrapeAmegoMetrics(t)
	if strings.Contains(body, `invoice_amego_response_code_total{code="0",endpoint="`+EndpointServerTime+`"}`) {
		t.Error("/json/time 有預先展開的序列，但沒有任何東西會觀測它 —— 恆為 0 會被讀成「一切正常」")
	}
}

// TestCode21ObservesAmegoRateLimited 守 M4 的 source="amego" 那一半。
//
// 它是「我方限速沒守住／對方調緊了」的**唯一**可觀測訊號：AMEGO 未公開 QPS 上限、
// 回應也沒有 Retry-After。source="local"（我方 limiter 擋下）是設計中的正常行為，
// 兩者混在一起會讓告警 ① 在尖峰長期紅著而被忽略 ⇒ 這裡連 biz_error 的 outcome
// 一起釘住，確認 code 21 沒有被記成傳輸面失敗。
func TestCode21ObservesAmegoRateLimited(t *testing.T) {
	before := counterValue(t, `invoice_amego_rate_limited_total{source="amego"}`)
	bizBefore := counterValue(t,
		`invoice_amego_request_duration_seconds_count{endpoint="`+EndpointIssue+`",outcome="biz_error"}`)

	h := newHarness(t, Options{AllowRealInvoice: true, RateLimitQPS: 100}, okJSON(`{"code":21,"msg":"人數過多"}`))
	if _, err := h.client.IssueInvoice(context.Background(), testCred(realInvoiceBAN), sampleIssueRequest()); err == nil {
		t.Fatal("code 21 應回錯")
	}

	if after := counterValue(t, `invoice_amego_rate_limited_total{source="amego"}`); after != before+1 {
		t.Errorf("M4 source=amego = %v, want %v —— 對方觸頂是唯一能觀測到的限速訊號", after, before+1)
	}
	if after := counterValue(t,
		`invoice_amego_request_duration_seconds_count{endpoint="`+EndpointIssue+`",outcome="biz_error"}`); after != bizBefore+1 {
		t.Errorf("code 21 的 outcome 不是 biz_error（%v → %v）—— 往返成功了，記成傳輸失敗會把 SLO-4 算錯",
			bizBefore, after)
	}
}

// TestObserveAuditWriteFailedIncrements 守 M14。
//
// 呼叫端在 internal/client（repoAuditSink.logFail），那一層的測試不看 /metrics
// ⇒ 這裡是「這支計數器真的會動」唯一的機械保證。
func TestObserveAuditWriteFailedIncrements(t *testing.T) {
	before := counterValue(t, "invoice_audit_write_failed_total")
	ObserveAuditWriteFailed()
	if after := counterValue(t, "invoice_audit_write_failed_total"); after != before+1 {
		t.Errorf("invoice_audit_write_failed_total = %v, want %v", after, before+1)
	}
}

func scrapeAmegoMetrics(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics 回 %d", rec.Code)
	}
	return rec.Body.String()
}

// counterValue 從文字格式的 /metrics 取一條序列的值。找不到回 0（＝尚未展開）。
func counterValue(t *testing.T, series string) float64 {
	t.Helper()
	for _, line := range strings.Split(scrapeAmegoMetrics(t), "\n") {
		rest, ok := strings.CutPrefix(line, series+" ")
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil {
			t.Fatalf("解析 %q 的值失敗：%v", series, err)
		}
		return v
	}
	return 0
}
