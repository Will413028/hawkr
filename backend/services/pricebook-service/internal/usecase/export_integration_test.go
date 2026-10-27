//go:build integration

// export_integration_test.go 驗證 Exporter.ObservationCSV 的 DB 讀取半部：
// 跟其他三個 usecase 的 integration test 同一檔案群組（package usecase_test），
// 重用 setupTestDB / newSeqTSID，不新開第三套 harness。
//
// Exporter 跟 PriceIngest/Backfill/RestDayIngest 同理，沒有可替換的 repository
// 介面可供注入 mock（直接持有 *pgxpool.Pool，每次呼叫才 gen.New(...)），要驗證
// 「同一 series 的多個 revision 只取最大那筆」「NULL avg_price 經過真實
// Postgres NUMERIC 欄位讀回仍是空字串而非 0.0000」「輸出即使來源以非排序順序
// 插入仍保證確定性排序」「numeric(12,4)/numeric(14,3) 的固定位數字串經過
// float64 轉換不失真」這幾個不變量，唯一誠實的辦法是打真的 Postgres——這正是
// Task 9 存在的理由本身：訓練資料對帳的可信度建立在這幾條不變量上，一個
// NOT NULL DEFAULT 0 或排序漏洞會讓對帳把休市誤讀成崩盤、或讓兩邊「看似一致」
// 而掩蓋真正的 train/serve skew（見 export.go 與 task-9-brief.md）。
package usecase_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// exportSeedSeries 建立一組 market / crop_group / crop，回傳 (marketID, cropID)。
// 與 internal/repository/postgres/schema_invariants_test.go 的 seedSeries 同構
// 但獨立宣告——package usecase_test 與 package postgres_test 是兩個不同的測試
// 套件，各自檔案作用域互不可見，不能共用彼此的 helper。
func exportSeedSeries(ctx context.Context, t *testing.T, q *gen.Queries, id int64, marketCode, cropCode string) (int64, int64) {
	t.Helper()

	marketID, err := q.UpsertMarket(ctx, gen.UpsertMarketParams{
		ID:         id,
		MarketCode: marketCode,
		Name:       "市場-" + marketCode,
	})
	require.NoError(t, err)

	groupID, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID:        id + 1,
		GroupCode: "GRP-" + cropCode,
		Name:      "分類-" + cropCode,
	})
	require.NoError(t, err)

	cropID, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID:       id + 2,
		CropCode: cropCode,
		GroupID:  groupID,
		Name:     "作物-" + cropCode,
	})
	require.NoError(t, err)

	return marketID, cropID
}

// exportInsertObservation 寫入一筆觀測。avgPrice/volumeKg 傳空字串代表 NULL
// （缺值）——刻意不呼叫 postgres.NumericFromString("0")，讓呼叫端在「沒有值」
// 與「值是 0」之間必須做出選擇，跟 numeric.go 的入口收斂點同一套規則。
func exportInsertObservation(
	ctx context.Context, t *testing.T, q *gen.Queries,
	id, marketID, cropID int64, tradeDate time.Time, revision int32, avgPrice, volumeKg, digest string,
) {
	t.Helper()

	var avg, vol pgtype.Numeric
	if avgPrice != "" {
		var err error
		avg, err = postgres.NumericFromString(avgPrice)
		require.NoError(t, err)
	}
	if volumeKg != "" {
		var err error
		vol, err = postgres.NumericFromString(volumeKg)
		require.NoError(t, err)
	}

	require.NoError(t, q.InsertPriceObservation(ctx, gen.InsertPriceObservationParams{
		ID:           id,
		MarketID:     marketID,
		CropID:       cropID,
		TradeDate:    pgtype.Date{Time: tradeDate, Valid: true},
		RevisionNo:   revision,
		AvailableAt:  pgtype.Timestamptz{Time: time.Date(2026, 7, 27, 2, 0, 0, 0, time.UTC), Valid: true},
		AvgPrice:     avg,
		VolumeKg:     vol,
		Source:       "moa",
		SourceDigest: digest,
	}))
}

// csvDataRows 拆解 WriteObservationCSV 的輸出，去掉表頭，回傳資料列。
func csvDataRows(t *testing.T, csvText string) []string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(csvText), "\n")
	require.NotEmpty(t, lines)
	require.Equal(t, "market_code,crop_code,trade_date,avg_price,volume_kg", lines[0])
	return lines[1:]
}

// TestExporterObservationCSVReturnsLatestRevisionOnly 驗證 (a)：同一
// (market, crop, trade_date) series 累積兩筆 revision 時，匯出只吐出
// revision_no 最大那一筆的值，不是每個修訂都吐、也不是插入順序決定輸出。
func TestExporterObservationCSVReturnsLatestRevisionOnly(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := exportSeedSeries(ctx, t, q, 810000000000000000, "104", "LA1")
	tradeDate := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)

	exportInsertObservation(ctx, t, q, 810000000000001001, marketID, cropID, tradeDate, 1, "40.0000", "100.000", "digest-rev1")
	exportInsertObservation(ctx, t, q, 810000000000001002, marketID, cropID, tradeDate, 2, "46.2000", "10957.000", "digest-rev2")

	exporter := usecase.NewExporter(pool)
	var buf bytes.Buffer
	rowCount, err := exporter.ObservationCSV(ctx, tradeDate, tradeDate, &buf)
	require.NoError(t, err)
	require.Equal(t, 1, rowCount, "回傳列數應反映實際寫出的資料列數")

	rows := csvDataRows(t, buf.String())
	require.Len(t, rows, 1, "同一 series 只應吐出一列（最新 revision），不是兩筆 revision 各一列")
	require.Equal(t, "104,LA1,2026-07-26,46.2000,10957.000", rows[0],
		"應是 revision 2 的值，不是 revision 1（revision_no 最大者才是目前最新已知值）")
}

// TestExporterObservationCSVNullPriceRendersEmptyFieldFromRealDB 驗證 (b)：
// 一筆真正 NULL 的 avg_price 經過真實 Postgres NUMERIC 欄位寫入、讀回、
// HasPrice 判定（pgtype.Numeric.Valid，非零值檢查），最終在 CSV 是空字串，
// 不是 "0.0000"。volume_kg 同一列刻意給非 NULL 值，證明兩欄位互不影響。
func TestExporterObservationCSVNullPriceRendersEmptyFieldFromRealDB(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := exportSeedSeries(ctx, t, q, 820000000000000000, "104", "LA1")
	tradeDate := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)

	// avgPrice 傳空字串 = 寫入 pgtype.Numeric 零值 = SQL NULL。
	exportInsertObservation(ctx, t, q, 820000000000001001, marketID, cropID, tradeDate, 1, "", "12345.000", "digest-null-price")

	exporter := usecase.NewExporter(pool)
	var buf bytes.Buffer
	rowCount, err := exporter.ObservationCSV(ctx, tradeDate, tradeDate, &buf)
	require.NoError(t, err)
	require.Equal(t, 1, rowCount)

	rows := csvDataRows(t, buf.String())
	require.Len(t, rows, 1)
	require.Equal(t, "104,LA1,2026-07-27,,12345.000", rows[0],
		"avg_price 缺值必須是空字串，volume_kg 有值必須正常渲染")
}

// TestExporterObservationCSVIsSortedDeterministicallyFromDB 驗證 (c)：即使
// 來源以非排序順序插入（跟 export_test.go 的 TestObservationCSVIsDeterministic
// 相同的三組 (market, crop) 排列組合），從真實 DB 讀出來再交給
// WriteObservationCSV 之後，輸出仍是固定排序——排序保證來自 Go 端就地重排，
// 不是意外碰巧符合 SQL 的 ORDER BY 或插入順序。
func TestExporterObservationCSVIsSortedDeterministicallyFromDB(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	tradeDate := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)

	market109, crop109LA1 := exportSeedSeries(ctx, t, q, 830000000000000000, "109", "LA1")
	market104, crop104LA2 := exportSeedSeries(ctx, t, q, 830000000000000100, "104", "LA2")
	_, crop104LA1 := exportSeedSeries(ctx, t, q, 830000000000000200, "104", "LA1")

	// 刻意以「非排序」順序插入：109/LA1 先、104/LA2 次、104/LA1 最後——
	// 跟 export_test.go 的 fixture 排列同構，確保純函式測試與 DB 讀出的
	// 行為一致，不是各自巧合對出同一個排序。
	exportInsertObservation(ctx, t, q, 830000000000001001, market109, crop109LA1, tradeDate, 1, "46.2000", "10957.000", "d1")
	exportInsertObservation(ctx, t, q, 830000000000001002, market104, crop104LA2, tradeDate, 1, "23.7000", "44650.000", "d2")
	exportInsertObservation(ctx, t, q, 830000000000001003, market104, crop104LA1, tradeDate, 1, "46.2000", "10957.000", "d3")

	exporter := usecase.NewExporter(pool)
	var buf bytes.Buffer
	rowCount, err := exporter.ObservationCSV(ctx, tradeDate, tradeDate, &buf)
	require.NoError(t, err)
	require.Equal(t, 3, rowCount)

	rows := csvDataRows(t, buf.String())
	want := []string{
		"104,LA1,2026-07-26,46.2000,10957.000",
		"104,LA2,2026-07-26,23.7000,44650.000",
		"109,LA1,2026-07-26,46.2000,10957.000",
	}
	require.Equal(t, want, rows, "輸出必須依 market_code, crop_code, trade_date 遞增排序，不受插入順序影響")
}
