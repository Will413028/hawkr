package usecase

import (
	"context"
	"errors"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// 作廢軸的測試替身
//
// 🔴 每一支 fake 的預設都是**拒絕**，不是回零值：本軸的錯誤形態幾乎全是靜默的
// （狀態沒推、票沒送、確認沒做），回零值會讓那些漏接在測試裡看起來完全正常。
// ---------------------------------------------------------------------------

type voidDocs struct {
	domain.DocumentRepository
	byID         map[int64]domain.Document
	byRequestNo  map[string]domain.Document
	voidingCalls int
	voidedCalls  int
	restoreCalls int
	cancelCalls  []int64
	markVoidedOK bool
	restoreOK    bool
	order        *[]string
}

func (f *voidDocs) GetByID(_ context.Context, id int64) (domain.Document, error) {
	d, ok := f.byID[id]
	if !ok {
		return domain.Document{}, errors.New("voidDocs: no such document")
	}
	return d, nil
}

func (f *voidDocs) GetByProviderRequestNo(_ context.Context, no string) (domain.Document, error) {
	d, ok := f.byRequestNo[no]
	if !ok {
		return domain.Document{}, bizNotFound()
	}
	return d, nil
}

func (f *voidDocs) MarkVoiding(_ context.Context, _ int64, _ time.Time) (bool, error) {
	f.voidingCalls++
	*f.order = append(*f.order, "doc_voiding")
	return true, nil
}

func (f *voidDocs) MarkVoided(_ context.Context, _ int64, _ time.Time) (bool, error) {
	f.voidedCalls++
	*f.order = append(*f.order, "doc_voided")
	return f.markVoidedOK, nil
}

func (f *voidDocs) RestoreToIssued(_ context.Context, _ int64, _ time.Time) (bool, error) {
	f.restoreCalls++
	*f.order = append(*f.order, "doc_restore_issued")
	return f.restoreOK, nil
}

func (f *voidDocs) CancelBeforeIssue(_ context.Context, id int64, _, _ string, _ time.Time) (bool, error) {
	f.cancelCalls = append(f.cancelCalls, id)
	*f.order = append(*f.order, "doc_cancel_before_issue")
	return true, nil
}

type voidOps struct {
	domain.OperationRepository
	byID      map[int64]domain.Operation
	byDoc     map[int64][]domain.Operation
	claimOK   bool
	failCalls int
	waitCalls int
	doneCalls int
	// recheckDoneCalls 與 doneCalls 分開計，兩者才分得出走了哪一條 lane。
	recheckDoneCalls int
	cancelCalls      []int64
	promoted         []int64
	promoteTo        string
	attempts         []int32
	exhaustCalls     int
	unresolved       []string
	order            *[]string
	// gotProviderCode 是 #3082 的斷言材料。
	gotProviderCode string
}

func (f *voidOps) GetByID(_ context.Context, id int64) (domain.Operation, error) {
	op, ok := f.byID[id]
	if !ok {
		return domain.Operation{}, errors.New("voidOps: no such operation")
	}
	return op, nil
}

func (f *voidOps) ListByDocument(_ context.Context, id int64) ([]domain.Operation, error) {
	return f.byDoc[id], nil
}

func (f *voidOps) Claim(_ context.Context, _ int64, _, _ time.Time) (bool, error) {
	*f.order = append(*f.order, "op_claim")
	return f.claimOK, nil
}

func (f *voidOps) MarkFailed(_ context.Context, _ int64, _ string, _ time.Time) (bool, error) {
	f.failCalls++
	*f.order = append(*f.order, "op_failed_permanent")
	return true, nil
}

// MarkFailedWithProviderResult **記下 provider 兩欄**（#3082）：AMEGO 的碼從
// invoice_status_history.reason_code 移到 invoice_operations.provider_code。
// 只計數的替身對「碼有沒有真的落到對的欄位」完全沒有反應。
func (f *voidOps) MarkFailedWithProviderResult(_ context.Context, in domain.MarkOperationProviderResult) (bool, error) {
	f.failCalls++
	f.gotProviderCode = in.ProviderCode
	*f.order = append(*f.order, "op_failed_permanent")
	return true, nil
}

// MarkWaitingConfirmation 鏡射 SQL 的 allowed-from
// `IN ('SENDING','PROVIDER_STATUS_RECHECK')` —— 後者是 INV.rc 查不到結果時
// 「繼續等」的那條路（J31），少了它 scheduleVoidRecheck 必然 rows=0。
func (f *voidOps) MarkWaitingConfirmation(_ context.Context, in domain.MarkOperationUnknown) (bool, error) {
	op := f.byID[in.ID]
	if op.Status != codemeta.InvoiceOperationStatusSending.Value &&
		op.Status != codemeta.InvoiceOperationStatusProviderStatusRecheck.Value {
		return false, nil
	}
	f.waitCalls++
	if op.UnknownSince == nil {
		at := in.Now
		op.UnknownSince = &at
	}
	op.Status = codemeta.InvoiceOperationStatusWaitingConfirmation.Value
	f.byID[in.ID] = op
	*f.order = append(*f.order, "op_waiting_confirmation")
	return true, nil
}

// MarkDone **鏡射 SQL 的 allowed-from**（`AND status = 'SENDING'`）——那是 INV.op lane。
//
// 🔴 原本無條件回 true，於是「從 INV.rc lane 呼叫它」這個真實缺陷在測試裡看不見：
// ClaimInvoiceRecheck 已把 DB status 蓋成 PROVIDER_STATUS_RECHECK ⇒ 真實 DB 上
// rows=0，而呼叫端把 rows 丟給 `_`。替身不鏡射守衛 ⇒ 驗的是替身不是系統。
func (f *voidOps) MarkDone(_ context.Context, id int64, _ time.Time) (bool, error) {
	if f.byID[id].Status != codemeta.InvoiceOperationStatusSending.Value {
		return false, nil
	}
	f.doneCalls++
	f.setStatus(id, codemeta.InvoiceOperationStatusSucceeded.Value)
	*f.order = append(*f.order, "op_succeeded")
	return true, nil
}

// MarkRecheckResolved 是 INV.rc lane 的收尾（db/queries 的 ⑧），
// allowed-from `AND status = 'PROVIDER_STATUS_RECHECK'`。
func (f *voidOps) MarkRecheckResolved(_ context.Context, in domain.MarkRecheckResult) (bool, error) {
	if f.byID[in.ID].Status != codemeta.InvoiceOperationStatusProviderStatusRecheck.Value {
		return false, nil
	}
	f.recheckDoneCalls++
	f.setStatus(in.ID, in.NextStatus)
	*f.order = append(*f.order, "op_succeeded")
	return true, nil
}

func (f *voidOps) setStatus(id int64, status string) {
	op := f.byID[id]
	op.Status = status
	f.byID[id] = op
}

// settledCalls 是「operation 真的被推到 SUCCEEDED」的次數，**不分 lane**。
// 斷言用它而不是 doneCalls：後者會把「走錯 lane」誤判成沒收尾，
// 前者才是呼叫端真正關心的不變式。
func (f *voidOps) settledCalls() int { return f.doneCalls + f.recheckDoneCalls }

// enterRecheckLane 模擬 INV.rc 的真實時序：ClaimInvoiceRecheck 先把 DB 上的 status
// 蓋成 PROVIDER_STATUS_RECHECK，**再**把 claim 之前讀到的快照傳進 ConfirmVoid。
// 回傳的就是那個快照 —— 它的 Status 停在 claim 之前的值，這是常態不是 fixture 寫錯。
func (f *voidOps) enterRecheckLane(id int64) domain.Operation {
	snapshot := f.byID[id]
	snapshot.Status = codemeta.InvoiceOperationStatusWaitingConfirmation.Value
	f.setStatus(id, codemeta.InvoiceOperationStatusProviderStatusRecheck.Value)
	return snapshot
}

func (f *voidOps) CancelBeforeIssue(_ context.Context, id int64, _, _ string, _ time.Time) (bool, error) {
	f.cancelCalls = append(f.cancelCalls, id)
	*f.order = append(*f.order, "op_cancel_before_issue")
	return true, nil
}

func (f *voidOps) Promote(_ context.Context, id int64, next string, _, _ time.Time) (bool, error) {
	f.promoted = append(f.promoted, id)
	f.promoteTo = next
	*f.order = append(*f.order, "op_promote")
	return true, nil
}

type voidCorrs struct {
	domain.CorrectionRequestRepository
	open     map[int64]domain.CorrectionRequest
	rejected []domain.RejectCorrection
	rejectOK bool
	order    *[]string
}

func (f *voidCorrs) GetOpenByDocument(_ context.Context, id int64) (domain.CorrectionRequest, error) {
	c, ok := f.open[id]
	if !ok {
		return domain.CorrectionRequest{}, bizNotFoundCorrection()
	}
	return c, nil
}

func (f *voidCorrs) Reject(_ context.Context, in domain.RejectCorrection) (bool, error) {
	f.rejected = append(f.rejected, in)
	*f.order = append(*f.order, "corr_rejected")
	return f.rejectOK, nil
}

type voidProvider struct {
	ProviderClient
	voidErr   error
	sentTo    []string
	detail    *amego.InvoiceDetail
	detailErr error
	queried   []string
}

func (f *voidProvider) VoidInvoice(_ context.Context, _ amego.Credential, req amego.VoidRequest) (*amego.VoidAccepted, error) {
	f.sentTo = append(f.sentTo, req.CancelInvoiceNumber)
	if f.voidErr != nil {
		return nil, f.voidErr
	}
	return &amego.VoidAccepted{Accepted: true}, nil
}

func (f *voidProvider) QueryInvoiceByNumber(_ context.Context, _ amego.Credential, n string) (*amego.InvoiceDetail, error) {
	f.queried = append(f.queried, n)
	return f.detail, f.detailErr
}

// bizNotFound／bizNotFoundCorrection 產生 repo 層對「查無列」用的業務碼錯誤。
//
// 用真的業務碼而不是 errors.New：usecase 靠 isBizCode 分辨「沒有」與「查詢失敗」，
// 而那兩者的處置相反（前者略過、後者整筆失敗）。fake 回錯型別的話這個分支永遠測不到。
func bizNotFound() error {
	return pkgerrors.NewBusiness(bizcode.InvoiceDocumentNotFound, "not found")
}

func bizNotFoundCorrection() error {
	return pkgerrors.NewBusiness(bizcode.InvoiceCorrectionNotAllowed, "no open correction")
}

// voidHarness 把作廢軸接起來。
type voidHarness struct {
	uc     *Voiding
	adjs   *voidAdjs
	docs   *voidDocs
	ops    *voidOps
	corrs  *voidCorrs
	prov   *voidProvider
	outbox *fakeOutbox
	calls  *[]string
}

const (
	voidDocID = int64(5001)
	voidOpID  = int64(6001)
	voidOrder = "O123456789"
	voidInvNo = "ZA10068421"
	voidAdjID = int64(7001)
)

// voidAdjIDVar 是 voidAdjID 的可取址版本（Operation.AdjustmentID 是 *int64）。
var voidAdjIDVar = voidAdjID

// voidAdjs 記下作廢軸把 adjustment 推到哪幾格（#3073）。
//
// 🔴 這一段在 #3073 之前不存在，而症狀是：dev 上 12 筆 VOID adjustment 全部停在
// PENDING，operation 早就進了終態、document 也已經 VOIDED —— 那張表的狀態機
// 從來沒有被驅動過，且沒有任何東西看得到。
type voidAdjs struct {
	domain.AdjustmentRepository
	adj      domain.Adjustment
	steps    []string
	reasons  []string
	markFail bool // true ⇒ 每一格都回 rows=0
}

func (f *voidAdjs) GetByID(context.Context, int64) (domain.Adjustment, error) { return f.adj, nil }

func (f *voidAdjs) MarkSending(context.Context, int64, time.Time) (bool, error) {
	f.steps = append(f.steps, "SENDING")
	return !f.markFail, nil
}

func (f *voidAdjs) MarkCompleted(context.Context, domain.MarkAdjustmentCompleted) (bool, error) {
	f.steps = append(f.steps, "COMPLETED")
	return !f.markFail, nil
}

func (f *voidAdjs) MarkFailedPermanent(_ context.Context, _ int64, reasonCode string, _ time.Time) (bool, error) {
	f.steps = append(f.steps, "FAILED_PERMANENT")
	f.reasons = append(f.reasons, reasonCode)
	return !f.markFail, nil
}

func (f *voidAdjs) MarkWaitingConfirmation(context.Context, int64, time.Time) (bool, error) {
	f.steps = append(f.steps, "WAITING_CONFIRMATION")
	return !f.markFail, nil
}

func (f *voidAdjs) MarkFailedUnknown(context.Context, int64, time.Time) (bool, error) {
	f.steps = append(f.steps, "FAILED_UNKNOWN")
	return !f.markFail, nil
}

func newVoidHarness(t *testing.T) *voidHarness {
	t.Helper()
	seq := &[]string{}
	orderRef := voidOrder
	invNo := voidInvNo
	doc := domain.Document{
		ID: voidDocID, IssuerID: 700001, SourceType: "ORDER", SourceRef: &orderRef,
		DocumentKind: "B2C", SnapshotVersion: 1, IssueAttemptSeq: 1,
		Status: codemeta.InvoiceDocumentStatusVoiding.Value, ProviderInvoiceNo: &invNo,
	}
	docs := &voidDocs{
		byID:         map[int64]domain.Document{voidDocID: doc},
		byRequestNo:  map[string]domain.Document{},
		markVoidedOK: true, restoreOK: true, order: seq,
	}
	ops := &voidOps{
		byID: map[int64]domain.Operation{voidOpID: {
			ID: voidOpID, DocumentID: voidDocID, AdjustmentID: &voidAdjIDVar,
			OperationType: codemeta.InvoiceOperationTypeVoid.Value,
			Status:        codemeta.InvoiceOperationStatusSending.Value,
			MaxAttempts:   13,
		}},
		byDoc: map[int64][]domain.Operation{}, claimOK: true, order: seq,
	}
	corrs := &voidCorrs{open: map[int64]domain.CorrectionRequest{}, rejectOK: true, order: seq}
	adjs := &voidAdjs{adj: domain.Adjustment{
		ID: voidAdjID, DocumentID: voidDocID,
		AdjustmentType: codemeta.InvoiceAdjustmentTypeVoid.Value,
		Status:         codemeta.InvoiceAdjustmentStatusPending.Value,
		TriggerKind:    codemeta.InvoiceAdjustmentTriggerBuyerCorrection.Value,
	}}
	prov := &voidProvider{}
	outbox := &fakeOutbox{}
	uc := NewVoiding(directTx{}, docs, ops, corrs,
		&fakeAccountRepo{account: domain.ProviderAccount{ID: 42}, taxID: "12345678", appKey: "k"},
		&fakeAudit{}, prov,
		outbox,
		// issuer_ref_id 與 PK 刻意取不同的值：P3 的 payload 只能用前者（store_id），
		// 用 PK 頂替的症狀是下游解析不到賣家而全程零錯誤。
		&fakeIssuerRepo{issuer: domain.Issuer{ID: 700001, IssuerRefID: 2636840881}},
		adjs,
		IssuanceConfig{MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute,
			IssueLease: 120 * time.Second, RecheckBase: 30 * time.Second},
		nil,
	).WithClock(func() time.Time { return testNow })
	return &voidHarness{uc: uc, adjs: adjs, docs: docs, ops: ops, corrs: corrs, prov: prov, outbox: outbox, calls: seq}
}

// ---------------------------------------------------------------------------
// SubmitVoid
// ---------------------------------------------------------------------------

// 受理（f0501 回 code:0）⇒ operation 進 WAITING_CONFIRMATION，document **留在 VOIDING**。
//
// 🔴 受理不等於已作廢：f0501 回 0 之後 invoice_query 的 invoice_type 仍是 C0401，
// 作廢只是排進 AMEGO 的 wait[]。這裡若直接寫 VOIDED，R10 會提早觸發 ⇒ 原票還沒真的
// 作廢就開出新票 ⇒ 同一筆交易兩張有效發票，而發票是法定憑證、無法回收。
func TestSubmitVoid_AcceptedGoesToWaitingConfirmationNotVoided(t *testing.T) {
	h := newVoidHarness(t)

	if err := h.uc.SubmitVoid(context.Background(), voidOpID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := h.prov.sentTo; len(got) != 1 || got[0] != voidInvNo {
		t.Fatalf("送出的發票號碼 = %v, want [%s]", got, voidInvNo)
	}
	if h.ops.waitCalls != 1 {
		t.Errorf("WAITING_CONFIRMATION 寫了 %d 次, want 1", h.ops.waitCalls)
	}
	if h.docs.voidedCalls != 0 {
		t.Error("受理當下就把 document 寫成 VOIDED —— 那會讓 R10 在原票還沒作廢時開出新票")
	}
}

// R05：AMEGO 明確拒絕作廢（3050141 已存在折讓）。
//
// 同一事務內四件事，缺一都會留下不一致的組合。特別是 ②：document 留在 VOIDING
// 的話沒有任何掃描器會把它撿回來（VOIDING 不在任一候選集合內）。
func TestSubmitVoid_RejectedRestoresDocumentAndRejectsCorrection(t *testing.T) {
	h := newVoidHarness(t)
	h.corrs.open[voidDocID] = domain.CorrectionRequest{
		ID: 8001, Status: codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value}
	h.prov.voidErr = &amego.APIError{Code: "3050141", Msg: "已存在折讓單", Class: amego.ClassPermanent}

	if err := h.uc.SubmitVoid(context.Background(), voidOpID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.ops.failCalls != 1 {
		t.Errorf("operation 沒進 FAILED_PERMANENT")
	}
	if h.docs.restoreCalls != 1 {
		t.Fatalf("document 沒有從 VOIDING 回到 ISSUED —— 它會永遠停在 VOIDING（無掃描器）")
	}
	if len(h.corrs.rejected) != 1 {
		t.Fatalf("correction 沒被拒絕，買家會一直看到「處理中」")
	}
	if got := h.corrs.rejected[0].ReasonCode; got != codemeta.InvoiceCorrectionRejectReasonAllowanceExists.Value {
		t.Errorf("reject_reason_code = %q, want ALLOWANCE_EXISTS（3050141）", got)
	}
}

// 3050126（已超過修改期限）對映 PAST_VOID_DEADLINE。
//
// 與上一支分開測而不是表格化：兩個碼各自來自不同的權威來源段落，合成一支測試
// 會讓其中一個改錯時另一個仍然綠。
func TestSubmitVoid_RejectedMapsPastVoidDeadline(t *testing.T) {
	h := newVoidHarness(t)
	h.corrs.open[voidDocID] = domain.CorrectionRequest{ID: 8001}
	h.prov.voidErr = &amego.APIError{Code: "3050126", Msg: "已超過修改期限", Class: amego.ClassPermanent}

	if err := h.uc.SubmitVoid(context.Background(), voidOpID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := h.corrs.rejected[0].ReasonCode; got != codemeta.InvoiceCorrectionRejectReasonPastVoidDeadline.Value {
		t.Errorf("reject_reason_code = %q, want PAST_VOID_DEADLINE（3050126）", got)
	}
}

// 3050121「開立中」是 Retryable，**不是** R05。
//
// 🔴 把它當永久失敗會讓買家在法定期限內明明作廢得了卻被拒——而 3050121 只代表
// 上一個異動還在 AMEGO 的佇列裡，等 30~60 秒就好。
func TestSubmitVoid_InProgressCodeRetriesNotRejects(t *testing.T) {
	h := newVoidHarness(t)
	h.corrs.open[voidDocID] = domain.CorrectionRequest{ID: 8001}
	h.prov.voidErr = &amego.APIError{Code: "3050121", Msg: "開立中", Class: amego.ClassRetryable}

	if err := h.uc.SubmitVoid(context.Background(), voidOpID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.corrs.rejected) != 0 {
		t.Fatal("3050121 被當成拒絕 —— 買家會在還能作廢時被擋掉")
	}
	if h.docs.restoreCalls != 0 {
		t.Error("重試中不該把 document 還原成 ISSUED（下一輪 claim 的述詞含 SENDING）")
	}
}

// R05 必須把 R04 建好的重開票一起收掉。
//
// 不收的後果是兩個靜默終點：那張 PENDING 的重開票沒有任何 operation 會撿它
// （REISSUE operation 停在 CREATED），而賣家面與 ADM 會把它顯示成「待開立」。
func TestSubmitVoid_RejectedCancelsPendingReissue(t *testing.T) {
	h := newVoidHarness(t)
	h.corrs.open[voidDocID] = domain.CorrectionRequest{ID: 8001}
	reissueNo := domain.ProviderRequestNoForOrder(voidOrder, 1, 2)
	h.docs.byRequestNo[reissueNo] = domain.Document{
		ID: 5002, Status: codemeta.InvoiceDocumentStatusPending.Value}
	h.ops.byDoc[5002] = []domain.Operation{{
		ID: 6002, DocumentID: 5002,
		OperationType: codemeta.InvoiceOperationTypeReissue.Value,
		Status:        codemeta.InvoiceOperationStatusCreated.Value,
	}}
	h.prov.voidErr = &amego.APIError{Code: "3050141", Class: amego.ClassPermanent}

	if err := h.uc.SubmitVoid(context.Background(), voidOpID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.docs.cancelCalls) != 1 || h.docs.cancelCalls[0] != 5002 {
		t.Errorf("重開票沒被取消：%v", h.docs.cancelCalls)
	}
	if len(h.ops.cancelCalls) != 1 || h.ops.cancelCalls[0] != 6002 {
		t.Errorf("REISSUE operation 沒被取消：%v", h.ops.cancelCalls)
	}
}

// ---------------------------------------------------------------------------
// ConfirmVoid
// ---------------------------------------------------------------------------

// invoice_type 還是 C0401 ⇒ 還在 wait[] 佇列裡，這是**正常**的中間狀態。
func TestConfirmVoid_StillPendingReschedules(t *testing.T) {
	h := newVoidHarness(t)
	h.prov.detail = &amego.InvoiceDetail{InvoiceNumber: voidInvNo, InvoiceType: "C0401"}

	if err := h.uc.ConfirmVoid(context.Background(), h.ops.enterRecheckLane(voidOpID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.docs.voidedCalls != 0 {
		t.Fatal("C0401 就寫 VOIDED —— 那是把「動作受理」當成「憑證已作廢」")
	}
	if h.ops.waitCalls != 1 {
		t.Errorf("沒有排回重查迴圈（waitCalls=%d）", h.ops.waitCalls)
	}
}

// P3：作廢確認成功必須發 invoice.voided（#3055 的橫向稽查）。
//
// 🔴 這一支從**外緣**（ConfirmVoid）進去，不直接呼叫 emitVoided：
// 「函式寫對了但沒有人呼叫它」是本服務反覆出現的那個 class，而直接呼叫 emitter 的
// 測試對「接線被拿掉」完全無感 —— 本支的第一版就是那樣，變異驗證沒轉紅才發現。
//
// 陰性對照在 TestConfirmVoid_StillPendingReschedules：C0401（還沒真的作廢）那條路
// 一列 outbox 都不該有，所以「每次確認都發」的寫法會讓它轉紅。
func TestConfirmVoid_EmitsVoidedEvent(t *testing.T) {
	h := newVoidHarness(t)
	h.prov.detail = &amego.InvoiceDetail{InvoiceNumber: voidInvNo, InvoiceType: "C0501"}

	if err := h.uc.ConfirmVoid(context.Background(), h.ops.enterRecheckLane(voidOpID)); err != nil {
		t.Fatalf("ConfirmVoid: %v", err)
	}
	if len(h.outbox.inserted) != 1 {
		t.Fatalf("outbox 應恰有一列 invoice.voided，實得 %d —— 少了它，買家手上那張憑證"+
			"失效了卻沒有任何人告訴他", len(h.outbox.inserted))
	}
	ev := h.outbox.inserted[0]
	if ev.RoutingKey != pkgmq.RoutingKeyInvoiceVoided ||
		ev.EventType != string(pkgmq.EventTypeInvoiceVoided) ||
		ev.Exchange != pkgmq.ExchangeInvoice {
		t.Fatalf("routing key／event_type／exchange = %q／%q／%q",
			ev.RoutingKey, ev.EventType, ev.Exchange)
	}
	// 🔴 store_id 必須是 issuer_ref_id（2636840881），不是 issuer PK（700001）：
	// dispatcher 拿它去 ResolveSellerAccountID，填 PK 的症狀是解析失敗、
	// notify_seller_account_id 恆 "0"、通知靜默不送，全程零錯誤零 DLQ。
	if !containsBytes(ev.Payload, `"store_id":"2636840881"`) {
		t.Errorf("store_id 不是 issuer_ref_id：%s", ev.Payload)
	}
	if containsBytes(ev.Payload, `"store_id":"700001"`) {
		t.Error("store_id 取到 issuer PK 了")
	}
	if !containsBytes(ev.Payload, `"voided_invoice_no":"`+voidInvNo+`"`) {
		t.Errorf("voided_invoice_no 不是被作廢的那個號碼：%s", ev.Payload)
	}
}

// 陰性對照：還沒真的作廢（C0401）⇒ 一列 outbox 都不該有。
// 沒有這一支，一個「每次確認都發」的實作也會讓上面那支通過。
func TestConfirmVoid_StillPendingEmitsNothing(t *testing.T) {
	h := newVoidHarness(t)
	h.prov.detail = &amego.InvoiceDetail{InvoiceNumber: voidInvNo, InvoiceType: "C0401"}

	if err := h.uc.ConfirmVoid(context.Background(), h.ops.enterRecheckLane(voidOpID)); err != nil {
		t.Fatalf("ConfirmVoid: %v", err)
	}
	if len(h.outbox.inserted) != 0 {
		t.Fatalf("憑證還沒作廢就發了 invoice.voided：%v —— 買家會收到一封說明"+
			"還沒發生的事的通知", h.outbox.inserted)
	}
}

// invoice_type = C0501 ⇒ 作廢真的完成。同一事務內 document → VOIDED
// 並把 REISSUE operation 推到 READY。
//
// 🔴 推進那一步是重開票**唯一**的執行者。少了它，原票已作廢、新票永遠不開、
// correction 永遠停在 VOID_REISSUE_IN_PROGRESS，而且完全不報錯。
func TestConfirmVoid_ConfirmedMarksVoidedAndPromotesReissue(t *testing.T) {
	h := newVoidHarness(t)
	h.prov.detail = &amego.InvoiceDetail{InvoiceNumber: voidInvNo, InvoiceType: "C0501"}
	reissueNo := domain.ProviderRequestNoForOrder(voidOrder, 1, 2)
	h.docs.byRequestNo[reissueNo] = domain.Document{ID: 5002}
	h.ops.byDoc[5002] = []domain.Operation{{
		ID: 6002, DocumentID: 5002,
		OperationType: codemeta.InvoiceOperationTypeReissue.Value,
		Status:        codemeta.InvoiceOperationStatusCreated.Value,
	}}

	if err := h.uc.ConfirmVoid(context.Background(), h.ops.enterRecheckLane(voidOpID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.docs.voidedCalls != 1 {
		t.Fatalf("document 沒進 VOIDED")
	}
	if len(h.ops.promoted) != 1 || h.ops.promoted[0] != 6002 {
		t.Fatalf("REISSUE operation 沒被推進：%v —— 新票永遠不會開", h.ops.promoted)
	}
	if h.ops.promoteTo != codemeta.InvoiceOperationStatusReady.Value {
		t.Errorf("推到 %q, want READY", h.ops.promoteTo)
	}
}

// 沒有重開票（退款折讓路徑建的 VOID）⇒ 確認成功但不推任何東西，且不得報錯。
func TestConfirmVoid_NoReissueIsNotAnError(t *testing.T) {
	h := newVoidHarness(t)
	h.prov.detail = &amego.InvoiceDetail{InvoiceNumber: voidInvNo, InvoiceType: "C0501"}

	if err := h.uc.ConfirmVoid(context.Background(), h.ops.enterRecheckLane(voidOpID)); err != nil {
		t.Fatalf("退款折讓路徑沒有重開票，不該報錯：%v", err)
	}
	if len(h.ops.promoted) != 0 {
		t.Errorf("不該推進任何 operation：%v", h.ops.promoted)
	}
}

func (f *voidOps) UpdateAttempt(_ context.Context, _ int64, attempt int32, _ time.Time, _ *string, _ time.Time) error {
	f.attempts = append(f.attempts, attempt)
	*f.order = append(*f.order, "op_update_attempt")
	return nil
}

func (f *voidOps) MarkRetryExhausted(_ context.Context, _ int64, _ string, _ time.Time) (bool, error) {
	f.exhaustCalls++
	*f.order = append(*f.order, "op_retry_exhausted")
	return true, nil
}

// 3050123「已存在註銷」→ PROVIDER_STATE_CONFLICT，**不是** ALREADY_REPORTED。
//
// 這一支守的是 DEC-20260831-234750 的裁決結果。它曾經是錯的：設計書與 CodeMeta
// 都寫「已申報」，而 AMEGO 官方碼表、amego/codes.go 與同一份設計書的時序圖都寫
// 「已存在註銷」——碼義不是我方能定義的。
//
// 錯的後果不是資料瑕疵而是**誤導**：買家讀到「發票已申報」會去找國稅局，
// 而實際上該去找賣家釐清那筆註銷。
func TestSubmitVoid_AlreadyCancelledMapsProviderStateConflict(t *testing.T) {
	h := newVoidHarness(t)
	h.corrs.open[voidDocID] = domain.CorrectionRequest{ID: 8001}
	h.prov.voidErr = &amego.APIError{Code: "3050123", Msg: "指定發票號碼 已存在註銷", Class: amego.ClassManual}

	if err := h.uc.SubmitVoid(context.Background(), voidOpID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.corrs.rejected) != 1 {
		t.Fatalf("3050123 沒有落成 R05 的自動終局，correction 會停在進行中")
	}
	got := h.corrs.rejected[0].ReasonCode
	if got == codemeta.InvoiceCorrectionRejectReasonAlreadyReported.Value {
		t.Fatal("3050123 對映回 ALREADY_REPORTED —— 那是 DEC-20260831-234750 推翻的舊對映；" +
			"買家會讀到「發票已申報」而實際原因是「已存在註銷」")
	}
	if got != codemeta.InvoiceCorrectionRejectReasonProviderStateConflict.Value {
		t.Errorf("reject_reason_code = %q, want PROVIDER_STATE_CONFLICT", got)
	}
}

// ALREADY_REPORTED 不得由任何 f0501 回應碼觸發。
//
// 它的唯一寫入者是 R02 的預檢（ReportedStatus 純函式）。若哪天有人把它接回某個
// provider 碼，這一支會轉紅——那正是本次要防的回歸。
func TestVoidRejectReasonOf_NeverMapsToAlreadyReported(t *testing.T) {
	for _, code := range []string{"3050121", "3050122", "3050123", "3050125", "3050126", "3050131", "3050141", "9999999"} {
		if got := voidRejectReasonOf(code); got == codemeta.InvoiceCorrectionRejectReasonAlreadyReported.Value {
			t.Errorf("f0501 的 %s 對映到 ALREADY_REPORTED —— 該理由碼只能由 R02 預檢寫入", code)
		}
	}
}

func (f *voidOps) MarkRecheckUnresolved(_ context.Context, in domain.MarkRecheckResult) (bool, error) {
	f.unresolved = append(f.unresolved, in.NextStatus)
	*f.order = append(*f.order, "op_recheck_unresolved")
	return true, nil
}

// 作廢確認不得無限重查。
//
// 🔴 少了上界，「AMEGO 一直不把 invoice_type 翻成 C0501」會讓 document 永遠停在
// VOIDING —— 而 VOIDING 不在任何掃描器的候選集合裡、也不在 M7／M32 的指標涵蓋內
// （#2959 ⑧）⇒ 沒有任何訊號的永久停滯，買家看到的是「處理中」。
func TestConfirmVoid_GivesUpAfterWindow(t *testing.T) {
	h := newVoidHarness(t)
	h.prov.detail = &amego.InvoiceDetail{InvoiceNumber: voidInvNo, InvoiceType: "C0401"}
	old := testNow.Add(-25 * time.Hour) // > recheckGiveUpAfter（24h）
	op := h.ops.byID[voidOpID]
	op.UnknownSince = &old
	h.ops.byID[voidOpID] = op

	if err := h.uc.ConfirmVoid(context.Background(), op); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.ops.unresolved) != 1 {
		t.Fatalf("超過 24h 仍在無界重查（unresolved=%v）", h.ops.unresolved)
	}
	if h.ops.unresolved[0] != codemeta.InvoiceOperationStatusFailedUnknown.Value {
		t.Errorf("落點 = %q, want FAILED_UNKNOWN", h.ops.unresolved[0])
	}
	// 🔴 不得還原成 ISSUED：AMEGO 可能其實已經作廢了，還原會讓帳上有一張「有效」的廢票。
	if h.docs.restoreCalls != 0 {
		t.Error("結果未明卻把 document 還原成 ISSUED —— 那是在宣稱一件我方並不知道的事")
	}
	// 也不得寫 VOIDED：同理，反方向。
	if h.docs.voidedCalls != 0 {
		t.Error("結果未明卻把 document 寫成 VOIDED")
	}
}

// 窗口內仍在重查 —— 這是**正常**的中間狀態，不得提早放棄。
func TestConfirmVoid_WithinWindowKeepsWaiting(t *testing.T) {
	h := newVoidHarness(t)
	h.prov.detail = &amego.InvoiceDetail{InvoiceNumber: voidInvNo, InvoiceType: "C0401"}
	recent := testNow.Add(-2 * time.Hour)
	op := h.ops.byID[voidOpID]
	op.UnknownSince = &recent
	h.ops.byID[voidOpID] = op

	if err := h.uc.ConfirmVoid(context.Background(), op); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.ops.unresolved) != 0 {
		t.Fatalf("才 2 小時就放棄了 —— AMEGO 的 wait[] 佇列本來就會花時間")
	}
	if h.ops.waitCalls != 1 {
		t.Errorf("沒有排回重查迴圈（waitCalls=%d）", h.ops.waitCalls)
	}
}

// ---------------------------------------------------------------------------
// adjustment 狀態機（#3073）
// ---------------------------------------------------------------------------

// TestVoidAxis_AdvancesAdjustmentAlongsideOperation 守「作廢軸要推進 adjustment」。
//
// 🔴 這一段在 #3073 之前**整個不存在**，而症狀完全無聲：dev 上 12 筆 VOID adjustment
// 全部停在 PENDING，operation 早就進了 FAILED_PERMANENT／PROVIDER_STATUS_RECHECK、
// document 也已經 VOIDED／SUPERSEDED —— 那張表的狀態機從來沒有被驅動過。
// 沒有任何測試、任何指標、任何告警看得到它，因為在折讓執行軸落地之前
// 那張表在 dev 上也只有 12 列、沒有人在看。
func TestVoidAxis_AdvancesAdjustmentAlongsideOperation(t *testing.T) {
	t.Run("claim ⇒ SENDING", func(t *testing.T) {
		h := newVoidHarness(t)
		ok, err := h.uc.ClaimForVoidAt(context.Background(), voidOpID, testNow, testNow.Add(time.Minute))
		if err != nil || !ok {
			t.Fatalf("ClaimForVoidAt: ok=%v err=%v", ok, err)
		}
		if got := h.adjs.steps; len(got) != 1 || got[0] != "SENDING" {
			t.Errorf("adjustment 走到 %v，want [SENDING] —— "+
				"claim 了卻不推進 adjustment，它會永遠停在 PENDING", got)
		}
	})

	t.Run("f0501 受理 ⇒ WAITING_CONFIRMATION", func(t *testing.T) {
		h := newVoidHarness(t)
		if err := h.uc.SubmitVoid(context.Background(), voidOpID); err != nil {
			t.Fatalf("SubmitVoid: %v", err)
		}
		if got := h.adjs.steps; len(got) != 1 || got[0] != "WAITING_CONFIRMATION" {
			t.Errorf("adjustment 走到 %v，want [WAITING_CONFIRMATION] —— "+
				"f0501 回 code:0 只代表受理，作廢是非同步的", got)
		}
	})

	t.Run("被拒 ⇒ FAILED_PERMANENT ＋ 理由碼", func(t *testing.T) {
		h := newVoidHarness(t)
		// 3050141「已存在折讓單則不得作廢」是 Permanent，走 settleVoidRejected；
		// 本 harness 沒有 open correction ⇒ 走「退款作廢路徑」那一支。
		h.prov.voidErr = &amego.APIError{Code: "3050141", Class: amego.ClassPermanent, KnownCode: true}

		if err := h.uc.SubmitVoid(context.Background(), voidOpID); err != nil {
			t.Fatalf("SubmitVoid: %v", err)
		}
		if got := h.adjs.steps; len(got) != 1 || got[0] != "FAILED_PERMANENT" {
			t.Fatalf("adjustment 走到 %v，want [FAILED_PERMANENT]", got)
		}
		// chk_invadj_blocked_reason：FAILED_PERMANENT ⇒ blocked_reason_code 不得為空。
		if len(h.adjs.reasons) != 1 || h.adjs.reasons[0] == "" {
			t.Errorf("理由碼 = %v，不得為空 —— chk_invadj_blocked_reason 會當場 23514",
				h.adjs.reasons)
		}
	})
}

// TestVoidAxis_AdjustmentNoRowsIsNotAnError 守「歷史列不得讓作廢重新失敗」。
//
// #3073 之前建的 VOID adjustment 停在 PENDING，而它們的 operation 可能已經走完。
// 那些 operation 重投時會走到 advanceAdjustment 而 allowed-from 對不上 ⇒ rows=0。
// 回 error 會讓一批早就處理完的作廢重新變成失敗。
//
// ⚠️ **折讓軸不能這樣寫**：那裡的 adjustment 是主體（單號、三個金額欄、累計落帳
// 都掛在它身上），rows=0 代表狀態機破了、必須 rollback。同一張表在兩條軸上的角色不同。
func TestVoidAxis_AdjustmentNoRowsIsNotAnError(t *testing.T) {
	h := newVoidHarness(t)
	h.adjs.markFail = true

	ok, err := h.uc.ClaimForVoidAt(context.Background(), voidOpID, testNow, testNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("adjustment rows=0 讓 claim 失敗了：%v —— 一批歷史列會讓作廢全面卡住", err)
	}
	if !ok {
		t.Fatal("claim 回 false —— rows=0 不該影響 operation 的搶佔結果")
	}
}

// ---------------------------------------------------------------------------
// ConfirmVoid：document 已經越過 VOIDING（2026-09-03）
// ---------------------------------------------------------------------------

// 作廢確認過（VOIDED）之後又被重開票取代（SUPERSEDED），而這個 operation 因為某種
// 原因回到 recheck ⇒ MarkVoided 的 allowed-from 只有 VOIDING ⇒ rows=0。
//
// 🔴 原本這裡回 error，而 INV.rc 的迴圈只 log 一行就 continue、unknown_since 從不推進
// ⇒ scheduleVoidRecheck 的 24h 放棄門檻永遠到不了 ⇒ **一個沒有出口的迴圈，而且每一輪
// 都真的打一次 AMEGO**。dev 實測 4 筆列在單一 pod 的 43 分鐘內產生 84 次
// invoice.rc.confirm_void_failed，沒有任何告警看得到
// （invoice_adjustment_stuck_rows 對 WAITING_CONFIRMATION 刻意不計數）。
func TestConfirmVoid_SupersededDocumentSettlesInsteadOfLooping(t *testing.T) {
	h := newVoidHarness(t)
	doc := h.docs.byID[voidDocID]
	doc.Status = codemeta.InvoiceDocumentStatusSuperseded.Value
	h.docs.byID[voidDocID] = doc
	h.docs.markVoidedOK = false // allowed-from 只有 VOIDING
	h.prov.detail = &amego.InvoiceDetail{InvoiceNumber: voidInvNo, InvoiceType: "C0501"}

	if err := h.uc.ConfirmVoid(context.Background(), h.ops.enterRecheckLane(voidOpID)); err != nil {
		t.Fatalf("SUPERSEDED 的 document 讓 ConfirmVoid 回錯 ⇒ INV.rc 會永遠重試："+
			"%v", err)
	}
	if h.ops.settledCalls() != 1 {
		t.Errorf("operation 沒有被收尾（done=%d recheckDone=%d）⇒ 它會留在 PROVIDER_STATUS_RECHECK",
			h.ops.doneCalls, h.ops.recheckDoneCalls)
	}
	if len(h.adjs.steps) == 0 || h.adjs.steps[len(h.adjs.steps)-1] != "COMPLETED" {
		t.Errorf("adjustment 沒有到終點，steps=%v", h.adjs.steps)
	}
	// 事件不得重發：走過 VOIDED 的那一次事務裡已經發過了。
	if len(h.outbox.inserted) != 0 {
		t.Errorf("重發了 %d 筆 invoice.voided —— document 早就過了那一格，"+
			"當初那個事務就發過了", len(h.outbox.inserted))
	}
}

// 🔴 dev 實測（第 3 輪 INV58R3-ALW01）撞到的那條：document **已經是 VOIDED**。
//
// 時序是這樣壞掉的：
//  1. 主路徑成功把 document VOIDING → VOIDED、adjustment → COMPLETED、發了 invoice.voided
//  2. 但同一個事務裡的 operation 收尾走 MarkDone（守 SENDING），而此刻 DB 上的 status
//     是 ClaimInvoiceRecheck 蓋上去的 PROVIDER_STATUS_RECHECK ⇒ **rows=0**，
//     且該回傳被丟給 `_` ⇒ 完全無聲，還照樣寫了一條 SUCCEEDED 的假軌跡
//  3. 之後每一輪 INV.rc 都落到 `doc.Status == VOIDED` 這個 early-return 分支，
//     而它**只呼叫 promoteReissue 就 return nil** ⇒ operation 永遠留在候選集合裡
//
// 實測證據：document VOIDED、adjustment COMPLETED、outbox invoice.voided 已 done，
// 而 operation 停在 PROVIDER_STATUS_RECHECK，updated_at 每 60 秒前進一次、
// unknown_since 從不推進 ⇒ scheduleVoidRecheck 的 24h 放棄門檻永遠到不了。
func TestConfirmVoid_AlreadyVoidedStillSettlesOperation(t *testing.T) {
	h := newVoidHarness(t)
	doc := h.docs.byID[voidDocID]
	doc.Status = codemeta.InvoiceDocumentStatusVoided.Value
	h.docs.byID[voidDocID] = doc

	if err := h.uc.ConfirmVoid(context.Background(), h.ops.enterRecheckLane(voidOpID)); err != nil {
		t.Fatalf("ConfirmVoid: %v", err)
	}
	if h.ops.settledCalls() != 1 {
		t.Fatalf("document 已 VOIDED 的重投分支沒有收尾 operation（done=%d recheckDone=%d）"+
			"⇒ 這一列會永遠留在 INV.rc 的候選集合裡被每 60 秒 claim 一次",
			h.ops.doneCalls, h.ops.recheckDoneCalls)
	}
	if got := h.ops.byID[voidOpID].Status; got != codemeta.InvoiceOperationStatusSucceeded.Value {
		t.Errorf("operation status = %q，want SUCCEEDED", got)
	}
	// 陰性：這條路不得再打一次 AMEGO（作廢早就確認過了），也不得重發事件。
	if len(h.prov.queried) != 0 {
		t.Errorf("重投分支打了 %d 次 AMEGO（%v）—— document 已 VOIDED，沒有東西需要再查",
			len(h.prov.queried), h.prov.queried)
	}
	if len(h.outbox.inserted) != 0 {
		t.Errorf("重發了 %d 筆 invoice.voided —— 當初推進 VOIDED 的那個事務就發過了",
			len(h.outbox.inserted))
	}
}

// 鑑別力：rows=0 不是一律吞掉。document 停在**不該**出現的狀態時仍要硬錯，
// 否則上面那個修法就等於把 MarkVoided 的 CAS 整個關掉。
func TestConfirmVoid_UnexpectedStatusStillErrorsOnZeroRows(t *testing.T) {
	h := newVoidHarness(t)
	doc := h.docs.byID[voidDocID]
	doc.Status = codemeta.InvoiceDocumentStatusIssued.Value // 既不是 VOIDED 也不是 SUPERSEDED
	h.docs.byID[voidDocID] = doc
	h.docs.markVoidedOK = false
	h.prov.detail = &amego.InvoiceDetail{InvoiceNumber: voidInvNo, InvoiceType: "C0501"}

	if err := h.uc.ConfirmVoid(context.Background(), h.ops.enterRecheckLane(voidOpID)); err == nil {
		t.Fatal("document 在非預期狀態下 rows=0 卻沒有報錯 —— " +
			"那等於把 MarkVoided 的 allowed-from 檢查整個關掉")
	}
	if h.ops.settledCalls() != 0 {
		t.Errorf("非預期狀態卻把 operation 收尾了（done=%d recheckDone=%d）",
			h.ops.doneCalls, h.ops.recheckDoneCalls)
	}
}

// G15 在作廢軸：operation 放棄（→ FAILED_UNKNOWN）時 adjustment 必須一起離開
// WAITING_CONFIRMATION。少了它，adjustment 永遠停在那一格，而
// invoice_adjustment_stuck_rows 對該狀態刻意不計數 ⇒ 指標上完全不存在。
func TestConfirmVoid_GiveUpAlsoAdvancesAdjustment(t *testing.T) {
	h := newVoidHarness(t)
	h.prov.detail = &amego.InvoiceDetail{InvoiceNumber: voidInvNo, InvoiceType: "C0401"}
	old := testNow.Add(-25 * time.Hour)
	op := h.ops.byID[voidOpID]
	op.UnknownSince = &old
	h.ops.byID[voidOpID] = op

	if err := h.uc.ConfirmVoid(context.Background(), op); err != nil {
		t.Fatalf("ConfirmVoid: %v", err)
	}
	found := false
	for _, s := range h.adjs.steps {
		if s == "FAILED_UNKNOWN" {
			found = true
		}
	}
	if !found {
		t.Errorf("operation 放棄了，adjustment 卻沒跟著走（steps=%v）——"+
			"它會永遠停在 WAITING_CONFIRMATION，而 stuck 指標看不到那一格", h.adjs.steps)
	}
}
