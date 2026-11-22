package bizcalendar

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// 推送給 invoice-service 的窗口：今天往前 30 天到往後 400 天（台北日）。窗口常數只由這裡擁有：invoice-service 只儲存收到的涵蓋區間，
// 不再有自己的窗口。
const (
	mirrorPastDays   = 30
	mirrorFutureDays = 400
)

// MirrorClient 是推送的對象（遷移期的 invoice-service，internal/legacy/invoiceservice）。from、to 是涵蓋區間（含兩端的台北日曆日），
// holidays 是區間內的放假日；一次呼叫整段覆蓋對方的鏡像，失敗時對方保留舊鏡像。
type MirrorClient interface {
	ReplaceBusinessDayMirror(ctx context.Context, from, to time.Time, holidays []time.Time) error
}

// MirrorBatch 算出要推送的批次：涵蓋區間是「包含今天（台北日）的連續已匯入年份」與窗口（今天前 30 天到後 400 天）的交集，
// 起日是窗口起日與該連續區段第一年 1 月 1 日的較晚者，迄日是窗口迄日與區段最後一年 12 月 31 日的較早者。
// 今天所在的年份沒有匯入就沒有任何涵蓋（ok = false，from 仍是窗口起日）。中間缺一年時只推到缺口之前。
// holidays 是區間內的放假日，由早到晚。
func MirrorBatch(cal *Calendar, now time.Time) (from, to time.Time, holidays []time.Time, ok bool) {
	today := twtime.Day(now)
	windowStart := today.AddDate(0, 0, -mirrorPastDays)
	windowEnd := today.AddDate(0, 0, mirrorFutureDays)
	if !cal.Covers(today.Year()) {
		return windowStart, time.Time{}, nil, false
	}
	first, last := today.Year(), today.Year()
	for cal.Covers(first - 1) {
		first--
	}
	for cal.Covers(last + 1) {
		last++
	}
	from = windowStart
	if runStart := time.Date(first, time.January, 1, 0, 0, 0, 0, time.UTC); runStart.After(from) {
		from = runStart
	}
	to = windowEnd
	if runEnd := time.Date(last, time.December, 31, 0, 0, 0, 0, time.UTC); runEnd.Before(to) {
		to = runEnd
	}
	return from, to, cal.HolidaysBetween(from, to), true
}

// Loader 讀出目前的行事曆（*Store）。
type Loader interface {
	Load(ctx context.Context) (*Calendar, error)
}

// Mirror 把行事曆推給 invoice-service：資料的擁有者是這裡，invoice 的鏡像只是快取。
type Mirror struct {
	store  Loader
	client MirrorClient
	now    func() time.Time
	log    *slog.Logger
	// trigger 讓其他 goroutine 要求立刻推送一次；容量 1，已有待處理的要求時合併。
	trigger chan struct{}
}

// NewMirror 建立 Mirror。
func NewMirror(store Loader, client MirrorClient, now func() time.Time, log *slog.Logger) *Mirror {
	if store == nil || client == nil || now == nil || log == nil {
		panic("bizcalendar: missing mirror dependency")
	}
	return &Mirror{store: store, client: client, now: now, log: log, trigger: make(chan struct{}, 1)}
}

// Trigger 要求 Run 的 goroutine 盡快推送一次，不等待也不阻塞；已有待處理的要求時合併成一次。
// 所有推送都在 Run 的 goroutine 裡串行執行，每次都在送出前才讀行事曆，所以較晚讀到的資料不會被較早讀到的覆蓋。
// 這個串行保證只在單一單體實例內成立：多個實例各有各的 Run，彼此的推送不會被排序。
func (m *Mirror) Trigger() {
	select {
	case m.trigger <- struct{}{}:
	default:
	}
}

// Push 讀出目前的行事曆並推送一次。窗口內沒有涵蓋時什麼都不推（對方保留舊鏡像）並記 warning，不是錯誤。冪等：同一份資料重推結果相同。
func (m *Mirror) Push(ctx context.Context) error {
	cal, err := m.store.Load(ctx)
	if err != nil {
		return err
	}
	from, to, holidays, ok := MirrorBatch(cal, m.now())
	if !ok {
		m.log.Warn("行事曆窗口內沒有已匯入的資料，不推送給 invoice-service", "window_start", from.Format(time.DateOnly))
		return nil
	}
	if err := m.client.ReplaceBusinessDayMirror(ctx, from, to, holidays); err != nil {
		return err
	}
	m.log.Info("行事曆已推送給 invoice-service", "from", from.Format(time.DateOnly), "to", to.Format(time.DateOnly), "holidays", len(holidays))
	return nil
}

// Run 啟動時推一次，之後每 every 對帳推一次，並在每次 Trigger 後推一次，直到 ctx 取消才回傳。失敗只記 log，下一輪重試。
func (m *Mirror) Run(ctx context.Context, every time.Duration) {
	m.pushLogged(ctx)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.pushLogged(ctx)
		case <-m.trigger:
			m.pushLogged(ctx)
		}
	}
}

func (m *Mirror) pushLogged(ctx context.Context) {
	if err := m.Push(ctx); err != nil && !errors.Is(err, context.Canceled) {
		m.log.Error("行事曆推送給 invoice-service 失敗，下次對帳重試", "error", err)
	}
}
