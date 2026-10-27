package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/stretchr/testify/require"
)

func TestObservedPrice_AllMarket_ExactAndNameLatestHistoryMetadata(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	older := now.AddDate(0, 0, -2)
	latest := now.AddDate(0, 0, -1)
	volume := "40.125"
	for _, name := range []bool{false, true} {
		for _, history := range []bool{false, true} {
			repo := &fakeObservedPriceRepo{daily: func(ctx context.Context, crops []string, market string, from, to, cutoff time.Time) ([]domain.DailyObservedPrice, error) {
				require.Equal(t, []string{"C"}, crops)
				require.Empty(t, market)
				require.Equal(t, now, cutoff)
				require.Equal(t, now, to)
				return []domain.DailyObservedPrice{
					{CropCode: "C", TradeDate: older, AvgPrice: "15", AvailableAt: now, SourceDigests: []string{"old"}, VolumeWeightFallback: true},
					{CropCode: "C", TradeDate: latest, AvgPrice: "17.50015", VolumeKg: &volume, AvailableAt: latest, SourceDigests: []string{"a", "b"}},
				}, nil
			}}
			uc := NewObservedPrice(wrapFakeRepo(repo), testSeedWithOneCropEntry(t, "crop", "C"), testConfig(defaultTestInference()), noopForecastUC(t))
			cropName, cropCode := "", "C"
			if name {
				cropName, cropCode = "crop", ""
			}
			resolved, rows, meta, err := uc.Get(context.Background(), cropName, "", cropCode, "", nil, history, now)
			require.NoError(t, err)
			require.True(t, resolved)
			expected := 1
			if history {
				expected = 2
			}
			require.Len(t, rows, expected)
			last := rows[len(rows)-1]
			require.Empty(t, last.MarketCode)
			require.Equal(t, "C", last.CropCode)
			require.Equal(t, 17.50015, *last.AvgPrice)
			require.Equal(t, 40.125, *last.VolumeKg)
			require.Nil(t, last.UpperPrice)
			require.Nil(t, last.MiddlePrice)
			require.Nil(t, last.LowerPrice)
			if history {
				require.Equal(t, []forecast.QualityReasonCode{forecast.QualityReasonVolumeWeightFallback}, meta.QualityReasonCodes)
				require.Equal(t, now, *meta.MaxFeatureAvailableAt)
			} else {
				require.Empty(t, meta.QualityReasonCodes)
				require.Equal(t, latest, *meta.MaxFeatureAvailableAt)
				require.Equal(t, forecast.DatasetDigestFromSourceDigests([]string{"a", "b"}), meta.DatasetDigest)
			}
		}
	}
}

func TestObservedPrice_AllMarket_EmptyAndError(t *testing.T) {
	for _, wantErr := range []error{nil, errors.New("database unavailable")} {
		repo := &fakeObservedPriceRepo{daily: func(context.Context, []string, string, time.Time, time.Time, time.Time) ([]domain.DailyObservedPrice, error) {
			return nil, wantErr
		}}
		uc := NewObservedPrice(wrapFakeRepo(repo), testSeedWithOneCropEntry(t, "crop", "C"), testConfig(defaultTestInference()), noopForecastUC(t))
		resolved, rows, meta, err := uc.Get(context.Background(), "", "", "C", "", nil, false, time.Now())
		require.True(t, resolved)
		require.Empty(t, rows)
		require.ErrorIs(t, err, wantErr)
		if wantErr == nil {
			require.Equal(t, forecast.UsabilityUnusable, meta.UsabilityStatus)
		}
	}
}

// Forecast fixtures never invoke the independent observed aggregation path.
func (f *fakeForecastRepo) DailyObservedPrices(context.Context, []string, string, time.Time, time.Time, time.Time) ([]domain.DailyObservedPrice, error) {
	panic("unexpected DailyObservedPrices call in forecast fixture")
}

func TestObservedPrice_AllMarket_PinnedDateKeepsNowCutoff(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	pinned := now.AddDate(0, 0, -20)
	repo := &fakeObservedPriceRepo{daily: func(ctx context.Context, crops []string, market string, from, to, cutoff time.Time) ([]domain.DailyObservedPrice, error) {
		require.Equal(t, pinned, from)
		require.Equal(t, pinned, to)
		require.Equal(t, now, cutoff)
		return []domain.DailyObservedPrice{{CropCode: "C", TradeDate: pinned, AvgPrice: "0", AvailableAt: now, SourceDigests: []string{"late-revision"}, VolumeWeightFallback: true}}, nil
	}}
	uc := NewObservedPrice(wrapFakeRepo(repo), testSeedWithOneCropEntry(t, "crop", "C"), testConfig(defaultTestInference()), noopForecastUC(t))
	_, rows, meta, err := uc.Get(context.Background(), "", "", "C", "", &pinned, false, now)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, float64(0), *rows[0].AvgPrice)
	require.Nil(t, rows[0].VolumeKg)
	require.Equal(t, forecast.UsabilityDegraded, meta.UsabilityStatus)
	require.Equal(t, []forecast.QualityReasonCode{forecast.QualityReasonVolumeWeightFallback}, meta.QualityReasonCodes)
}
