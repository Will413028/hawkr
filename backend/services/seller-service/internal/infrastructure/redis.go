package infrastructure

import (
	"context"

	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/seller-service/internal/config"
)

// NewRedisClient B-169: Redis client for public violation cache (D8).
// Fail-graceful at startup — cache miss / Redis 不可用都 fail-open 直接打 DB。
func NewRedisClient(lc fx.Lifecycle, cfg *config.Config, logger *zap.Logger) *redis.Client {
	client := redis.NewClient(pkgredis.OptionsFromEnv())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := client.Ping(ctx).Err(); err != nil {
				logger.Warn("Redis connection failed; public violation cache will fail-open", zap.Error(err))
				return nil
			}
			logger.Info("Redis connection established", zap.String("addr", client.Options().Addr))
			return nil
		},
		OnStop: func(_ context.Context) error {
			return client.Close()
		},
	})
	return client
}
