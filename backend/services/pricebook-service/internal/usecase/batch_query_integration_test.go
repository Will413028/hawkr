//go:build integration

package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

// seedMarketAndCrops 建立測試需要的 market/crop_group/crop 列，回傳
// marketID 與依序對應 cropCodes 的 cropIDs。
func seedMarketAndCrops(ctx context.Context, t *testing.T, q *gen.Queries, tsid func() int64, marketCode string, cropCodes []string) (int64, []int64) {
	t.Helper()
	marketID, err := q.UpsertMarket(ctx, gen.UpsertMarketParams{ID: tsid(), MarketCode: marketCode, Name: marketCode})
	require.NoError(t, err)

	categoryCode := "N04"
	groupID, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: tsid(), GroupCode: "TESTGROUP", Name: "測試大類",
		CategoryCode: &categoryCode,
	})
	require.NoError(t, err)

	cropIDs := make([]int64, len(cropCodes))
	for i, code := range cropCodes {
		id, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
			ID: tsid(), CropCode: code, GroupID: groupID, Name: code,
			CategoryCode: &categoryCode,
		})
		require.NoError(t, err)
		cropIDs[i] = id
	}
	return marketID, cropIDs
}

// TestLatestObservationRevisionsByMarketPairsKeysCorrectly 驗證 key 配對正確性：
// 建立 4 個 (crop_id, trade_date) 組合各自擁有不同的 digest，然後只查詢其中 2 個，
// 確認返回恰好 2 筆且 digest 對應正確。若實作有「陣列各自去重再 cross product 配對」
// 的 bug，因為 DB 裡 4 個組合都有資料，任何配對錯誤都會導致返回的 digest 或列數對不上。
func TestLatestObservationRevisionsByMarketPairsKeysCorrectly(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	tsid := newSeqTSID(940000000000000000)
	marketID, cropIDs := seedMarketAndCrops(ctx, t, q, tsid, "104", []string{"LA1", "LB1"})
	dateA := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	dateB := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)

	// 插入 4 個組合全部，各自不同的 digest：
	// (LA1, dateA) -> digest-a
	// (LA1, dateB) -> digest-c
	// (LB1, dateA) -> digest-d
	// (LB1, dateB) -> digest-b
	require.NoError(t, q.InsertPriceObservation(ctx, gen.InsertPriceObservationParams{
		ID: tsid(), MarketID: marketID, CropID: cropIDs[0], TradeDate: pgxutil.DateParam(&dateA),
		RevisionNo: 1, AvailableAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		AvgPrice: mustNumeric(t, "10.0000"), UpperPrice: mustNumeric(t, "10.0000"),
		MiddlePrice: mustNumeric(t, "10.0000"), LowerPrice: mustNumeric(t, "10.0000"),
		VolumeKg: mustNumeric(t, "1.000"), Source: "moa", SourceDigest: "digest-a",
	}))
	require.NoError(t, q.InsertPriceObservation(ctx, gen.InsertPriceObservationParams{
		ID: tsid(), MarketID: marketID, CropID: cropIDs[0], TradeDate: pgxutil.DateParam(&dateB),
		RevisionNo: 1, AvailableAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		AvgPrice: mustNumeric(t, "10.5000"), UpperPrice: mustNumeric(t, "10.5000"),
		MiddlePrice: mustNumeric(t, "10.5000"), LowerPrice: mustNumeric(t, "10.5000"),
		VolumeKg: mustNumeric(t, "1.000"), Source: "moa", SourceDigest: "digest-c",
	}))
	require.NoError(t, q.InsertPriceObservation(ctx, gen.InsertPriceObservationParams{
		ID: tsid(), MarketID: marketID, CropID: cropIDs[1], TradeDate: pgxutil.DateParam(&dateA),
		RevisionNo: 1, AvailableAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		AvgPrice: mustNumeric(t, "20.5000"), UpperPrice: mustNumeric(t, "20.5000"),
		MiddlePrice: mustNumeric(t, "20.5000"), LowerPrice: mustNumeric(t, "20.5000"),
		VolumeKg: mustNumeric(t, "2.000"), Source: "moa", SourceDigest: "digest-d",
	}))
	require.NoError(t, q.InsertPriceObservation(ctx, gen.InsertPriceObservationParams{
		ID: tsid(), MarketID: marketID, CropID: cropIDs[1], TradeDate: pgxutil.DateParam(&dateB),
		RevisionNo: 1, AvailableAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		AvgPrice: mustNumeric(t, "20.0000"), UpperPrice: mustNumeric(t, "20.0000"),
		MiddlePrice: mustNumeric(t, "20.0000"), LowerPrice: mustNumeric(t, "20.0000"),
		VolumeKg: mustNumeric(t, "2.000"), Source: "moa", SourceDigest: "digest-b",
	}))

	// 查詢時只傳「正確」的 2 個 key：(LA1, dateA) 和 (LB1, dateB)
	rows, err := q.LatestObservationRevisionsByMarket(ctx, gen.LatestObservationRevisionsByMarketParams{
		MarketID:   marketID,
		CropIds:    []int64{cropIDs[0], cropIDs[1]},
		TradeDates: []pgtype.Date{pgxutil.DateParam(&dateA), pgxutil.DateParam(&dateB)},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2, "查詢 2 個 key，應返回恰好 2 筆")

	// 建立 (cropID, digest) 對應表，驗證配對正確
	got := make(map[int64]string) // cropID -> digest
	for _, r := range rows {
		got[r.CropID] = r.SourceDigest
	}
	require.Equal(t, "digest-a", got[cropIDs[0]], "crop LA1 應配對到 (LA1, dateA) 的 digest-a，不是其他組合的資料")
	require.Equal(t, "digest-b", got[cropIDs[1]], "crop LB1 應配對到 (LB1, dateB) 的 digest-b，不是其他組合的資料")
}

// TestBatchInsertPriceObservationWritesAllRows 驗證批次寫入：一次呼叫寫入
// 3 筆橫跨 2 個 crop 的觀測列，逐筆核對欄位值正確落地。
func TestBatchInsertPriceObservationWritesAllRows(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	tsid := newSeqTSID(950000000000000000)
	marketID, cropIDs := seedMarketAndCrops(ctx, t, q, tsid, "104", []string{"LA1", "LB1"})
	date1 := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}

	err := q.BatchInsertPriceObservation(ctx, gen.BatchInsertPriceObservationParams{
		Ids:           []int64{tsid(), tsid(), tsid()},
		MarketIds:     []int64{marketID, marketID, marketID},
		CropIds:       []int64{cropIDs[0], cropIDs[0], cropIDs[1]},
		TradeDates:    []pgtype.Date{pgxutil.DateParam(&date1), pgxutil.DateParam(&date2), pgxutil.DateParam(&date1)},
		RevisionNos:   []int32{1, 1, 1},
		AvailableAts:  []pgtype.Timestamptz{now, now, now},
		AvgPrices:     []pgtype.Numeric{mustNumeric(t, "10.0000"), mustNumeric(t, "11.0000"), mustNumeric(t, "20.0000")},
		UpperPrices:   []pgtype.Numeric{mustNumeric(t, "10.0000"), mustNumeric(t, "11.0000"), mustNumeric(t, "20.0000")},
		MiddlePrices:  []pgtype.Numeric{mustNumeric(t, "10.0000"), mustNumeric(t, "11.0000"), mustNumeric(t, "20.0000")},
		LowerPrices:   []pgtype.Numeric{mustNumeric(t, "10.0000"), mustNumeric(t, "11.0000"), mustNumeric(t, "20.0000")},
		VolumeKgs:     []pgtype.Numeric{mustNumeric(t, "1.000"), mustNumeric(t, "1.000"), mustNumeric(t, "2.000")},
		Sources:       []string{"moa", "moa", "moa"},
		SourceDigests: []string{"d1", "d2", "d3"},
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM price_observation`).Scan(&count))
	require.Equal(t, 3, count)

	// Spot-check 列數據正確性：驗證 (cropIDs[1], date1) 那一列的 digest 和 avg_price 都對齊
	var digest string
	var avgPrice string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT source_digest, avg_price::text FROM price_observation WHERE crop_id = $1 AND trade_date = $2`,
		cropIDs[1], pgxutil.DateParam(&date1)).Scan(&digest, &avgPrice))
	require.Equal(t, "d3", digest, "source_digest 應對應第 3 筆的 d3")
	require.Equal(t, "20.0000", avgPrice, "avg_price 應對應第 3 筆的 20.0000")
}

func mustNumeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	require.NoError(t, n.Scan(s))
	return n
}
