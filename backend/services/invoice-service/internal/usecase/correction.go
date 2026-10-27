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

// 買家申請修改發票資料（B06）的執行面 —— §7.3.5 的 R02／R03。
//
// R04（作廢重開）不在本檔：它要建 VOID adjustment ＋ REISSUE operation ＋ 把 document
// 推進 VOIDING，是另一組副作用。**本檔遇到 R04 一律 fail-loud**，不要靜默當成成功——
// 那會讓買家看到「已受理」而什麼都沒發生。

// correctionHoldLease 是 R03 期間 operation 被 HELD 的長度（§7.3.2 J08）。
//
// 🔴 它必須撐過中間那一次跨服務 gRPC（ORD.ReviseOrderInvoiceSnapshotInternal）。
// 短於那次呼叫的 deadline ⇒ 我們還在等 order-service，租約就過期被掃回 READY
// ⇒ worker 撿走並用**舊內容**送出去，而我們接著把快照改成新的：買家手上那張
// 發票的內容永遠對不上他改過的資料，且沒有任何東西會報錯。
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
	// audit 是 I7 的寫入面（#3109 收尾，2026-09-04）。
	//
	// 🔴 在這之前 CORRECTION_REQUEST 軸只有 R04（作廢重開）那三條轉移有留痕，
	// R01／R03／R05 一列都沒有——dev 實測一筆剛走完 R03 的 correction，
	// invoice_status_history 是 **0 列**。
	//
	// ⚠️ 我的稽核判準原本抓不到它：四層過濾檢查「函式或呼叫者有沒有呼叫
	// appendStatusHistory」，而 Execute 的外層確實有呼叫——**但那是寫 DOCUMENT 軸的**。
	// 判準少了「寫的是哪個 subject_type」這一格，於是把它誤判成已覆蓋。
	audit domain.AuditRepository
	log   *zap.Logger
	now   func() time.Time
	newID func() int64
	// maxAttempts 是新建 operation 的重試上限。
	//
	// 🔴 **不能留 0**：chk_invop_attempts 要求 max_attempts >= 1，而 0 是 int32 零值
	// ⇒ 忘了填不是編譯錯、不是測試紅（fake 照單全收），是**真實請求時的 23514**。
	// 2026-08-31 dev 首次實打就撞到這個。
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
		// 不改任何狀態就退出。Submit 已經在建立 REQUESTED 之前擋過一次
		// （見該函式的同名守衛）；這裡是 Execute 被單獨呼叫時的第二道，
		// 兩道都在，是因為漏掉任一道的後果不同：Submit 漏了會留一筆假的
		// REJECTED，Execute 漏了會讓一張正在作廢的憑證走進 R02／R04。
		return ExecuteCorrectionResult{}, ErrCorrectionDocumentInProgress
	}

	// ---- R03：開立前直接改 ----
	//
	// ① 守衛。搶不到代表那筆已經在打 AMEGO（HoldInvoiceOperation 的述詞刻意不含
	//    SENDING）⇒ R03 這條邊不成立，**改走 R04**，不是重試。
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

	// ② 跨服務：order-service 是 snapshot_version 的唯一寫入者（I-50）。
	//    交易外——見本函式的 javadoc。
	nextVersion, err := u.orders.ReviseOrderInvoiceSnapshot(ctx, in.OrderRevision)
	if err != nil {
		// 這裡**不主動放掉 HELD**：租約 60 秒後會由 ReleaseExpiredHolds 掃回 READY。
		// 主動放掉看起來比較乾淨，但那需要另一次寫入，而我們剛剛才證明對外呼叫會失敗
		// ——此刻最不該做的就是再賭一次 DB 也還活著。讓租約做它本來就要做的事。
		return ExecuteCorrectionResult{}, fmt.Errorf("invoice: revise order snapshot: %w", err)
	}

	// ③ 落地。四件事必須在同一個交易內：少了任一件都會留下一個沒有人會發現的中間態。
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
			// rows=0 ⇒ 版本被推進或 status 已離開 PENDING／AWAITING_MANUAL。
			// 整筆 rollback：放行會留下「明細換了但快照沒換」的組合。
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
		// I7：R03 的狀態轉移要留痕（#3109 收尾）。**subject 是 CORRECTION_REQUEST**——
		// 上面 ReviseSnapshot 那一段寫的是 DOCUMENT 軸，兩者不可互相代替。
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

// reject 是 R02。理由碼由 decideCorrectionRoute 給，不由本層決定。
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
	// I7：R05 婉拒也是狀態轉移（#3109 收尾）。reason_detail 帶婉拒理由碼——
	// 事後查「為什麼這筆沒被處理」時，那是唯一的依據。
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
	// errCorrectionStateDrift：CAS rows=0，代表有別人先動了那一列。
	errCorrectionStateDrift = errors.New("invoice: correction state drift (rows=0)")
	// errCorrectionDocumentDrift：document 的版本或狀態已變，整筆 rollback。
	errCorrectionDocumentDrift = errors.New("invoice: correction document drift (rows=0)")
)

// startVoidReissue 是 §7.3.5 **R04**（預檢通過且 document 已 ISSUED）。
//
// 🔴 **順序是「先確保可開再作廢」**，而那個順序已經體現在呼叫本函式之前：
// decideCorrectionRoute 的四項預檢（字軌餘量／載具有效／期限內／無既存折讓）不過就
// 直接 RouteReject，走不到這裡。作廢不可逆，且明令不可用 DB restore 把 ISSUED 回復為
// 未開立——順序反了，買家手上一張都沒有。
//
// **本函式全程不打 AMEGO**：它只建 VOID adjustment ＋ VOID operation ＋ 推進 correction，
// 真正的 f0501 由 operation worker 送。所以整段可以是一個交易。
//
// document → VOIDING **不在這裡**：那一步發生在 VOID operation READY→SENDING
// （見 invoice_documents.sql 的對應 query 註解），由 worker 做。
func (u *Correction) startVoidReissue(ctx context.Context, in ExecuteCorrectionInput) (ExecuteCorrectionResult, error) {
	at := u.now()
	adjustmentID := u.newID()
	correctionID := in.CorrectionID

	// 原票要在交易外先讀：重開票的大部分欄位是它的副本（發票人、稅務、金額、
	// provider、法定期限），只有收件人那一組換成修正後的值。
	orig, err := u.documents.GetByID(ctx, in.DocumentID)
	if err != nil {
		return ExecuteCorrectionResult{}, fmt.Errorf("load original document: %w", err)
	}

	var result ExecuteCorrectionResult
	err = u.tx.WithTx(ctx, func(txCtx context.Context) error {
		// ① VOID adjustment。六個 ALLOWANCE 專用欄一律 nil——chk_invadj_allowance_shape
		//    是雙向的，傳錯組合當場 23514。
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
			// 🔴 唯一鍵擋下 ⇒ 這張發票已經有一筆作廢（uq_invadj_void_once）。
			// **不得再建 operation**：重複建會送出第二次作廢，而作廢不可逆。
			// 回既有列的 id，讓 correction 指向它即可。
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

		// ② VOID operation。key 走 domain.OperationKeyFor（void:{adjustment_id}）——
		//    不是設計圖寫的 void:{document_id}:{correction_version}。實作版更強：
		//    uq_invadj_void_once 保證一張發票最多一筆作廢 ⇒ 兩者唯一性等價，
		//    而 void:{adjustment_id} 直接把 operation 綁在它的 adjustment 上。
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
			// 🔴 rows=0 有**兩種**意義，必須回讀 operation_key 分辨
			// （`InsertInvoiceOperation` 檔頭逐字：「分辨方式是先讀 operation_key，不是猜」）：
			//
			//   ① 查得到 ⇒ 同一把 key 重送（重投）⇒ 冪等，記一行讓對帳看得到。
			//   ② 查不到 ⇒ 撞的是 `uq_invop_open_adjustment_per_document`（I5 序列閘門）：
			//      這張 document 上還有一筆非終態的調整類 operation 佔著。
			//
			// ⚠️ ② 在這條路徑上是**主要**成因而非邊界：key 是 `void:{adjustment_id}`，
			// 而 adj 剛剛才在上一句以新 ID 建出來（`created == true` 才走到這裡）
			// ⇒ operation_key 不可能撞，剩下的只有序列閘門。
			//
			// 原本兩種都只記 log 就往下走，後果與折讓軸那個「孤兒折讓單」同形：
			// VOID adjustment 建了、operation 一列都沒有、correction 進
			// VOID_REISSUE_IN_PROGRESS ⇒ 買家看到「處理中」而永遠不會有事情發生，
			// 且完全不報錯。② 一律 rollback：correction 沒有退避重試機制，
			// 回錯讓呼叫端看得到，比留下一筆沒有執行者的作廢好。
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
			// 🔴 **同一事務內把它推到 READY**（J02）。
			//
			// InsertInvoiceOperation 落的是 CREATED，而 **CREATED 不在任何一支 worker 的
			// 候選集合裡** ⇒ 少了這一句，那筆作廢永遠不會送出去：correction 停在
			// VOID_REISSUE_IN_PROGRESS、document 停在 ISSUED、買家看到「處理中」而
			// 什麼都不會發生，**且完全不報錯**。
			//
			// 2026-08-31 dev 實測就是這樣：等了 45 秒 operation 仍是 CREATED／attempt=0。
			// 那是 #2909 原始症狀的第三次變形（前兩次：REQUESTED 卡住、分支失敗卡住），
			// 三次的共同形狀都是「建了一列，而沒有任何東西會推它」。
			//
			// 作廢沒有 issue_not_before 的延後條件（那是開票才有的 J03），所以一律 READY
			// ＋ next_attempt_at = now：預檢已經在 decideCorrectionRoute 做完，
			// 這裡沒有第二個要等的條件。
			promoted, err := u.operations.Promote(txCtx,
				opID, codemeta.InvoiceOperationStatusReady.Value, at, at)
			if err != nil {
				return fmt.Errorf("promote void operation: %w", err)
			}
			if !promoted {
				// rows=0 ⇒ 那筆已不在 CREATED。整筆 rollback：放行會留下一筆
				// 沒有人會撿的作廢，而那正是本段要防的事。
				return errCorrectionStateDrift
			}
		}

		// ③ correction → VOID_REISSUE_IN_PROGRESS，指向剛建的 adjustment（fk_invcorr_void）。
		// ③ 重開票的 document ＋ REISSUE operation，**同一事務內就建好**。
		//
		// 🔴 為什麼不是等作廢成功之後再建：修正後的收件人明文只存在於這一刻。
		// `invoice_correction_requests.after_payload` 依 §5 是**已遮罩**的（那張表
		// 沒有加密欄而 ADM 讀得到它），而 `InvoiceInputSpec` 是可以改 email 的
		// （proto field 6）⇒ 遮罩之後再也還原不回來。作廢成功後才建的話，
		// **沒有任何地方拿得到要開什麼**，結果就是 §7.3.6 R08 那個
		// 「本設計書中最嚴重的單一狀態」：原票已作廢、新票開不出來。
		//
		// 🔴 REISSUE operation 刻意落在 CREATED 而**不推進**——與本檔上面 VOID
		// operation 的處置正好相反，那不是漏了：CREATED 不在任何 worker 的候選
		// 集合裡，正是「等作廢確認」要的語意。把它推到 READY 的唯一執行者是
		// 作廢確認路徑（INV.rc 的 VOID 分支），而作廢被拒時（R05）它會被取消。
		//
		// 實體軸 +1、內容軸不動：§5 的 issue_attempt_seq 逐字是「**同一
		// snapshot_version 下**第幾張實體發票（作廢重開 +1）」，而 snapshot_version
		// 的唯一寫入者是 order-service（I-50），本服務不得自行 +1。
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
			// uq_invdoc_order_identity 擋下 ⇒ 這一輪重投，新票早就建過了。
			// **不得**再建 operation：重複建會在作廢確認時送出第二張。
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

	// ⓪ 進行中的憑證：**在建立 REQUESTED 之前**就擋掉。
	//
	// 放在這裡而不是只靠 Execute，是因為 Submit 對 Execute 的錯誤一律走
	// releaseGateAfterBranchFailure ⇒ 會留下一筆 REJECTED／PROVIDER_UNAVAILABLE
	// 的申請紀錄，而真因既不是服務商也不是婉拒。買家與 ADM 讀得到那一列（#3020）。
	// decideCorrectionRoute 是純函式，多算一次的成本是零。
	if decideCorrectionRoute(in.Facts).Route == RouteOperationInProgress {
		u.log.Info("invoice.correction.document_in_progress",
			zap.Int64("document_id", in.DocumentID),
			zap.String("document_status", in.Facts.DocumentStatus),
			zap.String("action", "回 9351 請稍後再試；不建立 correction 請求"))
		return ExecuteCorrectionResult{}, ErrCorrectionDocumentInProgress
	}

	err := u.tx.WithTx(ctx, func(txCtx context.Context) error {
		// ① 鎖住 document 再判定。少了它，「判定當下相符、建立當下已被 R03 推進」
		//    這條路徑會寫進一個過期的 expected_snapshot_version，而它是後續送去
		//    order-service 的樂觀鎖基準 ⇒ 錯了會在跨服務那一層才炸。
		doc, err := u.documents.LockForCorrection(txCtx, in.DocumentID)
		if err != nil {
			return fmt.Errorf("lock document for correction: %w", err)
		}
		if doc.SnapshotVersion != in.ExpectedSnapshotVersion {
			return fmt.Errorf("%w (expected=%d actual=%d)",
				ErrCorrectionSnapshotStale, in.ExpectedSnapshotVersion, doc.SnapshotVersion)
		}

		// ② 建立。序列閘門的 DB 落點就是這一句的 ON CONFLICT
		//    （uq_invcorr_open_per_document）——不必先查再寫，那在併發下擋不住。
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

	// ③ 立刻執行分流後的那條邊。**不得省略**：見本函式 javadoc 的第一條紅線。
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

// releaseGateAfterBranchFailure 在分支執行失敗後把 correction 婉拒掉。
//
// 🔴 **少了這一段，一次暫時性失敗會永久鎖死那份 document。**
// R01 的交易已經 commit（分支在交易外，因為 R03 中間有跨服務 gRPC），所以失敗時
// 會留下一筆 REQUESTED；而 uq_invcorr_open_per_document 的述詞含 REQUESTED
// ⇒ 買家重送被擋、**沒有任何排程會來推它**。那正是 #2909 的原始症狀，
// 只是從另一條路徑到達。
//
// 2026-08-31 dev 首次實打就製造出一筆（VOID operation 漏填 MaxAttempts 觸發 23514）。
//
// 婉拒是正解而不是刪除：REJECTED 是終態且**不在該索引的述詞內** ⇒ 名額釋放、
// 買家可另提一次，同時保留這筆申請的軌跡供 ADM 與爭議查閱
// （見 RejectInvoiceCorrectionRequest 的 SQL 註解）。
//
// ⚠️ 理由碼一律 PROVIDER_UNAVAILABLE，即使真因是我方的錯（如上述 23514）。
// 六個值域裡沒有「系統暫時故障」那一格，而 PROVIDER_UNAVAILABLE 給買家的
// 下一步（稍後再送一次）**在兩種情況下都正確**。要更精確就得擴值域，
// 而那是前端可見的碼表變更（§8.5），不該為了措辭精準去動它。
//
// 本函式**不回傳錯誤**：它是補救，補救失敗不該蓋掉原始失敗原因。
func (u *Correction) releaseGateAfterBranchFailure(ctx context.Context, correctionID int64, cause error) {
	// ⚠️ **理由碼是「這一類失敗最常見的成因」，不是判斷結果**（2026-09-05 補）。
	//
	// 本函式接的是**任何**分支失敗：AMEGO 真的連不上是其中一種，我方自己的
	// DB 約束違反也是。前者「暫時無法連線、請稍後再試」說得通，後者說不通
	// ——那是永久失敗，買家照著提示重送幾次都一樣，而畫面不會告訴他去找客服。
	//
	// dev 實證（2026-09-05，correction 7501938550804316160）：一個
	// `chk_invdoc_carrier_pair` 的 23514 被標成 `PROVIDER_UNAVAILABLE`
	// 「服務商暫時無法連線」。真正指得出成因的只有下面那則 `gate_released`
	// 的 Warn log（帶 cause）。
	//
	// 沒有就地改成依 cause 分流，是因為分類本身可能分錯，而分錯的代價
	// （把一次真的 provider 中斷標成「請聯繫客服」）不比現在小。
	// 要改的話該做的是**新增一個值域**（例如 INTERNAL_ERROR）而不是挪用既有八值。
	ok, err := u.corrections.Reject(ctx, domain.RejectCorrection{
		ID:         correctionID,
		ReasonCode: codemeta.InvoiceCorrectionRejectReasonProviderUnavailable.Value,
		Now:        u.now(),
	})
	if err != nil || !ok {
		// 補救也失敗 ⇒ 那份 document 真的卡住了。這是唯一需要人介入的分支，
		// 所以它必須留下**指得出是哪一筆**的紀錄，而不是一句「處理失敗」。
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

// buildReissueDocument 把原票 ＋ 修正後的收件人資料組成重開票的建列輸入。
//
// 分成三組看：
//
//	① 收件人 —— 全部取修正後的值（明文，由 repository 加密後落庫）
//	② 身分與識別 —— 新 id、實體軸 +1、指回原票、重算 provider_request_no
//	③ 其餘 —— 原樣複製
//
// 🔴 ③ 一定要**逐欄複製而不是留零值**：invoice_documents 上有五個雙向 CHECK
// （金額、稅別、外部單號、作廢對、收據授權），任何一組半填就是建列當下 23514，
// 而錯誤訊息不會告訴你是哪一欄。
//
// 🔴 InvoiceDateTaipei／TaxPeriodCode／StatutoryDeadlineAt **依重開當下重算，
// 既不複製原值、也不留 nil**：重開票是一張新的法定憑證，它的發票日期是重開當下，
// 不是原票的日期——複製過去會讓新票落在已申報的期別裡，那是稅務違規而不是資料瑕疵。
//
// ⚠️ 2026-09-02（#3010）修正：這裡原本留 nil，註解寫著「讓建票路徑依現在算」，
// 而 settleIssued 的註解正好相反——它**原值帶回**（「我方認定的應開立日是收款日，
// document 建立當下就固定」）。兩段各自都對，合起來的結果是**沒有任何人算**
// ⇒ dev 上三張重開票的兩欄 3/3 全 NULL，而首次開立的 5 張全部有值。
// 後果不只是期別彙總漏算：`GatherCorrectionFacts` 在 tax_period_code 為 NULL 時
// 一律 fail-closed（`withinVoid=false`）⇒ 那張新票**再也不能被更正**，
// 買家看到的是「已超過法定作廢期限」（#3020 的同源）。
//
// 🔴 StatutoryDeadlineAt 也**不能複製**，有兩個獨立理由：
//
//	① `chk_invdoc_deadline` 要求 `statutory_deadline_at > issue_not_before`，而
//	   IssueNotBefore 是重開當下 ⇒ 原票開立超過期限天數後再作廢重開，複製過來
//	   的期限已經早於 now，建列當下就是 23514。
//	② 期限天數綁 document_kind（B2C 2 日／B2B 7 日），而本函式正上方剛依新的
//	   買受人重算過 kind ⇒ 個人改統編時複製過來的天數是錯的。
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

	// 🔴 **document_kind 與金額三欄必須依新的買受人重算，不能複製原票的。**
	//
	// 這是 2026-09-01 由 integration test 抓到的（unit test 的 fake 沒有 CHECK 約束）：
	// 買家把「個人」改成「公司統編」時，發票從二聯式（B2C）變成三聯式（B2B），
	// 而兩者的稅額呈現不同——B2C 的 tax_amount 恆 0、統編恆 '0000000000'
	// （chk_invdoc_b2c 雙向守著），B2B 則要把稅額從含稅總額裡拆出來。
	// 原樣複製的症狀是**建列當下 23514**，correction 整筆失敗、買家看到 500。
	//
	// 走 domain.BuildHeader 而不是自己拆：它是「先分攤再算稅、只在表頭做唯一一次除法」
	// 的那份唯一算式，自己算會與 AMEGO 的重算產生尾差並被 3040177／3040178 否決。
	kind := codemeta.InvoiceDocumentKindB2c.Value
	if domain.IsB2B(revised.RecipientTaxID) {
		kind = codemeta.InvoiceDocumentKindB2b.Value
	}
	header := domain.BuildHeader(reissueHeaderLines(lines), revised.RecipientTaxID)

	// 重開票的三個法定欄位（見本函式 javadoc 的第二段紅線）。
	// StatutoryDeadline 的第二個回傳值只有 RECEIPT 會是 false，而重開票必是 B2C／B2B
	// ⇒ 這裡不會落到 nil；仍照 chk_invdoc_deadline_presence 的雙向等式寫成條件式，
	// 因為 kind 的值域長胖時要的是留 NULL 而不是硬塞一個值。
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

		// ① 收件人：修正後的值
		RecipientType:  revised.RecipientType,
		RecipientRefID: revised.RecipientRefID,
		RecipientName:  revised.RecipientName,
		RecipientTaxID: revised.RecipientTaxID,
		RecipientEmail: revised.RecipientEmail,
		// 顯碼＝隱碼是 AMEGO 的硬要求（理由見 normalizeBuyerIdentity 的紅字）。
		// revised 這兩欄由 correction_intake 從已解析的身分填入；2026-09-05 之前
		// 它們**從未被賦值** ⇒ 重開票的載具是空的，而 carrier_type 還在
		// ⇒ 送 AMEGO 是「有載具類型、號碼皆空」＝ 3040132，重開票開不出來。
		CarrierID1:   revised.CarrierID1,
		CarrierID2:   revised.CarrierID2,
		CarrierType:  carrierType,
		DonationCode: donationCode,

		DocumentKind:    kind,
		FulfillmentMode: orig.FulfillmentMode,
		Status:          codemeta.InvoiceDocumentStatusPending.Value,

		// ② 兩個軸：內容軸不動（唯一寫入者是 order-service），實體軸 +1
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
		// 收據沒有稅期別（#3026）。今天收據走不到重開（更正 intake 已 fail-closed 擋掉），
		// 但 #3066 的 B 案可能開通收據的重開／註記 —— 三個建 document 的路徑共用同一份
		// 判準，不留「平行清單只更新一部分」。
		TaxPeriodCode: taxPeriodFor(kind, invoiceDate),

		IssueNotBefore: now,
		// 🔴 PaidAt **複製原票**：重開票是同一筆訂單的同一次收款，錢沒有再進來一次。
		// 這一欄與上面三個法定日期欄的方向正好相反（那三個依重開當下重算），
		// 因為它們問的是不同的問題：法定日期問「這張憑證什麼時候開立」，
		// paid_at 問「這筆訂單什麼時候收到錢」——後者不隨重開改變。
		PaidAt:              orig.PaidAt,
		StatutoryDeadlineAt: deadline,
		SourceSnapshot:      orig.SourceSnapshot,
		Now:                 now,
	}
}

// reissueHeaderLines 把落庫用的明細行折成 domain.Line 供 BuildHeader 使用。
//
// 只取表頭算式真正吃到的四欄（LineType 決定 Emitted、TaxCategory 決定歸類、
// NetAmount 是被加總的值、LineSeq 只為可讀）。**不要**把整個 struct 硬轉：
// 兩邊的欄位集合會各自演化，硬轉會在其中一邊長胖時默默帶進不該進表頭的值。
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

// correctionReason* 是 CORRECTION_REQUEST 軸的 invoice_status_history.reason_code。
//
// 用 R 碼而不是 CodeMeta 值：這一軸的轉移由 §7.3.6 的 R01–R10 定義，而
// reason_code 在別的軸裝的是 CodeMeta 值域——這裡刻意標明來源，
// 讓讀的人一眼看出它對應設計書的哪一列。
const (
	correctionReasonR03 = "CORRECTION_R03_UPDATED_BEFORE_ISSUE"
	correctionReasonR05 = "CORRECTION_R05_REJECTED"
)

// recordCorrectionTransition 是 CORRECTION_REQUEST 軸的 I7 寫入點。
//
// audit 為 nil 時靜默跳過：本 usecase 的其他路徑不因留痕缺席而失敗
// （wiring 缺它是設定問題，不該讓買家的更正送不出去）。
func (u *Correction) recordCorrectionTransition(
	ctx context.Context, correctionID int64, from, to, reasonCode string, now time.Time,
) error {
	if u.audit == nil || from == to {
		return nil
	}
	return appendStatusHistory(ctx, u.audit, subjectCorrectionRequest, correctionID,
		from, to, reasonCode, "", now)
}
