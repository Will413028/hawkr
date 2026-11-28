package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// CorrectionIntakeDeps 是組裝需要的五個讀取面。
//
// 刻意逐一列出而不是收一個大 Repository：介面越窄，「這一層會碰哪些東西」在型別上
// 就越說得清楚（同 HealthProbe／TrackLister 的理由）。
type CorrectionIntakeDeps struct {
	Documents  domain.DocumentRepository
	Operations domain.OperationRepository
	Accounts   domain.ProviderAccountRepository
	Tracks     TrackLister
	// Corrections 供序列閘門判「這條鏈上有沒有進行中的更正」（#3087）。
	// nil 容忍：既有測試與非 R04 路徑不需要它。
	Corrections domain.CorrectionRequestRepository
	// Preferences 是 R03 推導 carrier_selection_mode／buyer_invoice_preference_id
	// 所需的常用發票資料（#3085）。
	//
	// 🔴 少了它，ReviseSnapshotInput 的四個欄位只能留空，而 order-service 驗
	// invoice_kind 必填 ⇒ R03 一路回 400（包成 500 給買家）。
	// 歸屬驗證在 SQL 述詞裡（identityFromPreference 的註解：Tier A），
	// 這一層不得自己補 `if pref.BuyerID != buyerID`。
	Preferences domain.BuyerPreferenceRepository
}

// CorrectionIntake 組裝並委派。
type CorrectionIntake struct {
	deps       CorrectionIntakeDeps
	correction *Correction
}

// NewCorrectionIntake 建組裝層。
func NewCorrectionIntake(deps CorrectionIntakeDeps, correction *Correction) *CorrectionIntake {
	return &CorrectionIntake{deps: deps, correction: correction}
}

// SubmitCorrectionRequest 是端點 11 的原始輸入（handler 只做 proto→這個型別的轉換）。
type SubmitCorrectionRequest struct {
	BuyerID                 int64
	OrderNo                 string
	ExpectedSnapshotVersion int32
	// NewInvoice 是買家改後的發票資料。沿用 checkout 的型別——那是本服務唯一一份
	// 買家發票輸入的形狀，另立一份會讓兩邊的驗證規則漂移。
	NewInvoice CheckoutInvoiceSpec
	Reason     string
	// IdempotencyKey 一路傳到 ORD.ReviseOrderInvoiceSnapshotInternal 當冪等鍵。
	IdempotencyKey string
}

// SubmitCorrectionOutput 是端點 11 的回應素材。
//
// 🔴 handling_mode 與 status 的值域**零交集**（proto 註解逐字）：前者是「選定的做法」、
// 後者是「做完了沒」。放同一格會讓前端分不出「要作廢重開」與「已作廢重開完成」。
type SubmitCorrectionOutput struct {
	CorrectionID int64
	Route        CorrectionRoute
	Status       string
	HandlingMode string
	RejectReason string
}

const (
	handlingModeUpdateSnapshot = "UPDATE_SNAPSHOT"
	handlingModeVoidAndReissue = "VOID_AND_REISSUE"

	correctionStatusRejected           = "REJECTED"
	correctionStatusUpdatedBeforeIssue = "UPDATED_BEFORE_ISSUE"
	correctionStatusVoidReissue        = "VOID_REISSUE_IN_PROGRESS"
)

// Submit 是端點 11 的入口：組裝 → 委派 → 折成回應素材。
func (u *CorrectionIntake) Submit(ctx context.Context, req SubmitCorrectionRequest) (SubmitCorrectionOutput, error) {
	docs, err := u.deps.Documents.ListOrderDocumentsForBuyer(ctx, req.OrderNo, req.BuyerID)
	if err != nil {
		return SubmitCorrectionOutput{}, err
	}
	if len(docs) == 0 {
		return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(
			bizcode.InvoiceDocumentNotFound, "文件不存在")
	}
	doc := docs[0]

	if doc.OriginalDocumentID != nil && u.deps.Corrections != nil {
		_, err := u.deps.Corrections.GetOpenByDocument(ctx, *doc.OriginalDocumentID)
		switch {
		case err == nil:
			return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(
				bizcode.InvoiceOperationInProgress,
				"這張發票的上一次修改還在處理中，請等處理完成後再提出")
		case !isBizCode(err, bizcode.InvoiceCorrectionNotAllowed):
			return SubmitCorrectionOutput{}, err
		}
	}

	if doc.DocumentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(
			bizcode.InvoiceCorrectionNotAllowed,
			"平台代製收據不適用發票更正流程，請聯繫客服")
	}

	ops, err := u.deps.Operations.ListByDocument(ctx, doc.ID)
	if err != nil {
		return SubmitCorrectionOutput{}, fmt.Errorf("invoice: list operations: %w", err)
	}
	var openOpID int64
	for _, op := range ops {
		if op.Status == codemeta.InvoiceOperationStatusReady.Value ||
			op.Status == codemeta.InvoiceOperationStatusRetryScheduled.Value {
			openOpID = op.ID
			break
		}
	}

	account, err := u.deps.Accounts.GetByIssuer(ctx, doc.IssuerID, derefString(doc.Provider))
	if err != nil {
		return SubmitCorrectionOutput{}, fmt.Errorf("invoice: get provider account: %w", err)
	}

	existing, err := u.deps.Documents.ListLines(ctx, doc.ID)
	if err != nil {
		return SubmitCorrectionOutput{}, fmt.Errorf("invoice: list document lines: %w", err)
	}
	revisedLines := make([]domain.NewDocumentLine, 0, len(existing))
	for _, l := range existing {
		revisedLines = append(revisedLines, domain.NewDocumentLine{
			LineSeq: l.LineSeq, LineType: l.LineType,
			OrderItemID: int64OrZero(l.OrderItemID), SkuID: derefString(l.SkuID),
			Description: l.Description, Unit: derefString(l.Unit),
			Quantity: l.Quantity, UnitPrice: l.UnitPrice, NetAmount: l.NetAmount,
			TaxCategory: l.TaxCategory, TaxRateBps: l.TaxRateBps,
			Remark:            derefString(l.Remark),
			AllocatedDiscount: l.AllocatedDiscount, AllowanceableBase: l.AllowanceableBase,
		})
	}

	var cf CorrectionFacts
	err = u.deps.Accounts.WithDecryptedCredential(ctx, account.ID, func(taxID, appKey string) error {
		var gErr error
		cf, gErr = u.correction.GatherCorrectionFacts(ctx, u.deps.Tracks, CorrectionFactsInput{
			Document:        doc,
			OpenOperationID: openOpID,
			ProviderStatus:  account.Status,
			Credential:      amego.Credential{IssuerID: doc.IssuerID, Invoice: taxID, AppKey: appKey},
		})
		return gErr
	})
	if err != nil {
		return SubmitCorrectionOutput{}, err
	}
	cf.CarrierValid = carrierShapeValid(req.NewInvoice)

	id, rej, err := resolveInvoiceIdentity(ctx, u.deps.Preferences, req.BuyerID, req.NewInvoice)
	if err != nil {
		return SubmitCorrectionOutput{}, err
	}
	if !rej.ok() {
		return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(rej.Code, rej.Message)
	}
	if err := u.openCarrierValueInto(ctx, req.BuyerID, &id); err != nil {
		return SubmitCorrectionOutput{}, err
	}
	if code := validateIdentityShape(id); code != 0 {
		return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(code, bizcode.Message(code))
	}
	if utf8.RuneCountInString(req.NewInvoice.BuyerName) > buyerNameMaxRunes {
		return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(
			bizcode.InvoiceCarrierInvalid, "發票抬頭不得超過 60 字")
	}

	revisedKind := invoiceKindOf(id.BuyerTaxID)
	revisedTaxID := recipientTaxID(revisedKind, id.BuyerTaxID)
	revisedCarrierType, revisedCarrierValue, revisedDonationCode := revisedCarrierFromIdentity(revisedKind, id)
	revisedHeader := domain.BuildHeader(reissueHeaderLines(revisedLines), revisedTaxID)
	var revisedDeadline *time.Time
	if doc.PaidAt != nil {
		if d, ok := domain.StatutoryDeadline(domain.InvoiceDateTaipei(*doc.PaidAt), revisedKind); ok {
			revisedDeadline = &d
		}
	}

	out, err := u.correction.Submit(ctx, SubmitCorrectionInput{
		DocumentID:              doc.ID,
		BuyerID:                 req.BuyerID,
		ExpectedSnapshotVersion: req.ExpectedSnapshotVersion,
		BeforePayload:           maskedCorrectionPayloadFromDoc(doc),
		AfterPayload:            maskedCorrectionPayloadFromSpec(req.NewInvoice),
		OperationID:             openOpID,
		IssueAttemptSeq:         doc.IssueAttemptSeq,
		OrderNo:                 req.OrderNo,
		Facts:                   cf,
		RevisedLines:            revisedLines,
		Revised: domain.ReviseDocumentSnapshot{
			ExpectedVersion: doc.Version,
			RecipientType:   doc.RecipientType,
			RecipientRefID:  doc.RecipientRefID,
			RecipientName:   req.NewInvoice.BuyerName,
			RecipientTaxID:  revisedTaxID,
			RecipientEmail:  req.NewInvoice.Email,

			CarrierType:  revisedCarrierType,
			CarrierID1:   revisedCarrierValue,
			CarrierID2:   revisedCarrierValue,
			DonationCode: revisedDonationCode,

			DocumentKind:       revisedKind,
			SalesAmount:        revisedHeader.SalesAmount,
			FreeTaxSalesAmount: revisedHeader.FreeTaxSalesAmount,
			ZeroTaxSalesAmount: revisedHeader.ZeroTaxSalesAmount,
			TaxAmount:          revisedHeader.TaxAmount,
			TotalAmount:        revisedHeader.TotalAmount,
			StatutoryDeadline:  revisedDeadline,
		},
		OrderRevision: domain.ReviseSnapshotInput{
			OrderNo:                  req.OrderNo,
			ExpectedSnapshotVersion:  int64(req.ExpectedSnapshotVersion),
			InvoiceKind:              invoiceKindOf(id.BuyerTaxID),
			CarrierSelectionMode:     id.SelectionMode,
			BuyerInvoicePreferenceID: id.PreferenceID,
			BuyerTaxID:               id.BuyerTaxID,
			BuyerTitle:               id.BuyerName,
			DonationCode:             id.DonationCode,
			IdempotencyKey:           req.IdempotencyKey,
		},
	})
	if err != nil {
		return SubmitCorrectionOutput{}, err
	}
	return toSubmitOutput(out), nil
}

func toSubmitOutput(r ExecuteCorrectionResult) SubmitCorrectionOutput {
	switch r.Route {
	case RouteUpdateBeforeIssue:
		return SubmitCorrectionOutput{CorrectionID: r.CorrectionID, Route: r.Route,
			Status: correctionStatusUpdatedBeforeIssue, HandlingMode: handlingModeUpdateSnapshot}
	case RouteVoidAndReissue:
		return SubmitCorrectionOutput{CorrectionID: r.CorrectionID, Route: r.Route,
			Status: correctionStatusVoidReissue, HandlingMode: handlingModeVoidAndReissue}
	default:
		return SubmitCorrectionOutput{CorrectionID: r.CorrectionID, Route: r.Route,
			Status: correctionStatusRejected, RejectReason: r.RejectReason}
	}
}

func carrierShapeValid(spec CheckoutInvoiceSpec) bool {
	filled := 0
	if spec.BuyerTaxID != "" {
		filled++
	}
	if spec.ProfileID != 0 || spec.CarrierType != "" {
		filled++
	}
	if spec.CarrierType == codemeta.InvoiceCarrierTypeTaxId.Value && spec.BuyerTaxID != "" && spec.ProfileID == 0 {
		filled--
	}
	return filled <= 1
}

func maskedCorrectionPayloadFromDoc(d domain.Document) []byte {
	return marshalMasked(d.RecipientTaxID, d.RecipientName, derefString(d.CarrierType), derefString(d.DonationCode), "")
}

func maskedCorrectionPayloadFromSpec(s CheckoutInvoiceSpec) []byte {
	return marshalMasked(s.BuyerTaxID, s.BuyerName, s.CarrierType, "", s.Email)
}

func marshalMasked(taxID, name, carrierType, donation, email string) []byte {
	b, _ := json.Marshal(map[string]string{
		"buyerTaxId":   taxID,
		"buyerName":    name,
		"carrierType":  carrierType,
		"donationCode": donation,
		"emailMasked":  maskEmail(email),
	})
	return b
}

func int64OrZero(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func maskEmail(s string) string {
	at := -1
	for i, r := range s {
		if r == '@' {
			at = i
			break
		}
	}
	if at <= 0 {
		return ""
	}
	return s[:1] + "***" + s[at:]
}

func revisedCarrierFromIdentity(documentKind string, id checkoutIdentity) (carrierType *string, carrierValue string, donationCode *string) {
	if documentKind == codemeta.InvoiceDocumentKindB2b.Value {
		return nil, "", nil
	}
	if kind, ok := codemeta.InvoiceCarrierTypeFromValue(id.CarrierType); ok {
		if amego, ok := domain.CarrierTypeToAmego(kind); ok && amego != "" && id.CarrierValue != "" {
			carrierType = &amego
			carrierValue = id.CarrierValue
		}
	}
	if id.DonationCode != "" {
		d := id.DonationCode
		donationCode = &d
	}
	return carrierType, carrierValue, donationCode
}

func (u *CorrectionIntake) openCarrierValueInto(ctx context.Context, buyerID int64, id *checkoutIdentity) error {
	if id.PreferenceID == 0 || id.CarrierValue != "" || !id.HasCarrier {
		return nil
	}
	if u.deps.Preferences == nil {
		return fmt.Errorf("invoice: preferences repository unavailable (preference=%d)", id.PreferenceID)
	}
	var plain string
	if err := u.deps.Preferences.WithDecryptedCarrierValue(ctx, id.PreferenceID, buyerID,
		func(_, v string) error { plain = v; return nil }); err != nil {
		return fmt.Errorf("invoice: open carrier value (preference=%d): %w", id.PreferenceID, err)
	}
	if plain == "" {
		return fmt.Errorf("invoice: preference %d decrypted to an empty carrier value", id.PreferenceID)
	}
	id.CarrierValue = plain
	return nil
}
