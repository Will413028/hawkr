package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// 本檔的每一支陰性測試都在驗同一件事：**守衛不成立時什麼都不能發生**。
// 只斷言「回了哪個碼」不夠——rows=0 的危險形狀是「回了錯誤，但前面已經寫了一半」。
// 所以每一支陰性都額外數 repo 的呼叫次數。

var adminNow = time.Date(2026, 8, 30, 3, 0, 0, 0, time.UTC)

// jobAdjs 記下 ADM 對 job 操作時 adjustment 有沒有跟著動（G16／G19，#3073）。
//
// 它真的維護狀態並鏡射 SQL 的 allowed-from：G16 只認 FAILED_PERMANENT、
// G19 不認 SENDING。只記 calls 的替身驗不出「該不動的時候有沒有不動」。
type jobAdjs struct {
	domain.AdjustmentRepository
	adj      domain.Adjustment
	retried  int
	canceled int
	reasons  []string
}

func (f *jobAdjs) GetByID(context.Context, int64) (domain.Adjustment, error) { return f.adj, nil }

func (f *jobAdjs) AdminRetry(_ context.Context, _ int64, _ int64, _ time.Time) (bool, error) {
	f.retried++
	if f.adj.Status != codemeta.InvoiceAdjustmentStatusFailedPermanent.Value {
		return false, nil
	}
	f.adj.Status = codemeta.InvoiceAdjustmentStatusPending.Value
	return true, nil
}

func (f *jobAdjs) AdminCancel(_ context.Context, _ int64, _ int64, reasonCode string, _ time.Time) (bool, error) {
	f.canceled++
	f.reasons = append(f.reasons, reasonCode)
	switch f.adj.Status {
	case codemeta.InvoiceAdjustmentStatusPending.Value,
		codemeta.InvoiceAdjustmentStatusBlockedUnknownDocument.Value,
		codemeta.InvoiceAdjustmentStatusBlockedPendingCorrection.Value:
		f.adj.Status = codemeta.InvoiceAdjustmentStatusCancelled.Value
		return true, nil
	}
	return false, nil
}

// jobCorrs 是 R09 連動的替身。
//
// 🔴 **鏡射 SQL 的述詞，不是無條件回 true**：RetryInvoiceCorrectionReissue 的
// allowed-from 是 `status = 'REISSUE_FAILED' AND version = @expected_version`。
// 一個無條件回 true 的替身會讓「usecase 傳錯 version」「correction 根本不在
// REISSUE_FAILED」兩種真實缺陷都測不出來——那正是本輪要清的「替身沒鏡射真實守衛」
// 這個缺陷類（第 3 輪的 5 個修復中有 4 個的單元測試原本全綠，成因都是這個）。
type jobCorrs struct {
	domain.CorrectionRequestRepository
	// byDocument 的 key 是**原始** document id（correction 掛在原票上，不是替代票）。
	byDocument map[int64]domain.CorrectionRequest
	lookups    []int64
	retried    int
	completed  int
}

func (f *jobCorrs) GetReissueFailedByDocument(_ context.Context, documentID int64) (domain.CorrectionRequest, error) {
	f.lookups = append(f.lookups, documentID)
	c, ok := f.byDocument[documentID]
	if !ok {
		return domain.CorrectionRequest{}, pkgerrors.NewBusiness(
			bizcode.InvoiceCorrectionNotAllowed, "no reissue-failed correction")
	}
	return c, nil
}

func (f *jobCorrs) RetryReissue(_ context.Context, id, expectedVersion int64, _ time.Time) (bool, error) {
	f.retried++
	for docID, c := range f.byDocument {
		if c.ID != id {
			continue
		}
		// SQL 的兩道述詞逐字鏡射。
		if c.Status != codemeta.InvoiceCorrectionStatusReissueFailed.Value || c.Version != expectedVersion {
			return false, nil
		}
		c.Status = codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value
		c.Version++
		f.byDocument[docID] = c
		return true, nil
	}
	return false, nil
}

// GetOpenByDocument 鏡射 GetOpenInvoiceCorrectionRequestByDocument 的述詞
// （`status IN ('REQUESTED','VOID_REISSUE_IN_PROGRESS')`）。
//
// 🔴 少了它，settleReissueChainForExternal 會走到 embedded 介面的 nil 方法直接 panic，
// 而 panic 看起來像「fixture 寫壞了」而不是「這條路徑從來沒有人走過」。
func (f *jobCorrs) GetOpenByDocument(_ context.Context, documentID int64) (domain.CorrectionRequest, error) {
	f.lookups = append(f.lookups, documentID)
	c, ok := f.byDocument[documentID]
	if !ok || (c.Status != codemeta.InvoiceCorrectionStatusRequested.Value &&
		c.Status != codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value) {
		return domain.CorrectionRequest{}, pkgerrors.NewBusiness(
			bizcode.InvoiceCorrectionNotAllowed, "no open correction")
	}
	return c, nil
}

// Complete 鏡射 CompleteInvoiceCorrectionRequest 的 allowed-from
// （`status IN ('VOID_REISSUE_IN_PROGRESS','REISSUE_FAILED')`）並真的寫入
// reissued_document_id —— 那一欄是「替代票是哪一張」的唯一答案，
// 只記 calls 的替身對「填錯替代票 id」零鑑別力。
func (f *jobCorrs) Complete(_ context.Context, id, reissuedDocumentID int64, now time.Time) (bool, error) {
	f.completed++
	for docID, c := range f.byDocument {
		if c.ID != id {
			continue
		}
		if c.Status != codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value &&
			c.Status != codemeta.InvoiceCorrectionStatusReissueFailed.Value {
			return false, nil
		}
		c.Status = codemeta.InvoiceCorrectionStatusCompleted.Value
		c.ReissuedDocumentID = &reissuedDocumentID
		c.CompletedAt = &now
		c.Version++
		f.byDocument[docID] = c
		return true, nil
	}
	return false, nil
}

type jobFixture struct {
	uc      *AdminJob
	adjs    *jobAdjs
	corrs   *jobCorrs
	ops     *fakeOpRepo
	docs    *fakeDocRepo
	issuers *fakeIssuerRepo
	audit   *fakeAudit
	outbox  *fakeOutbox
	unlocks *fakeUnlockRepo
}

// testIDs 產出可預期的 TSID 替身：解鎖測試要斷言「回應帶的就是剛建立的那一列」，
// 而隨機 id 讓那個斷言只能寫成「非零」——非零連「回了別人的那一列」都攔不下來。
func testIDs() func() int64 {
	n := int64(9_000_000)
	return func() int64 { n++; return n }
}

func newJobFixture(op domain.Operation, doc domain.Document) *jobFixture {
	ops := &fakeOpRepo{stored: op}
	docs := &fakeDocRepo{stored: doc}
	issuers := &fakeIssuerRepo{issuer: domain.Issuer{ID: 900, IssuerType: "SELLER", IssuerRefID: 2685914682}}
	audit := &fakeAudit{}
	outbox := &fakeOutbox{}
	unlocks := &fakeUnlockRepo{}
	adjs := &jobAdjs{}
	corrs := &jobCorrs{byDocument: map[int64]domain.CorrectionRequest{}}
	uc := NewAdminJob(directTx{}, ops, docs, issuers, audit, outbox, unlocks, adjs, corrs, nil, IssuanceConfig{MaxAttempts: 13}, testIDs(), zap.NewNop()).
		WithClock(func() time.Time { return adminNow })
	return &jobFixture{uc: uc, adjs: adjs, corrs: corrs, ops: ops, docs: docs, issuers: issuers, audit: audit, outbox: outbox, unlocks: unlocks}
}

func opAt(status string, version int64) domain.Operation {
	return domain.Operation{
		// 🔴 OperationType 必填：DB 的 chk_invop_type 不接受空字串，而空字串的
		// fixture 讓「依型別分流」的守衛在測試裡完全失明（#3112）。
		ID: 7001, DocumentID: 8001, Status: status, Version: version,
		OperationType:       codemeta.InvoiceOperationTypeIssue.Value,
		RequestSnapshotHash: "hash-v1", ProviderRequestNo: "ORD-1-1",
	}
}

func docAt(status string, manualReason string) domain.Document {
	d := domain.Document{
		ID: 8001, IssuerID: 900, Status: status, Version: 3,
		IssueNotBefore: time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC),
		RecipientType:  codemeta.InvoiceRecipientTypeBuyer.Value,
		SourceRef:      strPtrOrNil("O2026082000001"),
	}
	if manualReason != "" {
		d.ManualReasonCode = &manualReason
	}
	buyer := int64(2623900520)
	d.RecipientRefID = &buyer
	return d
}

// ---------------------------------------------------------------------------
// 端點 52
// ---------------------------------------------------------------------------

func TestAdminJob_Retry_FailedPermanent(t *testing.T) {
	fx := newJobFixture(opAt("FAILED_PERMANENT", 5), docAt("ISSUE_FAILED", ""))

	got, err := fx.uc.Retry(context.Background(), RetryJobInput{
		ActorAccountID: 42, JobID: 7001, Reason: "已修正買方統編", ExpectedVersion: 5,
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatalf("Retry 失敗：%v", err)
	}
	if got.Status != "READY" {
		t.Fatalf("狀態應為 READY，實得 %q", got.Status)
	}
	// J18 走 AdminRetryAfterFix（attempt_count 不歸零），不是 AdminResetAttempts。
	if len(fx.ops.retryAfterFix) != 1 || len(fx.ops.resetAttempts) != 0 {
		t.Fatalf("應只呼叫 AdminRetryAfterFix 一次，實得 fix=%d reset=%d",
			len(fx.ops.retryAfterFix), len(fx.ops.resetAttempts))
	}
	if fx.ops.retryAfterFix[0].Version != 5 {
		t.Fatalf("CAS 必須帶 expectedVersion=5，實得 %d", fx.ops.retryAfterFix[0].Version)
	}
	// document 不在 AWAITING_MANUAL ⇒ 不得動它。
	if len(fx.docs.resumed) != 0 {
		t.Fatalf("document 不在 AWAITING_MANUAL 卻被 resume 了 %d 次", len(fx.docs.resumed))
	}
	if len(fx.audit.admin) != 1 {
		t.Fatalf("ADM 稽核應寫一列，實得 %d", len(fx.audit.admin))
	}
	if fx.audit.admin[0].RequestID == nil || *fx.audit.admin[0].RequestID != "idem-1" {
		t.Fatalf("Idempotency-Key 必須落進稽核的 request_id，實得 %v", fx.audit.admin[0].RequestID)
	}
	assertNoPIIInAudit(t, fx.audit.admin[0])
}

// TestAdminJob_AdjustmentLinkageRecordsTheRightActionAndActor 守 G16／G19 的**稽核軌跡**。
//
// 🔴 為什麼之前沒被抓到：opAt() 不設 AdjustmentID，而 advanceAdjustmentForAdmin 的
// 第一行就是 `if op.AdjustmentID == nil { return nil }` —— 所有既有的 AdminJob 測試
// 都沒有走進這條路徑，於是裡面寫死的 reason_code 與 actor=0 一路活到 dev。
//
// dev 實測（第 3 輪 ALW-06）按下 ADM「重試」之後的兩列軌跡：
//
//	OPERATION  FAILED_PERMANENT → READY   / ADMIN_JOB_RETRY  / actor=2600000022  ← 對
//	ADJUSTMENT FAILED_PERMANENT → PENDING / ADMIN_JOB_CANCEL / actor=0           ← 錯
//
// 同一個動作在稽核上被記成兩件不同的事，而且操作者不見了。
func TestAdminJob_AdjustmentLinkageRecordsTheRightActionAndActor(t *testing.T) {
	for _, c := range []struct {
		name       string
		adjStatus  string
		wantReason string
		wantTo     string
		run        func(fx *jobFixture) error
	}{
		{
			name:       "重試（G16）",
			adjStatus:  codemeta.InvoiceAdjustmentStatusFailedPermanent.Value,
			wantReason: "ADMIN_JOB_RETRY",
			wantTo:     codemeta.InvoiceAdjustmentStatusPending.Value,
			run: func(fx *jobFixture) error {
				_, err := fx.uc.Retry(context.Background(), RetryJobInput{
					ActorAccountID: 42, JobID: 7001, Reason: "r", ExpectedVersion: 5,
					IdempotencyKey: "idem-adj-retry",
				})
				return err
			},
		},
		{
			name:       "取消（G19）",
			adjStatus:  codemeta.InvoiceAdjustmentStatusPending.Value,
			wantReason: "ADMIN_JOB_CANCEL",
			wantTo:     codemeta.InvoiceAdjustmentStatusCancelled.Value,
			run: func(fx *jobFixture) error {
				_, err := fx.uc.Cancel(context.Background(), CancelJobInput{
					ActorAccountID: 42, JobID: 7001, ReasonCode: "ADM_MANUAL",
					ReasonDetail:    "INV58R3 ALW-06 稽核軌跡驗證",
					ExpectedVersion: 5, IdempotencyKey: "idem-adj-cancel",
				})
				return err
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			op := opAt(codemeta.InvoiceOperationStatusFailedPermanent.Value, 5)
			adjID := int64(9001)
			op.AdjustmentID = &adjID
			fx := newJobFixture(op, docAt("ISSUE_FAILED", ""))
			fx.adjs.adj = domain.Adjustment{ID: adjID, Status: c.adjStatus, Version: 1}

			if err := c.run(fx); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}

			var found *domain.NewStatusHistoryEntry
			for i := range fx.audit.entries {
				if fx.audit.entries[i].SubjectType == subjectAdjustment {
					found = &fx.audit.entries[i]
				}
			}
			if found == nil {
				t.Fatalf("adjustment 沒有留下任何歷程 —— 連動做了卻沒有稽核軌跡")
			}
			if found.ToStatus != c.wantTo {
				t.Errorf("to_status = %q，want %q", found.ToStatus, c.wantTo)
			}
			if found.ReasonCode == nil || *found.ReasonCode != c.wantReason {
				t.Errorf("adjustment 的 reason_code = %v，want %q —— "+
					"同一個 ADM 動作在 operation 與 adjustment 兩列上必須記成同一件事",
					derefOrNilStr(found.ReasonCode), c.wantReason)
			}
			if found.ActorAccountID == nil || *found.ActorAccountID != 42 {
				t.Errorf("adjustment 的 actor_account_id = %v，want 42 —— "+
					"寫 0 比留白更糟：該欄是 *int64，0 會實際落庫成 0，"+
					"讀的人分不出「系統做的」與「id 為 0 的人做的」",
					derefOrNilInt(found.ActorAccountID))
			}
		})
	}
}

func derefOrNilStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func derefOrNilInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestAdminJob_Retry_ResetAttemptsEdges(t *testing.T) {
	for _, from := range []string{"FAILED_RETRY_EXHAUSTED", "SELLER_ACTION_REQUIRED"} {
		t.Run(from, func(t *testing.T) {
			fx := newJobFixture(opAt(from, 2), docAt("ISSUE_FAILED", ""))
			if _, err := fx.uc.Retry(context.Background(), RetryJobInput{
				ActorAccountID: 42, JobID: 7001, Reason: "字軌已配", ExpectedVersion: 2,
			}); err != nil {
				t.Fatalf("Retry 失敗：%v", err)
			}
			// J19／J34 走 AdminResetAttempts（attempt_count 歸零）。走錯一支的後果是
			// 重試預算沒有重置，而畫面上完全看不出來。
			if len(fx.ops.resetAttempts) != 1 || len(fx.ops.retryAfterFix) != 0 {
				t.Fatalf("應走 AdminResetAttempts，實得 reset=%d fix=%d",
					len(fx.ops.resetAttempts), len(fx.ops.retryAfterFix))
			}
		})
	}
}

// 🔴 這支守的是「document 停在 AWAITING_MANUAL 而 operation 被推成 READY」那個洞：
// worker 撈到 READY 的 job、MarkIssuing 的 allowed-from 不成立 ⇒ rows=0 靜默失敗，
// 那一筆從此卡在「READY 但永遠送不出去」而沒有任何告警。
func TestAdminJob_Retry_ResumesAwaitingManualDocument(t *testing.T) {
	fx := newJobFixture(opAt("FAILED_PERMANENT", 5), docAt("AWAITING_MANUAL", "PROVIDER_RESULT_UNKNOWN"))

	if _, err := fx.uc.Retry(context.Background(), RetryJobInput{
		ActorAccountID: 42, JobID: 7001, Reason: "已人工確認可自動開", ExpectedVersion: 5,
	}); err != nil {
		t.Fatalf("Retry 失敗：%v", err)
	}
	if len(fx.docs.resumed) != 1 {
		t.Fatalf("AWAITING_MANUAL 的 document 必須在同一交易內回 PENDING，實得 %d 次", len(fx.docs.resumed))
	}
	if fx.docs.resumed[0][1] != 3 {
		t.Fatalf("document CAS 必須帶 document 自己的 version=3，實得 %d", fx.docs.resumed[0][1])
	}
}

func TestAdminJob_Retry_PlatformFundedDiscount(t *testing.T) {
	cases := []struct {
		name     string
		decision string
		wantErr  error
		wantCode int32
		wantCAS  int
	}{
		{"未裁決即拒絕", "", ErrJobFundingDecisionMissing, 0, 0},
		{"平台出資回 9341", fundingDecisionPlatformFunded, nil, bizcode.InvoiceJobNotRetryable, 0},
		{"賣家出資才放行", fundingDecisionSellerFunded, nil, 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newJobFixture(opAt("FAILED_PERMANENT", 5),
				docAt("AWAITING_MANUAL", codemeta.InvoiceManualReasonPlatformFundedDiscount.Value))

			_, err := fx.uc.Retry(context.Background(), RetryJobInput{
				ActorAccountID: 42, JobID: 7001, Reason: "裁決", ExpectedVersion: 5,
				FundingDecision: c.decision,
			})
			switch {
			case c.wantErr != nil:
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("期望 %v，實得 %v", c.wantErr, err)
				}
			case c.wantCode != 0:
				if got := bizCodeOf(t, err); got != c.wantCode {
					t.Fatalf("期望業務碼 %d，實得 %d", c.wantCode, got)
				}
			default:
				if err != nil {
					t.Fatalf("SELLER_FUNDED 應放行，實得 %v", err)
				}
			}
			// 陰性分支必須**什麼都沒寫**：只回錯誤但已經改了一半，比不回錯誤更難查。
			if len(fx.ops.retryAfterFix) != c.wantCAS {
				t.Fatalf("operation CAS 應為 %d 次，實得 %d", c.wantCAS, len(fx.ops.retryAfterFix))
			}
			if c.wantCAS == 0 && len(fx.docs.resumed) != 0 {
				t.Fatalf("陰性分支不得 resume document，實得 %d 次", len(fx.docs.resumed))
			}
		})
	}
}

// 「結果不明」一律不可重試——那正是它的定義；重送可能開出第二張發票。
func TestAdminJob_Retry_UnknownStatesRejected(t *testing.T) {
	for _, from := range []string{"WAITING_CONFIRMATION", "FAILED_UNKNOWN", "SENDING", "SUCCEEDED"} {
		t.Run(from, func(t *testing.T) {
			fx := newJobFixture(opAt(from, 5), docAt("ISSUING", ""))
			_, err := fx.uc.Retry(context.Background(), RetryJobInput{
				ActorAccountID: 42, JobID: 7001, Reason: "試試", ExpectedVersion: 5,
			})
			if got := bizCodeOf(t, err); got != bizcode.InvoiceJobNotRetryable {
				t.Fatalf("期望 9341，實得 %d", got)
			}
			if len(fx.ops.retryAfterFix)+len(fx.ops.resetAttempts) != 0 || len(fx.audit.admin) != 0 {
				t.Fatal("不可重試的狀態不得留下任何寫入")
			}
		})
	}
}

func TestAdminJob_Retry_VersionConflict(t *testing.T) {
	fx := newJobFixture(opAt("FAILED_PERMANENT", 5), docAt("ISSUE_FAILED", ""))
	_, err := fx.uc.Retry(context.Background(), RetryJobInput{
		ActorAccountID: 42, JobID: 7001, Reason: "r", ExpectedVersion: 4,
	})
	if got := bizCodeOf(t, err); got != bizcode.InvoiceJobVersionConflict {
		t.Fatalf("期望 9343，實得 %d", got)
	}
	if len(fx.ops.retryAfterFix) != 0 {
		t.Fatal("版本不符不得進 CAS")
	}
}

// CAS rows=0（別人搶先改走）必須回明確業務碼，不得靜默成功。
func TestAdminJob_Retry_CASRowsZero(t *testing.T) {
	fx := newJobFixture(opAt("FAILED_PERMANENT", 5), docAt("ISSUE_FAILED", ""))
	fx.ops.casFails = true
	_, err := fx.uc.Retry(context.Background(), RetryJobInput{
		ActorAccountID: 42, JobID: 7001, Reason: "r", ExpectedVersion: 5,
	})
	if got := bizCodeOf(t, err); got != bizcode.InvoiceJobVersionConflict {
		t.Fatalf("rows=0 必須回 9343，實得 %d", got)
	}
	if len(fx.audit.admin) != 0 {
		t.Fatal("CAS 失敗後不得寫稽核")
	}
}

func TestAdminJob_Retry_ReasonRequired(t *testing.T) {
	fx := newJobFixture(opAt("FAILED_PERMANENT", 5), docAt("ISSUE_FAILED", ""))
	// 一格空白不算填了理由：DB 的 chk_invadmaudit_reason（length >= 1）擋不住它。
	if _, err := fx.uc.Retry(context.Background(), RetryJobInput{
		ActorAccountID: 42, JobID: 7001, Reason: "   ", ExpectedVersion: 5,
	}); !errors.Is(err, ErrJobReasonRequired) {
		t.Fatalf("期望 ErrJobReasonRequired，實得 %v", err)
	}
}

// ---------------------------------------------------------------------------
// 端點 53
// ---------------------------------------------------------------------------

func TestAdminJob_Cancel_Success(t *testing.T) {
	fx := newJobFixture(opAt("FAILED_PERMANENT", 5), docAt("ISSUE_FAILED", ""))
	got, err := fx.uc.Cancel(context.Background(), CancelJobInput{
		ActorAccountID: 42, JobID: 7001, ReasonDetail: "賣家已自行處理", ExpectedVersion: 5,
		AllowManualAfterCancellation: true, IdempotencyKey: "idem-c",
	})
	if err != nil {
		t.Fatalf("Cancel 失敗：%v", err)
	}
	if got.Blocked {
		t.Fatal("正常取消不應被標記為 Blocked")
	}
	if len(fx.ops.cancels) != 1 {
		t.Fatalf("應呼叫 RequestCancel 一次，實得 %d", len(fx.ops.cancels))
	}
	// reasonCode 一律 ADM_MANUAL：寫成別的值會讓 A05 的分類統計出現查不出來源的資料。
	if fx.ops.cancels[0].ReasonCode != codemeta.InvoiceCancelReasonAdmManual.Value {
		t.Fatalf("reason_code 應為 ADM_MANUAL，實得 %q", fx.ops.cancels[0].ReasonCode)
	}
	assertNoPIIInAudit(t, fx.audit.admin[0])
}

func TestAdminJob_Cancel_BlockedWhenProviderResultUnknown(t *testing.T) {
	for _, from := range cancelUnknownStatuses {
		t.Run(from, func(t *testing.T) {
			op := opAt(from, 5)
			since := adminNow.Add(-2 * time.Hour)
			op.UnknownSince = &since
			fx := newJobFixture(op, docAt("ISSUING", ""))

			got, err := fx.uc.Cancel(context.Background(), CancelJobInput{
				ActorAccountID: 42, JobID: 7001, ReasonDetail: "想取消", ExpectedVersion: 5,
			})
			if err != nil {
				t.Fatalf("9342 是 200 不是錯誤，實得 %v", err)
			}
			if !got.Blocked {
				t.Fatal("結果不明時必須回絕取消")
			}
			// preserveData 的三個欄位是 ADM 的下一步；少了它們前端只剩一句紅字。
			if got.LastProviderCheckAt == nil || got.ProviderRequestID == "" || got.NextStep == "" {
				t.Fatalf("9342 必須帶 preserveData，實得 %+v", got)
			}
			if len(fx.ops.cancels) != 0 || len(fx.audit.admin) != 0 {
				t.Fatal("被回絕的取消不得留下任何寫入")
			}
		})
	}
}

func TestAdminJob_Cancel_ReasonCodeMustBeAdmManual(t *testing.T) {
	fx := newJobFixture(opAt("READY", 5), docAt("PENDING", ""))
	// 其餘三個 InvoiceCancelReason 值是系統路徑專用；從 A06 送進來即為契約違規。
	if _, err := fx.uc.Cancel(context.Background(), CancelJobInput{
		ActorAccountID: 42, JobID: 7001,
		ReasonCode:   codemeta.InvoiceCancelReasonFullyRefundedBeforeIssue.Value,
		ReasonDetail: "x", ExpectedVersion: 5,
	}); !errors.Is(err, ErrJobCancelReasonCodeInvalid) {
		t.Fatalf("期望 ErrJobCancelReasonCodeInvalid，實得 %v", err)
	}
	if len(fx.ops.cancels) != 0 {
		t.Fatal("值域錯誤不得進 CAS")
	}
}

func TestAdminJob_Cancel_DetailRequired(t *testing.T) {
	fx := newJobFixture(opAt("READY", 5), docAt("PENDING", ""))
	if _, err := fx.uc.Cancel(context.Background(), CancelJobInput{
		ActorAccountID: 42, JobID: 7001, ExpectedVersion: 5,
	}); !errors.Is(err, ErrJobCancelDetailRequired) {
		t.Fatalf("期望 ErrJobCancelDetailRequired，實得 %v", err)
	}
}

// ---------------------------------------------------------------------------
// 端點 56
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// 端點 56／62：外部開立的憑證**本身是一張重開票**時的鏈路收尾
// ---------------------------------------------------------------------------

// TestAdminJob_ReportManualResult_ReissueSettlesChain 守 2026-09-08 dev 實測抓到的缺口。
//
// 現象：ADM 把一張**重開票**標成 RESOLVED_EXTERNALLY（賣家自行開立）之後，
//
//	① 原票停在 VOIDED（沒有轉 SUPERSEDED）⇒ I8 InvoiceVoidWithoutReissue 永久紅
//	   （它的述詞是 status='VOIDED' 且無 ISSUED 後繼，而外部開立的後繼永遠不是 ISSUED）
//	② correction 停在 REISSUE_FAILED、reissued_document_id 永遠 NULL
//	   ⇒ InvoiceCorrectionReissueFailed 永久紅（§7.3.6「最嚴重的單一狀態」）
//
// dev 上兩筆各卡了 4～7 天，同一個缺口同時點亮兩條 critical。
//
// 成因是 #3089 A 組只修了一半：端點 52 的重試路徑補了 advanceCorrectionForAdmin，
// 而端點 56／62 這條**同樣讓 document 走到終態**的路徑沒有對應的收尾。
func TestAdminJob_ReportManualResult_ReissueSettlesChain(t *testing.T) {
	const originalID int64 = 4243
	op := opAt("MANUAL_PROCESS_ALLOWED", 5)
	op.OperationType = codemeta.InvoiceOperationTypeReissue.Value
	doc := docAt("AWAITING_MANUAL", "PROVIDER_RESULT_UNKNOWN")
	orig := originalID
	doc.OriginalDocumentID = &orig

	fx := newJobFixture(op, doc)
	fx.docs.byID = map[int64]domain.Document{
		// 真實情境：更正申請把原票作廢了，替代票開立失敗才走到人工。
		originalID: {ID: originalID, Status: codemeta.InvoiceDocumentStatusVoided.Value},
	}
	fx.corrs.byDocument[originalID] = domain.CorrectionRequest{
		ID: 5150, DocumentID: originalID, Version: 4,
		Status: codemeta.InvoiceCorrectionStatusReissueFailed.Value,
	}

	if _, err := fx.uc.ReportManualResult(context.Background(), reportInput()); err != nil {
		t.Fatalf("ReportManualResult: %v", err)
	}

	// ① 原票必須離開 VOIDED，否則 I8 永遠熄不掉。
	if got := fx.docs.byID[originalID].Status; got != codemeta.InvoiceDocumentStatusSuperseded.Value {
		t.Fatalf("原票應轉 SUPERSEDED，實得 %q", got)
	}
	// ② correction 必須收尾，且 reissued_document_id 指向**替代票**不是原票。
	corr := fx.corrs.byDocument[originalID]
	if corr.Status != codemeta.InvoiceCorrectionStatusCompleted.Value {
		t.Fatalf("correction 應轉 COMPLETED，實得 %q", corr.Status)
	}
	if corr.ReissuedDocumentID == nil || *corr.ReissuedDocumentID != doc.ID {
		t.Fatalf("reissued_document_id 應為替代票 %d，實得 %v", doc.ID, corr.ReissuedDocumentID)
	}
	// ③ 稽核軌跡：原票與 correction 各要留一列，否則下一個人看到的軌跡是假的。
	if !hasHistory(fx.audit, subjectDocument, originalID,
		codemeta.InvoiceDocumentStatusSuperseded.Value) {
		t.Fatalf("原票缺 SUPERSEDED 歷程，實得 %v", historySummary(fx.audit))
	}
	if !hasHistory(fx.audit, subjectCorrectionRequest, corr.ID,
		codemeta.InvoiceCorrectionStatusCompleted.Value) {
		t.Fatalf("correction 缺 COMPLETED 歷程，實得 %v", historySummary(fx.audit))
	}
}

// 反例：不是重開票時一格都不准碰。
//
// 🔴 這一支才是上面那支的鑑別力來源：少了它，「無條件對所有外部開立都跑一次鏈路收尾」
// 也會讓上面那支通過，而那會在一般的開票失敗轉外部開立上動到不該動的表。
func TestAdminJob_ReportManualResult_NonReissueLeavesChainAlone(t *testing.T) {
	fx := newJobFixture(opAt("MANUAL_PROCESS_ALLOWED", 5), docAt("AWAITING_MANUAL", "PROVIDER_RESULT_UNKNOWN"))
	// docAt 不帶 OriginalDocumentID ⇒ 這是一般的首次開票失敗轉外部開立。

	if _, err := fx.uc.ReportManualResult(context.Background(), reportInput()); err != nil {
		t.Fatalf("ReportManualResult: %v", err)
	}
	if len(fx.docs.superseded) != 0 {
		t.Fatalf("非重開票不得動原票，實得 %v", fx.docs.superseded)
	}
	if fx.corrs.completed != 0 || len(fx.corrs.lookups) != 0 {
		t.Fatalf("非重開票不得碰 correction，實得 completed=%d lookups=%v",
			fx.corrs.completed, fx.corrs.lookups)
	}
}

// hasHistory 問「這一軸上有沒有落到這個狀態的那一列」。
//
// 🔴 比對 **subject_type ＋ subject_id ＋ to_status 三者**：只比 to_status 會讓
// 「寫在 DOCUMENT 軸卻宣稱 CORRECTION 軸有留痕」通過（`06-no-small-tails.md` §3.10.1
// 記載的那次真實誤判，dev 上 CORRECTION_REQUEST 軸一列都沒有卻被判成已覆蓋）。
func hasHistory(a *fakeAudit, subjectType string, subjectID int64, toStatus string) bool {
	for _, e := range a.entries {
		if e.SubjectType == subjectType && e.SubjectID == subjectID && e.ToStatus == toStatus {
			return true
		}
	}
	return false
}

func historySummary(a *fakeAudit) []string {
	out := make([]string, 0, len(a.entries))
	for _, e := range a.entries {
		out = append(out, fmt.Sprintf("%s/%d→%s", e.SubjectType, e.SubjectID, e.ToStatus))
	}
	return out
}

func reportInput() ReportManualResultInput {
	return ReportManualResultInput{
		ActorAccountID: 42, JobID: 7001, InvoiceNo: "AB12345678",
		InvoiceDate: "2026-08-25", TotalAmount: "1200", Note: "賣家提供影本",
		ExpectedVersion: 5, IdempotencyKey: "idem-m",
	}
}

func TestAdminJob_ReportManualResult_Success(t *testing.T) {
	fx := newJobFixture(opAt("MANUAL_PROCESS_ALLOWED", 5), docAt("AWAITING_MANUAL", "PROVIDER_RESULT_UNKNOWN"))

	got, err := fx.uc.ReportManualResult(context.Background(), reportInput())
	if err != nil {
		t.Fatalf("ReportManualResult 失敗：%v", err)
	}
	if got.Status != "RESOLVED_EXTERNALLY" {
		t.Fatalf("job 應走向 RESOLVED_EXTERNALLY（不是 SUCCEEDED），實得 %q", got.Status)
	}
	// operation 與 document 必須同一交易一起轉：只轉一邊，賣家 S08 會同時看到
	// 「已由賣家開立」與「處理中」。
	if len(fx.ops.resolvedExternally) != 1 || len(fx.docs.resolvedExternally) != 1 {
		t.Fatalf("op/doc 兩邊都要轉，實得 op=%d doc=%d",
			len(fx.ops.resolvedExternally), len(fx.docs.resolvedExternally))
	}
	// 三個 external_* 欄同進同出（chk_invdoc_external_no）。
	w := fx.docs.resolvedExternally[0]
	if w.ExternalInvoiceNo != "AB12345678" || w.ExternalInvoiceDate.IsZero() || w.ExternalReportedAt.IsZero() {
		t.Fatalf("三個 external_* 欄必須一起寫，實得 %+v", w)
	}
	// P12。
	if len(fx.outbox.inserted) != 1 {
		t.Fatalf("應發一則 invoice.resolved_externally，實得 %d", len(fx.outbox.inserted))
	}
	var payload map[string]string
	if err := json.Unmarshal(fx.outbox.inserted[0].Payload, &payload); err != nil {
		t.Fatalf("payload 解不開：%v", err)
	}
	// 🔴 wire 名必須是 external_invoice_no。用 invoice_no 的後果是下游把賣家自行開立的
	// 號碼當成平台開的——那是平台冒充開立人，不只是欄名錯。
	if payload["external_invoice_no"] != "AB12345678" {
		t.Fatalf("P12 的 wire 欄名必須是 external_invoice_no，實得 %v", payload)
	}
	if _, bad := payload["invoice_no"]; bad {
		t.Fatal("P12 不得帶 invoice_no（那是 provider_invoice_no 的 wire 名）")
	}
	if payload["notification_ref_id"] != "INVEXT:8001" {
		t.Fatalf("冪等鍵應為 INVEXT:{document_id}，實得 %q", payload["notification_ref_id"])
	}
	// store_id 走 issuer_ref_id 不是 issuer_id。
	if payload["store_id"] != "2685914682" {
		t.Fatalf("store_id 應為 issuer_ref_id，實得 %q", payload["store_id"])
	}
	if _, bad := payload["random_code"]; bad {
		t.Fatal("payload 禁止帶 random_code（那是憑證不是資料）")
	}
	// 兩列狀態歷程（operation ＋ document），各帶 actor。
	if len(fx.audit.entries) != 2 {
		t.Fatalf("應寫兩列狀態歷程，實得 %d", len(fx.audit.entries))
	}
}

func TestAdminJob_ReportManualResult_StateGuard(t *testing.T) {
	for _, from := range []string{"FAILED_PERMANENT", "CANCELLED_BY_ADMIN", "READY", "SUCCEEDED"} {
		t.Run(from, func(t *testing.T) {
			fx := newJobFixture(opAt(from, 5), docAt("ISSUE_FAILED", ""))
			_, err := fx.uc.ReportManualResult(context.Background(), reportInput())
			if got := bizCodeOf(t, err); got != bizcode.InvoiceExternalIssuanceNotAllowed {
				t.Fatalf("期望 9349，實得 %d", got)
			}
			if len(fx.ops.resolvedExternally)+len(fx.docs.resolvedExternally)+len(fx.outbox.inserted) != 0 {
				t.Fatal("守衛不成立時不得有任何寫入或事件")
			}
		})
	}
}

func TestAdminJob_ReportManualResult_FormatGuards(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ReportManualResultInput)
		want error
	}{
		{"號碼小寫", func(i *ReportManualResultInput) { i.InvoiceNo = "ab12345678" }, ErrExternalInvoiceNoInvalid},
		{"號碼位數不對", func(i *ReportManualResultInput) { i.InvoiceNo = "AB1234567" }, ErrExternalInvoiceNoInvalid},
		{"日期格式", func(i *ReportManualResultInput) { i.InvoiceDate = "2026/08/25" }, ErrExternalInvoiceDateInvalid},
		{"金額空字串", func(i *ReportManualResultInput) { i.TotalAmount = "" }, ErrExternalTotalAmountInvalid},
		{"金額負數", func(i *ReportManualResultInput) { i.TotalAmount = "-1" }, ErrExternalTotalAmountInvalid},
		// 發票日早於付款日（doc.issue_not_before = 2026-08-20 UTC ⇒ 台北 08-20）。
		{"發票日早於付款日", func(i *ReportManualResultInput) { i.InvoiceDate = "2026-08-19" }, ErrExternalInvoiceDateTooEarly},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newJobFixture(opAt("MANUAL_PROCESS_ALLOWED", 5), docAt("AWAITING_MANUAL", "X"))
			in := reportInput()
			c.mut(&in)
			if _, err := fx.uc.ReportManualResult(context.Background(), in); !errors.Is(err, c.want) {
				t.Fatalf("期望 %v，實得 %v", c.want, err)
			}
			if len(fx.docs.resolvedExternally) != 0 {
				t.Fatal("格式錯誤不得寫入")
			}
		})
	}
}

// document 拒絕轉移（兩列不自洽）必須 rollback 整筆，不得放行。
func TestAdminJob_ReportManualResult_DocumentDriftFailsLoud(t *testing.T) {
	fx := newJobFixture(opAt("MANUAL_PROCESS_ALLOWED", 5), docAt("ISSUED", ""))
	fx.docs.resolveDocFails = true
	_, err := fx.uc.ReportManualResult(context.Background(), reportInput())
	if !errors.Is(err, errExternalDocumentStateDrift) {
		t.Fatalf("期望 errExternalDocumentStateDrift，實得 %v", err)
	}
	var de *pkgerrors.DomainError
	if errors.As(err, &de) {
		t.Fatal("資料不自洽是故障不是業務結果，不得折成 envelope 的業務碼")
	}
	if len(fx.outbox.inserted) != 0 {
		t.Fatal("document 沒轉成功就不該發 P12")
	}
}

// assertNoPIIInAudit 驗稽核的 before/after 只放狀態相關欄位。
//
// 🔴 invoice_admin_audit_logs 保存 5 年（§9.5）。把統編／抬頭／email 複製進去等於
// 憑空造一個五年期的 PII 倉庫，而稽核要回答的問題只是「誰把它從什麼狀態改成什麼狀態」。
func assertNoPIIInAudit(t *testing.T, e domain.NewAdminAuditLogEntry) {
	t.Helper()
	for _, raw := range []json.RawMessage{e.BeforeState, e.AfterState} {
		if len(raw) == 0 {
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("稽核狀態必須是 string map，實得 %s", raw)
		}
		for k := range m {
			switch k {
			case "recipientTaxId", "recipientName", "recipientEmail", "buyerTaxId", "email", "phone":
				t.Fatalf("稽核不得帶買受人身分欄：%s", k)
			}
		}
	}
}

// TestNeedsFundingDecision 釘住寫入端（端點 52 的閘門）與讀取端（A05 的旗標）
// 共用同一個述詞（#3031）。
//
// 兩份定義的後果沒有錯誤訊息：清單說「不需要裁決」而重試時被擋，或反過來——
// 使用者看到的只是「按鈕點了沒反應」。
func TestNeedsFundingDecision(t *testing.T) {
	// 🔴 狀態碼一律取自 CodeMeta 常數，**不得寫死字面值**：寫死的話碼值改動時
	// 這支測試會靜默失準（第一版寫了 "5205"，實際上 InvoiceDocumentStatus 的 Value
	// 是 "AWAITING_MANUAL" —— 猜錯的成本正好被下面那條對齊斷言擋下來）。
	awaiting := codemeta.InvoiceDocumentStatusAwaitingManual.Value
	other := codemeta.InvoiceDocumentStatusPending.Value
	cases := []struct {
		name   string
		status string
		reason string
		want   bool
	}{
		{"待人工＋平台出資折扣", awaiting, codemeta.InvoiceManualReasonPlatformFundedDiscount.Value, true},
		// 🔴 兩個條件都必要：只中一個就回 true 的實作會讓 ADM 對一堆不相干的案件
		// 被要求做資金裁決，而那個裁決一旦選 PLATFORM_FUNDED 會直接拒絕重試。
		{"待人工＋別的理由", awaiting, "SOMETHING_ELSE", false},
		{"別的狀態＋平台出資折扣", other, codemeta.InvoiceManualReasonPlatformFundedDiscount.Value, false},
		{"兩個都不是", other, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NeedsFundingDecision(tc.status, tc.reason); got != tc.want {
				t.Fatalf("NeedsFundingDecision(%q,%q) = %v, want %v", tc.status, tc.reason, got, tc.want)
			}
		})
	}

	// 兩個狀態必須真的不同，否則「別的狀態」那兩格與第一格是同一個輸入 ⇒ 零鑑別力。
	if awaiting == other {
		t.Fatal("awaiting 與 other 取到同一個碼值，陰性案例失效")
	}
}

// ---------------------------------------------------------------------------
// G16／G19：ADM 對 job 操作時 adjustment 的同一事務內連動（#3073）
// ---------------------------------------------------------------------------

// adjustmentOpAt 造一筆服務某個 adjustment 的 VOID／ALLOWANCE operation。
func adjustmentOpAt(status string, version int64, adjID int64) domain.Operation {
	op := opAt(status, version)
	op.OperationType = codemeta.InvoiceOperationTypeAllowance.Value
	op.AdjustmentID = &adjID
	return op
}

// TestAdminJob_RetryAlsoRevivesAdjustment 是 G16。
//
// 🔴 沒有這一格，ADM 對一個失敗的折讓 job 按端點 52 重試會**必然再失敗**：
// operation 回到 READY，而 adjustment 還停在 FAILED_PERMANENT ⇒ 下一輪 claim 的
// MarkSending（allowed-from = PENDING）rows=0 ⇒ 整條軸再死一次，
// 而 ADM 只會看到「它又失敗了」，看不出真正的原因。
func TestAdminJob_RetryAlsoRevivesAdjustment(t *testing.T) {
	fx := newJobFixture(adjustmentOpAt("FAILED_PERMANENT", 5, 7101), docAt("ISSUED", ""))
	fx.adjs.adj = domain.Adjustment{
		ID: 7101, Status: codemeta.InvoiceAdjustmentStatusFailedPermanent.Value, Version: 2,
	}

	if _, err := fx.uc.Retry(context.Background(), RetryJobInput{
		ActorAccountID: 42, JobID: 7001, Reason: "AMEGO 端已放行", ExpectedVersion: 5,
		IdempotencyKey: "idem-g16",
	}); err != nil {
		t.Fatalf("Retry 失敗：%v", err)
	}
	if fx.adjs.retried != 1 {
		t.Fatalf("adjustment 沒有跟著回到可執行狀態（retried=%d）—— 重試會必然再失敗",
			fx.adjs.retried)
	}
	if got := fx.adjs.adj.Status; got != codemeta.InvoiceAdjustmentStatusPending.Value {
		t.Errorf("adjustment = %q，want PENDING", got)
	}
}

// TestAdminJob_CancelAlsoCancelsUnsentAdjustment 是 G19。
//
// uq_invadj_* 都排除 CANCELLED ⇒ 唯一鍵同時釋放，同一筆退款之後可以重新建列。
// 不收的話那個鍵被一筆永遠不會執行的列佔著。
func TestAdminJob_CancelAlsoCancelsUnsentAdjustment(t *testing.T) {
	fx := newJobFixture(adjustmentOpAt("FAILED_PERMANENT", 5, 7101), docAt("ISSUED", ""))
	fx.adjs.adj = domain.Adjustment{
		ID: 7101, Status: codemeta.InvoiceAdjustmentStatusPending.Value, Version: 2,
	}

	if _, err := fx.uc.Cancel(context.Background(), CancelJobInput{
		ActorAccountID: 42, JobID: 7001, ReasonCode: "ADM_MANUAL",
		ReasonDetail: "賣家已自行處理", ExpectedVersion: 5,
		IdempotencyKey: "idem-g19",
	}); err != nil {
		t.Fatalf("Cancel 失敗：%v", err)
	}
	if fx.adjs.canceled != 1 {
		t.Fatalf("adjustment 沒有跟著收掉（canceled=%d）—— uq_invadj_* 會被一筆永遠不會執行的列佔著",
			fx.adjs.canceled)
	}
	if got := fx.adjs.adj.Status; got != codemeta.InvoiceAdjustmentStatusCancelled.Value {
		t.Errorf("adjustment = %q，want CANCELLED", got)
	}
}

// TestAdminJob_CancelLeavesSendingAdjustmentAlone 是上一支的**反面**，也是 G19
// allowed-from 不含 SENDING 的理由。
//
// 🔴 租約持有中代表**可能正在打 AMEGO**。那時把我方帳上的 adjustment 標成 CANCELLED，
// 會造出「我方已取消、對方端已生效」——而折讓單是法定憑證。
// SQL 的述詞負責擋，這支測試守「擋住之後不得回錯誤」：ADM 的意圖是對 operation 操作，
// adjustment 不在那一格時，正確結果是「不動它」而不是讓整個操作失敗。
func TestAdminJob_CancelLeavesSendingAdjustmentAlone(t *testing.T) {
	fx := newJobFixture(adjustmentOpAt("FAILED_PERMANENT", 5, 7101), docAt("ISSUED", ""))
	fx.adjs.adj = domain.Adjustment{
		ID: 7101, Status: codemeta.InvoiceAdjustmentStatusSending.Value, Version: 2,
	}

	if _, err := fx.uc.Cancel(context.Background(), CancelJobInput{
		ActorAccountID: 42, JobID: 7001, ReasonCode: "ADM_MANUAL",
		ReasonDetail: "賣家已自行處理", ExpectedVersion: 5,
		IdempotencyKey: "idem-g19-sending",
	}); err != nil {
		t.Fatalf("rows=0 讓整個 ADM 操作失敗了：%v —— 那是合法的「不動它」，不是錯誤", err)
	}
	if got := fx.adjs.adj.Status; got != codemeta.InvoiceAdjustmentStatusSending.Value {
		t.Errorf("adjustment = %q，want 留在 SENDING —— 它可能正在打 AMEGO", got)
	}
}

// ---------------------------------------------------------------------------
// R09：ADM 對重開票按端點 52 時，付款後修改請求要跟著回 VOID_REISSUE_IN_PROGRESS
//
// #3089 A 組第 1 項。在 2026-09-04 之前 RetryInvoiceCorrectionReissue 全 repo
// 零呼叫端（連 adapter 都不存在），而 SQL 檔頭寫著「沒有第二個觸發者」——
// 那個唯一的觸發者從來沒有被接上，於是 REISSUE_FAILED（設計書自稱「最嚴重的單一
// 狀態」）沒有任何出口。dev 上曾有 3 列卡了 1–2 天，而 invoice_correction_requests
// 在 monitoring/ 全域零命中 ⇒ 沒有任何告警看得到。
//
// 🔴 測試從 Retry 的**外緣**進，不直接呼叫 advanceCorrectionForAdmin：
// 後者會讓 fixture 直接把前提設成想要的值，而真正的缺陷正是「那條路從來沒被走過」。
// ---------------------------------------------------------------------------

// reissueDocAt 造一張**替代票**：它的 OriginalDocumentID 指向被作廢的原票。
func reissueDocAt(status string, originalID int64) domain.Document {
	d := docAt(status, "")
	d.ID = 8002
	d.OriginalDocumentID = &originalID
	return d
}

func TestAdminJob_Retry_AdvancesReissueFailedCorrection(t *testing.T) {
	const originalDocID = 8001
	fx := newJobFixture(opAt("FAILED_PERMANENT", 5), reissueDocAt("ISSUE_FAILED", originalDocID))
	fx.ops.stored.DocumentID = 8002
	fx.corrs.byDocument[originalDocID] = domain.CorrectionRequest{
		ID: 6001, DocumentID: originalDocID, Version: 3,
		Status: codemeta.InvoiceCorrectionStatusReissueFailed.Value,
	}

	if _, err := fx.uc.Retry(context.Background(), RetryJobInput{
		ActorAccountID: 42, JobID: 7001, Reason: "AMEGO 已恢復", ExpectedVersion: 5,
	}); err != nil {
		t.Fatalf("Retry 失敗：%v", err)
	}

	// 🔴 查的必須是**原票** id，不是替代票：correction 掛在原票上。
	// 查錯那一邊的症狀是「什麼都沒發生」——最難察覺的一種。
	if len(fx.corrs.lookups) != 1 || fx.corrs.lookups[0] != originalDocID {
		t.Fatalf("R09 應以原票 id %d 查 correction，實得 %v", originalDocID, fx.corrs.lookups)
	}
	got := fx.corrs.byDocument[originalDocID]
	if got.Status != codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value {
		t.Fatalf("correction 應回 VOID_REISSUE_IN_PROGRESS，實得 %q", got.Status)
	}
	// 稽核軌跡：operation 與 correction 各一列。少了後者，下一個人看到的軌跡是假的。
	var corrHistory int
	for _, h := range fx.audit.entries {
		if h.SubjectType == subjectCorrectionRequest {
			corrHistory++
			if h.ActorAccountID == nil || *h.ActorAccountID != 42 {
				t.Errorf("correction 的狀態歷程必須帶操作者，實得 %v", h.ActorAccountID)
			}
		}
	}
	if corrHistory != 1 {
		t.Fatalf("correction 應寫一列狀態歷程，實得 %d", corrHistory)
	}
}

// TestAdminJob_Retry_CorrectionCASUsesCorrectionOwnVersion 守「兩個獨立樂觀鎖不得混用」。
//
// operation 的 version 是 5、correction 的是 3。若 usecase 拿 ADM 帶進來的
// operation version 去做 correction 的 CAS，替身會回 rows=0（它鏡射了 SQL 的述詞），
// correction 就不會前進——而那是**靜默**的。
func TestAdminJob_Retry_CorrectionCASUsesCorrectionOwnVersion(t *testing.T) {
	const originalDocID = 8001
	fx := newJobFixture(opAt("FAILED_PERMANENT", 5), reissueDocAt("ISSUE_FAILED", originalDocID))
	fx.ops.stored.DocumentID = 8002
	fx.corrs.byDocument[originalDocID] = domain.CorrectionRequest{
		ID: 6001, DocumentID: originalDocID, Version: 3,
		Status: codemeta.InvoiceCorrectionStatusReissueFailed.Value,
	}
	if _, err := fx.uc.Retry(context.Background(), RetryJobInput{
		ActorAccountID: 42, JobID: 7001, Reason: "AMEGO 已恢復", ExpectedVersion: 5,
	}); err != nil {
		t.Fatalf("Retry 失敗：%v", err)
	}
	if fx.corrs.byDocument[originalDocID].Version != 4 {
		t.Fatalf("CAS 必須帶 correction 自己的 version(3)；替身鏡射了 SQL 述詞，"+
			"version 沒前進代表傳錯了。實得 %d", fx.corrs.byDocument[originalDocID].Version)
	}
}

// TestAdminJob_Retry_NoCorrectionIsNotAnError 守「rows=0 是常態」。
//
// 絕大多數重試的 document 根本不是重開票（OriginalDocumentID 為 nil）。
// 這條路徑回 error 會讓一個合法的 ADM 操作因為別的表沒有對應列而失敗。
func TestAdminJob_Retry_NoCorrectionIsNotAnError(t *testing.T) {
	fx := newJobFixture(opAt("FAILED_PERMANENT", 5), docAt("ISSUE_FAILED", ""))
	if _, err := fx.uc.Retry(context.Background(), RetryJobInput{
		ActorAccountID: 42, JobID: 7001, Reason: "一般重試", ExpectedVersion: 5,
	}); err != nil {
		t.Fatalf("非重開票的重試不該失敗：%v", err)
	}
	if len(fx.corrs.lookups) != 0 {
		t.Fatalf("OriginalDocumentID 為 nil 時不該去查 correction，實得 %v", fx.corrs.lookups)
	}
	if fx.corrs.retried != 0 {
		t.Fatalf("不該呼叫 RetryReissue，實得 %d 次", fx.corrs.retried)
	}
}

// TestAdminJob_ReportManualResult_DocumentRejects 重現 #3112。
//
// VOID 類型的 job 走完 A4（受控取消 → 雙人解鎖）之後，operation 進
// MANUAL_PROCESS_ALLOWED 而 document 停在 VOIDED；而
// MarkInvoiceDocumentResolvedExternally 的 allowed-from 只有
// `ISSUE_FAILED` 與 `AWAITING_MANUAL` ⇒ document 那句 rows=0 ⇒ 呼叫端回 error。
//
// 🔴 **這支測試在 2026-09-04 之前寫不出來**：`fakeDocRepo.MarkResolvedExternally`
// 無條件回 true，所以這條分支在測試裡根本不存在，而它是 dev 上真的炸出 500 的那一條。
// 這是「替身沒鏡射真實守衛」的教科書實例——修 fake 才讓缺陷變得可測。
//
// ⚠️ 本測試斷言的是**現狀**（回 error）。#3112 決定 VOID／ALLOWANCE 類型該落哪一格
// 之後，這裡要改成斷言那個可分辨的業務碼，而不是繼續容忍 500。
func TestAdminJob_ReportManualResult_DocumentRejects(t *testing.T) {
	// operation 可以轉，document 不行——這正是 #3112 的組合。
	// 型別仍是開立類（ISSUE），所以走得過上面那道 9349 守衛，落在真正的 drift。
	fx := newJobFixture(opAt("MANUAL_PROCESS_ALLOWED", 5), docAt("VOIDED", ""))

	_, err := fx.uc.ReportManualResult(context.Background(), reportInput())
	if err == nil {
		t.Fatal("document 的 allowed-from 不含 VOIDED，必須失敗而不是靜默成功")
	}
	// 🔴 兩邊同一交易：document 拒絕時 operation 那一步也不得留下已轉的痕跡。
	// （fake 不模擬 rollback，所以這裡驗的是「呼叫端沒有把它當成功回傳」。）
	if fx.docs.stored.Status == "RESOLVED_EXTERNALLY" {
		t.Fatal("document 不該被推到 RESOLVED_EXTERNALLY")
	}
}

// #3112 的實際成因：VOID 類型走完 A4 之後用端點 56 收尾。
//
// 修復前這一格回 500（errExternalDocumentStateDrift 把它當成故障），
// 而它其實是「這個類型沒有『自行開立的發票號碼』可回報」——說得清楚的業務結果。
func TestAdminJob_ReportManualResult_NonIssuanceTypeReturns9349(t *testing.T) {
	op := opAt("MANUAL_PROCESS_ALLOWED", 5)
	op.OperationType = codemeta.InvoiceOperationTypeVoid.Value
	fx := newJobFixture(op, docAt("VOIDED", ""))

	_, err := fx.uc.ReportManualResult(context.Background(), reportInput())
	assertBizCode(t, err, bizcode.InvoiceExternalIssuanceNotAllowed)
	// 守衛在最前面 ⇒ 兩邊都不得被碰到。
	if len(fx.ops.resolvedExternally) != 0 || len(fx.docs.resolvedExternally) != 0 {
		t.Fatalf("守衛不成立時不得寫入，實得 op=%d doc=%d",
			len(fx.ops.resolvedExternally), len(fx.docs.resolvedExternally))
	}
}

// 開立類仍然走得通（守衛沒有放寬過頭）。
func TestAdminJob_ReportManualResult_ReissueStillAllowed(t *testing.T) {
	op := opAt("MANUAL_PROCESS_ALLOWED", 5)
	op.OperationType = codemeta.InvoiceOperationTypeReissue.Value
	fx := newJobFixture(op, docAt("AWAITING_MANUAL", "PROVIDER_RESULT_UNKNOWN"))

	got, err := fx.uc.ReportManualResult(context.Background(), reportInput())
	if err != nil {
		t.Fatalf("REISSUE 是開立類，必須走得通：%v", err)
	}
	if got.Status != "RESOLVED_EXTERNALLY" {
		t.Fatalf("狀態應為 RESOLVED_EXTERNALLY，實得 %q", got.Status)
	}
}
