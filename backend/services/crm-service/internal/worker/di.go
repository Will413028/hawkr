package worker

import (
	"context"

	"go.uber.org/fx"

	"github.com/Will413028/hawkr/crm-service/internal/config"
	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/mq"
)

func startWorker(lc fx.Lifecycle, cfg *config.Config, uc *usecase.CrmUseCase) {
	ctx, cancel := context.WithCancel(context.Background())

	consumer := mq.NewResilientConsumer(cfg.RabbitMQURL, []mq.Subscription{
		{
			Exchange:    mq.ExchangeOrder,
			Queue:       "crm.order.completed",
			RoutingKeys: []string{mq.RoutingKeyOrderCompleted},
			Handler:     orderCompletedHandler(uc),
			MaxRetries:  3,
		},
		{
			Exchange:    mq.ExchangeAftersales,
			Queue:       "crm.aftersales.refund.completed",
			RoutingKeys: []string{mq.RoutingKeyAftersalesRefundCompleted},
			Handler:     refundCompletedHandler(uc),
			MaxRetries:  3,
		},
		{
			// #2847：買家註銷後抹除 crm_customers 的個資快取。
			// 🔴 這個 queue 必須同步登記在 infra/mq/topology/topology.yaml——
			// 服務端已 passive declare，漏登記不會報錯，只會靜默不訂閱。
			Exchange:    mq.ExchangeBuyer,
			Queue:       "crm.buyer.deleted",
			RoutingKeys: []string{mq.RoutingKeyBuyerDeleted},
			Handler:     buyerDeletedHandler(uc),
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

// Module provides the worker layer dependencies.
var Module = fx.Module("worker",
	fx.Invoke(startWorker),
)
