package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// DocumentRepository 是 invoice_documents ＋ invoice_document_lines 的 thin adapter，
// 並承載兩組租約：INV.pdf（pdf_fetch_next_at）與 INV.rc 第 4 組（upload_recheck_next_at）。
//
// 第 4 組與 invoice_operations 的第 2 組同屬 INV.rc 但落在**不同的表**，所以同一支 worker
// 持有 60s / 120s 兩種租約長度是安全的：沒有任何一列會同時被兩組看到。
type DocumentRepository struct {
	baseRepo
	ring *KeyRing
}

// NewDocumentRepository 建立 repository。
//
// 收 KeyRing 的理由與 ProviderAccountRepository 相同：收件人 email／電話／載具是密文欄，
// 加解密必須收斂在本層。差別是 invoice_documents **沒有** key_version 欄 ⇒ 解密走
// current→prev 的順序嘗試（見 mapper.go 的 decryptOptional）。
func NewDocumentRepository(pool *pgxpool.Pool, ring *KeyRing) *DocumentRepository {
	return &DocumentRepository{baseRepo: baseRepo{pool: pool}, ring: ring}
}

var _ domain.DocumentRepository = (*DocumentRepository)(nil)

// ClaimNotificationResend 是端點 32／49／71 的重寄窗口（§6.1.10）。
//
// 🔴 claimed=false ＝ **還在 10 分鐘窗口內**（不是錯誤、也不是查無）⇒ 呼叫端回 9352。
// 這句是 :one ＋ RETURNING，rows=0 時 pgx 回 ErrNoRows ——用它分辨「窗口未開」。
func (r *DocumentRepository) ClaimNotificationResend(
	ctx context.Context, id int64, windowStart, now time.Time,
) (int32, bool, error) {
	seq, err := r.q(ctx).ClaimInvoiceNotificationResend(ctx, gen.ClaimInvoiceNotificationResendParams{
		Now:         tp(now),
		ID:          id,
		WindowStart: tp(windowStart),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return seq, true, nil
}

// GetByID 查單張憑證。
func (r *DocumentRepository) GetByID(ctx context.Context, id int64) (domain.Document, error) {
	row, err := r.q(ctx).GetInvoiceDocumentByID(ctx, id)
	if err != nil {
		return domain.Document{}, notFound(err, bizcode.InvoiceDocumentNotFound)
	}
	return toDocument(row)
}

// GetBySourceRef 依來源單號（訂單號等）查。
func (r *DocumentRepository) GetBySourceRef(ctx context.Context, sourceRef string) (domain.Document, error) {
	row, err := r.q(ctx).GetInvoiceDocumentBySourceRef(ctx, sp(sourceRef))
	if err != nil {
		return domain.Document{}, notFound(err, bizcode.InvoiceDocumentNotFound)
	}
	return toDocument(row)
}

// GetForBuyer 是買家面的授權讀取：授權述詞（recipient_ref_id 必須是這位買家）寫在 SQL 裡。
//
// 越權與不存在**刻意不可區分**，兩者都回 9301（§8.1.4 Tier A）：能區分就等於提供一個
// 「這張發票存不存在」的預言機。
func (r *DocumentRepository) GetForBuyer(ctx context.Context, sourceRef string, buyerID int64) (domain.Document, error) {
	row, err := r.q(ctx).GetInvoiceDocumentForBuyer(ctx, gen.GetInvoiceDocumentForBuyerParams{
		SourceRef:      sp(sourceRef),
		RecipientRefID: &buyerID,
	})
	if err != nil {
		return domain.Document{}, notFound(err, bizcode.InvoiceDocumentNotFound)
	}
	return toDocument(row)
}

// ListLines 回單張憑證的明細行（依 line_seq）。
func (r *DocumentRepository) ListLines(ctx context.Context, documentID int64) ([]domain.DocumentLine, error) {
	rows, err := r.q(ctx).ListInvoiceDocumentLines(ctx, documentID)
	if err != nil {
		return nil, err
	}
	return toDocumentLines(rows)
}

// FetchApproachingStatutoryDeadlines 給法定期限告警用（唯讀，不搶佔）。
func (r *DocumentRepository) FetchApproachingStatutoryDeadlines(ctx context.Context, threshold time.Time, limit int32) ([]domain.Document, error) {
	rows, err := r.q(ctx).FetchApproachingStatutoryDeadlines(ctx, gen.FetchApproachingStatutoryDeadlinesParams{
		Threshold: tp(threshold),
		RowLimit:  limit,
	})
	if err != nil {
		return nil, err
	}
	return toDocuments(rows)
}

// FetchDuePdfFetches 是 INV.pdf 的候選。
func (r *DocumentRepository) FetchDuePdfFetches(ctx context.Context, now time.Time, limit int32) ([]domain.Document, error) {
	rows, err := r.q(ctx).FetchDuePdfFetches(ctx, gen.FetchDuePdfFetchesParams{
		Now:      tp(now),
		RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	return toDocuments(rows)
}

// ClaimPdfFetch 搶佔一列給 INV.pdf（租約 120s，下界 50.2s：invoice_file 10s ＋ 下載 20s
// ＋ MED 上傳 20s ＋ DB 0.2s）。
func (r *DocumentRepository) ClaimPdfFetch(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error) {
	rows, err := r.q(ctx).ClaimPdfFetch(ctx, gen.ClaimPdfFetchParams{
		LeaseUntil:      tp(leaseUntil),
		Now:             ts(now),
		ID:              id,
		ClaimableBefore: tp(now),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// MarkPdfFetchDone 落 objectKey 並把 pdf_fetch_next_at 清成 NULL（退出候選集合）。
//
// 存的是 objectKey 不是完整 URL（§25 / 49 號設計書）：URL 組裝的唯一入口是
// pkg/mediaurl.BuildURL，域名唯一來源是 MEDIA_BASE_URL。
func (r *DocumentRepository) MarkPdfFetchDone(ctx context.Context, id int64, objectKey string, now time.Time) error {
	_, err := r.q(ctx).MarkPdfFetchDone(ctx, gen.MarkPdfFetchDoneParams{
		PdfObjectKey: sp(objectKey),
		Now:          ts(now),
		ID:           id,
	})
	return err
}

// MarkPdfFetchStopped 放棄抓檔（次數用盡或憑證已作廢），同樣退出候選集合。
func (r *DocumentRepository) MarkPdfFetchStopped(ctx context.Context, id int64, now time.Time) error {
	_, err := r.q(ctx).MarkPdfFetchStopped(ctx, gen.MarkPdfFetchStoppedParams{
		Now: ts(now),
		ID:  id,
	})
	return err
}

// UpdatePdfFetchAttempt 記一次失敗並重排下次抓檔。
func (r *DocumentRepository) UpdatePdfFetchAttempt(ctx context.Context, id int64, nextAttemptAt, now time.Time) error {
	return r.q(ctx).UpdatePdfFetchAttempt(ctx, gen.UpdatePdfFetchAttemptParams{
		NextAttemptAt: tp(nextAttemptAt),
		Now:           ts(now),
		ID:            id,
	})
}

// FetchDueUploadRechecks 是 INV.rc 第 4 組的候選（provider_upload_status 走正面枚舉）。
func (r *DocumentRepository) FetchDueUploadRechecks(ctx context.Context, now time.Time, limit int32) ([]domain.Document, error) {
	rows, err := r.q(ctx).FetchDueUploadRechecks(ctx, gen.FetchDueUploadRechecksParams{
		Now:      tp(now),
		RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	return toDocuments(rows)
}

// ClaimUploadRecheck 搶佔一列給 INV.rc 第 4 組。
func (r *DocumentRepository) ClaimUploadRecheck(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error) {
	rows, err := r.q(ctx).ClaimUploadRecheck(ctx, gen.ClaimUploadRecheckParams{
		LeaseUntil:      tp(leaseUntil),
		Now:             ts(now),
		ID:              id,
		ClaimableBefore: tp(now),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// RecordUploadRecheckResult 落一輪上傳輪詢的結果。
//
// nextAt 傳 nil ＝ 退出候選集合（已到終態 91／-1 或完成 99）。傳非 nil ＝ 繼續輪詢。
// 判斷「還在飛」用正面枚舉（NULL／1／2／3／31／32），不要用 IS DISTINCT FROM 99——
// 那會讓 91 與 -1 永久滯留（§5.2.3 第 3 組）。
func (r *DocumentRepository) RecordUploadRecheckResult(ctx context.Context, id int64, providerUploadStatus *int16, now time.Time, nextAt *time.Time) error {
	_, err := r.q(ctx).RecordUploadRecheckResult(ctx, gen.RecordUploadRecheckResultParams{
		ProviderUploadStatus: providerUploadStatus,
		Now:                  tp(now),
		UploadRecheckNextAt:  nextAt,
		ID:                   id,
	})
	return err
}

// ---------------------------------------------------------------------------
// 寫入面（§7.1.2 步驟 8–9、步驟 14）
// ---------------------------------------------------------------------------

// Create 建立一列文件。inserted=false 代表這份文件已建過（重複消費 order.paid）。
//
// ON CONFLICT DO NOTHING **不指名 conflict target**：本表有四個 partial unique
// （uq_invdoc_order_identity／_provider_request_no／_provider_invoice_no／_receipt_no），
// 指名其一 ⇒ 另三個衝突時 23505 並讓整個交易 aborted（25P02）⇒ 之後每一句 SQL 都被拒。
//
// rows=0 時本層**不再補一次 SELECT**：呼叫端手上有冪等鍵，要既有列自己去讀
// （GetBySourceRef）。在這裡補讀會讓「衝突在哪個唯一鍵」變成猜測。
func (r *DocumentRepository) Create(ctx context.Context, in domain.NewDocument) (domain.Document, bool, error) {
	emailCT, err := r.ring.encryptOptional(in.RecipientEmail)
	if err != nil {
		return domain.Document{}, false, err
	}
	phoneCT, err := r.ring.encryptOptional(in.RecipientPhone)
	if err != nil {
		return domain.Document{}, false, err
	}
	carrier1CT, err := r.ring.encryptOptional(in.CarrierID1)
	if err != nil {
		return domain.Document{}, false, err
	}
	carrier2CT, err := r.ring.encryptOptional(in.CarrierID2)
	if err != nil {
		return domain.Document{}, false, err
	}

	row, err := r.q(ctx).InsertInvoiceDocument(ctx, gen.InsertInvoiceDocumentParams{
		ID:                          in.ID,
		IssuerID:                    in.IssuerID,
		IssuerNameSnapshot:          in.IssuerNameSnapshot,
		IssuerTaxIDSnapshot:         snp(in.IssuerTaxIDSnapshot),
		SourceType:                  in.SourceType,
		SourceRef:                   snp(in.SourceRef),
		TriggerType:                 in.TriggerType,
		RecipientType:               in.RecipientType,
		RecipientRefID:              in.RecipientRefID,
		RecipientName:               in.RecipientName,
		RecipientTaxID:              in.RecipientTaxID,
		RecipientEmailCiphertext:    emailCT,
		RecipientEmailHash:          r.ring.hashOptional(in.RecipientEmail),
		RecipientPhoneCiphertext:    phoneCT,
		CarrierType:                 snp(in.CarrierType),
		CarrierId1Ciphertext:        carrier1CT,
		CarrierId2Ciphertext:        carrier2CT,
		CarrierValueHash:            r.ring.hashOptional(in.CarrierID1),
		DonationCode:                snp(in.DonationCode),
		DocumentKind:                in.DocumentKind,
		FulfillmentMode:             in.FulfillmentMode,
		Status:                      in.Status,
		NotApplicableReason:         snp(in.NotApplicableReason),
		SnapshotVersion:             in.SnapshotVersion,
		IssueAttemptSeq:             in.IssueAttemptSeq,
		ProviderAccountVersion:      in.ProviderAccountVersion,
		OriginalDocumentID:          in.OriginalDocumentID,
		SalesAmount:                 in.SalesAmount,
		FreeTaxSalesAmount:          in.FreeTaxSalesAmount,
		ZeroTaxSalesAmount:          in.ZeroTaxSalesAmount,
		TaxAmount:                   in.TaxAmount,
		TotalAmount:                 in.TotalAmount,
		TaxType:                     in.TaxType,
		TaxRateBps:                  in.TaxRateBps,
		Provider:                    snp(in.Provider),
		ProviderRequestNo:           snp(in.ProviderRequestNo),
		ReceiptNo:                   snp(in.ReceiptNo),
		ReceiptAuthorizationVersion: snp(in.ReceiptAuthorizationVersion),
		InvoiceDateTaipei:           pgxutil.DateParam(in.InvoiceDateTaipei),
		TaxPeriodCode:               snp(in.TaxPeriodCode),
		IssueNotBefore:              ts(in.IssueNotBefore),
		PaidAt:                      in.PaidAt,
		StatutoryDeadlineAt:         in.StatutoryDeadlineAt,
		SourceSnapshot:              in.SourceSnapshot,
		Now:                         ts(in.Now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Document{}, false, nil
	}
	if err != nil {
		return domain.Document{}, false, err
	}
	doc, err := toDocument(row)
	if err != nil {
		return domain.Document{}, false, err
	}
	return doc, true, nil
}

// InsertLines 一次寫齊明細行（COPY）。空 slice 是合法輸入（零元訂單沒有明細行）。
//
// 用 COPY 而不是逐列 INSERT：一張發票最多 9999 行，逐列會是 9999 次往返。
// CHECK 約束在 COPY 路徑一樣生效 —— 這正是「呼叫端必須先算好」的兜底。
func (r *DocumentRepository) InsertLines(ctx context.Context, documentID int64, lines []domain.NewDocumentLine, now time.Time) error {
	if len(lines) == 0 {
		return nil
	}
	params := make([]gen.BulkInsertInvoiceDocumentLinesParams, 0, len(lines))
	for _, l := range lines {
		params = append(params, gen.BulkInsertInvoiceDocumentLinesParams{
			DocumentID:        documentID,
			LineSeq:           l.LineSeq,
			LineType:          l.LineType,
			OrderItemID:       inp(l.OrderItemID),
			SkuID:             inp(l.SkuID),
			Description:       l.Description,
			Unit:              snp(l.Unit),
			Quantity:          l.Quantity,
			UnitPrice:         l.UnitPrice,
			NetAmount:         l.NetAmount,
			TaxCategory:       l.TaxCategory,
			TaxRateBps:        l.TaxRateBps,
			Remark:            snp(l.Remark),
			AllocatedDiscount: l.AllocatedDiscount,
			AllowanceableBase: l.AllowanceableBase,
			CreatedAt:         ts(now),
		})
	}
	n, err := r.q(ctx).BulkInsertInvoiceDocumentLines(ctx, params)
	if err != nil {
		return err
	}
	if int(n) != len(lines) {
		return fmt.Errorf("invoice: bulk insert document lines wrote %d of %d rows (document_id=%d)", n, len(lines), documentID)
	}
	return nil
}

// LockForCorrection 是 R01 的第一步（同一交易內鎖住該列再判定）。
//
// 🔴 **必須在交易內呼叫**：交易外的 FOR UPDATE 拿到鎖之後立刻釋放，等於沒鎖。
// 本層無法強制那件事（pgx 的 conn 不會告訴我們現在在不在交易裡），所以它寫在
// domain port 的註解與這裡——這是靠紀律的那一種，不是機制擋得住的。
func (r *DocumentRepository) LockForCorrection(ctx context.Context, documentID int64) (domain.Document, error) {
	row, err := r.q(ctx).LockInvoiceDocumentForCorrection(ctx, documentID)
	if err != nil {
		return domain.Document{}, notFound(err, bizcode.InvoiceDocumentNotFound)
	}
	return toDocument(row)
}

// DeleteLines 清空未送出文件的明細行（R03 的「重建 lines」前半）。
//
// 述詞限定 PENDING／AWAITING_MANUAL 寫在 SQL 內——放在呼叫端會讓「誰保證不會刪到
// 已開立的發票」變成靠自覺。
func (r *DocumentRepository) DeleteLines(ctx context.Context, documentID int64) (int64, error) {
	return r.q(ctx).DeleteInvoiceDocumentLines(ctx, documentID)
}

// ReviseSnapshot 是 R03 的就地更新（§7.3.5）。就地而不是建新列的五條依據見
// db/queries/invoice_documents.sql 的 ReviseInvoiceDocumentSnapshot 註解。
//
// rows=0 ⇒ 版本被推進，或 status 已離開 PENDING／AWAITING_MANUAL。
// **不是錯誤，但絕不能當成成功**——那代表這份文件可能已經送出去了。
func (r *DocumentRepository) ReviseSnapshot(ctx context.Context, in domain.ReviseDocumentSnapshot) (bool, error) {
	// 明文進、密文出：與 Create 走同一個金鑰環，呼叫端永遠碰不到密文。
	emailCT, err := r.ring.encryptOptional(in.RecipientEmail)
	if err != nil {
		return false, err
	}
	carrier1CT, err := r.ring.encryptOptional(in.CarrierID1)
	if err != nil {
		return false, err
	}
	carrier2CT, err := r.ring.encryptOptional(in.CarrierID2)
	if err != nil {
		return false, err
	}

	rows, err := r.q(ctx).ReviseInvoiceDocumentSnapshot(ctx, gen.ReviseInvoiceDocumentSnapshotParams{
		SnapshotVersion:          in.SnapshotVersion,
		DocumentKind:             in.DocumentKind,
		SalesAmount:              in.SalesAmount,
		FreeTaxSalesAmount:       in.FreeTaxSalesAmount,
		ZeroTaxSalesAmount:       in.ZeroTaxSalesAmount,
		TaxAmount:                in.TaxAmount,
		TotalAmount:              in.TotalAmount,
		StatutoryDeadlineAt:      in.StatutoryDeadline,
		RecipientType:            in.RecipientType,
		RecipientRefID:           in.RecipientRefID,
		RecipientName:            in.RecipientName,
		RecipientTaxID:           in.RecipientTaxID,
		RecipientEmailCiphertext: emailCT,
		RecipientEmailHash:       r.ring.hashOptional(in.RecipientEmail),
		CarrierType:              in.CarrierType,
		CarrierId1Ciphertext:     carrier1CT,
		CarrierId2Ciphertext:     carrier2CT,
		CarrierValueHash:         r.ring.hashOptional(in.CarrierID1),
		DonationCode:             in.DonationCode,
		Now:                      ts(in.Now),
		ID:                       in.ID,
		ExpectedVersion:          in.ExpectedVersion,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// NextReceiptNo 取下一個平台代製收據編號。
func (r *DocumentRepository) NextReceiptNo(ctx context.Context) (string, error) {
	return r.q(ctx).NextReceiptNo(ctx)
}

// MarkIssuing 是 PENDING／ISSUE_FAILED → ISSUING（ISSUE operation READY→SENDING 的同一事務內連動）。
func (r *DocumentRepository) MarkIssuing(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceDocumentIssuing(ctx, gen.MarkInvoiceDocumentIssuingParams{
		Now: ts(now),
		ID:  id,
	})
	return rows > 0, err
}

// MarkIssued 是 ISSUING → ISSUED，並回填號碼與兩組候選集合入口。
//
// chk_invdoc_issued_has_no：AMEGO_AUTO ＋ ISSUED ⇒ provider_invoice_no 不得為空
// ⇒ 號碼必須在這一句寫進去，分兩句寫第一句就 23514。
func (r *DocumentRepository) MarkIssued(ctx context.Context, in domain.MarkDocumentIssued) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceDocumentIssued(ctx, gen.MarkInvoiceDocumentIssuedParams{
		Provider:            snp(in.Provider),
		ProviderInvoiceNo:   snp(in.ProviderInvoiceNo),
		ProviderRandomCode:  snp(in.ProviderRandomCode),
		ProviderIssuedAt:    in.ProviderIssuedAt,
		InvoiceDateTaipei:   pgxutil.DateParam(in.InvoiceDateTaipei),
		TaxPeriodCode:       snp(in.TaxPeriodCode),
		PdfFetchNextAt:      in.PdfFetchNextAt,
		UploadRecheckNextAt: in.UploadRecheckNextAt,
		Now:                 ts(in.Now),
		ID:                  in.ID,
	})
	return rows > 0, err
}

// MarkReceiptIssued 是 ISSUING → ISSUED 的**收據**版本（PLATFORM_RECEIPT，#3026）。
//
// 與 MarkIssued 分開的理由不是「兩種文件」，是 DB 約束互斥：
// chk_invdoc_receipt_no_provider 要求 RECEIPT 的 provider 與 provider_invoice_no
// 皆為 NULL，而 MarkIssued 那一句無條件寫入 provider ⇒ 拿它開收據必定 23514。
func (r *DocumentRepository) MarkReceiptIssued(ctx context.Context, in domain.MarkReceiptIssued) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceDocumentReceiptIssued(ctx, gen.MarkInvoiceDocumentReceiptIssuedParams{
		ReceiptAuthorizationVersion: snp(in.AuthorizationVersion),
		PdfObjectKey:                snp(in.PdfObjectKey),
		ProviderIssuedAt:            &in.IssuedAt,
		Now:                         ts(in.Now),
		ID:                          in.ID,
	})
	return rows > 0, err
}

// MarkIssueFailed 是 ISSUING → ISSUE_FAILED。
//
// ⚠️ 觸發它的第四個來源是 SELLER_ACTION_REQUIRED，那個名字**不叫 FAILED_***：把觸發條件
// 寫成「任一 FAILED_*」會漏掉它，而漏掉的後果是 document 永遠停在 ISSUING，賣家看到
// 「開立中」而系統早已停手在等他去財政部配字軌。判定在 usecase，本句只負責落地。
func (r *DocumentRepository) MarkIssueFailed(ctx context.Context, id int64, reasonCode string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceDocumentIssueFailed(ctx, gen.MarkInvoiceDocumentIssueFailedParams{
		ManualReasonCode: nilIfBlank(reasonCode),
		Now:              ts(now),
		ID:               id,
	})
	return rows > 0, err
}

// MarkPendingRetry 是 ISSUING → PENDING（operation 退回 RETRY_SCHEDULED）。
func (r *DocumentRepository) MarkPendingRetry(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceDocumentPendingRetry(ctx, gen.MarkInvoiceDocumentPendingRetryParams{
		Now: ts(now),
		ID:  id,
	})
	return rows > 0, err
}

// MarkAwaitingManual 是 PENDING／ISSUE_FAILED → AWAITING_MANUAL。
//
// chk_invdoc_manual_reason：AWAITING_MANUAL ⇒ manual_reason_code 不得為空 ⇒ 理由碼與狀態
// 必須同一句寫。它同時是 ADM NEEDS_MANUAL 工作台直接顯示的那個值。
func (r *DocumentRepository) MarkAwaitingManual(ctx context.Context, id int64, manualReasonCode string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceDocumentAwaitingManual(ctx, gen.MarkInvoiceDocumentAwaitingManualParams{
		ManualReasonCode: snp(manualReasonCode),
		Now:              ts(now),
		ID:               id,
	})
	return rows > 0, err
}

// WithDecryptedRecipient 在 callback 內提供一列文件的收件人明文。
//
// 🔴 明文不得逸出 fn：本函式不回傳它、不記 log、不放進任何 struct 欄位。
func (r *DocumentRepository) WithDecryptedRecipient(ctx context.Context, id int64, fn func(domain.RecipientSecrets) error) error {
	row, err := r.q(ctx).GetInvoiceDocumentByID(ctx, id)
	if err != nil {
		return notFound(err, bizcode.InvoiceDocumentNotFound)
	}
	secrets, err := r.ring.openRecipient(row)
	if err != nil {
		return err
	}
	return fn(secrets)
}

// ResumeFromManual 是 AWAITING_MANUAL → PENDING（ADM 給出判定，回自動線）。
//
// expectedVersion 是 document 自己的版本，不是 job 的：兩者是不同的列、各自遞增。
// 呼叫端在同一交易內先讀出 document 才組得出這個值。
func (r *DocumentRepository) ResumeFromManual(ctx context.Context, id, expectedVersion int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).ResumeInvoiceDocumentFromManual(ctx, gen.ResumeInvoiceDocumentFromManualParams{
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	return rows > 0, err
}

// MarkResolvedExternally 是 ISSUE_FAILED／AWAITING_MANUAL → RESOLVED_EXTERNALLY。
//
// 三個 external_* 欄由 chk_invdoc_external_no 綁成雙向等式 ⇒ 必須同一句一起寫，
// 只寫號碼當場 23514。provider_invoice_no 保持 NULL——平台不冒充開立人。
func (r *DocumentRepository) MarkResolvedExternally(ctx context.Context, in domain.MarkDocumentResolvedExternally) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceDocumentResolvedExternally(ctx, gen.MarkInvoiceDocumentResolvedExternallyParams{
		ExternalInvoiceNo:   &in.ExternalInvoiceNo,
		ExternalInvoiceDate: dateParam(in.ExternalInvoiceDate),
		ExternalReportedAt:  tp(in.ExternalReportedAt),
		Now:                 ts(in.Now),
		ID:                  in.ID,
	})
	return rows > 0, err
}

// GetByProviderRequestNo 依 provider_request_no 讀一列（端點 57 的冪等查詢）。
//
// 查不到映成 9301：那是「這把冪等鍵還沒有人用過」，呼叫端據此決定建新的。
func (r *DocumentRepository) GetByProviderRequestNo(ctx context.Context, providerRequestNo string) (domain.Document, error) {
	row, err := r.q(ctx).GetInvoiceDocumentByProviderRequestNo(ctx, &providerRequestNo)
	if err != nil {
		return domain.Document{}, notFound(err, bizcode.InvoiceDocumentNotFound)
	}
	return toDocument(row)
}

// ---------------------------------------------------------------------------
// 作廢軸（M1 的 VOIDING 三條邊 ＋ 重開的 SUPERSEDED）
//
// 四支都是 thin adapter，守衛全在 SQL 的 WHERE 述詞裡。rows==0 一律回 false 而不是
// error：那代表「來源狀態已經不是我預期的那個」，是**合法**的併發結果，該由呼叫端
// 依自己的語意決定要不要當成失敗（作廢是要的，重新 claim 到已在 VOIDING 的則不是）。
// ---------------------------------------------------------------------------

// MarkVoiding 是 ISSUED → VOIDING。
func (r *DocumentRepository) MarkVoiding(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceDocumentVoiding(ctx, gen.MarkInvoiceDocumentVoidingParams{
		Now: ts(now),
		ID:  id,
	})
	return rows > 0, err
}

// MarkVoided 是 VOIDING → VOIDED。
func (r *DocumentRepository) MarkVoided(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceDocumentVoided(ctx, gen.MarkInvoiceDocumentVoidedParams{
		Now: &now,
		ID:  id,
	})
	return rows > 0, err
}

// RestoreToIssued 是 VOIDING → ISSUED（R05：作廢被 provider 拒絕）。
func (r *DocumentRepository) RestoreToIssued(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).RestoreInvoiceDocumentToIssued(ctx, gen.RestoreInvoiceDocumentToIssuedParams{
		Now: ts(now),
		ID:  id,
	})
	return rows > 0, err
}

// MarkSuperseded 是 VOIDED → SUPERSEDED（R10：重開的新 document 轉 ISSUED）。
func (r *DocumentRepository) MarkSuperseded(ctx context.Context, id, supersededBy int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceDocumentSuperseded(ctx, gen.MarkInvoiceDocumentSupersededParams{
		SupersededByID: &supersededBy,
		Now:            ts(now),
		ID:             id,
	})
	return rows > 0, err
}

// CancelBeforeIssue 是 PENDING／AWAITING_MANUAL → CANCELLED_BEFORE_ISSUE。
func (r *DocumentRepository) CancelBeforeIssue(ctx context.Context, id int64, reasonCode, reason string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceDocumentCancelledBeforeIssue(ctx, gen.MarkInvoiceDocumentCancelledBeforeIssueParams{
		CancelReasonCode: &reasonCode,
		CancelReason:     nilIfBlank(reason),
		Now:              ts(now),
		ID:               id,
	})
	return rows > 0, err
}

// AddAllowanceTotal 是 I4 的單句 CAS（§7.3.7 G09，與 adjustment 的狀態轉移同交易）。
//
// rows=0 代表 chk_invdoc_allowance_cap 或 status 述詞擋下來了 —— 那是「這筆折讓不該
// 生效」，呼叫端必須整筆 rollback，不可當成功。
func (r *DocumentRepository) AddAllowanceTotal(
	ctx context.Context, documentID, delta int64, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).AddInvoiceDocumentAllowanceTotal(ctx, gen.AddInvoiceDocumentAllowanceTotalParams{
		ID: documentID, DeltaAmount: delta, Now: ts(now),
	})
	return rows > 0, err
}

// AddLineAllowancedAmount 逐行累加已折讓額（含稅）。
//
// 🔴 delta 是**含稅**金額：allowanceable_base 是含稅基數，而折讓明細存的是未稅
// ⇒ 呼叫端要先把稅額加回去。少加的後果是下一次折讓的剩餘額被高估。
// rows=0 代表 chk_invline_allowanced_cap 擋下來了（累計超過該行的可折讓基數）。
func (r *DocumentRepository) AddLineAllowancedAmount(
	ctx context.Context, lineID, delta int64, _ time.Time,
) (bool, error) {
	rows, err := r.q(ctx).AddInvoiceDocumentLineAllowancedAmount(ctx,
		gen.AddInvoiceDocumentLineAllowancedAmountParams{ID: lineID, DeltaAmount: delta})
	return rows > 0, err
}
