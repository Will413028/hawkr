//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	pbpostgres "github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// 固定 ID：測試不需要真 TSID，用可讀的定值比較好斷言。
const (
	testMarketID    int64 = 700000000000000001
	testCropGroupID int64 = 700000000000000002
	testCropID      int64 = 700000000000000003
	testObsIDBase   int64 = 700000000000001000
)

var testTradeDate = time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

// seedSeries 建立一組 market / crop_group / crop，回傳 (marketID, cropID)。
// Task 6 的 digest 去重測試可以直接重用 setupTestDB + seedSeries，不需要另立 harness。
func seedSeries(ctx context.Context, t *testing.T, q *gen.Queries) (int64, int64) {
	t.Helper()

	marketID, err := q.UpsertMarket(ctx, gen.UpsertMarketParams{
		ID:         testMarketID,
		MarketCode: "104",
		Name:       "台北一",
	})
	require.NoError(t, err)

	categoryCode := "V"
	variantName := "初秋"
	groupID, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID:           testCropGroupID,
		GroupCode:    "LA",
		Name:         "葉菜類",
		CategoryCode: &categoryCode,
	})
	require.NoError(t, err)

	cropID, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID:           testCropID,
		CropCode:     "LA1",
		GroupID:      groupID,
		Name:         "甘藍",
		VariantName:  &variantName,
		CategoryCode: &categoryCode,
	})
	require.NoError(t, err)

	return marketID, cropID
}

// observationParams 產生一筆行情觀測參數；價格欄位一律由呼叫端指定，
// 預設就是 pgtype.Numeric 零值（= SQL NULL），不會偷偷變成 0。
func observationParams(t *testing.T, id, marketID, cropID int64, revision int32, digest string) gen.InsertPriceObservationParams {
	t.Helper()

	return gen.InsertPriceObservationParams{
		ID:           id,
		MarketID:     marketID,
		CropID:       cropID,
		TradeDate:    pgtype.Date{Time: testTradeDate, Valid: true},
		RevisionNo:   revision,
		AvailableAt:  pgtype.Timestamptz{Time: time.Date(2026, 7, 21, 2, 0, 0, 0, time.UTC), Valid: true},
		Source:       "moa",
		SourceDigest: digest,
	}
}

// TestRevisionUniquenessAndAppend 釘住「官方修訂用新 revision 插入」這件事在 schema 層
// 真的成立：同一條 series 的 revision 可以往上追加，但同一個 (market, crop, date, revision)
// 不得出現第二筆——否則「最新修訂」就不再唯一，Task 6 的修訂偵測會讀到誰算誰。
//
// ⚠️ 本測試**不**證明 revision_no 單調遞增。schema 只有 UNIQUE，沒有任何約束阻止在
// revision 3 之後插入 revision 1。單調遞增是**應用層的義務**（Task 6 必須以
// LatestObservationRevision 的結果 +1 來決定新 revision），不是 DB 給的保證。
func TestRevisionUniquenessAndAppend(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := seedSeries(ctx, t, q)

	// revision 1、2 依序插入都要成功。
	require.NoError(t, q.InsertPriceObservation(ctx,
		observationParams(t, testObsIDBase+1, marketID, cropID, 1, "digest-rev1")))
	require.NoError(t, q.InsertPriceObservation(ctx,
		observationParams(t, testObsIDBase+2, marketID, cropID, 2, "digest-rev2")))

	// 同一組 (market, crop, date, revision) 再插一筆（換 id、換 digest）必須被拒。
	err := q.InsertPriceObservation(ctx,
		observationParams(t, testObsIDBase+3, marketID, cropID, 2, "digest-rev2-dup"))
	require.Error(t, err, "重複的 (market, crop, trade_date, revision_no) 必須被 UNIQUE 擋下")

	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "預期 PgError，實得 %T: %v", err, err)
	require.Equal(t, "23505", pgErr.Code, "預期 unique_violation")
	require.Equal(t,
		"price_observation_market_id_crop_id_trade_date_revision_no_key",
		pgErr.ConstraintName)

	// 被拒的那筆不得留下殘骸。
	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM price_observation WHERE market_id = $1 AND crop_id = $2 AND trade_date = $3`,
		marketID, cropID, testTradeDate).Scan(&count))
	require.Equal(t, 2, count)

	// 追加更高的 revision 是允許的（append-only，不是 UPDATE）。
	// 注意這只證明「可以追加」，不證明「只能往上追加」——見函式註解。
	require.NoError(t, q.InsertPriceObservation(ctx,
		observationParams(t, testObsIDBase+4, marketID, cropID, 3, "digest-rev3")))

	var maxRevision int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT max(revision_no) FROM price_observation WHERE market_id = $1 AND crop_id = $2 AND trade_date = $3`,
		marketID, cropID, testTradeDate).Scan(&maxRevision))
	require.Equal(t, int32(3), maxRevision)
}

// TestAvgPriceNullIsNotZero 是本檔最重要的一條：avg_price 缺值必須存成 NULL 並讀回 NULL，
// 絕不能變成 0。0 是合法價格——把休市寫成 0 元會讓 Task 9 的對帳看到一場不存在的崩盤，
// 而且從資料上完全看不出是壞掉的。
func TestAvgPriceNullIsNotZero(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := seedSeries(ctx, t, q)

	// 第一步：schema 本身不得帶 NOT NULL / DEFAULT 0。
	// 直接查 information_schema，讓「有人日後加上 NOT NULL DEFAULT 0」當場變紅。
	for _, col := range []string{"avg_price", "upper_price", "middle_price", "lower_price", "volume_kg"} {
		var isNullable string
		var columnDefault *string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT is_nullable, column_default FROM information_schema.columns
			 WHERE table_schema = 'public' AND table_name = 'price_observation' AND column_name = $1`,
			col).Scan(&isNullable, &columnDefault))
		require.Equal(t, "YES", isNullable, "%s 必須可為 NULL（NULL = 休市或缺值）", col)
		require.Nil(t, columnDefault, "%s 不得有預設值，尤其不得是 0", col)
	}

	// 第二步：缺值（pgtype.Numeric 零值）寫進去要是 NULL。
	missing := observationParams(t, testObsIDBase+1, marketID, cropID, 1, "digest-missing")
	require.False(t, missing.AvgPrice.Valid, "測資本身必須是 NULL 才有意義")
	require.NoError(t, q.InsertPriceObservation(ctx, missing))

	// 第三步：0 元（合法價格）寫進去要是 0，不是 NULL。
	zeroPrice, err := pbpostgres.NumericFromString("0.0000")
	require.NoError(t, err)
	priced := observationParams(t, testObsIDBase+2, marketID, cropID, 2, "digest-zero")
	priced.AvgPrice = zeroPrice
	require.NoError(t, q.InsertPriceObservation(ctx, priced))

	// 用 SQL 的 IS NULL 直接分辨兩筆——這是不會被 Go 端型別轉換掩蓋的判準。
	var nullCount, zeroCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM price_observation WHERE id = $1 AND avg_price IS NULL`,
		testObsIDBase+1).Scan(&nullCount))
	require.Equal(t, 1, nullCount, "缺值那筆的 avg_price 必須是 SQL NULL")

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM price_observation WHERE id = $1 AND avg_price = 0`,
		testObsIDBase+2).Scan(&zeroCount))
	require.Equal(t, 1, zeroCount, "0 元那筆的 avg_price 必須真的是 0")

	// 兩筆之間必須互斥：缺值那筆不得同時滿足 = 0。
	var nullRowIsZero int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM price_observation WHERE id = $1 AND avg_price = 0`,
		testObsIDBase+1).Scan(&nullRowIsZero))
	require.Equal(t, 0, nullRowIsZero, "NULL 不得被當成 0")

	// 第四步：讀回 Go 端也要保住這個分別。
	readAvgPrice := func(id int64) pgtype.Numeric {
		t.Helper()
		var n pgtype.Numeric
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT avg_price FROM price_observation WHERE id = $1`, id).Scan(&n))
		return n
	}

	gotNull := readAvgPrice(testObsIDBase + 1)
	require.False(t, gotNull.Valid, "NULL 讀回來必須是 Valid=false")
	require.Nil(t, pbpostgres.NumericToPtr(gotNull), "NULL 讀回來必須是 nil，不是 \"0\"")

	gotZero := readAvgPrice(testObsIDBase + 2)
	require.True(t, gotZero.Valid, "0 讀回來必須是 Valid=true")
	zeroStr := pbpostgres.NumericToPtr(gotZero)
	require.NotNil(t, zeroStr, "0 讀回來不得變成 nil")
	require.Equal(t, "0.0000", *zeroStr, "0 元必須保留 NUMERIC(12,4) 的位數")
}

// TestLatestObservationRevision 釘住 Task 6 依賴的查詢：回傳的是該 series 最大的
// revision_no 與「那一筆」的 digest，而不是最後插入的那筆。
func TestLatestObservationRevision(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := seedSeries(ctx, t, q)

	// 刻意亂序插入：1 → 3 → 2。若查詢誤用插入順序就會回 digest-rev2。
	require.NoError(t, q.InsertPriceObservation(ctx,
		observationParams(t, testObsIDBase+1, marketID, cropID, 1, "digest-rev1")))
	require.NoError(t, q.InsertPriceObservation(ctx,
		observationParams(t, testObsIDBase+3, marketID, cropID, 3, "digest-rev3")))
	require.NoError(t, q.InsertPriceObservation(ctx,
		observationParams(t, testObsIDBase+2, marketID, cropID, 2, "digest-rev2")))

	got, err := q.LatestObservationRevision(ctx, gen.LatestObservationRevisionParams{
		MarketID:  marketID,
		CropID:    cropID,
		TradeDate: pgtype.Date{Time: testTradeDate, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(3), got.RevisionNo)
	require.Equal(t, "digest-rev3", got.SourceDigest, "digest 必須來自最大 revision 那一筆")

	// 另一個 trade_date 不得被算進來（WHERE 有涵蓋 trade_date）。
	otherDate := testTradeDate.AddDate(0, 0, 1)
	other := observationParams(t, testObsIDBase+9, marketID, cropID, 99, "digest-other-day")
	other.TradeDate = pgtype.Date{Time: otherDate, Valid: true}
	require.NoError(t, q.InsertPriceObservation(ctx, other))

	got, err = q.LatestObservationRevision(ctx, gen.LatestObservationRevisionParams{
		MarketID:  marketID,
		CropID:    cropID,
		TradeDate: pgtype.Date{Time: testTradeDate, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(3), got.RevisionNo, "別的日期的 revision 不得溢進來")
	require.Equal(t, "digest-rev3", got.SourceDigest)

	// 尚無任何觀測的 series 回 pgx.ErrNoRows（preflight B19；Task 6 必須處理這個分支）。
	_, err = q.LatestObservationRevision(ctx, gen.LatestObservationRevisionParams{
		MarketID:  marketID,
		CropID:    cropID,
		TradeDate: pgtype.Date{Time: testTradeDate.AddDate(0, 0, 30), Valid: true},
	})
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

// coMonotonicViolationSQL 是 SeriesObservationsAsOf／GroupObservationsAsOf 的
// tie-break 正確性所依賴的判準：同一 (market, crop, trade_date) 內，
// revision_no 較大的列其 available_at 不得較早。ORDER BY revision_no DESC
// 選「cutoff 當下最新的那筆」，等價於上游按 knowledge_from 區間判定的前提，
// 就是兩者同向。
//
// 🔴 schema 層沒有任何 CHECK constraint 保證這件事——它是 ingest 寫入邏輯的
// 性質（source_digest 不同才插 revision_no+1、available_at 取當次 ingest
// 時間），不是 DB 給的保證。下面兩條測試分別守「真實 ingest 路徑是否仍然
// 遵守這個不變量」（TestRevisionNoAndAvailableAtAreCoMonotonicAcrossPriceIngestRuns）
// 與「這條判準 SQL 本身有沒有偵測力，不是恆真」
// （TestRevisionNoAndAvailableAtInvariantSQLHasDetectionPower）——
// 舊版只做了前者的「正面」半邊、且只餵自己造的合規資料，斷言必然恆真
// （review Finding 2，2026-07-30），已改寫。
const coMonotonicViolationSQL = `
	SELECT count(*) FROM price_observation a JOIN price_observation b
	ON a.market_id = b.market_id AND a.crop_id = b.crop_id AND a.trade_date = b.trade_date
	WHERE a.revision_no > b.revision_no AND a.available_at < b.available_at
`

// coMonotonicRestDaysFixture 只宣告市場 104，讓 PriceIngest.Handle 的市場迴圈
// 只跑一輪（比照 internal/usecase/price_ingest_integration_test.go 的
// newPriceFixtureServer：D8 之後 Handle 先呼叫 FetchRestDays() 取得市場清單，
// fixture server 必須依 URL path 分流，不能對任何路徑都回同一份行情 fixture）。
const coMonotonicRestDaysFixture = `{"RS":"OK","Data":[
 {"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
   {"MarketType":"F","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05"}]}]}
 ]}
]}`

const coMonotonicTradeFixtureV1 = `{"RS":"OK","Data":[
 {"TransDate":"115.07.26","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":46.2,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957}
]}`

const coMonotonicTradeFixtureV2 = `{"RS":"OK","Data":[
 {"TransDate":"115.07.26","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":47.0,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957}
]}`

func newCoMonotonicFixtureServer(t *testing.T, tradeBody string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "MarketRestDayFarmWCF") {
			_, _ = w.Write([]byte(coMonotonicRestDaysFixture))
			return
		}
		_, _ = w.Write([]byte(tradeBody))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newCoMonotonicSeqTSID(start int64) func() int64 {
	next := start
	return func() int64 {
		next++
		return next
	}
}

// TestRevisionNoAndAvailableAtAreCoMonotonicAcrossPriceIngestRuns 用真正的
// ingest 路徑（usecase.PriceIngest.Handle）驅動兩輪同一 (market, crop,
// trade_date) 的抓取——第二輪價格不同，觸發 source_digest 不同 →
// revision_no+1，available_at 取第二輪的 ingest 時間，必然晚於第一輪——再跑
// coMonotonicViolationSQL 斷言 0。
//
// 這條測試守的是「ingest 的寫入邏輯是否仍然保證 revision_no 與 available_at
// 同向遞增」，走的是本服務唯一真正產生 revision 的程式路徑，不是像舊版那樣
// 自己造兩列已經合規的資料再斷言合規（那種寫法的結論由建構方式保證，永遠不
// 會紅）。未來若有人改壞 ingest 的 revision/available_at 配對邏輯，這裡會紅。
func TestRevisionNoAndAvailableAtAreCoMonotonicAcrossPriceIngestRuns(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	tsid := newCoMonotonicSeqTSID(981200000000000000)

	srv1 := newCoMonotonicFixtureServer(t, coMonotonicTradeFixtureV1)
	ingest1 := usecase.NewPriceIngest(moa.NewClient(srv1.URL, srv1.URL), pool, tsid)
	stats1, err := ingest1.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 1, stats1.Inserted, "第一輪必須是全新 insert，測試才驅動到後面的 revise 分支")

	srv2 := newCoMonotonicFixtureServer(t, coMonotonicTradeFixtureV2)
	ingest2 := usecase.NewPriceIngest(moa.NewClient(srv2.URL, srv2.URL), pool, tsid)
	stats2, err := ingest2.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 1, stats2.Revised, "第二輪價格不同必須觸發新 revision，不然這條測試根本沒驅動到 revision_no+1 的路徑")

	var violations int
	require.NoError(t, pool.QueryRow(ctx, coMonotonicViolationSQL).Scan(&violations))
	require.Equal(t, 0, violations, "真正 ingest 出來的兩輪 revision 必須同向遞增")
}

// TestRevisionNoAndAvailableAtInvariantSQLHasDetectionPower 證明
// coMonotonicViolationSQL 本身有偵測力、不是恆真：先在合規資料上斷言 0，
// 手動插入一列違反不變量的資料（revision_no 較大但 available_at 早 48h），
// 斷言判準立刻回非 0，刪掉那筆之後斷言判準回到 0。
//
// 這是「反證」而非「正面路徑」：schema 層沒有任何 CHECK constraint 擋下這種
// 寫入，所以判準 SQL 是唯一防線——這條測試證明那道防線真的會響，不是裝飾
// （review Finding 2 的實測：revision_no=3、available_at 早 48h 這組配置下，
// 判準回 violations=2——插入的違規列同時比 rev1、rev2 都違規，各算一組）。
func TestRevisionNoAndAvailableAtInvariantSQLHasDetectionPower(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	marketID, cropID := seedSeries(ctx, t, q)

	t1 := time.Date(2026, 7, 21, 2, 0, 0, 0, time.UTC)
	t2 := t1.Add(24 * time.Hour)

	rev1 := observationParams(t, testObsIDBase+1, marketID, cropID, 1, "digest-rev1")
	rev1.AvailableAt = pgtype.Timestamptz{Time: t1, Valid: true}
	require.NoError(t, q.InsertPriceObservation(ctx, rev1))

	rev2 := observationParams(t, testObsIDBase+2, marketID, cropID, 2, "digest-rev2")
	rev2.AvailableAt = pgtype.Timestamptz{Time: t2, Valid: true}
	require.NoError(t, q.InsertPriceObservation(ctx, rev2))

	assertViolations := func(want int, msg string) {
		t.Helper()
		var got int
		require.NoError(t, pool.QueryRow(ctx, coMonotonicViolationSQL).Scan(&got))
		require.Equal(t, want, got, msg)
	}

	assertViolations(0, "合規資料下判準必須是 0（否則下面的非 0 斷言毫無意義）")

	// 手動插入一列違反不變量的資料：revision_no=3 比 rev1/rev2 都大，
	// 但 available_at 比 rev1 的 t1 還早 48h。schema 沒有 CHECK 擋這個寫入。
	violatingAt := t1.Add(-48 * time.Hour)
	violating := observationParams(t, testObsIDBase+3, marketID, cropID, 3, "digest-violating")
	violating.AvailableAt = pgtype.Timestamptz{Time: violatingAt, Valid: true}
	require.NoError(t, q.InsertPriceObservation(ctx, violating))

	assertViolations(2, "插入違規列後判準必須偵測到（違規列 vs rev1、違規列 vs rev2 各一組），證明判準不是恆真")

	_, err := pool.Exec(ctx, `DELETE FROM price_observation WHERE id = $1`, testObsIDBase+3)
	require.NoError(t, err)

	assertViolations(0, "刪掉違規列後判準必須回到 0")
}
