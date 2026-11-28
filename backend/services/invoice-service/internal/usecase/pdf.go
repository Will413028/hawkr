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

const (
	pdfBatch       = 20
	pdfLease       = 120 * time.Second
	pdfMaxAttempts = 8
)

const noTaxIDPlaceholder = "0000000000"

func downloadStyleFor(doc domain.Document) int {
	if doc.RecipientTaxID == "" || doc.RecipientTaxID == noTaxIDPlaceholder {
		return 0
	}
	return 1
}

func isNotDownloadable(err error) bool {
	var apiErr *amego.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Code == "53"
}

var pdfMagic = []byte("%PDF-")

const pdfContentType = "application/pdf"

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

		if isNotDownloadable(ferr) {
			if err := u.docs.MarkPdfFetchStopped(ctx, doc.ID, u.now()); err != nil {
				u.log.Error("invoice.pdf.mark_stopped_failed", zap.Int64("document_id", doc.ID), zap.Error(err))
				continue
			}
			stopped++
			u.log.Info("invoice.pdf.not_downloadable",
				zap.Int64("document_id", doc.ID), zap.Error(ferr))
			continue
		}

		if int(doc.PdfFetchAttempts)+1 >= pdfMaxAttempts {
			if err := u.docs.MarkPdfFetchStopped(ctx, doc.ID, u.now()); err != nil {
				u.log.Error("invoice.pdf.mark_stopped_failed", zap.Int64("document_id", doc.ID), zap.Error(err))
				continue
			}
			stopped++
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, u.maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > u.maxBytes {
		return "", fmt.Errorf("invoice: PDF 超過上限 %d bytes", u.maxBytes)
	}
	if !looksLikePDF(resp.Header.Get("Content-Type"), body) {
		return "", fmt.Errorf("invoice: 下載內容不是 PDF（content-type=%q, 前 8 bytes=%q）",
			resp.Header.Get("Content-Type"), string(body[:min(8, len(body))]))
	}

	filename := *doc.ProviderInvoiceNo + ".pdf"
	return u.uploader.UploadSecure(ctx, body, filename, pdfContentType, strconv.FormatInt(doc.ID, 10))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
