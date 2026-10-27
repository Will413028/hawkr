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
	identityMarketID    int64 = 700000000000000101
	identityCropGroupID int64 = 700000000000000102
	identityCropID      int64 = 700000000000000103
)

func TestEmptyIdentityValuesAreRejectedByTheDatabase(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)

	groupID, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: identityCropGroupID, GroupCode: "moa-group:identity", Name: "葉菜類",
	})
	require.NoError(t, err)

	t.Run("crop_group 名稱不得為空", func(t *testing.T) {
		_, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
			ID: identityCropGroupID + 900, GroupCode: "moa-group:blank", Name: "",
		})
		require.Error(t, err)
	})

	t.Run("crop 名稱不得為空", func(t *testing.T) {
		_, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
			ID: identityCropID, CropCode: "FE800", GroupID: groupID, Name: "",
		})
		require.Error(t, err)
	})

	t.Run("crop 名稱不得只有空白", func(t *testing.T) {
		_, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
			ID: identityCropID + 1, CropCode: "FE820", GroupID: groupID, Name: "   ",
		})
		require.Error(t, err)
	})

	t.Run("crop 代碼不得為空", func(t *testing.T) {
		_, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
			ID: identityCropID + 2, CropCode: "", GroupID: groupID, Name: "椰子",
		})
		require.Error(t, err)
	})

	t.Run("market 名稱不得為空", func(t *testing.T) {
		_, err := q.UpsertMarket(ctx, gen.UpsertMarketParams{
			ID: identityMarketID, MarketCode: "105", Name: "",
		})
		require.Error(t, err)
	})

	t.Run("market 代碼不得為空", func(t *testing.T) {
		_, err := q.UpsertMarket(ctx, gen.UpsertMarketParams{
			ID: identityMarketID + 1, MarketCode: "", Name: "台北市場",
		})
		require.Error(t, err)
	})
}

func TestWellFormedIdentityValuesStillUpsert(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)

	groupID, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: identityCropGroupID, GroupCode: "moa-group:ok", Name: "椰子",
	})
	require.NoError(t, err)

	_, err = q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID: identityCropID, CropCode: "11", GroupID: groupID, Name: "椰子",
	})
	require.NoError(t, err)

	_, err = q.UpsertMarket(ctx, gen.UpsertMarketParams{
		ID: identityMarketID, MarketCode: "105", Name: "台北市場",
	})
	require.NoError(t, err)
}

func TestRejectedIdentityRegisterIsKeyedByIdentity(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	const source = "moa_agri_products_trans_type"
	record := func(reason, tradeDate string, occurrences int64, seenAt time.Time) bool {
		t.Helper()
		isNew, err := q.RecordRejectedIdentity(ctx, gen.RecordRejectedIdentityParams{
			Source: source, CropCode: "FE800", MarketCode: "105", Reason: reason,
			FirstTradeDate: tradeDate, LastTradeDate: tradeDate,
			Occurrences: occurrences,
			SeenAt:      pgtype.Timestamptz{Time: seenAt, Valid: true},
		})
		require.NoError(t, err)
		return isNew
	}

	day1 := time.Date(2026, 9, 15, 2, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 16, 2, 0, 0, 0, time.UTC)

	require.True(t, record("missing_crop_name", "115.09.15", 1, day1),
		"第一次見到這個身分應回報為新")
	require.False(t, record("missing_crop_name", "115.09.16", 2, day2),
		"同一個身分第二次被擋不得回報為新")

	var count int64
	var occurrences int64
	var firstDate, lastDate string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) OVER (), occurrence_count, first_trade_date, last_trade_date
		FROM ingest_rejected_identity
		WHERE source = $1 AND crop_code = 'FE800' AND reason = 'missing_crop_name'`,
		source).Scan(&count, &occurrences, &firstDate, &lastDate))

	require.Equal(t, int64(1), count, "同一個身分不得長出第二列")
	require.Equal(t, int64(3), occurrences, "被擋的列數要累加")
	require.Equal(t, "115.09.15", firstDate, "首次見到的交易日不得被覆寫")
	require.Equal(t, "115.09.16", lastDate, "最後見到的交易日要更新")

	require.True(t, record("missing_market_name", "115.09.16", 1, day2),
		"不同原因應獨立登記為新身分")
}
