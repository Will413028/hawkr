//go:build integration

package usecase_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

const backfillRestDaysFixture = `{"RS":"OK","Data":[
 {"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
   {"MarketType":"F","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05"}]}]}
 ]}
]}`

const backfillJanTradeFixture = `{"RS":"OK","Data":[
 {"TransDate":"115.01.20","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":46.2,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957}
]}`

const backfillFebTradeFixture = `{"RS":"OK","Data":[
 {"TransDate":"115.02.01","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0},
 {"TransDate":"115.02.02","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0},
 {"TransDate":"115.02.03","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0},
 {"TransDate":"115.02.04","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0},
 {"TransDate":"115.02.05","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":50.0,"Upper_Price":58.0,"Middle_Price":51.0,"Lower_Price":38.0,"Trans_Quantity":9000},
 {"TransDate":"115.02.06","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0},
 {"TransDate":"115.02.07","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0},
 {"TransDate":"115.02.08","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0},
 {"TransDate":"115.02.09","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0},
 {"TransDate":"115.02.10","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0}
]}`

const backfillEmptyTradeFixture = `{"RS":"OK","Data":[]}`

const backfillAllMalformedTradeFixture = `{"RS":"OK","Data":[
 {"TransDate":"2026-01-20","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":46.2,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957},
 {"TransDate":"2026-01-21","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":48.0,"Upper_Price":55.0,"Middle_Price":49.0,"Lower_Price":35.0,"Trans_Quantity":11000}
]}`

func newBackfillFixtureServer(t *testing.T, monthBodies map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "MarketRestDayFarmWCF") {
			_, _ = w.Write([]byte(backfillRestDaysFixture))
			return
		}
		start := r.URL.Query().Get("Start_time")
		prefix := start
		if len(start) >= 6 {
			prefix = start[:6]
		}
		body, ok := monthBodies[prefix]
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestBackfillHandleSucceedsAcrossMultipleChunks(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newBackfillFixtureServer(t, map[string]string{
		"115.01": backfillJanTradeFixture,
		"115.02": backfillFebTradeFixture,
	})
	tsid := newSeqTSID(950000000000000000)
	priceIngest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, tsid)
	backfill := usecase.NewBackfill(priceIngest, pool, tsid)

	start := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)

	runID, err := backfill.Handle(ctx, start, end)
	require.NoError(t, err)
	require.NotZero(t, runID)

	var status int16
	var fetched, inserted, revised int32
	var rangeStart, rangeEnd time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, fetched_count, inserted_count, revised_count, range_start, range_end
		 FROM ingest_run WHERE id = $1`, runID).
		Scan(&status, &fetched, &inserted, &revised, &rangeStart, &rangeEnd))
	require.Equal(t, domain.IngestStatusSucceeded, status)
	require.Equal(t, int32(11), fetched,
		"一月一筆成交（Fetched=1）+ 二月 9 天休市哨兵＋1 天成交（Fetched=10，見 backfillFebTradeFixture 註解），共 11")
	require.Equal(t, int32(2), inserted, "休市哨兵列不落地，只有兩個月各一筆真實成交算 Inserted")
	require.Equal(t, int32(0), revised)
	require.Equal(t, "2026-01-15", rangeStart.Format("2006-01-02"))
	require.Equal(t, "2026-02-10", rangeEnd.Format("2006-01-02"))

	var runCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM ingest_run`).Scan(&runCount))
	require.Equal(t, 1, runCount, "一次 backfill（不論跨幾個月）只應留下一列 ingest_run")
}

func TestBackfillHandleMarksFailedWithoutRollingBackEarlierChunks(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newBackfillFixtureServer(t, map[string]string{
		"115.01": backfillJanTradeFixture,
	})
	tsid := newSeqTSID(960000000000000000)
	priceIngest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, tsid)
	backfill := usecase.NewBackfill(priceIngest, pool, tsid)

	start := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)

	runID, err := backfill.Handle(ctx, start, end)
	require.Error(t, err)
	require.NotZero(t, runID, "即使失敗，仍應回傳已建立的 ingest_run id 供人工排查")

	var status int16
	var errMsg string
	var inserted int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, error_message, inserted_count FROM ingest_run WHERE id = $1`, runID).
		Scan(&status, &errMsg, &inserted))
	require.Equal(t, domain.IngestStatusFailed, status)
	require.NotEmpty(t, errMsg)
	require.Equal(t, int32(1), inserted, "一月段落的成功計數應保留在稽核紀錄裡")

	var obsCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM price_observation`).Scan(&obsCount))
	require.Equal(t, 1, obsCount, "一月已成功寫入的 price_observation 不應因二月失敗被回滾")
}

func TestBackfillHandleMarksNeedsAttentionWhenEverythingSkipped(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newBackfillFixtureServer(t, map[string]string{
		"115.01": backfillAllMalformedTradeFixture,
	})
	tsid := newSeqTSID(970000000000000000)
	priceIngest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, tsid)
	backfill := usecase.NewBackfill(priceIngest, pool, tsid)

	start := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)

	runID, err := backfill.Handle(ctx, start, end)
	require.NoError(t, err, "整段被跳過不是流程失敗——Handle 不該回傳 error，正因如此才需要靠 status 留下訊號")
	require.NotZero(t, runID)

	var status int16
	var errMsg string
	var fetched, inserted, revised int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, COALESCE(error_message, ''), fetched_count, inserted_count, revised_count
		 FROM ingest_run WHERE id = $1`, runID).
		Scan(&status, &errMsg, &fetched, &inserted, &revised))
	require.Equal(t, domain.IngestStatusNeedsAttention, status,
		"抓到列卻一列都沒落地，必須與「本來就沒資料」在稽核表上可區分")
	require.NotEmpty(t, errMsg, "ingest_run 沒有 skipped_count 欄位，error_message 是唯一的持久化訊號")
	require.NotZero(t, fetched, "資料有抓到，問題出在解析")
	require.Zero(t, inserted)
	require.Zero(t, revised)

	var obsCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM price_observation`).Scan(&obsCount))
	require.Zero(t, obsCount, "全數解析失敗時不應有任何 price_observation 落地")
}

func TestBackfillHandleClampsStartToHistoricalFloor(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newBackfillFixtureServer(t, map[string]string{
		"101.01": backfillEmptyTradeFixture,
		"101.02": backfillEmptyTradeFixture,
	})
	tsid := newSeqTSID(970000000000000000)
	priceIngest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, tsid)
	backfill := usecase.NewBackfill(priceIngest, pool, tsid)

	start := time.Date(2005, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2012, 2, 15, 0, 0, 0, 0, time.UTC)

	runID, err := backfill.Handle(ctx, start, end)
	require.NoError(t, err)

	var status int16
	var rangeStart, rangeEnd time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, range_start, range_end FROM ingest_run WHERE id = $1`, runID).
		Scan(&status, &rangeStart, &rangeEnd))
	require.Equal(t, domain.IngestStatusSucceeded, status)
	require.Equal(t, "2012-01-01", rangeStart.Format("2006-01-02"),
		"range_start 記錄的應是夾到歷史下限後的日期，不是原始請求的 2005-06-01")
	require.Equal(t, "2012-02-15", rangeEnd.Format("2006-01-02"))
}

func TestBackfillAccumulatesEmptyAfterRetryAcrossMonths(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newBackfillFixtureServer(t, map[string]string{
		"115.01": backfillJanTradeFixture,
		"115.02": backfillEmptyTradeFixture,
	})
	priceIngest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(930000000000000000))
	priceIngest.SetRetryDelayForTest(time.Millisecond)
	backfill := usecase.NewBackfill(priceIngest, pool, newSeqTSID(930000000001000000))

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	runID, err := backfill.Handle(ctx, start, end)
	if err != nil {
		t.Fatalf("Backfill.Handle 不應回傳錯誤（持續空手不是 infra 錯誤）: %v", err)
	}

	run, err := getIngestRunForTest(ctx, pool, runID)
	require.NoError(t, err)
	require.Equal(t, domain.IngestStatusNeedsAttention, run.Status)
	require.NotNil(t, run.ErrorMessage)
	require.Contains(t, *run.ErrorMessage, "104")
}

func getIngestRunForTest(ctx context.Context, pool *pgxpool.Pool, runID int64) (gen.IngestRun, error) {
	return gen.New(pool).GetIngestRunByID(ctx, runID)
}
