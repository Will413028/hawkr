package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)


const (
	receiptConsentMaxRunes = 32

	receiptIneligibleNotApproved     = "NOT_APPROVED"
	receiptIneligibleApprovalRevoked = "APPROVAL_REVOKED"
	receiptIneligibleApprovalExpired = "APPROVAL_EXPIRED"

	receiptBasisNotDesignated   = "NOT_DESIGNATED"
	receiptBasisUploaderNotSelf = "UPLOADER_NOT_SELLER"

	receiptReasonAuthorized = "SELLER_RECEIPT_AUTHORIZED"
	receiptReasonRevoked    = "SELLER_RECEIPT_AUTHORIZATION_REVOKED"
)

// 型別邊界錯誤（契約違規 ⇒ 400，不借業務碼）。
var (
	ErrReceiptConsentRequired = errors.New("invoice: consent_version is required when authorizing")
	ErrReceiptConsentTooLong  = errors.New("invoice: consent_version exceeds 32 characters")
)

// ReceiptAuthorizationView 是端點 28／29 的回應（proto SellerReceiptAuthorizationData）。
type ReceiptAuthorizationView struct {
	Eligible           bool
	IneligibleReason   string
	ApprovalBasisKey   string
	ApprovedAt         *time.Time
	ApprovalValidUntil *time.Time
	AuthorizedAt       *time.Time
	ConsentVersion     string
	// ReceiptTemplateKey 目前恆為空字串：平台自己的收據樣式範本**還沒有這個 asset**
	// （§6.1.9 只定義了它的語意與待遇，沒有指定 objectKey）。§25 要求只存／只回
	// objectKey，而現在沒有 key 可回 ⇒ 空字串是誠實值，不是佔位。
	// 有了範本之後唯一要改的是它的來源，形狀不動。
	ReceiptTemplateKey string
	Version            int64

	ApprovalBasisAssetID           *int64
	ApprovalBasisAvailable         bool
	ApprovalBasisUnavailableReason string
}

// GetSellerReceiptAuthorization 是端點 28（S05 讀）。
func (u *Query) GetSellerReceiptAuthorization(ctx context.Context, storeID int64) (ReceiptAuthorizationView, error) {
	issuer, err := u.sellerIssuer(ctx, storeID, bizcode.SellerTaxProfileNotFound)
	if err != nil {
		return ReceiptAuthorizationView{}, err
	}
	profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
	if err != nil {
		return ReceiptAuthorizationView{}, err
	}
	return buildReceiptAuthorizationView(profile, u.now()), nil
}

// ConfirmReceiptAuthorizationInput 是端點 29 的輸入。
type ConfirmReceiptAuthorizationInput struct {
	StoreID         int64
	ConsentVersion  string
	Authorized      bool
	ExpectedVersion int64
	IdempotencyKey  string
}

// ConfirmReceiptAuthorization 是端點 29（S05 寫）。
//
// 🔴 資格守衛在 SQL 的 WHERE（ConfirmSellerReceiptAuthorization 的 allowed-from），本層的
// pre-read 只負責把 rows=0 翻成**可分辨**的碼：資格不符 ⇒ 9335、版本被搶先 ⇒ 9322。
// 兩者的下一步不同（一個是去補件／等複審，一個是重新整理），共用一個碼等於沒有回答。
//
// ⚠️ authorized=false 是**取消授權**，不是錯誤：兩欄一起清成 NULL
// （chk_stp_receipt_auth_pair 要求同進同出，只清一欄當場 23514）。
func (u *TaxProfile) ConfirmReceiptAuthorization(
	ctx context.Context, in ConfirmReceiptAuthorizationInput,
) (ReceiptAuthorizationView, error) {
	if in.Authorized {
		if isBlank(in.ConsentVersion) {
			return ReceiptAuthorizationView{}, ErrReceiptConsentRequired
		}
		if utf8.RuneCountInString(in.ConsentVersion) > receiptConsentMaxRunes {
			return ReceiptAuthorizationView{}, ErrReceiptConsentTooLong
		}
	}

	var out ReceiptAuthorizationView
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, in.StoreID)
		if err != nil {
			return err
		}
		profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
		if err != nil {
			return err
		}
		if profile.Version != in.ExpectedVersion {
			return pkgerrors.NewBusiness(bizcode.SellerTaxProfileVersionConflict,
				bizcode.Message(bizcode.SellerTaxProfileVersionConflict))
		}
		now := u.now()
		view := buildReceiptAuthorizationView(profile, now)
		if !view.Eligible {
			return pkgerrors.NewBusiness(bizcode.InvoiceReceiptAuthNotEligible,
				bizcode.Message(bizcode.InvoiceReceiptAuthNotEligible))
		}

		cas := domain.ConfirmReceiptAuthorization{
			ID: profile.ID, ExpectedVersion: profile.Version, Now: now,
		}
		reasonCode := receiptReasonRevoked
		if in.Authorized {
			at := now
			ver := in.ConsentVersion
			cas.AuthorizedAt = &at
			cas.ConsentVersion = &ver
			reasonCode = receiptReasonAuthorized
		}
		ok, err := u.profiles.ConfirmReceiptAuthorization(ctx, cas)
		if err != nil {
			return err
		}
		if !ok {
			return pkgerrors.NewBusiness(bizcode.SellerTaxProfileVersionConflict,
				bizcode.Message(bizcode.SellerTaxProfileVersionConflict))
		}

		if err := u.audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
			SubjectType:    taxSubjectType,
			SubjectID:      profile.ID,
			FromStatus:     &profile.ReviewStatus,
			ToStatus:       profile.ReviewStatus,
			ReasonCode:     &reasonCode,
			ReasonDetail:   nilIfEmpty(receiptHistoryDetail(in)),
			ActorType:      taxActorSeller,
			ActorAccountID: &in.StoreID,
			Now:            now,
		}); err != nil {
			return err
		}

		updated, err := u.profiles.GetByID(ctx, profile.ID)
		if err != nil {
			return err
		}
		out = buildReceiptAuthorizationView(updated, now)
		return nil
	})
	if err != nil {
		return ReceiptAuthorizationView{}, err
	}
	return out, nil
}

func receiptHistoryDetail(in ConfirmReceiptAuthorizationInput) string {
	if !in.Authorized {
		return "revoked"
	}
	return "consentVersion=" + in.ConsentVersion
}

func buildReceiptAuthorizationView(p domain.SellerTaxProfile, now time.Time) ReceiptAuthorizationView {
	v := ReceiptAuthorizationView{
		ApprovedAt:         p.ApprovedAt,
		ApprovalValidUntil: p.ApprovalValidUntil,
		AuthorizedAt:       p.ReceiptAuthorizedAt,
		ConsentVersion:     derefString(p.ReceiptConsentVersion),
		Version:            p.Version,
	}
	v.Eligible, v.IneligibleReason = receiptEligibility(p, now)
	v.ApprovalBasisKey, v.ApprovalBasisAssetID,
		v.ApprovalBasisAvailable, v.ApprovalBasisUnavailableReason = receiptApprovalBasis(p)
	return v
}

var receiptAuthEligibleStatuses = map[string]bool{
	codemeta.SellerTaxReviewStatusSubmitted.Value:             true,
	codemeta.SellerTaxReviewStatusUnderReview.Value:           true,
	codemeta.SellerTaxReviewStatusSupplementRequired.Value:    true,
	codemeta.SellerTaxReviewStatusSelfIssuePending.Value:      true,
	codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value: true,
}

func receiptEligibility(p domain.SellerTaxProfile, now time.Time) (bool, string) {
	if p.ReviewStatus == codemeta.SellerTaxReviewStatusRevoked.Value {
		return false, receiptIneligibleApprovalRevoked
	}
	if p.ReviewStatus == codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value &&
		p.ApprovalValidUntil != nil && !now.Before(*p.ApprovalValidUntil) {
		return false, receiptIneligibleApprovalExpired
	}
	if !receiptAuthEligibleStatuses[p.ReviewStatus] {
		return false, receiptIneligibleNotApproved
	}
	return true, ""
}

func receiptApprovalBasis(p domain.SellerTaxProfile) (key string, assetID *int64, available bool, reason string) {
	if p.ReceiptApprovalBasisKey == nil || *p.ReceiptApprovalBasisKey == "" {
		return "", nil, false, receiptBasisNotDesignated
	}
	key = *p.ReceiptApprovalBasisKey
	if !containsEvidenceKey(p.EvidenceObjectKeys, key) {
		return key, nil, false, receiptBasisUploaderNotSelf
	}
	id := p.ReceiptApprovalBasisAssetID
	return key, id, true, ""
}

func containsEvidenceKey(raw json.RawMessage, key string) bool {
	if len(raw) == 0 || key == "" {
		return false
	}
	var keys []string
	if err := json.Unmarshal(raw, &keys); err != nil {
		return false
	}
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}
