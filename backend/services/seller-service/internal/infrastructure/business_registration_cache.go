package infrastructure

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"github.com/redis/go-redis/v9"
)

// businessRegCacheTTL — company registration is slow-changing reference data, so
// a long TTL maximises hit-rate without meaningful staleness.
const businessRegCacheTTL = 30 * 24 * time.Hour

// businessRegistrationCache is a Redis cache-aside store for GCIS lookups. It
// fails open: when Redis is nil/unavailable, Get returns ok=false and Set is a
// no-op, so the use case falls through to a live GCIS call.
type businessRegistrationCache struct {
	rdb *redis.Client
}

// NewBusinessRegistrationCache wires the shared Redis client behind the
// usecase.BusinessRegistrationCache interface.
func NewBusinessRegistrationCache(rdb *redis.Client) usecase.BusinessRegistrationCache {
	return &businessRegistrationCache{rdb: rdb}
}

func businessRegCacheKey(ban string) string { return "ban:lookup:" + ban }

func (c *businessRegistrationCache) Get(ctx context.Context, ban string) (*usecase.BusinessRegistration, bool) {
	if c.rdb == nil {
		return nil, false
	}
	raw, err := c.rdb.Get(ctx, businessRegCacheKey(ban)).Bytes()
	if err != nil {
		return nil, false
	}
	var br usecase.BusinessRegistration
	if json.Unmarshal(raw, &br) != nil {
		return nil, false
	}
	return &br, true
}

func (c *businessRegistrationCache) Set(ctx context.Context, ban string, br *usecase.BusinessRegistration) {
	if c.rdb == nil {
		return
	}
	body, err := json.Marshal(br)
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, businessRegCacheKey(ban), body, businessRegCacheTTL).Err()
}
