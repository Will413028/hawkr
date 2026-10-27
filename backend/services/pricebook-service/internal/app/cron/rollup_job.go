// rollup_job.go 是每日批次分類價格彙總排程：對所有 resolved category_crop_
// mapping 各算一次 group 層價格，再由上而下走一次 product-service 分類樹算
// L2/L1，寫進 category_price_rollup。排在每日預測 cron（21:00 UTC）之後，
// 給它的 10 分鐘逾時預算留緩衝，避免兩支批次同時打 DB。
//
// 跟 forecast_job.go 不同，這支 job 刻意不比照 forecastJob 的
// catchUpIfNeeded/啟動補跑機制：category_price_rollup 不是 append-only
// 快照表，沒有需要保護的 per-day dedup index（不像
// crop_group_forecast_result_dedup_idx）。每次批次執行都是覆寫今天的彙總結果
// （見 rollupCategoryTree/rollupGroupLayer 的寫入邏輯），錯過一次排定執行
// （21:15 UTC）只是讓昨天的快照多留到下一次排程，不是「那天永遠沒有資料」
// ——outbox.DailyScheduler 本身的下一輪排程已經足以處理，不需要額外的啟動
// 補跑保護。
package cron

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

const (
	rollupHourUTC   = 21
	rollupMinuteUTC = 15
)

const rollupJobName = "pricebook_daily_category_rollup"

// rollupRunTimeout：跟 forecastRunTimeout 同樣是第一版保守估計（同一份
// 🔴 說明適用——上線後應依 dev 環境實測的分類樹規模與 group 數重新校準）。
const rollupRunTimeout = 10 * time.Minute

type rollupBatcher interface {
	Handle(ctx context.Context, origin time.Time) (usecase.RollupBatchStats, error)
}

type rollupJob struct {
	batch  rollupBatcher
	logger *zap.Logger
	now    func() time.Time
}

func newRollupJob(batch rollupBatcher, logger *zap.Logger) *rollupJob {
	return &rollupJob{batch: batch, logger: logger, now: time.Now}
}

func (j *rollupJob) runWithTimeout(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, rollupRunTimeout)
	defer cancel()
	j.run(runCtx)
}

// run 是一次完整的批次彙總執行。刻意不在這裡包 defer/recover：這個方法只透過
// runWithTimeout 被 outbox.NewDailyScheduler 排定呼叫，DailyScheduler.tick()
// 自己的 defer/recover 已經包住整個 run closure 的呼叫（見
// pkg/outbox/daily_scheduler.go），並產出本套件所有 cron job 共用的標準
// "cron job panicked"（job/recovered/stack 欄位）log——同 daily.go
// runWithTimeout 上方註解記錄的既有慣例（forecast_job.go 的 run() 同理沒有
// 本地 recover；forecast_job.go 唯一的本地 recover 在 runCatchUp，那是完全
// 不同、繞過 DailyScheduler 保護的啟動補跑路徑，rollup_job.go 沒有那條路徑，
// 因此也沒有本地 recover 存在的理由）。這裡若重複包一層，一旦真的 panic，
// 會被本地 recover 先吃掉、log 成不成套的 "daily_category_rollup: panic
// recovered"（沒有 job 欄位），繞過所有 cron job 共用的告警/儀表板規則。
func (j *rollupJob) run(ctx context.Context) {
	origin := j.now().UTC()
	stats, err := j.batch.Handle(ctx, origin)
	if err != nil {
		j.logger.Error("daily_category_rollup: 批次彙總失敗", zap.Error(err))
		return
	}
	j.logger.Info("daily_category_rollup: 批次彙總完成", zap.Int("categories_written", stats.CategoriesWritten))
}
