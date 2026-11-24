//go:build integration

package paymentserver

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/riverqueue/river"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/payment"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/payment/paymentconnect"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type jobLog struct {
	mu    sync.Mutex
	kinds []string
}

func (j *jobLog) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.kinds = append(j.kinds, args.Kind())
	return nil
}

func (j *jobLog) count(kind string) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	n := 0
	for _, k := range j.kinds {
		if k == kind {
			n++
		}
	}
	return n
}

type orders map[string]payment.PaymentOrder

func (o orders) OrderForPayment(_ context.Context, no string) (payment.PaymentOrder, error) {
	order, ok := o[no]
	if !ok {
		return payment.PaymentOrder{}, apperr.NotFound("沒有這張訂單")
	}
	return order, nil
}

type owners struct{}

func (owners) OwnerActive(context.Context, int64) (bool, error) { return true, nil }

func (owners) OwnerContactEmail(context.Context, int64) (string, bool, error) {
	return "owner@example.com", true, nil
}

type fixture struct {
	db     *db.DB
	jobs   *jobLog
	fake   *payment.ChargeFake
	module *payment.Module
	client paymentconnect.PaymentServiceClient
	now    time.Time
}

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: inbox.Schema, Dir: inbox.Migrations()}, migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()}, migrate.Module{Name: "payment", Dir: payment.Migrations()}))
	f := &fixture{db: d, jobs: &jobLog{}, fake: payment.NewChargeFake(), now: time.Date(2026, 11, 24, 4, 0, 0, 0, time.UTC)}
	clock := func() time.Time { return f.now }
	log := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	f.module = payment.New(payment.Deps{
		DB: d, Events: events.NewBus(), Jobs: f.jobs, Inbox: inbox.New(inbox.Deps{DB: d, Jobs: f.jobs, Now: clock, Logger: log}), Audit: adminaudit.New(d, clock),
		Owners: owners{}, Charge: f.fake, Now: clock, Logger: log,
		PaymentOrders: orders{
			"O-PAID": {OrderNo: "O-PAID", BuyerID: 11, StoreID: 7, Amount: 500, Payable: true, Description: "商品"},
			"O-OPEN": {OrderNo: "O-OPEN", BuyerID: 12, StoreID: 7, Amount: 300, Payable: true, Description: "商品"},
		},
	})
	srv := httptest.NewUnstartedServer(nil)
	_, handler := paymentconnect.NewPaymentServiceHandler(New(f.module))
	srv.Config.Handler = handler
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, ForceAttemptHTTP2: true}} //nolint:gosec // 測試用的自簽憑證
	f.client = paymentconnect.NewPaymentServiceClient(client, srv.URL, connect.WithGRPC())
	return f
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) { w.t.Log(string(p)); return len(p), nil }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) count(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	ctx := context.Background()
	var n int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, sql, args...).Scan(&n))
	return n
}

// paid 讓 O-PAID 入帳（金額 500，店 7）並回傳它的付款 ID。
func (f *fixture) paid(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	p, err := f.module.CreatePayment(ctx, 11, []string{"O-PAID"})
	must(t, err)
	var external string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT external_payment_id FROM payment.payments WHERE id = (SELECT payment_id FROM payment.payment_orders WHERE order_no = 'O-PAID')").Scan(&external))
	must(t, f.fake.Settle(external, payment.PaymentStateSucceeded, f.now))
	_, err = f.module.SyncPayment(ctx, p.ID)
	must(t, err)
}

func (f *fixture) open(t *testing.T) {
	t.Helper()
	_, err := f.module.CreatePayment(context.Background(), 12, []string{"O-OPEN"})
	must(t, err)
}

func (f *fixture) enableSeller(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO payment.accounts (store_id, provider, external_account_id, requirement_collection, status, transfers_active, create_called_at, create_completed_at, created_at, updated_at)
VALUES (7, $1, 'acct_7', 'PLATFORM', 'ENABLED', true, now(), now(), now(), now())`, payment.ChargeFakeName)
	must(t, err)
}

func TestReadsServeTheOldContractFromThePaymentRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.paid(t)
	f.open(t)

	active, err := f.client.GetActivePaymentStatusByOrderNo(ctx, connect.NewRequest(&pb.GetActivePaymentStatusByOrderNoRequest{OrderNo: "O-PAID"}))
	must(t, err)
	d := active.Msg.GetData()
	if active.Msg.GetCode() != 0 || d.GetStatus() != "1" || d.GetTradeNo() != "O-PAID" || d.GetBuyerId() != "11" || d.GetAmount() != "500" || d.GetPayTime() == "" {
		t.Fatalf("paid order = %v", active.Msg)
	}
	open, err := f.client.GetActivePaymentStatusByOrderNo(ctx, connect.NewRequest(&pb.GetActivePaymentStatusByOrderNoRequest{OrderNo: "O-OPEN"}))
	must(t, err)
	if open.Msg.GetData().GetStatus() != "0" {
		t.Fatalf("open order = %v", open.Msg)
	}
	none, err := f.client.GetActivePaymentStatusByOrderNo(ctx, connect.NewRequest(&pb.GetActivePaymentStatusByOrderNoRequest{OrderNo: "O-NONE"}))
	must(t, err)
	if none.Msg.GetCode() != 0 || none.Msg.GetData() != nil {
		t.Fatalf("no payment = %v", none.Msg)
	}

	batch, err := f.client.BatchGetProviderStatusByOrderIds(ctx, connect.NewRequest(&pb.BatchGetProviderStatusByOrderIdsRequest{OrderIds: []string{"O-PAID", "O-OPEN", "O-NONE"}}))
	must(t, err)
	if e := batch.Msg.GetEntries(); len(e) != 2 || e["O-PAID"].GetProviderStatus() != "SUCCEEDED" || e["O-OPEN"].GetProviderStatus() != "AWAITING_PAYMENT" {
		t.Fatalf("batch = %v", batch.Msg)
	}

	facts, err := f.client.GetPaymentFactsInternal(ctx, connect.NewRequest(&pb.GetPaymentFactsInternalRequest{OrderNo: "O-PAID"}))
	must(t, err)
	fd := facts.Msg.GetData()
	if facts.Msg.GetCode() != 0 || fd.GetTradeAmt() != "500" || fd.GetTradeNo() != "O-PAID" || fd.ProviderPaidAt == nil || fd.GetPayTime() != "2026-11-24T04:00:00Z" {
		t.Fatalf("facts = %v", facts.Msg)
	}
	for _, orderNo := range []string{"O-OPEN", "O-NONE"} {
		miss, err := f.client.GetPaymentFactsInternal(ctx, connect.NewRequest(&pb.GetPaymentFactsInternalRequest{OrderNo: orderNo}))
		must(t, err)
		if miss.Msg.GetCode() != 8015 || miss.Msg.GetData() != nil {
			t.Fatalf("%s facts = %v, want the not-found business code", orderNo, miss.Msg)
		}
	}
}

func TestGrantPaymentWritesOneTransferRowAndQueuesItsJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.paid(t)

	// 賣家收款帳戶還沒開通：gRPC 錯誤（order 的 worker 重試），不是業務碼，也不寫轉帳列。
	_, err := f.client.GrantPayment(ctx, connect.NewRequest(&pb.GrantPaymentRequest{OrderNo: "O-PAID"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || f.count(t, "SELECT count(*) FROM payment.transfers") != 0 {
		t.Fatalf("before the account is enabled: err=%v transfers=%d", err, f.count(t, "SELECT count(*) FROM payment.transfers"))
	}
	f.enableSeller(t)
	for range 2 {
		resp, err := f.client.GrantPayment(ctx, connect.NewRequest(&pb.GrantPaymentRequest{OrderNo: "O-PAID"}))
		must(t, err)
		if resp.Msg.GetCode() != 0 || resp.Msg.GetGrantOutcome() != "GRANTED" || resp.Msg.GetData().GetAmount() != "500" || resp.Msg.GetData().GetTradeNo() != "O-PAID" {
			t.Fatalf("grant = %v", resp.Msg)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM payment.transfers WHERE amount = 500 AND status = 'PENDING'"); n != 1 {
		t.Fatalf("transfer rows = %d, want exactly one for a repeated grant", n)
	}
	if n := f.jobs.count("payment.transfer_create"); n != 1 {
		t.Fatalf("transfer jobs = %d, want 1", n)
	}
	missing, err := f.client.GrantPayment(ctx, connect.NewRequest(&pb.GrantPaymentRequest{OrderNo: "O-NONE"}))
	must(t, err)
	if missing.Msg.GetCode() != 404 {
		t.Fatalf("unknown order = %v", missing.Msg)
	}
}

func TestRefundRoundTripThroughTheOldContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.paid(t)

	created, err := f.client.CreateRefund(ctx, connect.NewRequest(&pb.CreateRefundRequest{TradeNo: "O-PAID", Amount: "200", Reason: "售後退款", RefundNo: "C1-0"}))
	must(t, err)
	d := created.Msg.GetData()
	if created.Msg.GetCode() != 0 || d.GetRefundNo() != "C1-0" || d.GetTradeNo() != "O-PAID" || d.GetAmount() != "200" || d.GetStatus() != "0" {
		t.Fatalf("create = %v", created.Msg)
	}
	again, err := f.client.CreateRefund(ctx, connect.NewRequest(&pb.CreateRefundRequest{TradeNo: "O-PAID", Amount: "200", RefundNo: "C1-0"}))
	must(t, err)
	if again.Msg.GetCode() != 0 || f.count(t, "SELECT count(*) FROM payment.refunds WHERE request_key = 'C1-0'") != 1 {
		t.Fatalf("a repeated key must not write a second refund: %v", again.Msg)
	}
	if n := f.count(t, "SELECT count(*) FROM payment.refunds WHERE request_key = 'C1-0' AND NOT auto"); n != 1 {
		t.Fatalf("a case refund must not be marked automatic")
	}
	auto, err := f.client.CreateRefund(ctx, connect.NewRequest(&pb.CreateRefundRequest{TradeNo: "O-PAID", Amount: "100", Reason: "訂單取消自動退款", RefundNo: "AUTO:O-PAID:O-PAID"}))
	must(t, err)
	if auto.Msg.GetCode() != 0 || f.count(t, "SELECT count(*) FROM payment.refunds WHERE request_key = 'AUTO:O-PAID:O-PAID' AND auto") != 1 {
		t.Fatalf("auto refund = %v", auto.Msg)
	}
	over, err := f.client.CreateRefund(ctx, connect.NewRequest(&pb.CreateRefundRequest{TradeNo: "O-PAID", Amount: "201", RefundNo: "C2-0"}))
	must(t, err)
	if over.Msg.GetCode() != 428 || f.count(t, "SELECT count(*) FROM payment.refunds") != 2 {
		t.Fatalf("a refund over the cap = %v", over.Msg)
	}

	got, err := f.client.GetRefund(ctx, connect.NewRequest(&pb.GetRefundRequest{RefundNo: "C1-0"}))
	must(t, err)
	if got.Msg.GetData().GetAmount() != "200" || got.Msg.GetData().GetStatus() != "0" {
		t.Fatalf("get = %v", got.Msg)
	}
	missing, err := f.client.GetRefund(ctx, connect.NewRequest(&pb.GetRefundRequest{RefundNo: "nope"}))
	must(t, err)
	if missing.Msg.GetCode() != 404 {
		t.Fatalf("missing refund = %v", missing.Msg)
	}

	byCase, err := f.client.BatchGetRefundProviderStatusByCaseIds(ctx, connect.NewRequest(&pb.BatchGetRefundProviderStatusByCaseIdsRequest{CaseIds: []string{"C1-0", "C9-0"}}))
	must(t, err)
	if e := byCase.Msg.GetEntries(); len(e) != 1 || e["C1-0"].GetProviderStatus() != "PENDING" {
		t.Fatalf("by case = %v", byCase.Msg)
	}
	byOrder, err := f.client.BatchGetRefundProviderStatusByOrderIds(ctx, connect.NewRequest(&pb.BatchGetRefundProviderStatusByOrderIdsRequest{OrderIds: []string{"O-PAID", "O-OPEN"}}))
	must(t, err)
	if e := byOrder.Msg.GetEntries(); len(e) != 1 || e["O-PAID"] == nil {
		t.Fatalf("by order = %v", byOrder.Msg)
	}
}

func TestClosePaymentByOrderNoQueuesACancelOnlyForAnUnpaidPayment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.paid(t)
	f.open(t)

	for orderNo, want := range map[string]string{"O-PAID": nothingToCloseMessage, "O-NONE": nothingToCloseMessage} {
		resp, err := f.client.ClosePaymentByOrderNo(ctx, connect.NewRequest(&pb.ClosePaymentByOrderNoRequest{OrderNo: orderNo}))
		must(t, err)
		if resp.Msg.GetCode() != 0 || resp.Msg.GetMessage() != want || f.jobs.count("payment.charge_close") != 0 {
			t.Fatalf("%s = %v, close jobs=%d", orderNo, resp.Msg, f.jobs.count("payment.charge_close"))
		}
	}
	resp, err := f.client.ClosePaymentByOrderNo(ctx, connect.NewRequest(&pb.ClosePaymentByOrderNoRequest{OrderNo: "O-OPEN"}))
	must(t, err)
	if resp.Msg.GetMessage() != closedMessage || f.jobs.count("payment.charge_close") != 1 {
		t.Fatalf("open order = %v, close jobs=%d", resp.Msg, f.jobs.count("payment.charge_close"))
	}

	// job 在 transaction 外向 provider 取消，結果照一般路徑寫成 CANCELED。
	var id string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT id::text FROM payment.payments WHERE id = (SELECT payment_id FROM payment.payment_orders WHERE order_no = 'O-OPEN')").Scan(&id))
	paymentID, err := uuid.Parse(id)
	must(t, err)
	must(t, f.module.CloseOrderPaymentAttempt(ctx, paymentID))
	if n := f.count(t, "SELECT count(*) FROM payment.payments WHERE id = (SELECT payment_id FROM payment.payment_orders WHERE order_no = 'O-OPEN') AND status = 'CANCELED'"); n != 1 {
		t.Fatalf("the payment must be CANCELED after the close job runs")
	}
}
