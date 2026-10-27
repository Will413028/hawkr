package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// stubForecastBatcher 是 forecastBatcher 的測試替身，比照 daily_test.go
// 對 restDayIngester/priceIngester 的既有 stub 手法。
type stubForecastBatcher struct {
	stats usecase.ForecastBatchStats
	err   error
	calls []time.Time
}

func (s *stubForecastBatcher) Handle(_ context.Context, origin time.Time) (usecase.ForecastBatchStats, error) {
	s.calls = append(s.calls, origin)
	return s.stats, s.err
}

func TestForecastJobRun_Success_LogsStats(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	stub := &stubForecastBatcher{stats: usecase.ForecastBatchStats{
		GroupsTotal: 5, GroupsWritten: 4, GroupsSkipped: 1, SkippedGroups: []string{"grp-x"},
	}}
	job := newForecastJob(stub, nil, logger)
	job.now = func() time.Time { return time.Date(2026, 8, 8, 21, 0, 0, 0, time.UTC) }

	job.run(context.Background())

	require.Len(t, stub.calls, 1)
	require.Equal(t, time.Date(2026, 8, 8, 21, 0, 0, 0, time.UTC), stub.calls[0])

	entries := logs.FilterMessage("daily_forecast: 批次預測完成").All()
	require.Len(t, entries, 1)
}

func TestForecastJobRun_HandleError_LogsErrorNotPanic(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	stub := &stubForecastBatcher{err: errors.New("列出 active crop_group 失敗: connection refused")}
	job := newForecastJob(stub, nil, logger)

	job.run(context.Background())

	entries := logs.FilterMessage("daily_forecast: 批次預測失敗").All()
	require.Len(t, entries, 1)
}

// TestForecastJobRunWithTimeout_PanicPropagates 是遷移到
// pkg/outbox.DailyScheduler 後的行為變更的回歸測試：panic recovery 不再是
// forecastJob.runWithTimeout 自己的責任（那層防護搬去了排定路徑的
// outbox.DailyScheduler.tick()，見 forecast_job.go 的 runWithTimeout 說明），
// 所以直接呼叫 runWithTimeout（跳過 DailyScheduler）時 panic 必須原樣傳播出
// 去，不能被吞掉——用自己的 recover 接住它，斷言「這裡沒有 recover」這件事
// 本身。舊版 TestForecastJobRunOnceWithRecover_SurvivesPanic 斷言的「panic
// 被吞掉」行為，排定路徑那半已經在 pkg/outbox/daily_scheduler_test.go 的
// TestDailyScheduler_PanicInRunIsRecoveredNotPropagated 測過；補跑路徑那半見
// 下面的 TestForecastJobRunCatchUp_SurvivesPanic。
func TestForecastJobRunWithTimeout_PanicPropagates(t *testing.T) {
	logger := zap.NewNop()
	job := newForecastJob(&panickingForecastBatcher{}, nil, logger)

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("runWithTimeout 應該讓 panic 原樣傳播出去——recovery 已經搬到 outbox.DailyScheduler.tick()，這裡不該再攔一次")
		}
	}()
	job.runWithTimeout(context.Background())
}

// TestForecastJobRunCatchUp_SurvivesPanic 覆蓋的是 runWithTimeout 不再提供、
// 但 catchUpIfNeeded 呼叫路徑仍然需要的 panic 防護：catchUpIfNeeded 是在
// outbox.DailyScheduler.Start 被呼叫之前直接觸發一次補跑，完全繞過
// DailyScheduler.tick() 的 recover 邊界（見 forecast_job.go runCatchUp 的
// 說明）。若這裡沒有自己的 recover，一次批次預測 panic 會直接讓啟動這個
// job 的 goroutine 崩潰，進而終止整個 pricebook-service process。
func TestForecastJobRunCatchUp_SurvivesPanic(t *testing.T) {
	logger := zap.NewNop()
	job := newForecastJob(&panickingForecastBatcher{}, nil, logger)

	// 不應該讓呼叫端也跟著 panic——這是 runCatchUp 存在的唯一理由。
	job.runCatchUp(context.Background())
}

type panickingForecastBatcher struct{}

func (panickingForecastBatcher) Handle(context.Context, time.Time) (usecase.ForecastBatchStats, error) {
	panic("boom")
}

// --- Finding I2：啟動補跑檢查（catchUpIfNeeded）---

// stubForecastResultChecker 是 forecastResultChecker 的測試替身，比照本檔
// stubForecastBatcher 與 daily_test.go stubRecorder 的既有 stub 手法。
type stubForecastResultChecker struct {
	has    bool
	err    error
	called bool
}

func (s *stubForecastResultChecker) HasForecastResultToday(_ context.Context) (bool, error) {
	s.called = true
	return s.has, s.err
}

// newTestForecastJob 建構一個只用 fake 依賴的 forecastJob，繞過
// newForecastJob（避免需要真的 *pgxpool.Pool），比照 daily_test.go
// newTestJob 對 dailyJob 的既有手法：直接用 struct literal 注入
// checkerFactory。
func newTestForecastJob(batch forecastBatcher, checker forecastResultChecker, logger *zap.Logger) *forecastJob {
	return &forecastJob{
		batch: batch,
		checkerFactory: func(_ context.Context) forecastResultChecker {
			return checker
		},
		logger: logger,
		now:    time.Now,
	}
}

// scheduledForecastSlotToday 是本檔測試共用的固定「今天」，用來建構
// 「今天排定時刻之前／之後」兩種 now，避免各測試各自手算日期。
var scheduledForecastSlotToday = time.Date(2026, 8, 8, forecastHourUTC, forecastMinuteUTC, 0, 0, time.UTC)

// panicOnCallForecastResultChecker 是 forecastResultChecker 的測試替身：
// 只要被呼叫就 panic。用於證明「時刻未到」的閘門真的在呼叫
// HasForecastResultToday 之前就短路返回，不是查了但忽略結果——比單純斷言
// call count 為 0 更強：就算未來有人不小心讓程式碼路徑「查了但沒用上」，
// 這支替身也會讓測試立刻用 panic 曝露出來。
type panicOnCallForecastResultChecker struct{}

func (panicOnCallForecastResultChecker) HasForecastResultToday(context.Context) (bool, error) {
	panic("HasForecastResultToday 不應該在今日排定時刻之前被呼叫")
}

func TestForecastJobCatchUpIfNeeded_BeforeScheduledSlot_SkipsWithoutCheckingOrRunning(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	batch := &stubForecastBatcher{}
	job := newTestForecastJob(batch, panicOnCallForecastResultChecker{}, logger)
	job.now = func() time.Time { return scheduledForecastSlotToday.Add(-12 * time.Hour) } // 09:00 UTC

	job.catchUpIfNeeded(context.Background())

	require.Empty(t, batch.calls, "今日排定時刻尚未到，不該提早用 pre-ingest 資料跑批次預測")
	entries := logs.FilterMessage("daily_forecast: 今日排定時刻尚未到，略過啟動補跑（交由排程準時觸發）").All()
	require.Len(t, entries, 1)
}

func TestForecastJobCatchUpIfNeeded_AtScheduledSlot_NoResultToday_RunsOnce(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	batch := &stubForecastBatcher{}
	checker := &stubForecastResultChecker{has: false}
	job := newTestForecastJob(batch, checker, logger)
	job.now = func() time.Time { return scheduledForecastSlotToday.Add(5 * time.Minute) } // 21:05 UTC

	job.catchUpIfNeeded(context.Background())

	require.True(t, checker.called)
	require.Len(t, batch.calls, 1, "今天還沒有結果時必須立刻補跑一次")
	entries := logs.FilterMessage("daily_forecast: 今日尚無預測結果，執行啟動補跑").All()
	require.Len(t, entries, 1)
}

func TestForecastJobCatchUpIfNeeded_AtScheduledSlot_HasResultToday_SkipsRun(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	batch := &stubForecastBatcher{}
	checker := &stubForecastResultChecker{has: true}
	job := newTestForecastJob(batch, checker, logger)
	job.now = func() time.Time { return scheduledForecastSlotToday.Add(5 * time.Minute) } // 21:05 UTC

	job.catchUpIfNeeded(context.Background())

	require.True(t, checker.called, "必須先查過今天是否已有結果")
	require.Empty(t, batch.calls, "今天已有結果時不該補跑")
	entries := logs.FilterMessage("daily_forecast: 今日已有預測結果，略過啟動補跑").All()
	require.Len(t, entries, 1)
}

func TestForecastJobCatchUpIfNeeded_AtScheduledSlot_CheckError_RunsAnywayAsSafetyNet(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	batch := &stubForecastBatcher{}
	checker := &stubForecastResultChecker{err: errors.New("connection refused")}
	job := newTestForecastJob(batch, checker, logger)
	job.now = func() time.Time { return scheduledForecastSlotToday.Add(5 * time.Minute) } // 21:05 UTC

	job.catchUpIfNeeded(context.Background())

	require.True(t, checker.called)
	require.Len(t, batch.calls, 1,
		"檢查本身失敗時仍要補跑一次——ON CONFLICT DO NOTHING 讓多跑一次安全，"+
			"漏跑一次才是不可逆的成本（Finding I2 的核心理由）")
	entries := logs.FilterMessage("daily_forecast: 啟動補跑檢查失敗，仍執行一次補跑（ON CONFLICT DO NOTHING 保證重複執行安全）").All()
	require.Len(t, entries, 1)
}

// TestForecastJobCatchUpIfNeeded_ExactlyAtScheduledSlot_StillRuns 驗證邊界是
// inclusive 的：now 剛好等於排定時刻（不是之後）也要當成「已到」，不是
// 「尚未到」——閘門用 now.Before(scheduledToday)，剛好相等時 Before 為
// false，所以會放行，符合規格「at or after」的措辭。
func TestForecastJobCatchUpIfNeeded_ExactlyAtScheduledSlot_StillRuns(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	batch := &stubForecastBatcher{}
	checker := &stubForecastResultChecker{has: false}
	job := newTestForecastJob(batch, checker, logger)
	job.now = func() time.Time { return scheduledForecastSlotToday } // 21:00:00 UTC 整

	job.catchUpIfNeeded(context.Background())

	require.True(t, checker.called)
	require.Len(t, batch.calls, 1, "now 剛好等於排定時刻時也算「已到」，不是「尚未到」")
	entries := logs.FilterMessage("daily_forecast: 今日尚無預測結果，執行啟動補跑").All()
	require.Len(t, entries, 1)
}

// TestForecastJobCatchUpIfNeeded_ShortlyAfterScheduledSlot_StillRuns 覆蓋 I2
// 存在的核心情境（見 forecast_job.go 的 catchUpIfNeeded 註解「邊界情況」
// 段落）：pod 在排定時刻剛過不久重啟（例如 21:03 UTC），此時
// outbox.DailyScheduler 內部的 outbox.NextOccurrence 會算出明天的 21:00 UTC
// （今天這個時刻對它而言已經過去），若時刻閘門誤傷了這個情境，今天就會永遠
// 沒有快照——這是這次補的閘門唯一
// 不能犯的錯誤，所以獨立於「21:05 UTC」的其餘測試之外，明確用一個更貼近
// 排定時刻邊界的 now 再驗證一次。
func TestForecastJobCatchUpIfNeeded_ShortlyAfterScheduledSlot_StillRuns(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	batch := &stubForecastBatcher{}
	checker := &stubForecastResultChecker{has: false}
	job := newTestForecastJob(batch, checker, logger)
	job.now = func() time.Time { return scheduledForecastSlotToday.Add(3 * time.Minute) } // 21:03 UTC

	job.catchUpIfNeeded(context.Background())

	require.True(t, checker.called)
	require.Len(t, batch.calls, 1,
		"pod 在排定時刻剛過不久重啟時必須仍能補跑——這正是 I2 要處理的情境，"+
			"閘門不能誤傷它")
	entries := logs.FilterMessage("daily_forecast: 今日尚無預測結果，執行啟動補跑").All()
	require.Len(t, entries, 1)
}
