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

// 本檔是 S05 免用統一發票的收據代製授權（§6.1.9，端點 28 讀／29 寫）。
//
// 兩件事在這裡定版：
//
//	🔴 **fail-closed 三態不得只有兩態**（§6.1.9 逐字）。`approvalBasisAvailable=false`
//	   時必須同時說明是哪一種（NOT_DESIGNATED／UPLOADER_NOT_SELLER），而且
//	   **不得回下載路徑**——回一個點下去必定 404 的連結，賣家會以為系統壞了而發客服單，
//	   真正的問題（ADM 沒指名／指名錯人）沒有任何人看得到。
//
//	🔴 **「上傳者是不是本賣家」在本服務就判得出來，不必打 media**：
//	   `seller_tax_profiles.evidence_object_keys` 就是這位賣家自己在 S02 上傳的清單。
//	   指名的 key 不在那份清單裡 ⇒ 它不是本賣家上傳的 ⇒ UPLOADER_NOT_SELLER。
//	   走 media 反而更差：讀取端點多一次跨服務呼叫，而 media 暫時不可用時整頁打不開。

const (
	// receiptConsentMaxRunes 對齊 seller_tax_profiles.receipt_consent_version varchar(32)。
	// 在這裡擋而不是讓 DB 擋：22001 對賣家是一句 500，本層才回得出「條款版本格式不正確」。
	receiptConsentMaxRunes = 32

	// ineligibleReason 的三個值（proto SellerReceiptAuthorizationData.ineligible_reason）。
	receiptIneligibleNotApproved     = "NOT_APPROVED"
	receiptIneligibleApprovalRevoked = "APPROVAL_REVOKED"
	receiptIneligibleApprovalExpired = "APPROVAL_EXPIRED"

	// approvalBasisUnavailableReason 的兩個值（§6.1.9 的 fail-closed 三態）。
	receiptBasisNotDesignated   = "NOT_DESIGNATED"
	receiptBasisUploaderNotSelf = "UPLOADER_NOT_SELLER"

	// invoice_admin_audit_logs／invoice_status_history 的 reason_code。
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
			// 9335：非免用統一發票核准賣家。**不是 400**——賣家的輸入沒有問題，
			// 是資格不符，而 UI 對這兩者顯示的訊息完全不同。
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
			// pre-read 已把資格不符翻成 9335 ⇒ 走到這裡的是版本被搶先。
			return pkgerrors.NewBusiness(bizcode.SellerTaxProfileVersionConflict,
				bizcode.Message(bizcode.SellerTaxProfileVersionConflict))
		}

		// 稽核：actor 是**賣家**不是 ADM ⇒ 走 invoice_status_history 而不是
		// invoice_admin_audit_logs（後者的 actor_account_id 語意是管理員）。
		// review_status 沒有變 ⇒ from/to 同值，記的是「這一刻發生了什麼」。
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

// receiptHistoryDetail 記下同意的條款版本（取消時為空）。
//
// 版本字串是爭議時唯一能舉證「他同意的是哪一版」的東西，而 seller_tax_profiles 是
// 就地更新、沒有歷程表 ⇒ 取消授權會把 receipt_consent_version 清成 NULL。
// 歷程表是 append-only，所以它必須記在這裡。
func receiptHistoryDetail(in ConfirmReceiptAuthorizationInput) string {
	if !in.Authorized {
		return "revoked"
	}
	return "consentVersion=" + in.ConsentVersion
}

// buildReceiptAuthorizationView 把一列 profile 折成端點 28／29 的回應。
//
// 純函式（除了 now）：端點 28 與 29 必須回**完全一樣**的形狀，各寫一份必然漂移。
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

// receiptAuthEligibleStatuses 是端點 29 的 allowed-from。
//
// 🔴 **必須與 db/queries/seller_tax_profiles.sql 的 ConfirmSellerReceiptAuthorization
// 那句 WHERE 逐值相同**。兩邊漂移的後果是不對稱的、而且都很難查：
//   - Go 這邊多列一個 ⇒ 賣家在 UI 上看得到、按得下去，SQL 回 rows=0 ⇒ 9335，
//     而 UI 剛剛才告訴他「可以簽」
//   - Go 這邊少列一個 ⇒ SQL 放行了但本層先擋，那個狀態的賣家**永遠簽不了**，
//     且沒有任何錯誤指向這裡
//
// TestReceiptAuthEligibleStatusesMatchSQL 直接解析那句 SQL 來比對，不手寫第二份清單。
//
// 為什麼是這五個：前四個是「審核進行中、還可能走向 APPROVED_INVOICE_EXEMPT」的狀態
// （DEC-20260905-015836 選 A：讓賣家在核定前就能簽，消滅核定後的未授權窗口）；
// 第五個是核定之後仍要能取消／再授權（授權是冪等開關）。
var receiptAuthEligibleStatuses = map[string]bool{
	codemeta.SellerTaxReviewStatusSubmitted.Value:             true,
	codemeta.SellerTaxReviewStatusUnderReview.Value:           true,
	codemeta.SellerTaxReviewStatusSupplementRequired.Value:    true,
	codemeta.SellerTaxReviewStatusSelfIssuePending.Value:      true,
	codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value: true,
}

// receiptEligibility 判「這位賣家能不能授權平台代製收據」。
//
// 三個 ineligible 值互斥且窮舉：
//
//	APPROVAL_REVOKED  —— 曾經核定過但被撤銷（REVOKED 是終態）
//	APPROVAL_EXPIRED  —— 仍是 APPROVED_INVOICE_EXEMPT，但 approval_valid_until 已過
//	                     （§5.1.2：屆滿**不改狀態**，所以只看 status 會誤判成仍有效）
//	NOT_APPROVED      —— 其餘全部（草稿／被駁回／逾期／核定為「應開發票」）
//
// ⚠️ APPROVAL_EXPIRED 只對 APPROVED_INVOICE_EXEMPT 成立：approval_valid_until 是
// 核定當下才寫的欄位，審核進行中的賣家它恆為 NULL，拿去判會把「還沒核定」誤報成「已過期」。
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

// receiptApprovalBasis 實作 §6.1.9 的 fail-closed 三態。
//
// 🔴 後兩態一律**不回 assetId**：Gateway 用它組 approvalBasisDownloadPath，回了就等於
// 給賣家一個點下去必定 404 的連結。
func receiptApprovalBasis(p domain.SellerTaxProfile) (key string, assetID *int64, available bool, reason string) {
	if p.ReceiptApprovalBasisKey == nil || *p.ReceiptApprovalBasisKey == "" {
		return "", nil, false, receiptBasisNotDesignated
	}
	key = *p.ReceiptApprovalBasisKey
	if !containsEvidenceKey(p.EvidenceObjectKeys, key) {
		// 指名的文件不在這位賣家自己上傳的清單裡 ⇒ media 的授權述詞（只有上傳者本人）
		// 不成立，賣家點下載一律 404。
		return key, nil, false, receiptBasisUploaderNotSelf
	}
	id := p.ReceiptApprovalBasisAssetID
	return key, id, true, ""
}

// containsEvidenceKey 判 objectKey 是否在賣家自己上傳的稅籍文件清單裡。
//
// 解析失敗一律回 false（fail-closed）：把「讀不出清單」當成「有」，會讓一個下載必定
// 404 的連結被標成可用。
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
