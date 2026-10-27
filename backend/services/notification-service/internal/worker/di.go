package worker

import (
	"context"
	"runtime/debug"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mail"
	"github.com/Will413028/hawkr/go-pkg/mail/mailfactory"
	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/notification-service/internal/config"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
)

// Module provides the worker layer dependencies.
var Module = fx.Module("worker",
	fx.Provide(NewTemplateCache),
	fx.Invoke(startConsumer),
	fx.Invoke(startCacheRefresher),
	fx.Invoke(startCleanupJob),
)

func startConsumer(
	lc fx.Lifecycle,
	cfg *config.Config,
	sendPush *usecase.SendPushUseCase,
	inboxUC *usecase.NotificationInboxUseCase,
	sellerClient usecase.SellerClient,
	cache *TemplateCache,
	meterProvider *sdkmetric.MeterProvider,
	logger *zap.Logger,
) {
	ctx, cancel := context.WithCancel(context.Background())

	// Load templates from DB before building subscriptions.
	// Use background context so shutdown can't abort the initial load.
	if _, err := cache.LoadFromDB(context.Background()); err != nil {
		logger.Error("[worker] failed to load template cache at startup", zap.Error(err))
	}

	mailSender, err := mailfactory.New(mailfactory.Config{
		Provider:          cfg.Mail.Provider,
		APIKey:            cfg.Mail.ResendAPIKey,
		RecipientPolicy:   cfg.Mail.RecipientPolicy,
		AllowedRecipients: cfg.Mail.AllowedRecipients,
		RedirectTo:        cfg.Mail.RedirectTo,
	}, logger)
	if err != nil {
		logger.Fatal("[worker] failed to build mail sender", zap.Error(err))
	}
	vendorInvite := vendorInviteDeps{
		Invites:  sellerClient,
		Mail:     mailSender,
		From:     mail.Address{Email: cfg.Mail.From, Name: "Hawkr"},
		BaseURL:  cfg.VendorPortal.BaseURL,
		MarkSent: sellerClient.MarkInviteSecretDelivered,
		Logger:   logger,
	}

	subs := buildDynamicSubscriptions(cache, sendPush, inboxUC, cfg.MediaBaseURL, logger, vendorInvite)
	// buyer.deleted, account.deleted, and seller.deleted are not template-driven:
	// append static subscriptions that scrub chat-notification inbox rows for the
	// closed buyer stint / deleted account / closed seller stint respectively
	// (個資法), plus soft-delete the retired buyer stint's / seller incarnation's
	// own inbox rows (44 號設計書 §10 D6 for buyer; issue #2707 for seller). Split
	// per buyer store-stint rekey W2/W3 D6 — see account_deleted.go's doc comments
	// for why buyer.deleted and account.deleted are both needed (buyer.deleted
	// misses seller-only-account deletes; account.deleted misses buyer-role-only
	// closures on a dual-role account). seller.deleted is the third leg covering a
	// seller role closing alone on a multi-role account.
	subs = append(subs, buyerDeletedSubscription(inboxUC, logger))
	subs = append(subs, accountDeletedSubscription(inboxUC, logger))
	subs = append(subs, sellerDeletedSubscription(inboxUC, logger))
	consumer := mq.NewResilientConsumer(cfg.RabbitMQ.URL, subs, mq.WithOTelMetrics(meterProvider))

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go consumer.Run(ctx)
			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			return nil
		},
	})
}

const cacheRefreshInterval = 5 * time.Minute

// newCacheRefreshScheduler refreshes the template cache on a fixed cadence,
// isolated behind a panic guard so a bug in any single refresh can NEVER
// bring down the whole notification-service process. MQ subscriptions are
// NOT re-created on refresh — only the in-memory lookup map is updated.
// No WithInitialDelay: preserves the existing behavior of waiting a full
// interval before the first refresh (the consumer's own startup already
// does one synchronous LoadFromDB call before subscriptions are built —
// see startConsumer).
func newCacheRefreshScheduler(interval time.Duration, cache *TemplateCache, logger *zap.Logger) *outbox.Scheduler {
	return outbox.NewScheduler(interval,
		func(ctx context.Context) error {
			n, err := cache.LoadFromDB(ctx)
			if err != nil {
				return err
			}
			logger.Info("[worker] template cache refreshed", zap.Int("count", n))
			return nil
		},
		func(_ context.Context, r any) {
			logger.Error("[worker] template cache refresh panicked",
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		func(_ context.Context, err error) {
			logger.Error("[worker] template cache refresh failed", zap.Error(err))
		},
	)
}

// startCacheRefresher starts the background scheduler that refreshes the
// template cache every 5 minutes.
func startCacheRefresher(lc fx.Lifecycle, cache *TemplateCache, logger *zap.Logger) {
	s := newCacheRefreshScheduler(cacheRefreshInterval, cache, logger)
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.Stop(); return nil },
	})
}
