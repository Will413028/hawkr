package usecase

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// provideSellerDeletionRepository adapts domain.SellerRepository to SellerDeletionRepository.
// SellerDeletionRepository is a structural subset, so the assignment is direct.
func provideSellerDeletionRepository(r domain.SellerRepository) SellerDeletionRepository {
	return r
}

// provideSellerDeletionAuditRepository adapts the postgres repo to the usecase interface.
func provideSellerDeletionAuditRepository(pool *pgxpool.Pool) SellerDeletionAuditRepository {
	return postgres.NewSellerDeletionAuditRepository(pool)
}

// governancePublisherAdapter wraps pkg/mq.Publisher into SellerNotificationPublisher.
// Falls back to nil-publisher (no-op) when RabbitMQ unreachable; usecase fire-and-forget logs failure.
type governancePublisherAdapter struct {
	pub *pkgmq.Publisher
}

func (a *governancePublisherAdapter) Publish(ctx context.Context, exchange, routingKey string, event interface{}) error {
	if a == nil || a.pub == nil {
		return nil
	}
	return a.pub.Publish(ctx, exchange, routingKey, event)
}

func provideGovernancePublisher(cfg *config.Config, logger *zap.Logger) SellerNotificationPublisher {
	pub, err := pkgmq.NewPublisher(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Warn("[B-169] governance publisher init failed; events will no-op", zap.Error(err))
		return &governancePublisherAdapter{pub: nil}
	}
	return &governancePublisherAdapter{pub: pub}
}

// sellerProfileEventPublisherAdapter wraps pkg/mq.Publisher into SellerProfileEventPublisher (#1426).
// Falls back to nil-publisher (no-op) when RabbitMQ unreachable; usecase emit is fire-and-forget best-effort.
type sellerProfileEventPublisherAdapter struct {
	pub *pkgmq.Publisher
}

func (a *sellerProfileEventPublisherAdapter) PublishSellerProfileChanged(ctx context.Context, storeID int64) error {
	if a == nil || a.pub == nil {
		return nil
	}
	return a.pub.PublishSellerProfileChanged(ctx, pkgmq.NewSellerProfileChangedEvent(storeID))
}

func provideSellerProfileEventPublisher(cfg *config.Config, logger *zap.Logger, lc fx.Lifecycle) SellerProfileEventPublisher {
	pub, err := pkgmq.NewPublisher(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Warn("[#1426] seller profile publisher init failed; seller.profile.changed events will no-op", zap.Error(err))
		return &sellerProfileEventPublisherAdapter{pub: nil}
	}
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return pub.Close()
		},
	})
	return &sellerProfileEventPublisherAdapter{pub: pub}
}

// noopMediaAssetDeleter B-169: phone anonymize cron 同步刪 evidence OSS 的 stub。
// Phase 1 暫不實作 batch delete（OSS 物件保留 90 天後 anonymize 視為 acceptable）；
// Phase 2 wire 到 media-service gRPC。
type noopMediaAssetDeleter struct{}

func (noopMediaAssetDeleter) BatchDeleteAssets(_ context.Context, _ []string) error {
	return nil
}

func provideMediaAssetDeleter() MediaAssetDeleter {
	return noopMediaAssetDeleter{}
}

// provideRevalidateClient builds the hawkr-website /api/revalidate caller
// from env vars. Returns a Client even when env is blank — Client.BustAsync
// no-ops when disabled, so local dev doesn't blow up.
func provideRevalidateClient(cfg *config.Config, logger *zap.Logger) *revalidate.Client {
	rcfg := revalidate.Config{
		BaseURL: cfg.WebRevalidate.BaseURL,
		Secret:  cfg.WebRevalidate.Secret,
	}
	// 部署漏設 WEB_REVALIDATE_* 時 client 靜默 no-op → 前端快取永不失效卻無訊號。
	// 啟動時發一次 WARN（不 fail startup，本地無此 config 仍可開發）。§9.5
	if !rcfg.Enabled() {
		logger.Warn("revalidate disabled: WEB_REVALIDATE_BASE_URL / WEB_REVALIDATE_SECRET 未設定，前端快取將不失效")
	}
	return revalidate.New(rcfg, logger)
}

func provideVendorPortalRepository(
	pool *pgxpool.Pool, txManager database.TxManager, newID func() int64,
) domain.VendorPortalRepository {
	return postgres.NewVendorPortalRepository(pool, txManager, newID)
}

var Module = fx.Module("usecase",
	fx.Provide(tsid.NewGenerator),
	fx.Provide(domain.NewStoreCodeGenerator),
	fx.Provide(provideRevalidateClient),
	fx.Provide(provideSellerProfileEventPublisher),
	fx.Provide(NewSellerUseCase),
	fx.Provide(NewCertificationUseCase),
	fx.Provide(NewPayuniUseCase),
	// *PayuniUseCase satisfies payuniOutcomeApplier; expose it as the port so
	// Approve/relink can funnel through the shared ApplyOutcome path (§5.7).
	// No cycle: PayuniUseCase depends on certRepo, not CertificationUseCase.
	fx.Provide(func(p *PayuniUseCase) payuniOutcomeApplier { return p }),
	fx.Provide(NewBusinessRegistrationUseCase),
	fx.Provide(NewAddressGeocodeUseCase),
	fx.Provide(NewDeleteSellerUseCase),
	fx.Provide(NewStoreVisitorUseCase),
	fx.Provide(NewRecommendedSellersUseCase),
	fx.Provide(NewProductViewUseCase),
	fx.Provide(NewDeleteSellerAccountUseCase),
	// Adapters for DeleteSellerAccountUseCase dependencies
	fx.Provide(provideSellerDeletionRepository),
	fx.Provide(provideSellerDeletionAuditRepository),
	// domain.SellerDeletedOutboxRepository is provided by the postgres Module.

	// B-169 governance
	// Note: TxManager 由 config.newTxManager 提供（fx 一個 type 只能 provide 一次，
	// 之前在這裡重複註冊會導致 startup panic — 見 #25507857877 crash log）。
	fx.Provide(provideGovernancePublisher),
	fx.Provide(provideMediaAssetDeleter),
	fx.Provide(NewViolationReportUseCase),
	fx.Provide(NewFoodSafetyReportUseCase),
	fx.Provide(NewSellerViolationUseCase),

	// Admin manual seller suspension (32 §13 / 44 §13.2)
	// domain.SellerSuspensionRepository is provided by the postgres Module;
	// usecase.SellerSuspensionRedis is provided by the infrastructure Module.
	fx.Provide(NewSellerSuspensionUseCase),

	// Admin seller membership plan assignment (子系統 1). domain.SellerMembershipRepository
	// is provided by the postgres Module; domain.SellerDeletedOutboxRepository (shared
	// lifecycle outbox) is provided by the postgres Module as well.
	fx.Provide(NewSellerMembershipUseCase),

	// 站外業者主檔的分層識別比對（Wave A）。domain.SellerProspectRepository
	// 由 postgres Module 提供。
	fx.Provide(NewSellerProspectUseCase),

	// Vendor Portal (#2539 E1)
	fx.Provide(provideVendorPortalRepository),
)
