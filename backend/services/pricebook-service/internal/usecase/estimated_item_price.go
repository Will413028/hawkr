// services/pricebook-service/internal/usecase/estimated_item_price.go
package usecase

import (
	"context"
	"math"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
)

// EstimatedItemPriceResult is what Get returns: a resolved forecast point
// converted into a per-item price, or a status explaining why no price
// could be produced. Status is always set; the price fields and exactly
// one of ResolvedCropGroupCode/ResolvedCropCode are only set when a price
// was actually computed.
type EstimatedItemPriceResult struct {
	Status                forecast.PredictionStatus
	Low, Median, High     *int64
	ResolvedCropGroupCode *string
	ResolvedCropCode      *string
	MarketCode            *string
	ModelVersion          string
}

// EstimatedItemPrice serves GetEstimatedItemPrice: resolve a seller-facing
// crop name via the same P4 mapping seed table ObservedPrice uses, run the
// existing 7-day forecast for horizon=1 (nearest prediction point), and
// convert its P10/P50/P90 (TWD/kg) into a per-item price using the
// caller-supplied net weight.
//
// Unlike this package's other usecases (which depend only on repo/config
// types), EstimatedItemPrice depends on *Forecast directly: SeriesForecast/
// GroupForecast already encapsulate substantial domain logic (calendar
// handling, quality gates, status evaluation — see forecast.go) that must
// not be duplicated here.
type EstimatedItemPrice struct {
	forecastUC *Forecast
	repo       domain.ForecastRepository
	seed       mapping.Seed
	artifact   forecast.ModelArtifact
}

func NewEstimatedItemPrice(forecastUC *Forecast, repo domain.ForecastRepository, seed mapping.Seed, artifact forecast.ModelArtifact) *EstimatedItemPrice {
	return &EstimatedItemPrice{forecastUC: forecastUC, repo: repo, seed: seed, artifact: artifact}
}

// Get resolves cropName and returns the estimated per-item price. Mirrors
// ObservedPrice.Get's error philosophy: business "can't produce a price"
// outcomes come back as a Status, never as err. err is reserved for
// caller-contract violations (market_code misuse) and infrastructure
// failures (repo/forecast errors).
func (u *EstimatedItemPrice) Get(ctx context.Context, cropName string, netWeightKg float64, marketCode string, now time.Time) (EstimatedItemPriceResult, error) {
	// The trailing "", "" are resolveCropTarget's crop_code/group_code
	// direct-lookup bypasses, deliberately never used here: this usecase's
	// public contract is crop_name-only (the seller "這箱大概值多少" flow
	// always starts from a product name). Passing them empty keeps it on the
	// seed-resolution path, unchanged.
	target, err := resolveCropTarget(ctx, u.seed, u.repo, u.artifact, cropName, marketCode, "", "")
	if err != nil {
		return EstimatedItemPriceResult{}, err
	}
	if target.Unavailable {
		return EstimatedItemPriceResult{
			Status:                forecast.PredictionStatusUnavailable,
			ResolvedCropCode:      target.ResolvedCropCode,
			ResolvedCropGroupCode: target.ResolvedCropGroupCode,
			MarketCode:            target.MarketCode,
		}, nil
	}

	if target.IsCrop {
		out, err := u.forecastUC.SeriesForecast(ctx, SeriesForecastInput{
			Artifact: u.artifact, MarketCode: marketCode, CropCode: target.CropCode, Origin: now,
		})
		if err != nil {
			return EstimatedItemPriceResult{}, err
		}
		point, ok := pointAtHorizon1(out.Forecast)
		if !ok {
			cropCode := target.CropCode
			mc := marketCode
			return EstimatedItemPriceResult{Status: forecast.PredictionStatusUnavailable, ResolvedCropCode: &cropCode, MarketCode: &mc}, nil
		}
		cropCode := target.CropCode
		result := priceResultFromPoint(point.Status, point.P10, point.P50, point.P90, netWeightKg, out.ModelVersion)
		result.ResolvedCropCode = &cropCode
		result.MarketCode = &marketCode
		return result, nil
	}

	out, err := u.forecastUC.GroupForecast(ctx, GroupForecastInput{
		Artifact: u.artifact, GroupCode: target.GroupCode, Origin: now,
	})
	if err != nil {
		return EstimatedItemPriceResult{}, err
	}
	point, ok := groupPointAtHorizon1(out.Forecast)
	if !ok {
		groupCode := target.GroupCode
		return EstimatedItemPriceResult{Status: forecast.PredictionStatusUnavailable, ResolvedCropGroupCode: &groupCode}, nil
	}
	groupCode := target.GroupCode
	result := priceResultFromPoint(point.Status, point.P10, point.P50, point.P90, netWeightKg, out.ModelVersion)
	result.ResolvedCropGroupCode = &groupCode
	return result, nil
}

// pointAtHorizon1 is the D+1 special case of spatial_surface.go's
// pointAtHorizon. Kept as a named helper because "the nearest prediction
// point" is what this usecase and forecast_curve.go actually mean at their
// call sites, but it no longer carries its own copy of the scan.
func pointAtHorizon1(points []ForecastPoint) (ForecastPoint, bool) {
	return pointAtHorizon(points, 1)
}

func groupPointAtHorizon1(points []forecast.GroupPoint) (forecast.GroupPoint, bool) {
	for _, p := range points {
		if p.HorizonDays == 1 {
			return p, true
		}
	}
	return forecast.GroupPoint{}, false
}

// priceResultFromPoint converts a forecast point's P10/P50/P90 (TWD/kg)
// into per-item prices. Prices are only populated when status is ready or
// partial — partial is GroupForecast's existing "some sub-variants ready"
// semantics, where the aggregated P10/P50/P90 already only counts ready
// sub-variants (see group.go), so the number is still trustworthy. Every
// other status returns just the status, never a fake number — same
// fail-fast philosophy as provideMappingSeed/ObservedPrice.
func priceResultFromPoint(status forecast.PredictionStatus, p10, p50, p90 *float64, netWeightKg float64, modelVersion string) EstimatedItemPriceResult {
	result := EstimatedItemPriceResult{Status: status, ModelVersion: modelVersion}
	if status != forecast.PredictionStatusReady && status != forecast.PredictionStatusPartial {
		return result
	}
	result.Low = priceFromKg(p10, netWeightKg)
	result.Median = priceFromKg(p50, netWeightKg)
	result.High = priceFromKg(p90, netWeightKg)
	return result
}

func priceFromKg(pricePerKg *float64, netWeightKg float64) *int64 {
	if pricePerKg == nil {
		return nil
	}
	v := int64(math.Round(*pricePerKg * netWeightKg))
	return &v
}
