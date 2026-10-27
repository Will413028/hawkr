package postgres

import (
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"go.uber.org/fx"
)

var Module = fx.Module("repository",
	fx.Provide(fx.Annotate(NewSellerRepository, fx.As(new(domain.SellerRepository)))),
	fx.Provide(fx.Annotate(NewCertificationRepository, fx.As(new(domain.CertificationRepository)))),
	fx.Provide(fx.Annotate(NewPayuniRepository, fx.As(new(domain.PayuniRepository)))),
	fx.Provide(fx.Annotate(NewPayuniMerchantProfileUpdateRepository, fx.As(new(domain.PayuniMerchantProfileUpdateRepository)))),
	fx.Provide(fx.Annotate(NewStoreVisitorRepository, fx.As(new(domain.StoreVisitorRepository)))),
	fx.Provide(fx.Annotate(NewProductViewRepository, fx.As(new(domain.ProductViewRepository)))),
	// B-169 governance
	fx.Provide(fx.Annotate(NewSellerViolationReportRepository, fx.As(new(domain.SellerViolationReportRepository)))),
	fx.Provide(fx.Annotate(NewFoodSafetyReportRepository, fx.As(new(domain.FoodSafetyReportRepository)))),
	fx.Provide(fx.Annotate(NewSellerViolationRepository, fx.As(new(domain.SellerViolationRepository)))),
	fx.Provide(fx.Annotate(NewSellerNotificationLogRepository, fx.As(new(domain.SellerNotificationLogRepository)))),
	fx.Provide(fx.Annotate(NewSellerDeletedOutboxRepository, fx.As(new(domain.SellerDeletedOutboxRepository)))),
	fx.Provide(fx.Annotate(NewMerchantVerificationOutboxRepository, fx.As(new(domain.MerchantVerificationOutboxRepository)))),
	fx.Provide(fx.Annotate(NewPayuniMerchantProfileOutboxRepository, fx.As(new(domain.PayuniMerchantProfileOutboxRepository)))),
	// Admin manual seller suspension (32 §13 / 44 §13.2)
	fx.Provide(fx.Annotate(NewSellerSuspensionRepository, fx.As(new(domain.SellerSuspensionRepository)))),
	// Admin seller membership plan assignment (子系統 1)
	fx.Provide(NewSellerMembershipRepository),
	fx.Provide(NewSellerProspectRepository),
)
