package domain

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
)

// 本檔守的是「3 個工作天」這條算式（§7.3.5）。它錯了不會有任何東西轉紅：
// 算偏早 ⇒ 賣家被誤判逾期 ⇒ 違規計數 +1 ⇒ 90 天 3 次即停權建議。

// tpe 是台北時區的整點；period_test.go 已有一個六參數的 taipei，不重複定義。
func tpe(y int, m time.Month, d, hh int) time.Time {
	return time.Date(y, m, d, hh, 0, 0, 0, timeutil.LocationTaipei)
}

// syncedNear 讓涵蓋窗口一定罩得住測試用的日期。
func syncedNear(t time.Time) *time.Time { v := t; return &v }

func TestComplaintSlaDueAtPlainWeek(t *testing.T) {
	// 2026-03-02 是週一。建案週一 10:00 ⇒ 起算週二 00:00 ⇒ 推 3 個工作天 ⇒ 週五 00:00。
	created := tpe(2026, 3, 2, 10)
	cal := NewBusinessCalendar(nil, syncedNear(created))

	got := cal.ComplaintSlaDueAt(created)
	want := tpe(2026, 3, 6, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("due = %s, want %s", got, want)
	}
}

func TestComplaintSlaDueAtSkipsWeekend(t *testing.T) {
	// 週四建案 ⇒ 起算週五 ⇒ 週一(1)、週二(2)、週三(3)。
	created := tpe(2026, 3, 5, 23)
	cal := NewBusinessCalendar(nil, syncedNear(created))

	got := cal.ComplaintSlaDueAt(created)
	want := tpe(2026, 3, 11, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("due = %s, want %s", got, want)
	}
}

func TestComplaintSlaDueAtSkipsHolidays(t *testing.T) {
	created := tpe(2026, 3, 2, 10)
	// 週二、週三放假 ⇒ 起算日順延到週四，再推週五(1)、下週一(2)、下週二(3)。
	cal := NewBusinessCalendar([]time.Time{
		tpe(2026, 3, 3, 0), tpe(2026, 3, 4, 0),
	}, syncedNear(created))

	got := cal.ComplaintSlaDueAt(created)
	want := tpe(2026, 3, 10, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("due = %s, want %s", got, want)
	}
}

// 🔴 未涵蓋的日期一律視為假日 ⇒ 落在硬上限（偏晚），**不得**退化成 3 自然日。
func TestComplaintSlaDueAtFallsBackToHardCapWhenMirrorEmpty(t *testing.T) {
	created := tpe(2026, 3, 2, 10)
	cal := NewBusinessCalendar(nil, nil) // 鏡像從未同步

	got := cal.ComplaintSlaDueAt(created)
	want := tpe(2026, 3, 16, 0).UTC() // 建案日 00:00 + 14 自然日
	if !got.Equal(want) {
		t.Fatalf("due = %s, want %s（硬上限）", got, want)
	}
	if natural := created.AddDate(0, 0, 3); !got.After(natural) {
		t.Fatalf("fallback 偏早了：due=%s 不晚於 3 自然日 %s", got, natural)
	}
}

// 台北日界：UTC 前八小時建立的申訴必須算進台北的當天，不是前一天。
func TestComplaintSlaDueAtUsesTaipeiDayBoundary(t *testing.T) {
	// 2026-03-03 01:00 台北 ＝ 2026-03-02 17:00 UTC。起算週三 ⇒ 週四(1)週五(2)下週一(3)。
	created := tpe(2026, 3, 3, 1)
	cal := NewBusinessCalendar(nil, syncedNear(created))

	got := cal.ComplaintSlaDueAt(created.UTC())
	want := tpe(2026, 3, 9, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("due = %s, want %s", got, want)
	}
}

func TestRemainingBusinessDays(t *testing.T) {
	due := tpe(2026, 3, 6, 0)
	cal := NewBusinessCalendar(nil, syncedNear(due))

	cases := []struct {
		name string
		now  time.Time
		want int32
	}{
		{"起算日當天剩三天", tpe(2026, 3, 3, 9), 3},
		{"到期前一個工作天", tpe(2026, 3, 5, 9), 1},
		{"到期日當天為零", tpe(2026, 3, 6, 9), 0},
		{"逾期一個工作天為負", tpe(2026, 3, 9, 9), -1},
		{"逾期跨週末只算工作天", tpe(2026, 3, 10, 9), -2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := cal.RemainingBusinessDays(tc.now, due)
			if !ok {
				t.Fatalf("ok=false，本區間應在涵蓋內")
			}
			if got != tc.want {
				t.Fatalf("remaining = %d, want %d", got, tc.want)
			}
		})
	}
}

// 涵蓋不到就明說算不出來——回 0 會被前端讀成「今天到期」。
func TestRemainingBusinessDaysUncoveredReturnsNotOK(t *testing.T) {
	cal := NewBusinessCalendar(nil, nil)
	if _, ok := cal.RemainingBusinessDays(tpe(2026, 3, 3, 9), tpe(2026, 3, 6, 0)); ok {
		t.Fatal("鏡像空表時應回 ok=false")
	}

	synced := tpe(2026, 3, 3, 9)
	covered := NewBusinessCalendar(nil, syncedNear(synced))
	// 超出 +400 天的窗口。
	if _, ok := covered.RemainingBusinessDays(synced, synced.AddDate(0, 0, 401)); ok {
		t.Fatal("超出同步窗口時應回 ok=false")
	}
}

// 兩套曆制不可互相代入：同一個起點，自然日與工作天必須給出不同答案。
func TestBusinessDaysAndNaturalDaysDiverge(t *testing.T) {
	created := tpe(2026, 3, 5, 10) // 週四
	cal := NewBusinessCalendar(nil, syncedNear(created))

	business := cal.ComplaintSlaDueAt(created)
	natural := taipeiMidnight(created).AddDate(0, 0, ComplaintSlaBusinessDays+1).UTC()
	if business.Equal(natural) {
		t.Fatal("工作天與自然日算出同一個答案，表示週末沒有被扣掉")
	}
}
