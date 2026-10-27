// forecast_job.go 是每日批次預測排程：對所有 active crop_group 跑一次
// ForecastBatch.Handle，寫進 crop_group_forecast_result。跟 daily.go 的
// dailyJob 不同，這支 job 不記 ingest_run 稽核——結果表本身就是 audit
// trail（能查到「哪天跑了、跑出什麼」），不比照 ingest_run 硬套語意不合的
// fetched/inserted/revised 欄位（design spec「範圍外」段的既有決定）。
package cron

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// forecastHourUTC/forecastMinuteUTC：既有 ingest cron 排在 20:00 UTC，
// dailyRunTimeout=30 分鐘，這裡排 21:00 UTC 留至少 30 分鐘緩衝——確保
// 批次預測讀到的是當天已經 ingest 完成的最新行情，不是跑到一半的資料。
const (
	forecastHourUTC   = 21
	forecastMinuteUTC = 0
)

const forecastJobName = "pricebook_daily_forecast"

// forecastRunTimeout 是每次批次預測執行的上限。
//
// 🔴 這是第一版的保守估計，不是實測數字：GroupForecast 對單一 group 的成本
// 全部是本地 DB 查詢（CropsInGroup/SeriesObservations/GroupObservations/
// RestDays/RestDayCoverage）+ 記憶體內計算（SevenDays/AggregateGroup），
// 不像 ingest cron 要打 MOA 外部 API，沒有 60 秒級的單次逾時風險。10 分鐘
// 是「正常執行（預期數十秒到數分鐘量級）留足緩衝」與「DB 本身有狀況時不
// 無限期卡住排程迴圈」之間的粗估值，比照 daily.go 的 dailyRunTimeout 註解
// 風格——上線後應該用 dev 環境的真實 group 數量與耗時重新校準這個數字，
// 不要一直沿用這個第一版估計。
const forecastRunTimeout = 10 * time.Minute

// forecastBatcher 是 forecastJob 對批次預測需要的最小介面（比照 daily.go
// 的 restDayIngester/priceIngester 慣例：cron 套件宣告自己需要的單一方法
// 介面，方便測試注入 stub、不用起整個 usecase fx 圖）。*usecase.ForecastBatch
// 滿足此介面。
type forecastBatcher interface {
	Handle(ctx context.Context, origin time.Time) (usecase.ForecastBatchStats, error)
}

// forecastResultChecker 是 forecastJob 對 Finding I2 的啟動補跑檢查需要的
// 最小介面：「今天（UTC 曆日）是否已經有任何 crop_group_forecast_result 落
// 地」。*gen.Queries（HasForecastResultToday，db/queries/crop_group_forecast.sql）
// 滿足此介面。抽出介面純粹是為了讓 catchUpIfNeeded 可以用 fake 做單元測試，
// 不需要真的 Postgres——同 daily.go 的 ingestRunRecorder 慣例。
type forecastResultChecker interface {
	HasForecastResultToday(ctx context.Context) (bool, error)
}

// forecastJob 是每日批次預測排程的實際工作內容。
type forecastJob struct {
	batch forecastBatcher
	// checkerFactory 比照 daily.go dailyJob.queriesFactory 的既有慣例：每次
	// 呼叫時用 gen.New(database.GetDBTX(ctx, pool)) 現組 *gen.Queries，不把
	// *gen.Queries 存成 singleton 欄位，讓 database.TxManager.WithTx 開的交易
	// （若未來有需要）能被自動撿起來。
	checkerFactory func(ctx context.Context) forecastResultChecker
	logger         *zap.Logger
	now            func() time.Time
}

// newForecastJob 建構 forecastJob。pool 只用來組 checkerFactory（Finding I2
// 的啟動補跑檢查）——forecastBatcher（*usecase.ForecastBatch）已經自己持有
// pool，這裡不重複經手實際批次寫入。
func newForecastJob(batch forecastBatcher, pool *pgxpool.Pool, logger *zap.Logger) *forecastJob {
	return &forecastJob{
		batch: batch,
		checkerFactory: func(ctx context.Context) forecastResultChecker {
			return gen.New(database.GetDBTX(ctx, pool))
		},
		logger: logger,
		now:    time.Now,
	}
}

// catchUpIfNeeded 是 Finding I2 補的啟動補跑：crop_group_forecast_result 是
// append-only 快照表、沒有回填機制，pkg/outbox.DailyScheduler 只在下一個排定
// 的 21:00 UTC 觸發一次——pod 剛好在那個時刻前後重啟/panic/逾時就會讓那天
// 永遠沒有快照，且事後無法重建（那天的 price_observation/model_artifact
// 狀態已經是過去式）。這裡在啟動 DailyScheduler 之前先查一次「今天是否已經
// 有結果」，沒有就補跑一次。
//
// 🔴 時刻閘門（scoped re-review 抓到的迴歸，補在 Finding I2 之後）：只查
// 「今天是否已有結果」不夠——若 pod 在今天 21:00 UTC *之前*重啟（例如白天的
// 例行 rolling deploy，此時 20:00 UTC 的行情 ingest 通常都還沒跑），
// HasForecastResultToday 一樣會回傳 false（今天理所當然還沒有結果，因為
// 排定時刻根本還沒到），若沒有這道閘門就會被誤判成「補跑情境」，提早用
// 尚未完成 ingest 的資料跑一次批次預測。這次提早寫入的資料列會先佔走今天
// 的 dedup key（group_code, target_date, 今天 UTC 曆日），等真正的 21:00
// UTC 排程觸發時，它的 INSERT 撞上 ON CONFLICT DO NOTHING 被靜默吞掉——
// 而 :exec 沒有 rows-affected 檢查，ForecastBatch.Handle 的
// GroupsWritten 計數仍照樣累加，log 端完全看不出「這次寫入其實是 no-op」。
// 淨效果是整個 21:00 UTC 排程時間的存在意義被推翻：任何一天只要白天有
// pod 重啟，最終留下的快照就是重啟當下那個不完整、pre-ingest 的版本，
// 而且沒有任何可觀察訊號會暴露這件事。
//
// 因此在做任何檢查/補跑之前，先比較 j.now().UTC() 是否已經到達或超過
// 今天的排定時刻（forecastHourUTC:forecastMinuteUTC）。還沒到——代表今天
// 這次排定執行根本還沒發生過，不是「被跳過」，沒有東西需要補——直接略過，
// 交給稍後啟動的 DailyScheduler 在真正的排定時刻準時執行。這裡刻意連
// HasForecastResultToday 都不查（不是查了但忽略結果）：時刻未到時查詢
// 結果毫無意義（今天當然還沒有），省一次不必要的 DB 呼叫。
//
// 到達或超過排定時刻之後，以下既有邏輯完全不變：
// 檢查本身失敗（例如 DB 暫時性錯誤）時刻意仍執行補跑，不是略過：
// crop_group_forecast_result_dedup_idx 的 ON CONFLICT DO NOTHING 讓補跑在
// 「其實已經跑過」時是安全的 no-op，所以就算判斷不準，寧可多跑一次（成本是
// 一次批次預測的時間，且有 forecastRunTimeout 兜底）也不要漏跑一次（成本是
// 那天永遠拿不回來，這正是 I2 要解決的問題）。
//
// 邊界情況（I2 存在的核心理由，這道閘門必須不能誤傷）：pod 在排定時刻
// 「剛過」不久重啟，例如 21:03 UTC——此時 DailyScheduler 內部的
// outbox.NextOccurrence 會算出「明天」的 21:00 UTC（今天這個時刻對它而言
// 已經過去），若沒有這支 catchUpIfNeeded，今天就會永遠沒有快照。now() >=
// 排定時刻的比較在這個情境下成立（21:03 >= 21:00），閘門會放行，沿用既有的
// HasForecastResultToday 判斷是否真的需要補跑——不影響 I2 原本要處理的
// 這個核心情境。
func (j *forecastJob) catchUpIfNeeded(ctx context.Context) {
	now := j.now().UTC()
	scheduledToday := time.Date(now.Year(), now.Month(), now.Day(), forecastHourUTC, forecastMinuteUTC, 0, 0, time.UTC)
	if now.Before(scheduledToday) {
		j.logger.Info("daily_forecast: 今日排定時刻尚未到，略過啟動補跑（交由排程準時觸發）",
			zap.Time("now", now),
			zap.Time("scheduled_today", scheduledToday))
		return
	}

	checker := j.checkerFactory(ctx)
	has, err := checker.HasForecastResultToday(ctx)
	if err != nil {
		j.logger.Error("daily_forecast: 啟動補跑檢查失敗，仍執行一次補跑（ON CONFLICT DO NOTHING 保證重複執行安全）",
			zap.Error(err))
		j.runCatchUp(ctx)
		return
	}
	if has {
		j.logger.Info("daily_forecast: 今日已有預測結果，略過啟動補跑")
		return
	}
	j.logger.Info("daily_forecast: 今日尚無預測結果，執行啟動補跑")
	j.runCatchUp(ctx)
}

// runWithTimeout 是傳給 outbox.NewDailyScheduler 的 run closure：只包住
// forecastJob.run 的單次執行逾時（forecastRunTimeout），跟
// dailyJob.runWithTimeout 同一個骨架（daily.go）。panic recovery 不在這裡：
// 排定路徑（透過 DailyScheduler.Start 的迴圈觸發）由 DailyScheduler.tick()
// 的 defer/recover 吸收，同 daily.go 的說明。
func (j *forecastJob) runWithTimeout(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, forecastRunTimeout)
	defer cancel()

	j.run(runCtx)
}

// runCatchUp 是 catchUpIfNeeded 專用的呼叫路徑：包一層自己的 panic recovery
// 在 runWithTimeout 外面。
//
// 🔴 這層 recover 是 daily.go/forecast_job.go 遷移到 pkg/outbox.DailyScheduler
// 後仍必須手動保留的一塊——DailyScheduler.tick() 的 recover 邊界只包住「由
// DailyScheduler.Start 自己的排程迴圈觸發」的呼叫；catchUpIfNeeded 是在
// DailyScheduler.Start 被呼叫之前，繞過排程迴圈直接呼叫 runWithTimeout 補跑
// 一次，這條路徑完全在 DailyScheduler 的保護範圍之外。若這裡不自己包
// recover，forecastJob.run 內部一旦 panic（例如 ForecastBatch.Handle 的
// DB 查詢/記憶體計算出錯），會直接從 OnStart 啟動的 goroutine 逃逸——Go
// 對「goroutine 內未被 recover 的 panic」的處理是終止整個 process，不是只
// 終止該 goroutine，等同讓一次批次預測的 bug 直接打掛整個 pricebook-service
// pod。這正是遷移前 runOnceWithRecover 同時保護排定路徑與補跑路徑的理由，
// 遷移後排定路徑的保護搬去了 DailyScheduler，補跑路徑的保護必須留在這裡。
func (j *forecastJob) runCatchUp(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			j.logger.Error("daily_forecast: 啟動補跑 panic recovered",
				zap.Any("recovered", r),
				zap.ByteString("stack", debug.Stack()))
		}
	}()
	j.runWithTimeout(ctx)
}

// run 是一次完整的批次預測執行：對所有 active crop_group 跑一次
// ForecastBatch.Handle，log 記完整結果（成功/跳過的 group 數量），不記
// 任何 audit table。
func (j *forecastJob) run(ctx context.Context) {
	origin := j.now().UTC()

	stats, err := j.batch.Handle(ctx, origin)
	if err != nil {
		j.logger.Error("daily_forecast: 批次預測失敗",
			zap.Error(err),
			zap.Int("groups_total", stats.GroupsTotal),
			zap.Int("groups_written", stats.GroupsWritten),
			zap.Int("groups_skipped", stats.GroupsSkipped))
		return
	}

	j.logger.Info("daily_forecast: 批次預測完成",
		zap.Int("groups_total", stats.GroupsTotal),
		zap.Int("groups_written", stats.GroupsWritten),
		zap.Int("groups_skipped", stats.GroupsSkipped),
		zap.Strings("skipped_groups", stats.SkippedGroups))
}
