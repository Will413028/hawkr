// services/seller-service/internal/infrastructure/address_geocode_cache.go
package infrastructure

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"github.com/redis/go-redis/v9"
)

// geocodeCacheTTL — address geocode results are stable reference data; 24h
// balances hit-rate against staleness for freshly-added streets.
const geocodeCacheTTL = 24 * time.Hour

// geocodeCache is a Redis cache-aside store for address suggestions/validation.
// Fails open: when Redis is nil/unavailable, Get returns ok=false and Set is a
// no-op, so the use case falls through to a live Google Maps call.
type geocodeCache struct {
	rdb *redis.Client
}

// NewGeocodeCache wires the shared Redis client behind usecase.GeocodeCache.
func NewGeocodeCache(rdb *redis.Client) usecase.GeocodeCache {
	return &geocodeCache{rdb: rdb}
}

func (c *geocodeCache) GetSuggestions(ctx context.Context, key string) ([]usecase.Suggestion, bool) {
	if c.rdb == nil {
		return nil, false
	}
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	var items []usecase.Suggestion
	if json.Unmarshal(raw, &items) != nil {
		return nil, false
	}
	return items, true
}

func (c *geocodeCache) SetSuggestions(ctx context.Context, key string, items []usecase.Suggestion) {
	if c.rdb == nil {
		return
	}
	body, err := json.Marshal(items)
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, key, body, geocodeCacheTTL).Err()
}

func (c *geocodeCache) GetValidation(ctx context.Context, key string) (*usecase.GeocodeValidation, bool) {
	if c.rdb == nil {
		return nil, false
	}
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	var v usecase.GeocodeValidation
	if json.Unmarshal(raw, &v) != nil {
		return nil, false
	}
	return &v, true
}

func (c *geocodeCache) SetValidation(ctx context.Context, key string, v *usecase.GeocodeValidation) {
	if c.rdb == nil {
		return
	}
	body, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, key, body, geocodeCacheTTL).Err()
}
