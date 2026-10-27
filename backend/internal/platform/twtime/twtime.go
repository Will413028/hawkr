// Package twtime 提供台灣的時區。日曆日、週界線與期間（儀表板、統計、去重窗）一律用 Taipei 切，所有模組共用這一份。
//
// 用 IANA 的 Asia/Taipei，不用固定偏移（FixedZone）：歷史日期（台灣曾實施日光節約時間）才會正確。時區資料以
// time/tzdata 內嵌，不依賴執行環境的 zoneinfo。
package twtime

import (
	"time"
	_ "time/tzdata" // 內嵌時區資料，容器沒有 zoneinfo 也能載入
)

// Taipei 是 Asia/Taipei。
var Taipei = mustLoad("Asia/Taipei")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("twtime：無法載入時區 " + name + "：" + err.Error())
	}
	return loc
}

// MinuteCeil 把 t 進位到整分鐘：秒或奈秒有餘數時進位到下一分鐘，已經是整分鐘就原樣回傳。給「幾點幾分之後可以再做」這類句子用，
// 顯示的時間不會早於實際可以做的時間。
func MinuteCeil(t time.Time) time.Time {
	floor := t.Truncate(time.Minute)
	if floor.Equal(t) {
		return t
	}
	return floor.Add(time.Minute)
}

// Day 是 t 在台北的日曆日，以 UTC 午夜的 time.Time 表示（DATE 欄位在 Go 端的形狀：sqlc 把 DATE 對到 UTC 午夜）。「台北的哪一天」只在這裡算。
func Day(t time.Time) time.Time {
	y, m, d := t.In(Taipei).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Midnight 是 t 所在的台北日曆日的台北 00:00（時刻，不是日期值）：以台北日曆日為界的期間（儀表板、統計）用它。
func Midnight(t time.Time) time.Time {
	y, m, d := t.In(Taipei).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, Taipei)
}
