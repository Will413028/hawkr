package bizcalendar

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/bizcalendar/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/bizcalendar/v1alpha1/bizcalendarv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
)

const fixtureDir = "../../platform/bizcalendar/testdata/"

type fakeSource struct {
	actor adminaudit.Actor
	year  int
	cal   *bizcalendar.Calendar
}

func (f *fakeSource) Import(_ context.Context, actor adminaudit.Actor, year int, content string) (bizcalendar.ImportResult, error) {
	f.actor, f.year = actor, year
	days, err := bizcalendar.ParseOfficeCalendar(year, content)
	if err != nil {
		return bizcalendar.ImportResult{}, err
	}
	return bizcalendar.ImportResult{Year: year, DayCount: len(days), HolidayCount: 120, ReplacedDayCount: 3}, nil
}

func (f *fakeSource) Load(context.Context) (*bizcalendar.Calendar, error) { return f.cal, nil }

func newEnv(t *testing.T, src Source) bizcalendarv1alpha1connect.BizCalendarAdminServiceClient {
	t.Helper()
	return newEnvWith(t, NewAdmin(src, func() time.Time { return time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC) }))
}

func newEnvWith(t *testing.T, admin *Admin) bizcalendarv1alpha1connect.BizCalendarAdminServiceClient {
	t.Helper()
	path, handler := bizcalendarv1alpha1connect.NewBizCalendarAdminServiceHandler(admin, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 42})))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return bizcalendarv1alpha1connect.NewBizCalendarAdminServiceClient(srv.Client(), srv.URL)
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(fixtureDir + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestImportPassesTheCallerAndProcedureAndMapsTheResult(t *testing.T) {
	f := &fakeSource{}
	c := newEnv(t, f)
	resp, err := c.ImportHolidayCalendar(context.Background(), connect.NewRequest(&adminpb.ImportHolidayCalendarRequest{Year: 2026, CsvContent: fixture(t, "115.csv")}))
	if err != nil {
		t.Fatal(err)
	}
	if f.actor != (adminaudit.Actor{AdminID: 42, Action: bizcalendarv1alpha1connect.BizCalendarAdminServiceImportHolidayCalendarProcedure}) || f.year != 2026 {
		t.Errorf("actor=%+v year=%d", f.actor, f.year)
	}
	m := resp.Msg
	if m.GetYear().GetYear() != 2026 || m.GetYear().GetDayCount() != 365 || m.GetYear().GetHolidayCount() != 120 || m.GetReplacedDayCount() != 3 {
		t.Errorf("resp = %v", m)
	}
}

func TestImportParseErrorsPointAtTheRequestFields(t *testing.T) {
	c := newEnv(t, &fakeSource{})
	for name, tc := range map[string]struct {
		req   *adminpb.ImportHolidayCalendarRequest
		field string
	}{
		"csv":  {&adminpb.ImportHolidayCalendarRequest{Year: 2026, CsvContent: "bad"}, "csv_content"},
		"year": {&adminpb.ImportHolidayCalendarRequest{Year: 0, CsvContent: "bad"}, "year"},
	} {
		_, err := c.ImportHolidayCalendar(context.Background(), connect.NewRequest(tc.req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%s：code = %v", name, connect.CodeOf(err))
		}
		var ce *connect.Error
		if !errors.As(err, &ce) {
			t.Fatalf("%s：不是 connect 錯誤", name)
		}
		found := false
		for _, d := range ce.Details() {
			v, verr := d.Value()
			if verr != nil {
				t.Fatal(verr)
			}
			if br, ok := v.(*errdetails.BadRequest); ok {
				for _, fv := range br.GetFieldViolations() {
					found = found || fv.GetField() == tc.field
				}
			}
		}
		if !found {
			t.Errorf("%s：BadRequest 沒有指到 %s", name, tc.field)
		}
	}
}

func calendarOfYears(t *testing.T, files map[int]string) *bizcalendar.Calendar {
	t.Helper()
	var days []bizcalendar.Day
	for year, file := range files {
		parsed, err := bizcalendar.ParseOfficeCalendar(year, fixture(t, file))
		if err != nil {
			t.Fatal(err)
		}
		days = append(days, parsed...)
	}
	cal, err := bizcalendar.NewCalendar(days)
	if err != nil {
		t.Fatal(err)
	}
	return cal
}

func coverage(t *testing.T, cal *bizcalendar.Calendar) *adminpb.GetHolidayCalendarCoverageResponse {
	t.Helper()
	c := newEnv(t, &fakeSource{cal: cal})
	resp, err := c.GetHolidayCalendarCoverage(context.Background(), connect.NewRequest(&adminpb.GetHolidayCalendarCoverageRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg
}

// 今天固定在台北 2026-11-22。
func TestCoverageMapsYearsAndCoveredThrough(t *testing.T) {
	msg := coverage(t, calendarOfYears(t, map[int]string{2026: "115.csv"}))
	if ys := msg.GetYears(); len(ys) != 1 || ys[0].GetYear() != 2026 || ys[0].GetDayCount() != 365 || ys[0].GetHolidayCount() != 120 {
		t.Errorf("years = %v", ys)
	}
	if d := msg.GetCoveredThrough(); d.GetYear() != 2026 || d.GetMonth() != 12 || d.GetDay() != 31 {
		t.Errorf("covered_through = %v", d)
	}
}

func TestCoveredThroughRunsThroughContiguousYears(t *testing.T) {
	msg := coverage(t, calendarOfYears(t, map[int]string{2026: "115.csv", 2027: "116.csv"}))
	if d := msg.GetCoveredThrough(); d.GetYear() != 2027 || d.GetMonth() != 12 || d.GetDay() != 31 {
		t.Errorf("covered_through = %v", d)
	}
}

func TestCoveredThroughStopsBeforeAGapYear(t *testing.T) {
	days := []bizcalendar.Day{}
	for day := time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC); day.Year() == 2028; day = day.AddDate(0, 0, 1) {
		days = append(days, bizcalendar.Day{Date: day})
	}
	first, err := bizcalendar.ParseOfficeCalendar(2026, fixture(t, "115.csv"))
	if err != nil {
		t.Fatal(err)
	}
	withGap, err := bizcalendar.NewCalendar(append(first, days...))
	if err != nil {
		t.Fatal(err)
	}
	if d := coverage(t, withGap).GetCoveredThrough(); d.GetYear() != 2026 || d.GetMonth() != 12 || d.GetDay() != 31 {
		t.Errorf("2027 沒有匯入，涵蓋應停在 2026-12-31：%v", d)
	}
}

// 今天所在的年份沒有匯入就是沒有涵蓋（不設），即使之後的年份已匯入。
func TestCoveredThroughIsUnsetWhenTodaysYearIsNotImported(t *testing.T) {
	msg := coverage(t, calendarOfYears(t, map[int]string{2027: "116.csv"}))
	if len(msg.GetYears()) != 1 || msg.GetCoveredThrough() != nil {
		t.Fatalf("resp = %v", msg)
	}
}

func TestCoverageOfAnEmptyCalendarHasNoCoveredThrough(t *testing.T) {
	cal, _ := bizcalendar.NewCalendar(nil)
	msg := coverage(t, cal)
	if len(msg.GetYears()) != 0 || msg.GetCoveredThrough() != nil {
		t.Fatalf("resp=%v", msg)
	}
}
