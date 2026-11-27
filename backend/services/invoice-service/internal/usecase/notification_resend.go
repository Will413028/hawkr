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
			return domain.Document{}, err
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

		if !resendableStatus(doc.Status) {
			return pkgerrors.NewBusiness(bizcode.InvoiceDocumentNotFound,
				"這份文件尚未開立完成，沒有可重新寄送的通知")
		}

		seq, claimed, err := u.docs.ClaimNotificationResend(ctx, doc.ID, now.Add(-resendWindow), now)
		if err != nil {
			return err
		}
		if !claimed {
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

func resendableStatus(status string) bool {
	switch status {
	case codemeta.InvoiceDocumentStatusIssued.Value,
		codemeta.InvoiceDocumentStatusResolvedExternally.Value,
		codemeta.InvoiceDocumentStatusVoided.Value:
		return true
	}
	return false
}

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
