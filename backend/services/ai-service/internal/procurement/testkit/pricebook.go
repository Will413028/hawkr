package testkit

import (
	"context"

	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
)

type FakePricebookClient struct {
	Err error

	Regions        []pbusecase.RegionalComparisonRegion
	RegionsErr     error
	GotRegionInput pbusecase.RegionalComparisonInput
	GotHorizonDays int32

	RegionsSequence [][]pbusecase.RegionalComparisonRegion
	RegionCallCount int

	SourceDataAsOf *string

	GroupCrops       []pbusecase.CropCatalogMatch
	GroupCropsErr    error
	GotGroupCode     string
	CommodityPrice   pbusecase.CommodityPrice
	CommodityErr     error
	GotCommodityCode string

	PriceableMatches []pbusecase.PriceableEntityMatch
	PriceableErr     error
	GotVariants      []string
	PriceableCalls   int
}

func (f *FakePricebookClient) SearchPriceableEntities(_ context.Context, variants []string, _ int32) ([]pbusecase.PriceableEntityMatch, error) {
	f.PriceableCalls++
	f.GotVariants = append([]string(nil), variants...)
	return f.PriceableMatches, f.PriceableErr
}

func (f *FakePricebookClient) GetCommodityPrice(_ context.Context, code string) (pbusecase.CommodityPrice, error) {
	f.GotCommodityCode = code
	return f.CommodityPrice, f.CommodityErr
}

func (f *FakePricebookClient) GetObservedPrice(context.Context, string) ([]pbusecase.ObservedPrice, error) {
	return nil, nil
}

func (f *FakePricebookClient) ListGroupCropOptions(_ context.Context, groupCode string, _ int32) ([]pbusecase.CropCatalogMatch, error) {
	f.GotGroupCode = groupCode
	return f.GroupCrops, f.GroupCropsErr
}

func (f *FakePricebookClient) GetRegionalComparison(_ context.Context, in pbusecase.RegionalComparisonInput, horizonDays int32) (pbusecase.RegionalComparison, error) {
	f.GotRegionInput = in
	f.GotHorizonDays = horizonDays
	if f.RegionsSequence != nil {
		idx := f.RegionCallCount
		if idx >= len(f.RegionsSequence) {
			idx = len(f.RegionsSequence) - 1
		}
		f.RegionCallCount++
		return pbusecase.RegionalComparison{
			Regions: f.RegionsSequence[idx], SourceDataAsOf: f.SourceDataAsOf,
		}, f.RegionsErr
	}
	return pbusecase.RegionalComparison{Regions: f.Regions, SourceDataAsOf: f.SourceDataAsOf}, f.RegionsErr
}

type CountingPricebookClient struct {
	*FakePricebookClient
	RegionCalls int
}

func (c *CountingPricebookClient) GetRegionalComparison(ctx context.Context, in pbusecase.RegionalComparisonInput, horizonDays int32) (pbusecase.RegionalComparison, error) {
	c.RegionCalls++
	return c.FakePricebookClient.GetRegionalComparison(ctx, in, horizonDays)
}
