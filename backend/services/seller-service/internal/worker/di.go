package worker

import (
	"context"
	"fmt"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	internalmq "github.com/Will413028/hawkr/seller-service/internal/mq"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("worker",
	fx.Provide(NewPayuniSyncWorker),
	fx.Invoke(startPayuniSync),
	fx.Provide(provideSellerDeletedPublisher),
	fx.Provide(newOutboxMetrics),
	fx.Provide(newSellerDeletedOutboxWorker),
	fx.Invoke(startSellerDeletedOutboxScheduler, NewSellerDeletedOutboxMonitoringScheduler),

	fx.Provide(fx.Annotate(newMerchantVerificationOutboxMetrics, fx.ResultTags(`name:"merchantVerificationOutboxMetrics"`))),
	fx.Provide(fx.Annotate(newMerchantVerificationOutboxWorker, fx.ParamTags(``, ``, `name:"merchantVerificationOutboxMetrics"`))),
	fx.Invoke(startMerchantVerificationOutboxScheduler),
	fx.Invoke(fx.Annotate(NewMerchantVerificationOutboxMonitoringScheduler, fx.ParamTags(``, `name:"merchantVerificationOutboxMetrics"`, ``))),

	fx.Provide(fx.Annotate(newPayuniMerchantProfileOutboxMetrics, fx.ResultTags(`name:"payuniMerchantProfileOutboxMetrics"`))),
	fx.Provide(fx.Annotate(newPayuniMerchantProfileOutboxWorker, fx.ParamTags(``, ``, ``, `name:"payuniMerchantProfileOutboxMetrics"`))),
	fx.Invoke(startPayuniMerchantProfileOutboxScheduler),
	fx.Invoke(fx.Annotate(NewPayuniMerchantProfileOutboxMonitoringScheduler, fx.ParamTags(``, `name:"payuniMerchantProfileOutboxMetrics"`, ``))),
)

// newOutboxMetrics is provided once and shared by both the Worker (terminal
// failure counter) and Task 6's MonitoringScheduler (backlog/staleness
// gauges) — sharing one instance is required: outbox.NewMetrics("seller_deleted")
// twice would attempt duplicate Prometheus registration and panic at startup.
func newOutboxMetrics() *outbox.Metrics {
	return outbox.NewMetrics("seller_deleted")
}

// provideSellerDeletedPublisher creates a *pkgmq.Publisher for the outbox worker.
// Returns nil if MQ is unreachable; the worker skips the entire tick gracefully
// in that case (see SellerDeletedOutboxWorker.RunOnce).
func provideSellerDeletedPublisher(cfg *config.Config, logger *zap.Logger, lc fx.Lifecycle) *pkgmq.Publisher {
	if cfg.RabbitMQ.URL == "" {
		logger.Warn("[seller-deleted-outbox] RabbitMQ URL not configured, outbox worker will skip rows")
		return nil
	}
	pub, err := pkgmq.NewPublisher(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Warn("[seller-deleted-outbox] failed to create publisher, outbox worker will skip rows",
			zap.Error(err))
		return nil
	}
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return pub.Close()
		},
	})
	return pub
}

// newSellerDeletedOutboxWorker bridges fx-provided deps to NewSellerDeletedOutboxWorker.
// *pkgmq.Publisher satisfies the sellerDeletedPublisher interface;
// usecase.AccountRoleRemover / usecase.AccountRoleGranter (both already
// registered in internal/client/di.go) satisfy the role-sync seams;
// domain.SellerRepository satisfies currentStintLookup, which is what makes the
// onboarding grant read the account's live stint at apply time rather than the
// surrogate snapshotted into the row; usecase.BuyerClient (also already
// registered there) lets the worker's own role-removal retry query the buyer
// side itself, matching the primary synchronous delete path instead of
// assuming UNKNOWN.
func newSellerDeletedOutboxWorker(
	repo domain.SellerDeletedOutboxRepository,
	pub *pkgmq.Publisher,
	remover usecase.AccountRoleRemover,
	granter usecase.AccountRoleGranter,
	sellers domain.SellerRepository,
	buyers usecase.BuyerClient,
	metrics *outbox.Metrics,
) *SellerDeletedOutboxWorker {
	if pub == nil {
		return NewSellerDeletedOutboxWorker(repo, nil, remover, granter, sellers, buyers, metrics)
	}
	return NewSellerDeletedOutboxWorker(repo, pub, remover, granter, sellers, buyers, metrics)
}

// newMerchantVerificationOutboxMetrics is a distinct *outbox.Metrics instance
// from newOutboxMetrics's "seller_deleted" — two Metrics with the SAME
// outboxName in one process would panic on duplicate Prometheus registration
// (see pkg/outbox.NewMetrics's doc comment), and this is a different table.
func newMerchantVerificationOutboxMetrics() *outbox.Metrics {
	return outbox.NewMetrics("merchant_verification")
}

// newMerchantVerificationOutboxWorker bridges fx-provided deps.
// *internalmq.MerchantEventPublisher may be nil (MQ unreachable at startup);
// the nil check happens here, before it is handed to the worker as an
// interface value, to avoid the typed-nil-interface trap (same pattern as
// newSellerDeletedOutboxWorker above).
func newMerchantVerificationOutboxWorker(
	repo domain.MerchantVerificationOutboxRepository,
	pub *internalmq.MerchantEventPublisher,
	metrics *outbox.Metrics,
) *MerchantVerificationOutboxWorker {
	if pub == nil {
		return NewMerchantVerificationOutboxWorker(repo, nil, metrics)
	}
	return NewMerchantVerificationOutboxWorker(repo, pub, metrics)
}

// newPayuniMerchantProfileOutboxMetrics 又是一個獨立的 *outbox.Metrics——
// 同名字在同一個 process 註冊兩次會 panic（見 pkg/outbox.NewMetrics），而這是
// 第三張表。
func newPayuniMerchantProfileOutboxMetrics() *outbox.Metrics {
	return outbox.NewMetrics("payuni_merchant_profile")
}

// newPayuniMerchantProfileOutboxWorker 把 fx 提供的依賴接上 worker。
//
// *usecase.PayuniUseCase 滿足 merchantProfileUpdater；版本查詢包成 closure
// 而不是直接傳 repo，是為了讓 worker 只依賴「讀一個 int64」這件事，測試不必
// 攤開整個 domain.PayuniRepository。
func newPayuniMerchantProfileOutboxWorker(
	repo domain.PayuniMerchantProfileOutboxRepository,
	uc *usecase.PayuniUseCase,
	payuniRepo domain.PayuniRepository,
	metrics *outbox.Metrics,
) *PayuniMerchantProfileOutboxWorker {
	version := func(ctx context.Context, storeID int64) (int64, error) {
		info, err := payuniRepo.GetByStoreID(ctx, storeID)
		if err != nil {
			return 0, err
		}
		if info == nil {
			// 投遞當下註冊還在，現在查不到——不能拿 0 當版本硬送，那會被
			// UpdateMerchantProfile 的 CAS 擋掉卻先佔掉一組 idempotency key。
			return 0, fmt.Errorf("payuni registration not found for store_id %d", storeID)
		}
		return info.MerchantProfileVersion, nil
	}
	return NewPayuniMerchantProfileOutboxWorker(repo, uc, version, metrics)
}

func startPayuniSync(lc fx.Lifecycle, w *PayuniSyncWorker) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			w.Start(context.Background())
			return nil
		},
		OnStop: func(_ context.Context) error {
			w.Stop()
			return nil
		},
	})
}
