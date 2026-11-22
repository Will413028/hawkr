package bizcalendar

import (
	"fmt"
	"slices"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// Day 是行事曆的一天。Date 是日曆日（只看年月日，以 UTC 午夜表示，與 DATE 欄位在 Go 端的形狀相同）；Holiday 是辦公日曆表的「是否放假」，
// 這是唯一決定一天是不是工作天的欄位，星期幾不參與：補班的週六 Holiday 為 false，就是工作天。
type Day struct {
	Date    time.Time
	Holiday bool
	Note    string
}

// YearCoverage 是一個已匯入年份的涵蓋狀態。
type YearCoverage struct {
	Year         int
	DayCount     int
	HolidayCount int
}

// Calendar 是某一時刻已匯入資料的唯讀快照，全部是純函式，不碰資料庫與網路。
//
// 涵蓋以年為單位：一年的資料整年都在才算涵蓋（NewCalendar 檢查），所以「某一天有沒有被涵蓋」只看它的年份。需要的日子落在沒有涵蓋的年份時，
// 回報 ok=false 而不是替呼叫者猜：平台沒有週六日的預設、沒有天數上限，偏早或偏晚由呼叫者依自己的義務決定。零值的 Calendar 沒有涵蓋任何年份。
// 日界是台北：傳入的時刻先換成它所屬的台北日曆日。
type Calendar struct {
	days  map[int64]Day
	years []int
}

// NewCalendar 由資料列組出行事曆。同一天出現兩次、或某個年份沒有整年每一天各一列都回錯誤：那是資料不一致，不是輸入錯誤。
func NewCalendar(days []Day) (*Calendar, error) {
	c := &Calendar{days: make(map[int64]Day, len(days))}
	perYear := map[int]int{}
	for _, d := range days {
		date := civil(d.Date)
		key := dayNumber(date)
		if _, dup := c.days[key]; dup {
			return nil, fmt.Errorf("bizcalendar：%s 出現兩次", date.Format(time.DateOnly))
		}
		c.days[key] = Day{Date: date, Holiday: d.Holiday, Note: d.Note}
		perYear[date.Year()]++
	}
	for y, n := range perYear {
		if want := daysInYear(y); n != want {
			return nil, fmt.Errorf("bizcalendar：%d 年有 %d 天，整年應有 %d 天", y, n, want)
		}
		c.years = append(c.years, y)
	}
	slices.Sort(c.years)
	return c, nil
}

// Years 回傳已涵蓋的年份，由小到大。
func (c *Calendar) Years() []int { return slices.Clone(c.years) }

// Covers 回報 year 這一整年是否已涵蓋。
func (c *Calendar) Covers(year int) bool {
	_, ok := slices.BinarySearch(c.years, year)
	return ok
}

// Coverage 回傳每個已涵蓋年份的天數與放假天數，年份由小到大。
func (c *Calendar) Coverage() []YearCoverage {
	out := make([]YearCoverage, len(c.years))
	idx := make(map[int]int, len(c.years))
	for i, y := range c.years {
		out[i].Year = y
		idx[y] = i
	}
	for _, d := range c.days {
		i := idx[d.Date.Year()]
		out[i].DayCount++
		if d.Holiday {
			out[i].HolidayCount++
		}
	}
	return out
}

// CoveredThrough 回傳從 from 的台北日曆日起連續涵蓋到的最後一天（以 UTC 午夜的日曆日表示）：from 所在的年份沒有涵蓋時 ok 為 false，
// 否則是從該年起連續已涵蓋的最後一年的 12 月 31 日。中間缺一年時停在缺口之前。
func (c *Calendar) CoveredThrough(from time.Time) (time.Time, bool) {
	year := twtime.Day(from).Year()
	if !c.Covers(year) {
		return time.Time{}, false
	}
	for c.Covers(year + 1) {
		year++
	}
	return time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC), true
}

// IsBusinessDay 回報 t 所在的台北日曆日是不是工作天。那一天的年份沒有涵蓋時 ok 為 false。
func (c *Calendar) IsBusinessDay(t time.Time) (business, ok bool) {
	day, ok := c.lookup(twtime.Day(t))
	if !ok {
		return false, false
	}
	return !day.Holiday, true
}

// AddBusinessDays 回傳 from 的台北日曆日之後的第 n 個工作天（以 UTC 午夜的日曆日表示）：從 from 的次日開始數，from 本身不算，所以 n = 1 是 from 之後的
// 第一個工作天。n = 0 回傳 from 所在的日曆日本身；n 為負數、或數的過程中遇到沒有涵蓋的年份，ok 為 false。from 本身不需要在涵蓋的年份內：結果只取決於被數到的日子。
func (c *Calendar) AddBusinessDays(from time.Time, n int) (time.Time, bool) {
	if n < 0 {
		return time.Time{}, false
	}
	cursor := twtime.Day(from)
	for counted := 0; counted < n; {
		cursor = cursor.AddDate(0, 0, 1)
		day, ok := c.lookup(cursor)
		if !ok {
			return time.Time{}, false
		}
		if !day.Holiday {
			counted++
		}
	}
	return cursor, true
}

// BusinessDaysBetween 回傳 from 與 to 之間的工作天數（台北日曆日）：to 在 from 之後時是 (from, to] 的工作天數，to 在 from 之前時是 (to, from] 的工作天數取負，
// 同一天為 0。區間內有任何一天落在沒有涵蓋的年份，ok 為 false（0 與「算不出來」不同，不能混為一談）。
func (c *Calendar) BusinessDaysBetween(from, to time.Time) (int, bool) {
	a, b := twtime.Day(from), twtime.Day(to)
	sign := 1
	if b.Before(a) {
		a, b = b, a
		sign = -1
	}
	n := 0
	for cursor := a.AddDate(0, 0, 1); !cursor.After(b); cursor = cursor.AddDate(0, 0, 1) {
		day, ok := c.lookup(cursor)
		if !ok {
			return 0, false
		}
		if !day.Holiday {
			n++
		}
	}
	return sign * n, true
}

// HolidaysBetween 回傳 [from, to]（台北日曆日，含兩端）內已涵蓋的放假日，由早到晚，以 UTC 午夜的日曆日表示。沒有涵蓋的日子不在結果裡：
// 結果是否完整要對照 Years。
func (c *Calendar) HolidaysBetween(from, to time.Time) []time.Time {
	lo, hi := dayNumber(twtime.Day(from)), dayNumber(twtime.Day(to))
	var out []time.Time
	for key, d := range c.days {
		if d.Holiday && key >= lo && key <= hi {
			out = append(out, d.Date)
		}
	}
	slices.SortFunc(out, func(a, b time.Time) int { return a.Compare(b) })
	return out
}

func (c *Calendar) lookup(day time.Time) (Day, bool) {
	if !c.Covers(day.Year()) {
		return Day{}, false
	}
	d, ok := c.days[dayNumber(day)]
	return d, ok
}

// civil 取 t 自己的年月日（不換時區），以 UTC 午夜表示。
func civil(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func dayNumber(utcMidnight time.Time) int64 { return utcMidnight.Unix() / 86400 }

func daysInYear(y int) int {
	return int(time.Date(y+1, time.January, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(y, time.January, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24)
}
