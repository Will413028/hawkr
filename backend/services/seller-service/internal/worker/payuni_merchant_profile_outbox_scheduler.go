package worker

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// payuniMerchantProfileOutboxSchedulerInterval 是 RunOnce 的節奏。
// 5 分鐘而非 merchant_verification 的 1 分鐘：地址同步是賣家看不到的背景動作，
// 沒有人盯著等結果，而且每一次 tick 都可能打對外 API，輪詢太密只是浪費。
const payuniMerchantProfileOutboxSchedulerInterval = 5 * time.Minute

// PayuniMerchantProfileOutboxScheduler 把 worker 掛進 seller-service 主行程，
// 由 fx Lifecycle 以固定節奏跑 RunOnce。多副本安全：pkg/outbox.Worker 在任何
// 副作用之前先用 lease CAS 搶佔（多副本排程搶佔規範 §一 ②）。
type PayuniMerchantProfileOutboxScheduler struct {
	inner *outbox.Scheduler
}

// startPayuniMerchantProfileOutboxScheduler 由 worker.Module invoke。
func startPayuniMerchantProfileOutboxScheduler(w *PayuniMerchantProfileOutboxWorker, lc fx.Lifecycle) *PayuniMerchantProfileOutboxScheduler {
	s := &PayuniMerchantProfileOutboxScheduler{
		inner: outbox.NewScheduler(payuniMerchantProfileOutboxSchedulerInterval, w.RunOnce,
			func(ctx context.Context, r any) {
				logger.Error(ctx, "payuni-merchant-profile-outbox worker panicked",
					zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
			},
			func(ctx context.Context, err error) {
				logger.Warn(ctx, "payuni-merchant-profile-outbox run failed", zap.Error(err))
			},
		),
	}
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.inner.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.inner.Stop(); return nil },
	})
	return s
}
