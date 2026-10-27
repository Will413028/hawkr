package procurement

import (
	"context"
	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/client/buyer"
	"github.com/Will413028/hawkr/ai-service/internal/config"
	grpcLayer "github.com/Will413028/hawkr/ai-service/internal/grpc"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/commerce"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/handler"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/orderprovisioning"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/publicationoutbox"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	rootusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

var Module = fx.Module("procurement",
	fx.Provide(tsid.NewGenerator),

	fx.Provide(newProcurementPool),
	fx.Provide(func(pool *pgxpool.Pool) database.TxManager { return database.NewTxManager(pool) }),

	repository.Module,
	usecase.Module,
	cropidentity.Module,

	fx.Provide(llmprovider.NewProcurement),
	fx.Provide(usecase.NewSlotExtractor),
	fx.Provide(usecase.NewStepNarrator),

	fx.Provide(usecase.NewProcurementGraphRunner),

	fx.Provide(usecase.NewSurfaceRealizer),

	fx.Provide(func(r *repository.Repo) usecase.RevisionStore { return r }),

	fx.Provide(func(r *repository.Repo) handler.SnapshotReader { return r }),

	fx.Provide(func(r *repository.Repo) usecase.AnswerStore { return r }),

	fx.Provide(func(r *repository.Repo) usecase.RewindStore { return r }),

	fx.Provide(func(r *repository.Repo) usecase.ResumeRunStore { return r }),
	fx.Provide(usecase.NewRunDispatcher),

	fx.Provide(func(r *repository.Repo) grpcLayer.RunStore { return r }),

	fx.Provide(func(r *repository.Repo) grpcLayer.InviteSellerViewRepository { return r }),

	fx.Provide(func(r *repository.Repo) commerce.SellerResponseStore { return r }),
	fx.Provide(func(r *repository.Repo) commerce.BuyerAcceptanceStore { return r }),
	fx.Provide(func(store commerce.SellerResponseStore, tx database.TxManager, now func() time.Time) *commerce.SellerResponseUseCase {
		return commerce.NewSellerResponseUseCase(store, tx, now)
	}),
	fx.Provide(func(store commerce.BuyerAcceptanceStore, tx database.TxManager, seller rootusecase.SellerClient, now func() time.Time) *commerce.BuyerAcceptanceUseCase {
		return commerce.NewBuyerAcceptanceUseCase(store, tx, seller, now)
	}),
	fx.Provide(func(sr *commerce.SellerResponseUseCase, ba *commerce.BuyerAcceptanceUseCase) grpcLayer.SellerCommandDeps {
		return grpcLayer.SellerCommandDeps{SellerResponse: sr, BuyerAcceptance: ba}
	}),

	fx.Provide(func(r *repository.Repo) handler.CandidateReader { return r }),

	fx.Provide(func(tm database.TxManager) commerce.TxRunner { return tm }),

	// ⛔ 對外一律經過 WithTransientRetry：Resolve 的唯一錯誤來源是分類樹查詢抖動
	// （「查無此品項」是成功回傳的結果）。當場重試一次讓大多數抖動對買家無感；連兩次
	// 失敗才交給 crop_gate 的跨輪重試。
	fx.Provide(func(uc *cropidentity.DisambiguationUseCase) cropidentity.Resolver {
		return cropidentity.WithTransientRetry(uc)
	}),

	fx.Provide(func(client rootusecase.ProductClient, seller rootusecase.SellerClient, rankingFacts repository.RankingFactsRepository, now func() time.Time) *usecase.PlatformLaneSearcher {
		return usecase.NewPlatformLaneSearcher(client, seller, rankingFacts, now)
	}),
	fx.Provide(func(searcher rootusecase.CandidateLeadSearcher, categories *cache.CategoryIndex) *usecase.ExternalLaneSearcher {
		return usecase.NewExternalLaneSearcher(searcher, categories)
	}),
	fx.Provide(func() func() time.Time { return func() time.Time { return time.Now().UTC() } }),
	fx.Provide(func(store usecase.CandidateSearchStore, platform *usecase.PlatformLaneSearcher, external *usecase.ExternalLaneSearcher, newID func() int64) *usecase.CandidateSearchUseCase {
		return usecase.NewCandidateSearchUseCase(store, platform, external, newID)
	}),
	fx.Provide(func(store usecase.SelectionStore) *usecase.SelectionUseCase {
		return usecase.NewSelectionUseCase(store)
	}),
	fx.Provide(func(r *repository.Repo) usecase.ConversationStore { return r }),
	fx.Provide(func(store usecase.ConversationStore) *usecase.ConversationUseCase {
		return usecase.NewConversationUseCase(store)
	}),

	fx.Provide(func(r *repository.Repo) usecase.CandidateSearchStore { return r }),
	fx.Provide(func(r *repository.Repo) usecase.SelectionStore { return r }),

	fx.Provide(func(r *repository.Repo) pricing.PricePreviewRepo { return r }),
	fx.Provide(func(r *repository.Repo) pricing.RevisionReader { return r }),
	fx.Provide(pricing.NewPreviewRfqPriceUseCase),

	fx.Provide(func(r *repository.Repo) commerce.ConfirmedRfqSnapshotRepo { return r }),
	fx.Provide(commerce.NewPricePreviewReader),
	fx.Provide(commerce.NewConfirmRfqSnapshotUseCase),

	fx.Provide(func(r *repository.Repo) commerce.PublicationApprovalRepo { return r }),
	fx.Provide(func(r *repository.Repo) commerce.ConfirmedRfqSnapshotByIDReader { return r }),
	fx.Provide(func(r *repository.Repo) commerce.PricePreviewByIDReader { return r }),
	fx.Provide(func(r *repository.Repo) commerce.CandidateSetReader { return r }),
	fx.Provide(commerce.NewPublicationPreviewUseCase),

	fx.Provide(func(r *repository.Repo) commerce.PublicationCommandRepo { return r }),
	fx.Provide(func(r *repository.Repo) commerce.PublicationApprovalExecuteRepo { return r }),
	fx.Provide(func(r *repository.Repo) commerce.PublicationRunWriter { return r }),
	fx.Provide(func(r *repository.Repo) commerce.PublicationInviteRepo { return r }),
	fx.Provide(commerce.NewPublicationExecuteUseCase),

	publicationoutbox.Module,
	fx.Provide(func(r *repository.Repo) publicationoutbox.InviteSentMarker { return r }),

	// Wave B：acceptance → order shell 的投遞（doc 64 §6.4）。RfqOrderCreator 的綁定在
	// internal/client/order 的 Module——那一端才持有 gRPC 連線。
	orderprovisioning.Module,
	fx.Provide(func(r *repository.Repo) orderprovisioning.AcceptanceStore { return r }),

	fx.Provide(func(r *repository.Repo) usecase.PricePreviewChecker { return r }),

	fx.Provide(func(r *repository.Repo) usecase.RfqConfirmationReader { return r }),

	fx.Provide(func(r *repository.Repo) usecase.QuotaReserver { return r }),

	fx.Provide(func(c buyer.Client) usecase.EntitlementClient { return entitlementAdapter{client: c} }),
)

type entitlementAdapter struct {
	client buyer.Client
}

func (a entitlementAdapter) GetBuyerEntitlement(ctx context.Context, buyerID identity.BuyerID) (usecase.Entitlement, error) {
	e, err := a.client.GetBuyerEntitlement(ctx, buyerID)
	if err != nil {
		return usecase.Entitlement{}, err
	}
	return usecase.Entitlement{
		PlanCode:           e.PlanCode,
		MonthlyAnswerLimit: e.MonthlyAnswerLimit,
		PeriodStart:        e.PeriodStart,
		PeriodEnd:          e.PeriodEnd,
	}, nil
}

func newProcurementPool(lc fx.Lifecycle, cfg *config.Config) (*pgxpool.Pool, error) {
	dsn := database.BuildDSN(database.DBConfig{
		Host: cfg.DBHost, Port: cfg.DBPort, User: cfg.DBUser,
		Password: cfg.DBPassword, Name: cfg.DBName, SSLMode: cfg.DBSSLMode,
	})
	pool, err := database.NewPool(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})
	return pool, nil
}
