package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

func isBizCode(err error, code int32) bool {
	var de *pkgerrors.DomainError
	return errors.As(err, &de) && int32(de.Code) == code
}

func taxResolverFor(profile domain.SellerTaxProfile) (domain.TaxCategoryResolver, error) {
	cat := domain.TaxCategoryTaxable
	if profile.DefaultTaxCategory != 0 {
		if _, ok := domain.TaxCategoryFromDB(profile.DefaultTaxCategory); !ok {
			return nil, &domain.InvariantError{
				Code:   domain.InvariantCode(codemeta.InvoiceManualReasonMixedTaxCategory.Value),
				Detail: "seller_tax_profiles.default_tax_category has no CodeMeta entry: " + strconv.Itoa(int(profile.DefaultTaxCategory)),
			}
		}
		cat = domain.TaxCategory(profile.DefaultTaxCategory)
	}
	return domain.SellerDefaultTaxResolver{Default: cat}, nil
}

func creationStatus(d domain.FulfillmentDecision) string {
	if d.DocumentStatus == codemeta.InvoiceDocumentStatusNotApplicable.Value {
		return codemeta.InvoiceDocumentStatusNotApplicable.Value
	}
	return codemeta.InvoiceDocumentStatusPending.Value
}

func headerTaxType(h domain.Header) domain.TaxCategory {
	if h.TaxType == 0 {
		return domain.TaxCategoryTaxable
	}
	return h.TaxType
}

func recipientName(documentKind, buyerTitle string) string {
	if documentKind == codemeta.InvoiceDocumentKindB2b.Value && buyerTitle != "" {
		return truncateRunes(buyerTitle, 60)
	}
	return "消費者"
}

func recipientTaxID(documentKind, buyerTaxID string) string {
	if documentKind == codemeta.InvoiceDocumentKindB2b.Value && domain.IsBusinessTaxID(buyerTaxID) {
		return buyerTaxID
	}
	return domain.B2CBuyerIdentifier
}

func normalizeBuyerIdentity(documentKind string, draft *domain.IssueDraft, donationCode string) (carrierType, carrierID1, carrierID2, donation string) {
	if draft == nil {
		return "", "", "", ""
	}
	if documentKind == codemeta.InvoiceDocumentKindB2b.Value {
		return "", "", "", ""
	}
	if draft.CarrierType != domain.AmegoCarrierNone && draft.CarrierValue != "" {
		return draft.CarrierType, draft.CarrierValue, draft.CarrierValue, ""
	}
	return "", "", "", donationCode
}

func issuerNameSnapshot(issuer domain.Issuer, profile domain.SellerTaxProfile) string {
	if profile.LegalName != nil && *profile.LegalName != "" {
		return truncateRunes(*profile.LegalName, 120)
	}
	return truncateRunes(issuer.DisplayName, 120)
}

func providerAccountVersion(a domain.ProviderAccount) *int64 {
	if a.ID == 0 {
		return nil
	}
	v := a.Version
	return &v
}

func toNewLines(lines []domain.Line) []domain.NewDocumentLine {
	out := make([]domain.NewDocumentLine, 0, len(lines))
	for _, l := range lines {
		out = append(out, domain.NewDocumentLine{
			LineSeq:           l.LineSeq,
			LineType:          l.LineType,
			OrderItemID:       l.OrderItemID,
			SkuID:             l.SkuID,
			Description:       l.Description,
			Quantity:          l.Quantity,
			UnitPrice:         l.UnitPrice,
			NetAmount:         l.NetAmount,
			TaxCategory:       int16(l.TaxCategory),
			TaxRateBps:        domain.TaxRateBps,
			AllocatedDiscount: l.AllocatedDiscount,
			AllowanceableBase: l.AllowanceableBase,
		})
	}
	return out
}

type sourceSnapshot struct {
	OrderNo                   string `json:"order_no"`
	SnapshotVersion           int32  `json:"snapshot_version"`
	SellerReviewStatus        string `json:"seller_review_status"`
	ProviderStatus            string `json:"provider_status"`
	ProviderAccountVersion    *int64 `json:"provider_account_version"`
	AutomationCutoverAt       string `json:"automation_cutover_at,omitempty"`
	DefaultTaxCategory        int16  `json:"default_tax_category"`
	FulfillmentMode           string `json:"fulfillment_mode"`
	DocumentKind              string `json:"document_kind"`
	HasBuyerTaxID             bool   `json:"has_buyer_tax_id"`
	HasCarrier                bool   `json:"has_carrier"`
	HasDonation               bool   `json:"has_donation"`
	HasPlatformFundedDiscount bool   `json:"has_platform_funded_discount"`
	IsConsolidatedOrder       bool   `json:"is_consolidated_order"`
	TotalOriginalAmount       int64  `json:"total_original_amount"`
	ShippingFee               int64  `json:"shipping_fee"`
	DiscountAmount            int64  `json:"discount_amount"`
	TotalActualAmount         int64  `json:"total_actual_amount"`
}

// SnapshotAmounts 是 A06 第 3 層的金額拆解（#3031）。
//
// 🔴 來源是 **order_snapshot_json**，不是 invoice_documents 的金額欄：
// 那五欄（sales／freeTax／zeroTax／tax／total）是 **AMEGO 的分解**（依稅別），
// 而 A06 要的是**結帳的分解**（商品／運費／折扣），兩者是同一筆錢的兩種切法。
// 拿前者去湊後者會得出看似合理、實際對不上結帳頁的數字。
type SnapshotAmounts struct {
	ProductAmount  int64
	ShippingAmount int64
	DiscountAmount int64
	InvoiceAmount  int64
	TaxCategory    int16
}

// ParseSnapshotAmounts 從 order_snapshot_json 取出金額拆解。
//
// ok=false ⇒ 快照不存在或解不開 ⇒ 呼叫端不要填那個欄位（回 nil）。
// **不要回一組零值**：A06 會把它畫成「商品 0 元、運費 0 元」，而那是一個看起來
// 完全正常的錯誤畫面。
func ParseSnapshotAmounts(raw json.RawMessage) (SnapshotAmounts, bool) {
	if len(raw) == 0 {
		return SnapshotAmounts{}, false
	}
	var snap sourceSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return SnapshotAmounts{}, false
	}
	if snap.TotalActualAmount == 0 {
		return SnapshotAmounts{}, false
	}
	return SnapshotAmounts{
		ProductAmount:  snap.TotalOriginalAmount,
		ShippingAmount: snap.ShippingFee,
		DiscountAmount: snap.DiscountAmount,
		InvoiceAmount:  snap.TotalActualAmount,
		TaxCategory:    snap.DefaultTaxCategory,
	}, true
}

func marshalSourceSnapshot(
	in OrderPaidInput,
	decision domain.FulfillmentDecision,
	profile domain.SellerTaxProfile,
	account domain.ProviderAccount,
) (json.RawMessage, error) {
	snap := sourceSnapshot{
		OrderNo:                   in.OrderNo,
		SnapshotVersion:           in.SnapshotVersion,
		SellerReviewStatus:        profile.ReviewStatus,
		ProviderStatus:            account.Status,
		ProviderAccountVersion:    providerAccountVersion(account),
		DefaultTaxCategory:        profile.DefaultTaxCategory,
		FulfillmentMode:           decision.FulfillmentMode,
		DocumentKind:              decision.DocumentKind,
		HasBuyerTaxID:             domain.IsBusinessTaxID(in.BuyerTaxID),
		HasCarrier:                in.CarrierID1 != "",
		HasDonation:               in.DonationCode != "",
		HasPlatformFundedDiscount: in.HasPlatformFundedDiscount,
		IsConsolidatedOrder:       in.IsConsolidatedOrder,
		TotalOriginalAmount:       in.Snapshot.TotalOriginalAmount,
		ShippingFee:               in.Snapshot.ShippingFee,
		DiscountAmount:            in.Snapshot.DiscountAmount,
		TotalActualAmount:         in.Snapshot.TotalActualAmount,
	}
	if account.AutomationCutoverAt != nil {
		snap.AutomationCutoverAt = account.AutomationCutoverAt.UTC().Format(time.RFC3339)
	}
	return json.Marshal(snap)
}

func finishOperationSucceeded(
	ctx context.Context,
	ops domain.OperationRepository,
	audit domain.AuditRepository,
	op domain.Operation,
	now time.Time,
) error {
	completed := now
	ok, err := ops.MarkRecheckResolved(ctx, domain.MarkRecheckResult{
		ID:            op.ID,
		NextStatus:    codemeta.InvoiceOperationStatusSucceeded.Value,
		NextAttemptAt: now,
		CompletedAt:   &completed,
		Now:           now,
	})
	if err != nil {
		return err
	}
	from := codemeta.InvoiceOperationStatusProviderStatusRecheck.Value
	if !ok {
		from = codemeta.InvoiceOperationStatusSending.Value
		ok, err = ops.MarkDone(ctx, op.ID, now)
		if err != nil {
			return err
		}
	}
	if !ok {
		return fmt.Errorf(
			"invoice: operation %d could not enter SUCCEEDED (rows=0 on both lanes, snapshot status=%s)",
			op.ID, op.Status)
	}
	return appendStatusHistory(ctx, audit, subjectOperation, op.ID, from,
		codemeta.InvoiceOperationStatusSucceeded.Value, "", "", now)
}

func appendStatusHistory(ctx context.Context, audit domain.AuditRepository, subjectType string, subjectID int64, from, to, reasonCode, correlationID string, now time.Time) error {
	var fromPtr, reasonPtr, corrPtr *string
	if from != "" {
		fromPtr = &from
	}
	if reasonCode != "" {
		reasonPtr = &reasonCode
	}
	if correlationID != "" {
		corrPtr = &correlationID
	}
	return audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
		SubjectType:   subjectType,
		SubjectID:     subjectID,
		FromStatus:    fromPtr,
		ToStatus:      to,
		ReasonCode:    reasonPtr,
		ActorType:     actorTypeSystem,
		CorrelationID: corrPtr,
		Now:           now,
	})
}

func appendStatusHistoryActor(
	ctx context.Context, audit domain.AuditRepository, subjectType string, subjectID int64,
	from, to, reasonCode, reasonDetail, actorType string, actorAccountID *int64, now time.Time,
) error {
	var fromPtr, reasonPtr, detailPtr *string
	if from != "" {
		fromPtr = &from
	}
	if reasonCode != "" {
		reasonPtr = &reasonCode
	}
	if reasonDetail != "" {
		detailPtr = &reasonDetail
	}
	return audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
		SubjectType:    subjectType,
		SubjectID:      subjectID,
		FromStatus:     fromPtr,
		ToStatus:       to,
		ReasonCode:     reasonPtr,
		ReasonDetail:   detailPtr,
		ActorType:      actorType,
		ActorAccountID: actorAccountID,
		Now:            now,
	})
}

func appendStatusHistoryDetail(
	ctx context.Context, audit domain.AuditRepository, subjectType string, subjectID int64,
	from, to, reasonCode, reasonDetail, correlationID string, now time.Time,
) error {
	var fromPtr, reasonPtr, detailPtr, corrPtr *string
	if from != "" {
		fromPtr = &from
	}
	if reasonCode != "" {
		reasonPtr = &reasonCode
	}
	if reasonDetail != "" {
		detailPtr = &reasonDetail
	}
	if correlationID != "" {
		corrPtr = &correlationID
	}
	return audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
		SubjectType:   subjectType,
		SubjectID:     subjectID,
		FromStatus:    fromPtr,
		ToStatus:      to,
		ReasonCode:    reasonPtr,
		ReasonDetail:  detailPtr,
		ActorType:     actorTypeSystem,
		CorrelationID: corrPtr,
		Now:           now,
	})
}

func (u *Issuance) appendHistory(ctx context.Context, subjectType string, subjectID int64, from, to, reasonCode, correlationID string, now time.Time) error {
	return appendStatusHistory(ctx, u.audit, subjectType, subjectID, from, to, reasonCode, correlationID, now)
}

func (u *Issuance) logf(ctx context.Context) *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}


func taxPeriodFor(documentKind string, invoiceDate time.Time) string {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return ""
	}
	return domain.TaxPeriodCode(invoiceDate)
}
