package bizcalendar

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// 推送給 invoice-service 的窗口：今天往前 30 天到往後 400 天（台北日）。數字是 invoice-service 的同步窗口（它是獨立 module，
// 不能 import 這裡，所以以值重複；隨 A5 一起刪除）。
const (
	mirrorPastDays   = 30
	mirrorFutureDays = 400
)

// MirrorClient 是推送的對象（遷移期的 invoice-service，internal/legacy/invoiceservice）。from、to 是涵蓋區間（含兩端的台北日曆日），
// holidays 是區間內的放假日；一次呼叫整段覆蓋對方的鏡像，失敗時對方保留舊鏡像。
type MirrorClient interface {
	ReplaceBusinessDayMirror(ctx context.Context, from, to time.Time, holidays []time.Time) error
}

// MirrorBatch 算出要推送的批次：窗口是 now 所在台北日的前 30 天到後 400 天，取與已匯入年份的交集，而且要從窗口起日連續：
// 起日所在的年份沒有匯入就沒有任何涵蓋（ok = false，from 仍是窗口起日）；有的話往後延伸到連續已匯入年份的 12 月 31 日，或窗口迄日，取較早者。
// 中間缺一年時只推到缺口之前，缺口之後不宣稱涵蓋。holidays 是區間內的放假日，由早到晚。
func MirrorBatch(cal *Calendar, now time.Time) (from, to time.Time, holidays []time.Time, ok bool) {
	today := twtime.Day(now)
	from = today.AddDate(0, 0, -mirrorPastDays)
	windowEnd := today.AddDate(0, 0, mirrorFutureDays)
	if !cal.Covers(from.Year()) {
		return from, time.Time{}, nil, false
	}
	year := from.Year()
	for cal.Covers(year+1) && time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC).Before(windowEnd) {
		year++
	}
	to = time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC)
	if windowEnd.Before(to) {
		to = windowEnd
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
}

// NewMirror 建立 Mirror。
func NewMirror(store Loader, client MirrorClient, now func() time.Time, log *slog.Logger) *Mirror {
	if store == nil || client == nil || now == nil || log == nil {
		panic("bizcalendar: missing mirror dependency")
	}
	return &Mirror{store: store, client: client, now: now, log: log}
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

// Run 啟動時推一次，之後每 every 對帳推一次，直到 ctx 取消才回傳。失敗只記 log，下一輪重試。
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
		}
	}
}

func (m *Mirror) pushLogged(ctx context.Context) {
	if err := m.Push(ctx); err != nil && !errors.Is(err, context.Canceled) {
		m.log.Error("行事曆推送給 invoice-service 失敗，下次對帳重試", "error", err)
	}
}
