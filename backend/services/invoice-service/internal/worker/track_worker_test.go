package worker

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
)

// TestIsTrackScanSlot 釘住 §7.5.3 的排程規格：
// 「每日 INVOICE_TRACK_BALANCE_CHECK_HOUR_UTC；單月 1–5 日改為每 4 小時」。
//
// 🔴 期初判定用**台北**日期。最重要的兩格是 UTC 月初的 00:00–08:00：那八小時在
// 台北已經是下個月 1 日，若用 UTC 判定會整段漏掉加頻，而錯誤每兩個月才出現一次。
func TestIsTrackScanSlot(t *testing.T) {
	const base = 1 // INVOICE_TRACK_BALANCE_CHECK_HOUR_UTC 的 dev/prod 值

	utc := func(s string) time.Time {
		ts, err := time.Parse("2006-01-02T15:04:05Z", s)
		if err != nil {
			t.Fatalf("bad fixture %q: %v", s, err)
		}
		return ts
	}

	cases := []struct {
		name string
		at   string
		want bool
	}{
		// 平時（雙月＝非期初）：只有 base 小時跑
		{"雙月 base 小時 → 跑", "2026-08-20T01:00:00Z", true},
		{"雙月 base+4 → 不跑（加頻只在期初）", "2026-08-20T05:00:00Z", false},
		{"雙月 非 base 小時 → 不跑", "2026-08-20T13:00:00Z", false},

		// 期初（單月 1–5 日，台北時間）：每 4 小時、相位對齊 base
		{"單月 3 日 base → 跑", "2026-09-03T01:00:00Z", true},
		{"單月 3 日 base+4 → 跑", "2026-09-03T05:00:00Z", true},
		{"單月 3 日 base+8 → 跑", "2026-09-03T09:00:00Z", true},
		{"單月 3 日 base+2 → 不跑（相位不對）", "2026-09-03T03:00:00Z", false},

		// 🔴 UTC 月底 16:00 之後 == 台北次月 1 日 ⇒ 期初已開始
		{"UTC 08-31T16:00 ＝ 台北 09-01 00:00 期初 → 應加頻", "2026-08-31T17:00:00Z", true},
		// 同一時刻若用 UTC 判定會是 8 月（雙月、非期初）＋ 非 base 小時 ⇒ 不跑。
		// 上一格為 true 就是「用台北判定」的證據。

		// 期初窗口的右邊界：台北 5 日仍在、6 日不在
		{"單月 5 日 base+4（台北）→ 跑", "2026-09-05T05:00:00Z", true},
		{"單月 6 日 base+4（台北）→ 不跑", "2026-09-06T05:00:00Z", false},
		{"單月 6 日 base（台北）→ 跑（回到每日）", "2026-09-06T01:00:00Z", true},
	}

	for _, c := range cases {
		at := utc(c.at)
		if got := isTrackScanSlot(at, base); got != c.want {
			lt := at.In(timeutil.LocationTaipei)
			t.Errorf("%s：isTrackScanSlot(%s, %d)=%v want %v（台北 %s）",
				c.name, c.at, base, got, c.want, lt.Format("2006-01-02 15:04"))
		}
	}
}

// TestIsTrackScanSlot_PeriodStartCoversEveryOddMonth 防「只驗了 9 月就以為對」。
//
// 單月＝奇數月（1/3/5/7/9/11），與 §4.5.10 的雙月制期別起點一致；偶數月不得加頻。
func TestIsTrackScanSlot_PeriodStartCoversEveryOddMonth(t *testing.T) {
	const base = 1
	for m := 1; m <= 12; m++ {
		// 台北 m 月 2 日 05:00 ＝ UTC m 月 1 日 21:00
		tp := time.Date(2026, time.Month(m), 2, 5, 0, 0, 0, timeutil.LocationTaipei)
		got := isTrackScanSlot(tp, base)
		want := m%2 == 1
		if got != want {
			t.Errorf("%d 月 2 日 05:00（台北）加頻=%v want %v", m, got, want)
		}
	}
}
