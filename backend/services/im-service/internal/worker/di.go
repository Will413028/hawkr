package worker

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/im-service/internal/client/seller"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
)

// Module wires the im-service MQ consumers: buyer.deleted + account.deleted +
// seller.deleted (chat PII redaction, split per buyer-store-stint rekey W2/W3 D6 — see
// buyer_deleted.go / account_deleted.go / seller_deleted.go doc comments) and seller.suspended
// (force-disconnect existing WS, 14 §6.5.4).
var Module = fx.Module("worker",
	fx.Invoke(startConsumer),
)

func startConsumer(lc fx.Lifecycle, cfg *config.Config, msgUC *usecase.MessageUseCase, hub domain.Hub, sellerClient *seller.Client, logger *zap.Logger) {
	if cfg.RabbitMQURL == "" {
		logger.Warn("RabbitMQ URL not configured, skipping buyer.deleted / account.deleted / seller.deleted / seller.suspended consumers")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	consumer := mq.NewResilientConsumer(cfg.RabbitMQURL, []mq.Subscription{
		{
			Exchange:    mq.ExchangeBuyer,
			Queue:       mq.QueueIMBuyerDeleted,
			RoutingKeys: []string{mq.RoutingKeyBuyerDeleted},
			Handler:     newBuyerDeletedHandler(msgUC, logger),
			MaxRetries:  3,
		},
		{
			Exchange:    mq.ExchangeAccount,
			Queue:       mq.QueueIMAccountDeleted,
			RoutingKeys: []string{mq.RoutingKeyAccountDeleted},
			Handler:     newAccountDeletedHandler(msgUC, logger),
			MaxRetries:  3,
		},
		{
			Exchange:    mq.ExchangeSeller,
			Queue:       mq.QueueIMSellerDeleted,
			RoutingKeys: []string{mq.RoutingKeySellerDeleted},
			Handler:     newSellerDeletedHandler(msgUC, logger),
			MaxRetries:  3,
		},
		{
			Exchange:    mq.ExchangeSeller,
			Queue:       mq.QueueIMSellerSuspended,
			RoutingKeys: []string{mq.RoutingKeySellerSuspended},
			Handler:     newSellerSuspendedHandler(hub, sellerClient, logger),
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
