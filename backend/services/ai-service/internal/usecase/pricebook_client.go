package usecase

import "context"

type ObservedPrice struct {
	MarketCode  string   `json:"market_code"`
	CropCode    string   `json:"crop_code"`
	TradeDate   string   `json:"trade_date"`
	AvgPrice    *float64 `json:"avg_price,omitempty"`
	UpperPrice  *float64 `json:"upper_price,omitempty"`
	MiddlePrice *float64 `json:"middle_price,omitempty"`
	LowerPrice  *float64 `json:"lower_price,omitempty"`
	VolumeKg    *float64 `json:"volume_kg,omitempty"`
}

type PriceableEntityMatch struct {
	Kind        string
	Code        string
	Name        string
	CategoryIDs []string
	MatchedForm string
	FormKind    string
	Tier        int
	Rank        int
	HasForecast bool
	GroupCode   string
}

type CropCatalogMatch struct {
	MatchType    string
	GroupCode    string
	GroupName    string
	CropCode     *string
	CropName     *string
	VariantName  *string
	MatchedAlias *string
	HasForecast  bool
}

type RegionalComparisonRegion struct {
	RegionCode string
	RegionName string
	Status     string
	P10        *float64
	P50        *float64
	P90        *float64

	SupportingMarketCount int32

	ExcludedMarketCodes []string

	AggregationMethod *string

	FreshnessStatus string

	QualityReasonCodes []string
}

type RegionalComparison struct {
	Regions []RegionalComparisonRegion

	SourceDataAsOf *string
}

type RegionalComparisonInput struct {
	CropCode  *string
	GroupCode *string
}

type CommodityPrice struct {
	Status                string
	P50                   *float64
	PriceLayer            string
	Unit                  string
	SourceDataAsOf        string
	TargetDate            string
	SupportingMarketCount string
	TotalQuantity         *string
	QuantityUnit          string
}

type PricebookClient interface {
	GetObservedPrice(ctx context.Context, cropName string) ([]ObservedPrice, error)
	GetCommodityPrice(ctx context.Context, commodityCode string) (CommodityPrice, error)

	ListGroupCropOptions(ctx context.Context, groupCode string, limit int32) ([]CropCatalogMatch, error)

	SearchPriceableEntities(ctx context.Context, variants []string, limit int32) ([]PriceableEntityMatch, error)

	GetRegionalComparison(ctx context.Context, in RegionalComparisonInput, horizonDays int32) (RegionalComparison, error)
}
