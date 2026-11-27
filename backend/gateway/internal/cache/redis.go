package cache

import (
	"context"
	"fmt"
	"os"
	"time"

	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisClient 封裝 Redis 連線，提供快取清除功能
var RedisClient *redis.Client

var servicePatterns = map[string][]string{
	"product":  {"prod:detail:*", "prod:list:*", "prod:origins:summary:*", "cat:all"},
	"cart":     {"cart:b:*", "cart:g:*", "seller:meta:*"},
	"coupon":   {"coupon:tpl:*", "coupon:avail:*"},
	"address":  {"addr:sug:*", "addr:val:*"},
	"seller":   {"seller:violations:public:*"},
	"favorite": {"fav:set:*"},
}

// InitRedis 初始化 Redis 連線（可選，連不上不影響 Gateway 正常運作）
func InitRedis() {
	if os.Getenv("REDIS_HOST") == "" {
		zap.S().Warn("REDIS_HOST 未設定，快取清除 API 不可用")
		return
	}

	opts := pkgredis.OptionsFromEnv()
	opts.DialTimeout = 3 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second
	opts.MaxRetries = 3
	opts.MinRetryBackoff = 8 * time.Millisecond
	opts.MaxRetryBackoff = 256 * time.Millisecond

	RedisClient = redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		zap.S().Warnf("Redis 啟動時連線失敗（將於 Redis 可用時自動重連）: %v", err)
		return
	}

	zap.S().Infof("Redis 連線成功: %s", opts.Addr)
}

// FlushService 清除指定服務的快取，回傳清除的 key 數量
func FlushService(ctx context.Context, service string) (int64, error) {
	if RedisClient == nil {
		return 0, fmt.Errorf("redis 未連線")
	}

	patterns, ok := servicePatterns[service]
	if !ok {
		return 0, fmt.Errorf("無效的服務名稱，可用值：product, cart, coupon, address, seller, favorite")
	}

	var total int64
	for _, pattern := range patterns {
		count, err := deleteByPattern(ctx, pattern)
		if err != nil {
			return total, fmt.Errorf("清除 %s 失敗: %w", pattern, err)
		}
		total += count
	}
	return total, nil
}

// FlushAll 清除所有服務的快取，回傳各服務清除的 key 數量
func FlushAll(ctx context.Context) (map[string]int64, error) {
	if RedisClient == nil {
		return nil, fmt.Errorf("redis 未連線")
	}

	result := make(map[string]int64)
	for service := range servicePatterns {
		count, err := FlushService(ctx, service)
		if err != nil {
			return result, err
		}
		result[service] = count
	}
	return result, nil
}

func deleteByPattern(ctx context.Context, pattern string) (int64, error) {
	var cursor uint64
	var total int64

	for {
		keys, nextCursor, err := RedisClient.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return total, err
		}

		if len(keys) > 0 {
			deleted, err := RedisClient.Del(ctx, keys...).Result()
			if err != nil {
				return total, err
			}
			total += deleted
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return total, nil
}
