package worker

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// INV.gov —— 治理到期掃描 ＋ 跨期關帳窗口（§7.5.3、§8.2.3 M20）
//
// 排程：**每日 UTC 16:00**（＝台灣次日 00:00，§7.5.3 逐字）。走
// pkg/outbox.NewDailyScheduler——它就是為這種每日一次的工作存在的，
// 而 INV.trk 用不了它是因為期初要加頻，這支沒有那個需求。
// ---------------------------------------------------------------------------

// govDailyHourUTC／govDailyMinuteUTC 是每日排程時刻（§7.5.3：UTC 16:00 ＝ 台灣次日 00:00）。
const (
	govDailyHourUTC   = 16
	govDailyMinuteUTC = 0

	// 啟動補跑的重試（2026-09-04）。pod 剛起的那幾秒打不出去（NetworkPolicy 規則
	// 對新 pod 有套用延遲），補跑一次就放棄的話鏡像會空到隔天。
	// 5 × 30s ＝ 2.5 分鐘，涵蓋一般 rolling 的窗口；再不行就是真的壞了，
	// 那時 InvoiceBusinessDayMirrorEmpty 會在 30 分鐘後叫。
	govCatchUpAttempts = 5
	govCatchUpInterval = 30 * time.Second
)

// crossPeriodPending 是 M20（§8.2.3）。
//
// 🔴 三個 window 全部預先註冊成 0：告警 ⑯／⑰ 的 selector 對**不存在的序列**永遠
// 不會 fire，而這支指標先前完全沒有生產者。label 值域三值逐字取自 §8.2.3 M20。
// **不加 store_id**（label 基數紅線）。
var crossPeriodPending = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "invoice_cross_period_pending",
	Help: "Cross-period VOID/ALLOWANCE operations still open, by close-window state (58 M20)",
}, []string{"window"})

// approvalRecheckDue 是「屆期待複審」的家數（#3261 ③）。
//
// 🔴 這支 gauge 在 2026-09-08 之前**不存在**：CountApprovalRecheckDue 的 SQL 註解寫著
// 「INV.gov 每日推 invoice_seller_approval_recheck_due 指標」，而那個 metric 名在全 repo
// 零命中、那支查詢零呼叫端。而 approval_valid_until 屆滿**不改狀態**（刻意設計）
// ⇒ 除了這支指標之外沒有第二個訊號來源。
var approvalRecheckDue = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "invoice_seller_approval_recheck_due",
	Help: "Seller tax profiles whose approval_valid_until has passed and need re-review (58 §5.2.3)",
})

// registerGovernanceMetrics 讓三個 window 的序列在第一次掃描之前就存在。
func registerGovernanceMetrics() {
	for _, w := range []usecase.CrossPeriodWindow{usecase.WindowOpen, usecase.WindowClosing, usecase.WindowClosed} {
		crossPeriodPending.WithLabelValues(string(w)).Set(0)
	}
	// 同理：沒有這一行，第一次掃描之前該序列不存在，而「不存在」與「0」在 Prometheus
	// 上是兩件事——absent() 才抓得到前者，而告警多半寫的是 > 0。
	approvalRecheckDue.Set(0)
}

// governanceObserver 把 usecase 算出來的值寫進 gauge。
type governanceObserver struct{}

func (governanceObserver) ObserveCrossPeriodPending(window usecase.CrossPeriodWindow, count int64) {
	crossPeriodPending.WithLabelValues(string(window)).Set(float64(count))
}

func (governanceObserver) ObserveApprovalRecheckDue(count int64) {
	approvalRecheckDue.Set(float64(count))
}

// govRunner 是 INV.gov 的 usecase 面。
type govRunner interface {
	RunOnce(ctx context.Context) error
}

// governanceWorker 把 usecase 接上每日排程器。
type governanceWorker struct{ uc govRunner }

// RunDaily 是 pkg/outbox.NewDailyScheduler 要的 func(ctx) 形狀。
//
// 錯誤在這裡吞掉並不是疏忽：DailyScheduler 的簽章不收 error，而 usecase 內部
// 每一個失敗都已經各自 log 過（expire 與 count 互不阻擋）。
func (w *governanceWorker) RunDaily(ctx context.Context) { _ = w.uc.RunOnce(ctx) }

// RunOnce 讓啟動補跑拿得到錯誤（RunDaily 的簽章不收 error，那是 DailyScheduler 的要求）。
//
// 兩者不能合併：排程器那條路徑吞掉 error 是刻意的（usecase 內部每個失敗都各自 log 過，
// 而 tick 沒有地方回報），但補跑要**依錯誤決定要不要重試**。
func (w *governanceWorker) RunOnce(ctx context.Context) error { return w.uc.RunOnce(ctx) }
