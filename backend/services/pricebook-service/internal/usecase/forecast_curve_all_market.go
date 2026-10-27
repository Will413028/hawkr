package usecase

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// ForecastAggregationMeta describes the same participants and weights used for
// all three quantiles. Point metadata is authoritative for that target date;
// result metadata describes D+1, matching the result's status.
type ForecastAggregationMeta struct {
	Method                   string
	WeightWindowDays         int
	ParticipatingMarketCodes []string
	ExcludedMarkets          []ForecastMarketExclusion
}
type ForecastMarketExclusion struct{ MarketCode, Reason string }

// CodeMeta PricebookMarketExclusionReason.NO_POSITIVE_VOLUME wire value.
const forecastMarketExclusionNoPositiveVolume = "no_positive_volume"

func (u *ForecastCurve) allMarketForecast(ctx context.Context, crop string, now time.Time) (ForecastCurveResult, error) {
	result := ForecastCurveResult{Status: forecast.PredictionStatusUnavailable, ResolvedCropCode: &crop, DataMeta: unavailableDataMeta(now)}
	codesSet := map[string]bool{}
	for _, series := range u.artifact.Series {
		if series.CropID == "moa:"+crop {
			codesSet[strings.TrimPrefix(series.MarketID, "moa:")] = true
		}
	}
	result.Aggregation = &ForecastAggregationMeta{Method: forecast.TrendAggregationVolumeWeightedV1, WeightWindowDays: u.cfg.Inference.GroupVolumeWindowDays}
	origin, cutoff, err := forecast.OriginDateAndCutoff(now, calendarTimezoneOf(u.artifact))
	if err != nil {
		return result, err
	}
	volumes, err := u.forecastUC.ComputeCropVolumes(ctx, crop, origin, cutoff)
	if err != nil {
		return result, err
	}
	// The candidate universe combines trained series and markets with recent
	// observed volume, so missing model coverage is visible as partial.
	for code := range volumes {
		codesSet[code] = true
	}
	codes := make([]string, 0, len(codesSet))
	for code := range codesSet {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	if len(codes) == 0 {
		return result, nil
	}
	outputs := make(map[string]*SeriesForecastOutput, len(codes))
	for _, code := range codes {
		out := &SeriesForecastOutput{}
		if u.artifact.HasSeries(code, crop) {
			out, err = u.forecastUC.SeriesForecast(ctx, SeriesForecastInput{Artifact: u.artifact, MarketCode: code, CropCode: crop, Origin: now})
			if err != nil {
				return ForecastCurveResult{}, fmt.Errorf("all-market forecast %s/%s: %w", code, crop, err)
			}
		} else {
			for h := 1; h <= 7; h++ {
				out.Forecast = append(out.Forecast, ForecastPoint{HorizonDays: h, TargetDate: origin.AddDays(h), Status: forecast.PredictionStatusUnavailable})
			}
		}
		outputs[code] = out
		if result.ModelVersion == "" {
			result.ModelVersion = out.ModelVersion
		}
		entry := MarketComparisonEntry{MarketCode: code, MarketName: code, Status: forecast.PredictionStatusUnavailable}
		if p, ok := pointAtHorizon1(out.Forecast); ok {
			entry.Status, entry.P50 = p.Status, usableComparisonPrice(p)
		}
		ref, err := u.repo.MarketByCode(ctx, code)
		if err != nil {
			entry.Status, entry.P50 = forecast.PredictionStatusUnavailable, nil
		} else if ref.Name != "" {
			entry.MarketName = ref.Name
		}
		result.MarketComparison = append(result.MarketComparison, entry)
	}
	for h := 1; h <= 7; h++ {
		result.Points = append(result.Points, aggregateForecastMarkets(codes, outputs, volumes, origin.AddDays(h), h, u.cfg.Inference.GroupVolumeWindowDays))
	}
	result.Status = result.Points[0].Status
	result.Aggregation = result.Points[0].Aggregation
	// Metadata only includes series that actually contributed to at least one day.
	var maxAvailable *time.Time
	var through *forecast.Date
	var digests []string
	for _, code := range codes {
		used := false
		for _, p := range result.Points {
			for _, c := range p.Aggregation.ParticipatingMarketCodes {
				if c == code {
					used = true
				}
			}
		}
		if !used {
			continue
		}
		out := outputs[code]
		if out.MaxFeatureAvailableAt != nil && (maxAvailable == nil || out.MaxFeatureAvailableAt.After(*maxAvailable)) {
			maxAvailable = out.MaxFeatureAvailableAt
		}
		if out.DataThroughDate != nil && (through == nil || out.DataThroughDate.ToTime().After(through.ToTime())) {
			through = out.DataThroughDate
		}
		if out.DatasetDigest != "" {
			digests = append(digests, out.DatasetDigest)
		}
	}
	meta := observedDataMeta(now, freshnessConfigFrom(u.cfg.Inference), maxAvailable, through, forecast.DatasetDigestFromSourceDigests(digests))
	meta.UsabilityStatus = forecast.SeriesUsability(result.Status)
	if result.ModelVersion != "" {
		meta.ModelVersion = &result.ModelVersion
	}
	for _, p := range result.Points {
		if p.Aggregation.Method == forecast.AggregationMethodEqualWeightFallback {
			meta.QualityReasonCodes = append(meta.QualityReasonCodes, forecast.QualityReasonVolumeWeightFallback)
			break
		}
	}
	result.DataMeta = meta
	return result, nil
}

func aggregateForecastMarkets(codes []string, outputs map[string]*SeriesForecastOutput, volumes map[string]float64, date forecast.Date, horizon, window int) CurvePoint {
	meta := &ForecastAggregationMeta{Method: forecast.TrendAggregationVolumeWeightedV1, WeightWindowDays: window}
	result := CurvePoint{TargetDate: date, HorizonDays: horizon, Status: forecast.PredictionStatusUnavailable, Aggregation: meta}
	selected := map[string]ForecastPoint{}
	closed := 0
	unavailable := 0
	for _, code := range codes {
		reason := "unavailable"
		for _, p := range outputs[code].Forecast {
			if p.HorizonDays != horizon {
				continue
			}
			reason = string(p.Status)
			if p.Status == forecast.PredictionStatusUnavailable {
				unavailable++
			}
			if p.Status == forecast.PredictionStatusMarketClosed {
				closed++
			}
			if p.Status == forecast.PredictionStatusReady || p.Status == forecast.PredictionStatusPartial {
				if finiteQuantiles(p) {
					selected[code] = p
				} else {
					reason = string(forecast.PredictionStatusInsufficientData)
				}
			}
			break
		}
		if _, ok := selected[code]; !ok {
			meta.ExcludedMarkets = append(meta.ExcludedMarkets, ForecastMarketExclusion{code, reason})
		}
	}
	if len(selected) == 0 {
		if closed == len(codes) && closed > 0 {
			result.Status = forecast.PredictionStatusMarketClosed
		} else if unavailable == len(codes) && unavailable > 0 {
			result.Status = forecast.PredictionStatusUnavailable
		} else {
			result.Status = forecast.PredictionStatusInsufficientData
		}
		return result
	}
	total := 0.0
	for _, code := range codes {
		if _, ok := selected[code]; !ok {
			continue
		}
		if v := volumes[code]; v > 0 && !math.IsInf(v, 0) && !math.IsNaN(v) {
			total += v
		}
	}
	fallback := total == 0
	if fallback {
		meta.Method = forecast.AggregationMethodEqualWeightFallback
		total = float64(len(selected))
	}
	var p10, p50, p90 float64
	partial := len(selected) < len(codes)
	for _, code := range codes {
		p, ok := selected[code]
		if !ok {
			continue
		}
		w := volumes[code]
		if fallback {
			w = 1
		} else if w <= 0 || math.IsInf(w, 0) || math.IsNaN(w) {
			meta.ExcludedMarkets = append(meta.ExcludedMarkets, ForecastMarketExclusion{code, forecastMarketExclusionNoPositiveVolume})
			partial = true
			continue
		}
		w /= total
		p10 += *p.P10 * w
		p50 += *p.P50 * w
		p90 += *p.P90 * w
		meta.ParticipatingMarketCodes = append(meta.ParticipatingMarketCodes, code)
		partial = partial || p.Status == forecast.PredictionStatusPartial
	}
	result.P10, result.P50, result.P90 = &p10, &p50, &p90
	result.Status = forecast.PredictionStatusReady
	if partial {
		result.Status = forecast.PredictionStatusPartial
	}
	return result
}
func finiteQuantiles(p ForecastPoint) bool {
	for _, v := range []*float64{p.P10, p.P50, p.P90} {
		if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
			return false
		}
	}
	return true
}
