package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)


// ReceiptRenderer 把一份收據資料畫成 PDF bytes。
//
// 介面放在 usecase 而不是 domain：domain 那一側是 `NewReceiptRenderer` 這個具體實作
// （它持有字型），usecase 只需要「給我 bytes」這一個動作。
type ReceiptRenderer interface {
	Render(d domain.ReceiptData) ([]byte, error)
}

// ReceiptUploader 把產出的 PDF 落到自有儲存的機敏區，回 objectKey。
//
// 與 PDFUploader 同一個方法簽章、同一個實作（media client 的 UploadSecure）。
// 分成兩個介面名是因為兩條線的**理由不同**：INV.pdf 是「AMEGO 只留 180 天、法規要 5 年」，
// 收據是「這份文件除了我方沒有第二個持有者」。
type ReceiptUploader interface {
	UploadSecure(ctx context.Context, data []byte, filename, contentType, refID string) (string, error)
}

const receiptContentType = "application/pdf"

var errReceiptRendererMissing = errors.New("invoice: receipt renderer is not configured")

func (u *Issuance) settleReceipt(ctx context.Context, doc domain.Document, op domain.Operation, now time.Time) error {
	if u.receipts == nil || u.receiptStore == nil {
		return errReceiptRendererMissing
	}

	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return err
	}
	profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
	if err != nil {
		return err
	}

	if profile.ReceiptAuthorizedAt == nil || profile.ReceiptConsentVersion == nil {
		return u.settleAwaitingManual(ctx, doc, op,
			codemeta.InvoiceManualReasonReceiptAuthMissing.Value, now)
	}

	lines, err := u.docs.ListLines(ctx, doc.ID)
	if err != nil {
		return err
	}

	data, err := buildReceiptData(doc, profile, lines)
	if err != nil {
		u.logf(ctx).Warn("invoice.receipt.data_incomplete",
			zap.Int64("document_id", doc.ID), zap.Error(err))
		return u.settleAwaitingManual(ctx, doc, op,
			codemeta.InvoiceManualReasonAmountMismatch.Value, now)
	}

	pdf, err := u.receipts.Render(data)
	if err != nil {
		return fmt.Errorf("invoice: render receipt %s: %w", data.ReceiptNo, err)
	}

	objectKey, err := u.receiptStore.UploadSecure(ctx, pdf,
		data.ReceiptNo+".pdf", receiptContentType, strconv.FormatInt(doc.ID, 10))
	if err != nil {
		return fmt.Errorf("invoice: upload receipt %s: %w", data.ReceiptNo, err)
	}

	payload, err := u.issuedEventPayload(issuer, doc, &amego.IssueResult{}, nil, now)
	if err != nil {
		return err
	}

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		if _, err := u.outbox.Insert(ctx, domain.NewOutboxEvent{
			AggregateType: outboxAggregateD,
			AggregateID:   doc.ID,
			EventType:     string(issuedEventType(doc.DocumentKind)),
			Exchange:      pkgmq.ExchangeInvoice,
			RoutingKey:    issuedRoutingKey(doc.DocumentKind),
			Payload:       payload,
			NextAttemptAt: now,
			Now:           now,
		}); err != nil {
			return err
		}

		ok, err := u.docs.MarkReceiptIssued(ctx, domain.MarkReceiptIssued{
			ID:                   doc.ID,
			PdfObjectKey:         objectKey,
			AuthorizationVersion: *profile.ReceiptConsentVersion,
			IssuedAt:             now,
			Now:                  now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: receipt document %d could not enter ISSUED (rows=0, status=%s)", doc.ID, doc.Status)
		}
		if _, err := u.ops.MarkDone(ctx, op.ID, now); err != nil {
			return err
		}
		if err := u.appendHistory(ctx, subjectDocument, doc.ID,
			codemeta.InvoiceDocumentStatusIssuing.Value,
			codemeta.InvoiceDocumentStatusIssued.Value, "", "", now); err != nil {
			return err
		}
		return u.appendHistory(ctx, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusSucceeded.Value, "", "", now)
	})
}

func (u *Issuance) settleAwaitingManual(
	ctx context.Context, doc domain.Document, op domain.Operation, reasonCode string, now time.Time,
) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.docs.MarkPendingRetry(ctx, doc.ID, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: receipt document %d could not leave ISSUING (status=%s)", doc.ID, doc.Status)
		}
		ok, err = u.docs.MarkAwaitingManual(ctx, doc.ID, reasonCode, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: receipt document %d could not enter AWAITING_MANUAL", doc.ID)
		}
		if _, err := u.ops.MarkFailed(ctx, op.ID, reasonCode, now); err != nil {
			return err
		}
		if err := u.appendHistory(ctx, subjectDocument, doc.ID,
			codemeta.InvoiceDocumentStatusIssuing.Value,
			codemeta.InvoiceDocumentStatusAwaitingManual.Value, reasonCode, "", now); err != nil {
			return err
		}
		issuer, ierr := u.issuers.GetByID(ctx, doc.IssuerID)
		if ierr != nil {
			return ierr
		}
		if err := u.emitReviewRequired(ctx, issuer, doc, reasonCode, now); err != nil {
			return err
		}
		return u.appendHistory(ctx, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusFailedPermanent.Value, reasonCode, "", now)
	})
}

func buildReceiptData(
	doc domain.Document, profile domain.SellerTaxProfile, lines []domain.DocumentLine,
) (domain.ReceiptData, error) {
	if doc.ReceiptNo == nil || *doc.ReceiptNo == "" {
		return domain.ReceiptData{}, errors.New("receipt_no is empty")
	}
	if profile.LegalName == nil || *profile.LegalName == "" {
		return domain.ReceiptData{}, errors.New("seller legal_name is empty")
	}
	if len(lines) == 0 {
		return domain.ReceiptData{}, errors.New("no document lines")
	}

	issuedAt := time.Now().UTC()
	if doc.InvoiceDateTaipei != nil {
		issuedAt = *doc.InvoiceDateTaipei
	}

	out := domain.ReceiptData{
		ReceiptNo: *doc.ReceiptNo,
		OrderNo:   derefString(doc.SourceRef),
		IssuedAt:  issuedAt,
		Seller: domain.ReceiptSeller{
			LegalName: *profile.LegalName,
			TaxID:   derefString(profile.TaxID),
			Address: derefString(profile.BusinessAddress),
		},
		TotalAmount: doc.TotalAmount,
	}
	for _, l := range lines {
		out.Lines = append(out.Lines, domain.ReceiptLine{
			Name:      l.Description,
			Quantity:  l.Quantity,
			UnitPrice: l.UnitPrice,
			Amount:    l.NetAmount,
		})
	}
	return out, nil
}
