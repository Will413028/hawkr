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
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/ricecity"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

type stubRiceClient struct{ rows []moa.RiceRow }

func (s *stubRiceClient) FetchRice(context.Context, int, int) ([]moa.RiceRow, error) {
	return s.rows, nil
}

func riceRegistry(t *testing.T) ricecity.Registry {
	t.Helper()
	r, err := ricecity.Load(
		"../../reference/rice_city_registry.v1.csv",
		"../../reference/rice_city_registry.v1.manifest.json",
	)
	require.NoError(t, err)
	return r
}

var (
	ricePublishedDay   = time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	riceUnpublishedDay = time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
)

func riceFullDay(day time.Time, fill map[string]map[string]float64) []moa.RiceRow {
	cities := []string{
		"基隆市", "台北市", "新北市", "桃園市", "新竹市", "新竹縣", "苗栗縣", "台中市",
		"彰化縣", "南投縣", "雲林縣", "嘉義市", "嘉義縣", "台南市", "高雄市", "屏東縣",
		"宜蘭縣", "花蓮縣", "台東縣", "澎湖縣",
	}
	out := make([]moa.RiceRow, 0, len(cities))
	for _, c := range cities {
		prices := map[string]float64{}
		for k, v := range fill[c] {
			prices[k] = v
		}
		out = append(out, moa.RiceRow{TradeDate: day, CityName: c, Prices: prices})
	}
	return out
}

func TestRiceIngest_HectokilogramConvertedAndOriginalKept(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	uc := usecase.NewRiceIngestWithClient(&stubRiceClient{rows: riceFullDay(ricePublishedDay,
		map[string]map[string]float64{
			"基隆市": {"pt_1japt": 51.33, "pt_2japt": 4633.0},
		})}, pool, riceRegistry(t), newSeqTSID(940000000000000001))

	stats, err := uc.Handle(ctx, 1, 0)
	require.NoError(t, err)
	require.Equal(t, 2, stats.Inserted)
	require.Equal(t, 1, stats.Days)
	require.Equal(t, 0, stats.Unpublished)
	require.Equal(t, 0, stats.Incomplete)

	q := gen.New(pool)
	cs, err := q.ListCommoditiesBySource(ctx, usecase.SourceRice)
	require.NoError(t, err)
	require.Len(t, cs, 10, "十條序列都要建品項（零售 5＋躉售 5），即使當天沒有報價")

	byCode := map[string]gen.Commodity{}
	for _, c := range cs {
		byCode[c.CommodityCode] = c
	}
	retail := byCode["pt_1japt"]
	require.Equal(t, domain.UnitKilogram, retail.Unit)
	require.Equal(t, "retail", retail.PriceLayer)
	bulk := byCode["pt_2japt"]
	require.Equal(t, domain.UnitHectokilogram, bulk.Unit, "躉售的原始單位是元/百公斤")
	require.NotNil(t, bulk.CanonicalUnit)
	require.Equal(t, domain.UnitKilogram, *bulk.CanonicalUnit)

	require.Equal(t, "wholesale_bulk", bulk.PriceLayer)

	require.NotContains(t, byCode, "pt_4japt")

	mapped, merr := q.ListCommoditiesByCategory(ctx, "2022")
	require.NoError(t, merr)
	require.Len(t, mapped, 1)
	require.Equal(t, "pt_1japt", mapped[0].CommodityCode,
		"代表「米」的必須是稉種白米零售，不是躉售")

	fc := usecase.NewCommodityForecast(pool)
	gotBulk, err := fc.Handle(ctx, strconv.FormatInt(bulk.ID, 10), time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, gotBulk.P50)
	require.InDelta(t, 46.33, *gotBulk.P50, 0.001, "計算基準必須是元/公斤")
	require.NotNil(t, gotBulk.OriginalP50)
	require.InDelta(t, 4633.0, *gotBulk.OriginalP50, 0.01, "顯示基準要回得到來源公布的百公斤價")
	require.Less(t, *gotBulk.P50, *gotBulk.OriginalP50, "百公斤換公斤必須變小（方向回歸斷言）")

	gotRetail, err := fc.Handle(ctx, strconv.FormatInt(retail.ID, 10), time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, gotRetail.P50)
	require.InDelta(t, 51.33, *gotRetail.P50, 0.001)

	require.Greater(t, *gotRetail.P50, *gotBulk.P50, "零售必須高於躉售")
}

func TestRiceIngest_UnpublishedDayDoesNotLandAndStaysPending(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	rows := append(
		riceFullDay(ricePublishedDay, map[string]map[string]float64{"基隆市": {"pt_1japt": 51.33}}),
		riceFullDay(riceUnpublishedDay, nil)...,
	)
	uc := usecase.NewRiceIngestWithClient(&stubRiceClient{rows: rows}, pool,
		riceRegistry(t), newSeqTSID(940000000000001001))

	stats, err := uc.Handle(ctx, 2, 0)
	require.NoError(t, err)
	require.Equal(t, 2, stats.Days)
	require.Equal(t, 1, stats.Unpublished)
	require.Equal(t, 1, stats.Inserted, "尚未發布的那天不得產生任何列")

	q := gen.New(pool)
	pending, err := q.ListIncompleteCommodityIngestPartitions(ctx, gen.ListIncompleteCommodityIngestPartitionsParams{
		Source: usecase.SourceRice, CompleteStatus: domain.PartitionStatusComplete, MaxAttempts: 100,
	})
	require.NoError(t, err)
	require.Len(t, pending, 1, "尚未發布的那天必須留在待補清單，否則永遠補不回來")
	require.Equal(t, "2026-09-13", pending[0].TradeDate.Time.Format("2006-01-02"))
	require.Equal(t, domain.PartitionStatusPending, pending[0].Status)
}

func TestRiceIngest_TruncatedDayStaysPending(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	partial := riceFullDay(ricePublishedDay,
		map[string]map[string]float64{"基隆市": {"pt_1japt": 51.33}})[:5]
	uc := usecase.NewRiceIngestWithClient(&stubRiceClient{rows: partial}, pool,
		riceRegistry(t), newSeqTSID(940000000000002001))

	stats, err := uc.Handle(ctx, 1, 0)
	require.NoError(t, err)
	require.Equal(t, 1, stats.Incomplete)
	require.Equal(t, 1, stats.Inserted, "已取得的列是真資料，照樣寫入")

	q := gen.New(pool)
	pending, err := q.ListIncompleteCommodityIngestPartitions(ctx, gen.ListIncompleteCommodityIngestPartitionsParams{
		Source: usecase.SourceRice, CompleteStatus: domain.PartitionStatusComplete, MaxAttempts: 100,
	})
	require.NoError(t, err)
	require.Len(t, pending, 1, "被分頁切半的那天不算完整，必須留在待補清單")
	require.Equal(t, domain.PartitionStatusPending, pending[0].Status)
}

func TestRiceIngest_UnknownCityFailsClosed(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	uc := usecase.NewRiceIngestWithClient(&stubRiceClient{rows: []moa.RiceRow{
		{TradeDate: ricePublishedDay, CityName: "金門縣", Prices: map[string]float64{"pt_1japt": 51.33}},
	}}, pool, riceRegistry(t), newSeqTSID(940000000000003001))

	_, err := uc.Handle(ctx, 1, 0)
	require.ErrorIs(t, err, ricecity.ErrUnknownMarket)
}

func TestRiceIngest_SameDigestSkipped(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	rows := riceFullDay(ricePublishedDay,
		map[string]map[string]float64{"基隆市": {"pt_1japt": 51.33, "pt_2japt": 4633.0}})
	uc := usecase.NewRiceIngestWithClient(&stubRiceClient{rows: rows}, pool,
		riceRegistry(t), newSeqTSID(940000000000004001))

	first, err := uc.Handle(ctx, 1, 0)
	require.NoError(t, err)
	require.Equal(t, 2, first.Inserted)

	second, err := uc.Handle(ctx, 1, 0)
	require.NoError(t, err)
	require.Equal(t, 0, second.Inserted)
	require.Equal(t, 2, second.Skipped)
}

func TestRiceIngest_SameCityDifferentSeriesAreDistinctRows(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	rows := riceFullDay(ricePublishedDay, map[string]map[string]float64{
		"基隆市": {"pt_1japt": 51.33, "pt_1glutrt": 62.0},
		"台北市": {"pt_1japt": 53.10},
	})
	uc := usecase.NewRiceIngestWithClient(&stubRiceClient{rows: rows}, pool,
		riceRegistry(t), newSeqTSID(940000000000005001))

	stats, err := uc.Handle(ctx, 1, 0)
	require.NoError(t, err)
	require.Equal(t, 3, stats.Inserted)

	q := gen.New(pool)
	cs, err := q.ListCommoditiesBySource(ctx, usecase.SourceRice)
	require.NoError(t, err)
	var japtID int64
	for _, c := range cs {
		if c.CommodityCode == "pt_1japt" {
			japtID = c.ID
		}
	}
	require.NotZero(t, japtID)

	got, err := usecase.NewCommodityForecast(pool).Handle(ctx, strconv.FormatInt(japtID, 10), time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, 2, got.MarketCount, "兩個縣市各一列")
	require.NotNil(t, got.P50)
	require.InDelta(t, (51.33+53.10)/2, *got.P50, 0.001)
	require.Nil(t, got.TotalQuantity, "糧價來源不提供交易量，不得補 0")
}
