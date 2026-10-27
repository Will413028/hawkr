package timeutil

import "time"

// LocationTaipei is the shared Asia/Taipei *time.Location, loaded once from
// the IANA tzdata at package init. Taiwan does not currently observe DST, so
// a fixed UTC+8 offset would behave the same today — but tzdata (not a
// hand-picked fixed offset) is the correct source of truth if that ever
// changes, and it's what the rest of this codebase's ad-hoc
// time.LoadLocation("Asia/Taipei") call sites already do.
var LocationTaipei = mustLoadLocation("Asia/Taipei")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("timeutil: load location " + name + ": " + err.Error())
	}
	return loc
}

// Date truncates t to midnight in t's own location, discarding the
// time-of-day component.
func Date(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// EqualDate reports whether t1 and t2 fall on the same calendar date in
// their respective locations.
func EqualDate(t1, t2 time.Time) bool {
	return Date(t1).Equal(Date(t2))
}

// DefaultRangeDate returns the calendar-date range [from, to] covering today
// and the (daysAgo-1) days before it, evaluated in loc. daysAgo < 1 is
// treated as 1 (today only).
func DefaultRangeDate(daysAgo int, loc *time.Location) (from, to time.Time) {
	if daysAgo < 1 {
		daysAgo = 1
	}
	to = Date(time.Now().In(loc))
	from = to.AddDate(0, 0, -daysAgo+1)
	return from, to
}
