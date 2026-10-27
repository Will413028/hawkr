package domain

import (
	"context"
	"time"
)

type MarketOverviewQuery struct {
	Scope      string
	WindowDays int
	Page       int
	PageSize   int
	Query      string
	MarketCode string
}

type MarketOverviewRepository interface {
	LoadSnapshot(context.Context, int64, MarketOverviewQuery, time.Time) (MarketOverviewSnapshot, error)
}

type MarketOverviewSnapshot struct {
	Cutoff        time.Time
	TotalItems    int64
	MarketOptions []MarketRef
	Items         []MarketOverviewRow
}

type MarketOverviewRow struct {
	WatchTargetID                *int64
	CropCode, CropName           string
	VariantName                  *string
	MarketCode, MarketName       *string
	IsWatched                    bool
	Active                       bool
	LatestActual, PreviousActual *MarketOverviewActual
	Change                       *MarketOverviewChange
	Points                       []MarketOverviewPoint
	SourceDigests                []string
	AvailableAt                  time.Time
	VolumeWeightFallback         bool
}

type MarketOverviewActual struct {
	TradeDate time.Time
	AvgPrice  string
	VolumeKg  *string
}

type MarketOverviewPoint struct {
	TradeDate time.Time
	AvgPrice  string
}

type MarketOverviewChange struct {
	Absolute  string
	Percent   *string
	Direction string
}
