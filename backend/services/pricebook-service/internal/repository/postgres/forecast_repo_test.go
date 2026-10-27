//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	pbpostgres "github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

// TSID 基底，避開本 package 既有測試用掉的 700…/900…/910…/920…/950…/960…/980…段。
const (
	frObsIDBase     int64 = 981000000000001000
	frRestDayIDBase int64 = 981000000000002000
	frMarketIDBase  int64 = 981000000000003000
)

// TestSeriesObservationsPicksMaxRevisionVisibleAtCutoff 是本檔最重要的一條：
// 種三個 revision（不同 available_at），cutoff 卡在中間那個 revision 的
// available_at 時，SeriesObservations 必須回中間那筆的值，不是最新那筆——
// 否則就是本檔頂端 comment 警告的 look-ahead 洩漏。
func TestSeriesObservationsPicksMaxRevisionVisibleAtCutoff(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := seedSeries(ctx, t, q)

	t1 := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(1 * time.Hour)
	t3 := t1.Add(2 * time.Hour)

	rev1 := observationParams(t, frObsIDBase+1, marketID, cropID, 1, "digest-rev1")
	rev1.AvailableAt = pgtype.Timestamptz{Time: t1, Valid: true}
	price1, err := pbpostgres.NumericFromString("40.0000")
	require.NoError(t, err)
	rev1.AvgPrice = price1
	require.NoError(t, q.InsertPriceObservation(ctx, rev1))

	rev2 := observationParams(t, frObsIDBase+2, marketID, cropID, 2, "digest-rev2")
	rev2.AvailableAt = pgtype.Timestamptz{Time: t2, Valid: true}
	price2, err := pbpostgres.NumericFromString("45.0000")
	require.NoError(t, err)
	rev2.AvgPrice = price2
	require.NoError(t, q.InsertPriceObservation(ctx, rev2))

	rev3 := observationParams(t, frObsIDBase+3, marketID, cropID, 3, "digest-rev3")
	rev3.AvailableAt = pgtype.Timestamptz{Time: t3, Valid: true}
	price3, err := pbpostgres.NumericFromString("50.0000")
	require.NoError(t, err)
	rev3.AvgPrice = price3
	require.NoError(t, q.InsertPriceObservation(ctx, rev3))

	repo := pbpostgres.NewForecastRepository(pool)
	// cutoff = t2：rev3（available_at=t3）此刻對「我方視角」還不存在，
	// 必須取 rev2 的值，不是 rev3 的。
	got, err := repo.SeriesObservations(ctx, "104", "LA1", testTradeDate, testTradeDate, t2)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int32(2), got[0].RevisionNo, "cutoff=t2 必須選到 rev2，不是最新的 rev3")
	require.NotNil(t, got[0].AvgPrice)
	require.Equal(t, 45.0, *got[0].AvgPrice)
}

// TestSeriesObservationsExcludesRowsOutsideDateRange 確認 [from_date, to_date]
// 範圍外的觀測不會溢進來。
func TestSeriesObservationsExcludesRowsOutsideDateRange(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := seedSeries(ctx, t, q)

	inRange := observationParams(t, frObsIDBase+1, marketID, cropID, 1, "digest-in-range")
	price, err := pbpostgres.NumericFromString("40.0000")
	require.NoError(t, err)
	inRange.AvgPrice = price
	require.NoError(t, q.InsertPriceObservation(ctx, inRange))

	outOfRange := observationParams(t, frObsIDBase+2, marketID, cropID, 1, "digest-out-of-range")
	outOfRange.TradeDate = pgtype.Date{Time: testTradeDate.AddDate(0, 0, 10), Valid: true}
	outOfRange.AvgPrice = price
	require.NoError(t, q.InsertPriceObservation(ctx, outOfRange))

	repo := pbpostgres.NewForecastRepository(pool)
	cutoff := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	got, err := repo.SeriesObservations(ctx, "104", "LA1", testTradeDate, testTradeDate, cutoff)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.True(t, got[0].TradeDate.Equal(testTradeDate), "只有範圍內的 trade_date 該被回傳")
}

// TestRestDaysExcludesNullMarketType 種一列 market_type=NULL 與一列
// market_type='V'，RestDays 只能回 'V' 那筆——NULL 是 Task 3 之前的舊資料，
// 不得被當成任何營業類別的休市依據（見 schema.sql 對 market_rest_day 的註解）。
func TestRestDaysExcludesNullMarketType(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, _ := seedSeries(ctx, t, q)

	fetchedAt := pgtype.Timestamptz{Time: time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC), Valid: true}
	marketType := "V"

	require.NoError(t, q.UpsertMarketRestDay(ctx, gen.UpsertMarketRestDayParams{
		ID:         frRestDayIDBase + 1,
		MarketID:   marketID,
		MarketType: nil, // 舊資料，沒有類別維度
		RestDate:   pgtype.Date{Time: testTradeDate, Valid: true},
		FetchedAt:  fetchedAt,
	}))
	require.NoError(t, q.UpsertMarketRestDay(ctx, gen.UpsertMarketRestDayParams{
		ID:         frRestDayIDBase + 2,
		MarketID:   marketID,
		MarketType: &marketType,
		RestDate:   pgtype.Date{Time: testTradeDate, Valid: true},
		FetchedAt:  fetchedAt,
	}))

	repo := pbpostgres.NewForecastRepository(pool)
	got, err := repo.RestDays(ctx, "104", testTradeDate, testTradeDate)
	require.NoError(t, err)
	require.Len(t, got, 1, "market_type=NULL 的舊列不得被回傳")
	require.Equal(t, "V", got[0].MarketType)
}

// TestRecentVolumeCountsOnlyLatestRevisionPerDay 同一天種 rev1 volume=100、
// rev2 volume=200，RecentVolume 的總和必須是 200，不是 300——否則同一天的
// 多次修訂會被重複加總，大類加權會被灌水。
func TestRecentVolumeCountsOnlyLatestRevisionPerDay(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := seedSeries(ctx, t, q)

	t1 := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(1 * time.Hour)

	vol1, err := pbpostgres.NumericFromString("100.000")
	require.NoError(t, err)
	rev1 := observationParams(t, frObsIDBase+1, marketID, cropID, 1, "digest-rev1")
	rev1.AvailableAt = pgtype.Timestamptz{Time: t1, Valid: true}
	rev1.VolumeKg = vol1
	require.NoError(t, q.InsertPriceObservation(ctx, rev1))

	vol2, err := pbpostgres.NumericFromString("200.000")
	require.NoError(t, err)
	rev2 := observationParams(t, frObsIDBase+2, marketID, cropID, 2, "digest-rev2")
	rev2.AvailableAt = pgtype.Timestamptz{Time: t2, Valid: true}
	rev2.VolumeKg = vol2
	require.NoError(t, q.InsertPriceObservation(ctx, rev2))

	repo := pbpostgres.NewForecastRepository(pool)
	got, err := repo.RecentVolume(ctx, "LA", testTradeDate, testTradeDate, t2)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "104", got[0].MarketCode)
	require.Equal(t, "LA1", got[0].CropCode)
	require.Equal(t, 200.0, got[0].Volume, "只能計最新 revision，200 不是 100+200")
}

// TestRecentVolumeForCropCountsOnlyLatestRevisionPerDay 是
// TestRecentVolumeCountsOnlyLatestRevisionPerDay 的單一品項版——同一天種
// rev1 volume=100、rev2 volume=200，RecentVolumeForCrop 的總和必須是 200，
// 不是 300，跟 RecentVolume 同一條規則。
func TestRecentVolumeForCropCountsOnlyLatestRevisionPerDay(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := seedSeries(ctx, t, q)

	t1 := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(1 * time.Hour)

	vol1, err := pbpostgres.NumericFromString("100.000")
	require.NoError(t, err)
	rev1 := observationParams(t, frObsIDBase+1, marketID, cropID, 1, "digest-rev1")
	rev1.AvailableAt = pgtype.Timestamptz{Time: t1, Valid: true}
	rev1.VolumeKg = vol1
	require.NoError(t, q.InsertPriceObservation(ctx, rev1))

	vol2, err := pbpostgres.NumericFromString("200.000")
	require.NoError(t, err)
	rev2 := observationParams(t, frObsIDBase+2, marketID, cropID, 2, "digest-rev2")
	rev2.AvailableAt = pgtype.Timestamptz{Time: t2, Valid: true}
	rev2.VolumeKg = vol2
	require.NoError(t, q.InsertPriceObservation(ctx, rev2))

	repo := pbpostgres.NewForecastRepository(pool)
	got, err := repo.RecentVolumeForCrop(ctx, "LA1", testTradeDate, testTradeDate, t2)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "104", got[0].MarketCode)
	require.Equal(t, 200.0, got[0].Volume, "只能計最新 revision，200 不是 100+200")
}

// TestRecentVolumeForCropExcludesOtherCrops 確認查詢真的用 crop_code 篩選，
// 不是不小心撈了整個市場的成交量——這是 RecentVolumeByCrop 跟
// RecentVolumeByCropMarket 唯一的結構差異（少了 crop_group JOIN），必須驗到。
func TestRecentVolumeForCropExcludesOtherCrops(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := seedSeries(ctx, t, q)
	categoryCode := "V"
	otherCropID, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID:           frMarketIDBase + 900,
		CropCode:     "LA9",
		GroupID:      testCropGroupID, // same crop_group seedSeries already created
		Name:         "白菜",
		CategoryCode: &categoryCode,
	})
	require.NoError(t, err)

	vol, err := pbpostgres.NumericFromString("50.000")
	require.NoError(t, err)
	target := observationParams(t, frObsIDBase+1, marketID, cropID, 1, "digest-target")
	target.VolumeKg = vol
	require.NoError(t, q.InsertPriceObservation(ctx, target))

	otherVol, err := pbpostgres.NumericFromString("9999.000")
	require.NoError(t, err)
	other := observationParams(t, frObsIDBase+2, marketID, otherCropID, 1, "digest-other")
	other.VolumeKg = otherVol
	require.NoError(t, q.InsertPriceObservation(ctx, other))

	repo := pbpostgres.NewForecastRepository(pool)
	cutoff := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	got, err := repo.RecentVolumeForCrop(ctx, "LA1", testTradeDate, testTradeDate, cutoff)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 50.0, got[0].Volume, "LA9 的 9999 成交量不得漏進 LA1 的結果")
}

// TestSeriesObservationsNullPriceAndVolumeAreNilNotZero 透過真正的
// repository 出口（不是 schema_invariants_test.go 那條只測 NumericToPtr 本身
// 的版本）釘住：avg_price/volume_kg 為 SQL NULL 時，domain.Observation 的
// AvgPrice/VolumeKg 必須是 nil，不能被 numericToFloatPtr 或任何轉換路徑
// 悄悄變成 0——0 是合法價格/成交量，把兩者混為一談會讓下游看到一場不存在的
// 崩盤或零成交。另種一筆 avg_price=0 的觀測，證明真正的 0 不會被誤判成 nil。
func TestSeriesObservationsNullPriceAndVolumeAreNilNotZero(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := seedSeries(ctx, t, q)

	// rev1：avg_price / volume_kg 全部留空（observationParams 預設就是
	// pgtype.Numeric 零值 = SQL NULL，不需要額外賦值）。
	nullRow := observationParams(t, frObsIDBase+1, marketID, cropID, 1, "digest-null")
	require.False(t, nullRow.AvgPrice.Valid, "測資本身必須是 NULL 才有意義")
	require.False(t, nullRow.VolumeKg.Valid, "測資本身必須是 NULL 才有意義")
	require.NoError(t, q.InsertPriceObservation(ctx, nullRow))

	// rev 對照組：同一 crop、隔一天的 trade_date，avg_price 是真正的 0 元。
	zeroPrice, err := pbpostgres.NumericFromString("0.0000")
	require.NoError(t, err)
	zeroRow := observationParams(t, frObsIDBase+2, marketID, cropID, 1, "digest-zero")
	zeroRow.TradeDate = pgtype.Date{Time: testTradeDate.AddDate(0, 0, 1), Valid: true}
	zeroRow.AvgPrice = zeroPrice
	require.NoError(t, q.InsertPriceObservation(ctx, zeroRow))

	repo := pbpostgres.NewForecastRepository(pool)
	cutoff := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	got, err := repo.SeriesObservations(ctx, "104", "LA1", testTradeDate, testTradeDate.AddDate(0, 0, 1), cutoff)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byDate := make(map[string]int)
	for i, obs := range got {
		byDate[obs.TradeDate.Format("2006-01-02")] = i
	}

	nullObs := got[byDate[testTradeDate.Format("2006-01-02")]]
	require.Nil(t, nullObs.AvgPrice, "NULL avg_price 必須讀回 nil，不是 0")
	require.Nil(t, nullObs.VolumeKg, "NULL volume_kg 必須讀回 nil，不是 0")

	zeroObs := got[byDate[testTradeDate.AddDate(0, 0, 1).Format("2006-01-02")]]
	require.NotNil(t, zeroObs.AvgPrice, "真正的 0 元不得被讀回 nil")
	require.Equal(t, 0.0, *zeroObs.AvgPrice)
}

// TestGroupObservationsIncludesAllMarketsWhenMarketCodeEmpty 守 review
// Finding 1（2026-07-30，Critical）：GroupObservationsAsOf 的 DISTINCT ON
// 必須含市場鍵。同一個 (crop, trade_date) 種在市場 104（rev1）與市場 109
// （rev3——刻意用不同 revision_no，證明勝負不是靠 tie-break 撿到的），
// marketCode="" 查詢時兩個市場都必須回來，且各自的價格要對得上各自的列。
//
// 這條測試曾經在少了市場鍵的版本下紅過（見本檔 mutation 驗證紀錄，寫在
// task-10-report.md）：104/rev3(=50.0000) 併掉 109，109 整筆消失。
func TestGroupObservationsIncludesAllMarketsWhenMarketCodeEmpty(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	market104ID, cropID := seedSeries(ctx, t, q) // market_code "104"，group "LA"，crop "LA1"

	market109ID, err := q.UpsertMarket(ctx, gen.UpsertMarketParams{
		ID:         frMarketIDBase + 1,
		MarketCode: "109",
		Name:       "台北二",
	})
	require.NoError(t, err)

	price104, err := pbpostgres.NumericFromString("50.0000")
	require.NoError(t, err)
	row104 := observationParams(t, frObsIDBase+1, market104ID, cropID, 1, "digest-104-rev1")
	row104.AvgPrice = price104
	require.NoError(t, q.InsertPriceObservation(ctx, row104))

	price109, err := pbpostgres.NumericFromString("77.0000")
	require.NoError(t, err)
	row109 := observationParams(t, frObsIDBase+2, market109ID, cropID, 3, "digest-109-rev3")
	row109.AvgPrice = price109
	require.NoError(t, q.InsertPriceObservation(ctx, row109))

	repo := pbpostgres.NewForecastRepository(pool)
	cutoff := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	got, err := repo.GroupObservations(ctx, "LA", "", testTradeDate, testTradeDate, cutoff)
	require.NoError(t, err)
	require.Len(t, got, 2, "market_code=\"\" 必須回兩個市場各自的觀測，不能被 DISTINCT ON 併掉")

	byMarket := make(map[string]domain.Observation)
	for _, obs := range got {
		byMarket[obs.MarketCode] = obs
	}
	require.Contains(t, byMarket, "104")
	require.Contains(t, byMarket, "109")

	obs104 := byMarket["104"]
	require.NotNil(t, obs104.AvgPrice)
	require.Equal(t, 50.0, *obs104.AvgPrice, "104 的價格不得被 109 蓋掉")

	obs109 := byMarket["109"]
	require.NotNil(t, obs109.AvgPrice)
	require.Equal(t, 77.0, *obs109.AvgPrice, "109 不得因為市場鍵被併掉而消失")
}

// newTestForecastRepo 包一層 setupTestDB + NewForecastRepository，用
// t.Cleanup 取代逐條測試手寫的 `pool, cleanup := setupTestDB(...); defer
// cleanup()`——本檔新增的測試（Task 2：MarketByCode）不需要
// 直接碰 pool/gen.Queries，用這個精簡版；本檔既有測試需要 pool 本身去插
// 種子資料，仍沿用 setupTestDB 手動 defer，兩者保證等價（同一個
// dbtest.Clone 機制）。
func newTestForecastRepo(t *testing.T) domain.ForecastRepository {
	t.Helper()
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	t.Cleanup(cleanup)
	return pbpostgres.NewForecastRepository(pool)
}

// TestForecastRepo_MarketByCode_UnknownReturnsZeroValueNoError 釘住
// domain.ForecastRepository.MarketByCode 介面註解承諾的行為：查無市場代碼
// 是業務結果（回零值），不是錯誤——呼叫端不需要對這支方法做 error handling
// 就能區分「查無」與「查詢本身失敗」。
func TestForecastRepo_MarketByCode_UnknownReturnsZeroValueNoError(t *testing.T) {
	repo := newTestForecastRepo(t) // reuse this file's existing test-DB helper
	ref, err := repo.MarketByCode(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatalf("MarketByCode: unexpected error %v", err)
	}
	if ref != (domain.MarketRef{}) {
		t.Errorf("MarketByCode(unknown) = %+v, want zero value", ref)
	}
}

// TestForecastRepo_MarketByCode_FoundReturnsName 對照組：真的存在的
// market_code 必須把 name 讀回來，不能連查到列都回零值。
func TestForecastRepo_MarketByCode_FoundReturnsName(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	seedSeries(ctx, t, q) // market_code "104"，name "台北一"

	repo := pbpostgres.NewForecastRepository(pool)
	ref, err := repo.MarketByCode(ctx, "104")
	if err != nil {
		t.Fatalf("MarketByCode: unexpected error %v", err)
	}
	want := domain.MarketRef{MarketCode: "104", Name: "台北一"}
	if ref != want {
		t.Errorf("MarketByCode(104) = %+v, want %+v", ref, want)
	}
}

// TestForecastRepo_CropByCode_UnknownReturnsZeroValueNoError 對照
// MarketByCode 的既有慣例：查無 crop_code 是業務結果（回零值），不是錯誤。
// Task 11（trend-summary integration tests）新增 CropByCode 時發現的缺口：
// TrendSummary 的 crop path 原本只能從 ObservedPricesByCrop 的列讀
// categoryCode，市場整段休市（zero rows）時永遠讀不到，這支方法補上一條不
// 依賴觀測列的路徑——見 domain.ForecastRepository.CropByCode 的介面註解。
func TestForecastRepo_CropByCode_UnknownReturnsZeroValueNoError(t *testing.T) {
	repo := newTestForecastRepo(t)
	ref, err := repo.CropByCode(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatalf("CropByCode: unexpected error %v", err)
	}
	if ref.CropCode != "" || ref.Name != "" || ref.CategoryCode != nil {
		t.Errorf("CropByCode(unknown) = %+v, want zero value", ref)
	}
}

// TestForecastRepo_CropByCode_FoundReturnsCategoryCode 對照組：真實存在的
// crop_code 必須把 category_code 讀回來，且不必先有任何 price_observation
// 列——這正是這支方法存在的理由（見上方 Unknown 測試的註解）。
func TestForecastRepo_CropByCode_FoundReturnsCategoryCode(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	mustUpsertCropWithCategory(ctx, t, q, "A1", "花胡瓜", "N04") // 沒有任何 price_observation 列

	repo := pbpostgres.NewForecastRepository(pool)
	ref, err := repo.CropByCode(ctx, "A1")
	if err != nil {
		t.Fatalf("CropByCode: unexpected error %v", err)
	}
	if ref.CropCode != "A1" || ref.Name != "花胡瓜" {
		t.Errorf("CropByCode(A1) = %+v, want CropCode=A1 Name=花胡瓜", ref)
	}
	if ref.CategoryCode == nil || *ref.CategoryCode != "N04" {
		t.Errorf("CropByCode(A1).CategoryCode = %v, want \"N04\" (must resolve without any price_observation row present)", ref.CategoryCode)
	}
}

// TestForecastRepo_MarketsByCode_BatchesAndSkipsUnknownCodes 釘住批次版
// （最終 review Finding I3）：一次查回多個代碼、查無的代碼直接不出現。
//
// 🔴 這條非跑不可的理由是那個陣列參數：本服務的 pool 固定
// pgx.QueryExecModeExec（不向 server 要參數型別，靠 Go 值推斷），所以
// `= ANY($1::text[])` 能不能吃 []string 只有對真的 Postgres 跑過才算數——
// 單元測試那層是 fake repo，永遠不會碰到編碼問題。
func TestForecastRepo_MarketsByCode_BatchesAndSkipsUnknownCodes(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	seedSeries(ctx, t, q) // market_code "104"，name "台北一"
	_, err := q.UpsertMarket(ctx, gen.UpsertMarketParams{
		ID:         frMarketIDBase + 10,
		MarketCode: "109",
		Name:       "台北二",
	})
	require.NoError(t, err)

	repo := pbpostgres.NewForecastRepository(pool)
	refs, err := repo.MarketsByCode(ctx, []string{"104", "109", "does-not-exist"})
	require.NoError(t, err)

	byCode := map[string]string{}
	for _, ref := range refs {
		byCode[ref.MarketCode] = ref.Name
	}
	require.Equal(t, map[string]string{"104": "台北一", "109": "台北二"}, byCode,
		"unknown codes must be absent, not present with an empty name")

	// 空輸入短路，不打 DB、不回錯。
	empty, err := repo.MarketsByCode(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}
