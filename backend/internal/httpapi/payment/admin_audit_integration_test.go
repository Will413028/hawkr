//go:build integration

package payment

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/httpapi/adminauth"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1/paymentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type discardQueue struct{}

func (discardQueue) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }

type activeOwners struct{}

func (activeOwners) OwnerActive(context.Context, int64) (bool, error) { return true, nil }
func (activeOwners) OwnerContactEmail(context.Context, int64) (string, bool, error) {
	return "seller@example.com", true, nil
}

// adminauth 宣告為 audited 的每一支 payment 後台 RPC，經真的 Connect handler 與真的資料庫呼叫一次，都恰好寫一筆後台紀錄（動作是它的 procedure 名稱、
// 管理員是呼叫者、對象是店家）；宣告了 audited 卻沒有這裡的情境、或情境沒有寫紀錄，都失敗。
// mutation：把任一個 use case 的 s.audit 呼叫拿掉 → 該 procedure 的紀錄是 0 筆，失敗；宣告表新增一支 audited 的 payment RPC 而沒有補情境 → 「沒有情境」失敗。
func TestEveryAuditedPaymentProcedureWritesOneRecordForTheCaller(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: inbox.Schema, Dir: inbox.Migrations()},
		migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()},
		migrate.Module{Name: "payment", Dir: payment.Migrations()},
	))
	now := time.Date(2026, 11, 14, 4, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	log := slog.New(slog.DiscardHandler)
	audit := adminaudit.New(d, clock)
	m := payment.New(payment.Deps{
		DB: d, Events: events.NewBus(), Jobs: discardQueue{}, Inbox: inbox.New(inbox.Deps{DB: d, Jobs: discardQueue{}, Now: clock, Logger: log}),
		Audit: audit, Provider: payment.NewFake(), Owners: activeOwners{}, Now: clock, Logger: log,
	})
	path, handler := paymentv1alpha1connect.NewPaymentAdminServiceHandler(NewAdmin(m))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 77})))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := paymentv1alpha1connect.NewPaymentAdminServiceClient(srv.Client(), srv.URL)
	ctx := context.Background()

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := d.Conn(ctx).Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	const insertRow = `INSERT INTO payment.accounts (store_id, provider, requirement_collection, created_at, updated_at, %s) VALUES ($1, 'fake', 'PROVIDER', $2, $2, $3)`
	failedRow := func(store int64) {
		exec(strings.Replace(insertRow, "%s", "create_failed_at", 1), store, now, now)
	}
	blockedRow := func(store int64) {
		exec(`INSERT INTO payment.accounts (store_id, provider, requirement_collection, created_at, updated_at, purge_pending_at, purge_blocked_at, purge_blocked_reason)
VALUES ($1, 'fake', 'PROVIDER', $2, $2, $2, $2, 'provider_refused：no')`, store, now)
	}
	scenarios := map[string]func(store int64) error{
		paymentv1alpha1connect.PaymentAdminServiceRetryPaymentAccountCreateProcedure: func(store int64) error {
			failedRow(store)
			_, err := client.RetryPaymentAccountCreate(ctx, connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: rpc.FormatID(store)}))
			return err
		},
		paymentv1alpha1connect.PaymentAdminServiceRetryPaymentAccountPurgeProcedure: func(store int64) error {
			blockedRow(store)
			_, err := client.RetryPaymentAccountPurge(ctx, connect.NewRequest(&adminpb.RetryPaymentAccountPurgeRequest{StoreId: rpc.FormatID(store)}))
			return err
		},
		paymentv1alpha1connect.PaymentAdminServiceDeletePaymentAccountMappingProcedure: func(store int64) error {
			blockedRow(store)
			_, err := client.DeletePaymentAccountMapping(ctx, connect.NewRequest(&adminpb.DeletePaymentAccountMappingRequest{StoreId: rpc.FormatID(store)}))
			return err
		},
	}
	store := int64(500)
	for _, procedure := range adminauth.AuditedProcedures() {
		if !strings.HasPrefix(procedure, "/hawkr.admin.payment.") {
			continue
		}
		run, ok := scenarios[procedure]
		if !ok {
			t.Errorf("%s 宣告為 audited，但這裡沒有情境", procedure)
			continue
		}
		store++
		if err := run(store); err != nil {
			t.Errorf("%s：%v", procedure, err)
			continue
		}
		recs, err := audit.List(ctx, adminaudit.Filter{Action: procedure})
		if err != nil || len(recs) != 1 || recs[0].AdminID != 77 || recs[0].TargetType != "payment_account" || recs[0].TargetID != rpc.FormatID(store) {
			t.Errorf("%s：紀錄=%+v err=%v，預期恰好一筆（管理員 77、對象 payment_account/%d）", procedure, recs, err, store)
		}
	}
	for procedure := range scenarios {
		if !strings.Contains(strings.Join(adminauth.AuditedProcedures(), ","), procedure) {
			t.Errorf("%s 有情境但宣告表沒有標成 audited", procedure)
		}
	}
}
