package postgres

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)


// KeyRing 統一持有 master key 的現行版與前一版，依列上的 credential_key_version
// （smallint，不是字串標籤）選 cipher。
//
// 為什麼住在 repository 而不是 config：它唯一的消費者是本檔的 decryptCredential，
// 而 cipherFor 的語意（版本對不上就硬錯、不 try-and-fallback）是加解密收斂點的一部分。
type KeyRing struct {
	Current        *crypto.Cipher // 一定非 nil（Config.Validate ① 保證 master key 非空）
	Prev           *crypto.Cipher // 可能為 nil：非輪替期間就是 nil，解密端必須自己判
	CurrentVersion int16
	PrevVersion    int16 // 0 ＝ 沒有前一版（版本號從 1 起算）
	LookupHMACKey  string
}

func (k *KeyRing) cipherFor(v int16) (*crypto.Cipher, error) {
	switch v {
	case k.CurrentVersion:
		return k.Current, nil
	case k.PrevVersion:
		if k.Prev == nil {
			return nil, fmt.Errorf("row encrypted with retired key version %d", v)
		}
		return k.Prev, nil
	default:
		return nil, fmt.Errorf("unknown credential_key_version %d", v)
	}
}

func normalizeAppKey(appKey string) string { return strings.TrimSpace(appKey) }

func encryptCredential(enc *crypto.Cipher, plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("refusing to encrypt an empty app_key (no credential ⇒ no row)")
	}
	return enc.EncryptToBase64(plaintext)
}

func decryptCredential(enc *crypto.Cipher, stored string) (string, error) {
	plaintext, err := enc.DecryptFromBase64(stored)
	if err != nil {
		return "", errors.New("credential did not decrypt (unencrypted row, corrupted value, or wrong key version)")
	}
	return plaintext, nil
}

func credentialFingerprint(lookupHMACKey, providerTaxID, appKey string) string {
	return crypto.HMACLookup(lookupHMACKey, providerTaxID+"\n"+appKey)
}

func credentialLast4(appKey string) string {
	r := []rune(appKey)
	if len(r) <= 4 {
		return string(r)
	}
	return string(r[len(r)-4:])
}

type credentialMaterial struct {
	ciphertext  string
	keyVersion  int16
	fingerprint string
	last4       string
}

func (k *KeyRing) deriveCredential(providerTaxID, appKey string) (credentialMaterial, error) {
	plain := normalizeAppKey(appKey)
	ciphertext, err := encryptCredential(k.Current, plain)
	if err != nil {
		return credentialMaterial{}, err
	}
	return credentialMaterial{
		ciphertext:  ciphertext,
		keyVersion:  k.CurrentVersion,
		fingerprint: credentialFingerprint(k.LookupHMACKey, providerTaxID, plain),
		last4:       credentialLast4(plain),
	}, nil
}

func (k *KeyRing) openCredential(row gen.InvoiceProviderAccount) (string, error) {
	enc, err := k.cipherFor(row.CredentialKeyVersion)
	if err != nil {
		return "", err
	}
	return decryptCredential(enc, row.CredentialCiphertext)
}


func toProviderAccount(row gen.InvoiceProviderAccount) domain.ProviderAccount {
	return domain.ProviderAccount{
		ID:                   row.ID,
		IssuerID:             row.IssuerID,
		Provider:             row.Provider,
		ProviderTaxID:        row.ProviderTaxID,
		CredentialKeyVersion: row.CredentialKeyVersion,
		CredentialLast4:      row.CredentialLast4,
		Status:               row.Status,
		StatusReason:         row.StatusReason,
		VerifiedAt:           row.VerifiedAt,
		AutomationCutoverAt:  row.AutomationCutoverAt,
		LastHealthCheckedAt:  row.LastHealthCheckedAt,
		HealthCheckNextAt:    row.HealthCheckNextAt.Time,
		BreakerOpenedAt:      row.BreakerOpenedAt,
		BreakerProbeSuccess:  row.BreakerProbeSuccess,
		HasPendingCredential: row.PendingCiphertext != nil,
		PendingLast4:         row.PendingLast4,
		RotationStartedAt:    row.RotationStartedAt,
		Version:              row.Version,
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
		CredentialUpdatedAt:  row.CredentialUpdatedAt,
	}
}

func toProviderAccounts(rows []gen.InvoiceProviderAccount) []domain.ProviderAccount {
	out := make([]domain.ProviderAccount, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProviderAccount(row))
	}
	return out
}


func ts(t time.Time) pgtype.Timestamptz { return pgxutil.TimestamptzParam(t) }

func tp(t time.Time) *time.Time { return &t }

func sp(s string) *string { return &s }

func dateParam(t time.Time) pgtype.Date { return pgxutil.DateParam(&t) }
func toIssuer(row gen.InvoiceIssuer) domain.Issuer {
	return domain.Issuer{
		ID:          row.ID,
		IssuerType:  row.IssuerType,
		IssuerRefID: row.IssuerRefID,
		DisplayName: row.DisplayName,
		Status:      row.Status,
		RetiredAt:   row.RetiredAt,
		Version:     row.Version,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}

func toSellerTaxProfile(row gen.SellerTaxProfile) domain.SellerTaxProfile {
	return domain.SellerTaxProfile{
		ID:                          row.ID,
		IssuerID:                    row.IssuerID,
		ReviewStatus:                row.ReviewStatus,
		ClaimedObligation:           row.ClaimedObligation,
		EntityTaxType:               row.EntityTaxType,
		LegalName:                   row.LegalName,
		TaxID:                       row.TaxID,
		ResponsiblePerson:           row.ResponsiblePerson,
		BusinessAddress:             row.BusinessAddress,
		AttestedByAccountID:         row.AttestedByAccountID,
		AddressCity:                 row.AddressCity,
		AddressDistrict:             row.AddressDistrict,
		DefaultTaxCategory:          row.DefaultTaxCategory,
		EvidenceObjectKeys:          pgxutil.JSONRawMessage(row.EvidenceObjectKeys),
		AttestationVersion:          row.AttestationVersion,
		AttestedAt:                  row.AttestedAt,
		SubmittedAt:                 row.SubmittedAt,
		PendingValidUntil:           row.PendingValidUntil,
		ExtensionCount:              row.ExtensionCount,
		PendingOrderLimitCount:      row.PendingOrderLimitCount,
		PendingOrderLimitAmount:     row.PendingOrderLimitAmount,
		PendingOrderCount:           row.PendingOrderCount,
		PendingOrderAmount:          row.PendingOrderAmount,
		ReviewNote:                  row.ReviewNote,
		ReviewNoteAt:                row.ReviewNoteAt,
		ApprovedAt:                  row.ApprovedAt,
		CooldownUntil:               row.CooldownUntil,
		ApprovalValidUntil:          row.ApprovalValidUntil,
		RecheckReason:               row.RecheckReason,
		ReceiptAuthorizedAt:         row.ReceiptAuthorizedAt,
		ReceiptConsentVersion:       row.ReceiptConsentVersion,
		ReceiptApprovalBasisKey:     row.ReceiptApprovalBasisKey,
		ReceiptApprovalBasisAssetID: row.ReceiptApprovalBasisAssetID,
		ViolationWindowDays:         row.ViolationWindowDays,
		ViolationThreshold:          row.ViolationThreshold,
		SuspensionDays:              row.SuspensionDays,
		Version:                     row.Version,
		CreatedAt:                   row.CreatedAt.Time,
		UpdatedAt:                   row.UpdatedAt.Time,
	}
}

func toSellerTaxProfiles(rows []gen.SellerTaxProfile) []domain.SellerTaxProfile {
	out := make([]domain.SellerTaxProfile, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSellerTaxProfile(row))
	}
	return out
}

func toDocument(row gen.InvoiceDocument) (domain.Document, error) {
	if !domain.TaxCategory(row.TaxType).IsHeaderLevel() {
		return domain.Document{}, fmt.Errorf("invoice_documents.tax_type out of range: %d (document_id=%d)", row.TaxType, row.ID)
	}

	return domain.Document{
		ID:                          row.ID,
		IssuerID:                    row.IssuerID,
		IssuerNameSnapshot:          row.IssuerNameSnapshot,
		IssuerTaxIDSnapshot:         row.IssuerTaxIDSnapshot,
		SourceType:                  row.SourceType,
		SourceRef:                   row.SourceRef,
		TriggerType:                 row.TriggerType,
		RecipientType:               row.RecipientType,
		RecipientRefID:              row.RecipientRefID,
		RecipientName:               row.RecipientName,
		RecipientTaxID:              row.RecipientTaxID,
		RecipientEmailCiphertext:    row.RecipientEmailCiphertext,
		RecipientEmailHash:          row.RecipientEmailHash,
		RecipientPhoneCiphertext:    row.RecipientPhoneCiphertext,
		CarrierType:                 row.CarrierType,
		CarrierId1Ciphertext:        row.CarrierId1Ciphertext,
		CarrierId2Ciphertext:        row.CarrierId2Ciphertext,
		CarrierValueHash:            row.CarrierValueHash,
		DonationCode:                row.DonationCode,
		DocumentKind:                row.DocumentKind,
		FulfillmentMode:             row.FulfillmentMode,
		Status:                      row.Status,
		NotApplicableReason:         row.NotApplicableReason,
		ManualReasonCode:            row.ManualReasonCode,
		CancelReasonCode:            row.CancelReasonCode,
		CancelReason:                row.CancelReason,
		SnapshotVersion:             row.SnapshotVersion,
		IssueAttemptSeq:             row.IssueAttemptSeq,
		ProviderAccountVersion:      row.ProviderAccountVersion,
		SupersededByID:              row.SupersededByID,
		OriginalDocumentID:          row.OriginalDocumentID,
		SalesAmount:                 row.SalesAmount,
		FreeTaxSalesAmount:          row.FreeTaxSalesAmount,
		ZeroTaxSalesAmount:          row.ZeroTaxSalesAmount,
		TaxAmount:                   row.TaxAmount,
		TotalAmount:                 row.TotalAmount,
		TaxType:                     row.TaxType,
		TaxRateBps:                  row.TaxRateBps,
		AllowanceTotalAmount:        row.AllowanceTotalAmount,
		VoidedAt:                    row.VoidedAt,
		Provider:                    row.Provider,
		ProviderRequestNo:           row.ProviderRequestNo,
		ProviderInvoiceNo:           row.ProviderInvoiceNo,
		ProviderRandomCode:          row.ProviderRandomCode,
		ProviderIssuedAt:            row.ProviderIssuedAt,
		ExternalInvoiceNo:           row.ExternalInvoiceNo,
		ExternalInvoiceDate:         pgxutil.DateValue(row.ExternalInvoiceDate),
		ExternalReportedAt:          row.ExternalReportedAt,
		ProviderUploadStatus:        row.ProviderUploadStatus,
		ProviderUploadCheckedAt:     row.ProviderUploadCheckedAt,
		UploadRecheckNextAt:         row.UploadRecheckNextAt,
		UploadRecheckAttempts:       row.UploadRecheckAttempts,
		ReceiptNo:                   row.ReceiptNo,
		ReceiptAuthorizationVersion: row.ReceiptAuthorizationVersion,
		PdfObjectKey:                row.PdfObjectKey,
		PdfFetchNextAt:              row.PdfFetchNextAt,
		PdfFetchAttempts:            row.PdfFetchAttempts,
		InvoiceDateTaipei:           pgxutil.DateValue(row.InvoiceDateTaipei),
		PaidAt:                      row.PaidAt,
		TaxPeriodCode:               row.TaxPeriodCode,
		IssueNotBefore:              row.IssueNotBefore.Time,
		StatutoryDeadlineAt:         row.StatutoryDeadlineAt,
		SourceSnapshot:              pgxutil.JSONRawMessage(row.SourceSnapshot),
		NotificationResentAt:        row.NotificationResentAt,
		NotificationResendCount:     row.NotificationResendCount,
		Version:                     row.Version,
		CreatedAt:                   row.CreatedAt.Time,
		UpdatedAt:                   row.UpdatedAt.Time,
	}, nil
}

func toDocuments(rows []gen.InvoiceDocument) ([]domain.Document, error) {
	out := make([]domain.Document, 0, len(rows))
	for _, row := range rows {
		v, err := toDocument(row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func toDocumentLine(row gen.InvoiceDocumentLine) (domain.DocumentLine, error) {
	if _, ok := domain.TaxCategoryFromDB(row.TaxCategory); !ok {
		return domain.DocumentLine{}, fmt.Errorf("invoice_document_lines.tax_category has no CodeMeta entry: %d (line_id=%d)", row.TaxCategory, row.ID)
	}

	return domain.DocumentLine{
		ID:                row.ID,
		DocumentID:        row.DocumentID,
		LineSeq:           row.LineSeq,
		LineType:          row.LineType,
		OrderItemID:       row.OrderItemID,
		SkuID:             uuidPtrString(row.SkuID),
		Description:       row.Description,
		Unit:              row.Unit,
		Quantity:          row.Quantity,
		UnitPrice:         row.UnitPrice,
		NetAmount:         row.NetAmount,
		TaxCategory:       row.TaxCategory,
		TaxRateBps:        row.TaxRateBps,
		Remark:            row.Remark,
		AllocatedDiscount: row.AllocatedDiscount,
		AllowanceableBase: row.AllowanceableBase,
		AllowancedAmount:  row.AllowancedAmount,
		CreatedAt:         row.CreatedAt.Time,
	}, nil
}

func toDocumentLines(rows []gen.InvoiceDocumentLine) ([]domain.DocumentLine, error) {
	out := make([]domain.DocumentLine, 0, len(rows))
	for _, row := range rows {
		v, err := toDocumentLine(row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func toOperation(row gen.InvoiceOperation) domain.Operation {
	return domain.Operation{
		ID:                  row.ID,
		DocumentID:          row.DocumentID,
		AdjustmentID:        row.AdjustmentID,
		OperationType:       row.OperationType,
		OperationKey:        row.OperationKey,
		ProviderRequestNo:   row.ProviderRequestNo,
		Status:              row.Status,
		RequestSnapshotHash: row.RequestSnapshotHash,
		AttemptCount:        row.AttemptCount,
		MaxAttempts:         row.MaxAttempts,
		NextAttemptAt:       row.NextAttemptAt.Time,
		HeldUntil:           row.HeldUntil,
		RecheckListPage:     row.RecheckListPage,
		StatutoryDeadlineAt: row.StatutoryDeadlineAt,
		ProviderCode:        row.ProviderCode,
		ProviderMessage:     row.ProviderMessage,
		UnknownResult:       row.UnknownResult,
		UnknownSince:        row.UnknownSince,
		CancelRequestedBy:   row.CancelRequestedBy,
		CancelRequestedAt:   row.CancelRequestedAt,
		CancelConfirmedBy:   row.CancelConfirmedBy,
		CancelConfirmedAt:   row.CancelConfirmedAt,
		CancelReasonCode:    row.CancelReasonCode,
		CancelReason:        row.CancelReason,
		StartedAt:           row.StartedAt,
		CompletedAt:         row.CompletedAt,
		LastError:           row.LastError,
		Version:             row.Version,
		CreatedAt:           row.CreatedAt.Time,
		UpdatedAt:           row.UpdatedAt.Time,
	}
}

func toOperations(rows []gen.InvoiceOperation) []domain.Operation {
	out := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toOperation(row))
	}
	return out
}

func toAdjustment(row gen.InvoiceAdjustment) domain.Adjustment {
	return domain.Adjustment{
		ID:                      row.ID,
		DocumentID:              row.DocumentID,
		AdjustmentType:          row.AdjustmentType,
		Status:                  row.Status,
		TriggerKind:             row.TriggerKind,
		RefundNo:                row.RefundNo,
		CorrectionRequestID:     row.CorrectionRequestID,
		AllowanceNumber:         row.AllowanceNumber,
		AllowanceDateTaipei:     pgxutil.DateValue(row.AllowanceDateTaipei),
		NetAmountExTax:          row.NetAmountExTax,
		TaxAmount:               row.TaxAmount,
		GrossAmount:             row.GrossAmount,
		VoidReasonCode:          row.VoidReasonCode,
		VoidReasonDetail:        row.VoidReasonDetail,
		BlockedReasonCode:       row.BlockedReasonCode,
		CancelReasonCode:        row.CancelReasonCode,
		ProviderConfirmedAt:     row.ProviderConfirmedAt,
		ProviderUploadStatus:    row.ProviderUploadStatus,
		ProviderUploadCheckedAt: row.ProviderUploadCheckedAt,
		PdfObjectKey:            row.PdfObjectKey,
		Version:                 row.Version,
		CreatedAt:               row.CreatedAt.Time,
		UpdatedAt:               row.UpdatedAt.Time,
	}
}

func toAdjustments(rows []gen.InvoiceAdjustment) []domain.Adjustment {
	out := make([]domain.Adjustment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAdjustment(row))
	}
	return out
}

func toAdjustmentLine(row gen.InvoiceAdjustmentLine) (domain.AdjustmentLine, error) {
	if _, ok := domain.TaxCategoryFromDB(row.TaxCategory); !ok {
		return domain.AdjustmentLine{}, fmt.Errorf("invoice_adjustment_lines.tax_category has no CodeMeta entry: %d (line_id=%d)", row.TaxCategory, row.ID)
	}

	return domain.AdjustmentLine{
		ID:                  row.ID,
		AdjustmentID:        row.AdjustmentID,
		LineSeq:             row.LineSeq,
		SourceLineID:        row.SourceLineID,
		OriginalInvoiceNo:   row.OriginalInvoiceNo,
		OriginalInvoiceDate: pgxutil.DateValue(row.OriginalInvoiceDate),
		OriginalDescription: row.OriginalDescription,
		Quantity:            row.Quantity,
		UnitPriceExTax:      row.UnitPriceExTax,
		NetAmountExTax:      row.NetAmountExTax,
		TaxAmount:           row.TaxAmount,
		TaxCategory:         row.TaxCategory,
		CreatedAt:           row.CreatedAt.Time,
	}, nil
}

func toAdjustmentLines(rows []gen.InvoiceAdjustmentLine) ([]domain.AdjustmentLine, error) {
	out := make([]domain.AdjustmentLine, 0, len(rows))
	for _, row := range rows {
		v, err := toAdjustmentLine(row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func toComplaint(row gen.InvoiceComplaint) domain.Complaint {
	return domain.Complaint{
		ID:                 row.ID,
		DocumentID:         row.DocumentID,
		IssuerID:           row.IssuerID,
		OrderNo:            row.OrderNo,
		ComplainantBuyerID: row.ComplainantBuyerID,
		ReasonCode:         row.ReasonCode,
		Description:        row.Description,
		Status:             row.Status,
		SlaDueAt:           row.SlaDueAt.Time,
		OverdueAt:          row.OverdueAt,
		BuyerReviewDueAt:   row.BuyerReviewDueAt,
		FirstRespondedAt:   row.FirstRespondedAt,
		ResolvedAt:         row.ResolvedAt,
		ResolutionCode:     row.ResolutionCode,
		ResolutionNote:     row.ResolutionNote,
		ReopenCount:        row.ReopenCount,
		ReopenAllowedAfter: row.ReopenAllowedAfter,
		DecidedByAccountID: row.DecidedByAccountID,
		DecidedAt:          row.DecidedAt,
		Version:            row.Version,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}
}

func toComplaints(rows []gen.InvoiceComplaint) []domain.Complaint {
	out := make([]domain.Complaint, 0, len(rows))
	for _, row := range rows {
		out = append(out, toComplaint(row))
	}
	return out
}

func toComplaintEvent(row gen.InvoiceComplaintEvent) domain.ComplaintEvent {
	return domain.ComplaintEvent{
		ID:                   row.ID,
		ComplaintID:          row.ComplaintID,
		EventType:            row.EventType,
		Action:               row.Action,
		ActorType:            row.ActorType,
		ActorAccountID:       row.ActorAccountID,
		Message:              row.Message,
		AttachmentObjectKeys: pgxutil.JSONRawMessage(row.AttachmentObjectKeys),
		OccurredAt:           row.OccurredAt.Time,
	}
}

func toComplaintEvents(rows []gen.InvoiceComplaintEvent) []domain.ComplaintEvent {
	out := make([]domain.ComplaintEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toComplaintEvent(row))
	}
	return out
}

func toCorrectionRequest(row gen.InvoiceCorrectionRequest) domain.CorrectionRequest {
	return domain.CorrectionRequest{
		ID:                      row.ID,
		DocumentID:              row.DocumentID,
		RequestedByBuyerID:      row.RequestedByBuyerID,
		ExpectedSnapshotVersion: row.ExpectedSnapshotVersion,
		BeforePayload:           pgxutil.JSONRawMessage(row.BeforePayload),
		AfterPayload:            pgxutil.JSONRawMessage(row.AfterPayload),
		Status:                  row.Status,
		HandlingMode:            row.HandlingMode,
		RejectReasonCode:        row.RejectReasonCode,
		RejectReasonDetail:      row.RejectReasonDetail,
		VoidAdjustmentID:        row.VoidAdjustmentID,
		ReissuedDocumentID:      row.ReissuedDocumentID,
		Version:                 row.Version,
		CreatedAt:               row.CreatedAt.Time,
		UpdatedAt:               row.UpdatedAt.Time,
		CompletedAt:             row.CompletedAt,
	}
}

func toBuyerPreference(row gen.BuyerInvoicePreference) domain.BuyerPreference {
	return domain.BuyerPreference{
		ID:                     row.ID,
		BuyerID:                row.BuyerID,
		PreferenceKind:         row.PreferenceKind,
		CarrierType:            row.CarrierType,
		CarrierValueCiphertext: row.CarrierValueCiphertext,
		CarrierValueHash:       row.CarrierValueHash,
		CarrierValueMask:       row.CarrierValueMask,
		TaxID:                  row.TaxID,
		TaxTitle:               row.TaxTitle,
		DonationCode:           row.DonationCode,
		Label:                  row.Label,
		IsDefault:              row.IsDefault,
		SupersededAt:           row.SupersededAt,
		SupersededByID:         row.SupersededByID,
		DeletedAt:              row.DeletedAt,
		CreatedAt:              row.CreatedAt.Time,
		UpdatedAt:              row.UpdatedAt.Time,
	}
}

func toInboxEvent(row gen.InvoiceInboxEvent) domain.InboxEvent {
	return domain.InboxEvent{
		ID:                row.ID,
		EventID:           row.EventID,
		EventType:         row.EventType,
		RoutingKey:        row.RoutingKey,
		Exchange:          row.Exchange,
		BusinessDedupKey:  row.BusinessDedupKey,
		AggregateRef:      row.AggregateRef,
		Payload:           pgxutil.JSONRawMessage(row.Payload),
		Status:            row.Status,
		Disposition:       row.Disposition,
		DispositionReason: row.DispositionReason,
		AttemptCount:      row.AttemptCount,
		NextAttemptAt:     row.NextAttemptAt.Time,
		DeferredUntil:     row.DeferredUntil,
		LastError:         row.LastError,
		OccurredAt:        row.OccurredAt.Time,
		ReceivedAt:        row.ReceivedAt.Time,
		ProcessedAt:       row.ProcessedAt,
		PayloadPurgedAt:   row.PayloadPurgedAt,
	}
}

func toInboxEvents(rows []gen.InvoiceInboxEvent) []domain.InboxEvent {
	out := make([]domain.InboxEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInboxEvent(row))
	}
	return out
}

func toOutboxEvent(row gen.InvoiceOutboxEvent) domain.OutboxEvent {
	return domain.OutboxEvent{
		ID:            row.ID,
		AggregateType: row.AggregateType,
		AggregateID:   row.AggregateID,
		EventType:     row.EventType,
		Exchange:      row.Exchange,
		RoutingKey:    row.RoutingKey,
		Payload:       pgxutil.JSONRawMessage(row.Payload),
		Status:        row.Status,
		AttemptCount:  row.AttemptCount,
		NextAttemptAt: row.NextAttemptAt.Time,
		LastError:     row.LastError,
		CreatedAt:     row.CreatedAt.Time,
		CompletedAt:   row.CompletedAt,
	}
}

func toOutboxEvents(rows []gen.InvoiceOutboxEvent) []domain.OutboxEvent {
	out := make([]domain.OutboxEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toOutboxEvent(row))
	}
	return out
}

func toStatusHistoryEntry(row gen.InvoiceStatusHistory) domain.StatusHistoryEntry {
	return domain.StatusHistoryEntry{
		ID:             row.ID,
		SubjectType:    row.SubjectType,
		SubjectID:      row.SubjectID,
		FromStatus:     row.FromStatus,
		ToStatus:       row.ToStatus,
		ReasonCode:     row.ReasonCode,
		ReasonDetail:   row.ReasonDetail,
		ActorType:      row.ActorType,
		ActorAccountID: row.ActorAccountID,
		CorrelationID:  row.CorrelationID,
		OccurredAt:     row.OccurredAt.Time,
	}
}

func toStatusHistoryEntrys(rows []gen.InvoiceStatusHistory) []domain.StatusHistoryEntry {
	out := make([]domain.StatusHistoryEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toStatusHistoryEntry(row))
	}
	return out
}

func toProviderAuditLogEntry(row gen.InvoiceProviderAuditLog) domain.ProviderAuditLogEntry {
	return domain.ProviderAuditLogEntry{
		ID:              row.ID,
		OperationID:     row.OperationID,
		IssuerID:        row.IssuerID,
		Provider:        row.Provider,
		Endpoint:        row.Endpoint,
		AttemptSeq:      row.AttemptSeq,
		RequestMasked:   pgxutil.JSONRawMessage(row.RequestMasked),
		ResponseMasked:  pgxutil.JSONRawMessage(row.ResponseMasked),
		RawBodyHash:     row.RawBodyHash,
		HttpStatus:      row.HttpStatus,
		ProviderCode:    row.ProviderCode,
		ProviderMessage: row.ProviderMessage,
		LatencyMs:       row.LatencyMs,
		ErrorKind:       row.ErrorKind,
		CorrelationID:   row.CorrelationID,
		OccurredAt:      row.OccurredAt.Time,
	}
}

func toProviderAuditLogEntrys(rows []gen.InvoiceProviderAuditLog) []domain.ProviderAuditLogEntry {
	out := make([]domain.ProviderAuditLogEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProviderAuditLogEntry(row))
	}
	return out
}

func toAdminAuditLogEntry(row gen.InvoiceAdminAuditLog) domain.AdminAuditLogEntry {
	return domain.AdminAuditLogEntry{
		ID:                     row.ID,
		ActorAccountID:         row.ActorAccountID,
		Action:                 row.Action,
		ResourceType:           row.ResourceType,
		ResourceID:             row.ResourceID,
		BeforeState:            pgxutil.JSONRawMessage(row.BeforeState),
		AfterState:             pgxutil.JSONRawMessage(row.AfterState),
		ReasonCode:             row.ReasonCode,
		ReasonDetail:           row.ReasonDetail,
		ApprovalActorAccountID: row.ApprovalActorAccountID,
		TraceID:                row.TraceID,
		RequestID:              row.RequestID,
		CorrelationID:          row.CorrelationID,
		ClientIp:               row.ClientIp,
		UserAgent:              row.UserAgent,
		OccurredAt:             row.OccurredAt.Time,
	}
}

func toAdminAuditLogEntrys(rows []gen.InvoiceAdminAuditLog) []domain.AdminAuditLogEntry {
	out := make([]domain.AdminAuditLogEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAdminAuditLogEntry(row))
	}
	return out
}


func (k *KeyRing) encryptOptional(plaintext string) (*string, error) {
	if plaintext == "" {
		return nil, nil
	}
	ct, err := encryptCredential(k.Current, plaintext)
	if err != nil {
		return nil, err
	}
	return &ct, nil
}

func (k *KeyRing) hashOptional(plaintext string) *string {
	if plaintext == "" {
		return nil
	}
	h := crypto.HMACLookup(k.LookupHMACKey, plaintext)
	return &h
}

func (k *KeyRing) decryptOptional(stored *string) (string, error) {
	if stored == nil || *stored == "" {
		return "", nil
	}
	plain, err := decryptCredential(k.Current, *stored)
	if err == nil {
		return plain, nil
	}
	if k.Prev == nil {
		return "", err
	}
	plain, prevErr := decryptCredential(k.Prev, *stored)
	if prevErr != nil {
		return "", fmt.Errorf("invoice: decrypt with current and prev master key both failed: %w", err)
	}
	return plain, nil
}

func (k *KeyRing) openRecipient(row gen.InvoiceDocument) (domain.RecipientSecrets, error) {
	var out domain.RecipientSecrets
	var err error
	if out.Email, err = k.decryptOptional(row.RecipientEmailCiphertext); err != nil {
		return domain.RecipientSecrets{}, err
	}
	if out.Phone, err = k.decryptOptional(row.RecipientPhoneCiphertext); err != nil {
		return domain.RecipientSecrets{}, err
	}
	if out.CarrierID1, err = k.decryptOptional(row.CarrierId1Ciphertext); err != nil {
		return domain.RecipientSecrets{}, err
	}
	if out.CarrierID2, err = k.decryptOptional(row.CarrierId2Ciphertext); err != nil {
		return domain.RecipientSecrets{}, err
	}
	return out, nil
}

func nzp(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func snp(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func uuidPtrString(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	s := u.String()
	return &s
}

func inp(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
