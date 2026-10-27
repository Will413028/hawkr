package usecase

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

func TestTrendSummaryDataMetaAllReadyIsUsable(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	cfg := forecast.FreshnessConfig{FreshWithin: 36 * time.Hour, StaleWithin: 72 * time.Hour}
	meta := trendSummaryDataMeta(now, cfg, []forecast.PredictionStatus{forecast.PredictionStatusReady, forecast.PredictionStatusReady}, &now, nil, nil)
	if meta.UsabilityStatus != forecast.UsabilityUsable {
		t.Errorf("UsabilityStatus = %s, want usable", meta.UsabilityStatus)
	}
	if meta.ModelVersion != nil {
		t.Error("ModelVersion must be nil — trend-summary is observed-history only, never a forecast model")
	}
}

func TestTrendSummaryDataMetaWorstCaseRollup(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	cfg := forecast.FreshnessConfig{FreshWithin: 36 * time.Hour, StaleWithin: 72 * time.Hour}
	// One ready period, one insufficient_data period — rollup must reflect
	// the worse of the two, not just the last one evaluated.
	meta := trendSummaryDataMeta(now, cfg, []forecast.PredictionStatus{forecast.PredictionStatusReady, forecast.PredictionStatusInsufficientData}, &now, nil, nil)
	if meta.UsabilityStatus != forecast.UsabilityDegraded && meta.UsabilityStatus != forecast.UsabilityUnusable {
		t.Errorf("UsabilityStatus = %s, want degraded or unusable (worst-case rollup)", meta.UsabilityStatus)
	}
}

func TestTrendSummaryDataMetaEmptyPointsIsUnusable(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	cfg := forecast.FreshnessConfig{FreshWithin: 36 * time.Hour, StaleWithin: 72 * time.Hour}
	meta := trendSummaryDataMeta(now, cfg, nil, nil, nil, nil)
	if meta.UsabilityStatus != forecast.UsabilityUnusable {
		t.Errorf("UsabilityStatus = %s, want unusable for zero periods", meta.UsabilityStatus)
	}
}

func TestTrendSummaryDataMetaStaleFreshnessDowngradesUsableToDegraded(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	cfg := forecast.FreshnessConfig{FreshWithin: 36 * time.Hour, StaleWithin: 72 * time.Hour}
	// 48h before now: past FreshWithin (36h) but inside StaleWithin (72h) — classifies stale.
	staleAt := now.Add(-48 * time.Hour)
	// All periods individually ready (would roll up to usable on their own) so the
	// only thing that can produce a downgrade here is the freshness fold-in.
	meta := trendSummaryDataMeta(now, cfg, []forecast.PredictionStatus{forecast.PredictionStatusReady, forecast.PredictionStatusReady}, &staleAt, nil, nil)
	if meta.FreshnessStatus != forecast.FreshnessStale {
		t.Fatalf("FreshnessStatus = %s, want stale (test setup invariant)", meta.FreshnessStatus)
	}
	if meta.UsabilityStatus != forecast.UsabilityDegraded {
		t.Errorf("UsabilityStatus = %s, want degraded — stale freshness must downgrade an otherwise-usable rollup", meta.UsabilityStatus)
	}
}

func TestTrendSummaryDataMetaUnavailableFreshnessForcesUnusable(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	cfg := forecast.FreshnessConfig{FreshWithin: 36 * time.Hour, StaleWithin: 72 * time.Hour}
	// maxAvailableAt=nil → Classify returns unavailable regardless of thresholds.
	meta := trendSummaryDataMeta(now, cfg, []forecast.PredictionStatus{forecast.PredictionStatusReady}, nil, nil, nil)
	if meta.FreshnessStatus != forecast.FreshnessUnavailable {
		t.Fatalf("FreshnessStatus = %s, want unavailable (test setup invariant)", meta.FreshnessStatus)
	}
	if meta.UsabilityStatus != forecast.UsabilityUnusable {
		t.Errorf("UsabilityStatus = %s, want unusable — unavailable freshness must force unusable unconditionally, even with an all-ready period rollup", meta.UsabilityStatus)
	}
}
