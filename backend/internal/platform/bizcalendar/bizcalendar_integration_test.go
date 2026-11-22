//go:build integration

package bizcalendar_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type fixture struct {
	db    *db.DB
	store *bizcalendar.Store
	audit *adminaudit.Log
	bus   *events.Bus
	seen  *[]bizcalendar.CalendarImported
}

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()},
		migrate.Module{Name: bizcalendar.Schema, Dir: bizcalendar.Migrations()},
	))
	log := adminaudit.New(d, func() time.Time { return time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC) })
	bus := events.NewBus()
	seen := &[]bizcalendar.CalendarImported{}
	events.Subscribe(bus, func(_ context.Context, e bizcalendar.CalendarImported) error {
		*seen = append(*seen, e)
		return nil
	})
	return &fixture{db: d, store: bizcalendar.New(d, log, bus), audit: log, bus: bus, seen: seen}
}

func fixtureCSV(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var admin = adminaudit.Actor{AdminID: 7, Action: "/hawkr.admin.bizcalendar.v1alpha1.BizCalendarAdminService/ImportHolidayCalendar"}

func (f *fixture) count(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT count(*) FROM bizcalendar.days`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// 匯入寫滿全年、第一次匯入沒有被取代的天數，涵蓋看得到這一年，並寫一筆後台紀錄。
// mutation：Import 不呼叫 audit.Record → 紀錄筆數為 0，失敗。
func TestImportWritesTheWholeYearAndAnAuditRecord(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	res, err := f.store.Import(ctx, admin, 2026, fixtureCSV(t, "115.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if res != (bizcalendar.ImportResult{Year: 2026, DayCount: 365, HolidayCount: 120, ReplacedDayCount: 0}) {
		t.Fatalf("res = %+v", res)
	}
	cal, err := f.store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cov := cal.Coverage(); len(cov) != 1 || cov[0] != (bizcalendar.YearCoverage{Year: 2026, DayCount: 365, HolidayCount: 120}) {
		t.Fatalf("coverage = %+v", cov)
	}
	if b, ok := cal.IsBusinessDay(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)); !ok || !b {
		t.Errorf("2026-01-02 business=%v ok=%v", b, ok)
	}
	recs, err := f.audit.List(ctx, adminaudit.Filter{TargetType: bizcalendar.AuditTargetType})
	if err != nil || len(recs) != 1 || recs[0].TargetID != "2026" || recs[0].AdminID != 7 || recs[0].Action != admin.Action {
		t.Fatalf("紀錄 = %+v err=%v", recs, err)
	}
}

// 重匯同一年是整年取代：舊的備註與旗標被新內容覆蓋，天數不增加，回報被取代的天數。
// mutation：upsert 的 DO UPDATE 只更新 note 不更新 holiday → 旗標沒變，失敗。
func TestReimportReplacesTheYear(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	if _, err := f.store.Import(ctx, admin, 2026, fixtureCSV(t, "115.csv")); err != nil {
		t.Fatal(err)
	}
	// 2026-01-02（週五）改成放假、備註改成「更正」。
	edited := strings.Replace(fixtureCSV(t, "115.csv"), "20260102,五,0,", "20260102,五,2,更正", 1)
	res, err := f.store.Import(ctx, admin, 2026, edited)
	if err != nil {
		t.Fatal(err)
	}
	if res.ReplacedDayCount != 365 || res.HolidayCount != 121 {
		t.Fatalf("res = %+v", res)
	}
	if n := f.count(t); n != 365 {
		t.Fatalf("天數 = %d", n)
	}
	cal, _ := f.store.Load(ctx)
	if b, ok := cal.IsBusinessDay(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)); !ok || b {
		t.Errorf("更正後 2026-01-02 business=%v ok=%v", b, ok)
	}
	var note string
	if err := f.db.Pool().QueryRow(ctx, `SELECT note FROM bizcalendar.days WHERE day = '2026-01-02'`).Scan(&note); err != nil || note != "更正" {
		t.Errorf("note = %q err=%v", note, err)
	}
}

// 兩年各自涵蓋；沒有匯入的年份不在涵蓋裡，也不影響別年。
func TestImportingAnotherYearKeepsTheFirst(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for year, file := range map[int]string{2026: "115.csv", 2027: "116.csv"} {
		if _, err := f.store.Import(ctx, admin, year, fixtureCSV(t, file)); err != nil {
			t.Fatal(err)
		}
	}
	cal, err := f.store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := cal.Years(); len(got) != 2 || got[0] != 2026 || got[1] != 2027 {
		t.Fatalf("years = %v", got)
	}
	if through, ok := cal.CoveredThrough(time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)); !ok || !through.Equal(time.Date(2027, 12, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("through = %v", through)
	}
	if _, ok := cal.AddBusinessDays(time.Date(2027, 12, 30, 0, 0, 0, 0, time.UTC), 1); ok {
		t.Error("數到 2028 要算不出來")
	}
}

// 解析失敗的匯入不寫任何資料也沒有紀錄；已匯入的年份不受影響。
// mutation：Import 在解析前就刪掉該年資料 → 原有資料消失，失敗。
func TestRejectedImportChangesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	if _, err := f.store.Import(ctx, admin, 2026, fixtureCSV(t, "115.csv")); err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(fixtureCSV(t, "115.csv"), "20260102,五,0,", "20260102,五,1,", 1)
	_, err := f.store.Import(ctx, admin, 2026, bad)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
		t.Fatalf("err = %v", err)
	}
	if n := f.count(t); n != 365 {
		t.Fatalf("天數 = %d", n)
	}
	if recs, _ := f.audit.List(ctx, adminaudit.Filter{TargetType: bizcalendar.AuditTargetType}); len(recs) != 1 {
		t.Fatalf("紀錄 = %d 筆，只有第一次匯入的一筆", len(recs))
	}
}

// 後台紀錄寫不進去（管理員 ID 不合）時，資料跟著 rollback。
// mutation：把 audit.Record 移到 InTx 之外 → 資料已寫入，失敗。
func TestImportRollsBackWhenTheAuditRecordFails(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, err := f.store.Import(context.Background(), adminaudit.Actor{AdminID: 0, Action: admin.Action}, 2026, fixtureCSV(t, "115.csv"))
	if err == nil {
		t.Fatal("要失敗")
	}
	if n := f.count(t); n != 0 {
		t.Fatalf("天數 = %d，要 0", n)
	}
}

// 沒有匯入任何年份時 Load 回空行事曆，所有查詢都算不出來。
func TestEmptyStoreCoversNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	cal, err := f.store.Load(context.Background())
	if err != nil || len(cal.Years()) != 0 {
		t.Fatalf("err=%v years=%v", err, cal.Years())
	}
	if _, ok := cal.IsBusinessDay(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)); ok {
		t.Error("沒有涵蓋")
	}
}

// DB 的 CHECK 是後備：備註頭尾有空白寫不進去。年份範圍由解析檢查，資料表沒有 year 欄。
func TestSchemaRejectsPaddedNote(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	if _, err := f.db.Pool().Exec(ctx, `INSERT INTO bizcalendar.days (day, holiday, note) VALUES ('2026-01-01', true, ' x')`); err == nil {
		t.Error("note 頭有空白要被擋下")
	}
}

func TestImportPublishesCalendarImported(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	if _, err := f.store.Import(ctx, admin, 2026, fixtureCSV(t, "115.csv")); err != nil {
		t.Fatal(err)
	}
	if len(*f.seen) != 1 || (*f.seen)[0].Year != 2026 {
		t.Fatalf("events = %+v", *f.seen)
	}
	if _, err := f.store.Import(ctx, admin, 2026, "bad"); err == nil {
		t.Fatal("要被拒絕")
	}
	if len(*f.seen) != 1 {
		t.Fatalf("解析失敗不得發布事件：%+v", *f.seen)
	}
}

func TestImportRollsBackWhenASubscriberFails(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	events.Subscribe(f.bus, func(context.Context, bizcalendar.CalendarImported) error { return errors.New("boom") })
	if _, err := f.store.Import(ctx, admin, 2026, fixtureCSV(t, "115.csv")); err == nil {
		t.Fatal("訂閱者失敗時匯入要失敗")
	}
	cal, err := f.store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cal.Years()) != 0 {
		t.Fatalf("匯入要整個回滾：%v", cal.Years())
	}
}

// 表沒有 year 欄、檢查與索引：年份範圍由解析檢查。
func TestSchemaHasNoYearColumn(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var columns, constraints, indexes int
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'bizcalendar' AND table_name = 'days' AND column_name = 'year'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conname IN ('days_year_check', 'days_year_day_check')`).Scan(&constraints); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname = 'bizcalendar' AND indexname = 'days_by_year'`).Scan(&indexes); err != nil {
		t.Fatal(err)
	}
	if columns != 0 || constraints != 0 || indexes != 0 {
		t.Fatalf("year 欄 %d、檢查 %d、索引 %d 都應為 0", columns, constraints, indexes)
	}
}
