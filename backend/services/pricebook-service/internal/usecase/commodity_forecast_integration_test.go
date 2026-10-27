//go:build integration

package usecase_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

func dateParamU(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func tsParamU(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func numericU(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	n, err := postgres.NumericFromString(s)
	require.NoError(t, err)
	return n
}

func TestCommodityForecast_VolumeWeightedNationalAverage(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	uc := usecase.NewPorkIngestWithClient(&stubPorkClient{records: []moa.PorkRecord{
		porkRow("雲林縣", 300, 100), porkRow("彰化縣", 100, 120),
	}}, pool, porkRegistry(t), newSeqTSID(910000000000000001))
	_, err := uc.Handle(ctx, porkDay)
	require.NoError(t, err)

	q := gen.New(pool)
	cs, err := q.ListCommoditiesBySource(ctx, usecase.SourcePork)
	require.NoError(t, err)
	require.Len(t, cs, 1)

	got, err := usecase.NewCommodityForecast(pool).Handle(ctx, strconv.FormatInt(cs[0].ID, 10), time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, domain.PriceLayerLiveAnimal, got.PriceLayer, "毛豬必須標成活體層級")
	require.Equal(t, domain.ForecastDerivationSourcePassthrough, got.Derivation)
	require.NotNil(t, got.P50)
	require.InDelta(t, 105.0, *got.P50, 0.001, "必須以頭數加權，不是等權平均（等權會得到 110）")
	require.Equal(t, 2, got.MarketCount)
	require.Equal(t, "head", got.QuantityUnit)
	require.Equal(t, porkDay, got.SourceDate)
	require.Equal(t, porkDay.AddDate(0, 0, 1), got.TargetDate, "來源日的隔日才是參考日")
}

func TestCommodityForecast_EqualWeightFallbackWhenNoQuantity(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	canonicalUnit := domain.CanonicalUnitFor(domain.UnitKilogram)
	id, err := q.UpsertCommodity(ctx, gen.UpsertCommodityParams{
		ID: 910000000000001001, Source: "MOA_TEST", CommodityCode: "x", Name: "測試",
		PriceLayer: string(domain.PriceLayerFarmgate), Unit: domain.UnitKilogram,
		CanonicalUnit: &canonicalUnit,
	})
	require.NoError(t, err)
	for i, price := range []string{"40.0000", "60.0000"} {
		require.NoError(t, q.InsertCommodityObservation(ctx, gen.InsertCommodityObservationParams{
			ID: 910000000000002001 + int64(i), CommodityID: id, MarketCode: nil,
			TradeDate: dateParamU(porkDay), RevisionNo: int32(i + 1),
			AvailableAt:    tsParamU(time.Now().UTC()),
			AvgPrice:       numericU(t, price),
			CanonicalPrice: numericU(t, price),
			Source:         "MOA_TEST", SourceDigest: "d" + strconv.Itoa(i),
		}))
	}

	got, err := usecase.NewCommodityForecast(pool).Handle(ctx, strconv.FormatInt(id, 10), time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, got.P50)

	require.InDelta(t, 60.0, *got.P50, 0.001)
	require.Nil(t, got.TotalQuantity, "沒有量的支撐時不得回報加總量")
}

func TestCommodityForecast_RespectsAvailableAtCutoff(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	uc := usecase.NewPorkIngestWithClient(&stubPorkClient{records: []moa.PorkRecord{
		porkRow("雲林縣", 300, 100),
	}}, pool, porkRegistry(t), newSeqTSID(910000000000003001))
	_, err := uc.Handle(ctx, porkDay)
	require.NoError(t, err)

	q := gen.New(pool)
	cs, err := q.ListCommoditiesBySource(ctx, usecase.SourcePork)
	require.NoError(t, err)

	past := time.Now().UTC().Add(-24 * time.Hour)
	got, err := usecase.NewCommodityForecast(pool).Handle(ctx, strconv.FormatInt(cs[0].ID, 10), past)
	require.NoError(t, err)
	require.Nil(t, got.P50, "cutoff 之前不可見的資料不得被算進去")
	require.Equal(t, 0, got.MarketCount)
}

func TestCommodityForecast_UnknownCommodityFailsClosed(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	_, err := usecase.NewCommodityForecast(pool).Handle(ctx, "not-a-number", time.Now().UTC())
	require.ErrorIs(t, err, usecase.ErrCommodityNotFound)

	_, err = usecase.NewCommodityForecast(pool).Handle(ctx, "919999999999999999", time.Now().UTC())
	require.ErrorIs(t, err, usecase.ErrCommodityNotFound)
}

func TestCommodityForecast_CattySourceConvertsToKilogram(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	canonicalUnit := domain.CanonicalUnitFor(domain.UnitCatty)
	require.Equal(t, domain.UnitKilogram, canonicalUnit)
	id, err := q.UpsertCommodity(ctx, gen.UpsertCommodityParams{
		ID: 920000000000001001, Source: "MOA_TEST_EGG", CommodityCode: "egg_Producer_Price",
		Name: "雞蛋（產地價）", PriceLayer: string(domain.PriceLayerFarmgate),
		Unit: domain.UnitCatty, CanonicalUnit: &canonicalUnit,
	})
	require.NoError(t, err)

	canonical, ok := domain.CanonicalPricePerKg(42.5, domain.UnitCatty)
	require.True(t, ok)
	require.NoError(t, q.InsertCommodityObservation(ctx, gen.InsertCommodityObservationParams{
		ID: 920000000000002001, CommodityID: id, MarketCode: nil,
		TradeDate: dateParamU(porkDay), RevisionNo: 1, AvailableAt: tsParamU(time.Now().UTC()),
		AvgPrice:       numericU(t, "42.5000"),
		CanonicalPrice: numericU(t, strconv.FormatFloat(canonical, 'f', 4, 64)),
		Source:         "MOA_TEST_EGG", SourceDigest: "d1",
	}))

	got, err := usecase.NewCommodityForecast(pool).Handle(ctx, strconv.FormatInt(id, 10), time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, domain.UnitCatty, got.Unit, "原始單位必須保留——賣家去農業部查到的是台斤價")
	require.Equal(t, domain.UnitKilogram, got.CanonicalUnit)
	require.NotNil(t, got.P50)
	require.InDelta(t, 70.8333, *got.P50, 0.001, "計算基準必須是元/公斤，不是台斤價原值")
	require.NotNil(t, got.OriginalP50)
	require.InDelta(t, 42.5, *got.OriginalP50, 0.001, "顯示基準要能回到來源公布的數字")
	require.Greater(t, *got.P50, *got.OriginalP50, "台斤換公斤必須變大（方向寫反的回歸斷言）")
}
