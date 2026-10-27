package timeutil

import (
	"testing"
	"time"
)

func TestLocationTaipei(t *testing.T) {
	if LocationTaipei == nil {
		t.Fatal("LocationTaipei is nil")
	}
	got := time.Date(2026, 3, 15, 0, 0, 0, 0, LocationTaipei)
	_, offset := got.Zone()
	if offset != 8*60*60 {
		t.Fatalf("want UTC+8 offset, got %d seconds", offset)
	}
}

func TestDate_TruncatesTimeOfDay(t *testing.T) {
	in := time.Date(2026, 3, 15, 13, 45, 30, 0, time.UTC)
	want := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	if got := Date(in); !got.Equal(want) {
		t.Fatalf("Date(%v) = %v, want %v", in, got, want)
	}
}

func TestEqualDate(t *testing.T) {
	a := time.Date(2026, 3, 15, 1, 0, 0, 0, time.UTC)
	b := time.Date(2026, 3, 15, 23, 0, 0, 0, time.UTC)
	c := time.Date(2026, 3, 16, 0, 0, 1, 0, time.UTC)
	if !EqualDate(a, b) {
		t.Fatalf("expected %v and %v to be the same date", a, b)
	}
	if EqualDate(a, c) {
		t.Fatalf("expected %v and %v to be different dates", a, c)
	}
}

func TestDefaultRangeDate(t *testing.T) {
	cases := []struct {
		name    string
		daysAgo int
		want    int // inclusive day span, e.g. 1 = today only
	}{
		{"today only", 1, 1},
		{"zero treated as 1", 0, 1},
		{"negative treated as 1", -3, 1},
		{"seven day range", 7, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from, to := DefaultRangeDate(tc.daysAgo, time.UTC)
			if !to.Equal(Date(time.Now().In(time.UTC))) {
				t.Fatalf("to = %v, want today", to)
			}
			gotSpanDays := int(to.Sub(from).Hours()/24) + 1
			if gotSpanDays != tc.want {
				t.Fatalf("span = %d days, want %d (from=%v to=%v)", gotSpanDays, tc.want, from, to)
			}
		})
	}
}
