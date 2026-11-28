package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

const correctionHoldLease = 60 * time.Second

// OrderSnapshotReviser 是本 usecase 需要的 ORD 出向面。
//
// 刻意只要一支：介面越窄，「這個 usecase 會打哪些跨服務呼叫」在型別上就越說得清楚
// （同 HealthProbe 的理由）。
type OrderSnapshotReviser interface {
	ReviseOrderInvoiceSnapshot(ctx context.Context, in domain.ReviseSnapshotInput) (int64, error)
}

// Correction 是 B06 更正請求的執行 usecase。
type Correction struct {
	tx          database.TxManager
	corrections domain.CorrectionRequestRepository
	operations  domain.OperationRepository
	documents   domain.DocumentRepository
	adjustments domain.AdjustmentRepository
	orders      OrderSnapshotReviser
	audit       domain.AuditRepository
	log         *zap.Logger
	now         func() time.Time
	newID       func() int64
	maxAttempts int32
}

// NewCorrection 建 B06 執行 usecase。
func NewCorrection(
	tx database.TxManager,
	corrections domain.CorrectionRequestRepository,
	operations domain.OperationRepository,
	documents domain.DocumentRepository,
	adjustments domain.AdjustmentRepository,
	orders OrderSnapshotReviser,
	audit domain.AuditRepository,
	log *zap.Logger,
	now func() time.Time,
	newID func() int64,
	maxAttempts int32,
) *Correction {
	return &Correction{tx: tx, corrections: corrections, operations: operations,
		documents: documents, adjustments: adjustments, orders: orders, audit: audit,
		log: orNop(log), now: now, newID: newID, maxAttempts: maxAttempts}
}

// ExecuteCorrectionInput 是執行一筆已建立的更正請求所需的全部輸入。
//
// facts 由呼叫端查齊再傳（同 decideCorrectionRoute 的理由：查齊再判，測得完）。
// Revised 的密文與雜湊也由呼叫端算好——本層不碰明文（§5.1.1 的 PII 邊界）。
type ExecuteCorrectionInput struct {
	CorrectionID int64
	DocumentID   int64
	// DocumentVersion 是 document 的樂觀鎖基準（讀出當下取）。
	DocumentVersion int64
	// OperationID 是這張憑證上待送出的那一筆。Facts.OperationOpen 為 false 時可為 0。
	OperationID int64

	OrderNo string
	// ExpectedSnapshotVersion 送給 order-service 做樂觀鎖。
	ExpectedSnapshotVersion int64

	// IssueAttemptSeq 是 document 目前的實體軸值。R04 建的 REISSUE 會用 seq+1
	// （需求書稱 correction_version，設計書 §1808 對映到 invoice_documents.issue_attempt_seq）。
	IssueAttemptSeq int32
	// VoidReasonCode 是 R04 建 VOID adjustment 時要記的理由（可空）。
	VoidReasonCode *string

	Facts CorrectionFacts

	// Revised 是更正後要寫回 document 的收件人資料（已加密）。
	Revised domain.ReviseDocumentSnapshot
	// RevisedLines 是重建後的明細行。
	RevisedLines []domain.NewDocumentLine
	// OrderRevision 是要送給 order-service 的內容軸欄位（不含 notify_email，§6.2.5 定版 7）。
	OrderRevision domain.ReviseSnapshotInput
}

// ExecuteCorrectionResult 回報實際走了哪條邊。
type ExecuteCorrectionResult struct {
	// CorrectionID 是 R01 建出的那一筆。
	//
	// 🔴 **不可省**：買家要拿它去打端點 12 查進度。回空字串不會報錯、HTTP 也是 200
	// ——2026-08-31 dev 實打時就是這樣：路由與狀態全對，只有 correctionId 是空的。
	// 那條路徑上沒有任何斷言看著它（單元測試只斷言 Route／HandlingMode／Status）。
	CorrectionID int64

	Route CorrectionRoute
	// RejectReason 只在 Route == RouteReject 時非空。
	RejectReason string
	// NewSnapshotVersion 只在 R03 成功時非零。
	NewSnapshotVersion int64
}

// ErrCorrectionSendInFlight ＝ R03 的 HELD 搶不到，而 R04 又還不適用。
//
// 🔴 **這是一個真的競態窗口，不是遺漏**：
//   - Hold 搶不到 ⇒ 那筆 operation 已經是 SENDING（正在打 AMEGO），改快照已來不及
//   - 但 document 此刻**還不是 ISSUED** ⇒ R04（作廢重開）的守衛不成立，
//     沒有東西可以作廢
//
// §7.3.5 只寫「搶不到 HELD ⇒ 改走 R04」，沒有處理「送出中但尚未落地」這一格。
// 正解需要等那次送出有結果（成功 ⇒ ISSUED ⇒ R04 可行；失敗 ⇒ 回 READY ⇒ R03 可行），
// 而本 usecase 是同步的、等不了。
//
// 刻意不靜默成功（買家看到已受理而什麼都沒發生）、也不當成婉拒
// （會用假理由污染 reject_reason_code，那個碼買家直接讀得到）。
// 呼叫端應回一個「請稍後再試」類的業務碼，讓買家自己重送——REJECTED 是終態且不佔
// uq_invcorr_open_per_document 的名額，所以重送這條路是通的。
// ErrCorrectionDocumentInProgress ＝ 這張憑證正在被處理（ISSUING／VOIDING）。
//
// 與 ErrCorrectionSendInFlight 的差別：那一個是「R03 的 HELD 搶不到」（operation 層），
// 這一個是「document 已進入 ISSUING／VOIDING」（憑證層）——後者在買家於作廢重開的
// 窗口內再按一次時就會發生（#3020，dev 實測 15 秒內）。
//
// 🔴 呼叫端回 9351「請稍後再試」，**不得**折成 REJECTED：REJECTED 的每個理由碼都在說
// 「這件事做不成了」，而這裡的正確下一步是等幾秒再送一次。
var ErrCorrectionDocumentInProgress = errors.New(
	"invoice: correction cannot proceed: document has an operation in flight (ISSUING/VOIDING)")

var ErrCorrectionSendInFlight = errors.New(
	"invoice: correction cannot proceed: issuance in flight (R03 hold lost, document not yet ISSUED)")

// Execute 對一筆已在 REQUESTED 的更正請求執行分流後的那條邊。
//
// **交易邊界刻意分成三段，不是一個大交易**：
//
//	① Hold（自己的短交易）—— CAS 搶下 operation
//	② ReviseOrderInvoiceSnapshot（跨服務 gRPC，**交易外**）
//	③ 落地（一個交易：重建 lines → 改 document → 放掉 HELD → 標終態）
//
// 🔴 ② 不得包在交易裡：交易期間持有連線與鎖去等一次網路往返，是把 DB 連線池的壽命
// 綁在對方的 deadline 上。這正是 ① 要用**租約**而不是 SELECT FOR UPDATE 的理由——
// 租約在交易之外仍然有效，而行鎖不行。
func (u *Correction) Execute(ctx context.Context, in ExecuteCorrectionInput) (ExecuteCorrectionResult, error) {
	decision := decideCorrectionRoute(in.Facts)

	switch decision.Route {
	case RouteReject:
		return u.reject(ctx, in.CorrectionID, decision.RejectReason)
	case RouteVoidAndReissue:
		return u.startVoidReissue(ctx, in)
	case RouteOperationInProgress:
		return ExecuteCorrectionResult{}, ErrCorrectionDocumentInProgress
	}

	at := u.now()
	held, err := u.operations.Hold(ctx, in.OperationID, at, at.Add(correctionHoldLease))
	if err != nil {
		return ExecuteCorrectionResult{}, fmt.Errorf("invoice: hold operation for correction: %w", err)
	}
	if !held {
		u.log.Info("invoice.correction.hold_lost",
			zap.Int64("correction_id", in.CorrectionID),
			zap.Int64("operation_id", in.OperationID),
			zap.String("reason", "operation 已離開 READY／RETRY_SCHEDULED，R03 不成立"))
		return ExecuteCorrectionResult{}, ErrCorrectionSendInFlight
	}

	nextVersion, err := u.orders.ReviseOrderInvoiceSnapshot(ctx, in.OrderRevision)
	if err != nil {
		return ExecuteCorrectionResult{}, fmt.Errorf("invoice: revise order snapshot: %w", err)
	}

	revised := in.Revised
	revised.ID = in.DocumentID
	revised.ExpectedVersion = in.DocumentVersion
	revised.SnapshotVersion = int32(nextVersion)
	revised.Now = at

	err = u.tx.WithTx(ctx, func(txCtx context.Context) error {
		if _, err := u.documents.DeleteLines(txCtx, in.DocumentID); err != nil {
			return fmt.Errorf("delete lines: %w", err)
		}
		if err := u.documents.InsertLines(txCtx, in.DocumentID, in.RevisedLines, at); err != nil {
			return fmt.Errorf("insert lines: %w", err)
		}
		ok, err := u.documents.ReviseSnapshot(txCtx, revised)
		if err != nil {
			return fmt.Errorf("revise document snapshot: %w", err)
		}
		if !ok {
			return errCorrectionDocumentDrift
		}
		if _, err := u.operations.ReleaseHold(txCtx, in.OperationID, at); err != nil {
			return fmt.Errorf("release hold: %w", err)
		}
		ok, err = u.corrections.MarkUpdatedBeforeIssue(txCtx, in.CorrectionID, at)
		if err != nil {
			return fmt.Errorf("mark updated before issue: %w", err)
		}
		if !ok {
			return errCorrectionStateDrift
		}
		return u.recordCorrectionTransition(txCtx, in.CorrectionID,
			codemeta.InvoiceCorrectionStatusRequested.Value,
			codemeta.InvoiceCorrectionStatusUpdatedBeforeIssue.Value, correctionReasonR03, at)
	})
	if err != nil {
		return ExecuteCorrectionResult{}, err
	}

	u.log.Info("invoice.correction.updated_before_issue",
		zap.Int64("correction_id", in.CorrectionID),
		zap.Int64("document_id", in.DocumentID),
		zap.Int64("snapshot_version", nextVersion))
	return ExecuteCorrectionResult{CorrectionID: in.CorrectionID,
		Route: RouteUpdateBeforeIssue, NewSnapshotVersion: nextVersion}, nil
}

func (u *Correction) reject(ctx context.Context, correctionID int64, reason string) (ExecuteCorrectionResult, error) {
	at := u.now()
	ok, err := u.corrections.Reject(ctx, domain.RejectCorrection{
		ID: correctionID, ReasonCode: reason, Now: at,
	})
	if err != nil {
		return ExecuteCorrectionResult{}, fmt.Errorf("invoice: reject correction: %w", err)
	}
	if !ok {
		return ExecuteCorrectionResult{}, errCorrectionStateDrift
	}
	if err := u.recordCorrectionTransition(ctx, correctionID,
		codemeta.InvoiceCorrectionStatusRequested.Value,
		codemeta.InvoiceCorrectionStatusRejected.Value, correctionReasonR05, at); err != nil {
		return ExecuteCorrectionResult{}, err
	}
	u.log.Info("invoice.correction.rejected",
		zap.Int64("correction_id", correctionID), zap.String("reason_code", reason))
	return ExecuteCorrectionResult{CorrectionID: correctionID, Route: RouteReject, RejectReason: reason}, nil
}

var (
	errCorrectionStateDrift    = errors.New("invoice: correction state drift (rows=0)")
	errCorrectionDocumentDrift = errors.New("invoice: correction document drift (rows=0)")
)

func (u *Correction) startVoidReissue(ctx context.Context, in ExecuteCorrectionInput) (ExecuteCorrectionResult, error) {
	at := u.now()
	adjustmentID := u.newID()
	correctionID := in.CorrectionID

	orig, err := u.documents.GetByID(ctx, in.DocumentID)
	if err != nil {
		return ExecuteCorrectionResult{}, fmt.Errorf("load original document: %w", err)
	}

	var result ExecuteCorrectionResult
	err = u.tx.WithTx(ctx, func(txCtx context.Context) error {
		adj, created, err := u.adjustments.Insert(txCtx, domain.NewAdjustment{
			ID:                  adjustmentID,
			DocumentID:          in.DocumentID,
			AdjustmentType:      codemeta.InvoiceAdjustmentTypeVoid.Value,
			Status:              codemeta.InvoiceAdjustmentStatusPending.Value,
			TriggerKind:         codemeta.InvoiceAdjustmentTriggerBuyerCorrection.Value,
			CorrectionRequestID: &correctionID,
			VoidReasonCode:      in.VoidReasonCode,
			Now:                 at,
		})
		if err != nil {
			return fmt.Errorf("insert void adjustment: %w", err)
		}
		if !created {
			u.log.Info("invoice.correction.void_adjustment_exists",
				zap.Int64("correction_id", correctionID),
				zap.Int64("adjustment_id", adj.ID))
			ok, err := u.corrections.StartVoidReissue(txCtx, correctionID, adj.ID, at)
			if err != nil {
				return fmt.Errorf("start void reissue (existing adjustment): %w", err)
			}
			if !ok {
				return errCorrectionStateDrift
			}
			result = ExecuteCorrectionResult{CorrectionID: correctionID, Route: RouteVoidAndReissue}
			return nil
		}

		key, ok := domain.OperationKeyFor(codemeta.InvoiceOperationTypeVoid.Value,
			in.DocumentID, adj.ID, in.IssueAttemptSeq, 0)
		if !ok {
			return fmt.Errorf("invoice: build void operation key for adjustment %d", adj.ID)
		}
		opID := u.newID()
		_, opCreated, err := u.operations.Create(txCtx, domain.NewOperation{
			ID:            opID,
			DocumentID:    in.DocumentID,
			AdjustmentID:  adj.ID,
			OperationType: codemeta.InvoiceOperationTypeVoid.Value,
			OperationKey:  key,
			MaxAttempts:   u.maxAttempts,
			NextAttemptAt: at,
			Now:           at,
		})
		if err != nil {
			return fmt.Errorf("create void operation: %w", err)
		}
		if !opCreated {
			if _, getErr := u.operations.GetByKey(txCtx, key); getErr != nil {
				u.log.Error("invoice.correction.void_operation_gate_held",
					zap.Int64("correction_id", correctionID), zap.Int64("adjustment_id", adj.ID),
					zap.String("operation_key", key), zap.Error(getErr))
				return fmt.Errorf(
					"invoice: void operation_key %s not inserted and not found for document %d "+
						"(I5 sequence gate held by another adjustment, or key lookup failed: %w)",
					key, in.DocumentID, getErr)
			}
			u.log.Info("invoice.correction.void_operation_exists",
				zap.Int64("correction_id", correctionID), zap.String("operation_key", key))
		} else {
			promoted, err := u.operations.Promote(txCtx,
				opID, codemeta.InvoiceOperationStatusReady.Value, at, at)
			if err != nil {
				return fmt.Errorf("promote void operation: %w", err)
			}
			if !promoted {
				return errCorrectionStateDrift
			}
		}

		nextSeq := in.IssueAttemptSeq + 1
		if nextSeq < 2 {
			nextSeq = 2
		}
		reissueDoc, reissueCreated, err := u.documents.Create(txCtx,
			buildReissueDocument(u.newID(), orig, in.Revised, in.RevisedLines, in.OrderNo, nextSeq, at))
		if err != nil {
			return fmt.Errorf("create reissue document: %w", err)
		}
		if reissueCreated {
			if err := u.documents.InsertLines(txCtx, reissueDoc.ID, in.RevisedLines, at); err != nil {
				return fmt.Errorf("insert reissue lines: %w", err)
			}
			rkey, ok := domain.OperationKeyFor(codemeta.InvoiceOperationTypeReissue.Value,
				reissueDoc.ID, 0, nextSeq, 0)
			if !ok {
				return fmt.Errorf("no operation_key rule for REISSUE")
			}
			if _, _, err := u.operations.Create(txCtx, domain.NewOperation{
				ID:                u.newID(),
				DocumentID:        reissueDoc.ID,
				OperationType:     codemeta.InvoiceOperationTypeReissue.Value,
				OperationKey:      rkey,
				ProviderRequestNo: derefString(reissueDoc.ProviderRequestNo),
				MaxAttempts:       u.maxAttempts,
				NextAttemptAt:     at,
				Now:               at,
			}); err != nil {
				return fmt.Errorf("create reissue operation: %w", err)
			}
		} else {
			u.log.Info("invoice.correction.reissue_document_exists",
				zap.Int64("correction_id", correctionID),
				zap.Int64("reissue_document_id", reissueDoc.ID))
		}

		ok2, err := u.corrections.StartVoidReissue(txCtx, correctionID, adj.ID, at)
		if err != nil {
			return fmt.Errorf("start void reissue: %w", err)
		}
		if !ok2 {
			return errCorrectionStateDrift
		}

		result = ExecuteCorrectionResult{CorrectionID: correctionID, Route: RouteVoidAndReissue}
		return nil
	})
	if err != nil {
		return ExecuteCorrectionResult{}, err
	}

	u.log.Info("invoice.correction.void_reissue_started",
		zap.Int64("correction_id", correctionID),
		zap.Int64("document_id", in.DocumentID))
	return result, nil
}

// SubmitCorrectionInput 是 R01（買家於 B06 送出）的輸入。
//
// Facts 與 Revised／RevisedLines／OrderRevision 由呼叫端查齊再傳——同
// decideCorrectionRoute 的理由：查齊再判，測得完；而且 R01 與後續分支必須看到
// **同一份**事實，分兩次查會讓兩者對不上。
type SubmitCorrectionInput struct {
	DocumentID int64
	BuyerID    int64
	// ExpectedSnapshotVersion 是買家送出當下看到的內容版本。
	ExpectedSnapshotVersion int32

	// BeforePayload／AfterPayload **必須已遮罩**（密文欄位存 hash 與遮罩值）。
	// 這兩欄會被 ADM 讀到，而 invoice_correction_requests 沒有加密欄。
	BeforePayload []byte
	AfterPayload  []byte

	// 以下與 ExecuteCorrectionInput 同名欄位一一對應，R01 成功後原樣轉交。
	OperationID     int64
	IssueAttemptSeq int32
	VoidReasonCode  *string
	OrderNo         string
	Facts           CorrectionFacts
	Revised         domain.ReviseDocumentSnapshot
	RevisedLines    []domain.NewDocumentLine
	OrderRevision   domain.ReviseSnapshotInput
}

// ErrCorrectionAlreadyInProgress ＝ 序列閘門（I5）擋下。
//
// 呼叫端回 9351 INVOICE_OPERATION_IN_PROGRESS 並改讀 GetOpenByDocument——
// **不是重試**：那張發票上已經有一筆進行中的修改請求，重試只會再撞一次同一個索引。
var ErrCorrectionAlreadyInProgress = errors.New("invoice: correction already in progress for this document")

// ErrCorrectionSnapshotStale ＝ 買家送出當下看到的版本已經不是最新。
//
// 呼叫端應要求前端重新載入——買家正在改的是一份過期的資料，直接受理會讓他以為
// 改到了別的東西。
var ErrCorrectionSnapshotStale = errors.New("invoice: correction expected snapshot version is stale")

// Submit 是 §7.3.5 **R01**：建立更正請求，並在同一次呼叫內執行分流後的那條邊。
//
// 🔴 **R01 與分支必須是同一次呼叫**，不是兩個入口。R01 建出的 REQUESTED 若沒有立刻
// 被推走，它會佔住 uq_invcorr_open_per_document 的名額，而**沒有任何排程會來撿它**
// ⇒ 那份 document 從此再也提不出任何修改請求，且不報錯（#2909 的原始症狀）。
//
// 交易邊界：**只有「鎖 → 判定 → 建立」在交易內**。分支的執行（Execute）在交易外，
// 因為 R03 中間有一次跨服務 gRPC——把它包進來等於讓 DB 連線等網路往返。
// 代價是「已建立 REQUESTED 但分支執行失敗」這個窗口，而那正是 correction 有
// REQUESTED 這個狀態的理由：買家看得到、ADM 查得到、重送會被閘門擋住並讀回原件。
func (u *Correction) Submit(ctx context.Context, in SubmitCorrectionInput) (ExecuteCorrectionResult, error) {
	at := u.now()
	correctionID := u.newID()

	if decideCorrectionRoute(in.Facts).Route == RouteOperationInProgress {
		u.log.Info("invoice.correction.document_in_progress",
			zap.Int64("document_id", in.DocumentID),
			zap.String("document_status", in.Facts.DocumentStatus),
			zap.String("action", "回 9351 請稍後再試；不建立 correction 請求"))
		return ExecuteCorrectionResult{}, ErrCorrectionDocumentInProgress
	}

	err := u.tx.WithTx(ctx, func(txCtx context.Context) error {
		doc, err := u.documents.LockForCorrection(txCtx, in.DocumentID)
		if err != nil {
			return fmt.Errorf("lock document for correction: %w", err)
		}
		if doc.SnapshotVersion != in.ExpectedSnapshotVersion {
			return fmt.Errorf("%w (expected=%d actual=%d)",
				ErrCorrectionSnapshotStale, in.ExpectedSnapshotVersion, doc.SnapshotVersion)
		}

		_, created, err := u.corrections.Insert(txCtx, domain.NewCorrectionRequest{
			ID:                      correctionID,
			DocumentID:              in.DocumentID,
			RequestedByBuyerID:      in.BuyerID,
			ExpectedSnapshotVersion: in.ExpectedSnapshotVersion,
			BeforePayload:           in.BeforePayload,
			AfterPayload:            in.AfterPayload,
			Now:                     at,
		})
		if err != nil {
			return fmt.Errorf("insert correction request: %w", err)
		}
		if !created {
			return ErrCorrectionAlreadyInProgress
		}
		return nil
	})
	if err != nil {
		return ExecuteCorrectionResult{}, err
	}

	u.log.Info("invoice.correction.requested",
		zap.Int64("correction_id", correctionID),
		zap.Int64("document_id", in.DocumentID))

	out, execErr := u.Execute(ctx, ExecuteCorrectionInput{
		CorrectionID:            correctionID,
		DocumentID:              in.DocumentID,
		DocumentVersion:         in.Revised.ExpectedVersion,
		OperationID:             in.OperationID,
		OrderNo:                 in.OrderNo,
		ExpectedSnapshotVersion: int64(in.ExpectedSnapshotVersion),
		IssueAttemptSeq:         in.IssueAttemptSeq,
		VoidReasonCode:          in.VoidReasonCode,
		Facts:                   in.Facts,
		Revised:                 in.Revised,
		RevisedLines:            in.RevisedLines,
		OrderRevision:           in.OrderRevision,
	})
	if execErr != nil {
		u.releaseGateAfterBranchFailure(ctx, correctionID, execErr)
		return ExecuteCorrectionResult{}, execErr
	}
	return out, nil
}

func (u *Correction) releaseGateAfterBranchFailure(ctx context.Context, correctionID int64, cause error) {
	ok, err := u.corrections.Reject(ctx, domain.RejectCorrection{
		ID:         correctionID,
		ReasonCode: codemeta.InvoiceCorrectionRejectReasonProviderUnavailable.Value,
		Now:        u.now(),
	})
	if err != nil || !ok {
		u.log.Error("invoice.correction.gate_stuck",
			zap.Int64("correction_id", correctionID),
			zap.Bool("reject_applied", ok),
			zap.NamedError("reject_error", err),
			zap.NamedError("original_cause", cause),
			zap.String("impact", "該 document 的序列閘門被 REQUESTED 佔住，買家無法再提出任何修改"))
		return
	}
	u.log.Warn("invoice.correction.gate_released",
		zap.Int64("correction_id", correctionID),
		zap.NamedError("cause", cause),
		zap.String("action", "分支執行失敗，已婉拒以釋放序列閘門；買家可再送一次"))
}

func buildReissueDocument(
	id int64, orig domain.Document, revised domain.ReviseDocumentSnapshot,
	lines []domain.NewDocumentLine, orderNo string, nextSeq int32, now time.Time,
) domain.NewDocument {
	carrierType := ""
	if revised.CarrierType != nil {
		carrierType = *revised.CarrierType
	}
	donationCode := ""
	if revised.DonationCode != nil {
		donationCode = *revised.DonationCode
	}
	originalID := orig.ID

	kind := codemeta.InvoiceDocumentKindB2c.Value
	if domain.IsB2B(revised.RecipientTaxID) {
		kind = codemeta.InvoiceDocumentKindB2b.Value
	}
	header := domain.BuildHeader(reissueHeaderLines(lines), revised.RecipientTaxID)

	invoiceDate := domain.InvoiceDateTaipei(now)
	var deadline *time.Time
	if d, ok := domain.StatutoryDeadline(invoiceDate, kind); ok {
		deadline = &d
	}

	return domain.NewDocument{
		ID:                  id,
		IssuerID:            orig.IssuerID,
		IssuerNameSnapshot:  orig.IssuerNameSnapshot,
		IssuerTaxIDSnapshot: derefString(orig.IssuerTaxIDSnapshot),
		SourceType:          orig.SourceType,
		SourceRef:           derefString(orig.SourceRef),
		TriggerType:         orig.TriggerType,

		RecipientType:  revised.RecipientType,
		RecipientRefID: revised.RecipientRefID,
		RecipientName:  revised.RecipientName,
		RecipientTaxID: revised.RecipientTaxID,
		RecipientEmail: revised.RecipientEmail,
		CarrierID1:     revised.CarrierID1,
		CarrierID2:     revised.CarrierID2,
		CarrierType:    carrierType,
		DonationCode:   donationCode,

		DocumentKind:    kind,
		FulfillmentMode: orig.FulfillmentMode,
		Status:          codemeta.InvoiceDocumentStatusPending.Value,

		SnapshotVersion:        orig.SnapshotVersion,
		IssueAttemptSeq:        nextSeq,
		ProviderAccountVersion: orig.ProviderAccountVersion,
		OriginalDocumentID:     &originalID,

		SalesAmount:        header.SalesAmount,
		FreeTaxSalesAmount: header.FreeTaxSalesAmount,
		ZeroTaxSalesAmount: header.ZeroTaxSalesAmount,
		TaxAmount:          header.TaxAmount,
		TotalAmount:        header.TotalAmount,
		TaxType:            int16(header.TaxType), //nolint:gosec // 值域由 CodeMeta 守
		TaxRateBps:         orig.TaxRateBps,

		Provider:          derefString(orig.Provider),
		ProviderRequestNo: domain.ProviderRequestNoForOrder(orderNo, orig.SnapshotVersion, nextSeq),

		InvoiceDateTaipei: &invoiceDate,
		TaxPeriodCode:     taxPeriodFor(kind, invoiceDate),

		IssueNotBefore:      now,
		PaidAt:              orig.PaidAt,
		StatutoryDeadlineAt: deadline,
		SourceSnapshot:      orig.SourceSnapshot,
		Now:                 now,
	}
}

func reissueHeaderLines(in []domain.NewDocumentLine) []domain.Line {
	out := make([]domain.Line, 0, len(in))
	for _, l := range in {
		out = append(out, domain.Line{
			LineSeq:     l.LineSeq,
			LineType:    l.LineType,
			Description: l.Description,
			Quantity:    l.Quantity,
			UnitPrice:   l.UnitPrice,
			NetAmount:   l.NetAmount,
			TaxCategory: domain.TaxCategory(l.TaxCategory),
		})
	}
	return out
}

const (
	correctionReasonR03 = "CORRECTION_R03_UPDATED_BEFORE_ISSUE"
	correctionReasonR05 = "CORRECTION_R05_REJECTED"
)

func (u *Correction) recordCorrectionTransition(
	ctx context.Context, correctionID int64, from, to, reasonCode string, now time.Time,
) error {
	if u.audit == nil || from == to {
		return nil
	}
	return appendStatusHistory(ctx, u.audit, subjectCorrectionRequest, correctionID,
		from, to, reasonCode, "", now)
}
