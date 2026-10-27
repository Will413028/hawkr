package infrastructure

import (
	"context"

	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// NewRedisClient creates the Redis client used by im-service for the
// B-135 blocklist read path (pkg/blocklist.IsBlocked is fail-open so the
// service still starts if Redis is unavailable).
func NewRedisClient(lc fx.Lifecycle, cfg *config.Config, logger *zap.Logger) *redis.Client {
	client := redis.NewClient(pkgredis.OptionsFromEnv())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := client.Ping(ctx).Err(); err != nil {
				logger.Warn("Redis connection failed; blocklist will fail-open", zap.Error(err))
				return nil
			}
			logger.Info("Redis connection established", zap.String("addr", client.Options().Addr))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})
	return client
}
