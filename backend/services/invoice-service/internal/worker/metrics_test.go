package worker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var expectedMetricNames = []string{
	"invoice_issue_total",
	"invoice_amego_client_rate_limit_qps",
	"invoice_issue_e2e_duration_seconds",
	"invoice_worker_tick_duration_seconds",
	"invoice_worker_tick_interval_seconds",
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

	if testMetrics == nil {
		t.Fatal("testMetrics 未初始化")
	}

	body := scrapeMetrics(t)
	for _, name := range expectedMetricNames {
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
	for _, w := range []string{"op", "ob"} {
		if !strings.Contains(body, `invoice_worker_tick_interval_seconds{worker="`+w+`"}`) {
			t.Errorf("/metrics 缺少 worker=%s 的 tick interval", w)
		}
	}
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
