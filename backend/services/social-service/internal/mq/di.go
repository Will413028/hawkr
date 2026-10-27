package mq

import (
	"context"

	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("mq",
	fx.Provide(providePublisher),
)

func providePublisher(cfg *config.Config, lc fx.Lifecycle) usecase.ContentEventPublisher {
	if cfg.RabbitMQ.URL == "" {
		zap.L().Warn("RabbitMQ URL not configured, content events disabled")
		return nil
	}

	pub, err := NewPublisher(cfg.RabbitMQ.URL)
	if err != nil {
		zap.L().Warn("failed to create ContentEventPublisher, content events disabled", zap.Error(err))
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
