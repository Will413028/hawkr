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

// servicePatterns 定義各服務的快取 key pattern
var servicePatterns = map[string][]string{
	"product":  {"prod:detail:*", "prod:list:*", "prod:origins:summary:*", "cat:all"},
	"cart":     {"cart:b:*", "cart:g:*", "seller:meta:*"}, // b=buyer（cart:a: rename，見 CacheKeyConstants.CART_BUYER_PREFIX）
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

	// REDIS_HOST/PORT/PASSWORD 是唯一來源（見 pkgredis.OptionsFromEnv 的註解：
	// REDIS_URL 覆蓋機制已於 2026-07-29 移除）。
	opts := pkgredis.OptionsFromEnv()
	opts.DialTimeout = 3 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second
	// Explicit retry policy for transient network errors (connection refused,
	// timeout, failover, deploy blip) — applied uniformly to every Redis op.
	// Retrying here at the client is the right layer.
	opts.MaxRetries = 3
	opts.MinRetryBackoff = 8 * time.Millisecond
	opts.MaxRetryBackoff = 256 * time.Millisecond

	RedisClient = redis.NewClient(opts)

	// 啟動時的 ping 只是健康檢查：go-redis 延遲連線、需要時重連，所以 Redis 短暫中斷時仍保留 client，Redis 恢復後
	// 自己好。這裡設成 nil 的話，ws-ticket、限流與賣家 guard 會在啟動時的一次中斷之後永遠停在沒有 Redis 的降級
	// 路徑（B-199 / #939）。
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

// deleteByPattern 使用 SCAN + DEL 清除符合 pattern 的 key（避免 KEYS 阻塞）
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
