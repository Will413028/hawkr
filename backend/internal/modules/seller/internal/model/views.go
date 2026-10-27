package model

import (
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// 商品瀏覽數（設計書 28）：寫入端只追加原始事件，背景 job 解析店家並彙總成每家店每個台北日一列，總次數、不去重。

// ViewEventRetention 是已處理的原始瀏覽事件的保留期（依 occurred_at）；重算只涵蓋完整落在保留期內的台北日。
const ViewEventRetention = 7 * 24 * time.Hour

// 事件的處理結果（product_view_events.outcome）。
const (
	// ViewCounted 是已解析出店家並記進每日彙總。
	ViewCounted = "COUNTED"
	// ViewUnresolved 是商品不存在或沒有店家：不計入任何地方，留著除錯，不重試。
	ViewUnresolved = "UNRESOLVED"
)

// ViewRebuildRange 驗證重算的區間（台北日曆日，含兩端）：from 不晚於 to、to 不晚於今天，而且 from 那天的台北 00:00 不早於
// now − 保留期（更早的事件可能已被清理，重算會把彙總洗成殘缺）。回傳正規化成 UTC 午夜的日期。
func ViewRebuildRange(now, from, to time.Time) (time.Time, time.Time, error) {
	from, to = dateOnly(from), dateOnly(to)
	if from.After(to) {
		return time.Time{}, time.Time{}, apperr.InvalidInput("重算的起日不可晚於迄日")
	}
	if to.After(ViewDay(now)) {
		return time.Time{}, time.Time{}, apperr.InvalidInput("重算的迄日不可晚於今天")
	}
	earliest := now.Add(-ViewEventRetention)
	if time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, twtime.Taipei).Before(earliest) {
		return time.Time{}, time.Time{}, apperr.InvalidInput("重算的起日超出原始事件的保留期（7 天）")
	}
	return from, to, nil
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ViewDay 回傳 now 所在的台北日曆日。
func ViewDay(now time.Time) time.Time {
	return twtime.Day(now)
}

// ViewRange 驗證查詢的期間 [start, end)：兩端都必須是台北的 00:00（期間以整個日曆日為單位），start 早於 end。回傳對應的日曆日
// 範圍 [firstDay, endDay)，以 UTC 午夜表示（DATE 欄位的慣例）。
func ViewRange(start, end time.Time) (firstDay, endDay time.Time, err error) {
	for _, t := range []time.Time{start, end} {
		if local := t.In(twtime.Taipei); local.Hour() != 0 || local.Minute() != 0 || local.Second() != 0 || local.Nanosecond() != 0 {
			return time.Time{}, time.Time{}, apperr.InvalidInput("期間的兩端必須是台北的 00:00：%s", t.Format(time.RFC3339Nano))
		}
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, apperr.InvalidInput("期間的起點必須早於終點")
	}
	return twtime.Day(start), twtime.Day(end), nil
}
