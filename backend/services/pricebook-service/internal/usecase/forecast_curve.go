package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
)

// CurvePoint is one day of a 7-day forecast, unconverted (TWD/kg — no
// net_weight_kg multiplication; that's EstimatedItemPrice's job).
type CurvePoint struct {
	Aggregation   *ForecastAggregationMeta
	TargetDate    forecast.Date
	HorizonDays   int
	Status        forecast.PredictionStatus
	P10, P50, P90 *float64
}

// ForecastCurveResult is what Get returns: the full 7-day curve for a
// resolved crop or crop group, or a status explaining why no forecast
// could be produced (mirrors EstimatedItemPriceResult's own doc comment —
// same philosophy, different payload shape).
type ForecastCurveResult struct {
	Aggregation           *ForecastAggregationMeta
	Status                forecast.PredictionStatus
	ResolvedCropGroupCode *string
	ResolvedCropCode      *string
	MarketCode            *string
	ModelVersion          string
	Points                []CurvePoint
	// MarketComparison lists every sibling market with a trained series for
	// the crop or crop group being viewed (the viewed market excluded), each
	// with its own D+1 P50. On the crop path this is a single model's direct
	// output per sibling market (see marketComparison's own doc comment); on
	// the group path each entry's P50 is an arithmetic mean across the
	// group's crops and IsApproximate is true (see groupMarketComparison's
	// own doc comment, design spec §2b).
	MarketComparison []MarketComparisonEntry
	// DataMeta is issue #2249 Slice A's additive dataMeta object — always
	// populated (never a nil/zero DataMeta), even when Status is Unavailable
	// (in which case most of its fields are themselves nil/empty, describing
	// "nothing to report" rather than fake values).
	DataMeta DataMeta
}

// MarketComparisonEntry is one sibling market's D+1 P50 for the crop or
// crop group being viewed. P50 is nil when that market's own forecast
// couldn't produce a horizon-1 point (e.g. insufficient_data) — same "nil
// means no number, not zero" convention as CurvePoint/ForecastPoint.
// IsApproximate is true only on the group resolution path: a group's P50
// at a market is the arithmetic mean of its constituent crops' own,
// independently-trained per-market models — not a single model's direct
// output the way a crop query's P50 is (design spec §2b). It is never true
// for a crop query.
type MarketComparisonEntry struct {
	Status        forecast.PredictionStatus
	MarketCode    string
	MarketName    string
	P50           *float64
	IsApproximate bool
}

// ForecastCurve serves GetForecastCurve: resolve a seller-facing crop name
// via the same P4 mapping seed table ObservedPrice/EstimatedItemPrice use,
// run the existing 7-day forecast, and return every point unconverted.
//
// Like EstimatedItemPrice, depends on *Forecast directly (not just repo/
// config) for the same reason: SeriesForecast/GroupForecast already
// encapsulate substantial domain logic that must not be duplicated here.
type ForecastCurve struct {
	forecastUC *Forecast
	repo       domain.ForecastRepository
	seed       mapping.Seed
	artifact   forecast.ModelArtifact
	cfg        *config.Config
	logger     *zap.Logger
}

// cfg is used for dataMeta's group-path usabilityStatus derivation
// (cfg.Inference.GroupMinMarketSupportCount — ADR D3, see
// forecast.GroupUsability).
func NewForecastCurve(forecastUC *Forecast, repo domain.ForecastRepository, seed mapping.Seed, artifact forecast.ModelArtifact, cfg *config.Config, logger *zap.Logger) *ForecastCurve {
	return &ForecastCurve{forecastUC: forecastUC, repo: repo, seed: seed, artifact: artifact, cfg: cfg, logger: logger}
}

// Get resolves the request target — exactly one of cropName (via the seed
// table) / cropCode / groupCode (direct lookups; see resolveCropTarget) —
// and returns its full 7-day forecast curve. Mirrors EstimatedItemPrice.Get's
// error philosophy via the shared resolveCropTarget helper: business "can't
// produce a forecast" outcomes come back as a Status, never as err. Caller
// contract violations (zero or multiple targets named, market_code
// mismatched to the target kind) are the exception — those are errors.
func (u *ForecastCurve) Get(ctx context.Context, cropName, marketCode, cropCode, groupCode string, now time.Time) (ForecastCurveResult, error) {
	if err := validateResolutionInputs(cropName, cropCode, groupCode); err != nil {
		return ForecastCurveResult{}, err
	}
	if marketCode == "" {
		exactCrop := cropCode
		if cropName != "" {
			if entry, ok := u.seed.Resolve(cropName); ok {
				exactCrop = entry.CropCode
			}
		}
		if exactCrop != "" {
			return u.allMarketForecast(ctx, exactCrop, now)
		}
	}
	target, err := resolveCropTarget(ctx, u.seed, u.repo, u.artifact, cropName, marketCode, cropCode, groupCode)
	if errors.Is(err, ErrMarketCodeNotAllowedForGroup) {
		// resolveCropTarget's group+marketCode ban predates this feature and
		// still correctly protects EstimatedItemPrice.Get (out of scope for this
		// plan, see spec §6) — that usecase must keep forbidding it. ForecastCurve
		// is the one caller that now legitimately accepts marketCode on a group
		// resolution (design spec §2b: it's the "viewed market" to exclude from
		// groupMarketComparison). Retry resolution with marketCode cleared to get
		// the target (marketCode itself, unmodified, is still used below for
		// groupMarketComparison's excludeMarketCode) — resolveCropTarget's
		// group-resolution checks never use marketCode for anything besides this
		// ban, so this retry is behaviorally exact, not an approximation. Both
		// resolveCropTarget calls fail/succeed before any repo I/O happens for
		// this specific check (the ban is checked before the CropGroupsByName/
		// CropsInGroup repo calls on both of resolveCropTarget's group-resolving
		// paths), so this never doubles a real query.
		target, err = resolveCropTarget(ctx, u.seed, u.repo, u.artifact, cropName, "", cropCode, groupCode)
	}
	if err != nil {
		return ForecastCurveResult{}, err
	}
	if target.Unavailable {
		return ForecastCurveResult{
			Status:                forecast.PredictionStatusUnavailable,
			ResolvedCropCode:      target.ResolvedCropCode,
			ResolvedCropGroupCode: target.ResolvedCropGroupCode,
			MarketCode:            target.MarketCode,
			DataMeta:              unavailableDataMeta(now),
		}, nil
	}

	if target.IsCrop {
		out, err := u.forecastUC.SeriesForecast(ctx, SeriesForecastInput{
			Artifact: u.artifact, MarketCode: marketCode, CropCode: target.CropCode, Origin: now,
		})
		if err != nil {
			return ForecastCurveResult{}, err
		}
		// resolvedCropCode, not cropCode: the same-named parameter is only
		// the caller's *requested* bypass code (empty on the crop_name
		// path), while this is what resolution actually settled on. Shadowing
		// would make the two look interchangeable — they aren't.
		resolvedCropCode := target.CropCode
		mc := marketCode
		result := ForecastCurveResult{
			Status:           forecast.PredictionStatusUnavailable,
			ResolvedCropCode: &resolvedCropCode,
			MarketCode:       &mc,
			ModelVersion:     out.ModelVersion,
			Points:           toCurvePoints(out.Forecast),
		}
		if p, ok := pointAtHorizon1(out.Forecast); ok {
			result.Status = p.Status
		}
		comparison, err := u.marketComparison(ctx, target.CropCode, marketCode, now)
		if err != nil {
			return ForecastCurveResult{}, err
		}
		result.MarketComparison = comparison
		// dataMeta 用 result.Status（= horizon_days==1 那個點的 status，經過
		// 上面 pointAtHorizon1 賦值）算 usabilityStatus——與頂層 status 欄位
		// 描述同一件事，snapshot 一致。
		result.DataMeta = seriesForecastDataMeta(now, result.Status, out.ModelVersion, out)
		return result, nil
	}

	out, err := u.forecastUC.GroupForecast(ctx, GroupForecastInput{
		Artifact: u.artifact, GroupCode: target.GroupCode, Origin: now,
	})
	if err != nil {
		return ForecastCurveResult{}, err
	}
	resolvedGroupCode := target.GroupCode
	result := ForecastCurveResult{
		Status:                forecast.PredictionStatusUnavailable,
		ResolvedCropGroupCode: &resolvedGroupCode,
		ModelVersion:          out.ModelVersion,
		Points:                toGroupCurvePoints(out.Forecast),
	}
	horizon1, ok := groupPointAtHorizon1(out.Forecast)
	if ok {
		result.Status = horizon1.Status
	}
	crops, err := u.repo.CropsInGroup(ctx, target.GroupCode)
	if err != nil {
		return ForecastCurveResult{}, fmt.Errorf("usecase: CropsInGroup(%s): %w", target.GroupCode, err)
	}
	comparison, err := u.groupMarketComparison(ctx, crops, marketCode, now)
	if err != nil {
		return ForecastCurveResult{}, err
	}
	result.MarketComparison = comparison
	// dataMeta 的 usabilityStatus/qualityReasonCodes 純從 horizon_days==1 的
	// GroupPoint 算（ADR D2/D3）——與上面決定頂層 status 用的是同一個
	// horizon1，同一份 snapshot。!ok 理論上不可達（AggregateGroup 保證恆有
	// 7 個 horizon），仍防禦性地傳零值 GroupPoint（其 Status 零值不落在
	// GroupUsability 的 usable/degraded 分支，會落到 unusable 的
	// default-safe 行為，不會意外冒充 usable）。
	result.DataMeta = groupForecastDataMeta(now, horizon1, u.cfg.Inference.GroupMinMarketSupportCount, out.ModelVersion, out)
	return result, nil
}

func toCurvePoints(points []ForecastPoint) []CurvePoint {
	out := make([]CurvePoint, 0, len(points))
	for _, p := range points {
		out = append(out, CurvePoint{TargetDate: p.TargetDate, HorizonDays: p.HorizonDays, Status: p.Status, P10: p.P10, P50: p.P50, P90: p.P90})
	}
	return out
}

func toGroupCurvePoints(points []forecast.GroupPoint) []CurvePoint {
	out := make([]CurvePoint, 0, len(points))
	for _, p := range points {
		out = append(out, CurvePoint{TargetDate: p.TargetDate, HorizonDays: p.HorizonDays, Status: p.Status, P10: p.P10, P50: p.P50, P90: p.P90})
	}
	return out
}

// marketComparison preserves each model-supported sibling market's identity.
// Supplementary forecast or catalog failures produce unavailable with nil P50;
// they do not fail the primary result.
func (u *ForecastCurve) marketComparison(ctx context.Context, cropCode, excludeMarketCode string, now time.Time) ([]MarketComparisonEntry, error) {
	cropID := "moa:" + cropCode
	marketCodes := map[string]bool{}
	for _, series := range u.artifact.Series {
		if series.CropID == cropID {
			mc := strings.TrimPrefix(series.MarketID, "moa:")
			if mc != excludeMarketCode {
				marketCodes[mc] = true
			}
		}
	}
	codes := make([]string, 0, len(marketCodes))
	for code := range marketCodes {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	out := make([]MarketComparisonEntry, 0, len(codes))
	for _, code := range codes {
		entry := MarketComparisonEntry{MarketCode: code, MarketName: code, Status: forecast.PredictionStatusUnavailable}
		forecastOut, err := u.forecastUC.SeriesForecast(ctx, SeriesForecastInput{Artifact: u.artifact, MarketCode: code, CropCode: cropCode, Origin: now})
		if err != nil {
			u.logger.Warn("marketComparison: supplementary forecast failed", zap.String("market_code", code), zap.Error(err))
		} else if p, ok := pointAtHorizon1(forecastOut.Forecast); ok {
			entry.Status, entry.P50 = p.Status, usableComparisonPrice(p)
		}
		ref, err := u.repo.MarketByCode(ctx, code)
		if err != nil {
			u.logger.Warn("marketComparison: supplementary market lookup failed", zap.String("market_code", code), zap.Error(err))
			entry.Status, entry.P50 = forecast.PredictionStatusUnavailable, nil
		} else if ref.Name != "" {
			entry.MarketName = ref.Name
		}
		out = append(out, entry)
	}
	return out, nil
}

// groupMarketComparison averages available crop forecasts for each sibling
// market. A system failure in any contribution marks that market unavailable
// with nil P50, preserving identity without presenting a silently partial mean.
func (u *ForecastCurve) groupMarketComparison(ctx context.Context, crops []domain.CropRef, excludeMarketCode string, now time.Time) ([]MarketComparisonEntry, error) {
	marketCodes := map[string]bool{}
	for _, c := range crops {
		if !c.Active {
			continue
		}
		cropID := "moa:" + c.CropCode
		for _, series := range u.artifact.Series {
			if series.CropID == cropID {
				mc := strings.TrimPrefix(series.MarketID, "moa:")
				if mc != excludeMarketCode {
					marketCodes[mc] = true
				}
			}
		}
	}
	codes := make([]string, 0, len(marketCodes))
	for code := range marketCodes {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	out := make([]MarketComparisonEntry, 0, len(codes))
	for _, code := range codes {
		var sum float64
		var count int
		failed := false
		partial := false
		var unusableStatus forecast.PredictionStatus
		for _, c := range crops {
			if !c.Active {
				continue
			}
			if !u.artifact.HasSeries(code, c.CropCode) {
				continue
			}
			forecastOut, err := u.forecastUC.SeriesForecast(ctx, SeriesForecastInput{Artifact: u.artifact, MarketCode: code, CropCode: c.CropCode, Origin: now})
			if err != nil {
				u.logger.Warn("groupMarketComparison: SeriesForecast failed for sibling market, skipping this crop's contribution", zap.String("market_code", code), zap.String("crop_code", c.CropCode), zap.Error(err))
				failed = true
				continue
			}
			p, ok := pointAtHorizon1(forecastOut.Forecast)
			price := usableComparisonPrice(p)
			if !ok || price == nil {
				partial = true
				status := p.Status
				if !ok || status == forecast.PredictionStatusReady || status == forecast.PredictionStatusPartial {
					status = forecast.PredictionStatusInsufficientData
				}
				if unusableStatus == "" {
					unusableStatus = status
				} else if unusableStatus != status {
					unusableStatus = forecast.PredictionStatusUnavailable
				}
				continue
			}
			partial = partial || p.Status == forecast.PredictionStatusPartial
			sum += *price
			count++
		}
		entry := MarketComparisonEntry{MarketCode: code, MarketName: code, Status: forecast.PredictionStatusUnavailable, IsApproximate: true}
		ref, err := u.repo.MarketByCode(ctx, code)
		if err != nil {
			failed = true
			u.logger.Warn("groupMarketComparison: supplementary lookup failed", zap.String("market_code", code), zap.Error(err))
		}
		if ref.Name != "" {
			entry.MarketName = ref.Name
		}
		if count == 0 && !failed && unusableStatus != "" {
			entry.Status = unusableStatus
		}
		if count > 0 && !failed {
			avg := sum / float64(count)
			entry.P50 = &avg
			entry.Status = forecast.PredictionStatusReady
			if partial {
				entry.Status = forecast.PredictionStatusPartial
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

// SeriesForecast may retain model quantiles after a freshness gate overwrites
// the business status. Supplementary prices must respect both usability and
// numeric validity, rather than treating a non-nil model output as usable.
func usableComparisonPrice(p ForecastPoint) *float64 {
	if p.Status != forecast.PredictionStatusReady && p.Status != forecast.PredictionStatusPartial {
		return nil
	}
	if p.P50 == nil || math.IsNaN(*p.P50) || math.IsInf(*p.P50, 0) {
		return nil
	}
	return p.P50
}
