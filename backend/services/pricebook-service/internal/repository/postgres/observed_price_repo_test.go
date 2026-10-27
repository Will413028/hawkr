//go:build integration

package postgres_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	pbpostgres "github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

// opNextID 是本檔 fixture 用的 ID 產生器：每個 test 都起一個全新的隔離
// testcontainer（見 dbtest_test.go 的 setupTestDB），同一支測試內只需要
// table-scoped 唯一即可，不需要跨測試唯一——用簡單遞增取代手工排列
// 的 base+offset，因為本檔的 helper 簽名不接受呼叫端指定 ID
// （mustUpsertMarket/mustUpsertCrop 等只收業務欄位）。測試不平行執行
// （本 package 沒有任何 t.Parallel()），遞增不需要 atomic。
var opNextIDSeq int64 = 982000000000000000

func opNextID() int64 {
	opNextIDSeq++
	return opNextIDSeq
}

// observationPrices 是 mustInsertObservation 的價格輸入，四個價格欄位 +
// 成交量都用 float64 常數表達（測試資料，不需要 NumericFromString 的
// 精度保證，但仍走同一條轉換函式，見下方實作）。
type observationPrices struct {
	avg, upper, middle, lower, volume float64
}

func mustUpsertMarket(ctx context.Context, t *testing.T, q *gen.Queries, code, name string) int64 {
	t.Helper()
	id, err := q.UpsertMarket(ctx, gen.UpsertMarketParams{
		ID:         opNextID(),
		MarketCode: code,
		Name:       name,
	})
	require.NoError(t, err)
	return id
}

// mustUpsertCrop 建立一個 crop，連帶建立一個專屬的 crop_group（group_code
// 由 crop_code 推導，本檔測試不關心大類本身，只需要 crop 有一個合法的
// group_id 可以掛）。要測 crop_group 本身的行為（例如同名歧義）請用
// mustUpsertCropGroupWithCategory。
func mustUpsertCrop(ctx context.Context, t *testing.T, q *gen.Queries, code, name string) int64 {
	t.Helper()
	groupID, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID:        opNextID(),
		GroupCode: "op-grp-" + code,
		Name:      name,
	})
	require.NoError(t, err)
	cropID, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID:       opNextID(),
		CropCode: code,
		GroupID:  groupID,
		Name:     name,
	})
	require.NoError(t, err)
	return cropID
}

// mustUpsertCropWithCategory 同 mustUpsertCrop，但讓呼叫端指定
// crop.category_code——CategoryCode（issue #2249 Slice B trend-summary
// 前置，2026-08-07 加）測試需要驗證 ObservedPriceByCrop 真的把
// crop.category_code 讀回來，mustUpsertCrop 本身不帶這欄（零值 NULL），
// 這裡另開一支，不改動既有 helper 的簽名（避免動到其他測試的呼叫端）。
func mustUpsertCropWithCategory(ctx context.Context, t *testing.T, q *gen.Queries, code, name, categoryCode string) int64 {
	t.Helper()
	groupID, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID:        opNextID(),
		GroupCode: "op-grp-" + code,
		Name:      name,
	})
	require.NoError(t, err)
	cropID, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID:           opNextID(),
		CropCode:     code,
		GroupID:      groupID,
		Name:         name,
		CategoryCode: &categoryCode,
	})
	require.NoError(t, err)
	return cropID
}

// mustUpsertCropGroupWithCategory 建立一個 crop_group，供 CropGroupsByName
// 的歧義測試使用（同一個 name 底下可能有多個 group_code，見
// FindCropGroupByName 在 forecast.sql 的說明）。
func mustUpsertCropGroupWithCategory(ctx context.Context, t *testing.T, q *gen.Queries, groupCode, name, categoryCode string) int64 {
	t.Helper()
	id, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID:           opNextID(),
		GroupCode:    groupCode,
		Name:         name,
		CategoryCode: &categoryCode,
	})
	require.NoError(t, err)
	return id
}

// mustUpsertCropInGroupWithCategory 同 mustUpsertCropInGroup，但讓呼叫端指定
// crop.category_code——CategoryCode 測試需要驗證 ObservedPriceByGroup（join
// 大類的查詢版本）也把 crop 自己的 category_code 讀回來（不是
// crop_group.category_code），理由同 mustUpsertCropWithCategory。
func mustUpsertCropInGroupWithCategory(ctx context.Context, t *testing.T, q *gen.Queries, groupID int64, code, name, categoryCode string) int64 {
	t.Helper()
	cropID, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID:           opNextID(),
		CropCode:     code,
		GroupID:      groupID,
		Name:         name,
		CategoryCode: &categoryCode,
	})
	require.NoError(t, err)
	return cropID
}

// mustUpsertCropInGroup 建立一個 crop，掛在呼叫端已建好的 groupID 底下——
// 與 mustUpsertCrop 的差別是後者總是各建一個專屬 crop_group，這支則讓多個
// crop 共用同一個 crop_group，測 ObservedPricesByGroup 的 JOIN 需要這個形狀。
func mustUpsertCropInGroup(ctx context.Context, t *testing.T, q *gen.Queries, groupID int64, code, name string) int64 {
	t.Helper()
	cropID, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID:       opNextID(),
		CropCode: code,
		GroupID:  groupID,
		Name:     name,
	})
	require.NoError(t, err)
	return cropID
}

// mustParseDate 把 "YYYY-MM-DD" 字串轉成 time.Time（UTC，零時分秒）。
func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	require.NoError(t, err)
	return d
}

// mustFormatPrice 把測試用的 float64 常數轉成 NumericFromString 能吃的
// 十進位字串——測試資料一律是簡單的十進位常數，不會踩到科學記號那條
// 擋路的規則（見 numeric.go 對 NumericFromString 的說明）。
func mustFormatPrice(t *testing.T, v float64) string {
	t.Helper()
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// mustInsertObservation 種一筆 price_observation，四個價格欄位 + 成交量
// 全部由呼叫端指定（不像 observationParams 預設 NULL，這裡的用途就是要
// 驗證四個價格欄位都能被讀回來）。available_at 固定在 trade_date 當天
// +1 小時，落在 ObservedPriceByCrop/ByGroup 的 `to_date + 1 day` 可見窗內。
func mustInsertObservation(ctx context.Context, t *testing.T, q *gen.Queries, marketID, cropID int64, tradeDate string, revision int32, prices observationPrices) {
	t.Helper()
	date := mustParseDate(t, tradeDate)

	avg, err := pbpostgres.NumericFromString(mustFormatPrice(t, prices.avg))
	require.NoError(t, err)
	upper, err := pbpostgres.NumericFromString(mustFormatPrice(t, prices.upper))
	require.NoError(t, err)
	middle, err := pbpostgres.NumericFromString(mustFormatPrice(t, prices.middle))
	require.NoError(t, err)
	lower, err := pbpostgres.NumericFromString(mustFormatPrice(t, prices.lower))
	require.NoError(t, err)
	volume, err := pbpostgres.NumericFromString(mustFormatPrice(t, prices.volume))
	require.NoError(t, err)

	require.NoError(t, q.InsertPriceObservation(ctx, gen.InsertPriceObservationParams{
		ID:           opNextID(),
		MarketID:     marketID,
		CropID:       cropID,
		TradeDate:    pgtype.Date{Time: date, Valid: true},
		RevisionNo:   revision,
		AvailableAt:  pgtype.Timestamptz{Time: date.Add(1 * time.Hour), Valid: true},
		AvgPrice:     avg,
		UpperPrice:   upper,
		MiddlePrice:  middle,
		LowerPrice:   lower,
		VolumeKg:     volume,
		Source:       "test",
		SourceDigest: "digest-" + tradeDate,
	}))
}

// mustInsertObservationWithAvailableAt 是 mustInsertObservation 的變體，讓
// 呼叫端明確指定 available_at（預設的 trade_date+1h 不夠用來區分 cutoff
// 前後）——只有 cutoff 測試需要這個。
func mustInsertObservationWithAvailableAt(ctx context.Context, t *testing.T, q *gen.Queries, marketID, cropID int64, tradeDate string, revision int32, availableAt time.Time, prices observationPrices) {
	t.Helper()
	date := mustParseDate(t, tradeDate)

	avg, err := pbpostgres.NumericFromString(mustFormatPrice(t, prices.avg))
	require.NoError(t, err)
	upper, err := pbpostgres.NumericFromString(mustFormatPrice(t, prices.upper))
	require.NoError(t, err)
	middle, err := pbpostgres.NumericFromString(mustFormatPrice(t, prices.middle))
	require.NoError(t, err)
	lower, err := pbpostgres.NumericFromString(mustFormatPrice(t, prices.lower))
	require.NoError(t, err)
	volume, err := pbpostgres.NumericFromString(mustFormatPrice(t, prices.volume))
	require.NoError(t, err)

	require.NoError(t, q.InsertPriceObservation(ctx, gen.InsertPriceObservationParams{
		ID:           opNextID(),
		MarketID:     marketID,
		CropID:       cropID,
		TradeDate:    pgtype.Date{Time: date, Valid: true},
		RevisionNo:   revision,
		AvailableAt:  pgtype.Timestamptz{Time: availableAt, Valid: true},
		AvgPrice:     avg,
		UpperPrice:   upper,
		MiddlePrice:  middle,
		LowerPrice:   lower,
		VolumeKg:     volume,
		Source:       "test",
		SourceDigest: "digest-" + tradeDate + "-cutoff-test",
	}))
}

// TestObservedPricesByCropExcludesRevisionsNotYetVisibleAtCutoff covers
// review Finding I1 (2026-07-31): ObservedPriceByCrop must filter by an
// explicit cutoff parameter (point-in-time visibility), not derive it from
// to_date via `to_date + interval '1 day'`. Seeds a trade_date=T row whose
// available_at is T+2 days — the shape the daily cron's rolling re-ingest
// window produces when a row is late (see price_ingest.go/cron/daily.go) —
// and proves: (1) the OLD to_date-derived cutoff (T+1 day) would wrongly
// exclude it; (2) an explicit cutoff=now (what the usecase layer now passes)
// includes it even though the pinned trade_date window [T,T] is far in the
// past.
func TestObservedPricesByCropExcludesRevisionsNotYetVisibleAtCutoff(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID := mustUpsertMarket(ctx, t, q, "109", "台北一")
	cropID := mustUpsertCrop(ctx, t, q, "A1", "甘藍")
	tradeDate := mustParseDate(t, "2026-07-20")
	availableAt := tradeDate.AddDate(0, 0, 2) // late ingest: available 2 days after trade_date
	mustInsertObservationWithAvailableAt(ctx, t, q, marketID, cropID, "2026-07-20", 1, availableAt,
		observationPrices{avg: 25, upper: 30, middle: 25, lower: 20, volume: 500})

	repo := pbpostgres.NewForecastRepository(pool)

	// cutoff = to_date + 1 day (the OLD, buggy semantics this finding
	// removed) must NOT see this row: available_at is 2 days out, not 1.
	tooEarly := tradeDate.AddDate(0, 0, 1)
	got, err := repo.ObservedPricesByCrop(ctx, "109", "A1", tradeDate, tradeDate, tooEarly)
	if err != nil {
		t.Fatalf("ObservedPricesByCrop failed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d rows with cutoff=to_date+1day, want 0 (available_at is 2 days out; cutoff must be an independent parameter, not derived from to_date): %+v", len(got), got)
	}

	// cutoff = now (the caller's real "as of now" instant) DOES see it, even
	// though the pinned trade_date window [T,T] is far in the past.
	got, err = repo.ObservedPricesByCrop(ctx, "109", "A1", tradeDate, tradeDate, time.Now())
	if err != nil {
		t.Fatalf("ObservedPricesByCrop failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows with cutoff=now, want 1 (a pinned past trade_date must still surface rows whose available_at legitimately lags trade_date): %+v", len(got), got)
	}
}

// TestObservedPricesByCropReturnsAllFourPriceTiers 釘住 GetObservedPrice
// 存在的理由：SeriesObservations（P4 推論用）只選 avg_price，這支必須把
// upper/middle/lower 三個 P4 從不需要的欄位也讀出來。
func TestObservedPricesByCropReturnsAllFourPriceTiers(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID := mustUpsertMarket(ctx, t, q, "109", "台北一")
	cropID := mustUpsertCrop(ctx, t, q, "A1", "甘藍")
	mustInsertObservation(ctx, t, q, marketID, cropID, "2026-07-30", 1,
		observationPrices{avg: 25.5, upper: 30, middle: 25, lower: 20, volume: 1000})

	repo := pbpostgres.NewForecastRepository(pool)
	// cutoff generously past the fixture's available_at (trade_date + 1h) —
	// this test isn't exercising cutoff behavior itself, see
	// TestObservedPricesByCropExcludesRevisionsNotYetVisibleAtCutoff for that.
	got, err := repo.ObservedPricesByCrop(ctx, "109", "A1",
		mustParseDate(t, "2026-07-25"), mustParseDate(t, "2026-07-30"), mustParseDate(t, "2026-07-31"))
	if err != nil {
		t.Fatalf("ObservedPricesByCrop failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(got), got)
	}
	row := got[0]
	if row.AvgPrice == nil || *row.AvgPrice != 25.5 {
		t.Errorf("AvgPrice = %v, want 25.5", row.AvgPrice)
	}
	if row.UpperPrice == nil || *row.UpperPrice != 30 {
		t.Errorf("UpperPrice = %v, want 30 (this is the field SeriesObservations does NOT return)", row.UpperPrice)
	}
	if row.MiddlePrice == nil || *row.MiddlePrice != 25 {
		t.Errorf("MiddlePrice = %v, want 25", row.MiddlePrice)
	}
	if row.LowerPrice == nil || *row.LowerPrice != 20 {
		t.Errorf("LowerPrice = %v, want 20", row.LowerPrice)
	}
}

// TestObservedPricesByCropAndGroupReturnDataMetaSourceFields 是 issue #2249
// Slice A dataMeta 前置的回歸測試：ObservedPriceByCrop/ObservedPriceByGroup
// 現在必須把 source/source_digest/available_at/revision_no 四欄一併讀出來
// （domain.ObservedPrice 的新欄位），dataMeta 的 datasetDigest/
// freshnessStatus 都靠這幾欄算，不是另開一條查詢——晚一步在別的查詢重查
// 會違反「dataMeta 必須描述同一份 payload」的 snapshot 一致性規則。
func TestObservedPricesByCropAndGroupReturnDataMetaSourceFields(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID := mustUpsertMarket(ctx, t, q, "109", "台北一")
	cropID := mustUpsertCrop(ctx, t, q, "A1", "甘藍")
	availableAt := mustParseDate(t, "2026-07-30").Add(2 * time.Hour)
	mustInsertObservationWithAvailableAt(ctx, t, q, marketID, cropID, "2026-07-30", 3, availableAt,
		observationPrices{avg: 25.5, upper: 30, middle: 25, lower: 20, volume: 1000})

	repo := pbpostgres.NewForecastRepository(pool)

	t.Run("ObservedPricesByCrop", func(t *testing.T) {
		got, err := repo.ObservedPricesByCrop(ctx, "109", "A1",
			mustParseDate(t, "2026-07-25"), mustParseDate(t, "2026-07-30"), mustParseDate(t, "2026-07-31"))
		if err != nil {
			t.Fatalf("ObservedPricesByCrop failed: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d rows, want 1: %+v", len(got), got)
		}
		row := got[0]
		if row.Source != "test" {
			t.Errorf("Source = %q, want %q", row.Source, "test")
		}
		if row.SourceDigest != "digest-2026-07-30-cutoff-test" {
			t.Errorf("SourceDigest = %q, want %q", row.SourceDigest, "digest-2026-07-30-cutoff-test")
		}
		if row.RevisionNo != 3 {
			t.Errorf("RevisionNo = %d, want 3", row.RevisionNo)
		}
		if !row.AvailableAt.Equal(availableAt) {
			t.Errorf("AvailableAt = %v, want %v", row.AvailableAt, availableAt)
		}
	})

	t.Run("ObservedPricesByGroup", func(t *testing.T) {
		got, err := repo.ObservedPricesByGroup(ctx, "op-grp-A1", "",
			mustParseDate(t, "2026-07-25"), mustParseDate(t, "2026-07-30"), mustParseDate(t, "2026-07-31"))
		if err != nil {
			t.Fatalf("ObservedPricesByGroup failed: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d rows, want 1: %+v", len(got), got)
		}
		row := got[0]
		if row.Source != "test" {
			t.Errorf("Source = %q, want %q", row.Source, "test")
		}
		if row.SourceDigest != "digest-2026-07-30-cutoff-test" {
			t.Errorf("SourceDigest = %q, want %q", row.SourceDigest, "digest-2026-07-30-cutoff-test")
		}
		if row.RevisionNo != 3 {
			t.Errorf("RevisionNo = %d, want 3", row.RevisionNo)
		}
		if !row.AvailableAt.Equal(availableAt) {
			t.Errorf("AvailableAt = %v, want %v", row.AvailableAt, availableAt)
		}
	})
}

// TestObservedPricesByCropAndGroupReturnCategoryCode 是 issue #2249 Slice B
// trend-summary 前置的回歸測試：ObservedPriceByCrop/ObservedPriceByGroup 現在
// 必須把 c.category_code 一併讀出來（domain.ObservedPrice 的新欄位）——Task 4
// 的 period-completeness 判斷（見 domain/forecast/trend.go
// ExpectedTradingDay）靠這欄決定市場休市類別，晚一步在別的查詢重查一樣會
// 違反 dataMeta 必須描述同一份 payload 的 snapshot 一致性規則（同
// TestObservedPricesByCropAndGroupReturnDataMetaSourceFields 上方註解的理由）。
func TestObservedPricesByCropAndGroupReturnCategoryCode(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID := mustUpsertMarket(ctx, t, q, "109", "台北一")
	repo := pbpostgres.NewForecastRepository(pool)

	t.Run("ObservedPricesByCrop", func(t *testing.T) {
		cropID := mustUpsertCropWithCategory(ctx, t, q, "A1", "甘藍", "N04")
		mustInsertObservation(ctx, t, q, marketID, cropID, "2026-07-30", 1,
			observationPrices{avg: 25.5, upper: 30, middle: 25, lower: 20, volume: 1000})

		got, err := repo.ObservedPricesByCrop(ctx, "109", "A1",
			mustParseDate(t, "2026-07-25"), mustParseDate(t, "2026-07-30"), mustParseDate(t, "2026-07-31"))
		if err != nil {
			t.Fatalf("ObservedPricesByCrop failed: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d rows, want 1: %+v", len(got), got)
		}
		row := got[0]
		if row.CategoryCode == nil || *row.CategoryCode != "N04" {
			t.Errorf("CategoryCode = %v, want \"N04\"", row.CategoryCode)
		}
	})

	t.Run("ObservedPricesByGroup", func(t *testing.T) {
		groupID := mustUpsertCropGroupWithCategory(ctx, t, q, "op-grp-cc", "花椰菜", "N05")
		cropID := mustUpsertCropInGroupWithCategory(ctx, t, q, groupID, "C1", "花椰菜-白", "N05")
		mustInsertObservation(ctx, t, q, marketID, cropID, "2026-07-30", 1,
			observationPrices{avg: 40, upper: 50, middle: 40, lower: 30, volume: 500})

		got, err := repo.ObservedPricesByGroup(ctx, "op-grp-cc", "",
			mustParseDate(t, "2026-07-25"), mustParseDate(t, "2026-07-30"), mustParseDate(t, "2026-07-31"))
		if err != nil {
			t.Fatalf("ObservedPricesByGroup failed: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d rows, want 1: %+v", len(got), got)
		}
		row := got[0]
		if row.CategoryCode == nil || *row.CategoryCode != "N05" {
			t.Errorf("CategoryCode = %v, want \"N05\"", row.CategoryCode)
		}
	})
}

// TestObservedPricesByCropIncludesAllMarketsWhenMarketCodeEmpty covers review
// Finding I2 (2026-07-31): ObservedPriceByCrop used to have a bare
// `m.market_code = sqlc.arg(market_code)` filter with no empty-string bypass
// (unlike its sibling ObservedPriceByGroup) — silently returning zero rows
// whenever market_code was omitted, contradicting the proto's own "Omitted =
// all markets" doc comment. This also exercises the DISTINCT ON key fix that
// had to accompany the bypass: without m.market_code in the DISTINCT ON key,
// bypassing to multiple markets would collapse same-trade_date rows from
// different markets down to one (arbitrary) survivor — mirrors the exact
// defect class TestGroupObservationsIncludesAllMarketsWhenMarketCodeEmpty /
// ObservedPriceByGroup's DISTINCT ON key already guards against.
func TestObservedPricesByCropIncludesAllMarketsWhenMarketCodeEmpty(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	cropID := mustUpsertCrop(ctx, t, q, "A1", "甘藍")
	market109ID := mustUpsertMarket(ctx, t, q, "109", "台北一")
	market110ID := mustUpsertMarket(ctx, t, q, "110", "台北二")

	mustInsertObservation(ctx, t, q, market109ID, cropID, "2026-07-30", 1,
		observationPrices{avg: 25, upper: 30, middle: 25, lower: 20, volume: 500})
	mustInsertObservation(ctx, t, q, market110ID, cropID, "2026-07-30", 1,
		observationPrices{avg: 27, upper: 32, middle: 27, lower: 22, volume: 600})

	repo := pbpostgres.NewForecastRepository(pool)
	got, err := repo.ObservedPricesByCrop(ctx, "", "A1",
		mustParseDate(t, "2026-07-25"), mustParseDate(t, "2026-07-30"), mustParseDate(t, "2026-07-31"))
	if err != nil {
		t.Fatalf("ObservedPricesByCrop failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 (market_code=\"\" must return rows from both markets for the same crop_code, neither zero rows nor collapsed to one): %+v", len(got), got)
	}

	byMarket := make(map[string]domain.ObservedPrice, len(got))
	for _, row := range got {
		byMarket[row.MarketCode] = row
	}
	row109, ok := byMarket["109"]
	if !ok {
		t.Fatalf("missing market 109 row: %+v", got)
	}
	if row109.AvgPrice == nil || *row109.AvgPrice != 25 {
		t.Errorf("market 109 AvgPrice = %v, want 25", row109.AvgPrice)
	}
	row110, ok := byMarket["110"]
	if !ok {
		t.Fatalf("missing market 110 row (DISTINCT ON must not collapse multiple markets into one): %+v", got)
	}
	if row110.AvgPrice == nil || *row110.AvgPrice != 27 {
		t.Errorf("market 110 AvgPrice = %v, want 27", row110.AvgPrice)
	}
}

// TestObservedPricesByGroupIncludesAllMarketsAndCropsWhenMarketCodeEmpty 守
// ObservedPricesByGroup 的兩件事：(1) marketCode="" 的不過濾市場 bypass——同
// GroupObservations 那條先例 TestGroupObservationsIncludesAllMarketsWhenMarketCodeEmpty
// 測的 bypass 語意；(2) crop_group JOIN 真的涵蓋大類底下的多個子品項，不是
// 只挑一個。順帶驗證 upper/middle/lower 三個欄位在這條「大類版」查詢也真的
// 被讀出來（不只 ObservedPricesByCrop 那條 by-crop 版本才有）。
func TestObservedPricesByGroupIncludesAllMarketsAndCropsWhenMarketCodeEmpty(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	groupID := mustUpsertCropGroupWithCategory(ctx, t, q, "op-grp-b", "花椰菜", "V")

	market109ID := mustUpsertMarket(ctx, t, q, "109", "台北一")
	market110ID := mustUpsertMarket(ctx, t, q, "110", "台北二")

	cropB1ID := mustUpsertCropInGroup(ctx, t, q, groupID, "B1", "花椰菜-白")
	cropB2ID := mustUpsertCropInGroup(ctx, t, q, groupID, "B2", "花椰菜-青")

	mustInsertObservation(ctx, t, q, market109ID, cropB1ID, "2026-07-30", 1,
		observationPrices{avg: 40, upper: 50, middle: 40, lower: 30, volume: 500})
	mustInsertObservation(ctx, t, q, market110ID, cropB2ID, "2026-07-30", 1,
		observationPrices{avg: 60, upper: 70, middle: 60, lower: 50, volume: 800})

	repo := pbpostgres.NewForecastRepository(pool)
	got, err := repo.ObservedPricesByGroup(ctx, "op-grp-b", "",
		mustParseDate(t, "2026-07-25"), mustParseDate(t, "2026-07-30"), mustParseDate(t, "2026-07-31"))
	if err != nil {
		t.Fatalf("ObservedPricesByGroup failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 (both crops/markets under the group must come back, market_code=\"\" must not filter): %+v", len(got), got)
	}

	byCrop := make(map[string]domain.ObservedPrice)
	for _, row := range got {
		byCrop[row.CropCode] = row
	}

	rowB1, ok := byCrop["B1"]
	if !ok {
		t.Fatalf("missing B1 row: %+v", got)
	}
	if rowB1.MarketCode != "109" {
		t.Errorf("B1 MarketCode = %q, want 109", rowB1.MarketCode)
	}
	if rowB1.UpperPrice == nil || *rowB1.UpperPrice != 50 {
		t.Errorf("B1 UpperPrice = %v, want 50", rowB1.UpperPrice)
	}
	if rowB1.MiddlePrice == nil || *rowB1.MiddlePrice != 40 {
		t.Errorf("B1 MiddlePrice = %v, want 40", rowB1.MiddlePrice)
	}
	if rowB1.LowerPrice == nil || *rowB1.LowerPrice != 30 {
		t.Errorf("B1 LowerPrice = %v, want 30", rowB1.LowerPrice)
	}

	rowB2, ok := byCrop["B2"]
	if !ok {
		t.Fatalf("missing B2 row: %+v", got)
	}
	if rowB2.MarketCode != "110" {
		t.Errorf("B2 MarketCode = %q, want 110 (market_code=\"\" bypass must not collapse/drop markets)", rowB2.MarketCode)
	}
	if rowB2.UpperPrice == nil || *rowB2.UpperPrice != 70 {
		t.Errorf("B2 UpperPrice = %v, want 70", rowB2.UpperPrice)
	}
	if rowB2.MiddlePrice == nil || *rowB2.MiddlePrice != 60 {
		t.Errorf("B2 MiddlePrice = %v, want 60", rowB2.MiddlePrice)
	}
	if rowB2.LowerPrice == nil || *rowB2.LowerPrice != 50 {
		t.Errorf("B2 LowerPrice = %v, want 50", rowB2.LowerPrice)
	}
}

// TestCropGroupsByNameReturnsMultipleRowsWhenAmbiguous 守住 FindCropGroupByName
// 本來就記錄的歧義事實：同一個 crop_group 名稱可能在不同 category_code 下
// 各自存在一列（group_code 的 hash 含 category_code）。CropGroupsByName 是
// 這條查詢的 domain 包裝，不能把多筆悄悄併成一筆。
func TestCropGroupsByNameReturnsMultipleRowsWhenAmbiguous(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	// Same crop_group name under two different category codes — the known
	// ambiguity shape from wiki Pending "crop_group 舊列清理": group_code's
	// hash includes category_code, so "甘藍" can legitimately exist twice.
	mustUpsertCropGroupWithCategory(ctx, t, q, "grp-v", "甘藍", "N04")
	mustUpsertCropGroupWithCategory(ctx, t, q, "grp-f", "甘藍", "N05")

	repo := pbpostgres.NewForecastRepository(pool)
	got, err := repo.CropGroupsByName(ctx, "甘藍")
	if err != nil {
		t.Fatalf("CropGroupsByName failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 (ambiguous name must not be silently collapsed to one): %+v", len(got), got)
	}
}
