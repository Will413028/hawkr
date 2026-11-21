package bizcalendar

import (
	"slices"
	"testing"
	"time"
)

func TestImportColumnsAscendByDay(t *testing.T) {
	d := func(day int, holiday bool, note string) Day {
		return Day{Date: time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC), Holiday: holiday, Note: note}
	}
	in := []Day{d(3, true, "c"), d(1, false, "a"), d(2, true, "b")}
	dates, holidays, notes := importColumns(in)
	if !slices.IsSortedFunc(dates, func(a, b time.Time) int { return a.Compare(b) }) || len(dates) != 3 {
		t.Fatalf("dates not ascending: %v", dates)
	}
	if !slices.Equal(holidays, []bool{false, true, true}) || !slices.Equal(notes, []string{"a", "b", "c"}) {
		t.Fatalf("columns not aligned with dates: %v %v", holidays, notes)
	}
	if in[0].Note != "c" {
		t.Fatal("input mutated")
	}
}
