//go:build !integration

package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// 折讓執行軸（G08–G12）
//
// 🔴 這個檔案在 #3073 的第一版**不存在**，而後果具體且必然：`ClaimForAllowanceAt`
// 漏了 `MarkSending`，於是 adjustment 停在 PENDING，而 MarkCompleted 的 allowed-from
// 是 (SENDING, WAITING_CONFIRMATION, FAILED_UNKNOWN) ⇒ **每一筆折讓都會在
// settleAllowanceCompleted 撞上 rows=0**——而那時 AMEGO 已經開好折讓單了。
// 沒有一支測試從 claim 走到 completed，所以那個洞從頭到尾沒有訊號。
// ---------------------------------------------------------------------------

const (
	alwOpID  = int64(6101)
	alwAdjID = int64(7101)
	alwDocID = int64(5101)
)

// alwAdjs 是折讓軸的 adjustment 替身：**真的維護狀態**。
//
// 只記 calls 的替身在這裡沒有鑑別力——本檔要驗的正是「狀態機的 allowed-from 對不對」，
// 而那需要替身真的拒絕非法轉換（同 §0.3：fake 的鑑別力來自它拒絕什麼）。
type alwAdjs struct {
	domain.AdjustmentRepository
	adj       domain.Adjustment
	lines     []domain.AdjustmentLine
	steps     []string
	nextNo    string
	completed bool
}

// ListByDocument 回**當前**的 adj，不是建構時的快照。
//
// 快照式的 fake 在這裡會製造假失敗：adjustmentOf 走的正是這一句，而 SubmitAllowance
// 的 resume checkpoint（「已經 COMPLETED 就不再打 AMEGO」）讀的就是它的回傳值。
func (f *alwAdjs) ListByDocument(context.Context, int64) ([]domain.Adjustment, error) {
	return []domain.Adjustment{f.adj}, nil
}
func (f *alwAdjs) GetByID(context.Context, int64) (domain.Adjustment, error) { return f.adj, nil }
func (f *alwAdjs) ListLines(context.Context, int64) ([]domain.AdjustmentLine, error) {
	return f.lines, nil
}
func (f *alwAdjs) NextAllowanceNumber(context.Context) (string, error) { return f.nextNo, nil }

// transition 逐字鏡射 SQL 的 allowed-from。對不上就回 rows=0。
func (f *alwAdjs) transition(to string, from ...string) bool {
	for _, s := range from {
		if f.adj.Status == s {
			f.steps = append(f.steps, to)
			f.adj.Status = to
			return true
		}
	}
	return false
}

// MarkSending 的 allowed-from 含 SENDING 自己（＝租約回收），逐字鏡射 SQL。
// 少了自轉移，「持有者送出後、寫回前失敗」的那一列就再也 claim 不回來。
func (f *alwAdjs) MarkSending(context.Context, int64, time.Time) (bool, error) {
	return f.transition(codemeta.InvoiceAdjustmentStatusSending.Value,
		codemeta.InvoiceAdjustmentStatusPending.Value,
		codemeta.InvoiceAdjustmentStatusSending.Value), nil
}

func (f *alwAdjs) MarkCompleted(context.Context, domain.MarkAdjustmentCompleted) (bool, error) {
	ok := f.transition(codemeta.InvoiceAdjustmentStatusCompleted.Value,
		codemeta.InvoiceAdjustmentStatusSending.Value,
		codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value,
		codemeta.InvoiceAdjustmentStatusFailedUnknown.Value)
	f.completed = f.completed || ok
	return ok, nil
}

func (f *alwAdjs) MarkWaitingConfirmation(context.Context, int64, time.Time) (bool, error) {
	return f.transition(codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value,
		codemeta.InvoiceAdjustmentStatusSending.Value), nil
}

func (f *alwAdjs) MarkFailedPermanent(context.Context, int64, string, time.Time) (bool, error) {
	return f.transition(codemeta.InvoiceAdjustmentStatusFailedPermanent.Value,
		codemeta.InvoiceAdjustmentStatusSending.Value,
		codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value), nil
}

func (f *alwAdjs) Reschedule(context.Context, int64, time.Time) (bool, error) {
	return f.transition(codemeta.InvoiceAdjustmentStatusPending.Value,
		codemeta.InvoiceAdjustmentStatusSending.Value), nil
}

func (f *alwAdjs) MarkFailedUnknown(context.Context, int64, time.Time) (bool, error) {
	return f.transition(codemeta.InvoiceAdjustmentStatusFailedUnknown.Value,
		codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value), nil
}

// alwDocs 記下反正規化落帳。
//
// 🔴 **它必須鏡射 SQL 的述詞，不能什麼都接受。** AddAllowanceTotal 是單句 CAS，
// 述詞 `allowance_total_amount + delta <= total_amount` 不成立時回 rows=0
// （不是 error）。恆回 true 的替身零鑑別力：呼叫端把那個 bool 丟掉時，**沒有任何
// 測試走得到那條路**——那正是它被丟掉五輪都沒有人發現的原因（2026-09-05 第 6 輪稽核）。
// fake 的鑑別力來自它**拒絕什麼**。
type alwDocs struct {
	domain.DocumentRepository
	doc          domain.Document
	allowanceAdd []int64
	lineAdd      map[int64]int64
}

func (f *alwDocs) GetByID(context.Context, int64) (domain.Document, error) { return f.doc, nil }
func (f *alwDocs) AddAllowanceTotal(_ context.Context, _ int64, amount int64, _ time.Time) (bool, error) {
	var sum int64
	for _, a := range f.allowanceAdd {
		sum += a
	}
	// 鏡射 chk_invdoc_allowance_cap ＋ CAS 述詞（0 ≤ 累計 ≤ total_amount）。
	if next := f.doc.AllowanceTotalAmount + sum + amount; next > f.doc.TotalAmount || next < 0 {
		return false, nil
	}
	f.allowanceAdd = append(f.allowanceAdd, amount)
	return true, nil
}
func (f *alwDocs) AddLineAllowancedAmount(_ context.Context, lineID, amount int64, _ time.Time) (bool, error) {
	if f.lineAdd == nil {
		f.lineAdd = map[int64]int64{}
	}
	f.lineAdd[lineID] += amount
	return true, nil
}

type alwOps struct {
	domain.OperationRepository
	op      domain.Operation
	claimOK bool
	claimed int
	doneCnt int
	// recheckDoneCnt 是「經 INV.rc lane 收尾」的次數，與 doneCnt 分開計：
	// 兩者合併就分不出「走對 lane」與「走錯 lane 但 fake 放行」。
	recheckDoneCnt int
	waitNextAt     time.Time
	// gotProvider* 是 #3082 的斷言材料：provider_code／provider_message
	// 在此之前從未被任何路徑寫入。
	gotProviderCode    string
	gotProviderMessage string

	// claim 的兩個時間參數（見 alwOps.Claim 的說明）。
	claimNow        time.Time
	claimLeaseUntil time.Time

	// recheckUnresolvedCnt／gotUnresolvedNextAt 是 J26／J17 落點的斷言材料。
	recheckUnresolvedCnt int
	gotUnresolvedNextAt  time.Time
	// gotResolved 留下 MarkRecheckResolved 的完整輸入：J24 的兩個專屬欄位
	// （cancel_evidence_kind／cancel_confirmed_at）只有在這裡看得到，
	// 而少了 evidence_kind 在真實 DB 上是 23514（chk_invop_admin_cancel_needs_confirm）。
	gotResolved *domain.MarkRecheckResult
}

func (f *alwOps) GetByID(context.Context, int64) (domain.Operation, error) { return f.op, nil }

// Claim **記下兩個時間參數**。替身的鑑別力來自它拒絕什麼——原本這兩格連名字都沒有，
// 於是 `ClaimForAllowanceAt` 把 (now, leaseUntil) 對調了整整一波都沒有任何測試出聲。
// 對應的斷言在 TestAllowance_ClaimPassesNowAndLeaseInSqlOrder。
func (f *alwOps) Claim(_ context.Context, _ int64, now, leaseUntil time.Time) (bool, error) {
	f.claimed++
	f.claimNow, f.claimLeaseUntil = now, leaseUntil
	if f.claimOK {
		f.op.Status = codemeta.InvoiceOperationStatusSending.Value
	}
	return f.claimOK, nil
}

// MarkDone **鏡射 SQL 的 allowed-from**（`AND status = 'SENDING'`）。
//
// 🔴 原本這裡無條件回 true，於是它對「呼叫時 status 根本不是 SENDING」零鑑別力
// ——而那正是真實缺陷的形狀：ConfirmAllowance 由 INV.rc 分派進來時 status 已是
// PROVIDER_STATUS_RECHECK，MarkDone 在 DB 上必然 rows=0，測試卻照樣全綠。
// 替身不鏡射真實守衛 ⇒ 測試驗的是替身，不是系統。
func (f *alwOps) MarkDone(context.Context, int64, time.Time) (bool, error) {
	if f.op.Status != codemeta.InvoiceOperationStatusSending.Value {
		return false, nil
	}
	f.doneCnt++
	f.op.Status = codemeta.InvoiceOperationStatusSucceeded.Value
	return true, nil
}

// MarkRecheckResolved 是 INV.rc lane 的收尾（⑧），allowed-from
// `AND status = 'PROVIDER_STATUS_RECHECK'`。同樣鏡射守衛，兩支才分得出誰推動了它。
func (f *alwOps) MarkRecheckResolved(_ context.Context, in domain.MarkRecheckResult) (bool, error) {
	if f.op.Status != codemeta.InvoiceOperationStatusProviderStatusRecheck.Value {
		return false, nil
	}
	f.recheckDoneCnt++
	f.op.Status = in.NextStatus
	got := in
	f.gotResolved = &got
	return true, nil
}

// 以下五支是失敗／未明落點。**必須逐一實作**：靠 embedded 介面的話它們是 nil，
// 走到就 panic —— 而 panic 在測試裡看起來像「fixture 寫壞了」，很容易被當成測試問題
// 而不是「這條路徑從來沒有人走過」。
func (f *alwOps) MarkFailed(context.Context, int64, string, time.Time) (bool, error) {
	f.op.Status = codemeta.InvoiceOperationStatusFailedPermanent.Value
	return true, nil
}

// MarkFailedWithProviderResult **記下 provider 兩欄**（#3082）。
//
// 🔴 只把狀態設成 FAILED_PERMANENT 的替身，對「provider_code 有沒有真的被寫入」
// 完全沒有反應 —— 而那兩欄在此之前**從未被任何路徑寫入**，正是本次要修的東西。
func (f *alwOps) MarkFailedWithProviderResult(_ context.Context, in domain.MarkOperationProviderResult) (bool, error) {
	f.op.Status = codemeta.InvoiceOperationStatusFailedPermanent.Value
	f.gotProviderCode = in.ProviderCode
	f.gotProviderMessage = in.ProviderMessage
	return true, nil
}

// MarkWaitingConfirmation **鏡射 SQL 的 allowed-from**
// `IN ('SENDING','PROVIDER_STATUS_RECHECK')`，並鏡射 `unknown_since` 的
// COALESCE 語意（第一次進入未明狀態才起算，之後不重寫）。
//
// 🔴 原本無條件回 true，於是「從 INV.rc lane 呼叫它」這個真實缺陷看不見：
// dev 第 3 輪實測到一列每分鐘被 claim 一次、unknown_since 永遠 NULL、
// 狀態永遠不變，而每一輪都真的打一次 AMEGO。
func (f *alwOps) MarkWaitingConfirmation(_ context.Context, in domain.MarkOperationUnknown) (bool, error) {
	if f.op.Status != codemeta.InvoiceOperationStatusSending.Value &&
		f.op.Status != codemeta.InvoiceOperationStatusProviderStatusRecheck.Value {
		return false, nil
	}
	f.op.Status = codemeta.InvoiceOperationStatusWaitingConfirmation.Value
	if f.op.UnknownSince == nil {
		at := in.Now
		f.op.UnknownSince = &at
	}
	f.waitNextAt = in.NextAttemptAt
	return true, nil
}

func (f *alwOps) MarkRetryExhausted(context.Context, int64, string, time.Time) (bool, error) {
	f.op.Status = codemeta.InvoiceOperationStatusFailedRetryExhausted.Value
	return true, nil
}

func (f *alwOps) UpdateAttempt(_ context.Context, _ int64, attempt int32, nextAt time.Time, _ *string, _ time.Time) error {
	f.op.AttemptCount = attempt
	f.op.NextAttemptAt = nextAt
	return nil
}

// MarkRecheckUnresolved **鏡射 SQL 的 allowed-from**（`AND status = 'PROVIDER_STATUS_RECHECK'`）。
//
// 🔴 原本無條件回 true，於是 G15 那一段「operation 到底有沒有真的被推走」看不出來
// ——而 P0-2 的症狀正是它從來沒被執行到（整個交易在前一句就 rollback 了）。
func (f *alwOps) MarkRecheckUnresolved(_ context.Context, in domain.MarkRecheckResult) (bool, error) {
	if f.op.Status != codemeta.InvoiceOperationStatusProviderStatusRecheck.Value {
		return false, nil
	}
	f.recheckUnresolvedCnt++
	f.op.Status = in.NextStatus
	f.gotUnresolvedNextAt = in.NextAttemptAt
	return true, nil
}

type alwHarness struct {
	uc   *Allowance
	adjs *alwAdjs
	docs *alwDocs
	ops  *alwOps
	prov *fakeProvider
	out  *fakeOutbox
	// audit 讓測試看得到「訊息到底寫進了哪一欄」——correlation_id 只有 varchar(64)，
	// 而 reason_detail 是 text，兩者混用就是 SQLSTATE 22001 的成因。
	audit *fakeAudit
}

func newAllowanceHarness(t *testing.T) *alwHarness {
	t.Helper()
	invNo := "AB12345678"
	orderRef := "O000000123"
	date := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	no, netEx, tax, gross := "A000000000001", int64(286), int64(14), int64(300)

	adjs := &alwAdjs{
		nextNo: no,
		adj: domain.Adjustment{
			ID: alwAdjID, DocumentID: alwDocID,
			AdjustmentType:  codemeta.InvoiceAdjustmentTypeAllowance.Value,
			Status:          codemeta.InvoiceAdjustmentStatusPending.Value,
			TriggerKind:     codemeta.InvoiceAdjustmentTriggerOrderRefund.Value,
			AllowanceNumber: &no, NetAmountExTax: &netEx, TaxAmount: &tax, GrossAmount: &gross,
		},
		lines: []domain.AdjustmentLine{{
			ID: 9101, AdjustmentID: alwAdjID, LineSeq: 1, SourceLineID: ptrInt64(8001),
			OriginalInvoiceNo: invNo, OriginalInvoiceDate: &date,
			OriginalDescription: "有機高麗菜", Quantity: 1,
			UnitPriceExTax: 286, NetAmountExTax: 286, TaxAmount: 14, TaxCategory: 1,
		}},
	}
	docs := &alwDocs{doc: domain.Document{
		ID: alwDocID, IssuerID: 700001, SourceRef: &orderRef,
		Status:       codemeta.InvoiceDocumentStatusIssued.Value,
		DocumentKind: codemeta.InvoiceDocumentKindB2c.Value,
		TotalAmount:  1000, TaxAmount: 0, AllowanceTotalAmount: 0,
		ProviderInvoiceNo: &invNo, InvoiceDateTaipei: &date,
	}}
	ops := &alwOps{claimOK: true, op: domain.Operation{
		ID: alwOpID, DocumentID: alwDocID, AdjustmentID: ptrInt64(alwAdjID),
		OperationType: codemeta.InvoiceOperationTypeAllowance.Value,
		Status:        codemeta.InvoiceOperationStatusReady.Value,
		MaxAttempts:   13,
	}}
	prov := &fakeProvider{allowanceFn: func(amego.AllowanceRequest) (*amego.AllowanceResult, error) {
		return &amego.AllowanceResult{}, nil
	}}
	out := &fakeOutbox{}

	audit := &fakeAudit{}
	uc := NewAllowance(directTx{}, docs, ops, adjs,
		&fakeAccountRepo{account: domain.ProviderAccount{ID: 42}, taxID: "12345678", appKey: "k"},
		&fakeIssuerRepo{issuer: domain.Issuer{ID: 700001, IssuerRefID: 2685914682}},
		audit, out, prov,
		IssuanceConfig{MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute,
			IssueLease: 120 * time.Second, RecheckBase: 30 * time.Second},
		true, nil).WithClock(func() time.Time { return testNow })

	return &alwHarness{uc: uc, adjs: adjs, docs: docs, ops: ops, prov: prov, out: out, audit: audit}
}

// TestAllowance_ClaimThenSubmitReachesCompleted 是**整條軸唯一的端到端測試**。
//
// 🔴 它抓到的第一個 bug：`ClaimForAllowanceAt` 漏了 `MarkSending` ⇒ adjustment 停在
// PENDING ⇒ MarkCompleted 的 allowed-from 對不上 ⇒ 每一筆折讓都失敗，而 AMEGO 那邊
// 已經開好單了。只驗 SubmitAllowance（跳過 claim）的測試看不到它，因為那樣寫的話
// fixture 會直接把 adjustment 設成 SENDING。
func TestAllowance_ClaimThenSubmitReachesCompleted(t *testing.T) {
	h := newAllowanceHarness(t)
	ctx := context.Background()

	ok, err := h.uc.ClaimForAllowanceAt(ctx, alwOpID, testNow, testNow.Add(2*time.Minute))
	if err != nil || !ok {
		t.Fatalf("ClaimForAllowanceAt: ok=%v err=%v", ok, err)
	}
	if got := h.adjs.adj.Status; got != codemeta.InvoiceAdjustmentStatusSending.Value {
		t.Fatalf("claim 之後 adjustment = %q，want SENDING —— "+
			"停在 PENDING 的話 MarkCompleted 的 allowed-from 對不上，每一筆折讓都會失敗", got)
	}

	if err := h.uc.SubmitAllowance(ctx, alwOpID); err != nil {
		t.Fatalf("SubmitAllowance: %v", err)
	}
	if !h.adjs.completed {
		t.Fatalf("adjustment 沒有進 COMPLETED（走過 %v）—— AMEGO 已經開好折讓單了，"+
			"我方的帳卻是失敗的", h.adjs.steps)
	}

	// I4：累計與逐行落帳必須在同一交易內。
	if len(h.docs.allowanceAdd) != 1 || h.docs.allowanceAdd[0] != 300 {
		t.Errorf("allowance_total_amount 累加 = %v，want [300]", h.docs.allowanceAdd)
	}
	// 逐行加的是**含稅**額（明細存未稅）：少加稅額會讓下一次折讓的剩餘基數被高估。
	if got := h.docs.lineAdd[8001]; got != 300 {
		t.Errorf("line 8001 的 allowanced_amount 加了 %d，want 300（286 未稅 ＋ 14 稅）", got)
	}

	// P4 事件必須在同一交易內寫 outbox。
	if len(h.out.inserted) != 1 {
		t.Fatalf("outbox 有 %d 筆事件，want 1（invoice.allowance.issued）", len(h.out.inserted))
	}
	var ev map[string]any
	if err := json.Unmarshal(h.out.inserted[0].Payload, &ev); err != nil {
		t.Fatalf("payload 解不開：%v", err)
	}
	for _, k := range []string{"notification_ref_id", "invoice_document_id", "cumulative_allowance_amount"} {
		if _, ok := ev[k]; !ok {
			t.Errorf("payload 缺 %q —— notification 模板讀不到就走 Debug + ACK，訊息直接消失", k)
		}
	}
	if got := ev["cumulative_allowance_amount"]; got != "300" {
		t.Errorf("cumulative_allowance_amount = %v，want \"300\"（本次落帳後的累計）", got)
	}
}

// TestAllowance_SubmitWithoutClaimIsRejected 守「沒有租約就不准送」。
//
// 兩個副本同時打 AMEGO ⇒ 重複折讓，而折讓單是法定憑證、開錯不可逆。
func TestAllowance_SubmitWithoutClaimIsRejected(t *testing.T) {
	h := newAllowanceHarness(t)
	// operation 停在 READY（沒 claim 過）。
	if err := h.uc.SubmitAllowance(context.Background(), alwOpID); err == nil {
		t.Fatal("沒有租約也送出去了 —— 兩個副本會同時打 AMEGO g0401")
	}
	if h.prov.allowanceCalls != 0 {
		t.Fatalf("打了 %d 次 AMEGO", h.prov.allowanceCalls)
	}
}

// TestAllowance_AlreadyCompletedDoesNotResend 守 resume checkpoint。
//
// 折讓單號是我方自產且**送出前先落庫**，重送會讓 AMEGO 端出現第二張同號憑證。
func TestAllowance_AlreadyCompletedDoesNotResend(t *testing.T) {
	h := newAllowanceHarness(t)
	ctx := context.Background()
	if _, err := h.uc.ClaimForAllowanceAt(ctx, alwOpID, testNow, testNow.Add(2*time.Minute)); err != nil {
		t.Fatalf("claim: %v", err)
	}
	h.adjs.adj.Status = codemeta.InvoiceAdjustmentStatusCompleted.Value

	if err := h.uc.SubmitAllowance(ctx, alwOpID); err != nil {
		t.Fatalf("SubmitAllowance: %v", err)
	}
	if h.prov.allowanceCalls != 0 {
		t.Fatalf("已經 COMPLETED 還打了 %d 次 AMEGO —— 第二張同號折讓單", h.prov.allowanceCalls)
	}
}

// TestAllowance_FailureLandingsMatchTheStateMachine 守 G10／G11／G12 三條失敗邊。
//
// 🔴 這三條與 G09 一樣要求 adjustment 在 SENDING（allowed-from 逐字取自 SQL）。
// 少了 claim 那一格的 MarkSending，它們**全部**會 rows=0——也就是說一筆折讓不論
// 成功或失敗都寫不進帳。這支測試從 claim 走起，正是為了讓那個前提被真的執行到。
func TestAllowance_FailureLandingsMatchTheStateMachine(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{"永久失敗 ⇒ FAILED_PERMANENT",
			&amego.APIError{Code: "4040173", Class: amego.ClassPermanent, KnownCode: true},
			codemeta.InvoiceAdjustmentStatusFailedPermanent.Value},

		{"可重試 ⇒ 退回 PENDING 等下一輪",
			&amego.APIError{Code: "3050121", Class: amego.ClassRetryable, KnownCode: true},
			codemeta.InvoiceAdjustmentStatusPending.Value},

		// 🔴 結果不明**不得重送**：我方自產的折讓單號已經送出去了，重送會讓 AMEGO 端
		// 出現第二張同號憑證。唯一安全的下一步是查詢（G13／G14 由 recheck 軸收斂）。
		// 非 APIError（ctx 取消、transport 尚未分類）走同一個落點：折讓可能已經送達。
		{"結果不明 ⇒ WAITING_CONFIRMATION（不重送）",
			errors.New("context deadline exceeded"),
			codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newAllowanceHarness(t)
			ctx := context.Background()
			sendErr := c.err
			h.prov.allowanceFn = func(amego.AllowanceRequest) (*amego.AllowanceResult, error) {
				return nil, sendErr
			}

			if _, err := h.uc.ClaimForAllowanceAt(ctx, alwOpID, testNow, testNow.Add(2*time.Minute)); err != nil {
				t.Fatalf("claim: %v", err)
			}
			// SubmitAllowance 的契約：所有業務落點都回 nil（狀態已經寫定）。
			if err := h.uc.SubmitAllowance(ctx, alwOpID); err != nil {
				t.Fatalf("SubmitAllowance 回了 error：%v —— 業務落點必須回 nil，"+
					"否則 worker 的 attempt 簿記會打在錯的前提上", err)
			}
			if got := h.adjs.adj.Status; got != c.want {
				t.Errorf("adjustment = %q，want %q（走過 %v）", got, c.want, h.adjs.steps)
			}
			// 失敗路徑一律不得寫 P4：買家會收到一張根本沒開成的折讓單通知。
			if len(h.out.inserted) != 0 {
				t.Errorf("失敗卻寫了 %d 筆 outbox 事件", len(h.out.inserted))
			}
		})
	}
}

// TestAllowance_ClaimReclaimsAdjustmentStuckInSending 守 adjustment 的**租約回收**。
//
// 🔴 dev 實測（第 3 輪 INV58R3-ALW06）的死結：持有者在送出之後、狀態寫回之前失敗
// （當時是 status_history 撞 SQLSTATE 22001 讓整個交易 rollback），於是
// adjustment 停在 SENDING、operation 被退回 RETRY_SCHEDULED。
// 下一輪 INV.op claim 時 MarkSending（原本只認 PENDING）rows=0
// ⇒ ClaimForAllowanceAt 回 error ⇒ **claim 交易整個 rollback、operation 原地不動**
// ⇒ 每 10 秒重試一次、每次都失敗。實測 next_attempt_at 過期 13 分鐘、
// 約 80 次嘗試，兩列的 updated_at 一動也沒動。
//
// 而 invoice_adjustment_stuck_rows 的述詞**刻意不含 SENDING**，理由寫的是
// 「那個有自己的執行者」—— 執行者認不得 SENDING 時那句話不成立，
// 於是這種卡死在指標上完全不存在。
func TestAllowance_ClaimReclaimsAdjustmentStuckInSending(t *testing.T) {
	h := newAllowanceHarness(t)
	ctx := context.Background()
	// 前一個持有者留下的殘局：adjustment 已 SENDING，operation 已被退回可搶佔。
	h.adjs.adj.Status = codemeta.InvoiceAdjustmentStatusSending.Value
	h.ops.op.Status = codemeta.InvoiceOperationStatusRetryScheduled.Value

	ok, err := h.uc.ClaimForAllowanceAt(ctx, alwOpID, testNow, testNow.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("adjustment 停在 SENDING 讓 claim 回了 error：%v\n"+
			"—— 那會讓整個 claim 交易 rollback，operation 原地不動，"+
			"INV.op 每 10 秒重試一次、每次都失敗，而且沒有任何指標看得到", err)
	}
	if !ok {
		t.Fatalf("claim 回 false —— 這一列再也回不來了（steps=%v）", h.adjs.steps)
	}
	if got := h.adjs.adj.Status; got != codemeta.InvoiceAdjustmentStatusSending.Value {
		t.Errorf("adjustment = %q，want SENDING", got)
	}
}

// 鑑別力：自轉移不是把 CAS 關掉。真正的終態仍然必須擋下來，
// 否則 AMEGO 會收到一張我方帳上已經結案的折讓單。
func TestAllowance_ClaimRefusesAdjustmentInTerminalState(t *testing.T) {
	for _, st := range []string{
		codemeta.InvoiceAdjustmentStatusCompleted.Value,
		codemeta.InvoiceAdjustmentStatusFailedPermanent.Value,
		codemeta.InvoiceAdjustmentStatusCancelled.Value,
	} {
		t.Run(st, func(t *testing.T) {
			h := newAllowanceHarness(t)
			h.adjs.adj.Status = st
			h.ops.op.Status = codemeta.InvoiceOperationStatusRetryScheduled.Value
			if _, err := h.uc.ClaimForAllowanceAt(context.Background(), alwOpID,
				testNow, testNow.Add(2*time.Minute)); err == nil {
				t.Fatalf("adjustment 在 %s 卻讓 claim 通過 —— "+
					"那等於把 MarkSending 的 allowed-from 整個關掉", st)
			}
		})
	}
}

// TestAllowance_LongProviderMessageDoesNotBreakFailureHandling 守「失敗處置本身不會失敗」。
//
// 🔴 dev 實測（第 3 輪 INV58R3-ALW06）：把發票號改成 AMEGO 端不存在的號去送折讓，
// operation 的 last_error 落成
// `ERROR: value too long for type character varying(64) (SQLSTATE 22001)`
// —— **真正的 provider 錯誤訊息被一句資料庫錯誤取代了**。
//
// 成因：invoice_status_history 只有 reason_code(varchar 48)／reason_detail(text)／
// correlation_id(varchar 64) 三欄，而 appendStatusHistory 當時只開放 reasonCode 與
// correlationID 兩個入口 ⇒ 折讓軸六處把人看的錯誤訊息塞進 correlationID。
// AMEGO 的訊息帶 endpoint ＋ 碼 ＋ 中文原文，超過 64 是常態不是極端值
// ⇒ 折讓一失敗，**失敗處置的交易自己就 rollback**。
//
// 這支測試的鑑別力來自 fakeAudit 鏡射了那三個長度約束（見 fake_test.go）。
func TestAllowance_LongProviderMessageDoesNotBreakFailureHandling(t *testing.T) {
	// 逐字仿 AMEGO 實際回應的形狀：endpoint ＋ 碼 ＋ 中文原文。
	longMsg := "查無此發票號碼，請確認發票號碼與開立日期是否正確，" +
		"或該發票已被作廢／已逾可折讓期間，詳見折讓作業說明第三節"
	for _, c := range []struct {
		name string
		err  *amego.APIError
		want string
	}{
		{"可重試 ⇒ 退回 PENDING",
			&amego.APIError{Endpoint: "g0401", Code: "3050121", Msg: longMsg,
				Class: amego.ClassRetryable, KnownCode: true},
			codemeta.InvoiceAdjustmentStatusPending.Value},
		{"永久失敗 ⇒ FAILED_PERMANENT",
			&amego.APIError{Endpoint: "g0401", Code: "4040173", Msg: longMsg,
				Class: amego.ClassPermanent, KnownCode: true},
			codemeta.InvoiceAdjustmentStatusFailedPermanent.Value},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newAllowanceHarness(t)
			ctx := context.Background()
			if n := len(c.err.Error()); n <= 64 {
				t.Fatalf("這支測試的前提是訊息超過 correlation_id 的 varchar(64)，"+
					"實際只有 %d 字元 —— 前提不成立就零鑑別力", n)
			}
			h.prov.allowanceFn = func(amego.AllowanceRequest) (*amego.AllowanceResult, error) {
				return nil, c.err
			}
			if _, err := h.uc.ClaimForAllowanceAt(ctx, alwOpID, testNow, testNow.Add(2*time.Minute)); err != nil {
				t.Fatalf("claim: %v", err)
			}
			if err := h.uc.SubmitAllowance(ctx, alwOpID); err != nil {
				t.Fatalf("長訊息讓失敗處置自己失敗了：%v\n"+
					"—— 這正是 dev 上 SQLSTATE 22001 的形狀：錢退了、折讓沒開成，"+
					"而連「為什麼沒開成」都寫不進去", err)
			}
			if got := h.adjs.adj.Status; got != c.want {
				t.Errorf("adjustment = %q，want %q（走過 %v）", got, c.want, h.adjs.steps)
			}
			// 訊息必須真的被保存下來，而且是在 reason_detail 不是 correlation_id。
			var found bool
			for _, e := range h.audit.entries {
				if e.ReasonDetail != nil && strings.Contains(*e.ReasonDetail, "查無此發票號碼") {
					found = true
				}
				if e.CorrelationID != nil && strings.Contains(*e.CorrelationID, "查無此發票號碼") {
					t.Errorf("provider 訊息被寫進 correlation_id —— 那是追蹤 id 欄位，" +
						"且只有 varchar(64)")
				}
			}
			if !found {
				t.Errorf("provider 的錯誤原文沒有被保存進 reason_detail —— " +
					"ADM 會看不到「為什麼這筆折讓開不出來」")
			}
		})
	}
}

// TestAllowance_ConfirmReachesCompleted 守 G13：結果未明 → 查得到 → COMPLETED。
//
// 🔴 折讓**不能**走 recheck 預設的 probeOperation：那條打 invoice_query type=order
// 並以「查得到發票號碼」當成功，折讓走進去會查到仍然存在的**原發票** ⇒ operation
// 被標成完成，而 adjustment 永遠停在 WAITING_CONFIRMATION、沒有第二個執行者會碰它。
func TestAllowance_ConfirmReachesCompleted(t *testing.T) {
	h := newAllowanceHarness(t)
	ctx := context.Background()
	h.adjs.adj.Status = codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value

	// 🔴 真實時序（internal/usecase/recheck.go 的 reconcileOperations）：
	// INV.rc **先** ClaimInvoiceRecheck —— 那一句把 DB 上的 status 蓋成
	// PROVIDER_STATUS_RECHECK —— **再**把 claim 之前讀到的快照傳進 ConfirmAllowance。
	// 所以「參數裡的 op.Status」與「DB 上的 status」不同是常態，不是 fixture 寫錯。
	// 收尾守衛必須對得上 **DB 上的那一個**；原本這裡兩者都設成 WAITING_CONFIRMATION，
	// 於是測不出「MarkDone 守 SENDING ⇒ 這條 lane 必然 rows=0」。
	snapshot := h.ops.op
	snapshot.Status = codemeta.InvoiceOperationStatusWaitingConfirmation.Value
	h.ops.op.Status = codemeta.InvoiceOperationStatusProviderStatusRecheck.Value

	var queried string
	h.prov.allowanceQueryFn = func(no string) (*amego.AllowanceDetail, error) {
		queried = no
		return &amego.AllowanceDetail{AllowanceNumber: no}, nil
	}

	if err := h.uc.ConfirmAllowance(ctx, snapshot); err != nil {
		t.Fatalf("ConfirmAllowance: %v", err)
	}
	// operation 必須**真的**收尾，而且是經 INV.rc lane 收的。
	// 這兩句是本支的核心：少了它們，「軌跡寫了 SUCCEEDED 但 status 沒動」照樣全綠。
	if h.ops.recheckDoneCnt != 1 {
		t.Errorf("operation 應經 INV.rc lane（MarkRecheckResolved）收尾 1 次，實際 %d 次；"+
			"doneCnt=%d —— 走 MarkDone 的話 DB 上會 rows=0 而完全無聲",
			h.ops.recheckDoneCnt, h.ops.doneCnt)
	}
	if h.ops.op.Status != codemeta.InvoiceOperationStatusSucceeded.Value {
		t.Errorf("operation status = %q，want SUCCEEDED —— 停在原狀就是「確認成功但永遠不收尾」，"+
			"它會每 60 秒被 INV.rc 重新 claim 一次，且 unknown_since 從不推進 ⇒ 24h 放棄門檻永遠到不了",
			h.ops.op.Status)
	}
	// 拿的必須是**我方自產的折讓單號**，不是發票號碼。
	if queried != "A000000000001" {
		t.Errorf("查了 %q，want 折讓單號 A000000000001 —— 拿發票號碼去查會查到原發票", queried)
	}
	if !h.adjs.completed {
		t.Fatalf("adjustment 沒有進 COMPLETED（走過 %v）", h.adjs.steps)
	}
	if len(h.out.inserted) != 1 {
		t.Errorf("outbox 有 %d 筆，want 1 —— 確認路徑也要發 P4，"+
			"否則「送出當下就成功」與「事後查到成功」兩條路買家收到的東西不一樣",
			len(h.out.inserted))
	}
}

// TestAllowance_ConfirmKeepsWaitingWhenNotFound 守 G14：查不到就繼續等，**不重送**。
func TestAllowance_ConfirmKeepsWaitingWhenNotFound(t *testing.T) {
	h := newAllowanceHarness(t)
	ctx := context.Background()
	h.adjs.adj.Status = codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value

	// 真實時序：INV.rc 先 ClaimInvoiceRecheck（DB status → PROVIDER_STATUS_RECHECK），
	// 再把 claim 之前的快照傳進來（同 TestAllowance_ConfirmReachesCompleted）。
	snapshot := h.ops.op
	snapshot.Status = codemeta.InvoiceOperationStatusWaitingConfirmation.Value
	h.ops.op.Status = codemeta.InvoiceOperationStatusProviderStatusRecheck.Value

	h.prov.allowanceQueryFn = func(string) (*amego.AllowanceDetail, error) {
		return &amego.AllowanceDetail{}, nil // 查無
	}

	if err := h.uc.ConfirmAllowance(ctx, snapshot); err != nil {
		t.Fatalf("ConfirmAllowance: %v", err)
	}
	// 🔴 G14 的核心：這一列必須**真的回到** WAITING_CONFIRMATION，而且
	// unknown_since 要起算。留在 PROVIDER_STATUS_RECHECK 的話：
	//   ① 退避階梯整條失效，只被 60s 租約推著走 ⇒ 每分鐘打一次 AMEGO
	//   ② unknown_since 從不寫入 ⇒ `now − unknown_since` 恆為 0
	//      ⇒ 24h give-up 永遠到不了
	// dev 第 3 輪實測到的正是這個形狀（三次取樣：updated_at 每分鐘前進、
	// unknown_since 永遠 NULL、狀態永遠不變）。
	if got := h.ops.op.Status; got != codemeta.InvoiceOperationStatusWaitingConfirmation.Value {
		t.Fatalf("operation = %q，want 回到 WAITING_CONFIRMATION —— "+
			"留在重查中就是無限迴圈且 24h 門檻到不了", got)
	}
	if h.ops.op.UnknownSince == nil {
		t.Errorf("unknown_since 沒有起算 —— 它是退避階梯與 24h give-up 共用的事實時間戳")
	}
	if h.ops.waitNextAt.IsZero() {
		t.Errorf("next_attempt_at 沒有被設成退避值")
	}
	if got := h.adjs.adj.Status; got != codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value {
		t.Errorf("adjustment = %q，want 留在 WAITING_CONFIRMATION", got)
	}
	if h.prov.allowanceCalls != 0 {
		t.Fatalf("查不到就重送了 %d 次 —— 折讓單號已經送出，重送會產生第二張同號憑證",
			h.prov.allowanceCalls)
	}
	if len(h.out.inserted) != 0 {
		t.Errorf("還沒確認就寫了 %d 筆 outbox 事件", len(h.out.inserted))
	}
}

// #3082（DEC-20260904-120527 選 A）：reason_code 一律裝 CodeMeta 值，
// AMEGO 的碼歸 provider_code。
//
// 在此之前折讓軸把**所有** ClassPermanent／ClassManual 的失敗都硬編碼成
// ALLOWANCE_EXCEEDS_INVOICE（累計折讓超過原發票金額），不論真實原因是什麼。
// dev 實測：AMEGO 回 4040156「原發票不存在」的一筆，在 ADM 的異常工作台
// 被分類成「超額」⇒ 把診斷方向帶去查金額。
//
// 而 provider_code／provider_message 兩個專用欄位**從未被任何路徑寫入**
// （MarkFailed 只寫 last_error）⇒ ADM 只能去讀 last_error 的自由文字。
func TestAllowance_PermanentFailureRecordsNeutralReasonAndProviderResult(t *testing.T) {
	h := newAllowanceHarness(t)
	ctx := context.Background()
	h.prov.allowanceFn = func(amego.AllowanceRequest) (*amego.AllowanceResult, error) {
		return nil, &amego.APIError{
			Code: "4040156", Class: amego.ClassPermanent, KnownCode: true,
			Msg: "AA99999999 原發票不存在",
		}
	}
	if _, err := h.uc.ClaimForAllowanceAt(ctx, alwOpID, testNow, testNow.Add(2*time.Minute)); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := h.uc.SubmitAllowance(ctx, alwOpID); err != nil {
		t.Fatalf("SubmitAllowance: %v", err)
	}

	// ① provider 的原話落在 provider 專用欄，不是只有 last_error。
	if h.ops.gotProviderCode != "4040156" {
		t.Errorf("provider_code = %q，want \"4040156\" —— 這兩欄在修復前從未被寫入",
			h.ops.gotProviderCode)
	}
	if h.ops.gotProviderMessage != "AA99999999 原發票不存在" {
		t.Errorf("provider_message = %q —— proto 逐字要求「必須同時保存 msg」",
			h.ops.gotProviderMessage)
	}

	// ② reason_code 是中性的 CodeMeta 值，**不是**猜出來的「超額」。
	var sawNeutral, sawExceeds bool
	for _, e := range h.audit.entries {
		if e.ReasonCode == nil {
			continue
		}
		switch *e.ReasonCode {
		case codemeta.InvoiceManualReasonProviderRejectedPermanent.Value:
			sawNeutral = true
		case codemeta.InvoiceManualReasonAllowanceExceedsInvoice.Value:
			sawExceeds = true
		}
	}
	if !sawNeutral {
		t.Errorf("狀態歷程沒有 PROVIDER_REJECTED_PERMANENT：%+v", reasonCodesOf(h.audit))
	}
	if sawExceeds {
		t.Errorf("真實原因是「原發票不存在」，卻被分類成「超額」 —— "+
			"猜一個具體分類比不分類更糟：%+v", reasonCodesOf(h.audit))
	}
}

func reasonCodesOf(a *fakeAudit) []string {
	out := make([]string, 0, len(a.entries))
	for _, e := range a.entries {
		if e.ReasonCode != nil {
			out = append(out, *e.ReasonCode)
		}
	}
	return out
}

// TestAllowance_ClaimPassesNowAndLeaseInSqlOrder 守 `ops.Claim` 的兩個時間參數順序。
//
// 🔴 為什麼需要一支專門的測試：`Claim(ctx, id, now, leaseUntil)` 的兩個參數同型別，
// 對調之後**編譯得過、既有測試全綠**，因為替身原本兩格都不記錄。而 SQL 那一側
// （db/queries/invoice_operations.sql:ClaimInvoiceOperation）是這樣用的：
//
//	SET next_attempt_at = lease_until, started_at = COALESCE(started_at, now), updated_at = now
//	WHERE ... AND next_attempt_at <= claimable_before   -- claimable_before ＝ now
//
// 對調 ⇒ next_attempt_at 被寫成當下（**租約長度 0**）、started_at／updated_at 被寫成未來。
// 租約長度 0 的後果不只是「多副本會搶」：同一個副本的下一輪掃描就會再撈到這一列，
// 而 MarkInvoiceAdjustmentSending 的 allowed-from 刻意含 SENDING 自己 ⇒ 攔不住 ⇒
// 同一筆折讓打第二次 AMEGO。
//
// 期望值一律**硬寫字面值**，不呼叫被測函式、也不引用它用到的常數——否則常數一改，
// 斷言跟著變，這支測試就驗不到任何東西。
func TestAllowance_ClaimPassesNowAndLeaseInSqlOrder(t *testing.T) {
	h := newAllowanceHarness(t)
	ctx := context.Background()

	// 兩個值刻意取得可分辨：對調的話 claimNow 會變成 lease 那一個。
	lease := testNow.Add(2 * time.Minute)
	if _, err := h.uc.ClaimForAllowanceAt(ctx, alwOpID, testNow, lease); err != nil {
		t.Fatalf("ClaimForAllowanceAt: %v", err)
	}
	if h.ops.claimed != 1 {
		t.Fatalf("Claim 被呼叫 %d 次，want 1", h.ops.claimed)
	}
	if !h.ops.claimNow.Equal(testNow) {
		t.Errorf("Claim 收到的 now = %v，want %v —— 參數對調會讓 next_attempt_at 寫成當下（租約長度 0）",
			h.ops.claimNow, testNow)
	}
	if !h.ops.claimLeaseUntil.Equal(lease) {
		t.Errorf("Claim 收到的 leaseUntil = %v，want %v —— 參數對調會讓 started_at／updated_at 寫成未來",
			h.ops.claimLeaseUntil, lease)
	}
	// 租約必須嚴格落在 now 之後；相等或更早都代表沒有租約。
	if !h.ops.claimLeaseUntil.After(h.ops.claimNow) {
		t.Errorf("leaseUntil(%v) 沒有晚於 now(%v) —— 等於沒有租約",
			h.ops.claimLeaseUntil, h.ops.claimNow)
	}
}
