package usecase

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)


const (
	sellerExternalEvidenceMax = 5
)

// 型別邊界錯誤（契約違規 ⇒ 400，不借業務碼）。
//
// 🔴 號碼格式不合回 **400 而不是 9349**（§6.1 端點 62 逐字）：9349 的語意是「本作業尚未
// 開放自行開立回報」，賣家的下一步是等 ADM 解鎖；格式錯的下一步是把號碼重打一次。
var (
	ErrExternalEvidenceTooMany = errors.New("invoice: evidence_asset_keys exceeds 5 items")
	ErrExternalEvidenceInvalid = errors.New("invoice: evidence_asset_keys must be media object keys, not URLs")
)

// SellerJob 是端點 62 的寫入 usecase。
type SellerJob struct {
	jobs *AdminJob
	ops  domain.OperationRepository
	docs domain.DocumentRepository
}

// NewSellerJob 組出端點 62 的寫入 usecase。
//
// 它刻意**不持有自己的 TxManager**：交易邊界必須與 resolveExternally 用的是同一把，
// 否則那四件事會落在兩個交易裡，而「只轉 operation 不轉 document」正是它要防的組合。
func NewSellerJob(jobs *AdminJob, ops domain.OperationRepository, docs domain.DocumentRepository) *SellerJob {
	return &SellerJob{jobs: jobs, ops: ops, docs: docs}
}

// ReportExternalIssuanceInput 是端點 62 的輸入。
type ReportExternalIssuanceInput struct {
	StoreID           int64
	JobID             int64
	InvoiceNo         string
	InvoiceDate       string // YYYY-MM-DD（UTC+8 曆日）
	EvidenceAssetKeys []string
	Note              string
	ExpectedVersion   int64
	IdempotencyKey    string
}

// ReportExternalIssuance 讓賣家回報自己開立的發票號碼（端點 62 ＝ J30）。
//
// 🔴 三條不可協商的落點（與端點 56 逐字相同，因為走的是同一個 helper）：
//
//	operation.status → RESOLVED_EXTERNALLY（終態）
//	document.status  → RESOLVED_EXTERNALLY，號碼寫進 external_invoice_no
//	provider_invoice_no **必須留空** —— 平台不冒充開立人；責任歸屬在爭議時是法律問題
//
// 🔴 唯一的來源狀態是 MANUAL_PROCESS_ALLOWED（ADM 雙人解鎖的產物）。不在那一格就回
// 9349，**不是**靜默成功：賣家按了沒反應而 operation 永遠停在非終態，買家端永遠顯示
// 「處理中」，法定期限照樣倒數。
func (u *SellerJob) ReportExternalIssuance(ctx context.Context, in ReportExternalIssuanceInput) (domain.OperationView, error) {
	if !externalInvoiceNoPattern.MatchString(in.InvoiceNo) {
		return domain.OperationView{}, ErrExternalInvoiceNoInvalid
	}
	invoiceDate, err := time.Parse(time.DateOnly, in.InvoiceDate)
	if err != nil {
		return domain.OperationView{}, ErrExternalInvoiceDateInvalid
	}
	if utf8.RuneCountInString(in.Note) > adminNoteMaxRunes {
		return domain.OperationView{}, ErrJobNoteTooLong
	}
	if err := validateEvidenceAssetKeys(in.EvidenceAssetKeys); err != nil {
		return domain.OperationView{}, err
	}

	var out domain.OperationView
	err = u.jobs.tx.WithTx(ctx, func(ctx context.Context) error {
		view, err := u.ops.GetViewForSeller(ctx, in.JobID, in.StoreID)
		if err != nil {
			return err
		}
		op := view.Operation
		if op.Version != in.ExpectedVersion {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		doc, err := u.docs.GetByID(ctx, op.DocumentID)
		if err != nil {
			return err
		}
		if !canReportExternalIssuance(op, doc) {
			return jobBiz(bizcode.InvoiceExternalIssuanceNotAllowed)
		}
		if invoiceDate.Before(domain.InvoiceDateTaipei(doc.IssueNotBefore)) {
			return ErrExternalInvoiceDateTooEarly
		}

		now := u.jobs.now()
		if rerr := u.jobs.resolveExternally(ctx, op, doc, externalIssuance{
			InvoiceNo:   in.InvoiceNo,
			InvoiceDate: invoiceDate,
			TotalAmount: doc.TotalAmount,
			Note:        externalNoteWithEvidence(in.Note, in.EvidenceAssetKeys),
			ActorType:      providerActorSeller,
			ActorAccountID: in.StoreID,
			Now:            now,
		}); rerr != nil {
			return rerr
		}


		updated, err := u.ops.GetViewForSeller(ctx, in.JobID, in.StoreID)
		if err != nil {
			return err
		}
		out = updated
		return nil
	})
	if err != nil {
		return domain.OperationView{}, err
	}
	return out, nil
}

func validateEvidenceAssetKeys(keys []string) error {
	if len(keys) > sellerExternalEvidenceMax {
		return ErrExternalEvidenceTooMany
	}
	for _, k := range keys {
		if k == "" ||
			strings.HasPrefix(k, "http://") || strings.HasPrefix(k, "https://") ||
			strings.HasPrefix(k, "/") || strings.Contains(k, "..") {
			return ErrExternalEvidenceInvalid
		}
	}
	return nil
}

func externalNoteWithEvidence(note string, keys []string) string {
	if len(keys) == 0 {
		return note
	}
	joined := "evidence=" + strings.Join(keys, ",")
	if note == "" {
		return joined
	}
	return note + " / " + joined
}
