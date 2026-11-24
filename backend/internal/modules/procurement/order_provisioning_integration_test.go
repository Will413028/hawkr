//go:build integration

package procurement

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/provisioning"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

type scriptedOrders struct {
	results []provisioning.Result
	errs    []error
	got     []provisioning.Command
	// parties 與 partiesErr 是 OrderParties 的回答；onCreate 在每次 CreateOrder 開頭執行（模擬呼叫期間接受單被別處改了）。
	parties    provisioning.Parties
	partiesErr error
	onCreate   func()
}

func (s *scriptedOrders) OrderParties(context.Context, string) (provisioning.Parties, error) {
	return s.parties, s.partiesErr
}

func (s *scriptedOrders) CreateOrder(_ context.Context, cmd provisioning.Command) (provisioning.Result, error) {
	i := len(s.got)
	s.got = append(s.got, cmd)
	if s.onCreate != nil {
		s.onCreate()
	}
	if i < len(s.errs) && s.errs[i] != nil {
		return provisioning.Result{}, s.errs[i]
	}
	if i < len(s.results) {
		return s.results[i], nil
	}
	return provisioning.Result{}, errors.New("沒有腳本")
}

var paymentDeadline = epoch.Add(72 * time.Hour)

func okResult() provisioning.Result {
	return provisioning.Result{OrderNo: "HK1", PaymentDeadline: paymentDeadline}
}

// orderFixture 是一筆 CREATING_ORDER 的接受單（被接受的版本：外套、12.500 件）與一個用替身建單的模組。
type orderFixture struct {
	t      *testing.T
	d      *db.DB
	m      *Module
	c      chain
	orders *scriptedOrders
	logs   *bytes.Buffer
}

func newOrderFixture(t *testing.T) *orderFixture {
	t.Helper()
	ctx := context.Background()
	d := newDB(t)
	c := seed(ctx, t, store.New(d.Pool()), 21, true)
	if _, err := d.Pool().Exec(ctx, `INSERT INTO procurement.seller_response_versions (
			id, seller_response_id, version, response_type, spec_note, unit_basis_quantity, unit_basis_unit,
			offered_quantity, offered_quantity_unit, conversion_basis, currency, goods_amount, shipping_amount, tax_amount, tax_inclusion_basis, total_amount,
			valid_until, outside_buyer_requested_range, publication_version, client_request_id, submitted_at, created_at)
		VALUES ($1, $2, 1, 'MANUAL_QUOTE', '', 1, '件', 12.500, '件', '', 'TWD', 1000, 100, 50, 'TAX_EXCLUDED', 1150, $3, false, 1, $4, $5, $5)`,
		id(), c.sellerResponse, epoch.Add(time.Hour), id(), epoch); err != nil {
		t.Fatal(err)
	}
	f := &orderFixture{t: t, d: d, c: c, orders: &scriptedOrders{parties: provisioning.Parties{BuyerID: 21, StoreID: storeID}}, logs: &bytes.Buffer{}}
	f.m = New(Deps{
		DB: d, Now: func() time.Time { return epoch }, Logger: slog.New(slog.NewTextHandler(f.logs, nil)), Jobs: &fakeJobs{},
		Events: events.NewBus(), Orders: f.orders,
	})
	return f
}

func (f *orderFixture) state() (state, provisioning string, reason, orderNo *string) {
	f.t.Helper()
	if err := f.d.Pool().QueryRow(context.Background(),
		`SELECT state, order_provisioning_state, order_failure_reason, order_no FROM procurement.buyer_quote_acceptances WHERE id = $1`, f.c.acceptance,
	).Scan(&state, &provisioning, &reason, &orderNo); err != nil {
		f.t.Fatal(err)
	}
	return
}

func (f *orderFixture) orderRef() int64 {
	f.t.Helper()
	var ref int64
	if err := f.d.Pool().QueryRow(context.Background(), `SELECT order_ref FROM procurement.buyer_quote_acceptances WHERE id = $1`, f.c.acceptance).Scan(&ref); err != nil {
		f.t.Fatal(err)
	}
	return ref
}

func (f *orderFixture) wantFailed(reason string) {
	f.t.Helper()
	state, prov, got, orderNo := f.state()
	if state != "ORDER_FAILED" || prov != "ORDER_FAILED" || got == nil || *got != reason || orderNo != nil {
		f.t.Fatalf("要 ORDER_FAILED／%s，得到 %s %s %v %v", reason, state, prov, got, orderNo)
	}
}

func (f *orderFixture) wantCreating() {
	f.t.Helper()
	if state, prov, reason, _ := f.state(); state != "ACTIVE" || prov != "CREATING_ORDER" || reason != nil {
		f.t.Fatalf("要仍是 ACTIVE／CREATING_ORDER，得到 %s %s %v", state, prov, reason)
	}
}

func (f *orderFixture) workJob(attempt, max int) error {
	job := &river.Job[quote.OrderProvisioningArgs]{
		JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: max},
		Args:   quote.OrderProvisioningArgs{AcceptanceID: f.c.acceptance},
	}
	return f.m.orders.Work(context.Background(), job)
}

func (f *orderFixture) canAcceptAgain() {
	f.t.Helper()
	n, err := store.New(f.d.Pool()).InsertAcceptance(context.Background(), acceptanceParams(f.c, f.c.buyer, id(), id()))
	if err != nil || n != 1 {
		f.t.Fatalf("live 鎖解除後買家要能再接受：n=%d err=%v", n, err)
	}
}

func TestProvisionCreatesTheOrderWithTheFrozenAmountsAndTheOrderRef(t *testing.T) {
	f := newOrderFixture(t)
	f.orders.results = []provisioning.Result{okResult()}
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err != nil {
		t.Fatal(err)
	}
	want := provisioning.Command{OrderRef: f.orderRef(), BuyerID: 21, StoreID: storeID, TotalAmount: 1150, ShippingFee: 100, UnitPrice: 1050, ItemName: "外套 12.5 件"}
	if len(f.orders.got) != 1 || f.orders.got[0] != want {
		t.Fatalf("命令 = %+v，要 %+v", f.orders.got, want)
	}
	state, prov, _, orderNo := f.state()
	if state != "ACTIVE" || prov != "ORDER_CREATED" || orderNo == nil || *orderNo != "HK1" {
		t.Fatalf("結果 %s %s %v", state, prov, orderNo)
	}
	var deadline time.Time
	if err := f.d.Pool().QueryRow(context.Background(), `SELECT payment_deadline FROM procurement.buyer_quote_acceptances WHERE id = $1`, f.c.acceptance).Scan(&deadline); err != nil || !deadline.Equal(paymentDeadline) {
		t.Fatalf("付款期限 %v %v", deadline, err)
	}
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, true); err != nil || len(f.orders.got) != 1 {
		t.Fatalf("已建好的接受單不再呼叫 ordering：%v，呼叫 %d 次", err, len(f.orders.got))
	}
}

func TestProvisionRetriesUseTheSameOrderRefAndCreateOnce(t *testing.T) {
	f := newOrderFixture(t)
	f.orders.errs = []error{errors.New("連不上"), nil, nil}
	f.orders.results = []provisioning.Result{{}, {Code: 500, Message: "系統錯誤"}, okResult()}
	for i := range 2 {
		if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err == nil {
			t.Fatalf("第 %d 次暫時性失敗要回 error 讓 River 重試", i+1)
		}
		f.wantCreating()
	}
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err != nil {
		t.Fatal(err)
	}
	ref := f.orderRef()
	if len(f.orders.got) != 3 {
		t.Fatalf("呼叫 %d 次", len(f.orders.got))
	}
	for _, cmd := range f.orders.got {
		if cmd.OrderRef != ref {
			t.Fatalf("重試要用同一個 order_ref %d，得到 %d", ref, cmd.OrderRef)
		}
	}
	if _, prov, _, _ := f.state(); prov != "ORDER_CREATED" {
		t.Fatalf("provisioning %s", prov)
	}
}

func TestEachAcceptanceGetsItsOwnOrderRef(t *testing.T) {
	f := newOrderFixture(t)
	first := f.orderRef()
	f.orders.results = []provisioning.Result{{Code: 9504, Message: "拒絕"}}
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err != nil {
		t.Fatal(err)
	}
	second := id()
	if n, err := store.New(f.d.Pool()).InsertAcceptance(context.Background(), acceptanceParams(f.c, f.c.buyer, second, id())); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	var ref int64
	if err := f.d.Pool().QueryRow(context.Background(), `SELECT order_ref FROM procurement.buyer_quote_acceptances WHERE id = $1`, second).Scan(&ref); err != nil || ref == first {
		t.Fatalf("再接受的接受單要有新的 order_ref：%d vs %d %v", ref, first, err)
	}
}

func TestProvisionTerminalRejectionMarksFailedWithoutWaitingForTheLastAttempt(t *testing.T) {
	f := newOrderFixture(t)
	f.orders.results = []provisioning.Result{{Code: 9504, Message: "拒絕"}}
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err != nil {
		t.Fatalf("終態拒絕不回 error（不重試）：%v", err)
	}
	f.wantFailed(provisioning.ReasonRejected)
	f.canAcceptAgain()
}

func TestProvisionSuccessWithoutAnOrderNumberIsATerminalRejection(t *testing.T) {
	f := newOrderFixture(t)
	f.orders.results = []provisioning.Result{{PaymentDeadline: paymentDeadline}}
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err != nil {
		t.Fatal(err)
	}
	f.wantFailed(provisioning.ReasonRejected)
}

func TestLastAttemptFailureMarksOrderFailedReleasesTheLockAndTheBuyerCanAcceptAgain(t *testing.T) {
	f := newOrderFixture(t)
	f.orders.results = []provisioning.Result{{Code: 500}, {Code: 500}, {Code: 500}, {Code: 500}, {Code: 500}, {Code: 500}}
	for attempt := 1; attempt < quote.OrderMaxAttempts; attempt++ {
		if err := f.workJob(attempt, quote.OrderMaxAttempts); err == nil {
			t.Fatalf("第 %d 次要回 error", attempt)
		}
		f.wantCreating()
	}
	if err := f.workJob(quote.OrderMaxAttempts, quote.OrderMaxAttempts); err == nil {
		t.Fatal("最後一次仍要回 error（job 進 discarded）")
	}
	f.wantFailed(provisioning.ReasonRetryExhausted)
	if _, err := store.New(f.d.Pool()).GetLiveAcceptance(context.Background(), store.GetLiveAcceptanceParams{RequestID: f.c.request, BuyerID: &f.c.buyer}); err == nil {
		t.Fatal("建單失敗後不該還有 live 接受單")
	}
	f.canAcceptAgain()
}

func TestLastAttemptTransportFailureAlsoMarksOrderFailed(t *testing.T) {
	f := newOrderFixture(t)
	f.orders.errs = []error{errors.New("連不上"), errors.New("還是連不上")}
	if err := f.workJob(quote.OrderMaxAttempts, quote.OrderMaxAttempts); err == nil {
		t.Fatal("要回 error")
	}
	f.wantFailed(provisioning.ReasonRetryExhausted)
	if len(f.orders.got) != 2 {
		t.Fatalf("標失敗前要再呼叫一次：呼叫 %d 次", len(f.orders.got))
	}
}

// 回應遺失但單已建好：最後一次嘗試的再呼叫用同一個 order_ref 取回既有訂單，不標成失敗。
func TestLastAttemptRecallRecoversAnOrderWhoseResponseWasLost(t *testing.T) {
	f := newOrderFixture(t)
	f.orders.errs = []error{errors.New("回應遺失"), nil}
	replayed := okResult()
	replayed.Replayed = true
	f.orders.results = []provisioning.Result{{}, replayed}
	if err := f.workJob(quote.OrderMaxAttempts, quote.OrderMaxAttempts); err != nil {
		t.Fatalf("再呼叫取回訂單就不回 error：%v", err)
	}
	state, prov, _, orderNo := f.state()
	if state != "ACTIVE" || prov != "ORDER_CREATED" || orderNo == nil || *orderNo != "HK1" {
		t.Fatalf("結果 %s %s %v", state, prov, orderNo)
	}
	if len(f.orders.got) != 2 || f.orders.got[0].OrderRef != f.orders.got[1].OrderRef || f.orders.got[1].OrderRef != f.orderRef() {
		t.Fatalf("兩次要用同一個 order_ref：%+v", f.orders.got)
	}
}

// R1-1：order_ref 命中的既有訂單不是這張接受單的買家與店家（舊 order-service 的 acceptance_id 撞號）時不採用。
func TestProvisionRejectsAReplayedOrderOfAnotherBuyerOrStore(t *testing.T) {
	for name, parties := range map[string]provisioning.Parties{
		"別的買家": {BuyerID: 22, StoreID: storeID},
		"別的店家": {BuyerID: 21, StoreID: storeID + 1},
	} {
		t.Run(name, func(t *testing.T) {
			f := newOrderFixture(t)
			replayed := okResult()
			replayed.Replayed = true
			f.orders.results = []provisioning.Result{replayed}
			f.orders.parties = parties
			if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err != nil {
				t.Fatal(err)
			}
			f.wantFailed(provisioning.ReasonRejected)
			if got := f.logs.String(); !strings.Contains(got, "level=ERROR") || !strings.Contains(got, "order_no=HK1") {
				t.Fatalf("要以 error 等級記下命中的訂單號：%s", got)
			}
		})
	}
}

func TestProvisionAcceptsAReplayedOrderOfTheSameBuyerAndStore(t *testing.T) {
	f := newOrderFixture(t)
	replayed := okResult()
	replayed.Replayed = true
	f.orders.results = []provisioning.Result{replayed}
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err != nil {
		t.Fatal(err)
	}
	if _, prov, _, orderNo := f.state(); prov != "ORDER_CREATED" || orderNo == nil || *orderNo != "HK1" {
		t.Fatalf("provisioning %s %v", prov, orderNo)
	}
}

func TestProvisionRetriesWhenTheReplayedOrderCannotBeVerified(t *testing.T) {
	f := newOrderFixture(t)
	replayed := okResult()
	replayed.Replayed = true
	f.orders.results = []provisioning.Result{replayed}
	f.orders.partiesErr = errors.New("查不到")
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err == nil {
		t.Fatal("確認不了要回 error 讓 River 重試")
	}
	f.wantCreating()
}

// R1-4：job 因逾時（不是關機）結束時，最後一次嘗試仍要用新的 context 標成建單失敗。
func TestLastAttemptAfterTheJobTimedOutStillMarksOrderFailed(t *testing.T) {
	f := newOrderFixture(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if err := f.m.orders.Provision(ctx, f.c.acceptance, true); err == nil {
		t.Fatal("要回 error")
	}
	f.wantFailed(provisioning.ReasonRetryExhausted)
}

// R1-2：ordering 已建單但接受單已不在 CREATING_ORDER 時，訂單號不能無聲消失。
func TestProvisionLogsAnErrorWhenTheOrderNumberCannotBeRecorded(t *testing.T) {
	f := newOrderFixture(t)
	f.orders.results = []provisioning.Result{okResult()}
	f.orders.onCreate = func() {
		if _, err := f.d.Pool().Exec(context.Background(), `UPDATE procurement.buyer_quote_acceptances SET state = 'ORDER_FAILED', order_provisioning_state = 'ORDER_FAILED', order_failure_reason = 'X' WHERE id = $1`, f.c.acceptance); err != nil {
			t.Error(err)
		}
	}
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err != nil {
		t.Fatal(err)
	}
	if got := f.logs.String(); !strings.Contains(got, "level=ERROR") || !strings.Contains(got, "order_no=HK1") || !strings.Contains(got, f.c.acceptance.String()) {
		t.Fatalf("要以 error 等級記下訂單號與接受單 id：%s", got)
	}
}

func TestProvisionLogsAnErrorWhenTheFailureCannotBeRecorded(t *testing.T) {
	f := newOrderFixture(t)
	f.orders.results = []provisioning.Result{{Code: 9504, Message: "拒絕"}}
	f.orders.onCreate = func() {
		if _, err := f.d.Pool().Exec(context.Background(), `UPDATE procurement.buyer_quote_acceptances SET order_provisioning_state = 'ORDER_CREATED', order_no = 'HK9' WHERE id = $1`, f.c.acceptance); err != nil {
			t.Error(err)
		}
	}
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err != nil {
		t.Fatal(err)
	}
	if got := f.logs.String(); !strings.Contains(got, "level=ERROR") || !strings.Contains(got, f.c.acceptance.String()) {
		t.Fatalf("要以 error 等級記下接受單 id：%s", got)
	}
}

func TestShutdownOnTheLastAttemptDoesNotMarkOrderFailed(t *testing.T) {
	f := newOrderFixture(t)
	f.orders.errs = []error{context.Canceled}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.m.orders.Provision(ctx, f.c.acceptance, true); err == nil {
		t.Fatal("要回 error")
	}
	f.wantCreating()
}

func TestProvisionMarksFailedDirectlyWhenTheBuyerWasDeleted(t *testing.T) {
	f := newOrderFixture(t)
	if _, err := f.d.Pool().Exec(context.Background(), `UPDATE procurement.buyer_quote_acceptances SET buyer_id = NULL WHERE id = $1`, f.c.acceptance); err != nil {
		t.Fatal(err)
	}
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err != nil {
		t.Fatal(err)
	}
	if len(f.orders.got) != 0 {
		t.Fatal("買家已刪不得呼叫 ordering")
	}
	f.wantFailed(provisioning.ReasonBuyerDeleted)
}

type fakeStates map[int64]rivertype.JobState

func (s fakeStates) JobState(_ context.Context, id int64) (rivertype.JobState, bool, error) {
	st, ok := s[id]
	return st, ok, nil
}

func (f *orderFixture) setJob(id *int64) {
	f.t.Helper()
	if _, err := f.d.Pool().Exec(context.Background(), `UPDATE procurement.buyer_quote_acceptances SET order_job_id = $2 WHERE id = $1`, f.c.acceptance, id); err != nil {
		f.t.Fatal(err)
	}
}

func TestReconcilerMarksOrderFailedWhenTheJobIsDeadAndLeavesLiveJobsAlone(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name   string
		states fakeStates
		job    *int64
		failed bool
	}{
		{"discarded", fakeStates{7: rivertype.JobStateDiscarded}, ptr(int64(7)), true},
		{"cancelled", fakeStates{7: rivertype.JobStateCancelled}, ptr(int64(7)), true},
		{"completed 卻沒收尾", fakeStates{7: rivertype.JobStateCompleted}, ptr(int64(7)), true},
		{"job 列不存在", fakeStates{}, ptr(int64(7)), true},
		{"沒有記下 job", fakeStates{}, nil, true},
		{"排隊中", fakeStates{7: rivertype.JobStateAvailable}, ptr(int64(7)), false},
		{"等重試", fakeStates{7: rivertype.JobStateRetryable}, ptr(int64(7)), false},
		{"執行中", fakeStates{7: rivertype.JobStateRunning}, ptr(int64(7)), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newOrderFixture(t)
			f.setJob(c.job)
			n, err := f.m.ordersRec.Reconcile(ctx, c.states)
			if err != nil {
				t.Fatal(err)
			}
			if c.failed {
				if n != 1 {
					t.Fatalf("標失敗 %d 列", n)
				}
				f.wantFailed(provisioning.ReasonJobLost)
				f.canAcceptAgain()
				return
			}
			if n != 0 {
				t.Fatalf("job 還活著不該動：%d", n)
			}
			f.wantCreating()
		})
	}
}

// reconciler 標成失敗之前也先用同一個 order_ref 再呼叫一次，取回回應遺失的訂單。
func TestReconcilerRecallsOrderingBeforeMarkingFailed(t *testing.T) {
	f := newOrderFixture(t)
	f.setJob(nil)
	f.orders.results = []provisioning.Result{okResult()}
	n, err := f.m.ordersRec.Reconcile(context.Background(), fakeStates{})
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if _, prov, _, orderNo := f.state(); prov != "ORDER_CREATED" || orderNo == nil || *orderNo != "HK1" {
		t.Fatalf("provisioning %s %v", prov, orderNo)
	}
	if len(f.orders.got) != 1 || f.orders.got[0].OrderRef != f.orderRef() {
		t.Fatalf("呼叫 %+v", f.orders.got)
	}
}

func TestReconcilerIgnoresFinishedAcceptances(t *testing.T) {
	f := newOrderFixture(t)
	f.orders.results = []provisioning.Result{okResult()}
	if err := f.m.orders.Provision(context.Background(), f.c.acceptance, false); err != nil {
		t.Fatal(err)
	}
	if n, err := f.m.ordersRec.Reconcile(context.Background(), fakeStates{}); err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if _, prov, _, _ := f.state(); prov != "ORDER_CREATED" {
		t.Fatalf("provisioning %s", prov)
	}
}

func TestAddOrderWorkersRequiresTheOrdersPort(t *testing.T) {
	m := New(Deps{DB: new(db.DB), Now: time.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Events: events.NewBus()})
	defer func() {
		if recover() == nil {
			t.Fatal("沒給 Deps.Orders 要 panic")
		}
	}()
	m.AddOrderWorkers(river.NewWorkers())
}
