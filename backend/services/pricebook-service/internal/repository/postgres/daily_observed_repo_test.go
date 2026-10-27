//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	pbpostgres "github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/stretchr/testify/require"
)

func TestDailyObservedPrices_NumericWeightsRevisionAndEligibility(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	crop := mustUpsertCrop(ctx, t, q, "C", "crop")
	a := mustUpsertMarket(ctx, t, q, "A", "a")
	b := mustUpsertMarket(ctx, t, q, "B", "b")
	zeroWeight := mustUpsertMarket(ctx, t, q, "Z", "zero weight")
	inactive := mustUpsertMarket(ctx, t, q, "X", "inactive")
	_, err := pool.Exec(ctx, "UPDATE market SET active = false WHERE id = $1", inactive)
	require.NoError(t, err)
	day := mustParseDate(t, "2026-09-01")
	cutoff := day.Add(48 * time.Hour)
	insert := func(market int64, date time.Time, rev int32, price, volume *string, at time.Time, digest string) {
		p := gen.InsertPriceObservationParams{ID: opNextID(), MarketID: market, CropID: crop, TradeDate: pgxutil.DateParam(&date), RevisionNo: rev, AvailableAt: pgxutil.TimestamptzParam(at), Source: "MOA", SourceDigest: digest}
		if price != nil {
			p.AvgPrice, err = pbpostgres.NumericFromString(*price)
			require.NoError(t, err)
		}
		if volume != nil {
			p.VolumeKg, err = pbpostgres.NumericFromString(*volume)
			require.NoError(t, err)
		}
		require.NoError(t, q.InsertPriceObservation(ctx, p))
	}
	str := func(s string) *string { return &s }
	insert(a, day, 1, str("10.0001"), str("1"), day, "old")
	insert(a, day, 2, str("10.0003"), str("1"), day.Add(time.Hour), "a")
	insert(a, day, 3, str("999"), str("1"), cutoff.Add(time.Hour), "future")
	insert(b, day, 1, str("20.0001"), str("3"), day, "b")
	insert(zeroWeight, day, 1, str("800"), str("0"), cutoff, "zero-weight-excluded")
	insert(inactive, day, 1, str("900"), str("900"), day, "inactive")
	next := day.AddDate(0, 0, 1)
	insert(a, next, 1, str("10"), str("0"), next, "zero")
	insert(b, next, 1, str("20"), nil, next, "missing-volume")
	repo := pbpostgres.NewForecastRepository(pool)
	rows, err := repo.DailyObservedPrices(ctx, []string{"C"}, "", day, next, cutoff)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "17.5001500000000000", rows[0].AvgPrice)
	require.Equal(t, "4.000", *rows[0].VolumeKg)
	require.Equal(t, []string{"A", "B"}, rows[0].MarketCodes)
	require.ElementsMatch(t, []string{"a", "b"}, rows[0].SourceDigests)
	require.Equal(t, day.Add(time.Hour), rows[0].AvailableAt)
	require.False(t, rows[0].VolumeWeightFallback)
	require.Equal(t, "15.0000000000000000", rows[1].AvgPrice)
	require.True(t, rows[1].VolumeWeightFallback)
	require.Equal(t, "0.000", *rows[1].VolumeKg)
	// Apply eligibility after revisions; a NULL correction must not resurrect
	// an older price. Unknown volume remains NULL, not zero.
	insert(a, next, 2, nil, str("100"), next.Add(time.Hour), "null-price")
	rows, err = repo.DailyObservedPrices(ctx, []string{"C"}, "", next, next, cutoff)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Nil(t, rows[0].VolumeKg)
	require.Equal(t, []string{"B"}, rows[0].MarketCodes)
	require.Equal(t, []string{"missing-volume"}, rows[0].SourceDigests)
	single, err := repo.DailyObservedPrices(ctx, []string{"C"}, "A", day, day, cutoff)
	require.NoError(t, err)
	require.Len(t, single, 1)
	require.Equal(t, "10.0003000000000000", single[0].AvgPrice)
	closed, err := repo.DailyObservedPrices(ctx, []string{"C"}, "", next.AddDate(0, 0, 1), next.AddDate(0, 0, 1), cutoff)
	require.NoError(t, err)
	require.Empty(t, closed)
	// Batch another crop with a repeating decimal. This assertion would lose
	// digits if NUMERIC were converted to float64 and rendered back to string.
	otherCrop := mustUpsertCrop(ctx, t, q, "D", "other")
	savedCrop := crop
	crop = otherCrop
	insert(a, day, 1, str("1"), str("1"), day, "d-a")
	insert(b, day, 1, str("2"), str("2"), day, "d-b")
	crop = savedCrop
	batch, err := repo.DailyObservedPrices(ctx, []string{"D", "C"}, "", day, day, cutoff)
	require.NoError(t, err)
	require.Len(t, batch, 2)
	require.Equal(t, "C", batch[0].CropCode)
	require.Equal(t, "D", batch[1].CropCode)
	require.Equal(t, "1.6666666666666667", batch[1].AvgPrice)

	_, err = pool.Exec(ctx, "UPDATE crop SET active = false WHERE id = $1", crop)
	require.NoError(t, err)
	rows, err = repo.DailyObservedPrices(ctx, []string{"C"}, "", day, next, cutoff)
	require.NoError(t, err)
	require.Empty(t, rows)
}
