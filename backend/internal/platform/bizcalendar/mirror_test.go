package bizcalendar

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func calendarOf(t *testing.T, years ...int) *Calendar {
	t.Helper()
	var days []Day
	for _, y := range years {
		switch y {
		case 2026:
			days = append(days, mustParse(t, 2026, readFixture(t, "115.csv"))...)
		case 2027:
			days = append(days, mustParse(t, 2027, readFixture(t, "116.csv"))...)
		default:
			t.Fatalf("沒有 %d 年的 fixture", y)
		}
	}
	c, err := NewCalendar(days)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func at(s string) time.Time { return d(s).Add(4 * time.Hour) } // 台北中午前，與 UTC 同一天

// 期望值全部硬寫日期，不呼叫被測的常數：自己對自己會讓改壞的窗口照樣全綠。
func TestMirrorBatchIsWindowIntersectImportedYearsContiguousFromStart(t *testing.T) {
	tests := []struct {
		name       string
		years      []int
		now        string
		ok         bool
		from, to   string
		wantFirstH string // 區間內第一個假日；空字串 = 不檢查
	}{
		// 2026-11-22 −30 天 ＝ 2026-10-23；+400 天 ＝ 2027-12-27；兩年都匯入 ⇒ 取窗口迄日。
		{"both years cover the whole window", []int{2026, 2027}, "2026-11-22", true, "2026-10-23", "2027-12-27", "2026-10-24"},
		// 只匯入 2026：涵蓋只到年底，不宣稱 2027 已涵蓋。
		{"only the first year imported", []int{2026}, "2026-11-22", true, "2026-10-23", "2026-12-31", "2026-10-24"},
		// 今天在 2027（匯入的第二年），2028 沒有 ⇒ 只到 2027-12-31。
		{"today in the last imported year", []int{2026, 2027}, "2027-12-20", true, "2027-11-20", "2027-12-31", ""},
		// 窗口起日在前一年（沒匯入），今天的年份有 ⇒ 從今天所在連續區段的 1 月 1 日起。
		{"window start year not imported but today's is", []int{2027}, "2027-01-10", true, "2027-01-01", "2027-12-31", ""},
		// 今天的年份沒有匯入 ⇒ 沒有涵蓋，即使窗口後段落在已匯入的年份。
		{"today's year not imported", []int{2027}, "2026-11-22", false, "2026-10-23", "", ""},
		{"nothing imported", nil, "2026-11-22", false, "2026-10-23", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cal *Calendar
			if len(tc.years) == 0 {
				cal = &Calendar{}
			} else {
				cal = calendarOf(t, tc.years...)
			}
			from, to, holidays, ok := MirrorBatch(cal, at(tc.now))
			if ok != tc.ok || from.Format(time.DateOnly) != tc.from {
				t.Fatalf("ok=%v from=%s，want ok=%v from=%s", ok, from.Format(time.DateOnly), tc.ok, tc.from)
			}
			if !ok {
				if len(holidays) != 0 {
					t.Fatalf("沒有涵蓋時不得帶假日：%v", holidays)
				}
				return
			}
			if to.Format(time.DateOnly) != tc.to {
				t.Fatalf("to=%s，want %s", to.Format(time.DateOnly), tc.to)
			}
			for _, h := range holidays {
				if h.Before(from) || h.After(to) {
					t.Fatalf("假日 %s 落在區間外", h.Format(time.DateOnly))
				}
			}
			if tc.wantFirstH != "" && (len(holidays) == 0 || holidays[0].Format(time.DateOnly) != tc.wantFirstH) {
				t.Fatalf("第一個假日 = %v，want %s", holidays, tc.wantFirstH)
			}
		})
	}
}

// 中間缺一年時只推到缺口之前。
func TestMirrorBatchStopsAtTheFirstGap(t *testing.T) {
	cal := testCalendar(t) // 2026、2027、2030
	_, to, _, ok := MirrorBatch(cal, at("2027-06-01"))
	if !ok || to.Format(time.DateOnly) != "2027-12-31" {
		t.Fatalf("2028、2029 沒有匯入，涵蓋應停在 2027-12-31：ok=%v to=%s", ok, to.Format(time.DateOnly))
	}
}

type fakeLoader struct{ cal *Calendar }

func (f fakeLoader) Load(context.Context) (*Calendar, error) { return f.cal, nil }

type pushed struct {
	from, to string
	holidays int
}

type fakeMirrorClient struct {
	mu    sync.Mutex
	calls []pushed
	err   error
}

func (f *fakeMirrorClient) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeMirrorClient) ReplaceBusinessDayMirror(_ context.Context, from, to time.Time, holidays []time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, pushed{from.Format(time.DateOnly), to.Format(time.DateOnly), len(holidays)})
	return f.err
}

func newTestMirror(cal *Calendar, c MirrorClient) *Mirror {
	now := at("2026-11-22")
	return NewMirror(fakeLoader{cal}, c, func() time.Time { return now }, slog.New(slog.DiscardHandler))
}

func TestMirrorPushSendsTheBatchAndIsIdempotent(t *testing.T) {
	c := &fakeMirrorClient{}
	m := newTestMirror(calendarOf(t, 2026), c)
	for range 2 {
		if err := m.Push(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.calls) != 2 || c.calls[0] != c.calls[1] || c.calls[0].from != "2026-10-23" || c.calls[0].to != "2026-12-31" || c.calls[0].holidays == 0 {
		t.Fatalf("calls = %+v", c.calls)
	}
}

func TestMirrorPushWithoutCoverageSendsNothing(t *testing.T) {
	c := &fakeMirrorClient{}
	if err := newTestMirror(&Calendar{}, c).Push(context.Background()); err != nil {
		t.Fatalf("沒有涵蓋不是錯誤（只記 warning）：%v", err)
	}
	if len(c.calls) != 0 {
		t.Fatalf("窗口內沒有涵蓋時不得推送：%+v", c.calls)
	}
}

func TestMirrorPushReturnsClientError(t *testing.T) {
	boom := errors.New("boom")
	if err := newTestMirror(calendarOf(t, 2026), &fakeMirrorClient{err: boom}).Push(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

// Run 啟動時先推一次、ctx 取消時結束；失敗只記 log 不結束。
func TestMirrorRunPushesAtStartAndStopsOnCancel(t *testing.T) {
	c := &fakeMirrorClient{err: errors.New("down")}
	m := newTestMirror(calendarOf(t, 2026), c)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx, time.Hour); close(done) }()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("啟動時沒有推送")
		default:
		}
		if c.count() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ctx 取消後 Run 沒有結束")
	}
}

type gatedClient struct {
	mu      sync.Mutex
	active  int
	maxSeen int
	calls   int
	release chan struct{}
	entered chan struct{}
}

func (g *gatedClient) ReplaceBusinessDayMirror(context.Context, time.Time, time.Time, []time.Time) error {
	g.mu.Lock()
	g.active++
	g.calls++
	if g.active > g.maxSeen {
		g.maxSeen = g.active
	}
	n := g.calls
	g.mu.Unlock()
	if n == 1 {
		g.entered <- struct{}{}
		<-g.release
	}
	g.mu.Lock()
	g.active--
	g.mu.Unlock()
	return nil
}

func TestMirrorTriggerSerializesPushesAndCoalescesPendingRequests(t *testing.T) {
	g := &gatedClient{release: make(chan struct{}), entered: make(chan struct{}, 1)}
	m := newTestMirror(calendarOf(t, 2026), g)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx, time.Hour); close(done) }()
	<-g.entered // 啟動時的推送卡在送出
	for range 5 {
		m.Trigger() // 不得阻塞；五次合併成一次待處理
	}
	close(g.release)
	deadline := time.After(5 * time.Second)
	for {
		g.mu.Lock()
		n := g.calls
		g.mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("Trigger 後沒有再推送")
		case <-time.After(5 * time.Millisecond):
		}
	}
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.maxSeen != 1 || g.calls != 2 {
		t.Fatalf("maxConcurrent=%d calls=%d，want 1 與 2", g.maxSeen, g.calls)
	}
}
