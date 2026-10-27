package forecast

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Date is a calendar date with no time-of-day or timezone component,
// mirroring Python's datetime.date. PricePoint.trade_date and the
// calibration lineage dates in model.json are all calendar dates — using
// time.Time for them would smuggle a timezone into comparisons that must
// stay timezone-free.
type Date struct {
	Year  int
	Month int // 1-12
	Day   int
}

func NewDate(year int, month, day int) Date {
	return Date{Year: year, Month: month, Day: day}
}

func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, fmt.Errorf("forecast: invalid date %q: %w", s, err)
	}
	return Date{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}, nil
}

// DateFromTime 把只有日期部分有意義的 time.Time（例如 repository 回傳的
// trade_date）轉成 Date，捨棄時間部分。usecase.Forecast 與
// cmd/pricebook-feature-health 都需要這個轉換，原本各自複製一份
// （P4 review followup）。
func DateFromTime(t time.Time) Date {
	return NewDate(t.Year(), int(t.Month()), t.Day())
}

func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// ToTime 把 Date 轉成 UTC 午夜的 time.Time。固定 UTC：Date 本身沒有時區概念，
// 這只是為了餵給需要 time.Time 型別的呼叫端（例如 Postgres DATE 欄位參數），
// 任何時區選擇都不影響落在哪個日曆日。
func (d Date) ToTime() time.Time {
	return time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
}

// AddDays returns the date n calendar days later (n may be negative).
func (d Date) AddDays(n int) Date {
	t := d.ToTime().AddDate(0, 0, n)
	return Date{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}
}

func (d Date) Before(o Date) bool { return d.ToTime().Before(o.ToTime()) }
func (d Date) After(o Date) bool  { return d.ToTime().After(o.ToTime()) }

// PyWeekday returns the weekday using Python's date.weekday() convention:
// Monday=0 ... Sunday=6. Go's time.Weekday is Sunday=0 ... Saturday=6, so it
// must be rotated — features.py's target_weekday_sin/cos assumes Monday=0,
// and getting this rotation backwards silently shifts every weekday feature
// by one day without any error.
func (d Date) PyWeekday() int {
	return (int(d.ToTime().Weekday()) + 6) % 7
}

func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// locationCache memoizes time.LoadLocation results (map[string]*time.Location).
// Go does not cache non-UTC time.Location lookups internally — LoadLocation
// re-parses tzdata from disk every call (~29µs), and this package is called
// once per feature-vector/seasonal-naive computation, so at production scale
// (thousands of series × 7 horizons) that adds up to real, avoidable latency
// and filesystem I/O in an otherwise pure, zero-I/O package. sync.Map (rather
// than a plain map) because Go maps aren't safe for concurrent read+write and
// future callers of this package may not be single-goroutine.
var locationCache sync.Map

func cachedLoadLocation(name string) (*time.Location, error) {
	if v, ok := locationCache.Load(name); ok {
		return v.(*time.Location), nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, err
	}
	locationCache.Store(name, loc)
	return loc, nil
}

// OriginDateAndCutoff mirrors origin_date_and_cutoff (features.py:61-84) for
// the datetime-input path — the only path P3 exercises (forecast_seven_days
// always calls with a datetime origin, never a bare date; that variant, and
// the tz-naive-datetime rejection, are not ported — see plan's 範圍決策 #4).
// calendarTimezone must be an IANA zone name, e.g. "Asia/Taipei".
func OriginDateAndCutoff(origin time.Time, calendarTimezone string) (Date, time.Time, error) {
	loc, err := cachedLoadLocation(calendarTimezone)
	if err != nil {
		return Date{}, time.Time{}, fmt.Errorf("forecast: unknown calendar_timezone %q: %w", calendarTimezone, err)
	}
	local := origin.In(loc)
	originDate := Date{Year: local.Year(), Month: int(local.Month()), Day: local.Day()}
	return originDate, origin.UTC(), nil
}
