package mq

import (
	"context"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module provides the event publisher for im-service domain events.
var Module = fx.Module("mq",
	fx.Provide(provideChatEventPublisher),
)

func provideChatEventPublisher(cfg *config.Config, logger *zap.Logger, lc fx.Lifecycle) usecase.ChatEventPublisher {
	pub, err := pkgmq.NewPublisher(cfg.RabbitMQURL)
	if err != nil {
		// Non-fatal: chat offline push is best-effort. The service still serves
		// real-time WebSocket delivery and persistence without the publisher.
		logger.Warn("[MQ] failed to create publisher, chat push events disabled", zap.Error(err))
		return nil
	}
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return pub.Close()
		},
	})
	return &chatEventPublisher{pub: pub, logger: logger}
}
