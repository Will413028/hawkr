package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// B-169 (D8) public violation cache — Redis 5min TTL + invalidate-on-write + fail-open.

const (
	publicViolationCacheTTL    = 5 * time.Minute
	publicViolationCacheKeyFmt = "seller:violations:public:%d"
)

// Prometheus metrics 對齊 B-135 pkg/blocklist 的命名 pattern。
var (
	publicViolationCacheHit = promauto.NewCounter(prometheus.CounterOpts{
		Name: "seller_violation_public_cache_hit_total",
		Help: "Public violation cache hits (B-169 D8).",
	})
	publicViolationCacheMiss = promauto.NewCounter(prometheus.CounterOpts{
		Name: "seller_violation_public_cache_miss_total",
		Help: "Public violation cache misses (B-169 D8).",
	})
	publicViolationCacheUnavailable = promauto.NewCounter(prometheus.CounterOpts{
		Name: "seller_violation_public_cache_unavailable_total",
		Help: "Public violation cache fail-open events (Redis unavailable, B-169 D8).",
	})
)

// publicViolationCache implements usecase.PublicViolationCache backed by Redis.
//
// Behavior：
//   - Get：cache hit 回 violations + true；miss / Redis err 回 nil + false。
//   - Set：best-effort，失敗只 metric +1。
//   - Invalidate：DEL key；失敗只 metric +1。
type publicViolationCache struct {
	rdb *redis.Client
}

// NewPublicViolationCache constructs the cache. nil rdb 也 OK（測試 / disable Redis 時）— 全 miss 路徑。
func NewPublicViolationCache(rdb *redis.Client) usecase.PublicViolationCache {
	return &publicViolationCache{rdb: rdb}
}

func (c *publicViolationCache) Get(ctx context.Context, sellerID int64) ([]*domain.SellerViolation, bool) {
	if c.rdb == nil {
		publicViolationCacheUnavailable.Inc()
		return nil, false
	}
	key := fmt.Sprintf(publicViolationCacheKeyFmt, sellerID)
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			publicViolationCacheMiss.Inc()
		} else {
			publicViolationCacheUnavailable.Inc()
		}
		return nil, false
	}
	var violations []*domain.SellerViolation
	if err := json.Unmarshal(raw, &violations); err != nil {
		publicViolationCacheMiss.Inc()
		return nil, false
	}
	publicViolationCacheHit.Inc()
	return violations, true
}

func (c *publicViolationCache) Set(ctx context.Context, sellerID int64, violations []*domain.SellerViolation) {
	if c.rdb == nil {
		return
	}
	key := fmt.Sprintf(publicViolationCacheKeyFmt, sellerID)
	body, err := json.Marshal(violations)
	if err != nil {
		return
	}
	if err := c.rdb.Set(ctx, key, body, publicViolationCacheTTL).Err(); err != nil {
		publicViolationCacheUnavailable.Inc()
	}
}

func (c *publicViolationCache) Invalidate(ctx context.Context, sellerID int64) {
	if c.rdb == nil {
		return
	}
	key := fmt.Sprintf(publicViolationCacheKeyFmt, sellerID)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		publicViolationCacheUnavailable.Inc()
	}
}
