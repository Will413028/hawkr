package mq

import (
	"context"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("mq",
	fx.Provide(provideMerchantEventPublisher),
	fx.Invoke(startConsumers),
)

func provideMerchantEventPublisher(cfg *config.Config, logger *zap.Logger, lc fx.Lifecycle) *MerchantEventPublisher {
	pub, err := NewMerchantEventPublisher(cfg.RabbitMQ.URL, logger)
	if err != nil {
		// Non-fatal: log warning, return nil so the service still starts without MQ
		logger.Warn("[MQ] Failed to create MerchantEventPublisher, verification events disabled", zap.Error(err))
		return nil
	}
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			pub.Close()
			return nil
		},
	})
	return pub
}

func startConsumers(lc fx.Lifecycle, cfg *config.Config, payuniUC *usecase.PayuniUseCase, sellerRepo domain.SellerRepository, productViewUC *usecase.ProductViewUseCase, mediaClient usecase.MediaClient) {
	ctx, cancel := context.WithCancel(context.Background())

	consumer := pkgmq.NewResilientConsumer(cfg.RabbitMQ.URL, []pkgmq.Subscription{
		{
			Exchange:    pkgmq.ExchangeSellerEvents,
			Queue:       "seller.review.result",
			RoutingKeys: []string{"seller.review.result"},
			Handler:     agentNotifyHandler(payuniUC),
			MaxRetries:  3,
		},
		{
			Exchange:    pkgmq.ExchangeReview,
			Queue:       pkgmq.QueueSellerReviewCreated,
			RoutingKeys: []string{pkgmq.RoutingKeyReviewCreated},
			Handler:     reviewRatingHandler(sellerRepo),
			MaxRetries:  3,
		},
		{
			// #1156：admin 隱藏評論後重算賣家 rating（修 staleness）。
			// review.hidden payload 同樣帶 store_id + avg_rating，沿用 reviewRatingHandler。
			Exchange:    pkgmq.ExchangeReview,
			Queue:       pkgmq.QueueSellerReviewHidden,
			RoutingKeys: []string{pkgmq.RoutingKeyReviewHidden},
			Handler:     reviewRatingHandler(sellerRepo),
			MaxRetries:  3,
		},
		{
			Exchange:    pkgmq.ExchangeContent,
			Queue:       "seller.product.viewed",
			RoutingKeys: []string{pkgmq.RoutingKeyProductViewed},
			Handler:     productViewHandler(productViewUC),
			MaxRetries:  3,
		},
		{
			// #1489: release seller logo/cover media references on account teardown.
			Exchange:    pkgmq.ExchangeAccount,
			Queue:       pkgmq.QueueSellerAccountDeleted,
			RoutingKeys: []string{pkgmq.RoutingKeyAccountDeleted},
			Handler:     accountDeletedHandler(mediaClient),
			MaxRetries:  3,
		},
	})

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
