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

// ---------------------------------------------------------------------------
// 平台代製收據的產製（PLATFORM_RECEIPT，#3026／§7.1.2 S-6／D17・ADR-32）
//
// 設計書 S-6 逐字：「`INV.op` **不打 AMEGO**，改走本地 PDF 產生」。狀態機沿用同一條
// （§7.1.2 M1：`PENDING → ISSUING → ISSUED`），事件走 P6 `invoice.receipt.issued`。
//
// 🔴 §5 L11「`PLATFORM_RECEIPT` 不可混入同一條發票狀態機」指的是**語意**不是狀態值：
// 它不是「延後開票」也不是「開票失敗」。所以底下每一條失敗路徑都**不得**落
// `ISSUE_FAILED`——那正是 #3022 的病（依法不該開發票的賣家，每一筆訂單在 ADM 產生
// 一筆假的「開立失敗」）。收據的失敗只有兩種歸宿：可重試（留在 operation 的退避曲線上）
// 或 `AWAITING_MANUAL`（有人要去處理）。
// ---------------------------------------------------------------------------

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

// receiptContentType 是收據的 MIME。**不是 application/octet-stream**：
// media 的機敏上傳會把密文寫進儲存，但 mime_type 欄位記的是**明文**的型別
// （#3024 定版），下載通道靠它決定 Content-Type。
const receiptContentType = "application/pdf"

// errReceiptRendererMissing 是「字型／renderer 沒有裝起來」。
//
// 它不是業務錯誤而是部署錯誤 ⇒ 走可重試路徑（下一次部署修好之後自己會過），
// 而且 DI 那一層已經 fail-closed（字型讀不起來 pod 起不來），所以正常情況取不到。
var errReceiptRendererMissing = errors.New("invoice: receipt renderer is not configured")

// settleReceipt 產製一份平台代製收據並落 ISSUED。
//
// 呼叫端（Submit）必須已經 ClaimForSubmit 成功 ⇒ document 在 ISSUING、operation 在 SENDING。
//
// 順序是刻意的：**渲染與上傳都在交易外**。把一次 media 上傳放進交易，會讓交易持有時間
// 綁在外部依賴上（同 settleIssued 的 issuer 讀取那一段），而收據的 PDF 是幾十 KB、
// 上傳逾時完全可能發生。
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

	// 🔴 第二道授權檢查（第一道在 §7.1.3 決策樹）。兩道都要，因為**建立與產製之間隔著
	// 一段時間**：賣家可以在這中間按下 S05 的「取消授權」。少了這一道，取消授權之後
	// 佇列裡的每一筆仍會以他的名義產出收據，而他剛剛才明確說不要。
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
		// 資料湊不齊（缺法定名稱、缺收據編號、沒有任何品項行）＝ 重試一百次也一樣。
		// 落 AWAITING_MANUAL 讓 ADM 看得到，不落 ISSUE_FAILED。
		u.logf(ctx).Warn("invoice.receipt.data_incomplete",
			zap.Int64("document_id", doc.ID), zap.Error(err))
		return u.settleAwaitingManual(ctx, doc, op,
			codemeta.InvoiceManualReasonAmountMismatch.Value, now)
	}

	pdf, err := u.receipts.Render(data)
	if err != nil {
		return fmt.Errorf("invoice: render receipt %s: %w", data.ReceiptNo, err)
	}

	// filename 用收據編號而不是 document id：買家下載下來看到的就是這個名字，
	// 而收據編號是這份文件對外的唯一識別。
	objectKey, err := u.receiptStore.UploadSecure(ctx, pdf,
		data.ReceiptNo+".pdf", receiptContentType, strconv.FormatInt(doc.ID, 10))
	if err != nil {
		return fmt.Errorf("invoice: upload receipt %s: %w", data.ReceiptNo, err)
	}

	// 事件 payload 沿用 P1／P6 共用的那一支。res 是空的 IssueResult：收據**沒有發票號碼**，
	// 而 invoice_no 留空正是消費端分辨兩者的方式之一（routing key 是另一個，且更可靠）。
	payload, err := u.issuedEventPayload(issuer, doc, &amego.IssueResult{}, nil, now)
	if err != nil {
		return err
	}

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		// outbox 排最前面，理由與 settleIssued 逐字相同：那是「事件寫入有沒有跟隨業務交易」
		// 唯一可被機械驗證的形狀（後面任一句守衛失敗就會把它一起帶走）。
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

// settleAwaitingManual 把一筆開立作業轉人工：document → AWAITING_MANUAL、
// operation → FAILED_PERMANENT、留痕、發 P7 讓 ADM 工作台看得到。
//
// 原名 settleReceiptAwaitingManual，2026-09-08 改名並複用於發票線（載具驗證未通過）：
// 函式體從來沒有任何收據專屬邏輯，而那個名字會讓下一個人在發票線上重寫一份。
//
// 🔴 document 的邊是 ISSUING → **PENDING** → AWAITING_MANUAL，兩句：
// `MarkAwaitingManual` 的 allowed-from 是 `('PENDING','ISSUE_FAILED')`，而此刻它在
// ISSUING。走 ISSUE_FAILED 過渡是不行的——那會在 invoice_status_history 留下一筆
// 「開立失敗」，正是 L11 禁止的那件事，而歷程是賣家 S06 讀得到的。
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
		// operation 落 FAILED_PERMANENT：這一筆作業確實不會再自己成功。
		// document 那一側走的是 AWAITING_MANUAL（不是 ISSUE_FAILED）⇒ 對外顯示不含
		// 「開立失敗」語意，而 ADM 的 NEEDS_MANUAL 工作台看得到它。
		if _, err := u.ops.MarkFailed(ctx, op.ID, reasonCode, now); err != nil {
			return err
		}
		if err := u.appendHistory(ctx, subjectDocument, doc.ID,
			codemeta.InvoiceDocumentStatusIssuing.Value,
			codemeta.InvoiceDocumentStatusAwaitingManual.Value, reasonCode, "", now); err != nil {
			return err
		}
		// P7：與建單那條路同一則（#3055 的橫向稽查）。收據線也會走到人工出口，
		// 而賣家在這條路上一樣需要知道 —— 少了它，代製收據卡住是完全無聲的。
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

// buildReceiptData 把 document ＋ 賣家稅務資格 ＋ 明細行組成收據版面資料。
//
// 🔴 出據人是**賣家**不是平台（Q-B7 逐字：「收據以賣家為出據人、不以平台名義開立」）。
// 所以法定名稱缺席時**不得**用店名或平台名稱頂替——那會產出一份出據人是錯的對外文件。
//
// 🔴 買受人恆為「消費者」：本服務不持有買家真名（收據上不放任何買家個資，
// 而收據會落 media secure/ 保存 5 年）。
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
			// 🔴 這一欄在收據線上**恆為空字串**，而那是 DB 保證的不是巧合：
			// `chk_stp_exempt_no_taxid` 要求 review_status='APPROVED_INVOICE_EXEMPT'
			// 的列 tax_id 必為 NULL（2026-09-02 實測：帶著統編改成 EXEMPT 當場 23514）。
			// 留著它而不是寫死空字串，是因為「出據人有沒有統編」是版面的問題不是這裡的，
			// 而 ReceiptRenderer 對空值的處理是**整行不印**（不是印一個空欄位）。
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
