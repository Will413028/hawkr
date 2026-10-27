package domain

import "time"

// DailyObservedPrice is a batch observation aggregate, with decimal strings
// produced directly by PostgreSQL NUMERIC. Metadata describes only rows that
// participate in the price (positive weights, or every eligible fallback row).
// A nil VolumeKg means all participating rows have unknown volume.
type DailyObservedPrice struct {
	CropCode             string
	TradeDate            time.Time
	AvgPrice             string
	VolumeKg             *string
	MarketCodes          []string
	SourceDigests        []string
	AvailableAt          time.Time
	VolumeWeightFallback bool
}
