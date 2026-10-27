package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// INV.pdf —— 發票 PDF 抓檔落自有儲存（§4.3.3 第 5 組、§7.5.3、§8.1.5 P8）
//
// 為什麼非做不可（P8 逐字）：「發票 PDF 與明細**只在開立後 180 天內**可取；
// `file_url` 僅 10 分鐘有效」，而**法規要保存 5 年**。開立成功後不立刻抓檔落到
// 平台自有儲存，180 天後就再也拿不回來。
//
// 🔴 P8 的第二句同樣是硬要求：「下載後**必須檢查** Content-Type == application/pdf
// 或 magic bytes `%PDF-`，**只看 HTTP status 會把錯誤頁存成 PDF**」——AMEGO 的
// file_url 過期／被竄改時回 **HTTP 200 ＋ 中文純文字**。
//
// 租約 120s，下界 50.2s ＝ invoice_file 10s ＋ 下載 20s ＋ MED 上傳 20s ＋ DB 0.2s。
// **刻意不併進 INV.op**（§7.5.4）：併進去下界逼近 110s、租約得放大到 4 分鐘，
// 而開票是法定期限最緊的路徑。
// ---------------------------------------------------------------------------

// pdfBatch／pdfLease／pdfMaxAttempts 取自 §7.5.3 的表（FetchBatch 20／租約 120s／
// MaxAttempts 8 ＝ 7 段退避約 2.0 小時）。
const (
	pdfBatch       = 20
	pdfLease       = 120 * time.Second
	pdfMaxAttempts = 8
)

// noTaxIDPlaceholder 是「沒有統編」的佔位值（十個 0）。B2C 發票一律是它。
const noTaxIDPlaceholder = "0000000000"

// downloadStyleFor 依發票有沒有統編決定 download_style（§8.1.5 第 3 條）。
//
// 🔴 規格逐字：「`style` 以白名單 `{0,1,2,3,5}` 驗證；**沒打統編的發票只有 `0`**，
// 其餘值一律降為 `0`」。
//
// 🔴 這個判定不能省，而且**AMEGO 不會幫你擋**：規格同一條寫著「AMEGO 對
// `download_style` 無值域驗證，傳 `9` 仍回 `code:0` 並產出 URL」——但對「沒統編卻
// 傳非 0」它會回 **code 53「該發票類型不可下載」**。
//
// ⚠️ 2026-08-31 dev 實測就是踩這一格：我原本寫死 `DownloadStyle: 1`，
// 而那張 B2C 發票的 `recipient_tax_id` 是十個 0 ⇒ 連續四輪 code 53、
// 眼看要燒完 8 次重試然後以 Error 級「放棄」收場，而根本沒有任何東西壞掉。
func downloadStyleFor(doc domain.Document) int {
	if doc.RecipientTaxID == "" || doc.RecipientTaxID == noTaxIDPlaceholder {
		return 0
	}
	return 1
}

// isNotDownloadable 判斷這次失敗是「這一份現在下載不了」而不是「暫時性故障」。
//
// 🔴 AMEGO 的 **code 53 在官方錯誤表出現兩次且語意不同**（§8.1.5 第 4 條逐字），
// 所以**不可只靠 code 分支**：
//
//	① 載具發票中獎前無 PDF ⇒ 對應業務碼 9312，**正常業務分支不是錯誤**
//	② 該發票類型不可下載
//
// 兩者的共同點是「再重試 8 次也不會變」⇒ 一律停止並記錄，不燒 attempt。
//
// 為什麼不繼續重試：燒完 8 次之後會走 MarkPdfFetchStopped ＋ **Error 級 log**，
// 而那條 Error 是留給「法規要 5 年、AMEGO 只給 180 天，這份沒補就永久取不回」
// 的真實風險用的。把正常業務分支混進去，第二次就會被當成狼來了。
func isNotDownloadable(err error) bool {
	var apiErr *amego.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Code == "53"
}

// pdfMagic 是 PDF 的檔頭 magic bytes（P8 指定的兩道驗證之一）。
var pdfMagic = []byte("%PDF-")

// pdfContentType 是 P8 指定的另一道驗證。
const pdfContentType = "application/pdf"

// pdfDownloadTimeout 是下載 file_url 的預算（§7.5.4 的下界輸入：20s）。
const pdfDownloadTimeout = 20 * time.Second

// PDFFileURLFetcher 取回 AMEGO 的下載連結（**僅 10 分鐘有效，取得後必須立即下載**）。
type PDFFileURLFetcher interface {
	FetchInvoiceFileURL(ctx context.Context, c amego.Credential, req amego.FileRequest) (string, error)
}

// PDFUploader 把抓回來的檔案落到自有儲存，回 objectKey。
type PDFUploader interface {
	UploadSecure(ctx context.Context, data []byte, filename, contentType, refID string) (string, error)
}

// PDFDocumentStore 是 INV.pdf 需要的 repository 面（outbox.Repository 的五方法對應）。
type PDFDocumentStore interface {
	FetchDuePdfFetches(ctx context.Context, now time.Time, limit int32) ([]domain.Document, error)
	ClaimPdfFetch(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error)
	MarkPdfFetchDone(ctx context.Context, id int64, objectKey string, now time.Time) error
	MarkPdfFetchStopped(ctx context.Context, id int64, now time.Time) error
	UpdatePdfFetchAttempt(ctx context.Context, id int64, nextAttemptAt, now time.Time) error
}

// PDFCredentialSource 供解密憑證。
type PDFCredentialSource interface {
	GetByIssuer(ctx context.Context, issuerID int64, provider string) (domain.ProviderAccount, error)
	WithDecryptedCredential(ctx context.Context, id int64, fn func(providerTaxID, appKey string) error) error
}

// PDF 是 INV.pdf 的 usecase。
type PDF struct {
	docs     PDFDocumentStore
	accounts PDFCredentialSource
	amego    PDFFileURLFetcher
	uploader PDFUploader
	http     *http.Client
	log      *zap.Logger
	now      func() time.Time
	maxBytes int64
}

// NewPDF 建 INV.pdf usecase。
func NewPDF(
	docs PDFDocumentStore, accounts PDFCredentialSource,
	fetcher PDFFileURLFetcher, uploader PDFUploader,
	log *zap.Logger, now func() time.Time, maxBytes int64,
) *PDF {
	return &PDF{
		docs: docs, accounts: accounts, amego: fetcher, uploader: uploader,
		http: &http.Client{Timeout: pdfDownloadTimeout},
		log:  orNop(log), now: now, maxBytes: maxBytes,
	}
}

// looksLikePDF 是 P8 指定的兩道驗證：Content-Type 或 magic bytes 任一成立即可。
//
// 🔴 **不能只看 HTTP status**：AMEGO 的 file_url 過期或被竄改時回 **200 ＋ 中文純文字**，
// 只看狀態碼會把那段文字當成 PDF 存進自有儲存 —— 而業務表會記下 pdf_object_key、
// 買家點下載才發現拿到一份壞檔，且此時 AMEGO 那份可能已超過 180 天。
func looksLikePDF(contentType string, body []byte) bool {
	if contentType == pdfContentType {
		return true
	}
	return bytes.HasPrefix(body, pdfMagic)
}

// RunOnce 跑一輪 PDF 抓檔。
func (u *PDF) RunOnce(ctx context.Context) error {
	due, err := u.docs.FetchDuePdfFetches(ctx, u.now(), pdfBatch)
	if err != nil {
		return err
	}
	var claimed, done, retried, stopped int
	for _, doc := range due {
		// 租約基準逐筆在 claim 當下取（《多副本排程搶佔規範》）。
		at := u.now()
		ok, err := u.docs.ClaimPdfFetch(ctx, doc.ID, at, at.Add(pdfLease))
		if err != nil {
			u.log.Error("invoice.pdf.claim_failed", zap.Int64("document_id", doc.ID), zap.Error(err))
			continue
		}
		if !ok {
			continue
		}
		claimed++

		key, ferr := u.fetchOne(ctx, doc)
		if ferr == nil {
			if err := u.docs.MarkPdfFetchDone(ctx, doc.ID, key, u.now()); err != nil {
				u.log.Error("invoice.pdf.mark_done_failed", zap.Int64("document_id", doc.ID), zap.Error(err))
				continue
			}
			done++
			continue
		}

		// 🔴 code 53「這一份現在下載不了」：停止並記錄，**不燒 attempt**（見 isNotDownloadable）。
		if isNotDownloadable(ferr) {
			if err := u.docs.MarkPdfFetchStopped(ctx, doc.ID, u.now()); err != nil {
				u.log.Error("invoice.pdf.mark_stopped_failed", zap.Int64("document_id", doc.ID), zap.Error(err))
				continue
			}
			stopped++
			// Info 而不是 Error：這是正常業務分支（載具未中獎／該類型不可下載），
			// 讀路徑的 FileAvailability 會據此回 9312／不提供下載按鈕。
			u.log.Info("invoice.pdf.not_downloadable",
				zap.Int64("document_id", doc.ID), zap.Error(ferr))
			continue
		}

		// 失敗：達上限就停、否則退避重試。
		if int(doc.PdfFetchAttempts)+1 >= pdfMaxAttempts {
			if err := u.docs.MarkPdfFetchStopped(ctx, doc.ID, u.now()); err != nil {
				u.log.Error("invoice.pdf.mark_stopped_failed", zap.Int64("document_id", doc.ID), zap.Error(err))
				continue
			}
			stopped++
			// 🔴 停止不是「沒事了」：法規要 5 年而 AMEGO 只給 180 天 ⇒ 這一份若不人工補，
			// 期限一過就永久取不回。用 Error 而不是 Warn，讓它進得了告警。
			u.log.Error("invoice.pdf.gave_up",
				zap.Int64("document_id", doc.ID),
				zap.Int("attempts", int(doc.PdfFetchAttempts)+1),
				zap.Error(ferr))
			continue
		}
		next := u.now().Add(recheckBackoff(time.Duration(doc.PdfFetchAttempts) * time.Minute))
		if err := u.docs.UpdatePdfFetchAttempt(ctx, doc.ID, next, u.now()); err != nil {
			u.log.Error("invoice.pdf.update_attempt_failed", zap.Int64("document_id", doc.ID), zap.Error(err))
			continue
		}
		retried++
		u.log.Warn("invoice.pdf.retry_scheduled", zap.Int64("document_id", doc.ID), zap.Error(ferr))
	}
	u.log.Info("invoice.pdf.scanned",
		zap.Int("due", len(due)), zap.Int("claimed", claimed),
		zap.Int("done", done), zap.Int("retried", retried), zap.Int("stopped", stopped))
	return nil
}

// fetchOne 對單一文件做「取連結 → 下載 → 驗型別 → 上傳」四步，回 objectKey。
func (u *PDF) fetchOne(ctx context.Context, doc domain.Document) (string, error) {
	if doc.ProviderInvoiceNo == nil || *doc.ProviderInvoiceNo == "" {
		return "", fmt.Errorf("invoice: document %d 沒有 provider_invoice_no，不該進 INV.pdf 候選集合", doc.ID)
	}
	acct, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, derefOr(doc.Provider))
	if err != nil {
		return "", err
	}

	var fileURL string
	var fetchErr error
	if err := u.accounts.WithDecryptedCredential(ctx, acct.ID, func(providerTaxID, appKey string) error {
		fileURL, fetchErr = u.amego.FetchInvoiceFileURL(ctx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey},
			amego.FileRequest{Type: "invoice", InvoiceNumber: *doc.ProviderInvoiceNo, DownloadStyle: downloadStyleFor(doc)})
		return nil
	}); err != nil {
		return "", err
	}
	if fetchErr != nil {
		return "", fetchErr
	}
	if fileURL == "" {
		return "", fmt.Errorf("invoice: AMEGO 回空的 file_url")
	}

	// 🔴 URL 僅 10 分鐘有效，取得後立即下載（§8.1.5 P8）。
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := u.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invoice: 下載 file_url 回 HTTP %d", resp.StatusCode)
	}
	// 讀滿 maxBytes+1 才能分辨「剛好等於上限」與「超過上限」。
	body, err := io.ReadAll(io.LimitReader(resp.Body, u.maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > u.maxBytes {
		return "", fmt.Errorf("invoice: PDF 超過上限 %d bytes", u.maxBytes)
	}
	// 🔴 P8 的硬要求：只看 HTTP status 會把錯誤頁存成 PDF。
	if !looksLikePDF(resp.Header.Get("Content-Type"), body) {
		return "", fmt.Errorf("invoice: 下載內容不是 PDF（content-type=%q, 前 8 bytes=%q）",
			resp.Header.Get("Content-Type"), string(body[:min(8, len(body))]))
	}

	filename := *doc.ProviderInvoiceNo + ".pdf"
	// refID 用 invoice_documents.id 不用 order_no（§8.6 T-04）。
	return u.uploader.UploadSecure(ctx, body, filename, pdfContentType, strconv.FormatInt(doc.ID, 10))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
