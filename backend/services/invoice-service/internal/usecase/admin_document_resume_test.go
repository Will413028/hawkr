package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

func resumeDoc(manualReason string) domain.Document {
	d := docAt(codemeta.InvoiceDocumentStatusAwaitingManual.Value, manualReason)
	d.IssueAttemptSeq = 1
	d.ProviderRequestNo = strPtrOrNil("ORD-O2026082000001-1-1")
	return d
}

func newResumeFixture(doc domain.Document) *jobFixture {
	fx := newJobFixture(domain.Operation{}, doc)
	fx.ops.byDocument = map[int64][]domain.Operation{}
	return fx
}

func TestAdminJob_ResumeDocument_BuildsMissingOperation(t *testing.T) {
	fx := newResumeFixture(resumeDoc(codemeta.InvoiceManualReasonReceiptAuthMissing.Value))

	got, err := fx.uc.ResumeDocumentFromManual(context.Background(), ResumeDocumentInput{
		ActorAccountID: 42, DocumentID: 8001, Reason: "賣家已補簽收據代製授權，送回自動線",
		ExpectedVersion: 3, IdempotencyKey: "idem-resume-1",
	})
	if err != nil {
		t.Fatalf("ResumeDocumentFromManual 失敗：%v", err)
	}

	if len(fx.docs.resumed) != 1 {
		t.Fatalf("應呼叫 ResumeFromManual 一次，實得 %d", len(fx.docs.resumed))
	}
	if fx.docs.resumed[0] != [2]int64{8001, 3} {
		t.Fatalf("CAS 必須帶 (documentID=8001, expectedVersion=3)，實得 %v", fx.docs.resumed[0])
	}
	if fx.docs.stored.Status != codemeta.InvoiceDocumentStatusPending.Value {
		t.Fatalf("document 應回 PENDING，實得 %q", fx.docs.stored.Status)
	}

	if len(fx.ops.created) != 1 {
		t.Fatalf("應補建 1 件 operation，實得 %d", len(fx.ops.created))
	}
	op := fx.ops.created[0]
	if op.OperationType != codemeta.InvoiceOperationTypeIssue.Value {
		t.Fatalf("operation_type 應為 ISSUE，實得 %q", op.OperationType)
	}
	if op.OperationKey != "issue:8001:1" {
		t.Fatalf("operation_key 應為 issue:8001:1，實得 %q", op.OperationKey)
	}
	if op.MaxAttempts != 13 {
		t.Fatalf("max_attempts 應取 IssuanceConfig 的 13，實得 %d", op.MaxAttempts)
	}
	if op.ProviderRequestNo != "ORD-O2026082000001-1-1" {
		t.Fatalf("provider_request_no 必須沿用 document 的值，實得 %q", op.ProviderRequestNo)
	}
	if op.RequestSnapshotHash != "resume:8001" {
		t.Fatalf("request_snapshot_hash 應為 resume:8001，實得 %q", op.RequestSnapshotHash)
	}
	if len(fx.ops.promoted) != 1 || fx.ops.promoted[0] != codemeta.InvoiceOperationStatusReady.Value {
		t.Fatalf("應推進 CREATED → READY 一次，實得 %v", fx.ops.promoted)
	}
	if got.Status != codemeta.InvoiceOperationStatusReady.Value {
		t.Fatalf("回應狀態應為 READY，實得 %q", got.Status)
	}
	if got.JobID != op.ID || got.JobID == 0 {
		t.Fatalf("回應的 jobId 應是剛建立的那一列（%d），實得 %d", op.ID, got.JobID)
	}

	var docHist, opHist int
	for _, e := range fx.audit.entries {
		switch e.SubjectType {
		case subjectDocument:
			if e.FromStatus != nil && *e.FromStatus == codemeta.InvoiceDocumentStatusAwaitingManual.Value &&
				e.ToStatus == codemeta.InvoiceDocumentStatusPending.Value {
				docHist++
			}
		case subjectOperation:
			if e.FromStatus != nil && *e.FromStatus == codemeta.InvoiceOperationStatusCreated.Value &&
				e.ToStatus == codemeta.InvoiceOperationStatusReady.Value {
				opHist++
			}
		}
	}
	if docHist != 1 || opHist != 1 {
		t.Fatalf("狀態歷程應為 DOCUMENT 1 列 ＋ OPERATION 1 列，實得 %d/%d", docHist, opHist)
	}
	for _, e := range fx.audit.entries {
		if e.ActorType != jobActorAdmin || e.ActorAccountID == nil || *e.ActorAccountID != 42 {
			t.Fatalf("歷程的 actor 應為 ADMIN/42，實得 %q/%v", e.ActorType, e.ActorAccountID)
		}
	}

	if len(fx.audit.admin) != 1 {
		t.Fatalf("ADM 稽核應寫一列，實得 %d", len(fx.audit.admin))
	}
	a := fx.audit.admin[0]
	if a.Action != docAuditActionResume || a.ResourceType != docResourceType || a.ResourceID != 8001 {
		t.Fatalf("稽核應為 INVOICE_DOCUMENT_RESUME/DOCUMENT/8001，實得 %q/%q/%d",
			a.Action, a.ResourceType, a.ResourceID)
	}
}

// 作廢重開的文件走 REISSUE 而不是 ISSUE：兩者 operation_key 前綴不同，推錯不會編譯錯，
// 只是同一張 document 上冒出兩列互不衝突的 operation（issue_operation.go 檔頭第 ① 條）。
func TestAdminJob_ResumeDocument_ReissueWhenOriginalPresent(t *testing.T) {
	doc := resumeDoc(codemeta.InvoiceManualReasonReceiptAuthMissing.Value)
	original := int64(7999)
	doc.OriginalDocumentID = &original
	doc.IssueAttemptSeq = 2
	fx := newResumeFixture(doc)

	if _, err := fx.uc.ResumeDocumentFromManual(context.Background(), ResumeDocumentInput{
		ActorAccountID: 42, DocumentID: 8001, Reason: "原票已作廢，重開票送回自動線",
		ExpectedVersion: 3, IdempotencyKey: "idem-resume-2",
	}); err != nil {
		t.Fatalf("ResumeDocumentFromManual 失敗：%v", err)
	}
	op := fx.ops.created[0]
	if op.OperationType != codemeta.InvoiceOperationTypeReissue.Value {
		t.Fatalf("operation_type 應為 REISSUE，實得 %q", op.OperationType)
	}
	if op.OperationKey != "reissue:8001:2" {
		t.Fatalf("operation_key 應為 reissue:8001:2，實得 %q", op.OperationKey)
	}
}

// 已經有 operation ⇒ 正確入口是端點 52。從這裡再建一列只會撞 uq_invop_operation_key
// 或多出一件孤兒作業，而且會繞過端點 52 的 G16／R09 連動。
func TestAdminJob_ResumeDocument_RejectsWhenJobExists(t *testing.T) {
	fx := newResumeFixture(resumeDoc(codemeta.InvoiceManualReasonReceiptAuthMissing.Value))
	fx.ops.byDocument[8001] = []domain.Operation{opAt(codemeta.InvoiceOperationStatusFailedPermanent.Value, 5)}

	_, err := fx.uc.ResumeDocumentFromManual(context.Background(), ResumeDocumentInput{
		ActorAccountID: 42, DocumentID: 8001, Reason: "測試：已有作業",
		ExpectedVersion: 3, IdempotencyKey: "idem-resume-3",
	})
	assertResumeBlocked(t, fx, err, bizcode.InvoiceJobNotRetryable)
}

// 不在人工出口 ⇒ 沒有可套用的動作。ResumeInvoiceDocumentFromManual 的 allowed-from
// 只認 AWAITING_MANUAL，放行到 CAS 只會 rows=0，而 rows=0 是靜默的。
func TestAdminJob_ResumeDocument_RejectsWhenNotAwaitingManual(t *testing.T) {
	for _, status := range []string{
		codemeta.InvoiceDocumentStatusPending.Value,
		codemeta.InvoiceDocumentStatusIssued.Value,
		codemeta.InvoiceDocumentStatusNotApplicable.Value,
	} {
		t.Run(status, func(t *testing.T) {
			fx := newResumeFixture(docAt(status, ""))
			_, err := fx.uc.ResumeDocumentFromManual(context.Background(), ResumeDocumentInput{
				ActorAccountID: 42, DocumentID: 8001, Reason: "測試：狀態不符",
				ExpectedVersion: 3, IdempotencyKey: "idem-resume-4",
			})
			assertResumeBlocked(t, fx, err, bizcode.InvoiceJobNotRetryable)
		})
	}
}

func TestAdminJob_ResumeDocument_VersionConflict(t *testing.T) {
	fx := newResumeFixture(resumeDoc(codemeta.InvoiceManualReasonReceiptAuthMissing.Value))

	_, err := fx.uc.ResumeDocumentFromManual(context.Background(), ResumeDocumentInput{
		ActorAccountID: 42, DocumentID: 8001, Reason: "測試：版本過期",
		ExpectedVersion: 2, IdempotencyKey: "idem-resume-5",
	})
	assertResumeBlocked(t, fx, err, bizcode.InvoiceJobVersionConflict)
}

// CAS 在讀與寫之間落空（別人先動了那一列）⇒ 回 9343，而且**不得留下半件 operation**。
func TestAdminJob_ResumeDocument_CasLostRace(t *testing.T) {
	fx := newResumeFixture(resumeDoc(codemeta.InvoiceManualReasonReceiptAuthMissing.Value))
	fx.docs.resumeFails = true

	_, err := fx.uc.ResumeDocumentFromManual(context.Background(), ResumeDocumentInput{
		ActorAccountID: 42, DocumentID: 8001, Reason: "測試：CAS 落空",
		ExpectedVersion: 3, IdempotencyKey: "idem-resume-6",
	})
	if !isBizCode(err, bizcode.InvoiceJobVersionConflict) {
		t.Fatalf("應回 9343，實得 %v", err)
	}
	if len(fx.ops.created) != 0 {
		t.Fatalf("CAS 落空後不得建 operation，實得 %d 件", len(fx.ops.created))
	}
	if len(fx.audit.admin) != 0 {
		t.Fatalf("CAS 落空後不得寫稽核，實得 %d 列", len(fx.audit.admin))
	}
}

// 平台出資折扣：判準與端點 52 共用 needsFundingDecision——同一份 document 從兩個入口
// 進來不該得到不同的答案。
func TestAdminJob_ResumeDocument_FundingDecision(t *testing.T) {
	reason := codemeta.InvoiceManualReasonPlatformFundedDiscount.Value

	t.Run("未裁決即拒絕", func(t *testing.T) {
		fx := newResumeFixture(resumeDoc(reason))
		_, err := fx.uc.ResumeDocumentFromManual(context.Background(), ResumeDocumentInput{
			ActorAccountID: 42, DocumentID: 8001, Reason: "測試：缺裁決",
			ExpectedVersion: 3, IdempotencyKey: "idem-resume-7",
		})
		if !errors.Is(err, ErrJobFundingDecisionMissing) {
			t.Fatalf("應回 ErrJobFundingDecisionMissing，實得 %v", err)
		}
		assertResumeWroteNothing(t, fx)
	})

	t.Run("PLATFORM_FUNDED 一律拒絕", func(t *testing.T) {
		fx := newResumeFixture(resumeDoc(reason))
		_, err := fx.uc.ResumeDocumentFromManual(context.Background(), ResumeDocumentInput{
			ActorAccountID: 42, DocumentID: 8001, Reason: "測試：平台出資",
			ExpectedVersion: 3, IdempotencyKey: "idem-resume-8",
			FundingDecision: fundingDecisionPlatformFunded,
		})
		if !isBizCode(err, bizcode.InvoiceJobNotRetryable) {
			t.Fatalf("應回 9341，實得 %v", err)
		}
		assertResumeWroteNothing(t, fx)
	})

	t.Run("SELLER_FUNDED 放行", func(t *testing.T) {
		fx := newResumeFixture(resumeDoc(reason))
		if _, err := fx.uc.ResumeDocumentFromManual(context.Background(), ResumeDocumentInput{
			ActorAccountID: 42, DocumentID: 8001, Reason: "測試：賣家出資",
			ExpectedVersion: 3, IdempotencyKey: "idem-resume-9",
			FundingDecision: fundingDecisionSellerFunded,
		}); err != nil {
			t.Fatalf("SELLER_FUNDED 應放行，實得 %v", err)
		}
		if len(fx.ops.created) != 1 {
			t.Fatalf("應補建 1 件 operation，實得 %d", len(fx.ops.created))
		}
	})
}

// 讀端（端點 48 的 can_resume_from_manual）與寫端共用這一支。分家的後果是按鈕亮在
// 一個 CAS 必定 rows=0 的狀態上，而 rows=0 沒有訊號（#3031／#3089 第 8 項）。
func TestCanResumeDocumentFromManual(t *testing.T) {
	cases := []struct {
		status string
		ops    int
		want   bool
	}{
		{codemeta.InvoiceDocumentStatusAwaitingManual.Value, 0, true},
		{codemeta.InvoiceDocumentStatusAwaitingManual.Value, 1, false},
		{codemeta.InvoiceDocumentStatusPending.Value, 0, false},
		{codemeta.InvoiceDocumentStatusIssueFailed.Value, 0, false},
		{codemeta.InvoiceDocumentStatusIssued.Value, 0, false},
		{codemeta.InvoiceDocumentStatusNotApplicable.Value, 0, false},
	}
	for _, c := range cases {
		if got := CanResumeDocumentFromManual(c.status, c.ops); got != c.want {
			t.Errorf("CanResumeDocumentFromManual(%q, %d) = %v，期望 %v", c.status, c.ops, got, c.want)
		}
	}
}

func assertResumeBlocked(t *testing.T, fx *jobFixture, err error, want int32) {
	t.Helper()
	if !isBizCode(err, want) {
		t.Fatalf("應回 %d，實得 %v", want, err)
	}
	assertResumeWroteNothing(t, fx)
}

func assertResumeWroteNothing(t *testing.T, fx *jobFixture) {
	t.Helper()
	if len(fx.docs.resumed) != 0 {
		t.Fatalf("守衛不成立時不得呼叫 ResumeFromManual，實得 %d 次", len(fx.docs.resumed))
	}
	if len(fx.ops.created) != 0 {
		t.Fatalf("守衛不成立時不得建 operation，實得 %d 件", len(fx.ops.created))
	}
	if len(fx.audit.admin) != 0 {
		t.Fatalf("守衛不成立時不得寫稽核，實得 %d 列", len(fx.audit.admin))
	}
	if len(fx.audit.entries) != 0 {
		t.Fatalf("守衛不成立時不得寫狀態歷程，實得 %d 列", len(fx.audit.entries))
	}
}
