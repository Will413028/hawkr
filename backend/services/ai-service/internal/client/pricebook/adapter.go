package pricebook

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pricebookpb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"google.golang.org/grpc"
)

type pricebookClient struct {
	client pricebookpb.PricebookServiceClient
}

func NewPricebookClient(cfg *config.Config) (usecase.PricebookClient, error) {
	target := fmt.Sprintf("%s:%s", cfg.PricebookServiceHost, cfg.PricebookServicePort)
	conn, err := grpcclient.Dial(target,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),

		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("ai-service", false)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to pricebook-service at %s: %w", target, err)
	}
	return &pricebookClient{client: pricebookpb.NewPricebookServiceClient(conn)}, nil
}

func (c *pricebookClient) GetObservedPrice(ctx context.Context, cropName string) ([]usecase.ObservedPrice, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetObservedPrice(ctx, &pricebookpb.GetObservedPriceRequest{CropName: cropName})
	if err != nil {
		return nil, err
	}

	observations := make([]usecase.ObservedPrice, 0, len(resp.GetObservations()))
	for _, o := range resp.GetObservations() {
		observations = append(observations, usecase.ObservedPrice{
			MarketCode:  o.GetMarketCode(),
			CropCode:    o.GetCropCode(),
			TradeDate:   o.GetTradeDate(),
			AvgPrice:    o.AvgPrice,
			UpperPrice:  o.UpperPrice,
			MiddlePrice: o.MiddlePrice,
			LowerPrice:  o.LowerPrice,
			VolumeKg:    o.VolumeKg,
		})
	}
	return observations, nil
}

func (c *pricebookClient) SearchPriceableEntities(ctx context.Context, variants []string, limit int32) ([]usecase.PriceableEntityMatch, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.SearchPriceableEntities(ctx, &pricebookpb.SearchPriceableEntitiesRequest{Variants: variants, Limit: limit})
	if err != nil {
		return nil, err
	}
	matches := make([]usecase.PriceableEntityMatch, 0, len(resp.GetMatches()))
	for _, m := range resp.GetMatches() {
		tier, err := strconv.Atoi(m.GetTier())
		if err != nil {
			return nil, fmt.Errorf("pricebook SearchPriceableEntities: tier %q: %w", m.GetTier(), err)
		}
		rank, err := strconv.Atoi(m.GetRank())
		if err != nil {
			return nil, fmt.Errorf("pricebook SearchPriceableEntities: rank %q: %w", m.GetRank(), err)
		}
		matches = append(matches, usecase.PriceableEntityMatch{
			Kind: m.GetKind(), Code: m.GetCode(), Name: m.GetName(), CategoryIDs: m.GetCategoryIds(),
			MatchedForm: m.GetMatchedForm(), FormKind: m.GetFormKind(), Tier: tier, Rank: rank,
			HasForecast: m.GetHasForecast(), GroupCode: m.GetGroupCode(),
		})
	}
	return matches, nil
}

func (c *pricebookClient) ListGroupCropOptions(ctx context.Context, groupCode string, limit int32) ([]usecase.CropCatalogMatch, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ListGroupCropOptions(ctx, &pricebookpb.ListGroupCropOptionsRequest{GroupCode: groupCode, Limit: limit})
	if err != nil {
		return nil, err
	}

	crops := make([]usecase.CropCatalogMatch, 0, len(resp.GetCrops()))
	for _, m := range resp.GetCrops() {
		crops = append(crops, usecase.CropCatalogMatch{
			MatchType:    m.GetMatchType(),
			GroupCode:    m.GetGroupCode(),
			GroupName:    m.GetGroupName(),
			CropCode:     m.CropCode,
			CropName:     m.CropName,
			VariantName:  m.VariantName,
			MatchedAlias: m.MatchedAlias,
			HasForecast:  m.GetHasForecast(),
		})
	}
	return crops, nil
}

func (c *pricebookClient) GetRegionalComparison(ctx context.Context, in usecase.RegionalComparisonInput, horizonDays int32) (usecase.RegionalComparison, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	req := &pricebookpb.GetRegionalComparisonRequest{HorizonDays: horizonDays}
	req.CropCode = in.CropCode
	req.GroupCode = in.GroupCode

	resp, err := c.client.GetRegionalComparison(ctx, req)
	if err != nil {
		return usecase.RegionalComparison{}, err
	}

	regions := make([]usecase.RegionalComparisonRegion, 0, len(resp.GetRegions()))
	for _, r := range resp.GetRegions() {
		regions = append(regions, usecase.RegionalComparisonRegion{
			RegionCode:            r.GetRegionCode(),
			RegionName:            r.GetRegionName(),
			Status:                r.GetStatus(),
			P10:                   r.P10,
			P50:                   r.P50,
			P90:                   r.P90,
			SupportingMarketCount: r.GetSupportingMarketCount(),
			ExcludedMarketCodes:   r.GetExcludedMarketCodes(),
			AggregationMethod:     r.AggregationMethod,
			FreshnessStatus:       r.GetFreshnessStatus(),
			QualityReasonCodes:    r.GetQualityReasonCodes(),
		})
	}

	out := usecase.RegionalComparison{Regions: regions}
	if lineage := resp.GetLineage(); lineage != nil {
		out.SourceDataAsOf = lineage.SourceDataAsOf
	}
	return out, nil
}

func (c *pricebookClient) GetCommodityPrice(ctx context.Context, commodityCode string) (usecase.CommodityPrice, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetForecastCurve(ctx, &pricebookpb.GetForecastCurveRequest{
		CommodityCode: &commodityCode,
	})
	if err != nil {
		return usecase.CommodityPrice{}, err
	}

	out := usecase.CommodityPrice{
		Status:                resp.GetStatus(),
		PriceLayer:            resp.GetPriceLayer(),
		Unit:                  resp.GetUnit(),
		SourceDataAsOf:        resp.GetSourceDataAsOf(),
		SupportingMarketCount: resp.GetSupportingMarketCount(),
		TotalQuantity:         resp.TotalQuantity,
		QuantityUnit:          resp.GetQuantityUnit(),
	}
	for _, p := range resp.GetPoints() {
		if p.GetHorizonDays() != 1 {
			continue
		}
		out.P50 = p.P50
		out.TargetDate = p.GetTargetDate()
		break
	}
	return out, nil
}
