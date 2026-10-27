package orderprovisioning

import (
	"context"
	"runtime/debug"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/go-pkg/outbox"
)

const (
	// schedulerInterval 是 Worker.RunOnce 的節奏。1 分鐘，與 publicationoutbox 相同——
	// 買家正盯著「已接受，訂單建立中」這個中間態（doc 64 §6.2），對延遲同樣敏感。
	schedulerInterval = 1 * time.Minute

	// retention 界住一列 done 存活多久才被 cleanup 刪掉。上線前量體低，30 天是保守
	// 預設值，不是調校過的數字（同 publicationoutbox）。
	retention = 30 * 24 * time.Hour
	// cleanupBatchLimit 界住單次 cleanup tick 刪幾列。
	cleanupBatchLimit = 500
)

// Module 只提供本 package 自己的建構子與自己的兩個排程器；兩個介面
// （AcceptanceStore、RfqOrderCreator）的綁定放 procurement/module.go——介面是本層
// 宣告的，若在這裡去 import repository 或 client 綁定它，依賴方向就反了。
var Module = fx.Module("procurement_order_provisioning_outbox",
	fx.Provide(NewRepository),
	// ⚠️ fx.Private 是**必要**不是整齊：publicationoutbox.Module 也 provide 一個
	// *outbox.Metrics，同一個型別在根圖上只能有一個提供者，掛上去會讓整個 fx graph
	// 建不起來（app_validate_test 的 TestFxGraph_Valid 會抓到）。兩條 outbox 線的
	// metrics 本來就該各自私有——它們的 outbox_name 標籤不同，共用一個 instance 反而
	// 會把兩條線的計數混在一起。
	fx.Provide(newMetrics, fx.Private),
	fx.Provide(newWorker),
	fx.Invoke(startSchedulers),
)

// newMetrics 只 provide 一次，由 Worker（終態失敗計數）與 monitoring tick
// （backlog／staleness gauge）共用。共用是**必要**不只是整齊：outbox.NewMetrics 走
// promauto，同一個 outbox_name 註冊兩次會在啟動時 panic。
func newMetrics() *outbox.Metrics {
	return outbox.NewMetrics(outboxName)
}

// newWorker 把設定值解成 worker 需要的形狀。
//
// ⚠️ PROCUREMENT_SETTLEMENT_STORE_ID 未設定（0）**不阻止建構**：見 NewWorker 的註解，
// 它只影響 EXTERNAL lane，PLATFORM lane 照常。這裡只留一行 Warn，讓「dev 還沒填」與
// 「線上掉了設定」在啟動日誌裡看得見。
func newWorker(
	cfg *config.Config, repo *Repository, store AcceptanceStore, orders RfqOrderCreator,
	metrics *outbox.Metrics, logger *zap.Logger,
) *Worker {
	if cfg.ProcurementSettlementStoreID == 0 {
		logger.Warn("rfq-order-provisioning-outbox: PROCUREMENT_SETTLEMENT_STORE_ID not configured, " +
			"EXTERNAL lane rows will terminally fail with CONFIGURATION_MISSING")
	}
	return NewWorker(repo, store, orders, cfg.ProcurementSettlementStoreID, metrics, logger)
}

// startSchedulers 掛上兩個 tick：排空與監測。
//
// 監測那一支是既有 internal/procurement/outbox **沒有**的東西（ADR D3：那份不遷移），
// 本 package 不重蹈：沒有 Metrics.Refresh 就沒有 backlog／staleness 可看，沒有
// CleanupDone 就沒有東西界住 done 列的成長。兩件事都便宜且冪等，所以與排空共用同一個
// 節奏常數，是刻意的簡單而不是疏漏。
func startSchedulers(lc fx.Lifecycle, worker *Worker, repo *Repository, metrics *outbox.Metrics, logger *zap.Logger) {
	drain := outbox.NewScheduler(schedulerInterval, worker.RunOnce,
		func(_ context.Context, r any) {
			logger.Error("rfq-order-provisioning-outbox scheduler panicked",
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		func(_ context.Context, err error) {
			logger.Error("rfq-order-provisioning-outbox RunOnce failed", zap.Error(err))
		},
	)

	monitor := outbox.NewScheduler(schedulerInterval, func(ctx context.Context) error {
		if err := metrics.Refresh(ctx, repo, time.Now()); err != nil {
			return err
		}
		deleted, err := outbox.CleanupDone(ctx, repo, time.Now(), retention, cleanupBatchLimit)
		if err != nil {
			return err
		}
		if deleted > 0 {
			logger.Info("rfq-order-provisioning-outbox cleanup deleted done rows", zap.Int("deleted", deleted))
		}
		return nil
	},
		func(_ context.Context, r any) {
			logger.Error("rfq-order-provisioning-outbox monitoring panicked",
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		func(_ context.Context, err error) {
			logger.Warn("rfq-order-provisioning-outbox monitoring tick failed", zap.Error(err))
		},
	)

	lc.Append(fx.Hook{
		// ⛔ Start 收 context.Background() 而不是 OnStart 拿到的 ctx：後者是 fx 自己的
		// start-timeout context，OnStart 一返回就被 cancel，迴圈會立刻無聲結束
		// （pkg/outbox.Scheduler.Start 的註解）。
		OnStart: func(context.Context) error {
			drain.Start(context.Background())
			monitor.Start(context.Background())
			return nil
		},
		OnStop: func(context.Context) error {
			drain.Stop()
			monitor.Stop()
			return nil
		},
	})
}
