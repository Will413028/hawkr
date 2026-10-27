package worker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// expectedMetricNames 是「起服務後 /metrics 上必須看得到」的清單。
//
// 🔴 為什麼是「存在」而不是「值 > 0」：Go 的 counter/histogram 在第一次觀測之前不會
// 出現在 /metrics，而 Prometheus 告警的 selector 對**不存在的序列**永遠不會 fire
// ⇒ business-alerts.yml 裡的 invoice 告警會是永久靜默的擺設。
// **值為 0 才是正確的初始狀態；序列不存在才是缺陷。**
//
// ⚠️ **本清單只列 worker package 自己註冊的那幾支。** M2／M3／M4／M14 的觀測點在
// internal/client/amego（`do()` 的 defer 與 repoAuditSink.logFail），指標本體與這支
// 測試的對應版本一起搬去了那個 package —— 它們原本掛在這裡而觀測點在依賴的另一邊，
// 結果是「有註冊、零觀測」十個月（2026-09-05 第 6 輪稽核）。
// **指標與它唯一的觀測點放在同一個 package**，是那個失效形狀唯一的結構性解法。
var expectedMetricNames = []string{
	"invoice_issue_total",                  // M1
	"invoice_amego_client_rate_limit_qps",  // M5（值來自設定、啟動時設一次，與往返無關）
	"invoice_issue_e2e_duration_seconds",   // M22
	"invoice_worker_tick_duration_seconds", // M25
	"invoice_worker_tick_interval_seconds", // M26
}

// TestMetricsArePreRegisteredOnStartup 起一個 promhttp handler（與 pkg/server 的
// /metrics 走同一個 DefaultGatherer），抓文字輸出，逐名斷言序列存在。
//
// 🔴 先斷言清單長度 > 0：清單寫空的話這支測試會恆綠，而那正是它要防的那種缺陷的
// 同一個形狀。
func TestMetricsArePreRegisteredOnStartup(t *testing.T) {
	if len(expectedMetricNames) == 0 {
		t.Fatal("expectedMetricNames 是空的 —— 這支測試會恆綠")
	}

	// NewMetrics 走 promauto ⇒ 註冊進 DefaultRegisterer。同一個 process 內只能一次，
	// 所以整個 package 共用 testMetrics（TestMain 建立）。
	if testMetrics == nil {
		t.Fatal("testMetrics 未初始化")
	}

	body := scrapeMetrics(t)
	for _, name := range expectedMetricNames {
		// 比對 "# HELP <name>"：只 grep 名字會被別的 metric 的 help 文字誤命中。
		if !strings.Contains(body, "# HELP "+name+" ") {
			t.Errorf("/metrics 缺少 %s —— 對應的告警規則永遠不會 fire", name)
		}
	}
}

// TestMetricsLabelCombinationsArePreExpanded 守「預先展開」而不只是「型別存在」。
//
// 一個 CounterVec 即使註冊了，沒有 WithLabelValues 過的 label 組合仍然不會出現在
// /metrics。告警若按 result="permanent_failed" 過濾，缺那一條就等於沒有告警。
func TestMetricsLabelCombinationsArePreExpanded(t *testing.T) {
	body := scrapeMetrics(t)
	// ⚠️ 只比對到 label 那一層，**不含值**：驗收判準是「序列存在」不是「值為 0」。
	// 同一個 package 的其他測試（尤其 -tags=integration 的端到端那支）會把某些計數器
	// 推到 1，把值寫進斷言會讓這支測試變成「測試執行順序的函式」而不是測預先註冊。
	mustContain := []string{
		`invoice_issue_total{fulfillment_mode="AMEGO_AUTO",result="succeeded"} `,
		`invoice_issue_total{fulfillment_mode="AMEGO_AUTO",result="permanent_failed"} `,
		`invoice_issue_total{fulfillment_mode="LEGACY_MANUAL",result="not_applicable"} `,
	}
	for _, want := range mustContain {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics 缺少預先展開的序列：%s", want)
		}
	}
	// M25／M26 只預先展開本波真的存在的兩支 worker。
	for _, w := range []string{"op", "ob"} {
		if !strings.Contains(body, `invoice_worker_tick_interval_seconds{worker="`+w+`"}`) {
			t.Errorf("/metrics 缺少 worker=%s 的 tick interval", w)
		}
	}
	// 反向：不存在的 worker 不該有序列（那會讓告警 ⑲ 對一支沒在跑的 worker 算出「健康」）。
	if strings.Contains(body, `invoice_worker_tick_duration_seconds_count{worker="pdf"}`) {
		t.Error("worker=pdf 不該被預先註冊 —— 本波沒有這支排程器")
	}
}

// TestIssueResultDomainMatchesSpec 釘住 M1 的 7 個 result 值（§8.2.3 M1）。
// 值域改了而告警規則沒跟著改，症狀是告警查一個不存在的 label 值 ⇒ 永不觸發。
func TestIssueResultDomainMatchesSpec(t *testing.T) {
	want := []string{"succeeded", "permanent_failed", "retry_exhausted", "unknown_terminal",
		"cancelled", "not_applicable", "resolved_externally"}
	if len(invoiceIssueResults) != len(want) {
		t.Fatalf("result 值域有 %d 值，spec 是 %d 值", len(invoiceIssueResults), len(want))
	}
	for _, w := range want {
		if !containsString(invoiceIssueResults, w) {
			t.Errorf("result 值域缺 %q", w)
		}
	}
}

func scrapeMetrics(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics 回 %d", rec.Code)
	}
	return rec.Body.String()
}
