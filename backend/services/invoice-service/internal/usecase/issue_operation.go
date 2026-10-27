package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 本檔是「一張 document 的開票 operation 長什麼樣」的**唯一**推導處。
//
// 抽出來的理由不是去重，是**那三件事錯了都不會編譯錯**：
//
//	① operation_type 的推導（OriginalDocumentID != nil ⇒ REISSUE）——推錯不會報錯，
//	   只是 operation_key 前綴不同 ⇒ 同一張 document 上冒出兩列互不衝突的 operation。
//	② domain.OperationKeyFor 的**參數順序**（documentID, adjustmentID, issueAttemptSeq,
//	   queryRound）四個都是數字，調換位置編譯照過、鍵值錯到別人身上。
//	③ issue_attempt_seq 必須 >= 1（chk_invdoc_versions 家族），零值落庫當場 23514。
//
// 三個呼叫端：自動線（issuance.buildPlan）、ADM 手動開票（admin_issuance.createOperation）、
// ADM 人工出口回自動線（admin_job.ResumeDocumentFromManual）。第三個是 2026-09-05
// 才補的——在那之前自動線落 AWAITING_MANUAL 的文件零 operation 且**沒有任何入口**
// 建得出來，M1 的三條出邊全部以 operation id 為鍵 ⇒ 那些文件永久卡死。

// issueOperationSpec 是組一列 ISSUE／REISSUE operation 所需的全部輸入。
//
// 刻意收欄位而不是收 domain.Document：自動線在 document **還沒落庫**時就要組出這一列
// （persist 是純寫入區塊），那時手上只有 domain.NewDocument。
type issueOperationSpec struct {
	OperationID int64
	DocumentID  int64
	// OriginalDocumentID 非 nil ⇒ 這是作廢重開，operation_type 為 REISSUE。
	OriginalDocumentID  *int64
	IssueAttemptSeq     int32
	ProviderRequestNo   string
	RequestSnapshotHash string
	MaxAttempts         int32
	NextAttemptAt       time.Time
	StatutoryDeadlineAt *time.Time
	Now                 time.Time
}

// newIssueOperation 把 spec 折成 domain.NewOperation。
//
// 第二個回傳值不存在——鍵組不出來一律回 error：fallback 成任一前綴就是把兩件事
// 當成同一件（domain.OperationKeyFor 的紅字）。
func newIssueOperation(spec issueOperationSpec) (domain.NewOperation, error) {
	opType := codemeta.InvoiceOperationTypeIssue.Value
	if spec.OriginalDocumentID != nil {
		opType = codemeta.InvoiceOperationTypeReissue.Value
	}
	seq := spec.IssueAttemptSeq
	if seq < 1 {
		seq = 1
	}
	key, ok := domain.OperationKeyFor(opType, spec.DocumentID, 0, seq, 0)
	if !ok {
		return domain.NewOperation{}, fmt.Errorf("invoice: no operation_key rule for %s", opType)
	}
	return domain.NewOperation{
		ID:                  spec.OperationID,
		DocumentID:          spec.DocumentID,
		OperationType:       opType,
		OperationKey:        key,
		ProviderRequestNo:   spec.ProviderRequestNo,
		RequestSnapshotHash: spec.RequestSnapshotHash,
		MaxAttempts:         spec.MaxAttempts,
		NextAttemptAt:       spec.NextAttemptAt,
		StatutoryDeadlineAt: spec.StatutoryDeadlineAt,
		Now:                 spec.Now,
	}, nil
}

// createAndPromoteIssueOperation 建列並在**同一事務內**推進 J02（CREATED → READY）。
//
// 🔴 少了 Promote 那一列會永遠停在 CREATED，而 CREATED 不在任何 worker 的候選述詞裡
// ⇒ 這張發票永遠不會被送出，且**沒有任何告警**。
//
// appendHistory 由呼叫端傳入（兩個呼叫端的 actor 與 reason 不同，而 history 的 actor
// 欄位不得由這裡代填成 SYSTEM——那會讓稽核軌跡說謊）。
//
// 回傳 (operationID, status, 是否新建)。!inserted ＝ operation_key 撞到既有列
// ＝冪等命中：ISSUE／REISSUE 不進 uq_invop_open_adjustment_per_document 的述詞，
// 所以這裡的 rows=0 只可能是前者。
func createAndPromoteIssueOperation(
	ctx context.Context,
	ops domain.OperationRepository,
	spec issueOperationSpec,
	appendHistory func(ctx context.Context, operationID int64, from, to string) error,
) (int64, string, bool, error) {
	newOp, err := newIssueOperation(spec)
	if err != nil {
		return 0, "", false, err
	}
	op, inserted, err := ops.Create(ctx, newOp)
	if err != nil {
		return 0, "", false, err
	}
	if !inserted {
		op, err = ops.GetByKey(ctx, newOp.OperationKey)
		if err != nil {
			return 0, "", false, err
		}
		return op.ID, op.Status, false, nil
	}
	next := codemeta.InvoiceOperationStatusReady.Value
	promoted, err := ops.Promote(ctx, op.ID, next, spec.NextAttemptAt, spec.Now)
	if err != nil {
		return 0, "", false, err
	}
	if !promoted {
		return 0, "", false, fmt.Errorf("invoice: operation %d could not leave CREATED (rows=0)", op.ID)
	}
	if appendHistory != nil {
		if err := appendHistory(ctx, op.ID, codemeta.InvoiceOperationStatusCreated.Value, next); err != nil {
			return 0, "", false, err
		}
	}
	return op.ID, next, true, nil
}
