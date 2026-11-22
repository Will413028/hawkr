// Package bizcalendarrelay 把平台的工作日行事曆推給還在舊服務的 invoice-service（遷移計畫 10f-7 的 B#1、B#2）。資料的擁有者是
// platform/bizcalendar，invoice 的鏡像只是快取：後台匯入成功時 OnCalendarImported 在匯入的 transaction 內排入推送的 job，另有每 24 小時的
// 對帳 job（啟動時也跑一次）補齊推送失敗、invoice 重開與窗口隨日期前進的差異。推送由 worker 在 transaction 外呼叫 invoice-service。
//
// 推送的區間是窗口（今天前 30 天到後 400 天，台北日）與「包含今天的連續已匯入年份」的交集，連同區間內的放假日；今天所在的年份沒有匯入就
// 什麼都不推（對方保留舊鏡像）並記 warning，不是錯誤。每次都在送出前才讀行事曆，所以較晚讀到的資料不會被較早讀到的覆蓋。
//
// 隨 A5（invoice 搬進單體）刪除，同時刪 platform/bizcalendar 的 CalendarImported 事件型別與 worker 的 invoice client 接線。
package bizcalendarrelay

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// 推送給 invoice-service 的窗口：今天往前 30 天到往後 400 天（台北日）。窗口常數只由這裡擁有：invoice-service 只儲存收到的涵蓋區間，
// 不再有自己的窗口。
const (
	pastDays   = 30
	futureDays = 400
)

// reconcileInterval 是對帳推送的間隔；monitoring 的警報（business-alerts.yml）假設每天推送一次。
const reconcileInterval = 24 * time.Hour

// Client 是推送的對象（internal/legacy/invoiceservice）。from、to 是涵蓋區間（含兩端的台北日曆日），holidays 是區間內的放假日；
// 一次呼叫整段覆蓋對方的鏡像，失敗時對方保留舊鏡像。
type Client interface {
	ReplaceBusinessDayMirror(ctx context.Context, from, to time.Time, holidays []time.Time) error
}

// Loader 讀出目前的行事曆（*bizcalendar.Store）。
type Loader interface {
	Load(ctx context.Context) (*bizcalendar.Calendar, error)
}

// PushArgs 是一次推送。沒有參數：推送的內容永遠是執行當下的行事曆，所以尚未結束的同類 job 合併成一個。
type PushArgs struct{}

func (PushArgs) Kind() string { return "legacy.bizcalendar_push" }

func (PushArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(0) }

// OnCalendarImported 在匯入的 transaction 內排入推送：排入失敗時匯入一起回滾。
func OnCalendarImported(q jobs.Enqueuer) func(context.Context, bizcalendar.CalendarImported) error {
	return func(ctx context.Context, _ bizcalendar.CalendarImported) error {
		return q.Enqueue(ctx, PushArgs{}, nil)
	}
}

// Batch 算出要推送的批次：涵蓋區間是包含今天（台北日）的連續已匯入年份與窗口的交集，起日是窗口起日與該連續區段第一年 1 月 1 日的較晚者，
// 迄日是窗口迄日與區段最後一年 12 月 31 日的較早者。今天所在的年份沒有匯入就沒有任何涵蓋（ok = false，from 仍是窗口起日）。
// 中間缺一年時只推到缺口之前。holidays 是區間內的放假日，由早到晚。
func Batch(cal *bizcalendar.Calendar, now time.Time) (from, to time.Time, holidays []time.Time, ok bool) {
	today := twtime.Day(now)
	windowStart := today.AddDate(0, 0, -pastDays)
	windowEnd := today.AddDate(0, 0, futureDays)
	through, covered := cal.CoveredThrough(today)
	if !covered {
		return windowStart, time.Time{}, nil, false
	}
	first := today.Year()
	for cal.Covers(first - 1) {
		first--
	}
	from = windowStart
	if runStart := time.Date(first, time.January, 1, 0, 0, 0, 0, time.UTC); runStart.After(from) {
		from = runStart
	}
	to = windowEnd
	if through.Before(to) {
		to = through
	}
	return from, to, cal.HolidaysBetween(from, to), true
}

// Worker 執行推送的 job。
type Worker struct {
	store  Loader
	client Client
	now    func() time.Time
	log    *slog.Logger
}

// NewWorker 建立 Worker。
func NewWorker(store Loader, client Client, now func() time.Time, log *slog.Logger) *Worker {
	if store == nil || client == nil || now == nil || log == nil {
		panic("bizcalendarrelay: missing dependency")
	}
	return &Worker{store: store, client: client, now: now, log: log}
}

type pushWorker struct {
	river.WorkerDefaults[PushArgs]
	w *Worker
}

func (p *pushWorker) Work(ctx context.Context, _ *river.Job[PushArgs]) error { return p.w.Push(ctx) }

// AddWorkers 登記推送的 River worker。
func (w *Worker) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &pushWorker{w: w})
}

// PeriodicJobs 是每 24 小時（啟動時也跑一次）的對帳推送。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(reconcileInterval),
		func() (river.JobArgs, *river.InsertOpts) { return PushArgs{}, nil },
		&river.PeriodicJobOpts{ID: "legacy.bizcalendar_push", RunOnStart: true},
	)}
}

// Push 讀出目前的行事曆並推送一次。窗口內沒有涵蓋時什麼都不推並記 warning，不是錯誤。冪等：同一份資料重推結果相同。
func (w *Worker) Push(ctx context.Context) error {
	cal, err := w.store.Load(ctx)
	if err != nil {
		return err
	}
	from, to, holidays, ok := Batch(cal, w.now())
	if !ok {
		w.log.Warn("行事曆窗口內沒有已匯入的資料，不推送給 invoice-service", "window_start", from.Format(time.DateOnly))
		return nil
	}
	if err := w.client.ReplaceBusinessDayMirror(ctx, from, to, holidays); err != nil {
		return err
	}
	w.log.Info("行事曆已推送給 invoice-service", "from", from.Format(time.DateOnly), "to", to.Format(time.DateOnly), "holidays", len(holidays))
	return nil
}
