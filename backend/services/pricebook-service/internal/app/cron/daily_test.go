package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// --- stubs：只滿足 cron 套件自己宣告的窄介面，不打真實 MOA / 真實 DB ---
//
// nextOccurrence/runDailyAt 已被 pkg/outbox.NextOccurrence/DailyScheduler 取代
// 並移除；等效覆蓋（今天已過/尚未過目標時刻、月份邊界等）已在
// pkg/outbox/daily_scheduler_test.go 的 TestNextOccurrence_* 系列測試，這裡
// 不再重複。

type stubRestDayIngester struct {
	written int
	err     error
	called  bool
}

func (s *stubRestDayIngester) Handle(_ context.Context) (int, error) {
	s.called = true
	return s.written, s.err
}

// panicRestDayIngester 用於 TestRunWithTimeout_PanicPropagates（見本檔下方）
// 證明 dailyJob.runWithTimeout 不再自己 recover panic——那層防護已經搬到
// pkg/outbox.DailyScheduler.tick()，由呼叫端（di.go 的 startDailyIngestJob）
// 負責透過 outbox.NewDailyScheduler 接上。
type panicRestDayIngester struct{}

func (panicRestDayIngester) Handle(_ context.Context) (int, error) {
	panic("boom: restday")
}

type stubPriceIngester struct {
	stats            usecase.IngestStats
	err              error
	declareErr       error
	called           bool
	declared         bool
	gotCtx           context.Context
	gotStart, gotEnd time.Time
}

// DeclareDue 記下宣告區間；gotStart/gotEnd 沿用原本欄位，因為排程現在傳給
// 宣告的就是它原本傳給 Handle 的那組日期（語意已從「抓取區間」變成「宣告
// 區間」，見 daily.go）。
func (s *stubPriceIngester) DeclareDue(ctx context.Context, from, through time.Time) error {
	s.declared = true
	s.gotCtx = ctx
	s.gotStart = from
	s.gotEnd = through
	return s.declareErr
}

func (s *stubPriceIngester) HandleDue(ctx context.Context) (usecase.IngestStats, error) {
	s.called = true
	s.gotCtx = ctx
	return s.stats, s.err
}

type stubRecorder struct {
	createErr    error
	finishErr    error
	createCalled bool
	finishCalled bool
	createArg    gen.CreateIngestRunParams
	finishArg    gen.FinishIngestRunParams
	finishCtxErr error // ctx.Err() 在 FinishIngestRun 被呼叫的當下——用來偵測有沒有沿用一個已經過期的外層 ctx
}

func (s *stubRecorder) CreateIngestRun(_ context.Context, arg gen.CreateIngestRunParams) (int64, error) {
	s.createCalled = true
	s.createArg = arg
	if s.createErr != nil {
		return 0, s.createErr
	}
	return arg.ID, nil
}

func (s *stubRecorder) FinishIngestRun(ctx context.Context, arg gen.FinishIngestRunParams) error {
	s.finishCalled = true
	s.finishArg = arg
	s.finishCtxErr = ctx.Err()
	return s.finishErr
}

// newTestJob 建構一個只用 fake 依賴的 dailyJob，供本檔案的邏輯測試共用。
// fixedNow 固定 dailyJob.now()，讓行情窗口的起訖時間可預期比對。
func newTestJob(t *testing.T, restDay restDayIngester, price priceIngester, recorder *stubRecorder, logger *zap.Logger, fixedNow time.Time) *dailyJob {
	t.Helper()
	return &dailyJob{
		restDay: restDay,
		price:   price,
		tsid:    func() int64 { return 999000000000000001 },
		queriesFactory: func(_ context.Context) ingestRunRecorder {
			return recorder
		},
		logger: logger,
		now:    func() time.Time { return fixedNow },
	}
}

// TestDailyJobRun_PriceOnlySkipsEverythingButPrice 釘住 2026-09-14 加入的補抓
// 排程：一天多跑兩輪的目的只有一個——把上游稍晚才上架的格子早點撿回來，所以那
// 兩輪只做「宣告 + 消費待補分區」，不得順便重跑休市日曆與肉品／禽蛋／米價。
//
// 背景：排程原本一天只跑一次（20:00 UTC ＝ 台北 04:00）。2026-09-13 那次跑的時候
// MOA 還沒放出 09-12 的資料，抓到 0 筆；而當天稍晚資料就上架了，我方卻要等到隔天
// 04:00 才會再問一次，實測讓約 3,500 列已可取得的資料多躺了將近一天。
func TestDailyJobRun_PriceOnlySkipsEverythingButPrice(t *testing.T) {
	restDay := &stubRestDayIngester{written: 3}
	price := &stubPriceIngester{stats: usecase.IngestStats{Fetched: 7, Inserted: 7}}
	recorder := &stubRecorder{}
	job := newTestJob(t, restDay, price, recorder, zap.NewNop(), time.Now())
	job.priceOnly = true

	job.run(context.Background())

	if restDay.called {
		t.Error("priceOnly 輪次不得同步休市日曆——一天三次是浪費，而且那份資料極少變動")
	}
	if !price.declared {
		t.Error("priceOnly 仍要宣告分區：跨 UTC 日界時會有新的一天要宣告，DeclareDue 本身冪等")
	}
	if !price.called {
		t.Error("priceOnly 的全部意義就是消費待補分區，HandleDue 必須被呼叫")
	}
	if !recorder.createCalled || !recorder.finishCalled {
		t.Error("補抓輪次一樣要留下 ingest_run 稽核紀錄，否則查不到它跑過沒有")
	}
}

func TestDailyJobRun_RestDayFailureDoesNotBlockPriceIngest(t *testing.T) {
	restDay := &stubRestDayIngester{err: errors.New("moa 休市日曆逾時")}
	price := &stubPriceIngester{stats: usecase.IngestStats{Fetched: 5, Inserted: 5}}
	recorder := &stubRecorder{}
	job := newTestJob(t, restDay, price, recorder, zap.NewNop(), time.Now())

	job.run(context.Background())

	if !restDay.called {
		t.Fatalf("restDay.Handle 應被呼叫")
	}
	if !price.called {
		t.Fatalf("RestDayIngest 失敗不應擋下 PriceIngest.Handle 的呼叫")
	}
	if !recorder.finishCalled {
		t.Fatalf("FinishIngestRun 應被呼叫")
	}
	if recorder.finishArg.Status != domain.IngestStatusFailed {
		t.Errorf("休市日曆失敗仍應把整輪 ingest_run 記為 Failed，got status=%d", recorder.finishArg.Status)
	}
	if recorder.finishArg.ErrorMessage == nil || *recorder.finishArg.ErrorMessage == "" {
		t.Errorf("error_message 不應為空——不能悄悄吞掉唯一的失敗訊號")
	}
	// PriceIngest 本身成功，統計仍應正確落地到 ingest_run。
	if recorder.finishArg.FetchedCount != 5 || recorder.finishArg.InsertedCount != 5 {
		t.Errorf("PriceIngest 的統計應完整記錄，got fetched=%d inserted=%d",
			recorder.finishArg.FetchedCount, recorder.finishArg.InsertedCount)
	}
}

func TestDailyJobRun_BothFail_LogsAndRecordsFailure(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	restDay := &stubRestDayIngester{err: errors.New("休市日曆 API 500")}
	price := &stubPriceIngester{err: errors.New("行情 API 逾時")}
	recorder := &stubRecorder{}
	job := newTestJob(t, restDay, price, recorder, zap.New(core), time.Now())

	job.run(context.Background())

	if logs.Len() < 2 {
		t.Fatalf("兩個步驟都失敗應各留一筆 error log，got %d 筆", logs.Len())
	}
	if !recorder.finishCalled {
		t.Fatalf("FinishIngestRun 應被呼叫（即使兩者都失敗，也要留下稽核紀錄，不能靜默）")
	}
	if recorder.finishArg.Status != domain.IngestStatusFailed {
		t.Errorf("兩者皆失敗應記為 Failed，got status=%d", recorder.finishArg.Status)
	}
	if recorder.finishArg.ErrorMessage == nil || *recorder.finishArg.ErrorMessage == "" {
		t.Errorf("error_message 應同時帶有兩個失敗原因")
	}
}

func TestDailyJobRun_BothSucceed_RecordsSuccessWithStats(t *testing.T) {
	restDay := &stubRestDayIngester{written: 7}
	price := &stubPriceIngester{stats: usecase.IngestStats{Fetched: 30, Inserted: 12, Revised: 4}}
	recorder := &stubRecorder{}
	job := newTestJob(t, restDay, price, recorder, zap.NewNop(), time.Now())

	job.run(context.Background())

	if recorder.finishArg.Status != domain.IngestStatusSucceeded {
		t.Errorf("兩者皆成功應記為 Succeeded，got status=%d", recorder.finishArg.Status)
	}
	if recorder.finishArg.ErrorMessage != nil {
		t.Errorf("成功時 error_message 不應被設定，got %q", *recorder.finishArg.ErrorMessage)
	}
	if recorder.finishArg.FetchedCount != 30 || recorder.finishArg.InsertedCount != 12 || recorder.finishArg.RevisedCount != 4 {
		t.Errorf("統計應原封不動落地，got fetched=%d inserted=%d revised=%d",
			recorder.finishArg.FetchedCount, recorder.finishArg.InsertedCount, recorder.finishArg.RevisedCount)
	}
}

// TestDailyJobRun_EverythingSkipped_RecordsNeedsAttention 補的是一個監控盲點：
// MOA 若改動 TransDate 格式，當日每一列都會在解析階段被跳過，但 ingest 流程
// 本身不會回傳任何錯誤。舊行為把這輪記成 succeeded / inserted=0，而 ingest_run
// 沒有 skipped_count 欄位——除了逐列 log 之外沒有任何持久化訊號，行情靜靜停止
// 更新而不會有人發現。
func TestDailyJobRun_EverythingSkipped_RecordsNeedsAttention(t *testing.T) {
	restDay := &stubRestDayIngester{written: 7}
	price := &stubPriceIngester{stats: usecase.IngestStats{Fetched: 4292, Skipped: 4292}}
	recorder := &stubRecorder{}
	job := newTestJob(t, restDay, price, recorder, zap.NewNop(), time.Now())

	job.run(context.Background())

	if recorder.finishArg.Status != domain.IngestStatusNeedsAttention {
		t.Errorf("抓到 4292 列卻一列都沒落地，應記為 NeedsAttention，got status=%d", recorder.finishArg.Status)
	}
	if recorder.finishArg.ErrorMessage == nil || *recorder.finishArg.ErrorMessage == "" {
		t.Errorf("error_message 是唯一的持久化訊號（ingest_run 無 skipped_count 欄位），不可為空")
	}
}

// TestDailyJobRun_PartialSkipStillSucceeds 是上一個測試的反面護欄：有列被跳過
// 但也有列真的寫進去了，代表 pipeline 是通的（壞的是個別資料列），不該升成
// 需人工關注——否則單筆髒資料就會天天觸發假警報，把這個訊號稀釋掉。
func TestDailyJobRun_PartialSkipStillSucceeds(t *testing.T) {
	restDay := &stubRestDayIngester{written: 7}
	price := &stubPriceIngester{stats: usecase.IngestStats{Fetched: 30, Inserted: 12, Revised: 4, Skipped: 3}}
	recorder := &stubRecorder{}
	job := newTestJob(t, restDay, price, recorder, zap.NewNop(), time.Now())

	job.run(context.Background())

	if recorder.finishArg.Status != domain.IngestStatusSucceeded {
		t.Errorf("有列成功落地就仍算成功，got status=%d", recorder.finishArg.Status)
	}
	if recorder.finishArg.ErrorMessage != nil {
		t.Errorf("部分跳過不應寫 error_message，got %q", *recorder.finishArg.ErrorMessage)
	}
}

// TestDailyJobRun_NoDataAtAllStillSucceeds 守住另一個邊界：休市日本來就抓不到
// 任何行情列（Fetched=0、Skipped=0），那是正常狀態，不是需要人工看的異常。
func TestDailyJobRun_NoDataAtAllStillSucceeds(t *testing.T) {
	restDay := &stubRestDayIngester{written: 7}
	price := &stubPriceIngester{stats: usecase.IngestStats{}}
	recorder := &stubRecorder{}
	job := newTestJob(t, restDay, price, recorder, zap.NewNop(), time.Now())

	job.run(context.Background())

	if recorder.finishArg.Status != domain.IngestStatusSucceeded {
		t.Errorf("休市日零資料是正常狀態，應記為 Succeeded，got status=%d", recorder.finishArg.Status)
	}
}

// TestDailyJobRun_DeclarationBounds 釘住宣告區間的起訖。
//
// 這個測試原本守的是「抓取窗口」（today-windowDays ~ today）。分區模型下排程
// 不再決定抓取範圍（工作清單住在 ingest_partition），它決定的是「宣告到哪一天
// 為止」，兩個邊界各有理由、都不是隨意的：
//
//	上界 today-1：今天的行情上游還沒出，宣告了只會白白燒掉一次 attempt。
//	下界 today-7：容忍連續漏跑一週；跟舊窗口不同的是漏掉的格子會留成 pending，
//	              不會因為時間過去就消失。
func TestDailyJobRun_DeclarationBounds(t *testing.T) {
	fixedNow := time.Date(2026, 7, 28, 20, 0, 0, 0, time.UTC)
	restDay := &stubRestDayIngester{}
	price := &stubPriceIngester{}
	recorder := &stubRecorder{}
	job := newTestJob(t, restDay, price, recorder, zap.NewNop(), fixedNow)

	job.run(context.Background())

	wantStart := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC) // today - DeclareLookbackDays
	wantEnd := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)   // today - 1（今天的行情還沒出）
	if !price.declared {
		t.Fatalf("DeclareDue 應被呼叫——沒有宣告就消費，等於把「還沒宣告」誤當成「沒有要抓的」")
	}
	if !price.gotStart.Equal(wantStart) {
		t.Errorf("DeclareDue 收到的 from = %v，預期 %v", price.gotStart, wantStart)
	}
	if !price.gotEnd.Equal(wantEnd) {
		t.Errorf("DeclareDue 收到的 through = %v，預期 %v", price.gotEnd, wantEnd)
	}

	gotRangeStart := pgxutilDateValue(t, recorder.createArg.RangeStart)
	gotRangeEnd := pgxutilDateValue(t, recorder.createArg.RangeEnd)
	if !gotRangeStart.Equal(wantStart) {
		t.Errorf("ingest_run.range_start = %v，預期 %v", gotRangeStart, wantStart)
	}
	if !gotRangeEnd.Equal(wantEnd) {
		t.Errorf("ingest_run.range_end = %v，預期 %v", gotRangeEnd, wantEnd)
	}

	// IngestKindDaily 與 IngestKindBackfill 是同檔案裡相鄰的常數
	// （internal/domain/constants.go），釘住這個斷言防止一次筆誤把 daily 排程
	// 誤記成 backfill 也能讓整個測試套件綠燈通過。
	if recorder.createArg.Kind != domain.IngestKindDaily {
		t.Errorf("ingest_run.kind = %d，預期 domain.IngestKindDaily (%d)",
			recorder.createArg.Kind, domain.IngestKindDaily)
	}
}

// TestDailyJobRun_FinishIngestRunSurvivesExpiredCtx 是 review fix round 的
// 回歸測試：run 收到的 ctx 在跑到 FinishIngestRun 之前就已經過期
// （dailyRunTimeout 逾時、或 OnStop 的 cancel() 撞上尾聲都是同一種情境），
// 收尾寫入必須用剝離過 deadline 的 ctx，否則「記錄這次逾時失敗」這個動作會
// 自己也連帶失敗，ingest_run 卡在 status=running、error_message 是 NULL，
// 稽核紀錄反而遺漏了最需要被看到的那次失敗。
func TestDailyJobRun_FinishIngestRunSurvivesExpiredCtx(t *testing.T) {
	restDay := &stubRestDayIngester{}
	price := &stubPriceIngester{}
	recorder := &stubRecorder{}
	job := newTestJob(t, restDay, price, recorder, zap.NewNop(), time.Now())

	expiredCtx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-expiredCtx.Done() // 確定真的過期，不靠時序運氣

	job.run(expiredCtx)

	if !recorder.finishCalled {
		t.Fatalf("即使外層 ctx 已過期，FinishIngestRun 仍應被呼叫——收尾寫入不能因為外層 ctx 死掉而被跳過")
	}
	if recorder.finishCtxErr != nil {
		t.Errorf("FinishIngestRun 收到的 ctx.Err() = %v，預期 nil"+
			"（代表 FinishIngestRun 沿用了外層已過期的 ctx，收尾寫入在真實 DB 上會連帶失敗）",
			recorder.finishCtxErr)
	}
}

// pgxutilDateValue 是 pgxutil.DateValue 的測試小幫手：斷言轉換一定成功
// （測試資料本身就是本檔案自己組出來的合法 pgtype.Date，不該失敗）。
func pgxutilDateValue(t *testing.T, d pgtype.Date) time.Time {
	t.Helper()
	got := pgxutil.DateValue(d)
	if got == nil {
		t.Fatalf("pgxutil.DateValue(%v) 回傳 nil，預期一個合法日期", d)
	}
	return *got
}

// TestRunWithTimeout_PanicPropagates 是遷移到 pkg/outbox.DailyScheduler 後的
// 行為變更的回歸測試：panic recovery 不再是 dailyJob.runWithTimeout 自己的
// 責任（那層防護搬去了 outbox.DailyScheduler.tick()，見 daily.go 的
// runWithTimeout 說明），所以現在直接呼叫 runWithTimeout（跳過
// DailyScheduler）時 panic 必須原樣傳播出去，不能被吞掉——這個測試故意用
// 自己的 recover 接住它，藉此斷言「這裡沒有 recover」這件事本身，而不是像
// 舊版 TestRunOnceWithRecover_SurvivesPanicAndLeavesRunRunning 那樣斷言
// panic 被吞掉（那個行為已經搬到 pkg/outbox/daily_scheduler_test.go 的
// TestDailyScheduler_PanicInRunIsRecoveredNotPropagated 去測）。
// price.called/recorder.finishCalled 兩個斷言則沿用舊測試：panic 發生在
// PriceIngest.Handle 之前，兩者都不該被呼叫到。
func TestRunWithTimeout_PanicPropagates(t *testing.T) {
	price := &stubPriceIngester{}
	recorder := &stubRecorder{}
	job := newTestJob(t, panicRestDayIngester{}, price, recorder, zap.NewNop(), time.Now())

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("runWithTimeout 應該讓 panic 原樣傳播出去——recovery 已經搬到 outbox.DailyScheduler.tick()，這裡不該再攔一次")
			}
		}()
		job.runWithTimeout(context.Background())
	}()

	if price.called {
		t.Errorf("休市日曆 panic 發生在 PriceIngest.Handle 之前，不應該還被呼叫到")
	}
	if recorder.finishCalled {
		t.Errorf("panic 後不應該還呼叫 FinishIngestRun——ingest_run 應維持 running，供人工排查")
	}
}

func TestRunWithTimeout_AppliesDeadline(t *testing.T) {
	price := &stubPriceIngester{}
	recorder := &stubRecorder{}
	job := newTestJob(t, &stubRestDayIngester{}, price, recorder, zap.NewNop(), time.Now())

	job.runWithTimeout(context.Background())

	if price.gotCtx == nil {
		t.Fatalf("PriceIngest.Handle 應該被呼叫過")
	}
	if _, ok := price.gotCtx.Deadline(); !ok {
		t.Errorf("runWithTimeout 應該替每次執行套上逾時，傳給 PriceIngest.Handle 的 ctx 卻沒有 deadline")
	}
}
