package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/domain/fsm"
	"go.uber.org/zap"
)

const recheckOpLease = 60 * time.Second

const recheckUploadLease = 120 * time.Second

const recheckBatch = 50

const providerRetentionWindow = 180 * 24 * time.Hour

func derefOr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// RecheckQuerier 是候選集合①需要的出站面。
type RecheckQuerier interface {
	QueryInvoiceByOrderID(ctx context.Context, c amego.Credential, orderID string) (*amego.InvoiceDetail, error)
}

// RecheckOperationStore 是候選集合①需要的 repository 面。
type RecheckOperationStore interface {
	FetchPendingRechecks(ctx context.Context, now time.Time, limit int32) ([]domain.Operation, error)
	ClaimRecheck(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error)
	MarkRecheckResolved(ctx context.Context, in domain.MarkRecheckResult) (bool, error)
	MarkRecheckUnresolved(ctx context.Context, in domain.MarkRecheckResult) (bool, error)
	UpdateRecheckAttempt(ctx context.Context, in domain.UpdateRecheckAttempt) (bool, error)
}

// RecheckDocumentStore 是兩組候選都要用的 document 面。
type RecheckDocumentStore interface {
	GetByID(ctx context.Context, id int64) (domain.Document, error)
	FetchDueUploadRechecks(ctx context.Context, now time.Time, limit int32) ([]domain.Document, error)
	ClaimUploadRecheck(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error)
	RecordUploadRecheckResult(ctx context.Context, id int64, providerUploadStatus *int16, now time.Time, nextAt *time.Time) error
}

// RecheckVoidConfirmer 是作廢軸的確認面。
//
// 🔴 為什麼 INV.rc 要分派而不是讓 VOID operation 走原本那條路：probeOperation 打的是
// invoice_query type=order 並以「查得到發票號碼」當成「開立成功」。作廢單走進去
// 會查到那張**仍然存在的**發票 ⇒ 判成 Resolved ⇒ 把 document 寫回 ISSUED，
// 等於把剛送出去的作廢從帳上抹掉，而且完全不報錯。
type RecheckVoidConfirmer interface {
	ConfirmVoid(ctx context.Context, op domain.Operation) error
}

// RecheckAllowanceConfirmer 是折讓軸的確認面。
//
// 🔴 與作廢同一個理由、不同的後果：折讓 operation 走 probeOperation 會查到那張
// **仍然存在的原發票**、判成 Resolved ⇒ operation 被標成完成，而折讓到底開沒開
// 沒有任何人知道，adjustment 永遠停在 WAITING_CONFIRMATION。折讓只能拿我方自產的
// allowance_number 打 allowance_query，見 Allowance.ConfirmAllowance。
type RecheckAllowanceConfirmer interface {
	ConfirmAllowance(ctx context.Context, op domain.Operation) error
}

// RecheckCredentialSource 供解密憑證。
type RecheckCredentialSource interface {
	GetByIssuer(ctx context.Context, issuerID int64, provider string) (domain.ProviderAccount, error)
	WithDecryptedCredential(ctx context.Context, id int64, fn func(providerTaxID, appKey string) error) error
}

// Recheck 是 INV.rc 的 usecase。
type Recheck struct {
	voids      RecheckVoidConfirmer
	allowances RecheckAllowanceConfirmer
	ops        RecheckOperationStore
	docs       RecheckDocumentStore
	accounts   RecheckCredentialSource
	amego      RecheckQuerier
	audit      domain.AuditRepository
	log        *zap.Logger
	now        func() time.Time
}

// NewRecheck 建 INV.rc usecase。
func NewRecheck(
	ops RecheckOperationStore, docs RecheckDocumentStore,
	accounts RecheckCredentialSource, querier RecheckQuerier,
	voids RecheckVoidConfirmer, allowances RecheckAllowanceConfirmer,
	audit domain.AuditRepository, log *zap.Logger, now func() time.Time,
) *Recheck {
	return &Recheck{voids: voids, allowances: allowances, ops: ops, docs: docs, accounts: accounts,
		amego: querier, audit: audit, log: orNop(log), now: now}
}

// RunOnce 跑一輪：先對帳①、再覆查②。
//
// 兩組獨立，①失敗不擋②——它們是不同的候選集合、不同的租約、不同的業務後果。
func (u *Recheck) RunOnce(ctx context.Context) error {
	var firstErr error
	if err := u.reconcileOperations(ctx); err != nil {
		firstErr = err
		u.log.Error("invoice.rc.operations_failed", zap.Error(err))
	}
	if err := u.recheckUploads(ctx); err != nil {
		if firstErr == nil {
			firstErr = err
		}
		u.log.Error("invoice.rc.uploads_failed", zap.Error(err))
	}
	return firstErr
}

func (u *Recheck) reconcileOperations(ctx context.Context) error {
	due, err := u.ops.FetchPendingRechecks(ctx, u.now(), recheckBatch)
	if err != nil {
		return err
	}
	var claimed, resolved, stillWaiting, terminal int
	for _, op := range due {
		at := u.now()
		ok, err := u.ops.ClaimRecheck(ctx, op.ID, at, at.Add(recheckOpLease))
		if err != nil {
			u.log.Error("invoice.rc.claim_failed", zap.Int64("operation_id", op.ID), zap.Error(err))
			continue
		}
		if !ok {
			continue
		}
		claimed++
		if err := u.recordClaimTransition(ctx, op.ID, op.Status, at); err != nil {
			u.log.Error("invoice.rc.claim_history_failed",
				zap.Int64("operation_id", op.ID), zap.String("from", op.Status), zap.Error(err))
		}
		op.Status = codemeta.InvoiceOperationStatusProviderStatusRecheck.Value
		if op.OperationType == codemeta.InvoiceOperationTypeVoid.Value {
			if err := u.voids.ConfirmVoid(ctx, op); err != nil {
				u.log.Error("invoice.rc.confirm_void_failed",
					zap.Int64("operation_id", op.ID), zap.Error(err))
			}
			stillWaiting++
			continue
		}
		if op.OperationType == codemeta.InvoiceOperationTypeAllowance.Value {
			if err := u.allowances.ConfirmAllowance(ctx, op); err != nil {
				u.log.Error("invoice.rc.confirm_allowance_failed",
					zap.Int64("operation_id", op.ID), zap.Error(err))
			}
			stillWaiting++
			continue
		}
		probe, err := u.probeOperation(ctx, op)
		if err != nil {
			u.log.Warn("invoice.rc.probe_failed", zap.Int64("operation_id", op.ID), zap.Error(err))
		}
		d := decideRecheck(op, probe, u.now())
		if err := u.applyDecision(ctx, op, d); err != nil {
			u.log.Error("invoice.rc.apply_failed", zap.Int64("operation_id", op.ID), zap.Error(err))
			continue
		}
		switch d.Kind {
		case RecheckResolved:
			resolved++
		case RecheckStillWaiting:
			stillWaiting++
		case RecheckUnresolvedTerminal:
			terminal++
		}
	}
	u.log.Info("invoice.rc.operations_scanned",
		zap.Int("due", len(due)), zap.Int("claimed", claimed),
		zap.Int("resolved", resolved), zap.Int("still_waiting", stillWaiting), zap.Int("terminal", terminal))
	return nil
}

func (u *Recheck) probeOperation(ctx context.Context, op domain.Operation) (RecheckProbe, error) {
	var out RecheckProbe
	doc, err := u.docs.GetByID(ctx, op.DocumentID)
	if err != nil {
		return out, err
	}
	if doc.InvoiceDateTaipei != nil {
		out.WithinProviderRetention = u.now().Sub(*doc.InvoiceDateTaipei) <= providerRetentionWindow
	}

	acct, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, derefOr(doc.Provider))
	if err != nil {
		return out, err
	}
	var queryErr error
	var detail *amego.InvoiceDetail
	if err := u.accounts.WithDecryptedCredential(ctx, acct.ID, func(providerTaxID, appKey string) error {
		detail, queryErr = u.amego.QueryInvoiceByOrderID(ctx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey},
			derefOr(doc.SourceRef))
		return nil
	}); err != nil {
		return out, err
	}

	if queryErr == nil && detail != nil {
		out.Found = true
		out.InvoiceNumber = detail.InvoiceNumber
		return out, nil
	}
	var apiErr *amego.APIError
	if errors.As(queryErr, &apiErr) {
		code := apiErr.Code
		msg := apiErr.Msg
		out.ProviderCode = &code
		out.ProviderMessage = &msg
		out.NotIssued = apiErr.NotFound
	}
	return out, queryErr
}

func (u *Recheck) applyDecision(ctx context.Context, op domain.Operation, d RecheckDecision) error {
	now := u.now()
	var ok bool
	var err error
	switch d.Kind {
	case RecheckResolved:
		completed := now
		ok, err = u.ops.MarkRecheckResolved(ctx, domain.MarkRecheckResult{
			ID: op.ID, NextStatus: d.NextStatus, NextAttemptAt: d.NextAttemptAt,
			CompletedAt:        &completed,
			CancelEvidenceKind: d.CancelEvidenceKind,
			CancelConfirmedAt:  d.CancelConfirmedAt,
			Now:                now,
		})
	case RecheckUnresolvedTerminal:
		ok, err = u.ops.MarkRecheckUnresolved(ctx, domain.MarkRecheckResult{
			ID: op.ID, NextStatus: d.NextStatus, NextAttemptAt: d.NextAttemptAt, Now: now,
		})
	default:
		ok, err = u.ops.UpdateRecheckAttempt(ctx, domain.UpdateRecheckAttempt{
			ID: op.ID, NextStatus: d.NextStatus, NextAttemptAt: d.NextAttemptAt,
			RecheckListPage: op.RecheckListPage, Now: now,
		})
	}
	if err != nil || !ok {
		return err
	}
	return u.recordTransition(ctx, op.ID, op.Status, d.NextStatus, now)
}

func (u *Recheck) recordTransition(ctx context.Context, id int64, from, to string, now time.Time) error {
	kind, err := fsm.CheckOperation(from, to)
	if err != nil {
		u.log.Error("invoice.rc.illegal_transition",
			zap.Int64("operation_id", id), zap.String("from", from), zap.String("to", to), zap.Error(err))
		return err
	}
	if kind == fsm.TransitionSelfLoop {
		return nil
	}
	if u.audit == nil {
		return nil
	}
	return appendStatusHistory(ctx, u.audit, subjectOperation, id, from, to, recheckReasonCode, "", now)
}

const recheckReasonCode = "INV_RECHECK"

const recheckClaimReasonCode = "INV_RECHECK_CLAIMED"

func (u *Recheck) recordClaimTransition(ctx context.Context, id int64, from string, now time.Time) error {
	if from == codemeta.InvoiceOperationStatusWaitingConfirmation.Value {
		return nil
	}
	to := codemeta.InvoiceOperationStatusProviderStatusRecheck.Value
	kind, err := fsm.CheckOperation(from, to)
	if err != nil {
		u.log.Error("invoice.rc.illegal_claim_transition",
			zap.Int64("operation_id", id), zap.String("from", from), zap.Error(err))
		return err
	}
	if kind == fsm.TransitionSelfLoop {
		return nil
	}
	if u.audit == nil {
		return nil
	}
	return appendStatusHistory(ctx, u.audit, subjectOperation, id, from, to,
		recheckClaimReasonCode, "", now)
}

var uploadTerminalStatuses = map[int16]bool{99: true, 91: true, -1: true}

const uploadRecheckMaxAttempts = 48

func uploadNextAt(doc domain.Document, status *int16, probeErr error, now time.Time) *time.Time {
	if status != nil && uploadTerminalStatuses[*status] {
		return nil
	}
	if isPermanentProbeError(probeErr) {
		return nil
	}
	if int(doc.UploadRecheckAttempts)+1 >= uploadRecheckMaxAttempts {
		return nil
	}
	base := now
	if doc.ProviderIssuedAt != nil {
		base = *doc.ProviderIssuedAt
	}
	next := now.Add(recheckBackoff(now.Sub(base)))
	return &next
}

func isPermanentProbeError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *amego.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Class == amego.ClassPermanent
}

func (u *Recheck) recheckUploads(ctx context.Context) error {
	due, err := u.docs.FetchDueUploadRechecks(ctx, u.now(), recheckBatch)
	if err != nil {
		return err
	}
	var claimed, updated int
	for _, doc := range due {
		at := u.now()
		ok, err := u.docs.ClaimUploadRecheck(ctx, doc.ID, at, at.Add(recheckUploadLease))
		if err != nil {
			u.log.Error("invoice.rc.upload_claim_failed", zap.Int64("document_id", doc.ID), zap.Error(err))
			continue
		}
		if !ok {
			continue
		}
		claimed++
		status, probeErr := u.probeUploadStatus(ctx, doc)
		if probeErr != nil {
			u.log.Warn("invoice.rc.upload_probe_failed",
				zap.Int64("document_id", doc.ID),
				zap.Int16("attempts", doc.UploadRecheckAttempts),
				zap.Bool("permanent", isPermanentProbeError(probeErr)),
				zap.Error(probeErr))
		}
		nextAt := uploadNextAt(doc, status, probeErr, u.now())
		if nextAt == nil && status == nil {
			u.log.Error("invoice.rc.upload_recheck_stopped",
				zap.Int64("document_id", doc.ID),
				zap.Int16("attempts", doc.UploadRecheckAttempts),
				zap.Bool("permanent", isPermanentProbeError(probeErr)),
				zap.String("impact", "provider_upload_status 停留在 NULL，需 ADM 人工確認"),
				zap.Error(probeErr))
		}
		if err := u.docs.RecordUploadRecheckResult(ctx, doc.ID, status, u.now(), nextAt); err != nil {
			u.log.Error("invoice.rc.upload_record_failed", zap.Int64("document_id", doc.ID), zap.Error(err))
			continue
		}
		updated++
	}
	u.log.Info("invoice.rc.uploads_scanned",
		zap.Int("due", len(due)), zap.Int("claimed", claimed), zap.Int("updated", updated))
	return nil
}

func (u *Recheck) probeUploadStatus(ctx context.Context, doc domain.Document) (*int16, error) {
	acct, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, derefOr(doc.Provider))
	if err != nil {
		return nil, err
	}
	var detail *amego.InvoiceDetail
	var queryErr error
	if err := u.accounts.WithDecryptedCredential(ctx, acct.ID, func(providerTaxID, appKey string) error {
		detail, queryErr = u.amego.QueryInvoiceByOrderID(ctx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey},
			derefOr(doc.SourceRef))
		return nil
	}); err != nil {
		return nil, err
	}
	if queryErr != nil || detail == nil {
		return nil, queryErr
	}
	s := int16(detail.InvoiceStatus)
	return &s, nil
}
