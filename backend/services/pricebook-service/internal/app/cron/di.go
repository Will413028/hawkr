package cron

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// dailyIngestJobName 用於 outbox.DailyScheduler 的 "cron next run scheduled"
// log 與 "cron job panicked" log 的 job 欄位，方便 log 過濾。
const dailyIngestJobName = "pricebook_daily_ingest"

// catchupIngestJobName 是補抓輪次在排程 log 與 metric 上的名字。刻意跟完整
// 排程分開：兩者的失敗意義不同——完整排程沒跑代表當天的休市日曆與各類行情
// 全部沒更新，補抓沒跑只代表晚一點才補上。
const catchupIngestJobName = "pricebook_catchup_ingest"

// Module 註冊每日排程的 fx lifecycle hook。
//
// 🔴 刻意不做 leader election、不用 advisory lock、不限制成單一 instance：
// 本 repo 對任何 cron/worker 都沒有這類先例（`grep -rn "advisory_lock\|leader"
// services/` 零命中），且這個 chart 對 pricebook-service 的 replicas 預設就是
// 1（chart 預設值、dev/prod overlay 皆同）。唯一會讓兩個 pod 同時存在的情況
// 是 rolling update（`maxSurge:1`/`maxUnavailable:0`）短暫重疊，若那個瞬間剛好
// 跨過 20:00 UTC，兩個 pod 都會各自跑一次、各留一列 ingest_run——這是已知且
// 接受的性質，不是要修的 bug：資料側由 PriceIngest.Handle 內部的
// digest-dedupe 吸收（不會產生重複 revision），稽核側頂多多一列 Succeeded/
// Failed 紀錄。leader election 要解的問題在這裡根本不存在，請不要「修」它。
var Module = fx.Module("cron",
	fx.Invoke(startDailyIngestJob),
	fx.Invoke(startDailyForecastJob),
	fx.Invoke(startDailyRollupJob),
)

// startDailyIngestJob 把 dailyJob 掛進 fx lifecycle。
//
// 參數型別刻意用 *usecase.RestDayIngest / *usecase.PriceIngest（fx 圖裡實際會
// 解析的具體型別——internal/usecase/di.go 的 fx.Provide(NewRestDayIngest) /
// fx.Provide(NewPriceIngest) 就是這樣註冊的），而不是本套件宣告的
// restDayIngester / priceIngester 介面：dig 用「精確型別」做解析鍵，對介面型別
// 參數只有在某個 provider 明確宣告回傳該介面（或用 fx.Annotate(..., fx.As(...))）
// 時才會解析，單純的具體型別滿足介面的結構化事實不會被自動撿起來——這點已用
// go.uber.org/fx@v1.24.0 對照組驗證過（fx.ValidateApp 對一個只提供 *Concrete
// 卻要求 Iface 參數的 fx.Invoke 直接回報 "missing type: Iface"）。
// 交給 newDailyJob 時做一般 Go 介面賦值即可，不需要額外的 fx provider。
func startDailyIngestJob(
	lc fx.Lifecycle,
	restDay *usecase.RestDayIngest,
	price *usecase.PriceIngest,
	pork *usecase.PorkIngest,
	poultryChickenEgg *usecase.PoultryChickenEggIngest,
	poultryGooseDuck *usecase.PoultryGooseDuckIngest,
	rice *usecase.RiceIngest,
	pool *pgxpool.Pool,
	tsid func() int64,
	logger *zap.Logger,
) {
	job := newDailyJob(restDay, price, pork,
		[]poultryIngester{poultryChickenEgg.PoultryIngest, poultryGooseDuck.PoultryIngest},
		rice, pool, tsid, logger)
	scheduler := outbox.NewDailyScheduler(dailyHourUTC, dailyMinuteUTC, dailyIngestJobName, job.runWithTimeout, logger)

	// 補抓輪次：同一份依賴、同一支 run，只是 priceOnly 讓它跳過休市日曆與
	// 肉品／禽蛋／米價。用既有的 DailyScheduler 各註冊一個（而不是替 pkg/outbox
	// 新增一個 interval 原語）——pkg 是所有服務共用的，為了這件事動它不划算，
	// 而且 repo 內本來就有一個檔註冊三個 DailyScheduler 的先例。
	catchupSchedulers := make([]*outbox.DailyScheduler, 0, len(catchupHoursUTC))
	for _, hour := range catchupHoursUTC {
		catchupJob := newDailyJob(restDay, price, pork,
			[]poultryIngester{poultryChickenEgg.PoultryIngest, poultryGooseDuck.PoultryIngest},
			rice, pool, tsid, logger)
		catchupJob.priceOnly = true
		catchupSchedulers = append(catchupSchedulers,
			outbox.NewDailyScheduler(hour, catchupMinuteUTC, catchupIngestJobName, catchupJob.runWithTimeout, logger))
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			// Pass context.Background(), not the OnStart hook's ctx — see
			// outbox.DailyScheduler.Start's doc comment for why (fx
			// start-timeout pitfall: that ctx gets cancelled once fx
			// considers startup finished, which would kill this loop almost
			// immediately).
			scheduler.Start(context.Background())
			for _, s := range catchupSchedulers {
				s.Start(context.Background())
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			scheduler.Stop()
			for _, s := range catchupSchedulers {
				s.Stop()
			}
			return nil
		},
	})
}

// startDailyForecastJob 把 forecastJob 掛進 fx lifecycle，跟
// startDailyIngestJob 同一個骨架（生命週期管理、OnStop 的 cancel()）。
//
// 參數型別用 *usecase.ForecastBatch（具體型別，理由同 startDailyIngestJob
// 上方註解：dig 用精確型別做解析鍵，*usecase.ForecastBatch 已滿足本套件的
// forecastBatcher 窄介面，交給 newForecastJob 時做一般 Go 介面賦值即可）。
//
// pool（final whole-branch review Finding I2 新增的參數）：newForecastJob
// 用它組 checkerFactory，供 OnStart 啟動補跑檢查「今天是否已有預測結果」
// 用——*pgxpool.Pool 已由 pricebookpostgres.Module 提供，不需要新 provider，
// 與 startDailyIngestJob 既有的 pool 參數是同一份 fx 圖依賴。
func startDailyForecastJob(
	lc fx.Lifecycle,
	batch *usecase.ForecastBatch,
	pool *pgxpool.Pool,
	logger *zap.Logger,
) {
	job := newForecastJob(batch, pool, logger)
	scheduler := outbox.NewDailyScheduler(forecastHourUTC, forecastMinuteUTC, forecastJobName, job.runWithTimeout, logger)

	// ctx/cancel is kept local here (unlike startDailyIngestJob, which just
	// uses context.Background()+scheduler.Stop()) because catchUpIfNeeded's
	// runCatchUp call below happens *before* scheduler.Start is ever called
	// — outbox.DailyScheduler.Stop() has nothing to cancel yet at that point
	// (its internal cancel func is nil until Start runs), so it cannot reach
	// an in-flight catch-up run. Threading our own cancelable ctx through
	// both catchUpIfNeeded and scheduler.Start(ctx) preserves the pre-
	// migration behavior: OnStop's cancellation signal reaches an in-flight
	// catch-up run exactly as it did when both were driven by the same
	// shared ctx under the old runDailyAt.
	ctx, cancel := context.WithCancel(context.Background())

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			// Finding I2：先做一次啟動補跑檢查，再啟動 DailyScheduler 接手
			// 往後的排程。刻意放進 go routine、不放在 OnStart 本身同步執行：
			// OnStart 同步呼叫會讓 fx 啟動（進而 pod readiness）被卡住最多
			// forecastRunTimeout（10 分鐘，補跑真的觸發時）；放進 go routine
			// 讓 fx.Invoke(startDailyForecastJob) 維持原本「立刻回傳」的行為，
			// 跟 startDailyIngestJob 一致——代價是補跑完成前這段時間排程的
			// "cron next run scheduled" log 會延後出現，可接受，補跑本身也
			// 會留下自己的 log（見 forecastJob.catchUpIfNeeded）。
			go func() {
				job.catchUpIfNeeded(ctx)
				scheduler.Start(ctx)
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			scheduler.Stop()
			return nil
		},
	})
}

// startDailyRollupJob 把 rollupJob 掛進 fx lifecycle，跟 startDailyIngestJob
// 同一個骨架（context.Background()+scheduler.Stop()）。不比照
// startDailyForecastJob 的 catchUpIfNeeded/啟動補跑機制——見 rollup_job.go
// 檔案頂端註解：category_price_rollup 不是 append-only 表、沒有需要保護的
// per-day dedup index，錯過一次排定執行只是讓昨天的快照多留到下一次排程，
// 不像 crop_group_forecast_result 那樣「那天永遠沒有快照」。
//
// 參數型別用 *usecase.RollupBatch（具體型別，理由同 startDailyIngestJob/
// startDailyForecastJob 上方註解：dig 用精確型別做解析鍵，*usecase.RollupBatch
// 已滿足本套件的 rollupBatcher 窄介面，交給 newRollupJob 時做一般 Go 介面
// 賦值即可）。
func startDailyRollupJob(lc fx.Lifecycle, batch *usecase.RollupBatch, logger *zap.Logger) {
	job := newRollupJob(batch, logger)
	scheduler := outbox.NewDailyScheduler(rollupHourUTC, rollupMinuteUTC, rollupJobName, job.runWithTimeout, logger)
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			scheduler.Start(context.Background())
			return nil
		},
		OnStop: func(_ context.Context) error {
			scheduler.Stop()
			return nil
		},
	})
}
