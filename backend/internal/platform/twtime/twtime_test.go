package twtime

import (
	"testing"
	"time"
)

func TestTaipeiIsUTCPlus8AndIsTheIANAZone(t *testing.T) {
	if Taipei.String() != "Asia/Taipei" {
		t.Fatalf("name=%s", Taipei)
	}
	at := time.Date(2026, 10, 3, 16, 0, 0, 0, time.UTC).In(Taipei)
	if y, m, d := at.Date(); y != 2026 || m != 10 || d != 4 || at.Hour() != 0 {
		t.Fatalf("at=%s", at)
	}
	// 歷史上台灣實施過日光節約時間（1979 年夏季），固定偏移做不到。
	summer := time.Date(1979, 7, 1, 12, 0, 0, 0, Taipei)
	if _, off := summer.Zone(); off != 9*3600 {
		t.Fatalf("1979-07-01 offset=%d", off)
	}
}

func TestMinuteCeilRoundsUpAnyRemainder(t *testing.T) {
	base := time.Date(2026, 10, 17, 15, 5, 0, 0, Taipei)
	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{"整分鐘不動", base, base},
		{"一奈秒進位", base.Add(time.Nanosecond), base.Add(time.Minute)},
		{"一秒進位", base.Add(time.Second), base.Add(time.Minute)},
		{"五十九秒進位", base.Add(59 * time.Second), base.Add(time.Minute)},
		{"跨日進位", time.Date(2026, 10, 17, 23, 59, 30, 0, Taipei), time.Date(2026, 10, 18, 0, 0, 0, 0, Taipei)},
	}
	for _, c := range cases {
		got := MinuteCeil(c.in)
		if !got.Equal(c.want) {
			t.Errorf("%s：MinuteCeil(%s) = %s，期望 %s", c.name, c.in, got, c.want)
		}
		if got.Location() != c.in.Location() {
			t.Errorf("%s：時區被改成 %s", c.name, got.Location())
		}
	}
}

func TestDayAndMidnightCutAtTaipeiMidnight(t *testing.T) {
	// UTC 15:59:59 仍是台北的前一天 23:59:59；UTC 16:00 是台北的隔天 00:00。
	before := time.Date(2026, 10, 19, 15, 59, 59, 0, time.UTC)
	after := before.Add(time.Second)
	if got, want := Day(before), time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("Day(before)=%s，期望 %s（UTC 午夜）", got, want)
	}
	if got, want := Day(after), time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("Day(after)=%s，期望 %s", got, want)
	}
	if got, want := Midnight(after), time.Date(2026, 10, 20, 0, 0, 0, 0, Taipei); !got.Equal(want) || got.Location() != Taipei {
		t.Fatalf("Midnight(after)=%s，期望 %s（台北）", got, want)
	}
	if got, want := Midnight(before), time.Date(2026, 10, 19, 0, 0, 0, 0, Taipei); !got.Equal(want) {
		t.Fatalf("Midnight(before)=%s，期望 %s", got, want)
	}
}
