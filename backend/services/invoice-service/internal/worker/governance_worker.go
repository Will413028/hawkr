package worker

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	govDailyHourUTC   = 16
	govDailyMinuteUTC = 0

	govCatchUpAttempts = 5
	govCatchUpInterval = 30 * time.Second
)

var crossPeriodPending = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "invoice_cross_period_pending",
	Help: "Cross-period VOID/ALLOWANCE operations still open, by close-window state (58 M20)",
}, []string{"window"})

var approvalRecheckDue = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "invoice_seller_approval_recheck_due",
	Help: "Seller tax profiles whose approval_valid_until has passed and need re-review (58 §5.2.3)",
})

func registerGovernanceMetrics() {
	for _, w := range []usecase.CrossPeriodWindow{usecase.WindowOpen, usecase.WindowClosing, usecase.WindowClosed} {
		crossPeriodPending.WithLabelValues(string(w)).Set(0)
	}
	approvalRecheckDue.Set(0)
}

type governanceObserver struct{}

func (governanceObserver) ObserveCrossPeriodPending(window usecase.CrossPeriodWindow, count int64) {
	crossPeriodPending.WithLabelValues(string(window)).Set(float64(count))
}

func (governanceObserver) ObserveApprovalRecheckDue(count int64) {
	approvalRecheckDue.Set(float64(count))
}

type govRunner interface {
	RunOnce(ctx context.Context) error
}

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
