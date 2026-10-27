package forecast

import (
	"fmt"
	"math"
	"time"
)

// PricePoint is a normalized daily market/crop observation, mirroring
// Python's features.py PricePoint (features.py:87-101). AveragePrice/Volume
// are *float64 because "no trade recorded" (nil) and "traded at zero"
// (non-nil 0.0) are different facts.
type PricePoint struct {
	SeriesID            string
	MarketID            string
	CropID              string
	TradeDate           Date
	AveragePrice        *float64
	Volume              *float64
	AvailableAt         time.Time // always normalized to UTC by NewPricePoint
	IsMarketClosed      bool
	AvailabilityQuality string
	CategoryCode        *string
	IsActive            bool
}

var allowedAvailabilityQuality = map[string]bool{
	"observed":           true,
	"official_archive":   true,
	"backfilled_current": true,
	"reconstructed":      true,
}

// PricePointInput is the unvalidated shape passed to NewPricePoint.
// IsActive is a pointer so nil can mean "use the default (true)", mirroring
// the dataclass default is_active=True (features.py:101).
type PricePointInput struct {
	SeriesID            string
	MarketID            string
	CropID              string
	TradeDate           Date
	AveragePrice        *float64
	Volume              *float64
	AvailableAt         time.Time
	IsMarketClosed      bool
	AvailabilityQuality string // "" defaults to "observed"
	CategoryCode        *string
	IsActive            *bool // nil defaults to true
}

// NewPricePoint validates and normalizes an input, mirroring
// PricePoint.__post_init__ (features.py:103-128).
func NewPricePoint(in PricePointInput) (PricePoint, error) {
	if in.SeriesID == "" || in.MarketID == "" || in.CropID == "" {
		return PricePoint{}, fmt.Errorf("forecast: series_id, market_id, and crop_id must be non-empty")
	}
	quality := in.AvailabilityQuality
	if quality == "" {
		quality = "observed"
	}
	if !allowedAvailabilityQuality[quality] {
		return PricePoint{}, fmt.Errorf("forecast: availability_quality %q must be one of observed, official_archive, backfilled_current, reconstructed", quality)
	}
	if in.AveragePrice != nil {
		if math.IsNaN(*in.AveragePrice) || math.IsInf(*in.AveragePrice, 0) || *in.AveragePrice < 0 {
			return PricePoint{}, fmt.Errorf("forecast: average_price must be finite and non-negative, got %v", *in.AveragePrice)
		}
	}
	if in.Volume != nil {
		if math.IsNaN(*in.Volume) || math.IsInf(*in.Volume, 0) || *in.Volume < 0 {
			return PricePoint{}, fmt.Errorf("forecast: volume must be finite and non-negative, got %v", *in.Volume)
		}
	}
	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	return PricePoint{
		SeriesID:            in.SeriesID,
		MarketID:            in.MarketID,
		CropID:              in.CropID,
		TradeDate:           in.TradeDate,
		AveragePrice:        in.AveragePrice,
		Volume:              in.Volume,
		AvailableAt:         in.AvailableAt.UTC(),
		IsMarketClosed:      in.IsMarketClosed,
		AvailabilityQuality: quality,
		CategoryCode:        in.CategoryCode,
		IsActive:            isActive,
	}, nil
}
