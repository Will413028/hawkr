package mq

import (
	"context"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module 是 ai-service 第一次接 RabbitMQ 連線（Task 9）。Task 12 把
// order.completed 的真實 subscription 接進 startConsumers；Task 13 加上
// seller.membership_changed，兩者共用同一個 rankingFacts repository（Task 11
// 只交付了 repository 層，沒有自己的 MQ consumer——真正的 consumer 是這裡）。
var Module = fx.Module("mq",
	fx.Invoke(startConsumers),
)

func startConsumers(lc fx.Lifecycle, cfg *config.Config, logger *zap.Logger, rankingFacts repository.RankingFactsRepository) {
	if cfg.RabbitMQ.URL == "" {
		logger.Warn("[MQ] RabbitMQ URL not configured, seller_ranking_facts consumers disabled")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())

	consumer := pkgmq.NewResilientConsumer(cfg.RabbitMQ.URL, []pkgmq.Subscription{
		{
			Exchange:    pkgmq.ExchangeOrder,
			Queue:       "ai_service.order.completed",
			RoutingKeys: []string{pkgmq.RoutingKeyOrderCompleted},
			Handler:     orderCompletedHandler(rankingFacts),
			MaxRetries:  3,
		},
		{
			Exchange:    pkgmq.ExchangeSeller,
			Queue:       "ai_service.seller.membership_changed",
			RoutingKeys: []string{pkgmq.RoutingKeySellerMembershipChanged},
			Handler:     sellerMembershipChangedHandler(rankingFacts),
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
