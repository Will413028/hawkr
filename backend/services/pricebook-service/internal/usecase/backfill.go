package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

// historicalFloor 是 MOA 行情 API 資料的下限：2011 年以前查詢一律回空
// （task-4-brief.md 的 verified-facts 已用即時 API 驗證）。Handle 對早於此值
// 的 start 直接夾到此值，見 Handle 的說明。
var historicalFloor = time.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC)

// finishIngestRunTimeout 是 FinishIngestRun 這通「收尾寫入」自己的逾時預算。
//
// 🔴 比照 internal/app/cron/daily.go 的 finishIngestRunTimeout：刻意用
// context.WithoutCancel(ctx) 剝離掉呼叫端傳入的 ctx（可能就是導致這段/這次
// 失敗的原因本身），只留一個新的 10 秒短逾時。不這樣做的話，「這一段 backfill
// 失敗了」這個最需要被記下來的結果，會因為沿用同一個已經出狀況的 ctx 而讓
// 收尾寫入自己也連帶失敗，留下一列卡在 status=running、error_message 是
// NULL 的稽核紀錄——那正是本檔案上面 Handle 註解說「這比悄悄吞掉 finishErr
// 誠實」想避免的情況，卻在 ctx 本身有問題時整個失效（final whole-branch
// review Fix M3，與 daily.go 同一個修法解決同一類問題）。
const finishIngestRunTimeout = 10 * time.Second

// Chunk 一段 ingest 區間（[Start, End] 含頭尾）。
type Chunk struct {
	Start time.Time
	End   time.Time
}

// MonthlyChunks 把 [start, end]（含頭尾）切成以「日曆月」為界的小段。
//
// 實測：單一作物查 1 個月約 431 列、查 3 個月即觸及 1000 列上限，因此以月為
// backfill 的預設粗粒度；PriceIngest.Handle 內部仍有自己的第二層切分（先按
// 市場列舉、單一市場的月範圍仍會觸頂時再對半切日期，見 decisions.md D8）——
// 兩層切分處理的是不同問題，互不取代：這裡切的是「backfill 要跑多久、失敗要
// 重試多小的範圍」，PriceIngest 內部切的是「單次 HTTP 呼叫別觸頂」。
func MonthlyChunks(start, end time.Time) []Chunk {
	var chunks []Chunk
	cur := start
	for !cur.After(end) {
		monthEnd := time.Date(cur.Year(), cur.Month(), 1, 0, 0, 0, 0, time.UTC).
			AddDate(0, 1, -1)
		if monthEnd.After(end) {
			monthEnd = end
		}
		chunks = append(chunks, Chunk{Start: cur, End: monthEnd})
		cur = monthEnd.AddDate(0, 0, 1)
	}
	return chunks
}

// Backfill 以月為粒度重播歷史區間的行情 ingest，並在 ingest_run 留下一列稽核
// 紀錄。cmd/pricebook-backfill（手動觸發）是目前唯一的呼叫端；未來若有第二個
// 手動觸發路徑，一樣經由本方法，不得繞過稽核紀錄直接呼叫 PriceIngest.Handle。
type Backfill struct {
	priceIngest *PriceIngest
	pool        *pgxpool.Pool
	tsid        func() int64
}

// NewBackfill 建構 Backfill。
//
// 兩個結構決定與 PriceIngest / RestDayIngest 同理（見 price_ingest.go /
// restday_ingest.go 的建構子註解）：
//
//  1. tsid 逐字宣告成 func() int64——dig/fx 用型別做解析鍵，具名型別會讓
//     graph 解析不到 internal/usecase/di.go 裡 fx.Provide(tsid.NewGenerator)
//     提供的值；該 provider 在整個 fx 圖中只出現這一次，這裡不重複 provide。
//  2. 持有 *pgxpool.Pool 而非 *gen.Queries：*gen.Queries 不能被 fx 提供成
//     singleton，要在每次呼叫時用 gen.New(database.GetDBTX(ctx, pool)) 現組，
//     這樣 database.TxManager.WithTx 開的交易才會被自動撿起來。
func NewBackfill(p *PriceIngest, pool *pgxpool.Pool, tsid func() int64) *Backfill {
	return &Backfill{priceIngest: p, pool: pool, tsid: tsid}
}

// Handle 對 [start, end]（含頭尾）按月切分，逐段呼叫 PriceIngest.Handle，
// 在 ingest_run 留下一列稽核紀錄，成功時回傳該列的 id。
//
// 🔴 任一段失敗時，已成功段落的寫入不會回滾：本方法只把 ingest_run 標記為
// IngestStatusFailed（累計目前為止的統計、附上錯誤訊息）後回傳錯誤，不會、
// 也不能撤銷更早段落已寫入的 price_observation——這是刻意的設計，不是遺漏。
// price_observation 是 append-only + digest-dedupe：重新呼叫同一個
// [start, end] 時，已成功的段落因 digest 相同會被跳過（見 PriceIngest.Handle
// 的 revision 判定），不會產生重複列或重複 revision；「重跑同一個 range」本身
// 就是這個失敗模式下的復原手段，不需要、也不應該用一個包住所有月份的巨大
// transaction 把整個 backfill 綁在一起、失敗就整段回滾——那會讓一次跨兩年的
// backfill 因為最後一個月的暫時性錯誤而丟光前面幾百個已成功月份的工作。
func (u *Backfill) Handle(ctx context.Context, start, end time.Time) (int64, error) {
	if end.Before(start) {
		return 0, fmt.Errorf("backfill 區間反轉：start=%s 晚於 end=%s",
			start.Format("2006-01-02"), end.Format("2006-01-02"))
	}

	// 歷史下限：在 CreateIngestRun 之前夾好，讓稽核紀錄的 range_start 反映
	// 「實際嘗試的區間」，而不是呼叫端原始傳入、可能整段落在資料下限之前的值。
	if start.Before(historicalFloor) {
		start = historicalFloor
	}
	if end.Before(start) {
		// 夾完後區間為空（原始 end 本身就早於歷史下限）——沒有任何東西可以
		// 嘗試，視為呼叫方式錯誤，不建立 ingest_run（沒有東西可稽核）。
		return 0, fmt.Errorf("backfill 區間夾到歷史下限 %s 後為空：原始 end=%s 早於下限",
			historicalFloor.Format("2006-01-02"), end.Format("2006-01-02"))
	}

	q := gen.New(database.GetDBTX(ctx, u.pool))

	runID := u.tsid()
	if _, err := q.CreateIngestRun(ctx, gen.CreateIngestRunParams{
		ID:         runID,
		Kind:       domain.IngestKindBackfill,
		RangeStart: pgxutil.DateParam(&start),
		RangeEnd:   pgxutil.DateParam(&end),
		Status:     domain.IngestStatusRunning,
		StartedAt:  pgxutil.TimestamptzParam(time.Now().UTC()),
	}); err != nil {
		return 0, fmt.Errorf("建立 ingest_run 失敗: %w", err)
	}

	var total IngestStats
	for _, chunk := range MonthlyChunks(start, end) {
		stats, err := u.priceIngest.Handle(ctx, chunk.Start, chunk.End)
		total.Fetched += stats.Fetched
		total.Inserted += stats.Inserted
		total.Revised += stats.Revised
		total.Skipped += stats.Skipped
		total.EmptyAfterRetry = append(total.EmptyAfterRetry, stats.EmptyAfterRetry...)

		if err == nil {
			// 一段（月）約 8 分鐘（2026-07-28 本計畫實跑觀測），全歷史 backfill
			// 約 170 段、逼近 22 小時——run 完全靜默曾讓 implementer 本人
			// 誤判 36 分鐘的正常進度為疑似卡死（final whole-branch review Fix 3）。
			// 用套件層級 zap.L()（而非替 Backfill 加 logger 建構參數）：
			// review-service/notification-service 的 usecase 層已是這個慣例
			// （直接呼叫 zap.L()/zap.S()，不逐一注入），沿用它可以讓這裡的修正
			// 不必牽動 NewBackfill 簽章與其 10+ 處既有呼叫端（fx 圖與各整合測試）。
			// cmd/pricebook-backfill 啟動時會呼叫 zap.ReplaceGlobals 讓輸出實際
			// 印得出來（比照 internal/config/di.go 對正式服務的做法）；主服務
			// 的 cron 路徑本來就已經呼叫過 ReplaceGlobals，這裡不需要重複處理。
			zap.L().Info("backfill: 段落完成",
				zap.String("chunk_start", chunk.Start.Format("2006-01-02")),
				zap.String("chunk_end", chunk.End.Format("2006-01-02")),
				zap.Int("fetched_total", total.Fetched),
				zap.Int("inserted_total", total.Inserted),
				zap.Int("revised_total", total.Revised),
				zap.Int("skipped_total", total.Skipped))
		}

		if err != nil {
			chunkErr := fmt.Errorf("backfill 段落失敗（%s ~ %s）: %w",
				chunk.Start.Format("2006-01-02"), chunk.End.Format("2006-01-02"), err)
			finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), finishIngestRunTimeout)
			finishErr := q.FinishIngestRun(finishCtx, gen.FinishIngestRunParams{
				ID:            runID,
				Status:        domain.IngestStatusFailed,
				FetchedCount:  int32(total.Fetched),
				InsertedCount: int32(total.Inserted),
				RevisedCount:  int32(total.Revised),
				ErrorMessage:  stringPointer(chunkErr.Error()),
				FinishedAt:    timePointer(time.Now().UTC()),
			})
			finishCancel()
			if finishErr != nil {
				// ingest_run 停在 status=running：下次人工排查會看到一列卡住的
				// running 紀錄，而不是誤判成功——這比悄悄吞掉 finishErr 誠實。
				return runID, fmt.Errorf("%w（另外，標記 ingest_run 失敗也失敗: %w）", chunkErr, finishErr)
			}
			return runID, chunkErr
		}
	}

	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), finishIngestRunTimeout)
	defer finishCancel()
	finish := gen.FinishIngestRunParams{
		ID:            runID,
		Status:        domain.IngestStatusSucceeded,
		FetchedCount:  int32(total.Fetched),
		InsertedCount: int32(total.Inserted),
		RevisedCount:  int32(total.Revised),
		FinishedAt:    timePointer(time.Now().UTC()),
	}
	// 跨所有段落抓到了列、卻一列都沒落地：同 cron 路徑的取捨（見
	// IngestStats.AttentionReason），用獨立狀態把它跟「這段期間本來就沒行情」分開。
	if attention := total.AttentionReason(); attention != "" {
		finish.Status = domain.IngestStatusNeedsAttention
		finish.ErrorMessage = stringPointer(attention)
	}
	if err := q.FinishIngestRun(finishCtx, finish); err != nil {
		return runID, fmt.Errorf("標記 ingest_run 完成失敗: %w", err)
	}

	return runID, nil
}
