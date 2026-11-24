package payment

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"uuid"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1/paymentv1alpha1connect"
	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type paymentAdminModule struct {
	// 清單與人工結案：記下收到的篩選、排序鍵與頁大小，回固定的一頁。
	paymentFilter  payment.PaymentFilter
	refundFilter   payment.RefundFilter
	transferFilter payment.TransferFilter
	gotAfter       *payment.RecordCursor
	gotSize        int32
	payments       payment.PaymentRecordPage
	transfers      payment.TransferRecordPage
	refunds        payment.RefundRecordPage
	resolved       []resolveCall
	account        payment.Account
	// refreshedAccount 是 RefreshAccount 回的重讀後帳戶，和 account（Get 回的舊快取）分開，測試才分得出回應用的是哪一個。
	refreshedAccount payment.Account
	refreshed        int64
	retried          int64
	retriedAck       bool
	purgeRetried     int64
	mappingDeleted   int64
	// actors 是三個寫入方法收到的操作者（依呼叫順序）。
	actors []payment.AdminActor
	err    error
}

func (m *paymentAdminModule) RetryAccountPurge(_ context.Context, actor payment.AdminActor, id int64) (payment.Account, error) {
	m.purgeRetried = id
	m.actors = append(m.actors, actor)
	return m.account, m.err
}

func (m *paymentAdminModule) DeleteAccountMapping(_ context.Context, actor payment.AdminActor, id int64) error {
	m.mappingDeleted = id
	m.actors = append(m.actors, actor)
	return m.err
}

func (m *paymentAdminModule) Account(context.Context, int64) (payment.Account, error) {
	return m.account, m.err
}

func (m *paymentAdminModule) RefreshAccount(_ context.Context, id int64) (payment.Account, error) {
	m.refreshed = id
	return m.refreshedAccount, m.err
}

func (m *paymentAdminModule) RetryAccountCreate(_ context.Context, actor payment.AdminActor, id int64, ack bool) (payment.Account, error) {
	m.retried, m.retriedAck = id, ack
	m.actors = append(m.actors, actor)
	return m.account, m.err
}

func TestAdminPaymentAccountHandlersMapTheAccountAndPassTheStore(t *testing.T) {
	ext := "acct_1"
	reason := "rejected.fraud"
	failed := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	m := &paymentAdminModule{account: payment.Account{
		StoreID: 11, Provider: "stripe", ExternalAccountID: &ext, Status: payment.StatusDisabled, RequirementCollection: payment.CollectionProvider,
		StatusReason: &reason, CreateFailedAt: &failed, CreatedAt: failed, UpdatedAt: failed,
	}}
	m.refreshedAccount = m.account
	m.refreshedAccount.Status = payment.StatusEnabled
	h := NewAdmin(m)
	adminCtx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 77})
	got, err := h.GetPaymentAccount(context.Background(), connect.NewRequest(&adminpb.GetPaymentAccountRequest{StoreId: "11"}))
	if err != nil {
		t.Fatal(err)
	}
	a := got.Msg.GetAccount()
	if a.GetStoreId() != "11" || a.GetProvider() != "stripe" || a.GetExternalAccountId() != "acct_1" || a.GetStatus() != paymentpb.PaymentAccountStatus_PAYMENT_ACCOUNT_STATUS_DISABLED ||
		a.GetStatusReason() != reason || a.GetCreateFailedTime() == nil || a.GetFetchTime() != nil {
		t.Fatalf("account=%v", a)
	}
	refreshed, err := h.RefreshPaymentAccount(context.Background(), connect.NewRequest(&adminpb.RefreshPaymentAccountRequest{StoreId: "11"}))
	if err != nil || m.refreshed != 11 {
		t.Fatalf("refresh: %v %d", err, m.refreshed)
	}
	if ra := refreshed.Msg.GetAccount(); ra.GetStoreId() != "11" || ra.GetStatus() != paymentpb.PaymentAccountStatus_PAYMENT_ACCOUNT_STATUS_ENABLED {
		t.Fatalf("refresh 的回應沒有帶重讀後的帳戶：%v", ra)
	}
	if _, err := h.RetryPaymentAccountCreate(adminCtx, connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "11"})); err != nil || m.retried != 11 || m.retriedAck {
		t.Fatalf("retry: %v %d ack=%v", err, m.retried, m.retriedAck)
	}
	if _, err := h.RetryPaymentAccountCreate(adminCtx, connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "11", AcknowledgeNoProviderAccount: true})); err != nil || !m.retriedAck {
		t.Fatalf("retry 帶確認：%v ack=%v", err, m.retriedAck)
	}
	for name, call := range map[string]func() error{
		"Get": func() error {
			_, err := h.GetPaymentAccount(context.Background(), connect.NewRequest(&adminpb.GetPaymentAccountRequest{StoreId: "x"}))
			return err
		},
		"Refresh": func() error {
			_, err := h.RefreshPaymentAccount(context.Background(), connect.NewRequest(&adminpb.RefreshPaymentAccountRequest{}))
			return err
		},
		"Retry": func() error {
			_, err := h.RetryPaymentAccountCreate(adminCtx, connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "0"}))
			return err
		},
	} {
		if apperr.KindOf(call()) != apperr.KindInvalidInput {
			t.Errorf("%s：壞的 store_id 不是 invalid argument", name)
		}
	}
}

func TestAdminPurgeHandlersPassTheStoreAndTheCallingAdmin(t *testing.T) {
	blocked := time.Date(2026, 11, 14, 1, 0, 0, 0, time.UTC)
	reason := "provider_refused：no"
	m := &paymentAdminModule{account: payment.Account{StoreID: 11, Provider: "stripe", PurgePendingAt: &blocked, PurgeBlockedAt: &blocked, PurgeBlockedReason: &reason, CreateUnresolved: true, CreatedAt: blocked, UpdatedAt: blocked}}
	h := NewAdmin(m)
	adminCtx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 77})
	got, err := h.RetryPaymentAccountPurge(adminCtx, connect.NewRequest(&adminpb.RetryPaymentAccountPurgeRequest{StoreId: "11"}))
	if err != nil || m.purgeRetried != 11 || got.Msg.GetAccount().GetPurgeBlockedReason() != reason || got.Msg.GetAccount().GetPurgeBlockedTime() == nil || !got.Msg.GetAccount().GetPurgePendingTime().AsTime().Equal(blocked) || !got.Msg.GetAccount().GetCreateUnresolved() {
		t.Fatalf("retry purge: %v %d %v", err, m.purgeRetried, got)
	}
	ctx := adminCtx
	if _, err := h.DeletePaymentAccountMapping(ctx, connect.NewRequest(&adminpb.DeletePaymentAccountMappingRequest{StoreId: "11"})); err != nil || m.mappingDeleted != 11 || m.actors[len(m.actors)-1].AdminID != 77 {
		t.Fatalf("delete mapping: %v %v %v", err, m.mappingDeleted, m.actors)
	}
	if _, err := h.DeletePaymentAccountMapping(context.Background(), connect.NewRequest(&adminpb.DeletePaymentAccountMappingRequest{StoreId: "11"})); !errors.Is(err, rpc.ErrNoPrincipal) {
		t.Fatalf("沒有管理員 principal 時預期 rpc.ErrNoPrincipal：%v", err)
	}
	if _, err := h.DeletePaymentAccountMapping(ctx, connect.NewRequest(&adminpb.DeletePaymentAccountMappingRequest{StoreId: "0"})); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("壞的 store_id 不是 invalid argument：%v", err)
	}
}

// 三個寫入方法把管理員 ID 與「這個 RPC 的 procedure 名稱」交給模組當後台紀錄的動作（D62）；procedure 名稱來自 Connect 的 req.Spec()，所以走真的 Connect handler。
// mutation：handler 把 Action 寫死或留空 → 動作不是 procedure 名稱，失敗；少傳管理員 ID → AdminID 不是 77，失敗。
func TestWriteHandlersPassTheAdminAndTheProcedureNameAsTheAuditAction(t *testing.T) {
	m := &paymentAdminModule{account: payment.Account{StoreID: 11, Provider: "stripe", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	path, handler := paymentv1alpha1connect.NewPaymentAdminServiceHandler(NewAdmin(m))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 77})))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := paymentv1alpha1connect.NewPaymentAdminServiceClient(srv.Client(), srv.URL)
	ctx := context.Background()
	if _, err := c.RetryPaymentAccountCreate(ctx, connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "11"})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RetryPaymentAccountPurge(ctx, connect.NewRequest(&adminpb.RetryPaymentAccountPurgeRequest{StoreId: "11"})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeletePaymentAccountMapping(ctx, connect.NewRequest(&adminpb.DeletePaymentAccountMappingRequest{StoreId: "11"})); err != nil {
		t.Fatal(err)
	}
	want := []payment.AdminActor{
		{AdminID: 77, Action: paymentv1alpha1connect.PaymentAdminServiceRetryPaymentAccountCreateProcedure},
		{AdminID: 77, Action: paymentv1alpha1connect.PaymentAdminServiceRetryPaymentAccountPurgeProcedure},
		{AdminID: 77, Action: paymentv1alpha1connect.PaymentAdminServiceDeletePaymentAccountMappingProcedure},
	}
	if !slices.Equal(m.actors, want) {
		t.Fatalf("actors=%v，預期 %v", m.actors, want)
	}
}

type resolveCall struct {
	actor    payment.AdminActor
	kind     payment.AnomalyKind
	id       uuid.UUID
	outcome  payment.AnomalyOutcome
	reason   payment.AnomalyResolutionReason
	note     string
	evidence string
}

func (m *paymentAdminModule) ListPayments(_ context.Context, f payment.PaymentFilter, after *payment.RecordCursor, size int32) (payment.PaymentRecordPage, error) {
	m.paymentFilter, m.gotAfter, m.gotSize = f, after, size
	return m.payments, m.err
}

func (m *paymentAdminModule) ListPaymentTransfers(_ context.Context, f payment.TransferFilter, after *payment.RecordCursor, size int32) (payment.TransferRecordPage, error) {
	m.transferFilter, m.gotAfter, m.gotSize = f, after, size
	return m.transfers, m.err
}

func (m *paymentAdminModule) ListPaymentRefunds(_ context.Context, f payment.RefundFilter, after *payment.RecordCursor, size int32) (payment.RefundRecordPage, error) {
	m.refundFilter, m.gotAfter, m.gotSize = f, after, size
	return m.refunds, m.err
}

func (m *paymentAdminModule) ResolveAnomaly(_ context.Context, actor payment.AdminActor, kind payment.AnomalyKind, id uuid.UUID, outcome payment.AnomalyOutcome, reason payment.AnomalyResolutionReason, note, evidence string) error {
	m.resolved = append(m.resolved, resolveCall{actor, kind, id, outcome, reason, note, evidence})
	return m.err
}

func adminCtx(id int64) context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: id})
}

func TestListPaymentsMapsEveryFieldAndPaginatesWithAFilterBoundToken(t *testing.T) {
	id := uuid.MustParse("01930000-0000-7000-8000-000000000001")
	next := uuid.MustParse("01930000-0000-7000-8000-000000000000")
	paid := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
	m := &paymentAdminModule{payments: payment.PaymentRecordPage{
		Records: []payment.PaymentRecord{{
			Payment:           payment.Payment{ID: id, Orders: []payment.PaymentOrderReceivable{{ID: uuid.MustParse("01930000-0000-7000-8000-0000000000d1"), OrderNo: "O1", StoreID: 7, Amount: 70}, {ID: uuid.MustParse("01930000-0000-7000-8000-0000000000d2"), OrderNo: "O2", StoreID: 9, Amount: 50}}, BuyerID: 8, Amount: 120, Provider: "stripe", Status: payment.PaymentSucceeded, PaidAt: &paid, LastError: "card_declined", CreatedAt: paid},
			ExternalPaymentID: "pi_1", FailureReason: "", UpdatedAt: paid,
		}},
		Next: &payment.RecordCursor{ID: next},
	}}
	h := NewAdmin(m)
	st := paymentpb.PaymentStatus_PAYMENT_STATUS_SUCCEEDED
	got, err := h.ListPayments(adminCtx(1), connect.NewRequest(&adminpb.ListPaymentsRequest{Status: &st, OrderNo: "O1", PageSize: 5}))
	if err != nil {
		t.Fatal(err)
	}
	if m.paymentFilter.Status != payment.PaymentSucceeded || m.paymentFilter.OrderNo != "O1" || m.gotSize != 5 || m.gotAfter != nil {
		t.Fatalf("模組收到 filter=%v after=%v size=%d", m.paymentFilter, m.gotAfter, m.gotSize)
	}
	r := got.Msg.GetPayments()[0]
	if len(r.GetOrders()) != 2 || r.GetOrders()[1].GetOrderNo() != "O2" || r.GetOrders()[1].GetStoreId() != "9" || r.GetOrders()[1].GetAmount() != 50 || r.GetOrders()[0].GetId() == "" {
		t.Fatalf("orders=%v", r.GetOrders())
	}
	if r.GetId() != id.String() || r.GetOrderNo() != "O1" || r.GetStoreId() != "7" || r.GetBuyerId() != "8" || r.GetAmount() != 120 || r.GetProvider() != "stripe" || r.GetExternalPaymentId() != "pi_1" ||
		r.GetStatus() != st || r.GetPaidTime() == nil || r.GetLastError() != "card_declined" || r.GetCreateTime() == nil || r.GetUpdateTime() == nil {
		t.Fatalf("payment=%v", r)
	}
	if got.Msg.GetNextPageToken() == "" {
		t.Fatal("有下一頁卻沒有 token")
	}
	// token 帶回同一組篩選：排序鍵原樣交給模組。
	if _, err := h.ListPayments(adminCtx(1), connect.NewRequest(&adminpb.ListPaymentsRequest{Status: &st, OrderNo: "O1", PageToken: got.Msg.GetNextPageToken()})); err != nil {
		t.Fatal(err)
	}
	if m.gotAfter == nil || m.gotAfter.ID != next {
		t.Fatalf("排序鍵 %v", m.gotAfter)
	}
	// 換了篩選的 token 不能用。
	_, err = h.ListPayments(adminCtx(1), connect.NewRequest(&adminpb.ListPaymentsRequest{Status: &st, OrderNo: "O2", PageToken: got.Msg.GetNextPageToken()}))
	if codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("換篩選的 token err=%v", err)
	}
}

func TestListPaymentsRejectsAnUnspecifiedStatusInsteadOfTreatingItAsNoFilter(t *testing.T) {
	m := &paymentAdminModule{}
	un := paymentpb.PaymentStatus_PAYMENT_STATUS_UNSPECIFIED
	_, err := NewAdmin(m).ListPayments(adminCtx(1), connect.NewRequest(&adminpb.ListPaymentsRequest{Status: &un}))
	if codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("err=%v", err)
	}
}

func TestListRefundsAndTransfersMapStatusesAndReversalIDs(t *testing.T) {
	rid := uuid.MustParse("01930000-0000-7000-8000-0000000000a1")
	vid := uuid.MustParse("01930000-0000-7000-8000-0000000000b1")
	tid := uuid.MustParse("01930000-0000-7000-8000-0000000000c1")
	oid := uuid.MustParse("01930000-0000-7000-8000-0000000000d1")
	pid := uuid.MustParse("01930000-0000-7000-8000-0000000000e1")
	at := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
	m := &paymentAdminModule{
		refunds: payment.RefundRecordPage{Records: []payment.RefundRecord{{
			Refund:    payment.PaymentRefund{ID: rid, PaymentOrderID: oid, RequestKey: "case-1", Amount: 50, Auto: true, Status: payment.RefundSucceeded, ReversalAmount: 40, ReversalStatus: payment.ReversalAnomaly, TopupStatus: payment.TopupNone, FailureReason: "x", CreatedAt: at},
			PaymentID: pid, OrderNo: "O1", ExternalRefundID: "re_1",
			Reversals: []payment.ReversalRecord{{ID: vid, Seq: 0, Amount: 40, Status: payment.ReversalAnomaly, Reason: "lookup", CreatedAt: at}},
		}}},
		transfers: payment.TransferRecordPage{Records: []payment.TransferRecord{{
			Transfer:  payment.PaymentTransfer{ID: tid, PaymentOrderID: oid, StoreID: 7, Amount: 100, ReversedAmount: 40, Status: payment.TransferAnomaly, FailureReason: "mismatch", CreatedAt: at},
			PaymentID: pid, OrderNo: "O1", ExternalTransferID: "tr_1",
		}}},
	}
	h := NewAdmin(m)
	rr, err := h.ListPaymentRefunds(adminCtx(1), connect.NewRequest(&adminpb.ListPaymentRefundsRequest{AnomalyOnly: true, OrderNo: "O1"}))
	if err != nil {
		t.Fatal(err)
	}
	if !m.refundFilter.AnomalyOnly || m.refundFilter.OrderNo != "O1" || m.refundFilter.Status != "" {
		t.Fatalf("filter=%v", m.refundFilter)
	}
	r := rr.Msg.GetRefunds()[0]
	if r.GetId() != rid.String() || r.GetOrderNo() != "O1" || !r.GetAuto() || r.GetStatus() != paymentpb.PaymentRefundStatus_PAYMENT_REFUND_STATUS_SUCCEEDED ||
		r.GetReversalStatus() != paymentpb.PaymentReversalStatus_PAYMENT_REVERSAL_STATUS_ANOMALY || r.GetTopupStatus() != paymentpb.PaymentTopupStatus_PAYMENT_TOPUP_STATUS_NONE ||
		r.GetExternalRefundId() != "re_1" || r.GetPaymentOrderId() != oid.String() || r.GetPaymentId() != pid.String() || r.GetRequestKey() != "case-1" || len(r.GetReversals()) != 1 || r.GetReversals()[0].GetId() != vid.String() ||
		r.GetReversals()[0].GetStatus() != paymentpb.PaymentReversalStatus_PAYMENT_REVERSAL_STATUS_ANOMALY || r.GetReversals()[0].GetReason() != "lookup" {
		t.Fatalf("refund=%v", r)
	}
	tr, err := h.ListPaymentTransfers(adminCtx(1), connect.NewRequest(&adminpb.ListPaymentTransfersRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	x := tr.Msg.GetTransfers()[0]
	if x.GetId() != tid.String() || x.GetStoreId() != "7" || x.GetReversedAmount() != 40 || x.GetStatus() != paymentpb.PaymentTransferStatus_PAYMENT_TRANSFER_STATUS_ANOMALY ||
		x.GetExternalTransferId() != "tr_1" || x.GetPaymentOrderId() != oid.String() || x.GetPaymentId() != pid.String() || x.GetFailureReason() != "mismatch" || x.GetOrderNo() != "O1" {
		t.Fatalf("transfer=%v", x)
	}
	if tr.Msg.GetNextPageToken() != "" {
		t.Fatal("最後一頁不該有 token")
	}
}

func TestResolvePaymentAnomalyPassesKindIDOutcomeReasonNoteAndTheCallingAdmin(t *testing.T) {
	id := uuid.MustParse("01930000-0000-7000-8000-0000000000a1")
	m := &paymentAdminModule{}
	mux := http.NewServeMux()
	path, handler := paymentv1alpha1connect.NewPaymentAdminServiceHandler(NewAdmin(m))
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 77})))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := paymentv1alpha1connect.NewPaymentAdminServiceClient(srv.Client(), srv.URL)
	req := &adminpb.ResolvePaymentAnomalyRequest{
		Kind: paymentpb.PaymentAnomalyKind_PAYMENT_ANOMALY_KIND_REVERSAL, Id: id.String(),
		Outcome: paymentpb.PaymentAnomalyOutcome_PAYMENT_ANOMALY_OUTCOME_SUCCEEDED, Evidence: "trr_1",
		Reason: paymentpb.PaymentAnomalyResolutionReason_PAYMENT_ANOMALY_RESOLUTION_REASON_PROVIDER_EXECUTED_NOT_RECORDED, Note: "  在 Stripe dashboard 核對過  ",
	}
	if _, err := c.ResolvePaymentAnomaly(context.Background(), connect.NewRequest(req)); err != nil {
		t.Fatal(err)
	}
	if len(m.resolved) != 1 {
		t.Fatalf("calls=%v", m.resolved)
	}
	got := m.resolved[0]
	want := resolveCall{
		actor: payment.AdminActor{AdminID: 77, Action: paymentv1alpha1connect.PaymentAdminServiceResolvePaymentAnomalyProcedure},
		kind:  payment.AnomalyReversal, id: id, outcome: payment.AnomalySucceeded, reason: payment.ResolutionProviderExecutedNotRecorded,
		note: "在 Stripe dashboard 核對過", evidence: "trr_1",
	}
	if got != want {
		t.Fatalf("call=%+v，預期 %+v", got, want)
	}
}

// 缺原因代碼、備註空白或超過上限都在進模組之前回 InvalidArgument，而且指到出錯的欄位。
// mutation：handler 不檢查 reason（或 note）→ 對應的案例失敗；handler 不 trim note → 空白備註的案例失敗；拿掉長度上限 → 過長的案例失敗。
func TestResolvePaymentAnomalyRequiresAReasonAndANonBlankBoundedNote(t *testing.T) {
	id := uuid.MustParse("01930000-0000-7000-8000-0000000000a1").String()
	ok := paymentpb.PaymentAnomalyResolutionReason_PAYMENT_ANOMALY_RESOLUTION_REASON_PROVIDER_NOT_EXECUTED
	base := func() *adminpb.ResolvePaymentAnomalyRequest {
		return &adminpb.ResolvePaymentAnomalyRequest{
			Kind: paymentpb.PaymentAnomalyKind_PAYMENT_ANOMALY_KIND_PAYMENT, Id: id,
			Outcome: paymentpb.PaymentAnomalyOutcome_PAYMENT_ANOMALY_OUTCOME_FAILED, Reason: ok, Note: "確認沒有執行",
		}
	}
	for name, c := range map[string]struct {
		mutate func(*adminpb.ResolvePaymentAnomalyRequest)
		field  string
	}{
		"reason 沒設": {func(r *adminpb.ResolvePaymentAnomalyRequest) { r.Reason = 0 }, "reason"},
		"note 是空字串": {func(r *adminpb.ResolvePaymentAnomalyRequest) { r.Note = "" }, "note"},
		"note 只有空白": {func(r *adminpb.ResolvePaymentAnomalyRequest) { r.Note = " \t\u3000 " }, "note"},
		"note 超過上限": {func(r *adminpb.ResolvePaymentAnomalyRequest) {
			r.Note = strings.Repeat("字", payment.AnomalyResolutionNoteMaxRunes+1)
		}, "note"},
	} {
		m := &paymentAdminModule{}
		req := base()
		c.mutate(req)
		_, err := NewAdmin(m).ResolvePaymentAnomaly(adminCtx(1), connect.NewRequest(req))
		if codeOf(err) != connect.CodeInvalidArgument || len(m.resolved) != 0 {
			t.Errorf("%s：err=%v 模組呼叫=%v", name, err, m.resolved)
			continue
		}
		if !strings.Contains(fieldViolations(err), c.field) {
			t.Errorf("%s：錯誤沒有指到 %s：%v", name, c.field, err)
		}
	}
	exact := base()
	exact.Note = strings.Repeat("字", payment.AnomalyResolutionNoteMaxRunes)
	m := &paymentAdminModule{}
	if _, err := NewAdmin(m).ResolvePaymentAnomaly(adminCtx(1), connect.NewRequest(exact)); err != nil || len(m.resolved) != 1 {
		t.Fatalf("剛好上限應該通過：err=%v 呼叫=%v", err, m.resolved)
	}
}

func TestResolvePaymentAnomalyRejectsUnspecifiedKindOutcomeAndBadIDsBeforeTheModule(t *testing.T) {
	id := uuid.MustParse("01930000-0000-7000-8000-0000000000a1").String()
	reason := paymentpb.PaymentAnomalyResolutionReason_PAYMENT_ANOMALY_RESOLUTION_REASON_OTHER
	for name, req := range map[string]*adminpb.ResolvePaymentAnomalyRequest{
		"kind 沒設":    {Id: id, Outcome: paymentpb.PaymentAnomalyOutcome_PAYMENT_ANOMALY_OUTCOME_FAILED, Reason: reason, Note: "n"},
		"outcome 沒設": {Kind: paymentpb.PaymentAnomalyKind_PAYMENT_ANOMALY_KIND_PAYMENT, Id: id, Reason: reason, Note: "n"},
		"id 不是 uuid": {Kind: paymentpb.PaymentAnomalyKind_PAYMENT_ANOMALY_KIND_PAYMENT, Id: "1", Outcome: paymentpb.PaymentAnomalyOutcome_PAYMENT_ANOMALY_OUTCOME_FAILED, Reason: reason, Note: "n"},
	} {
		m := &paymentAdminModule{}
		_, err := NewAdmin(m).ResolvePaymentAnomaly(adminCtx(1), connect.NewRequest(req))
		if codeOf(err) != connect.CodeInvalidArgument || len(m.resolved) != 0 {
			t.Errorf("%s：err=%v 模組呼叫=%v", name, err, m.resolved)
		}
	}
}

// fieldViolations 取出錯誤的 BadRequest 欄位路徑（以空白連接）。
func fieldViolations(err error) string {
	var out []string
	for _, d := range rpc.ConnectError(context.Background(), err).Details() {
		v, derr := d.Value()
		if derr != nil {
			continue
		}
		if bad, ok := v.(*errdetails.BadRequest); ok {
			for _, f := range bad.GetFieldViolations() {
				out = append(out, f.GetField())
			}
		}
	}
	return strings.Join(out, " ")
}
