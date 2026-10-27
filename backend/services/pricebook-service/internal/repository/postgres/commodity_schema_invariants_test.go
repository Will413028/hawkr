//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

const (
	testCommodityPorkID  int64 = 700000000000002001
	testCommodityEggFG   int64 = 700000000000002002
	testCommodityEggWB   int64 = 700000000000002003
	testCommodityObsBase int64 = 700000000000003000
)

func dateParam(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func tsParam(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func upsertCommodity(ctx context.Context, t *testing.T, q *gen.Queries, id int64, code, layer string, _ *string) int64 {
	t.Helper()
	got, err := q.UpsertCommodity(ctx, gen.UpsertCommodityParams{
		ID: id, Source: "MOA_TEST", CommodityCode: code, Name: "測試品項",
		PriceLayer: layer, Unit: "kg",
	})
	require.NoError(t, err)
	return got
}

func TestCommodity_SameCodeDifferentLayer_Coexists(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)

	fg := upsertCommodity(ctx, t, q, testCommodityEggFG, "egg", "farmgate", nil)
	wb := upsertCommodity(ctx, t, q, testCommodityEggWB, "egg", "wholesale_bulk", nil)

	require.NotEqual(t, fg, wb, "同 code 不同層級必須是兩列")
}

func TestCommodity_UndefinedPriceLayer_Rejected(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)

	_, err := q.UpsertCommodity(ctx, gen.UpsertCommodityParams{
		ID: testCommodityPorkID, Source: "MOA_TEST", CommodityCode: "x",
		Name: "n", PriceLayer: "futures", Unit: "kg",
	})
	require.Error(t, err, "未定義的層級必須被 commodity_price_layer_check 擋下")
}

func TestCommodityObservation_NationalSeriesDuplicateRejected(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)

	id := upsertCommodity(ctx, t, q, testCommodityEggFG, "egg", "farmgate", nil)
	day := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	insert := func(obsID int64, rev int32) error {
		return q.InsertCommodityObservation(ctx, gen.InsertCommodityObservationParams{
			ID: obsID, CommodityID: id, MarketCode: nil, TradeDate: dateParam(day),
			RevisionNo: rev, AvailableAt: tsParam(time.Now()),
			Source: "MOA_TEST", SourceDigest: "sha256:a",
		})
	}
	require.NoError(t, insert(testCommodityObsBase+1, 1))
	require.Error(t, insert(testCommodityObsBase+2, 1), "同格同 revision 必須被擋下")
	require.NoError(t, insert(testCommodityObsBase+3, 2), "官方修訂（新 revision）必須可寫入")
}

func TestCommodityObservation_LatestRevisionMatchesNullMarket(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)

	id := upsertCommodity(ctx, t, q, testCommodityEggFG, "egg", "farmgate", nil)
	day := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	require.NoError(t, q.InsertCommodityObservation(ctx, gen.InsertCommodityObservationParams{
		ID: testCommodityObsBase + 11, CommodityID: id, MarketCode: nil, TradeDate: dateParam(day),
		RevisionNo: 3, AvailableAt: tsParam(time.Now()), Source: "MOA_TEST", SourceDigest: "sha256:a",
	}))

	got, err := q.GetLatestCommodityObservation(ctx, gen.GetLatestCommodityObservationParams{
		CommodityID: id, MarketCode: nil, TradeDate: dateParam(day),
	})
	require.NoError(t, err, "NULL market_code 的比對失敗會讓這裡回 ErrNoRows、revision 永遠從 0 重來")
	require.Equal(t, int32(3), got.RevisionNo)
	require.Equal(t, "sha256:a", got.SourceDigest)
}

func TestCommodityObservation_DifferentMarketsSameDayCoexist(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)

	id := upsertCommodity(ctx, t, q, testCommodityPorkID, "pig", "live_animal", nil)
	day := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	m1, m2 := "PIG-632", "PIG-514"
	for i, m := range []*string{&m1, &m2} {
		require.NoError(t, q.InsertCommodityObservation(ctx, gen.InsertCommodityObservationParams{
			ID: testCommodityObsBase + int64(21+i), CommodityID: id, MarketCode: m,
			TradeDate: dateParam(day), RevisionNo: 1, AvailableAt: tsParam(time.Now()),
			Source: "MOA_TEST", SourceDigest: "sha256:c",
		}))
	}
	require.Error(t, q.InsertCommodityObservation(ctx, gen.InsertCommodityObservationParams{
		ID: testCommodityObsBase + 23, CommodityID: id, MarketCode: &m1,
		TradeDate: dateParam(day), RevisionNo: 1, AvailableAt: tsParam(time.Now()),
		Source: "MOA_TEST", SourceDigest: "sha256:d",
	}), "同市場同 revision 必須被擋下")
}

func TestCategoryCommodityMapping_OneSeriesToManyL3(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)

	id := upsertCommodity(ctx, t, q, testCommodityPorkID, "pig", "live_animal", nil)
	note := "毛豬活體拍賣均價"
	for _, l3 := range []string{"2008", "2257"} {
		require.NoError(t, q.UpsertCategoryCommodityMapping(ctx, gen.UpsertCategoryCommodityMappingParams{
			CategoryID: l3, CommodityID: &id, MappingVersion: "v1", SourceNote: &note,
		}))
	}
	for _, l3 := range []string{"2008", "2257"} {
		got, err := q.ListCommoditiesByCategory(ctx, l3)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, id, got[0].ID)
	}
}

func TestCategoryCommodityMapping_ReIngestDoesNotOverwriteCuration(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)

	curated := upsertCommodity(ctx, t, q, testCommodityEggFG, "curated", "farmgate", nil)
	note := "人工核准"
	require.NoError(t, q.UpsertCategoryCommodityMapping(ctx, gen.UpsertCategoryCommodityMappingParams{
		CategoryID: "2008", CommodityID: &curated, MappingVersion: "manual", SourceNote: &note,
		UpdatedBy: 900000001,
	}))

	pig := upsertCommodity(ctx, t, q, testCommodityPorkID, "pig", "live_animal", nil)
	adapterNote := "adapter"
	require.NoError(t, q.UpsertCategoryCommodityMapping(ctx, gen.UpsertCategoryCommodityMappingParams{
		CategoryID: "2008", CommodityID: &pig, MappingVersion: "v1", SourceNote: &adapterNote,
	}))

	got, err := q.ListCommoditiesByCategory(ctx, "2008")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, curated, got[0].ID, "人工映射被 adapter 重跑覆蓋了")
}

func TestCategoryMapping_AdminUpsertOverridesCommodityMapping(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)

	pig := upsertCommodity(ctx, t, q, testCommodityPorkID, "pig", "live_animal", nil)
	adapterNote := "adapter"
	require.NoError(t, q.UpsertCategoryCommodityMapping(ctx, gen.UpsertCategoryCommodityMappingParams{
		CategoryID: "2008", CommodityID: &pig, MappingVersion: "v1", SourceNote: &adapterNote,
	}))

	t.Run("改指 crop_group", func(t *testing.T) {
		groupCode := "PORK-CURATED"
		row, err := q.UpsertCategoryCropMapping(ctx, gen.UpsertCategoryCropMappingParams{
			CategoryID: "2008", Status: "resolved", GroupCode: &groupCode,
			MappingVersion: "pricebook-category-v1", UpdatedBy: 900000001,
		})
		require.NoError(t, err, "admin 改指 group 不該被 CHECK 擋下")
		require.NotNil(t, row.GroupCode)
		require.Equal(t, groupCode, *row.GroupCode)
		require.Nil(t, row.CommodityID, "改指 group 之後 commodity 目標必須清掉——兩者互斥")
	})

	t.Run("改成 unmapped", func(t *testing.T) {
		require.NoError(t, q.UpsertCategoryCommodityMapping(ctx, gen.UpsertCategoryCommodityMappingParams{
			CategoryID: "2257", CommodityID: &pig, MappingVersion: "v1", SourceNote: &adapterNote,
		}))
		row, err := q.UpsertCategoryCropMapping(ctx, gen.UpsertCategoryCropMappingParams{
			CategoryID: "2257", Status: "unmapped",
			MappingVersion: "pricebook-category-v1", UpdatedBy: 900000001,
		})
		require.NoError(t, err, "admin 標 unmapped 不該被 CHECK 擋下")
		require.Equal(t, "unmapped", row.Status)
		require.Nil(t, row.CommodityID)
		require.Nil(t, row.GroupCode)
	})
}
