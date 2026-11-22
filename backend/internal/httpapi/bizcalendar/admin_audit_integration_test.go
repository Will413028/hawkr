//go:build integration

package bizcalendar

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/adminauth"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/bizcalendar/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/bizcalendar/v1alpha1/bizcalendarv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// adminauth 宣告為 audited 的每一支 bizcalendar 後台 RPC，經真的 Connect handler 與真的資料庫呼叫一次，都恰好寫一筆後台紀錄
// （動作是 procedure 名稱、管理員是呼叫者、對象是年份）；匯入之後涵蓋查詢讀得到。
// mutation：Import 不寫後台紀錄 → 該 procedure 的紀錄是 0 筆，失敗；宣告表新增一支 audited 的 bizcalendar RPC 而沒有補情境 → 「沒有情境」失敗。
func TestEveryAuditedBizcalendarProcedureWritesOneRecordForTheCaller(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()},
		migrate.Module{Name: bizcalendar.Schema, Dir: bizcalendar.Migrations()},
	))
	audit := adminaudit.New(d, func() time.Time { return time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC) })
	path, handler := bizcalendarv1alpha1connect.NewBizCalendarAdminServiceHandler(NewAdmin(bizcalendar.New(d, audit, events.NewBus()), func() time.Time { return time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC) }), connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 77})))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := bizcalendarv1alpha1connect.NewBizCalendarAdminServiceClient(srv.Client(), srv.URL)
	ctx := context.Background()

	scenarios := map[string]func() (target string, err error){
		bizcalendarv1alpha1connect.BizCalendarAdminServiceImportHolidayCalendarProcedure: func() (string, error) {
			_, err := client.ImportHolidayCalendar(ctx, connect.NewRequest(&adminpb.ImportHolidayCalendarRequest{Year: 2026, CsvContent: fixture(t, "115.csv")}))
			return "2026", err
		},
	}
	for _, procedure := range adminauth.AuditedProcedures() {
		if !strings.HasPrefix(procedure, "/hawkr.admin.bizcalendar.") {
			continue
		}
		run, ok := scenarios[procedure]
		if !ok {
			t.Errorf("%s 宣告為 audited，但這裡沒有情境", procedure)
			continue
		}
		target, err := run()
		if err != nil {
			t.Errorf("%s：%v", procedure, err)
			continue
		}
		recs, err := audit.List(ctx, adminaudit.Filter{Action: procedure})
		if err != nil || len(recs) != 1 || recs[0].AdminID != 77 || recs[0].TargetType != bizcalendar.AuditTargetType || recs[0].TargetID != target {
			t.Errorf("%s：紀錄=%+v err=%v，預期恰好一筆（管理員 77、對象 %s/%s）", procedure, recs, err, bizcalendar.AuditTargetType, target)
		}
	}
	for procedure := range scenarios {
		if !strings.Contains(strings.Join(adminauth.AuditedProcedures(), ","), procedure) {
			t.Errorf("%s 有情境但宣告表沒有標成 audited", procedure)
		}
	}
	resp, err := client.GetHolidayCalendarCoverage(ctx, connect.NewRequest(&adminpb.GetHolidayCalendarCoverageRequest{}))
	if err != nil || len(resp.Msg.GetYears()) != 1 || resp.Msg.GetYears()[0].GetYear() != 2026 {
		t.Fatalf("coverage = %v err=%v", resp, err)
	}
}
