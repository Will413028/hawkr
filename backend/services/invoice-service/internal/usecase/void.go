package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// 作廢送件（f0501）——§7.3.4 M1 的 VOIDING 三條邊 ＋ §7.3.6 的 R05
//
// 為什麼與 Issuance 分開成一個 struct：兩者共用同一條 lane（INV.op，設計書 §7 表列
// 「READY／RETRY_SCHEDULED／SENDING｜打 AMEGO 寫入類（f0401／f0501／g0401／g0501）」），
// 但**業務軸不同**——開票推 document 往 ISSUED，作廢推它往 VOIDED，兩條邊的守衛、
// 落點與失敗語意沒有一項共用。塞進 Issuance 會讓每個 settle* 都多一個「這是哪一軸」
// 的分支，而那正是 §7.3.0 規則 3 要避免的東西。
//
// 🔴 **這個檔案存在的直接理由**：在它之前，R04 建了 VOID operation 而
// `amego.Client.VoidInvoice` 的非測試呼叫端是 **0** ——
// 作廢永遠不會送出去、correction 永遠停在 VOID_REISSUE_IN_PROGRESS、
// 買家看到「處理中」而什麼都不會發生，**且完全不報錯**。
// ---------------------------------------------------------------------------

// Voiding 是作廢軸的 usecase。
type Voiding struct {
	tx          database.TxManager
	docs        domain.DocumentRepository
	ops         domain.OperationRepository
	corrections domain.CorrectionRequestRepository
	accounts    domain.ProviderAccountRepository
	audit       domain.AuditRepository
	provider    ProviderClient
	// outbox／issuers 是 P3（invoice.voided）的兩個依賴（#3055 的橫向稽查）。
	// issuers 只用來取 issuer_ref_id 當 store_id —— 事件 payload 的 store_id 一律走
	// eventwire.StoreIDOf(issuer)，不得由 doc.IssuerID（PK）推導。
	outbox  domain.OutboxRepository
	issuers domain.IssuerRepository
	// adjustments 只用來讀該 operation 服務的那一筆調整，取它的 void_reason_code
	// 決定送給 AMEGO 的 CancelReason（#3073）。作廢有兩個來源——買家更正與訂單退款
	// ——而印在法定憑證稽核上的原因不能兩者共用一句。
	adjustments domain.AdjustmentRepository

	now func() time.Time
	cfg IssuanceConfig
	log *zap.Logger
}

// NewVoiding 組出作廢軸。cfg 與 Issuance 共用同一份——租約長度與退避必須一致，
// 因為兩者搶的是**同一張表的同一組候選**。
func NewVoiding(
	tx database.TxManager,
	docs domain.DocumentRepository,
	ops domain.OperationRepository,
	corrections domain.CorrectionRequestRepository,
	accounts domain.ProviderAccountRepository,
	audit domain.AuditRepository,
	provider ProviderClient,
	outbox domain.OutboxRepository,
	issuers domain.IssuerRepository,
	adjustments domain.AdjustmentRepository,
	cfg IssuanceConfig,
	log *zap.Logger,
) *Voiding {
	return &Voiding{
		tx: tx, docs: docs, ops: ops, corrections: corrections,
		accounts: accounts, audit: audit, provider: provider,
		outbox: outbox, issuers: issuers, adjustments: adjustments,
		now: func() time.Time { return time.Now().UTC() },
		cfg: cfg,
		log: orNop(log),
	}
}

// WithClock 讓測試注入時鐘。
func (u *Voiding) WithClock(now func() time.Time) *Voiding {
	if now != nil {
		u.now = now
	}
	return u
}

func orNop(l *zap.Logger) *zap.Logger {
	if l == nil {
		return zap.NewNop()
	}
	return l
}

// ---------------------------------------------------------------------------
// Claim
// ---------------------------------------------------------------------------

// ClaimForVoidAt 在**單一事務**內做兩件必須同時成立的事：
//
//	① operation READY／RETRY_SCHEDULED／SENDING → SENDING（CAS，租約推到 now+120s）
//	② document ISSUED → VOIDING（M1 的對應邊，觸發者逐字就是 ①）
//
// 與 ClaimForSubmitAt 是**鏡像**關係，差別只在 ② 的目標狀態。拆開之後只要 ② 失敗，
// document 會停在 ISSUED 而 operation 已在 SENDING —— 買家看到「發票有效」而系統
// 正在作廢它，兩邊都不報錯。
//
// 🔴 ② 回 false **不當失敗**：租約到期被重新 claim 時 document 已經在 VOIDING
// （候選述詞含 SENDING，否則崩潰的租約永遠回不來）。真正的守衛是 ① 的 CAS。
//
// 🔴 leaseUntil 由 worker 逐筆現算後傳進來，不在這裡算——算一次會讓不變式從
// 「單筆最壞耗時 < 租約」退化成「批次總耗時 < 租約」，隨 batch-size 一起破。
func (u *Voiding) ClaimForVoidAt(ctx context.Context, operationID int64, now, leaseUntil time.Time) (bool, error) {
	claimed := false
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		op, err := u.ops.GetByID(ctx, operationID)
		if err != nil {
			return err
		}
		ok, err := u.ops.Claim(ctx, operationID, now, leaseUntil)
		if err != nil || !ok {
			return err
		}
		if _, err := u.docs.MarkVoiding(ctx, op.DocumentID, now); err != nil {
			return err
		}
		if err := appendStatusHistory(ctx, u.audit, subjectOperation, op.ID, op.Status,
			codemeta.InvoiceOperationStatusSending.Value, "", "", now); err != nil {
			return err
		}
		// G08：adjustment 跟著進 SENDING。
		//
		// 🔴 這一段在 #3073 之前**整個不存在**：作廢軸只推進 document 與 operation，
		// 從來沒有人動過 invoice_adjustments。實測 dev 上 12 筆 VOID adjustment 全部
		// 停在 PENDING，而它們的 operation 早就進了 FAILED_PERMANENT／
		// PROVIDER_STATUS_RECHECK、document 也已經 VOIDED／SUPERSEDED —— 也就是說
		// 那張表的狀態機從來沒有被驅動過，而在 invoice_adjustment_stuck_rows 出現
		// 之前沒有任何東西看得到這件事。
		if err := u.advanceAdjustment(ctx, op, adjustmentSending, "", now); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return claimed, err
}

// ---------------------------------------------------------------------------
// Submit
// ---------------------------------------------------------------------------

// ErrVoidMissingInvoiceNumber 是「要作廢的 document 沒有發票號碼」。
//
// 這在結構上不該發生（M1 只讓 ISSUED 進 VOIDING，而 ISSUED 必有號碼），所以它是
// 我方資料錯 ⇒ 永久失敗進 ADM 工作台，不重試。
var ErrVoidMissingInvoiceNumber = errors.New("invoice: document has no provider invoice number, cannot void")

// SubmitVoid 送出一次 f0501。呼叫端（INV.op worker）必須已經 ClaimForVoidAt 成功。
//
// 回 error 只代表「這一輪連狀態都沒寫成」（DB 斷線之類）——所有業務落點都回 nil，
// 狀態已經寫定。這與 Issuance.Submit 的契約逐字相同，因為兩者共用同一個 worker。
func (u *Voiding) SubmitVoid(ctx context.Context, operationID int64) error {
	op, err := u.ops.GetByID(ctx, operationID)
	if err != nil {
		return err
	}
	if op.Status != codemeta.InvoiceOperationStatusSending.Value {
		// 沒有租約就送 ＝ 可能有第二個持有者同時在打 AMEGO ⇒ 重複作廢。
		return fmt.Errorf("invoice: operation %d is %s, expected SENDING (claim first)", op.ID, op.Status)
	}

	doc, err := u.docs.GetByID(ctx, op.DocumentID)
	if err != nil {
		return err
	}

	// resume checkpoint：已經是 VOIDED 就不再打 AMEGO。與開票路徑的
	// 「已有發票號碼就不再送」同一道防線，擋的是本地已知結果之後的任何重試。
	if doc.Status == codemeta.InvoiceDocumentStatusVoided.Value {
		return u.settleVoidAccepted(ctx, doc, op, u.now())
	}

	if doc.ProviderInvoiceNo == nil || *doc.ProviderInvoiceNo == "" {
		return u.settleVoidPermanent(ctx, doc, op, "", "", ErrVoidMissingInvoiceNumber.Error(), u.now())
	}

	account, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, providerAmego)
	if err != nil {
		return err
	}

	req := amego.VoidRequest{
		CancelInvoiceNumber: *doc.ProviderInvoiceNo,
		CancelReason:        u.cancelReasonFor(ctx, op),
	}

	var sendErr error
	// 🔴 AppKey 明文的活動範圍只到這個 callback：不回傳、不 log、不進任何 struct 欄位。
	// op.AttemptCount 是「先前失敗次數」（bumpAttempt 在 process 失敗後才加）⇒ +1 才是 1-based。
	auditCtx := amego.WithAuditContext(ctx, op.ID, int(op.AttemptCount)+1)
	if err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(providerTaxID, appKey string) error {
		_, sendErr = u.provider.VoidInvoice(auditCtx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey}, req)
		return nil
	}); err != nil {
		return err
	}

	now := u.now()
	if sendErr == nil {
		return u.settleVoidAccepted(ctx, doc, op, now)
	}
	return u.settleVoidFailure(ctx, doc, op, sendErr, now)
}

// voidCancelReason 產生 CancelReason。
//
// 🔴 **上限 20 字**（AMEGO 3050113：CancelReason 超過 20 字 ⇒ 我方送出前截斷）。
// 用固定字串而不是把買家填的理由帶出去：買家的文字長度不受控，而截斷一個法定憑證
// 的作廢理由會產生半句話。真正的理由留在 invoice_correction_requests 裡供稽核。
const (
	// voidReasonCorrection 是買家更正路徑的 CancelReason（M6 R04）。
	voidReasonCorrection = "買方資料更正"
	// voidReasonOrderRefund 是訂單退款路徑的 CancelReason（§7.2.2 決策樹的 VOID 葉子）。
	// 兩者都在 20 字以內。
	voidReasonOrderRefund = "訂單退款"
)

// cancelReasonFor 依這筆作廢的來源決定 CancelReason。
//
// 🔴 兩個來源不能共用一句：印在法定憑證稽核上的作廢原因寫錯，是對外的不實記載。
// 讀不到 adjustment 時退回「買方資料更正」——那是本服務第一個、也是目前多數的來源，
// 而 fail 掉整次作廢的代價（發票停在 VOIDING、買家的更正卡住）遠大於原因文字不精準。
func (u *Voiding) cancelReasonFor(ctx context.Context, op domain.Operation) string {
	if op.AdjustmentID == nil {
		return voidReasonCorrection
	}
	adj, err := u.adjustments.GetByID(ctx, *op.AdjustmentID)
	if err != nil {
		u.log.Warn("invoice.void.adjustment_lookup_failed",
			zap.Int64("operation_id", op.ID), zap.Int64p("adjustment_id", op.AdjustmentID), zap.Error(err))
		return voidReasonCorrection
	}
	if adj.TriggerKind == codemeta.InvoiceAdjustmentTriggerOrderRefund.Value {
		return voidReasonOrderRefund
	}
	return voidReasonCorrection
}

// ---------------------------------------------------------------------------
// 落點
// ---------------------------------------------------------------------------

// voidRejectReasonOf 是 §7.3.6 R05 的碼對映：f0501 的拒絕碼 → 買家看得到的理由碼。
//
// 回空字串代表「這個碼不是 R05 的自動終局」——呼叫端要走別的落點，**不得**塞一個
// 猜的理由給買家（發票是法定憑證，錯的因會讓買家做出錯的後續動作）。
//
// 🔴 3050123 曾有一個權威來源矛盾，已於 2026-09-01 由 DEC-20260831-234750 裁決（選 A）：
//   - 58 號設計書 §7.3.6 R05 表 ＋ CodeMeta 原本說它是「已申報」→ ALREADY_REPORTED
//   - AMEGO 官方碼表（docs/第三方整合 :869）、amego/codes.go、**以及同一份設計書的
//     §7 時序圖**都說它是「已存在**註銷**」
//
// 碼義不是我方能定義的 ⇒ 依官方語意改對應到新增的 PROVIDER_STATE_CONFLICT。
// ALREADY_REPORTED 保留給 R02 的預檢——那是我方依期別自己算得出來的（ReportedStatus
// 是純函式），它沒有、也不需要對應的 AMEGO 錯誤碼。
//
// ⚠️ 註銷（3050123）與作廢（3050122，我方視為成功等價）是兩種不同動作，不要合併。
func voidRejectReasonOf(providerCode string) string {
	switch providerCode {
	case "3050123":
		return codemeta.InvoiceCorrectionRejectReasonProviderStateConflict.Value
	case "3050126":
		return codemeta.InvoiceCorrectionRejectReasonPastVoidDeadline.Value
	case "3050141":
		return codemeta.InvoiceCorrectionRejectReasonAllowanceExists.Value
	default:
		return ""
	}
}

// settleVoidAccepted 是「AMEGO 受理了作廢」的落點。
//
// 🔴 **受理 ≠ 已作廢**：f0501 回 code:0 之後，invoice_query 的 invoice_type 仍是
// C0401，作廢只是排進 AMEGO 的 wait[] 陣列（設計書 §5.1.5、光貿 API 參考 §2.3.4 實測）。
// 所以這裡**不能**寫 VOIDED —— document 留在 VOIDING，operation 落
// WAITING_CONFIRMATION 進入 INV.rc 的重查迴圈，由那條路徑確認 invoice_type = C0501
// 之後才轉 VOIDED。
//
// 把「動作成功」當成「憑證已變成作廢」會讓 R10 提早觸發 ⇒ 在原票還沒真的作廢時就
// 開出新票 ⇒ 同一筆交易兩張有效發票。
func (u *Voiding) settleVoidAccepted(ctx context.Context, doc domain.Document, op domain.Operation, now time.Time) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.ops.MarkWaitingConfirmation(ctx, domain.MarkOperationUnknown{
			ID:            op.ID,
			NextAttemptAt: now.Add(u.cfg.RecheckBase),
			Now:           now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter WAITING_CONFIRMATION (rows=0)", op.ID)
		}
		if err := appendStatusHistory(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusWaitingConfirmation.Value, "", "", now); err != nil {
			return err
		}
		// G12：f0501 受理但作廢是非同步的 ⇒ adjustment 也進 WAITING_CONFIRMATION，
		// 由 ConfirmVoid（INV.rc 分派）推到 COMPLETED。
		return u.advanceAdjustment(ctx, op, adjustmentWaitingConfirmation, "", now)
	})
}

// settleVoidRejected 是 R05：AMEGO 明確拒絕作廢。
//
// 同一事務內四件事，缺一都會留下不一致：
//
//	① operation → FAILED_PERMANENT（這條路走不通了，不再重試）
//	② document VOIDING → ISSUED（**買家手上那張仍然有效**，只是內容改不了）
//	③ correction → REJECTED ＋ reject_reason_code（B06 顯示後端給的具體理由）
//	④ 釋放 uq_invcorr_open_per_document 的名額 ⇒ 買家可另提一次修改請求（③ 的副作用）
//
// ③ 找不到 open correction **不是錯誤**：VOID operation 也可能由退款折讓路徑建立
// （那條沒有 correction）。這時只做 ①②。
func (u *Voiding) settleVoidRejected(
	ctx context.Context, doc domain.Document, op domain.Operation,
	providerCode, providerMessage, reasonCode, lastError string, now time.Time,
) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		// 🔴 走 MarkFailedWithProviderResult：AMEGO 的碼歸 provider_code
		// （#3082，DEC-20260904-120527 選 A）。在此之前它被寫進
		// invoice_status_history.reason_code，而那一欄的值域是 CodeMeta ——
		// 折讓軸裝 CodeMeta 值、作廢軸裝 AMEGO 碼，同一欄兩種語意，
		// ADM 要分類統計只能靠字串長相猜。
		ok, err := u.ops.MarkFailedWithProviderResult(ctx, domain.MarkOperationProviderResult{
			// 🔴 ProviderMessage 是**原文**（APIError.Msg），與 LastError 不是同一件事
			// （#3082 收尾，2026-09-04 dev 實測發現）：LastError 是我方組的診斷字串
			// （`amego: /json/f0401: code=… class=… kind=…`），provider_message 要的是
			// 對方原樣回的那句話——§8.2.1 稱它「落庫供稅務舉證」。
			//
			// dev 實證：重試一筆 REISSUE 後 provider_code=3040122 有落庫、
			// provider_message 仍是 NULL ⇒ 本票的宣稱只修好了一半。
			ID:              op.ID,
			ProviderCode:    providerCode,
			ProviderMessage: providerMessage,
			LastError:       lastError,
			Now:             now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter FAILED_PERMANENT (rows=0)", op.ID)
		}
		// reason_code 一律是 CodeMeta 值；provider 的原話在 provider_code 與 detail。
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusFailedPermanent.Value,
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value,
			lastError, "", now); err != nil {
			return err
		}

		restored, err := u.docs.RestoreToIssued(ctx, doc.ID, now)
		if err != nil {
			return err
		}
		if !restored {
			// document 不在 VOIDING ⇒ 我對狀態的理解與 DB 不符。整筆 rollback：
			// 放行會留下「operation 說作廢失敗、document 卻停在 VOIDING」的組合，
			// 而 VOIDING 沒有任何掃描器會把它撿回來。
			return fmt.Errorf("invoice: document %d could not return to ISSUED from VOIDING (rows=0, status=%s)",
				doc.ID, doc.Status)
		}
		// reason_code 一律是 CodeMeta 值（#3082）；provider 的原話走 detail。
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectDocument, doc.ID,
			codemeta.InvoiceDocumentStatusVoiding.Value,
			codemeta.InvoiceDocumentStatusIssued.Value,
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value,
			lastError, "", now); err != nil {
			return err
		}

		// 收掉 R04 同交易內建好的重開票 —— 作廢被拒 ⇒ 那張票永遠不會開。
		//
		// 🔴 不收的後果是兩個靜默終點：document 永遠停在 PENDING（沒有 operation
		// 會撿它，因為它的 REISSUE operation 停在 CREATED），而那張 PENDING 會被
		// 賣家面與 ADM 列表當成「待開立」顯示。
		//
		// 用**決定性的 provider_request_no** 反查而不是掃 original_document_id：
		// 後者沒有索引（fk 不會自動建），前者有 uq_invdoc_provider_request_no。
		if err := u.cancelPendingReissue(ctx, doc, providerCode, now); err != nil {
			return err
		}

		corr, err := u.corrections.GetOpenByDocument(ctx, doc.ID)
		if err != nil {
			// 「沒有 open correction」在 repo 層被折成業務碼（§8.1.4：查無列與越權
			// 不可區分），所以這裡用 isBizCode 判而不是 errors.Is 某個 sentinel。
			if isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
				// 退款作廢路徑建的 VOID operation 沒有 correction。①② 已經做完。
				u.log.Info("invoice.void.rejected_without_correction",
					zap.Int64("operation_id", op.ID), zap.Int64("document_id", doc.ID),
					zap.String("provider_code", providerCode))
				// G10：adjustment 也要落地，否則它會永遠停在 SENDING。
				return u.advanceAdjustment(ctx, op, adjustmentFailedPermanent, reasonCode, now)
			}
			return err
		}
		detail := lastError
		rejected, err := u.corrections.Reject(ctx, domain.RejectCorrection{
			ID:           corr.ID,
			ReasonCode:   reasonCode,
			ReasonDetail: &detail,
			Now:          now,
		})
		if err != nil {
			return err
		}
		if !rejected {
			return fmt.Errorf("invoice: correction %d could not enter REJECTED (rows=0, status=%s)",
				corr.ID, corr.Status)
		}
		if err := appendStatusHistory(ctx, u.audit, subjectCorrectionRequest, corr.ID,
			corr.Status, codemeta.InvoiceCorrectionStatusRejected.Value, reasonCode, "", now); err != nil {
			return err
		}
		// G10：correction 那條路徑的 adjustment 同樣要落地。
		return u.advanceAdjustment(ctx, op, adjustmentFailedPermanent, reasonCode, now)
	})
}

// settleVoidPermanent 是「作廢永久失敗、但不是 R05 的三個碼」的落點。
//
// 與 settleVoidRejected 的差別：那個有一個**能告訴買家的具體理由**，這個沒有。
// 硬塞一個理由碼給買家比讓它進 ADM 工作台更糟——所以這裡只做 ①②，correction
// 留在 VOID_REISSUE_IN_PROGRESS 由 ADM 處置（A06）。
func (u *Voiding) settleVoidPermanent(
	ctx context.Context, doc domain.Document, op domain.Operation,
	providerCode, providerMessage, lastError string, now time.Time,
) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		// 🔴 同 settleVoidRejected：AMEGO 的碼歸 provider_code，不進 reason_code（#3082）。
		ok, err := u.ops.MarkFailedWithProviderResult(ctx, domain.MarkOperationProviderResult{
			// 🔴 ProviderMessage 是**原文**（APIError.Msg），與 LastError 不是同一件事
			// （#3082 收尾，2026-09-04 dev 實測發現）：LastError 是我方組的診斷字串
			// （`amego: /json/f0401: code=… class=… kind=…`），provider_message 要的是
			// 對方原樣回的那句話——§8.2.1 稱它「落庫供稅務舉證」。
			//
			// dev 實證：重試一筆 REISSUE 後 provider_code=3040122 有落庫、
			// provider_message 仍是 NULL ⇒ 本票的宣稱只修好了一半。
			ID:              op.ID,
			ProviderCode:    providerCode,
			ProviderMessage: providerMessage,
			LastError:       lastError,
			Now:             now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter FAILED_PERMANENT (rows=0)", op.ID)
		}
		// reason_code 一律是 CodeMeta 值；provider 的原話在 provider_code 與 detail。
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusFailedPermanent.Value,
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value,
			lastError, "", now); err != nil {
			return err
		}
		// document 只有在真的進過 VOIDING 時才需要還原。
		if doc.Status == codemeta.InvoiceDocumentStatusVoiding.Value {
			if _, err := u.docs.RestoreToIssued(ctx, doc.ID, now); err != nil {
				return err
			}
		}
		// G10：adjustment 跟著進 FAILED_PERMANENT。理由碼不得為空
		// （chk_invadj_blocked_reason）。
		//
		// 🔴 blocked_reason_code 的值域是 **InvoiceManualReason**（CodeMeta 文件
		// 逐字列了 invoice_adjustments.blocked_reason_code 這個落點）⇒ 在此之前
		// 這裡塞 AMEGO 碼是第三處混用（#3082）。沒有 provider code 時原本退回
		// InvoiceCancelReasonVoidRejected —— 那又是**另一個值域**的值。
		reason := codemeta.InvoiceManualReasonProviderRejectedPermanent.Value
		if providerCode == "" {
			// 不是 provider 拒絕（例如原票沒有發票號碼就走到這裡）。
			reason = codemeta.InvoiceManualReasonAllowanceNoSourceInvoice.Value
		}
		return u.advanceAdjustment(ctx, op, adjustmentFailedPermanent, reason, now)
	})
}

// settleVoidFailure 把一次 f0501 失敗翻成落點。
//
// 分類的權威是 amego.APIError.Class（與開票路徑同一份分類表），**不是** HTTP status
// 也不是 msg 字串。四類的處置在作廢軸上與開票軸**不同**，所以不能共用 settleFailure：
//
//	Retryable  → 留在 VOIDING 重試（3050121／3050131「上一個異動還在排隊」，延遲 30~60s）
//	Reconcile  → WAITING_CONFIRMATION（含 3050122「已存在開立作廢」＝ successEquivalent，
//	             以及逾時／連線中斷——作廢已經送出去了，**不得盲目重送**，去查）
//	Manual／Permanent → R05（碼在對映表內）或永久失敗（不在表內）
//
// 🔴 Retryable 這一支特別重要：作廢有法定期限（§7.3.6 R02 的 PAST_VOID_DEADLINE），
// 而 3050121／3050131 只是排隊。把它當永久失敗會讓買家在期限內明明作廢得了卻被拒。
func (u *Voiding) settleVoidFailure(
	ctx context.Context, doc domain.Document, op domain.Operation, sendErr error, now time.Time,
) error {
	var apiErr *amego.APIError
	if !errors.As(sendErr, &apiErr) {
		// 非 provider 錯誤（ctx 取消、transport 尚未分類）⇒ 結果不明。作廢可能已經
		// 送達 ⇒ **不得重送**，走對帳。
		return u.settleVoidAccepted(ctx, doc, op, now)
	}

	switch apiErr.Class {
	case amego.ClassRetryable:
		return u.settleVoidRetry(ctx, doc, op, apiErr, now)

	case amego.ClassReconcile:
		// 含 successEquivalent（3050122 已存在開立作廢）與逾時。兩者的處置相同：
		// 去問 AMEGO。盲目重送才是會造成狀態機衝突的那條路。
		return u.settleVoidAccepted(ctx, doc, op, now)

	case amego.ClassManual, amego.ClassPermanent:
		if reason := voidRejectReasonOf(apiErr.Code); reason != "" {
			return u.settleVoidRejected(ctx, doc, op, apiErr.Code, apiErr.Msg, reason, apiErr.Error(), now)
		}
		return u.settleVoidPermanent(ctx, doc, op, apiErr.Code, apiErr.Msg, apiErr.Error(), now)

	default:
		// 分類值域長胖時預設走對帳：多打一次唯讀查詢，代價最小且會自己收斂。
		return u.settleVoidAccepted(ctx, doc, op, now)
	}
}

// settleVoidRetry 是 J09 SENDING → RETRY_SCHEDULED（帶退避）。
//
// document **留在 VOIDING**：下一輪的 claim 述詞含 SENDING，而 MarkVoiding 對已在
// VOIDING 的列回 false 是合法的。把它還原成 ISSUED 再重試會讓買家面在兩個狀態之間
// 閃爍，而且每一次還原都要多寫一筆歷程。
func (u *Voiding) settleVoidRetry(
	ctx context.Context, doc domain.Document, op domain.Operation, apiErr *amego.APIError, now time.Time,
) error {
	newAttempt := op.AttemptCount + 1
	exhausted := newAttempt >= op.MaxAttempts
	delay := backoffFor(int(newAttempt), u.cfg.BaseBackoff, u.cfg.MaxBackoff)
	lastErr := apiErr.Error()

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := u.ops.UpdateAttempt(ctx, op.ID, newAttempt, now.Add(delay), &lastErr, now); err != nil {
			return err
		}
		// #3082 第二階段：reason_code 一律 CodeMeta 值，AMEGO 的原話走 reason_detail。
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusRetryScheduled.Value,
			codemeta.InvoiceManualReasonProviderRetryableError.Value,
			lastErr, "", now); err != nil {
			return err
		}
		if !exhausted {
			// document **留在 VOIDING**：下一輪 claim 的述詞含 SENDING，而 MarkVoiding
			// 對已在 VOIDING 的列回 false 是合法的。還原成 ISSUED 再重試會讓買家面在
			// 兩個狀態之間閃爍，每一次還原還要多寫一筆歷程。
			return nil
		}

		ok, err := u.ops.MarkRetryExhausted(ctx, op.ID, lastErr, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter FAILED_RETRY_EXHAUSTED (rows=0)", op.ID)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusRetryScheduled.Value,
			codemeta.InvoiceOperationStatusFailedRetryExhausted.Value,
			codemeta.InvoiceManualReasonRetryBudgetExhausted.Value,
			lastErr, "", now); err != nil {
			return err
		}
		// 🔴 重試用盡時 document **必須**離開 VOIDING。
		// VOIDING 不在任何掃描器的候選集合裡（#2959 ⑧ 逐字記載這一點），留在那裡
		// 就是一個靜默終點：買家看到「處理中」而永遠不會有下一步。
		// 還原成 ISSUED 是正確的落點——那張憑證從頭到尾都沒被作廢掉。
		if doc.Status == codemeta.InvoiceDocumentStatusVoiding.Value ||
			doc.Status == codemeta.InvoiceDocumentStatusIssued.Value {
			if _, err := u.docs.RestoreToIssued(ctx, doc.ID, now); err != nil {
				return err
			}
		}
		return nil
	})
}

// cancelPendingReissue 收掉 R04 建好但永遠不會開的重開票（document ＋ REISSUE operation）。
//
// 查無 ⇒ 不是錯誤：退款折讓路徑建的 VOID operation 沒有重開票。
func (u *Voiding) cancelPendingReissue(
	ctx context.Context, orig domain.Document, providerCode string, now time.Time,
) error {
	if orig.SourceRef == nil || *orig.SourceRef == "" {
		return nil // MANUAL 來源沒有作廢重開
	}
	requestNo := domain.ProviderRequestNoForOrder(*orig.SourceRef, orig.SnapshotVersion, orig.IssueAttemptSeq+1)
	reissue, err := u.docs.GetByProviderRequestNo(ctx, requestNo)
	if err != nil {
		if isBizCode(err, bizcode.InvoiceDocumentNotFound) {
			return nil
		}
		return err
	}
	if reissue.Status != codemeta.InvoiceDocumentStatusPending.Value {
		// 已經被收過（重投），或它已經開出去了（那不該發生，但不由本函式判定）。
		return nil
	}

	ops, err := u.ops.ListByDocument(ctx, reissue.ID)
	if err != nil {
		return err
	}
	for _, op := range ops {
		if op.OperationType != codemeta.InvoiceOperationTypeReissue.Value {
			continue
		}
		if _, err := u.ops.CancelBeforeIssue(ctx, op.ID,
			codemeta.InvoiceCancelReasonVoidRejected.Value, "作廢被 provider 拒絕（"+providerCode+"）", now); err != nil {
			return err
		}
	}
	if _, err := u.docs.CancelBeforeIssue(ctx, reissue.ID,
		codemeta.InvoiceCancelReasonVoidRejected.Value, "作廢被 provider 拒絕（"+providerCode+"）", now); err != nil {
		return err
	}
	u.log.Info("invoice.void.reissue_cancelled",
		zap.Int64("original_document_id", orig.ID), zap.Int64("reissue_document_id", reissue.ID),
		zap.String("provider_code", providerCode))
	return nil
}

// ---------------------------------------------------------------------------
// 作廢確認（INV.rc 的 VOID 分支）
// ---------------------------------------------------------------------------

// ConfirmVoid 對一筆 WAITING_CONFIRMATION 的 VOID operation 打**一次** invoice_query，
// 確認 invoice_type 是否已經變成 C0501。
//
// 🔴 為什麼作廢需要「第二段」：f0501 回 code:0 只代表 AMEGO **受理**了請求，
// invoice_query 的 invoice_type 仍是 C0401，作廢只排進了 wait[] 陣列
// （設計書 §5.1.5、光貿 API 參考 §2.3.4 實測）。「動作成功」與「憑證已變成作廢」
// 之間必然有時間差，用同一個欄位表達那段時間只能靠猜。
//
// 🔴 為什麼它在 INV.rc 這條 lane 而不是另開一支 worker：WAITING_CONFIRMATION 已經在
// FetchPendingInvoiceRechecks 的候選述詞裡，兩支掃描器掃同一個集合就是搶同一批列。
//
// 確認成功時同一事務內兩件事：
//
//	① document VOIDING → VOIDED
//	② R04 建好的 REISSUE operation CREATED → READY（**推進它的唯一執行者就是這裡**）
//
// ② 少了就是本服務反覆出現的那個 class：建了一列而沒有東西會推它。症狀是原票已作廢、
// 新票永遠不開、correction 永遠停在 VOID_REISSUE_IN_PROGRESS，而且完全不報錯。
func (u *Voiding) ConfirmVoid(ctx context.Context, op domain.Operation) error {
	doc, err := u.docs.GetByID(ctx, op.DocumentID)
	if err != nil {
		return err
	}
	if doc.Status == codemeta.InvoiceDocumentStatusVoided.Value {
		// 已經確認過（重投）。仍要確保 ② 做過——它與 ① 在同一事務，但重投時
		// ① 會 rows=0，所以這裡走一次不含 ① 的路徑。
		//
		// 🔴 **operation 的收尾也必須在這裡做。** 原本這一支只呼叫 promoteReissue 就
		// return，於是「主路徑把 document 推進 VOIDED、但 operation 沒收尾成功」之後，
		// 每一輪 INV.rc 都落到這裡、每一輪都 return nil ⇒ 這一列永遠留在候選集合裡。
		// dev 實測（INV58R3-ALW01）：document VOIDED、adjustment COMPLETED、
		// invoice.voided 已送達，而 operation 停在 PROVIDER_STATUS_RECHECK 被無限 claim。
		now := u.now()
		return u.tx.WithTx(ctx, func(ctx context.Context) error {
			if err := u.promoteReissue(ctx, doc, now); err != nil {
				return err
			}
			return finishOperationSucceeded(ctx, u.ops, u.audit, op, now)
		})
	}
	if doc.ProviderInvoiceNo == nil || *doc.ProviderInvoiceNo == "" {
		return fmt.Errorf("invoice: document %d has no invoice number, cannot confirm void", doc.ID)
	}

	account, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, providerAmego)
	if err != nil {
		return err
	}
	var detail *amego.InvoiceDetail
	var queryErr error
	confirmAuditCtx := amego.WithAuditContext(ctx, op.ID, int(op.AttemptCount)+1)
	if err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(providerTaxID, appKey string) error {
		detail, queryErr = u.provider.QueryInvoiceByNumber(confirmAuditCtx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey},
			*doc.ProviderInvoiceNo)
		return nil
	}); err != nil {
		return err
	}

	now := u.now()
	if queryErr != nil || detail == nil {
		// 查不到結果 ⇒ 仍不確定。**不得**當成「沒作廢成功」而重送：作廢已經送出去了。
		// 退避後再查；租約由呼叫端（INV.rc）的 claim 管。
		u.log.Warn("invoice.void.confirm_query_failed",
			zap.Int64("operation_id", op.ID), zap.Error(queryErr))
		return u.scheduleVoidRecheck(ctx, op, now)
	}
	if detail.InvoiceType != voidedInvoiceType {
		// 還在 wait[] 佇列裡。這是**正常**的中間狀態不是錯誤。
		return u.scheduleVoidRecheck(ctx, op, now)
	}

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.docs.MarkVoided(ctx, doc.ID, now)
		if err != nil {
			return err
		}
		// alreadyApplied ＝ document 已經越過 VOIDING 那一格。MarkVoided 的
		// allowed-from 只有 VOIDING，所以這裡的 rows=0 有兩種可能，而上面那句
		// 「AMEGO 回報 invoice_type = C0501」已經把「作廢沒生效」排除掉了 ⇒
		// 剩下的唯一解釋是這一步早就做完，收尾沒做完。
		//
		// 🔴 原本這裡無條件回 error，而 INV.rc 的迴圈只 log 一行就 continue ⇒
		// operation 永遠留在 PROVIDER_STATUS_RECHECK、**每一輪都真的打一次 AMEGO**，
		// 且 unknown_since 從不推進 ⇒ scheduleVoidRecheck 的 24h 放棄門檻永遠到不了。
		// 這是一個沒有出口的迴圈，而且完全無聲：stuck 指標不計 WAITING_CONFIRMATION、
		// 唯一的訊號是沒有人在看的 error log。dev 實測 4 筆列在單一 pod 的 43 分鐘內
		// 產生 84 次 invoice.rc.confirm_void_failed。
		alreadyApplied := !ok
		if alreadyApplied &&
			doc.Status != codemeta.InvoiceDocumentStatusVoided.Value &&
			doc.Status != codemeta.InvoiceDocumentStatusSuperseded.Value {
			return fmt.Errorf("invoice: document %d could not enter VOIDED (rows=0, status=%s)", doc.ID, doc.Status)
		}
		if alreadyApplied {
			u.log.Warn("invoice.void.document_already_past_voiding",
				zap.Int64("operation_id", op.ID), zap.Int64("document_id", doc.ID),
				zap.String("document_status", doc.Status))
		} else if err := appendStatusHistory(ctx, u.audit, subjectDocument, doc.ID,
			codemeta.InvoiceDocumentStatusVoiding.Value,
			codemeta.InvoiceDocumentStatusVoided.Value, "", "", now); err != nil {
			return err
		}
		// 🔴 走 finishOperationSucceeded 而不是 MarkDone：本函式由 INV.rc 分派進來，
		// claim 已把 status 蓋成 PROVIDER_STATUS_RECHECK ⇒ MarkDone（守 SENDING）
		// 必然 rows=0，而原本那個 rows 被丟給 `_`、軌跡照寫 ⇒ operation 永遠不收尾。
		if err := finishOperationSucceeded(ctx, u.ops, u.audit, op, now); err != nil {
			return err
		}
		// 事件只在**本次**真的把 document 推進 VOIDED 時發：已經走過那一格的話，
		// 當初那個事務裡就發過了，重發只是給下游一筆重複。
		if !alreadyApplied {
			if err := u.emitVoided(ctx, doc, now); err != nil {
				return err
			}
		}
		// G09：作廢經 invoice_query 確認生效 ⇒ adjustment 也到終點。
		if err := u.advanceAdjustment(ctx, op, adjustmentCompleted, "", now); err != nil {
			return err
		}
		return u.promoteReissue(ctx, doc, now)
	})
}

// voidedEvent 是 P3（`invoice.voided`，58 §6.3.9 定版 13 列的第 4 列）。
//
// 欄位取自該模板實際用到的每一個 `#{...}` 與 data_fields[*].field，不是我猜的：
// notification_ref_id／invoice_document_id／voided_invoice_no。
type voidedEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	InvoiceDocumentID string            `json:"invoice_document_id"`
	OrderNo           string            `json:"order_no"`
	BuyerID           eventwire.BuyerID `json:"buyer_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	VoidedInvoiceNo   string            `json:"voided_invoice_no"`
	VoidReasonCode    string            `json:"void_reason_code"`
	OccurredAt        string            `json:"occurred_at"`
}

// emitVoided 在原票真的轉 VOIDED 的**同一個交易內**寫 P3。
//
// 🔴 這一則與 R10 的 `invoice.reissued` 是**兩件事**：作廢可以單獨發生（退款折讓路徑
// 就不重開），而買家在那條路徑上完全收不到通知 —— 手上那張憑證失效了卻沒有人告訴他。
// 作廢重開的情境下兩則都會發，順序是「已作廢」→「已重新開立」，那正是事情發生的順序。
//
// 冪等鍵 `INV:{invoice_document_id}`（CodeMeta `INVOICE_VOIDED` 逐字）：與同一份文件的
// `INVOICE_ISSUED` 同鍵但不同 event_type ⇒ notification 端的
// UNIQUE (user_id, user_type, event_type, reference_id) 不會把它吞掉。
func (u *Voiding) emitVoided(ctx context.Context, doc domain.Document, now time.Time) error {
	// issuer 讀不到就不發，而不是用 doc.IssuerID（PK）頂替：那個值送到下游會被當成
	// store_id 去解析賣家，解析必失敗且全程零錯誤（#3055 的 migration 檔頭記過這個 class）。
	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(voidedEvent{
		NotificationRefID: "INV:" + strconv.FormatInt(doc.ID, 10),
		InvoiceDocumentID: strconv.FormatInt(doc.ID, 10),
		OrderNo:           derefString(doc.SourceRef),
		BuyerID:           eventwire.BuyerIDOfDocument(doc),
		StoreID:           eventwire.StoreIDOf(issuer),
		VoidedInvoiceNo:   derefString(doc.ProviderInvoiceNo),
		VoidReasonCode:    derefString(doc.ManualReasonCode),
		OccurredAt:        timeutil.FormatISO8601(now),
	})
	if err != nil {
		return err
	}
	_, err = u.outbox.Insert(ctx, domain.NewOutboxEvent{
		AggregateType: outboxAggregateD,
		AggregateID:   doc.ID,
		EventType:     string(pkgmq.EventTypeInvoiceVoided),
		Exchange:      pkgmq.ExchangeInvoice,
		RoutingKey:    pkgmq.RoutingKeyInvoiceVoided,
		Payload:       payload,
		NextAttemptAt: now,
		Now:           now,
	})
	return err
}

// voidedInvoiceType 是 AMEGO 對「已作廢的 B2C 發票」回的 invoice_type。
//
// 錨在這一個常數而不是散在判斷式裡：它是我方與 AMEGO 之間唯一「作廢真的完成了」的
// 判準，寫錯的後果是永遠確認不到（correction 永遠停在進行中）或提早確認
// （原票還沒作廢就開新票 ⇒ 同一筆交易兩張有效發票）。
const voidedInvoiceType = "C0501"

// promoteReissue 把 R04 建好、停在 CREATED 的 REISSUE operation 推到 READY。
//
// 查無重開票 ⇒ 不是錯誤：退款折讓路徑建的 VOID operation 沒有重開票。
func (u *Voiding) promoteReissue(ctx context.Context, orig domain.Document, now time.Time) error {
	if orig.SourceRef == nil || *orig.SourceRef == "" {
		return nil
	}
	requestNo := domain.ProviderRequestNoForOrder(*orig.SourceRef, orig.SnapshotVersion, orig.IssueAttemptSeq+1)
	reissue, err := u.docs.GetByProviderRequestNo(ctx, requestNo)
	if err != nil {
		if isBizCode(err, bizcode.InvoiceDocumentNotFound) {
			return nil
		}
		return err
	}
	ops, err := u.ops.ListByDocument(ctx, reissue.ID)
	if err != nil {
		return err
	}
	for _, op := range ops {
		if op.OperationType != codemeta.InvoiceOperationTypeReissue.Value ||
			op.Status != codemeta.InvoiceOperationStatusCreated.Value {
			continue
		}
		promoted, err := u.ops.Promote(ctx, op.ID, codemeta.InvoiceOperationStatusReady.Value, now, now)
		if err != nil {
			return err
		}
		if !promoted {
			return fmt.Errorf("invoice: reissue operation %d could not be promoted to READY (rows=0)", op.ID)
		}
		u.log.Info("invoice.void.reissue_promoted",
			zap.Int64("original_document_id", orig.ID),
			zap.Int64("reissue_document_id", reissue.ID), zap.Int64("operation_id", op.ID))
	}
	return nil
}

// scheduleVoidRecheck 把這一筆排回重查迴圈，**但有上界**。
//
// 🔴 上界不可省。少了它，「AMEGO 一直不把 invoice_type 翻成 C0501」會讓這一筆
// 無限重查，而 document 停在 VOIDING —— VOIDING **不在任何掃描器的候選集合裡**，
// 也不在 M7／M32 的指標涵蓋範圍內（#2959 ⑧ 逐字記載這一點）⇒ 那是一個
// 沒有任何訊號的永久停滯，而買家看到的是「處理中」。
//
// 門檻沿用開票軸的 recheckGiveUpAfter（24h，§7.3.2 J17／J31 的同一個窗口）：
// 兩條路徑面對的是同一件事——「送出去了但問不到結果」，沒有理由給不同的耐心。
//
// 超過門檻時落 FAILED_UNKNOWN 而**不是** FAILED_PERMANENT，也不還原 document：
//   - 落 PERMANENT 等於宣稱「作廢沒成功」，而我方並不知道
//   - 還原成 ISSUED 更糟：如果 AMEGO 其實已經作廢了，帳上會有一張「有效」的廢票
//
// ⇒ 正確的落點是「我不知道」，交給人。document 留在 VOIDING 是**誠實**的，
// 前提是那個狀態要被看得見——那一半屬於 #2959 ⑧ 的指標缺口，不在本函式。
func (u *Voiding) scheduleVoidRecheck(ctx context.Context, op domain.Operation, now time.Time) error {
	elapsed := time.Duration(0)
	if op.UnknownSince != nil {
		elapsed = now.Sub(*op.UnknownSince)
	}
	if elapsed > recheckGiveUpAfter {
		lastErr := "invoice: void confirmation still unknown after give-up window"
		// 走 MarkRecheckUnresolved（⑧ 的落點）而不是自建一句：J17 的目標狀態、
		// unknown_since 保留與退避基準三件事都在那一句裡，各寫一份必然漂。
		ok, err := u.ops.MarkRecheckUnresolved(ctx, domain.MarkRecheckResult{
			ID:            op.ID,
			NextStatus:    codemeta.InvoiceOperationStatusFailedUnknown.Value,
			NextAttemptAt: now.Add(recheckBackoff(elapsed)),
			LastError:     &lastErr,
			Now:           now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter FAILED_UNKNOWN (rows=0)", op.ID)
		}
		u.log.Error("invoice.void.confirm_gave_up",
			zap.Int64("operation_id", op.ID), zap.Int64("document_id", op.DocumentID),
			zap.Duration("elapsed", elapsed))
		if err := appendStatusHistory(ctx, u.audit, subjectOperation, op.ID,
			op.Status, codemeta.InvoiceOperationStatusFailedUnknown.Value, "", "", now); err != nil {
			return err
		}
		// G15：operation 放棄了，adjustment 必須跟著離開 WAITING_CONFIRMATION。
		//
		// 🔴 少了這一句，adjustment 永遠停在 WAITING_CONFIRMATION —— 而
		// invoice_adjustment_stuck_rows 對該狀態刻意不計數（它是合法的中間狀態），
		// 於是「作廢結果永遠不明」這件事在指標上完全不存在。折讓軸的對應段落在
		// Allowance.ConfirmAllowance 的 G15，這條軸原本缺了。
		return u.advanceAdjustment(ctx, op, adjustmentFailedUnknown, "", now)
	}
	// 🔴 rows 不得丟給 `_`：本句的 allowed-from 是
	// ('SENDING','PROVIDER_STATUS_RECHECK')，rows=0 代表這一列既不在送出中、
	// 也不在重查中 —— 那時「繼續等」根本沒寫進去，而退避與 unknown_since
	// 都靠這一句。無聲吞掉的後果是那一列被 60s 租約推著無限重查
	// （dev 第 3 輪實測）。
	ok, err := u.ops.MarkWaitingConfirmation(ctx, domain.MarkOperationUnknown{
		ID:            op.ID,
		NextAttemptAt: now.Add(recheckBackoff(elapsed)),
		Now:           now,
	})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf(
			"invoice: operation %d could not re-enter WAITING_CONFIRMATION (rows=0, snapshot status=%s)",
			op.ID, op.Status)
	}
	return nil
}

// adjustmentStep 是作廢軸要把 adjustment 推到哪一格。
type adjustmentStep int

const (
	adjustmentSending adjustmentStep = iota
	adjustmentCompleted
	adjustmentFailedPermanent
	adjustmentWaitingConfirmation
	adjustmentFailedUnknown
)

// advanceAdjustment 把 operation 服務的那一筆 adjustment 推進一格（G08／G09／G10／G12）。
//
// 🔴 **rows=0 一律不當錯誤**，只記一行 log。理由：
//   - 這一段是 #3073 才補上的，dev／prod 上都存在「operation 已經走完、adjustment
//     還停在 PENDING」的歷史列。它們的 operation 重投時會走到這裡，而 allowed-from
//     對不上 ⇒ rows=0。回 error 會讓一批早就處理完的作廢重新變成失敗。
//   - 作廢的權威是 invoice_documents.status（VOIDED），不是 adjustment。adjustment
//     在這條軸上是稽核用的側寫，讓它落後不會讓任何錢或憑證出錯。
//
// ⚠️ 反過來說：**折讓軸不能這樣寫**。那裡的 adjustment 是主體（allowance_number、
// 三個金額欄、累計落帳都掛在它身上），rows=0 代表狀態機破了，必須 rollback。
// 兩條軸的處置不同不是不一致，是因為同一張表在兩條軸上的角色不同。
func (u *Voiding) advanceAdjustment(
	ctx context.Context, op domain.Operation, step adjustmentStep, reasonCode string, now time.Time,
) error {
	if op.AdjustmentID == nil {
		// VOID operation 依 chk 必須帶 adjustment_id；沒有代表是更早的資料。
		return nil
	}
	id := *op.AdjustmentID

	var (
		ok  bool
		err error
		to  string
	)
	switch step {
	case adjustmentSending:
		ok, err = u.adjustments.MarkSending(ctx, id, now)
		to = codemeta.InvoiceAdjustmentStatusSending.Value
	case adjustmentCompleted:
		ok, err = u.adjustments.MarkCompleted(ctx, domain.MarkAdjustmentCompleted{ID: id, Now: now})
		to = codemeta.InvoiceAdjustmentStatusCompleted.Value
	case adjustmentFailedPermanent:
		ok, err = u.adjustments.MarkFailedPermanent(ctx, id, reasonCode, now)
		to = codemeta.InvoiceAdjustmentStatusFailedPermanent.Value
	case adjustmentWaitingConfirmation:
		ok, err = u.adjustments.MarkWaitingConfirmation(ctx, id, now)
		to = codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value
	case adjustmentFailedUnknown:
		ok, err = u.adjustments.MarkFailedUnknown(ctx, id, now)
		to = codemeta.InvoiceAdjustmentStatusFailedUnknown.Value
	default:
		return fmt.Errorf("invoice: unknown adjustment step %d", step)
	}
	if err != nil {
		return err
	}
	if !ok {
		u.log.Info("invoice.void.adjustment_step_no_rows",
			zap.Int64("adjustment_id", id), zap.Int64("operation_id", op.ID),
			zap.String("want_status", to))
		return nil
	}
	return appendStatusHistory(ctx, u.audit, subjectAdjustment, id, "", to, reasonCode, "", now)
}
