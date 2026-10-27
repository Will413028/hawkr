//go:build integration

package usecase_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

type stubPoultryClient struct{ rows []moa.PoultryRow }

func (s *stubPoultryClient) FetchPoultry(context.Context, moa.PoultryEndpoint, time.Time, time.Time) ([]moa.PoultryRow, error) {
	return s.rows, nil
}

var poultryDay = time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)

func TestPoultryIngest_CattyConvertedToKilogramAndOriginalKept(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	uc := usecase.NewPoultryIngestWithClient(&stubPoultryClient{rows: []moa.PoultryRow{
		{TradeDate: poultryDay, Prices: map[string]float64{
			"egg_Producer_Price": 39.5,
			"egg_Price":          42.5,
		}},
	}}, pool, newSeqTSID(930000000000000001), moa.PoultryChickenEgg, usecase.SourcePoultryChickenEgg)

	stats, err := uc.Handle(ctx, poultryDay, poultryDay)
	require.NoError(t, err)
	require.Equal(t, 2, stats.Inserted)

	require.Equal(t, 3, stats.Absent)

	q := gen.New(pool)
	cs, err := q.ListCommoditiesBySource(ctx, usecase.SourcePoultryChickenEgg)
	require.NoError(t, err)
	require.Len(t, cs, 5, "五個欄位都要建品項，即使當天沒有報價")

	byCode := map[string]gen.Commodity{}
	for _, c := range cs {
		byCode[c.CommodityCode] = c
	}
	producer := byCode["egg_Producer_Price"]
	require.Equal(t, domain.UnitCatty, producer.Unit, "原始單位必須是台斤——賣家去農業部查到的是它")
	require.NotNil(t, producer.CanonicalUnit)
	require.Equal(t, domain.UnitKilogram, *producer.CanonicalUnit)
	require.Equal(t, "farmgate", producer.PriceLayer)

	mapped, merr := q.ListCommoditiesByCategory(ctx, "1999")
	require.NoError(t, merr)
	require.Len(t, mapped, 1)
	require.Equal(t, "egg_Producer_Price", mapped[0].CommodityCode,
		"代表分類的必須是產地價，不是大運輸價")

	require.Equal(t, "wholesale_bulk", byCode["egg_Price"].PriceLayer)

	got, err := usecase.NewCommodityForecast(pool).Handle(ctx, strconv.FormatInt(producer.ID, 10), time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, got.P50)
	require.InDelta(t, 65.8333, *got.P50, 0.001, "計算基準必須是元/公斤")
	require.NotNil(t, got.OriginalP50)
	require.InDelta(t, 39.5, *got.OriginalP50, 0.001, "顯示基準要回得到來源公布的台斤價")
	require.Greater(t, *got.P50, *got.OriginalP50, "台斤換公斤必須變大（方向回歸斷言）")
}

func TestPoultryIngest_AbsentFieldsDoNotLand(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	uc := usecase.NewPoultryIngestWithClient(&stubPoultryClient{rows: []moa.PoultryRow{
		{TradeDate: poultryDay, Prices: map[string]float64{}},
	}}, pool, newSeqTSID(930000000000001001), moa.PoultryGooseDuck, usecase.SourcePoultryGooseDuck)

	stats, err := uc.Handle(ctx, poultryDay, poultryDay)
	require.NoError(t, err)
	require.Equal(t, 0, stats.Inserted)
	require.Equal(t, 4, stats.Absent)

	q := gen.New(pool)
	cs, err := q.ListCommoditiesBySource(ctx, usecase.SourcePoultryGooseDuck)
	require.NoError(t, err)
	require.Len(t, cs, 4)
	got, err := usecase.NewCommodityForecast(pool).Handle(ctx, strconv.FormatInt(cs[0].ID, 10), time.Now().UTC())
	require.NoError(t, err)
	require.Nil(t, got.P50, "整天休市不該產生任何價格")
}

func TestPoultryIngest_SameDigestSkipped(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	rows := []moa.PoultryRow{{TradeDate: poultryDay, Prices: map[string]float64{"egg_Producer_Price": 39.5}}}
	uc := usecase.NewPoultryIngestWithClient(&stubPoultryClient{rows: rows}, pool,
		newSeqTSID(930000000000002001), moa.PoultryChickenEgg, usecase.SourcePoultryChickenEgg)

	first, err := uc.Handle(ctx, poultryDay, poultryDay)
	require.NoError(t, err)
	require.Equal(t, 1, first.Inserted)

	second, err := uc.Handle(ctx, poultryDay, poultryDay)
	require.NoError(t, err)
	require.Equal(t, 0, second.Inserted)
	require.Equal(t, 1, second.Skipped)
}
