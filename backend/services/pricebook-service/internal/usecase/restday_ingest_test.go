//go:build integration

package usecase_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

const restDayFixture = `{"RS":"OK","Data":[
 {"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
   {"MarketType":"F","YearList":[{"Year":115,"MonthList":[
     {"Month":1,"Rest":"05、12、19、26"},
     {"Month":2,"Rest":"02、09"}
   ]}]}
 ]},
 {"MarkerNo":"108","MarkerName":"台中","MarketTypeList":[
   {"MarketType":"F","YearList":[{"Year":115,"MonthList":[
     {"Month":1,"Rest":"06、13、20、27"}
   ]}]}
 ]}
]}`

func newSeqTSID(start int64) func() int64 {
	next := start
	return func() int64 {
		next++
		return next
	}
}

func TestHandleUpsertsMarketsAndRestDaysIdempotently(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(restDayFixture))
	}))
	t.Cleanup(srv.Close)

	client := moa.NewClient(srv.URL, srv.URL)
	ingest := usecase.NewRestDayIngest(client, pool, newSeqTSID(900000000000000000), zap.NewNop())

	written, err := ingest.Handle(ctx)
	require.NoError(t, err)
	require.Equal(t, 10, written, "104 有 6 個休市日、108 有 4 個，共 10 筆")

	var marketCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM market`).Scan(&marketCount))
	require.Equal(t, 2, marketCount, "同一市場出現在多筆休市日，market 表應只收斂成 2 列")

	var restDayCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM market_rest_day`).Scan(&restDayCount))
	require.Equal(t, 10, restDayCount)

	var marketID104 int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM market WHERE market_code = '104'`).Scan(&marketID104))
	var linkedCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM market_rest_day WHERE market_id = $1`, marketID104).Scan(&linkedCount))
	require.Equal(t, 6, linkedCount, "104 的 6 個休市日都應連到同一個 market id")

	rows, err := pool.Query(ctx,
		`SELECT rest_date, reason IS NULL, fetched_at IS NOT NULL
		 FROM market_rest_day WHERE market_id = $1 ORDER BY rest_date`, marketID104)
	require.NoError(t, err)
	defer rows.Close()

	var gotDates []string
	for rows.Next() {
		var d time.Time
		var reasonIsNull, fetchedAtNotNull bool
		require.NoError(t, rows.Scan(&d, &reasonIsNull, &fetchedAtNotNull))
		gotDates = append(gotDates, d.Format("2006-01-02"))
		require.True(t, reasonIsNull, "MOA 休市日曆回應沒有原因欄位，reason 應存 NULL 而非空字串")
		require.True(t, fetchedAtNotNull, "fetched_at 必須有值")
	}
	require.NoError(t, rows.Err())
	require.Equal(t,
		[]string{"2026-01-05", "2026-01-12", "2026-01-19", "2026-01-26", "2026-02-02", "2026-02-09"},
		gotDates,
		"104 的休市日期必須逐一對上 fixture 展開後的日期，而不是只數量對")

	written2, err := ingest.Handle(ctx)
	require.NoError(t, err)
	require.Equal(t, 10, written2)

	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM market`).Scan(&marketCount))
	require.Equal(t, 2, marketCount, "重跑不應新增 market 列")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM market_rest_day`).Scan(&restDayCount))
	require.Equal(t, 10, restDayCount, "重跑不應新增 market_rest_day 列（冪等）")
}

const twoMarketTypeRestDayIngestFixture = `{"RS":"OK","Data":[{
  "MarkerNo":"400","MarkerName":"台中市",
  "MarketTypeList":[
    {"MarketType":"V","YearList":[{"Year":115,"MonthList":[{"Month":7,"Rest":"05、12"}]}]},
    {"MarketType":"L","YearList":[{"Year":115,"MonthList":[{"Month":7,"Rest":"05、20"}]}]}
  ]}],"Next":false}`

func TestRestDayIngestKeysByMarketTypeAndCleansLegacyRows(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(twoMarketTypeRestDayIngestFixture))
	}))
	t.Cleanup(srv.Close)

	ingest := usecase.NewRestDayIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(980000000000000000), zap.NewNop())

	var marketID int64
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO market (id, market_code, name) VALUES ($1,'400','台中市') RETURNING id`,
		int64(980000000000000900)).Scan(&marketID))
	_, err := pool.Exec(ctx,
		`INSERT INTO market_rest_day (id, market_id, market_type, rest_date, fetched_at)
		 VALUES ($1,$2,NULL,'2026-07-20',now())`,
		int64(980000000000000901), marketID)
	require.NoError(t, err)

	written, err := ingest.Handle(ctx)
	require.NoError(t, err)
	require.Equal(t, 4, written, "V 兩天 + L 兩天，跨 type 不去重")

	var legacy int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM market_rest_day WHERE market_type IS NULL`).Scan(&legacy))
	require.Equal(t, 0, legacy, "無 market_type 的舊列必須被清掉")

	var sameDayBothTypes int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM market_rest_day WHERE rest_date = '2026-07-05'`).Scan(&sameDayBothTypes))
	require.Equal(t, 2, sameDayBothTypes, "同一天在 V 與 L 下各一列")

	written2, err := ingest.Handle(ctx)
	require.NoError(t, err)
	require.Equal(t, 4, written2)
	var total int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM market_rest_day`).Scan(&total))
	require.Equal(t, 4, total)
}

const blankNameRestDayFixture = `{"RS":"OK","Data":[
 {"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
   {"MarketType":"F","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05、12"}]}]}
 ]},
 {"MarkerNo":"999","MarkerName":"","MarketTypeList":[
   {"MarketType":"F","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"06、13"}]}]}
 ]}
]}`

func TestRestDayIngestQuarantinesBlankMarketNameAndKeepsSiblings(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(blankNameRestDayFixture))
	}))
	t.Cleanup(srv.Close)

	client := moa.NewClient(srv.URL, srv.URL)
	ingest := usecase.NewRestDayIngest(client, pool, newSeqTSID(916000000000000000), zap.NewNop())

	written, err := ingest.Handle(ctx)
	require.NoError(t, err, "單一市場沒有名字不得讓整條休市日曆線失敗")
	require.Equal(t, 2, written, "只有帶完整身分的 104 那兩天該落地")

	var marketCodes []string
	rows, err := pool.Query(ctx, `SELECT market_code FROM market ORDER BY market_code`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var code string
		require.NoError(t, rows.Scan(&code))
		marketCodes = append(marketCodes, code)
	}
	require.Equal(t, []string{"104"}, marketCodes, "沒有名字的市場不得建立")

	var restDayCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM market_rest_day`).Scan(&restDayCount))
	require.Equal(t, 2, restDayCount)

	var reason string
	var occurrences int64
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT reason, occurrence_count FROM ingest_rejected_identity
		WHERE market_code = '999'`).Scan(&reason, &occurrences))
	require.Equal(t, "missing_market_name", reason)
	require.Equal(t, int64(1), occurrences, "同一個市場在一輪裡只登記一次，不是每個休市日一次")
}
