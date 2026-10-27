package postgres

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/Will413028/hawkr/notification-service/internal/config"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module provides all Postgres repository implementations.
var Module = fx.Module("repository.postgres",
	fx.Provide(NewDBPool),
	fx.Provide(tsid.NewGenerator),
	fx.Provide(
		fx.Annotate(NewDeviceTokenRepository, fx.As(new(domain.DeviceTokenRepository))),
		fx.Annotate(NewPushNotificationRepository, fx.As(new(domain.PushNotificationRepository))),
		fx.Annotate(NewNotificationPreferenceRepository, fx.As(new(domain.NotificationPreferenceRepository))),
		fx.Annotate(NewNotificationTemplateRepository, fx.As(new(domain.NotificationTemplateRepository))),
		fx.Annotate(NewNotificationInboxRepository, fx.As(new(domain.NotificationInboxRepository))),
	),
)

func NewDBPool(lc fx.Lifecycle, cfg *config.Config, logger *zap.Logger) (*pgxpool.Pool, error) {
	pool, err := database.NewPool(context.Background(), cfg.Database.DSN())
	if err != nil {
		return nil, err
	}

	logger.Info("Database connection established")

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing database connection")
			pool.Close()
			return nil
		},
	})

	return pool, nil
}
