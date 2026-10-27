package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
)

// ErrAmbiguousCropGroup: the seed table resolved a crop name to a
// crop_group name that matches more than one crop_group row (the known
// ambiguity from group_code's category-inclusive hash — see
// domain.CropGroupMatch / cmd/pricebook-seed-check's own handling of the
// same condition). The caller must not guess; this surfaces as an error,
// not a business "not found".
var ErrAmbiguousCropGroup = errors.New("usecase: crop group name matches more than one crop_group row")

// ObservedPrice serves GetObservedPrice: resolve a seller-facing crop name
// via P4's mapping seed table, then fetch the matching raw observed-price
// rows from domain.ForecastRepository.
type ObservedPrice struct {
	repo               domain.ForecastRepository
	seed               mapping.Seed
	observedWindowDays int
	forecastUC         *Forecast
	// freshnessCfg（issue #2249 Slice A dataMeta 前置，2026-08-07 加）：
	// dataMeta.freshnessStatus 重用既有 forecast.FreshnessConfig.Classify，
	// 不新開一條新鮮度邏輯——同 cfg.Inference.FreshWithinHours/
	// StaleWithinHours，SeriesForecast/GroupForecast 用的是同一份門檻。
	freshnessCfg forecast.FreshnessConfig
}

// NewObservedPrice wires the repository + mapping seed together.
// observedWindowDays comes from cfg.Inference.ObservedWindowDays — the same
// "近 30 日" default the design doc §4 PriceSeries.observed field uses.
// forecastUC is used by the group branch — see Get's group branch
// (groupObserved) for why (bug fix: aggregate the raw multi-market grid
// instead of returning it zigzag-shaped; every group path now uses this
// aggregation — both history=true and history=false, and both all-markets
// and a specific market_code — see groupObserved's own doc comment).
func NewObservedPrice(repo domain.ForecastRepository, seed mapping.Seed, cfg *config.Config, forecastUC *Forecast) *ObservedPrice {
	return &ObservedPrice{
		repo: repo, seed: seed, observedWindowDays: cfg.Inference.ObservedWindowDays, forecastUC: forecastUC,
		freshnessCfg: freshnessConfigFrom(cfg.Inference),
	}
}

// Get resolves the request target and returns the raw observation rows.
// The target is named by exactly one of cropName (resolved via the seed
// table) / cropCode / groupCode (direct lookups that skip the seed table
// entirely — see resolveCropTarget's doc comment for why that bypass
// exists). resolved=false means the crop NAME didn't match the seed table
// at all — distinct from resolved=true with a nil/empty observations slice
// (matched, but no price data in range). The two bypass paths are resolved
// by definition, so they never return resolved=false.
//
// This usecase deliberately does not call resolveCropTarget: its downstream
// shape is raw observation rows, not a forecast target, so resolvedTarget's
// artifact-coverage pre-check is meaningless here (observations exist
// independent of whether a model was trained). Only the *caller contract*
// is shared, via validateResolutionInputs.
//
// cutoff passed to the repository is always `now`, never `to`: `to` bounds
// which trade_dates to search (the window() helper may pin it to a past
// date), while cutoff bounds which revisions are visible "as of now" —
// collapsing the two would make a pinned-past-trade_date query silently
// exclude legitimately-late-arriving data (review Finding I1, 2026-07-31;
// see domain.ForecastRepository's doc comment for the full rationale).
//
// meta is issue #2249 Slice A's additive dataMeta — always populated (never
// a zero-value DataMeta), even when resolved=false (a legitimate business
// "not found" outcome, not an error: freshnessStatus/usabilityStatus are
// unavailable/unusable, describing "nothing to report", not fake values).
// It is always built from the exact rows this call is about to return (the
// snapshot-consistency rule), never from a separate query.
func (u *ObservedPrice) Get(ctx context.Context, cropName, marketCode, cropCode, groupCode string, tradeDate *time.Time, history bool, now time.Time) (resolved bool, observations []domain.ObservedPrice, meta DataMeta, err error) {
	if err := validateResolutionInputs(cropName, cropCode, groupCode); err != nil {
		return false, nil, DataMeta{}, err
	}

	from, to := u.window(tradeDate, now)

	if cropCode != "" {
		if marketCode == "" {
			obs, meta, err := u.allMarketObserved(ctx, cropCode, from, to, now, tradeDate, history)
			return true, obs, meta, err
		}
		obs, err := u.repo.ObservedPricesByCrop(ctx, marketCode, cropCode, from, to, now)
		if err != nil {
			return true, nil, DataMeta{}, err
		}
		reduced := u.latestPerSeries(obs, tradeDate != nil || history)
		return true, reduced, u.buildDataMeta(now, reduced), nil
	}

	if groupCode != "" {
		obs, groupMeta, err := u.groupObserved(ctx, groupCode, marketCode, from, to, now, tradeDate, history)
		if err != nil {
			return true, nil, DataMeta{}, err
		}
		return true, obs, groupMeta, nil
	}

	entry, found := u.seed.Resolve(cropName)
	if !found {
		return false, nil, unavailableDataMeta(now), nil
	}

	if entry.CropCode != "" {
		if marketCode == "" {
			obs, meta, err := u.allMarketObserved(ctx, entry.CropCode, from, to, now, tradeDate, history)
			return true, obs, meta, err
		}
		obs, err := u.repo.ObservedPricesByCrop(ctx, marketCode, entry.CropCode, from, to, now)
		if err != nil {
			return true, nil, DataMeta{}, err
		}
		reduced := u.latestPerSeries(obs, tradeDate != nil || history)
		return true, reduced, u.buildDataMeta(now, reduced), nil
	}

	matches, err := u.repo.CropGroupsByName(ctx, entry.CropGroupName)
	if err != nil {
		return true, nil, DataMeta{}, err
	}
	if len(matches) == 0 {
		return true, nil, unavailableDataMeta(now), nil
	}
	if len(matches) > 1 {
		return true, nil, DataMeta{}, ErrAmbiguousCropGroup
	}

	obs, groupMeta, err := u.groupObserved(ctx, matches[0].GroupCode, marketCode, from, to, now, tradeDate, history)
	if err != nil {
		return true, nil, DataMeta{}, err
	}
	return true, obs, groupMeta, nil
}

// buildDataMeta builds dataMeta from the exact rows about to be returned —
// the non-aggregated path (crop lookup only — every group lookup is now
// aggregated via groupObserved, regardless of history/market_code) returns
// real domain.ObservedPrice rows carrying their own
// Source/SourceDigest/AvailableAt (repository-level fields, see
// domain.ObservedPrice), so this can read them directly.
func (u *ObservedPrice) buildDataMeta(now time.Time, obs []domain.ObservedPrice) DataMeta {
	maxAvailableAt := observedRowsMaxAvailableAt(obs)
	latestTradeDate := observedRowsLatestTradeDate(obs)
	digest := forecast.DatasetDigestFromSourceDigests(observedRowsSourceDigests(obs))
	return observedDataMeta(now, u.freshnessCfg, maxAvailableAt, latestTradeDate, digest)
}

// groupObserved is the crop_group observation path, shared by Get's two
// entry points into it: cropName that resolved to a group via the seed
// table, and a group_code supplied directly. Always aggregates via
// GroupObservedSeries — for both history=true and history=false, and for
// both marketCode=="" (all markets) and a specific market (design spec §1b:
// GroupObservedSeriesInput.MarketCode reuses the same two-stage algorithm
// unchanged, degenerating correctly to a single-market blend). There is no
// longer a raw-grid fallback: a crop_group's raw observed-price grid always
// has at least one row per (crop, date) even within one market, and
// plotting/reporting that unaggregated has the same "which row is 'the'
// value" problem regardless of whether a market filter is applied.
func (u *ObservedPrice) groupObserved(ctx context.Context, groupCode, marketCode string, from, to, now time.Time, tradeDate *time.Time, history bool) ([]domain.ObservedPrice, DataMeta, error) {
	out, err := u.forecastUC.GroupObservedSeries(ctx, GroupObservedSeriesInput{
		GroupCode: groupCode, MarketCode: marketCode, From: from, To: to, Now: now,
	})
	if err != nil {
		return nil, DataMeta{}, err
	}
	// out.Points is sorted ascending by TradeDate (AggregateGroupObservations
	// sorts its dates before building the slice) — so once nil-Value points
	// are filtered out, the last element is the latest one.
	observations := make([]domain.ObservedPrice, 0, len(out.Points))
	for _, p := range out.Points {
		if p.Value == nil {
			continue
		}
		observations = append(observations, domain.ObservedPrice{TradeDate: p.TradeDate.ToTime(), AvgPrice: p.Value})
	}
	if !history && tradeDate == nil && len(observations) > 0 {
		observations = observations[len(observations)-1:]
	}
	digest := forecast.DatasetDigestFromSourceDigests(out.SourceDigests)
	var latestTradeDate *forecast.Date
	if len(out.Points) > 0 {
		for _, p := range out.Points {
			if p.Value == nil {
				continue
			}
			d := p.TradeDate
			if latestTradeDate == nil || d.After(*latestTradeDate) {
				latestTradeDate = &d
			}
		}
	}
	meta := observedDataMeta(now, u.freshnessCfg, out.MaxAvailableAt, latestTradeDate, digest)
	return observations, meta, nil
}

// window returns the [from, to] query bound: an exact trade_date pins both
// ends to that single day; otherwise it's the trailing observedWindowDays
// ending at now.
func (u *ObservedPrice) window(tradeDate *time.Time, now time.Time) (from, to time.Time) {
	if tradeDate != nil {
		return *tradeDate, *tradeDate
	}
	return now.AddDate(0, 0, -u.observedWindowDays), now
}

// latestPerSeries reduces the (market,crop) x trade_date grid the
// repository returns down to one row per (market,crop) — the most recent
// trade_date — UNLESS the caller pinned an exact trade_date (pinExact),
// in which case every row already matches that single date and no
// reduction is needed.
//
// Both return paths are sorted deterministically by (MarketCode, CropCode)
// before returning (review Finding I3, 2026-07-31): the reduction path
// builds its result by ranging over a Go map, whose iteration order is
// deliberately randomized per range — so identical inputs could otherwise
// produce differently-ordered output across calls. The pinned passthrough
// path is sorted too, for consistency between the two request modes (it
// would otherwise depend on incidental SQL row order). The consumer
// (ai-service, an LLM prompt builder) needs reproducible ordering for
// incident-triage response diffs.
func (u *ObservedPrice) latestPerSeries(obs []domain.ObservedPrice, pinExact bool) []domain.ObservedPrice {
	if pinExact || len(obs) == 0 {
		sortObservedPrices(obs)
		return obs
	}
	latest := make(map[string]domain.ObservedPrice, len(obs))
	for _, o := range obs {
		key := o.MarketCode + "|" + o.CropCode
		if existing, ok := latest[key]; !ok || o.TradeDate.After(existing.TradeDate) {
			latest[key] = o
		}
	}
	out := make([]domain.ObservedPrice, 0, len(latest))
	for _, o := range latest {
		out = append(out, o)
	}
	sortObservedPrices(out)
	return out
}

// sortObservedPrices sorts in place by (MarketCode, CropCode, TradeDate).
//
// TradeDate is a review Finding I1 (2026-08-02) addition: before the
// history=true field existed, (MarketCode, CropCode) was effectively a
// unique key (latestPerSeries had already reduced to one row per key, or a
// pinned trade_date meant every row shared the same date). history=true
// made a third case reachable where many rows share the same
// (MarketCode, CropCode) — the full trailing observed_window_days window,
// unreduced — and sort.Slice is not stable, so without this tiebreaker
// their relative order would be unspecified, discarding the repository's
// own `ORDER BY ..., trade_date, revision_no DESC`.
func sortObservedPrices(obs []domain.ObservedPrice) {
	sort.Slice(obs, func(i, j int) bool {
		if obs[i].MarketCode != obs[j].MarketCode {
			return obs[i].MarketCode < obs[j].MarketCode
		}
		if obs[i].CropCode != obs[j].CropCode {
			return obs[i].CropCode < obs[j].CropCode
		}
		return obs[i].TradeDate.Before(obs[j].TradeDate)
	})
}

// allMarketObserved has an explicit crop-wide path, independent of model
// coverage. The repository keeps decimal precision until this legacy double
// response boundary. Only the returned dates contribute freshness and digest.
func (u *ObservedPrice) allMarketObserved(ctx context.Context, cropCode string, from, to, now time.Time, tradeDate *time.Time, history bool) ([]domain.ObservedPrice, DataMeta, error) {
	rows, err := u.repo.DailyObservedPrices(ctx, []string{cropCode}, "", from, to, now)
	if err != nil {
		return nil, DataMeta{}, err
	}
	if !history && tradeDate == nil && len(rows) > 0 {
		rows = rows[len(rows)-1:]
	}
	observations := make([]domain.ObservedPrice, 0, len(rows))
	var digests []string
	var maxAvailableAt *time.Time
	var latestDate *forecast.Date
	fallback := false
	for _, row := range rows {
		price, err := strconv.ParseFloat(row.AvgPrice, 64)
		if err != nil {
			return nil, DataMeta{}, fmt.Errorf("all-market observed price: %w", err)
		}
		var volume *float64
		if row.VolumeKg != nil {
			v, err := strconv.ParseFloat(*row.VolumeKg, 64)
			if err != nil {
				return nil, DataMeta{}, fmt.Errorf("all-market observed volume: %w", err)
			}
			volume = &v
		}
		observations = append(observations, domain.ObservedPrice{CropCode: row.CropCode, TradeDate: row.TradeDate, AvgPrice: &price, VolumeKg: volume})
		digests = append(digests, row.SourceDigests...)
		if !row.AvailableAt.IsZero() && (maxAvailableAt == nil || row.AvailableAt.After(*maxAvailableAt)) {
			at := row.AvailableAt
			maxAvailableAt = &at
		}
		date := forecast.DateFromTime(row.TradeDate)
		if latestDate == nil || date.After(*latestDate) {
			latestDate = &date
		}
		fallback = fallback || row.VolumeWeightFallback
	}
	meta := observedDataMeta(now, u.freshnessCfg, maxAvailableAt, latestDate, forecast.DatasetDigestFromSourceDigests(digests))
	if fallback {
		meta.QualityReasonCodes = []forecast.QualityReasonCode{forecast.QualityReasonVolumeWeightFallback}
		if meta.UsabilityStatus == forecast.UsabilityUsable {
			meta.UsabilityStatus = forecast.UsabilityDegraded
		}
	}
	return observations, meta, nil
}
