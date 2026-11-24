package paymentserver

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakePayments struct {
	transfer    payment.PaymentTransfer
	transferErr error
	gotRefund   payment.RefundRequest
	refund      payment.PaymentRefund
	refundErr   error
	refunds     []payment.PaymentRefund
	payment     payment.Payment
	paymentErr  error
	payments    []payment.Payment
	closed      bool
	closeErr    error
	gotKeys     []string
}

func (f *fakePayments) RequestPaymentTransfer(context.Context, string) (payment.PaymentTransfer, error) {
	return f.transfer, f.transferErr
}

func (f *fakePayments) CreatePaymentRefund(_ context.Context, in payment.RefundRequest) (payment.PaymentRefund, error) {
	f.gotRefund = in
	return f.refund, f.refundErr
}

func (f *fakePayments) RefundByRequestKey(_ context.Context, key string) (payment.PaymentRefund, error) {
	f.gotKeys = []string{key}
	return f.refund, f.refundErr
}

func (f *fakePayments) RefundsByRequestKeys(_ context.Context, keys []string) ([]payment.PaymentRefund, error) {
	f.gotKeys = keys
	return f.refunds, f.refundErr
}

func (f *fakePayments) LatestRefundsByOrderNos(_ context.Context, keys []string) ([]payment.PaymentRefund, error) {
	f.gotKeys = keys
	return f.refunds, f.refundErr
}

func (f *fakePayments) PaymentByOrderNo(context.Context, string) (payment.Payment, error) {
	return f.payment, f.paymentErr
}

func (f *fakePayments) PaymentsByOrderNos(context.Context, []string) ([]payment.Payment, error) {
	return f.payments, f.paymentErr
}

func (f *fakePayments) CloseOrderPayment(context.Context, string) (bool, error) {
	return f.closed, f.closeErr
}

var created = time.Date(2026, 11, 24, 1, 2, 3, 0, time.UTC)

func TestPaymentStatusMapsEveryStateAndNeverInventsOne(t *testing.T) {
	t.Parallel()
	for state, want := range map[payment.PaymentStatus]string{
		payment.PaymentCreating: "0", payment.PaymentAwaitingPayment: "0", payment.PaymentProcessing: "0",
		payment.PaymentSucceeded: "1", payment.PaymentCreateFailed: "2", payment.PaymentCanceled: "5",
	} {
		if got, err := paymentStatus(state); err != nil || got != want {
			t.Errorf("%s: got %q, %v; want %q", state, got, err, want)
		}
	}
	for _, bad := range []payment.PaymentStatus{payment.PaymentAnomaly, "", "UNKNOWN"} {
		if got, err := paymentStatus(bad); err == nil {
			t.Errorf("%q must not map to a status, got %q", bad, got)
		}
	}
}

func TestRefundStatusMapsEveryStateAndNeverInventsOne(t *testing.T) {
	t.Parallel()
	for state, want := range map[payment.PaymentRefundStatus]string{
		payment.RefundPending: "0", payment.RefundRefunding: "1", payment.RefundAnomaly: "1", payment.RefundSucceeded: "2", payment.RefundFailed: "3",
	} {
		if got, err := refundStatus(state); err != nil || got != want {
			t.Errorf("%s: got %q, %v; want %q", state, got, err, want)
		}
	}
	for _, bad := range []payment.PaymentRefundStatus{"", "UNKNOWN"} {
		if got, err := refundStatus(bad); err == nil {
			t.Errorf("%q must not map to a status, got %q", bad, got)
		}
	}
}

func TestCreateRefundPassesTheKeyAndMarksOnlyAutoPrefixedKeysAsAutomatic(t *testing.T) {
	t.Parallel()
	for key, wantAuto := range map[string]bool{"AUTO:O1:O1": true, "C123-0": false} {
		f := &fakePayments{refund: payment.PaymentRefund{RequestKey: key, OrderNo: "O1", Amount: 120, Status: payment.RefundPending, CreatedAt: created}}
		resp, err := New(f).CreateRefund(context.Background(), connect.NewRequest(&pb.CreateRefundRequest{TradeNo: " O1 ", Amount: "120", Reason: "售後退款", RefundNo: key}))
		if err != nil {
			t.Fatal(err)
		}
		if f.gotRefund != (payment.RefundRequest{OrderNo: "O1", Amount: 120, RequestKey: key, Auto: wantAuto}) {
			t.Errorf("request to the module = %+v", f.gotRefund)
		}
		d := resp.Msg.GetData()
		if resp.Msg.GetCode() != 0 || d.GetRefundNo() != key || d.GetTradeNo() != "O1" || d.GetOrderId() != "O1" || d.GetAmount() != "120" || d.GetStatus() != "0" || d.GetReason() != "售後退款" {
			t.Errorf("response = %v", resp.Msg)
		}
	}
}

func TestCreateRefundRejectsABadRequestWithoutCallingTheModule(t *testing.T) {
	t.Parallel()
	for name, req := range map[string]*pb.CreateRefundRequest{
		"missing key":     {TradeNo: "O1", Amount: "10"},
		"zero amount":     {TradeNo: "O1", Amount: "0", RefundNo: "k"},
		"negative amount": {TradeNo: "O1", Amount: "-5", RefundNo: "k"},
		"decimal amount":  {TradeNo: "O1", Amount: "1.5", RefundNo: "k"},
		"empty amount":    {TradeNo: "O1", RefundNo: "k"},
	} {
		f := &fakePayments{}
		resp, err := New(f).CreateRefund(context.Background(), connect.NewRequest(req))
		if err != nil || resp.Msg.GetCode() != codeInvalidInput || f.gotRefund != (payment.RefundRequest{}) {
			t.Errorf("%s: code=%d err=%v module=%+v", name, resp.Msg.GetCode(), err, f.gotRefund)
		}
	}
}

func TestModuleErrorsBecomeBusinessCodesOrGRPCErrors(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		err      error
		wantCode int32
		wantGRPC bool
	}{
		"not found":           {apperr.NotFound("x"), 404, false},
		"invalid":             {apperr.InvalidInput("x"), 400, false},
		"failed precondition": {apperr.FailedPrecondition("x"), 428, false},
		"unavailable":         {apperr.Unavailable("x"), 0, true},
		"plain":               {errors.New("boom"), 0, true},
	} {
		f := &fakePayments{refundErr: tc.err}
		resp, err := New(f).CreateRefund(context.Background(), connect.NewRequest(&pb.CreateRefundRequest{TradeNo: "O1", Amount: "10", RefundNo: "k"}))
		if tc.wantGRPC {
			if err == nil {
				t.Errorf("%s: want a gRPC error", name)
			}
			continue
		}
		if err != nil || resp.Msg.GetCode() != tc.wantCode {
			t.Errorf("%s: code=%d err=%v, want %d", name, resp.Msg.GetCode(), err, tc.wantCode)
		}
	}
}

func TestGrantPaymentRetriesUnmetPreconditionsAsGRPCErrorsButNotFoundIsABusinessCode(t *testing.T) {
	t.Parallel()
	ok, err := New(&fakePayments{transfer: payment.PaymentTransfer{Amount: 300, CreatedAt: created}}).GrantPayment(context.Background(), connect.NewRequest(&pb.GrantPaymentRequest{OrderNo: "O1"}))
	if err != nil || ok.Msg.GetCode() != 0 || ok.Msg.GetGrantOutcome() != "GRANTED" || ok.Msg.GetData().GetAmount() != "300" || ok.Msg.GetData().GetTradeNo() != "O1" || ok.Msg.GetData().GetGrantedAt() == "" {
		t.Fatalf("success = %v, %v", ok, err)
	}
	if _, err := New(&fakePayments{transferErr: apperr.FailedPrecondition("seller account not enabled")}).GrantPayment(context.Background(), connect.NewRequest(&pb.GrantPaymentRequest{OrderNo: "O1"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("precondition error = %v, want a FailedPrecondition gRPC error so the order worker retries", err)
	}
	nf, err := New(&fakePayments{transferErr: apperr.NotFound("none")}).GrantPayment(context.Background(), connect.NewRequest(&pb.GrantPaymentRequest{OrderNo: "O1"}))
	if err != nil || nf.Msg.GetCode() != 404 {
		t.Fatalf("not found = %v, %v", nf, err)
	}
}

func TestActivePaymentStatusHasNoDataWhenThereIsNoPaymentAndFailsClosedOnAnomaly(t *testing.T) {
	t.Parallel()
	none, err := New(&fakePayments{paymentErr: apperr.NotFound("none")}).GetActivePaymentStatusByOrderNo(context.Background(), connect.NewRequest(&pb.GetActivePaymentStatusByOrderNoRequest{OrderNo: "O1"}))
	if err != nil || none.Msg.GetCode() != 0 || none.Msg.GetData() != nil {
		t.Fatalf("no payment = %v, %v", none, err)
	}
	paid := created.Add(time.Minute)
	got, err := New(&fakePayments{payment: payment.Payment{OrderNo: "O1", BuyerID: 7, Amount: 99, Status: payment.PaymentSucceeded, PaidAt: &paid, CreatedAt: created}}).
		GetActivePaymentStatusByOrderNo(context.Background(), connect.NewRequest(&pb.GetActivePaymentStatusByOrderNoRequest{OrderNo: "O1"}))
	d := got.Msg.GetData()
	if err != nil || d.GetStatus() != "1" || d.GetTradeNo() != "O1" || d.GetBuyerId() != "7" || d.GetAmount() != "99" || d.GetPayTime() != strconv.FormatInt(paid.UnixMilli(), 10) || d.GetProviderStatus() != "SUCCEEDED" {
		t.Fatalf("paid = %v, %v", got, err)
	}
	if _, err := New(&fakePayments{payment: payment.Payment{OrderNo: "O1", Status: payment.PaymentAnomaly}}).GetActivePaymentStatusByOrderNo(context.Background(), connect.NewRequest(&pb.GetActivePaymentStatusByOrderNoRequest{OrderNo: "O1"})); err == nil {
		t.Fatal("an ANOMALY payment must be a gRPC error so the order side does not cancel")
	}
}

func TestPaymentFactsOnlyExistForAPaidPaymentAndAlwaysCarryTheProviderTime(t *testing.T) {
	t.Parallel()
	paid := time.Date(2026, 11, 24, 9, 30, 15, 987, time.FixedZone("TST", 8*3600))
	got, err := New(&fakePayments{payment: payment.Payment{OrderNo: "O1", Amount: 250, Status: payment.PaymentSucceeded, PaidAt: &paid}}).
		GetPaymentFactsInternal(context.Background(), connect.NewRequest(&pb.GetPaymentFactsInternalRequest{OrderNo: "O1"}))
	d := got.Msg.GetData()
	if err != nil || got.Msg.GetCode() != 0 || d.GetTradeNo() != "O1" || d.GetTradeAmt() != "250" || d.GetPayTime() != "2026-11-24T01:30:15Z" || d.ProviderPaidAt == nil || *d.ProviderPaidAt != "2026-11-24T01:30:15Z" || d.GetIsDuplicate() {
		t.Fatalf("facts = %v, %v", got, err)
	}
	for name, f := range map[string]*fakePayments{
		"no payment": {paymentErr: apperr.NotFound("none")},
		"unpaid":     {payment: payment.Payment{OrderNo: "O1", Status: payment.PaymentAwaitingPayment, PaidAt: &paid}},
		"no paid_at": {payment: payment.Payment{OrderNo: "O1", Status: payment.PaymentSucceeded}},
	} {
		r, err := New(f).GetPaymentFactsInternal(context.Background(), connect.NewRequest(&pb.GetPaymentFactsInternalRequest{OrderNo: "O1"}))
		if err != nil || r.Msg.GetCode() != 8015 || r.Msg.GetData() != nil {
			t.Errorf("%s: %v, %v; want the not-found business code and no data", name, r, err)
		}
	}
}

func TestBatchEntriesAreKeyedByCaseKeyAndOrderAndOmitMissing(t *testing.T) {
	t.Parallel()
	f := &fakePayments{refunds: []payment.PaymentRefund{
		{ID: uuid.NewV7(), OrderNo: "O1", RequestKey: "C1-0", Status: payment.RefundFailed, FailureReason: "provider:expired"},
	}}
	byCase, err := New(f).BatchGetRefundProviderStatusByCaseIds(context.Background(), connect.NewRequest(&pb.BatchGetRefundProviderStatusByCaseIdsRequest{CaseIds: []string{"C1-0", "C2-0"}}))
	if err != nil || len(byCase.Msg.GetEntries()) != 1 || byCase.Msg.GetEntries()["C1-0"].GetProviderStatus() != "FAILED" || byCase.Msg.GetEntries()["C1-0"].GetProviderMessage() != "provider:expired" {
		t.Fatalf("by case = %v, %v", byCase, err)
	}
	byOrder, err := New(f).BatchGetRefundProviderStatusByOrderIds(context.Background(), connect.NewRequest(&pb.BatchGetRefundProviderStatusByOrderIdsRequest{OrderIds: []string{"O1", "O9"}}))
	if err != nil || len(byOrder.Msg.GetEntries()) != 1 || byOrder.Msg.GetEntries()["O1"] == nil {
		t.Fatalf("by order = %v, %v", byOrder, err)
	}
	pays, err := New(&fakePayments{payments: []payment.Payment{{OrderNo: "O1", Status: payment.PaymentCreateFailed, LastError: "card_declined"}}}).
		BatchGetProviderStatusByOrderIds(context.Background(), connect.NewRequest(&pb.BatchGetProviderStatusByOrderIdsRequest{OrderIds: []string{"O1", "O2"}}))
	if err != nil || len(pays.Msg.GetEntries()) != 1 || pays.Msg.GetEntries()["O1"].GetProviderStatus() != "CREATE_FAILED" || pays.Msg.GetEntries()["O1"].GetProviderMessage() != "card_declined" {
		t.Fatalf("payments = %v, %v", pays, err)
	}
}

func TestClosePaymentByOrderNoReportsWhetherAJobWasQueued(t *testing.T) {
	t.Parallel()
	for queued, want := range map[bool]string{true: closedMessage, false: nothingToCloseMessage} {
		r, err := New(&fakePayments{closed: queued}).ClosePaymentByOrderNo(context.Background(), connect.NewRequest(&pb.ClosePaymentByOrderNoRequest{OrderNo: "O1"}))
		if err != nil || r.Msg.GetCode() != 0 || r.Msg.GetMessage() != want {
			t.Errorf("queued=%v: %v, %v", queued, r, err)
		}
	}
	if _, err := New(&fakePayments{closeErr: errors.New("db down")}).ClosePaymentByOrderNo(context.Background(), connect.NewRequest(&pb.ClosePaymentByOrderNoRequest{OrderNo: "O1"})); err == nil {
		t.Fatal("a failure to queue must reach the caller")
	}
}

func TestGetRefundIsKeyedByTheCallersKey(t *testing.T) {
	t.Parallel()
	f := &fakePayments{refund: payment.PaymentRefund{RequestKey: "C1-0", OrderNo: "O1", Amount: 5, Status: payment.RefundSucceeded, CreatedAt: created}}
	r, err := New(f).GetRefund(context.Background(), connect.NewRequest(&pb.GetRefundRequest{RefundNo: "C1-0"}))
	if err != nil || f.gotKeys[0] != "C1-0" || r.Msg.GetData().GetStatus() != "2" || r.Msg.GetData().GetRefundNo() != "C1-0" {
		t.Fatalf("%v, %v, keys=%v", r, err, f.gotKeys)
	}
	nf, err := New(&fakePayments{refundErr: apperr.NotFound("none")}).GetRefund(context.Background(), connect.NewRequest(&pb.GetRefundRequest{RefundNo: "x"}))
	if err != nil || nf.Msg.GetCode() != 404 || nf.Msg.GetData() != nil {
		t.Fatalf("not found = %v, %v", nf, err)
	}
}
