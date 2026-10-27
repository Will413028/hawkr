package domain

import (
	"context"
	"time"
)

// RollupStatus is category_price_rollup.status.
type RollupStatus string

const (
	RollupStatusReady       RollupStatus = "ready"
	RollupStatusMixedLayer  RollupStatus = "mixed_layer"
	RollupStatusUnavailable RollupStatus = "unavailable"
)

// RollupPricePoint is one day of category_price_rollup.points' JSONB array —
// shape mirrors the existing crop-level sparkline points the website already
// consumes (see design spec §3b), field names chosen to match it directly.
type RollupPricePoint struct {
	TradeDate string  `json:"tradeDate"` // YYYY-MM-DD
	AvgPrice  float64 `json:"avgPrice"`
}

// RollupForecastPoint is one horizon day of category_price_rollup.
// forecast_points' JSONB array — the group-level GroupForecast curve
// (L3) or the per-horizon average of ready children's curves (L2/L1).
// P10/P50/P90 are nil when that horizon has no usable value, never
// zero-filled (fail-closed, same convention as RollupPricePoint.AvgPrice
// requiring a non-nil upstream value before this type is populated).
type RollupForecastPoint struct {
	HorizonDays int      `json:"horizonDays"`
	TargetDate  string   `json:"targetDate"` // YYYY-MM-DD
	P10         *float64 `json:"p10,omitempty"`
	P50         *float64 `json:"p50,omitempty"`
	P90         *float64 `json:"p90,omitempty"`
}

// CategoryPriceRollup is one category_price_rollup row — the daily-computed
// price snapshot for a product-service category at any tree level.
type CategoryPriceRollup struct {
	CategoryID       string
	Level            int16
	Status           RollupStatus
	LatestPrice      *float64
	PrevPrice        *float64
	Points           []RollupPricePoint
	ForecastPoints   []RollupForecastPoint
	SourceGroupCount int16
	PriceLayer       *PriceLayer
	Unit             *string
	ComputedAt       time.Time
}

// CategoryPriceRollupRepository is category_price_rollup's narrow
// repository interface — the daily batch (Upsert) and the read API (Get)
// are its only two callers.
type CategoryPriceRollupRepository interface {
	// Upsert overwrites the row for r.CategoryID (the table has no history,
	// only a "current" snapshot per category_id).
	Upsert(ctx context.Context, r CategoryPriceRollup) error
	// Get returns nil, nil when no row exists yet for categoryID (e.g. the
	// batch hasn't run since this category was created) — the caller
	// decides how to represent "no snapshot yet" (see Task 10).
	Get(ctx context.Context, categoryID string) (*CategoryPriceRollup, error)
}
