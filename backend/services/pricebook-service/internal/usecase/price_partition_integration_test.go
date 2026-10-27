//go:build integration

// 攝取分區的行為測試。需要本機 Docker daemon，用 `make test-integration`
// （或 `go test -tags integration ./...`）跑。
//
// 重用同檔案群組（package usecase_test）既有的 setupTestDB / newSeqTSID /
// newPriceFixtureServer 等 harness，不新開第二套——比照
// price_ingest_integration_test.go 檔頭的理由。
//
// 這裡驗的是分區模型的核心不變量，它們沒有一個能只靠單元測試表達，因為判定
// 本身跨越「上游回了什麼」與「DB 現在是什麼狀態」兩邊：
//
//	宣告 —— 休市日直接落終態、其他落 pending、重跑不把 complete 打回 pending
//	消費 —— 有交易列→complete、只有哨兵列→rest_day、上游空手→attempt+1 留 pending
//	放棄 —— attempt 用盡才轉 abandoned，而且是「用盡」不是「用到一半」
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
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// partitionRow 是測試對 ingest_partition 一列的視角。
type partitionRow struct {
	status         int16
	attemptCount   int32
	completedAt    *time.Time
	acceptedAt     *time.Time
	acceptedBy     *string
	acceptedReason *string
}

func readPartition(ctx context.Context, t *testing.T, pool *pgxpool.Pool, marketCode string, day time.Time) partitionRow {
	t.Helper()
	var got partitionRow
	err := pool.QueryRow(ctx, `
		SELECT p.status, p.attempt_count, p.completed_at,
		       p.accepted_at, p.accepted_by, p.accepted_reason
		FROM ingest_partition p JOIN market m ON m.id = p.market_id
		WHERE m.market_code = $1 AND p.trade_date = $2`,
		marketCode, day).Scan(&got.status, &got.attemptCount, &got.completedAt,
		&got.acceptedAt, &got.acceptedBy, &got.acceptedReason)
	require.NoError(t, err)
	return got
}

// starveToAbandoned 把 (104, day) 那一格餵到 abandoned：上游一直空手、attempt
// 用盡。三個 accept-gap 測試的共同前置，抽出來免得各寫一次。
func starveToAbandoned(ctx context.Context, t *testing.T, pool *pgxpool.Pool, day time.Time, seed int64) {
	t.Helper()
	srv := newPriceFixtureServer(t, emptyTradeFixture)
	client := moa.NewClient(srv.URL, srv.URL)
	restDay := usecase.NewRestDayIngest(client, pool, newSeqTSID(seed), zap.NewNop())
	_, err := restDay.Handle(ctx)
	require.NoError(t, err)

	starve := usecase.NewPriceIngest(client, pool, newSeqTSID(seed+1000000000000))
	starve.SetRetryDelayForTest(time.Millisecond)
	require.NoError(t, starve.DeclareDue(ctx, day, day))
	for i := 0; i < domain.MaxIngestAttempts; i++ {
		_, err = starve.HandleDue(ctx)
		require.NoError(t, err)
	}
	require.Equal(t, domain.PartitionStatusAbandoned,
		readPartition(ctx, t, pool, "104", day).status, "前置條件：這一格已被放棄")
}

// TestDeclareDue_MarksRestDaysTerminalAndOthersPending 釘住宣告階段的兩個去向。
// 休市日在宣告當下就落終態，不進工作清單——否則每個休市日都會白白燒掉一次
// attempt，而 attempt 是有限的（MaxIngestAttempts），等於用假失敗稀釋真訊號。
func TestDeclareDue_MarksRestDaysTerminalAndOthersPending(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	// 休市日曆 fixture 宣告市場 104 在 115.01.05（= 2026-01-05）休市。
	srv := newPriceFixtureServer(t, emptyTradeFixture)
	client := moa.NewClient(srv.URL, srv.URL)
	restDay := usecase.NewRestDayIngest(client, pool, newSeqTSID(940000000000000000), zap.NewNop())
	_, err := restDay.Handle(ctx)
	require.NoError(t, err)

	ingest := usecase.NewPriceIngest(client, pool, newSeqTSID(941000000000000000))

	from := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)
	through := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
	require.NoError(t, ingest.DeclareDue(ctx, from, through))

	restDate := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	require.Equal(t, domain.PartitionStatusRestDay,
		readPartition(ctx, t, pool, "104", restDate).status,
		"休市日應在宣告當下就落終態，不該進工作清單消耗 attempt")

	for _, d := range []time.Time{from, through} {
		require.Equal(t, domain.PartitionStatusPending,
			readPartition(ctx, t, pool, "104", d).status,
			"非休市日應宣告成 pending")
	}
}

// TestDeclareDue_IsIdempotentAndNeverResetsCompleted 是重跑安全的護欄：宣告
// 每天都會跑，而且宣告區間刻意涵蓋過去幾天（容忍漏跑），所以它一定會重複碰到
// 已經完成的格子。把 complete 打回 pending 會讓那些格子被無限重抓。
func TestDeclareDue_IsIdempotentAndNeverResetsCompleted(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, singleTradeFixture)
	client := moa.NewClient(srv.URL, srv.URL)
	restDay := usecase.NewRestDayIngest(client, pool, newSeqTSID(942000000000000000), zap.NewNop())
	_, err := restDay.Handle(ctx)
	require.NoError(t, err)

	ingest := usecase.NewPriceIngest(client, pool, newSeqTSID(943000000000000000))
	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)

	require.NoError(t, ingest.DeclareDue(ctx, day, day))
	_, err = ingest.HandleDue(ctx)
	require.NoError(t, err)
	require.Equal(t, domain.PartitionStatusComplete,
		readPartition(ctx, t, pool, "104", day).status)

	// 第二次宣告同一天：不得把它打回 pending。
	require.NoError(t, ingest.DeclareDue(ctx, day, day))
	after := readPartition(ctx, t, pool, "104", day)
	require.Equal(t, domain.PartitionStatusComplete, after.status,
		"重複宣告不得把已完成的格子打回 pending")
	require.NotNil(t, after.completedAt, "completed_at 應保留")
}

// TestHandleDue_CompletesOnTradeRowsAndRestDayOnSentinelOnly 驗證消費階段用
// 上游回應本身判定去向：哨兵列是上游對「那天休市」的第一手陳述，比我方 rest
// 表更即時（rest 表靠每日同步，可能還沒拿到最新公告）。
func TestHandleDue_CompletesOnTradeRowsAndRestDayOnSentinelOnly(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	// sentinelOnly：只有哨兵列、沒有任何成交列。
	const sentinelOnlyFixture = `{"RS":"OK","Data":[
	 {"TransDate":"115.07.26","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
	  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0}
	]}`

	srv := newPriceFixtureServer(t, sentinelOnlyFixture)
	client := moa.NewClient(srv.URL, srv.URL)
	restDay := usecase.NewRestDayIngest(client, pool, newSeqTSID(944000000000000000), zap.NewNop())
	_, err := restDay.Handle(ctx)
	require.NoError(t, err)

	ingest := usecase.NewPriceIngest(client, pool, newSeqTSID(945000000000000000))
	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	require.NoError(t, ingest.DeclareDue(ctx, day, day))

	_, err = ingest.HandleDue(ctx)
	require.NoError(t, err)

	got := readPartition(ctx, t, pool, "104", day)
	require.Equal(t, domain.PartitionStatusRestDay, got.status,
		"上游只回哨兵列＝它明說那天休市，應落 rest_day 而不是留在 pending 重試")
}

// TestHandleDue_EmptyUpstreamKeepsPendingAndCountsAttempt 是整個重構的核心
// 不變量：上游這次沒給，那一格**留著**。舊設計在這裡什麼都不記，下一次執行
// 只是盲目重抓一個固定窗口，窗口滑過去那一格就永遠消失。
func TestHandleDue_EmptyUpstreamKeepsPendingAndCountsAttempt(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, emptyTradeFixture)
	client := moa.NewClient(srv.URL, srv.URL)
	restDay := usecase.NewRestDayIngest(client, pool, newSeqTSID(946000000000000000), zap.NewNop())
	_, err := restDay.Handle(ctx)
	require.NoError(t, err)

	ingest := usecase.NewPriceIngest(client, pool, newSeqTSID(947000000000000000))
	ingest.SetRetryDelayForTest(time.Millisecond)

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	require.NoError(t, ingest.DeclareDue(ctx, day, day))

	stats, err := ingest.HandleDue(ctx)
	require.NoError(t, err)

	got := readPartition(ctx, t, pool, "104", day)
	require.Equal(t, domain.PartitionStatusPending, got.status,
		"上游空手不是終態——這一格要留著等下次，這正是舊窗口做不到的事")
	require.Equal(t, int32(1), got.attemptCount, "attempt 應該進位，退避與放棄判定都讀它")

	// 2026-09-14：分區層記了 attempt 與 last_error，但這個訊號原本沒進 IngestStats，
	// 所以 ingest_run 把「問了、全空」記成 succeeded（2026-09-13 20:00Z 的實際事故）。
	// 這兩條斷言守的就是那條接線。
	require.Positive(t, stats.PartitionsAttempted,
		"本輪確實向上游求取過分區，PartitionsAttempted 必須反映出來——否則 AttentionReason 分不出「沒事做」與「全空」")
	require.Zero(t, stats.Fetched, "fixture 是空上游，不該拿到任何列")
	require.NotEmpty(t, stats.AttentionReason(),
		"問了分區卻一列都沒拿到，ingest_run 必須標成 needs_attention，不能記成一次乾淨的成功")

	// 再跑一輪：同一格仍在工作清單裡，attempt 繼續累加。
	_, err = ingest.HandleDue(ctx)
	require.NoError(t, err)
	require.Equal(t, int32(2), readPartition(ctx, t, pool, "104", day).attemptCount,
		"跨執行的記憶是重點：舊設計每次都從零開始，永遠不知道自己試過幾次")
}

// TestHandleDue_AbandonsOnlyAfterAttemptsExhausted 釘住放棄的門檻。
// 提早放棄會把「上游還沒上架」誤判成永久缺口（實測延遲尾巴有 11 天）；
// 永不放棄則讓一格真的不存在的資料無限重抓、也永遠不會有人被叫來看。
func TestHandleDue_AbandonsOnlyAfterAttemptsExhausted(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, emptyTradeFixture)
	client := moa.NewClient(srv.URL, srv.URL)
	restDay := usecase.NewRestDayIngest(client, pool, newSeqTSID(948000000000000000), zap.NewNop())
	_, err := restDay.Handle(ctx)
	require.NoError(t, err)

	ingest := usecase.NewPriceIngest(client, pool, newSeqTSID(949000000000000000))
	ingest.SetRetryDelayForTest(time.Millisecond)

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	require.NoError(t, ingest.DeclareDue(ctx, day, day))

	for i := 1; i < domain.MaxIngestAttempts; i++ {
		_, err = ingest.HandleDue(ctx)
		require.NoError(t, err)
		require.Equal(t, domain.PartitionStatusPending,
			readPartition(ctx, t, pool, "104", day).status,
			"第 %d 次嘗試（上限 %d）還不該放棄", i, domain.MaxIngestAttempts)
	}

	_, err = ingest.HandleDue(ctx)
	require.NoError(t, err)
	got := readPartition(ctx, t, pool, "104", day)
	require.Equal(t, domain.PartitionStatusAbandoned, got.status,
		"用盡 %d 次後才轉 abandoned", domain.MaxIngestAttempts)
	require.Equal(t, int32(domain.MaxIngestAttempts), got.attemptCount)

	// abandoned 是終態：不再進工作清單，attempt 也不再累加。
	_, err = ingest.HandleDue(ctx)
	require.NoError(t, err)
	require.Equal(t, int32(domain.MaxIngestAttempts),
		readPartition(ctx, t, pool, "104", day).attemptCount,
		"abandoned 之後不該再被撿起來重試——它等的是人，不是下一輪排程")
}

// TestHandle_BackfillPathMarksPartitionComplete 補的是一個很難自己發現的不一致：
// cmd/pricebook-backfill 走的是 Handle(start, end)（區間由人指定），不是
// HandleDue 的工作清單路徑。若它補回一格已經 abandoned 的資料而不更新分區，
// 那一格會永遠停在 abandoned——abandoned 是終態，HandleDue 不會再撿它——於是
// 監控上就有一個永遠亮著、且怎麼補資料都不會消失的假警報。
func TestHandle_BackfillPathMarksPartitionComplete(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)

	// 先讓那一格走到 abandoned：上游一直空手，attempt 用盡。
	emptySrv := newPriceFixtureServer(t, emptyTradeFixture)
	emptyClient := moa.NewClient(emptySrv.URL, emptySrv.URL)
	restDay := usecase.NewRestDayIngest(emptyClient, pool, newSeqTSID(950000000000000000), zap.NewNop())
	_, err := restDay.Handle(ctx)
	require.NoError(t, err)

	starve := usecase.NewPriceIngest(emptyClient, pool, newSeqTSID(951000000000000000))
	starve.SetRetryDelayForTest(time.Millisecond)
	require.NoError(t, starve.DeclareDue(ctx, day, day))
	for i := 0; i < domain.MaxIngestAttempts; i++ {
		_, err = starve.HandleDue(ctx)
		require.NoError(t, err)
	}
	require.Equal(t, domain.PartitionStatusAbandoned,
		readPartition(ctx, t, pool, "104", day).status, "前置條件：這一格已被放棄")

	// 人工 backfill：上游現在有資料了。
	liveSrv := newPriceFixtureServer(t, singleTradeFixture)
	backfill := usecase.NewPriceIngest(moa.NewClient(liveSrv.URL, liveSrv.URL), pool, newSeqTSID(952000000000000000))
	stats, err := backfill.Handle(ctx, day, day)
	require.NoError(t, err)
	require.Positive(t, stats.Inserted, "backfill 應該真的寫進資料")

	got := readPartition(ctx, t, pool, "104", day)
	require.Equal(t, domain.PartitionStatusComplete, got.status,
		"backfill 補回資料後，分區必須跟著回到 complete，否則假警報永遠消不掉")
	require.NotNil(t, got.completedAt)
}

// TestHandle_PersistsPartitionsEvenWhenAMarketFails 釘住錯誤路徑。
//
// Handle 對任何一個市場的抓取失敗都是 fail-fast（直接 return），但在那之前
// 已經有市場的資料寫進去了。第一版把分區的 persist 放在市場迴圈**之後**，
// 於是那些已經寫入的格子完全不會被標記——「資料寫了、狀態沒更新」，正是這個
// 重構要消滅的不一致，只是改由錯誤路徑製造。
//
// 2026-09-02 的 live e2e 撞到真實案例：單日 backfill 在 market=420 逾時中止，
// ingest_run 記 fetched/inserted 各 1725，而那天的 21 格分區原地不動。單元
// 測試當時抓不到，因為它們的 fixture server 從不失敗。
func TestHandle_PersistsPartitionsEvenWhenAMarketFails(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)

	// 休市日曆宣告兩個市場：104 有資料、999 直接 500。
	const twoMarketRestDays = `{"RS":"OK","Data":[
	 {"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
	   {"MarketType":"F","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05"}]}]}]},
	 {"MarkerNo":"999","MarkerName":"故障市場","MarketTypeList":[
	   {"MarketType":"F","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05"}]}]}]}
	]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "MarketRestDayFarmWCF") {
			_, _ = w.Write([]byte(twoMarketRestDays))
			return
		}
		if strings.Contains(r.URL.RawQuery, "MarketCode=999") {
			w.WriteHeader(http.StatusInternalServerError) // 模擬那個逾時／故障的市場
			return
		}
		_, _ = w.Write([]byte(singleTradeFixture))
	}))
	defer srv.Close()

	client := moa.NewClient(srv.URL, srv.URL)
	restDay := usecase.NewRestDayIngest(client, pool, newSeqTSID(953000000000000000), zap.NewNop())
	_, err := restDay.Handle(ctx)
	require.NoError(t, err)

	ingest := usecase.NewPriceIngest(client, pool, newSeqTSID(954000000000000000))
	require.NoError(t, ingest.DeclareDue(ctx, day, day))

	_, err = ingest.Handle(ctx, day, day)
	require.Error(t, err, "有市場失敗時 Handle 應回錯（fail-fast 行為不變）")

	require.Equal(t, domain.PartitionStatusComplete,
		readPartition(ctx, t, pool, "104", day).status,
		"已經成功寫入的市場，分區必須被標記——不能因為後面某個市場失敗就整批不記")
}

// ── 接受缺口（accepted_gap，status = 5）─────────────────────────────────────
//
// 這一組守的是一個很容易被做壞的東西：一個「讓告警閉嘴」的按鈕。
//
// abandoned 混了「我方重試耗盡」與「上游本來就沒有」兩種語意，後者永遠補不到、
// 也就永遠不會解除，最後只會被整條靜音，連帶把真缺口一起弄瞎。拆出
// accepted_gap 是為了讓後者有終局；而拆出來的當下就必須釘住它**不能**變成
// 一個橡皮圖章——所以每一條測試問的都是「什麼情況下它應該拒絕接受」。

// TestAcceptGap_AcceptsOnlyWhenUpstreamStillEmpty 是正常路徑：重查後上游確實
// 什麼都沒有，才寫成終態，而且三個稽核欄位必須同時有值。
func TestAcceptGap_AcceptsOnlyWhenUpstreamStillEmpty(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	starveToAbandoned(ctx, t, pool, day, 960000000000000000)

	srv := newPriceFixtureServer(t, emptyTradeFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(962000000000000000))
	ingest.SetRetryDelayForTest(time.Millisecond)

	outcome, err := ingest.AcceptGap(ctx, "104", day, "will", "MOA 對該日回 Data: []，鄰日有資料")
	require.NoError(t, err)
	require.Equal(t, usecase.AcceptGapAccepted, outcome)

	got := readPartition(ctx, t, pool, "104", day)
	require.Equal(t, domain.PartitionStatusAcceptedGap, got.status)
	require.NotNil(t, got.acceptedAt, "沒有時間的『已接受』等於沒有稽核")
	require.NotNil(t, got.acceptedBy)
	require.Equal(t, "will", *got.acceptedBy)
	require.NotNil(t, got.acceptedReason)
	require.Contains(t, *got.acceptedReason, "Data: []")
}

// TestAcceptGap_RefusesWhenUpstreamNowHasData 是這組裡最重要的一條：內建的
// positive control。接受動作在寫入之前重新問一次上游，拿得到資料就改走正常
// 寫入並標 complete、**拒絕**接受。
//
// 沒有這一條，accepted_gap 就退化成「把告警關掉」——而關掉一個其實補得回來的
// 缺口，跟 2026-07-24 那 21 個市場一樣，是永久失去資料而且沒人會發現。
func TestAcceptGap_RefusesWhenUpstreamNowHasData(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	starveToAbandoned(ctx, t, pool, day, 963000000000000000)

	// 上游現在有資料了（人在下 accept 指令時並不知道）。
	liveSrv := newPriceFixtureServer(t, singleTradeFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(liveSrv.URL, liveSrv.URL), pool, newSeqTSID(965000000000000000))
	ingest.SetRetryDelayForTest(time.Millisecond)

	outcome, err := ingest.AcceptGap(ctx, "104", day, "will", "以為上游沒有")
	require.NoError(t, err)
	require.Equal(t, usecase.AcceptGapUpstreamHasData, outcome,
		"上游有資料時必須拒絕接受，否則這個狀態就只是靜音鍵")

	got := readPartition(ctx, t, pool, "104", day)
	require.Equal(t, domain.PartitionStatusComplete, got.status,
		"拒絕接受不等於什麼都不做——查到的資料要寫進去、分區要標完成")
	require.NotNil(t, got.completedAt)
	require.Nil(t, got.acceptedAt, "沒有被接受，稽核欄位就不該有值")

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM price_observation WHERE trade_date = $1`, day).Scan(&count))
	require.Positive(t, count, "重查到的行情必須真的落地，不能只改狀態")
}

// TestAcceptGap_ResolvesAsRestDayOnSentinelOnly：上游回的是休市哨兵時，正確的
// 終態是 rest_day 而不是 accepted_gap。兩者都不告警，但語意不同——把休市記成
// 「資料缺漏但我們接受了」會讓之後任何一份完整度分析都算錯分母。
func TestAcceptGap_ResolvesAsRestDayOnSentinelOnly(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	starveToAbandoned(ctx, t, pool, day, 966000000000000000)

	const sentinelOnlyFixture = `{"RS":"OK","Data":[
	 {"TransDate":"115.07.26","CropCode":"-","CropName":"休市","MarketCode":"104","MarketName":"台北二",
	  "Avg_Price":0,"Upper_Price":0,"Middle_Price":0,"Lower_Price":0,"Trans_Quantity":0}
	]}`
	srv := newPriceFixtureServer(t, sentinelOnlyFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(968000000000000000))
	ingest.SetRetryDelayForTest(time.Millisecond)

	outcome, err := ingest.AcceptGap(ctx, "104", day, "will", "以為上游沒有")
	require.NoError(t, err)
	require.Equal(t, usecase.AcceptGapRestDay, outcome)

	got := readPartition(ctx, t, pool, "104", day)
	require.Equal(t, domain.PartitionStatusRestDay, got.status)
	require.Nil(t, got.acceptedAt)
}

// TestAcceptGap_RejectsNonAbandonedPartition：只有 abandoned 能被接受。
// pending 代表系統自己的重試還沒用完，這時候接受等於繞過系統的判定——那正是
// 「人可以隨手關掉告警」的形狀。
func TestAcceptGap_RejectsNonAbandonedPartition(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, emptyTradeFixture)
	client := moa.NewClient(srv.URL, srv.URL)
	restDay := usecase.NewRestDayIngest(client, pool, newSeqTSID(969000000000000000), zap.NewNop())
	_, err := restDay.Handle(ctx)
	require.NoError(t, err)

	ingest := usecase.NewPriceIngest(client, pool, newSeqTSID(970000000000000000))
	ingest.SetRetryDelayForTest(time.Millisecond)

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	require.NoError(t, ingest.DeclareDue(ctx, day, day))
	_, err = ingest.HandleDue(ctx)
	require.NoError(t, err)
	require.Equal(t, domain.PartitionStatusPending,
		readPartition(ctx, t, pool, "104", day).status, "前置條件：還在重試中")

	_, err = ingest.AcceptGap(ctx, "104", day, "will", "還沒試完就想接受")
	require.ErrorIs(t, err, usecase.ErrPartitionNotAbandoned)
	require.Equal(t, domain.PartitionStatusPending,
		readPartition(ctx, t, pool, "104", day).status, "被拒絕的接受不得留下任何痕跡")
}

// TestAcceptGap_RequiresEvidence：沒有經手人與理由的「已接受」跟直接刪掉那筆
// 告警沒有差別。這一條擋空字串，DB 的 CHECK 擋 NULL，兩層都要有。
func TestAcceptGap_RequiresEvidence(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	srv := newPriceFixtureServer(t, emptyTradeFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(971000000000000000))
	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)

	_, err := ingest.AcceptGap(ctx, "104", day, "   ", "有理由但沒有人")
	require.Error(t, err)
	require.Contains(t, err.Error(), "acceptedBy")

	_, err = ingest.AcceptGap(ctx, "104", day, "will", "  ")
	require.Error(t, err)
	require.Contains(t, err.Error(), "reason")
}

// TestAcceptGap_EvidenceIsEnforcedByTheDatabase 直接繞過 usecase 寫 DB，確認
// CHECK 真的在。應用層的驗證會被下一個寫入路徑繞過，DB 的不會——而這個狀態
// 的全部價值就在於它一定帶著證據。
func TestAcceptGap_EvidenceIsEnforcedByTheDatabase(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	starveToAbandoned(ctx, t, pool, day, 972000000000000000)

	_, err := pool.Exec(ctx, `
		UPDATE ingest_partition SET status = $1
		WHERE trade_date = $2`, domain.PartitionStatusAcceptedGap, day)
	require.Error(t, err, "沒有 accepted_* 三欄就把 status 改成 5 必須被 DB 擋下")
	require.Contains(t, err.Error(), "ingest_partition_accepted_evidence")
}

// TestAcceptGap_AcceptedIsTerminalAndOutOfTheWorkList：接受之後排程不得再碰它。
// 若它還會被撿回工作清單，attempt 會繼續累加、狀態會被覆寫，接受就白做了。
func TestAcceptGap_AcceptedIsTerminalAndOutOfTheWorkList(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	day := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	starveToAbandoned(ctx, t, pool, day, 973000000000000000)

	srv := newPriceFixtureServer(t, emptyTradeFixture)
	ingest := usecase.NewPriceIngest(moa.NewClient(srv.URL, srv.URL), pool, newSeqTSID(975000000000000000))
	ingest.SetRetryDelayForTest(time.Millisecond)
	_, err := ingest.AcceptGap(ctx, "104", day, "will", "上游確認無資料")
	require.NoError(t, err)

	before := readPartition(ctx, t, pool, "104", day)

	// 再宣告一次同一天（宣告區間本來就會重複涵蓋過去幾天），再跑一輪排程。
	require.NoError(t, ingest.DeclareDue(ctx, day, day))
	_, err = ingest.HandleDue(ctx)
	require.NoError(t, err)

	after := readPartition(ctx, t, pool, "104", day)
	require.Equal(t, domain.PartitionStatusAcceptedGap, after.status,
		"宣告不得把已接受的格子打回 pending")
	require.Equal(t, before.attemptCount, after.attemptCount,
		"已接受的格子不該再消耗 attempt")
	require.NotNil(t, after.acceptedAt)
}
