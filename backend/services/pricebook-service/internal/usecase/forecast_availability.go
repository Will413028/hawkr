package usecase

import (
	"context"
	"sort"
	"strings"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// forecastAvailability 回答「這個實體算不算得出價」：crop 看 artifact 有沒有它的序列，
// group 看任一 active 子品項有沒有。SearchCropCatalog 與 SearchPriceableEntities 共用，
// 兩支 RPC 的 has_forecast 語意因此不可能分岔。
type forecastAvailability struct {
	repo     domain.ForecastRepository
	artifact forecast.ModelArtifact
}

func (f forecastAvailability) cropHasForecast(cropCode string) bool {
	return len(f.marketsForCrop(cropCode)) > 0
}

func (f forecastAvailability) groupHasForecast(ctx context.Context, groupCode string) bool {
	crops, err := f.repo.CropsInGroup(ctx, groupCode)
	if err != nil {
		return false
	}
	cropCodes := make([]string, 0, len(crops))
	for _, c := range crops {
		if c.Active {
			cropCodes = append(cropCodes, c.CropCode)
		}
	}
	return f.artifact.HasAnySeriesForCrops(cropCodes)
}

// marketsForCrop enumerates every market with a trained series for
// cropCode by scanning artifact.Series' "moa:market:crop" keys — the same
// key convention ModelArtifact.HasSeries uses, just enumerated instead of
// point-checked. Sorted by market_code for determinism.
//
// Pure in-memory (the artifact is already loaded): the returned options
// carry MarketCode only, MarketName is left blank for resolveMarketNames
// to fill in one batch.
func (f forecastAvailability) marketsForCrop(cropCode string) []CatalogMarketOption {
	cropID := "moa:" + cropCode
	marketCodes := map[string]bool{}
	for _, series := range f.artifact.Series {
		if series.CropID == cropID {
			marketCodes[strings.TrimPrefix(series.MarketID, "moa:")] = true
		}
	}
	codes := make([]string, 0, len(marketCodes))
	for code := range marketCodes {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	out := make([]CatalogMarketOption, 0, len(codes))
	for _, code := range codes {
		out = append(out, CatalogMarketOption{MarketCode: code})
	}
	return out
}
