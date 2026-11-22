package bizcalendarrelay

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
)

func d(s string) time.Time {
	t, err := time.ParseInLocation(time.DateOnly, s, time.UTC)
	if err != nil {
		panic(err)
	}
	return t
}

func at(s string) time.Time { return d(s).Add(4 * time.Hour) } // 台北中午前，與 UTC 同一天

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../../platform/bizcalendar/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func calendarOf(t *testing.T, years ...int) *bizcalendar.Calendar {
	t.Helper()
	var days []bizcalendar.Day
	for _, y := range years {
		file := map[int]string{2026: "115.csv", 2027: "116.csv"}[y]
		if file == "" {
			t.Fatalf("沒有 %d 年的 fixture", y)
		}
		parsed, err := bizcalendar.ParseOfficeCalendar(y, readFixture(t, file))
		if err != nil {
			t.Fatal(err)
		}
		days = append(days, parsed...)
	}
	c, err := bizcalendar.NewCalendar(days)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// 期望值全部硬寫日期，不呼叫被測的常數：自己對自己會讓改壞的窗口照樣全綠。
func TestBatchIsWindowIntersectImportedYearsContiguousFromStart(t *testing.T) {
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
		// 窗口起日在前一年且前一年也已匯入 ⇒ 往回延伸到窗口起日，不停在今天所在年的 1 月 1 日。
		{"window start year imported too", []int{2026, 2027}, "2027-01-10", true, "2026-12-11", "2027-12-31", ""},
		// 今天的年份沒有匯入 ⇒ 沒有涵蓋，即使窗口後段落在已匯入的年份。
		{"today's year not imported", []int{2027}, "2026-11-22", false, "2026-10-23", "", ""},
		{"nothing imported", nil, "2026-11-22", false, "2026-10-23", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cal *bizcalendar.Calendar
			if len(tc.years) == 0 {
				cal = &bizcalendar.Calendar{}
			} else {
				cal = calendarOf(t, tc.years...)
			}
			from, to, holidays, ok := Batch(cal, at(tc.now))
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

// 中間缺一年時只推到缺口之前（2026、2027、2030 已匯入的情形以 2026、2027 加缺口的日曆表示）。
func TestBatchStopsAtTheFirstGap(t *testing.T) {
	days := []bizcalendar.Day{}
	for _, y := range []int{2026, 2027} {
		parsed, err := bizcalendar.ParseOfficeCalendar(y, readFixture(t, map[int]string{2026: "115.csv", 2027: "116.csv"}[y]))
		if err != nil {
			t.Fatal(err)
		}
		days = append(days, parsed...)
	}
	for day := d("2030-01-01"); day.Year() == 2030; day = day.AddDate(0, 0, 1) {
		days = append(days, bizcalendar.Day{Date: day})
	}
	cal, err := bizcalendar.NewCalendar(days)
	if err != nil {
		t.Fatal(err)
	}
	_, to, _, ok := Batch(cal, at("2027-06-01"))
	if !ok || to.Format(time.DateOnly) != "2027-12-31" {
		t.Fatalf("2028、2029 沒有匯入，涵蓋應停在 2027-12-31：ok=%v to=%s", ok, to.Format(time.DateOnly))
	}
}

type fakeLoader struct{ cal *bizcalendar.Calendar }

func (f fakeLoader) Load(context.Context) (*bizcalendar.Calendar, error) { return f.cal, nil }

type pushed struct {
	from, to string
	holidays int
}

type fakeClient struct {
	calls []pushed
	err   error
}

func (f *fakeClient) ReplaceBusinessDayMirror(_ context.Context, from, to time.Time, holidays []time.Time) error {
	f.calls = append(f.calls, pushed{from.Format(time.DateOnly), to.Format(time.DateOnly), len(holidays)})
	return f.err
}

func newTestWorker(cal *bizcalendar.Calendar, c Client) *Worker {
	now := at("2026-11-22")
	return NewWorker(fakeLoader{cal}, c, func() time.Time { return now }, slog.New(slog.DiscardHandler))
}

func TestPushSendsTheBatchAndIsIdempotent(t *testing.T) {
	c := &fakeClient{}
	w := newTestWorker(calendarOf(t, 2026), c)
	for range 2 {
		if err := w.Push(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.calls) != 2 || c.calls[0] != c.calls[1] || c.calls[0].from != "2026-10-23" || c.calls[0].to != "2026-12-31" || c.calls[0].holidays == 0 {
		t.Fatalf("calls = %+v", c.calls)
	}
}

func TestPushWithoutCoverageSendsNothing(t *testing.T) {
	c := &fakeClient{}
	if err := newTestWorker(&bizcalendar.Calendar{}, c).Push(context.Background()); err != nil {
		t.Fatalf("沒有涵蓋不是錯誤（只記 warning）：%v", err)
	}
	if len(c.calls) != 0 {
		t.Fatalf("窗口內沒有涵蓋時不得推送：%+v", c.calls)
	}
}

func TestPushReturnsClientErrorSoRiverRetries(t *testing.T) {
	boom := errors.New("boom")
	if err := newTestWorker(calendarOf(t, 2026), &fakeClient{err: boom}).Push(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

type recordingJobs struct {
	args []river.JobArgs
	opts []*river.InsertOpts
}

func (q *recordingJobs) Enqueue(_ context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	q.args = append(q.args, args)
	q.opts = append(q.opts, opts)
	return nil
}

type failingJobs struct{ err error }

func (q failingJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return q.err }

func TestOnCalendarImportedEnqueuesOnePush(t *testing.T) {
	q := &recordingJobs{}
	if err := OnCalendarImported(q)(context.Background(), bizcalendar.CalendarImported{Year: 2026}); err != nil {
		t.Fatal(err)
	}
	if len(q.args) != 1 || !reflect.DeepEqual(q.args[0], PushArgs{}) {
		t.Fatalf("args = %+v", q.args)
	}
	if len(q.opts) != 1 || q.opts[0] != nil {
		t.Fatalf("opts = %+v，要是 nil：套用 PushArgs.InsertOpts（MaxAttempts 10、不去重）", q.opts)
	}
}

// 排入失敗要回傳錯誤：匯入的 transaction 因此回滾，不會留下沒有推送的匯入。
func TestOnCalendarImportedReturnsEnqueueError(t *testing.T) {
	boom := errors.New("boom")
	if err := OnCalendarImported(failingJobs{boom})(context.Background(), bizcalendar.CalendarImported{Year: 2026}); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

// 不設唯一鍵：執行中的 job 已讀了舊行事曆，此時排入的新推送不能被合併掉。失敗的 job 在一天內放棄，由每天的對帳重新排入。
func TestPushArgsHaveNoUniquenessAndBoundedAttempts(t *testing.T) {
	opts := PushArgs{}.InsertOpts()
	if opts.UniqueOpts.ByArgs || opts.UniqueOpts.ByPeriod != 0 || len(opts.UniqueOpts.ByState) != 0 {
		t.Fatalf("unique opts = %+v", opts.UniqueOpts)
	}
	if opts.MaxAttempts != 10 {
		t.Fatalf("MaxAttempts = %d", opts.MaxAttempts)
	}
}

// seqLoader 依序回傳行事曆；用完後一直回傳最後一個。
type seqLoader struct {
	cals []*bizcalendar.Calendar
	n    int
}

func (l *seqLoader) Load(context.Context) (*bizcalendar.Calendar, error) {
	c := l.cals[min(l.n, len(l.cals)-1)]
	l.n++
	return c, nil
}

func TestPushRepushesWhenTheCalendarChangedDuringThePush(t *testing.T) {
	c := &fakeClient{}
	now := at("2026-11-22")
	l := &seqLoader{cals: []*bizcalendar.Calendar{calendarOf(t, 2026), calendarOf(t, 2026, 2027)}}
	w := NewWorker(l, c, func() time.Time { return now }, slog.New(slog.DiscardHandler))
	if err := w.Push(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(c.calls) != 2 || c.calls[0].to != "2026-12-31" || c.calls[1].to != "2027-12-27" {
		t.Fatalf("calls = %+v", c.calls)
	}
}

func TestPushWithAnUnchangedCalendarCallsTheClientOnce(t *testing.T) {
	c := &fakeClient{}
	if err := newTestWorker(calendarOf(t, 2026), c).Push(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(c.calls) != 1 {
		t.Fatalf("calls = %+v", c.calls)
	}
}

// 每次讀到的都不同：有限輪後回傳錯誤，讓 River 重試。
func TestPushFailsWhenTheCalendarNeverStabilises(t *testing.T) {
	c := &fakeClient{}
	now := at("2026-11-22")
	a, b := calendarOf(t, 2026), calendarOf(t, 2026, 2027)
	l := &seqLoader{cals: []*bizcalendar.Calendar{a, b, a, b, a, b}}
	w := NewWorker(l, c, func() time.Time { return now }, slog.New(slog.DiscardHandler))
	if err := w.Push(context.Background()); err == nil {
		t.Fatal("要回傳錯誤")
	}
	if len(c.calls) != 3 {
		t.Fatalf("推送次數 = %d，要 3", len(c.calls))
	}
}

func TestPushLogsTheFailureSentenceTheAlertSearchesFor(t *testing.T) {
	var buf bytes.Buffer
	now := at("2026-11-22")
	w := NewWorker(fakeLoader{calendarOf(t, 2026)}, &fakeClient{err: errors.New("boom")}, func() time.Time { return now }, slog.New(slog.NewTextHandler(&buf, nil)))
	if err := w.Push(context.Background()); err == nil {
		t.Fatal("要回傳錯誤")
	}
	if !strings.Contains(buf.String(), "level=ERROR") || !strings.Contains(buf.String(), "行事曆推送給 invoice-service 失敗") {
		t.Fatalf("log = %s", buf.String())
	}
}

func TestPeriodicConfiguration(t *testing.T) {
	periodicJobs := PeriodicJobs()
	if len(periodicJobs) != 1 {
		t.Fatalf("periodic jobs=%d", len(periodicJobs))
	}
	// River 沒有公開 PeriodicJob 的設定；這裡驗證建構時用的排程、選項與參數。
	t0 := at("2026-11-22")
	if next := periodicSchedule().Next(t0); !next.Equal(t0.Add(24 * time.Hour)) {
		t.Fatalf("next = %s", next)
	}
	if opts := periodicOpts(); !opts.RunOnStart || opts.ID != "legacy.bizcalendar_push" {
		t.Fatalf("opts = %+v", opts)
	}
	args, insertOpts := periodicArgs()
	if !reflect.DeepEqual(args, PushArgs{}) {
		t.Fatalf("args = %+v", args)
	}
	if insertOpts != nil {
		t.Fatalf("insert opts = %+v，要是 nil：套用 PushArgs.InsertOpts（MaxAttempts 10、不去重）", insertOpts)
	}
}
