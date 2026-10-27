package usecase

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)

// 本檔是「重新寄送發票通知」的三個入口（§6.1.10 端點 32 賣家／§6.1.14 端點 49 ADM／
// 端點 71 買家）。
//
// 四條定版：
//
//	🔴 **三個入口共用同一個計數器**（key ＝ `document_id`，不是 `document_id × actor`）。
//	   否則 ADM 幫忙點一次、賣家再點一次，買家會在十分鐘內收到兩封。
//
//	🔴 **窗口守衛是 SQL 的 WHERE**，rows=0 回 9352（429）。應用層先讀再判擋不住
//	   「賣家與 ADM 同時按下」——那正是這個計數器要防的情境。
//
//	🔴 **歸屬鍵三者不同，而且都在 SQL 裡**：賣家走 GetForSeller(store_id)、
//	   買家走 GetForBuyerByID(buyer_id)、ADM 無歸屬述詞。越權與查無同回 9301
//	   （Tier A，§8.1.4）——分開回會洩漏「這個 document 存在」。
//
//	🔴 **重寄沿用 P1／P6 的 routing key，但 `notification_ref_id` 必須換新**
//	   （`INVRS:{documentID}:{seq}`）。notification-service 的冪等鍵含 reference_id ⇒
//	   沿用開立當下那個 `INV:{id}` 的第二封信會被**靜默吞掉**，而按下去的人看到的是成功。
//	   seq 取自 DB 遞增後的 `notification_resend_count`，不是 Go 端自己數的。

// resendWindow 是重寄冷卻窗口（§6.1.10 定版：同一 document 每 10 分鐘 1 次）。
const resendWindow = 10 * time.Minute

// NotificationResend 是三個重寄入口的共用 usecase。
//
// 它獨立成一個 usecase 而不是掛進 Query：它需要 TxManager 與 outbox（窗口 claim 與
// 事件必須同進同退——claim 成功但事件沒發，那十分鐘內誰都再也按不動，而買家什麼都沒收到）。
type NotificationResend struct {
	tx      database.TxManager
	docs    domain.DocumentRepository
	issuers domain.IssuerRepository
	outbox  domain.OutboxRepository

	now func() time.Time
	log *zap.Logger
}

// NewNotificationResend 組出重寄 usecase。
func NewNotificationResend(
	tx database.TxManager,
	docs domain.DocumentRepository,
	issuers domain.IssuerRepository,
	outbox domain.OutboxRepository,
	log *zap.Logger,
) *NotificationResend {
	return &NotificationResend{
		tx: tx, docs: docs, issuers: issuers, outbox: outbox,
		now: func() time.Time { return time.Now().UTC() },
		log: orNop(log),
	}
}

// WithClock 注入時間來源（測試用）。
func (u *NotificationResend) WithClock(now func() time.Time) *NotificationResend {
	if now != nil {
		u.now = now
	}
	return u
}

// ResendResult 是三支端點的共通回應（proto ResendInvoiceNotificationData）。
type ResendResult struct {
	Resent   bool
	SentAt   time.Time
	ToMasked string
}

// ResendForSeller 是端點 32（S07／S10 的「重新寄送」）。
func (u *NotificationResend) ResendForSeller(ctx context.Context, storeID, documentID int64) (ResendResult, error) {
	return u.resend(ctx, func(ctx context.Context) (domain.Document, error) {
		item, err := u.docs.GetForSeller(ctx, documentID, storeID)
		if err != nil {
			return domain.Document{}, err // 越權與查無同回 9301
		}
		return item.Document, nil
	})
}

// ResendForBuyer 是端點 71（買家發票中心的「重新寄送通知」）。
func (u *NotificationResend) ResendForBuyer(ctx context.Context, buyerID, documentID int64) (ResendResult, error) {
	return u.resend(ctx, func(ctx context.Context) (domain.Document, error) {
		return u.docs.GetForBuyerByID(ctx, documentID, buyerID)
	})
}

// ResendForAdmin 是端點 49（A06／A13 的「重新寄送」）。
//
// ADM 沒有歸屬述詞——它是全平台視角。查無仍回 9301（與另外兩支同碼，前端只有一套處理）。
func (u *NotificationResend) ResendForAdmin(ctx context.Context, documentID int64) (ResendResult, error) {
	return u.resend(ctx, func(ctx context.Context) (domain.Document, error) {
		return u.docs.GetByID(ctx, documentID)
	})
}

// resend 是三個入口共用的那一段：載入（帶各自的歸屬述詞）→ claim 窗口 → 發事件。
func (u *NotificationResend) resend(
	ctx context.Context, load func(context.Context) (domain.Document, error),
) (ResendResult, error) {
	var out ResendResult
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		doc, err := load(ctx)
		if err != nil {
			return err
		}
		now := u.now()

		// 🔴 沒有東西可寄的文件不得進入窗口：NOT_APPLICABLE／PENDING 這類還沒有發票
		//    號碼的文件，重寄出去的是一封沒有內容的信。**先擋再 claim**，否則按一次
		//    就把十分鐘的額度用掉了，而什麼都沒發生。
		if !resendableStatus(doc.Status) {
			return pkgerrors.NewBusiness(bizcode.InvoiceDocumentNotFound,
				"這份文件尚未開立完成，沒有可重新寄送的通知")
		}

		seq, claimed, err := u.docs.ClaimNotificationResend(ctx, doc.ID, now.Add(-resendWindow), now)
		if err != nil {
			return err
		}
		if !claimed {
			// 還在 10 分鐘窗口內。429 的 data 一定是空的（§6.1.10）——倒數靠端點 31 的
			// nextResendAvailableAt 在進畫面時就給，不是在 429 當下補。
			return pkgerrors.NewBusiness(bizcode.InvoiceNotificationResendTooFrequent,
				bizcode.Message(bizcode.InvoiceNotificationResendTooFrequent))
		}

		if err := u.emitResend(ctx, doc, seq, now); err != nil {
			return err
		}
		masked, err := u.maskedRecipient(ctx, doc)
		if err != nil {
			return err
		}
		out = ResendResult{Resent: true, SentAt: now, ToMasked: masked}
		return nil
	})
	if err != nil {
		return ResendResult{}, err
	}
	return out, nil
}

// maskedRecipient 回收件人 email 的遮罩形式（`toMasked`）。
//
// 🔴 明文只以參數形式進入 callback 的區域變數（WithDecryptedRecipient 沒有回傳值）
// ⇒ 不可能被誤存進 struct、log 或 response（§8.1.1(h) 的同一條規約）。
// 沒有 email 的文件（載具發票／收據）回空字串——那是誠實的「沒有寄件對象」，
// 不是遮罩失敗。
func (u *NotificationResend) maskedRecipient(ctx context.Context, doc domain.Document) (string, error) {
	if doc.RecipientEmailCiphertext == nil {
		return "", nil
	}
	masked := ""
	if err := u.docs.WithDecryptedRecipient(ctx, doc.ID, func(s domain.RecipientSecrets) error {
		masked = MaskEmail(s.Email)
		return nil
	}); err != nil {
		return "", err
	}
	return masked, nil
}

// resendableStatus 回報這個狀態的文件有沒有東西可寄。
//
// 三個終態各自都有可寄的內容：ISSUED／RESOLVED_EXTERNALLY（賣家自行開立的號碼）
// 與 VOIDED（作廢通知本身也是買家要收到的）。其餘（PENDING／ISSUING／ISSUE_FAILED／
// AWAITING_MANUAL／NOT_APPLICABLE／CANCELLED_BEFORE_ISSUE）都還沒有內容。
func resendableStatus(status string) bool {
	switch status {
	case codemeta.InvoiceDocumentStatusIssued.Value,
		codemeta.InvoiceDocumentStatusResolvedExternally.Value,
		codemeta.InvoiceDocumentStatusVoided.Value:
		return true
	}
	return false
}

// emitResend 把重寄事件寫進 outbox（與 claim 同交易）。
//
// 🔴 routing key 沿用 P1／P6（`invoice.issued`／`invoice.receipt.issued`）：
// `hawkr.invoice` 目前只有 `notification.invoice` 這一個綁定（`#`），
// 所以重放這個 key 只會走到通知這一條路，不會讓別的 consumer 重跑一次業務。
// 新增一個專屬 routing key 反而要連動 36 號 catalog、CodeMeta EventType 與
// notification 的模板對照——那是跨服務的一整波，不是本波的 schema 缺口。
//
// 🔴 `notification_ref_id` 必須換新（見檔頭紅字）：`INVRS:{id}:{seq}`。
func (u *NotificationResend) emitResend(ctx context.Context, doc domain.Document, seq int32, now time.Time) error {
	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return err
	}
	invoiceDate := ""
	if doc.InvoiceDateTaipei != nil {
		invoiceDate = doc.InvoiceDateTaipei.Format(time.DateOnly)
	}
	payload, err := json.Marshal(issuedEvent{
		NotificationRefID: "INVRS:" + strconv.FormatInt(doc.ID, 10) + ":" + strconv.FormatInt(int64(seq), 10),
		InvoiceDocumentID: strconv.FormatInt(doc.ID, 10),
		OrderNo:           derefString(doc.SourceRef),
		BuyerID:           eventwire.BuyerIDOfDocument(doc),
		StoreID:           eventwire.StoreIDOf(issuer),
		DocumentKind:      doc.DocumentKind,
		InvoiceNo:         resendInvoiceNo(doc),
		InvoiceDate:       invoiceDate,
		TotalAmount:       strconv.FormatInt(doc.TotalAmount, 10),
		HasCarrier:        doc.CarrierType != nil && *doc.CarrierType != "",
		HasBuyerTaxID:     doc.RecipientTaxID != domain.B2CBuyerIdentifier,
		OccurredAt:        timeutil.FormatISO8601(now),
	})
	if err != nil {
		return err
	}
	_, err = u.outbox.Insert(ctx, domain.NewOutboxEvent{
		AggregateType: outboxAggregateDocument,
		AggregateID:   doc.ID,
		EventType:     string(issuedEventType(doc.DocumentKind)),
		Exchange:      pkgmq.ExchangeInvoice,
		RoutingKey:    issuedRoutingKey(doc.DocumentKind),
		Payload:       payload,
		NextAttemptAt: now,
		Now:           now,
	})
	return err
}

// resendInvoiceNo 取這份文件對外的號碼。
//
// 🔴 三個欄位**永不互相 fallback 到錯的語意**：平台開的走 provider_invoice_no、
// 賣家自行開立的走 external_invoice_no、收據走 receipt_no。取錯的後果不是欄位錯，
// 是**平台冒充開立人**（§6.1.14 對 P12 的同一條紅字）。
func resendInvoiceNo(doc domain.Document) string {
	if doc.DocumentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return derefString(doc.ReceiptNo)
	}
	if doc.Status == codemeta.InvoiceDocumentStatusResolvedExternally.Value {
		return derefString(doc.ExternalInvoiceNo)
	}
	return derefString(doc.ProviderInvoiceNo)
}

// NextResendAvailableAt 回「下一次可以重寄的時點」；已經可以重寄時回 nil。
//
// 端點 31 靠它在**進畫面時**就把按鈕停用並顯示倒數：端點 32 的 429 回應帶不出 data
// （§6.1.10），所以前端沒有第二個來源。
//
// ⚠️ 列表端點 30 刻意**不帶此欄**（每列都要算一次＝N+1）。
func NextResendAvailableAt(doc domain.Document, now time.Time) *time.Time {
	if doc.NotificationResentAt == nil {
		return nil
	}
	next := doc.NotificationResentAt.Add(resendWindow)
	if !next.After(now) {
		return nil
	}
	return &next
}
