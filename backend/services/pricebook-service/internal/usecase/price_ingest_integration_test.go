//go:build integration

package usecase_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

const singleMarketRestDaysFixture = `{"RS":"OK","Data":[
 {"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
   {"MarketType":"F","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05"}]}]}
 ]}
]}`

const sentinelAndTradeFixture = `{"RS":"OK","Data":[
 {"TransDate":"115.07.26","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0},
 {"TransDate":"115.07.26","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":46.2,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957}
]}`

const singleTradeFixture = `{"RS":"OK","Data":[
 {"TransDate":"115.07.26","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":46.2,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957}
]}`

func newSequencedPriceFixtureServer(t *testing.T, tradeBodies []string) *httptest.Server {
	t.Helper()
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "MarketRestDayFarmWCF") {
			_, _ = w.Write([]byte(singleMarketRestDaysFixture))
			return
		}
		idx := calls
		if idx >= len(tradeBodies) {
			idx = len(tradeBodies) - 1
		}
		calls++
		_, _ = w.Write([]byte(tradeBodies[idx]))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const emptyTradeFixture = `{"RS":"OK","Data":[]}`

const revisedTradeFixture = `{"RS":"OK","Data":[
 {"TransDate":"115.07.26","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":47.0,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957}
]}`

func newPriceFixtureServer(t *testing.T, tradeBody string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "MarketRestDayFarmWCF") {
			_, _ = w.Write([]byte(singleMarketRestDaysFixture))
			return
		}
		_, _ = w.Write([]byte(tradeBody))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPriceIngestFiltersClosedMarketSentinelRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, sentinelAndTradeFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(910000000000000000))

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	stats, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 2, stats.Fetched, "Fetched 計的是 Validate 過濾前的原始列數（含休市哨兵列）")
	require.Equal(t, 1, stats.Inserted)
	require.Equal(t, 0, stats.Revised)

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM price_observation`).Scan(&count))
	require.Equal(t, 1, count, "休市哨兵列不得寫入 price_observation")

	var cropCode string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT c.crop_code FROM price_observation po JOIN crop c ON c.id = po.crop_id`).
		Scan(&cropCode))
	require.Equal(t, "LA1", cropCode, "落地的應是真實成交列，不是哨兵列的 CropCode \"-\"")
}

func TestPriceIngestSkipsInsertWhenDigestUnchanged(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, singleTradeFixture)
	tsid := newSeqTSID(920000000000000000)
	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)

	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, tsid)

	stats1, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 1, stats1.Inserted)
	require.Equal(t, 0, stats1.Revised)

	stats2, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 0, stats2.Inserted, "同一筆資料重跑不應視為新 insert")
	require.Equal(t, 0, stats2.Revised, "digest 相同不是修訂，不該計入 Revised")

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM price_observation`).Scan(&count))
	require.Equal(t, 1, count, "重跑同一筆資料不應新增列")
}

func TestPriceIngestWritesNewRevisionOnPriceChange(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	tsid := newSeqTSID(930000000000000000)
	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)

	srv1 := newPriceFixtureServer(t, singleTradeFixture)
	ingest1 := usecase.NewPriceIngest(moa.NewClient(srv1.URL, srv1.URL), pool, tsid)
	stats1, err := ingest1.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 1, stats1.Inserted)
	require.Equal(t, 0, stats1.Revised)

	srv2 := newPriceFixtureServer(t, revisedTradeFixture)
	ingest2 := usecase.NewPriceIngest(moa.NewClient(srv2.URL, srv2.URL), pool, tsid)
	stats2, err := ingest2.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 0, stats2.Inserted)
	require.Equal(t, 1, stats2.Revised)

	rows, err := pool.Query(ctx,
		`SELECT revision_no, avg_price::text FROM price_observation ORDER BY revision_no`)
	require.NoError(t, err)
	defer rows.Close()

	type row struct {
		revisionNo int32
		avgPrice   string
	}
	var got []row
	for rows.Next() {
		var rn int32
		var ap string
		require.NoError(t, rows.Scan(&rn, &ap))
		got = append(got, row{revisionNo: rn, avgPrice: ap})
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 2, "應累積 2 筆 revision，第一筆絕不能被 UPDATE 掉")
	require.Equal(t, int32(1), got[0].revisionNo)
	require.Equal(t, "46.2000", got[0].avgPrice)
	require.Equal(t, int32(2), got[1].revisionNo)
	require.Equal(t, "47.0000", got[1].avgPrice)
}

const twoMarketRestDaysFixture = `{"RS":"OK","Data":[
 {"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
   {"MarketType":"F","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05"}]}]}
 ]},
 {"MarkerNo":"108","MarkerName":"台中","MarketTypeList":[
   {"MarketType":"F","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"06"}]}]}
 ]}
]}`

const tradeFixtureMarket104 = `{"RS":"OK","Data":[
 {"TransDate":"115.07.26","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":46.2,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957}
]}`

const tradeFixtureMarket108 = `{"RS":"OK","Data":[
 {"TransDate":"115.07.26","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"108","MarketName":"台中",
  "Avg_Price":38.5,"Upper_Price":45.0,"Middle_Price":39.0,"Lower_Price":30.0,"Trans_Quantity":5000}
]}`

func newTwoMarketFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "MarketRestDayFarmWCF") {
			_, _ = w.Write([]byte(twoMarketRestDaysFixture))
			return
		}
		switch r.URL.Query().Get("MarketCode") {
		case "104":
			_, _ = w.Write([]byte(tradeFixtureMarket104))
		case "108":
			_, _ = w.Write([]byte(tradeFixtureMarket108))
		default:
			_, _ = w.Write([]byte(`{"RS":"OK","Data":[]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPriceIngestIngestsMarketsKnownOnlyViaFetchRestDays(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	var marketCountBefore int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM market`).Scan(&marketCountBefore))
	require.Equal(t, 0, marketCountBefore, "測試前提：market 表應是空的，兩個市場都未預先 seed")

	srv := newTwoMarketFixtureServer(t)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(940000000000000000))

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	stats, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 2, stats.Fetched, "104 與 108 各 1 筆，Fetched 應是跨市場加總，不是只算到其中一個")
	require.Equal(t, 2, stats.Inserted)
	require.Equal(t, 0, stats.Revised)

	rows, err := pool.Query(ctx, `SELECT market_code FROM market ORDER BY market_code`)
	require.NoError(t, err)
	defer rows.Close()
	var codes []string
	for rows.Next() {
		var c string
		require.NoError(t, rows.Scan(&c))
		codes = append(codes, c)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"104", "108"}, codes, "兩個只透過 FetchRestDays 得知的市場都應正確落地")

	var obsCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM price_observation`).Scan(&obsCount))
	require.Equal(t, 2, obsCount, "兩個市場各一筆觀測，不應互相覆蓋或重複計入")
}

const malformedAndTradeFixtureMarket104 = `{"RS":"OK","Data":[
 {"TransDate":"not-a-date","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":46.2,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957},
 {"TransDate":"115.07.26","CropCode":"LA2","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":23.7,"Upper_Price":30.0,"Middle_Price":25.0,"Lower_Price":18.0,"Trans_Quantity":5000}
]}`

func newMalformedRowFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "MarketRestDayFarmWCF") {
			_, _ = w.Write([]byte(twoMarketRestDaysFixture))
			return
		}
		switch r.URL.Query().Get("MarketCode") {
		case "104":
			_, _ = w.Write([]byte(malformedAndTradeFixtureMarket104))
		case "108":
			_, _ = w.Write([]byte(tradeFixtureMarket108))
		default:
			_, _ = w.Write([]byte(`{"RS":"OK","Data":[]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPriceIngestSkipsMalformedTradeDateAndContinuesInsertingRest(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newMalformedRowFixtureServer(t)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(950000000000000000))

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	stats, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err, "單筆壞資料不應讓整個 Handle 失敗")
	require.Equal(t, 3, stats.Fetched, "市場 104 兩筆（含壞列）+ 市場 108 一筆")
	require.Equal(t, 1, stats.Skipped, "只有 104 的那筆壞 TransDate 算跳過")
	require.Equal(t, 2, stats.Inserted, "104 的正常列與 108 的正常列都應落地")
	require.Equal(t, 0, stats.Revised)

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM price_observation`).Scan(&count))
	require.Equal(t, 2, count, "壞列不得寫入，正常列（含市場 108）都應落地")

	rows, err := pool.Query(ctx,
		`SELECT c.crop_code FROM price_observation po JOIN crop c ON c.id = po.crop_id ORDER BY c.crop_code`)
	require.NoError(t, err)
	defer rows.Close()
	var cropCodes []string
	for rows.Next() {
		var cc string
		require.NoError(t, rows.Scan(&cc))
		cropCodes = append(cropCodes, cc)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"LA1", "LA2"}, cropCodes,
		"108 的正常成交（LA1）與 104 的正常成交（LA2）都應落地；104 的壞列（同樣 CropCode=LA1 但從未通過解析）不應造成混淆或額外落地")
}

const twoCategoryFixture = `{"RS":"OK","Data":[
{"TransDate":"115.07.28","TcType":"N04","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"109","MarketName":"台北一","Upper_Price":30.0,"Middle_Price":25.0,"Lower_Price":20.0,"Avg_Price":25.0,"Trans_Quantity":1000.0},
{"TransDate":"115.07.28","TcType":"N05","CropCode":"11","CropName":"椰子","MarketCode":"109","MarketName":"台北一","Upper_Price":26.7,"Middle_Price":21.4,"Lower_Price":18.0,"Avg_Price":21.8,"Trans_Quantity":1695.0}
]}`

func TestPriceIngestPersistsCategoryCodeFromTcType(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, twoCategoryFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(960000000000000000))

	day := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	stats, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 2, stats.Inserted)

	rows, err := pool.Query(ctx, `
		SELECT c.crop_code, c.category_code, g.group_code, g.category_code
		FROM crop c JOIN crop_group g ON g.id = c.group_id
		ORDER BY c.crop_code`)
	require.NoError(t, err)
	defer rows.Close()

	type got struct{ cropCode, cropCat, groupCode, groupCat string }
	var all []got
	for rows.Next() {
		var g got
		require.NoError(t, rows.Scan(&g.cropCode, &g.cropCat, &g.groupCode, &g.groupCat))
		all = append(all, g)
	}
	require.NoError(t, rows.Err())
	require.Len(t, all, 2)

	require.Equal(t, "11", all[0].cropCode)
	require.Equal(t, "N05", all[0].cropCat, "crop.category_code 必須來自 TcType")
	require.Equal(t, "N05", all[0].groupCat, "crop_group.category_code 必須來自 TcType")

	require.Equal(t, "LA1", all[1].cropCode)
	require.Equal(t, "N04", all[1].cropCat)
	require.Equal(t, "N04", all[1].groupCat)

	require.NotEqual(t, all[0].groupCode, all[1].groupCode,
		"不同 category 必須是不同的 crop_group")
	require.Equal(t, moa.GroupCode("椰子", "N05"), all[0].groupCode,
		"group_code 必須是 GroupCode(大類名, TcType) 的 hash，不是 GroupCode(大類名, \"\")")
	require.Equal(t, moa.GroupCode("甘藍", "N04"), all[1].groupCode)

	// upsert 同交易登記 surface form：大類 CANONICAL、子類 CANONICAL 與 VARIANT。
	// 這是索引唯一的寫入點，這裡沒登記到就代表新品項永遠搜不到。
	var forms []string
	frows, err := pool.Query(ctx, `
		SELECT entity_kind || '|' || form_kind || '|' || form_raw
		FROM priceable_surface_form ORDER BY 1`)
	require.NoError(t, err)
	defer frows.Close()
	for frows.Next() {
		var f string
		require.NoError(t, frows.Scan(&f))
		forms = append(forms, f)
	}
	require.NoError(t, frows.Err())
	require.Contains(t, forms, "CROP_GROUP|CANONICAL|甘藍")
	require.Contains(t, forms, "CROP|CANONICAL|甘藍-初秋")
	require.Contains(t, forms, "CROP|VARIANT|初秋")
}

func TestPriceIngestRetriesEmptyResponseWhenMarketHasPriorData(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv1 := newPriceFixtureServer(t, singleTradeFixture)
	seed := usecase.NewPriceIngest(moa.NewClient(srv1.URL, srv1.URL), pool, newSeqTSID(920000000000000000))
	seedDate := parseTestDate(t, "2026-07-26")
	if _, err := seed.Handle(ctx, seedDate, seedDate); err != nil {
		t.Fatalf("種子資料 Handle 失敗: %v", err)
	}

	retryDate := parseTestDate(t, "2026-07-27")
	tradeBodyForRetryDate := strings.ReplaceAll(singleTradeFixture, "115.07.26", "115.07.27")
	srv2 := newSequencedPriceFixtureServer(t, []string{emptyTradeFixture, tradeBodyForRetryDate})
	ingest := usecase.NewPriceIngest(moa.NewClient(srv2.URL, srv2.URL), pool, newSeqTSID(920000000001000000))
	ingest.SetRetryDelayForTest(time.Millisecond)

	stats, err := ingest.Handle(ctx, retryDate, retryDate)
	if err != nil {
		t.Fatalf("Handle 失敗: %v", err)
	}
	require.Equal(t, 1, stats.Inserted, "重試成功後應有 1 筆新 insert")
	require.Empty(t, stats.EmptyAfterRetry, "重試成功不該記進 EmptyAfterRetry")
}

func TestPriceIngestFlagsNeedsAttentionWhenMarketStaysEmptyAfterRetry(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv1 := newPriceFixtureServer(t, singleTradeFixture)
	seed := usecase.NewPriceIngest(moa.NewClient(srv1.URL, srv1.URL), pool, newSeqTSID(920000000002000000))
	seedDate := parseTestDate(t, "2026-07-26")
	if _, err := seed.Handle(ctx, seedDate, seedDate); err != nil {
		t.Fatalf("種子資料 Handle 失敗: %v", err)
	}

	retryDate := parseTestDate(t, "2026-07-27")
	srv2 := newSequencedPriceFixtureServer(t, []string{emptyTradeFixture})
	ingest := usecase.NewPriceIngest(moa.NewClient(srv2.URL, srv2.URL), pool, newSeqTSID(920000000003000000))
	ingest.SetRetryDelayForTest(time.Millisecond)

	stats, err := ingest.Handle(ctx, retryDate, retryDate)
	if err != nil {
		t.Fatalf("Handle 不應回傳錯誤（持續空手不是 infra 錯誤）: %v", err)
	}
	require.Equal(t, []usecase.EmptyLeafGap{
		{MarketCode: "104", Start: retryDate, End: retryDate},
	}, stats.EmptyAfterRetry)
	require.NotEmpty(t, stats.AttentionReason())
}

func TestPriceIngestSkipsRetryDuringHistoricalBackfillDespiteRecentObservation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv1 := newPriceFixtureServer(t, singleTradeFixture)
	seed := usecase.NewPriceIngest(moa.NewClient(srv1.URL, srv1.URL), pool, newSeqTSID(990000000000000000))
	seedDate := parseTestDate(t, "2026-07-26")
	if _, err := seed.Handle(ctx, seedDate, seedDate); err != nil {
		t.Fatalf("種子資料 Handle 失敗: %v", err)
	}

	var tradeCalls int32
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "MarketRestDayFarmWCF") {
			_, _ = w.Write([]byte(singleMarketRestDaysFixture))
			return
		}
		atomic.AddInt32(&tradeCalls, 1)
		_, _ = w.Write([]byte(emptyTradeFixture))
	}))
	t.Cleanup(srv2.Close)

	ingest := usecase.NewPriceIngest(moa.NewClient(srv2.URL, srv2.URL), pool, newSeqTSID(990000000001000000))
	ingest.SetRetryDelayForTest(time.Millisecond)

	historicalDay := parseTestDate(t, "2012-01-31")
	stats, err := ingest.Handle(ctx, historicalDay, historicalDay)
	if err != nil {
		t.Fatalf("Handle 失敗: %v", err)
	}
	require.Empty(t, stats.EmptyAfterRetry,
		"市場尚未存在於這個歷史區間（在 beforeDate 之前沒有任何觀測），不該被標記 EmptyAfterRetry")
	require.Equal(t, int32(1), atomic.LoadInt32(&tradeCalls),
		"MarketHasAnyObservation 應以 beforeDate 為界，市場在 2012-01-31（含）之前沒有觀測時 fast path 應生效、不重試")
}

func parseTestDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("解析測試日期失敗: %v", err)
	}
	return d
}

const duplicateKeyInBatchFixture = `{"RS":"OK","Data":[
 {"TransDate":"115.07.26","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":46.2,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957},
 {"TransDate":"115.07.26","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":47.0,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957}
]}`

func TestPriceIngestChainsRevisionForDuplicateKeyWithinSameBatch(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, duplicateKeyInBatchFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(960000000000000000))

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	stats, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 2, stats.Fetched)
	require.Equal(t, 1, stats.Inserted, "第一筆是新 key，應計入 Inserted")
	require.Equal(t, 1, stats.Revised, "第二筆同 key 不同 digest，應計入 Revised（不是又一次 Inserted）")

	rows, err := pool.Query(ctx,
		`SELECT revision_no, avg_price::text FROM price_observation ORDER BY revision_no`)
	require.NoError(t, err)
	defer rows.Close()

	type row struct {
		revisionNo int32
		avgPrice   string
	}
	var got []row
	for rows.Next() {
		var rn int32
		var ap string
		require.NoError(t, rows.Scan(&rn, &ap))
		got = append(got, row{revisionNo: rn, avgPrice: ap})
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 2)
	require.Equal(t, int32(1), got[0].revisionNo)
	require.Equal(t, "46.2000", got[0].avgPrice)
	require.Equal(t, int32(2), got[1].revisionNo, "第二筆必須是 revision_no=2，不能兩筆都算成 1")
	require.Equal(t, "47.0000", got[1].avgPrice)
}

func TestPriceIngestChainsRevisionAcrossChunkBoundary(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, duplicateKeyInBatchFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(962000000000000000))
	ingest.SetBatchSizeForTest(1)

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	stats, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 2, stats.Fetched)
	require.Equal(t, 1, stats.Inserted,
		"第一筆落在 chunk 1、DB 裡還沒有既有 revision，應計入 Inserted")
	require.Equal(t, 1, stats.Revised,
		"第二筆落在 chunk 2、必須從 DB 讀回 chunk 1 剛寫的 revision 才能算對，"+
			"不是又一次 Inserted——若計成 Inserted=2，代表跨 chunk 鏈接失效")

	rows, err := pool.Query(ctx,
		`SELECT revision_no, avg_price::text FROM price_observation ORDER BY revision_no`)
	require.NoError(t, err)
	defer rows.Close()

	type row struct {
		revisionNo int32
		avgPrice   string
	}
	var got []row
	for rows.Next() {
		var rn int32
		var ap string
		require.NoError(t, rows.Scan(&rn, &ap))
		got = append(got, row{revisionNo: rn, avgPrice: ap})
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 2, "跨 chunk 邊界不應漏寫任何一筆")
	require.Equal(t, int32(1), got[0].revisionNo)
	require.Equal(t, "46.2000", got[0].avgPrice, "chunk 1 寫入的第一筆")
	require.Equal(t, int32(2), got[1].revisionNo,
		"第二筆必須是 revision_no=2，證明 chunk 2 真的鏈到了 chunk 1 寫進 DB 的 revision")
	require.Equal(t, "47.0000", got[1].avgPrice, "chunk 2 寫入的第二筆")
}

const threeCropTradeFixture = `{"RS":"OK","Data":[
 {"TransDate":"115.07.26","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":46.2,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957},
 {"TransDate":"115.07.26","CropCode":"LB1","CropName":"甘藍-春夏","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":30.0,"Upper_Price":35.0,"Middle_Price":31.0,"Lower_Price":25.0,"Trans_Quantity":5000},
 {"TransDate":"115.07.26","CropCode":"LC1","CropName":"高麗菜","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":20.0,"Upper_Price":25.0,"Middle_Price":21.0,"Lower_Price":15.0,"Trans_Quantity":3000}
]}`

func TestPriceIngestHandlesBatchSizeBoundaryAcrossMultipleChunks(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, threeCropTradeFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(965000000000000000))
	ingest.SetBatchSizeForTest(2)

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	stats, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Equal(t, 3, stats.Fetched)
	require.Equal(t, 3, stats.Inserted, "3 筆都是新 key，跨 2 個 chunk（2+1）處理，加總應為 3")
	require.Equal(t, 0, stats.Revised)

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM price_observation`).Scan(&count))
	require.Equal(t, 3, count, "跨 chunk 邊界不應漏寫任何一筆")
}

const namelessAndTradeFixture = `{"RS":"OK","Data":[
 {"TransDate":"115.07.26","TcType":"","CropCode":"FE800","CropName":"","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":54.2,"Upper_Price":85,"Middle_Price":50,"Lower_Price":35,"Trans_Quantity":407},
 {"TransDate":"115.07.26","CropCode":"LA1","CropName":"甘藍-初秋","MarketCode":"104","MarketName":"台北二",
  "Avg_Price":46.2,"Upper_Price":54.6,"Middle_Price":47.5,"Lower_Price":34.1,"Trans_Quantity":10957}
]}`

func TestPriceIngestQuarantinesNamelessRowAndKeepsSiblings(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, namelessAndTradeFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(913000000000000000))

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	stats, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err)

	require.Equal(t, 2, stats.Fetched, "Fetched 計的是邊界過濾前的原始列數")
	require.Equal(t, 1, stats.Inserted, "只有帶完整身分的那列該落地")
	require.Equal(t, 1, stats.RejectedDefects[moa.RejectMissingCropName])

	var cropCodes []string
	rows, err := pool.Query(ctx, `SELECT crop_code FROM crop ORDER BY crop_code`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var code string
		require.NoError(t, rows.Scan(&code))
		cropCodes = append(cropCodes, code)
	}
	require.Equal(t, []string{"LA1"}, cropCodes, "沒有品名的列不得產生品項")

	var obsCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM price_observation`).Scan(&obsCount))
	require.Equal(t, 1, obsCount, "沒有品名的列不得產生行情")

	var reason, marketCode string
	var occurrences int64
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT reason, market_code, occurrence_count
		FROM ingest_rejected_identity WHERE crop_code = 'FE800'`).
		Scan(&reason, &marketCode, &occurrences))
	require.Equal(t, "missing_crop_name", reason)
	require.Equal(t, "104", marketCode)
	require.Equal(t, int64(1), occurrences)

	require.Len(t, stats.NewRejectedIdentities, 1, "第一次見到的身分要回報給告警判準")
	require.NotEmpty(t, stats.AttentionReason(), "新身分進隔離區時應要求人工確認")
}

func TestPriceIngestDoesNotRealertOnKnownRejectedIdentity(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, namelessAndTradeFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(914000000000000000))

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	_, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err)

	stats, err := ingest.Handle(ctx, day, day)
	require.NoError(t, err)

	require.Empty(t, stats.NewRejectedIdentities, "已知身分不得再回報為新")
	require.Empty(t, stats.AttentionReason(), "已知身分持續被擋不得告警")

	var occurrences int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT occurrence_count FROM ingest_rejected_identity WHERE crop_code = 'FE800'`).
		Scan(&occurrences))
	require.Equal(t, int64(2), occurrences, "被擋的列數要跨執行累加")
}
